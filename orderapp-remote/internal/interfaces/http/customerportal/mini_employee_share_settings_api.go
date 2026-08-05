package customerportal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

const miniappShareImageNeedShowEntranceKey = "miniapp.share_image.need_show_entrance"

type EmployeeShareSettingsStore interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, actor, key, value string) error
}

type miniEmployeeShareSettings struct {
	ImageNeedShowEntrance bool `json:"image_need_show_entrance"`
}

type miniEmployeeShareSettingsResponse struct {
	Settings  miniEmployeeShareSettings `json:"settings"`
	CanManage bool                      `json:"can_manage"`
}

type miniEmployeeShareSettingsRequest struct {
	ImageNeedShowEntrance *bool `json:"image_need_show_entrance"`
}

func loadMiniEmployeeShareSettings(ctx context.Context, store EmployeeShareSettingsStore) (miniEmployeeShareSettings, error) {
	settings := miniEmployeeShareSettings{ImageNeedShowEntrance: true}
	if store == nil {
		return miniEmployeeShareSettings{}, fmt.Errorf("employee share settings store required")
	}
	raw, ok, err := store.Get(ctx, miniappShareImageNeedShowEntranceKey)
	if err != nil {
		return miniEmployeeShareSettings{}, err
	}
	if !ok {
		return settings, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return miniEmployeeShareSettings{}, fmt.Errorf("invalid %s value: %w", miniappShareImageNeedShowEntranceKey, err)
	}
	settings.ImageNeedShowEntrance = parsed
	return settings, nil
}

func registerMiniEmployeeShareSettingsAPI(e *echo.Echo, portal Service, store EmployeeShareSettingsStore) {
	e.GET("/api/mini/employee/share-settings", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.read")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		actor := miniEmployeeActor(employee)
		c.Set("actor", actor)
		c.Set("employee_id", employee.EmployeeID)
		settings, err := loadMiniEmployeeShareSettings(c.Request().Context(), store)
		if err != nil {
			return miniInternalError(c)
		}
		return c.JSON(http.StatusOK, miniEmployeeShareSettingsResponse{
			Settings:  settings,
			CanManage: containsMiniRole(employee.Roles, "admin") && containsMiniRole(employee.Permissions, "settings.write"),
		})
	})

	e.PUT("/api/mini/employee/share-settings", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "settings.write")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if !containsMiniRole(employee.Roles, "admin") {
			return miniEmployeeAuthError(c, errMiniEmployeeForbidden)
		}
		actor := miniEmployeeActor(employee)
		c.Set("actor", actor)
		c.Set("employee_id", employee.EmployeeID)

		var req miniEmployeeShareSettingsRequest
		if err := c.Bind(&req); err != nil || req.ImageNeedShowEntrance == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "分享图片设置不正确"})
		}
		if store == nil {
			return miniInternalError(c)
		}
		if err := store.Set(
			c.Request().Context(),
			actor,
			miniappShareImageNeedShowEntranceKey,
			strconv.FormatBool(*req.ImageNeedShowEntrance),
		); err != nil {
			return miniInternalError(c)
		}
		return c.JSON(http.StatusOK, miniEmployeeShareSettingsResponse{
			Settings:  miniEmployeeShareSettings{ImageNeedShowEntrance: *req.ImageNeedShowEntrance},
			CanManage: true,
		})
	})
}
