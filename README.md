# LFX V2 Mailing List Service

The LFX v2 Mailing List Service is a comprehensive microservice that manages mailing lists and their members within the Linux Foundation's LFX platform. Built with Go and the Goa framework, it provides robust CRUD operations for GroupsIO services, mailing lists, and members with direct Groups.io API integration and NATS JetStream persistence.

## 🚀 Quick Start

### For Deployment (Helm)

If you just need to run the service without developing on the service, use the Helm chart:

```bash
# Install the mailing list service
helm upgrade --install lfx-v2-mailing-list-service ./charts/lfx-v2-mailing-list-service \
  --namespace lfx \
  --create-namespace \
  --set image.tag=latest
```

### For Local Development

1. **Prerequisites**
   - Go 1.24+ installed
   - Make installed
   - Docker (optional, for containerized development)
   - NATS server running (for local testing)

2. **Clone and Setup**

   ```bash
   git clone https://github.com/linuxfoundation/lfx-v2-mailing-list-service.git
   cd lfx-v2-mailing-list-service

   # Install dependencies and generate API code
   make deps
   make apigen
   ```

3. **Configure Environment (Optional)**

   ```bash
   # For local development without Groups.io
   export GROUPSIO_SOURCE=mock
   export AUTH_SOURCE=mock
   export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="test-admin"
   export LOG_LEVEL=debug
   ```

4. **Run the Service**

   ```bash
   # Run with default settings
   make run
   ```

## 🏗️ Architecture

The service is built using a clean architecture pattern with the following layers:

- **API Layer**: Goa-generated HTTP handlers and OpenAPI specifications
- **Service Layer**: Business logic and orchestration for mailing list operations
- **Domain Layer**: Core business models, entities, and interfaces
- **Infrastructure Layer**: NATS persistence, JWT authentication, and GroupsIO API integration

### Key Features

- **GroupsIO Service Management**: Complete CRUD operations for GroupsIO service configurations (primary, formation, shared types)
- **Mailing List Management**: Full lifecycle management of mailing lists/subgroups with comprehensive validation
- **Member Management**: Member operations including delivery modes, moderation status, and subscription management
- **GroupsIO Integration**: Direct Groups.io API integration with authentication, retry logic, and timeout configuration
- **Project Integration**: Mailing lists associated with projects and services for organizational structure
- **NATS JetStream Storage**: Scalable and resilient data persistence across multiple KV buckets
- **NATS Messaging**: Event-driven communication for indexing and access control
- **JWT Authentication**: Secure API access via Heimdall integration
- **Mock Mode**: Complete testing capability without external GroupsIO API dependencies
- **OpenAPI Documentation**: Auto-generated API specifications
- **Comprehensive Testing**: Full unit test coverage with mocks
- **ETag Support**: Optimistic concurrency control for update operations
- **Health Checks**: Built-in `/livez` and `/readyz` endpoints for Kubernetes probes
- **Structured Logging**: JSON-formatted logs with contextual information using Go's slog package

## 📁 Project Structure

