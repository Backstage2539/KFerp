package support

import (
	"os"
	"strings"
	"testing"
)

func TestDev319CustomerBeanListSkuCategoryMetadata(t *testing.T) {
	reqStore, err := os.ReadFile("internal/interfaces/http/support/req_store.go")
	if err != nil {
		t.Fatal(err)
	}
	reqText := string(reqStore)
	for _, want := range []string{
		"PR-319-CUSTOMER-BEAN-LIST-SKU-CATEGORY-METADATA",
		"DEV-319-CUSTOMER-BEAN-LIST-SKU-CATEGORY-METADATA",
		"UT-319-CUSTOMER-BEAN-LIST-SKU-CATEGORY-METADATA",
		"API-319-CUSTOMER-BEAN-LIST-SKU-CATEGORY-METADATA",
		"REV-319-CUSTOMER-BEAN-LIST-SKU-CATEGORY-METADATA",
		"SKU设置 的客户分类路径",
		"TestCostingCalculateAPIReturnsCustomerSkuCategoryBeanListMetadata",
	} {
		if !strings.Contains(reqText, want) {
			t.Fatalf("req store missing %q", want)
		}
	}

	engine, err := os.ReadFile("internal/domain/costing/engine.go")
	if err != nil {
		t.Fatal(err)
	}
	engineText := string(engine)
	for _, want := range []string{
		"CategoryPrimaryName",
		"CategorySecondaryName",
		"ProductCategoryPosition",
		"customerCategoryBeanListDisplay",
		"customerBeanListCategoryName",
	} {
		if !strings.Contains(engineText, want) {
			t.Fatalf("costing engine missing %q", want)
		}
	}

	repository, err := os.ReadFile("internal/infrastructure/postgres/costing/repository.go")
	if err != nil {
		t.Fatal(err)
	}
	repositoryText := string(repository)
	for _, want := range []string{
		"p.product_category_position",
		"parent_pc.name",
		"&input.CategoryPrimaryName",
		"&input.CategorySecondaryName",
		"&input.ProductCategoryPosition",
	} {
		if !strings.Contains(repositoryText, want) {
			t.Fatalf("costing repository missing %q", want)
		}
	}

	manual, err := os.ReadFile("docs/OP_MANUAL_COSTING.md")
	if err != nil {
		t.Fatal(err)
	}
	manualText := string(manual)
	for _, want := range []string{
		"客户自有/客户定制熟豆不再依赖旧 Excel 豆单资料",
		"咖啡豆 / 定制咖啡熟豆",
		"客户新增熟豆 SKU 在客户豆单中不出现",
	} {
		if !strings.Contains(manualText, want) {
			t.Fatalf("manual missing %q", want)
		}
	}

	acceptance, err := os.ReadFile("docs/acceptance/2026-05-22-customer-bean-list-sku-category-metadata.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(acceptance), "PR-319-CUSTOMER-BEAN-LIST-SKU-CATEGORY-METADATA") {
		t.Fatalf("acceptance doc missing PR-319 marker")
	}
}
