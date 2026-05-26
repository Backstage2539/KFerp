package support

import (
	"os"
	"strings"
	"testing"
)

func TestDev318CustomerBeanListPublicCategoryScope(t *testing.T) {
	reqStore, err := os.ReadFile("internal/interfaces/http/support/req_store.go")
	if err != nil {
		t.Fatal(err)
	}
	reqText := string(reqStore)
	for _, want := range []string{
		"PR-318-CUSTOMER-BEAN-LIST-PUBLIC-CATEGORY-SCOPE",
		"DEV-318-CUSTOMER-BEAN-LIST-PUBLIC-CATEGORY-SCOPE",
		"UT-318-CUSTOMER-BEAN-LIST-PUBLIC-CATEGORY-SCOPE",
		"API-318-CUSTOMER-BEAN-LIST-PUBLIC-CATEGORY-SCOPE",
		"REV-318-CUSTOMER-BEAN-LIST-PUBLIC-CATEGORY-SCOPE",
		"是否使用公共商品分类",
		"关闭时不展示公共分类或公共 SKU",
	} {
		if !strings.Contains(reqText, want) {
			t.Fatalf("req store missing %q", want)
		}
	}

	costingView, err := os.ReadFile("frontend-vue-shell/src/views/CostingView.vue")
	if err != nil {
		t.Fatal(err)
	}
	costingText := string(costingView)
	for _, want := range []string{
		"const customerPublicUsages = ref([])",
		"const activeCustomerPublicUsage = computed",
		"const activeBeanListScopeOptions = computed",
		"apiGet('/api/product-settings')",
		"use_public_categories: Boolean(row.use_public_categories)",
		"filterBeanListItemsForScope(items.value, activeCostingScope.value, activeBeanListCustomerID.value, activeBeanListScopeOptions.value)",
	} {
		if !strings.Contains(costingText, want) {
			t.Fatalf("CostingView missing %q", want)
		}
	}

	beanListHelper, err := os.ReadFile("frontend-vue-shell/src/lib/bean-list-pdf.js")
	if err != nil {
		t.Fatal(err)
	}
	helperText := string(beanListHelper)
	for _, want := range []string{
		"options.usePublicCategories",
		"options.use_public_categories",
		"if (itemCustomerID <= 0) return Boolean(usePublicCategories)",
	} {
		if !strings.Contains(helperText, want) {
			t.Fatalf("bean-list helper missing %q", want)
		}
	}

	manual, err := os.ReadFile("docs/OP_MANUAL_COSTING.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"选择某个履约客户范围时，产品价格表默认只展示该客户自己的 SKU",
		"不混入公共分类或其他客户 SKU",
		"需要客户复用公共产品时，先在 SKU设置 的客户视图点击“SKU复制”",
	} {
		if !strings.Contains(string(manual), want) {
			t.Fatalf("manual missing %q", want)
		}
	}

	acceptance, err := os.ReadFile("docs/acceptance/2026-05-22-customer-bean-list-public-category-scope.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(acceptance), "PR-318-CUSTOMER-BEAN-LIST-PUBLIC-CATEGORY-SCOPE") {
		t.Fatalf("acceptance doc missing PR-317 marker")
	}
}
