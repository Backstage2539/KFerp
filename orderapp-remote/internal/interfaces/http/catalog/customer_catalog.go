package catalog

import (
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/catalog"
	"orderapp/internal/interfaces/http/support"
)

func registerCustomerCatalogRoutes(e *echo.Echo, h productHandler) {
	e.POST("/api/product-settings/customer-catalog/copy", h.copyCustomerCatalogAPI)
	e.POST("/api/product-settings/customer-catalog/remove", h.removeCustomerCatalogAPI)
	e.GET("/api/product-settings/customer-catalog", h.customerCatalogAPI)
	e.PUT("/api/product-settings/customer-catalog/nodes/:id", h.renameCustomerCatalogNodeAPI)
	e.POST("/api/product-settings/customer-catalog/migration", h.migrateCustomerCatalogAPI)
}
func customerCatalogError(c echo.Context, e error) error {
	if app.IsValidationError(e) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": e.Error()})
	}
	c.Logger().Error(e)
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "客户商品目录处理失败，请重试"})
}
func (h productHandler) copyCustomerCatalogAPI(c echo.Context) error {
	var cmd app.CopyCustomerCatalogCommand
	if e := c.Bind(&cmd); e != nil {
		return c.JSON(400, map[string]string{"error": "bad request"})
	}
	cmd.Actor = support.ActorOf(c)
	out, e := h.catalog.CopyCustomerCatalog(c.Request().Context(), cmd)
	if e != nil {
		return customerCatalogError(c, e)
	}
	return c.JSON(200, out)
}
func (h productHandler) customerCatalogAPI(c echo.Context) error {
	id, e := parseOptionalInt64(c.QueryParam("customer_id"))
	if e != nil {
		return c.JSON(400, map[string]string{"error": "invalid customer_id"})
	}
	out, e := h.catalog.CustomerCatalog(c.Request().Context(), id)
	if e != nil {
		return customerCatalogError(c, e)
	}
	return c.JSON(200, out)
}
func (h productHandler) renameCustomerCatalogNodeAPI(c echo.Context) error {
	var cmd app.RenameCustomerCatalogNodeCommand
	if e := c.Bind(&cmd); e != nil {
		return c.JSON(400, map[string]string{"error": "bad request"})
	}
	id, e := parseOptionalInt64(c.Param("id"))
	if e != nil {
		return c.JSON(400, map[string]string{"error": "invalid id"})
	}
	cmd.ID = id
	cmd.Actor = support.ActorOf(c)
	if e = h.catalog.RenameCustomerCatalogNode(c.Request().Context(), cmd); e != nil {
		return customerCatalogError(c, e)
	}
	return c.JSON(200, map[string]bool{"ok": true})
}
func (h productHandler) migrateCustomerCatalogAPI(c echo.Context) error {
	var req struct {
		Apply bool `json:"apply"`
	}
	if e := c.Bind(&req); e != nil {
		return c.JSON(400, map[string]string{"error": "bad request"})
	}
	out, e := h.catalog.MigrateCustomerCatalog(c.Request().Context(), !req.Apply, support.ActorOf(c))
	if e != nil {
		return customerCatalogError(c, e)
	}
	return c.JSON(200, out)
}

func (h productHandler) removeCustomerCatalogAPI(c echo.Context) error {
	var cmd app.CopyCustomerCatalogCommand
	if e := c.Bind(&cmd); e != nil {
		return c.JSON(400, map[string]string{"error": "bad request"})
	}
	cmd.Actor = support.ActorOf(c)
	if e := h.catalog.RemoveCustomerCatalogProducts(c.Request().Context(), cmd); e != nil {
		return customerCatalogError(c, e)
	}
	return c.JSON(200, map[string]bool{"ok": true})
}
