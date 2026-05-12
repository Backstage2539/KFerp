package messagecenter

import (
	"net/http"
	"strconv"
	"strings"

	app "orderapp/internal/application/messagecenter"
	"orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type api struct {
	svc Service
}

func (a api) listNotifications(c echo.Context) error {
	if a.svc == nil {
		return c.JSON(http.StatusOK, map[string]any{"notifications": []app.Notification{}})
	}
	employeeID := support.CurrentEmployeeID(c)
	if employeeID <= 0 {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "employee required"})
	}
	limit := support.IntParam(c, "limit", 20)
	afterID, _ := strconv.ParseInt(strings.TrimSpace(c.QueryParam("after_id")), 10, 64)
	rows, err := a.svc.ListNotifications(c.Request().Context(), app.NotificationQuery{
		EmployeeID: employeeID,
		Channel:    strings.TrimSpace(c.QueryParam("channel")),
		Status:     strings.TrimSpace(c.QueryParam("status")),
		AfterID:    afterID,
		Limit:      limit,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"notifications": rows})
}

func (a api) markRead(c echo.Context) error {
	if a.svc == nil {
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	}
	employeeID := support.CurrentEmployeeID(c)
	if employeeID <= 0 {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "employee required"})
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid notification id"})
	}
	if err := a.svc.MarkRead(c.Request().Context(), id, employeeID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}
