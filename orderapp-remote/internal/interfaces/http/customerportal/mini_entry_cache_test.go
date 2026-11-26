package customerportal

import (
	"context"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	"net/url"
	salesapp "orderapp/internal/application/sales"
	"testing"
)

type entryCacheFixture struct {
	miniEmployeeSalesFake
	reads    int
	revision string
}

func (f *entryCacheFixture) EmployeeEntryRevision(context.Context, string, int64) (string, error) {
	return f.revision, nil
}
func (f *entryCacheFixture) EmployeeEntryData(_ context.Context, part string, customerID int64) (salesapp.OrderFormData, error) {
	f.reads++
	return f.formFixture(0)
}

func TestEmployeeEntryConditionalReads(t *testing.T) {
	e := echo.New()
	data := &entryCacheFixture{revision: "one"}
	registerMiniEmployeeAPI(e, employeePortalService(), data)
	read := func(path string) map[string]any {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/mini/employee/order-entry/"+path, nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	base := read("base")
	if base["customers"] == nil || base["product_families"] != nil {
		t.Fatalf("unexpected bootstrap fields %v", base)
	}
	cat := read("catalog?customer_id=8")
	rev := cat["revision"].(string)
	if cat["product_families"] == nil || cat["customers"] != nil {
		t.Fatal("catalog shape")
	}
	if read("catalog?customer_id=8&known_revision=" + url.QueryEscape(rev))["unchanged"] != true || data.reads != 2 || data.orderFormCalls != 0 {
		t.Fatal("conditional read rebuilt the form")
	}
	// Both cold clients and clients that adopt the resolved canonical IDs reuse the same build.
	if read("catalog?customer_id=8")["revision"] != rev || data.reads != 2 {
		t.Fatal("cold client rebuilt warm catalog")
	}
	explicit := cat["selected_revision"].(string)
	if read("catalog?customer_id=8&selected_price_table_ids=901&known_revision=" + explicit)["unchanged"] != true || data.reads != 2 {
		t.Fatal("resolved selection missed cache")
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/api/mini/employee/order-entry/catalog?customer_id=8&known_revision="+rev, nil))
	if rec.Code != 401 || data.reads != 2 {
		t.Fatal("unauthenticated cache hit")
	}
	data.revision = "two"
	if read("catalog?customer_id=8&known_revision=" + url.QueryEscape(rev))["product_families"] == nil {
		t.Fatal("changed revision did not refresh")
	}
}