```bash
lfx-v2-mailing-list-service/
├── cmd/                            # Application entry points
│   └── mailing-list-api/           # Main API server
│       ├── design/                 # Goa API design files
│       │   ├── mailing_list.go     # Service and endpoint definitions
│       │   └── type.go             # Type definitions and data structures
│       ├── service/                # GOA service implementations
│       ├── main.go                 # Application entry point
│       └── http.go                 # HTTP server setup
├── charts/                         # Helm chart for Kubernetes deployment
│   └── lfx-v2-mailing-list-service/
│       ├── templates/              # Kubernetes resource templates
│       ├── values.yaml             # Production configuration
│       └── values.local.yaml       # Local development configuration
├── gen/                            # Generated code (DO NOT EDIT)
│   ├── http/                       # HTTP transport layer
│   │   ├── openapi.yaml            # OpenAPI 2.0 specification
│   │   └── openapi3.yaml           # OpenAPI 3.0 specification
│   └── mailing_list/               # Service interfaces
├── internal/                       # Private application code
│   ├── domain/                     # Business domain layer
│   │   ├── model/                  # Domain models and conversions
│   │   └── port/                   # Repository and service interfaces
│   ├── service/                    # Service layer implementation
│   │   └── grpsio_service_reader.go # GroupsIO service reader
│   ├── infrastructure/             # Infrastructure layer
│   │   ├── auth/                   # JWT authentication
│   │   ├── groupsio/               # GroupsIO API client implementation
│   │   ├── nats/                   # NATS messaging and storage
│   │   │   ├── messaging_publish.go # Message publishing
│   │   │   ├── messaging_request.go # Request/reply messaging
│   │   │   └── storage.go          # KV store repositories
│   │   └── mock/                   # Mock implementations for testing
│   │       ├── auth.go             # Mock authentication
│   │       └── grpsio.go           # Mock GroupsIO repository
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
│   └── utils/                      # Utility functions
├── Dockerfile                      # Container build configuration
├── Makefile                        # Build and development commands
├── CLAUDE.md                       # Claude Code assistant instructions
└── go.mod                          # Go module definition
```

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

#### GroupsIO Integration Development

The GroupsIO integration follows a clean orchestrator pattern:

**Architecture Pattern:**

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

**Configuration Modes:**

- **Production**: `GROUPSIO_SOURCE=real` - Uses actual Groups.io API client
- **Testing**: `GROUPSIO_SOURCE=mock` - Returns nil client, enables pure domain testing
- **Domain Logic**: All business logic flows through `MockRepository` in `internal/infrastructure/mock/grpsio.go`

**Benefits:**
1. **Clean Separation**: Infrastructure (HTTP calls) vs Domain (business logic)
2. **Nil-Safe**: Orchestrator gracefully handles disabled Groups.io integration
3. **Testable**: Domain logic fully tested without external API dependencies
4. **Configurable**: Easy switching between mock and real modes

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
   - Mock GroupsIO integration

2. **Test Individual Endpoints**:
   ```bash
   # Any Bearer token works with mock auth
   curl -H "Authorization: Bearer test-token" \
        http://lfx-v2-mailing-list-service.lfx.svc.cluster.local:8080/groupsio/services?v=1
   ```

**⚠️ Security Warning**: Never use mock authentication in production environments.

## 🚀 Deployment

### Helm Chart

The service includes a Helm chart for Kubernetes deployment:

```bash
# Install with default values
make helm-install

# Install with custom values
helm upgrade --install lfx-v2-mailing-list-service ./charts/lfx-v2-mailing-list-service \
  --namespace lfx \
  --values custom-values.yaml

# Install with GroupsIO credentials
helm upgrade --install lfx-v2-mailing-list-service ./charts/lfx-v2-mailing-list-service \
  --namespace lfx \
  --set groupsio.email="your-email@example.com" \
  --set groupsio.password="your-password"

# View templates
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

The service uses NATS for event-driven communication with other LFX platform services.

### Published Subjects

The service publishes messages to the following NATS subjects:

| Subject | Purpose | Message Schema |
|---------|---------|----------------|
| `lfx.index.groupsio_service` | GroupsIO service indexing events | Indexer message with tags |
| `lfx.index.groupsio_mailing_list` | Mailing list indexing events | Indexer message with tags |
| `lfx.index.groupsio_member` | Member indexing events | Indexer message with tags |
| `lfx.update_access.groupsio_service` | Service access control updates | Access control message |
| `lfx.delete_all_access.groupsio_service` | Service access control deletion | Access control message |
| `lfx.update_access.groupsio_mailing_list` | Mailing list access control updates | Access control message |
| `lfx.delete_all_access.groupsio_mailing_list` | Mailing list access control deletion | Access control message |

### Request/Reply Subjects

The service handles incoming requests on these subjects:

| Subject | Purpose |
|---------|---------|
| `lfx.projects-api.get_slug` | Project slug requests |
| `lfx.projects-api.get_name` | Project name requests |
| `lfx.committee-api.get_name` | Committee name requests |

### Message Publisher Interface

The service uses two message types:

- **Indexer Messages**: For search indexing operations (consumed by indexer services)
- **Access Messages**: For permission management (consumed by fga-sync service)

### Event Flow

When services, mailing lists, or members are modified, the service automatically:

1. **Updates NATS KV storage** for persistence
2. **Publishes indexing messages** for search services
3. **Publishes access control messages** for permission services
4. **Handles cleanup messages** for cascading deletions

## 📖 API Documentation

The service automatically generates OpenAPI documentation:

- **OpenAPI 2.0**: `gen/http/openapi.yaml`
- **OpenAPI 3.0**: `gen/http/openapi3.yaml`
- **JSON formats**: Also available in `gen/http/`

Access the documentation at: `http://localhost:8080/openapi.json`

