package sales

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type OutsourceTemplate struct {
	ID                int64
	Name              string
	IsDefault         bool
	RoastUnitPrice    float64
	BeanPackUnitPrice float64
	DripPackUnitPrice float64
	SCUnitPrice       float64
}

type OutsourceTemplateTier struct {
	ID         int64
	TemplateID int64
	MinQty     int64
	MaxQty     *int64
	Multiplier float64
}

type OutsourceSettingsPageData struct {
	Rows []OutsourceTemplate
	Ok   bool
	Err  string
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

func listOutsourceTemplates(ctx context.Context, pool *pgxpool.Pool, schema string) ([]OutsourceTemplate, error) {
	q := fmt.Sprintf(`SELECT id,name,is_default,COALESCE(roast_unit_price,0),COALESCE(bean_pack_unit_price,0),COALESCE(drip_pack_unit_price,0),COALESCE(sc_unit_price,0)
		FROM %s.outsource_templates WHERE active=true ORDER BY is_default DESC, id DESC`, schema)
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OutsourceTemplate, 0)
	for rows.Next() {
		var r OutsourceTemplate
		if err := rows.Scan(&r.ID, &r.Name, &r.IsDefault, &r.RoastUnitPrice, &r.BeanPackUnitPrice, &r.DripPackUnitPrice, &r.SCUnitPrice); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func registerOutsourceSettingsRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/api/outsource/templates", func(c echo.Context) error {
		rows, err := listOutsourceTemplates(c.Request().Context(), pool, schema)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "rows": rows})
	})

	e.GET("/settings/outsource", func(c echo.Context) error {
		data := OutsourceSettingsPageData{Ok: strings.TrimSpace(c.QueryParam("ok")) == "1"}
		if v := strings.TrimSpace(c.QueryParam("err")); v != "" {
			data.Err = v
		}
		rows, err := listOutsourceTemplates(c.Request().Context(), pool, schema)
		if err != nil {
			data.Err = err.Error()
		} else {
			data.Rows = rows
		}
		return c.Render(http.StatusOK, "outsource_settings.html", data)
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
		if isDefault {
			_, _ = pool.Exec(c.Request().Context(), fmt.Sprintf(`UPDATE %s.outsource_templates SET is_default=false WHERE is_default=true`, schema))
		}
		_, err := pool.Exec(c.Request().Context(), fmt.Sprintf(`INSERT INTO %s.outsource_templates(name,is_default,roast_unit_price,bean_pack_unit_price,drip_pack_unit_price,sc_unit_price,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,now())
			ON CONFLICT (name) DO UPDATE SET
				is_default=excluded.is_default,
				roast_unit_price=excluded.roast_unit_price,
				bean_pack_unit_price=excluded.bean_pack_unit_price,
				drip_pack_unit_price=excluded.drip_pack_unit_price,
				sc_unit_price=excluded.sc_unit_price,
				updated_at=now()`, schema), name, isDefault, parse("roast_unit_price"), parse("bean_pack_unit_price"), parse("drip_pack_unit_price"), parse("sc_unit_price"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/settings/outsource?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/settings/outsource?ok=1")
	})
}
