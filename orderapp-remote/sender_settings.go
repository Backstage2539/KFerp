package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type SenderProfile struct {
	Name    string `json:"sender_name"`
	Phone   string `json:"sender_phone"`
	Addr    string `json:"sender_addr"`
	Company string `json:"sender_company"`
	Goods   string `json:"sender_goods"`
	BizType string `json:"sf_biz_type"`
}

func ensureSenderSettingsTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sender_settings (
		id SMALLINT PRIMARY KEY DEFAULT 1,
		sender_name TEXT NOT NULL DEFAULT '',
		sender_phone TEXT NOT NULL DEFAULT '',
		sender_addr TEXT NOT NULL DEFAULT '',
		sender_company TEXT NOT NULL DEFAULT '',
		sender_goods TEXT NOT NULL DEFAULT '咖啡',
		sf_biz_type TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	INSERT INTO %s.sender_settings(id) VALUES(1) ON CONFLICT (id) DO NOTHING;`, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func loadSenderProfile(ctx context.Context, pool *pgxpool.Pool, schema string) SenderProfile {
	p := SenderProfile{
		Name:    env("SENDER_NAME", ""),
		Phone:   env("SENDER_PHONE", ""),
		Addr:    env("SENDER_ADDR", ""),
		Company: env("SENDER_COMPANY", ""),
		Goods:   env("SENDER_GOODS", "咖啡"),
		BizType: env("SF_BIZ_TYPE", ""),
	}
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sender_name,''),COALESCE(sender_phone,''),COALESCE(sender_addr,''),COALESCE(sender_company,''),COALESCE(sender_goods,''),COALESCE(sf_biz_type,'') FROM %s.sender_settings WHERE id=1`, schema)).Scan(
		&p.Name, &p.Phone, &p.Addr, &p.Company, &p.Goods, &p.BizType,
	)
	if strings.TrimSpace(p.Goods) == "" {
		p.Goods = "咖啡"
	}
	return p
}

func registerSenderSettingsPage(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/settings/sender", func(c echo.Context) error {
		if strings.TrimSpace(c.QueryParam("legacy")) != "1" {
			return vueShellRedirect(c, "senderSettings")
		}
		p := loadSenderProfile(c.Request().Context(), pool, schema)
		return c.Render(http.StatusOK, "sender_settings.html", map[string]any{"P": p, "Ok": c.QueryParam("ok") == "1"})
	})
	e.POST("/settings/sender", func(c echo.Context) error {
		_, _ = pool.Exec(c.Request().Context(), fmt.Sprintf(`UPDATE %s.sender_settings SET sender_name=$1,sender_phone=$2,sender_addr=$3,sender_company=$4,sender_goods=$5,sf_biz_type=$6,updated_at=now() WHERE id=1`, schema),
			strings.TrimSpace(c.FormValue("sender_name")),
			strings.TrimSpace(c.FormValue("sender_phone")),
			strings.TrimSpace(c.FormValue("sender_addr")),
			strings.TrimSpace(c.FormValue("sender_company")),
			strings.TrimSpace(c.FormValue("sender_goods")),
			strings.TrimSpace(c.FormValue("sf_biz_type")),
		)
		return c.Redirect(http.StatusSeeOther, "/settings/sender?ok=1")
	})
	e.GET("/api/settings/sender", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"profile": loadSenderProfile(c.Request().Context(), pool, schema)})
	})
	e.POST("/api/settings/sender", func(c echo.Context) error {
		var p SenderProfile
		if err := c.Bind(&p); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if err := saveSenderProfile(c.Request().Context(), pool, schema, p); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}

func saveSenderProfile(ctx context.Context, pool *pgxpool.Pool, schema string, p SenderProfile) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.sender_settings SET sender_name=$1,sender_phone=$2,sender_addr=$3,sender_company=$4,sender_goods=$5,sf_biz_type=$6,updated_at=now() WHERE id=1`, schema),
		strings.TrimSpace(p.Name),
		strings.TrimSpace(p.Phone),
		strings.TrimSpace(p.Addr),
		strings.TrimSpace(p.Company),
		strings.TrimSpace(p.Goods),
		strings.TrimSpace(p.BizType),
	)
	return err
}
