package samples

import (
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	portal "orderapp/internal/application/customerportal"
	messages "orderapp/internal/application/messagecenter"
	app "orderapp/internal/application/samples"
	pg "orderapp/internal/infrastructure/postgres/samples"
	"orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
)

type Portal interface {
	Me(context.Context, string) (portal.CurrentContext, error)
}
type Dependencies struct {
	Service  *app.Service
	Portal   Portal
	Authz    support.AuthzService
	Messages *messages.Service
}
type handler struct{ Dependencies }

func RegisterRoutes(e *echo.Echo, d Dependencies) {
	h := handler{d}
	for _, base := range []string{"/api/mini/sample-requests", "/api/mini/employee/sample-requests", "/api/sample-requests"} {
		e.GET(base, h.list)
		e.GET(base+"/:id", h.get)
		if base == "/api/mini/sample-requests" {
			e.POST(base, h.submit)
		} else {
			e.GET(base+"/options", h.options)
			e.GET(base+"/:id/options", h.options)
			e.POST(base+"/:id/assign", h.assign)
			e.POST(base+"/:id/confirm", h.confirm)
			e.POST(base+"/:id/followup", h.followup)
		}
	}
	e.GET("/api/mini/employee/notifications", h.notifications)
	e.POST("/api/mini/employee/notifications/:id/read", h.readNotification)
}
func (h handler) principal(c echo.Context) (app.Principal, error) {
	var p app.Principal
	if strings.HasPrefix(c.Path(), "/api/mini/") {
		if h.Portal == nil {
			return p, echo.NewHTTPError(503, "登录服务暂不可用")
		}
		cur, err := h.Portal.Me(c.Request().Context(), strings.TrimPrefix(c.Request().Header.Get("Authorization"), "Bearer "))
		if err != nil {
			return p, echo.NewHTTPError(401, "登录已失效，请重新登录")
		}
		p = app.Principal{EmployeeID: cur.EmployeeID, MiniUserID: cur.MiniUserID, Registered: cur.RegistrationComplete, Actor: fmt.Sprintf("mini-user:%d", cur.MiniUserID)}
		if cur.AccountType != "employee" && cur.CurrentCustomerID > 0 && cur.HasCapability(portal.CapabilityBeanList) && cur.HasCapability(portal.CapabilityProductOrder) {
			p.PriceCustomerID = cur.CurrentCustomerID
		}
		if cur.AccountType == "employee" {
			for _, role := range cur.Roles {
				p.Admin = p.Admin || role == "admin"
				p.Sales = p.Sales || role == "sales"
			}
			p.Actor = fmt.Sprintf("employee:%d", cur.EmployeeID)
		}
		staff := strings.HasPrefix(c.Path(), "/api/mini/employee/")
		if staff && !p.Staff() || !staff && (p.EmployeeID > 0 || !p.Registered) {
			return p, echo.NewHTTPError(403, "当前身份无权使用此功能")
		}
	} else {
		a, ok, err := support.CurrentActor(c, h.Authz)
		if err != nil || !ok {
			return p, echo.NewHTTPError(401, "请先登录")
		}
		if a.AccountType == support.AccountTypeChannelCustomer {
			return p, echo.NewHTTPError(403, "需要员工身份")
		}
		p = app.Principal{EmployeeID: a.EmployeeID, Admin: a.IsAdmin(), Actor: support.ActorOf(c)}
		for _, role := range a.Roles {
			p.Sales = p.Sales || role.Code == "sales"
		}
		if !p.Staff() {
			return p, echo.NewHTTPError(403, "需要销售或管理员角色")
		}
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return p, nil
}
func (h handler) result(c echo.Context, v any, err error, status int) error {
	if err == nil {
		return c.JSON(status, v)
	}
	switch {
	case errors.Is(err, app.ErrForbidden):
		return c.JSON(403, map[string]string{"error": err.Error()})
	case errors.Is(err, app.ErrNotFound):
		return c.JSON(404, map[string]string{"error": err.Error()})
	case errors.Is(err, app.ErrConflict):
		return c.JSON(409, map[string]string{"error": err.Error()})
	}
	if msg, known := pg.ErrorMessage(err); known {
		return c.JSON(400, map[string]string{"error": msg})
	}
	c.Logger().Error("sample operation: ", err)
	return c.JSON(500, map[string]string{"error": "样品服务暂不可用，请重试"})
}
func id(c echo.Context) int64 { v, _ := strconv.ParseInt(c.Param("id"), 10, 64); return v }
func (h handler) list(c echo.Context) error {
	p, err := h.principal(c)
	if err != nil {
		return err
	}
	page, _ := strconv.Atoi(c.QueryParam("page"))
	size, _ := strconv.Atoi(c.QueryParam("page_size"))
	user, _ := strconv.ParseInt(c.QueryParam("mini_user_id"), 10, 64)
	v, err := h.Service.List(c.Request().Context(), p, app.Query{Search: c.QueryParam("q"), Status: c.QueryParam("status"), Page: page, PageSize: size, MiniUserID: user})
	return h.result(c, v, err, 200)
}
func (h handler) get(c echo.Context) error {
	p, err := h.principal(c)
	if err != nil {
		return err
	}
	v, err := h.Service.Get(c.Request().Context(), p, id(c))
	return h.result(c, v, err, 200)
}
func bind(c echo.Context, v any) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 128<<10)
	if err := c.Bind(v); err != nil {
		return echo.NewHTTPError(400, "提交资料无效")
	}
	return nil
}
func (h handler) submit(c echo.Context) error {
	p, err := h.principal(c)
	if err != nil {
		return err
	}
	var cmd app.SubmitCommand
	if err = bind(c, &cmd); err != nil {
		return err
	}
	if err = app.ValidateSubmit(cmd); err != nil {
		return h.result(c, nil, err, 400)
	}
	v, err := h.Service.Submit(c.Request().Context(), p, cmd)
	return h.result(c, v, err, 201)
}
func (h handler) assign(c echo.Context) error {
	p, err := h.principal(c)
	if err != nil {
		return err
	}
	if !p.Admin {
		return echo.NewHTTPError(403, "只有管理员可以分配")
	}
	var cmd app.AssignCommand
	if err = bind(c, &cmd); err != nil {
		return err
	}
	v, err := h.Service.Assign(c.Request().Context(), p, id(c), cmd)
	return h.result(c, v, err, 200)
}
func (h handler) confirm(c echo.Context) error {
	p, err := h.principal(c)
	if err != nil {
		return err
	}
	var cmd app.ConfirmCommand
	if err = bind(c, &cmd); err != nil {
		return err
	}
	v, err := h.Service.Confirm(c.Request().Context(), p, id(c), cmd)
	return h.result(c, v, err, 200)
}
func (h handler) followup(c echo.Context) error {
	p, err := h.principal(c)
	if err != nil {
		return err
	}
	var cmd app.FollowupCommand
	if err = bind(c, &cmd); err != nil {
		return err
	}
	v, err := h.Service.Followup(c.Request().Context(), p, id(c), cmd)
	return h.result(c, v, err, 200)
}
func (h handler) options(c echo.Context) error {
	p, err := h.principal(c)
	if err != nil {
		return err
	}
	v, err := h.Service.Options(c.Request().Context(), p, id(c), c.QueryParam("q"))
	return h.result(c, v, err, 200)
}
func (h handler) notifications(c echo.Context) error {
	p, err := h.principal(c)
	if err != nil {
		return err
	}
	if h.Messages == nil {
		return c.JSON(200, map[string]any{"notifications": []any{}})
	}
	v, err := h.Messages.ListNotifications(c.Request().Context(), messages.NotificationQuery{EmployeeID: p.EmployeeID, Status: c.QueryParam("status"), BeforeID: int64(support.IntParam(c, "before_id", 0)), Limit: 100})
	if err != nil {
		return h.result(c, nil, err, 200)
	}
	unread, err := h.Messages.UnreadCount(c.Request().Context(), p.EmployeeID)
	return h.result(c, map[string]any{"notifications": v, "unread_count": unread}, err, 200)
}
func (h handler) readNotification(c echo.Context) error {
	p, err := h.principal(c)
	if err != nil {
		return err
	}
	if h.Messages == nil {
		return echo.NewHTTPError(503, "消息服务暂不可用")
	}
	err = h.Messages.MarkRead(c.Request().Context(), id(c), p.EmployeeID)
	return h.result(c, map[string]bool{"ok": err == nil}, err, 200)
}
