package sales

import (
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type SenderProfile = salesapp.SenderProfile

func registerSenderSettingsPage(e *echo.Echo, salesSvc *salesapp.Service) {
	e.GET("/settings/sender", func(c echo.Context) error {
		return support.VueShellRedirect(c, "senderSettings")
	})
	e.GET("/api/settings/sender", func(c echo.Context) error {
		profile, err := salesSvc.LoadSenderProfile(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"profile": profile})
	})
	e.POST("/api/settings/sender", func(c echo.Context) error {
		var p SenderProfile
		if err := c.Bind(&p); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
		}
		if err := salesSvc.SaveSenderProfile(c.Request().Context(), p); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}
