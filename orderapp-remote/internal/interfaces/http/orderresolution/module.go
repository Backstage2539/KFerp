package orderresolution

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	domain "orderapp/internal/domain/orderresolution"
	repo "orderapp/internal/infrastructure/postgres/orderresolution"
	"orderapp/internal/interfaces/http/support"
	"strconv"
)

type Backend interface {
	Context(context.Context, int64) (domain.Context, error)
	Submit(context.Context, int64, domain.Command, string) (repo.Result, error)
	History(context.Context, int64) ([]repo.Result, error)
	SaveDraft(context.Context, int64, domain.Command, string) error
	Draft(context.Context, int64, string) (domain.Command, error)
}

func RegisterRoutes(e *echo.Echo, b Backend, auth support.AuthzService) {
	fail := func(c echo.Context, err error) error {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && (databaseError.Code == "40001" || databaseError.Code == "40P01") {
			err = repo.ErrStale
		}
		status := 400
		if errors.Is(err, repo.ErrStale) {
			status = 409
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	registerPolicyRoutes(e, b, auth, fail)
	authorize := func(c echo.Context, write bool) error {
		a, ok, err := support.CurrentActor(c, auth)
		if err != nil || !ok {
			return echo.NewHTTPError(401, "请先登录")
		}
		if a.AccountType == support.AccountTypeChannelCustomer {
			return echo.NewHTTPError(403, "订单处理仅供内部员工使用")
		}
		allowed := a.Can("orders.resolve")
		if !write {
			allowed = allowed || a.Can("orders.read") || a.Can("finance.resolve") || a.Can("stock.resolve") || a.Can("production.resolve")
		}
		if !allowed {
			return echo.NewHTTPError(403, "没有订单处理权限")
		}
		return nil
	}
	parse := func(c echo.Context) (int64, error) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return 0, fmt.Errorf("订单编号无效")
		}
		return id, nil
	}
	e.GET("/api/orders/:id/resolution", func(c echo.Context) error {
		if err := authorize(c, false); err != nil {
			return err
		}
		id, err := parse(c)
		if err != nil {
			return fail(c, err)
		}
		out, err := b.Context(c.Request().Context(), id)
		if err != nil {
			return fail(c, err)
		}
		history, err := b.History(c.Request().Context(), id)
		if err != nil {
			return fail(c, err)
		}
		draft, err := b.Draft(c.Request().Context(), id, support.ActorOf(c))
		if err != nil {
			return fail(c, err)
		}
		options := domain.OptionsFor(out)
		for i := range options["action"] {
			v := &options["action"][i]
			switch {
			case out.Void:
				v.Disabled = true
				v.Reason = "原单已取消，查看关联修订单"
			case v.Value == "resume" && !out.Paused:
				v.Disabled = true
				v.Reason = "当前订单未暂停"
			case v.Value == "hold" && out.Paused:
				v.Disabled = true
				v.Reason = "当前订单已暂停"
			case (v.Value == "cancel" || v.Value == "amend") && (out.PaidCents > 0 || out.Invoiced || out.Billed):
				v.Disabled = true
				v.Reason = "须先处理收款、开票或账单"
			}
		}
		actor, _, _ := support.CurrentActor(c, auth)
		permissions := map[string]bool{}
		for _, permission := range []string{"orders.resolve", "production.resolve", "stock.resolve", "finance.resolve", "finance.close", "settings.write"} {
			permissions[permission] = actor.Can(permission)
		}
		targets := []repo.Target{}
		if permissions["finance.resolve"] {
			if store, ok := b.(interface {
				Targets(context.Context, int64) ([]repo.Target, error)
			}); ok {
				targets, err = store.Targets(c.Request().Context(), id)
				if err != nil {
					return fail(c, err)
				}
			}
		}
		balances := []repo.Balance{}
		balanceHistory := []repo.BalanceUse{}
		balanceTargets := append([]repo.Target{}, targets...)
		if permissions["finance.resolve"] {
			if store, ok := b.(interface {
				Balances(context.Context, int64) ([]repo.Balance, error)
			}); ok {
				balances, err = store.Balances(c.Request().Context(), id)
				if err != nil {
					return fail(c, err)
				}
			}
			if store, ok := b.(interface {
				BalanceHistory(context.Context, int64) ([]repo.BalanceUse, error)
			}); ok {
				balanceHistory, err = store.BalanceHistory(c.Request().Context(), id)
				if err != nil {
					return fail(c, err)
				}
			}
			pending := false
			for _, r := range history {
				if r.Status == "pending" {
					pending = true
				}
			}
			due := max(int64(0), out.TotalCents-out.CreditCents-max(int64(0), out.PaidCents+out.OffsetCents-out.CreditUsedCents))
			if !out.Void && !pending && due > 0 {
				balanceTargets = append(balanceTargets, repo.Target{ID: id, Label: out.OrderNo, AvailableCents: due})
			}
		}
		return c.JSON(200, map[string]any{"context": out, "options": options, "history": history, "draft": draft, "permissions": permissions, "targets": targets, "balances": balances, "balance_history": balanceHistory, "balance_targets": balanceTargets})
	})
	e.POST("/api/orders/:id/resolution/balance", func(c echo.Context) error {
		if err := authorize(c, false); err != nil {
			return err
		}
		actor, _, err := support.CurrentActor(c, auth)
		if err != nil || !actor.Can("finance.resolve") {
			return echo.NewHTTPError(403, "客户余额须由财务处理")
		}
		id, err := parse(c)
		if err != nil {
			return fail(c, err)
		}
		var cmd repo.BalanceCommand
		if err = c.Bind(&cmd); err != nil {
			return fail(c, fmt.Errorf("请求格式无效"))
		}
		store, ok := b.(interface {
			UseBalance(context.Context, int64, repo.BalanceCommand, string) error
		})
		if !ok {
			return echo.NewHTTPError(503, "客户余额服务不可用")
		}
		if err = store.UseBalance(c.Request().Context(), id, cmd, support.ActorOf(c)); err != nil {
			return fail(c, err)
		}
		return c.JSON(200, map[string]bool{"saved": true})
	})
	e.POST("/api/orders/:id/resolution/:request/refresh", func(c echo.Context) error {
		if err := authorize(c, true); err != nil {
			return err
		}
		id, err := parse(c)
		if err != nil {
			return fail(c, err)
		}
		requestID, err := strconv.ParseInt(c.Param("request"), 10, 64)
		if err != nil || requestID <= 0 {
			return fail(c, fmt.Errorf("处理单编号无效"))
		}
		var cmd domain.Command
		if err = c.Bind(&cmd); err != nil {
			return fail(c, fmt.Errorf("请求格式无效"))
		}
		store, ok := b.(interface {
			Refresh(context.Context, int64, int64, domain.Command, string) (repo.Result, error)
		})
		if !ok {
			return echo.NewHTTPError(503, "更新方案服务不可用")
		}
		result, err := store.Refresh(c.Request().Context(), id, requestID, cmd, support.ActorOf(c))
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(200, result)
	})
	e.POST("/api/orders/:id/resolution/:request/withdraw", func(c echo.Context) error {
		if err := authorize(c, true); err != nil {
			return err
		}
		id, err := parse(c)
		if err != nil {
			return fail(c, err)
		}
		requestID, err := strconv.ParseInt(c.Param("request"), 10, 64)
		if err != nil || requestID <= 0 {
			return fail(c, fmt.Errorf("处理单编号无效"))
		}
		var cmd struct {
			Note string `json:"note"`
		}
		if err = c.Bind(&cmd); err != nil {
			return fail(c, fmt.Errorf("请求格式无效"))
		}
		store, ok := b.(interface {
			Withdraw(context.Context, int64, int64, string, string) (repo.Result, error)
		})
		if !ok {
			return echo.NewHTTPError(503, "撤回服务不可用")
		}
		result, err := store.Withdraw(c.Request().Context(), id, requestID, cmd.Note, support.ActorOf(c))
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(200, result)
	})
	e.POST("/api/orders/:id/resolution/:request/steps/:step", func(c echo.Context) error {
		if err := authorize(c, false); err != nil {
			return err
		}
		id, err := parse(c)
		if err != nil {
			return fail(c, err)
		}
		requestID, e1 := strconv.ParseInt(c.Param("request"), 10, 64)
		stepID, e2 := strconv.ParseInt(c.Param("step"), 10, 64)
		if e1 != nil || e2 != nil || requestID <= 0 || stepID <= 0 {
			return fail(c, fmt.Errorf("处理单或待办编号无效"))
		}
		records, err := b.History(c.Request().Context(), id)
		if err != nil {
			return fail(c, err)
		}
		var step *repo.Step
		for _, record := range records {
			if record.ID == requestID {
				for _, v := range record.Steps {
					if v.ID == stepID {
						copy := v
						step = &copy
					}
				}
			}
		}
		if step == nil {
			return echo.NewHTTPError(404, "待办不存在")
		}
		actor, _, err := support.CurrentActor(c, auth)
		if err != nil || !actor.Can(step.Permission) || (step.Kind == "period" && !actor.Can("finance.close")) {
			return echo.NewHTTPError(403, "没有该项处置的确认权限")
		}
		store, ok := b.(interface {
			CompleteStep(context.Context, int64, int64, int64, repo.StepCommand, string) (repo.Result, error)
		})
		if !ok {
			return echo.NewHTTPError(503, "处置确认服务不可用")
		}
		var cmd repo.StepCommand
		if err = c.Bind(&cmd); err != nil {
			return fail(c, fmt.Errorf("请求格式无效"))
		}
		result, err := store.CompleteStep(c.Request().Context(), id, requestID, stepID, cmd, support.ActorOf(c))
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(200, result)
	})

	for _, action := range []string{"preview", "submit", "draft"} {
		action := action
		e.POST("/api/orders/:id/resolution/"+action, func(c echo.Context) error {
			if err := authorize(c, true); err != nil {
				return err
			}
			id, err := parse(c)
			if err != nil {
				return fail(c, err)
			}
			var cmd domain.Command
			if err = c.Bind(&cmd); err != nil {
				return fail(c, fmt.Errorf("请求格式无效"))
			}
			if action == "draft" {
				err = b.SaveDraft(c.Request().Context(), id, cmd, support.ActorOf(c))
				if err != nil {
					return fail(c, err)
				}
				return c.JSON(200, map[string]bool{"saved": true})
			}
			if action == "submit" {
				out, err := b.Submit(c.Request().Context(), id, cmd, support.ActorOf(c))
				if err != nil {
					return fail(c, err)
				}
				return c.JSON(200, out)
			}
			if previewer, ok := b.(interface {
				Preview(context.Context, int64, domain.Command) (domain.Plan, string, error)
			}); ok {
				plan, version, err := previewer.Preview(c.Request().Context(), id, cmd)
				if err != nil {
					return fail(c, err)
				}
				return c.JSON(200, map[string]any{"plan": plan, "version": version})
			}
			current, err := b.Context(c.Request().Context(), id)
			if err != nil {
				return fail(c, err)
			}
			if cmd.Version != "" && cmd.Version != current.Version {
				return fail(c, repo.ErrStale)
			}
			plan, err := domain.Preview(current, cmd)
			if err != nil {
				return fail(c, err)
			}
			return c.JSON(200, map[string]any{"plan": plan, "version": current.Version})
		})
	}
}
