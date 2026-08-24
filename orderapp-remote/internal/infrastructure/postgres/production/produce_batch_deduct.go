package production

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func loadBatchDeductSummary(ctx context.Context, tx pgx.Tx, schema, batchID string) ([]ProduceBatchSummaryItem, error) {
	rows, err := tx.Query(ctx,
		"SELECT i.product_id,COALESCE((SELECT name FROM "+schema+".products p WHERE p.id=i.product_id),''),i.spec_g,i.need_units,i.need_g,COALESCE(l.deducted_g,0),COALESCE(l.gap_g,0) FROM "+schema+".produce_batch_items i LEFT JOIN (SELECT product_id,spec_g,SUM(deducted_g)::bigint AS deducted_g,SUM(gap_g)::bigint AS gap_g FROM "+schema+".finished_allocation_logs WHERE batch_id=$1 GROUP BY product_id,spec_g) l ON l.product_id=i.product_id AND l.spec_g=i.spec_g WHERE i.batch_id=$1 ORDER BY i.product_id,i.spec_g", batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProduceBatchSummaryItem, 0)
	for rows.Next() {
		var s ProduceBatchSummaryItem
		if err := rows.Scan(&s.ProductID, &s.ProductName, &s.SpecG, &s.NeedUnits, &s.NeedG, &s.DeductedG, &s.GapG); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func confirmProduceBatchDeduct(ctx context.Context, pool *pgxpool.Pool, schema, batchID, operator string) ([]ProduceBatchSummaryItem, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id required")
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		operator = "order"
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	if err := tx.QueryRow(ctx, "SELECT status FROM "+schema+".produce_batches WHERE batch_id=$1 FOR UPDATE", batchID).Scan(&status); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("batch not found")
		}
		return nil, err
	}
	if status == "deducted" {
		s, err := loadBatchDeductSummary(ctx, tx, schema, batchID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return s, nil
	}

	rows, err := tx.Query(ctx,
		"SELECT i.product_id,COALESCE((SELECT name FROM "+schema+".products p WHERE p.id=i.product_id),''),i.spec_g,i.need_units,i.need_g FROM "+schema+".produce_batch_items i WHERE i.batch_id=$1 ORDER BY i.product_id,i.spec_g FOR UPDATE", batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rowItem struct {
		pid      int64
		name     string
		specG    int64
		needUnit int64
		needG    int64
	}
	items := make([]rowItem, 0)
	for rows.Next() {
		var x rowItem
		if err := rows.Scan(&x.pid, &x.name, &x.specG, &x.needUnit, &x.needG); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, it := range items {
		units, loose, err := finishedInventoryQtyIdentityTx(ctx, tx, schema, it.pid, 0, it.specG, "finished_goods")
		if err != nil {
			return nil, err
		}

		remain, deductedG, gapG, derr := invDeduct(it.specG, InvQty{Units: units, LooseG: loose}, it.needG)
		if derr != nil {
			return nil, derr
		}
		if err := upsertFinishedInventoryIdentityTx(ctx, tx, schema, it.pid, 0, 0, it.specG, "finished_goods", remain.Units, remain.LooseG); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_allocation_logs(batch_id,product_id,spec_g,need_g,deducted_g,gap_g,operator)
			VALUES($1,$2,$3,$4,$5,$6,$7)
		`, schema), batchID, it.pid, it.specG, it.needG, deductedG, gapG, operator); err != nil {
			return nil, err
		}
	}

	if _, err := tx.Exec(ctx, "UPDATE "+schema+".produce_batches SET status='deducted' WHERE batch_id=$1", batchID); err != nil {
		return nil, err
	}

	s, err := loadBatchDeductSummary(ctx, tx, schema, batchID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s, nil
}
