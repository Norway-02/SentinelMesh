# SentinelMesh Architecture Overview

SentinelMesh is an AI-native distributed execution platform designed for secure, reliable, observable, and cost-aware execution of long-running AI agents. 

The core architectural principle of SentinelMesh is the strict separation between the **Control Plane** and the **Data Plane**.
> Control plane decides what should happen. Data plane safely executes it.

## High-Level Architecture

```text
                         SENTINELMESH
                              │
                ┌─────────────┴─────────────┐
                │                           │
          CONTROL PLANE                DATA PLANE
        "brain of platform"          "runs the agents"
                │                           │
       ┌────────┼─────────┐        ┌────────┼─────────┐
       │        │         │        │        │         │
       ▼        ▼         ▼        ▼        ▼         ▼
   API Server Scheduler Policy   Runtime  Sandbox  Checkpoint
       │        │         │        │        │         │
       └────────┼─────────┘        └────────┼─────────┘
                │                           │
                ▼                           ▼
          State / Metadata            Agent Workloads
                │                           │
       ┌────────┼────────┐                  │
       ▼        ▼        ▼                  │
   PostgreSQL Redis  Event Bus              │
                                           │
                              ┌────────────┼────────────┐
                              ▼            ▼            ▼
                           Cluster A    Cluster B     Edge
                              │            │            │
                         Kubernetes   Kubernetes    Kubernetes
```

### Components Summary

**Control Plane Components:**
- **API Server:** Handles REST/gRPC traffic, authentication, authorization, validation.
- **Agent Registry:** Stores agent configurations and definitions.
- **Scheduler:** Matches agent requirements to node capabilities considering CPU, memory, GPU, security, and cost.
- **Policy Engine:** Evaluates requested actions against declared policies to make ALLOW/DENY decisions.
- **Verifier:** Evaluates the correctness of agent actions using deterministic signals.
- **Model Router:** Dynamically routes model requests based on task class, quality, SLA, and cost.

### Core Domain Layer
The system uses a highly decoupled domain layer that defines the foundational business rules:
- **Explicit State Machine:** The agent execution lifecycle (e.g., `SCHEDULED`, `RUNNING`, `FAILED`, `RECOVERING`) is strictly enforced by the domain layer. All state transitions are deterministic and evaluated without side effects.
- **Strict Validation:** Agents, Policies, and Checkpoints are subject to immediate structured validation (rejecting negative resources, empty names, invalid configurations).
- **Transport Agnostic:** The domain operates independently of HTTP, gRPC, PostgreSQL, and Kubernetes constraints, ensuring easily testable and pure business logic.

**Data Plane Components:**
- **Agent Runtime:** Controls the actual execution lifecycle (Queued -> Scheduled -> Running -> Checkpointing -> Completed/Failed).
- **Sandbox:** Provides security via namespaces, cgroups, seccomp, and eBPF.
- **Checkpoint Manager:** Handles durable application-level checkpointing for long-running recovery.

### Storage & Messaging
- **PostgreSQL:** Primary metadata store.
- **Redis:** Caching and hot state.
- **MinIO/S3:** Object storage for artifacts and checkpoints.
- **NATS JetStream:** Event bus for loosely coupled, asynchronous service communication.
