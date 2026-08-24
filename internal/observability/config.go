package observability

import (
	"os"
	"strconv"
)

// Config encapsulates telemetry settings for distributed tracing, metrics, and logging.
type Config struct {
	ServiceName    string  `json:"service_name"`
	ServiceVersion string  `json:"service_version"`
	Environment    string  `json:"environment"`
	OTLPEndpoint   string  `json:"otlp_endpoint"`
	PrometheusPort int     `json:"prometheus_port"`
	SamplingRatio  float64 `json:"sampling_ratio"`
	LogLevel       string  `json:"log_level"`
	Enabled        bool    `json:"enabled"`
}

// DefaultConfig returns baseline production-ready observability configuration.
func DefaultConfig(serviceName string) Config {
	port := 9090
	if pStr := os.Getenv("SENTINEL_PROMETHEUS_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil {
			port = p
		}
	}

	sampling := 1.0
	if sStr := os.Getenv("SENTINEL_TRACE_SAMPLING_RATIO"); sStr != "" {
		if s, err := strconv.ParseFloat(sStr, 64); err == nil {
			sampling = s
		}
	}

	env := os.Getenv("SENTINEL_ENV")
	if env == "" {
		env = "development"
	}

	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "localhost:4317"
	}

	return Config{
		ServiceName:    serviceName,
		ServiceVersion: "1.0.0",
		Environment:    env,
		OTLPEndpoint:   otlpEndpoint,
		PrometheusPort: port,
		SamplingRatio:  sampling,
		LogLevel:       "INFO",
		Enabled:        true,
	}
}
