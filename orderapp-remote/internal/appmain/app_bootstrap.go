package appmain

import (
	"html/template"
	"strings"

	appconfig "orderapp/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type appConfig = appconfig.Runtime

func loadAppConfig() (appConfig, error) {
	return appconfig.LoadRuntime(nil)
}

func newHTTPServer(cfg appConfig, pool *pgxpool.Pool) *echo.Echo {
	// Note: templates are baked into the image at /app/templates.
	t := template.Must(template.New("").Funcs(templateFuncMap()).ParseGlob("/app/templates/*.html"))

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())
	e.Use(operationLogMiddleware(pool, cfg.Schema))
	e.Use(basicAuth(cfg.AuthUser, cfg.AuthPass, cfg.Schema, pool))
	e.Use(employeeContextMiddleware(pool, cfg.Schema))
	e.Renderer = &TemplateRenderer{t: t}
	return e
}

func employeeContextMiddleware(pool *pgxpool.Pool, schema string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if u, _, ok := c.Request().BasicAuth(); ok {
				if eid, err := resolveEmployeeIDByLogin(c, pool, schema, u); err == nil && eid > 0 {
					c.Set("employee_id", eid)
					if ename, err := resolveEmployeeNameByID(c, pool, schema, eid); err == nil && strings.TrimSpace(ename) != "" {
						c.Set("operator_employee", strings.TrimSpace(ename))
					}
				}
			}
			authz := strings.TrimSpace(c.Request().Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				token := strings.TrimSpace(authz[7:])
				if eid, ename, err := resolveEmployeeBySessionToken(c, pool, schema, token); err == nil && eid > 0 {
					c.Set("employee_id", eid)
					if ename != "" {
						c.Set("operator_employee", ename)
					}
				}
			}
			return next(c)
		}
	}
}
