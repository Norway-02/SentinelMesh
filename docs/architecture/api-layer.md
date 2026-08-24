# API Layer Architecture

The SentinelMesh API layer is designed using clean architecture principles to separate external concerns from business logic.

## Structure

- **`api/proto`**: Contains the source of truth for the API contract (`sentinelmesh.proto`). All structural definitions for gRPC and data interchange are defined here.
- **`internal/transport/grpc`**: Contains the gRPC server implementation and mappers from protobuf structures to domain models. Maps domain errors to gRPC `status` codes.
- **`internal/transport/rest`**: Contains standard `net/http` handlers providing JSON REST APIs. Maps domain errors to HTTP status codes. Provides `Logger` and `Recoverer` middlewares.
- **`internal/application`**: The coordination layer. Application services (`AgentService`, `RunService`) orchestrate the retrieval, validation, and persistence of domain aggregates using repositories. They do not know about HTTP or gRPC.
- **`internal/repository`**: Defines interfaces for data access (e.g. `AgentRepository`, `RunRepository`). The implementations (`internal/repository/memory`) fulfill these contracts.

## Dependency Flow

Requests enter via `transport/rest` or `transport/grpc`. The transport layer extracts the data, creates domain structs or inputs, and calls `application` services. The application service loads domain aggregates from `repository`, calls domain methods to mutate state (which applies pure business rules), and saves the changes back to the repository. The result is returned and mapped by the transport layer into JSON or Protobuf.

## Future Extensibility
This structure prepares the system for:
1. **PostgreSQL**: A new implementation in `internal/repository/postgres` can implement the repository interfaces.
2. **Kubernetes Integration**: The application services can trigger side-effects like pod creation without the API endpoints knowing how the scheduling occurs.
