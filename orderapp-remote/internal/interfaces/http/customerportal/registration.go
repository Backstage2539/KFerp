package customerportal

import (
	"context"
	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerportal"
	"orderapp/internal/interfaces/http/support"
	"strconv"
)

type registrationService interface {
	UpdateRegistration(context.Context, string, string) (app.CurrentContext, error)
	ListRegistrations(context.Context, string, int, int) ([]app.RegistrationProfile, int, error)
	DisableRegistration(context.Context, int64, string) error
}

func registerRegistrationAPI(e *echo.Echo, svc Service, authz support.AuthzService) {
	admin := func(c echo.Context, permission string) error {
		a, ok, err := support.CurrentActor(c, authz)
		if err != nil || !ok || a.AccountType == support.AccountTypeChannelCustomer || !a.Can(permission) {
			return echo.NewHTTPError(403, "需要客户管理权限")
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return nil
	}
	e.POST("/api/mini/registration", func(c echo.Context) error {
		if svc == nil {
			return miniInternalError(c)
		}
		var req miniLoginRequest
		if c.Bind(&req) != nil {
			return c.JSON(400, map[string]string{"error": "登记资料无效"})
		}
		if !app.ValidRegistrationNickname(req.Nickname) {
			return c.JSON(400, map[string]string{"error": "请填写 1 至 32 字的昵称"})
		}
		result, err := svc.Login(c.Request().Context(), app.LoginCommand{Mode: "register", Code: req.Code, PhoneCode: req.PhoneCode, Nickname: req.Nickname})
		if err != nil {
			return miniLoginError(c, err)
		}
		return c.JSON(200, result)
	})
	e.PUT("/api/mini/registration", func(c echo.Context) error {
		s, ok := svc.(registrationService)
		if !ok {
			return miniInternalError(c)
		}
		var req struct {
			Nickname string `json:"nickname"`
		}
		if c.Bind(&req) != nil {
			return c.JSON(400, map[string]string{"error": "昵称格式无效"})
		}
		result, err := s.UpdateRegistration(c.Request().Context(), miniTokenFromHeader(c.Request().Header.Get("Authorization")), req.Nickname)
		if err != nil {
			return miniBusinessError(c, err)
		}
		return c.JSON(200, result)
	})
	e.GET("/api/customer-portal/admin/registrations", func(c echo.Context) error {
		if err := admin(c, "customers.read"); err != nil {
			return err
		}
		s, ok := svc.(registrationService)
		if !ok {
			return miniInternalError(c)
		}
		page, _ := strconv.Atoi(c.QueryParam("page"))
		size, _ := strconv.Atoi(c.QueryParam("page_size"))
		if page < 1 {
			page = 1
		}
		if size < 1 || size > 100 {
			size = 30
		}
		rows, total, err := s.ListRegistrations(c.Request().Context(), c.QueryParam("q"), page, size)
		if err != nil {
			return miniInternalError(c)
		}
		return c.JSON(200, map[string]any{"rows": rows, "total": total, "page": page, "page_size": size})
	})
	e.POST("/api/customer-portal/admin/registrations/:id/disable", func(c echo.Context) error {
		if err := admin(c, "customers.write"); err != nil {
			return err
		}
		s, ok := svc.(registrationService)
		if !ok {
			return miniInternalError(c)
		}
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		if err := s.DisableRegistration(c.Request().Context(), id, support.ActorOf(c)); err != nil {
			return miniBusinessError(c, err)
		}
		return c.JSON(200, map[string]bool{"ok": true})
	})
}
