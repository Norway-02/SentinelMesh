# ADR 001: Strict Control Plane and Data Plane Separation

## Status
Accepted

## Context
SentinelMesh must handle dynamic, bursty, and long-running AI agent workloads across multiple nodes and clusters securely. If orchestration, scheduling, and execution logic are tightly coupled, the system becomes difficult to scale, secure, and debug. 

## Decision
We will enforce a strict architectural separation between the **Control Plane** and the **Data Plane**.

- **Control Plane**: Acts as the "brain". Responsible for API serving, metadata storage, scheduling decisions, policy evaluation, and verification. The Control Plane runs globally or per-cluster and does not run any untrusted workload code.
- **Data Plane**: Acts as the "muscle". Responsible for executing agent workloads securely via sandboxes, managing the execution runtime, collecting telemetry, and handling checkpoints. It blindly follows instructions from the Control Plane but enforces security boundaries via seccomp, eBPF, and cgroups.

Communication between these planes will happen asynchronously via **NATS JetStream** (Event Bus) and synchronously via well-defined gRPC interfaces where immediate responses (e.g., policy evaluation) are required.

## Consequences
**Positive:**
- Horizontal scalability: We can scale the Control Plane independently of the Data Plane.
- Security isolation: A compromised agent in the Data Plane has no direct access to the Control Plane's database or internal logic, except through tightly controlled APIs.
- Manageability: Different teams or modules can focus on scheduling vs. sandbox execution.

**Negative:**
- Increased architectural complexity.
- Network latency between Control Plane and Data Plane must be managed.
- State synchronization requires eventual consistency handling (e.g., via events and reconciliation).
