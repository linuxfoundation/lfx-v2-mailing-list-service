# LFX V2 MailingList Service

This repository contains the source code for the LFX v2 platform mailing list service.

## Overview

The LFX v2 Mailing List Service is designed to manage mailing lists within the LFX v2 platform. 

## File Structure

```
├── bin/                        # Compiled binaries
├── charts/                     # Kubernetes Helm charts
│   └── lfx-v2-mailing-list-service/
│       └── templates/
├── cmd/                        # Application entry points
│   └── mailing-list-api/
│       ├── design/             # GOA API design files
│       ├── gen/                # Generated GOA code
│       └── service/            # Service implementations
├── gen/                        # Generated code (GOA)
│   ├── http/
│   └── mailing_list/
├── internal/                   # Private application code
│   ├── domain/
│   │   ├── model/              # Domain models
│   │   └── port/               # Interface definitions
│   ├── infrastructure/
│   │   ├── auth/               # JWT authentication
│   │   ├── config/             # Configuration
│   │   ├── mock/               # Mock implementations
│   │   └── nats/               # NATS client and messaging
│   ├── middleware/             # HTTP middleware
│   └── service/                # Business logic layer
├── pkg/                        # Public packages
│   ├── constants/              # Application constants
│   ├── errors/                 # Custom error types
│   └── log/                    # Logging utilities
└── Dockerfile                  # Container build configuration
```

## Key Features

- Health check endpoints for Kubernetes probes 
- JWT authentication integration
- NATS messaging and storage support
- Structured logging with Go's slog package
- Docker containerization
- Kubernetes deployment ready

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
