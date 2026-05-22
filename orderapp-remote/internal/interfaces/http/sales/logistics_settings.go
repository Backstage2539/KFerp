package sales

import (
	"net/http"
	"strconv"
	"strings"

	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type logisticsSettingsHandler struct {
	sales *salesapp.Service
}

type logisticsCompanyRequest struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Sort   int    `json:"sort"`
	Active bool   `json:"active"`
}

type logisticsProductRequest struct {
	ID        int64  `json:"id"`
	CompanyID int64  `json:"company_id"`
	Name      string `json:"name"`
	Sort      int    `json:"sort"`
	Active    bool   `json:"active"`
}

func registerLogisticsSettingsRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	h := logisticsSettingsHandler{sales: salesSvc}
	e.GET("/settings/logistics", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, "/vue-shell?view=logisticsSettings"))
	})
	e.GET("/api/settings/logistics", h.get)
	e.POST("/api/settings/logistics/companies", h.saveCompany)
	e.POST("/api/settings/logistics/products", h.saveProduct)
	e.PUT("/api/settings/logistics/companies/:id", h.saveCompany)
	e.PUT("/api/settings/logistics/products/:id", h.saveProduct)
}

func (h logisticsSettingsHandler) get(c echo.Context) error {
	rows, err := h.sales.ListLogisticsCompanies(c.Request().Context(), true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"companies": rows})
}

func (h logisticsSettingsHandler) saveCompany(c echo.Context) error {
	var req logisticsCompanyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if id := strings.TrimSpace(c.Param("id")); id != "" {
		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
		}
		req.ID = parsed
	}
	row, err := h.sales.SaveLogisticsCompany(c.Request().Context(), salesapp.SaveLogisticsCompanyCommand{
		Actor:  support.ActorOf(c),
		ID:     req.ID,
		Name:   req.Name,
		Sort:   req.Sort,
		Active: req.Active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"company": row})
}

func (h logisticsSettingsHandler) saveProduct(c echo.Context) error {
	var req logisticsProductRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if id := strings.TrimSpace(c.Param("id")); id != "" {
		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
		}
		req.ID = parsed
	}
	row, err := h.sales.SaveLogisticsProduct(c.Request().Context(), salesapp.SaveLogisticsProductCommand{
		Actor:     support.ActorOf(c),
		ID:        req.ID,
		CompanyID: req.CompanyID,
		Name:      req.Name,
		Sort:      req.Sort,
		Active:    req.Active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"product": row})
}
