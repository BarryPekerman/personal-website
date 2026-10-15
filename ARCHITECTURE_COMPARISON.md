# Architecture Comparison: Pod-per-User vs Process-per-User

## Executive Summary

This document compares two architectural approaches for implementing an interactive terminal portfolio:

1. **Pod-per-User** (Current Implementation): Each user session spawns an isolated Kubernetes pod
2. **Process-per-User** (Proposed SSH Model): Single pod spawns individual shell processes per user

---

## 1. Architecture Complexity

### Pod-per-User Approach

**Complexity: HIGH**

**Components Required:**
- Kubernetes client-go library integration
- ServiceAccount with RBAC permissions
- Pod creation/deletion orchestration
- Pod lifecycle management (waiting for Running state)
- `kubectl exec` via SPDY protocol for shell access
- ConfigMap mounting for content
- NetworkPolicy for isolation
- Resource quotas and limits
- Pod cleanup logic on disconnect

**Code Complexity:**
- ~200+ lines of Kubernetes client code
- Complex error handling for pod states
- Requires deep Kubernetes API knowledge
- Multiple failure points (pod creation, exec connection, cleanup)

**Dependencies:**
```go
k8s.io/client-go/kubernetes
k8s.io/client-go/tools/remotecommand
k8s.io/api/core/v1
```

### Process-per-User Approach

**Complexity: LOW**

**Components Required:**
- PTY library (`github.com/creack/pty`)
- WebSocket handler
- Process spawning (`exec.Command`)
- Standard Go stdlib only

**Code Complexity:**
- ~100 lines of core logic
- Simple process management
- Standard Go patterns
- Fewer failure points

**Dependencies:**
```go
github.com/creack/pty
github.com/gorilla/websocket
```

**Winner: Process-per-User** ⭐ (Significantly simpler)

---

## 2. Security Model

### Pod-per-User Approach

**Isolation Level: CONTAINER-LEVEL**

**Security Features:**
- ✅ Complete container isolation (each user in separate pod)
- ✅ Kernel-level namespace isolation (PID, network, filesystem)
- ✅ NetworkPolicy prevents internet access
- ✅ Resource limits per pod (CPU, memory)
- ✅ Read-only root filesystem per pod
- ✅ Non-root execution enforced
- ✅ Capabilities dropped
- ✅ Pod deletion on disconnect (clean slate)

**Attack Surface:**
- Each pod has its own kernel namespace
- User cannot affect other users' containers
- Container escape would only affect one user's pod
- Kubernetes API server is attack surface (if compromised)

**Security Concerns:**
- Pod creation requires Kubernetes API access
- ServiceAccount token could be compromised
- RBAC misconfiguration could allow privilege escalation
- Pod creation/deletion operations are privileged

### Process-per-User Approach

**Isolation Level: PROCESS-LEVEL**

**Security Features:**
- ✅ Read-only root filesystem (via Kubernetes Deployment)
- ✅ NetworkPolicy prevents internet access (same as pod approach)
- ✅ Resource limits on entire pod (shared across users)
- ✅ Non-root execution
- ✅ Process limits enforced by kernel (PIDs, CPU, memory)
- ✅ All users share same container (no cross-user isolation)

**Attack Surface:**
- All users share same container filesystem (except /tmp)
- Process-level isolation only (no kernel namespaces)
- User could potentially see other users' processes
- Fork bombs affect all users (but limited by pod resources)
- Container escape affects all users

**Security Concerns:**
- No isolation between users (same container)
- User could potentially interfere with other users' processes
- Shared /tmp directory (though emptyDir is ephemeral)
- Process list visible to all users (`ps aux`)
- No automatic cleanup of user processes (relies on WebSocket disconnect)

**Winner: Pod-per-User** ⭐ (Superior isolation, but Process-per-User is acceptable for portfolio use case)

---

## 3. Performance Characteristics

### Pod-per-User Approach

**Startup Time:**
- Pod creation: 2-5 seconds (image pull, container start, health checks)
- Pod scheduling: 1-3 seconds (depends on cluster load)
- Exec connection: 0.5-1 second
- **Total: 3-9 seconds** before user can type

