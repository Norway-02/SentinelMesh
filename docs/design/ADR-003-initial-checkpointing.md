# ADR 003: Application-Level Durable Checkpointing

## Status
Accepted

## Context
Long-running AI agents can fail at any point (node failure, OOM, timeouts). Losing progress on a multi-hour task is unacceptable. While complete OS-level process-memory checkpoint/restore (like CRIU) is technically possible, it is brittle and overly complex for the MVP stage.

## Decision
We will implement **application-level durable checkpointing**.

The Agent Runtime will provide a structured way for agents to periodically save their internal state (e.g., workflow step, files processed, internal memory variables). 

The `Checkpoint Manager` will store:
1. **Metadata** in PostgreSQL (version, run ID, timestamp, status).
2. **State Artifacts/Blobs** in MinIO/S3 compatible storage.

When an agent fails and is rescheduled, it will pull the latest verified checkpoint from the Checkpoint Manager and initialize its application state from that point, rather than starting from zero.

## Consequences
**Positive:**
- Technically achievable and highly reliable.
- Architecture remains OS-agnostic and container-runtime agnostic.
- Simplifies debugging, as checkpoints are structured data rather than opaque memory dumps.

**Negative:**
- Requires agent logic to explicitly support serialization and deserialization of state.
- Will not automatically "resume" mid-execution instruction; it resumes from the last logical saved step.
