package sales

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

// Staff fulfillment edits change only the recipient. Stock-backed lines have
// already been paid for and must never pass through ordinary order repricing.
func (h orderAPIHandler) staffRecipient(c echo.Context) error {
	actor, authenticated, err := support.CurrentActor(c, h.authz)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	if !authenticated {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "auth required"})
	}
	if support.CustomerFulfillmentOrderScopeLimited(c) || actor.AccountType == support.AccountTypeChannelCustomer || !actor.Can("orders.write") {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "请使用有订单编辑权限的内部账号修改收件信息"})
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid order id"})
	}
	var req struct {
		CustomerID      int64   `json:"customer_id"`
		EditRevision    string  `json:"edit_revision"`
		ReceiverName    string  `json:"receiver_name"`
		ReceiverPhone   string  `json:"receiver_phone"`
		ReceiverAddress string  `json:"receiver_address"`
		ReceiverCompany *string `json:"receiver_company"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "仅支持修改收件人、电话、地址和公司"})
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "无效收件信息"})
	}
	order, err := h.sales.OrderConfirmation(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "订单不存在"})
	}
	if req.CustomerID != 0 && req.CustomerID != order.CustomerID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "订单不属于所选客户"})
	}
	cmd := salesapp.CustomerRecipientCommand{
		OrderID: id, CustomerID: order.CustomerID, Actor: support.ActorOf(c),
		ReceiverName: req.ReceiverName, ReceiverPhone: req.ReceiverPhone,
		ReceiverAddress: req.ReceiverAddress, ReceiverCompany: req.ReceiverCompany,
		ExpectedEditRevision: req.EditRevision, StaffFulfillmentEdit: true,
	}
	if err := h.sales.UpdateCustomerRecipient(c.Request().Context(), cmd); err != nil {
		if message, conflict := salesapp.OrderEditConflictMessage(err); conflict {
			return c.JSON(http.StatusConflict, map[string]string{"error": message})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "order_id": id})
}
