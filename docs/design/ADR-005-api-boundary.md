# ADR 005: API Boundary and Application Services

## Status
Accepted

## Context
As SentinelMesh introduces an API for agent creation and execution (Stage 3), we need to establish clear boundaries between the external world (REST/gRPC), the application logic (coordination), the core business rules (domain), and the persistence mechanisms (repositories). Without this separation, domain models leak into APIs, databases bleed into business rules, and HTTP concerns mix with transaction management.

## Decision
We enforce a strict dependency inversion architecture around the API layer:

```text
REST / gRPC (Transport)
     |
     v
Application Service (Coordination)
     |
     v
Domain (Business Logic)
     |
     v
Repository Interface (Data access abstraction)
     |
     +--> PostgreSQL / In-Memory (Implementations)
```

1. **Canonical Interface**: gRPC is the canonical internal service interface, leveraging Protocol Buffers for strongly-typed contracts.
2. **REST Convenience**: REST/JSON endpoints are provided using the Go 1.22 standard library mux, mapping to the same application services for external ease of use.
3. **Application Services**: The `AgentService` and `RunService` are responsible for coordinating use-cases. They fetch data via repositories, invoke domain aggregate methods, and persist the results.
4. **Transport Mappers**: We strictly map protobuf requests to domain inputs, and domain outputs back to protobuf or JSON responses. Domain entities are not directly serialized over the wire without a DTO mapping.
5. **Typed Errors**: We use a taxonomy of domain errors and a translation layer in the transport handlers to map these to standard HTTP/gRPC status codes.

## Consequences
**Positive:**
- Complete decoupling of the domain from HTTP/gRPC mechanics.
- Repositories can be cleanly swapped (e.g., In-Memory to PostgreSQL in Stage 4) without touching the API.
- Error handling is standardized and doesn't leak internal DB state to consumers.
- Highly testable application services using mocked or in-memory repositories.

**Negative:**
- Increased boilerplate: we need `transport`, `application`, `repository`, and `domain` layers, along with explicit DTO mapping functions, even for simple CRUD operations.
