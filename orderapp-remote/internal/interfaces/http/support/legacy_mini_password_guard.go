package support

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Mini password sessions are no longer accepted as bearer credentials. Web
// customer sessions use a separate HttpOnly cookie and are not inspected here.
func LegacyMiniPasswordGuard(check func(context.Context, string) (bool, error)) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if !(strings.HasPrefix(path, "/api/mini/") || strings.HasPrefix(path, "/api/pages/") || path == "/api/customer-recipient/parse") {
				return next(c)
			}
			if CurrentEmployeeID(c) > 0 {
				return next(c)
			}
			auth := strings.TrimSpace(c.Request().Header.Get(echo.HeaderAuthorization))
			if len(auth) < 7 || !strings.EqualFold(auth[:7], "Bearer ") {
				return next(c)
			}
			token := strings.TrimSpace(auth[7:])
			if token == "" {
				return next(c)
			}
			legacy, err := check(c.Request().Context(), token)
			if err != nil {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "登录服务暂不可用，请稍后重试"})
			}
			if legacy {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "登录方式已更新，请使用微信手机号重新登录"})
			}
			return next(c)
		}
	}
}

func NewLegacyMiniPasswordChecker(pool *pgxpool.Pool, schema string) func(context.Context, string) (bool, error) {
	return func(ctx context.Context, token string) (bool, error) {
		var legacy bool
		err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %[1]s.mini_sessions s JOIN %[1]s.mini_users u ON u.id=s.mini_user_id WHERE s.token=$1 AND (u.openid LIKE 'erp-employee:%%' OR u.openid LIKE 'erp-internal-employee:%%'))`, schema), token).Scan(&legacy)
		return legacy, err
	}
}
