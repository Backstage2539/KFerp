package pageentry

import (
	"orderapp/internal/infrastructure/wechatweb"

	"github.com/labstack/echo/v4"

	"net/http"
	"net/url"
	portal "orderapp/internal/application/customerportal"
	app "orderapp/internal/application/pageentry"
	repo "orderapp/internal/infrastructure/postgres/pageentry"

	"strings"
)

type Config = wechatweb.Config
type OAuthIdentity = wechatweb.OAuthIdentity

func ConfigFromEnv() Config { return wechatweb.ConfigFromEnv() }

const sessionCookie = "__Host-kferp_page"
const stateCookie = "__Host-kferp_page_oauth"

func cookie(c echo.Context, name string) string {
	v, err := c.Cookie(name)
	if err != nil {
		return ""
	}
	return v.Value
}
func setCookie(c echo.Context, name, value string, age int) {
	c.SetCookie(&http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func sameOrigin(c echo.Context) bool {
	raw := c.Request().Header.Get("Origin")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != c.Request().Host || u.User != nil || u.Path != "" {
		return false
	}
	return c.Request().Header.Get("Sec-Fetch-Site") != "cross-site"
}
func (h *Handler) webContext(c echo.Context) (portal.CurrentContext, repo.Session, error) {
	s, err := h.Repo.Session(c.Request().Context(), cookie(c, sessionCookie))
	if err != nil {
		return portal.CurrentContext{}, s, err
	}
	if s.MiniToken != "" {
		cur, err := h.Portal.Me(c.Request().Context(), s.MiniToken)
		if cur.AccountType == "employee" || (s.CustomerID > 0 && cur.CurrentCustomerID != s.CustomerID) {
			err = app.ErrDenied
		}
		if s.CustomerID == 0 && len(cur.Bindings) > 1 {
			cur.CurrentCustomerID = 0
			cur.CurrentCustomerName = ""
			cur.Capabilities = nil
		}
		return cur, s, err
	}
	b, err := h.Official.Binding(c.Request().Context(), s.AppID, s.OpenID)
	if err != nil || !b.Active || b.MiniUserID != s.UserID || s.BoundAt == nil || !b.BoundAt.Equal(*s.BoundAt) {
		return portal.CurrentContext{}, s, app.ErrDenied
	}
	cur, err := h.Repo.Portal.OfficialAccountContext(c.Request().Context(), s.UserID, s.CustomerID, *s.BoundAt)
	return cur, s, err
}
func (h *Handler) current(c echo.Context) (portal.CurrentContext, error) {
	auth := c.Request().Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		cur, err := h.Portal.Me(c.Request().Context(), strings.TrimPrefix(auth, "Bearer "))
		if cur.AccountType == "employee" {
			err = app.ErrDenied
		}
		return cur, err
	}
	cur, _, err := h.webContext(c)
	return cur, err
}
func (h *Handler) authStatus(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	cur, _, err := h.webContext(c)
	if err != nil {
		cur = portal.CurrentContext{}
	}
	return c.JSON(200, map[string]any{"authenticated": err == nil, "customer": cur, "oauth_ready": h.Config.Ready()})
}
func (h *Handler) login(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	if !sameOrigin(c) {
		return fail(c, 403, "请从本站页面登录")
	}
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 8192)
	if c.Bind(&req) != nil {
		return fail(c, 400, "请输入账号密码")
	}
	for _, id := range []string{"ip:" + c.RealIP(), "account:" + strings.ToLower(strings.TrimSpace(req.Login))} {
		ok, err := h.Repo.AllowLogin(c.Request().Context(), id)
		if err != nil {
			return respond(c, nil, err)
		}
		if !ok {
			return fail(c, 429, "尝试次数过多，请 5 分钟后重试")
		}
	}
	result, err := h.Portal.LoginWithPassword(c.Request().Context(), portal.PasswordLoginCommand{Login: req.Login, Password: req.Password})
	if err != nil || result.AccountType == "employee" || len(result.Bindings) == 0 {
		return fail(c, 401, "账号密码无效或尚未完成客户认证")
	}
	selectedCustomer := result.CurrentCustomerID
	if len(result.Bindings) > 1 {
		selectedCustomer = 0
	}
	token, err := h.Repo.NewSession(c.Request().Context(), repo.Session{MiniToken: result.Token, UserID: result.MiniUserID, CustomerID: selectedCustomer})
	if err != nil {
		return respond(c, nil, err)
	}
	_ = h.Repo.EndSession(c.Request().Context(), cookie(c, sessionCookie))
	setCookie(c, sessionCookie, token, 7*24*3600)
	return c.JSON(200, map[string]bool{"ok": true})
}
func (h *Handler) logout(c echo.Context) error {
	if !sameOrigin(c) {
		return fail(c, 403, "请从本站操作")
	}
	err := h.Repo.EndSession(c.Request().Context(), cookie(c, sessionCookie))
	setCookie(c, sessionCookie, "", -1)
	return respond(c, map[string]bool{"ok": true}, err)
}
func (h *Handler) selectCustomer(c echo.Context) error {
	if !sameOrigin(c) {
		return fail(c, 403, "请从本站操作")
	}
	cur, s, err := h.webContext(c)
	if err != nil {
		return fail(c, 401, "请先登录")
	}
	var req struct {
		CustomerID int64 `json:"customer_id"`
	}
	if c.Bind(&req) != nil {
		return fail(c, 400, "请选择客户")
	}
	found := false
	for _, b := range cur.Bindings {
		if b.CustomerID == req.CustomerID && b.Status == "approved" {
			found = true
		}
	}
	if !found {
		return fail(c, 403, "客户认证不可用")
	}
	if s.MiniToken != "" {
		_, err = h.Portal.SwitchCurrentCustomer(c.Request().Context(), s.MiniToken, req.CustomerID)
		if err == nil {
			err = h.Repo.SelectCustomer(c.Request().Context(), cookie(c, sessionCookie), req.CustomerID)
		}
	} else {
		live, e := h.Repo.Portal.OfficialAccountContext(c.Request().Context(), s.UserID, req.CustomerID, *s.BoundAt)
		err = e
		if live.CurrentCustomerID != req.CustomerID {
			err = app.ErrDenied
		}
		if err == nil {
			err = h.Repo.SelectCustomer(c.Request().Context(), cookie(c, sessionCookie), req.CustomerID)
		}
	}
	return respond(c, map[string]bool{"ok": err == nil}, err)
}
func (h *Handler) oauthStart(c echo.Context) error {
	key := c.QueryParam("entry")
	if !app.ValidKey(key) {
		return fail(c, 400, "页面地址无效")
	}
	if !h.Config.Ready() {
		return c.Redirect(303, "/app/p/"+key+"?auth=password")
	}
	e, err := h.Repo.Entry(c.Request().Context(), key)
	if err != nil || !e.Enabled || e.Deleted || e.Published == nil {
		return respond(c, nil, app.ErrUnavailable)
	}
	state, err := app.NewKey()
	if err != nil {
		return respond(c, nil, err)
	}
	browser, err := app.NewKey()
	if err != nil {
		return respond(c, nil, err)
	}
	if err = h.Repo.OAuthState(c.Request().Context(), state, browser, key); err != nil {
		return respond(c, nil, err)
	}
	setCookie(c, stateCookie, browser, 300)
	q := url.Values{"appid": {h.Config.AppID}, "redirect_uri": {h.Config.PublicOrigin + "/app/api/page-auth/callback"}, "response_type": {"code"}, "scope": {"snsapi_base"}, "state": {state}}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Redirect(303, "https://open.weixin.qq.com/connect/oauth2/authorize?"+q.Encode()+"#wechat_redirect")
}
func (h *Handler) oauthCallback(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	key, err := h.Repo.ConsumeState(c.Request().Context(), c.QueryParam("state"), cookie(c, stateCookie))
	setCookie(c, stateCookie, "", -1)
	if err != nil || !app.ValidKey(key) {
		return fail(c, 400, "授权已失效，请返回原页面重试或使用账号密码登录")
	}
	fallback := func() error { return c.Redirect(303, "/app/p/"+key+"?auth=password") }
	if !h.Config.Ready() || c.QueryParam("code") == "" {
		return fallback()
	}
	identity, err := h.Config.ExchangeCode(c.Request().Context(), c.QueryParam("code"))
	if err != nil {
		return fallback()
	}
	b, err := h.Official.Binding(c.Request().Context(), h.Config.AppID, identity.OpenID)
	if repo.IsMissing(err) && h.Config.UnionIDEnabled {
		b, err = h.Official.AutoBind(c.Request().Context(), h.Config.AppID, identity.OpenID, identity.UnionID)
	}
	if err != nil || !b.Active {
		return fallback()
	}
	cur, err := h.Repo.Portal.OfficialAccountContext(c.Request().Context(), b.MiniUserID, 0, b.BoundAt)
	if err != nil || len(cur.Bindings) == 0 {
		return fallback()
	}
	token, err := h.Repo.NewSession(c.Request().Context(), repo.Session{UserID: b.MiniUserID, CustomerID: cur.CurrentCustomerID, AppID: h.Config.AppID, OpenID: identity.OpenID, BoundAt: &b.BoundAt})
	if err != nil {
		return fallback()
	}
	_ = h.Repo.EndSession(c.Request().Context(), cookie(c, sessionCookie))
	setCookie(c, sessionCookie, token, 7*24*3600)
	return c.Redirect(303, "/app/p/"+key+"?auth=done")
}
