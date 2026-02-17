package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BomRow struct {
	ProductID int64
	Product   string
	YieldRate float64
	UpdatedAt string
}

func ensureBomTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.product_bom (
		product_id BIGINT PRIMARY KEY,
		yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func listBom(ctx context.Context, pool *pgxpool.Pool, schema string) ([]BomRow, error) {
	q := fmt.Sprintf(`
		SELECT b.product_id, COALESCE(p.name,''), b.yield_rate, to_char(b.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.product_bom b
		LEFT JOIN %s.products p ON p.id=b.product_id
		ORDER BY p.name, b.product_id
	`, schema, schema)
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BomRow, 0)
	for rows.Next() {
		var r BomRow
		if err := rows.Scan(&r.ProductID, &r.Product, &r.YieldRate, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
