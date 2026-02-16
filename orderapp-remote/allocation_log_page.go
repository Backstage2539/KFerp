package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type AllocationLogViewRow struct {
	BatchID   string
	Product   string
	SpecG     int64
	NeedG     int64
	DeductedG int64
	GapG      int64
	Operator  string
	CreatedAt string
}

type AllocationLogPageData struct {
	BatchID string
	Rows    []AllocationLogViewRow
	Batches []string
	Error   string
}

func listAllocationBatches(ctx context.Context, pool *pgxpool.Pool, schema string, limit int) ([]string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := fmt.Sprintf(`SELECT batch_id
		FROM %s.finished_allocation_logs
		GROUP BY batch_id
		ORDER BY max(created_at) DESC
		LIMIT $1`, schema)
	rows, err := pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func fetchAllocationLogsByBatch(ctx context.Context, pool *pgxpool.Pool, schema, batchID string) ([]AllocationLogViewRow, error) {
	q := fmt.Sprintf(`
		SELECT l.batch_id, COALESCE(p.name,''), l.spec_g, l.need_g, l.deducted_g, l.gap_g,
		       COALESCE(l.operator,''), to_char(l.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.finished_allocation_logs l
		LEFT JOIN %s.products p ON p.id = l.product_id
		WHERE l.batch_id = $1
		ORDER BY l.gap_g DESC, COALESCE(p.name,''), l.spec_g
	`, schema, schema)
	rows, err := pool.Query(ctx, q, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AllocationLogViewRow, 0)
	for rows.Next() {
		var r AllocationLogViewRow
		if err := rows.Scan(&r.BatchID, &r.Product, &r.SpecG, &r.NeedG, &r.DeductedG, &r.GapG, &r.Operator, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func registerAllocationLogPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/produce/allocations", func(c echo.Context) error {
		data := AllocationLogPageData{}
		data.BatchID = strings.TrimSpace(c.QueryParam("batch"))
		batches, err := listAllocationBatches(c.Request().Context(), pool, schema, 50)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Batches = batches
		}
		if data.BatchID != "" {
			rows, err := fetchAllocationLogsByBatch(c.Request().Context(), pool, schema, data.BatchID)
			if err != nil {
				data.Error = err.Error()
			} else {
				data.Rows = rows
			}
		}
		return c.Render(http.StatusOK, "allocation_logs.html", data)
	})
}
