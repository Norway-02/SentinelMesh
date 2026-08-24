# ADR-007: Cross-Cluster Execution Authority and Workload Fencing

## Status
Accepted

## Context
In a distributed, multi-cluster execution environment (such as SentinelMesh orchestrating across multiple Kubernetes, K3s, and Edge clusters), network partitions, temporary control plane loss-of-contact, and regional infrastructure degradations will occur. 

When a cluster hosting an active agent workload becomes unreachable, the Global Control Plane must failover the run to a healthy alternate cluster, restoring state from the latest durable checkpoint. However, network partitions are inherently asymmetric: the original cluster may still be physically running the old workload instance. 

Without an explicit fencing mechanism, reconnecting the partitioned cluster can result in **split-brain execution** (two separate containers across two clusters concurrently claiming authority over the same run ID, emitting conflicting state mutations, checkpoints, and verification requests).

## Decision
We establish the following core distributed systems invariants:

1. **Monotonic Execution Authority**:
   A SentinelMesh `Run` may have multiple execution attempts across different clusters over its lifecycle, but **only the attempt carrying the latest `execution_generation` and authoritative `fencing_token` is valid and authoritative**.

2. **Fencing Token Generation**:
   Every scheduling or rescheduling placement atomically increments `execution_generation` and generates an unforgeable, cryptographically unique `fencing_token` (UUID) within the control plane transaction.

3. **Targeted Subject Routing**:
   NATS JetStream routing for workload dispatch is strictly cluster-targeted:
   `sentinel.run.v1.scheduled.<cluster_id>`
   Each per-cluster operator subscribes exclusively to its designated cluster subject.

4. **Workload Ingestion & Environment Injection**:
   The target cluster's Kubernetes Operator maps the scheduled payload into the custom resource `AgentRun` (carrying `spec.executionGeneration` and `spec.fencingToken`), and injects:
   - `SENTINEL_RUN_ID`
   - `SENTINEL_EXECUTION_GENERATION`
   - `SENTINEL_FENCING_TOKEN`
   into the agent container environment.

5. **Stale Execution Detection and Quarantine**:
   When a partitioned cluster reconnects or when its local Operator observes cluster state:
   - If an `AgentRun` Pod or CR exists with `execution_generation < current_authoritative_generation`, the Operator immediately marks the CR as `Fenced` / `Quarantined`.
   - The local Pod is terminated and prevented from making downstream state mutations or checkpoint saves.
   - The Operator emits a `RunExecutionFenced` event to the control plane for audit logging.

## Consequences
- **Positive**: Complete split-brain protection across heterogeneous clusters and network partitions.
- **Positive**: Clear observability and audit trails for failover and quarantined workloads.
- **Positive**: Clean separation between cluster selection (Tier 1) and node placement (Tier 2).
- **Negative**: Operators must verify authority before allowing resurrected pods to mutate persistent state.

## Validation
The monotonic execution authority and split-brain fencing invariants specified in this ADR are experimentally verified under adversarial chaos and network partitions as documented in [ADR-008: Deterministic Chaos Testing](ADR-008-chaos-validation.md) and tested in `test/chaos/fencing_chaos_test.go` (Scenario F12).

