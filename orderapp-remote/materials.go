package main

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MaterialRow struct {
	ID            int64   `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"` // bean|pack|other
	Unit          string  `json:"unit"` // g|unit
	PurchasePrice float64 `json:"purchase_price"`
	SalePrice     float64 `json:"sale_price"`
	OnhandG       int64   `json:"onhand_g"`
	OnhandUnits   int64   `json:"onhand_units"`
	MinLevelG     int64   `json:"min_level_g"`
	MinLevelUnits int64   `json:"min_level_units"`
	UpdatedAt     string  `json:"updated_at"`
}

type MaterialInput struct {
	Code          string  `json:"code" form:"code"`
	Name          string  `json:"name" form:"name"`
	Kind          string  `json:"kind" form:"kind"`
	Unit          string  `json:"unit" form:"unit"`
	PurchasePrice float64 `json:"purchase_price" form:"purchase_price"`
	SalePrice     float64 `json:"sale_price" form:"sale_price"`
	OnhandG       int64   `json:"onhand_g" form:"onhand_g"`
	OnhandUnits   int64   `json:"onhand_units" form:"onhand_units"`
	MinLevelG     int64   `json:"min_level_g" form:"min_level_g"`
	MinLevelUnits int64   `json:"min_level_units" form:"min_level_units"`
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

func normalizeMaterialInput(in MaterialInput) (MaterialInput, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.TrimSpace(in.Kind)
	in.Unit = strings.TrimSpace(in.Unit)
	if in.Code == "" {
		return MaterialInput{}, fmt.Errorf("code required")
	}
	if in.Name == "" {
		return MaterialInput{}, fmt.Errorf("name required")
	}
	if in.Kind == "" {
		in.Kind = "other"
	}
	if in.Unit == "" {
		in.Unit = "g"
	}
	switch in.Kind {
	case "bean", "pack", "other":
	default:
		return MaterialInput{}, fmt.Errorf("invalid kind")
	}
	if in.PurchasePrice < 0 || in.SalePrice < 0 || math.IsNaN(in.PurchasePrice) || math.IsNaN(in.SalePrice) || math.IsInf(in.PurchasePrice, 0) || math.IsInf(in.SalePrice, 0) {
		return MaterialInput{}, fmt.Errorf("negative price")
	}
	if in.OnhandG < 0 || in.OnhandUnits < 0 || in.MinLevelG < 0 || in.MinLevelUnits < 0 {
		return MaterialInput{}, fmt.Errorf("negative qty")
	}
	return in, nil
}

func upsertMaterial(ctx context.Context, pool *pgxpool.Pool, schema string, code, name, kind, unit string, purchasePrice, salePrice float64, onhandG, onhandUnits, minG, minUnits int64) error {
	in, err := normalizeMaterialInput(MaterialInput{
		Code:          code,
		Name:          name,
		Kind:          kind,
		Unit:          unit,
		PurchasePrice: purchasePrice,
		SalePrice:     salePrice,
		OnhandG:       onhandG,
		OnhandUnits:   onhandUnits,
		MinLevelG:     minG,
		MinLevelUnits: minUnits,
	})
	if err != nil {
		return err
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
	_, err = pool.Exec(ctx, q, in.Code, in.Name, in.Kind, in.Unit, in.PurchasePrice, in.SalePrice, in.OnhandG, in.OnhandUnits, in.MinLevelG, in.MinLevelUnits)
	return err
}

func updateMaterialInline(ctx context.Context, pool *pgxpool.Pool, schema, actor string, id int64, in MaterialInput) (MaterialRow, error) {
	if id <= 0 {
		return MaterialRow{}, fmt.Errorf("invalid id")
	}
	next, err := normalizeMaterialInput(in)
	if err != nil {
		return MaterialRow{}, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return MaterialRow{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return MaterialRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var old MaterialRow
	qOld := fmt.Sprintf(`SELECT id, code, name, kind, unit,
		       COALESCE(purchase_price,0), COALESCE(sale_price,0),
		       onhand_g, onhand_units, min_level_g, min_level_units,
		       to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.materials WHERE id=$1 FOR UPDATE`, schema)
	if err := tx.QueryRow(ctx, qOld, id).Scan(&old.ID, &old.Code, &old.Name, &old.Kind, &old.Unit, &old.PurchasePrice, &old.SalePrice, &old.OnhandG, &old.OnhandUnits, &old.MinLevelG, &old.MinLevelUnits, &old.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return MaterialRow{}, fmt.Errorf("not found")
		}
		return MaterialRow{}, err
	}

	q := fmt.Sprintf(`UPDATE %s.materials SET
			code=$2,
			name=$3,
			kind=$4,
			unit=$5,
			purchase_price=$6,
			sale_price=$7,
			onhand_g=$8,
			onhand_units=$9,
			min_level_g=$10,
			min_level_units=$11,
			updated_at=now()
		WHERE id=$1`, schema)
	if _, err := tx.Exec(ctx, q, id, next.Code, next.Name, next.Kind, next.Unit, next.PurchasePrice, next.SalePrice, next.OnhandG, next.OnhandUnits, next.MinLevelG, next.MinLevelUnits); err != nil {
		return MaterialRow{}, err
	}
	if err := logMaterialDiffsTx(ctx, tx, schema, actor, old, next); err != nil {
		return MaterialRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MaterialRow{}, err
	}

	rows, err := listMaterials(ctx, pool, schema, next.Code, 1)
	if err != nil {
		return MaterialRow{}, err
	}
	if len(rows) == 0 {
		return MaterialRow{}, fmt.Errorf("not found")
	}
	return rows[0], nil
}

func logMaterialDiffsTx(ctx context.Context, tx pgx.Tx, schema, actor string, old MaterialRow, next MaterialInput) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}
	id := old.ID
	log := func(field, oldValue, newValue string) error {
		if oldValue == newValue {
			return nil
		}
		q := fmt.Sprintf(`INSERT INTO %s.audit_logs(actor, entity_type, entity_id, action, field, old_value, new_value, meta)
			VALUES($1,'material',$2,'update',$3,$4,$5,jsonb_build_object('material_id',$2::bigint,'code',$6::text))`, schema)
		_, err := tx.Exec(ctx, q, actor, id, field, oldValue, newValue, next.Code)
		return err
	}
	for _, item := range []struct {
		field string
		old   string
		next  string
	}{
		{"code", old.Code, next.Code},
		{"name", old.Name, next.Name},
		{"kind", old.Kind, next.Kind},
		{"unit", old.Unit, next.Unit},
		{"purchase_price", fmt.Sprintf("%.2f", old.PurchasePrice), fmt.Sprintf("%.2f", next.PurchasePrice)},
		{"sale_price", fmt.Sprintf("%.2f", old.SalePrice), fmt.Sprintf("%.2f", next.SalePrice)},
		{"onhand_g", fmt.Sprintf("%d", old.OnhandG), fmt.Sprintf("%d", next.OnhandG)},
		{"onhand_units", fmt.Sprintf("%d", old.OnhandUnits), fmt.Sprintf("%d", next.OnhandUnits)},
		{"min_level_g", fmt.Sprintf("%d", old.MinLevelG), fmt.Sprintf("%d", next.MinLevelG)},
		{"min_level_units", fmt.Sprintf("%d", old.MinLevelUnits), fmt.Sprintf("%d", next.MinLevelUnits)},
	} {
		if err := log(item.field, item.old, item.next); err != nil {
			return err
		}
	}
	return nil
}
