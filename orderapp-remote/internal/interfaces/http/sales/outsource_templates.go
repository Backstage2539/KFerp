package sales

import (
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strings"

	"github.com/labstack/echo/v4"
)

type OutsourceTemplateTier struct {
	ID         int64
	TemplateID int64
	MinQty     int64
	MaxQty     *int64
	Multiplier float64
}

func registerOutsourceSettingsRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	e.GET("/api/outsource/templates", func(c echo.Context) error {
		rows, err := salesSvc.ListOutsourceTemplates(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "rows": rows})
	})

	e.POST("/api/outsource/templates", func(c echo.Context) error {
		var req salesapp.SaveOutsourceTemplateCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid request"})
		}
		if err := salesSvc.SaveOutsourceTemplate(c.Request().Context(), req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/settings/outsource", func(c echo.Context) error {
		target := "/vue-shell?view=outsourceSettings"
		if raw := strings.TrimSpace(c.QueryString()); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, target))
	})

}
