package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	stockItemTypeMaterial        = "material"
	stockItemTypeFinishedProduct = "finished_product"
	stockSourceProductionRun     = "production_run"
)

type stockLedgerQty struct {
	BeforeG     int64
	ChangeG     int64
	AfterG      int64
	BeforeUnits int64
	ChangeUnits int64
	AfterUnits  int64
}

func ensureStockLedgerTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS stock_batches_source_uq
	ON %s.stock_batches(source_doc_type, source_doc_id, item_type, item_id, spec_g)
	WHERE source_doc_type <> '';
CREATE INDEX IF NOT EXISTS stock_batches_item_idx
	ON %s.stock_batches(item_type, item_id, spec_g, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.stock_ledger_entries (
	id BIGSERIAL PRIMARY KEY,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_code TEXT NOT NULL DEFAULT '',
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_before_g BIGINT NOT NULL DEFAULT 0,
	qty_change_g BIGINT NOT NULL DEFAULT 0,
	qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,
	qty_change_units BIGINT NOT NULL DEFAULT 0,
	qty_after_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS stock_ledger_source_idx
	ON %s.stock_ledger_entries(source_doc_type, source_doc_id, id);
CREATE INDEX IF NOT EXISTS stock_ledger_item_idx
	ON %s.stock_ledger_entries(item_type, item_id, spec_g, created_at DESC);
`, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS remaining_g BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS remaining_units BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0`, schema))
	return nil
}

func EnsureStockLedgerTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return ensureStockLedgerTables(ctx, pool, schema)
}

func finishedProductionBatchCode(runningItemID int64) string {
	return fmt.Sprintf("FP-%010d", runningItemID)
}

func finishedInventoryLedgerQty(specG int64, before, add, after InvQty) (stockLedgerQty, error) {
	beforeG, err := invTotalG(specG, before)
	if err != nil {
		return stockLedgerQty{}, err
	}
	changeG, err := invTotalG(specG, add)
	if err != nil {
		return stockLedgerQty{}, err
	}
	afterG, err := invTotalG(specG, after)
	if err != nil {
		return stockLedgerQty{}, err
	}
	return stockLedgerQty{
		BeforeG:     beforeG,
		ChangeG:     changeG,
		AfterG:      afterG,
		BeforeUnits: before.Units,
		ChangeUnits: add.Units,
		AfterUnits:  after.Units,
	}, nil
}

func createFinishedStockBatchTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, add InvQty, finishedTotalG int64, operator string) (string, error) {
	batchCode := finishedProductionBatchCode(r.ID)
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,
			source_doc_type,source_doc_id,source_batch_id,
			qty_g,qty_units,remaining_g,remaining_units,operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$9,$10,$11,now())
		ON CONFLICT (batch_code) DO UPDATE SET
			item_type=excluded.item_type,
			item_id=excluded.item_id,
			item_name=excluded.item_name,
			spec_g=excluded.spec_g,
			source_doc_type=excluded.source_doc_type,
			source_doc_id=excluded.source_doc_id,
			source_batch_id=excluded.source_batch_id,
			qty_g=excluded.qty_g,
			qty_units=excluded.qty_units,
			remaining_g=excluded.remaining_g,
			remaining_units=excluded.remaining_units,
			operator=excluded.operator
	`, schema),
		batchCode, stockItemTypeFinishedProduct, r.ProductID, r.Product, r.SpecG,
		stockSourceProductionRun, r.ID, r.BatchID,
		finishedTotalG, add.Units, operator,
	)
	if err != nil {
		return "", err
	}
	return batchCode, nil
}

func insertStockLedgerEntryTx(ctx context.Context, tx pgx.Tx, schema string, itemType string, itemID int64, itemName string, specG int64, warehouse string, sourceDocType string, sourceDocID int64, sourceBatchCode string, sourceBatchID string, qty stockLedgerQty, operator string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,spec_g,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,
			qty_before_units,qty_change_units,qty_after_units,
			operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,now())
	`, schema),
		itemType, itemID, itemName, specG, warehouse,
		sourceDocType, sourceDocID, sourceBatchCode, sourceBatchID,
		qty.BeforeG, qty.ChangeG, qty.AfterG,
		qty.BeforeUnits, qty.ChangeUnits, qty.AfterUnits,
		operator,
	)
	return err
}

func recordFinishedProductStockMovementTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, before, add, after InvQty, finishedTotalG int64, warehouse string, operator string) error {
	batchCode, err := createFinishedStockBatchTx(ctx, tx, schema, r, add, finishedTotalG, operator)
	if err != nil {
		return err
	}
	qty, err := finishedInventoryLedgerQty(r.SpecG, before, add, after)
	if err != nil {
		return err
	}
	return insertStockLedgerEntryTx(ctx, tx, schema,
		stockItemTypeFinishedProduct, r.ProductID, r.Product, r.SpecG, warehouse,
		stockSourceProductionRun, r.ID, batchCode, r.BatchID,
		qty, operator,
	)
}
