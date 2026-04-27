package bom

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureBomTables(ctx, pool, schema); err != nil {
		return err
	}
	return ensureBagSpecMappingTable(ctx, pool, schema)
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

func ensureBagSpecMappingTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.packaging_spec_material_map (
		spec_g BIGINT PRIMARY KEY,
		material_id BIGINT NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
