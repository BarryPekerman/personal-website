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
	Namespace = "public-sessions"
	ImageName = "portfolio-guest:latest"
)

type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	logger    *logrus.Logger
}

type PodExecOptions struct {
	Command   []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	TTY       bool
}

func NewClient() (*Client, error) {
	// Try in-cluster config first, then kubeconfig
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig for local development
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Client{
		clientset: clientset,
		config:    config,
		logger:    logger,
	}, nil
}

func (c *Client) CreateTerminalPod(sessionID string) (*corev1.Pod, error) {
	podName := fmt.Sprintf("terminal-%s", sessionID)
	
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: Namespace,
			Labels: map[string]string{
				"app":        "portfolio-terminal",
				"session-id": sessionID,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &[]bool{true}[0],
				RunAsUser:    &[]int64{1000}[0],
				FSGroup:      &[]int64{1000}[0],
			},
			Containers: []corev1.Container{
				{
					Name:  "terminal",
					Image: ImageName,
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", "while true; do sleep 30; done"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: &[]bool{false}[0],
						RunAsNonRoot:             &[]bool{true}[0],
						RunAsUser:                &[]int64{1000}[0],
						ReadOnlyRootFilesystem:   &[]bool{true}[0],
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
						{
							Name:      "portfolio-content",
							MountPath: "/home/portfoliouser",
							ReadOnly:  true,
						},
						{
							Name:      "tmp",
							MountPath: "/tmp",
						},
					},
					WorkingDir: "/home/portfoliouser",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "portfolio-content",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "portfolio-content",
							},
						},
					},
				},
				{
					Name: "tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	_, err := c.clientset.CoreV1().Pods(Namespace).Create(
		context.TODO(),
		pod,
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"pod_name":   podName,
		"session_id": sessionID,
	}).Info("Created terminal pod")

	// Wait for pod to be running
	return c.waitForPodRunning(podName, 30*time.Second)
}

func (c *Client) DeletePod(podName string) error {
	err := c.clientset.CoreV1().Pods(Namespace).Delete(
		context.TODO(),
		podName,
		metav1.DeleteOptions{
			GracePeriodSeconds: &[]int64{0}[0], // Force immediate deletion
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete pod %s: %w", podName, err)
	}

	c.logger.WithField("pod_name", podName).Info("Deleted terminal pod")
	return nil
}

func (c *Client) ExecInPod(podName string, options PodExecOptions) error {
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

	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  options.Stdin,
		Stdout: options.Stdout,
		Stderr: options.Stderr,
		Tty:    options.TTY,
	})
}

func (c *Client) waitForPodRunning(podName string, timeout time.Duration) (*corev1.Pod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for pod %s to be running", podName)
		default:
			pod, err := c.clientset.CoreV1().Pods(Namespace).Get(
				context.TODO(),
				podName,
				metav1.GetOptions{},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to get pod %s: %w", podName, err)
			}

			if pod.Status.Phase == corev1.PodRunning {
				return pod, nil
			}

			if pod.Status.Phase == corev1.PodFailed {
				return nil, fmt.Errorf("pod %s failed to start", podName)
			}

			time.Sleep(1 * time.Second)
		}
	}
}
