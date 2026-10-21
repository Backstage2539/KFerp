package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
)

type miniEmployeeOrderStatusUpdater interface {
	UpdateOrderStatus(context.Context, salesapp.UpdateOrderStatusCommand) error
}

type miniEmployeeOrderStatusOptions struct {
	PayStatuses  []salesapp.Option `json:"pay_statuses"`
	ShipStatuses []salesapp.Option `json:"ship_statuses"`
}

func miniEmployeeStatusOptions(form salesapp.OrderFormData, row salesapp.OrderRow) miniEmployeeOrderStatusOptions {
	includeCurrent := func(options []salesapp.Option, id int64, name string) []salesapp.Option {
		result := append([]salesapp.Option{}, options...)
		for _, option := range result {
			if option.ID == id {
				return result
			}
		}
		if strings.TrimSpace(name) == "" {
			name = "未设置"
		}
		return append(result, salesapp.Option{ID: id, Name: name})
	}
	return miniEmployeeOrderStatusOptions{
		PayStatuses:  includeCurrent(form.PayStatuses, form.EditData.PayStatusID, row.PayStatus),
		ShipStatuses: includeCurrent(form.ShipStatuses, form.EditData.ShipStatusID, row.ShipStatus),
	}
}

type miniEmployeeOrderStatusRequest struct {
	EditRevision          string   `json:"edit_revision"`
	PayStatusID           int64    `json:"pay_status_id"`
	ShipStatusID          int64    `json:"ship_status_id"`
	PaymentMethod         string   `json:"payment_method"`
	PrepaymentAmount      *float64 `json:"prepayment_amount"`
	PaymentGoodsAmount    *float64 `json:"payment_goods_amount"`
	PaymentShippingAmount *float64 `json:"payment_shipping_amount"`
	PaymentVoucherAssetID *int64   `json:"payment_voucher_asset_id"`
}

func miniEmployeeStatusBlockReason(access miniEmployeeOrderAccess, ed *salesapp.OrderEditData) string {
	if !containsMiniRole(access.employee.Permissions, "orders.write") {
		return "当前账号没有修改订单状态权限"
	}
	if access.row.IsVoid || ed != nil && ed.IsVoid {
		return "订单已作废，不能修改状态"
	}
	if access.row.ConfirmationRequired && access.row.ConfirmationStatus != "accepted" {
		return "订单尚未确认，请先确认订单后再修改付款或发货状态"
	}
	return ""
}

func (h miniEmployeeOrderHandler) updateStatus(c echo.Context) error {
	access, ok, err := h.ensureMiniEmployeeOrderAccess(c, "orders.write")
	if !ok {
		return err
	}
	updater, ok := h.sales.(miniEmployeeOrderStatusUpdater)
	if !ok {
		return miniInternalError(c)
	}
	var req miniEmployeeOrderStatusRequest
	decoder := json.NewDecoder(io.LimitReader(c.Request().Body, 32*1024))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式不正确，请仅提交付款和发货状态"})
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
	}
	form, err := h.sales.OrderForm(c.Request().Context(), access.orderID)
	if err != nil {
		return miniInternalError(c)
	}
	if form.EditData == nil || form.EditData.ID != access.orderID || form.EditData.CustomerID != access.row.CustomerID {
		return miniEmployeeOrderNotFound(c)
	}
	if reason := miniEmployeeStatusBlockReason(access, form.EditData); reason != "" {
		return c.JSON(http.StatusConflict, map[string]string{"error": reason})
	}
	if access.row.ConfirmationRequired && req.ShipStatusID != form.EditData.ShipStatusID {
		return c.JSON(http.StatusConflict, map[string]string{"error": "履约订单的发货状态由实际履约进度更新，请通过发货操作处理"})
	}
	if strings.TrimSpace(req.EditRevision) == "" || strings.TrimSpace(req.EditRevision) != strings.TrimSpace(form.EditData.EditRevision) {
		return c.JSON(http.StatusConflict, map[string]string{"error": "订单已被其他操作修改，请刷新订单后再保存状态"})
	}
	err = updater.UpdateOrderStatus(c.Request().Context(), salesapp.UpdateOrderStatusCommand{OrderID: access.orderID, ExpectedEditRevision: strings.TrimSpace(req.EditRevision), PayStatusID: req.PayStatusID, ShipStatusID: req.ShipStatusID, PaymentMethod: req.PaymentMethod, PrepaymentAmount: req.PrepaymentAmount, PaymentGoodsAmount: req.PaymentGoodsAmount, PaymentShippingAmount: req.PaymentShippingAmount, PaymentVoucherAssetID: req.PaymentVoucherAssetID, Actor: miniEmployeeActor(access.employee)})
	if err != nil {
		var constraint *pgconn.PgError
		if errors.As(err, &constraint) && constraint.Code == "23514" && strings.HasPrefix(constraint.Message, "待补收件信息") {
			return c.JSON(http.StatusConflict, map[string]string{"error": constraint.Message})
		}
		if message, conflict := salesapp.OrderEditConflictMessage(err); conflict {
			return c.JSON(http.StatusConflict, map[string]string{"error": message})
		}
		if message, known := miniOrderKnownBusinessErrorText(err); known {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
		}
		if strings.HasPrefix(err.Error(), "实收金额") {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return miniInternalError(c)
	}
	return c.JSON(http.StatusOK, map[string]any{"order_id": access.orderID, "order_no": access.row.OrderNo})
}
