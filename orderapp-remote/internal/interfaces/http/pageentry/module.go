package pageentry

import (
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	portal "orderapp/internal/application/customerportal"
	official "orderapp/internal/application/officialaccount"
	app "orderapp/internal/application/pageentry"
	officialpg "orderapp/internal/infrastructure/postgres/officialaccount"
	repo "orderapp/internal/infrastructure/postgres/pageentry"
	support "orderapp/internal/interfaces/http/support"
	"strings"
)

type Handler struct {
	Repo     repo.Repository
	Portal   *portal.Service
	Authz    support.AuthzService
	Config   Config
	Official officialpg.Repository
}

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, p *portal.Service, authz support.AuthzService, cfg Config) {
	h := &Handler{repo.NewRepository(pool, schema), p, authz, cfg, officialpg.NewRepository(pool, schema)}
	b := "/api/admin/page-entries"
	e.GET(b, h.list)
	e.POST(b, h.create)
	e.GET(b+"/targets", h.targets)
	e.GET(b+"/history", h.history)
	e.POST(b+"/history/:key/disable", h.disableHistory)
	e.GET(b+"/:key", h.get)
	e.PUT(b+"/:key", h.change)
	e.DELETE(b+"/:key", h.change)
	e.POST(b+"/:key/publish", h.change)
	e.POST(b+"/:key/disable", h.change)
	e.GET(b+"/:key/preview", h.preview)
	e.GET(b+"/:key/references", h.references)
	e.POST(b+"/:key/images", h.upload)
	e.GET(b+"/:key/images/:asset", h.adminImage)
	e.GET("/api/mini/bean-center", h.beanCenter)
	e.GET("/api/pages/:key", h.resolve)
	e.GET("/api/pages/:key/images/:asset", h.image)
	e.GET("/api/page-auth/status", h.authStatus)
	e.POST("/api/page-auth/login", h.login)
	e.POST("/api/page-auth/logout", h.logout)
	e.POST("/api/page-auth/customer", h.selectCustomer)
	e.GET("/api/page-auth/wechat", h.oauthStart)
	e.GET("/api/page-auth/callback", h.oauthCallback)
}
func fail(c echo.Context, status int, msg string) error {
	return c.JSON(status, map[string]string{"error": msg})
}
func respond(c echo.Context, v any, err error) error {
	if err != nil {
		switch {
		case errors.Is(err, app.ErrConflict):
			return fail(c, 409, err.Error())
		case errors.Is(err, app.ErrDenied):
			return fail(c, 403, err.Error())
		case errors.Is(err, app.ErrInvalid):
			return fail(c, 400, err.Error())
		case errors.Is(err, app.ErrUnavailable), repo.IsMissing(err):
			return fail(c, 404, app.ErrUnavailable.Error())
		default:
			c.Logger().Error("page entry operation failed: ", err)
			return fail(c, 500, "页面服务暂不可用")
		}
	}
	return c.JSON(200, v)
}
func (h *Handler) admin(c echo.Context) (string, error) {
	a, ok, err := support.CurrentActor(c, h.Authz)
	if err != nil || !ok || a.AccountType == support.AccountTypeChannelCustomer || !a.Can("settings.write") {
		return "", echo.NewHTTPError(403, "需要系统设置维护权限")
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return support.ActorOf(c), nil
}
func (h *Handler) list(c echo.Context) error {
	if _, err := h.admin(c); err != nil {
		return err
	}
	rows, err := h.Repo.List(c.Request().Context())
	return respond(c, map[string]any{"rows": rows}, err)
}
func (h *Handler) get(c echo.Context) error {
	if _, err := h.admin(c); err != nil {
		return err
	}
	e, err := h.Repo.Entry(c.Request().Context(), c.Param("key"))
	return respond(c, e, err)
}
func (h *Handler) create(c echo.Context) error {
	actor, err := h.admin(c)
	if err != nil {
		return err
	}
	var req struct {
		Draft app.Document `json:"draft"`
	}
	if c.Bind(&req) != nil {
		return fail(c, 400, "草稿格式无效")
	}
	e, err := h.Repo.Create(c.Request().Context(), req.Draft, actor)
	if err != nil {
		return respond(c, nil, err)
	}
	return c.JSON(201, e)
}
func (h *Handler) change(c echo.Context) error {
	actor, err := h.admin(c)
	if err != nil {
		return err
	}
	var req struct {
		Draft    app.Document `json:"draft"`
		Revision int64        `json:"revision"`
	}
	if c.Bind(&req) != nil {
		return fail(c, 400, "配置格式无效")
	}
	action := "save"
	if c.Request().Method == "DELETE" {
		action = "delete"
	} else if strings.HasSuffix(c.Path(), "/publish") {
		action = "publish"
	} else if strings.HasSuffix(c.Path(), "/disable") {
		action = "disable"
	}
	e, err := h.Repo.Change(c.Request().Context(), c.Param("key"), req.Revision, action, req.Draft, actor)
	return respond(c, e, err)
}
func (h *Handler) targets(c echo.Context) error {
	if _, err := h.admin(c); err != nil {
		return err
	}
	rows, err := h.Repo.Targets(c.Request().Context())
	return respond(c, map[string]any{"rows": rows, "functions": app.Functions}, err)
}
func (h *Handler) references(c echo.Context) error {
	if _, err := h.admin(c); err != nil {
		return err
	}
	rows, err := h.Repo.References(c.Request().Context(), c.Param("key"))
	return respond(c, map[string]any{"rows": rows}, err)
}
func (h *Handler) preview(c echo.Context) error {
	if _, err := h.admin(c); err != nil {
		return err
	}
	e, err := h.Repo.Entry(c.Request().Context(), c.Param("key"))
	if err != nil || e.Deleted {
		return respond(c, nil, app.ErrUnavailable)
	}
	return h.content(c, e.Draft)
}
func (h *Handler) content(c echo.Context, d app.Document) error {
	out := map[string]any{"document": d, "target_path": app.FunctionPath(d.Target)}
	if d.Kind == "price" && d.PublicationID > 0 {
		p, err := h.Repo.Portal.LoadEntryPublication(c.Request().Context(), d.PublicationID)
		if err != nil {
			return respond(c, nil, app.ErrUnavailable)
		}
		out["publication"] = p
	}
	return c.JSON(200, out)
}
func (h *Handler) published(c echo.Context) (app.Document, error) {
	c.Response().Header().Set("Cache-Control", "no-store, private")
	c.Response().Header().Set("Vary", "Cookie, Authorization")
	e, err := h.Repo.Entry(c.Request().Context(), c.Param("key"))
	if err != nil || e.Deleted || !e.Enabled || e.Published == nil {
		return app.Document{}, echo.NewHTTPError(404, map[string]string{"error": app.ErrUnavailable.Error()})
	}
	d := *e.Published
	var cur *portal.CurrentContext
	if d.Visibility == "registered" {
		if !strings.HasPrefix(c.Request().Header.Get("Authorization"), "Bearer ") {
			return d, echo.NewHTTPError(401, map[string]string{"error": "请在棵凡小程序中完成手机号和昵称登记后查看", "code": "mini_registration_required"})
		}
		x, err := h.current(c)
		if err != nil || !x.RegistrationComplete {
			return d, echo.NewHTTPError(401, map[string]string{"error": "请先完成手机号和昵称登记", "code": "mini_registration_required"})
		}
		cur = &x
	} else if d.Visibility != "public" {
		x, err := h.current(c)
		if err != nil {
			return d, echo.NewHTTPError(401, map[string]string{"error": "请登录并完成客户认证"})
		}
		if x.CurrentCustomerID == 0 {
			return d, echo.NewHTTPError(403, map[string]string{"error": "请先选择已认证客户"})
		}
		approved := false
		for _, b := range x.Bindings {
			if b.CustomerID == x.CurrentCustomerID && b.Status == "approved" {
				approved = true
			}
		}
		if !approved {
			return d, echo.NewHTTPError(403, map[string]string{"error": "请先完成客户认证"})
		}
		cur = &x
	}
	if d.Kind == "price" {
		var owner, key, status string
		var deleted bool
		err = h.Repo.Pool.QueryRow(c.Request().Context(), "SELECT owner_type,owner_key,status,deleted_at IS NOT NULL FROM "+h.Repo.Schema+".bean_list_publications WHERE id=$1", d.PublicationID).Scan(&owner, &key, &status, &deleted)
		if err != nil || deleted || status != "published" {
			return d, echo.NewHTTPError(404, map[string]string{"error": app.ErrUnavailable.Error()})
		}
		if d.Visibility == "registered" {
			if owner != "official" {
				return d, echo.NewHTTPError(403, map[string]string{"error": app.ErrDenied.Error()})
			}
		} else if err = official.CheckEntry(official.Entry{Enabled: true, Status: status, OwnerType: owner, OwnerKey: key, Visibility: d.Visibility}, cur); err != nil {
			return d, echo.NewHTTPError(403, map[string]string{"error": app.ErrDenied.Error()})
		}
	}
	return d, nil
}
func (h *Handler) resolve(c echo.Context) error {
	d, err := h.published(c)
	if err != nil {
		return err
	}
	return h.content(c, d)
}
func (h *Handler) upload(c echo.Context) error {
	actor, err := h.admin(c)
	if err != nil {
		return err
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 9<<20)
	f, err := c.FormFile("file")
	if err != nil {
		return fail(c, 400, "请选择不超过 8 MB 的图片")
	}
	r, err := f.Open()
	if err != nil {
		return fail(c, 400, "无法读取图片")
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return fail(c, 400, "图片不能超过 8 MB")
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
	default:
		return fail(c, 400, "只支持 PNG、JPEG、GIF、WebP 图片")
	}
	id, err := h.Repo.SaveAsset(c.Request().Context(), c.Param("key"), mime, data, actor)
	return respond(c, map[string]string{"asset_id": id}, err)
}
func (h *Handler) serveImage(c echo.Context) error {
	data, mime, err := h.Repo.Asset(c.Request().Context(), c.Param("key"), c.Param("asset"))
	if err != nil {
		return respond(c, nil, app.ErrUnavailable)
	}
	c.Response().Header().Set("Cache-Control", "no-store, private")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return c.Blob(200, mime, data)
}
func (h *Handler) adminImage(c echo.Context) error {
	if _, err := h.admin(c); err != nil {
		return err
	}
	e, err := h.Repo.Entry(c.Request().Context(), c.Param("key"))
	if err != nil || e.Deleted {
		return respond(c, nil, app.ErrUnavailable)
	}
	return h.serveImage(c)
}
func (h *Handler) image(c echo.Context) error {
	d, err := h.published(c)
	if err != nil {
		return err
	}
	if !d.HasAsset(c.Param("asset")) {
		return respond(c, nil, app.ErrUnavailable)
	}
	return h.serveImage(c)
}
func (h *Handler) history(c echo.Context) error {
	if _, err := h.admin(c); err != nil {
		return err
	}
	rows, err := h.Official.HistoricalEntries(c.Request().Context())
	return respond(c, map[string]any{"rows": rows}, err)
}
func (h *Handler) disableHistory(c echo.Context) error {
	actor, err := h.admin(c)
	if err != nil {
		return err
	}
	err = h.Official.DisableHistoricalEntry(c.Request().Context(), c.Param("key"), actor)
	return respond(c, map[string]bool{"ok": err == nil}, err)
}
