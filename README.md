# LFX V2 Mailing List Service

The LFX v2 Mailing List Service is a lightweight proxy microservice that delegates all GroupsIO operations to the ITX HTTP API. Built with Go and the Goa framework, it authenticates via Auth0 M2M OAuth2, translates LFX v2 UUIDs to v1 SFIDs via NATS request/reply, and forwards requests to the ITX backend.

## 🚀 Quick Start

### For Deployment (Helm)

Both flows below require the Kubernetes secret to be created first. If the `lfx` namespace doesn't exist yet, create it:

```bash
kubectl create namespace lfx
```

Then create the secret (values are in 1Password → **LFX V2** vault → **LFX Platform Chart Values Secrets - Local Development**):

```bash
kubectl create secret generic lfx-v2-mailing-list-service -n lfx \
  --from-literal=ITX_CLIENT_ID="<value-from-1password>" \
  --from-literal=ITX_CLIENT_PRIVATE_KEY="<value-from-1password>" \
  --from-literal=ITX_AUTH0_DOMAIN="<value-from-1password>" \
  --from-literal=ITX_AUDIENCE="<value-from-1password>" \
  --from-literal=ITX_BASE_URL="<value-from-1password>"
```

#### Deploy from GHCR (no local code changes)

Pulls the published image from GHCR — no local build required:

```bash
make helm-install
```

#### Deploy a local build (with code changes)

Build the image locally, then install using the local values override (which sets `pullPolicy: Never` and the local image repository). Copy the example file first — `values.local.yaml` is not tracked by git so it is safe to modify:

```bash
cp charts/lfx-v2-mailing-list-service/values.local.example.yaml \
   charts/lfx-v2-mailing-list-service/values.local.yaml

make docker-build
make helm-install-local
```

### For Local Development

1. **Prerequisites**
   - Go 1.24+ installed
   - Make installed
   - Docker (optional, for containerized development)
   - NATS server running (required for ID translation)

2. **Clone and Setup**

   ```bash
   git clone https://github.com/linuxfoundation/lfx-v2-mailing-list-service.git
   cd lfx-v2-mailing-list-service

   # Install dependencies and generate API code
   make deps
   make apigen
   ```

3. **Configure Environment**

   ```bash
   # For local development with mock translator (no NATS required)
   export TRANSLATOR_SOURCE=mock
   export TRANSLATOR_MAPPINGS_FILE=translator_mappings.yaml
   export AUTH_SOURCE=mock
   export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="test-admin"
   export LOG_LEVEL=debug

   # ITX proxy credentials (required even locally unless you stub the proxy)
   export ITX_BASE_URL="https://itx-api.example.com"
   export ITX_CLIENT_ID="your-client-id"
   export ITX_CLIENT_PRIVATE_KEY="$(cat private.key)"
   export ITX_AUTH0_DOMAIN="your-tenant.auth0.com"
   export ITX_AUDIENCE="https://itx-api.example.com"
   ```

4. **Run the Service**

   ```bash
   make run
   ```

## 🏗️ Architecture

The service is a thin proxy layer built using clean architecture:

- **API Layer**: Goa-generated HTTP handlers and OpenAPI specifications
- **Service Layer**: Orchestrators that resolve v2 UUIDs to v1 SFIDs and forward calls to the ITX proxy
- **Domain Layer**: Core business models, typed domain errors, and port interfaces
- **Infrastructure Layer**: ITX HTTP proxy client (Auth0 M2M), NATS ID translator, and JWT authentication

### Key Features

- **ITX Proxy**: All GroupsIO operations (services, mailing lists, members) are delegated to the ITX HTTP API
- **Auth0 M2M Authentication**: ITX requests authenticated via private-key JWT assertion with token caching via `oauth2.ReuseTokenSource`
- **ID Translation**: Transparent v2 UUID ↔ v1 SFID mapping via NATS request/reply to the v1-sync-helper service
- **GroupsIO Service Management**: List, get, create, update, delete, and find-parent operations for GroupsIO services
- **Mailing List Management**: Full lifecycle management including list count and member count endpoints
- **Member Management**: Add, get, update, delete, invite, and subscriber-check operations
- **JWT Authentication**: Secure API access via Heimdall integration
- **Mock Mode**: Complete testing capability without real ITX or NATS dependencies
- **OpenAPI Documentation**: Auto-generated API specifications
- **Comprehensive Testing**: Unit test coverage with mocks
- **Health Checks**: Built-in `/livez` and `/readyz` endpoints for Kubernetes probes
- **Structured Logging**: JSON-formatted logs with contextual information using Go's slog package
- **v1→v2 Data Stream**: Consumes DynamoDB change events and publishes them to the indexer and FGA-sync services

