// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"

	"testing"
)

// otelEnvVars lists all OTEL-related environment variables used in tests.
var otelEnvVars = []string{
	"OTEL_SERVICE_NAME",
	"OTEL_SERVICE_VERSION",
	"OTEL_EXPORTER_OTLP_PROTOCOL",
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"OTEL_EXPORTER_OTLP_INSECURE",
	"OTEL_TRACES_EXPORTER",
	"OTEL_TRACES_SAMPLE_RATIO",
	"OTEL_METRICS_EXPORTER",
	"OTEL_LOGS_EXPORTER",
	"OTEL_PROPAGATORS",
}

// clearOTelEnvVars clears all OTEL-related environment variables for the
// duration of the test using t.Setenv (auto-restored after the test).
func clearOTelEnvVars(t *testing.T) {
	t.Helper()
	for _, env := range otelEnvVars {
		t.Setenv(env, "")
	}
}

// TestOTelConfigFromEnv_Defaults verifies that OTelConfigFromEnv returns
// sensible default values when no environment variables are set.
func TestOTelConfigFromEnv_Defaults(t *testing.T) {
	// Clear all relevant environment variables
	clearOTelEnvVars(t)

	cfg := OTelConfigFromEnv()

	if cfg.ServiceName != "lfx-v2-mailing-list-service" {
		t.Errorf("expected default ServiceName 'lfx-v2-mailing-list-service', got %q", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "" {
		t.Errorf("expected empty ServiceVersion, got %q", cfg.ServiceVersion)
	}
	if cfg.Protocol != OTelProtocolGRPC {
		t.Errorf("expected default Protocol %q, got %q", OTelProtocolGRPC, cfg.Protocol)
	}
	if cfg.Endpoint != "" {
		t.Errorf("expected empty Endpoint, got %q", cfg.Endpoint)
	}
	if cfg.Insecure != false {
		t.Errorf("expected Insecure false, got %t", cfg.Insecure)
	}
	if cfg.TracesExporter != OTelExporterNone {
		t.Errorf("expected default TracesExporter %q, got %q", OTelExporterNone, cfg.TracesExporter)
	}
	if cfg.TracesSampleRatio != 1.0 {
		t.Errorf("expected default TracesSampleRatio 1.0, got %f", cfg.TracesSampleRatio)
	}
	if cfg.MetricsExporter != OTelExporterNone {
		t.Errorf("expected default MetricsExporter %q, got %q", OTelExporterNone, cfg.MetricsExporter)
	}
	if cfg.LogsExporter != OTelExporterNone {
		t.Errorf("expected default LogsExporter %q, got %q", OTelExporterNone, cfg.LogsExporter)
	}
	if cfg.Propagators != OTelDefaultPropagators {
		t.Errorf("expected default Propagators %q, got %q", OTelDefaultPropagators, cfg.Propagators)
	}
}

// TestOTelConfigFromEnv_CustomValues verifies that OTelConfigFromEnv correctly
// reads and parses all supported OTEL_* environment variables.
func TestOTelConfigFromEnv_CustomValues(t *testing.T) {
	// Set all environment variables
	t.Setenv("OTEL_SERVICE_NAME", "test-service")
	t.Setenv("OTEL_SERVICE_VERSION", "1.2.3")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_TRACES_SAMPLE_RATIO", "0.5")
	t.Setenv("OTEL_METRICS_EXPORTER", "otlp")
	t.Setenv("OTEL_LOGS_EXPORTER", "otlp")
	t.Setenv("OTEL_PROPAGATORS", "tracecontext,baggage")

	cfg := OTelConfigFromEnv()

	if cfg.ServiceName != "test-service" {
		t.Errorf("expected ServiceName 'test-service', got %q", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "1.2.3" {
		t.Errorf("expected ServiceVersion '1.2.3', got %q", cfg.ServiceVersion)
	}
	if cfg.Protocol != OTelProtocolHTTP {
		t.Errorf("expected Protocol %q, got %q", OTelProtocolHTTP, cfg.Protocol)
	}
	if cfg.Endpoint != "localhost:4318" {
		t.Errorf("expected Endpoint 'localhost:4318', got %q", cfg.Endpoint)
	}
	if cfg.Insecure != true {
		t.Errorf("expected Insecure true, got %t", cfg.Insecure)
	}
	if cfg.TracesExporter != OTelExporterOTLP {
		t.Errorf("expected TracesExporter %q, got %q", OTelExporterOTLP, cfg.TracesExporter)
	}
	if cfg.TracesSampleRatio != 0.5 {
		t.Errorf("expected TracesSampleRatio 0.5, got %f", cfg.TracesSampleRatio)
	}
	if cfg.MetricsExporter != OTelExporterOTLP {
		t.Errorf("expected MetricsExporter %q, got %q", OTelExporterOTLP, cfg.MetricsExporter)
	}
	if cfg.LogsExporter != OTelExporterOTLP {
		t.Errorf("expected LogsExporter %q, got %q", OTelExporterOTLP, cfg.LogsExporter)
	}
	if cfg.Propagators != "tracecontext,baggage" {
		t.Errorf("expected Propagators 'tracecontext,baggage', got %q", cfg.Propagators)
	}
}

// TestOTelConfigFromEnv_TracesSampleRatio tests the parsing and validation of
// the OTEL_TRACES_SAMPLE_RATIO environment variable, including edge cases like
// invalid values, out-of-range numbers, and empty strings.
func TestOTelConfigFromEnv_TracesSampleRatio(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectedRatio float64
	}{
		{"valid zero", "0.0", 0.0},
		{"valid half", "0.5", 0.5},
		{"valid one", "1.0", 1.0},
		{"valid small", "0.01", 0.01},
		{"invalid negative", "-0.5", 1.0},      // defaults to 1.0
		{"invalid above one", "1.5", 1.0},      // defaults to 1.0
		{"invalid non-number", "invalid", 1.0}, // defaults to 1.0
		{"empty string", "", 1.0},              // defaults to 1.0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLE_RATIO", tt.envValue)

			cfg := OTelConfigFromEnv()

			if cfg.TracesSampleRatio != tt.expectedRatio {
				t.Errorf("expected TracesSampleRatio %f, got %f", tt.expectedRatio, cfg.TracesSampleRatio)
			}
		})
	}
}

// TestOTelConfigFromEnv_InsecureFlag tests the parsing of the
// OTEL_EXPORTER_OTLP_INSECURE environment variable. Only the literal string
// "true" enables insecure mode; all other values default to false.
func TestOTelConfigFromEnv_InsecureFlag(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true", "true", true},
		{"false", "false", false},
		{"empty", "", false},
		{"TRUE uppercase", "TRUE", false}, // only "true" is recognized
		{"1", "1", false},                 // only "true" is recognized
		{"yes", "yes", false},             // only "true" is recognized
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", tt.envValue)

			cfg := OTelConfigFromEnv()

			if cfg.Insecure != tt.expected {
				t.Errorf("expected Insecure %t, got %t", tt.expected, cfg.Insecure)
			}
		})
	}
}

