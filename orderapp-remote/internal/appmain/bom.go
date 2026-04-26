package appmain

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BomRow struct {
	ProductID  int64
	Product    string
	RoastLevel string
	YieldRate  float64
	UpdatedAt  string
}

type BomItemRow struct {
	ID           int64
	MaterialID   int64
	MaterialName string
	RatioPct     float64
}

func ensureBomTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	ddls := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.product_bom (
			product_id BIGINT PRIMARY KEY,
			yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.product_bom_items (
			id BIGSERIAL PRIMARY KEY,
			product_id BIGINT NOT NULL,
			material_id BIGINT NOT NULL,
			ratio_pct NUMERIC(10,4) NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(product_id, material_id)
		)`, schema),
	}
	for _, q := range ddls {
		if _, err := pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func listBom(ctx context.Context, pool *pgxpool.Pool, schema string) ([]BomRow, error) {
	q := fmt.Sprintf(`
		SELECT p.id,
		       COALESCE(p.name,''),
		       COALESCE(p.roast_level,''),
		       COALESCE(b.yield_rate,0.8),
		       COALESCE(to_char(b.updated_at,'YYYY-MM-DD HH24:MI'),'-')
		FROM %s.products p
		LEFT JOIN %s.product_bom b ON b.product_id=p.id
		WHERE p.active=true
		ORDER BY p.name, p.id
	`, schema, schema)
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BomRow, 0)
	for rows.Next() {
		var r BomRow
		var fallback float64
		if err := rows.Scan(&r.ProductID, &r.Product, &r.RoastLevel, &fallback, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.YieldRate = resolveYieldRate(r.RoastLevel, fallback)
		out = append(out, r)
	}
	return out, rows.Err()
}

func listBomItems(ctx context.Context, pool *pgxpool.Pool, schema string, productID int64) ([]BomItemRow, float64, error) {
	q := fmt.Sprintf(`
		SELECT bi.id, bi.material_id, COALESCE(m.name,''), bi.ratio_pct
		FROM %s.product_bom_items bi
		LEFT JOIN %s.materials m ON m.id = bi.material_id
		WHERE bi.product_id=$1
		ORDER BY m.name, bi.id
	`, schema, schema)
	rows, err := pool.Query(ctx, q, productID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]BomItemRow, 0)
	total := 0.0
	for rows.Next() {
		var r BomItemRow
		if err := rows.Scan(&r.ID, &r.MaterialID, &r.MaterialName, &r.RatioPct); err != nil {
			return nil, 0, err
		}
		total += r.RatioPct
		out = append(out, r)
	}
	return out, total, rows.Err()
}
