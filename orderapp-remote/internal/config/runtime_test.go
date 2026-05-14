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
	if cfg.CustomerPortalDevLogin {
		t.Fatal("CustomerPortalDevLogin = true, want default false")
	}
	if cfg.DocxConverterCommand != "soffice" {
		t.Fatalf("DocxConverterCommand = %q", cfg.DocxConverterCommand)
	}
	if cfg.DocxConverterURL != "" {
		t.Fatalf("DocxConverterURL = %q", cfg.DocxConverterURL)
	}
}

func TestLoadRuntimeCustomerPortalDevLogin(t *testing.T) {
	cfg, err := LoadRuntime(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return "postgres://example"
		case "APP_PASS":
			return "secret"
		case "CUSTOMER_PORTAL_DEV_LOGIN":
			return " true "
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}
	if !cfg.CustomerPortalDevLogin {
		t.Fatal("CustomerPortalDevLogin = false, want true")
	}
}

func TestLoadRuntimeCustomerPortalWechatConfig(t *testing.T) {
	cfg, err := LoadRuntime(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return "postgres://example"
		case "APP_PASS":
			return "secret"
		case "WECHAT_MINI_APP_ID":
			return " wx-test-app "
		case "WECHAT_MINI_APP_SECRET":
			return " wx-test-secret "
		case "CUSTOMER_PORTAL_DEV_OPENID":
			return " dev-openid-van "
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}
	if cfg.WechatMiniAppID != "wx-test-app" {
		t.Fatalf("WechatMiniAppID = %q", cfg.WechatMiniAppID)
	}
	if cfg.WechatMiniAppSecret != "wx-test-secret" {
		t.Fatalf("WechatMiniAppSecret = %q", cfg.WechatMiniAppSecret)
	}
	if cfg.CustomerPortalDevOpenID != "dev-openid-van" {
		t.Fatalf("CustomerPortalDevOpenID = %q", cfg.CustomerPortalDevOpenID)
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
		case "DOCX_CONVERTER_CMD":
			return " /usr/local/bin/soffice "
		case "DOCX_CONVERTER_URL":
			return " http://docconvert:3000/forms/libreoffice/convert "
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
	if cfg.DocxConverterCommand != "/usr/local/bin/soffice" {
		t.Fatalf("DocxConverterCommand = %q", cfg.DocxConverterCommand)
	}
	if cfg.DocxConverterURL != "http://docconvert:3000/forms/libreoffice/convert" {
		t.Fatalf("DocxConverterURL = %q", cfg.DocxConverterURL)
	}
}
