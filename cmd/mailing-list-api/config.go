// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// cmdFlags holds the command-line flags for the mailing list service.
type cmdFlags struct {
	Debug bool
	Port  string
	Bind  string
}

// environment holds all environment variable configuration for the service.
type environment struct {
	// HTTP
	Port string

	// NATS connection
	NatsURL           string
	NatsTimeout       time.Duration
	NatsMaxReconnect  int
	NatsReconnectWait time.Duration

	// Authentication
	AuthSource       string
	JWKSURL          string
	JWTAudience      string
	JWTMockPrincipal string

	// ID translator
	TranslatorSource   string
	TranslatorMappings string

	// ITX proxy
	ITXBaseURL          string
	ITXClientID         string
	ITXClientPrivateKey string
	ITXAuth0Domain      string
	ITXAudience         string

	// Repository / publisher backend
	RepositorySource string

	// LFID invite feature
	InvitesEnabled   bool
	SelfServeBaseURL string

	// Data stream (eventing)
	EventingEnabled       bool
	EventingConsumerName  string
	EventingMaxDeliver    int
	EventingMaxAckPending int
	EventingAckWaitSecs   int

	// HTTP
	KODataPath string
}

// parseFlags parses command-line flags and returns the result.
func parseFlags(defaultPort string) cmdFlags {
	var debug = flag.Bool("d", false, "enable debug logging")
	var port = flag.String("p", defaultPort, "listen port")
	var bind = flag.String("bind", "*", "interface to bind on")
	flag.Usage = func() {
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	return cmdFlags{Debug: *debug, Port: *port, Bind: *bind}
}

// parseEnv reads all environment variables and returns a populated environment.
func parseEnv() environment {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	natsTimeoutStr := os.Getenv("NATS_TIMEOUT")
	if natsTimeoutStr == "" {
		natsTimeoutStr = "10s"
	}
	natsTimeout, err := time.ParseDuration(natsTimeoutStr)
	if err != nil {
		slog.Warn("invalid NATS_TIMEOUT value, using default", "value", natsTimeoutStr, "default", "10s")
		natsTimeout = 10 * time.Second
	}

	natsReconnectWaitStr := os.Getenv("NATS_RECONNECT_WAIT")
	if natsReconnectWaitStr == "" {
		natsReconnectWaitStr = "2s"
	}
	natsReconnectWait, err := time.ParseDuration(natsReconnectWaitStr)
	if err != nil {
		slog.Warn("invalid NATS_RECONNECT_WAIT value, using default", "value", natsReconnectWaitStr, "default", "2s")
		natsReconnectWait = 2 * time.Second
	}

	authSource := os.Getenv("AUTH_SOURCE")
	if authSource == "" {
		authSource = "jwt"
	}

	translatorSource := os.Getenv("TRANSLATOR_SOURCE")
	if translatorSource == "" {
		translatorSource = "nats"
	}

	translatorMappings := os.Getenv("TRANSLATOR_MAPPINGS_FILE")
	if translatorMappings == "" {
		translatorMappings = "translator_mappings.yaml"
	}

	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	invitesEnabled := strings.EqualFold(os.Getenv("INVITES_ENABLED"), "true") ||
		strings.EqualFold(os.Getenv("INVITES_ENABLED"), "yes")

	selfServeBaseURL := ""
	if invitesEnabled {
		selfServeBaseURL = os.Getenv("LFX_SELF_SERVE_BASE_URL")
		if selfServeBaseURL == "" {
			selfServeBaseURL = selfServeBaseURLForEnv(os.Getenv("LFX_ENVIRONMENT"))
		}
	}

	eventingConsumerName := os.Getenv("EVENTING_CONSUMER_NAME")
	if eventingConsumerName == "" {
		eventingConsumerName = "mailing-list-service-kv-consumer"
	}

	return environment{
		Port:                  port,
		NatsURL:               natsURL,
		NatsTimeout:           natsTimeout,
		NatsMaxReconnect:      envInt("NATS_MAX_RECONNECT", 3),
		NatsReconnectWait:     natsReconnectWait,
		AuthSource:            authSource,
		JWKSURL:               os.Getenv("JWKS_URL"),
		JWTAudience:           os.Getenv("JWT_AUDIENCE"),
		JWTMockPrincipal:      os.Getenv("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL"),
		TranslatorSource:      translatorSource,
		TranslatorMappings:    translatorMappings,
		ITXBaseURL:            os.Getenv("ITX_BASE_URL"),
		ITXClientID:           os.Getenv("ITX_CLIENT_ID"),
		ITXClientPrivateKey:   os.Getenv("ITX_CLIENT_PRIVATE_KEY"),
		ITXAuth0Domain:        os.Getenv("ITX_AUTH0_DOMAIN"),
		ITXAudience:           os.Getenv("ITX_AUDIENCE"),
		RepositorySource:      repoSource,
		InvitesEnabled:        invitesEnabled,
		SelfServeBaseURL:      selfServeBaseURL,
		EventingEnabled:       os.Getenv("EVENTING_ENABLED") == "true",
		EventingConsumerName:  eventingConsumerName,
		EventingMaxDeliver:    envInt("EVENTING_MAX_DELIVER", 3),
		EventingMaxAckPending: envInt("EVENTING_MAX_ACK_PENDING", 1000),
		EventingAckWaitSecs:   envInt("EVENTING_ACK_WAIT_SECS", 30),
		KODataPath:            koDataPath(),
	}
}

// koDataPath returns the path to the ko static data directory.
// KO_DATA_PATH is set by ko at runtime; the fallback is used during local development.
func koDataPath() string {
	if p := os.Getenv("KO_DATA_PATH"); p != "" {
		return p
	}
	return "../../gen/http/"
}

// selfServeBaseURLForEnv returns the default self-serve base URL for the given
// LFX_ENVIRONMENT value. An empty or unrecognised value defaults to production.
func selfServeBaseURLForEnv(env string) string {
	switch strings.ToLower(env) {
	case "staging":
		return "https://app.staging.lfx.dev"
	case "dev":
		return "https://app.dev.lfx.dev"
	default:
		return "https://app.lfx.dev"
	}
}

// envInt reads an integer environment variable, returning defaultVal on absence or parse error.
// A warning is logged when the variable is set but cannot be parsed.
func envInt(key string, defaultVal int) int {
	s := os.Getenv(key)
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		slog.Warn("invalid integer environment variable, using default", "key", key, "value", s, "default", defaultVal)
		return defaultVal
	}
	return n
}