## 📁 Project Structure

```bash
lfx-v2-mailing-list-service/
├── cmd/                            # Application entry points
│   └── mailing-list-api/           # Main API server
│       ├── design/                 # Goa API design files
│       │   ├── mailing_list.go     # Service and endpoint definitions
│       │   └── type.go             # Type definitions and data structures
│       ├── eventing/               # v1→v2 data stream event processing
│       │   ├── event_processor.go  # JetStream consumer lifecycle
│       │   └── handler.go          # Key-prefix router (delegates to internal/service)
│       ├── service/                # GOA service implementations and providers
│       │   ├── mailing_list_api.go # GOA service implementation
│       │   ├── providers.go        # Dependency initialization (auth, translator, ITX config)
│       │   └── converters.go       # Domain ↔ GOA type converters
│       ├── data_stream.go          # Data stream startup wiring and env config
│       ├── main.go                 # Application entry point
│       └── http.go                 # HTTP server setup
├── charts/                         # Helm chart for Kubernetes deployment
│   └── lfx-v2-mailing-list-service/
│       ├── templates/              # Kubernetes resource templates
│       ├── values.yaml             # Production configuration
│       └── values.local.yaml       # Local development configuration
├── docs/                           # Additional documentation
│   └── event-processing.md         # v1→v2 data stream event processing
├── gen/                            # Generated code (DO NOT EDIT)
│   ├── http/                       # HTTP transport layer
│   │   ├── openapi.yaml            # OpenAPI 2.0 specification
│   │   └── openapi3.yaml           # OpenAPI 3.0 specification
│   └── mailing_list/               # Service interfaces
├── internal/                       # Private application code
│   ├── domain/                     # Business domain layer
│   │   ├── errors.go               # Typed domain errors (DomainError with constructors)
│   │   ├── model/                  # Domain models (GroupsIOService, GroupsIOMailingList, GrpsIOMember)
│   │   └── port/                   # Repository and service interfaces
│   │       ├── translator.go       # Translator interface (MapID v2↔v1)
│   │       └── mapping_store.go    # MappingReader / MappingWriter / MappingReaderWriter
│   ├── service/                    # Service layer implementation
│   │   ├── grpsio_service_reader.go         # Service reader orchestrator
│   │   ├── grpsio_service_writer.go         # Service writer orchestrator
│   │   ├── grpsio_mailing_list_reader.go    # Mailing list reader orchestrator
│   │   ├── grpsio_mailing_list_writer.go    # Mailing list writer orchestrator
│   │   ├── grpsio_member_reader.go          # Member reader orchestrator
│   │   ├── grpsio_member_writer.go          # Member writer orchestrator
│   │   ├── datastream_service_handler.go    # v1-sync service transform + publish
│   │   ├── datastream_subgroup_handler.go   # v1-sync mailing list transform + publish
│   │   └── datastream_member_handler.go     # v1-sync member transform + publish
│   ├── infrastructure/             # Infrastructure layer
│   │   ├── auth/                   # JWT authentication
│   │   ├── proxy/                  # ITX HTTP proxy client
│   │   │   ├── itx.go              # ITX client (implements all GroupsIO port interfaces)
│   │   │   ├── types.go            # Wire types for ITX API requests/responses
│   │   │   └── converters.go       # Domain ↔ wire type converters
│   │   ├── nats/                   # NATS messaging and ID translation
│   │   │   ├── translator.go       # NATS request/reply ID translator
│   │   │   ├── mapping_store.go    # MappingReaderWriter backed by JetStream KV
│   │   │   ├── messaging_publish.go # Message publishing
│   │   │   └── client.go           # NATS connection management
│   │   └── mock/                   # Mock implementations for testing
│   │       ├── auth.go             # Mock authentication
│   │       └── translator.go       # Mock ID translator (file-backed YAML mappings)
│   └── middleware/                 # HTTP middleware components
│       ├── authorization.go        # JWT-based authorization
│       └── request_id.go           # Request ID injection
├── pkg/                            # Public packages
│   ├── constants/                  # Application constants
│   │   ├── context.go              # Context keys
│   │   ├── global.go               # Global constants
│   │   ├── storage.go              # Storage bucket names
│   │   └── subjects.go             # NATS subject definitions
│   ├── errors/                     # Error types
│   └── auth/                       # Auth0 token source helpers
├── Dockerfile                      # Container build configuration
├── Makefile                        # Build and development commands
├── CLAUDE.md                       # Claude Code assistant instructions
└── go.mod                          # Go module definition
```

