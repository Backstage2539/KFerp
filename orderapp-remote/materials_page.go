package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type MaterialsPageData struct {
	Q    string
	Rows []MaterialRow
	Ok   bool
	Err  string
}

func registerMaterialsPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/materials", func(c echo.Context) error {
		data := MaterialsPageData{Q: strings.TrimSpace(c.QueryParam("q"))}
		data.Ok = strings.TrimSpace(c.QueryParam("ok")) == "1"
		if v := strings.TrimSpace(c.QueryParam("err")); v != "" {
			data.Err = v
		}
		rows, err := listMaterials(c.Request().Context(), pool, schema, data.Q, 200)
		if err != nil {
			data.Err = err.Error()
		} else {
			data.Rows = rows
		}
		return c.Render(http.StatusOK, "materials.html", data)
	})

	e.POST("/materials/save", func(c echo.Context) error {
		code := strings.TrimSpace(c.FormValue("code"))
		name := strings.TrimSpace(c.FormValue("name"))
		kind := strings.TrimSpace(c.FormValue("kind"))
		unit := strings.TrimSpace(c.FormValue("unit"))
		onhandG, _ := parseI64(c.FormValue("onhand_g"))
		onhandUnits, _ := parseI64(c.FormValue("onhand_units"))
		minG, _ := parseI64(c.FormValue("min_g"))
		minUnits, _ := parseI64(c.FormValue("min_units"))

		if err := upsertMaterial(c.Request().Context(), pool, schema, code, name, kind, unit, onhandG, onhandUnits, minG, minUnits); err != nil {
			return c.Redirect(http.StatusSeeOther, "/materials?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/materials?ok=1")
	})
}