**Resource Overhead:**
- Per-pod overhead: ~50-100MB memory (container runtime, kernel namespaces)
- Per-pod CPU: ~10-50m (idle container overhead)
- Network overhead: Pod IP allocation, CNI setup
- Storage: Container image layers (shared but cached)

**Latency:**
- WebSocket → Go → Kubernetes API → Pod Exec → Shell: **Higher latency**
- Multiple network hops (API server, kubelet, container runtime)

**Scalability:**
- Limited by cluster resources (nodes, pods per node)
- Each user consumes full pod resources
- Kubernetes API server becomes bottleneck at scale

### Process-per-User Approach

**Startup Time:**
- Process spawn: <10 milliseconds
- PTY creation: <5 milliseconds
- **Total: <50 milliseconds** before user can type

**Resource Overhead:**
- Per-process overhead: ~1-5MB memory (just the shell process)
- Per-process CPU: ~1-5m (minimal)
- No network overhead (local process)
- No storage overhead (process only)

**Latency:**
- WebSocket → Go → PTY → Shell: **Minimal latency**
- Single process, no network hops

**Scalability:**
- Limited by pod resources (CPU, memory, PIDs)
- Can handle 100+ concurrent users in single pod
- No Kubernetes API bottleneck

**Winner: Process-per-User** ⭐ (Dramatically faster startup, lower overhead)

---

## 4. Resource Efficiency

### Pod-per-User Approach

**Memory Usage:**
- Base pod: ~64MB (container + runtime)
- Per user: ~64MB minimum
- 10 concurrent users: ~640MB
- 100 concurrent users: ~6.4GB (would need multiple nodes)

**CPU Usage:**
- Base pod: ~50m CPU
- Per user: ~50m CPU minimum
- 10 concurrent users: ~500m CPU
- 100 concurrent users: ~5 CPU cores

**Storage:**
- Container image: ~50-100MB (shared across pods)
- ConfigMap: ~1-5MB per pod mount
- 100 users: ~100-500MB ConfigMap storage

**Network:**
- 100 pod IPs allocated
- CNI overhead per pod

### Process-per-User Approach

**Memory Usage:**
- Base pod: ~128MB (container + Go server)
- Per user: ~2-5MB (shell process)
- 10 concurrent users: ~150MB total
- 100 concurrent users: ~600MB total

**CPU Usage:**
- Base pod: ~100m CPU (Go server)
- Per user: ~1-5m CPU
- 10 concurrent users: ~150m CPU total
- 100 concurrent users: ~600m CPU total

**Storage:**
- Container image: ~50-100MB (single pod)
- Content: ~1-5MB (mounted once)
- 100 users: ~1-5MB total (shared)

**Network:**
- Single pod IP
- Minimal CNI overhead

**Winner: Process-per-User** ⭐ (10-20x more efficient)

---

## 5. Scalability

### Pod-per-User Approach

**Horizontal Scaling:**
- ✅ Can scale across multiple nodes
- ✅ Kubernetes handles load distribution
- ❌ Limited by cluster capacity
- ❌ API server becomes bottleneck (1000+ pods)

**Vertical Scaling:**
- ❌ Each pod has fixed resource limits
- ❌ Cannot dynamically adjust per-user resources
- ✅ Can increase cluster size

**Concurrent Users:**
- Practical limit: ~100-500 pods per cluster (depends on cluster size)
- Requires multiple nodes for high concurrency
- Pod scheduling delays increase with load

### Process-per-User Approach

**Horizontal Scaling:**
- ✅ Can run multiple pods (replicas)
- ✅ Load balancer distributes connections
- ✅ Each pod handles 100+ users independently
- ✅ No Kubernetes API bottleneck

**Vertical Scaling:**
- ✅ Single pod can handle many users
- ✅ Resource limits apply to entire pod (shared)
- ✅ Can adjust pod resources based on load

**Concurrent Users:**
- Practical limit: ~200-500 users per pod (depends on resources)
- 10 pods = 2000-5000 concurrent users
- No scheduling delays (process spawn is instant)

