package customerportal

import (
	"context"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	"net/url"
	salesapp "orderapp/internal/application/sales"
	"sync"
	"sync/atomic"
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

func TestEmployeeEntryRecipientRevisionIsScopedToLogin(t *testing.T) {
	e := echo.New()
	reader := &entryCacheFixture{revision: "one"}
	registerMiniEmployeeAPI(e, employeePortalService(), reader)
	read := func(session, revision string) map[string]any {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/mini/employee/order-entry/base?known_revision="+url.QueryEscape(revision), nil)
		req.Header.Set("Authorization", "Bearer "+session)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		var result map[string]any
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &result) != nil {
			t.Fatalf("base response status %d", rec.Code)
		}
		return result
	}
	original := read("first-login", "")["revision"].(string)
	if read("first-login", original)["unchanged"] != true || reader.reads != 1 {
		t.Fatal("current session did not reuse its recipient data")
	}
	next := read("second-login", original)
	if next["customers"] == nil || next["revision"] == original || reader.reads != 2 {
		t.Fatal("a new login reused the prior session's recipient data")
	}
}

type concurrentEntryFixture struct {
	entryCacheFixture
	count atomic.Int32
	gate  chan struct{}
	first chan struct{}
}

func (f *concurrentEntryFixture) EmployeeEntryData(_ context.Context, _ string, _ int64) (salesapp.OrderFormData, error) {
	if f.count.Add(1) == 1 {
		close(f.first)
	}
	<-f.gate
	return f.formFixture(0)
}
func TestEquivalentEmployeeCatalogRequestsShareOneBuild(t *testing.T) {
	e := echo.New()
	data := &concurrentEntryFixture{entryCacheFixture: entryCacheFixture{revision: "one"}, gate: make(chan struct{}), first: make(chan struct{})}
	registerMiniEmployeeAPI(e, employeePortalService(), data)
	var wg sync.WaitGroup
	get := func(selection string) {
		defer wg.Done()
		req := httptest.NewRequest("GET", "/api/mini/employee/order-entry/catalog?customer_id=8"+selection, nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("status %d", rec.Code)
		}
	}
	wg.Add(1)
	go get("&selected_price_table_ids=901")
	<-data.first
	wg.Add(1)
	go get("")
	close(data.gate)
	wg.Wait()
	wg.Add(1)
	get("&preferred_price_table_ids=901")
	if got := data.count.Load(); got != 1 {
		t.Fatalf("read full catalog %d times for one effective selection", got)
	}
}
