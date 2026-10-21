package stock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	stockapp "orderapp/internal/application/stock"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
)

func TestVirtualWarehousePostgresAPICompleteLifecycle(t *testing.T) {
	pool, schema := newStockHTTPPostgresTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	mustExecStockHTTPPostgres(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.customers(id BIGINT PRIMARY KEY,name TEXT,active BOOLEAN DEFAULT true);
INSERT INTO %[1]s.customers(id,name)VALUES(1,'客户甲'),(2,'客户乙');
CREATE TABLE %[1]s.materials(id BIGINT PRIMARY KEY,name TEXT,unit TEXT DEFAULT 'g',kind TEXT DEFAULT 'bean',purchase_price NUMERIC DEFAULT 0,sale_price NUMERIC DEFAULT 0,onhand_g BIGINT DEFAULT 0,onhand_units BIGINT DEFAULT 0);
CREATE TABLE %[1]s.products(id BIGINT PRIMARY KEY,name TEXT);
CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
`, schema))
	if err := postgresstock.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(postgresstock.NewRepository(pool, schema))})
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s status=%d body=%s want=%d", method, path, rec.Code, rec.Body.String(), status)
		}
		return rec
	}
	created := request("POST", "/api/stock/virtual-warehouses", `{"name":"代发成品仓","kind":"finished","customer_id":1}`, 200)
	var row stockapp.WarehouseRow
	if err := json.Unmarshal(created.Body.Bytes(), &row); err != nil {
		t.Fatal(err)
	}
	if row.Code == "" || !row.IsVirtual || row.Revision != 1 {
		t.Fatalf("created=%+v", row)
	}
	path := "/api/stock/virtual-warehouses/" + row.Code
	list := request("GET", "/api/stock/virtual-warehouses?customer_id=1&page=1&limit=1", "", 200)
	var result stockapp.VirtualWarehouseResult
	if err := json.Unmarshal(list.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Page != 1 || result.TotalPages != 1 || len(result.Rows) != 1 {
		t.Fatalf("list=%+v", result)
	}
	request("PUT", path, `{"name":"编辑后","description":"说明","customer_id":2,"expected_revision":1}`, 200)
	request("PUT", path, `{"name":"旧修改","customer_id":1,"expected_revision":1}`, 409)
	request("PUT", path, `{"name":"改类型","kind":"raw","customer_id":2,"expected_revision":2}`, 400)
	request("DELETE", path, "", 400)
	request("DELETE", path+"?expected_revision=1", "", 409)
	request("DELETE", path+"?expected_revision=2", "", 200)
	request("PUT", path, `{"name":"编辑删除仓","customer_id":2,"expected_revision":3}`, 404)
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE meta->>'warehouse'=$1`, schema), row.Code).Scan(&count); err != nil || count != 3 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
	request("POST", "/api/stock/virtual-warehouses", `{"name":"无效客户仓","kind":"finished","customer_id":999}`, 400)
	request("DELETE", "/api/stock/virtual-warehouses/finished_goods?expected_revision=1", "", 404)
}
