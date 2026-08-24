# ADR 002: Kubernetes Integration and CRDs

## Status
Accepted

## Context
SentinelMesh needs an orchestration layer to map high-level Agent requirements (CPU, memory, priority, security constraints) to actual running sandboxed environments. Reinventing container orchestration, networking, and node management is out of scope for the MVP. 

## Decision
We will use **Kubernetes** as the foundational underlying infrastructure layer for the Data Plane and utilize the **Kubernetes Operator pattern**. 

We will define Custom Resource Definitions (CRDs) for `Agent` and `AgentRun`.
The `SentinelMesh Operator` (built via Kubebuilder/controller-runtime) will reconcile these high-level resources into native Kubernetes resources (Pods, Jobs, ConfigMaps, NetworkPolicies).

The Control Plane will interact with Kubernetes APIs to submit workloads, while SentinelMesh's own Scheduler and Runtime abstractions can wrap Kubernetes concepts, providing room for multi-cluster routing and specialized scheduling logic (e.g., node scoring).

## Consequences
**Positive:**
- Immediately leverages Kubernetes' robustness for node management, restarting pods, network isolation, and resource quotas.
- Uses standard declarative patterns which are well understood in the cloud-native ecosystem.
- Facilitates the multi-cluster goals natively.

**Negative:**
- Imposes a dependency on Kubernetes (requires `kind` or `minikube` locally).
- High learning curve for developers unfamiliar with operators.
