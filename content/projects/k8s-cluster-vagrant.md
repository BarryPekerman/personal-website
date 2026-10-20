# K8s-Cluster-Vagrant

https://github.com/BarryPekerman/K8s-Cluster-Vagrant

A multi-node Kubernetes cluster built from scratch on local VMs, the
bare-metal way.

PROBLEM
-------
Managed clusters (EKS, OKE) hide the parts worth understanding: how a control
plane and workers actually come up, how the container runtime and CNI get
wired in, how monitoring attaches. I wanted a cluster I bootstrapped myself,
reproducibly, without paying a cloud provider to learn.

BUILD
-----
- Vagrant provisions three VMs: one control plane and two workers
- kubeadm bootstraps the cluster; Ansible orchestrates every node so a full
  rebuild is one command, not a checklist
- containerd as the runtime, Calico for pod networking, and the
  kube-prometheus-stack for metrics and dashboards out of the box

RESULT
------
A disposable, from-zero Kubernetes cluster on a laptop that comes back
identically every time it is rebuilt -- a sandbox for learning cluster
internals and testing manifests with no cloud bill.

STACK: Vagrant, kubeadm, Ansible, containerd, Calico, kube-prometheus-stack
