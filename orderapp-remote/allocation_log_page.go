package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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

type AllocationBatchRow struct {
	BatchID   string
	Items     int64
	Operator  string
	CreatedAt string
}

type AllocationLogPageData struct {
	BatchID   string
	Rows      []AllocationLogViewRow
	Batches   []AllocationBatchRow
	Page      int
	PerPage   int
	HasNext   bool
	PrevPage  int
	NextPage  int
	Error     string
}

func listAllocationBatches(ctx context.Context, pool *pgxpool.Pool, schema string, limit, offset int) ([]AllocationBatchRow, bool, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q := fmt.Sprintf(`
		SELECT batch_id,
		       count(*)::bigint as items,
		       COALESCE(max(operator), '') as operator,
		       to_char(max(created_at),'YYYY-MM-DD HH24:MI') as created_at
		FROM %s.finished_allocation_logs
		GROUP BY batch_id
		ORDER BY max(created_at) DESC
		LIMIT $1 OFFSET $2
	`, schema)
	rows, err := pool.Query(ctx, q, limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := make([]AllocationBatchRow, 0)
	for rows.Next() {
		var r AllocationBatchRow
		if err := rows.Scan(&r.BatchID, &r.Items, &r.Operator, &r.CreatedAt); err != nil {
			return nil, false, err
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

		page := 1
		per := 20
		if v := strings.TrimSpace(c.QueryParam("page")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				page = n
			}
		}
		if v := strings.TrimSpace(c.QueryParam("per_page")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				per = n
			}
		}
		data.Page = page
		data.PerPage = per
		if page > 1 {
			data.PrevPage = page - 1
		}
		data.NextPage = page + 1

		offset := (page - 1) * per
		batches, hasNext, err := listAllocationBatches(c.Request().Context(), pool, schema, per, offset)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Batches = batches
			data.HasNext = hasNext
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