## Committee Member Sync

This service does not implement committee-to-mailing-list member synchronization. That sync is fully handled by the system this service proxies to (the ITX/v1 backend).

The sync logic works as follows:

- **When a committee member is added**, the member is subscribed to all linked mailing lists they are eligible for based on the list's voting status filters.
- **When a committee member is removed**, the member is unsubscribed from all private mailing lists linked to that committee. Public lists are not affected.
- **When a committee is deleted**, its association with linked mailing lists is cleared. Existing members are left as-is — no one is removed.
- **When a committee association or its filters are updated** on a mailing list, the membership is reconciled: members who now match are added, and members who no longer match are removed (private lists only).

Because this service reuses the same database and infrastructure as the proxied backend, this sync loop is already closed and no additional implementation is needed here.

---

## 📚 Additional Documentation

| Document | Description |
|---|---|
| [docs/event-processing.md](docs/event-processing.md) | v1→v2 data stream: how DynamoDB change events are consumed, transformed, and published to the indexer and FGA-sync services |

## 🛠️ Development

### Prerequisites

- Go 1.24+
- Make
- Git

### Getting Started

1. **Install Dependencies**

   ```bash
   make deps
   ```

   This installs:
   - Go module dependencies
   - Goa CLI for code generation

2. **Generate API Code**

   ```bash
   make apigen
   ```

   Generates HTTP transport, client, and OpenAPI documentation from design files.

3. **Build the Application**

   ```bash
   make build
   ```

   Creates the binary in `bin/lfx-v2-mailing-list-service`.

### Development Workflow

#### Running the Service

```bash
# Run with auto-regeneration
make run

# Build and run binary
make build
./bin/lfx-v2-mailing-list-service
```

#### Code Quality

**Always run these before committing:**

```bash
# Run linter
make lint

# Run all tests
make test

# Run complete pipeline (setup + lint + test + build)
make all
```

#### Testing

```bash
# Run all tests with race detection and coverage
make test

# View coverage report
go tool cover -html=coverage.out
```

**Writing Tests:**

- Place test files alongside source files with `_test.go` suffix
- Use table-driven tests for multiple test cases
- Mock external dependencies using the provided mock interfaces in `internal/infrastructure/mock/`
- Achieve high test coverage (aim for >80%)
- Test both happy path and error cases

Example test structure:

```go
func TestServiceMethod(t *testing.T) {
    tests := []struct {
        name        string
        input       InputType
        setupMocks  func(*MockRepository)
        expected    ExpectedType
        expectError bool
    }{
        // Test cases here
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

#### API Development

When modifying the API:

1. **Update Design Files** in `cmd/mailing-list-api/design/` directory
2. **Regenerate Code**:

   ```bash
   make apigen
   ```

3. **Run Tests** to ensure nothing breaks:

   ```bash
   make test
   ```

4. **Update Service Implementation** in `cmd/mailing-list-api/service/`

#### ITX Proxy Architecture

All GroupsIO operations are delegated to the ITX HTTP API. The proxy layer handles Auth0 M2M token acquisition and transparent v2 UUID → v1 SFID translation.

**Authentication:**

```go
// ITX proxy uses Auth0 private-key JWT assertion with token caching
tokenSource := pkgauth.NewAuth0TokenSource(ctx, authConfig, config.Audience, itxScope)
oauthHTTPClient := oauth2.NewClient(ctx, oauth2.ReuseTokenSource(nil, tokenSource))
```

**ID Translation:**

```go
// Orchestrators translate v2 UUIDs to v1 SFIDs before forwarding to ITX
sfid, err := translator.MapID(ctx, constants.TranslationSubjectProject,
    constants.TranslationDirectionV2ToV1, projectUID)
