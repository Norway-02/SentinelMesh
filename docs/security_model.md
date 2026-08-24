# SentinelMesh Security Architecture & Threat Model (Stage 10)

## Executive Summary

SentinelMesh implements a defense-in-depth security model for autonomous AI agent execution. The architecture decouples **pure deterministic policy evaluation** from **infrastructure enforcement mechanisms** and **forensic audit pipelines**.

```
                     Agent Action
                          │
                          ▼
                 Pure Policy Engine (internal/policy)
                          │
                    ALLOW / DENY
                          │
             ┌────────────┴────────────┐
             ▼                         ▼
      Kubernetes Enforcer       Process Enforcer
    (internal/operator)       (internal/runtime)
             │                         │
             ▼                         ▼
      Real Containment          Real Containment
      - SecurityContext         - Setpgid / Process Groups
      - Seccomp RuntimeDefault  - Environment Scrubbing
      - Capabilities Drop ALL   - Working Directory Bounds
      - ReadOnly RootFS         - Timeout Termination
      - No ServiceAccount Token
      - NetworkPolicy Egress
             │                         │
             └────────────┬────────────┘
                          │
                          ▼
                     Audit Event
                          │
              ┌───────────┴───────────┐
              ▼                       ▼
         PostgreSQL             Outbox / NATS
    (security_audit_events)  (sentinel.security.v1.*)
```

---

## 1. Pure Policy Engine (`internal/policy`)

The policy engine is a pure domain evaluator with zero infrastructure dependencies (no Kubernetes, Linux APIs, PostgreSQL, or NATS imports).

### Profile-Driven Configuration
Security rules are organized into structured, composable `SecurityProfile` configurations:

```go
type SecurityProfile struct {
    Name       ProfileName
    Filesystem FilesystemPolicy
    Network    NetworkPolicy
    Syscalls   SyscallPolicy
    Resources  ResourcePolicy
    Tools      ToolPolicy
}
```

### Profile Matrix

| Attribute | Standard | Restricted | Confidential |
| :--- | :--- | :--- | :--- |
| **Filesystem Write** | `/workspace`, `/tmp` | `/workspace` | `/workspace/secure` |
| **Filesystem Deny** | `/etc/shadow`, `/etc/sudoers`, `/root`, `/var/run/docker.sock` | `/etc/**`, `/root/**`, `/proc/kcore`, `/var/run/**` | `/**` (all except `/workspace/secure`) |
| **Root Filesystem** | Read-Only | Read-Only | Read-Only |
| **Network Egress** | Public Internet (ports 80, 443, 8080) | Internal VPC (`10.0.0.0/16`) on ports 443, 5432, 4222 | Zero Egress (Default Deny All) |
| **Denied CIDRs** | `169.254.169.254/32` (Metadata), `10.0.0.0/8` (VPC) | `169.254.169.254/32`, `10.0.0.0/24` | All CIDRs |
| **Seccomp Profile** | `RuntimeDefault` | `RuntimeDefault` | `RuntimeDefault` |
| **Denied Syscalls** | `ptrace`, `bpf`, `reboot`, `kexec_load`, `mount` | `ptrace`, `bpf`, `reboot`, `mount`, `chroot`, `init_module` | `ptrace`, `bpf`, `reboot`, `mount`, `setuid`, `setgid`, `socket` |
| **CPU Limit** | 2.0 Cores | 1.0 Cores | 0.5 Cores |
| **Memory Limit** | 2048 MB | 1024 MB | 512 MB |
| **Tool Whitelist** | All (`*`) | `read_file`, `write_file`, `code_exec` | `secure_compute` |

### Path Normalization & Traversal Prevention
Every filesystem path is processed through a strict normalization algorithm:
1. `filepath.Clean(rawPath)` to resolve `..`, `.`, and duplicate slashes.
2. Canonical absolute path conversion (`/` prefix if relative).
3. Explicit match against denied paths and prefixes.
4. Read-only verification for write/delete/create operations.
5. Whitelist verification against allowed paths.
6. Default deny.

### Network Rule Scoping
Stage 10 network policy evaluation strictly evaluates **CIDR + Port + Protocol** tuples (e.g. `10.0.0.0/8:5432`, `0.0.0.0/0:443`). SentinelMesh does not claim hostname/DNS-level inspection at this layer; DNS and hostname proxying are reserved for egress proxy integrations.

---

## 2. Kubernetes Security Hardening (`internal/operator`)

For production deployments, the Kubernetes reconciler configures hardened Pod specifications:

```yaml
spec:
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 10001
    runAsGroup: 10001
    fsGroup: 10001
    seccompProfile:
      type: RuntimeDefault
  volumes:
    - name: workspace
      emptyDir: {}
    - name: tmp
      emptyDir: {}
  containers:
    - name: agent
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop:
            - ALL
      volumeMounts:
        - name: workspace
          mountPath: /workspace
        - name: tmp
          mountPath: /tmp
      resources:
        requests:
          cpu: "500m"
          memory: "512Mi"
        limits:
          cpu: "1"
          memory: "1Gi"
```

