package support

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func BasicAuth(user, pass, schema string, pool *pgxpool.Pool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Path()
			requestPath := c.Request().URL.Path
			if isAuthPublicPath(path) || isAuthPublicPath(requestPath) || path == "/login" || isPublicUnauthenticatedPath(path) || isPublicUnauthenticatedPath(requestPath) {
				return next(c)
			}

			if u, p, ok := c.Request().BasicAuth(); ok {
				if subtle.ConstantTimeCompare([]byte(u), []byte(user)) == 1 && subtle.ConstantTimeCompare([]byte(p), []byte(pass)) == 1 {
					c.Set("actor", u)
					c.Set("basic_auth_admin", true)
					return next(c)
				}
			}

			authz := strings.TrimSpace(c.Request().Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				token := strings.TrimSpace(authz[7:])
				if eid, ename, err := resolveEmployeeBySessionToken(c, pool, schema, token); err == nil && eid > 0 {
					c.Set("employee_id", eid)
					if ename != "" {
						c.Set("operator_employee", ename)
						c.Set("actor", ename)
					}
					return next(c)
				}
			}

			if shouldAdvertiseBasicAuthChallenge(path, requestPath, authz) {
				c.Response().Header().Set("WWW-Authenticate", `Basic realm="orderapp"`)
				return c.NoContent(http.StatusUnauthorized)
			}
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "auth required"})
		}
	}
}

func shouldAdvertiseBasicAuthChallenge(path, requestPath, authz string) bool {
	authz = strings.TrimSpace(authz)
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return false
	}
	return !isAPIPath(path) && !isAPIPath(requestPath)
}

func isAPIPath(path string) bool {
	path = strings.TrimSpace(path)
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/app/api/")
}

func isPublicUnauthenticatedPath(path string) bool {
	return strings.HasPrefix(path, "/api/mini/") ||
		strings.HasPrefix(path, "/public/bean-list/") ||
		strings.HasPrefix(path, "/share/") ||
		strings.HasPrefix(path, "/assets/sales_order_assets/") ||
		path == "/vue-shell" ||
		strings.HasPrefix(path, "/vue-shell/")
}

func isAuthPublicPath(path string) bool {
	switch path {
	case "/api/auth/login", "/api/auth/sms/send", "/api/auth/password/set":
		return true
	default:
		return false
	}
}

func ActorOf(c echo.Context) string {
	if v := c.Get("operator_employee"); v != nil {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	if v := c.Get("actor"); v != nil {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return "unknown"
}
