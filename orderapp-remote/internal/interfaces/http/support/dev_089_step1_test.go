package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductSettingsRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-089",
		"DEV-089-01",
		"DEV-089-02",
		"DEV-089-03",
		"UT-089-01",
		"API-089-01",
		"REV-089-01",
		"成本核算改名为产品设置",
		"商品一、二级分类",
		"拖入某个分类",
		"旧商品档案删除阶梯价编辑",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings requirement seed missing %q", want)
		}
	}
}

func TestProductSettingsVueWiringAndLegacyTierEditorRemoval(t *testing.T) {
	app, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "App.vue"))
	if err != nil {
		t.Fatal(err)
	}
	menu, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"))
	if err != nil {
		t.Fatal(err)
	}
	settings, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	products, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProductsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	combined := string(app) + "\n" + string(menu) + "\n" + string(settings)
	for _, want := range []string{
		"ProductSettingsView",
		"productSettings",
		"产品设置",
		"CostingView",
		"dragstart",
		"drop",
		"/api/product-settings",
		"/api/product-settings/categories",
		"/api/product-settings/products/",
		"一级分类",
		"二级分类",
		"商品编号",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("product settings Vue source missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"添加阶梯",
		"function addTier",
		"function removeTier",
		"v-model.number=\"tier.spec_g\"",
		"v-model.number=\"tier.unit_price\"",
	} {
		if strings.Contains(string(products), forbidden) {
			t.Fatalf("legacy product archive should remove tier editor concern %q", forbidden)
		}
	}
	if strings.Contains(string(menu), "label: '商品档案'") || strings.Contains(string(menu), "label: '成本核算'") {
		t.Fatalf("primary product menu should expose 产品设置 instead of 商品档案/成本核算")
	}
}
