package sales

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/sales"
	"orderapp/internal/interfaces/http/support"
)

func (h orderAPIHandler) registerOrderReceipt(c echo.Context) error {
	if !h.batchStatusAuthorized(c) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "需要内部账号的订单编辑权限"})
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "订单编号无效"})
	}
	var cmd app.OrderReceiptCommand
	if decodeBatchStatus(c, &cmd) != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "收款资料无效", "not_committed": true})
	}
	cmd.OrderID = id
	cmd.Actor = support.ActorOf(c)
	if err = app.ValidateOrderReceipt(cmd); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error(), "not_committed": true})
	}
	list, err := h.sales.ListOrders(c.Request().Context(), app.OrderListQuery{OrderID: id, Void: "all", Limit: 1})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "查询订单失败"})
	}
	if len(list.Rows) != 1 || list.Rows[0].ID != id {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "订单不存在或无权访问"})
	}
	result, err := h.sales.RegisterOrderReceipt(c.Request().Context(), cmd)
	if err != nil {
		if msg, ok := app.CombinedPaymentValidationMessage(err); ok {
			return c.JSON(http.StatusConflict, map[string]any{"error": msg, "not_committed": msg != "收款操作标识已用于其他操作"})
		}
		if msg, ok := app.OrderEditConflictMessage(err); ok {
			return c.JSON(http.StatusConflict, map[string]any{"error": msg, "not_committed": true})
		}
		c.Logger().Errorf("register order receipt: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "收款结果未确认，请使用同一请求重试"})
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, result)
}

func (h orderAPIHandler) orderReceipts(c echo.Context) error {
	actor, authenticated, err := support.CurrentActor(c, h.authz)
	if err != nil || !authenticated || (!actor.Can("orders.read") && !actor.Can("orders.write")) || actor.AccountType == support.AccountTypeChannelCustomer || support.CustomerFulfillmentOrderScopeLimited(c) || c.Get("portal_customer_id") != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "需要内部账号的订单查看权限"})
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "订单编号无效"})
	}
	list, err := h.sales.ListOrders(c.Request().Context(), app.OrderListQuery{OrderID: id, Void: "all", Limit: 1})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "查询订单失败"})
	}
	if len(list.Rows) != 1 || list.Rows[0].ID != id {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "订单不存在或无权访问"})
	}
	rows, err := h.sales.OrderReceipts(c.Request().Context(), id)
	if err != nil {
		c.Logger().Errorf("order receipts: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "读取收款记录失败"})
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, map[string]any{"receipts": rows})
}
