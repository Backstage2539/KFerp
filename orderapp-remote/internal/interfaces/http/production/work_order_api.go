package production

import (
	"encoding/json"
	"fmt"
	"net/http"
	productionapp "orderapp/internal/application/production"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type jobCardActualsRequest struct {
	PlannedInputQty float64         `json:"planned_input_qty"`
	ActualInputQty  float64         `json:"actual_input_qty"`
	ActualOutputQty float64         `json:"actual_output_qty"`
	ActualLossQty   float64         `json:"actual_loss_qty"`
	LossReason      string          `json:"loss_reason"`
	ExceptionReason string          `json:"exception_reason"`
	MetricsJSON     json.RawMessage `json:"metrics_json"`
}

type workOrderCompleteRequest struct {
	FinishedUnits  int64  `json:"finished_units"`
	FinishedLooseG int64  `json:"finished_loose_g"`
	ConsumedInputG int64  `json:"consumed_input_g"`
	Warehouse      string `json:"warehouse"`
	Note           string `json:"note"`
}

func registerWorkOrderAPI(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/produce/work-orders", func(c echo.Context) error {
		target := "/vue-shell?view=workOrders"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})
	e.GET("/produce/job-cards", func(c echo.Context) error {
		target := "/vue-shell?view=jobCards"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})
	e.GET("/produce/costs", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, "/vue-shell?view=productionCosts"))
	})
	e.GET("/api/produce/work-orders", func(c echo.Context) error {
		rows, err := productionSvc.ListWorkOrders(c.Request().Context(), productionapp.WorkOrderQuery{
			Status: strings.TrimSpace(c.QueryParam("status")),
			Limit:  support.IntParam(c, "limit", 200),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/work-orders/:id/start", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid work_order_id"})
		}
		res, err := productionSvc.StartWorkOrder(c.Request().Context(), productionapp.WorkOrderStartCommand{ID: id, Operator: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "batch_id": res.BatchID, "running_item_id": res.RunningItemID, "work_order": res.WorkOrder})
	})
	e.POST("/api/work-orders/:id/complete", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid work_order_id"})
		}
		var req workOrderCompleteRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		res, err := productionSvc.CompleteWorkOrder(c.Request().Context(), productionapp.WorkOrderCompleteCommand{
			ID:             id,
			FinishedUnits:  req.FinishedUnits,
			FinishedLooseG: req.FinishedLooseG,
			ConsumedInputG: req.ConsumedInputG,
			Warehouse:      req.Warehouse,
			Operator:       support.ActorOf(c),
			Note:           req.Note,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "work_order": res.WorkOrder, "stock_entries": res.StockEntries, "cost": res.Cost})
	})
	e.GET("/api/produce/job-cards", func(c echo.Context) error {
		rows, err := productionSvc.ListJobCards(c.Request().Context(), productionapp.JobCardQuery{
			Status: strings.TrimSpace(c.QueryParam("status")),
			Limit:  support.IntParam(c, "limit", 200),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	updateJobCard := func(c echo.Context, returnRow bool) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid job_card_id"})
		}
		var req jobCardActualsRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		metricsJSON, ok := normalizeJobCardMetricsJSON(req.MetricsJSON)
		if !ok {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid metrics_json"})
		}
		err = productionSvc.UpdateJobCardActuals(c.Request().Context(), productionapp.JobCardActualsCommand{
			ID:              id,
			PlannedInputQty: req.PlannedInputQty,
			ActualInputQty:  req.ActualInputQty,
			ActualOutputQty: req.ActualOutputQty,
			ExceptionReason: req.ExceptionReason,
			MetricsJSON:     metricsJSON,
			Actor:           support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if returnRow {
			rows, err := productionSvc.ListJobCards(c.Request().Context(), productionapp.JobCardQuery{Limit: 500})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			for _, row := range rows {
				if row.ID == id {
					return c.JSON(http.StatusOK, row)
				}
			}
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "job card not found"})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	}
	runJobCardAction := func(c echo.Context, action string) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid job_card_id"})
		}
		var req jobCardActualsRequest
		if c.Request().Body != nil && c.Request().ContentLength != 0 {
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			}
		}
		metricsJSON, ok := normalizeJobCardMetricsJSON(req.MetricsJSON)
		if !ok {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid metrics_json"})
		}
		cmd := productionapp.JobCardActionCommand{
			ID:              id,
			Operator:        support.ActorOf(c),
			ActualInputQty:  req.ActualInputQty,
			ActualOutputQty: req.ActualOutputQty,
			ActualLossQty:   req.ActualLossQty,
			LossReason:      req.LossReason,
			ExceptionReason: req.ExceptionReason,
			MetricsJSON:     metricsJSON,
		}
		var res productionapp.JobCardActionResult
		switch action {
		case "start":
			res, err = productionSvc.StartJobCard(c.Request().Context(), cmd)
		case "pause":
			res, err = productionSvc.PauseJobCard(c.Request().Context(), cmd)
		case "resume":
			res, err = productionSvc.ResumeJobCard(c.Request().Context(), cmd)
		case "complete":
			res, err = productionSvc.CompleteJobCard(c.Request().Context(), cmd)
		default:
			err = fmt.Errorf("invalid job card action")
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "job_card": res.JobCard, "work_order": res.WorkOrder})
	}
	e.POST("/api/produce/job-cards/:id/actuals", func(c echo.Context) error {
		return updateJobCard(c, false)
	})
	e.POST("/api/produce/job-cards/:id/metrics", func(c echo.Context) error {
		return updateJobCard(c, true)
	})
	e.POST("/api/job-cards/:id/start", func(c echo.Context) error {
		return runJobCardAction(c, "start")
	})
	e.POST("/api/job-cards/:id/pause", func(c echo.Context) error {
		return runJobCardAction(c, "pause")
	})
	e.POST("/api/job-cards/:id/resume", func(c echo.Context) error {
		return runJobCardAction(c, "resume")
	})
	e.POST("/api/job-cards/:id/complete", func(c echo.Context) error {
		return runJobCardAction(c, "complete")
	})
	e.GET("/api/produce/costs", func(c echo.Context) error {
		rows, err := productionSvc.ListBatchCosts(c.Request().Context(), productionapp.BatchCostQuery{Limit: support.IntParam(c, "limit", 200)})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
}

func normalizeJobCardMetricsJSON(raw json.RawMessage) (string, bool) {
	metricsJSON := strings.TrimSpace(string(raw))
	if metricsJSON == "" || metricsJSON == "null" {
		return "{}", true
	}
	var nested string
	if err := json.Unmarshal(raw, &nested); err == nil {
		metricsJSON = strings.TrimSpace(nested)
		if metricsJSON == "" {
			metricsJSON = "{}"
		}
	}
	if !json.Valid([]byte(metricsJSON)) {
		return "", false
	}
	return metricsJSON, true
}
