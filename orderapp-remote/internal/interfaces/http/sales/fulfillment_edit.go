package sales

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	fulfillmentapp "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

// FulfillmentEditor is separate from the ordinary sales writer: an edit must
// reconcile customer-owned inventory, its request and the independent fees.
type FulfillmentEditor interface {
	FulfillmentOrderEdit(context.Context, int64) (fulfillmentapp.FulfillmentOrderEditContext, error)
	PreviewFulfillmentOrderEdit(context.Context, fulfillmentapp.FulfillmentOrderEditCommand) (fulfillmentapp.FulfillmentOrderEditPreview, error)
	UpdateFulfillmentOrder(context.Context, fulfillmentapp.FulfillmentOrderEditCommand) (fulfillmentapp.FulfillmentOrderEditResult, error)
}

func (h orderAPIHandler) staffFulfillmentAccess(c echo.Context) (int64, bool, error) {
	actor, authenticated, err := support.CurrentActor(c, h.authz)
	if err != nil {
		return 0, false, c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	if !authenticated {
		return 0, false, c.JSON(http.StatusUnauthorized, map[string]string{"error": "auth required"})
	}
	if support.CustomerFulfillmentOrderScopeLimited(c) || actor.AccountType == support.AccountTypeChannelCustomer || !actor.Can("orders.write") {
		return 0, false, c.JSON(http.StatusForbidden, map[string]string{"error": "请使用有订单编辑权限的内部账号修改代发订单"})
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false, c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid order id"})
	}
	if h.fulfillmentEditor == nil {
		return 0, false, c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "代发编辑服务暂不可用，请稍后重试"})
	}
	return id, true, nil
}

func fulfillmentEditError(c echo.Context, err error) error {
	if message, conflict := salesapp.OrderEditConflictMessage(err); conflict {
		return c.JSON(http.StatusConflict, map[string]string{"error": message})
	}
	return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func (h orderAPIHandler) fulfillmentEdit(c echo.Context) error {
	id, ok, err := h.staffFulfillmentAccess(c)
	if !ok {
		return err
	}
	out, err := h.fulfillmentEditor.FulfillmentOrderEdit(c.Request().Context(), id)
	if err != nil {
		return fulfillmentEditError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (h orderAPIHandler) fulfillmentEditCommand(c echo.Context, save bool) error {
	id, ok, err := h.staffFulfillmentAccess(c)
	if !ok {
		return err
	}
	var cmd fulfillmentapp.FulfillmentOrderEditCommand
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cmd); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "无效代发订单内容，请刷新后重试"})
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "无效代发订单内容"})
	}
	// These identities are authoritative on the server, never supplied by a client.
	cmd.OrderID = id
	cmd.Actor = support.ActorOf(c)
	if cmd.ServiceFeeCents != nil {
		actor, _, _ := support.CurrentActor(c, h.authz)
		if !actor.Can("customers.write") {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "调整代发服务费需要客户费用维护权限"})
		}
	}
	if strings.TrimSpace(cmd.ExpectedEditRevision) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "缺少订单版本，请刷新后重试"})
	}
	if save {
		if strings.TrimSpace(cmd.PreviewToken) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请先重新核对代发商品、库存及费用"})
		}
		out, err := h.fulfillmentEditor.UpdateFulfillmentOrder(c.Request().Context(), cmd)
		if err != nil {
			return fulfillmentEditError(c, err)
		}
		return c.JSON(http.StatusOK, out)
	}
	out, err := h.fulfillmentEditor.PreviewFulfillmentOrderEdit(c.Request().Context(), cmd)
	if err != nil {
		return fulfillmentEditError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}
