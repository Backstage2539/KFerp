package customerportal

import (
	"errors"
	"net/http"
	"strconv"

	customerportalapp "orderapp/internal/application/customerportal"
	"orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type portalVisibilityRequest struct {
	DisplayName  string                               `json:"display_name"`
	Enabled      *bool                                `json:"enabled"`
	ThemeKey     string                               `json:"theme_key"`
	Capabilities []customerportalapp.CapabilityOption `json:"capabilities"`
}

func registerAdminAPI(e *echo.Echo, svc Service) {
	e.GET("/api/customer-portal/admin/customers", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		rows, err := svc.ListPortalAdminCustomers(c.Request().Context(), customerportalapp.PortalAdminCustomerQuery{
			Query: c.QueryParam("q"),
			Limit: support.IntParam(c, "limit", 20),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.GET("/api/customer-portal/admin/customers/:id", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
		}
		detail, err := svc.PortalAdminDetail(c.Request().Context(), id)
		if err != nil {
			return portalAdminError(c, err)
		}
		return c.JSON(http.StatusOK, detail)
	})

	e.PUT("/api/customer-portal/admin/customers/:id/visibility", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
		}
		var req portalVisibilityRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		detail, err := svc.UpdatePortalVisibility(c.Request().Context(), customerportalapp.UpdatePortalVisibilityCommand{
			CustomerID:   id,
			DisplayName:  req.DisplayName,
			Enabled:      enabled,
			ThemeKey:     req.ThemeKey,
			Capabilities: req.Capabilities,
			UpdatedBy:    support.ActorOf(c),
		})
		if err != nil {
			return portalAdminError(c, err)
		}
		return c.JSON(http.StatusOK, detail)
	})
}

func portalAdminError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrPortalCustomerNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "customer not found"})
	}
	if isMiniValidationError(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
}