**Winner: Process-per-User** ⭐ (Better scalability characteristics)

---

## 6. Development & Maintenance

### Pod-per-User Approach

**Development Complexity:**
- Requires Kubernetes cluster for testing
- Complex error handling (pod states, API errors)
- Difficult to debug (multiple components)
- Requires kubectl access for troubleshooting

**Maintenance Burden:**
- Kubernetes API version compatibility
- RBAC policy management
- ServiceAccount token rotation
- Pod cleanup logic (zombie pods)
- Monitoring pod health
- Debugging pod creation failures

**Testing:**
- Requires full Kubernetes cluster
- Cannot test locally without minikube/kind
- Integration tests are complex
- Mocking Kubernetes API is difficult

**Deployment:**
- Requires Kubernetes cluster
- Complex manifest management
- RBAC setup required
- ServiceAccount creation

### Process-per-User Approach

**Development Complexity:**
- Simple Go application
- Easy to test locally (no cluster needed)
- Standard debugging tools work
- Minimal error handling

**Maintenance Burden:**
- Standard Go application maintenance
- PTY library updates (rare)
- WebSocket library updates (rare)
- Process cleanup (handled by Go runtime)

**Testing:**
- Can test on any machine
- Simple unit tests
- Easy to mock dependencies
- Fast test execution

**Deployment:**
- Standard container deployment
- Simple Dockerfile
- No Kubernetes-specific requirements
- Works on any container runtime

**Winner: Process-per-User** ⭐ (Much easier to develop and maintain)

---

## 7. Reliability & Failure Modes

### Pod-per-User Approach

**Failure Points:**
1. Kubernetes API server unavailable
2. Pod creation fails (resource exhaustion, image pull failure)
3. Pod scheduling delays
4. Exec connection failures
5. Pod deletion failures (zombie pods)
6. NetworkPolicy misconfiguration
7. RBAC permission issues

**Recovery:**
- Pod failures are isolated (one user affected)
- Automatic retry logic needed
- Requires monitoring and alerting
- Complex failure scenarios

**User Experience:**
- Users wait 3-9 seconds for pod creation
- Connection failures are frustrating
- Retry logic needed in frontend

### Process-per-User Approach

**Failure Points:**
1. Process spawn failure (rare, resource exhaustion)
2. PTY creation failure (rare)
3. WebSocket disconnection
4. Pod crash (affects all users)

**Recovery:**
- Process failures are isolated (one user affected)
- Pod crash affects all users (but pod restarts automatically)
- Simple retry logic
- Fewer failure modes

**User Experience:**
- Instant connection (<50ms)
- Fewer failure scenarios
- Better user experience

**Winner: Process-per-User** ⭐ (Fewer failure points, faster recovery)

---

## 8. Use Case Fit Analysis

### Portfolio Website Requirements

**Typical Usage:**
- Low to moderate concurrent users (10-50 typical, 100+ peak)
- Short session duration (2-5 minutes average)
- Read-only exploration (viewing CV, projects)
- No sensitive data processing
- Demonstrating technical skills

**Security Requirements:**
- Prevent malicious commands
- Prevent resource exhaustion
- Prevent internet access
- **Do NOT need** complete user isolation (not a multi-tenant SaaS)

**Performance Requirements:**
- Fast initial connection (good UX)
- Responsive terminal
- Low resource usage (cost-effective)

### Pod-per-User: Fit Assessment

**Overkill for Portfolio Use Case:**
- ❌ Complete isolation unnecessary (not multi-tenant SaaS)
- ❌ Complex setup for simple use case
- ❌ Slow startup hurts UX
- ❌ High resource overhead
- ✅ Excellent security (but excessive)
- ✅ Good for demonstrating K8s skills

**Best For:**
- Multi-tenant SaaS platforms
- Untrusted user code execution
- Compliance requirements (HIPAA, SOC2)
- Enterprise security requirements

### Process-per-User: Fit Assessment

**Perfect Fit for Portfolio Use Case:**
- ✅ Sufficient security (read-only FS, network isolation, resource limits)
- ✅ Fast startup (excellent UX)
- ✅ Low resource overhead (cost-effective)
- ✅ Simple implementation (easier to maintain)
- ✅ Still demonstrates technical skills (PTY, WebSockets, security)
- ⚠️ Less isolation (but acceptable for portfolio)

