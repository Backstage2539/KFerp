package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
)

type EmployeeOrderCancellation interface {
	CancelEmployeeOrder(ctx context.Context, orderID, employeeID int64, allOrders bool, reason, actor string) error
}

func (h miniEmployeeOrderHandler) cancel(c echo.Context) error {
	access, ok, err := h.ensureMiniEmployeeOrderAccess(c, "orders.resolve")
	if !ok {
		return err
	}
	var req struct {
		Reason string `json:"reason"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 2048))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&req); err != nil || decoder.Decode(new(any)) != io.EOF || !domain.IsPresetCancelReason(req.Reason) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "请选择一个有效的取消原因"})
	}
	if h.cancellation == nil {
		return miniInternalError(c)
	}
	err = h.cancellation.CancelEmployeeOrder(c.Request().Context(), access.orderID, access.employee.EmployeeID, containsMiniRole(access.employee.Roles, "admin"), req.Reason, miniEmployeeActor(access.employee))
	if err != nil {
		var dbErr *pgconn.PgError
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return miniEmployeeOrderNotFound(c)
		case errors.Is(err, resolutionpg.ErrEmployeeCancelNeedsERP):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		case errors.As(err, &dbErr) && (dbErr.Code == "40001" || dbErr.Code == "40P01"):
			return c.JSON(http.StatusConflict, map[string]string{"error": "订单刚刚发生变化，请刷新后再试"})
		default:
			return miniInternalError(c)
		}
	}
	return c.JSON(http.StatusOK, map[string]int64{"order_id": access.orderID})
}
