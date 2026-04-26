package inventory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	inventoryapp "orderapp/internal/application/inventory"
	inventorydomain "orderapp/internal/domain/inventory"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	stockItemTypeFinishedProduct = "finished_product"
	stockSourceManualAdjustment  = "manual_adjustment"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) ListFinished(ctx context.Context, query inventoryapp.FinishedInventoryQuery) (inventoryapp.FinishedInventoryResult, error) {
	where := ""
	args := []any{}
	argn := 1
	if s := strings.TrimSpace(query.Q); s != "" {
		where = fmt.Sprintf("WHERE p.name ILIKE $%d", argn)
		args = append(args, "%"+s+"%")
		argn++
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg := argn
	offsetArg := argn + 1

	sql := fmt.Sprintf(`
		SELECT fi.product_id, COALESCE(p.name,''), fi.spec_g, fi.onhand_units, fi.onhand_loose_g,
		       to_char(fi.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.finished_inventory fi
		LEFT JOIN %s.products p ON p.id = fi.product_id
		%s
		ORDER BY COALESCE(p.name,''), fi.spec_g
		LIMIT $%d OFFSET $%d
	`, r.schema, r.schema, where, limitArg, offsetArg)
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return inventoryapp.FinishedInventoryResult{}, err
	}
	defer rows.Close()

	out := make([]inventoryapp.FinishedInventoryRow, 0)
	for rows.Next() {
		var row inventoryapp.FinishedInventoryRow
		if err := rows.Scan(&row.ProductID, &row.Product, &row.SpecG, &row.Units, &row.LooseG, &row.UpdatedAt); err != nil {
			return inventoryapp.FinishedInventoryResult{}, err
		}
		if total, err := inventorydomain.TotalGrams(row.SpecG, inventorydomain.Quantity{Units: row.Units, LooseG: row.LooseG}); err == nil {
			row.TotalG = total
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return inventoryapp.FinishedInventoryResult{}, err
	}

	products, err := r.listProducts(ctx)
	if err != nil {
		return inventoryapp.FinishedInventoryResult{}, err
	}
	hasNext := false
	if len(out) > query.Limit {
		out = out[:query.Limit]
		hasNext = true
	}
	return inventoryapp.FinishedInventoryResult{Rows: out, Products: products, HasNext: hasNext}, nil
}

func (r Repository) AdjustFinished(ctx context.Context, cmd inventoryapp.AdjustFinishedInventoryCommand) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1`, r.schema), cmd.ProductID).Scan(&productName); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("product not found")
		}
		return err
	}

	before := inventorydomain.Quantity{}
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g
		FROM %s.finished_inventory
		WHERE product_id=$1 AND spec_g=$2
		FOR UPDATE
	`, r.schema), cmd.ProductID, cmd.SpecG).Scan(&before.Units, &before.LooseG)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	after := inventorydomain.Quantity{Units: cmd.Units, LooseG: cmd.LooseG}
	beforeG, err := inventorydomain.TotalGrams(cmd.SpecG, before)
	if err != nil {
		return err
	}
	afterG, err := inventorydomain.TotalGrams(cmd.SpecG, after)
	if err != nil {
		return err
	}
	changeG := afterG - beforeG

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,now())
		ON CONFLICT (product_id,spec_g) DO UPDATE
		SET onhand_units=excluded.onhand_units, onhand_loose_g=excluded.onhand_loose_g, updated_at=now()
	`, r.schema), cmd.ProductID, cmd.SpecG, after.Units, after.LooseG); err != nil {
		return err
	}

	batchCode := manualAdjustmentBatchCode()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,
			source_doc_type,source_doc_id,source_batch_id,
			qty_g,qty_units,operator,created_at
		) VALUES($1,$2,$3,$4,$5,'',0,'',$6,$7,$8,now())
	`, r.schema), batchCode, stockItemTypeFinishedProduct, cmd.ProductID, productName, cmd.SpecG, changeG, after.Units-before.Units, cmd.Operator); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,spec_g,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,
			qty_before_units,qty_change_units,qty_after_units,
			operator,created_at
		) VALUES($1,$2,$3,$4,'finished_goods',$5,0,$6,'',$7,$8,$9,$10,$11,$12,$13,now())
	`, r.schema),
		stockItemTypeFinishedProduct, cmd.ProductID, productName, cmd.SpecG,
		stockSourceManualAdjustment, batchCode,
		beforeG, changeG, afterG,
		before.Units, after.Units-before.Units, after.Units,
		cmd.Operator,
	); err != nil {
		return err
	}

	oldValue := fmt.Sprintf("%d+%dg", before.Units, before.LooseG)
	newValue := fmt.Sprintf("%d+%dg", after.Units, after.LooseG)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "finished_inventory", nil, "adjust", postgresinfra.StrPtr("quantity"), postgresinfra.StrPtr(oldValue), postgresinfra.StrPtr(newValue), postgresinfra.AuditMeta{
		"product_id": cmd.ProductID,
		"spec_g":     cmd.SpecG,
		"change_g":   changeG,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r Repository) listProducts(ctx context.Context) ([]inventoryapp.ProductOption, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id, name FROM %s.products WHERE active=true ORDER BY name`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]inventoryapp.ProductOption, 0)
	for rows.Next() {
		var p inventoryapp.ProductOption
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func manualAdjustmentBatchCode() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("ADJ-%s-%s", time.Now().Format("20060102150405"), hex.EncodeToString(b[:]))
}
