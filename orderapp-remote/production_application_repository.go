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
