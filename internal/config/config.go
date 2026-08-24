package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr              string
	GRPCAddr              string
	DatabaseURL           string
	NATSURL               string
	LogLevel              string
	Environment           string
	ShutdownTimeout       time.Duration
	ExecutionMode         string
	OpenAIAPIKey          string
	OpenAIModel           string
	OpenAISmallModel      string
	OpenAIMediumModel     string
	OpenAILargeModel      string
	OpenAIBaseURL         string
	EnableTestEndpoints   bool
}

func Load() (*Config, error) {
	env := getEnv("SENTINEL_ENVIRONMENT", "development")
	defaultEnableTest := env == "development"

	defaultModel := getEnv("OPENAI_MODEL", "gpt-4o-mini")

	cfg := &Config{
		HTTPAddr:            getEnv("SENTINEL_HTTP_ADDR", "127.0.0.1:8787"),
		GRPCAddr:            getEnv("SENTINEL_GRPC_ADDR", ":9090"),
		DatabaseURL:         getEnv("SENTINEL_DB_URL", ""),
		NATSURL:             getEnv("SENTINEL_NATS_URL", ""),
		LogLevel:            getEnv("SENTINEL_LOG_LEVEL", "info"),
		Environment:         env,
		ShutdownTimeout:     getEnvDuration("SENTINEL_SHUTDOWN_TIMEOUT", 10*time.Second),
		ExecutionMode:       strings.ToUpper(getEnv("SENTINEL_EXECUTION_MODE", "SYNTHETIC")),
		OpenAIAPIKey:        os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:         defaultModel,
		OpenAISmallModel:    getEnv("OPENAI_SMALL_MODEL", defaultModel),
		OpenAIMediumModel:   getEnv("OPENAI_MEDIUM_MODEL", defaultModel),
		OpenAILargeModel:    getEnv("OPENAI_LARGE_MODEL", "gpt-4o"),
		OpenAIBaseURL:       getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		EnableTestEndpoints: getEnvBool("SENTINEL_ENABLE_PROVIDER_TEST_ENDPOINTS", defaultEnableTest),
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		if sec, errInt := strconv.Atoi(val); errInt == nil {
			return time.Duration(sec) * time.Second
		}
		return fallback
	}
	return d
}

// LoggableDatabaseURL returns the DatabaseURL with credentials masked
func (c *Config) LoggableDatabaseURL() string {
	if c.DatabaseURL == "" {
		return ""
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return "invalid-url"
	}
	if u.User != nil {
		password, hasPassword := u.User.Password()
		if hasPassword && password != "" {
			u.User = url.UserPassword(u.User.Username(), "xxx")
		}
	}
	return u.String()
}

func (c *Config) PrintDiagnostics() {
	fmt.Printf("environment=%s\n", c.Environment)
	fmt.Printf("http_addr=%s\n", c.HTTPAddr)
	fmt.Printf("grpc_addr=%s\n", c.GRPCAddr)
	fmt.Printf("execution_mode=%s\n", c.ExecutionMode)
	fmt.Printf("openai_configured=%t\n", c.OpenAIAPIKey != "")
	fmt.Printf("openai_model_small=%s\n", c.OpenAISmallModel)
	fmt.Printf("openai_model_medium=%s\n", c.OpenAIMediumModel)
	fmt.Printf("openai_model_large=%s\n", c.OpenAILargeModel)
	if c.DatabaseURL == "" {
		fmt.Printf("repository=memory\n")
	} else {
		fmt.Printf("repository=postgres (url=%s)\n", c.LoggableDatabaseURL())
	}
}
