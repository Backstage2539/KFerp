package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type BomPageData struct {
	Products []Option
	Rows     []BomRow
	Ok       bool
	Err      string
}

func registerBomPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/bom", func(c echo.Context) error {
		data := BomPageData{Ok: strings.TrimSpace(c.QueryParam("ok")) == "1"}
		if s := strings.TrimSpace(c.QueryParam("err")); s != "" {
			data.Err = s
		}
		data.Products, _ = fetchOptions(c.Request().Context(), pool, "SELECT id, name FROM "+schema+".products ORDER BY name")
		rows, err := listBom(c.Request().Context(), pool, schema)
		if err != nil {
			data.Err = err.Error()
		} else {
			data.Rows = rows
		}
		return c.Render(http.StatusOK, "bom.html", data)
	})

	e.POST("/bom/save", func(c echo.Context) error {
		pid, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("product_id")), 10, 64)
		yieldRate, _ := strconv.ParseFloat(strings.TrimSpace(c.FormValue("yield_rate")), 64)
		if pid <= 0 {
			return c.Redirect(http.StatusSeeOther, "/bom?err="+url.QueryEscape("product required"))
		}
		if yieldRate <= 0 || yieldRate > 1 {
			return c.Redirect(http.StatusSeeOther, "/bom?err="+url.QueryEscape("yield_rate must be (0,1]"))
		}
		q := "INSERT INTO " + schema + ".product_bom(product_id,yield_rate,updated_at) VALUES($1,$2,now()) ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, updated_at=now()"
		if _, err := pool.Exec(c.Request().Context(), q, pid, yieldRate); err != nil {
			return c.Redirect(http.StatusSeeOther, "/bom?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/bom?ok=1")
	})
}
