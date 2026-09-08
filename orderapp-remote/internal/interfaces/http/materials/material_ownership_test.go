package materials

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	materialsapp "orderapp/internal/application/materials"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"

	"github.com/labstack/echo/v4"
)

func TestMaterialOwnershipCreateFilterChangeGuardAndLegacyEndpointRetirement(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	var customerA, customerB int64
	for _, item := range []struct {
		name string
		id   *int64
	}{{"PR639客户A", &customerA}, {"PR639客户B", &customerB}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES($1,true) RETURNING id`, schema), item.name).Scan(item.id); err != nil {
			t.Fatal(err)
		}
	}
	var factorySelfCustomerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES('工厂自营',true) RETURNING id`, schema)).Scan(&factorySelfCustomerID); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("actor", "pr639-test"); return next(c) }
	})
	registerMaterialsAPI(e, materialsapp.NewService(postgresmaterials.NewRepository(pool, schema)))

	create := func(code, ownerType string, customerID int64) (materialsapp.Material, *httptest.ResponseRecorder) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"code": code, "name": "同规格豆袋", "kind": "pack", "unit": "件", "cost_unit": "件", "owner_type": ownerType, "owner_customer_id": customerID})
		req := httptest.NewRequest(http.MethodPost, "/api/materials", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		var row materialsapp.Material
		_ = json.Unmarshal(rec.Body.Bytes(), &row)
		return row, rec
	}
	if _, rec := create("PR639-MISSING", "", 0); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "物料归属") {
		t.Fatalf("missing owner status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, rec := create("PR639-FACTORY-SELF", "customer", factorySelfCustomerID); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "本公司归属") {
		t.Fatalf("factory self pseudo-customer status=%d body=%s", rec.Code, rec.Body.String())
	}
	factory, rec := create("PR639-F", "factory", 0)
	if rec.Code != http.StatusOK {
		t.Fatalf("factory status=%d body=%s", rec.Code, rec.Body.String())
	}
	a, rec := create("PR639-A", "customer", customerA)
	if rec.Code != http.StatusOK || a.OwnerCustomerID != customerA || a.OwnerName != "PR639客户A" {
		t.Fatalf("A row=%+v status=%d body=%s", a, rec.Code, rec.Body.String())
	}
	b, rec := create("PR639-B", "customer", customerB)
	if rec.Code != http.StatusOK || b.OwnerCustomerID != customerB {
		t.Fatalf("B row=%+v status=%d", b, rec.Code)
	}
	if factory.ID == a.ID || a.ID == b.ID {
		t.Fatal("owner-specific materials must have independent ids")
	}
	copyBody, _ := json.Marshal(map[string]any{
		"code":                    "PR639-COPY",
		"name":                    "复制档案",
		"kind":                    "pack",
		"unit":                    "件",
		"cost_unit":               "件",
		"owner_type":              "customer",
		"owner_customer_id":       customerA,
		"copied_from_material_id": factory.ID,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/materials", bytes.NewReader(copyBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("copy create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var copied materialsapp.Material
	if err := json.Unmarshal(rec.Body.Bytes(), &copied); err != nil {
		t.Fatal(err)
	}
	if copied.ID == factory.ID || copied.OwnerCustomerID != customerA || copied.OnhandUnits != 0 || copied.PurchasePrice != 0 {
		t.Fatalf("copied material must be independent without inventory or cost: %+v", copied)
	}
	var copyAuditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='material' AND entity_id=$1 AND action='copy' AND field='copied_from_material_id'`, schema), copied.ID).Scan(&copyAuditCount); err != nil || copyAuditCount != 1 {
		t.Fatalf("copy audit count=%d err=%v", copyAuditCount, err)
	}
	legacyBody, _ := json.Marshal(map[string]any{"code": "PR639-LEGACY", "name": "旧多客户请求", "kind": "pack", "unit": "件", "cost_unit": "件", "owner_type": "customer", "owner_customer_id": customerA, "customer_ids": []int64{customerA, customerB}})
	req = httptest.NewRequest(http.MethodPost, "/api/materials", bytes.NewReader(legacyBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "多客户") {
		t.Fatalf("legacy multi-customer create status=%d body=%s", rec.Code, rec.Body.String())
	}

	for _, statement := range []string{
		fmt.Sprintf(`CREATE TABLE %s.customer_erp_user_bindings(id BIGSERIAL PRIMARY KEY,customer_id BIGINT NOT NULL,employee_id BIGINT NOT NULL,status TEXT NOT NULL DEFAULT 'active')`, schema),
		fmt.Sprintf(`CREATE TABLE %s.stock_entry_items(id BIGSERIAL PRIMARY KEY,material_id BIGINT NOT NULL)`, schema),
		fmt.Sprintf(`INSERT INTO %s.company_employees(name,phone,account_type,department_id,active) SELECT 'PR639客户A账号','13900000639','channel_customer',id,true FROM %s.company_departments ORDER BY id LIMIT 1`, schema, schema),
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,status) SELECT $1,id,'active' FROM %s.company_employees WHERE phone='13900000639'`, schema, schema), customerA); err != nil {
		t.Fatal(err)
	}
	var customerEmployeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.company_employees WHERE phone='13900000639'`, schema)).Scan(&customerEmployeeID); err != nil {
		t.Fatal(err)
	}
	eCustomer := echo.New()
	eCustomer.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "pr639-customer-a")
			c.Set("employee_id", customerEmployeeID)
			return next(c)
		}
	})
	registerMaterialsAPI(eCustomer, materialsapp.NewService(postgresmaterials.NewRepository(pool, schema)))
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/materials?owner_customer_id=%d", customerB), nil)
	rec = httptest.NewRecorder()
	eCustomer.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("customer A cross-customer list status=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/materials/%d", b.ID), bytes.NewReader([]byte(`{"code":"PR639-B","name":"越权修改","kind":"pack","unit":"件","cost_unit":"件"}`)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	eCustomer.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("customer A cross-customer update status=%d body=%s", rec.Code, rec.Body.String())
	}
	history, historyRec := create("PR639-HISTORY", "factory", 0)
	if historyRec.Code != http.StatusOK {
		t.Fatalf("history material status=%d body=%s", historyRec.Code, historyRec.Body.String())
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_entry_items(material_id) VALUES($1)`, schema), history.ID); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/materials/%d/owner", history.ID), strings.NewReader(fmt.Sprintf(`{"owner_type":"customer","owner_customer_id":%d}`, customerA)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "业务单据") {
		t.Fatalf("historical business reference owner change status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/materials?owner_customer_id=%d&q=同规格", customerA), nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var listed MaterialListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || len(listed.Rows) != 1 || listed.Rows[0].ID != a.ID {
		t.Fatalf("owner filter status=%d body=%s", rec.Code, rec.Body.String())
	}

	ownerBody := strings.NewReader(`{"owner_type":"factory","owner_customer_id":0}`)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/materials/%d/owner", a.ID), ownerBody)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unused change owner status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_units=1 WHERE id=$1`, schema), b.ID); err != nil {
		t.Fatal(err)
	}
	ownerBody = strings.NewReader(`{"owner_type":"factory","owner_customer_id":0}`)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/materials/%d/owner", b.ID), ownerBody)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "库存") {
		t.Fatalf("in-use change status=%d body=%s", rec.Code, rec.Body.String())
	}

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		path := "/api/material-customer-references"
		if method == http.MethodPut {
			path += "/1"
		}
		req = httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), "已下线") {
			t.Fatalf("legacy %s status=%d body=%s", method, rec.Code, rec.Body.String())
		}
	}
	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='material' AND actor='pr639-test' AND field='owner_customer_id'`, schema)).Scan(&auditCount); err != nil || auditCount < 4 {
		t.Fatalf("owner audit count=%d err=%v", auditCount, err)
	}
}
