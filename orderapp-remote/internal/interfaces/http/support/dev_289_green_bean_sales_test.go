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
		"UT-289-01",
		"API-289-01",
		"REV-289-01",
		"生豆销售",
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
			want: []string{"products ADD COLUMN IF NOT EXISTS product_kind", "order_items ADD COLUMN IF NOT EXISTS product_kind"},
		},
		{
			path: "orderapp-remote/internal/infrastructure/postgres/costing/repository.go",
			want: []string{"product_kind", "green_bean", "GreenBeanSaleTiers", "PublishRun"},
		},
		{
			path: "orderapp-remote/internal/infrastructure/postgres/sales/repository.go",
			want: []string{"product_kind", "INSERT INTO %s.order_items", "productKindForOrderItem"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue",
			want: []string{"productKindLabel", "kind-green"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/lib/order-entry.js",
			want: []string{"productKindLabel", "生豆", "熟豆"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/views/OrdersView.vue",
			want: []string{"product_kind_summary", "productKindLabel", "kind-green"},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.js",
			want: []string{"green_bean_list", "green_bean_sale_tiers", "生豆豆单"},
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
			want: []string{"生豆销售", "产品设置", "生豆豆单", "小程序"},
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
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	return filepath.Join(root, filepath.FromSlash(rel))
}
