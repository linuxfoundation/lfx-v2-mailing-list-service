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
- `make helm-install-local` - Deploy to Kubernetes with mock authentication for local testing

## Architecture

### GOA Framework
This service uses GOA v3 for API design and code generation:
- API design definitions are in `cmd/mailing-list-api/design/`
- Generated code is in `gen/` directory (never edit manually)
- Run `make apigen` after modifying design files
- Service implementations are in `cmd/mailing-list-api/service/`

### API Endpoint Documentation

**Whenever you add, remove, or change an endpoint in `cmd/mailing-list-api/design/mailing_list.go`, you MUST update `docs/api-endpoints.md`:**

- Add/remove the row in the relevant summary table
- Add/remove/update the corresponding curl example in the Examples section

This applies to method changes, path changes, new endpoints, and deleted endpoints.

### Indexer Contract Documentation

**Whenever you add, remove, or change what this service sends to the indexer (tags, schema fields, IndexingConfig, access messages), you MUST update `docs/indexer-contract.md`:**

- Update the relevant resource type section (schema, tags, access control, search behavior, parent references)
- Add a new resource type section if a new indexed type is introduced

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
- `internal/service/` - Domain service implementations
  - `grpsio_service_reader.go`, `grpsio_service_writer.go` - GroupsIO service orchestrators
  - `grpsio_mailing_list_reader.go`, `grpsio_mailing_list_writer.go` - Mailing list orchestrators
  - `grpsio_member_reader.go`, `grpsio_member_writer.go` - Member orchestrators
  - `grpsio_webhook_processor.go` - Webhook event processor
  - `datastream_service_handler.go` - v1 DynamoDB KV event processor for GroupsIO services
  - `datastream_member_handler.go` - v1 DynamoDB KV event processor for GroupsIO members
  - `datastream_subgroup_handler.go` - v1 DynamoDB KV event processor for GroupsIO subgroups

**Middleware Layer** (`internal/middleware/`):
- `authorization.go` - JWT-based authorization middleware
- `request_id.go` - Request ID injection middleware

### Authentication & Authorization
- Uses JWT tokens from Heimdall service
- Principal extraction from custom claims: `HeimdallClaims{Principal, Email}`
- JWT validation with PS256 algorithm and JWKS endpoint
- Context-based principal propagation using `constants.PrincipalContextID`

### NATS Integration
- **JetStream Storage**: Key-value storage for services, mailing lists, and members
- **Message Publishing**: Publishes indexing and access control events
- **Data Stream**: Subscribes to v1 DynamoDB KV events for data migration/sync
- **Connection Management**: Reconnection handling and readiness checks
- **Queue Groups**: Uses `lfx-v2-mailing-list-api` queue for load balancing

#### NATS Subjects (Publishing)
- `lfx.index.groupsio_service` - Service indexing
- `lfx.index.groupsio_mailing_list` - Mailing list indexing
- `lfx.index.groupsio_member` - Member indexing
- `lfx.update_access.groupsio_service` - Service access control
- `lfx.delete_all_access.groupsio_service` - Service access deletion
- `lfx.update_access.groupsio_mailing_list` - Mailing list access control
- `lfx.delete_all_access.groupsio_mailing_list` - Mailing list access deletion

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
- Run individual test: `go test -v ./path/to/package -run TestName`
- Run with coverage: `go test -v -race -coverprofile=coverage.out ./...`
- Always run `make test` before committing

### Configuration
Environment-based configuration for:
- `NATS_URL` - NATS server connection (default: `nats://lfx-platform-nats.lfx.svc.cluster.local:4222`)
- `JWT_AUDIENCE` and `JWKS_URL` - Authentication configuration
- `GROUPSIO_SOURCE` - Set to `mock` to bypass Groups.io API calls
- `AUTH_SOURCE` - Set to `mock` to bypass JWT authentication
- `REPOSITORY_SOURCE` - Set to `mock` to use in-memory storage
- Service runs on port 8080 by default

### Key Files to Understand
- [`cmd/mailing-list-api/main.go`](cmd/mailing-list-api/main.go) - Application entry point, service initialization
- [`internal/infrastructure/proxy/`](internal/infrastructure/proxy/) - ITX proxy client and converters
- [`internal/service/datastream_member_handler.go`](internal/service/datastream_member_handler.go) - v1 KV event processing for members
- [`pkg/constants/subjects.go`](pkg/constants/subjects.go) - NATS subject definitions

### Local Development & Testing

#### Environment Variables
For local testing with mocks:
- `export NATS_URL=nats://localhost:4222`
- `export AUTH_SOURCE=mock`
- `export REPOSITORY_SOURCE=mock`
- `export GROUPSIO_SOURCE=mock`
- `export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="test-user"`

#### Local Kubernetes Deployment with Mock Authentication
For comprehensive integration testing using local Kubernetes cluster:

1. **Deploy with Mock Authentication**:
   ```bash
   make helm-install-local
   ```
   This deploys the service with:
   - `AUTH_SOURCE=mock` - Bypasses JWT validation
   - `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL=test-super-admin` - Mock principal
   - `openfga.enabled=false` - Disables authorization
   - `heimdall.enabled=false` - Bypasses middleware

2. **Run Integration Tests**:
   ```bash
   ./scripts/integration_test_mailing_list.sh
   ```

3. **Test Individual Endpoints**:
   ```bash
   # Any Bearer token works with mock auth
   curl -H "Authorization: Bearer <your-token>" \
        http://lfx-v2-mailing-list-service.lfx.svc.cluster.local:8080/services
   ```

