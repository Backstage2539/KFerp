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
		if strings.TrimSpace(c.QueryParam("legacy")) != "1" {
			return vueShellRedirect(c, "inventory")
		}
		data := FinishedInvPageData{Q: strings.TrimSpace(c.QueryParam("q"))}
		data.Ok = strings.TrimSpace(c.QueryParam("ok")) == "1"
		if qerr := strings.TrimSpace(c.QueryParam("err")); qerr != "" {
			data.Error = qerr
		}
		// products source aligned with 商品档案列表（仅启用商品，按名称排序）
		prods, _ := fetchProducts(c.Request().Context(), pool, schema)
		data.Products = make([]Option, 0, len(prods))
		for _, p := range prods {
			data.Products = append(data.Products, Option{ID: p.ID, Name: p.Name})
		}
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
	e.GET("/api/products/inventory", func(c echo.Context) error {
		q := strings.TrimSpace(c.QueryParam("q"))
		limit := intParam(c, "limit", 50)
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		offset := intParam(c, "offset", 0)
		if page := intParam(c, "page", 0); page > 0 {
			offset = (page - 1) * limit
		}
		rows, hasNext, err := listFinishedInventory(c.Request().Context(), pool, schema, q, limit, offset)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		prods, _ := fetchProducts(c.Request().Context(), pool, schema)
		options := make([]apiOption, 0, len(prods))
		for _, p := range prods {
			options = append(options, apiOption{ID: p.ID, Name: p.Name})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"rows":     rows,
			"products": options,
			"page":     (offset / limit) + 1,
			"limit":    limit,
			"has_prev": offset > 0,
			"has_next": hasNext,
		})
	})
	e.POST("/api/products/inventory", func(c echo.Context) error {
		var req struct {
			ProductID int64 `json:"product_id"`
			SpecG     int64 `json:"spec_g"`
			Units     int64 `json:"units"`
			LooseG    int64 `json:"loose_g"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if err := upsertFinishedInventory(c.Request().Context(), pool, schema, req.ProductID, req.SpecG, req.Units, req.LooseG); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}
