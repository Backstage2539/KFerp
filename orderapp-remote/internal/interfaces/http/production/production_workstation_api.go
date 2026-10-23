package production

import (
	"errors"
	"net/http"
	productionapp "orderapp/internal/application/production"
	"orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type workstationTaskIssueRequest struct {
	ExceptionReason string `json:"exception_reason"`
	Note            string `json:"note"`
}

func registerProductionWorkstationAPI(e *echo.Echo, productionSvc *productionapp.Service) {
	e.POST("/api/production/workstation/material-issue/preview", func(c echo.Context) error {
		var cmd productionapp.BatchMaterialIssueCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(400, ErrorResponse{Error: "invalid request"})
		}
		cmd.Operator = support.ActorOf(c)
		result, err := productionSvc.PreviewBatchMaterialIssue(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(400, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(200, result)
	})
	e.POST("/api/production/workstation/material-issue/submit", func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(400, ErrorResponse{Error: err.Error()})
		}
		var cmd productionapp.BatchMaterialIssueCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(400, ErrorResponse{Error: "invalid request"})
		}
		cmd.Operator = support.ActorOf(c)
		result, err := productionSvc.SubmitBatchMaterialIssue(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(400, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(200, result)
	})

	e.GET("/production-overview", func(c echo.Context) error {
		target := "/vue-shell?view=workstationView"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})
	e.GET("/production-workstations", func(c echo.Context) error {
		target := "/vue-shell?view=workstationView"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})
	e.GET("/api/production/workstation-overview", func(c echo.Context) error {
		rows, err := productionSvc.ProductionWorkstationOverview(c.Request().Context(), productionapp.ProductionWorkstationOverviewQuery{
			Limit:      support.IntParam(c, "limit", 500),
			EmployeeID: support.CurrentEmployeeID(c),
			Scope:      strings.TrimSpace(c.QueryParam("scope")),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})
	e.POST("/api/production/workstation/tasks/:id/exception", func(c echo.Context) error {
		id, err := parseJobCardID(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req workstationTaskIssueRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		reason := strings.TrimSpace(req.ExceptionReason)
		if reason == "" {
			reason = strings.TrimSpace(req.Note)
		}
		if reason == "" {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "exception_reason required"})
		}
		res, err := productionSvc.PauseJobCard(c.Request().Context(), productionapp.JobCardActionCommand{
			ID:              id,
			Operator:        support.ActorOf(c),
			ExceptionReason: reason,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "job_card": res.JobCard, "work_order": res.WorkOrder})
	})
	e.POST("/api/production/workstation/tasks/:id/material-call", func(c echo.Context) error {
		id, err := parseJobCardID(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req workstationTaskIssueRequest
		if c.Request().Body != nil && c.Request().ContentLength != 0 {
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			}
		}
		reason := "呼叫补料"
		if note := strings.TrimSpace(req.Note); note != "" {
			reason += ": " + note
		}
		res, err := productionSvc.PauseJobCard(c.Request().Context(), productionapp.JobCardActionCommand{
			ID:              id,
			Operator:        support.ActorOf(c),
			ExceptionReason: reason,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "job_card": res.JobCard, "work_order": res.WorkOrder})
	})
	e.POST("/api/production/workstation/tasks/:id/claim", func(c echo.Context) error {
		return c.JSON(http.StatusGone, ErrorResponse{Error: "任务已改为跟随当日工位排班，无需领取"})
	})
}

func parseJobCardID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid job_card_id")
	}
	return id, nil
}
