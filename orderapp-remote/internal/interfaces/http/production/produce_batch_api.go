package production

import (
	"fmt"
	"net/http"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"

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
	productionSvc := productionapp.NewService(postgresproduction.NewRepository(pool, schema))

	e.POST("/api/produce/batch/create", func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
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
		rows, err := productionSvc.ListBatches(c.Request().Context(), productionapp.ListBatchesCommand{
			Limit:    limit,
			Status:   c.QueryParam("status"),
			Operator: c.QueryParam("operator"),
			From:     c.QueryParam("from"),
			To:       c.QueryParam("to"),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, produceBatchListFromApp(rows))
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
		if err := support.RequireEmployeeBound(c); err != nil {
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

		detail, err := productionSvc.Detail(c.Request().Context(), bid)
		if err != nil {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "batch not found"})
		}
		return c.JSON(http.StatusOK, produceBatchDetailFromApp(detail))
	})
}

func produceBatchListFromApp(rows []productionapp.BatchListItem) []ProduceBatchListItem {
	out := make([]ProduceBatchListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, ProduceBatchListItem{
			BatchID:         row.BatchID,
			Status:          row.Status,
			Operator:        row.Operator,
			CreatedAt:       row.CreatedAt,
			OrderCount:      row.OrderCount,
			DeductStatus:    row.DeductStatus,
			DeductedAt:      row.DeductedAt,
			NeedG:           row.NeedG,
			DeductedG:       row.DeductedG,
			GapG:            row.GapG,
			CreatedBy:       row.CreatedBy,
			CreatedTime:     row.CreatedTime,
			StatusChangedAt: row.StatusChangedAt,
			StatusText:      row.StatusText,
			CreateTime:      row.CreateTime,
			DeductTime:      row.DeductTime,
			DeductState:     row.DeductState,
		})
	}
	return out
}

func produceBatchDetailFromApp(row productionapp.BatchDetail) ProduceBatchDetail {
	return ProduceBatchDetail{
		BatchID:      row.BatchID,
		Status:       row.Status,
		Operator:     row.Operator,
		CreatedAt:    row.CreatedAt,
		Orders:       row.Orders,
		Summary:      productionSummaryFromApp(row.Summary),
		CreatedBy:    row.CreatedBy,
		CreatedTime:  row.CreatedTime,
		StatusSource: row.StatusSource,
	}
}
