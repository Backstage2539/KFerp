package materials

import (
	"context"
	"fmt"
	"strings"

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
