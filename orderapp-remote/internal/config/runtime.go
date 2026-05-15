package config

import (
	"fmt"
	"os"
	"strings"
)

// Runtime holds process-level settings needed to compose the application.
type Runtime struct {
	DatabaseURL              string
	Schema                   string
	AssetDir                 string
	TemplateDir              string
	AuthUser                 string
	AuthPass                 string
	ListenAddr               string
	CustomerPortalDevLogin   bool
	CustomerPortalDevOpenID  string
	CustomerPortalDevUnionID string
	WechatMiniAppID          string
	WechatMiniAppSecret      string
	DocxConverterCommand     string
	DocxConverterURL         string
}

// LoadRuntime reads runtime configuration from the provided lookup function.
// Pass nil to read from the process environment.
func LoadRuntime(lookup func(string) string) (Runtime, error) {
	if lookup == nil {
		lookup = os.Getenv
	}

	cfg := Runtime{
		DatabaseURL:              env(lookup, "DATABASE_URL", ""),
		Schema:                   env(lookup, "DB_SCHEMA", "p2rms15pepb5ciz"),
		AssetDir:                 env(lookup, "ASSET_DIR", "/app/data/assets"),
		TemplateDir:              env(lookup, "TEMPLATE_DIR", "templates"),
		AuthUser:                 env(lookup, "APP_USER", "order"),
		AuthPass:                 env(lookup, "APP_PASS", ""),
		ListenAddr:               env(lookup, "LISTEN", ":8080"),
		CustomerPortalDevLogin:   envBool(lookup, "CUSTOMER_PORTAL_DEV_LOGIN", false),
		CustomerPortalDevOpenID:  env(lookup, "CUSTOMER_PORTAL_DEV_OPENID", "dev-openid-local"),
		CustomerPortalDevUnionID: env(lookup, "CUSTOMER_PORTAL_DEV_UNIONID", ""),
		WechatMiniAppID:          env(lookup, "WECHAT_MINI_APP_ID", ""),
		WechatMiniAppSecret:      env(lookup, "WECHAT_MINI_APP_SECRET", ""),
		DocxConverterCommand:     env(lookup, "DOCX_CONVERTER_CMD", "soffice"),
		DocxConverterURL:         env(lookup, "DOCX_CONVERTER_URL", ""),
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.AuthPass == "" {
		return cfg, fmt.Errorf("APP_PASS is required")
	}
	return cfg, nil
}

func envBool(lookup func(string) string, key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(lookup(key))) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func env(lookup func(string) string, key, def string) string {
	v := strings.TrimSpace(lookup(key))
	if v == "" {
		return def
	}
	return v
}
