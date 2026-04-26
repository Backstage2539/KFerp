package sales

import (
	"context"
	"fmt"
	"net/http"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type OutsourceTemplateTier struct {
	ID         int64
	TemplateID int64
	MinQty     int64
	MaxQty     *int64
	Multiplier float64
}

func ensureOutsourceTemplateTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q1 := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.outsource_templates (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		is_default BOOLEAN NOT NULL DEFAULT false,
		roast_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		bean_pack_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		drip_pack_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		sc_unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		active BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema)
	if _, err := pool.Exec(ctx, q1); err != nil {
		return err
	}
	q2 := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.outsource_template_tiers (
		id BIGSERIAL PRIMARY KEY,
		template_id BIGINT NOT NULL REFERENCES %s.outsource_templates(id) ON DELETE CASCADE,
		min_qty BIGINT NOT NULL DEFAULT 1,
		max_qty BIGINT,
		multiplier NUMERIC(10,4) NOT NULL DEFAULT 1,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema, schema)
	if _, err := pool.Exec(ctx, q2); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_outsource_templates_default_true ON %s.outsource_templates ((is_default)) WHERE is_default=true`, schema, schema))
	return nil
}

func registerOutsourceSettingsRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	salesSvc := salesapp.NewService(postgressales.NewRepository(pool, schema))

	e.GET("/api/outsource/templates", func(c echo.Context) error {
		rows, err := salesSvc.ListOutsourceTemplates(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "rows": rows})
	})

	e.POST("/api/outsource/templates", func(c echo.Context) error {
		var req salesapp.SaveOutsourceTemplateCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request"})
		}
		if err := salesSvc.SaveOutsourceTemplate(c.Request().Context(), req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/settings/outsource", func(c echo.Context) error {
		target := "/vue-shell?view=outsourceSettings"
		if raw := strings.TrimSpace(c.QueryString()); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, target)
	})

}
