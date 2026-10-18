package customerportal

import (
	"errors"
	"github.com/labstack/echo/v4"
	portalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	"strconv"
)

func (h miniEmployeeOrderHandler) copyDetails(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	access, ok, err := h.ensureMiniEmployeeOrderAccess(c, "orders.read")
	if !ok {
		return err
	}
	reader, ok := h.sales.(salesapp.OrderDetailsTextReader)
	if !ok {
		return miniInternalError(c)
	}
	data, err := reader.CopyOrderDetails(c.Request().Context(), access.orderID)
	if err != nil {
		return miniCopyError(c, err)
	}
	return c.JSON(200, data)
}
func registerMiniOrderCopyAPI(e *echo.Echo, portal Service, sales EmployeeSales) {
	e.GET("/api/mini/orders/:id/copy-details", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(401, map[string]string{"error": "请先登录"})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(400, map[string]string{"error": "订单编号无效"})
		}
		if portal == nil {
			return miniInternalError(c)
		}
		current, err := portal.Me(c.Request().Context(), token)
		if err != nil {
			return miniBusinessError(c, err)
		}
		allowed := current.HasAnyCapability([]string{portalapp.CapabilityProductOrder, portalapp.CapabilityDirectShip, portalapp.CapabilityShippingQuery, portalapp.CapabilityMall})
		if !allowed || current.CurrentCustomerID <= 0 {
			return c.JSON(403, map[string]string{"error": "客户订单权限未开通"})
		}
		if err = portal.EnsureOrderAccess(c.Request().Context(), token, id); err != nil {
			return miniBusinessError(c, err)
		}
		reader, ok := sales.(salesapp.OrderDetailsTextReader)
		if !ok {
			return miniInternalError(c)
		}
		data, err := reader.CopyOrderDetails(c.Request().Context(), id)
		if err != nil {
			return miniCopyError(c, err)
		}
		if data.CustomerID != current.CurrentCustomerID {
			return c.JSON(404, map[string]string{"error": "订单不存在或不属于当前客户"})
		}
		return c.JSON(200, data)
	})
}
func miniCopyError(c echo.Context, err error) error {
	if errors.Is(err, salesapp.ErrOrderCopyNotFound) {
		return c.JSON(404, map[string]string{"error": "订单不存在"})
	}
	return miniInternalError(c)
}
