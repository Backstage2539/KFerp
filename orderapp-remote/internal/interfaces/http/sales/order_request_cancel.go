package sales

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/sales"
	"orderapp/internal/interfaces/http/support"
)

// This administrative action is deliberately separate from ordinary order
// permissions: it permanently prevents a matching late request from creating.
func (h orderAPIHandler) resolvePendingOrderRequest(c echo.Context) error {
	actor, ok, err := support.CurrentActor(c, h.authz)
	if err != nil || !ok || !actor.IsAdmin() || support.CustomerFulfillmentOrderScopeLimited(c) || c.Get("portal_customer_id") != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "仅管理员可以核对并解除待确认请求"})
	}
	var input struct {
		Original      orderSaveAPIRequest `json:"original"`
		RequestID     string              `json:"request_id"`
		OriginalActor string              `json:"original_actor"`
		Reason        string              `json:"reason"`
	}
	if c.Bind(&input) != nil || input.Original.EditID != 0 || (strings.TrimSpace(input.Original.RequestID) == "" && strings.TrimSpace(input.RequestID) == "") || strings.TrimSpace(input.Reason) == "" {
		return c.JSON(400, map[string]string{"error": "原请求和核对原因必填"})
	}
	owner := strings.TrimSpace(input.OriginalActor)
	if owner == "" {
		owner = support.ActorOf(c)
	}
	id, hash := strings.TrimSpace(input.Original.RequestID), confirmationRequestHash(input.Original)
	if id == "" {
		id = strings.TrimSpace(input.RequestID)
		owner = "unknown"
		hash = "unknown"
	}
	result, err := h.sales.CancelUncommittedOrderRequest(c.Request().Context(), app.OrderRequestCancelCommand{RequestID: id, RequestActor: owner, RequestHash: hash, Actor: support.ActorOf(c), Reason: strings.TrimSpace(input.Reason)})
	if err != nil {
		c.Logger().Errorf("resolve pending order request: %v", err)
		return c.JSON(409, map[string]string{"error": "核对或解除未确认，请保留原请求并联系管理员"})
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(200, result)
}

func (h orderAPIHandler) pendingOrderRequestStatus(c echo.Context) error {
	actor, ok, err := support.CurrentActor(c, h.authz)
	if err != nil || !ok || actor.AccountType == support.AccountTypeChannelCustomer || !actor.Can("orders.write") || support.CustomerFulfillmentOrderScopeLimited(c) || c.Get("portal_customer_id") != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "需要内部订单编辑权限"})
	}
	cancelled, err := h.sales.OrderRequestCancelled(c.Request().Context(), c.Param("requestID"))
	if err != nil {
		return c.JSON(409, map[string]string{"error": "无法核对请求状态，请保留原请求"})
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(200, map[string]bool{"cancelled": cancelled})
}
