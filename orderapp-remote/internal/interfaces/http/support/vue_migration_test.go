package support_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	supporthttp "orderapp/internal/interfaces/http/support"
	"os"
	"strings"
	"testing"

	bomhttp "orderapp/internal/interfaces/http/bom"
	cataloghttp "orderapp/internal/interfaces/http/catalog"
	companyhttp "orderapp/internal/interfaces/http/company"
	customerhttp "orderapp/internal/interfaces/http/customer"
	inventoryhttp "orderapp/internal/interfaces/http/inventory"
	productionhttp "orderapp/internal/interfaces/http/production"
	saleshttp "orderapp/internal/interfaces/http/sales"

	"github.com/labstack/echo/v4"
)

func TestVueShellMigratesOrdersAuditAndRequirementTables(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{
		"import OrdersView from './views/OrdersView.vue'",
		"import AuditView from './views/AuditView.vue'",
		"import RequirementsView from './views/RequirementsView.vue'",
		"orders: OrdersView",
		"audit: AuditView",
		"reqProduct: RequirementsView",
		"reqDev: RequirementsView",
		"reqUnit: RequirementsView",
		"reqApi: RequirementsView",
		"reqReview: RequirementsView",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("App.vue missing migrated Vue wiring %q", want)
		}
	}
}

func TestLegacyListRoutesRedirectToVueShell(t *testing.T) {
	e := echo.New()
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{})
	supporthttp.RegisterRoutes(e, nil, "public")

	cases := []struct {
		path string
		want string
	}{
		{path: "/orders?q=SO-1", want: "/vue-shell?view=orders&q=SO-1"},
		{path: "/audit?type=order", want: "/vue-shell?view=audit&type=order"},
		{path: "/req/product?page=2", want: "/vue-shell?view=reqProduct&page=2"},
		{path: "/req/dev", want: "/vue-shell?view=reqDev"},
		{path: "/req/unit", want: "/vue-shell?view=reqUnit"},
		{path: "/req/api", want: "/vue-shell?view=reqApi"},
		{path: "/req/review", want: "/vue-shell?view=reqReview"},
	}
	assertRedirects(t, e, cases)
}

func TestVueShellMigratesCatalogAndSettingsPages(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{
		"import CustomersView from './views/CustomersView.vue'",
		"import ProductsView from './views/ProductsView.vue'",
		"import ProductSettingsView from './views/ProductSettingsView.vue'",
		"import BomView from './views/BomView.vue'",
		"import CompanyStaffView from './views/CompanyStaffView.vue'",
		"import InventoryView from './views/InventoryView.vue'",
		"import MachinesView from './views/MachinesView.vue'",
		"import AllocationLogsView from './views/AllocationLogsView.vue'",
		"import SenderSettingsView from './views/SenderSettingsView.vue'",
		"import OutsourceSettingsView from './views/OutsourceSettingsView.vue'",
		"customers: CustomersView",
		"products: ProductSettingsView",
		"productSettings: ProductSettingsView",
		"bom: BomView",
		"departments: CompanyStaffView",
		"employees: CompanyStaffView",
		"inventory: InventoryView",
		"quotePrint: ProductsView",
		"machines: MachinesView",
		"allocationLogs: AllocationLogsView",
		"senderSettings: SenderSettingsView",
		"outsourceSettings: OutsourceSettingsView",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("App.vue missing catalog/settings Vue wiring %q", want)
		}
	}
	if strings.Contains(content, "BOM_REACT_URL") {
		t.Fatal("App.vue should not route BOM through the React fallback")
	}
}

func TestCatalogAndSettingsRoutesRedirectToVueShell(t *testing.T) {
	e := echo.New()
	customerhttp.RegisterRoutes(e, customerhttp.Dependencies{})
	cataloghttp.RegisterRoutes(e, cataloghttp.Dependencies{})
	bomhttp.RegisterRoutes(e, bomhttp.Dependencies{})
	inventoryhttp.RegisterRoutes(e, inventoryhttp.Dependencies{})
	productionhttp.RegisterRoutes(e, productionhttp.Dependencies{})
	companyhttp.RegisterRoutes(e, companyhttp.Dependencies{})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{})

	cases := []struct {
		path string
		want string
	}{
		{path: "/customers?q=Karen", want: "/vue-shell?view=customers&q=Karen"},
		{path: "/customers/new", want: "/vue-shell?view=customers&mode=new"},
		{path: "/customers/new?from=order", want: "/vue-shell?view=customers&from=order&mode=new"},
		{path: "/customers/7", want: "/vue-shell?view=customers&edit_id=7"},
		{path: "/products", want: "/vue-shell?view=productSettings"},
		{path: "/products/7", want: "/vue-shell?view=productSettings&edit_id=7"},
		{path: "/products/print", want: "/vue-shell?view=quotePrint"},
		{path: "/bom?product_id=7", want: "/vue-shell?view=bom&product_id=7"},
		{path: "/company/departments", want: "/vue-shell?view=departments"},
		{path: "/company/employees?department_id=1", want: "/vue-shell?view=employees&department_id=1"},
		{path: "/products/inventory", want: "/vue-shell?view=inventory"},
		{path: "/produce/machines", want: "/vue-shell?view=machines"},
		{path: "/produce/allocations?batch=B1", want: "/vue-shell?view=allocationLogs&batch=B1"},
		{path: "/settings/sender", want: "/vue-shell?view=senderSettings"},
		{path: "/settings/outsource", want: "/vue-shell?view=outsourceSettings"},
	}
	assertRedirects(t, e, cases)
}

func assertRedirects(t *testing.T, e *echo.Echo, cases []struct {
	path string
	want string
}) {
	t.Helper()
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("GET %s status = %d, want %d body=%s", tc.path, rec.Code, http.StatusFound, rec.Body.String())
		}
		got := rec.Header().Get("Location")
		if strings.HasPrefix(got, "/") {
			t.Fatalf("GET %s Location = %q; redirects must be relative so /app prefix is preserved", tc.path, got)
		}
		base, err := url.Parse("https://example.test" + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		loc, err := url.Parse(got)
		if err != nil {
			t.Fatalf("GET %s invalid Location %q: %v", tc.path, got, err)
		}
		if resolved := base.ResolveReference(loc).RequestURI(); resolved != tc.want {
			t.Fatalf("GET %s Location = %q resolves to %q, want %q", tc.path, got, resolved, tc.want)
		}
	}
}
