package main

import (
	"net/http"
	"net/url"
	"strings"

	productionapp "orderapp/internal/application/production"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type ProduceStartAPIRequest struct {
	From       string           `json:"from"`
	To         string           `json:"to"`
	CustomerID int64            `json:"customer_id"`
	Selected   []string         `json:"selected"`
	InputByKey map[string]int64 `json:"input_by_key"`
}

type ProduceStartAPIResponse struct {
	OK      bool   `json:"ok"`
	BatchID string `json:"batch_id"`
}

type ProduceRunningAPIResponse struct {
	Rows []ProduceRunningAPIRow `json:"rows"`
}

type ProduceRunningAPIRow struct {
	ID           int64   `json:"id"`
	BatchID      string  `json:"batch_id"`
	ProductID    int64   `json:"product_id"`
	ProductName  string  `json:"product_name"`
	SpecG        int64   `json:"spec_g"`
	NeedG        int64   `json:"need_g"`
	InputG       int64   `json:"input_g"`
	BomYieldRate float64 `json:"bom_yield_rate"`
	PlanUnits    int64   `json:"plan_units"`
	PlanLooseG   int64   `json:"plan_loose_g"`
	OrderNos     string  `json:"order_nos"`
	StartedBy    string  `json:"started_by"`
	StartedAt    string  `json:"started_at"`
}

type ProduceRunningFinishAPIRequest struct {
	ID             int64 `json:"id"`
	FinishedUnits  int64 `json:"finished_units"`
	FinishedLooseG int64 `json:"finished_loose_g"`
}

type ProduceRunningCancelAPIRequest struct {
	ID int64 `json:"id"`
}

type ProduceRunningActionAPIResponse struct {
	OK bool `json:"ok"`
}

func registerProductionFlowPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	productionSvc := productionapp.NewService(postgresProductionRepository{pool: pool, schema: schema})

	e.POST("/api/produce/start", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req ProduceStartAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if len(req.Selected) == 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请先生成计划并选择项目"})
		}
		selected := map[string]bool{}
		for _, key := range req.Selected {
			key = strings.TrimSpace(key)
			if key != "" {
				selected[key] = true
			}
		}
		res, err := productionSvc.Start(c.Request().Context(), productionapp.StartCommand{
			From:       req.From,
			To:         req.To,
			CustomerID: req.CustomerID,
			Selected:   selected,
			InputByKey: req.InputByKey,
			Operator:   actorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, ProduceStartAPIResponse{OK: true, BatchID: res.BatchID})
	})

	e.GET("/produce/running", func(c echo.Context) error {
		target := "/vue-shell?view=produceRunning"
		if strings.TrimSpace(c.QueryParam("ok")) == "1" {
			target += "&ok=1"
		}
		if errText := strings.TrimSpace(c.QueryParam("err")); errText != "" {
			target += "&err=" + url.QueryEscape(errText)
		}
		return c.Redirect(http.StatusSeeOther, target)
	})

	e.GET("/api/produce/running", func(c echo.Context) error {
		rows, err := productionSvc.ListRunning(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, ProduceRunningAPIResponse{Rows: produceRunningAPIRows(rows)})
	})

	e.POST("/api/produce/running/finish", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req ProduceRunningFinishAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if req.ID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		if err := productionSvc.Finish(c.Request().Context(), productionapp.FinishCommand{
			ID:               req.ID,
			FinishedUnits:    req.FinishedUnits,
			FinishedLooseG:   req.FinishedLooseG,
			HasFinishedInput: true,
			Operator:         actorOf(c),
		}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, ProduceRunningActionAPIResponse{OK: true})
	})

	e.POST("/api/produce/running/cancel", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req ProduceRunningCancelAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if req.ID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		if err := productionSvc.Cancel(c.Request().Context(), productionapp.CancelCommand{ID: req.ID, Operator: actorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, ProduceRunningActionAPIResponse{OK: true})
	})
}

func produceRunningAPIRows(rows []productionapp.RunningItem) []ProduceRunningAPIRow {
	out := make([]ProduceRunningAPIRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProduceRunningAPIRow{
			ID:           r.ID,
			BatchID:      r.BatchID,
			ProductID:    r.ProductID,
			ProductName:  r.ProductName,
			SpecG:        r.SpecG,
			NeedG:        r.NeedG,
			InputG:       r.InputG,
			BomYieldRate: r.BomYieldRate,
			PlanUnits:    r.PlanUnits,
			PlanLooseG:   r.PlanLooseG,
			OrderNos:     r.OrderNos,
			StartedBy:    r.StartedBy,
			StartedAt:    r.StartedAt,
		})
	}
	return out
}
