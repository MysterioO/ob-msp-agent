package config

import (
	"os"
	"time"
)

type Config struct {
	ServerName      string
	ServerVersion   string
	LogLevel        string
	OTelServiceName string
	OTelEndpoint    string
	MetricsURL      string
	LokiURL         string
	TempoURL        string
	AlertmanagerURL string
	SlackBotToken   string
	SlackDefaultChn string
	GrafanaURL      string
	GrafanaAPIToken string
	QueryTimeout    time.Duration
}

// MetricsQueryURL returns the configured metrics URL.
func (c *Config) MetricsQueryURL() string {
	return c.MetricsURL
}

func Load() (*Config, error) {
	// Defaults
	timeout := 10 * time.Second
	if t := os.Getenv("QUERY_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			timeout = d
		}
	}

	cfg := &Config{
		ServerName:      getEnvStr("SERVER_NAME", "monitoring-mcp"),
		ServerVersion:   getEnvStr("SERVER_VERSION", "1.0.0"),
		LogLevel:        getEnvStr("LOG_LEVEL", "info"),
		OTelServiceName: getEnvStr("OTEL_SERVICE_NAME", "monitoring-mcp"),
		OTelEndpoint:    getEnvStr("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		MetricsURL:      getEnvStr("METRICS_URL", "http://localhost:9090"),
		LokiURL:         getEnvStr("LOKI_URL", "http://localhost:3100"),
		TempoURL:        getEnvStr("TEMPO_URL", "http://localhost:3200"),
		AlertmanagerURL: getEnvStr("ALERTMANAGER_URL", "http://localhost:9093"),
		SlackBotToken:   getEnvStr("SLACK_BOT_TOKEN", ""),
		SlackDefaultChn: getEnvStr("SLACK_DEFAULT_CHANNEL", ""),
		GrafanaURL:      getEnvStr("GRAFANA_URL", "http://localhost:3000"),
		GrafanaAPIToken: getEnvStr("GRAFANA_API_TOKEN", ""),
		QueryTimeout:    timeout,
	}

	return cfg, nil
}

func getEnvStr(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
