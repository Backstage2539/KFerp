package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	catalogapp "orderapp/internal/application/catalog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerProductRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	h := productHandler{
		catalog: catalogapp.NewService(postgresCatalogRepository{pool: pool, schema: schema}),
	}

	e.GET("/products", h.index)
	e.GET("/products/print", h.print)
	e.GET("/api/products", h.listAPI)
	e.GET("/products/:id", h.edit)
	e.POST("/products/:id", h.update)
}

type productHandler struct {
	catalog *catalogapp.Service
}

func (h productHandler) index(c echo.Context) error {
	if strings.TrimSpace(c.QueryParam("legacy")) != "1" {
		return vueShellRedirect(c, "products")
	}
	return h.renderList(c, "products.html")
}

func (h productHandler) print(c echo.Context) error {
	if strings.TrimSpace(c.QueryParam("legacy")) != "1" {
		return vueShellRedirect(c, "quotePrint")
	}
	return h.renderList(c, "products_print.html")
}

func (h productHandler) listAPI(c echo.Context) error {
	ps, err := h.catalog.ListProducts(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": productOptionsFromCatalog(ps)})
}

func (h productHandler) renderList(c echo.Context, templateName string) error {
	data := struct {
		Products []ProductOption
		Error    string
	}{}
	ps, err := h.catalog.ListProducts(c.Request().Context())
	if err != nil {
		data.Error = err.Error()
	}
	data.Products = productOptionsFromCatalog(ps)
	return c.Render(http.StatusOK, templateName, data)
}

func (h productHandler) edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	p, err := h.catalog.GetProduct(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if p == nil {
		return c.String(http.StatusNotFound, "not found")
	}
	data := productOptionFromCatalog(*p)
	return c.Render(http.StatusOK, "product_edit.html", data)
}

func (h productHandler) update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	roastLevel := normalizeRoastLevel(c.FormValue("roast_level"))
	if roastLevel == "" {
		return c.String(http.StatusBadRequest, "invalid roast_level")
	}
	specArr := c.Request().PostForm["tier_spec_g[]"]
	minArr := c.Request().PostForm["min[]"]
	maxArr := c.Request().PostForm["max[]"]
	priceArr := c.Request().PostForm["price[]"]
	parseRetail := func(field string) (float64, error) {
		v := strings.TrimSpace(c.FormValue(field))
		if v == "" {
			return 0, nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 {
			return 0, fmt.Errorf("invalid %s", field)
		}
		return f, nil
	}
	retailPrice100G, err := parseRetail("retail_price_100g")
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	retailPrice200G, err := parseRetail("retail_price_200g")
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	retailPrice227G, err := parseRetail("retail_price_227g")
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	retailPrice250G, err := parseRetail("retail_price_250g")
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	tiers := make([]catalogapp.PriceTier, 0, len(minArr))
	for i := 0; i < len(minArr); i++ {
		mn := strings.TrimSpace(minArr[i])
		if mn == "" {
			continue
		}
		specG := int64(454)
		if i < len(specArr) {
			if n, err := strconv.ParseInt(strings.TrimSpace(specArr[i]), 10, 64); err == nil && n > 0 {
				specG = n
			}
		}
		minv, err := strconv.ParseFloat(mn, 64)
		if err != nil {
			continue
		}
		var max *float64
		if i < len(maxArr) {
			mx := strings.TrimSpace(maxArr[i])
			if mx != "" {
				if mxv, err := strconv.ParseFloat(mx, 64); err == nil {
					max = &mxv
				}
			}
		}
		pv := 0.0
		if i < len(priceArr) {
			if f, err := strconv.ParseFloat(strings.TrimSpace(priceArr[i]), 64); err == nil {
				pv = f
			}
		}
		tiers = append(tiers, catalogapp.PriceTier{SpecG: specG, MinQty: minv, MaxQty: max, UnitPrice: pv})
	}
	if err := h.catalog.ReplacePriceTiers(c.Request().Context(), catalogapp.ReplacePriceTiersCommand{
		Actor:           actorOf(c),
		ProductID:       id,
		RoastLevel:      roastLevel,
		RetailPrice100G: retailPrice100G,
		RetailPrice200G: retailPrice200G,
		RetailPrice227G: retailPrice227G,
		RetailPrice250G: retailPrice250G,
		Tiers:           tiers,
	}); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/products/%d", id))
}
