package appmain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"net/url"
	portal "orderapp/internal/application/customerportal"
	app "orderapp/internal/application/pageentry"
	portalpg "orderapp/internal/infrastructure/postgres/customerportal"
	officialpg "orderapp/internal/infrastructure/postgres/officialaccount"
	pagepg "orderapp/internal/infrastructure/postgres/pageentry"
	pages "orderapp/internal/interfaces/http/pageentry"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPageEntryAccessPostgres(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated test PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_page_access_%d", time.Now().UnixNano())
	t.Cleanup(func() { pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	if err = ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, fmt.Sprintf(q, schema), args...); err != nil {
			t.Fatal(err)
		}
	}
	id := func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(q, schema), args...).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	r := pagepg.NewRepository(pool, schema)
	portalRepo := portalpg.NewRepository(pool, schema)
	official := officialpg.NewRepository(pool, schema)
	a := id(`INSERT INTO %s.customers(name) VALUES('页面客户甲') RETURNING id`)
	b := id(`INSERT INTO %s.customers(name) VALUES('页面客户乙') RETURNING id`)
	user := id(`INSERT INTO %s.mini_users(openid,unionid) VALUES('page-mini','page-union') RETURNING id`)
	exec(`INSERT INTO %s.customer_portal_profiles(customer_id) VALUES($1),($2)`, a, b)
	exec(`INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id) VALUES($1,$2),($1,$3)`, user, a, b)
	exec(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code) VALUES($1,'product_order'),($1,'bean_list'),($2,'product_order'),($2,'bean_list')`, a, b)
	exec(`INSERT INTO %s.wechat_bindings(app_id,openid,mini_user_id,customer_id) VALUES('page-app','page-open',$1,$2)`, user, a)
	cfg := pages.Config{OAuthEnabled: true, UnionIDEnabled: true, PublicOrigin: "https://pages.example", AppID: "page-app", AppSecret: "test-only", Exchange: func(_ context.Context, code string) (pages.OAuthIdentity, error) {
		if code == "valid" {
			return pages.OAuthIdentity{OpenID: "page-open", UnionID: "page-union"}, nil
		}
		if code == "union" {
			return pages.OAuthIdentity{OpenID: "auto-open", UnionID: "page-union"}, nil
		}
		return pages.OAuthIdentity{}, fmt.Errorf("authorization failed")
	}}
	e := echo.New()
	pages.RegisterRoutes(e, pool, schema, portal.NewService(portalRepo, nil), wechatTestAuthz{}, cfg)
	call := func(method, path, body string, cookie *http.Cookie, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "https://pages.example"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://pages.example")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s got %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec
	}
	create := func(d app.Document) app.Entry {
		t.Helper()
		v, err := r.Create(ctx, d, "test")
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	change := func(v app.Entry, action string, d app.Document) app.Entry {
		t.Helper()
		v, err := r.Change(ctx, v.Key, v.Revision, action, d, "test")
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	article := create(app.Document{Name: "欢迎", Kind: "article", Visibility: "authenticated", Blocks: []app.Block{{Kind: "text", Text: "会员内容"}}})
	article = change(article, "publish", app.Document{})
	route := "/api/pages/" + article.Key
	call("GET", route, "", nil, 401)
	authorize := func(code string) *http.Cookie {
		t.Helper()
		start := call("GET", "/api/page-auth/wechat?entry="+article.Key, "", nil, 303)
		u, _ := url.Parse(start.Header().Get("Location"))
		state := u.Query().Get("state")
		if u.Query().Get("redirect_uri") != "https://pages.example/app/api/page-auth/callback" {
			t.Fatal("wrong callback")
		}
		cookie := start.Result().Cookies()[0]
		call("GET", "/api/page-auth/callback?code="+code+"&state="+state, "", nil, 400) // Wrong browser cannot consume valid state.
		callback := call("GET", "/api/page-auth/callback?code="+code+"&state="+state, "", cookie, 303)
		if callback.Header().Get("Location") != "/app/p/"+article.Key+"?auth=done" {
			t.Fatal("login return path", callback.Header())
		}
		call("GET", "/api/page-auth/callback?code="+code+"&state="+state, "", cookie, 400)
		for _, c := range callback.Result().Cookies() {
			if c.Name == "__Host-kferp_page" {
				if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
					t.Fatal("unsafe session cookie")
				}
				return c
			}
		}
		t.Fatal("no session")
		return nil
	}
	cookie := authorize("valid")
	call("GET", route, "", cookie, 403) // Multiple customers require selection.
	call("POST", "/api/page-auth/customer", fmt.Sprintf(`{"customer_id":%d}`, a), cookie, 200)
	call("GET", route, "", cookie, 200)
	call("POST", "/api/page-auth/customer", `{"customer_id":999999}`, cookie, 403)
	binding, _ := official.Binding(ctx, "page-app", "page-open")
	if binding.CustomerID != a {
		t.Fatal("unexpected binding")
	}
	call("POST", "/api/page-auth/customer", fmt.Sprintf(`{"customer_id":%d}`, b), cookie, 200)
	binding, _ = official.Binding(ctx, "page-app", "page-open")
	if binding.CustomerID != a {
		t.Fatal("web selection overwrote message query customer")
	}
	t.Run("four actual price targets independent of versions and ownership", func(t *testing.T) {
		pub := func(table, owner, key string) int64 {
			return id(`INSERT INTO %s.bean_list_publications(list_type,publication_table_key,publication_table_name,version_no,owner_type,owner_key,config_json,content_json) VALUES('commercial',$1,$1,'v1',$2,$3,'{"title":"价格表"}','{"groups":[{"category":"熟豆","items":[{"name":"咖啡","prices":[{"label":"227g","value":"50"}]}]}]}') RETURNING id`, table, owner, key)
		}
		var price app.Entry
		for i := 0; i < 4; i++ {
			d := app.Document{Name: fmt.Sprintf("表%d", i), Kind: "price", Visibility: "public", PublicationID: pub(fmt.Sprint(i), "official", "")}
			price = create(d)
			price = change(price, "publish", app.Document{})
			call("GET", "/api/pages/"+price.Key, "", nil, 200)
		}
		all, _ := r.List(ctx)
		before := len(all)
		private := pub("私有表", "customer", fmt.Sprint(a))
		pub("更多版本", "official", "")
		if err := ensureAppSchema(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
		all, _ = r.List(ctx)
		if len(all) != before {
			t.Fatal("automatic entries")
		}
		d := price.Draft
		d.PublicationID = private
		if _, err := r.Change(ctx, price.Key, price.Revision, "save", d, "test"); err == nil {
			t.Fatal("private price accepted as public")
		}
		d.Visibility = "authenticated"
		d.Name = "修改目标"
		key := price.Key
		price = change(price, "save", d)
		price = change(price, "publish", d)
		if price.Key != key {
			t.Fatal("path changed")
		}
		call("GET", "/api/pages/"+price.Key, "", cookie, 403)
		call("POST", "/api/page-auth/customer", fmt.Sprintf(`{"customer_id":%d}`, a), cookie, 200)
		call("GET", "/api/pages/"+price.Key, "", cookie, 200)
		exec(`UPDATE %s.bean_list_publications SET status='withdrawn' WHERE id=$1`, private)
		call("GET", "/api/pages/"+price.Key, "", cookie, 404)
	})
	t.Run("published image membership and live permissions", func(t *testing.T) {
		image, err := r.SaveAsset(ctx, article.Key, "image/png", []byte("test-image"), "test")
		if err != nil {
			t.Fatal(err)
		}
		imageURL := route + "/images/" + image
		call("GET", imageURL, "", cookie, 404)
		d := article.Draft
		d.Blocks = append(d.Blocks, app.Block{Kind: "image", AssetID: image})
		article = change(article, "save", d)
		call("GET", imageURL, "", cookie, 404)
		article = change(article, "publish", d)
		call("GET", imageURL, "", cookie, 200)
		call("GET", imageURL, "", nil, 401)
		other := create(app.Document{Name: "其他页", Kind: "article", Visibility: "authenticated"})
		other.Draft.Blocks = []app.Block{{Kind: "image", AssetID: image}}
		if _, err := r.Change(ctx, other.Key, other.Revision, "save", other.Draft, "test"); err == nil {
			t.Fatal("foreign image attached")
		}
		article = change(article, "disable", d)
		call("GET", imageURL, "", cookie, 404)
		article = change(article, "publish", d)
		exec(`UPDATE %s.wechat_bindings SET active=false WHERE app_id='page-app' AND openid='page-open'`)
		call("GET", imageURL, "", cookie, 401)
	})
	t.Run("trusted union identity and account revocation", func(t *testing.T) {
		auto := authorize("union")
		call("POST", "/api/page-auth/customer", fmt.Sprintf(`{"customer_id":%d}`, a), auto, 200)
		call("GET", route, "", auto, 200)
		exec(`UPDATE %s.mini_users SET active=false WHERE id=$1`, user)
		call("GET", route, "", auto, 401)
		exec(`UPDATE %s.mini_users SET active=true WHERE id=$1`, user)
	})
	t.Run("one time expired state and login rate limit", func(t *testing.T) {
		state, _ := app.NewKey()
		browser, _ := app.NewKey()
		if err := r.OAuthState(ctx, state, browser, article.Key); err != nil {
			t.Fatal(err)
		}
		exec(`UPDATE %s.page_oauth_states SET expires_at=now()-interval '1 second'`)
		if _, err := r.ConsumeState(ctx, state, browser); err == nil {
			t.Fatal("expired state")
		}
		for i := 0; i < 11; i++ {
			ok, err := r.AllowLogin(ctx, "test-limit")
			if err != nil || ok != (i < 10) {
				t.Fatal("rate limit", i, ok, err)
			}
		}
		call("GET", "/api/page-auth/wechat?entry=https://evil.example", "", nil, 400)
	})
	t.Run("password fallback separate from official binding and CSRF", func(t *testing.T) {
		employee := id(`INSERT INTO %[1]s.company_employees(name,phone,account_type,department_id,active) VALUES('客户密码账号','13800138099','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true) RETURNING id`)
		hash := sha256.Sum256([]byte("orderapp-mobile-auth:test-password"))
		exec(`INSERT INTO %s.employee_login_passwords(employee_id,password_hash) VALUES($1,$2)`, employee, hex.EncodeToString(hash[:]))
		exec(`INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,role,status,updated_by) VALUES($1,$2,'owner','active','test')`, a, employee)
		before := id(`SELECT count(*) FROM %s.wechat_bindings`)
		req := httptest.NewRequest("POST", "https://pages.example/api/page-auth/login", bytes.NewBufferString(`{"login":"13800138099","password":"test-password"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 403 {
			t.Fatal("CSRF accepted")
		}
		call("POST", "/api/page-auth/login", `{"login":"13800138099","password":"wrong"}`, nil, 401)
		res := call("POST", "/api/page-auth/login", `{"login":"13800138099","password":"test-password"}`, nil, 200)
		web := res.Result().Cookies()[0]
		if strings.Contains(res.Body.String(), "token") {
			t.Fatal("session token exposed")
		}
		call("GET", route, "", web, 200)
		if id(`SELECT count(*) FROM %s.wechat_bindings`) != before {
			t.Fatal("password login created official binding")
		}
		exec(`UPDATE %s.employee_login_passwords SET login_disabled=true,updated_at=now() WHERE employee_id=$1`, employee)
		call("GET", route, "", web, 401)
	})
	t.Run("authorization failure returns only to original local page", func(t *testing.T) {
		start := call("GET", "/api/page-auth/wechat?entry="+article.Key, "", nil, 303)
		u, _ := url.Parse(start.Header().Get("Location"))
		browser := start.Result().Cookies()[0]
		callback := call("GET", "/api/page-auth/callback?code=invalid&state="+u.Query().Get("state")+"&return_to=https://evil.example", "", browser, 303)
		if callback.Header().Get("Location") != "/app/p/"+article.Key+"?auth=password" {
			t.Fatal("unsafe fallback", callback.Header())
		}
	})
	t.Run("menu references survive drafts and publications", func(t *testing.T) {
		menu, _ := json.Marshal(map[string]any{"button": []any{map[string]string{"name": "页面", "type": "view", "url": "https://pages.example/app/p/" + article.Key}}})
		exec(`INSERT INTO %s.wechat_menu_versions(app_id,menu_json,status,actor) VALUES('page-app',$1,'published','test')`, menu)
		refs, err := r.References(ctx, article.Key)
		if err != nil || len(refs) != 1 || refs[0].Name != "页面" {
			t.Fatalf("refs=%+v err=%v", refs, err)
		}
	})
}
