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
	t.Run("canonical entries are product type by configured use", func(t *testing.T) {
		typeID := int64(71001)
		typedPublication := func(table, version, owner, ownerKey, typeName string, productTypeID int64) int64 {
			return id(`INSERT INTO %s.bean_list_publications(list_type,product_type_category_id,product_type_name,publication_table_key,publication_table_name,version_no,owner_type,owner_key,config_json,content_json)
				VALUES('commercial',$1,$2,$3,$3,$4,$5,$6,'{"title":"测试豆单"}','{"groups":[{"category":"熟豆","items":[]}]}') RETURNING id`, productTypeID, typeName, table, version, owner, ownerKey)
		}
		first := typedPublication("coffee-wholesale", "v1", "official", "", "咖啡豆", typeID)
		second := typedPublication("coffee-direct-ship", "v2", "official", "", "咖啡豆", typeID)
		private := typedPublication("coffee-customer", "v1", "customer", fmt.Sprint(customer), "咖啡豆", typeID)
		typedPublication("drip-wholesale", "v1", "official", "", "挂耳咖啡", typeID+1)
		typedPublication("instant-direct", "v1", "official", "", "速溶咖啡", typeID+2)
		if err = repo.EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatalf("re-running the migration failed: %v", err)
		}
		var legacyEntryCount int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.wechat_price_entries WHERE type_key=''`, schema)).Scan(&legacyEntryCount); err != nil {
			t.Fatal(err)
		}
		if legacyEntryCount != 0 {
			t.Fatalf("new publications must not create legacy per-table entries, got %d", legacyEntryCount)
		}
		rows, err := r.Entries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 6 {
			t.Fatalf("three product types must create six entries, got %d: %+v", len(rows), rows)
		}
		seen := map[string]bool{}
		var wholesale app.Entry
		var directShip app.Entry
		for _, row := range rows {
			if row.PublicationID != 0 || row.Enabled || row.Visibility != "authenticated" {
				t.Fatalf("new entry must remain unconfigured and disabled: %+v", row)
			}
			seen[row.TypeKey+"/"+row.Purpose] = true
			if row.TypeKey == fmt.Sprintf("product-type:%d", typeID) && row.Purpose == "wholesale" {
				wholesale = row
			}
			if row.TypeKey == fmt.Sprintf("product-type:%d", typeID) && row.Purpose == "direct_ship" {
				directShip = row
			}
		}
		for _, categoryID := range []int64{typeID, typeID + 1, typeID + 2} {
			for _, purpose := range []string{"wholesale", "direct_ship"} {
				if !seen[fmt.Sprintf("product-type:%d/%s", categoryID, purpose)] {
					t.Fatalf("missing type/use entry %d/%s", categoryID, purpose)
				}
			}
		}
		for _, added := range []int64{
			typedPublication("coffee-wholesale-copy", "v2", "official", "", "咖啡豆", typeID),
			typedPublication("coffee-other-customer", "v1", "customer", fmt.Sprint(other), "咖啡豆", typeID),
		} {
			_ = added
		}
		rows, err = r.Entries(ctx)
		if err != nil || len(rows) != 6 {
			t.Fatalf("new tables, versions or owners must not create entries: count=%d err=%v", len(rows), err)
		}
		pair, err := r.EntriesForPublication(ctx, first)
		if err != nil || len(pair) != 2 {
			t.Fatalf("a price table shortcut must show its two type entries: %+v err=%v", pair, err)
		}
		versions, err := r.Versions(ctx, wholesale.Key)
		if err != nil || len(versions) < 4 {
			t.Fatalf("same type candidates across tables missing: %+v err=%v", versions, err)
		}
		configured := wholesale
		configured.PublicationID, configured.Enabled, configured.Visibility = second, true, "public"
		configured, err = r.SaveEntry(ctx, wholesale.Key, configured, "test")
		if err != nil || configured.PublicationID != second || configured.TableName != "coffee-direct-ship" || configured.Version != "v2" {
			t.Fatalf("same type table/version must be selectable: %+v err=%v", configured, err)
		}
		directShip.PublicationID, directShip.Enabled = first, true
		directShip.Purpose = "wholesale" // Client input must not override the authoritative entry purpose.
		directShip, err = r.SaveEntry(ctx, directShip.Key, directShip, "test")
		if err != nil || directShip.PublicationID != first || directShip.Purpose != "direct_ship" {
			t.Fatalf("the second use must be independently configurable and retain its purpose: %+v err=%v", directShip, err)
		}
		var auditedPurpose string
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT meta->>'purpose' FROM %s.audit_logs WHERE entity_type='wechat_official_account' AND action='entry_update' ORDER BY id DESC LIMIT 1`, schema)).Scan(&auditedPurpose); err != nil || auditedPurpose != "direct_ship" {
			t.Fatalf("audit must record the stored purpose, got %q err=%v", auditedPurpose, err)
		}
		configured, err = r.Entry(ctx, wholesale.Key)
		if err != nil || configured.PublicationID != second {
			t.Fatalf("saving one use changed the other use: %+v err=%v", configured, err)
		}
		wrongType := configured
		wrongType.PublicationID = id(`SELECT id FROM %s.bean_list_publications WHERE product_type_category_id=$1 ORDER BY id LIMIT 1`, typeID+1)
		if _, err = r.SaveEntry(ctx, wholesale.Key, wrongType, "test"); err == nil {
			t.Fatal("cross-type publication was accepted")
		}
		privateSelection := configured
		privateSelection.PublicationID, privateSelection.Visibility = private, "public"
		if _, err = r.SaveEntry(ctx, wholesale.Key, privateSelection, "test"); err == nil {
			t.Fatal("customer-private publication became public")
		}
		originalKey := configured.Key
		exec(`UPDATE %s.bean_list_publications SET product_type_name='咖啡豆新名称' WHERE product_type_category_id=$1`, typeID)
		if err = repo.EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
		refreshed, err := r.EntriesForPublication(ctx, first)
		if err != nil || len(refreshed) != 2 || refreshed[0].Key != originalKey && refreshed[1].Key != originalKey {
			t.Fatalf("renaming the product type changed its fixed path: %+v err=%v", refreshed, err)
		}

		api := echo.New()
		api.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("employee_id", int64(93))
				return next(c)
			}
		})
		officialhttp.RegisterRoutes(api, pool, schema, portal.NewService(r.Portal, nil), wechatTestAuthz{}, officialhttp.Config{})
		requestAPI := func(method, path string, body []byte) *httptest.ResponseRecorder {
			t.Helper()
			req := httptest.NewRequest(method, path, strings.NewReader(string(body)))
			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			res := httptest.NewRecorder()
			api.ServeHTTP(res, req)
			return res
		}
		pairResponse := requestAPI(http.MethodGet, fmt.Sprintf("/api/customer-portal/admin/wechat/entries?publication_id=%d", first), nil)
		var pairBody struct {
			Rows []app.Entry `json:"rows"`
		}
		if pairResponse.Code != http.StatusOK || json.Unmarshal(pairResponse.Body.Bytes(), &pairBody) != nil || len(pairBody.Rows) != 2 {
			t.Fatalf("price-table API must return both type uses: %d %s", pairResponse.Code, pairResponse.Body.String())
		}
		listResponse := requestAPI(http.MethodGet, "/api/customer-portal/admin/wechat/entries", nil)
		var listBody struct {
			Rows []app.Entry `json:"rows"`
		}
		if listResponse.Code != http.StatusOK || json.Unmarshal(listResponse.Body.Bytes(), &listBody) != nil || len(listBody.Rows) != 6 {
			t.Fatalf("new menu API must expose only canonical type/use rows: %d %s", listResponse.Code, listResponse.Body.String())
		}
		apiCrossType := pairBody.Rows[0]
		apiCrossType.PublicationID = id(`SELECT id FROM %s.bean_list_publications WHERE product_type_category_id=$1 ORDER BY id LIMIT 1`, typeID+2)
		payload, _ := json.Marshal(apiCrossType)
		badSave := requestAPI(http.MethodPut, "/api/customer-portal/admin/wechat/entries/"+apiCrossType.Key, payload)
		if badSave.Code != http.StatusConflict {
			t.Fatalf("cross-type API save was accepted: %d %s", badSave.Code, badSave.Body.String())
		}
		classificationID := typeID + 1000
		classifiedPublication := func(table, version string, legacyTypeID int64) int64 {
			return id(`INSERT INTO %s.bean_list_publications(list_type,product_type_category_id,product_type_name,classification_template_id,classification_template_name,publication_table_key,publication_table_name,version_no,owner_type,owner_key,config_json,content_json)
				VALUES('commercial',$1,'旧类型名称',$2,'当前分类类型',$3,$3,$4,'official','', '{"title":"分类豆单"}', '{"groups":[]}') RETURNING id`, legacyTypeID, classificationID, table, version)
		}
		classifiedFirst := classifiedPublication("classified-first", "v1", typeID+100)
		classifiedSecond := classifiedPublication("classified-second", "v2", typeID+200)
		rows, err = r.Entries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var classifiedWholesale app.Entry
		classificationEntryCount := 0
		for _, row := range rows {
			if row.TypeKey == fmt.Sprintf("classification-template:%d", classificationID) {
				classificationEntryCount++
				if row.Purpose == "wholesale" {
					classifiedWholesale = row
				}
			}
		}
		if classificationEntryCount != 2 {
			t.Fatalf("current classification identity must override differing legacy type IDs; entries=%d rows=%+v", classificationEntryCount, rows)
		}
		classifiedVersions, versionErr := r.Versions(ctx, classifiedWholesale.Key)
		if versionErr != nil || len(classifiedVersions) != 2 || classifiedVersions[0].ID != classifiedSecond && classifiedVersions[1].ID != classifiedFirst {
			t.Fatalf("same classification versions not grouped: %+v err=%v", classifiedVersions, versionErr)
		}
		classifiedWholesale.PublicationID = classifiedSecond
		if saved, saveErr := r.SaveEntry(ctx, classifiedWholesale.Key, classifiedWholesale, "test"); saveErr != nil || saved.PublicationID != classifiedSecond {
			t.Fatalf("same current classification version rejected because of legacy IDs: %+v err=%v", saved, saveErr)
		}
		exec(`UPDATE %s.bean_list_publications SET status='withdrawn' WHERE id=$1`, first)
		withdrawn, err := r.Entry(ctx, directShip.Key)
		if err != nil || withdrawn.PublicationID != first || app.CheckEntry(withdrawn, nil) == nil {
			t.Fatalf("withdrawal changed the fixed target or remained visible: %+v err=%v", withdrawn, err)
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
			t.Fatal(err)
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
		if got := call(event("ORDERS_RECENT_1")); !strings.Contains(got, "ORDER-54") || strings.Contains(got, "ORDER-53") {
			t.Fatal("recent 1", got)
		}
		if got := call(event("ORDERS_RECENT_3")); !strings.Contains(got, "ORDER-52") || strings.Contains(got, "ORDER-51") {
			t.Fatal("recent 3", got)
		}
		if err = r.ChangeBinding(ctx, "app", "oa-callback", user, 0, false, "test"); err != nil {
			t.Fatal(err)
		}
		if got := call(event("ORDERS_RECENT_3")); strings.Contains(got, "ORDER-") || !strings.Contains(got, "认证") {
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