**Best For:**
- Portfolio websites
- Interactive demos
- Educational terminals
- Low-risk interactive applications
- Cost-sensitive deployments

**Winner: Process-per-User** ⭐ (Better fit for portfolio use case)

---

## 9. Cost Analysis

### Pod-per-User Approach

**Infrastructure Costs:**
- Higher memory usage (64MB per user)
- Higher CPU usage (50m per user)
- More pod IPs (if limited)
- Kubernetes API server load

**Example (100 concurrent users):**
- Memory: ~6.4GB
- CPU: ~5 cores
- Estimated cost: $50-100/month (cloud provider)

### Process-per-User Approach

**Infrastructure Costs:**
- Lower memory usage (2-5MB per user)
- Lower CPU usage (1-5m per user)
- Single pod IP
- Minimal API server load

**Example (100 concurrent users):**
- Memory: ~600MB
- CPU: ~600m
- Estimated cost: $10-20/month (cloud provider)

**Winner: Process-per-User** ⭐ (5-10x cost savings)

---

## 10. Feature Comparison

| Feature | Pod-per-User | Process-per-User |
|---------|--------------|------------------|
| User Isolation | ✅ Complete (container-level) | ⚠️ Process-level only |
| Startup Time | ❌ 3-9 seconds | ✅ <50ms |
| Resource Efficiency | ❌ High overhead | ✅ Low overhead |
| Scalability | ⚠️ Limited by cluster | ✅ Excellent |
| Development Complexity | ❌ High | ✅ Low |
| Maintenance Burden | ❌ High | ✅ Low |
| Security Level | ✅ Enterprise-grade | ✅ Good (sufficient) |
| Cost | ❌ Higher | ✅ Lower |
| Kubernetes Skills Demo | ✅ Excellent | ⚠️ Moderate |
| User Experience | ❌ Slow connection | ✅ Instant connection |
| Failure Recovery | ⚠️ Complex | ✅ Simple |

---

## 11. Recommendation

### For Portfolio Website: **Process-per-User** ⭐

**Rationale:**
1. **Better User Experience**: Instant connection vs 3-9 second wait
2. **Sufficient Security**: Read-only FS + NetworkPolicy + resource limits provide adequate protection
3. **Cost Effective**: 5-10x lower resource usage
4. **Simpler Maintenance**: Easier to develop, test, and maintain
5. **Better Fit**: Portfolio doesn't need enterprise-grade isolation
6. **Still Impressive**: Demonstrates PTY, WebSockets, security hardening

### For Enterprise/Multi-Tenant: **Pod-per-User** ⭐

**Rationale:**
1. **Complete Isolation**: Required for untrusted users
2. **Compliance**: Meets regulatory requirements
3. **Security**: Kernel-level isolation prevents cross-user attacks
4. **Auditability**: Each user has isolated audit trail

---

## 12. Hybrid Approach (Best of Both Worlds)

Consider a **hybrid approach** for maximum flexibility:

1. **Default**: Process-per-User for normal users (fast, efficient)
2. **Premium/Demo Mode**: Pod-per-User for specific demonstrations
3. **Configuration Flag**: Switch between modes via environment variable

This allows:
- Fast UX for most users
- K8s orchestration demo when needed
- Cost optimization
- Flexibility to showcase both approaches

---

## Conclusion

For a **portfolio website**, the **Process-per-User (SSH Model)** approach is superior because:

1. ✅ **10-20x faster** connection time
2. ✅ **5-10x lower** resource usage
3. ✅ **Sufficient security** for the use case
4. ✅ **Simpler** to develop and maintain
5. ✅ **Better user experience**
6. ✅ **Lower cost**

The Pod-per-User approach is excellent for demonstrating Kubernetes expertise, but it's **over-engineered** for a portfolio website where users are just exploring your CV and projects.

**Recommendation: Migrate to Process-per-User architecture** while keeping Pod-per-User code as a reference or optional demo mode.