// TestSetupOTelSDKWithConfig_AllDisabled verifies that the SDK can be
// initialized successfully when all exporters (traces, metrics, logs) are
// disabled, and that the returned shutdown function works correctly.
func TestSetupOTelSDKWithConfig_AllDisabled(t *testing.T) {
	cfg := OTelConfig{
		ServiceName:       "test-service",
		ServiceVersion:    "1.0.0",
		Protocol:          OTelProtocolGRPC,
		TracesExporter:    OTelExporterNone,
		TracesSampleRatio: 1.0,
		MetricsExporter:   OTelExporterNone,
		LogsExporter:      OTelExporterNone,
	}

	ctx := context.Background()
	shutdown, err := SetupOTelSDKWithConfig(ctx, cfg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Call shutdown to ensure it works without error
	err = shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown returned unexpected error: %v", err)
	}
}

// TestSetupOTelSDKWithConfig_ShutdownIdempotent verifies that the shutdown
// function can be called multiple times without error. This is important for
// graceful shutdown scenarios where shutdown may be triggered multiple times.
func TestSetupOTelSDKWithConfig_ShutdownIdempotent(t *testing.T) {
	cfg := OTelConfig{
		ServiceName:       "test-service",
		ServiceVersion:    "1.0.0",
		Protocol:          OTelProtocolGRPC,
		TracesExporter:    OTelExporterNone,
		TracesSampleRatio: 1.0,
		MetricsExporter:   OTelExporterNone,
		LogsExporter:      OTelExporterNone,
	}

	ctx := context.Background()
	shutdown, err := SetupOTelSDKWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Call shutdown multiple times
	err = shutdown(ctx)
	if err != nil {
		t.Errorf("first shutdown returned unexpected error: %v", err)
	}

	// Second call should also succeed (shutdownFuncs is cleared)
	err = shutdown(ctx)
	if err != nil {
		t.Errorf("second shutdown returned unexpected error: %v", err)
	}
}

