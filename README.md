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
