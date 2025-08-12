# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Development Workflow
- `make setup` - Setup development environment and install dependencies
- `make apigen` - Generate GOA API code from design files (run after modifying design/*.go)
- `make build` - Build the application for local development
- `make run` - Build and run the service locally on port 8080
- `make test` - Run all tests with race detection and coverage
- `make lint` - Run golangci-lint for code quality checks
- `make all` - Complete pipeline: setup, lint, test, build

### Docker & Deployment
- `make docker-build` - Build Docker image
- `make docker-run` - Run service in Docker container locally
- `make helm-install` - Deploy to Kubernetes using Helm

## Architecture

### GOA Framework
This service uses GOA v3 for API design and code generation:
- API design definitions are in `cmd/mailing-list-api/design/`
- Generated code is in `gen/` directory (never edit manually)
- Run `make apigen` after modifying design files
- Service implementations are in `cmd/mailing-list-api/service/`

### Clean Architecture Structure
The codebase follows hexagonal/clean architecture principles:

**Domain Layer** (`internal/domain/`):
- `model/` - Domain entities (GrpsioMailingList, GrpsioService)
- `port/` - Interface definitions for external dependencies (auth, project readers, grpsio service)

**Infrastructure Layer** (`internal/infrastructure/`):
- `auth/` - JWT authentication using Heimdall tokens
- `nats/` - NATS messaging client, JetStream key-value storage, and storage abstractions
- `mock/` - Mock implementations for testing (auth, grpsio service)

**Application Layer**:
- `cmd/mailing-list-api/service/` - GOA service implementations
- `internal/service/` - Domain service implementations (grpsio service reader)

**Middleware Layer** (`internal/middleware/`):
- `authorization.go` - JWT-based authorization middleware
- `request_id.go` - Request ID injection middleware

### Authentication & Authorization
- Uses JWT tokens from Heimdall service
- Principal extraction from custom claims: `HeimdallClaims{Principal, Email}`
- JWT validation with PS256 algorithm and JWKS endpoint
- Context-based principal propagation using `constants.PrincipalContextID`

### NATS Integration
- JetStream for message streaming and key-value storage
- Connection management with reconnection handling
- Readiness checks for service health
- Key-value stores accessed by bucket name

### Error Handling
Custom error types in `pkg/errors/`:
- `NewServiceUnavailable()` - For infrastructure failures
- `NewUnexpected()` - For unexpected conditions
- Structured logging with slog package throughout

### Request Context
Request-scoped data flows through context.Context:
- Request IDs via middleware
- Principal from JWT auth
- Context keys defined in `pkg/constants/context.go`
- Storage constants defined in `pkg/constants/storage.go`

## Development Notes

### Adding New Endpoints
1. Define API contract in `cmd/mailing-list-api/design/mailing_list.go`
2. Run `make apigen` to generate boilerplate
3. Implement service methods in `cmd/mailing-list-api/service/`
4. Add domain models to `internal/domain/model/` if needed
5. Create infrastructure adapters in `internal/infrastructure/` for external dependencies
6. Add middleware in `internal/middleware/` for cross-cutting concerns

### Testing Strategy
- Unit tests alongside source files (`*_test.go`)
- Mock implementations in `internal/infrastructure/mock/`
- Integration tests use testify/suite patterns
- Always run `make test` before committing

### Configuration
Environment-based configuration for:
- NATS_URL for messaging service connectivity  
- JWT_AUDIENCE and JWKS_URL for authentication
- Service runs on port 8080 by default