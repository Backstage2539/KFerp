package customerportal

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
	"strings"
)

func registerMiniLoginOptionsAPI(e *echo.Echo, store EmployeeShareSettingsStore, disabled bool) {
	e.GET("/api/mini/login-options", func(c echo.Context) error {
		visible := false
		if !disabled && store != nil {
			raw, ok, err := store.Get(c.Request().Context(), "miniapp.login.show_password")
			if err == nil && ok {
				visible, _ = strconv.ParseBool(strings.TrimSpace(raw))
			}
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(http.StatusOK, map[string]bool{"show_password_login": visible})
	})
}
