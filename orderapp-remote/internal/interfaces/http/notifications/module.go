package notifications

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"net/http"
	portal "orderapp/internal/application/customerportal"
	messages "orderapp/internal/application/messagecenter"
	sales "orderapp/internal/application/sales"
	portalpg "orderapp/internal/infrastructure/postgres/customerportal"
	pg "orderapp/internal/infrastructure/postgres/messagecenter"
	wechat "orderapp/internal/infrastructure/wechataccount"
	"orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
)

type Portal interface {
	Me(context.Context, string) (portal.CurrentContext, error)
}
type Handler struct {
	Repo   pg.Repository
	Portal Portal
	Authz  support.AuthzService
	Client *wechat.WechatClient
	Worker *pg.Worker
}

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, p Portal, auth support.AuthzService, salesService *sales.Service, cfg wechat.Config) *Handler {
	r := pg.NewRepository(pool, schema)
	client := &wechat.WechatClient{Config: cfg}
	h := &Handler{Repo: r, Portal: p, Authz: auth, Client: client}
	h.Worker = &pg.Worker{Repo: r, AppID: cfg.AppID, Sender: client, Authorize: func(ctx context.Context, v pg.Recipient, n messages.OrderNotice) (bool, error) {
		if len(n.Milestones) == 1 && (n.Milestones[0] == "receipt_received" || n.Milestones[0] == "bill_issued") {
			if v.EmployeeID > 0 || v.CustomerID <= 0 || n.CustomerID != v.CustomerID {
				return false, nil
			}
			current, err := portalpg.NewRepository(pool, schema).OfficialAccountContext(ctx, v.MiniUserID, v.CustomerID, v.BoundAt)
			if err != nil {
				return false, officialAuthorizationError(err)
			}
			if current.CurrentCustomerID != n.CustomerID || !current.HasCapability(portal.CapabilitySettlement) {
				return false, nil
			}
			if n.Milestones[0] == "receipt_received" {
				_, err = r.ReceiptDetail(ctx, v.CustomerID, n.ReceiptID)
			} else {
				_, err = r.BillDetail(ctx, v.CustomerID, n.BillID)
			}
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return err == nil, err
		}
		if v.EmployeeID > 0 {
			available, err := employeeNotificationAvailable(ctx, pool, schema, v.EmployeeID, v.MiniUserID, v.BoundAt, v.AppID, v.OpenID)
			if err != nil || !available {
				return false, err
			}
			a, err := auth.ActorByEmployeeID(ctx, v.EmployeeID)
			if err != nil {
				return false, err
			}
			miniAllowed := a.IsAdmin()
			for _, role := range a.Roles {
				if role.Code == "sales" {
					miniAllowed = true
				}
			}
			if !a.Can("orders.read") || !a.Can("orders.write") || !miniAllowed {
				return false, nil
			}
			relevant := v.EmployeeID == n.ResponsibleID
			for _, role := range a.Roles {
				if role.Code == "production" && messages.NotifyProduction(n.Milestones) {
					relevant = true
				}
			}
			if !relevant {
				return false, nil
			}
			q := sales.OrderListQuery{OrderID: n.OrderID, Void: "all", Limit: 1}
			if !a.IsAdmin() {
				q.Scope = "mine"
				q.EmployeeID = v.EmployeeID
			}
			rows, err := salesService.ListOrders(ctx, q)
			return err == nil && len(rows.Rows) == 1, err
		}
		if n.CustomerID <= 0 || v.CustomerID != n.CustomerID {
			return false, nil
		}
		var ownsOrder bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+schema+".orders WHERE id=$1 AND customer_id=$2)", n.OrderID, v.CustomerID).Scan(&ownsOrder); err != nil || !ownsOrder {
			return false, err
		}
		current, err := portalpg.NewRepository(pool, schema).OfficialAccountContext(ctx, v.MiniUserID, v.CustomerID, v.BoundAt)
		if err != nil {
			return false, officialAuthorizationError(err)
		}
		return current.CurrentCustomerID == n.CustomerID && current.HasAnyCapability([]string{portal.CapabilityProductOrder, portal.CapabilityDirectShip, portal.CapabilityShippingQuery, portal.CapabilityMall}), nil
	}}
	h.register(e)
	return h
}
func officialAuthorizationError(err error) error {
	if errors.Is(err, portal.ErrCustomerBindingNotFound) || errors.Is(err, portal.ErrCapabilityNotEnabled) {
		return nil
	}
	return err
}
func (h *Handler) register(e *echo.Echo) {
	admin := "/api/message-center/external"
	e.GET(admin, h.overview)
	e.PUT(admin+"/config", h.saveConfig)
	e.GET(admin+"/templates", h.templates)
	e.POST(admin+"/tests", h.test)
	e.POST(admin+"/deliveries/:id/verify-business-test", h.verifyBusinessTest)
	e.POST(admin+"/deliveries/:id/retry", h.retry)
	e.POST(admin+"/deliveries/:id/confirm", h.confirmSent)
	e.POST(admin+"/devices", h.addDevice)
	e.DELETE(admin+"/devices/:id", h.revokeDevice)
	mini := "/api/mini/order-notifications"
	e.GET("/api/mini/notification-details/receipts/:id", h.receiptDetail)
	e.GET("/api/mini/notification-details/bills/:id", h.billDetail)
	e.GET(mini, h.preferences)
	e.PUT(mini, h.savePreferences)
	e.POST(mini+"/binding", h.startBinding)
	e.GET(mini+"/binding/:token", h.bindingStatus)
	e.POST(mini+"/binding/:token/confirm", h.confirmBinding)
	agent := "/api/mini/notification-agent"
	e.POST(agent+"/heartbeat", h.heartbeat)
	e.POST(agent+"/claim", h.claim)
	e.POST(agent+"/jobs/:id/result", h.result)
}
func bearer(c echo.Context) string {
	value := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(value, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
}
func (h *Handler) admin(c echo.Context) (string, error) {
	a, ok, err := support.CurrentActor(c, h.Authz)
	if err != nil || !ok || a.AccountType == support.AccountTypeChannelCustomer || !a.Can("settings.write") {
		return "", echo.NewHTTPError(403, "需要系统设置维护权限")
	}
	return support.ActorOf(c), nil
}
func (h *Handler) me(c echo.Context) (portal.CurrentContext, error) {
	if h.Portal == nil {
		return portal.CurrentContext{}, echo.NewHTTPError(401, "请先登录")
	}
	cur, e := h.Portal.Me(c.Request().Context(), bearer(c))
	if e != nil || cur.MiniUserID <= 0 {
		return cur, echo.NewHTTPError(401, "请先登录")
	}
	if cur.EmployeeID <= 0 && cur.CurrentCustomerID <= 0 {
		return cur, echo.NewHTTPError(403, "请先选择有效客户")
	}
	if cur.AccountType == "employee" {
		cur.CurrentCustomerID = 0
	}
	return cur, nil
}
func respond(c echo.Context, data any, e error) error {
	if e != nil {
		message := e.Error()
		var db *pgconn.PgError
		if errors.As(e, &db) || errors.Is(e, pgx.ErrNoRows) {
			message = "记录不存在、已过期或暂时不可用，请刷新后重试"
		}
		return c.JSON(400, map[string]string{"error": message})
	}
	return c.JSON(200, data)
}
func (h *Handler) overview(c echo.Context) error {
	if _, e := h.admin(c); e != nil {
		return e
	}
	cfg, e := h.Repo.ExternalConfig(c.Request().Context())
	if e != nil {
		return respond(c, nil, e)
	}
	d, e := h.Repo.ExternalDeliveries(c.Request().Context())
	if e != nil {
		return respond(c, nil, e)
	}
	devices, e := h.Repo.Devices(c.Request().Context())
	if e != nil {
		return respond(c, nil, e)
	}
	failures, e := h.Repo.BusinessEventFailures(c.Request().Context())
	return respond(c, map[string]any{"config": cfg, "deliveries": d, "devices": devices, "business_event_failures": failures, "official_ready": h.Client.Config.Ready()}, e)
}
func (h *Handler) saveConfig(c echo.Context) error {
	actor, e := h.admin(c)
	if e != nil {
		return e
	}
	var v pg.ExternalConfig
	if c.Bind(&v) != nil {
		return echo.NewHTTPError(400, "配置无效")
	}
	if v.OfficialEnabled && !h.Client.Config.Ready() {
		return echo.NewHTTPError(400, "服务号凭据尚未配置完整")
	}
	return respond(c, map[string]bool{"ok": true}, h.Repo.SaveExternalConfig(c.Request().Context(), v, actor))
}
func (h *Handler) templates(c echo.Context) error {
	if _, e := h.admin(c); e != nil {
		return e
	}
	rows, e := h.Client.Templates(c.Request().Context())
	return respond(c, map[string]any{"templates": rows}, e)
}
func (h *Handler) test(c echo.Context) error {
	actor, e := h.admin(c)
	if e != nil {
		return e
	}
	var v struct {
		Event       string `json:"event"`
		Channel     string `json:"channel"`
		MiniID      int64  `json:"mini_user_id"`
		DeviceID    int64  `json:"device_id"`
		StatusValue string `json:"status_value"`
		DetailID    int64  `json:"detail_id"`
	}
	if c.Bind(&v) != nil {
		return echo.NewHTTPError(400, "测试参数无效")
	}
	id, e := h.Repo.QueueTestDetail(c.Request().Context(), v.Event, v.Channel, v.MiniID, v.DeviceID, h.Client.Config.AppID, actor, v.StatusValue, v.DetailID)
	return respond(c, map[string]int64{"event_id": id}, e)
}
func (h *Handler) verifyBusinessTest(c echo.Context) error {
	actor, err := h.admin(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return echo.NewHTTPError(400, "测试记录无效")
	}
	var request struct {
		Confirmed bool `json:"confirmed_received_and_opened"`
	}
	if c.Bind(&request) != nil || !request.Confirmed {
		return echo.NewHTTPError(400, "请先确认实际收到测试消息并打开测试链接")
	}
	return respond(c, map[string]bool{"ok": true}, h.Repo.VerifyBusinessTest(c.Request().Context(), id, actor))
}
func (h *Handler) retry(c echo.Context) error {
	actor, e := h.admin(c)
	if e != nil {
		return e
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var v struct {
		ConfirmedNotSent bool `json:"confirmed_not_sent"`
	}
	if c.Bind(&v) != nil {
		return echo.NewHTTPError(400, "请核对发送结果")
	}
	return respond(c, map[string]bool{"ok": true}, h.Repo.RetryExternal(c.Request().Context(), id, actor, v.ConfirmedNotSent))
}
func (h *Handler) confirmSent(c echo.Context) error {
	actor, e := h.admin(c)
	if e != nil {
		return e
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	return respond(c, map[string]bool{"ok": true}, h.Repo.ConfirmExternal(c.Request().Context(), id, actor))
}
func (h *Handler) addDevice(c echo.Context) error {
	actor, e := h.admin(c)
	if e != nil {
		return e
	}
	var v struct {
		Name      string `json:"name"`
		GroupName string `json:"group_name"`
	}
	if c.Bind(&v) != nil {
		return echo.NewHTTPError(400, "设备参数无效")
	}
	id, token, e := h.Repo.AddDevice(c.Request().Context(), v.Name, v.GroupName, actor)
	c.Response().Header().Set("Cache-Control", "no-store")
	return respond(c, map[string]any{"id": id, "token": token}, e)
}
func (h *Handler) revokeDevice(c echo.Context) error {
	actor, e := h.admin(c)
	if e != nil {
		return e
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	return respond(c, map[string]bool{"ok": true}, h.Repo.RevokeDevice(c.Request().Context(), id, actor))
}
func (h *Handler) financialDetailContext(c echo.Context) (int64, int64, error) {
	current, err := h.me(c)
	if err != nil {
		return 0, 0, err
	}
	if current.EmployeeID > 0 || current.CurrentCustomerID <= 0 || !current.HasCapability(portal.CapabilitySettlement) {
		return 0, 0, echo.NewHTTPError(403, "需要当前客户的结算权限")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, 0, echo.NewHTTPError(400, "单据编号无效")
	}
	return current.CurrentCustomerID, id, nil
}
func (h *Handler) receiptDetail(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	customer, id, err := h.financialDetailContext(c)
	if err != nil {
		return err
	}
	data, err := h.Repo.ReceiptDetail(c.Request().Context(), customer, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(404, "收款单不存在或不可查看")
	}
	if err != nil {
		return echo.NewHTTPError(500, "暂时无法读取收款单")
	}
	return c.JSON(200, data)
}
func (h *Handler) billDetail(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	customer, id, err := h.financialDetailContext(c)
	if err != nil {
		return err
	}
	data, err := h.Repo.BillDetail(c.Request().Context(), customer, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(404, "账单不存在或不可查看")
	}
	if err != nil {
		return echo.NewHTTPError(500, "暂时无法读取账单")
	}
	return c.JSON(200, data)
}
func (h *Handler) preferences(c echo.Context) error {
	cur, e := h.me(c)
	if e != nil {
		return e
	}
	v, e := h.Repo.NotificationPreferences(c.Request().Context(), h.Client.Config.AppID, cur.MiniUserID)
	return respond(c, map[string]any{"rows": v, "enabled": h.Client.Config.Ready()}, e)
}
func (h *Handler) savePreferences(c echo.Context) error {
	cur, e := h.me(c)
	if e != nil {
		return e
	}
	var v struct {
		Enabled bool     `json:"enabled"`
		Events  []string `json:"events"`
	}
	if c.Bind(&v) != nil {
		return echo.NewHTTPError(400, "通知偏好无效")
	}
	employee := int64(0)
	if cur.AccountType == "employee" {
		employee = cur.EmployeeID
	}
	return respond(c, map[string]bool{"ok": true}, h.Repo.SaveNotificationPreference(c.Request().Context(), h.Client.Config.AppID, cur.MiniUserID, cur.CurrentCustomerID, employee, v.Enabled, v.Events))
}
func (h *Handler) startBinding(c echo.Context) error {
	cur, e := h.me(c)
	if e != nil {
		return e
	}
	if !h.Client.Config.Ready() {
		return echo.NewHTTPError(503, "服务号尚未就绪")
	}
	employee := int64(0)
	if cur.AccountType == "employee" {
		employee = cur.EmployeeID
	}
	token, e := h.Repo.StartNotificationBinding(c.Request().Context(), cur.MiniUserID, cur.CurrentCustomerID, employee)
	if e != nil {
		return respond(c, nil, e)
	}
	image, e := h.Client.NotificationQRCode(c.Request().Context(), token)
	c.Response().Header().Set("Cache-Control", "no-store")
	return respond(c, map[string]any{"token": token, "qr_url": image, "expires_in": 300}, e)
}
func (h *Handler) bindingStatus(c echo.Context) error {
	cur, e := h.me(c)
	if e != nil {
		return e
	}
	openid, e := h.Repo.BindingChallenge(c.Request().Context(), c.Param("token"), cur.MiniUserID)
	return respond(c, map[string]bool{"scanned": openid != ""}, e)
}
func (h *Handler) confirmBinding(c echo.Context) error {
	cur, e := h.me(c)
	if e != nil {
		return e
	}
	openid, e := h.Repo.BindingChallenge(c.Request().Context(), c.Param("token"), cur.MiniUserID)
	if e != nil || openid == "" {
		return echo.NewHTTPError(400, "请先扫码关注服务号")
	}
	yes, e := h.Client.Subscribed(c.Request().Context(), openid)
	if e != nil || !yes {
		return echo.NewHTTPError(400, "未确认关注状态，请关注后重试")
	}
	employee := int64(0)
	if cur.AccountType == "employee" {
		employee = cur.EmployeeID
	}
	return respond(c, map[string]bool{"ok": true}, h.Repo.ConfirmNotificationBinding(c.Request().Context(), c.Param("token"), h.Client.Config.AppID, cur.MiniUserID, cur.CurrentCustomerID, employee))
}
func (h *Handler) device(c echo.Context) (pg.DesktopDevice, error) {
	d, e := h.Repo.AuthenticateDevice(c.Request().Context(), bearer(c))
	if e != nil {
		return d, echo.NewHTTPError(401, "设备凭据无效或已撤销")
	}
	return d, nil
}
func (h *Handler) heartbeat(c echo.Context) error {
	d, e := h.device(c)
	if e != nil {
		return e
	}
	var v struct {
		Health      string `json:"health"`
		Fingerprint string `json:"fingerprint"`
	}
	if c.Bind(&v) != nil {
		return echo.NewHTTPError(400, "状态无效")
	}
	return respond(c, map[string]any{"device_id": d.ID, "group_name": d.GroupName}, h.Repo.Heartbeat(c.Request().Context(), d, v.Health, v.Fingerprint))
}
func (h *Handler) claim(c echo.Context) error {
	d, e := h.device(c)
	if e != nil {
		return e
	}
	job, e := h.Repo.ClaimDesktop(c.Request().Context(), d)
	if errors.Is(e, pgx.ErrNoRows) {
		return c.NoContent(204)
	}
	return respond(c, map[string]any{"job": job, "group_name": d.GroupName, "text": job.Notice.GroupText()}, e)
}
func (h *Handler) result(c echo.Context) error {
	d, e := h.device(c)
	if e != nil {
		return e
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var v struct {
		Lease    string `json:"lease"`
		Status   string `json:"status"`
		Evidence string `json:"evidence"`
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 16384)
	if c.Bind(&v) != nil {
		return echo.NewHTTPError(400, "回执无效")
	}
	return respond(c, map[string]bool{"ok": true}, h.Repo.DesktopResult(c.Request().Context(), d, id, v.Lease, v.Status, v.Evidence))
}
