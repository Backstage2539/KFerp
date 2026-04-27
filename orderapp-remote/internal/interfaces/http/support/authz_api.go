package support

import (
	"context"
	"net/http"

	authzapp "orderapp/internal/application/authz"

	"github.com/labstack/echo/v4"
)

type AuthzService interface {
	ActorByEmployeeID(ctx context.Context, employeeID int64) (authzapp.Actor, error)
	ListRoles(ctx context.Context) ([]authzapp.Role, error)
	ListEmployeeRoles(ctx context.Context) (map[int64][]string, error)
	AssignEmployeeRoles(ctx context.Context, cmd authzapp.AssignmentCommand) error
}

func registerAuthzAPI(e *echo.Echo, authz AuthzService) {
	e.GET("/api/auth/me", func(c echo.Context) error {
		actor, ok, err := CurrentActor(c, authz)
		if err != nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		if !ok {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "auth required"})
		}
		return c.JSON(http.StatusOK, actor)
	})

	e.GET("/api/auth/roles", func(c echo.Context) error {
		if err := requireCurrentPermission(c, authz, "auth.manage"); err != nil {
			return err
		}
		roles, err := authz.ListRoles(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"roles": roles})
	})

	e.GET("/api/auth/employee-roles", func(c echo.Context) error {
		if err := requireCurrentPermission(c, authz, "auth.manage"); err != nil {
			return err
		}
		assignments, err := authz.ListEmployeeRoles(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"assignments": assignments})
	})

	e.POST("/api/auth/employee-roles", func(c echo.Context) error {
		if err := requireCurrentPermission(c, authz, "auth.manage"); err != nil {
			return err
		}
		var cmd authzapp.AssignmentCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		if err := authz.AssignEmployeeRoles(c.Request().Context(), cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}

func CurrentActor(c echo.Context, authz AuthzService) (authzapp.Actor, bool, error) {
	if isBasicAuthAdmin(c) {
		return authzapp.Actor{Name: ActorOf(c), BasicAuthAdmin: true}, true, nil
	}
	if authz == nil {
		return authzapp.Actor{}, false, nil
	}
	employeeID := currentEmployeeID(c)
	if employeeID <= 0 {
		return authzapp.Actor{}, false, nil
	}
	actor, err := authz.ActorByEmployeeID(c.Request().Context(), employeeID)
	if err != nil {
		return authzapp.Actor{}, true, err
	}
	return actor, true, nil
}

func requireCurrentPermission(c echo.Context, authz AuthzService, permission string) error {
	actor, ok, err := CurrentActor(c, authz)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "auth required"})
	}
	if !actor.Can(permission) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "permission denied", "permission": permission})
	}
	return nil
}

func isBasicAuthAdmin(c echo.Context) bool {
	if v := c.Get("basic_auth_admin"); v != nil {
		if ok, _ := v.(bool); ok {
			return true
		}
	}
	return false
}