```

**Configuration Modes:**

- **Production**: `TRANSLATOR_SOURCE=nats` — translates via NATS request/reply to the v1-sync-helper
- **Testing**: `TRANSLATOR_SOURCE=mock` — loads mappings from a local YAML file (`TRANSLATOR_MAPPINGS_FILE`)

### Available Make Targets

| Target | Description |
|--------|-------------|
| `make all` | Complete build pipeline (setup, lint, test, build) |
| `make deps` | Install dependencies and Goa CLI |
| `make setup` | Setup development environment |
| `make setup-dev` | Install development tools (golangci-lint) |
| `make apigen` | Generate API code from design files |
| `make build` | Build the binary |
| `make run` | Run the service locally |
| `make test` | Run unit tests with race detection |
| `make lint` | Run code linter |
| `make clean` | Remove build artifacts |
| `make docker-build` | Build Docker image |
| `make docker-run` | Run Docker container locally |
| `make helm-install` | Install Helm chart |
| `make helm-install-local` | Install with mock authentication |
| `make helm-templates` | Print Helm templates |
| `make helm-uninstall` | Uninstall Helm chart |

## 🧪 Testing

### Running Tests

```bash
# Run all tests
make test

# Run specific package tests
go test -v ./internal/service/...

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Structure

The project follows Go testing best practices:

- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test component interactions
- **Mock Interfaces**: Located in `internal/infrastructure/mock/`
- **Test Coverage**: Aim for high coverage with meaningful tests

### Writing Tests

When adding new functionality:

1. **Write tests first** (TDD approach recommended)
2. **Use table-driven tests** for multiple scenarios
3. **Mock external dependencies** using provided interfaces
4. **Test error conditions** not just happy paths
5. **Keep tests focused** and independent

### Local Testing with Mock Authentication

For comprehensive integration testing using local Kubernetes cluster:

1. **Deploy with Mock Authentication**:

   ```bash
   make helm-install-local
   ```

   This deploys the service with:
   - `AUTH_SOURCE=mock` - Bypasses JWT validation
   - `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL=test-super-admin` - Mock principal
   - `TRANSLATOR_SOURCE=mock` - File-backed ID mappings

2. **Test Individual Endpoints**:

   ```bash
   # Any Bearer token works with mock auth
   curl -H "Authorization: Bearer test-token" \
        http://lfx-v2-mailing-list-service.lfx.svc.cluster.local:8080/groupsio/services
   ```

**⚠️ Security Warning**: Never use mock authentication in production environments.

## 🚀 Deployment

### Kubernetes Secret

Before deploying, create the Kubernetes secret with ITX credentials. The command below is idempotent and safe to re-run (e.g. for credential rotation):

```bash
kubectl create secret generic lfx-v2-mailing-list-service -n lfx \
  --from-literal=ITX_CLIENT_ID="<value-from-1password>" \
  --from-literal=ITX_CLIENT_PRIVATE_KEY="<value-from-1password>" \
  --from-literal=ITX_AUTH0_DOMAIN="<value-from-1password>" \
  --from-literal=ITX_AUDIENCE="<value-from-1password>" \
  --from-literal=ITX_BASE_URL="<value-from-1password>" \
  --dry-run=client -o yaml | kubectl apply -f -
```

> **Where to find the secret values**: Look in 1Password under the **LFX V2** vault, in the secured note titled **LFX Platform Chart Values Secrets - Local Development**.

### Helm Chart

The service includes a Helm chart for Kubernetes deployment:

```bash
# Install using make (recommended)
make helm-install

# Install with local values override using make
make helm-install-local

# Install directly with helm
helm upgrade --install lfx-v2-mailing-list-service ./charts/lfx-v2-mailing-list-service \
  --namespace lfx \
  --create-namespace

# Install with local values override directly
helm upgrade --install lfx-v2-mailing-list-service ./charts/lfx-v2-mailing-list-service \
  --namespace lfx \
  --create-namespace \
  --values ./charts/lfx-v2-mailing-list-service/values.local.yaml

# View rendered templates
make helm-templates
```

### Docker

```bash
# Build Docker image
make docker-build

# Run with Docker
docker run -p 8080:8080 linuxfoundation/lfx-v2-mailing-list-service:latest
```

## 📡 NATS Messaging

NATS serves two roles in this service: ID translation and event publishing.

### ID Translation

The service uses NATS request/reply to translate v2 UUIDs to v1 SFIDs (and vice versa) via the v1-sync-helper service:

| Subject | Purpose |
|---------|---------|
| `lfx.lookup_v1_mapping` | Translate project/committee UIDs ↔ SFIDs |

Key format sent to the v1-sync-helper:
- `project.uid.<uuid>` — v2 UUID → v1 SFID
- `project.sfid.<sfid>` — v1 SFID → v2 UUID
- `committee.uid.<uuid>` — v2 UUID → v1 SFID (response: `projectSFID:committeeSFID`)

