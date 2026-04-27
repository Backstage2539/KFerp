package production

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AllocationLogRow struct {
	BatchID   string
	ProductID int64
	SpecG     int64
	NeedG     int64
	DeductedG int64
	GapG      int64
	Operator  string
	CreatedAt time.Time
}

func newBatchID() string {
	// Keep it human-friendly.
	// Example: A20260216-144305-7f
	b := make([]byte, 1)
	_, _ = rand.Read(b)
	return fmt.Sprintf("A%s-%s", time.Now().Format("20060102-150405"), hex.EncodeToString(b))
}

// allocateUnproducedBySummary deducts inventory based on summary needs (product+spec) and writes a batch log.
// It is intentionally summary-level (not per-order) per Van's instruction.
func allocateUnproducedBySummary(ctx context.Context, pool *pgxpool.Pool, schema, from, to string, customerID int64, operator string) (batchID string, logs []AllocationLogRow, hasLowWarning bool, err error) {
	needsAll, err := fetchUnproducedNeeds(ctx, pool, schema, from, to, customerID)
	if err != nil {
		return "", nil, false, err
	}
	needs := make([]UnprodNeedRow, 0, len(needsAll))
	for _, n := range needsAll {
		if n.GapG > 0 {
			needs = append(needs, n)
		}
	}
	return allocateUnproducedRows(ctx, pool, schema, needs, operator)
}

func allocateUnproducedRows(ctx context.Context, pool *pgxpool.Pool, schema string, needs []UnprodNeedRow, operator string) (batchID string, logs []AllocationLogRow, hasLowWarning bool, err error) {
	batchID = newBatchID()
	if len(needs) == 0 {
		return batchID, nil, false, nil
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", nil, false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	for _, n := range needs {
		// lock inventory row if exists; otherwise treat as 0
		var units, loose int64
		row := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units, onhand_loose_g
			FROM %s.finished_inventory
			WHERE product_id=$1 AND spec_g=$2 AND warehouse='finished_goods'
			FOR UPDATE
		`, schema), n.ProductID, n.SpecG)
		scanErr := row.Scan(&units, &loose)
		if scanErr != nil {
			if scanErr == pgx.ErrNoRows {
				units, loose = 0, 0
			} else {
				err = scanErr
				return "", nil, false, err
			}
		}

		remain, deductedG, gapG, derr := invDeduct(n.SpecG, InvQty{Units: units, LooseG: loose}, n.NeedG)
		if derr != nil {
			err = derr
			return "", nil, false, err
		}

		// upsert inventory with remain (ensure row exists)
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
			VALUES($1,$2,'finished_goods',$3,$4,now())
			ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
			SET onhand_units=excluded.onhand_units, onhand_loose_g=excluded.onhand_loose_g, updated_at=now()
		`, schema), n.ProductID, n.SpecG, remain.Units, remain.LooseG)
		if err != nil {
			return "", nil, false, err
		}

		// write log
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_allocation_logs(batch_id,product_id,spec_g,need_g,deducted_g,gap_g,operator)
			VALUES($1,$2,$3,$4,$5,$6,$7)
		`, schema), batchID, n.ProductID, n.SpecG, n.NeedG, deductedG, gapG, operator)
		if err != nil {
			return "", nil, false, err
		}

		// Unified policy: below warning line is allowed but should return warning.
		// Current warning line uses 1-pack equivalent grams for this SKU/spec.
		if deductedG > 0 {
			if remainTotal, terr := invTotalG(n.SpecG, remain); terr == nil && remainTotal < n.SpecG {
				hasLowWarning = true
			}
		}

		logs = append(logs, AllocationLogRow{
			BatchID: batchID, ProductID: n.ProductID, SpecG: n.SpecG,
			NeedG: n.NeedG, DeductedG: deductedG, GapG: gapG,
			Operator: operator,
		})
	}

	if err = tx.Commit(ctx); err != nil {
		return "", nil, false, err
	}
	return batchID, logs, hasLowWarning, nil
}
