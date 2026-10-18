package sales

import (
	"errors"
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/interfaces/http/support"
	"strconv"
)

func (h orderAPIHandler) copyDetails(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	if err := h.requireCustomerOrdersCapability(c); err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(400, "订单编号无效")
	}
	limited := support.CustomerFulfillmentOrderScopeLimited(c)
	var allowedCustomerID int64
	if limited {
		employeeID := support.CurrentEmployeeID(c)
		if employeeID <= 0 {
			return echo.NewHTTPError(403, "无权查看订单")
		}
		result, readErr := h.sales.ListOrders(c.Request().Context(), salesapp.OrderListQuery{OrderID: id, Scope: "fulfillment", EmployeeID: employeeID, FulfillmentEmployeeID: employeeID, Void: "all", Limit: 1})
		if readErr != nil {
			return echo.NewHTTPError(500, "读取订单失败")
		}
		for _, row := range result.Rows {
			if row.ID == id {
				allowedCustomerID = row.CustomerID
				break
			}
		}
		if allowedCustomerID <= 0 {
			return echo.NewHTTPError(403, "无权查看订单")
		}
	}
	data, err := h.sales.CopyOrderDetails(c.Request().Context(), id)
	if err != nil {
		return orderCopyError(err)
	}
	if limited && data.CustomerID != allowedCustomerID {
		return echo.NewHTTPError(403, "无权查看订单")
	}

	return c.JSON(http.StatusOK, data)
}
func (h orderAPIHandler) customerCopyDetails(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	customerID, err := h.accountScope(c, false)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(400, "订单编号无效")
	}
	account, err := h.readAccount(c, app.AccountQuery{CurrentVersion: true, CustomerID: customerID, OrderID: id, IncludeVoid: true, Page: 1, Limit: 1})
	if err != nil {
		return echo.NewHTTPError(500, "读取订单失败")
	}
	if len(account.Rows) != 1 || account.Rows[0].ID != id {
		return echo.NewHTTPError(404, "订单不存在或不属于当前客户")
	}
	data, err := h.sales.CopyOrderDetails(c.Request().Context(), id)
	if err != nil {
		return orderCopyError(err)
	}
	if data.CustomerID != customerID {
		return echo.NewHTTPError(404, "订单不存在或不属于当前客户")
	}
	return c.JSON(http.StatusOK, data)
}
func orderCopyError(err error) error {
	if errors.Is(err, salesapp.ErrOrderCopyNotFound) {
		return echo.NewHTTPError(404, "订单不存在")
	}
	return echo.NewHTTPError(500, "读取订单详情失败，请重试")
}
