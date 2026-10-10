package appmain

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	app "orderapp/internal/application/customerportal"
	page "orderapp/internal/application/pageentry"
	repo "orderapp/internal/infrastructure/postgres/customerportal"
	pagerepo "orderapp/internal/infrastructure/postgres/pageentry"
	portalhttp "orderapp/internal/interfaces/http/customerportal"
	pages "orderapp/internal/interfaces/http/pageentry"
	"os"
	"strings"
	"testing"
	"time"
)

type registrationIdentity struct{}

func (registrationIdentity) Resolve(_ context.Context, code string) (app.MiniIdentity, error) {
	return app.MiniIdentity{OpenID: code}, nil
}
func (registrationIdentity) ResolvePhoneNumber(_ context.Context, code string) (app.MiniPhoneNumber, error) {
	if code == "invalid" {
		return app.MiniPhoneNumber{}, app.ErrMiniInvalidLogin
	}
	return app.MiniPhoneNumber{PhoneNumber: strings.Split(code, ":")[0]}, nil
}
func TestBeanCenterRegistrationPostgres(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_beans_%d", time.Now().UnixNano())
	t.Cleanup(func() { pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	if err = ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	r := repo.NewRepository(pool, schema)
	svc := app.NewService(r, registrationIdentity{})
	pr := pagerepo.NewRepository(pool, schema)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("X-Test-Admin") == "yes" {
				c.Set("basic_auth_admin", true)
			}
			return next(c)
		}
	})
	portalhttp.RegisterRoutes(e, portalhttp.Dependencies{CustomerPortal: svc})
	pages.RegisterRoutes(e, pool, schema, svc, wechatTestAuthz{}, pages.Config{})
	call := func(method, path, body, token string, want int) map[string]json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token == "test-admin" {
			req.Header.Set("X-Test-Admin", "yes")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		var out map[string]json.RawMessage
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	var token string
	out := call("POST", "/api/mini/registration", `{"code":"visitor","phone_code":"13800000001:first","nickname":"新访客"}`, "", 200)
	json.Unmarshal(out["token"], &token)
	if string(out["registration_complete"]) != "true" || string(out["current_customer_id"]) != "0" {
		t.Fatal("visitor must register without customer", out)
	}
	call("POST", "/api/mini/registration", `{"code":"other","phone_code":"13800000001:first","nickname":"重放"}`, "", 400)
	call("POST", "/api/mini/registration", `{"code":"other","phone_code":"invalid","nickname":"无效"}`, "", 401)
	call("POST", "/api/mini/registration", `{"code":"other","phone":"13800000001","nickname":"伪造"}`, "", 400)
	call("GET", "/api/mini/services/orders", "", token, 403)
	call("PUT", "/api/mini/registration", `{"nickname":"新昵称","verified_phone":"fake"}`, token, 200)
	out = call("POST", "/api/mini/login", `{"code":"visitor","mode":"wechat","phone":"fake"}`, "", 200)
	json.Unmarshal(out["token"], &token)
	if !strings.Contains(string(out["registration"]), "13800000001") || !strings.Contains(string(out["registration"]), "新昵称") {
		t.Fatal("verified profile must persist independently", out)
	}
	var registrationAudits int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".audit_logs WHERE entity_type='mini_registration' AND action IN ('verify_phone','update_nickname')").Scan(&registrationAudits); err != nil || registrationAudits != 2 {
		t.Fatalf("registration and profile update must be audited: count=%d err=%v", registrationAudits, err)
	}
	if string(call("GET", "/api/mini/bean-center", "", "", 200)["rows"]) != "[]" {
		t.Fatal("initial catalogue not empty")
	}
	publication := func(owner, key, version string) int64 {
		t.Helper()
		var id int64
		err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,publication_table_key,publication_table_name,version_no,owner_type,owner_key,config_json,content_json) VALUES('commercial','table','表',$1,$2,$3,'{"title":"豆单"}','{"groups":[{"category":"豆","items":[{"name":"咖啡","prices":[{"label":"227g","value":"50"}]}]}]}') RETURNING id`, schema), version, owner, key).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	var entries []page.Entry
	for i := 0; i < 4; i++ {
		v, err := pr.Create(ctx, page.Document{Name: fmt.Sprint("表", i), Kind: "price", Visibility: "registered", PublicationID: publication("official", "", fmt.Sprint("v", i)), BeanCenter: true, BeanSort: 4 - i}, "test")
		if err != nil {
			t.Fatal(err)
		}
		v, err = pr.Change(ctx, v.Key, v.Revision, "publish", page.Document{}, "test")
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, v)
	}
	catalog := func() []map[string]any {
		t.Helper()
		var rows []map[string]any
		json.Unmarshal(call("GET", "/api/mini/bean-center", "", "", 200)["rows"], &rows)
		return rows
	}
	rows := catalog()
	if len(rows) != 4 || rows[0]["name"] != "表3" {
		t.Fatal("manual count/order", rows)
	}
	publication("official", "", "v99")
	if len(catalog()) != 4 {
		t.Fatal("publication added card")
	}
	entry := entries[0]
	path := "/api/pages/" + entry.Key
	call("GET", path, "", "", 401)
	call("GET", path, "", token, 200)
	private := publication("customer", "999", "private")
	draft := entry.Draft
	draft.PublicationID = private
	if _, err = pr.Change(ctx, entry.Key, entry.Revision, "save", draft, "test"); err == nil {
		t.Fatal("customer price accepted for registration")
	}
	draft = entry.Draft
	draft.Name = "草稿名称"
	draft.PublicationID = publication("official", "", "v2")
	entry, err = pr.Change(ctx, entry.Key, entry.Revision, "save", draft, "test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(call("GET", path, "", token, 200)["document"]), "草稿名称") {
		t.Fatal("draft leaked")
	}
	entry, err = pr.Change(ctx, entry.Key, entry.Revision, "publish", page.Document{}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(call("GET", path, "", token, 200)["document"]), "草稿名称") {
		t.Fatal("publication not switched")
	}
	entry, err = pr.Change(ctx, entry.Key, entry.Revision, "disable", page.Document{}, "test")
	if err != nil {
		t.Fatal(err)
	}
	call("GET", path, "", token, 404)
	if len(catalog()) != 3 {
		t.Fatal("disabled card")
	}
	pool.Exec(ctx, "UPDATE "+schema+".bean_list_publications SET status='withdrawn' WHERE id=$1", entries[1].Draft.PublicationID)
	call("GET", "/api/pages/"+entries[1].Key, "", token, 404)
	if len(catalog()) != 2 {
		t.Fatal("withdrawn card")
	}

	// The phone belongs to exactly one active account before any customer rights are projected.
	id := func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(q, schema), args...).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, fmt.Sprintf(q, schema), args...); err != nil {
			t.Fatal(err)
		}
	}
	customerA := id(`INSERT INTO %s.customers(name) VALUES('甲') RETURNING id`)
	customerB := id(`INSERT INTO %s.customers(name) VALUES('乙') RETURNING id`)
	exec(`INSERT INTO %s.customer_portal_profiles(customer_id) VALUES($1),($2)`, customerA, customerB)
	employee := id(`INSERT INTO %[1]s.company_employees(name,phone,account_type,department_id) VALUES('豆单测试账号','13800000002','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1)) RETURNING id`)
	exec(`INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,status) VALUES($1,$2,'active')`, customerA, employee)
	multiUser := id(`INSERT INTO %s.mini_users(openid) VALUES('customer-user') RETURNING id`)
	exec(`INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,status,approved_by) VALUES($1,$2,'approved','admin')`, multiUser, customerB)
	exec(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code) VALUES($1,'product_order'),($2,'product_order'),($1,'bean_list'),($2,'bean_list')`, customerA, customerB)
	out = call("POST", "/api/mini/registration", `{"code":"customer-user","phone_code":"13800000002:first","nickname":"两客户"}`, "", 200)
	var customerToken string
	json.Unmarshal(out["token"], &customerToken)
	if string(out["current_customer_id"]) != "0" {
		t.Fatal("multiple customers must explicitly select")
	}
	call("GET", "/api/mini/services/orders", "", customerToken, 403)
	call("POST", "/api/mini/current-customer", fmt.Sprintf(`{"customer_id":%d}`, customerA), customerToken, 200)
	call("GET", "/api/mini/services/orders?page=2&page_size=20", "", customerToken, 200)
	call("POST", "/api/mini/current-customer", `{"customer_id":999999}`, customerToken, 403)
	// A customer-owned price still requires its actual owner, even after registration.
	ownedID := publication("customer", fmt.Sprint(customerB), "private-v1")
	owned, err := pr.Create(ctx, page.Document{Name: "专属", Kind: "price", Visibility: "authenticated", PublicationID: ownedID}, "test")
	if err != nil {
		t.Fatal(err)
	}
	owned, err = pr.Change(ctx, owned.Key, owned.Revision, "publish", page.Document{}, "test")
	if err != nil {
		t.Fatal(err)
	}
	call("GET", "/api/pages/"+owned.Key, "", token, 403)
	call("GET", "/api/pages/"+owned.Key, "", customerToken, 403)
	call("POST", "/api/mini/current-customer", fmt.Sprintf(`{"customer_id":%d}`, customerB), customerToken, 200)
	call("GET", "/api/pages/"+owned.Key, "", customerToken, 200)
	// Simulate legacy duplicate phone data in this isolated schema only; production keeps its unique index.
	exec(`DROP INDEX %s.company_employees_phone_uq`)
	id(`INSERT INTO %[1]s.company_employees(name,phone,account_type,department_id) VALUES('重号测试账号','13800000002','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1)) RETURNING id`)
	out = call("POST", "/api/mini/registration", `{"code":"ambiguous","phone_code":"13800000002:again","nickname":"重号访客"}`, "", 200)
	if string(out["current_customer_id"]) != "0" || string(out["registration_complete"]) != "true" || string(out["bindings"]) != "[]" {
		t.Fatal("ambiguous phone gained permissions", out)
	}
	call("GET", "/api/pages/"+entries[2].Key+"/images/00000000000000000000000000000000", "", "", 401)
	call("GET", "/api/pages/"+entries[2].Key+"/images/00000000000000000000000000000000", "", token, 404)
	call("GET", "/api/customer-portal/admin/registrations", "", "", 403)
	// Session expiration restores only verified registration, without resubmitting a phone credential.
	exec(`UPDATE %s.mini_sessions SET expire_at=now() WHERE token=$1`, token)
	call("GET", "/api/mini/me", "", token, 401)
	out = call("POST", "/api/mini/login", `{"code":"visitor","mode":"wechat"}`, "", 200)
	json.Unmarshal(out["token"], &token)
	if string(out["registration_complete"]) != "true" {
		t.Fatal("expired registration not restored")
	}
	cur, err := svc.Me(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	out = call("GET", "/api/customer-portal/admin/registrations?q=13800000001&page=1&page_size=1", "", "test-admin", 200)
	if string(out["total"]) != "1" || !strings.Contains(string(out["rows"]), "新昵称") {
		t.Fatal("admin search/page", out)
	}
	call("POST", fmt.Sprintf("/api/customer-portal/admin/registrations/%d/disable", cur.MiniUserID), "{}", "", 403)
	call("POST", fmt.Sprintf("/api/customer-portal/admin/registrations/%d/disable", cur.MiniUserID), "{}", "test-admin", 200)
	call("GET", "/api/pages/"+entries[2].Key, "", token, 401)
	call("POST", "/api/mini/registration", `{"code":"visitor","phone_code":"13800000001:again","nickname":"再登记"}`, "", 403)
}