### Containment Mechanisms
1. **Disabled Service Account Token**: Prevents compromised agents from exfiltrating Kubernetes API tokens.
2. **Non-Root & Dropped Capabilities**: Agents run as unprivileged UID 10001 with `CAP_SYS_ADMIN`, `CAP_NET_ADMIN`, etc. completely dropped.
3. **Read-Only Root Filesystem**: Prevents binary tampering or backdoor installation in system paths (`/bin`, `/lib`, `/usr`).
4. **Writable `/workspace` Volume**: Isolate scratch data to an emptyDir ephemeral volume.
5. **RuntimeDefault Seccomp**: Blocks dangerous Linux kernel syscalls by default.
6. **NetworkPolicies**: Kubernetes NetworkPolicy objects restrict Pod egress according to profile CIDRs and ports.

---

## 3. Process Sandbox Boundary (`internal/runtime`)

SentinelMesh explicitly distinguishes local process execution from container execution:

- **`ProcessRuntime`**: Provides **best-effort local containment** (process grouping via `Setpgid`, environment isolation, timeout enforcement, signal handling). ProcessRuntime does NOT provide multi-tenant isolation.
- **`KubernetesRuntime`**: Serves as the **primary production isolation boundary** providing kernel namespace, cgroup, seccomp, and network isolation.

---

## 4. Forensic Audit & Outbox Integration

Security audit events are durably persisted to PostgreSQL via migration `000005_security_audit_events.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS security_audit_events (
    id VARCHAR(64) PRIMARY KEY,
    run_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    correlation_id VARCHAR(64) NOT NULL,
    source VARCHAR(64) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    operation VARCHAR(64) NOT NULL,
    resource TEXT NOT NULL,
    decision VARCHAR(32) NOT NULL,
    rule_id VARCHAR(64) NOT NULL,
    reason TEXT NOT NULL,
    severity VARCHAR(32) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB
);
```

### Violation Event Pipeline
When a policy evaluation denies an action, or when runtime containment triggers:
1. An audit record is written with correlation ID and source (`policy-engine`, `process-enforcer`, `kubernetes-enforcer`, `runtime`).
2. A `sentinel.security.v1.policy_violation` or `sentinel.security.v1.sandbox_violation` event is enqueued in the transactional outbox.
3. Outbox publisher broadcasts the event across NATS for security operations monitoring.

---

## 5. Threat Model & Residual Risk

### Threat Vectors & Mitigations

| Threat | Attack Vector | Mitigation Layer | Observed Enforcement |
| :--- | :--- | :--- | :--- |
| **Path Traversal** | `/workspace/../etc/shadow` | Policy Engine | Operation Denied (`fs-denied-path`) |
| **Sensitive File Access** | `/root/.ssh/id_rsa`, `/var/run/docker.sock` | Policy Engine & ReadOnly RootFS | Operation Denied & Container FS Read-Only |
| **Metadata Exfiltration** | `169.254.169.254:80` | Policy Engine & K8s NetworkPolicy | Operation Denied (`net-denied-cidr`) & Network Drop |
| **VPC Lateral Movement** | Scanning internal subnets from standard agent | Policy Engine & K8s NetworkPolicy | Operation Denied (`net-denied-cidr`) |
| **Kernel Exploitation** | `ptrace`, `bpf`, `kexec_load` | Policy Engine & RuntimeDefault Seccomp | Operation Denied & Syscall Filtered |
| **Privilege Escalation** | `setuid`, `sudo`, binary modification | SecurityContext (`allowPrivilegeEscalation: false`, read-only root) | Kernel blocked (`EPERM` / `EROFS`) |
| **Resource Starvation** | Infinite loops / memory leak | Kubernetes Requests + Limits & Process Timeout | CPU Throttling / Container OOM Termination |

### Residual Risks & Scoped Boundaries
1. **0-day Linux Kernel Vulnerabilities**: Container isolation shares the host kernel. Hardened seccomp profiles reduce the attack surface, but kernel zero-days remain a residual risk mitigated by node isolation and VM boundaries (e.g. Kata/gVisor in future stages).
2. **DNS Exfiltration**: Hostname-level tunneling is not blocked by Layer-4 CIDR filtering. Egress proxy inspection is planned for advanced stages.
3. **ProcessRuntime**: Local OS process containment does not prevent local kernel reads if run as root; developers must use `KubernetesRuntime` for untrusted workloads.

### Defensible Security Verdict
> **"SentinelMesh successfully prevented the defined forbidden operations in the declared test environment and recorded corresponding audit events."**
