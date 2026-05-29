package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type UpdateJobCardMetricsRequest struct {
	PlannedInputQty float64 `json:"planned_input_qty"`
	ActualInputQty  float64 `json:"actual_input_qty"`
	ActualOutputQty float64 `json:"actual_output_qty"`
	ActualLossQty   float64 `json:"actual_loss_qty"`
	ExceptionReason string  `json:"exception_reason"`
	MetricsJSON     string  `json:"metrics_json"`
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
	e.POST("/api/produce/job-cards/:id/metrics", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		var req UpdateJobCardMetricsRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := productionSvc.UpdateJobCardMetrics(c.Request().Context(), productionapp.UpdateJobCardMetricsCommand{
			ID:              id,
			PlannedInputQty: req.PlannedInputQty,
			ActualInputQty:  req.ActualInputQty,
			ActualOutputQty: req.ActualOutputQty,
			ActualLossQty:   req.ActualLossQty,
			ExceptionReason: req.ExceptionReason,
			MetricsJSON:     req.MetricsJSON,
			Operator:        support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})
	e.GET("/api/produce/costs", func(c echo.Context) error {
		rows, err := productionSvc.ListBatchCosts(c.Request().Context(), productionapp.BatchCostQuery{Limit: support.IntParam(c, "limit", 200)})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
}