### Published Subjects

The service publishes messages to the following NATS subjects (primarily via the v1→v2 data stream processor):

| Subject | Purpose | Message Schema |
|---------|---------|----------------|
| `lfx.index.groupsio_service` | GroupsIO service indexing events | Indexer message with tags |
| `lfx.index.groupsio_mailing_list` | Mailing list indexing events | Indexer message with tags |
| `lfx.index.groupsio_member` | Member indexing events | Indexer message with tags |
| `lfx.update_access.groupsio_service` | Service access control updates | Access control message |
| `lfx.delete_all_access.groupsio_service` | Service access control deletion | Access control message |
| `lfx.update_access.groupsio_mailing_list` | Mailing list access control updates | Access control message |
| `lfx.delete_all_access.groupsio_mailing_list` | Mailing list access control deletion | Access control message |

### Message Publisher Interface

The service uses two message types:

- **Indexer Messages**: For search indexing operations (consumed by indexer services)
- **Access Messages**: For permission management (consumed by fga-sync service)

## 📖 API Documentation

The service automatically generates OpenAPI documentation:

- **OpenAPI 2.0**: `gen/http/openapi.yaml`
- **OpenAPI 3.0**: `gen/http/openapi3.yaml`
- **JSON formats**: Also available in `gen/http/`

Access the documentation at: `http://localhost:8080/openapi.json`

### Available Endpoints

The full list of available endpoints is documented via Swagger. Access the live spec at:

- `http://localhost:8080/openapi.json` (JSON)
- `http://localhost:8080/openapi3.yaml` (YAML)

## 🔧 Configuration

The service can be configured via environment variables:

### Core Service Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NATS_URL` | NATS server URL | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` |
| `NATS_TIMEOUT` | NATS connection timeout | `10s` |
| `NATS_MAX_RECONNECT` | Maximum NATS reconnect attempts | `3` |
| `NATS_RECONNECT_WAIT` | Wait between NATS reconnect attempts | `2s` |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `LOG_ADD_SOURCE` | Add source location to logs | `true` |
| `PORT` | HTTP server port | `8080` |

### Authentication Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `JWKS_URL` | JWKS URL for JWT verification | `http://lfx-platform-heimdall.lfx.svc.cluster.local:4457/.well-known/jwks` |
| `JWT_AUDIENCE` | JWT token audience | `lfx-v2-mailing-list-service` |
| `AUTH_SOURCE` | Authentication source (`jwt` or `mock`) | `jwt` |
| `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL` | Mock principal for local dev (dev only) | `""` |

### ITX Proxy Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ITX_BASE_URL` | ITX HTTP API base URL | Required |
| `ITX_CLIENT_ID` | Auth0 client ID for M2M authentication | Required |
| `ITX_CLIENT_PRIVATE_KEY` | RSA private key (PEM) for Auth0 JWT assertion | Required |
| `ITX_AUTH0_DOMAIN` | Auth0 tenant domain | Required |
| `ITX_AUDIENCE` | Auth0 audience for the ITX API | Required |

### ID Translator Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `TRANSLATOR_SOURCE` | Translator backend (`nats` or `mock`) | `nats` |
| `TRANSLATOR_MAPPINGS_FILE` | YAML file for mock translator mappings | `translator_mappings.yaml` |

### Development Environment Variables

For local development with mock backends (no real ITX or NATS required):

```bash
export AUTH_SOURCE="mock"
export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="test-admin"
export TRANSLATOR_SOURCE="mock"
export TRANSLATOR_MAPPINGS_FILE="translator_mappings.yaml"
export LOG_LEVEL="debug"
```

For local development with real NATS but mock auth:

```bash
export NATS_URL="nats://localhost:4222"
export AUTH_SOURCE="mock"
export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="test-admin"
export TRANSLATOR_SOURCE="nats"
export ITX_BASE_URL="https://itx-api.example.com"
export ITX_CLIENT_ID="your-client-id"
export ITX_CLIENT_PRIVATE_KEY="$(cat private.key)"
export ITX_AUTH0_DOMAIN="your-tenant.auth0.com"
export ITX_AUDIENCE="https://itx-api.example.com"
export LOG_LEVEL="debug"
```

## 📄 License

Copyright The Linux Foundation and each contributor to LFX.

SPDX-License-Identifier: MIT
