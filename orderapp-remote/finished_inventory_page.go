package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type FinishedInvPageData struct {
	Q        string
	Products []Option
	Rows     []FinishedInvRow
	Ok       bool
	Error    string
}

func registerFinishedInventoryPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/products/inventory", func(c echo.Context) error {
		data := FinishedInvPageData{Q: strings.TrimSpace(c.QueryParam("q"))}
		data.Ok = strings.TrimSpace(c.QueryParam("ok")) == "1"
		if qerr := strings.TrimSpace(c.QueryParam("err")); qerr != "" {
			data.Error = qerr
		}
		// products for dropdown
		ps, _ := fetchOptions(c.Request().Context(), pool, "SELECT id, name FROM "+schema+".products ORDER BY name")
		data.Products = ps
		rows, _, err := listFinishedInventory(c.Request().Context(), pool, schema, data.Q, 200, 0)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Rows = rows
		}
		return c.Render(http.StatusOK, "finished_inventory.html", data)
	})
	e.POST("/products/inventory/save", func(c echo.Context) error {
		pid, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("product_id")), 10, 64)
		specG, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("spec_g")), 10, 64)
		units, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("units")), 10, 64)
		looseG, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("loose_g")), 10, 64)
		if err := upsertFinishedInventory(c.Request().Context(), pool, schema, pid, specG, units, looseG); err != nil {
			return c.Redirect(http.StatusSeeOther, "/products/inventory?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/products/inventory?ok=1")
	})
}
