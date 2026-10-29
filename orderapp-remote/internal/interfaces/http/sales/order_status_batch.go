package sales

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/interfaces/http/support"
)

// Batch status operations are an internal staff operation. Customer portal and
// scoped fulfillment accounts must not gain access to arbitrary order IDs.
func (h orderAPIHandler) batchStatusAuthorized(c echo.Context) bool {
	actor, authenticated, err := support.CurrentActor(c, h.authz)
	return err == nil && authenticated && actor.Can("orders.write") && actor.AccountType != support.AccountTypeChannelCustomer && !support.CustomerFulfillmentOrderScopeLimited(c) && c.Get("portal_customer_id") == nil
}

type batchStatusPreviewRow struct {
	OrderID                int64  `json:"order_id"`
	OrderNo                string `json:"order_no,omitempty"`
	EditRevision           string `json:"edit_revision,omitempty"`
	PayStatusID            int64  `json:"pay_status_id"`
	ShipStatusID           int64  `json:"ship_status_id"`
	PaymentMethod          string `json:"payment_method"`
	PrepaymentAmount       string `json:"prepayment_amount"`
	PaymentGoodsAmount     string `json:"payment_goods_amount"`
	PaymentShippingAmount  string `json:"payment_shipping_amount"`
	PaymentVoucherAssetID  int64  `json:"payment_voucher_asset_id"`
	PaymentVoucherFilename string `json:"payment_voucher_filename"`
	DeliveryMode           string `json:"delivery_mode"`
	SettlementReason       string `json:"settlement_reason"`
	ShipMethod             string `json:"ship_method"`
	LogisticsCompanyID     int64  `json:"logistics_company_id"`
	LogisticsProductID     int64  `json:"logistics_product_id"`
	ConfirmationRequired   bool   `json:"confirmation_required"`
	ConfirmationStatus     string `json:"confirmation_status"`
	Error                  string `json:"error,omitempty"`
}

type batchStatusItem struct {
	OrderID               int64    `json:"order_id"`
	EditRevision          string   `json:"edit_revision"`
	PayStatusID           *int64   `json:"pay_status_id"`
	ShipStatusID          *int64   `json:"ship_status_id"`
	PaymentMethod         *string  `json:"payment_method"`
	PrepaymentAmount      *float64 `json:"prepayment_amount"`
	PaymentGoodsAmount    *float64 `json:"payment_goods_amount"`
	PaymentShippingAmount *float64 `json:"payment_shipping_amount"`
	PaymentVoucherAssetID *int64   `json:"payment_voucher_asset_id"`
}

