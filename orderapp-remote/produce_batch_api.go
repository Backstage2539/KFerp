package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type ProduceBatchAllocateItem struct {
	OrderItemID   int64 `json:"order_item_id"`
	AllocateUnits int64 `json:"allocate_units"`
}

type CreateProduceBatchRequest struct {
	OrderIDs      []int64                    `json:"order_ids"`
	BatchID       string                     `json:"batch_id"`
	Operator      string                     `json:"operator"`
	IdempotencyKey string                    `json:"idempotency_key"`
	Allocations   []ProduceBatchAllocateItem `json:"allocations"`
}

func validateCreateProduceBatchRequest(req CreateProduceBatchRequest) error {
	if len(req.OrderIDs) == 0 {
		return fmt.Errorf("order_ids required")
	}
	if strings.TrimSpace(req.BatchID) == "" {
		return fmt.Errorf("batch_id required")
	}
	if strings.TrimSpace(req.Operator) == "" {
		return fmt.Errorf("operator required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return fmt.Errorf("idempotency_key required")
	}
	return nil
}

type ProduceBatchListItem struct {
	BatchID      string `json:"batch_id"`
	Status       string `json:"status"`
	Operator     string `json:"operator"`
	CreatedAt    string `json:"created_at"`
	OrderCount   int64  `json:"order_count"`
	DeductStatus string `json:"deduct_status"`
	DeductedAt   string `json:"deducted_at"`
}

type ProduceBatchDetail struct {
	BatchID   string                    `json:"batch_id"`
	Status    string                    `json:"status"`
	Operator  string                    `json:"operator"`
	CreatedAt string                    `json:"created_at"`
	Orders    []int64                   `json:"orders"`
	Summary   []ProduceBatchSummaryItem `json:"summary"`
}

type ProduceBatchPreviewItem struct {
	ProductID       int64  `json:"product_id"`
	ProductName     string `json:"product_name"`
	SpecG           int64  `json:"spec_g"`
	NeedUnits       int64  `json:"need_units"`
	NeedG           int64  `json:"need_g"`
	InvUnits        int64  `json:"inv_units"`
	InvLooseG       int64  `json:"inv_loose_g"`
	InvTotalG       int64  `json:"inv_total_g"`
	DeductedG       int64  `json:"deducted_g"`
	GapG            int64  `json:"gap_g"`
	WarningLowStock bool   `json:"warning_low_stock"`
}

type ProduceBatchDeductPreview struct {
	BatchID string                   `json:"batch_id"`
	Summary []ProduceBatchPreviewItem `json:"summary"`
}

func registerProduceBatchAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.POST("/api/produce/batch/create", func(c echo.Context) error {
		var req CreateProduceBatchRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if err := validateCreateProduceBatchRequest(req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		allocMap := map[int64]int64{}
		for _, a := range req.Allocations {
			if a.OrderItemID > 0 && a.AllocateUnits > 0 {
				allocMap[a.OrderItemID] = a.AllocateUnits
			}
		}
		res, err := createProduceBatchFromOrders(c.Request().Context(), pool, schema, req.OrderIDs, req.Operator, allocMap)
		if err != nil {
			if strings.Contains(err.Error(), "exceed") {
				return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
			}
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, res)
	})

	e.GET("/api/produce/batch/list", func(c echo.Context) error {
		limit := 20
		if v := strings.TrimSpace(c.QueryParam("limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		q := fmt.Sprintf(`
			SELECT b.batch_id, b.status, b.operator, to_char(b.created_at,'YYYY-MM-DD HH24:MI:SS'),
			       COALESCE((SELECT COUNT(DISTINCT x.order_id) FROM %s.produce_batch_order_items x WHERE x.batch_id=b.batch_id),0),
			       CASE
			         WHEN l.cnt IS NULL THEN 'none'
			         WHEN l.total_gap_g = 0 THEN 'done'
			         ELSE 'partial'
			       END AS deduct_status,
			       COALESCE(to_char(l.last_deducted_at,'YYYY-MM-DD HH24:MI:SS'),'') AS deducted_at
			FROM %s.produce_batches b
			LEFT JOIN (
			  SELECT batch_id, COUNT(*) AS cnt, SUM(gap_g)::bigint AS total_gap_g, MAX(created_at) AS last_deducted_at
			  FROM %s.finished_allocation_logs
			  GROUP BY batch_id
			) l ON l.batch_id=b.batch_id
			ORDER BY b.created_at DESC
			LIMIT $1
		`, schema, schema, schema)
		rows, err := pool.Query(c.Request().Context(), q, limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		defer rows.Close()
		out := make([]ProduceBatchListItem, 0)
		for rows.Next() {
			var r ProduceBatchListItem
			if err := rows.Scan(&r.BatchID, &r.Status, &r.Operator, &r.CreatedAt, &r.OrderCount, &r.DeductStatus, &r.DeductedAt); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			out = append(out, r)
		}
		return c.JSON(http.StatusOK, out)
	})

	e.GET("/api/produce/batch/:batch_id/deduct-preview", func(c echo.Context) error {
		bid := strings.TrimSpace(c.Param("batch_id"))
		if bid == "" {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "batch_id required"})
		}

		tx, err := pool.BeginTx(c.Request().Context(), pgx.TxOptions{})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		defer tx.Rollback(c.Request().Context())

		// ensure batch exists and lock its rows in current tx
		var exists int
		if err := tx.QueryRow(c.Request().Context(), "SELECT 1 FROM "+schema+".produce_batches WHERE batch_id=$1 FOR UPDATE", bid).Scan(&exists); err != nil {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "batch not found"})
		}

		rows, err := tx.Query(c.Request().Context(),
			"SELECT i.product_id,COALESCE((SELECT name FROM "+schema+".products p WHERE p.id=i.product_id),''),i.spec_g,i.need_units,i.need_g,COALESCE(fi.onhand_units,0),COALESCE(fi.onhand_loose_g,0) FROM "+schema+".produce_batch_items i LEFT JOIN LATERAL (SELECT onhand_units,onhand_loose_g FROM "+schema+".finished_inventory f WHERE f.product_id=i.product_id AND f.spec_g=i.spec_g FOR UPDATE) fi ON true WHERE i.batch_id=$1 ORDER BY i.product_id,i.spec_g FOR UPDATE OF i", bid)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		defer rows.Close()
		out := ProduceBatchDeductPreview{BatchID: bid, Summary: make([]ProduceBatchPreviewItem, 0)}
		for rows.Next() {
			var s ProduceBatchPreviewItem
			if err := rows.Scan(&s.ProductID, &s.ProductName, &s.SpecG, &s.NeedUnits, &s.NeedG, &s.InvUnits, &s.InvLooseG); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			totalG, terr := invTotalG(s.SpecG, InvQty{Units: s.InvUnits, LooseG: s.InvLooseG})
			if terr != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: terr.Error()})
			}
			s.InvTotalG = totalG
			_, deductedG, gapG, derr := invDeduct(s.SpecG, InvQty{Units: s.InvUnits, LooseG: s.InvLooseG}, s.NeedG)
			if derr != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: derr.Error()})
			}
			s.DeductedG = deductedG
			s.GapG = gapG
			// unified rule: low inventory is allowed with warning
			if s.DeductedG > 0 && (s.InvTotalG-s.DeductedG) < s.SpecG {
				s.WarningLowStock = true
			}
			out.Summary = append(out.Summary, s)
		}
		if err := rows.Err(); err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		if err := tx.Commit(c.Request().Context()); err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, out)
	})

	e.GET("/api/produce/batch/:batch_id", func(c echo.Context) error {
		bid := strings.TrimSpace(c.Param("batch_id"))
		if bid == "" {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "batch_id required"})
		}

		var d ProduceBatchDetail
		err := pool.QueryRow(c.Request().Context(),
			"SELECT batch_id,status,operator,to_char(created_at,'YYYY-MM-DD HH24:MI:SS') FROM "+schema+".produce_batches WHERE batch_id=$1",
			bid,
		).Scan(&d.BatchID, &d.Status, &d.Operator, &d.CreatedAt)
		if err != nil {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "batch not found"})
		}

		orows, err := pool.Query(c.Request().Context(),
			"SELECT DISTINCT order_id FROM "+schema+".produce_batch_order_items WHERE batch_id=$1 ORDER BY order_id", bid)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		defer orows.Close()
		d.Orders = make([]int64, 0)
		for orows.Next() {
			var oid int64
			if err := orows.Scan(&oid); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			d.Orders = append(d.Orders, oid)
		}

		srows, err := pool.Query(c.Request().Context(),
			"SELECT i.product_id,COALESCE((SELECT name FROM "+schema+".products p WHERE p.id=i.product_id),''),i.spec_g,i.need_units,i.need_g,COALESCE(l.deducted_g,0),COALESCE(l.gap_g,0) FROM "+schema+".produce_batch_items i LEFT JOIN (SELECT product_id,spec_g,SUM(deducted_g)::bigint AS deducted_g,SUM(gap_g)::bigint AS gap_g FROM "+schema+".finished_allocation_logs WHERE batch_id=$1 GROUP BY product_id,spec_g) l ON l.product_id=i.product_id AND l.spec_g=i.spec_g WHERE i.batch_id=$1 ORDER BY i.product_id,i.spec_g", bid)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		defer srows.Close()
		d.Summary = make([]ProduceBatchSummaryItem, 0)
		for srows.Next() {
			var s ProduceBatchSummaryItem
			if err := srows.Scan(&s.ProductID, &s.ProductName, &s.SpecG, &s.NeedUnits, &s.NeedG, &s.DeductedG, &s.GapG); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			d.Summary = append(d.Summary, s)
		}

		return c.JSON(http.StatusOK, d)
	})
}
