package orderresolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
)

var (
	ErrEmployeeCancelReason   = errors.New("请选择取消原因")
	ErrEmployeeCancelNeedsERP = errors.New("请到后台「订单处理」核对")
	errPendingResolution      = errors.New("已有订单处理单待完成，请先处理或撤回")
	errClosedPostingPeriod    = errors.New("所选账期已结账，请选择开放账期或申请历史更正")
	errPartialBillPayment     = errors.New("关联账单存在部分收款，请先核对或撤销收款后再处理订单")
)

// CancelEmployeeOrder only submits whole-order cancellations that need no other business decisions.
func (r Repository) CancelEmployeeOrder(ctx context.Context, id, employeeID int64, allOrders bool, reason, actor string) error {
	if !domain.IsPresetCancelReason(reason) {
		return ErrEmployeeCancelReason
	}
	if employeeID <= 0 || strings.TrimSpace(actor) == "" {
		return pgx.ErrNoRows
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = r.lock(ctx, tx, id); err != nil {
		return err
	}
	var status, shipStatus string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT o.resolution_status,COALESCE(s.name,'') FROM %[1]s.orders o
 LEFT JOIN %[1]s.ship_statuses s ON s.id=o.ship_status_id
 WHERE o.id=$1 AND ($2 OR (o.responsible_party_type='employee' AND o.responsible_party_id=$3))`, r.schema), id, allOrders, employeeID).Scan(&status, &shipStatus)
	if err != nil {
		return err
	}
	if status == "cancelled" {
		return nil
	}
	if strings.Contains(shipStatus, "部分发货") || strings.Contains(shipStatus, "已发货") || one(shipStatus, "已出库", "已签收", "已收货", "已完成") {
		return fmt.Errorf("订单已有发货记录，%w", ErrEmployeeCancelNeedsERP)
	}
	current, err := r.load(ctx, tx, id)
	if errors.Is(err, errPartialBillPayment) {
		return fmt.Errorf("%s，%w", err, ErrEmployeeCancelNeedsERP)
	}
	if err != nil {
		return err
	}
	cmd := domain.Command{Action: "cancel", Reason: reason, Scope: "all", Timing: "settle_first", ExternalStatus: "none", PostingMode: "open_period",
		PostingDate: time.Now().In(time.FixedZone("Asia/Shanghai", 8*3600)).Format(time.DateOnly), Version: current.Version, IdempotencyKey: "mini-cancel-" + current.Version}
	plan, err := domain.Preview(current, cmd)
	if err != nil {
		return fmt.Errorf("%s，%w", err, ErrEmployeeCancelNeedsERP)
	}
	if len(plan.Tasks) > 0 || current.FulfillmentMode != "" {
		return fmt.Errorf("此单需要处理关联单据或审批，%w", ErrEmployeeCancelNeedsERP)
	}
	body, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	_, err = r.submitPlan(ctx, tx, id, current, cmd, body, plan, actor)
	if errors.Is(err, errPendingResolution) || errors.Is(err, errClosedPostingPeriod) {
		return fmt.Errorf("%s，%w", err, ErrEmployeeCancelNeedsERP)
	}
	return err
}
