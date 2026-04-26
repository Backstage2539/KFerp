package main

import (
	"context"
	"fmt"
	"strings"

	productionapp "orderapp/internal/application/production"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresProductionRepository struct {
	pool   *pgxpool.Pool
	schema string
}

func (r postgresProductionRepository) CreateBatch(ctx context.Context, cmd productionapp.CreateBatchCommand) (productionapp.CreateBatchResult, error) {
	res, err := createProduceBatchFromOrders(ctx, r.pool, r.schema, cmd.OrderIDs, cmd.Operator, cmd.IdempotencyKey, cmd.RequestUnitsByItemID)
	if err != nil {
		return productionapp.CreateBatchResult{}, err
	}
	auditInsert(ctx, r.pool, r.schema, cmd.Operator, "produce_batch", nil, "create", strPtrStr("batch_id"), nil, strPtrStr(res.BatchID), AuditMeta{"batch_id": res.BatchID, "order_count": res.OrderCount})
	return productionapp.CreateBatchResult{
		BatchID:    res.BatchID,
		OrderCount: res.OrderCount,
		Summary:    productionSummaryToApp(res.Summary),
	}, nil
}

func (r postgresProductionRepository) ListBatches(ctx context.Context, cmd productionapp.ListBatchesCommand) ([]productionapp.BatchListItem, error) {
	args := []any{}
	where := " WHERE 1=1"
	if cmd.Status != "" {
		args = append(args, cmd.Status)
		where += fmt.Sprintf(" AND b.status=$%d", len(args))
	}
	if cmd.Operator != "" {
		args = append(args, cmd.Operator)
		where += fmt.Sprintf(" AND b.operator=$%d", len(args))
	}
	if cmd.From != "" {
		args = append(args, cmd.From)
		where += fmt.Sprintf(" AND b.created_at >= $%d::date", len(args))
	}
	if cmd.To != "" {
		args = append(args, cmd.To)
		where += fmt.Sprintf(" AND b.created_at < ($%d::date + INTERVAL '1 day')", len(args))
	}
	args = append(args, cmd.Limit)
	limitArg := len(args)

	q := fmt.Sprintf(`
		SELECT b.batch_id, b.status, b.operator, to_char(b.created_at,'YYYY-MM-DD HH24:MI:SS'),
		       COALESCE((SELECT COUNT(DISTINCT x.order_id) FROM %s.produce_batch_order_items x WHERE x.batch_id=b.batch_id),0),
		       CASE
		         WHEN l.cnt IS NULL THEN 'none'
		         WHEN l.total_gap_g = 0 THEN 'done'
		         ELSE 'partial'
		       END AS deduct_status,
		       COALESCE(to_char(l.last_deducted_at,'YYYY-MM-DD HH24:MI:SS'),'') AS deducted_at,
		       COALESCE(i.total_need_g,0) AS need_g,
		       COALESCE(l.total_deducted_g,0) AS deducted_g,
		       GREATEST(0, COALESCE(i.total_need_g,0) - COALESCE(l.total_deducted_g,0)) AS gap_g
		FROM %s.produce_batches b
		LEFT JOIN (
		  SELECT batch_id, SUM(need_g)::bigint AS total_need_g
		  FROM %s.produce_batch_items
		  GROUP BY batch_id
		) i ON i.batch_id=b.batch_id
		LEFT JOIN (
		  SELECT batch_id, COUNT(*) AS cnt, SUM(gap_g)::bigint AS total_gap_g, SUM(deducted_g)::bigint AS total_deducted_g, MAX(created_at) AS last_deducted_at
		  FROM %s.finished_allocation_logs
		  GROUP BY batch_id
		) l ON l.batch_id=b.batch_id
		%s
		ORDER BY b.created_at DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, where, limitArg)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.BatchListItem, 0)
	for rows.Next() {
		var item productionapp.BatchListItem
		if err := rows.Scan(&item.BatchID, &item.Status, &item.Operator, &item.CreatedAt, &item.OrderCount, &item.DeductStatus, &item.DeductedAt, &item.NeedG, &item.DeductedG, &item.GapG); err != nil {
			return nil, err
		}
		item.CreatedBy = item.Operator
		item.CreatedTime = item.CreatedAt
		item.StatusChangedAt = item.CreatedAt
		if strings.TrimSpace(item.DeductedAt) != "" {
			item.StatusChangedAt = item.DeductedAt
		}
		item.StatusText = item.Status
		item.CreateTime = item.CreatedAt
		item.DeductTime = item.DeductedAt
		item.DeductState = item.DeductStatus
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r postgresProductionRepository) Detail(ctx context.Context, batchID string) (productionapp.BatchDetail, error) {
	var d productionapp.BatchDetail
	err := r.pool.QueryRow(ctx,
		"SELECT batch_id,status,operator,to_char(created_at,'YYYY-MM-DD HH24:MI:SS') FROM "+r.schema+".produce_batches WHERE batch_id=$1",
		batchID,
	).Scan(&d.BatchID, &d.Status, &d.Operator, &d.CreatedAt)
	if err != nil {
		return productionapp.BatchDetail{}, fmt.Errorf("batch not found")
	}

	orows, err := r.pool.Query(ctx, "SELECT DISTINCT order_id FROM "+r.schema+".produce_batch_order_items WHERE batch_id=$1 ORDER BY order_id", batchID)
	if err != nil {
		return productionapp.BatchDetail{}, err
	}
	defer orows.Close()
	d.Orders = make([]int64, 0)
	for orows.Next() {
		var orderID int64
		if err := orows.Scan(&orderID); err != nil {
			return productionapp.BatchDetail{}, err
		}
		d.Orders = append(d.Orders, orderID)
	}
	if err := orows.Err(); err != nil {
		return productionapp.BatchDetail{}, err
	}

	srows, err := r.pool.Query(ctx,
		"SELECT i.product_id,COALESCE((SELECT name FROM "+r.schema+".products p WHERE p.id=i.product_id),''),i.spec_g,i.need_units,i.need_g,COALESCE(l.deducted_g,0),COALESCE(l.gap_g,0) FROM "+r.schema+".produce_batch_items i LEFT JOIN (SELECT product_id,spec_g,SUM(deducted_g)::bigint AS deducted_g,SUM(gap_g)::bigint AS gap_g FROM "+r.schema+".finished_allocation_logs WHERE batch_id=$1 GROUP BY product_id,spec_g) l ON l.product_id=i.product_id AND l.spec_g=i.spec_g WHERE i.batch_id=$1 ORDER BY i.product_id,i.spec_g", batchID)
	if err != nil {
		return productionapp.BatchDetail{}, err
	}
	defer srows.Close()
	d.Summary = make([]productionapp.SummaryItem, 0)
	for srows.Next() {
		var item productionapp.SummaryItem
		if err := srows.Scan(&item.ProductID, &item.ProductName, &item.SpecG, &item.NeedUnits, &item.NeedG, &item.DeductedG, &item.GapG); err != nil {
			return productionapp.BatchDetail{}, err
		}
		d.Summary = append(d.Summary, item)
	}
	if err := srows.Err(); err != nil {
		return productionapp.BatchDetail{}, err
	}

	d.CreatedBy = d.Operator
	d.CreatedTime = d.CreatedAt
	d.StatusSource = "produce_batches.status"
	return d, nil
}

func (r postgresProductionRepository) PreviewDeduct(ctx context.Context, batchID string) (productionapp.DeductPreview, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return productionapp.DeductPreview{}, fmt.Errorf("batch_id required")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.DeductPreview{}, err
	}
	defer tx.Rollback(ctx)

	var exists int
	if err := tx.QueryRow(ctx, "SELECT 1 FROM "+r.schema+".produce_batches WHERE batch_id=$1 FOR UPDATE", batchID).Scan(&exists); err != nil {
		return productionapp.DeductPreview{}, fmt.Errorf("batch not found")
	}

	rows, err := tx.Query(ctx,
		"SELECT i.product_id,COALESCE((SELECT name FROM "+r.schema+".products p WHERE p.id=i.product_id),''),i.spec_g,i.need_units,i.need_g,COALESCE(fi.onhand_units,0),COALESCE(fi.onhand_loose_g,0) FROM "+r.schema+".produce_batch_items i LEFT JOIN LATERAL (SELECT onhand_units,onhand_loose_g FROM "+r.schema+".finished_inventory f WHERE f.product_id=i.product_id AND f.spec_g=i.spec_g FOR UPDATE) fi ON true WHERE i.batch_id=$1 ORDER BY i.product_id,i.spec_g FOR UPDATE OF i", batchID)
	if err != nil {
		return productionapp.DeductPreview{}, err
	}
	defer rows.Close()

	out := productionapp.DeductPreview{BatchID: batchID, Summary: make([]productionapp.DeductPreviewItem, 0)}
	for rows.Next() {
		var s productionapp.DeductPreviewItem
		if err := rows.Scan(&s.ProductID, &s.ProductName, &s.SpecG, &s.NeedUnits, &s.NeedG, &s.InvUnits, &s.InvLooseG); err != nil {
			return productionapp.DeductPreview{}, err
		}
		totalG, err := invTotalG(s.SpecG, InvQty{Units: s.InvUnits, LooseG: s.InvLooseG})
		if err != nil {
			return productionapp.DeductPreview{}, err
		}
		s.InvTotalG = totalG
		_, deductedG, gapG, err := invDeduct(s.SpecG, InvQty{Units: s.InvUnits, LooseG: s.InvLooseG}, s.NeedG)
		if err != nil {
			return productionapp.DeductPreview{}, err
		}
		s.DeductedG = deductedG
		s.GapG = gapG
		if s.DeductedG > 0 && (s.InvTotalG-s.DeductedG) < s.SpecG {
			s.WarningLowStock = true
		}
		out.Summary = append(out.Summary, s)
	}
	if err := rows.Err(); err != nil {
		return productionapp.DeductPreview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.DeductPreview{}, err
	}
	return out, nil
}

func (r postgresProductionRepository) ConfirmDeduct(ctx context.Context, batchID, operator string) (productionapp.DeductConfirmResult, error) {
	summary, err := confirmProduceBatchDeduct(ctx, r.pool, r.schema, batchID, operator)
	if err != nil {
		return productionapp.DeductConfirmResult{}, err
	}
	auditInsert(ctx, r.pool, r.schema, operator, "produce_batch", nil, "update", strPtrStr("deduct_status"), nil, strPtrStr("deducted"), AuditMeta{"batch_id": strings.TrimSpace(batchID), "summary_count": len(summary)})
	return productionapp.DeductConfirmResult{BatchID: strings.TrimSpace(batchID), Status: "deducted", Summary: productionSummaryToApp(summary)}, nil
}

func (r postgresProductionRepository) ListRunning(ctx context.Context) ([]productionapp.RunningItem, error) {
	rows, err := listRunningItems(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	return productionRunningToApp(rows), nil
}

func (r postgresProductionRepository) Start(ctx context.Context, cmd productionapp.StartCommand) (productionapp.StartResult, error) {
	batchID, err := startProductionWithInputs(ctx, r.pool, r.schema, cmd.From, cmd.To, cmd.CustomerID, cmd.Selected, cmd.InputByKey, cmd.Operator)
	if err != nil {
		return productionapp.StartResult{}, err
	}
	return productionapp.StartResult{BatchID: batchID}, nil
}

func (r postgresProductionRepository) Finish(ctx context.Context, cmd productionapp.FinishCommand) error {
	return finishRunningItem(ctx, r.pool, r.schema, cmd.ID, cmd.FinishedUnits, cmd.FinishedLooseG, cmd.HasFinishedInput, cmd.Operator)
}

func (r postgresProductionRepository) Cancel(ctx context.Context, cmd productionapp.CancelCommand) error {
	return cancelRunningItem(ctx, r.pool, r.schema, cmd.ID, cmd.Operator)
}

func productionSummaryToApp(items []ProduceBatchSummaryItem) []productionapp.SummaryItem {
	out := make([]productionapp.SummaryItem, 0, len(items))
	for _, it := range items {
		out = append(out, productionapp.SummaryItem{
			ProductID:   it.ProductID,
			ProductName: it.ProductName,
			SpecG:       it.SpecG,
			NeedUnits:   it.NeedUnits,
			NeedG:       it.NeedG,
			DeductedG:   it.DeductedG,
			GapG:        it.GapG,
		})
	}
	return out
}
