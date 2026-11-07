package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	infra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) confirmCompanyProduction(ctx context.Context, tx pgx.Tx, orderID, requestID, planID int64, note, actor string) error {
	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT p.status FROM %[1]s.production_plans p WHERE p.id=$1 AND EXISTS(SELECT 1 FROM %[1]s.production_plan_items i JOIN %[1]s.orders o ON o.id=$2 WHERE i.production_plan_id=p.id AND o.order_no=ANY(string_to_array(replace(i.order_nos,' ',''),',')))FOR UPDATE`, r.schema), planID, orderID).Scan(&status); err != nil {
		return err
	}
	if one(status, "completed", "cancelled") {
		return fmt.Errorf("生产计划状态已变化，请重新预览")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`SELECT %s.order_resolution_require_company_production($1)`, r.schema), planID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_production_dispositions(request_id,order_id,plan_id,choice,actor,note)VALUES($1,$2,$3,'continue_company',$4,$5)`, r.schema), requestID, orderID, planID, actor, note); err != nil {
		return err
	}
	return infra.AuditInsertTx(ctx, tx, r.schema, actor, "production_plan", &planID, "resolution_continue_company", nil, nil, nil, infra.AuditMeta{"order_id": orderID, "resolution_id": requestID, "note": note, "preserve_shared_orders": true})
}
