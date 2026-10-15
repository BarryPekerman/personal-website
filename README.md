# Portfolio Terminal

A secure, Kubernetes-native terminal portfolio that demonstrates cloud-native engineering skills through an interactive terminal experience.

## Overview

This project showcases a portfolio website where visitors interact through a web-based terminal interface. Users can navigate using standard Linux commands (ls, cat, grep) to explore CV information, project details, and contact information - all running in isolated Kubernetes pods.

## Architecture

### Core Components

- **Go Backend**: WebSocket server using Gin framework and Kubernetes client-go
- **Frontend**: xterm.js-based terminal emulator with WebSocket connectivity
- **Kubernetes**: Isolated namespace with strict security policies
- **Container Images**: Alpine-based containers with minimal attack surface

### Security Features

- **Pod Isolation**: Each user session runs in a separate, ephemeral pod
- **Network Policies**: Deny-all egress, preventing internet access
- **Resource Quotas**: Hard limits on CPU, memory, and pod count
- **Security Context**: Non-root execution, dropped capabilities, read-only filesystem
- **Command Filtering**: Blocks dangerous commands and resource exhaustion attacks
- **RBAC**: Least-privilege service account permissions

## Quick Start

### Prerequisites

- Kubernetes cluster (K3s recommended)
- kubectl configured
- Docker for building images
- Go 1.21+ (for local development)

### Deployment

1. **Clone the repository**
   ```bash
   git clone https://github.com/your-username/portfolio-terminal.git
   cd portfolio-terminal
   ```

2. **Update image references**
   Edit `k8s/deployment.yaml` and `internal/k8s/client.go` to use your registry:
   ```yaml
   image: ghcr.io/your-username/portfolio-terminal:latest-backend
   ```
   ```go
   Image: "ghcr.io/your-username/portfolio-terminal:latest-terminal"
   ```

3. **Deploy to Kubernetes**
   ```bash
   # Apply all manifests
   kubectl apply -f k8s/

   # Or use the ArgoCD application
   kubectl apply -f argocd/portfolio-app.yaml
   ```

4. **Access the application**
   ```bash
   # Get the service URL
   kubectl get svc -n public-sessions
   ```

## Usage

### For Visitors

1. Open the portfolio URL in a web browser
2. You'll see a terminal prompt: `guest@portfolio:~$`
3. Use standard Linux commands to explore:
   ```bash
   ls                    # List available files
   cat cv.txt           # View CV
   cat contact_info.md  # View contact information
   projects/            # Explore project directory
   help                 # Show available commands
   ```

### Available Commands

- `cv` / `resume` - View CV
- `contact` - View contact information
- `projects` - List projects
- `cat` - View file contents
- `less` - View files with paging
- `ls` / `ll` - List directory contents
- `help` - Show help

## Development

### Local Development

1. **Start minikube or kind**
   ```bash
   minikube start
   ```

2. **Build and load images**
   ```bash
   # Build backend
   docker build -f manifests/docker/Dockerfile.backend -t portfolio-backend:latest .

   # Build terminal image
   docker build -f manifests/docker/Dockerfile -t portfolio-terminal:latest .

   # Load into minikube
   minikube image load portfolio-backend:latest
   minikube image load portfolio-terminal:latest
   ```

3. **Deploy manifests**
   ```bash
   kubectl apply -f k8s/
   ```

4. **Port forward**
   ```bash
   kubectl port-forward -n public-sessions svc/portfolio-backend 8080:80
   ```

5. **Access at http://localhost:8080**

### Project Structure

```
.
├── cmd/server/           # Go application entry point
├── internal/
│   ├── handlers/         # HTTP/WebSocket handlers
│   ├── k8s/             # Kubernetes client operations
│   └── websocket/       # Terminal WebSocket management
├── k8s/                 # Kubernetes manifests
├── manifests/docker/    # Dockerfiles
├── web/                 # Frontend assets
│   ├── static/          # CSS, JS files
│   └── templates/       # HTML templates
├── content/             # Portfolio content (CV, projects)
└── argocd/              # GitOps configurations
```

## Security Considerations

### Command Filtering

The system blocks dangerous commands including:
- System destruction: `rm -rf /`, `sudo rm /`
- Resource exhaustion: `dd if=/dev/zero`, fork bombs
- Privilege escalation: `sudo`, `su`
- Network tools: `nmap`, `netcat`
- System information disclosure: `cat /etc/passwd`

### Resource Protection

- **Pod Limits**: 100m CPU, 128Mi memory per pod
- **Namespace Quota**: Max 10 pods, 1 CPU, 2Gi memory
- **Network Isolation**: No internet access from pods
- **Automatic Cleanup**: Pods deleted immediately on disconnect

### Monitoring

- Comprehensive logging of suspicious activities
- Resource usage tracking
- Failed authentication attempts
- Command execution patterns

## CI/CD

The project includes GitHub Actions for:
- Automated testing (Go tests, vetting)
- Multi-stage Docker builds
- Container registry publishing
- Kubernetes deployment
- Rollback capabilities

## GitOps with ArgoCD

Content updates are managed through GitOps:
- CV and project data stored in ConfigMap
- Automatic deployment on content changes
- Rollback capability for content versions

## Customization

### Updating Content

1. Edit files in `content/` directory
2. Update the ConfigMap in `k8s/configmap.yaml`
3. Commit and push to trigger ArgoCD sync

### Adding Projects

1. Create new markdown files in `content/projects/`
2. Update the ConfigMap data section
3. Include GitHub repository links

### Modifying Commands

Edit `internal/handlers/terminal.go` to add new aliases or modify the help system.

## Troubleshooting

### Common Issues

1. **Pods fail to start**: Check resource quotas and image availability
2. **WebSocket connection fails**: Verify service account permissions
3. **Commands not working**: Check ConfigMap mounting and file permissions
4. **High resource usage**: Monitor pod logs and adjust resource limits

### Logs

```bash
# Backend logs
kubectl logs -n public-sessions deployment/portfolio-backend

# Terminal pod logs
kubectl logs -n public-sessions -l app=portfolio-terminal
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Submit a pull request

## License

This project demonstrates enterprise-grade Kubernetes and security practices for portfolio purposes.

---

**Built with**: Go, Kubernetes, Docker, WebSockets, xterm.js
**Security**: Pod isolation, network policies, RBAC, command filtering
**Scalability**: Resource quotas, horizontal pod autoscaling ready



