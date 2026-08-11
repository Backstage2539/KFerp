package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type productionPlanCreateRequest struct {
	From       string           `json:"from"`
	To         string           `json:"to"`
	CustomerID int64            `json:"customer_id"`
	SourceType string           `json:"source_type"`
	Selected   []string         `json:"selected"`
	InputByKey map[string]int64 `json:"input_by_key"`
}

type productionPlanBatchSubmitRequest struct {
	IDs []int64 `json:"ids"`
}

type productionPlanOperationSplitsRequest struct {
	Items []productionapp.ProductionPlanOperationSplit `json:"items"`
}

type productionPlanCancelRequest struct {
	Note string `json:"note"`
}

type productionPlanItemTargetWarehouseRequest struct {
	TargetWarehouse string `json:"target_warehouse"`
}

func registerProductionPlanAPI(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/api/production-plans", func(c echo.Context) error {
		rows, err := productionSvc.ListProductionPlans(c.Request().Context(), productionapp.ProductionPlanQuery{
			Status:    strings.TrimSpace(c.QueryParam("status")),
			TimeField: strings.TrimSpace(c.QueryParam("time_field")),
			From:      strings.TrimSpace(c.QueryParam("from")),
			To:        strings.TrimSpace(c.QueryParam("to")),
			Limit:     support.IntParam(c, "limit", 50),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/production-plans", func(c echo.Context) error {
		var req productionPlanCreateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		selected := map[string]bool{}
		for _, key := range req.Selected {
			key = strings.TrimSpace(key)
			if key != "" {
				selected[key] = true
			}
		}
		plan, err := productionSvc.CreateProductionPlan(c.Request().Context(), productionapp.CreateProductionPlanCommand{
			From:       req.From,
			To:         req.To,
			CustomerID: req.CustomerID,
			SourceType: req.SourceType,
			Selected:   selected,
			InputByKey: req.InputByKey,
			Operator:   support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, plan)
	})
	e.POST("/api/production-plans/submit", func(c echo.Context) error {
		var req productionPlanBatchSubmitRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		res, err := productionSvc.SubmitProductionPlans(c.Request().Context(), productionapp.SubmitProductionPlansCommand{IDs: req.IDs, Operator: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, res)
	})
	e.GET("/api/production-plans/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid production_plan_id"})
		}
		plan, err := productionSvc.GetProductionPlan(c.Request().Context(), id)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, plan)
	})
	e.PATCH("/api/production-plans/:id/items/:item_id/target-warehouse", func(c echo.Context) error {
		planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || planID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid production_plan_id"})
		}
		itemID, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
		if err != nil || itemID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid production_plan_item_id"})
		}
		var req productionPlanItemTargetWarehouseRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		item, err := productionSvc.UpdateProductionPlanItemTargetWarehouse(c.Request().Context(), productionapp.UpdateProductionPlanItemTargetWarehouseCommand{
			ProductionPlanID: planID, ProductionPlanItemID: itemID,
			TargetWarehouse: req.TargetWarehouse, Operator: support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, item)
	})
	e.GET("/api/production-plans/:id/operation-splits", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid production_plan_id"})
		}
		plan, err := productionSvc.GetProductionPlan(c.Request().Context(), id)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": plan.OperationSplits})
	})
	e.POST("/api/production-plans/:id/operation-splits/preview", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid production_plan_id"})
		}
		var req productionPlanOperationSplitsRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		preview, err := productionSvc.PreviewProductionPlanOperationSplits(c.Request().Context(), productionapp.PreviewProductionPlanOperationSplitsCommand{
			ID:    id,
			Items: req.Items,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, preview)
	})
	e.POST("/api/production-plans/:id/operation-splits", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid production_plan_id"})
		}
		var req productionPlanOperationSplitsRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		rows, err := productionSvc.SaveProductionPlanOperationSplits(c.Request().Context(), productionapp.SaveProductionPlanOperationSplitsCommand{
			ID:       id,
			Items:    req.Items,
			Operator: support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/production-plans/:id/submit", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid production_plan_id"})
		}
		res, err := productionSvc.SubmitProductionPlan(c.Request().Context(), productionapp.SubmitProductionPlanCommand{ID: id, Operator: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, res)
	})
	e.POST("/api/production-plans/:id/cancel", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid production_plan_id"})
		}
		var req productionPlanCancelRequest
		if c.Request().Body != nil && c.Request().ContentLength != 0 {
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			}
		}
		plan, err := productionSvc.CancelProductionPlan(c.Request().Context(), productionapp.CancelProductionPlanCommand{
			ID:       id,
			Operator: support.ActorOf(c),
			Note:     req.Note,
		})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			}
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, plan)
	})
}
