package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerPrefs struct {
	ID              int64   `json:"id"`
	DefaultSourceID *int    `json:"default_source_id"`
	SourceName      *string `json:"source_name"`
	DefaultTypeID   *int    `json:"default_order_type_id"`
	TypeName        *string `json:"order_type_name"`
	Address         *string `json:"address"`
}

func fetchCustomerPrefs(ctx context.Context, pool *pgxpool.Pool, schema string, id int64) (*CustomerPrefs, error) {
	q := fmt.Sprintf(`
		SELECT c.id, c.default_source_id, s.name, c.default_order_type_id, t.name, c.address
		FROM %s.customers c
		LEFT JOIN %s.sources s ON s.id = c.default_source_id
		LEFT JOIN %s.order_types t ON t.id = c.default_order_type_id
		WHERE c.id=$1
	`, schema, schema, schema)
	var p CustomerPrefs
	if err := pool.QueryRow(ctx, q, id).Scan(&p.ID, &p.DefaultSourceID, &p.SourceName, &p.DefaultTypeID, &p.TypeName, &p.Address); err != nil {
		return nil, err
	}
	return &p, nil
}
