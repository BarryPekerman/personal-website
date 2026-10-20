// Package k8s manages the throwaway pods that back k8s-mode shell sessions.
package k8s

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	// Namespace holds every visitor pod
	Namespace = "public-sessions"
	// ImageName is the guest image built from manifests/docker/Dockerfile.guest
	ImageName = "portfolio-guest:latest"

	// maxSessionSeconds is a hard lifetime for visitor pods, so pods the
	// backend failed to delete (e.g. it crashed) still go away on their own
	maxSessionSeconds = 30 * 60
)

// Client creates, execs into and deletes visitor pods
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	logger    *logrus.Logger
}

// PodExecOptions are the streams and command for ExecInPod
type PodExecOptions struct {
	Command []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	TTY     bool
}

// NewClient connects using the in-cluster config, falling back to
// ~/.kube/config for local development
func NewClient(logger *logrus.Logger) (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		clientset: clientset,
		config:    config,
		logger:    logger,
	}, nil
}

// CreateTerminalPod starts a locked-down pod for one session and waits until it is running
func (c *Client) CreateTerminalPod(ctx context.Context, sessionID string) (*corev1.Pod, error) {
	podName := fmt.Sprintf("terminal-%s", sessionID)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: Namespace,
			Labels: map[string]string{
				"app":        "portfolio-session",
				"session-id": sessionID,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyNever,
			ActiveDeadlineSeconds:        ptr(int64(maxSessionSeconds)),
			AutomountServiceAccountToken: ptr(false),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr(true),
				RunAsUser:    ptr(int64(1000)),
				FSGroup:      ptr(int64(1000)),
			},
			Containers: []corev1.Container{
				{
					Name:    "terminal",
					Image:   ImageName,
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", "while true; do sleep 30; done"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr(false),
						RunAsNonRoot:             ptr(true),
						RunAsUser:                ptr(int64(1000)),
						ReadOnlyRootFilesystem:   ptr(true),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "portfolio-content", MountPath: "/home/portfoliouser", ReadOnly: true},
						{Name: "tmp", MountPath: "/tmp"},
					},
					WorkingDir: "/home/portfoliouser",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "portfolio-content",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "portfolio-content"},
						},
					},
				},
				{
					Name:         "tmp",
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
			},
		},
	}

	if _, err := c.clientset.CoreV1().Pods(Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}
	c.logger.WithFields(logrus.Fields{
		"pod_name":   podName,
		"session_id": sessionID,
	}).Info("Created terminal pod")

	return c.waitForPodRunning(ctx, podName)
}

// DeletePod removes a pod immediately
func (c *Client) DeletePod(ctx context.Context, podName string) error {
	err := c.clientset.CoreV1().Pods(Namespace).Delete(ctx, podName, metav1.DeleteOptions{
		GracePeriodSeconds: ptr(int64(0)),
	})
	if err != nil {
		return fmt.Errorf("failed to delete pod %s: %w", podName, err)
	}

	c.logger.WithField("pod_name", podName).Info("Deleted terminal pod")
	return nil
}

// ExecInPod runs a command in the pod and streams its I/O until it exits or ctx is cancelled
func (c *Client) ExecInPod(ctx context.Context, podName string, options PodExecOptions) error {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: options.Command,
		Stdin:   options.Stdin != nil,
		Stdout:  options.Stdout != nil,
		Stderr:  options.Stderr != nil,
		TTY:     options.TTY,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  options.Stdin,
		Stdout: options.Stdout,
		Stderr: options.Stderr,
		Tty:    options.TTY,
	})
}

// waitForPodRunning polls until the pod is running, fails, or ctx expires
func (c *Client) waitForPodRunning(ctx context.Context, podName string) (*corev1.Pod, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		pod, err := c.clientset.CoreV1().Pods(Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod %s: %w", podName, err)
		}
		switch pod.Status.Phase {
		case corev1.PodRunning:
			return pod, nil
		case corev1.PodFailed:
			return nil, fmt.Errorf("pod %s failed to start", podName)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for pod %s to be running: %w", podName, ctx.Err())
		case <-ticker.C:
		}
	}
}

func ptr[T any](v T) *T {
	return &v
}
