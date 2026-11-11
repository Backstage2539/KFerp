package appmain

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	sales "orderapp/internal/application/sales"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	wechat "orderapp/internal/infrastructure/wechataccount"
	notify "orderapp/internal/interfaces/http/notifications"
	officialhttp "orderapp/internal/interfaces/http/officialaccount"
)

func TestFirstBindingSignedUnsubscribeBeforeConfirmHTTP(t *testing.T) {
	f := newSampleFixture(t)
	cfg := wechat.Config{Enabled: true, AppID: "notify-app", AppSecret: "fixture-secret", Token: "fixture-token-16-chars", AESKey: strings.Repeat("a", 43), MiniAppID: "fixture-mini"}
	h := notify.RegisterRoutes(f.e, f.pool, f.schema, f.portal, wechatTestAuthz{}, sales.NewService(salespg.NewRepository(f.pool, f.schema)), cfg)
	officialhttp.RegisterRoutes(f.e, f.pool, f.schema, f.portal, wechatTestAuthz{}, cfg)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			fmt.Fprint(w, `{"access_token":"token","expires_in":7200}`)
		case "/cgi-bin/qrcode/create":
			fmt.Fprint(w, `{"ticket":"qr"}`)
		case "/cgi-bin/user/info":
			fmt.Fprint(w, `{"subscribe":1}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer provider.Close()
	h.Client.BaseURL = provider.URL
	call := func(method, path, body string, want int) map[string]json.RawMessage {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+f.salesToken)
		w := httptest.NewRecorder()
		f.e.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s =%d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		out := map[string]json.RawMessage{}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	mini := "/api/mini/order-notifications"
	choice := fmt.Sprintf(`{"version":1,"enabled":true,"events":["created"],"mini_user_id":%d,"customer_id":0,"employee_id":%d}`, f.sales.MiniUserID, f.sales.EmployeeID)
	call("PUT", mini+"/prompt", choice, 200)
	out := call("POST", mini+"/binding", `{}`, 200)
	var challenge string
	json.Unmarshal(out["token"], &challenge)
	raw, err := hex.DecodeString(challenge)
	if err != nil {
		t.Fatal(err)
	}
	scene := "notify_" + base64.RawURLEncoding.EncodeToString(raw)
	callback := func(event, key string, created int64) {
		t.Helper()
		msg := fmt.Sprintf(`<xml><ToUserName>service-account</ToUserName><FromUserName>race-openid</FromUserName><CreateTime>%d</CreateTime><MsgType>event</MsgType><Event>%s</Event><EventKey>%s</EventKey></xml>`, created, event, key)
		cipher, err := officialhttp.EncryptMessage(cfg.AESKey, cfg.AppID, []byte(msg))
		if err != nil {
			t.Fatal(err)
		}
		ts := fmt.Sprint(time.Now().Unix())
		parts := []string{cfg.Token, ts, "nonce", cipher}
		sort.Strings(parts)
		sum := sha1.Sum([]byte(strings.Join(parts, "")))
		q := url.Values{"encrypt_type": {"aes"}, "timestamp": {ts}, "nonce": {"nonce"}, "msg_signature": {fmt.Sprintf("%x", sum)}}
		r := httptest.NewRequest("POST", "/api/wechat/official-account/callback?"+q.Encode(), strings.NewReader("<xml><Encrypt>"+cipher+"</Encrypt></xml>"))
		w := httptest.NewRecorder()
		f.e.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s callback: %d", event, w.Code)
		}
	}
	now := time.Now().Unix()
	callback("SCAN", scene, now-2)
	callback("unsubscribe", "", now-1)
	call("POST", mini+"/binding/"+challenge+"/confirm", `{}`, 400)
	var count int
	if err = f.pool.QueryRow(f.ctx, "SELECT count(*) FROM "+f.schema+".wechat_bindings WHERE active AND notifications_enabled AND openid='race-openid'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("unsubscribed user enabled: %d %v", count, err)
	}
}