#### Helm Values Files
- `charts/lfx-v2-mailing-list-service/values.yaml` - Default configuration; pulls image from GHCR (`ghcr.io/...`), `pullPolicy: IfNotPresent`. Used by `make helm-install`. Do not commit local overrides here.
- `charts/lfx-v2-mailing-list-service/values.local.yaml` - Not tracked by git. User-created override file for local development. Used by `make helm-install-local`.
- `charts/lfx-v2-mailing-list-service/values.local.example.yaml` - Tracked example to copy from. Sets `image.repository: linuxfoundation/lfx-v2-mailing-list-service`, `image.tag: latest`, `image.pullPolicy: Never` for use with locally-built images.

**Two deployment modes:**
1. **Run from GHCR (no local code changes)**: `make helm-install` — pulls the published image, no docker build needed.
2. **Run local code changes**: Copy the example file (`cp charts/lfx-v2-mailing-list-service/values.local.example.yaml charts/lfx-v2-mailing-list-service/values.local.yaml`), then `make docker-build && make helm-install-local` — uses the locally-built image.

**⚠️ Security Warning**: Never use mock authentication in production environments.

### GroupsIO Integration & Mocking

#### GroupsIO Client Architecture
The service integrates with Groups.io API through a clean orchestrator pattern:

```go
// Orchestrator with nil-safe design
type grpsIOWriterOrchestrator struct {
    groupsClient *groupsio.Client // May be nil for mock/disabled mode
}

// Usage pattern throughout service
if o.groupsClient != nil {
    result, err := o.groupsClient.CreateGroup(ctx, domain, options)
    // Handle Groups.io operations
} else {
    // Mock mode: operations bypassed, domain logic continues
}
```

#### Mock Configuration
- **Production**: `GROUPSIO_SOURCE=groupsio` - Uses actual Groups.io API client
- **Testing**: `GROUPSIO_SOURCE=mock` - Returns nil client, enables pure domain testing
- **Domain Logic**: All business logic flows through `MockRepository` in `internal/infrastructure/mock/grpsio.go`
- **Error Simulation**: Comprehensive error testing available through domain mock

#### Benefits of This Pattern
1. **Clean Separation**: Infrastructure (HTTP calls) vs Domain (business logic)
2. **Nil-Safe**: Orchestrator gracefully handles disabled Groups.io integration
3. **Testable**: Domain logic fully tested without external API dependencies
4. **Configurable**: Easy switching between mock and real modes

### Committee–Mailing List Sync (handled by the proxied system)

This service does **not** implement committee-to-mailing-list member synchronization. The sync is fully handled within the ITX/v1 backend that this service proxies to. No NATS-based flow or additional service needs to be built here.

**How the sync works in the proxied system:**
- `groups_subgroups.go` (API) — publishes `CommitteeAssociated`, `CommitteeChanged`, and `CommitteeUnassociated` SNS events on subgroup committee config changes.
- `cmd/committee-association` (Lambda) — consumes those events via SQS:
  - `CommitteeAssociated`: full member sync into the mailing list.
  - `CommitteeUnassociated`: removes all committee-type members from the list.
  - `CommitteeChanged`: diffs old vs. new `AllowedVotingStatuses` filters — removes members who no longer match, adds members who now do.
  - `CommitteeMemberV2Created` / `CommitteeMemberV2Deleted`: individual member add/remove.

**Intentional gap in the proxied system:** when a committee is associated with a subgroup that previously had none (via `PUT`), the `CommitteeAssociated` event is deliberately not fired (commented out in `groups_subgroups.go`). Pre-existing product decision, out of scope.

**If this sync ever needs to be implemented here** (e.g., if the service is decoupled from the ITX backend and gets its own database), the logic to implement is:
- `handleCreated`: on mailing list creation with committees configured, call a `SyncCommitteeMembersToMailingList` for each committee. Skip committees with errors (log and continue).
- `handleUpdated`: compare old and new committee configurations using committee UID as the stable key. Classify changes into added/removed/modified (modified = same UID but different `AllowedVotingStatuses`). Act on each category.
- `detectCommitteeChanges`: diff helper that returns three slices — added, removed, modified committees.

### Committee Member Sync (handled by the proxied system)

This service does **not** implement committee-to-mailing-list member synchronization. That sync is fully handled by the `committee-association` Lambda (`cmd/committee-association`) in `itx-service-groupsio`.

The flow is event-driven: `project-management` publishes SNS events (`CommitteeMemberV2Created`, `CommitteeMemberV2Deleted`, `CommitteeV2Deleted`), and `itx-service-groupsio` publishes `CommitteeChanged` when a subgroup's committee association or filters are updated. Those events flow through SQS into the Lambda, which calls this service's ITX proxy endpoints to subscribe or remove members in Groups.io.

No additional implementation is needed here — this service only needs to correctly proxy the member subscribe/remove calls the Lambda makes.

### Testing the Data Stream Processor
To test the v1 DynamoDB KV event processor locally:

1. **Start the service with mock mode**:
   ```bash
   export NATS_URL=nats://localhost:4222
   export AUTH_SOURCE=mock
   export REPOSITORY_SOURCE=mock
   export GROUPSIO_SOURCE=mock
   export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="test-user"
   make run
   ```

2. **Run unit tests**:
   ```bash
   go test -v ./internal/service/...
   ```