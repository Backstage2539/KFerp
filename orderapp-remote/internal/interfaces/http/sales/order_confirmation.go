package sales

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
)

func confirmationRequestHash(req orderSaveAPIRequest) string {
	raw, _ := json.Marshal(req)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
func (h orderAPIHandler) confirmationAccess(c echo.Context) (salesapp.OrderConfirmation, error) {
	if err := h.requireCustomerOrdersCapability(c); err != nil {
		return salesapp.OrderConfirmation{}, err
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	status, err := h.sales.OrderConfirmation(c.Request().Context(), id)
	if err != nil {
		return status, echo.NewHTTPError(400, err.Error())
	}
	if support.CustomerFulfillmentOrderScopeLimited(c) {
		if err := h.ensureFulfillmentOrderDetailAccess(c, id, status.CustomerID); err != nil {
			return status, err
		}
	}
	actor, _, err := support.CurrentActor(c, h.authz)
	if err != nil {
		return status, echo.NewHTTPError(403, err.Error())
	}
	status.CanEdit = status.CanEdit && (actor.Can("orders.write") || (support.CustomerFulfillmentOrderScopeLimited(c) && actor.Can("customer_processing.read")))
	employee := support.CurrentEmployeeID(c)
	status.CanConfirm = actor.AccountType != support.AccountTypeChannelCustomer && !support.CustomerFulfillmentOrderScopeLimited(c) && status.Required && status.Status == "pending" && (actor.IsAdmin() || (employee > 0 && employee == status.ResponsibleEmployeeID))
	return status, nil
}
func (h orderAPIHandler) confirmation(c echo.Context) error {
	status, err := h.confirmationAccess(c)
	if err != nil {
		return err
	}
	return c.JSON(200, status)
}
func (h orderAPIHandler) reviewConfirmation(c echo.Context) error {
	if support.CustomerFulfillmentOrderScopeLimited(c) {
		return c.JSON(403, map[string]string{"error": "请由客户负责人或管理员确认订单"})
	}
	status, err := h.confirmationAccess(c)
	if err != nil {
		return err
	}
	if !status.CanConfirm {
		return c.JSON(403, map[string]string{"error": "仅客户负责人或管理员可以处理待确认订单"})
	}
	var cmd salesapp.ReviewOrderCommand
	if err := c.Bind(&cmd); err != nil {
		return c.JSON(400, map[string]string{"error": "请求格式错误"})
	}
	actor, _, err := support.CurrentActor(c, h.authz)
	if err != nil {
		return echo.NewHTTPError(403, err.Error())
	}
	cmd.OrderID = status.OrderID
	cmd.EmployeeID = support.CurrentEmployeeID(c)
	cmd.Admin = actor.IsAdmin()
	cmd.Actor = support.ActorOf(c)
	result, err := h.sales.ReviewOrder(c.Request().Context(), cmd)
	if err != nil {
		return c.JSON(409, map[string]string{"error": err.Error()})
	}
	if cmd.Decision == "accepted" && status.AcceptedRevision == 0 {
		h.publishOrderCreated(c, salesapp.SaveOrderResult{OrderID: status.OrderID, OrderNo: status.OrderNo})
	}
	return c.JSON(200, result)
}
