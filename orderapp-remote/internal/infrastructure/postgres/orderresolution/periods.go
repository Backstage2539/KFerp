package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
	financepg "orderapp/internal/infrastructure/postgres/finance"
)

func (r Repository) finalizeHistoricalPeriod(ctx context.Context, tx pgx.Tx, requestID int64, cmd domain.Command, actor string) error {
	if cmd.PostingMode != "historical_correction" {
		return nil
	}
	var before, beforeReport []byte
	var matched bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT f.snapshot_json,to_jsonb(f),a.original_snapshot=f.snapshot_json FROM %[1]s.finance_monthly_reports f JOIN %[1]s.order_resolution_period_approvals a ON a.month=f.month AND a.request_id=$1 WHERE f.month=left($2,7) AND f.status IN('closed','adjusted')FOR UPDATE OF f`, r.schema), requestID, cmd.PostingDate).Scan(&before, &beforeReport, &matched)
	if err != nil || !matched {
		return fmt.Errorf("%w：历史月结快照已变化或尚未批准，请更新方案并重新审批", ErrStale)
	}
	month := cmd.PostingDate[:7]
	if err = financepg.NewRepository(r.pool, r.schema).RebuildClosedMonthTx(ctx, tx, month, actor); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_resolution_period_revisions(request_id,month,before_json,after_json,before_report_json,after_report_json,actor)SELECT $1,f.month,$2,f.snapshot_json,$3,to_jsonb(f),$4 FROM %[1]s.finance_monthly_reports f WHERE f.month=$5`, r.schema), requestID, before, beforeReport, actor, month)
	if err != nil {
		return err
	}
	return infra.AuditInsertTx(ctx, tx, r.schema, actor, "finance_monthly_report", nil, "order_resolution_revision", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "month": month})
}
