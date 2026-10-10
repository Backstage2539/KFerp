package appmain

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	authzapp "orderapp/internal/application/authz"
	portal "orderapp/internal/application/customerportal"
	app "orderapp/internal/application/officialaccount"
	repo "orderapp/internal/infrastructure/postgres/officialaccount"
	officialhttp "orderapp/internal/interfaces/http/officialaccount"
)

type wechatTestAuthz struct{}

func (wechatTestAuthz) ActorByEmployeeID(_ context.Context, employeeID int64) (authzapp.Actor, error) {
	return authzapp.Actor{EmployeeID: employeeID, Name: "公众号配置测试员", Permissions: []string{"settings.write"}}, nil
}
func (wechatTestAuthz) ListRoles(context.Context) ([]authzapp.Role, error) { return nil, nil }
func (wechatTestAuthz) ListEmployeeRoles(context.Context) (map[int64][]string, error) {
	return map[int64][]string{}, nil
}
func (wechatTestAuthz) AssignEmployeeRoles(context.Context, authzapp.AssignmentCommand) error {
	return nil
}

func TestWechatOfficialPostgres(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated test PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_wechat_%d", time.Now().UnixNano())
	t.Cleanup(func() { pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	if err = ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	r := repo.NewRepository(pool, schema)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, fmt.Sprintf(q, schema), args...); e != nil {
			t.Fatal(e)
		}
	}
	id := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		if e := pool.QueryRow(ctx, fmt.Sprintf(q, schema), args...).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	customer := id(`INSERT INTO %s.customers(name) VALUES('公众号客户甲') RETURNING id`)
	other := id(`INSERT INTO %s.customers(name) VALUES('公众号客户乙') RETURNING id`)
	user := id(`INSERT INTO %s.mini_users(openid,unionid) VALUES('mini-test','trusted-union') RETURNING id`)
	exec(`INSERT INTO %s.customer_portal_profiles(customer_id) VALUES($1),($2)`, customer, other)
	exec(`INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id) VALUES($1,$2)`, user, customer)
	exec(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code) VALUES($1,'product_order'),($1,'bean_list'),($2,'product_order')`, customer, other)
	contextFor := func(cid int64) portal.CurrentContext {
		t.Helper()
		c, e := r.Portal.OfficialAccountContext(ctx, user, cid, time.Now())
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	publication := func(table, version, owner, key string) int64 {
		publicationID := id(`INSERT INTO %s.bean_list_publications(list_type,publication_table_key,publication_table_name,version_no,owner_type,owner_key,config_json,content_json) VALUES('commercial',$1,$1,$2,$3,$4,'{"title":"测试豆单"}','{"groups":[{"category":"熟豆","items":[{"name":"测试豆","prices":[{"label":"227g","value":"¥50"}]}]}]}') RETURNING id`, table, version, owner, key)
		exec(`INSERT INTO %[1]s.wechat_price_entries(scope_key,name,publication_id) SELECT %[1]s.wechat_price_scope(p),COALESCE(NULLIF(p.publication_table_name,''),p.list_type),p.id FROM %[1]s.bean_list_publications p WHERE p.id=$1 ON CONFLICT(scope_key) DO NOTHING`, publicationID)
		return publicationID
	}
	t.Run("automatic generation retired and legacy state preserved", func(t *testing.T) {
		before := id(`SELECT count(*) FROM %s.wechat_price_entries`)
		pid := id(`INSERT INTO %s.bean_list_publications(list_type,classification_template_id,classification_template_name,publication_table_key) VALUES('commercial',71001,'咖啡豆','manual-only') RETURNING id`)
		if after := id(`SELECT count(*) FROM %s.wechat_price_entries`); after != before {
			t.Fatal("publication generated entries")
		}
		exec(`INSERT INTO %s.wechat_price_entries(entry_key,scope_key,type_key,purpose,name,publication_id,visibility,enabled) VALUES('legacy-unchanged','legacy-scope','product-type:991','wholesale','历史名称',$1,'public',true)`, pid)
		old, err := r.Entry(ctx, "legacy-unchanged")
		if err != nil {
			t.Fatal(err)
		}
		if err = repo.EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
		now, err := r.Entry(ctx, "legacy-unchanged")
		if err != nil || old != now {
			t.Fatalf("legacy row mutated: %#v %#v %v", old, now, err)
		}
	})

	first := publication("roasted", "v1", "official", "")
	e, err := r.EntryForPublication(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("fixed path version isolation and withdrawal", func(t *testing.T) {
		if len(e.Key) != 32 || e.Visibility != "authenticated" {
			t.Fatalf("unsafe default: %+v", e)
		}
		second := publication("roasted", "v2", "official", "")
		separate := publication("drip", "v1", "official", "")
		stable, _ := r.EntryForPublication(ctx, second)
		if stable.Key != e.Key || stable.PublicationID != first {
			t.Fatal("automatic version switch or path changed")
		}
		versions, err := r.Versions(ctx, e.Key)
		if err != nil || len(versions) != 2 {
			t.Fatalf("versions=%+v err=%v", versions, err)
		}
		bad := e
		bad.PublicationID = separate
		if _, err = r.SaveEntry(ctx, e.Key, bad, "test"); err == nil {
			t.Fatal("cross table allowed")
		}
		e.PublicationID = second
		e.Visibility = "public"
		e, err = r.SaveEntry(ctx, e.Key, e, "test")
		if err != nil {
			t.Fatal(err)
		}
		if app.CheckEntry(e, nil) != nil {
			t.Fatal("public denied")
		}
		before := e
		e.Name = "新版入口"
		e, err = r.SaveEntry(ctx, e.Key, e, "test")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.SaveEntry(ctx, e.Key, before, "test"); err == nil {
			t.Fatal("stale revision overwrote configuration")
		}
		e.Enabled = false
		e, err = r.SaveEntry(ctx, e.Key, e, "test")
		if err != nil || app.CheckEntry(e, nil) == nil {
			t.Fatal("disabled entry accessible", err)
		}
		e.Enabled = true
		e, err = r.SaveEntry(ctx, e.Key, e, "test")
		if err != nil {
			t.Fatal(err)
		}
		exec(`UPDATE %s.bean_list_publications SET status='withdrawn' WHERE id=$1`, second)
		withdrawn, _ := r.Entry(ctx, e.Key)
		if app.CheckEntry(withdrawn, nil) == nil || withdrawn.PublicationID != second {
			t.Fatal("withdrawal fell back or accessible")
		}
		if _, err = r.Portal.LoadEntryPublication(ctx, second); err == nil {
			t.Fatal("withdrawn content readable")
		}
		// Disabling an already withdrawn entry remains possible.
		withdrawn.Enabled = false
		if _, err = r.SaveEntry(ctx, e.Key, withdrawn, "test"); err != nil {
			t.Fatal("cannot disable withdrawn entry", err)
		}
		exec(`UPDATE %s.bean_list_publications SET status='published' WHERE id=$1`, second)
		if err = repo.EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatalf("reapplying official-account schema failed: %v", err)
		}
		preserved, _ := r.Entry(ctx, e.Key)
		if preserved.Enabled {
			t.Fatal("migration revived disabled entry")
		}
	})
	t.Run("legacy independent sheets and real batch metadata", func(t *testing.T) {
		a := publication("", "old-a", "official", "")
		b := publication("", "old-b", "official", "")
		ea, err := r.EntryForPublication(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		eb, err := r.EntryForPublication(ctx, b)
		if err != nil || ea.Key == eb.Key {
			t.Fatal("unrelated legacy sheets collapsed", err)
		}
		pub := id(`INSERT INTO %s.bean_list_publications(list_type,config_json,content_json) VALUES('commercial','{"publication_batch":{"table_key":"actual-batch","table_name":"挂耳批发"}}','{"groups":[]}') RETURNING id`)
		exec(`INSERT INTO %[1]s.wechat_price_entries(scope_key,name,publication_id) SELECT %[1]s.wechat_price_scope(p),COALESCE(NULLIF(p.config_json->'publication_batch'->>'table_name',''),p.list_type),p.id FROM %[1]s.bean_list_publications p WHERE p.id=$1 ON CONFLICT(scope_key) DO NOTHING`, pub)
		before, err := r.EntryForPublication(ctx, pub)
		if err != nil || before.Name != "挂耳批发" {
			t.Fatal("batch metadata missing", before, err)
		}
		exec(`UPDATE %s.bean_list_publications SET publication_table_key='actual-batch',publication_table_name='挂耳批发' WHERE id=$1`, pub)
		after, err := r.EntryForPublication(ctx, pub)
		if err != nil || after.Key != before.Key {
			t.Fatal("metadata sync changed scope", err)
		}
		if _, err = r.Portal.LoadEntryPublication(ctx, pub); err == nil {
			t.Fatal("empty content accepted")
		}
	})

	t.Run("customer table never public and authenticated resolver rechecks", func(t *testing.T) {
		pid := publication("private", "v1", "customer", fmt.Sprint(customer))
		entry, err := r.EntryForPublication(ctx, pid)
		if err != nil {
			t.Fatal(err)
		}
		cur := contextFor(customer)
		if app.CheckEntry(entry, &cur) != nil {
			t.Fatal("owner denied")
		}
		cur.CurrentCustomerID = other
		if app.CheckEntry(entry, &cur) == nil {
			t.Fatal("other customer allowed")
		}
		entry.Visibility = "public"
		if _, err = r.SaveEntry(ctx, entry.Key, entry, "test"); err == nil {
			t.Fatal("private became public")
		}
		e := echo.New()
		svc := portal.NewService(r.Portal, nil)
		officialhttp.RegisterRoutes(e, pool, schema, svc, nil, officialhttp.Config{})
		exec(`INSERT INTO %s.mini_sessions(token,mini_user_id,current_customer_id,expire_at) VALUES('wechat-test-token',$1,$2,now()+interval '1 hour')`, user, customer)
		request := func(token string, want int) {
			t.Helper()
			req := httptest.NewRequest(http.MethodGet, "/api/mini/price-table-entries/"+entry.Key, nil)
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			res := httptest.NewRecorder()
			e.ServeHTTP(res, req)
			if res.Code != want {
				t.Fatalf("resolver: %d %s", res.Code, res.Body.String())
			}
			if res.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cache allowed")
			}
		}
		request("", 401)
		request("wechat-test-token", 200)
		exec(`UPDATE %s.customer_portal_user_bindings SET status='pending' WHERE mini_user_id=$1 AND customer_id=$2`, user, customer)
		request("wechat-test-token", 403)
		exec(`UPDATE %s.customer_portal_user_bindings SET status='approved' WHERE mini_user_id=$1 AND customer_id=$2`, user, customer)
	})
	t.Run("automatic identity multi customer choice and revoke tombstone", func(t *testing.T) {
		b, err := r.AutoBind(ctx, "app", "oa-auto", "trusted-union")
		if err != nil || b.CustomerID != customer {
			t.Fatal("auto binding", b, err)
		}
		if _, err = r.AutoBind(ctx, "app", "oa-forged", "mini-test"); err == nil {
			t.Fatal("openid treated as unionid")
		}
		exec(`INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id) VALUES($1,$2)`, user, other)
		b, err = r.AutoBind(ctx, "app", "oa-multi", "trusted-union")
		if err != nil || b.CustomerID != 0 {
			t.Fatal("multi customer guessed", b, err)
		}
		if err = r.ChangeBinding(ctx, "app", "oa-multi", user, other, true, "test"); err != nil {
			t.Fatal(err)
		}
		if err = r.ChangeBinding(ctx, "app", "oa-multi", user, 0, false, "test"); err != nil {
			t.Fatal(err)
		}
		b, err = r.AutoBind(ctx, "app", "oa-multi", "trusted-union")
		if err != nil || b.Active {
			t.Fatal("automatic rebind after revoke", err)
		}
		if err = r.ChangeBinding(ctx, "app", "oa-multi", user, other, true, "test"); err == nil {
			t.Fatal("selection revived revoked binding")
		}
	})
	t.Run("one time expiry attempts and live account permission", func(t *testing.T) {
		cur := contextFor(customer)
		code, err := r.CreateCode(ctx, "app", cur)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		success := make(chan bool, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); success <- r.ConsumeCode(ctx, "app", "oa-code", code) == nil }()
		}
		wg.Wait()
		close(success)
		n := 0
		for ok := range success {
			if ok {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("code consumed %d times", n)
		}
		code, err = r.CreateCode(ctx, "app", cur)
		if err != nil {
			t.Fatal(err)
		}
		exec(`UPDATE %s.wechat_binding_codes SET expires_at=now()-interval '1 second'`)
		if r.ConsumeCode(ctx, "app", "oa-expired", code) == nil {
			t.Fatal("expired code accepted")
		}
		for i := 0; i < 5; i++ {
			r.ConsumeCode(ctx, "app", "oa-limited", "invalid")
		}
		code, err = r.CreateCode(ctx, "app", cur)
		if err != nil {
			t.Fatal(err)
		}
		if r.ConsumeCode(ctx, "app", "oa-limited", code) == nil {
			t.Fatal("rate limit absent")
		}
		code, err = r.CreateCode(ctx, "app", cur)
		if err != nil {
			t.Fatal(err)
		}
		exec(`UPDATE %s.customer_portal_user_bindings SET status='pending' WHERE mini_user_id=$1 AND customer_id=$2`, user, customer)
		if r.ConsumeCode(ctx, "app", "oa-pending", code) == nil {
			t.Fatal("pending customer bound")
		}
		exec(`UPDATE %s.customer_portal_user_bindings SET status='approved' WHERE mini_user_id=$1 AND customer_id=$2`, user, customer)
		exec(`UPDATE %s.mini_users SET active=false WHERE id=$1`, user)
		if _, err = r.Portal.OfficialAccountContext(ctx, user, customer, time.Now()); err == nil {
			t.Fatal("inactive account allowed")
		}
		exec(`UPDATE %s.mini_users SET active=true WHERE id=$1`, user)
		exec(`UPDATE %s.customers SET active=false WHERE id=$1`, customer)
		c, _ := r.Portal.OfficialAccountContext(ctx, user, customer, time.Now())
		if c.CurrentCustomerID != 0 {
			t.Fatal("inactive customer allowed")
		}
		exec(`UPDATE %s.customers SET active=true WHERE id=$1`, customer)
		exec(`UPDATE %s.customer_service_capabilities SET enabled=false WHERE customer_id=$1`, customer)
		c = contextFor(customer)
		if _, err = r.Portal.OfficialRecentOrders(ctx, c, 1); err == nil {
			t.Fatal("revoked order permission allowed")
		}
		exec(`UPDATE %s.customer_service_capabilities SET enabled=true WHERE customer_id=$1`, customer)
	})
	t.Run("all history pagination recent scope ordering and void filter", func(t *testing.T) {
		for i := 0; i < 55; i++ {
			id(`INSERT INTO %s.orders(customer_id,order_no,order_date,grand_total,portal_service_code) VALUES($1,$2,'2026-10-08',123.45,$3) RETURNING id`, customer, fmt.Sprintf("ORDER-%02d", i), []string{"", "product_order"}[i%2])
		}
		id(`INSERT INTO %s.orders(customer_id,order_no,order_date,is_void) VALUES($1,'VOID','2026-10-08',true) RETURNING id`, customer)
		id(`INSERT INTO %s.orders(customer_id,order_no,order_date) VALUES($1,'OTHER-CUSTOMER','2026-10-08') RETURNING id`, other)
		page, err := r.Portal.LoadServicePage(ctx, portal.ServicePageQuery{Key: "orders", CustomerID: customer, Limit: 20, Offset: 40})
		if err != nil || len(page.Orders) != 15 || page.HasMore {
			t.Fatalf("page %#v err=%v", page, err)
		}
		page, err = r.Portal.LoadServicePage(ctx, portal.ServicePageQuery{Key: "orders", CustomerID: customer, Limit: 20})
		if err != nil || len(page.Orders) != 20 || !page.HasMore {
			t.Fatal("first page", err)
		}
		recent, err := r.Portal.OfficialRecentOrders(ctx, contextFor(customer), 3)
		if err != nil || len(recent) != 3 {
			t.Fatal("recent", err)
		}
		if recent[0].OrderNo != "ORDER-54" || recent[2].OrderNo != "ORDER-52" {
			t.Fatalf("sort or scope %+v", recent)
		}
		if strings.Contains(app.OrderSummary("甲", recent), "OTHER-CUSTOMER") {
			t.Fatal("other customer leaked")
		}
	})
	t.Run("encrypted callback uses current permissions even on duplicate delivery", func(t *testing.T) {
		cfg := officialhttp.Config{Enabled: true, AppID: "app", AppSecret: "test-secret", MiniAppID: "mini", Token: "test-callback-token", AESKey: strings.TrimRight(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))), "=")}
		h := officialhttp.Handler{Config: cfg, Repo: r, Portal: portal.NewService(r.Portal, nil)}
		server := echo.New()
		server.POST("/callback", h.Callback)
		call := func(content string) string {
			t.Helper()
			cipher, err := officialhttp.EncryptMessage(cfg.AESKey, cfg.AppID, []byte(content))
			if err != nil {
				t.Fatal(err)
			}
			timestamp := fmt.Sprint(time.Now().Unix())
			nonce := "test-nonce"
			parts := []string{cfg.Token, timestamp, nonce, cipher}
			sort.Strings(parts)
			sum := sha1.Sum([]byte(strings.Join(parts, "")))
			q := url.Values{"encrypt_type": {"aes"}, "timestamp": {timestamp}, "nonce": {nonce}, "msg_signature": {fmt.Sprintf("%x", sum)}}
			req := httptest.NewRequest("POST", "/callback?"+q.Encode(), strings.NewReader("<xml><Encrypt>"+cipher+"</Encrypt></xml>"))
			res := httptest.NewRecorder()
			server.ServeHTTP(res, req)
			if res.Code != 200 {
				t.Fatalf("callback %d %s", res.Code, res.Body.String())
			}
			if res.Body.String() == "success" {
				return ""
			}
			var enc struct {
				Encrypt string `xml:"Encrypt"`
			}
			if err = xml.Unmarshal(res.Body.Bytes(), &enc); err != nil {
				t.Fatal(err)
			}
			raw, err := officialhttp.DecryptMessage(cfg.AESKey, cfg.AppID, enc.Encrypt)
			if err != nil {
				t.Fatal(err)
			}
			var out struct {
				Content string `xml:"Content"`
			}
			if err = xml.Unmarshal(raw, &out); err != nil {
				t.Fatal(err)
			}
			return out.Content
		}
		code, err := r.CreateCode(ctx, "app", contextFor(customer))
		if err != nil {
			t.Fatal(err)
		}
		bind := "<xml><ToUserName>gh_test</ToUserName><FromUserName>oa-callback</FromUserName><CreateTime>12</CreateTime><MsgType>text</MsgType><MsgId>123</MsgId><Content>绑定 " + code + "</Content></xml>"
		if got := call(bind); !strings.Contains(got, "绑定成功") {
			t.Fatal(got)
		}
		if got := call(bind); !strings.Contains(got, "已处理") {
			t.Fatal("duplicate bind", got)
		}
		event := func(key string) string {
			return "<xml><ToUserName>gh_test</ToUserName><FromUserName>oa-callback</FromUserName><CreateTime>13</CreateTime><MsgType>event</MsgType><Event>CLICK</Event><EventKey>" + key + "</EventKey></xml>"
		}
		if got := call(event("ORDERS_RECENT_1")); !strings.Contains(got, "小程序") || strings.Contains(got, "ORDER-") {
			t.Fatal("recent 1", got)
		}
		if got := call(event("ORDERS_RECENT_3")); !strings.Contains(got, "小程序") || strings.Contains(got, "ORDER-") {
			t.Fatal("recent 3", got)
		}
		if err = r.ChangeBinding(ctx, "app", "oa-callback", user, 0, false, "test"); err != nil {
			t.Fatal(err)
		}
		if got := call(event("ORDERS_RECENT_3")); strings.Contains(got, "ORDER-") || !strings.Contains(got, "小程序") {
			t.Fatal("duplicate callback leaked after revoke", got)
		}
	})

	t.Run("callback idempotency and menu audit persistence", func(t *testing.T) {
		a, err := r.FirstEvent(ctx, "app", "event1")
		if err != nil || !a {
			t.Fatal(a, err)
		}
		b, err := r.FirstEvent(ctx, "app", "event1")
		if err != nil || b {
			t.Fatal("duplicate accepted")
		}
		m := json.RawMessage(`{"button":[{"name":"最近一次","type":"click","key":"ORDERS_RECENT_1"}]}`)
		if _, err = r.SaveMenu(ctx, "app", "previous", "test", m); err != nil {
			t.Fatal(err)
		}
		rows, err := r.Menus(ctx, "app")
		if err != nil || len(rows) != 1 || rows[0].Status != "previous" {
			t.Fatal(rows, err)
		}
	})
}
