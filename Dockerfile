# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

# checkov:skip=CKV_DOCKER_7:No free access to Chainguard versioned labels.
# hadolint global ignore=DL3007

FROM cgr.dev/chainguard/go:latest AS builder

# Set necessary environment variables needed for our image. Allow building to
# other architectures via cross-compilation build-arg.
ARG TARGETARCH
ENV CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH

# Move to working directory /build
WORKDIR /build

# Download dependencies to go modules cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the code into the container
COPY . .

# Build the packages
RUN go build -o /go/bin/mailing-list-svc -trimpath -ldflags="-w -s" ./cmd/mailing-list-api

# Run our go binary standalone
FROM cgr.dev/chainguard/static:latest

# Implicit with base image; setting explicitly for linters.
USER nonroot

EXPOSE 8080

COPY --from=builder /go/bin/mailing-list-svc /cmd/mailing-list-api

ENTRYPOINT ["/cmd/mailing-list-api"]