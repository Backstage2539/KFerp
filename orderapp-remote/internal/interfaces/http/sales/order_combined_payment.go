package sales

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/interfaces/http/support"
)

// combinedPayment is one all-or-nothing receipt, unlike the existing per-order
// batch endpoint. Request amounts are integral cents, not JSON floats.
func (h orderAPIHandler) combinedPayment(c echo.Context) error {
	if !h.batchStatusAuthorized(c) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "需要内部账号的订单编辑权限"})
	}
	var request struct {
		Orders                []salesapp.CombinedPaymentOrder `json:"orders"`
		ActualCents           *int64                          `json:"actual_cents"`
		ExpectedDueCents      *int64                          `json:"expected_due_cents"`
		PaymentMethod         string                          `json:"payment_method"`
		PaymentVoucherAssetID int64                           `json:"payment_voucher_asset_id"`
		ShipStatusID          *int64                          `json:"ship_status_id"`
		IdempotencyKey        string                          `json:"idempotency_key"`
	}
	if decodeBatchStatus(c, &request) != nil || len(request.Orders) < 1 || len(request.Orders) > 200 || request.ActualCents == nil || request.ExpectedDueCents == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "合并收款资料无效，请刷新后重试"})
	}
	// Recheck visibility, but do not compare revisions here: the transaction
	// owns the authoritative revision checks and must be able to replay an
	// already committed idempotent request after order versions changed.
	seen := make(map[int64]bool, len(request.Orders))
	for _, order := range request.Orders {
		if order.OrderID <= 0 || strings.TrimSpace(order.EditRevision) == "" || seen[order.OrderID] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "订单重复或版本缺失，请刷新后重试"})
		}
		seen[order.OrderID] = true
		list, err := h.sales.ListOrders(c.Request().Context(), salesapp.OrderListQuery{OrderID: order.OrderID, Void: "all", Limit: 1})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "查询订单失败，请稍后重试"})
		}
		if len(list.Rows) != 1 || list.Rows[0].ID != order.OrderID {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "订单不存在或无权访问"})
		}
	}
	cmd := salesapp.CombinedOrderPaymentCommand{
		Orders: request.Orders, ActualCents: *request.ActualCents, ExpectedDueCents: *request.ExpectedDueCents,
		PaymentMethod: strings.TrimSpace(request.PaymentMethod), PaymentVoucherAssetID: request.PaymentVoucherAssetID,
		ShipStatusID: request.ShipStatusID, IdempotencyKey: strings.TrimSpace(request.IdempotencyKey), Actor: support.ActorOf(c),
	}
	result, err := h.sales.CombinedOrderPayment(c.Request().Context(), cmd)
	if err != nil {
		if message, ok := salesapp.CombinedPaymentValidationMessage(err); ok {
			switch message {
			case "payment_goods_amount required", "payment_voucher_asset_id required", "invalid payment_voucher_asset_id", "logistics_company_id required", "logistics_product_id required", "invalid logistics product", "invalid pay_status_id", "invalid ship_status_id":
				message = batchStatusError(err)
			case "payment_method required":
				message = "请选择收款方式"
			}
			return c.JSON(http.StatusConflict, map[string]string{"error": message})
		}
		if message, ok := salesapp.OrderEditConflictMessage(err); ok {
			return c.JSON(http.StatusConflict, map[string]string{"error": message})
		}
		// A commit/network error may be ambiguous. Returning 5xx tells the
		// client to retry the identical request and idempotency key.
		c.Logger().Errorf("combined order payment: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "收款结果未确认，请使用同一请求重试"})
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, result)
}

// Internal order readers can inspect the receipt attached to this order. Do
// not return the other order allocations to an unrelated order detail view.
func (h orderAPIHandler) orderCombinedPayments(c echo.Context) error {
	actor, authenticated, err := support.CurrentActor(c, h.authz)
	if err != nil || !authenticated || (!actor.Can("orders.read") && !actor.Can("orders.write")) || actor.AccountType == support.AccountTypeChannelCustomer || support.CustomerFulfillmentOrderScopeLimited(c) || c.Get("portal_customer_id") != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "需要内部账号的订单查看权限"})
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "订单编号无效"})
	}
	list, err := h.sales.ListOrders(c.Request().Context(), salesapp.OrderListQuery{OrderID: id, Void: "all", Limit: 1})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "查询订单失败，请稍后重试"})
	}
	if len(list.Rows) != 1 || list.Rows[0].ID != id {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "订单不存在或无权访问"})
	}
	payments, err := h.sales.OrderCombinedPayments(c.Request().Context(), id)
	if err != nil {
		c.Logger().Errorf("order combined payment records: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "读取合并收款记录失败"})
	}
	for i := range payments {
		own := make([]salesapp.CombinedPaymentAllocation, 0, 1)
		for _, allocation := range payments[i].Allocations {
			if allocation.OrderID == id {
				own = append(own, allocation)
			}
		}
		payments[i].Allocations = own
	}
	if payments == nil {
		payments = []salesapp.CombinedOrderPaymentResult{}
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, map[string]any{"payments": payments})
}
