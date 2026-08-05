package appmain

import (
	"html/template"
	"path/filepath"

	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	appconfig "orderapp/internal/config"
	postgrescustomerfulfillment "orderapp/internal/infrastructure/postgres/customerfulfillment"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type appConfig = appconfig.Runtime

func loadAppConfig() (appConfig, error) {
	return appconfig.LoadRuntime(nil)
}

func newHTTPServer(cfg appConfig, pool *pgxpool.Pool) *echo.Echo {
	t := template.Must(template.New("").Funcs(supporthttp.TemplateFuncMap()).ParseGlob(filepath.Join(cfg.TemplateDir, "*.html")))

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())
	e.Use(supporthttp.OperationLogMiddleware(pool, cfg.Schema))
	erpLoginEligibility := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, cfg.Schema))
	if shouldInstallBasicAuth(cfg) {
		e.Use(supporthttp.BasicAuth(cfg.AuthUser, cfg.AuthPass, cfg.Schema, pool, erpLoginEligibility))
	}
	e.Use(supporthttp.EmployeeContextMiddleware(pool, cfg.Schema, erpLoginEligibility))
	e.Renderer = supporthttp.NewTemplateRenderer(t)
	return e
}

func shouldInstallBasicAuth(cfg appConfig) bool {
	return !cfg.DisableBasicAuth
}