### Available Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/livez` | GET | Health check |
| `/readyz` | GET | Readiness check |
| `/groupsio/services` | GET, POST | List/create GroupsIO services |
| `/groupsio/services/{uid}` | GET, PUT, DELETE | Get/update/delete service |
| `/groupsio/mailing-lists` | POST | Create mailing list |
| `/groupsio/mailing-lists/{uid}` | GET, PUT, DELETE | Get/update/delete mailing list |
| `/groupsio/mailing-lists/{uid}/members` | GET, POST | List/create members |
| `/groupsio/mailing-lists/{uid}/members/{member_uid}` | GET, PUT, DELETE | Get/update/delete member |

## 🔧 Configuration

The service can be configured via environment variables:

### Core Service Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NATS_URL` | NATS server URL | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` |
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
| `SKIP_ETAG_VALIDATION` | Skip ETag validation (dev only) | `false` |

### GroupsIO Integration Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `GROUPSIO_EMAIL` | Groups.io account email for authentication | Required for production |
| `GROUPSIO_PASSWORD` | Groups.io account password for authentication | Required for production |
| `GROUPSIO_BASE_URL` | Groups.io API base URL | `https://groups.io/api` |
| `GROUPSIO_TIMEOUT` | HTTP timeout for Groups.io API calls | `30s` |
| `GROUPSIO_MAX_RETRIES` | Maximum retry attempts for failed requests | `3` |
| `GROUPSIO_RETRY_DELAY` | Delay between retry attempts | `1s` |
| `GROUPSIO_SOURCE` | Set to `mock` to disable real Groups.io calls | `""` |

### GroupsIO Domain Configuration

The Groups.io domain can be specified in two ways:

1. **API Field Parameter (Recommended)**: Pass the `domain` field in service creation requests
2. **Default**: Uses `groups.io` if no domain is specified

#### Sandbox Testing with Linux Foundation Groups.io

**Important**: For sandbox testing with Linux Foundation's Groups.io tenant, you **must** specify the domain as `linuxfoundation.groups.io` in your API requests.

Example service creation with domain:

```bash
curl -X POST "localhost:8080/groupsio/services?v=1" \
  -H "Content-Type: application/json" \
  -d '{
    "project_uid": "550e8400-e29b-41d4-a716-446655440000",
    "type": "primary",
    "domain": "linuxfoundation.groups.io",
    "global_owners": ["admin@example.com"],
    "project_name": "Test Project"
  }'
```

### Development Environment Variables

For local development with Groups.io integration:

```bash
export GROUPSIO_EMAIL="your-groups-io-email@example.com"
export GROUPSIO_PASSWORD="your-groups-io-password"
export AUTH_SOURCE="mock"
export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="test-admin"
export LOG_LEVEL="debug"
```

For local development without Groups.io:

```bash
export GROUPSIO_SOURCE="mock"
export AUTH_SOURCE="mock"
export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="test-admin"
export LOG_LEVEL="debug"
export NATS_URL="nats://localhost:4222"
```

## 📄 License

Copyright The Linux Foundation and each contributor to LFX.

SPDX-License-Identifier: MIT