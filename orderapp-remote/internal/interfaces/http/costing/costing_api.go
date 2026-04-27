package costing

import (
	"net/http"
	appcosting "orderapp/internal/application/costing"
	support "orderapp/internal/interfaces/http/support"
	"strconv"

	"github.com/labstack/echo/v4"
)

func registerCostingAPI(e *echo.Echo, svc Service) {
	e.GET("/api/costing/parameters", func(c echo.Context) error {
		params, err := svc.Parameters(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, params)
	})

	e.POST("/api/costing/calculate", func(c echo.Context) error {
		var req appcosting.CalculateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		resp, err := svc.Calculate(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.GET("/api/costing/bean-list", func(c echo.Context) error {
		resp, err := svc.BeanList(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.POST("/api/costing/runs", func(c echo.Context) error {
		run, err := svc.CreateRun(c.Request().Context(), support.ActorOf(c))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, run)
	})

	e.POST("/api/costing/runs/:id/publish", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
		}
		if err := svc.PublishRun(c.Request().Context(), support.ActorOf(c), id); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "id": id})
	})
}
