package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"
	support "orderapp/internal/interfaces/http/support"
	"strings"

	"github.com/labstack/echo/v4"
)

func registerWorkOrderAPI(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/produce/work-orders", func(c echo.Context) error {
		target := "/vue-shell?view=workOrders"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, target)
	})
	e.GET("/produce/job-cards", func(c echo.Context) error {
		target := "/vue-shell?view=jobCards"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, target)
	})
	e.GET("/produce/costs", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, "/vue-shell?view=productionCosts")
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
	e.GET("/api/produce/costs", func(c echo.Context) error {
		rows, err := productionSvc.ListBatchCosts(c.Request().Context(), productionapp.BatchCostQuery{Limit: support.IntParam(c, "limit", 200)})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
}