type batchStatusResult struct {
	OrderID int64  `json:"order_id"`
	OrderNo string `json:"order_no,omitempty"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

func decodeBatchStatus(c echo.Context, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 256<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return echo.NewHTTPError(http.StatusBadRequest, "请求格式不正确")
	}
	return nil
}

func (h orderAPIHandler) batchStatusOrder(c echo.Context, id int64) (batchStatusPreviewRow, *salesapp.OrderEditData) {
	out := batchStatusPreviewRow{OrderID: id}
	if id <= 0 {
		out.Error = "订单编号无效"
		return out, nil
	}
	// Re-check visibility against the authoritative list rather than trusting IDs
	// sent by the browser. Also obtains the fulfillment confirmation flags.
	list, err := h.sales.ListOrders(c.Request().Context(), salesapp.OrderListQuery{OrderID: id, Void: "all", Limit: 1})
	if err != nil {
		out.Error = "查询订单失败，请稍后重试"
		return out, nil
	}
	if len(list.Rows) != 1 || list.Rows[0].ID != id {
		out.Error = "订单不存在或无权访问"
		return out, nil
	}
	row := list.Rows[0]
	ed, err := h.sales.OrderStatusEditData(c.Request().Context(), id)
	if err != nil {
		out.Error = "查询订单失败，请稍后重试"
		return out, nil
	}
	if ed == nil || ed.ID != id || ed.CustomerID != row.CustomerID {
		out.Error = "订单不存在或无权访问"
		return out, nil
	}
	out.OrderNo, out.EditRevision = ed.OrderNo, ed.EditRevision
	out.PayStatusID, out.ShipStatusID, out.PaymentMethod = ed.PayStatusID, ed.ShipStatusID, ed.PaymentMethod
	out.PrepaymentAmount, out.PaymentGoodsAmount, out.PaymentShippingAmount = ed.PrepaymentAmount, ed.PaymentGoodsAmount, ed.PaymentShippingAmount
	out.PaymentVoucherAssetID = ed.PaymentVoucherAssetID
	if ed.PaymentVoucher != nil {
		out.PaymentVoucherFilename = ed.PaymentVoucher.Filename
	}
	out.DeliveryMode, out.SettlementReason, out.ShipMethod = ed.DeliveryMode, ed.SettlementReason, ed.ShipMethod
	out.LogisticsCompanyID, out.LogisticsProductID = ed.LogisticsCompanyID, ed.LogisticsProductID
	out.ConfirmationRequired, out.ConfirmationStatus = row.ConfirmationRequired, row.ConfirmationStatus
	if row.IsVoid || ed.IsVoid {
		out.Error = "订单已作废，不能修改状态"
	}
	if row.ConfirmationRequired && row.ConfirmationStatus != "accepted" {
		out.Error = "订单尚未确认，请先确认订单后再修改状态"
	}
	return out, ed
}

func (h orderAPIHandler) batchStatusPreview(c echo.Context) error {
	if !h.batchStatusAuthorized(c) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "需要内部账号的订单编辑权限"})
	}
	var req struct {
		OrderIDs []int64 `json:"order_ids"`
	}
	if decodeBatchStatus(c, &req) != nil || len(req.OrderIDs) == 0 || len(req.OrderIDs) > 200 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "请选择 1 至 200 张订单"})
	}
	rows := make([]batchStatusPreviewRow, 0, len(req.OrderIDs))
	seen := map[int64]bool{}
	for _, id := range req.OrderIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		row, _ := h.batchStatusOrder(c, id)
		rows = append(rows, row)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, map[string]any{"orders": rows})
}

func batchStatusError(err error) string {
	if message, ok := salesapp.OrderEditConflictMessage(err); ok {
		return message
	}
	switch err.Error() {
	case "payment_goods_amount required":
		return "请填写实收货款"
	case "payment_voucher_asset_id required":
		return "请上传收款凭证"
	case "invalid payment_voucher_asset_id":
		return "收款凭证无效，请重新上传"
	case "logistics_company_id required":
		return "请先填写物流公司"
	case "logistics_product_id required":
		return "请先填写物流产品"
	case "invalid logistics product":
		return "物流公司或物流产品无效，请先更新"
	case "invalid pay_status_id", "invalid ship_status_id":
		return "状态选项无效，请刷新后重试"
	}
	if strings.HasPrefix(err.Error(), "实收金额") || strings.Contains(err.Error(), "预付款") || strings.Contains(err.Error(), "付款方式") || strings.Contains(err.Error(), "收款方式") {
		return err.Error()
	}
	return "状态更新失败，请稍后重试"
}

func (h orderAPIHandler) batchStatusSave(c echo.Context) error {
	if !h.batchStatusAuthorized(c) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "需要内部账号的订单编辑权限"})
	}
	var req struct {
		Orders []batchStatusItem `json:"orders"`
	}
	if decodeBatchStatus(c, &req) != nil || len(req.Orders) == 0 || len(req.Orders) > 200 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "请选择 1 至 200 张订单"})
	}
	results := make([]batchStatusResult, 0, len(req.Orders))
	seen := map[int64]bool{}
	for _, item := range req.Orders {
		result := batchStatusResult{OrderID: item.OrderID, Status: "failed"}
		if seen[item.OrderID] {
			result.Error = "订单重复，请刷新后重试"
			results = append(results, result)
			continue
		}
		seen[item.OrderID] = true
		row, ed := h.batchStatusOrder(c, item.OrderID)
		result.OrderNo = row.OrderNo
		if row.Error != "" {
			result.Error = row.Error
			results = append(results, result)
			continue
		}
		if strings.TrimSpace(item.EditRevision) == "" || item.EditRevision != row.EditRevision {
			result.Error = "订单已被其他操作修改，请刷新订单后再保存状态"
			results = append(results, result)
			continue
		}
		if item.PayStatusID == nil && item.ShipStatusID == nil {
			result.Error = "请选择要修改的付款或发货状态"
			results = append(results, result)
			continue
		}
		if item.PayStatusID == nil && (item.PaymentMethod != nil || item.PrepaymentAmount != nil || item.PaymentGoodsAmount != nil || item.PaymentShippingAmount != nil || item.PaymentVoucherAssetID != nil) {
			result.Error = "未选择付款状态，不能修改收款资料"
			results = append(results, result)
			continue
		}
		if item.ShipStatusID != nil && *item.ShipStatusID != ed.ShipStatusID && (row.ConfirmationRequired || ed.DeliveryMode == "customer_stock") {
			result.Error = "履约订单的发货状态由实际履约进度更新，请通过发货操作处理"
			results = append(results, result)
			continue
		}
		pay, ship, method := ed.PayStatusID, ed.ShipStatusID, ed.PaymentMethod
		if item.PayStatusID != nil {
			pay = *item.PayStatusID
		}
		if item.ShipStatusID != nil {
			ship = *item.ShipStatusID
		}
		if item.PaymentMethod != nil {
			method = *item.PaymentMethod
		}
		changed := false
		cmd := salesapp.UpdateOrderStatusCommand{OrderID: item.OrderID, ExpectedEditRevision: item.EditRevision, PayStatusID: pay, ShipStatusID: ship, PaymentMethod: method, PrepaymentAmount: item.PrepaymentAmount, PaymentGoodsAmount: item.PaymentGoodsAmount, PaymentShippingAmount: item.PaymentShippingAmount, PaymentVoucherAssetID: item.PaymentVoucherAssetID, Actor: support.ActorOf(c), Source: "erp-batch-order-status", PaymentUnchanged: item.PayStatusID == nil, Changed: &changed}
		if err := h.sales.UpdateOrderStatus(c.Request().Context(), cmd); err != nil {
			result.Error = batchStatusError(err)
		} else if !changed {
			result.Status = "unchanged"
		} else {
			result.Status = "updated"
		}
		results = append(results, result)
	}
	return c.JSON(http.StatusOK, map[string]any{"results": results})
}
