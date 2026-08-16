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
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS stock_batches_source_uq
	ON %s.stock_batches(source_doc_type, source_doc_id, item_type, item_id, bom_spec_id, spec_g)
	WHERE source_doc_type <> '';
CREATE INDEX IF NOT EXISTS stock_batches_item_idx
	ON %s.stock_batches(item_type, item_id, bom_spec_id, spec_g, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.stock_ledger_entries (
	id BIGSERIAL PRIMARY KEY,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
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
ALTER TABLE %s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS stock_ledger_source_idx
	ON %s.stock_ledger_entries(source_doc_type, source_doc_id, id);
CREATE INDEX IF NOT EXISTS stock_ledger_item_idx
	ON %s.stock_ledger_entries(item_type, item_id, bom_spec_id, spec_g, created_at DESC);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS remaining_g BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS remaining_units BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS quality_status TEXT NOT NULL DEFAULT 'unchecked'`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`DROP INDEX IF EXISTS %s.stock_batches_source_uq`, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX stock_batches_source_uq ON %s.stock_batches(source_doc_type,source_doc_id,item_type,item_id,bom_spec_id,spec_g) WHERE source_doc_type<>''`, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS stock_batches_quality_idx ON %s.stock_batches(item_type, quality_status, batch_code)`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func EnsureStockLedgerTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return ensureStockLedgerTables(ctx, pool, schema)
}

func finishedProductionBatchCode(runningItemID int64) string {
	return fmt.Sprintf("FP-%010d", runningItemID)
}

func finishedProductionBatchCodeForSpec(runningItemID int64, specG int64) string {
	return fmt.Sprintf("FP-%010d-%dg", runningItemID, specG)
}

func finishedInventoryLedgerQty(specG int64, before, add, after InvQty) (stockLedgerQty, error) {
	if specG <= 0 {
		if before.Units < 0 || add.Units < 0 || after.Units < 0 || before.LooseG != 0 || add.LooseG != 0 || after.LooseG != 0 {
			return stockLedgerQty{}, fmt.Errorf("invalid count-based finished inventory quantity")
		}
		return stockLedgerQty{BeforeUnits: before.Units, ChangeUnits: add.Units, AfterUnits: after.Units}, nil
	}
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
	return createFinishedStockBatchWithCodeTx(ctx, tx, schema, finishedProductionBatchCode(r.ID), r, add, finishedTotalG, operator)
}

func createFinishedStockBatchWithCodeTx(ctx context.Context, tx pgx.Tx, schema string, batchCode string, r ProduceRunRow, add InvQty, finishedTotalG int64, operator string) (string, error) {
	hasBomSpec, err := schemaColumnExistsTx(ctx, tx, schema, "stock_batches", "bom_spec_id")
	if err != nil {
		return "", err
	}
	if !hasBomSpec {
		if r.BomSpecID > 0 || r.BomVariantID > 0 {
			return "", fmt.Errorf("stock batch BOM specification columns are not available")
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.stock_batches(
				batch_code,item_type,item_id,item_name,spec_g,
				source_doc_type,source_doc_id,source_batch_id,
				qty_g,qty_units,remaining_g,remaining_units,operator,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$9,$10,$11,now())
			ON CONFLICT (batch_code) DO UPDATE SET
				item_type=excluded.item_type,item_id=excluded.item_id,item_name=excluded.item_name,
				spec_g=excluded.spec_g,source_doc_type=excluded.source_doc_type,
				source_doc_id=excluded.source_doc_id,source_batch_id=excluded.source_batch_id,
				qty_g=stock_batches.qty_g+excluded.qty_g,
				qty_units=stock_batches.qty_units+excluded.qty_units,
				remaining_g=stock_batches.remaining_g+excluded.remaining_g,
				remaining_units=stock_batches.remaining_units+excluded.remaining_units,
				operator=excluded.operator
		`, schema), batchCode, stockItemTypeFinishedProduct, r.ProductID, r.Product, r.SpecG,
			stockSourceProductionRun, r.ID, r.BatchID, finishedTotalG, add.Units, operator)
		if err != nil {
			return "", err
		}
		return batchCode, nil
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,
			source_doc_type,source_doc_id,source_batch_id,
			qty_g,qty_units,remaining_g,remaining_units,operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$11,$12,$13,now())
		ON CONFLICT (batch_code) DO UPDATE SET
			item_type=excluded.item_type,
			item_id=excluded.item_id,
			item_name=excluded.item_name,
			bom_spec_id=excluded.bom_spec_id,
			bom_variant_id=excluded.bom_variant_id,
			spec_g=excluded.spec_g,
			source_doc_type=excluded.source_doc_type,
			source_doc_id=excluded.source_doc_id,
			source_batch_id=excluded.source_batch_id,
			qty_g=stock_batches.qty_g+excluded.qty_g,
			qty_units=stock_batches.qty_units+excluded.qty_units,
			remaining_g=stock_batches.remaining_g+excluded.remaining_g,
			remaining_units=stock_batches.remaining_units+excluded.remaining_units,
			operator=excluded.operator
	`, schema),
		batchCode, stockItemTypeFinishedProduct, r.ProductID, r.Product, r.BomSpecID, r.BomVariantID, r.SpecG,
		stockSourceProductionRun, r.ID, r.BatchID,
		finishedTotalG, add.Units, operator,
	)
	if err != nil {
		return "", err
	}
	return batchCode, nil
}

func insertStockLedgerEntryTx(ctx context.Context, tx pgx.Tx, schema string, itemType string, itemID int64, itemName string, specG int64, warehouse string, sourceDocType string, sourceDocID int64, sourceBatchCode string, sourceBatchID string, qty stockLedgerQty, operator string) error {
	return insertStockLedgerEntryWithBomSpecTx(ctx, tx, schema, itemType, itemID, itemName, 0, 0, specG, warehouse, sourceDocType, sourceDocID, sourceBatchCode, sourceBatchID, qty, operator)
}

func insertStockLedgerEntryWithBomSpecTx(ctx context.Context, tx pgx.Tx, schema string, itemType string, itemID int64, itemName string, bomSpecID, bomVariantID, specG int64, warehouse string, sourceDocType string, sourceDocID int64, sourceBatchCode string, sourceBatchID string, qty stockLedgerQty, operator string) error {
	hasBomSpec, err := schemaColumnExistsTx(ctx, tx, schema, "stock_ledger_entries", "bom_spec_id")
	if err != nil {
		return err
	}
	if !hasBomSpec {
		if bomSpecID > 0 || bomVariantID > 0 {
			return fmt.Errorf("stock ledger BOM specification columns are not available")
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.stock_ledger_entries(
				item_type,item_id,item_name,spec_g,warehouse,
				source_doc_type,source_doc_id,source_batch_code,source_batch_id,
				qty_before_g,qty_change_g,qty_after_g,
				qty_before_units,qty_change_units,qty_after_units,
				operator,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,now())
		`, schema), itemType, itemID, itemName, specG, warehouse,
			sourceDocType, sourceDocID, sourceBatchCode, sourceBatchID,
			qty.BeforeG, qty.ChangeG, qty.AfterG,
			qty.BeforeUnits, qty.ChangeUnits, qty.AfterUnits, operator)
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,
			qty_before_units,qty_change_units,qty_after_units,
			operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,now())
	`, schema),
		itemType, itemID, itemName, bomSpecID, bomVariantID, specG, warehouse,
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
	return insertStockLedgerEntryWithBomSpecTx(ctx, tx, schema,
		stockItemTypeFinishedProduct, r.ProductID, r.Product, r.BomSpecID, r.BomVariantID, r.SpecG, warehouse,
		stockSourceProductionRun, r.ID, batchCode, r.BatchID,
		qty, operator,
	)
}

func recordFinishedProductStockMovementWithBatchCodeTx(ctx context.Context, tx pgx.Tx, schema string, batchCode string, r ProduceRunRow, before, add, after InvQty, finishedTotalG int64, warehouse string, operator string) error {
	batchCode, err := createFinishedStockBatchWithCodeTx(ctx, tx, schema, batchCode, r, add, finishedTotalG, operator)
	if err != nil {
		return err
	}
	qty, err := finishedInventoryLedgerQty(r.SpecG, before, add, after)
	if err != nil {
		return err
	}
	return insertStockLedgerEntryWithBomSpecTx(ctx, tx, schema,
		stockItemTypeFinishedProduct, r.ProductID, r.Product, r.BomSpecID, r.BomVariantID, r.SpecG, warehouse,
		stockSourceProductionRun, r.ID, batchCode, r.BatchID,
		qty, operator,
	)
}
