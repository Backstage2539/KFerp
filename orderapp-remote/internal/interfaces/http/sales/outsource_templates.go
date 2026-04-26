package sales

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	salesapp "orderapp/internal/application/sales"
	"strconv"
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

func (r postgresSalesRepository) ListOutsourceTemplates(ctx context.Context) ([]salesapp.OutsourceTemplate, error) {
	q := fmt.Sprintf(`SELECT id,name,is_default,COALESCE(roast_unit_price,0),COALESCE(bean_pack_unit_price,0),COALESCE(drip_pack_unit_price,0),COALESCE(sc_unit_price,0)
		FROM %s.outsource_templates WHERE active=true ORDER BY is_default DESC, id DESC`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.OutsourceTemplate, 0)
	for rows.Next() {
		var row salesapp.OutsourceTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.IsDefault, &row.RoastUnitPrice, &row.BeanPackUnitPrice, &row.DripPackUnitPrice, &row.SCUnitPrice); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r postgresSalesRepository) SaveOutsourceTemplate(ctx context.Context, cmd salesapp.SaveOutsourceTemplateCommand) error {
	if cmd.IsDefault {
		if _, err := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.outsource_templates SET is_default=false WHERE is_default=true`, r.schema)); err != nil {
			return err
		}
	}
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.outsource_templates(name,is_default,roast_unit_price,bean_pack_unit_price,drip_pack_unit_price,sc_unit_price,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (name) DO UPDATE SET
			is_default=excluded.is_default,
			roast_unit_price=excluded.roast_unit_price,
			bean_pack_unit_price=excluded.bean_pack_unit_price,
			drip_pack_unit_price=excluded.drip_pack_unit_price,
			sc_unit_price=excluded.sc_unit_price,
			updated_at=now()`, r.schema),
		cmd.Name, cmd.IsDefault, cmd.RoastUnitPrice, cmd.BeanPackUnitPrice, cmd.DripPackUnitPrice, cmd.SCUnitPrice)
	return err
}

func registerOutsourceSettingsRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	salesSvc := salesapp.NewService(postgresSalesRepository{pool: pool, schema: schema})

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

	e.POST("/settings/outsource/save", func(c echo.Context) error {
		name := strings.TrimSpace(c.FormValue("name"))
		if name == "" {
			return c.Redirect(http.StatusSeeOther, "/settings/outsource?err="+url.QueryEscape("name required"))
		}
		parse := func(k string) float64 {
			v := strings.TrimSpace(c.FormValue(k))
			if v == "" {
				return 0
			}
			f, _ := strconv.ParseFloat(v, 64)
			if f < 0 {
				return 0
			}
			return f
		}
		isDefault := strings.TrimSpace(c.FormValue("is_default")) != ""
		err := salesSvc.SaveOutsourceTemplate(c.Request().Context(), salesapp.SaveOutsourceTemplateCommand{
			Name:              name,
			IsDefault:         isDefault,
			RoastUnitPrice:    parse("roast_unit_price"),
			BeanPackUnitPrice: parse("bean_pack_unit_price"),
			DripPackUnitPrice: parse("drip_pack_unit_price"),
			SCUnitPrice:       parse("sc_unit_price"),
		})
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/settings/outsource?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/settings/outsource?ok=1")
	})
}