// TestNewResource verifies that newResource creates a valid OpenTelemetry
// resource with the expected service.name attribute for various input values,
// including edge cases like empty versions and unicode characters.
func TestNewResource(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		serviceVersion string
	}{
		{"basic", "test-service", "1.0.0"},
		{"empty version", "test-service", ""},
		{"unicode name", "测试服务", "2.0.0"},
		{"special chars", "test-service-123", "1.0.0-beta.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := OTelConfig{
				ServiceName:    tt.serviceName,
				ServiceVersion: tt.serviceVersion,
			}

			res, err := newResource(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if res == nil {
				t.Fatal("expected non-nil resource")
			}

			// Verify resource contains expected attributes
			attrs := res.Attributes()
			found := false
			for _, attr := range attrs {
				if string(attr.Key) == "service.name" && attr.Value.AsString() == tt.serviceName {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("resource missing service.name attribute with value %q", tt.serviceName)
			}
		})
	}
}

// TestNewPropagator_Defaults verifies that newPropagator with default config
// returns a composite TextMapPropagator that includes the standard W3C trace
// context fields (traceparent, tracestate), baggage, and jaeger (uber-trace-id).
func TestNewPropagator_Defaults(t *testing.T) {
	cfg := OTelConfig{Propagators: OTelDefaultPropagators}
	prop, err := newPropagator(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prop == nil {
		t.Fatal("expected non-nil propagator")
	}

	fields := prop.Fields()
	expectedFields := map[string]bool{
		"traceparent":  false,
		"tracestate":   false,
		"baggage":      false,
		"uber-trace-id": false,
	}

	for _, field := range fields {
		expectedFields[field] = true
	}

	for field, found := range expectedFields {
		if !found {
			t.Errorf("expected propagator to include field %q", field)
		}
	}
}

// TestNewPropagator_Override verifies that OTEL_PROPAGATORS can override
// the default propagator set to use only a subset.
func TestNewPropagator_Override(t *testing.T) {
	cfg := OTelConfig{Propagators: "tracecontext"}
	prop, err := newPropagator(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields := prop.Fields()
	fieldSet := make(map[string]bool)
	for _, f := range fields {
		fieldSet[f] = true
	}

	if !fieldSet["traceparent"] {
		t.Error("expected traceparent field")
	}
	if fieldSet["baggage"] {
		t.Error("did not expect baggage field with tracecontext-only config")
	}
	if fieldSet["uber-trace-id"] {
		t.Error("did not expect uber-trace-id field with tracecontext-only config")
	}
}

// TestNewPropagator_UnsupportedError verifies that newPropagator returns an
// error when an unsupported propagator name is provided.
func TestNewPropagator_UnsupportedError(t *testing.T) {
	tests := []struct {
		name        string
		propagators string
	}{
		{"unknown propagator", "b3"},
		{"mixed valid and invalid", "tracecontext,b3multi"},
		{"completely invalid", "zipkin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := OTelConfig{Propagators: tt.propagators}
			_, err := newPropagator(cfg)
			if err == nil {
				t.Errorf("expected error for propagators %q, got nil", tt.propagators)
			}
		})
	}
}

// TestNewPropagator_EmptyString verifies that an empty propagators string
// results in a propagator with no fields (no-op composite).
func TestNewPropagator_EmptyString(t *testing.T) {
	cfg := OTelConfig{Propagators: ""}
	prop, err := newPropagator(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields := prop.Fields()
	if len(fields) != 0 {
		t.Errorf("expected no fields for empty propagators, got %v", fields)
	}
}

// TestOTelConstants verifies that the exported OTel constants have their
// expected string values, ensuring API compatibility.
func TestOTelConstants(t *testing.T) {
	// Verify constants have expected values
	if OTelProtocolGRPC != "grpc" {
		t.Errorf("expected OTelProtocolGRPC to be 'grpc', got %q", OTelProtocolGRPC)
	}
	if OTelProtocolHTTP != "http" {
		t.Errorf("expected OTelProtocolHTTP to be 'http', got %q", OTelProtocolHTTP)
	}
	if OTelExporterOTLP != "otlp" {
		t.Errorf("expected OTelExporterOTLP to be 'otlp', got %q", OTelExporterOTLP)
	}
	if OTelExporterNone != "none" {
		t.Errorf("expected OTelExporterNone to be 'none', got %q", OTelExporterNone)
	}
}

// TestSetupOTelSDK tests the convenience function SetupOTelSDK which reads
// configuration from environment variables. With no env vars set, it should
// use defaults and successfully initialize the SDK.
func TestSetupOTelSDK(t *testing.T) {
	// Clear environment to use defaults
	clearOTelEnvVars(t)

	ctx := context.Background()
	shutdown, err := SetupOTelSDK(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	err = shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown returned unexpected error: %v", err)
	}
}

// TestOTelConfig_ZeroValue verifies that a zero-value OTelConfig is safe to use.
// Empty string exporter values are treated as disabled (same as "none").
func TestOTelConfig_ZeroValue(t *testing.T) {
	// A zero-value OTelConfig should be safe to use - empty strings are treated as disabled
	cfg := OTelConfig{}

	// Verify isExporterEnabled treats empty string as disabled
	if isExporterEnabled(cfg.TracesExporter) {
		t.Error("expected zero-value TracesExporter to be treated as disabled")
	}
	if isExporterEnabled(cfg.MetricsExporter) {
		t.Error("expected zero-value MetricsExporter to be treated as disabled")
	}
	if isExporterEnabled(cfg.LogsExporter) {
		t.Error("expected zero-value LogsExporter to be treated as disabled")
	}

	// Verify that a zero-value config can be used to initialize the SDK without error
	ctx := context.Background()
	shutdown, err := SetupOTelSDKWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error with zero-value config: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}
	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown returned unexpected error: %v", err)
	}
}

