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
	portal "orderapp/internal/application/customerportal"
	messages "orderapp/internal/application/messagecenter"
	portalpg "orderapp/internal/infrastructure/postgres/customerportal"
	officialhttp "orderapp/internal/interfaces/http/officialaccount"
	"sort"
	"time"

	sales "orderapp/internal/application/sales"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	wechat "orderapp/internal/infrastructure/wechataccount"
	notify "orderapp/internal/interfaces/http/notifications"
	"strings"
	"testing"
)

func TestOrderNotificationsAPISixDesktopBindingAndGuards(t *testing.T) {
	f := newSampleFixture(t)
	cfg := wechat.Config{Enabled: true, AppID: "notify-app", AppSecret: "fixture-secret", Token: "fixture-token-16-chars", AESKey: strings.Repeat("a", 43), MiniAppID: "fixture-mini"}
	h := notify.RegisterRoutes(f.e, f.pool, f.schema, f.portal, wechatTestAuthz{}, sales.NewService(salespg.NewRepository(f.pool, f.schema)), cfg)
	officialhttp.RegisterRoutes(f.e, f.pool, f.schema, f.portal, wechatTestAuthz{}, cfg)
	sent := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			fmt.Fprint(w, `{"access_token":"fixture-token","expires_in":7200}`)
		case "/cgi-bin/qrcode/create":
			fmt.Fprint(w, `{"ticket":"fixture-qr"}`)
		case "/cgi-bin/message/template/send":
			sent++
			fmt.Fprintf(w, `{"errcode":0,"msgid":%d}`, sent)
		case "/cgi-bin/user/info":
			fmt.Fprint(w, `{"subscribe":1}`)
		default:
			t.Errorf("unexpected provider path %s", r.URL.Path)
		}
	}))
	defer provider.Close()
	h.Client.BaseURL = provider.URL
	call := func(method, path, token string, body any, admin bool, want int) map[string]json.RawMessage {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if admin {
			r.Header.Set("X-Test-Admin", "yes")
		}
		w := httptest.NewRecorder()
		f.e.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		out := map[string]json.RawMessage{}
		if w.Code != 204 {
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	const admin = "/api/message-center/external"
	const mini = "/api/mini/order-notifications"
	const agent = "/api/mini/notification-agent"
	call("GET", admin, f.salesToken, nil, false, 403)
	call("GET", mini, "", nil, false, 401)
	out := call("POST", mini+"/binding", f.salesToken, nil, false, 200)
	var challenge string
	json.Unmarshal(out["token"], &challenge)
	if len(challenge) != 64 {
		t.Fatal("missing QR challenge")
	}
	call("GET", mini+"/binding/"+challenge, f.otherToken, nil, false, 400)
	// Exercise the signed callback rather than bypassing it. New compact and
	// previously issued legacy scenes must both recover the same challenge.
	for _, tc := range []struct{ format, event string }{{"compact", "SCAN"}, {"legacy", "SCAN"}, {"compact", "subscribe"}} {
		current := challenge
		if tc.format == "legacy" || tc.event == "subscribe" {
			other := call("POST", mini+"/binding", f.salesToken, nil, false, 200)
			json.Unmarshal(other["token"], &current)
		}
		scene := "notify_" + current
		if tc.format == "compact" {
			raw, err := hex.DecodeString(current)
			if err != nil {
				t.Fatal(err)
			}
			scene = "notify_" + base64.RawURLEncoding.EncodeToString(raw)
		}
		if tc.event == "subscribe" {
			scene = "qrscene_" + scene
		}
		message := fmt.Sprintf(`<xml><ToUserName>service-account</ToUserName><FromUserName>notification-openid</FromUserName><CreateTime>%d</CreateTime><MsgType>event</MsgType><Event>%s</Event><EventKey>%s</EventKey></xml>`, time.Now().Unix(), tc.event, scene)
		cipher, err := officialhttp.EncryptMessage(cfg.AESKey, cfg.AppID, []byte(message))
		if err != nil {
			t.Fatal(err)
		}
		timestamp := fmt.Sprint(time.Now().Unix())
		parts := []string{cfg.Token, timestamp, "nonce", cipher}
		sort.Strings(parts)
		sum := sha1.Sum([]byte(strings.Join(parts, "")))
		q := url.Values{"encrypt_type": {"aes"}, "timestamp": {timestamp}, "nonce": {"nonce"}, "msg_signature": {fmt.Sprintf("%x", sum)}}
		forged := url.Values{"encrypt_type": {"aes"}, "timestamp": {timestamp}, "nonce": {"nonce"}, "msg_signature": {strings.Repeat("0", 40)}}
		forgedReq := httptest.NewRequest("POST", "/api/wechat/official-account/callback?"+forged.Encode(), strings.NewReader("<xml><Encrypt>"+cipher+"</Encrypt></xml>"))
		forgedRes := httptest.NewRecorder()
		f.e.ServeHTTP(forgedRes, forgedReq)
		if forgedRes.Code != 403 {
			t.Fatalf("%s %s forged signature accepted: %d", tc.format, tc.event, forgedRes.Code)
		}
		before, err := h.Repo.BindingChallenge(f.ctx, current, f.sales.MiniUserID)
		if err != nil || before != "" {
			t.Fatalf("%s %s forged callback changed binding", tc.format, tc.event)
		}
		req := httptest.NewRequest("POST", "/api/wechat/official-account/callback?"+q.Encode(), strings.NewReader("<xml><Encrypt>"+cipher+"</Encrypt></xml>"))
		res := httptest.NewRecorder()
		f.e.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("%s %s callback: %d", tc.format, tc.event, res.Code)
		}
		openid, err := h.Repo.BindingChallenge(f.ctx, current, f.sales.MiniUserID)
		if err != nil || openid != "notification-openid" {
			t.Fatalf("%s %s callback was not recorded: %v", tc.format, tc.event, err)
		}
	}
	call("POST", mini+"/binding/"+challenge+"/confirm", f.otherToken, nil, false, 400)
	call("POST", mini+"/binding/"+challenge+"/confirm", f.salesToken, nil, false, 200)
	call("POST", mini+"/binding/"+challenge+"/confirm", f.salesToken, nil, false, 400)
	out = call("GET", mini, f.salesToken, nil, false, 200)
	var prefs []messagepg.NotificationPreference
	json.Unmarshal(out["rows"], &prefs)
	if len(prefs) != 1 || prefs[0].EmployeeID != f.sales.EmployeeID || !prefs[0].Enabled {
		t.Fatalf("bad preference %s", out["rows"])
	}
	call("PUT", mini, f.salesToken, map[string]any{"enabled": false, "events": []string{"created"}}, false, 200)
	call("POST", admin+"/tests", "", map[string]any{"channel": "wechat_service_account", "event": "created", "mini_user_id": f.sales.MiniUserID}, true, 400)
	out = call("POST", admin+"/devices", "", map[string]string{"name": "测试专用 Mac", "group_name": "测试生产群"}, true, 200)
	var token string
	var deviceID int64
	json.Unmarshal(out["token"], &token)
	json.Unmarshal(out["id"], &deviceID)
	out2 := call("POST", admin+"/devices", "", map[string]string{"name": "其他 Mac", "group_name": "其他测试群"}, true, 200)
	var token2 string
	json.Unmarshal(out2["token"], &token2)
	call("POST", agent+"/heartbeat", "invalid", map[string]string{"health": "ready", "fingerprint": strings.Repeat("a", 64)}, false, 401)
	call("POST", agent+"/heartbeat", token, map[string]string{"health": "ready", "fingerprint": strings.Repeat("a", 64)}, false, 200)
	disabled := messagepg.DefaultExternalConfig()
	disabled.DesktopEnabled = true
	disabled.DesktopID = deviceID
	call("PUT", admin+"/config", "", disabled, true, 400)
	for _, event := range []string{"created", "paid", "production_started", "production_finished", "shipped", "cancelled"} {
		call("POST", admin+"/tests", "", map[string]any{"channel": "wechat_desktop", "event": event, "device_id": deviceID}, true, 200)
		out = call("POST", agent+"/claim", token, nil, false, 200)
		var job messagepg.Delivery
		var content string
		json.Unmarshal(out["job"], &job)
		json.Unmarshal(out["text"], &content)
		if !job.Notice.Test || !strings.Contains(content, "测试通知·虚拟订单") || len(job.Notice.Milestones) != 1 || job.Notice.Milestones[0] != event {
			t.Fatal("bad test job")
		}
		path := fmt.Sprintf("%s/jobs/%d/result", agent, job.ID)
		payload := map[string]string{"lease": job.Lease, "status": "delivered", "evidence": content}
		call("POST", path, token2, payload, false, 400)
		payload["evidence"] = "wrong"
		call("POST", path, token, payload, false, 400)
		payload["evidence"] = content
		call("POST", path, token, payload, false, 200)
		call("POST", path, token, payload, false, 400)
		f.exec("UPDATE %s.message_deliveries SET updated_at=now()-interval '10 seconds'")
	}
	call("PUT", admin+"/config", "", disabled, true, 200)
	// Six actual handler -> queue -> HTTP transport -> signed callback contracts.
	settings := messagepg.DefaultExternalConfig()
	for _, key := range settings.Events {
		settings.Templates[key] = wechat.TemplateMapping{ID: "fixture-business-template", Fields: map[string]string{"thing1": "status", "character_string2": "order_no"}}
	}
	call("PUT", admin+"/config", "", settings, true, 200)
	call("PUT", mini, f.salesToken, map[string]any{"enabled": true, "events": settings.Events}, false, 200)
	for _, key := range settings.Events {
		call("POST", admin+"/tests", "", map[string]any{"channel": "wechat_service_account", "event": key, "mini_user_id": f.sales.MiniUserID}, true, 200)
		if err := h.Worker.Tick(f.ctx); err != nil {
			t.Fatal(err)
		}
		ds, err := h.Repo.ExternalDeliveries(f.ctx)
		if err != nil || ds[0].Status != "accepted" {
			t.Fatalf("missing provider acceptance %v %v", ds, err)
		}
		callback := fmt.Sprintf(`<xml><ToUserName>service-account</ToUserName><FromUserName>notification-openid</FromUserName><CreateTime>%d</CreateTime><MsgType>event</MsgType><Event>TEMPLATESENDJOBFINISH</Event><MsgID>%d</MsgID><Status>success</Status></xml>`, time.Now().Unix(), sent)
		cipher, err := officialhttp.EncryptMessage(cfg.AESKey, cfg.AppID, []byte(callback))
		if err != nil {
			t.Fatal(err)
		}
		timestamp := fmt.Sprint(time.Now().Unix())
		parts := []string{cfg.Token, timestamp, "nonce", cipher}
		sort.Strings(parts)
		sum := sha1.Sum([]byte(strings.Join(parts, "")))
		q := url.Values{"encrypt_type": {"aes"}, "timestamp": {timestamp}, "nonce": {"nonce"}, "msg_signature": {fmt.Sprintf("%x", sum)}}
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("POST", "/api/wechat/official-account/callback?"+q.Encode(), strings.NewReader("<xml><Encrypt>"+cipher+"</Encrypt></xml>"))
			res := httptest.NewRecorder()
			f.e.ServeHTTP(res, req)
			if res.Code != 200 {
				t.Fatalf("callback %d %s", res.Code, res.Body.String())
			}
		}
		ds, _ = h.Repo.ExternalDeliveries(f.ctx)
		if ds[0].Status != "delivered" {
			t.Fatal("signed receipt not recorded")
		}
	}
	if sent != 6 {
		t.Fatalf("sent %d", sent)
	}

	if n := f.id("SELECT count(*) FROM %s.orders"); n != 0 {
		t.Fatal("test created real orders")
	}
	if n := f.id("SELECT count(*) FROM %s.audit_logs WHERE entity_type='notification'"); n < 10 {
		t.Fatal("notification writes not audited")
	}

	call("POST", agent+"/heartbeat", token, map[string]string{"health": "ready", "fingerprint": strings.Repeat("b", 64)}, false, 200)
	call("PUT", admin+"/config", "", disabled, true, 400)
	call("POST", admin+"/tests", "", map[string]any{"channel": "wechat_desktop", "event": "created", "device_id": deviceID}, true, 200)
	out = call("POST", agent+"/claim", token, nil, false, 200)
	var uncertain messagepg.Delivery
	json.Unmarshal(out["job"], &uncertain)
	call("POST", fmt.Sprintf("%s/jobs/%d/result", agent, uncertain.ID), token, map[string]string{"lease": uncertain.Lease, "status": "unknown", "evidence": "测试断网"}, false, 200)
	call("POST", agent+"/claim", token, nil, false, 204)
	call("POST", fmt.Sprintf("%s/deliveries/%d/retry", admin, uncertain.ID), "", map[string]bool{"confirmed_not_sent": false}, true, 400)
	call("POST", fmt.Sprintf("%s/deliveries/%d/confirm", admin, uncertain.ID), "", nil, true, 200)
	call("PUT", admin+"/config", "", disabled, true, 400)
	// Explicit order lookup includes cancelled orders but keeps customer ownership.
	own := f.id("INSERT INTO %s.customers(name) VALUES('通知测试客户') RETURNING id")
	other := f.id("INSERT INTO %s.customers(name) VALUES('其他客户') RETURNING id")
	target := f.id("INSERT INTO %s.orders(customer_id,order_no,is_void) VALUES($1,'NOTICE-VOID',true) RETURNING id", own)
	foreign := f.id("INSERT INTO %s.orders(customer_id,order_no) VALUES($1,'FOREIGN') RETURNING id", other)
	repo := portalpg.NewRepository(f.pool, f.schema)
	f.exec("INSERT INTO %s.customer_portal_profiles(customer_id) VALUES($1)", own)
	f.exec("INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id) VALUES($1,$2)", f.visitor.MiniUserID, own)
	f.exec("INSERT INTO %s.customer_service_capabilities(customer_id,capability_code) VALUES($1,'product_order')", own)
	recipient := messagepg.Recipient{MiniUserID: f.visitor.MiniUserID, CustomerID: own, BoundAt: time.Now()}
	ok, authErr := h.Worker.Authorize(f.ctx, recipient, messages.OrderNotice{OrderID: target, CustomerID: own})
	if authErr != nil || !ok {
		t.Fatalf("own order denied %v", authErr)
	}
	ok, authErr = h.Worker.Authorize(f.ctx, recipient, messages.OrderNotice{OrderID: foreign, CustomerID: own})
	if authErr != nil || ok {
		t.Fatalf("stale snapshot authorized another customer's current order: %v %v", ok, authErr)
	}
	page, err := repo.LoadServicePage(f.ctx, portal.ServicePageQuery{CustomerID: own, OrderID: target, Key: portal.ServiceKeyOrders, Limit: 2})
	if err != nil || len(page.Orders) != 1 || !page.Orders[0].IsVoid {
		t.Fatalf("cancelled lookup %v %v", page.Orders, err)
	}
	page, err = repo.LoadServicePage(f.ctx, portal.ServicePageQuery{CustomerID: own, OrderID: foreign, Key: portal.ServiceKeyOrders, Limit: 2})
	if err != nil || len(page.Orders) != 0 {
		t.Fatalf("foreign lookup exposed %v %v", page.Orders, err)
	}
	call("DELETE", fmt.Sprintf("%s/devices/%d", admin, deviceID), "", nil, true, 200)
	call("POST", agent+"/heartbeat", token, map[string]string{"health": "ready", "fingerprint": strings.Repeat("a", 64)}, false, 401)

}
