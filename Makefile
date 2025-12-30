# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

APP_NAME := lfx-v2-mailing-list-service
VERSION := $(shell git describe --tags --always)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse HEAD)

# Goa CLI
GOA_VERSION := v3.21.5
MODULE      := $(shell go list -m)

# Docker
DOCKER_REGISTRY := linuxfoundation
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(APP_NAME)
DOCKER_TAG := $(VERSION)

# Helm variables
HELM_CHART_PATH=./charts/lfx-v2-mailing-list-service
HELM_RELEASE_NAME=lfx-v2-mailing-list-service
HELM_NAMESPACE=lfx
HELM_VALUES_FILE=./charts/lfx-v2-mailing-list-service/values.local.yaml

# Go
GO_VERSION := 1.24.0
GOOS := linux
GOARCH := amd64

# Linting
GOLANGCI_LINT_VERSION := v2.3.1
LINT_TIMEOUT := 10m
LINT_TOOL=$(shell go env GOPATH)/bin/golangci-lint

##@ Development

.PHONY: setup-dev
setup-dev: ## Setup development tools
	@echo "Installing development tools..."
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: setup
setup: ## Setup development environment
	@echo "Setting up development environment..."
	go mod download
	go mod tidy

.PHONY: deps
deps: ## Install dependencies
	@echo "Installing dependencies..."
	go install goa.design/goa/v3/cmd/goa@$(GOA_VERSION)

.PHONY: apigen
apigen: ## Generate API code using Goa
	go run goa.design/goa/v3/cmd/goa@$(GOA_VERSION) gen $(MODULE)/cmd/mailing-list-api/design

.PHONY: lint
lint: ## Run golangci-lint (local Go linting)
	@echo "Running golangci-lint..."
	@which golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..." && go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION))
	@golangci-lint run ./... && echo "==> Lint OK"

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

.PHONY: build
build: ## Build the application for local OS
	@echo "Building application for local development..."
	go build \
		-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)" \
		-o bin/$(APP_NAME) ./cmd/mailing-list-api

.PHONY: run
run: build ## Run the application for local development
	@echo "Running application for local development..."
	./bin/$(APP_NAME)

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest


.PHONY: docker-run
docker-run: ## Run Docker container locally
	@echo "Running Docker container..."
	@docker rm -f $(APP_NAME) >/dev/null 2>&1 || true
	docker run \
		--rm \
		--name $(APP_NAME) \
		-p 8080:8080 \
		-e NATS_URL=nats://lfx-platform-nats.lfx.svc.cluster.local:4222 \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

##@ Helm/Kubernetes
# Install Helm chart
.PHONY: helm-install
helm-install:
	@echo "==> Installing Helm chart..."
	helm upgrade --force --install $(HELM_RELEASE_NAME) $(HELM_CHART_PATH) --namespace $(HELM_NAMESPACE)
	@echo "==> Helm chart installed: $(HELM_RELEASE_NAME)"

# Install Helm chart with local development values (mock authentication)
.PHONY: helm-install-local
helm-install-local:
	@echo "==> Installing Helm chart with local development configuration..."
	helm upgrade --force --install $(HELM_RELEASE_NAME) $(HELM_CHART_PATH) --namespace $(HELM_NAMESPACE) --values $(HELM_VALUES_FILE)
	@echo "==> Helm chart installed with mock authentication: $(HELM_RELEASE_NAME)"

# Print templates for Helm chart
.PHONY: helm-templates
helm-templates:
	@echo "==> Printing templates for Helm chart..."
	helm template $(HELM_RELEASE_NAME) $(HELM_CHART_PATH) --namespace $(HELM_NAMESPACE)
	@echo "==> Templates printed for Helm chart: $(HELM_RELEASE_NAME)"

# Print templates for Helm chart with local values file
.PHONY: helm-templates-local
helm-templates-local:
	@echo "==> Printing templates for Helm chart with local values file..."
	helm template $(HELM_RELEASE_NAME) $(HELM_CHART_PATH) --namespace $(HELM_NAMESPACE) --values $(HELM_VALUES_FILE)
	@echo "==> Templates printed for Helm chart: $(HELM_RELEASE_NAME)"

# Uninstall Helm chart
.PHONY: helm-uninstall
helm-uninstall:
	@echo "==> Uninstalling Helm chart..."
	helm uninstall $(HELM_RELEASE_NAME) --namespace $(HELM_NAMESPACE)
	@echo "==> Helm chart uninstalled: $(HELM_RELEASE_NAME)"

.PHONY: all
all: setup lint test build ## Run common pipeline locally

.PHONY: clean
clean: ## Remove build artifacts and stale containers/images
	@echo "Cleaning..."
	rm -rf bin coverage.out
	@docker rm -f $(APP_NAME) >/dev/null 2>&1 || true
	@docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) >/dev/null 2>&1 || true