// TestIsExporterEnabled verifies the isExporterEnabled helper function correctly
// identifies when an exporter should be enabled or disabled.
func TestIsExporterEnabled(t *testing.T) {
	tests := []struct {
		name     string
		exporter string
		expected bool
	}{
		{"otlp enabled", OTelExporterOTLP, true},
		{"none disabled", OTelExporterNone, false},
		{"empty string disabled", "", false},
		{"custom exporter enabled", "custom", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExporterEnabled(tt.exporter)
			if result != tt.expected {
				t.Errorf("isExporterEnabled(%q) = %t, want %t", tt.exporter, result, tt.expected)
			}
		})
	}
}

// TestOTelConfig_MinimalConfig verifies that the SDK can be initialized with
// a minimal configuration where only the exporter settings are specified.
func TestOTelConfig_MinimalConfig(t *testing.T) {
	// Test minimal config with all exporters explicitly disabled
	cfg := OTelConfig{
		TracesExporter:  OTelExporterNone,
		MetricsExporter: OTelExporterNone,
		LogsExporter:    OTelExporterNone,
	}

	ctx := context.Background()
	shutdown, err := SetupOTelSDKWithConfig(ctx, cfg)

	if err != nil {
		t.Fatalf("unexpected error with minimal config: %v", err)
	}

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	err = shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown returned unexpected error: %v", err)
	}
}

// TestEndpointURL verifies that endpointURL prepends the correct scheme
// when missing and preserves existing schemes.
func TestEndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		insecure bool
		want     string
	}{
		{
			name:     "IP:port insecure",
			raw:      "127.0.0.1:4317",
			insecure: true,
			want:     "http://127.0.0.1:4317",
		},
		{
			name:     "IP:port secure",
			raw:      "127.0.0.1:4317",
			insecure: false,
			want:     "https://127.0.0.1:4317",
		},
		{
			name:     "localhost:port insecure",
			raw:      "localhost:4317",
			insecure: true,
			want:     "http://localhost:4317",
		},
		{
			name:     "hostname without port",
			raw:      "collector",
			insecure: true,
			want:     "http://collector",
		},
		{
			name:     "http URL preserved",
			raw:      "http://collector.example.com:4318",
			insecure: false,
			want:     "http://collector.example.com:4318",
		},
		{
			name:     "https URL preserved",
			raw:      "https://collector.example.com:4318",
			insecure: true,
			want:     "https://collector.example.com:4318",
		},
		{
			name:     "https URL with path preserved",
			raw:      "https://collector.example.com:4318/v1/traces",
			insecure: false,
			want:     "https://collector.example.com:4318/v1/traces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := endpointURL(tt.raw, tt.insecure)
			if got != tt.want {
				t.Errorf("endpointURL(%q, %t) = %q, want %q", tt.raw, tt.insecure, got, tt.want)
			}
		})
	}
}

// TestSetupOTelSDKWithConfig_IPEndpoint verifies that SetupOTelSDKWithConfig
// normalizes a bare IP:port endpoint to include a scheme, preventing the
// "first path segment in URL cannot contain colon" error from the SDK.
func TestSetupOTelSDKWithConfig_IPEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "127.0.0.1:4317")

	cfg := OTelConfig{
		ServiceName:       "test-service",
		ServiceVersion:    "1.0.0",
		Protocol:          OTelProtocolGRPC,
		Endpoint:          "127.0.0.1:4317",
		Insecure:          true,
		TracesExporter:    OTelExporterOTLP,
		TracesSampleRatio: 1.0,
		MetricsExporter:   OTelExporterNone,
		LogsExporter:      OTelExporterNone,
		Propagators:       "tracecontext,baggage",
	}

	ctx := context.Background()
	shutdown, err := SetupOTelSDKWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	_ = shutdown(ctx)
}
