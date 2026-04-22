package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerProductRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/products", func(c echo.Context) error {
		data := struct {
			Products []ProductOption
			Error    string
		}{}
		ps, err := fetchProducts(c.Request().Context(), pool, schema)
		if err != nil {
			data.Error = err.Error()
		}
		data.Products = ps
		return c.Render(http.StatusOK, "products.html", data)
	})
	e.GET("/products/print", func(c echo.Context) error {
		data := struct {
			Products []ProductOption
			Error    string
		}{}
		ps, err := fetchProducts(c.Request().Context(), pool, schema)
		if err != nil {
			data.Error = err.Error()
		}
		data.Products = ps
		return c.Render(http.StatusOK, "products_print.html", data)
	})
	e.GET("/products/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		p, err := fetchProductByID(c.Request().Context(), pool, schema, id)
		if err != nil {
			return err
		}
		if p == nil {
			return c.String(http.StatusNotFound, "not found")
		}
		return c.Render(http.StatusOK, "product_edit.html", p)
	})
	e.POST("/products/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		mins := c.Request().FormValue("min[]")
		_ = mins
		// parse arrays
		minArr := c.Request().PostForm["min[]"]
		maxArr := c.Request().PostForm["max[]"]
		priceArr := c.Request().PostForm["price[]"]

		ctx := c.Request().Context()
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return err
		}
		defer conn.Release()
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// Replace tiers for simplicity
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.product_price_tiers WHERE product_id=$1", schema), id); err != nil {
			return err
		}
		ins := fmt.Sprintf("INSERT INTO %s.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) VALUES ($1,$2,$3,$4,true)", schema)
		for i := 0; i < len(minArr); i++ {
			mn := strings.TrimSpace(minArr[i])
			if mn == "" {
				continue
			}
			minv, err := strconv.ParseFloat(mn, 64)
			if err != nil {
				continue
			}
			var maxAny any = nil
			if i < len(maxArr) {
				mx := strings.TrimSpace(maxArr[i])
				if mx != "" {
					if mxv, err := strconv.ParseFloat(mx, 64); err == nil {
						maxAny = mxv
					}
				}
			}
			pv := 0.0
			if i < len(priceArr) {
				if f, err := strconv.ParseFloat(strings.TrimSpace(priceArr[i]), 64); err == nil {
					pv = f
				}
			}
			if _, err := tx.Exec(ctx, ins, id, minv, maxAny, pv); err != nil {
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/products/%d", id))
	})

}
