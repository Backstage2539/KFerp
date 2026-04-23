package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	productionapp "orderapp/internal/application/production"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type ProduceBatchAllocateItem struct {
	OrderItemID   int64 `json:"order_item_id"`
	AllocateUnits int64 `json:"allocate_units"`
}

type CreateProduceBatchRequest struct {
	OrderIDs       []int64                    `json:"order_ids"`
	BatchID        string                     `json:"batch_id"`
	Operator       string                     `json:"operator"`
	IdempotencyKey string                     `json:"idempotency_key"`
	Allocations    []ProduceBatchAllocateItem `json:"allocations"`
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
	NeedG        int64  `json:"need_g"`
	DeductedG    int64  `json:"deducted_g"`
	GapG         int64  `json:"gap_g"`

	// DEV-042 traceability fields (additive, non-breaking)
	CreatedBy       string `json:"created_by"`
	CreatedTime     string `json:"created_time"`
	StatusChangedAt string `json:"status_changed_at"`

	// DEV-045 compatibility aliases (keep old clients working)
	StatusText  string `json:"status_text"`
	CreateTime  string `json:"create_time"`
	DeductTime  string `json:"deduct_time"`
	DeductState string `json:"deduct_state"`
}

type ProduceBatchDetail struct {
	BatchID   string                    `json:"batch_id"`
	Status    string                    `json:"status"`
	Operator  string                    `json:"operator"`
	CreatedAt string                    `json:"created_at"`
	Orders    []int64                   `json:"orders"`
	Summary   []ProduceBatchSummaryItem `json:"summary"`

	// DEV-041 traceability fields (additive, compatible)
	CreatedBy    string `json:"created_by"`
	CreatedTime  string `json:"created_time"`
	StatusSource string `json:"status_source"`
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
	BatchID string                    `json:"batch_id"`
	Summary []ProduceBatchPreviewItem `json:"summary"`
}

type ProduceBatchDeductConfirmResponse struct {
	BatchID string                    `json:"batch_id"`
	Status  string                    `json:"status"`
	Summary []ProduceBatchSummaryItem `json:"summary"`
}

func registerProduceBatchAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	productionSvc := productionapp.NewService(postgresProductionRepository{pool: pool, schema: schema})

	e.POST("/api/produce/batch/create", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
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
		res, err := productionSvc.CreateBatch(c.Request().Context(), productionapp.CreateBatchCommand{
			OrderIDs:             req.OrderIDs,
			Operator:             req.Operator,
			IdempotencyKey:       req.IdempotencyKey,
			RequestUnitsByItemID: allocMap,
		})
		if err != nil {
			if strings.Contains(err.Error(), "exceed") {
				return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
			}
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, productionCreateResultFromApp(res))
	})

	e.GET("/api/produce/batch/list", func(c echo.Context) error {
		limit := 20
		if v := strings.TrimSpace(c.QueryParam("limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		statusQ := strings.TrimSpace(c.QueryParam("status"))
		operatorQ := strings.TrimSpace(c.QueryParam("operator"))
		fromQ := strings.TrimSpace(c.QueryParam("from"))
		toQ := strings.TrimSpace(c.QueryParam("to"))

		args := []any{}
		where := " WHERE 1=1"
		if statusQ != "" {
			args = append(args, statusQ)
			where += fmt.Sprintf(" AND b.status=$%d", len(args))
		}
		if operatorQ != "" {
			args = append(args, operatorQ)
			where += fmt.Sprintf(" AND b.operator=$%d", len(args))
		}
		if fromQ != "" {
			args = append(args, fromQ)
			where += fmt.Sprintf(" AND b.created_at >= $%d::date", len(args))
		}
		if toQ != "" {
			args = append(args, toQ)
			where += fmt.Sprintf(" AND b.created_at < ($%d::date + INTERVAL '1 day')", len(args))
		}
		args = append(args, limit)
		limitArg := len(args)

		q := fmt.Sprintf(`
			SELECT b.batch_id, b.status, b.operator, to_char(b.created_at,'YYYY-MM-DD HH24:MI:SS'),
			       COALESCE((SELECT COUNT(DISTINCT x.order_id) FROM %s.produce_batch_order_items x WHERE x.batch_id=b.batch_id),0),
			       CASE
			         WHEN l.cnt IS NULL THEN 'none'
			         WHEN l.total_gap_g = 0 THEN 'done'
			         ELSE 'partial'
			       END AS deduct_status,
			       COALESCE(to_char(l.last_deducted_at,'YYYY-MM-DD HH24:MI:SS'),'') AS deducted_at,
			       COALESCE(i.total_need_g,0) AS need_g,
			       COALESCE(l.total_deducted_g,0) AS deducted_g,
			       GREATEST(0, COALESCE(i.total_need_g,0) - COALESCE(l.total_deducted_g,0)) AS gap_g
			FROM %s.produce_batches b
			LEFT JOIN (
			  SELECT batch_id, SUM(need_g)::bigint AS total_need_g
			  FROM %s.produce_batch_items
			  GROUP BY batch_id
			) i ON i.batch_id=b.batch_id
			LEFT JOIN (
			  SELECT batch_id, COUNT(*) AS cnt, SUM(gap_g)::bigint AS total_gap_g, SUM(deducted_g)::bigint AS total_deducted_g, MAX(created_at) AS last_deducted_at
			  FROM %s.finished_allocation_logs
			  GROUP BY batch_id
			) l ON l.batch_id=b.batch_id
			%s
			ORDER BY b.created_at DESC
			LIMIT $%d
		`, schema, schema, schema, schema, where, limitArg)
		rows, err := pool.Query(c.Request().Context(), q, args...)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		defer rows.Close()
		out := make([]ProduceBatchListItem, 0)
		for rows.Next() {
			var r ProduceBatchListItem
			if err := rows.Scan(&r.BatchID, &r.Status, &r.Operator, &r.CreatedAt, &r.OrderCount, &r.DeductStatus, &r.DeductedAt, &r.NeedG, &r.DeductedG, &r.GapG); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			// DEV-042 traceability mirrors (additive)
			r.CreatedBy = r.Operator
			r.CreatedTime = r.CreatedAt
			if strings.TrimSpace(r.DeductedAt) != "" {
				r.StatusChangedAt = r.DeductedAt
			} else {
				r.StatusChangedAt = r.CreatedAt
			}
			// compatibility mirrors
			r.StatusText = r.Status
			r.CreateTime = r.CreatedAt
			r.DeductTime = r.DeductedAt
			r.DeductState = r.DeductStatus
			out = append(out, r)
		}
		return c.JSON(http.StatusOK, out)
	})

	e.GET("/api/produce/batch/:batch_id/deduct-preview", func(c echo.Context) error {
		bid := strings.TrimSpace(c.Param("batch_id"))
		if bid == "" {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "batch_id required"})
		}
		res, err := productionSvc.PreviewDeduct(c.Request().Context(), bid)
		if err != nil {
			if strings.Contains(err.Error(), "batch not found") {
				return c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, productionPreviewFromApp(res))
	})

	e.POST("/api/produce/batch/:batch_id/deduct-confirm", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		bid := strings.TrimSpace(c.Param("batch_id"))
		if bid == "" {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "batch_id required"})
		}
		op := strings.TrimSpace(c.QueryParam("operator"))
		res, err := productionSvc.ConfirmDeduct(c.Request().Context(), bid, op)
		if err != nil {
			if strings.Contains(err.Error(), "batch not found") {
				return c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			}
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, ProduceBatchDeductConfirmResponse{BatchID: res.BatchID, Status: res.Status, Summary: productionSummaryFromApp(res.Summary)})
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

		// DEV-041 traceability mirrors
		d.CreatedBy = d.Operator
		d.CreatedTime = d.CreatedAt
		d.StatusSource = "produce_batches.status"

		return c.JSON(http.StatusOK, d)
	})
}
