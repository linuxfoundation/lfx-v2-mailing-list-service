# LFX V2 MailingList Service

This repository contains the source code for the LFX v2 platform mailing list service.

## Overview

The LFX v2 Mailing List Service is a RESTful API service that manages mailing lists and their members within the Linux Foundation's LFX platform. It provides endpoints for creating, reading, updating, and deleting GroupsIO services, mailing lists, and members with built-in authorization and validation capabilities. The service integrates directly with Groups.io for real mailing list management while supporting mock mode for development and testing. 

## File Structure

```bash
├── .github/                        # GitHub files
│   └── workflows/                  # GitHub Action workflow files
├── charts/                         # Helm charts for running the service in kubernetes
├── cmd/                            # Services (main packages)
│   └── mailing-list-api/          # Mailing list service code
│       ├── design/                 # API design specifications (Goa)
│       ├── service/                # Service implementation
│       ├── main.go                 # Application entry point
│       └── http.go                 # HTTP server setup
├── gen/                            # Generated code from Goa design
├── internal/                       # Internal service packages
│   ├── domain/                     # Domain logic layer (business logic)
│   │   ├── model/                  # Domain models and entities
│   │   └── port/                   # Repository and service interfaces
│   ├── service/                    # Service logic layer (use cases)
│   ├── infrastructure/             # Infrastructure layer
│   │   ├── auth/                   # Authentication implementations
│   │   ├── groupsio/               # GroupsIO API client implementation
│   │   ├── nats/                   # NATS storage implementation
│   │   └── mock/                   # Mock implementations for testing
│   └── middleware/                 # HTTP middleware components
└── pkg/                            # Shared packages
```

## Key Features

- **RESTful API**: Full CRUD operations for GroupsIO services, mailing lists, and member management
- **GroupsIO Integration**: Direct Groups.io API integration with authentication and error handling
- **Service Management**: Support for different GroupsIO service types (primary, formation, shared)
- **Member Management**: Comprehensive mailing list member operations including delivery modes and moderation status
- **Project Integration**: Mailing lists associated with projects and services for organizational structure
- **Clean Architecture**: Follows clean architecture principles with clear separation of domain, service, and infrastructure layers
- **NATS Storage**: Uses NATS key-value buckets for persistent mailing list data storage
- **Authorization**: JWT-based authentication with Heimdall middleware integration
- **Mock Mode**: Complete testing capability without external GroupsIO API dependencies
- **Health Checks**: Built-in `/livez` and `/readyz` endpoints for Kubernetes probes
- **Request Tracking**: Automatic request ID generation and propagation
- **Structured Logging**: JSON-formatted logs with contextual information using Go's slog package
- **Validation**: Comprehensive input validation and business rules enforcement
- **ETag Support**: Optimistic concurrency control for update operations

## Configuration

The service is configured through environment variables:

### Groups.io Integration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GROUPSIO_EMAIL` | Yes* | - | Groups.io account email for authentication |
| `GROUPSIO_PASSWORD` | Yes* | - | Groups.io account password for authentication |
| `GROUPSIO_BASE_URL` | No | `https://api.groups.io` | Groups.io API base URL |
| `GROUPSIO_TIMEOUT` | No | `30s` | HTTP timeout for Groups.io API calls |
| `GROUPSIO_MAX_RETRIES` | No | `3` | Maximum retry attempts for failed requests |
| `GROUPSIO_RETRY_DELAY` | No | `1s` | Delay between retry attempts |
| `GROUPSIO_SOURCE` | No | - | Set to `mock` to disable real Groups.io calls |

*Required for production use (Groups.io integration is always enabled)

### Authentication

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AUTH_SOURCE` | No | `jwt` | Authentication source (`jwt` or `mock`) |
| `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL` | No | - | Mock user for local development (when `AUTH_SOURCE=mock`) |

### Service Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LOG_LEVEL` | No | `info` | Logging level (`debug`, `info`, `warn`, `error`) |
| `PORT` | No | `8080` | HTTP server port |

### Groups.io Domain Configuration

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

### Example Configuration

For development with Groups.io integration:

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
```

## Development

### Prerequisites
- Go 1.24+
- GOA v3
- Docker (optional)

### Getting Started

1. **Clone the repository:**
   ```bash
   git clone https://github.com/linuxfoundation/lfx-v2-mailing-list-service.git
   cd lfx-v2-mailing-list-service
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Generate API code:**
   ```bash
   make generate
   ```

4. **Build the application:**
   ```bash
   make build
   ```

5. **Run the service:**
   ```bash
   make run
   ```

### Available Commands

- `make build` - Build the application
- `make generate` - Generate GOA code from design
- `make run` - Run the service locally
- `make test` - Run tests
- `make clean` - Clean build artifacts

## License

Copyright The Linux Foundation and each contributor to LFX.
SPDX-License-Identifier: MIT
