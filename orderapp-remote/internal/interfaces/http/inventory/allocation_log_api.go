package inventory

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
	BatchID      string
	Product      string
	SpecG        int64
	NeedG        int64
	DeductedG    int64
	GapG         int64
	Operator     string
	CreatedAt    string
	OperatorName string `json:"operator_name"`
}

type AllocationBatchRow struct {
	BatchID      string
	Items        int64
	Operator     string
	CreatedAt    string
	OperatorName string `json:"operator_name"`
}

type AllocationLogPageData struct {
	BatchID  string
	Rows     []AllocationLogViewRow
	Batches  []AllocationBatchRow
	Page     int
	PerPage  int
	HasNext  bool
	PrevPage int
	NextPage int
	Error    string
}

type AllocationLogAPIResponse struct {
	BatchID  string                 `json:"batch_id"`
	Rows     []AllocationLogViewRow `json:"rows"`
	Batches  []AllocationBatchRow   `json:"batches"`
	Page     int                    `json:"page"`
	PerPage  int                    `json:"per_page"`
	HasNext  bool                   `json:"has_next"`
	HasPrev  bool                   `json:"has_prev"`
	PrevPage int                    `json:"prev_page"`
	NextPage int                    `json:"next_page"`
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
		r.OperatorName = strings.TrimSpace(r.Operator)
		if r.OperatorName == "" {
			r.OperatorName = "未知"
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
		r.OperatorName = strings.TrimSpace(r.Operator)
		if r.OperatorName == "" {
			r.OperatorName = "未知"
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func registerAllocationLogPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/produce/allocations", func(c echo.Context) error {
		target := "/vue-shell?view=allocationLogs"
		if raw := strings.TrimSpace(c.QueryString()); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, target)
	})

	e.GET("/api/produce/allocations", func(c echo.Context) error {
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

		offset := (page - 1) * per
		batches, hasNext, err := listAllocationBatches(c.Request().Context(), pool, schema, per, offset)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}

		batchID := strings.TrimSpace(c.QueryParam("batch"))
		if batchID == "" && len(batches) > 0 {
			batchID = strings.TrimSpace(batches[0].BatchID)
		}
		rows := []AllocationLogViewRow{}
		if batchID != "" {
			rows, err = fetchAllocationLogsByBatch(c.Request().Context(), pool, schema, batchID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
		}
		prevPage := 0
		if page > 1 {
			prevPage = page - 1
		}
		return c.JSON(http.StatusOK, AllocationLogAPIResponse{
			BatchID:  batchID,
			Rows:     rows,
			Batches:  batches,
			Page:     page,
			PerPage:  per,
			HasNext:  hasNext,
			HasPrev:  page > 1,
			PrevPage: prevPage,
			NextPage: page + 1,
		})
	})
}
