package config

import "testing"

func TestLoadRuntimeDefaults(t *testing.T) {
	cfg, err := LoadRuntime(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return "postgres://example"
		case "APP_PASS":
			return "secret"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}
	if cfg.Schema != "p2rms15pepb5ciz" {
		t.Fatalf("Schema = %q", cfg.Schema)
	}
	if cfg.AssetDir != "/app/data/assets" {
		t.Fatalf("AssetDir = %q", cfg.AssetDir)
	}
	if cfg.TemplateDir != "templates" {
		t.Fatalf("TemplateDir = %q", cfg.TemplateDir)
	}
	if cfg.AuthUser != "order" {
		t.Fatalf("AuthUser = %q", cfg.AuthUser)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
}

func TestLoadRuntimeRequiredValues(t *testing.T) {
	if _, err := LoadRuntime(func(string) string { return "" }); err == nil {
		t.Fatal("LoadRuntime() error = nil, want DATABASE_URL error")
	}

	if _, err := LoadRuntime(func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://example"
		}
		return ""
	}); err == nil {
		t.Fatal("LoadRuntime() error = nil, want APP_PASS error")
	}
}

func TestLoadRuntimeTrimsValues(t *testing.T) {
	cfg, err := LoadRuntime(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return " postgres://example "
		case "APP_PASS":
			return " secret "
		case "DB_SCHEMA":
			return " tenant_schema "
		case "TEMPLATE_DIR":
			return " templates "
		case "LISTEN":
			return " :9090 "
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.AuthPass != "secret" {
		t.Fatalf("AuthPass = %q", cfg.AuthPass)
	}
	if cfg.Schema != "tenant_schema" {
		t.Fatalf("Schema = %q", cfg.Schema)
	}
	if cfg.TemplateDir != "templates" {
		t.Fatalf("TemplateDir = %q", cfg.TemplateDir)
	}
	if cfg.ListenAddr != ":9090" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
}
