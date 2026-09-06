package materials

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	materialsapp "orderapp/internal/application/materials"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"

	"github.com/labstack/echo/v4"
)

func TestMaterialCustomerReferencesCreateFilterDeactivateAndRemainIdempotent(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	customerIDs := make([]int64, 3)
	for i, name := range []string{"PR628客户A", "PR628客户B", "PR628客户C"} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name,active) VALUES($1,true) RETURNING id`, schema), name).Scan(&customerIDs[i]); err != nil {
			t.Fatal(err)
		}
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "pr628-test")
			return next(c)
		}
	})
	registerMaterialsAPI(e, materialsapp.NewService(postgresmaterials.NewRepository(pool, schema)))

	createBody, _ := json.Marshal(map[string]any{
		"code": "PR628-MAT", "name": "PR628共享包材", "kind": "pack", "unit": "件", "cost_unit": "件",
		"customer_ids": []int64{customerIDs[0], customerIDs[1], customerIDs[0]},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/materials", bytes.NewReader(createBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create material status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created materialsapp.Material
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	var referenceCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.material_customer_references WHERE material_id=$1 AND active=true`, schema), created.ID).Scan(&referenceCount); err != nil || referenceCount != 2 {
		t.Fatalf("reference count=%d err=%v", referenceCount, err)
	}

	assertVisible := func(customerID int64, want bool) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/materials?customer_id=%d&q=PR628-MAT", customerID), nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("list customer %d status=%d body=%s", customerID, rec.Code, rec.Body.String())
		}
		var payload MaterialListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if got := len(payload.Rows) == 1; got != want {
			t.Fatalf("customer %d visible=%v want=%v payload=%s", customerID, got, want, rec.Body.String())
		}
	}
	assertVisible(customerIDs[0], true)
	assertVisible(customerIDs[1], true)
	assertVisible(customerIDs[2], false)

	refBody, _ := json.Marshal(map[string]any{"material_id": created.ID, "customer_id": customerIDs[0], "active": true})
	for range 2 {
		req = httptest.NewRequest(http.MethodPost, "/api/material-customer-references", bytes.NewReader(refBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("idempotent reference status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.material_customer_references WHERE material_id=$1 AND customer_id=$2`, schema), created.ID, customerIDs[0]).Scan(&referenceCount); err != nil || referenceCount != 1 {
		t.Fatalf("idempotent count=%d err=%v", referenceCount, err)
	}
	var referenceID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.material_customer_references WHERE material_id=$1 AND customer_id=$2`, schema), created.ID, customerIDs[0]).Scan(&referenceID); err != nil {
		t.Fatal(err)
	}
	disableBody, _ := json.Marshal(map[string]any{"material_id": created.ID, "customer_id": customerIDs[0], "active": false})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/material-customer-references/%d", referenceID), bytes.NewReader(disableBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable reference status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertVisible(customerIDs[0], false)
	assertVisible(customerIDs[1], true)

	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='material_customer_reference' AND actor='pr628-test'`, schema)).Scan(&auditCount); err != nil || auditCount < 4 {
		t.Fatalf("reference audit count=%d err=%v", auditCount, err)
	}
}

func TestMaterialCustomerReferenceAPIRequestBindsSnakeCaseIdentity(t *testing.T) {
	var req materialCustomerReferenceAPIRequest
	if err := json.Unmarshal([]byte(`{"material_id":75,"customer_id":74,"active":false,"remark":"停用验收"}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.MaterialID != 75 || req.CustomerID != 74 || req.Active || req.Remark != "停用验收" {
		t.Fatalf("request=%+v", req)
	}
}
