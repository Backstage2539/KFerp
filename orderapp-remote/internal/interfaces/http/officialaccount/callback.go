package officialaccount

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	app "orderapp/internal/application/officialaccount"
	"strconv"
	"strings"
	"time"
)

type incoming struct {
	XMLName xml.Name `xml:"xml"`
	To      string   `xml:"ToUserName"`
	From    string   `xml:"FromUserName"`
	Created int64    `xml:"CreateTime"`
	MsgType string   `xml:"MsgType"`
	Event   string   `xml:"Event"`
	Key     string   `xml:"EventKey"`
	Content string   `xml:"Content"`
	MsgID   string   `xml:"MsgId"`
	Encrypt string   `xml:"Encrypt"`
}
type outgoing struct {
	XMLName xml.Name `xml:"xml"`
	To      string   `xml:"ToUserName"`
	From    string   `xml:"FromUserName"`
	Created int64    `xml:"CreateTime"`
	MsgType string   `xml:"MsgType"`
	Content string   `xml:"Content"`
}
type encryptedReply struct {
	XMLName   xml.Name `xml:"xml"`
	Encrypt   string   `xml:"Encrypt"`
	Signature string   `xml:"MsgSignature"`
	Timestamp string   `xml:"TimeStamp"`
	Nonce     string   `xml:"Nonce"`
}

func (h *Handler) Callback(c echo.Context) error {
	if !h.Config.Ready() {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	timestamp, nonce := c.QueryParam("timestamp"), c.QueryParam("nonce")
	encrypted := c.QueryParam("encrypt_type") == "aes"
	if c.Request().Method == http.MethodGet {
		if !same(c.QueryParam("signature"), signature(h.Config.Token, timestamp, nonce)) {
			return c.NoContent(http.StatusForbidden)
		}
		return c.String(http.StatusOK, c.QueryParam("echostr"))
	}
	if !encrypted && !same(c.QueryParam("signature"), signature(h.Config.Token, timestamp, nonce)) {
		return c.NoContent(http.StatusForbidden)
	}
	sec, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(sec, 0)).Abs() > 5*time.Minute {
		return c.NoContent(http.StatusForbidden)
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 65537))
	if err != nil || len(body) > 65536 {
		return c.NoContent(http.StatusBadRequest)
	}
	var msg incoming
	if xml.Unmarshal(body, &msg) != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if encrypted {
		if !same(c.QueryParam("msg_signature"), signature(h.Config.Token, timestamp, nonce, msg.Encrypt)) {
			return c.NoContent(http.StatusForbidden)
		}
		body, err = DecryptMessage(h.Config.AESKey, h.Config.AppID, msg.Encrypt)
		if err != nil || xml.Unmarshal(body, &msg) != nil {
			return c.NoContent(http.StatusForbidden)
		}
	} else if h.Config.AESKey != "" {
		return c.NoContent(http.StatusForbidden)
	}
	if msg.From == "" || msg.To == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3500*time.Millisecond)
	defer cancel()
	text := h.dispatch(ctx, msg)
	if text == "" {
		return c.String(http.StatusOK, "success")
	}
	reply, err := xml.Marshal(outgoing{To: msg.From, From: msg.To, Created: time.Now().Unix(), MsgType: "text", Content: text})
	if err != nil {
		return err
	}
	if encrypted {
		enc, err := EncryptMessage(h.Config.AESKey, h.Config.AppID, reply)
		if err != nil {
			return err
		}
		ts := fmt.Sprint(time.Now().Unix())
		reply, err = xml.Marshal(encryptedReply{Encrypt: enc, Signature: signature(h.Config.Token, ts, nonce, enc), Timestamp: ts, Nonce: nonce})
		if err != nil {
			return err
		}
	}
	return c.Blob(http.StatusOK, "application/xml; charset=utf-8", reply)
}
func (h *Handler) dispatch(ctx context.Context, msg incoming) string {
	if msg.MsgType == "event" && msg.Event == "unsubscribe" {
		sum := sha256.Sum256([]byte("unsubscribe:" + msg.From + fmt.Sprint(msg.Created)))
		fresh, err := h.Repo.FirstEvent(ctx, h.Config.AppID, hex.EncodeToString(sum[:]))
		if err != nil || !fresh {
			return ""
		}
		_ = h.Repo.ChangeBinding(ctx, h.Config.AppID, msg.From, 0, 0, false, "wechat:unsubscribe")
		return ""
	}
	if msg.MsgType == "text" && strings.HasPrefix(strings.TrimSpace(msg.Content), "绑定") {
		sum := sha256.Sum256([]byte(msg.From + msg.MsgID + fmt.Sprint(msg.Created) + msg.Content))
		fresh, err := h.Repo.FirstEvent(ctx, h.Config.AppID, hex.EncodeToString(sum[:]))
		if err != nil {
			return "服务繁忙，请稍后重试"
		}
		if !fresh {
			return "绑定请求已处理，请点击最近一次查询，或在小程序查看绑定状态。"
		}
		code := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(msg.Content), "绑定")))
		if len(code) != 12 {
			return "请在小程序登录后，从个人中心获取绑定码，并发送「绑定 绑定码」。"
		}
		if err = h.Repo.ConsumeCode(ctx, h.Config.AppID, msg.From, code); err != nil {
			return bindingErrorMessage(err)
		}
		return "绑定成功。现在可以点击「我的订单」查询最近订单。"
	}
	count := 0
	if msg.MsgType == "event" && msg.Event == "CLICK" {
		switch msg.Key {
		case "ORDERS_RECENT_1":
			count = 1
		case "ORDERS_RECENT_3":
			count = 3
		}
	}
	if count == 0 {
		return ""
	}
	b, err := h.Repo.Binding(ctx, h.Config.AppID, msg.From)
	if errors.Is(err, pgx.ErrNoRows) && h.Config.UnionIDEnabled {
		union, e := h.Wechat.UnionID(ctx, msg.From)
		if e == nil {
			b, err = h.Repo.AutoBind(ctx, h.Config.AppID, msg.From, union)
		}
	}
	if err != nil || !b.Active {
		return "请先点击「我的订单 → 全部订单」登录小程序并完成客户认证。若仍未识别，请到个人中心获取公众号绑定码，在这里发送「绑定 绑定码」。"
	}
	current, err := h.Repo.Portal.OfficialAccountContext(ctx, b.MiniUserID, b.CustomerID, b.BoundAt)
	if err != nil {
		return app.ErrDenied.Error()
	}
	if current.CurrentCustomerID == 0 {
		return "您关联了多个客户，请先到小程序个人中心选择公众号查询客户。"
	}
	orders, err := h.Repo.Portal.OfficialRecentOrders(ctx, current, count)
	if err != nil {
		return "当前无法查询订单，请在小程序检查客户权限或稍后重试。"
	}
	return app.OrderSummary(current.CurrentCustomerName, orders)
}

// Only fixed domain messages may cross the public callback boundary.
func bindingErrorMessage(err error) string {
	for _, known := range []error{app.ErrDenied, app.ErrInvalidCode, app.ErrBindingConflict, app.ErrRateLimited} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return "服务繁忙，请稍后重试"
}
