package appmain

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	portal "orderapp/internal/application/customerportal"
	portalrepo "orderapp/internal/infrastructure/postgres/customerportal"
	pages "orderapp/internal/interfaces/http/pageentry"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPageEntryPostgres(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_pages_%d", time.Now().UnixNano())
	t.Cleanup(func() { pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	if err = ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("X-Test-Admin") == "yes" {
				c.Set("employee_id", int64(93))
			}
			return next(c)
		}
	})
	pages.RegisterRoutes(e, pool, schema, portal.NewService(portalrepo.NewRepository(pool, schema), nil), wechatTestAuthz{}, pages.Config{})
	request := func(method, path, body string, admin bool, want int) map[string]json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if admin {
			req.Header.Set("X-Test-Admin", "yes")
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		out := map[string]json.RawMessage{}
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	base := "/api/admin/page-entries"
	request("GET", base, "", false, 403)
	out := request("GET", base, "", true, 200)
	if string(out["rows"]) != "[]" {
		t.Fatalf("must start empty: %s", out["rows"])
	}
	var key string
	for i := 0; i < 4; i++ {
		out = request("POST", base, fmt.Sprintf(`{"draft":{"name":"手工页%d","kind":"article","visibility":"public","blocks":[{"kind":"text","text":"已发布"}]}}`, i), true, 201)
		json.Unmarshal(out["key"], &key)
	}
	out = request("GET", base, "", true, 200)
	var rows []any
	json.Unmarshal(out["rows"], &rows)
	if len(rows) != 4 {
		t.Fatal("manual count", len(rows))
	}
	// Published snapshots, including renamed types and additional versions, create no entries.
	for i := 0; i < 6; i++ {
		_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,classification_template_id,classification_template_name,publication_table_key,version_no) VALUES('commercial',701,'咖啡豆',$1,$2)`, schema), fmt.Sprint(i), fmt.Sprint(i))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	var count int
	pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".wechat_price_entries").Scan(&count)
	if count != 0 {
		t.Fatal("legacy generation still active", count)
	}
	out = request("GET", base, "", true, 200)
	json.Unmarshal(out["rows"], &rows)
	if len(rows) != 4 {
		t.Fatal("automatic page generated")
	}
	url := base + "/" + key
	public := "/api/pages/" + key
	request("GET", public, "", false, 404)
	out = request("POST", url+"/publish", `{"revision":1}`, true, 200)
	request("GET", public, "", false, 200)
	out = request("PUT", url, `{"revision":2,"draft":{"name":"改名","kind":"article","visibility":"public","blocks":[{"kind":"text","text":"草稿秘密"}]}}`, true, 200)
	if string(out["key"]) != `"`+key+`"` {
		t.Fatal("path changed")
	}
	out = request("GET", public, "", false, 200)
	if strings.Contains(string(out["document"]), "草稿秘密") {
		t.Fatal("draft leaked")
	}
	request("POST", url+"/publish", `{"revision":2}`, true, 409)
	request("POST", url+"/disable", `{"revision":3}`, true, 200)
	request("GET", public, "", false, 404)
	request("DELETE", url, `{"revision":4}`, true, 200)
	request("GET", public, "", false, 404)
}
