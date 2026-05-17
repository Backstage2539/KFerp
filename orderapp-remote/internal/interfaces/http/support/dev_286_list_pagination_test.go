package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev286ListPaginationUsesSharedVueControls(t *testing.T) {
	component := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "components", "PaginationControls.vue")))
	helper := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "pagination.js")))
	autoPager := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "table-auto-pagination.js")))
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))

	for _, want := range []string{
		"共 {{ displayTotal }} 条 / {{ totalPages }} 页",
		"跳至",
		"每页",
		"update:pageSize",
	} {
		if !strings.Contains(component, want) {
			t.Fatalf("PaginationControls.vue missing shared pagination marker %q", want)
		}
	}
	for _, want := range []string{"pageCount", "clampPage", "slicePageRows", "paginationFromApi"} {
		if !strings.Contains(helper, want) {
			t.Fatalf("pagination.js missing helper %q", want)
		}
	}
	for _, want := range []string{"installTableAutoPagination", "MutationObserver", "data-auto-pagination"} {
		if !strings.Contains(autoPager, want) {
			t.Fatalf("table-auto-pagination.js missing auto pagination marker %q", want)
		}
	}
	if !strings.Contains(app, "installTableAutoPagination") {
		t.Fatal("App.vue must install table auto pagination once for list pages that use full client-side arrays")
	}
}

func TestDev286ServerPagedViewsUseSharedPaginationControls(t *testing.T) {
	files := []string{
		filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "CustomersView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "AuditView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "RequirementsView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "AllocationLogsView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "InventoryView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "MaterialBatchesView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "StockBatchesView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "StockLedgerView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "StockOutboundLogsView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "WipMaterialsView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "CustomerFulfillmentView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "CustomerProcessingPortalView.vue"),
	}
	for _, rel := range files {
		src := string(readOrderAppFileForTest(t, rel))
		if !strings.Contains(src, "PaginationControls") {
			t.Fatalf("%s must use shared PaginationControls instead of custom pager markup", rel)
		}
		if strings.Contains(src, `class="pager"`) {
			t.Fatalf("%s still contains old pager markup", rel)
		}
	}
}

func TestDev286ListPaginationRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-286-LIST-PAGINATION",
		"DEV-286-SHARED-PAGINATION-WHEEL",
		"DEV-286-SERVER-PAGINATION-TOTALS",
		"DEV-286-ALL-VUE-LISTS-PAGINATION",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing pagination requirement seed %q", want)
		}
	}
}
