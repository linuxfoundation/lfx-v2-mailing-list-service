// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service provides provider functions for initializing service dependencies.
package service

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/auth"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/idmapper"
	infrastructure "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/proxy"
	itxsvc "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service/itx"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// AuthService initializes the authentication service implementation
func AuthService(ctx context.Context) port.Authenticator {
	var authService port.Authenticator

	authSource := os.Getenv("AUTH_SOURCE")
	if authSource == "" {
		authSource = "jwt"
	}

	switch authSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock authentication service")
		authService = infrastructure.NewMockAuthService()
	case "jwt":
		slog.InfoContext(ctx, "initializing JWT authentication service")
		jwtConfig := auth.JWTAuthConfig{
			JWKSURL:            os.Getenv("JWKS_URL"),
			Audience:           os.Getenv("JWT_AUDIENCE"),
			MockLocalPrincipal: os.Getenv("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL"),
		}
		jwtAuth, err := auth.NewJWTAuth(jwtConfig)
		if err != nil {
			log.Fatalf("failed to initialize JWT authentication service: %v", err)
		}
		authService = jwtAuth
	default:
		log.Fatalf("unsupported authentication service implementation: %s", authSource)
	}

	return authService
}

// ITXProxyConfig reads ITX proxy configuration from environment variables.
func ITXProxyConfig() proxy.Config {
	return proxy.Config{
		BaseURL:     os.Getenv("ITX_BASE_URL"),
		ClientID:    os.Getenv("ITX_CLIENT_ID"),
		PrivateKey:  os.Getenv("ITX_CLIENT_PRIVATE_KEY"),
		Auth0Domain: os.Getenv("ITX_AUTH0_DOMAIN"),
		Audience:    os.Getenv("ITX_AUDIENCE"),
		Timeout:     30 * time.Second,
	}
}

// IDMapper initializes the ID mapper based on configuration.
func IDMapper(ctx context.Context) domain.IDMapper {
	if os.Getenv("ID_MAPPING_DISABLED") == "true" {
		slog.WarnContext(ctx, "ID mapping is DISABLED - using no-op mapper (IDs will pass through unchanged)")
		return idmapper.NewNoOpMapper()
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		slog.WarnContext(ctx, "NATS_URL not set, using no-op ID mapper")
		return idmapper.NewNoOpMapper()
	}

	natsMapper, err := idmapper.NewNATSMapper(idmapper.Config{
		URL:     natsURL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		slog.With("error", err).WarnContext(ctx, "Failed to initialize NATS ID mapper, falling back to no-op mapper")
		return idmapper.NewNoOpMapper()
	}

	slog.InfoContext(ctx, "ID mapping enabled - using NATS mapper for v1/v2 ID conversions")
	return natsMapper
}

// GroupsioServiceService initializes the GroupsIO service handler.
func GroupsioServiceService(ctx context.Context, client domain.ITXGroupsioClient, mapper domain.IDMapper) *itxsvc.GroupsioServiceService {
	slog.InfoContext(ctx, "initializing GroupsIO service service")
	return itxsvc.NewGroupsioServiceService(client, mapper)
}

// GroupsioSubgroupService initializes the GroupsIO subgroup handler.
func GroupsioSubgroupService(ctx context.Context, client domain.ITXGroupsioClient, mapper domain.IDMapper) *itxsvc.GroupsioSubgroupService {
	slog.InfoContext(ctx, "initializing GroupsIO subgroup service")
	return itxsvc.NewGroupsioSubgroupService(client, mapper)
}

// GroupsioMemberService initializes the GroupsIO member handler.
func GroupsioMemberService(ctx context.Context, client domain.ITXGroupsioClient) *itxsvc.GroupsioMemberService {
	slog.InfoContext(ctx, "initializing GroupsIO member service")
	return itxsvc.NewGroupsioMemberService(client)
}

// MappingReaderWriter initializes the v1-mappings KV abstraction used by the
// data stream event handler for idempotency tracking.
func MappingReaderWriter(ctx context.Context) port.MappingReaderWriter {
	client := GetNATSClient(ctx)
	kv, err := client.KeyValue(ctx, constants.KVBucketNameV1Mappings)
	if err != nil {
		log.Fatalf("failed to access %s KV bucket: %v", constants.KVBucketNameV1Mappings, err)
	}
	return nats.NewMappingReaderWriter(kv)
}
