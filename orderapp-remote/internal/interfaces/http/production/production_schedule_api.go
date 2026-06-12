package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"
	"orderapp/internal/interfaces/http/support"
	"strings"

	"github.com/labstack/echo/v4"
)

type scheduleAssignmentRequest struct {
	WorkOrderID    int64  `json:"work_order_id"`
	JobCardID      int64  `json:"job_card_id"`
	WorkCenter     string `json:"work_center"`
	PlannedStartAt string `json:"planned_start_at"`
	PlannedEndAt   string `json:"planned_end_at"`
	ShiftCode      string `json:"shift_code"`
	AssignedTo     string `json:"assigned_to"`
	Priority       int    `json:"priority"`
	Note           string `json:"note"`
}

type capacityCalendarRequest struct {
	ID               int64  `json:"id"`
	WorkCenter       string `json:"work_center"`
	WorkDate         string `json:"work_date"`
	ShiftCode        string `json:"shift_code"`
	AvailableMinutes int    `json:"available_minutes"`
	DowntimeMinutes  int    `json:"downtime_minutes"`
	Note             string `json:"note"`
}

func registerProductionScheduleAPI(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/api/production-schedule", func(c echo.Context) error {
		limit := support.IntParam(c, "limit", 200)
		rows, err := productionSvc.ScheduleBoard(c.Request().Context(), productionapp.ScheduleBoardQuery{
			From:       strings.TrimSpace(c.QueryParam("from")),
			To:         strings.TrimSpace(c.QueryParam("to")),
			WorkCenter: strings.TrimSpace(c.QueryParam("work_center")),
			Status:     strings.TrimSpace(c.QueryParam("status")),
			Limit:      limit,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/mrp/suggestions", func(c echo.Context) error {
		limit := support.IntParam(c, "limit", 50)
		rows, err := productionSvc.MRPSuggestions(c.Request().Context(), productionapp.MRPSuggestionQuery{
			From:       strings.TrimSpace(c.QueryParam("from")),
			To:         strings.TrimSpace(c.QueryParam("to")),
			Status:     strings.TrimSpace(c.QueryParam("status")),
			WorkCenter: strings.TrimSpace(c.QueryParam("work_center")),
			MaterialID: parseInt64(c.QueryParam("material_id")),
			Limit:      limit,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/production-trace/analytics", func(c echo.Context) error {
		limit := support.IntParam(c, "limit", 50)
		rows, err := productionSvc.ProductionTraceAnalytics(c.Request().Context(), productionapp.ProductionTraceAnalyticsQuery{
			WorkOrderID: parseInt64(c.QueryParam("work_order_id")),
			BatchID:     strings.TrimSpace(c.QueryParam("batch_id")),
			Limit:       limit,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.POST("/api/production-schedule/assign", func(c echo.Context) error {
		var req scheduleAssignmentRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		res, err := productionSvc.SaveScheduleAssignment(c.Request().Context(), productionapp.ScheduleAssignmentCommand{
			WorkOrderID:    req.WorkOrderID,
			JobCardID:      req.JobCardID,
			WorkCenter:     req.WorkCenter,
			PlannedStartAt: req.PlannedStartAt,
			PlannedEndAt:   req.PlannedEndAt,
			ShiftCode:      req.ShiftCode,
			AssignedTo:     req.AssignedTo,
			Priority:       req.Priority,
			Note:           req.Note,
			Operator:       support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, res)
	})

	e.POST("/api/production-capacity-calendar", func(c echo.Context) error {
		var req capacityCalendarRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := productionSvc.SaveCapacityCalendar(c.Request().Context(), productionapp.CapacityCalendarCommand{
			ID:               req.ID,
			WorkCenter:       req.WorkCenter,
			WorkDate:         req.WorkDate,
			ShiftCode:        req.ShiftCode,
			AvailableMinutes: req.AvailableMinutes,
			DowntimeMinutes:  req.DowntimeMinutes,
			Note:             req.Note,
			Operator:         support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/production-schedule", func(c echo.Context) error {
		target := "/vue-shell?view=productionSchedule"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})
}
