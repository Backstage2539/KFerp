package officialaccount

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
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
			return "绑定请求已处理。请进入小程序查看订单。"
		}
		code := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(msg.Content), "绑定")))
		if len(code) != 12 {
			return "无需公众号绑定码。请直接进入棵凡小程序查看豆单和订单。"
		}
		if err = h.Repo.ConsumeCode(ctx, h.Config.AppID, msg.From, code); err != nil {
			return bindingErrorMessage(err)
		}
		return "绑定成功。请进入小程序的「我的订单」查看全部订单。"
	}
	if msg.MsgType == "event" && msg.Event == "CLICK" && (msg.Key == "ORDERS_RECENT_1" || msg.Key == "ORDERS_RECENT_3") {
		return "请进入棵凡小程序的「我的订单」查看全部订单，最新订单排在前面。无需公众号绑定码。"
	}
	return ""
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
