package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FinishedInvRow struct {
	ProductID int64
	Product   string
	SpecG     int64
	Units     int64
	LooseG    int64
	UpdatedAt string
	TotalG    int64
}

func ensureFinishedInventoryTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.finished_inventory (
		product_id BIGINT NOT NULL,
		spec_g BIGINT NOT NULL,
		onhand_units BIGINT NOT NULL DEFAULT 0,
		onhand_loose_g BIGINT NOT NULL DEFAULT 0,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY(product_id, spec_g)
	)`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	return nil
}

func listFinishedInventory(ctx context.Context, pool *pgxpool.Pool, schema string, q string, limit, offset int) ([]FinishedInvRow, bool, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where := ""
	args := []any{}
	argn := 1
	if s := strings.TrimSpace(q); s != "" {
		where = fmt.Sprintf("WHERE p.name ILIKE $%d", argn)
		args = append(args, "%"+s+"%")
		argn++
	}
	args = append(args, limit+1, offset)
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
	`, schema, schema, where, limitArg, offsetArg)
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	out := make([]FinishedInvRow, 0)
	for rows.Next() {
		var r FinishedInvRow
		if err := rows.Scan(&r.ProductID, &r.Product, &r.SpecG, &r.Units, &r.LooseG, &r.UpdatedAt); err != nil {
			return nil, false, err
		}
		if tg, e := invTotalG(r.SpecG, InvQty{Units: r.Units, LooseG: r.LooseG}); e == nil {
			r.TotalG = tg
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

func upsertFinishedInventory(ctx context.Context, pool *pgxpool.Pool, schema string, productID, specG, units, looseG int64) error {
	if productID <= 0 {
		return fmt.Errorf("product required")
	}
	if specG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	if units < 0 || looseG < 0 {
		return fmt.Errorf("negative qty")
	}
	// normalize to keep loose_g < spec_g
	n, err := invNormalize(specG, InvQty{Units: units, LooseG: looseG})
	if err != nil {
		return err
	}
	q := fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,now())
		ON CONFLICT (product_id,spec_g) DO UPDATE SET onhand_units=excluded.onhand_units, onhand_loose_g=excluded.onhand_loose_g, updated_at=now()`, schema)
	_, err = pool.Exec(ctx, q, productID, specG, n.Units, n.LooseG)
	return err
}

func parseI64(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

// helper for templates
func nowHHMM() string {
	return time.Now().Format("15:04")
}
