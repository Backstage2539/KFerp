package production

import (
	"net/http"
	"strings"

	productionapp "orderapp/internal/application/production"
	"orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

func registerProductionRosterAPI(e *echo.Echo, svc *productionapp.Service) {
	e.GET("/api/production-roster", func(c echo.Context) error {
		result, err := svc.ProductionRosterWeek(c.Request().Context(), productionapp.ProductionRosterQuery{WeekStart: strings.TrimSpace(c.QueryParam("week_start"))})
		if err != nil {
			return scheduleAPIError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})
	e.GET("/api/production-roster/today", func(c echo.Context) error {
		employeeID := support.CurrentEmployeeID(c)
		if strings.TrimSpace(c.QueryParam("scope")) == "all" {
			employeeID = 0
		}
		result, err := svc.ProductionTodayRoster(c.Request().Context(), strings.TrimSpace(c.QueryParam("date")), employeeID)
		if err != nil {
			return scheduleAPIError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})
	for _, route := range []struct {
		path    string
		preview bool
	}{{"/api/production-roster/preview", true}, {"/api/production-roster/save", false}} {
		route := route
		e.POST(route.path, func(c echo.Context) error {
			var cmd productionapp.SaveProductionRosterCommand
			if err := c.Bind(&cmd); err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
			}
			cmd.Operator = support.ActorOf(c)
			result, err := svc.SaveProductionRoster(c.Request().Context(), cmd, route.preview)
			if err != nil {
				return scheduleAPIError(c, err)
			}
			return c.JSON(http.StatusOK, result)
		})
	}
	e.POST("/api/production-roster/handover", func(c echo.Context) error {
		var cmd productionapp.HandoverWorkstationCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		cmd.Operator = support.ActorOf(c)
		result, err := svc.HandoverWorkstation(c.Request().Context(), cmd)
		if err != nil {
			return scheduleAPIError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})
}
