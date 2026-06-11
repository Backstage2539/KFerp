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

func registerProductionPlanAPI(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/api/production-plans", func(c echo.Context) error {
		rows, err := productionSvc.ListProductionPlans(c.Request().Context(), productionapp.ProductionPlanQuery{
			Status: strings.TrimSpace(c.QueryParam("status")),
			Limit:  support.IntParam(c, "limit", 200),
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
}
