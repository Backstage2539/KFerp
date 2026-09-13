package customerportal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

const (
	miniappShareImageNeedShowEntranceKey = "miniapp.share_image.need_show_entrance"
	miniappShareScopeKey                 = "miniapp.share.scope"
	miniappShareScopeAdmin               = "admin"
	miniappShareScopeEmployee            = "employee"
	miniappShareScopeAll                 = "all"
)

type EmployeeShareSettingsStore interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, actor, key, value string) error
}

type miniEmployeeShareSettings struct {
	ImageNeedShowEntrance bool   `json:"image_need_show_entrance"`
	ShareScope            string `json:"share_scope"`
}

type miniEmployeeShareSettingsResponse struct {
	Settings  miniEmployeeShareSettings `json:"settings"`
	CanManage bool                      `json:"can_manage"`
}

type miniEmployeeShareSettingsRequest struct {
	ImageNeedShowEntrance *bool   `json:"image_need_show_entrance"`
	ShareScope            *string `json:"share_scope"`
}

type miniappSharePolicyResponse struct {
	Settings miniEmployeeShareSettings `json:"settings"`
	CanShare bool                      `json:"can_share"`
}

func loadMiniEmployeeShareSettings(ctx context.Context, store EmployeeShareSettingsStore) (miniEmployeeShareSettings, error) {
	settings := miniEmployeeShareSettings{
		ImageNeedShowEntrance: true,
		ShareScope:            miniappShareScopeEmployee,
	}
	if store == nil {
		return miniEmployeeShareSettings{}, fmt.Errorf("employee share settings store required")
	}
	imageRaw, imageExists, err := store.Get(ctx, miniappShareImageNeedShowEntranceKey)
	if err != nil {
		return miniEmployeeShareSettings{}, err
	}
	if imageExists {
		parsed, err := strconv.ParseBool(strings.TrimSpace(imageRaw))
		if err != nil {
			return miniEmployeeShareSettings{}, fmt.Errorf("invalid %s value: %w", miniappShareImageNeedShowEntranceKey, err)
		}
		settings.ImageNeedShowEntrance = parsed
	}
	scopeRaw, scopeExists, err := store.Get(ctx, miniappShareScopeKey)
	if err != nil {
		return miniEmployeeShareSettings{}, err
	}
	if scopeExists {
		scope, ok := normalizeMiniappShareScope(scopeRaw)
		if !ok {
			return miniEmployeeShareSettings{}, fmt.Errorf("invalid %s value", miniappShareScopeKey)
		}
		settings.ShareScope = scope
	}
	return settings, nil
}

func normalizeMiniappShareScope(raw string) (string, bool) {
	scope := strings.ToLower(strings.TrimSpace(raw))
	switch scope {
	case miniappShareScopeAdmin, miniappShareScopeEmployee, miniappShareScopeAll:
		return scope, true
	default:
		return "", false
	}
}

func canUseMiniappShare(scope string, current customerportalapp.CurrentContext) bool {
	switch scope {
	case miniappShareScopeAll:
		return true
	case miniappShareScopeEmployee:
		return strings.EqualFold(strings.TrimSpace(current.AccountType), "employee")
	case miniappShareScopeAdmin:
		return strings.EqualFold(strings.TrimSpace(current.AccountType), "employee") && containsMiniRole(current.Roles, "admin")
	default:
		return false
	}
}

func registerMiniEmployeeShareSettingsAPI(e *echo.Echo, portal Service, store EmployeeShareSettingsStore) {
	e.GET("/api/mini/share-settings", func(c echo.Context) error {
		settings, err := loadMiniEmployeeShareSettings(c.Request().Context(), store)
		if err != nil {
			return miniInternalError(c)
		}
		canShare := settings.ShareScope == miniappShareScopeAll
		if !canShare && portal != nil {
			token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
			if token != "" {
				if current, currentErr := portal.Me(c.Request().Context(), token); currentErr == nil {
					canShare = canUseMiniappShare(settings.ShareScope, current)
				}
			}
		}
		return c.JSON(http.StatusOK, miniappSharePolicyResponse{
			Settings: settings,
			CanShare: canShare,
		})
	})

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
		if err := c.Bind(&req); err != nil || (req.ImageNeedShowEntrance == nil) == (req.ShareScope == nil) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "分享设置不正确"})
		}
		if store == nil {
			return miniInternalError(c)
		}

		settings := miniEmployeeShareSettings{
			ImageNeedShowEntrance: true,
			ShareScope:            miniappShareScopeEmployee,
		}
		key := miniappShareImageNeedShowEntranceKey
		value := ""
		if req.ImageNeedShowEntrance != nil {
			settings.ImageNeedShowEntrance = *req.ImageNeedShowEntrance
			value = strconv.FormatBool(*req.ImageNeedShowEntrance)
		} else {
			scope, ok := normalizeMiniappShareScope(*req.ShareScope)
			if !ok {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "分享范围不正确"})
			}
			key = miniappShareScopeKey
			value = scope
			settings.ShareScope = scope
		}
		if err := store.Set(c.Request().Context(), actor, key, value); err != nil {
			return miniInternalError(c)
		}
		return c.JSON(http.StatusOK, miniEmployeeShareSettingsResponse{
			Settings:  settings,
			CanManage: true,
		})
	})
}
