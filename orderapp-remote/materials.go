package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MaterialRow struct {
	ID            int64
	Code          string
	Name          string
	Kind          string // bean|pack|other
	Unit          string // g|unit
	PurchasePrice float64
	SalePrice     float64
	OnhandG       int64
	OnhandUnits   int64
	MinLevelG     int64
	MinLevelUnits int64
	UpdatedAt     string
}

func ensureMaterialTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.materials (
		id BIGSERIAL PRIMARY KEY,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		kind TEXT NOT NULL DEFAULT 'other',
		unit TEXT NOT NULL DEFAULT 'g',
		purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
		onhand_g BIGINT NOT NULL DEFAULT 0,
		onhand_units BIGINT NOT NULL DEFAULT 0,
		min_level_g BIGINT NOT NULL DEFAULT 0,
		min_level_units BIGINT NOT NULL DEFAULT 0,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.materials ADD COLUMN IF NOT EXISTS purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.materials ADD COLUMN IF NOT EXISTS sale_price NUMERIC(12,2) NOT NULL DEFAULT 0`, schema))
	logQ := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.material_consumption_logs (
		id BIGSERIAL PRIMARY KEY,
		running_item_id BIGINT NOT NULL,
		batch_id TEXT NOT NULL DEFAULT '',
		product_id BIGINT NOT NULL DEFAULT 0,
		product_name TEXT NOT NULL DEFAULT '',
		spec_g BIGINT NOT NULL DEFAULT 0,
		material_id BIGINT NOT NULL,
		material_name TEXT NOT NULL DEFAULT '',
		unit TEXT NOT NULL DEFAULT '',
		deduct_g BIGINT NOT NULL DEFAULT 0,
		deduct_units BIGINT NOT NULL DEFAULT 0,
		before_g BIGINT NOT NULL DEFAULT 0,
		after_g BIGINT NOT NULL DEFAULT 0,
		before_units BIGINT NOT NULL DEFAULT 0,
		after_units BIGINT NOT NULL DEFAULT 0,
		operator TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS material_consumption_logs_running_idx ON %s.material_consumption_logs(running_item_id, id);
	CREATE INDEX IF NOT EXISTS material_consumption_logs_material_idx ON %s.material_consumption_logs(material_id, created_at DESC);`, schema, schema, schema)
	_, err := pool.Exec(ctx, logQ)
	return err
}

func ensureProductionLogTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.production_logs (
		id BIGSERIAL PRIMARY KEY,
		running_item_id BIGINT NOT NULL UNIQUE,
		batch_id TEXT NOT NULL DEFAULT '',
		product_id BIGINT NOT NULL DEFAULT 0,
		product_name TEXT NOT NULL DEFAULT '',
		spec_g BIGINT NOT NULL DEFAULT 0,
		order_nos TEXT NOT NULL DEFAULT '',
		planned_need_g BIGINT NOT NULL DEFAULT 0,
		input_g BIGINT NOT NULL DEFAULT 0,
		bom_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000,
		finished_units BIGINT NOT NULL DEFAULT 0,
		finished_loose_g BIGINT NOT NULL DEFAULT 0,
		finished_total_g BIGINT NOT NULL DEFAULT 0,
		actual_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
		started_by TEXT NOT NULL DEFAULT '',
		started_at TIMESTAMPTZ,
		finished_by TEXT NOT NULL DEFAULT '',
		finished_at TIMESTAMPTZ,
		inventory_units_before BIGINT NOT NULL DEFAULT 0,
		inventory_loose_g_before BIGINT NOT NULL DEFAULT 0,
		inventory_units_after BIGINT NOT NULL DEFAULT 0,
		inventory_loose_g_after BIGINT NOT NULL DEFAULT 0,
		material_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS production_logs_finished_idx ON %s.production_logs(finished_at DESC, id DESC);
	CREATE INDEX IF NOT EXISTS production_logs_product_idx ON %s.production_logs(product_id, finished_at DESC);`, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func listMaterials(ctx context.Context, pool *pgxpool.Pool, schema, q string, limit int) ([]MaterialRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	where := ""
	args := []any{}
	argn := 1
	if s := strings.TrimSpace(q); s != "" {
		where = fmt.Sprintf("WHERE (name ILIKE $%d OR code ILIKE $%d)", argn, argn)
		args = append(args, "%"+s+"%")
		argn++
	}
	args = append(args, limit)
	limitArg := argn

	sql := fmt.Sprintf(`
		SELECT id, code, name, kind, unit,
		       COALESCE(purchase_price,0), COALESCE(sale_price,0),
		       onhand_g, onhand_units, min_level_g, min_level_units,
		       to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.materials
		%s
		ORDER BY kind, name, id DESC
		LIMIT $%d
	`, schema, where, limitArg)
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MaterialRow, 0)
	for rows.Next() {
		var r MaterialRow
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Kind, &r.Unit, &r.PurchasePrice, &r.SalePrice, &r.OnhandG, &r.OnhandUnits, &r.MinLevelG, &r.MinLevelUnits, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func upsertMaterial(ctx context.Context, pool *pgxpool.Pool, schema string, code, name, kind, unit string, purchasePrice, salePrice float64, onhandG, onhandUnits, minG, minUnits int64) error {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	unit = strings.TrimSpace(unit)
	if code == "" {
		return fmt.Errorf("code required")
	}
	if name == "" {
		return fmt.Errorf("name required")
	}
	if kind == "" {
		kind = "other"
	}
	if unit == "" {
		unit = "g"
	}
	if purchasePrice < 0 || salePrice < 0 {
		return fmt.Errorf("negative price")
	}
	if onhandG < 0 || onhandUnits < 0 || minG < 0 || minUnits < 0 {
		return fmt.Errorf("negative qty")
	}
	q := fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,purchase_price,sale_price,onhand_g,onhand_units,min_level_g,min_level_units,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now())
		ON CONFLICT (code) DO UPDATE SET
			name=excluded.name,
			kind=excluded.kind,
			unit=excluded.unit,
			purchase_price=excluded.purchase_price,
			sale_price=excluded.sale_price,
			onhand_g=excluded.onhand_g,
			onhand_units=excluded.onhand_units,
			min_level_g=excluded.min_level_g,
			min_level_units=excluded.min_level_units,
			updated_at=now()`, schema)
	_, err := pool.Exec(ctx, q, code, name, kind, unit, purchasePrice, salePrice, onhandG, onhandUnits, minG, minUnits)
	return err
}
