# Project Overview

## Purpose

This is an interactive portfolio website that presents professional information (CV, projects, contact details) through a web-based Linux terminal interface. Instead of traditional web pages, visitors explore the portfolio using standard command-line tools like `ls`, `cat`, and `grep` - creating a unique, memorable experience that demonstrates technical skills while showcasing professional background. The project serves both as a functional portfolio and a demonstration of cloud-native engineering capabilities, including Kubernetes orchestration, container security, and real-time WebSocket communication.

## Architecture

The system follows a three-tier architecture: a **Go web server** handles HTTP/WebSocket connections from browsers, a **Kubernetes cluster** orchestrates isolated container sessions, and **Alpine Linux containers** provide the actual terminal environment with BusyBox shell and portfolio content. When a visitor connects, the Go backend creates an ephemeral Kubernetes pod containing an Alpine container with the portfolio files (CV, projects) mounted via ConfigMap. The Go server then establishes a WebSocket connection with the browser (using xterm.js for terminal rendering) and pipes data bidirectionally between the browser and a shell process running inside the pod via `kubectl exec`. This architecture ensures each visitor gets complete isolation in their own container while maintaining security through network policies (no internet access), resource quotas, read-only filesystems, and command filtering that blocks dangerous operations. The entire system is deployed on Kubernetes with GitOps workflows, allowing content updates to propagate automatically through ArgoCD.



