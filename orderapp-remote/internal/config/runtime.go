package config

import (
	"fmt"
	"os"
	"strings"
)

// Runtime holds process-level settings needed to compose the application.
type Runtime struct {
	DatabaseURL string
	Schema      string
	AssetDir    string
	AuthUser    string
	AuthPass    string
	ListenAddr  string
}

// LoadRuntime reads runtime configuration from the provided lookup function.
// Pass nil to read from the process environment.
func LoadRuntime(lookup func(string) string) (Runtime, error) {
	if lookup == nil {
		lookup = os.Getenv
	}

	cfg := Runtime{
		DatabaseURL: env(lookup, "DATABASE_URL", ""),
		Schema:      env(lookup, "DB_SCHEMA", "p2rms15pepb5ciz"),
		AssetDir:    env(lookup, "ASSET_DIR", "/app/data/assets"),
		AuthUser:    env(lookup, "APP_USER", "order"),
		AuthPass:    env(lookup, "APP_PASS", ""),
		ListenAddr:  env(lookup, "LISTEN", ":8080"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.AuthPass == "" {
		return cfg, fmt.Errorf("APP_PASS is required")
	}
	return cfg, nil
}

func env(lookup func(string) string, key, def string) string {
	v := strings.TrimSpace(lookup(key))
	if v == "" {
		return def
	}
	return v
}
