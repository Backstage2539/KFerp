package orderresolution

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	domain "orderapp/internal/domain/orderresolution"
	repo "orderapp/internal/infrastructure/postgres/orderresolution"
	"orderapp/internal/interfaces/http/support"
)

func registerPolicyRoutes(e *echo.Echo, b Backend, auth support.AuthzService, fail func(echo.Context, error) error) {
	for _, method := range []string{"GET", "POST"} {
		e.Add(method, "/api/settings/order-resolution", func(c echo.Context) error {
			actor, ok, err := support.CurrentActor(c, auth)
			if err != nil || !ok {
				return echo.NewHTTPError(401, "请先登录")
			}
			if actor.AccountType == support.AccountTypeChannelCustomer || !actor.Can("settings.write") {
				return echo.NewHTTPError(403, "没有订单处理配置权限")
			}
			store, ok := b.(interface {
				Policy(context.Context) (repo.PolicyRecord, error)
				SavePolicy(context.Context, int64, domain.Policy, string) (repo.PolicyRecord, error)
			})
			if !ok {
				return echo.NewHTTPError(503, "配置服务不可用")
			}
			var out repo.PolicyRecord
			if c.Request().Method == "GET" {
				out, err = store.Policy(c.Request().Context())
			} else {
				var cmd repo.PolicyRecord
				if err = c.Bind(&cmd); err != nil {
					return fail(c, fmt.Errorf("配置格式无效"))
				}
				out, err = store.SavePolicy(c.Request().Context(), cmd.Revision, cmd.Policy, support.ActorOf(c))
			}
			if err != nil {
				return fail(c, err)
			}
			return c.JSON(200, map[string]any{"revision": out.Revision, "policy": out.Policy, "options": domain.Options()})
		})
	}
}
