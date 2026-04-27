package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"

	"github.com/labstack/echo/v4"
)

func registerUnprodSummaryAPI(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/api/produce/unproduced", func(c echo.Context) error {
		data, err := productionSvc.PlanSummary(c.Request().Context(), parseUnprodSummaryQuery(c))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, data)
	})
}
