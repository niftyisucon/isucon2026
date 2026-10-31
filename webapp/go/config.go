package isureal

import (
	"net"
	"net/url"
	"os"
)

type Config struct {
	Region          string
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	LogLevel        string
	TracingEnabled  bool
	ImageDir        string
	EdgeUpstream    string
	EdgeKey         string
	ListenRegions   string
	DiagnosticsAddr string
}

func ConfigFromEnv() *Config {
	return &Config{
		Region:          envOrDefault("ISUREAL_REGION", "asia"),
		DBHost:          envOrDefault("ISUREAL_DB_HOST", "127.0.0.1"),
		DBPort:          envOrDefault("ISUREAL_DB_PORT", "5432"),
		DBUser:          envOrDefault("ISUREAL_DB_USER", "isureal"),
		DBPassword:      envOrDefault("ISUREAL_DB_PASSWORD", "isureal"),
		DBName:          envOrDefault("ISUREAL_DB_NAME", "isureal"),
		LogLevel:        envOrDefault("ISUREAL_LOG_LEVEL", "debug"),
		TracingEnabled:  os.Getenv("ISUREAL_TRACING_ENABLED") == "true",
		ImageDir:        os.Getenv("ISUREAL_IMAGE_DIR"),
		EdgeUpstream:    os.Getenv("ISUREAL_EDGE_UPSTREAM"),
		EdgeKey:         os.Getenv("ISUREAL_EDGE_KEY"),
		ListenRegions:   os.Getenv("ISUREAL_LISTEN_REGIONS"),
		DiagnosticsAddr: os.Getenv("ISUREAL_DIAGNOSTICS_ADDR"),
	}
}

func (c *Config) DatabaseDSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   net.JoinHostPort(c.DBHost, c.DBPort),
		Path:   "/" + c.DBName,
	}
	return u.String()
}

func envOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
