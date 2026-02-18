package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type CreateProduceBatchRequest struct {
	OrderIDs []int64 `json:"order_ids"`
	Operator string  `json:"operator"`
}

type ProduceBatchListItem struct {
	BatchID    string `json:"batch_id"`
	Status     string `json:"status"`
	Operator   string `json:"operator"`
	CreatedAt  string `json:"created_at"`
	OrderCount int64  `json:"order_count"`
}

type ProduceBatchDetail struct {
	BatchID   string                    `json:"batch_id"`
	Status    string                    `json:"status"`
	Operator  string                    `json:"operator"`
	CreatedAt string                    `json:"created_at"`
	Orders    []int64                   `json:"orders"`
	Summary   []ProduceBatchSummaryItem `json:"summary"`
}

func registerProduceBatchAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.POST("/api/produce/batch/create", func(c echo.Context) error {
		var req CreateProduceBatchRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if len(req.OrderIDs) == 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "order_ids required"})
		}
		res, err := createProduceBatchFromOrders(c.Request().Context(), pool, schema, req.OrderIDs, req.Operator)
		if err != nil {
			if strings.Contains(err.Error(), "already in active batch") {
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
			       COALESCE((SELECT COUNT(DISTINCT x.order_id) FROM %s.produce_batch_order_items x WHERE x.batch_id=b.batch_id),0)
			FROM %s.produce_batches b
			ORDER BY b.created_at DESC
			LIMIT $1
		`, schema, schema)
		rows, err := pool.Query(c.Request().Context(), q, limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		defer rows.Close()
		out := make([]ProduceBatchListItem, 0)
		for rows.Next() {
			var r ProduceBatchListItem
			if err := rows.Scan(&r.BatchID, &r.Status, &r.Operator, &r.CreatedAt, &r.OrderCount); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			out = append(out, r)
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
			"SELECT product_id,COALESCE((SELECT name FROM "+schema+".products p WHERE p.id=i.product_id),''),spec_g,need_units,need_g FROM "+schema+".produce_batch_items i WHERE batch_id=$1 ORDER BY product_id,spec_g", bid)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		defer srows.Close()
		d.Summary = make([]ProduceBatchSummaryItem, 0)
		for srows.Next() {
			var s ProduceBatchSummaryItem
			if err := srows.Scan(&s.ProductID, &s.ProductName, &s.SpecG, &s.NeedUnits, &s.NeedG); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			d.Summary = append(d.Summary, s)
		}

		return c.JSON(http.StatusOK, d)
	})
}
