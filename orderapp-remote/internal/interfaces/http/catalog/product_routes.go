package catalog

import (
	"net/http"
	support "orderapp/internal/interfaces/http/support"
	"strconv"

	catalogapp "orderapp/internal/application/catalog"
	postgrescatalog "orderapp/internal/infrastructure/postgres/catalog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerProductRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	h := productHandler{
		catalog: catalogapp.NewService(postgrescatalog.NewRepository(pool, schema)),
	}

	e.GET("/products", h.index)
	e.GET("/products/print", h.print)
	e.GET("/api/products", h.listAPI)
	e.GET("/api/products/:id", h.detailAPI)
	e.PUT("/api/products/:id", h.updateAPI)
	e.GET("/products/:id", h.edit)
}

type productHandler struct {
	catalog *catalogapp.Service
}

type productUpdateAPIRequest struct {
	RoastLevel      string                    `json:"roast_level"`
	RetailPrice100G float64                   `json:"retail_price_100g"`
	RetailPrice200G float64                   `json:"retail_price_200g"`
	RetailPrice227G float64                   `json:"retail_price_227g"`
	RetailPrice250G float64                   `json:"retail_price_250g"`
	Tiers           []productTierAPIUpsertRow `json:"tiers"`
}

type productTierAPIUpsertRow struct {
	SpecG     int64    `json:"spec_g"`
	MinQty    float64  `json:"min_qty"`
	MaxQty    *float64 `json:"max_qty"`
	UnitPrice float64  `json:"unit_price"`
}

func (h productHandler) index(c echo.Context) error {
	return support.VueShellRedirect(c, "products")
}

func (h productHandler) print(c echo.Context) error {
	return support.VueShellRedirect(c, "quotePrint")
}

func (h productHandler) listAPI(c echo.Context) error {
	ps, err := h.catalog.ListProducts(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": productOptionsFromCatalog(ps)})
}

func (h productHandler) detailAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	p, err := h.catalog.GetProduct(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(*p)})
}

func (h productHandler) updateAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req productUpdateAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	roastLevel := NormalizeRoastLevel(req.RoastLevel)
	if roastLevel == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid roast_level"})
	}
	tiers := make([]catalogapp.PriceTier, 0, len(req.Tiers))
	for _, row := range req.Tiers {
		if row.MinQty <= 0 || row.UnitPrice < 0 {
			continue
		}
		specG := row.SpecG
		if specG <= 0 {
			specG = 454
		}
		tiers = append(tiers, catalogapp.PriceTier{SpecG: specG, MinQty: row.MinQty, MaxQty: row.MaxQty, UnitPrice: row.UnitPrice})
	}
	if err := h.catalog.ReplacePriceTiers(c.Request().Context(), catalogapp.ReplacePriceTiersCommand{
		Actor:           support.ActorOf(c),
		ProductID:       id,
		RoastLevel:      roastLevel,
		RetailPrice100G: req.RetailPrice100G,
		RetailPrice200G: req.RetailPrice200G,
		RetailPrice227G: req.RetailPrice227G,
		RetailPrice250G: req.RetailPrice250G,
		Tiers:           tiers,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	p, err := h.catalog.GetProduct(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": productOptionFromCatalog(*p)})
}

func (h productHandler) edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	return support.VueShellRedirectWith(c, "products", map[string]string{"edit_id": strconv.FormatInt(id, 10)})
}
