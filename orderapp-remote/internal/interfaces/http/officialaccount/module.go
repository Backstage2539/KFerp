package officialaccount

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/url"
	portal "orderapp/internal/application/customerportal"
	app "orderapp/internal/application/officialaccount"
	repo "orderapp/internal/infrastructure/postgres/officialaccount"
	pagepg "orderapp/internal/infrastructure/postgres/pageentry"
	pageconfig "orderapp/internal/infrastructure/wechatweb"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Handler struct {
	Config Config
	Repo   repo.Repository
	Portal *portal.Service
	Wechat *WechatClient
	Authz  support.AuthzService
	menuMu sync.Mutex
}

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, p *portal.Service, authz support.AuthzService, cfg Config) {
	h := &Handler{Config: cfg, Repo: repo.NewRepository(pool, schema), Portal: p, Wechat: &WechatClient{Config: cfg}, Authz: authz}
	e.GET("/api/wechat/official-account/callback", h.Callback)
	e.POST("/api/wechat/official-account/callback", h.Callback)
	e.GET("/api/mini/price-table-entries/:key", h.resolveEntry)
	e.GET("/api/mini/official-account/binding", h.binding)
	e.POST("/api/mini/official-account/binding-codes", h.createCode)
	e.PUT("/api/mini/official-account/binding", h.updateBinding)
	e.DELETE("/api/mini/official-account/binding", h.updateBinding)
	base := "/api/customer-portal/admin/wechat"
	e.GET(base+"/status", h.adminStatus)
	e.GET(base+"/entries", h.entries)
	e.GET(base+"/entries/:key/preview", h.previewEntry)
	e.GET(base+"/entries/:key/versions", h.versions)
	e.PUT(base+"/entries/:key", h.saveEntry)
	e.GET(base+"/bindings", h.adminBindings)
	e.DELETE(base+"/bindings/:openid", h.adminUnbind)
	e.GET(base+"/menus", h.menus)
	e.POST(base+"/menus/import", h.importMenu)
	e.POST(base+"/menus/draft", h.saveDraft)
	e.POST(base+"/menus/preview", h.previewMenu)
	e.POST(base+"/menus/publish", h.publishMenu)
}
func bearer(c echo.Context) string {
	s := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(s, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(s, "Bearer "))
}
func fail(c echo.Context, status int, msg string) error {
	return c.JSON(status, map[string]string{"error": msg})
}
func (h *Handler) admin(c echo.Context, write bool) (string, error) {
	a, ok, err := support.CurrentActor(c, h.Authz)
	perm := "settings.write"
	if write {
		perm = "settings.write"
	}
	if err != nil || !ok || a.AccountType == support.AccountTypeChannelCustomer || !a.Can(perm) {
		return "", echo.NewHTTPError(http.StatusForbidden, "需要系统设置维护权限")
	}
	return support.ActorOf(c), nil
}
func (h *Handler) me(c echo.Context) (portal.CurrentContext, error) {
	cur, err := h.Portal.Me(c.Request().Context(), bearer(c))
	if err != nil {
		return cur, echo.NewHTTPError(http.StatusUnauthorized, map[string]string{"error": "请先登录小程序"})
	}
	if cur.AccountType == "employee" || cur.CurrentCustomerID <= 0 {
		return cur, echo.NewHTTPError(http.StatusForbidden, map[string]string{"error": "请先完成客户认证"})
	}
	return cur, nil
}
func (h *Handler) resolveEntry(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	e, err := h.Repo.Entry(c.Request().Context(), c.Param("key"))
	if err != nil {
		return fail(c, 404, app.ErrUnavailable.Error())
	}
	if !e.Enabled || e.Status != "published" {
		return fail(c, 404, app.ErrUnavailable.Error())
	}
	var cur *portal.CurrentContext
	if e.Visibility != "public" {
		x, err := h.me(c)
		if err != nil {
			return err
		}
		cur = &x
	}
	if err = app.CheckEntry(e, cur); err != nil {
		return fail(c, 403, err.Error())
	}
	row, err := h.Repo.Portal.LoadEntryPublication(c.Request().Context(), e.PublicationID)
	if err != nil {
		return fail(c, 404, app.ErrUnavailable.Error())
	}
	return c.JSON(200, map[string]any{"entry": e, "publication": row})
}
func (h *Handler) entries(c echo.Context) error {
	if _, err := h.admin(c, false); err != nil {
		return err
	}
	if raw := c.QueryParam("publication_id"); raw != "" {
		id, _ := strconv.ParseInt(raw, 10, 64)
		entries, err := h.Repo.EntriesForPublication(c.Request().Context(), id)
		if err != nil {
			return fail(c, 500, "无法读取该类型的价格表入口")
		}
		return c.JSON(200, map[string]any{"rows": entries})
	}
	rows, err := h.Repo.Entries(c.Request().Context())
	if err != nil {
		return fail(c, 500, "无法读取入口")
	}
	return c.JSON(200, map[string]any{"rows": rows})
}
func (h *Handler) versions(c echo.Context) error {
	if _, err := h.admin(c, false); err != nil {
		return err
	}
	rows, err := h.Repo.Versions(c.Request().Context(), c.Param("key"))
	if err != nil {
		return fail(c, 500, "无法读取版本")
	}
	return c.JSON(200, map[string]any{"rows": rows})
}
func (h *Handler) saveEntry(c echo.Context) error {
	if _, err := h.admin(c, true); err != nil {
		return err
	}
	return fail(c, 409, "历史入口只支持在页面入口管理中查看和停用；请手工新增页面以修改目标")
}
func (h *Handler) binding(c echo.Context) error {
	cur, err := h.me(c)
	if err != nil {
		return err
	}
	rows, err := h.Repo.Bindings(c.Request().Context(), h.Config.AppID, cur.MiniUserID)
	if err != nil {
		return fail(c, 500, "无法读取绑定")
	}
	return c.JSON(200, map[string]any{"rows": rows, "enabled": h.Config.Ready()})
}
func (h *Handler) createCode(c echo.Context) error {
	cur, err := h.me(c)
	if err != nil {
		return err
	}
	if !h.Config.Ready() {
		return fail(c, 503, "公众号尚未启用，请稍后再试")
	}
	code, err := h.Repo.CreateCode(c.Request().Context(), h.Config.AppID, cur)
	if err != nil {
		return fail(c, 403, "无法生成绑定码，请检查客户认证")
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(200, map[string]any{"code": code, "expires_in": 300, "customer_name": cur.CurrentCustomerName})
}
func (h *Handler) updateBinding(c echo.Context) error {
	cur, err := h.me(c)
	if err != nil {
		return err
	}
	var req struct {
		OpenID     string `json:"openid"`
		CustomerID int64  `json:"customer_id"`
	}
	if c.Bind(&req) != nil {
		return fail(c, 400, "请求格式无效")
	}
	active := c.Request().Method != http.MethodDelete
	if active {
		found := false
		for _, b := range cur.Bindings {
			if b.CustomerID == req.CustomerID && b.Status == "approved" {
				found = true
			}
		}
		if !found {
			return fail(c, 403, "客户绑定不可用")
		}
	}
	if err = h.Repo.ChangeBinding(c.Request().Context(), h.Config.AppID, req.OpenID, cur.MiniUserID, req.CustomerID, active, fmt.Sprint("mini-user:", cur.MiniUserID)); err != nil {
		return fail(c, 403, "绑定不可用")
	}
	return c.JSON(200, map[string]bool{"ok": true})
}
func (h *Handler) adminStatus(c echo.Context) error {
	if _, err := h.admin(c, false); err != nil {
		return err
	}
	return c.JSON(200, map[string]any{"enabled": h.Config.Ready(), "app_id": h.Config.AppID, "mini_app_id": h.Config.MiniAppID, "secret_configured": h.Config.AppSecret != "", "token_configured": len(h.Config.Token) >= 16, "encryption_configured": len(h.Config.AESKey) == 43, "unionid_enabled": h.Config.UnionIDEnabled, "menu_publish_enabled": h.Config.Ready() && h.Config.MenuPublishEnabled, "web_oauth_ready": pageconfig.ConfigFromEnv().Ready(), "web_oauth_callback": pageconfig.ConfigFromEnv().PublicOrigin + "/app/api/page-auth/callback", "callback_path": "/app/api/wechat/official-account/callback"})
}
func (h *Handler) adminBindings(c echo.Context) error {
	if _, err := h.admin(c, false); err != nil {
		return err
	}
	rows, err := h.Repo.Bindings(c.Request().Context(), h.Config.AppID, 0)
	if err != nil {
		return fail(c, 500, "无法读取绑定")
	}
	return c.JSON(200, map[string]any{"rows": rows})
}
func (h *Handler) adminUnbind(c echo.Context) error {
	actor, err := h.admin(c, true)
	if err != nil {
		return err
	}
	if err = h.Repo.ChangeBinding(c.Request().Context(), h.Config.AppID, c.Param("openid"), 0, 0, false, actor); err != nil {
		return fail(c, 404, "绑定不存在")
	}
	return c.JSON(200, map[string]bool{"ok": true})
}
func (h *Handler) menus(c echo.Context) error {
	if _, err := h.admin(c, false); err != nil {
		return err
	}
	rows, err := h.Repo.Menus(c.Request().Context(), h.Config.AppID)
	if err != nil {
		return fail(c, 500, "无法读取发布记录")
	}
	return c.JSON(200, map[string]any{"rows": rows})
}
func (h *Handler) importMenu(c echo.Context) error {
	actor, err := h.admin(c, true)
	if err != nil {
		return err
	}
	if !h.Config.Ready() {
		return fail(c, 503, "请先完成公众号服务端配置")
	}
	menu, err := h.Wechat.Menu(c.Request().Context())
	if err != nil {
		return fail(c, 502, err.Error())
	}
	id, err := h.Repo.SaveMenu(c.Request().Context(), h.Config.AppID, "imported", actor, menu)
	if err != nil {
		return fail(c, 500, "导入保存失败")
	}
	return c.JSON(200, map[string]any{"id": id, "menu": menu})
}

type menuRequest struct {
	Menu         app.Menu `json:"menu"`
	PreviewToken string   `json:"preview_token"`
}

func (h *Handler) validateMenu(c echo.Context, m app.Menu) error {
	if err := app.ValidateMenu(m, h.Config.MiniAppID); err != nil {
		return err
	}
	var check func([]app.Button) error
	check = func(buttons []app.Button) error {
		for _, b := range buttons {
			if len(b.SubButtons) > 0 {
				if err := check(b.SubButtons); err != nil {
					return err
				}
			}

			pageKey := ""
			if u, err := url.Parse(b.PagePath); err == nil && u.Path == "pages/page-entry/page-entry" {
				pageKey = u.Query().Get("entry")
			}
			if u, err := url.Parse(b.URL); err == nil && b.Type == "view" && strings.HasPrefix(u.Path, "/app/p/") {
				pageKey = strings.TrimPrefix(u.Path, "/app/p/")
			}
			if pageKey != "" {
				page, err := pagepg.NewRepository(h.Repo.Pool, h.Repo.Schema).Entry(c.Request().Context(), pageKey)
				if err != nil || page.Deleted || !page.Enabled || page.Published == nil {
					return errors.New("菜单引用的页面未发布或已停用")
				}
				if b.Type == "view" && page.Published.Kind == "function" {
					return errors.New("功能页面仅支持小程序打开")
				}
				if page.Published.Kind == "price" {
					if _, err = h.Repo.Portal.LoadEntryPublication(c.Request().Context(), page.Published.PublicationID); err != nil {
						return errors.New("页面引用的价格表已不可用")
					}
				}
			}
			if strings.HasPrefix(b.PagePath, "pages/price-list/price-list?entry=") {
				key := strings.TrimPrefix(b.PagePath, "pages/price-list/price-list?entry=")
				entry, err := h.Repo.Entry(c.Request().Context(), key)
				if err != nil || !entry.Enabled || entry.Status != "published" {
					return errors.New("菜单引用的价格表入口未配置、已停用或版本不可用")
				}
			}
		}
		return nil
	}
	return check(m.Buttons)
}
func (h *Handler) previewSignature(menu, current []byte, expires string) string {
	mac := hmac.New(sha256.New, []byte(h.Config.Token))
	var compact any
	_ = json.Unmarshal(current, &compact)
	canonical, _ := json.Marshal(compact)
	mac.Write([]byte(h.Config.AppID))
	mac.Write(menu)
	mac.Write(canonical)
	mac.Write([]byte(expires))
	return hex.EncodeToString(mac.Sum(nil))
}
func (h *Handler) previewMenu(c echo.Context) error {
	if _, err := h.admin(c, true); err != nil {
		return err
	}
	var req menuRequest
	if c.Bind(&req) != nil {
		return fail(c, 400, "菜单格式无效")
	}
	if err := h.validateMenu(c, req.Menu); err != nil {
		return fail(c, 400, err.Error())
	}
	menu, _ := json.Marshal(req.Menu)
	var current json.RawMessage = json.RawMessage(`{"button":[]}`)
	var err error
	if h.Config.Ready() {
		current, err = h.Wechat.Menu(c.Request().Context())
		if err != nil {
			return fail(c, 502, err.Error())
		}
	}
	expires := strconv.FormatInt(time.Now().Add(5*time.Minute).Unix(), 10)
	token := expires + "." + h.previewSignature(menu, current, expires)
	return c.JSON(200, map[string]any{"menu": req.Menu, "current": current, "preview_token": token, "publish_allowed": h.Config.Ready() && h.Config.MenuPublishEnabled})
}
func (h *Handler) publishMenu(c echo.Context) error {
	actor, err := h.admin(c, true)
	if err != nil {
		return err
	}
	if !h.Config.Ready() || !h.Config.MenuPublishEnabled {
		return fail(c, 503, "正式小程序发布及接入核对完成后才能开启菜单发布")
	}
	var req menuRequest
	if c.Bind(&req) != nil {
		return fail(c, 400, "菜单格式无效")
	}
	if err = h.validateMenu(c, req.Menu); err != nil {
		return fail(c, 400, err.Error())
	}
	h.menuMu.Lock()
	defer h.menuMu.Unlock()
	current, err := h.Wechat.Menu(c.Request().Context())
	if err != nil {
		return fail(c, 502, err.Error())
	}
	menu, _ := json.Marshal(req.Menu)
	parts := strings.Split(req.PreviewToken, ".")
	if len(parts) != 2 {
		return fail(c, 409, "请先预览菜单")
	}
	expires, _ := strconv.ParseInt(parts[0], 10, 64)
	if expires < time.Now().Unix() || expires > time.Now().Add(6*time.Minute).Unix() || !same(parts[1], h.previewSignature(menu, current, parts[0])) {
		return fail(c, 409, "菜单或预览已变化，请重新预览")
	}
	if _, err = h.Repo.SaveMenu(c.Request().Context(), h.Config.AppID, "previous", actor, current); err != nil {
		return fail(c, 500, "未能保留原菜单，发布已停止")
	}
	id, err := h.Repo.SaveMenu(c.Request().Context(), h.Config.AppID, "publishing", actor, menu)
	if err != nil {
		return fail(c, 500, "发布记录保存失败")
	}
	if err = h.Wechat.Publish(c.Request().Context(), menu); err != nil {
		_, _ = h.Repo.SaveMenu(c.Request().Context(), h.Config.AppID, "failed", actor, menu)
		return fail(c, 502, err.Error())
	}
	if _, err = h.Repo.SaveMenu(c.Request().Context(), h.Config.AppID, "published", actor, menu); err != nil {
		return fail(c, 500, "微信已接收菜单，发布记录未完成，请重新导入核对")
	}
	return c.JSON(200, map[string]any{"ok": true, "id": id})
}

func (h *Handler) previewEntry(c echo.Context) error {
	if _, err := h.admin(c, false); err != nil {
		return err
	}
	e, err := h.Repo.Entry(c.Request().Context(), c.Param("key"))
	if err != nil {
		return fail(c, 404, "入口不存在")
	}
	if e.PublicationID <= 0 {
		return fail(c, 409, "此价格表入口尚未选择展示版本")
	}
	p, err := h.Repo.Portal.LoadEntryPublication(c.Request().Context(), e.PublicationID)
	if err != nil {
		return fail(c, 404, "所选版本不可用")
	}
	return c.JSON(200, p)
}

func (h *Handler) saveDraft(c echo.Context) error {
	actor, err := h.admin(c, true)
	if err != nil {
		return err
	}
	var req menuRequest
	if c.Bind(&req) != nil {
		return fail(c, 400, "菜单格式无效")
	}
	if err = h.validateMenu(c, req.Menu); err != nil {
		return fail(c, 400, err.Error())
	}
	menu, _ := json.Marshal(req.Menu)
	id, err := h.Repo.SaveMenu(c.Request().Context(), h.Config.AppID, "draft", actor, menu)
	if err != nil {
		return fail(c, 500, "草稿保存失败")
	}
	return c.JSON(200, map[string]any{"id": id})
}
