package support

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGreenBeanSalesRequirementSeedsExist(t *testing.T) {
	body, err := os.ReadFile(supportFilePath(t, "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	src := string(body)
	for _, want := range []string{
		"PR-289-GREEN-BEAN-SALES",
		"DEV-289-01",
		"DEV-289-02",
		"DEV-289-03",
		"UT-289-01",
		"API-289-01",
		"REV-289-01",
		"绑定熟豆 BOM",
		"最新通过生产质检",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("green bean sales requirement seed missing %q", want)
		}
	}
}

func TestGreenBeanSalesWiringAndManuals(t *testing.T) {
	checks := []struct {
		path string
		want []string
	}{
		{
			path: "orderapp-remote/internal/infrastructure/postgres/core/schema.go",
			want: []string{
				"products ADD COLUMN IF NOT EXISTS product_kind",
				"order_items ADD COLUMN IF NOT EXISTS product_kind",
				"order_items ADD COLUMN IF NOT EXISTS sales_unit",
				"order_items ADD COLUMN IF NOT EXISTS unit_bag_count",
				"order_items ADD COLUMN IF NOT EXISTS unit_bean_g",
				"order_items ADD COLUMN IF NOT EXISTS matched_price_qty",
				"order_items ADD COLUMN IF NOT EXISTS price_source_json",
			},
		},
		{
			path: "orderapp-remote/internal/infrastructure/postgres/catalog/schema.go",
			want: []string{"products ADD COLUMN IF NOT EXISTS green_bean_type", "products ADD COLUMN IF NOT EXISTS green_bean_bom_product_id"},
		},
		{
			path: "orderapp-remote/internal/infrastructure/postgres/costing/repository.go",
			want: []string{"product_kind", "green_bean_bom_product_id", "BeanListQuality", "quality_inspections qi", "ORDER BY qi.created_at DESC, qi.id DESC"},
		},
		{
			path: "orderapp-remote/internal/domain/costing/engine.go",
			want: []string{"GreenBeanSaleTiers", "buildGreenBeanTemplateSaleTiers", "BeanListQuality"},
		},
		{
			path: "orderapp-remote/internal/infrastructure/postgres/orderbeans/usage.go",
			want: []string{"ListTypeGreen", "ListTypeForProductKind", "ResolvePublishedUnitPrice", "green_bean_sale_tiers"},
		},
		{
			path: "orderapp-remote/internal/infrastructure/postgres/sales/repository.go",
			want: []string{"product_kind", "ListTypeForProductKind", "ResolvePublishedUnitPrice", "productKindForOrderItem"},
		},
		{
			path: "orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go",
			want: []string{"ListTypeGreen", "ResolvePublishedUnitPrice", "beanListQuality"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/lib/product-settings.js",
			want: []string{"filterSkuRows", "paginatedSkuRows", "buildProductCreatePayload", "green_bean_type", "green_bean_bom_product_id"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue",
			want: []string{"skuFilters", "filteredSkuRows", "paginatedSkuRows", "data-auto-pagination=\"off\"", "PaginationControls", "green_bean_type", "green_bean_bom_product_id"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue",
			want: []string{"productKindLabel", "kind-green", "wholesaleSpecOptions"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/lib/order-entry.js",
			want: []string{"productKindLabel", "生豆", "熟豆", "CUSTOM_SPEC_VALUE"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/views/OrdersView.vue",
			want: []string{"product_kind_summary", "productKindLabel", "kind-green"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.js",
			want: []string{"green_bean_list", "green_bean_sale_tiers", "生豆豆单", "beanListQualityLines"},
		},
		{
			path: "orderapp-remote/internal/infrastructure/postgres/stock/schema.go",
			want: []string{"crop_season", "origin", "producer_flavor_description"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/views/QualityInspectionsView.vue",
			want: []string{"factory_flavor_description", "moisture", "density"},
		},
		{
			path: "miniapp/src/pages/service/service.vue",
			want: []string{"productKindLabel", "生豆"},
		},
		{
			path: "miniapp/src/pages/mall/mall.vue",
			want: []string{"mallProductKindLabel"},
		},
		{
			path: "miniapp/src/utils/mall.ts",
			want: []string{"product_kind", "mallProductKindLabel", "生豆"},
		},
		{
			path: "orderapp-remote/docs/OP_MANUAL_GREEN_BEAN_SALES.md",
			want: []string{"生豆销售", "SKU设置", "绑定熟豆 BOM", "生豆豆单", "最新通过生产质检", "小程序"},
		},
		{
			path: "orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md",
			want: []string{"产季", "产地", "产家风味描述", "入库质检", "工厂风味描述", "水分", "密度"},
		},
	}
	for _, check := range checks {
		body, err := os.ReadFile(repoFilePath(t, check.path))
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		src := string(body)
		for _, want := range check.want {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", check.path, want)
			}
		}
	}
}

func supportFilePath(t *testing.T, elems ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve support test path")
	}
	parts := append([]string{filepath.Dir(file)}, elems...)
	return filepath.Join(parts...)
}

func repoFilePath(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repo test path")
	}
	supportDir := filepath.Dir(file)
	roots := []string{
		filepath.Clean(filepath.Join(supportDir, "../../../../..")),
		filepath.Clean(filepath.Join(supportDir, "../../../..")),
	}
	paths := []string{rel}
	if trimmed := strings.TrimPrefix(rel, "orderapp-remote/"); trimmed != rel {
		paths = append(paths, trimmed)
	}
	for _, root := range roots {
		for _, path := range paths {
			candidate := filepath.Join(root, filepath.FromSlash(path))
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return filepath.Join(roots[0], filepath.FromSlash(rel))
}
