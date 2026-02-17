package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type BomPageData struct {
	Products        []Option
	BeanMaterials   []Option
	Rows            []BomRow
	Items           []BomItemRow
	SelectedProduct int64
	TotalRatio      float64
	Ok              bool
	Err             string
}

func registerBomPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/bom", func(c echo.Context) error {
		data := BomPageData{Ok: strings.TrimSpace(c.QueryParam("ok")) == "1"}
		if s := strings.TrimSpace(c.QueryParam("err")); s != "" {
			data.Err = s
		}
		if v := strings.TrimSpace(c.QueryParam("product_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				data.SelectedProduct = n
			}
		}

		// Align with 商品档案列表口径：仅 active=true
		data.Products, _ = fetchOptions(c.Request().Context(), pool, "SELECT id, name FROM "+schema+".products WHERE active=true ORDER BY name")
		data.BeanMaterials, _ = fetchOptions(c.Request().Context(), pool, "SELECT id, name FROM "+schema+".materials WHERE kind='bean' ORDER BY name")

		rows, err := listBom(c.Request().Context(), pool, schema)
		if err != nil {
			data.Err = err.Error()
		} else {
			data.Rows = rows
		}
		if data.SelectedProduct > 0 {
			items, total, err := listBomItems(c.Request().Context(), pool, schema, data.SelectedProduct)
			if err != nil {
				data.Err = err.Error()
			} else {
				data.Items = items
				data.TotalRatio = total
			}
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
		return c.Redirect(http.StatusSeeOther, "/bom?ok=1&product_id="+strconv.FormatInt(pid, 10))
	})

	e.POST("/bom/item/save", func(c echo.Context) error {
		pid, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("product_id")), 10, 64)
		mid, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("material_id")), 10, 64)
		ratio, _ := strconv.ParseFloat(strings.TrimSpace(c.FormValue("ratio_pct")), 64)
		if pid <= 0 || mid <= 0 {
			return c.Redirect(http.StatusSeeOther, "/bom?err="+url.QueryEscape("product/material required")+"&product_id="+strconv.FormatInt(pid, 10))
		}
		if ratio <= 0 || ratio > 100 {
			return c.Redirect(http.StatusSeeOther, "/bom?err="+url.QueryEscape("ratio must be (0,100]")+"&product_id="+strconv.FormatInt(pid, 10))
		}
		_, total, err := listBomItems(c.Request().Context(), pool, schema, pid)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/bom?err="+url.QueryEscape(err.Error())+"&product_id="+strconv.FormatInt(pid, 10))
		}
		qNow := "SELECT COALESCE(ratio_pct,0) FROM " + schema + ".product_bom_items WHERE product_id=$1 AND material_id=$2"
		var old float64
		_ = pool.QueryRow(c.Request().Context(), qNow, pid, mid).Scan(&old)
		if total-old+ratio > 100.0001 {
			return c.Redirect(http.StatusSeeOther, "/bom?err="+url.QueryEscape(fmt.Sprintf("ratio sum exceed 100%% (current %.2f)", total))+"&product_id="+strconv.FormatInt(pid, 10))
		}
		q := "INSERT INTO " + schema + ".product_bom_items(product_id,material_id,ratio_pct,updated_at) VALUES($1,$2,$3,now()) ON CONFLICT (product_id,material_id) DO UPDATE SET ratio_pct=excluded.ratio_pct, updated_at=now()"
		if _, err := pool.Exec(c.Request().Context(), q, pid, mid, ratio); err != nil {
			return c.Redirect(http.StatusSeeOther, "/bom?err="+url.QueryEscape(err.Error())+"&product_id="+strconv.FormatInt(pid, 10))
		}
		return c.Redirect(http.StatusSeeOther, "/bom?ok=1&product_id="+strconv.FormatInt(pid, 10))
	})

	e.POST("/bom/item/delete", func(c echo.Context) error {
		pid, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("product_id")), 10, 64)
		id, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("id")), 10, 64)
		if id > 0 {
			_, _ = pool.Exec(c.Request().Context(), "DELETE FROM "+schema+".product_bom_items WHERE id=$1", id)
		}
		return c.Redirect(http.StatusSeeOther, "/bom?ok=1&product_id="+strconv.FormatInt(pid, 10))
	})
}
