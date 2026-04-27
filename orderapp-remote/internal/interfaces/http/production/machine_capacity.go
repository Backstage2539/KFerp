package production

import (
	"net/http"
	productionapp "orderapp/internal/application/production"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type RoastMachine = productionapp.RoastMachine

func registerMachineCapacityPages(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/produce/machines", func(c echo.Context) error {
		return support.VueShellRedirect(c, "machines")
	})
	e.GET("/api/produce/machines", func(c echo.Context) error {
		rows, err := productionSvc.ListMachines(c.Request().Context(), false)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/produce/machines", func(c echo.Context) error {
		var req productionapp.RoastMachineCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if err := productionSvc.SaveMachine(c.Request().Context(), req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}
