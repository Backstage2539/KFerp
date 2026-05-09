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

func TestProductSettingsRefinementRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-090",
		"DEV-090-01",
		"DEV-090-02",
		"UT-090-01",
		"API-090-01",
		"REV-090-01",
		"二级分类拖动时显示插入横线",
		"产品出品率",
		"商品分类和商品基础信息支持收起",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings refinement requirement seed missing %q", want)
		}
	}
}

func TestProductSettingsDragAndBomYieldFollowupRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-092",
		"DEV-092-01",
		"DEV-092-02",
		"UT-092-01",
		"API-092-01",
		"REV-092-01",
		"修正二级分类拖拽插入线可落位保存",
		"BOM出品率",
		"product_bom.yield_rate",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings drag/yield follow-up requirement seed missing %q", want)
		}
	}
}

func TestProductSettingsRobustPointerDragRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-093",
		"DEV-093-01",
		"UT-093-01",
		"API-093-01",
		"REV-093-01",
		"二级分类拖拽改为指针定位",
		"鼠标松开后按当前插入线保存",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings robust pointer drag requirement seed missing %q", want)
		}
	}
}

func TestProductSettingsDeleteCategoryRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-094",
		"DEV-094-01",
		"DEV-094-02",
		"UT-094-01",
		"API-094-01",
		"REV-094-01",
		"支持产品设置分类删除",
		"删除分类后商品回到未分类",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings category delete requirement seed missing %q", want)
		}
	}
}

func TestBeanListAdvancedGenerationRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-095",
		"DEV-095-01",
		"DEV-095-02",
		"DEV-095-03",
		"UT-095-01",
		"API-095-01",
		"REV-095-01",
		"价格试算支持折叠",
		"豆单生成支持选择产品、分级显示、重排编号",
		"豆卡样式",
		"表格样式",
		"发布和撤回发布",
		"上传logo",
		"品牌介绍",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("advanced bean-list generation requirement seed missing %q", want)
		}
	}
}

func TestBeanListPublicationFollowupRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-096",
		"DEV-096-01",
		"DEV-096-02",
		"UT-096-01",
		"API-096-01",
		"REV-096-01",
		"支持设置品牌名字",
		"更新日志放在底部",
		"修复发布豆单 conn busy",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean-list publication follow-up requirement seed missing %q", want)
		}
	}
}

func TestBeanListCardSelectionLayoutFollowupRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-097",
		"DEV-097-01",
		"DEV-097-02",
		"DEV-097-03",
		"UT-097-01",
		"API-097-01",
		"REV-097-01",
		"价格文本可通过标红词标红",
		"豆卡报价对齐",
		"按分类剩余产品数量动态列数",
		"分类和产品选择联动",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean-list card/selection follow-up requirement seed missing %q", want)
		}
	}
}

func TestSidebarMenuDensityRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-098",
		"DEV-098-01",
		"UT-098-01",
		"API-098-01",
		"REV-098-01",
		"左树菜单",
		"更饱满",
		"方便点击",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sidebar menu density requirement seed missing %q", want)
		}
	}
}

func TestVueShellSidebarMenuHasLargeClickTargets(t *testing.T) {
	app, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "App.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(app)
	for _, want := range []string{
		`:class="{ active: currentGroupId === g.id }"`,
		".sidebar { width: 260px;",
		"padding: 18px 14px",
		".brand { font-size: 28px;",
		".section-toggle {",
		"min-height: 48px;",
		"border: 1px solid #d8d8d8;",
		"background: #fff;",
		".section-toggle.active { border-color: #111; box-shadow: 0 0 0 1px #111 inset; background: #fff; color: #111; }",
		".section-name { font-size: 16px;",
		".menu {",
		"min-height: 44px;",
		".menu.active { border-color: #111; background: #f5f5f5; color: #111; box-shadow: 0 0 0 1px #111 inset; }",
		".sidebar.mobile",
		"width: 280px;",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("Vue shell sidebar menu density missing %q", want)
		}
	}
}

func TestBeanListPreviewPrintLayoutFollowupRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-099",
		"DEV-099-01",
		"DEV-099-02",
		"DEV-099-03",
		"UT-099-01",
		"API-099-01",
		"REV-099-01",
		"豆卡同排字段与报价对齐",
		"末行单卡/双卡占满整排",
		"删除标红价格档选项",
		"打印只输出豆单预览内容",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean-list preview/print layout follow-up requirement seed missing %q", want)
		}
	}
}

func TestBeanListPublicCustomerLinkRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-102",
		"DEV-102-01",
		"DEV-102-02",
		"UT-102-01",
		"API-102-01",
		"REV-102-01",
		"客户可以通过网址直接访问已发布豆单",
		"/public/bean-list/commercial",
		"/public/bean-list/retail",
		"免登录只读",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean-list public customer link requirement seed missing %q", want)
		}
	}
}

func TestBeanListCopyPublishedConfigRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-105",
		"DEV-105-01",
		"DEV-105-02",
		"UT-105-01",
		"API-105-01",
		"REV-105-01",
		"生成豆单支持复制已有豆单配置",
		"复制已有豆单配置",
		"当前最新产品和价格重新生成预览",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean-list copy config requirement seed missing %q", want)
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

func TestProductSettingsCategoryDragYieldAndCollapseRefinements(t *testing.T) {
	settings, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(settings)
	for _, want := range []string{
		"category-drop-line",
		"categoryDropTarget",
		"dropCategoryAtPosition",
		"yield_percent",
		"saveProductBasics(row)",
		"categoryCollapsed",
		"productsCollapsed",
		"toggle-section",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings refinement missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"<th>默认价</th>",
		"<th>100g</th>",
		"<th>200g</th>",
		"<th>227g</th>",
		"<th>250g</th>",
		"<th>操作</th>",
		"编辑基础信息",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("product basics list should remove price/action editor fragment %q", forbidden)
		}
	}
}

func TestProductSettingsDragEndAndBomYieldAreWiredToSingleSource(t *testing.T) {
	settings, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	repo, err := os.ReadFile(filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"))
	if err != nil {
		t.Fatal(err)
	}
	queries, err := os.ReadFile(filepath.Join("internal", "infrastructure", "postgres", "catalog_queries.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(settings)
	for _, want := range []string{
		"scheduleClearDrag",
		"@dragend=\"scheduleClearDrag\"",
		"dropCategoryOrProductOnSecondary",
		"BOM出品率",
		"yield_rate: Number((yieldPercent / 100).toFixed(4))",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings drag/yield wiring missing %q", want)
		}
	}
	if strings.Contains(src, "@dragend=\"clearDrag\"") {
		t.Fatalf("dragend must not synchronously clear drag state before drop handlers run")
	}
	for _, want := range []string{
		"INSERT INTO %s.product_bom(product_id,yield_rate,updated_at)",
		"ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate",
	} {
		if !strings.Contains(string(repo), want) {
			t.Fatalf("catalog repository must persist product settings yield to product_bom: missing %q", want)
		}
	}
	if !strings.Contains(string(queries), "LEFT JOIN %[1]s.product_bom b ON b.product_id=p.id") {
		t.Fatalf("product settings list must read yield_rate from product_bom")
	}
}

func TestProductSettingsSecondaryCategoryDragUsesPointerPositionInsteadOfNativeDrop(t *testing.T) {
	settings, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(settings)
	for _, want := range []string{
		"@pointerdown=\"startCategoryPointerDrag($event, primary, index + 1, secondary)\"",
		"handleCategoryPointerMove",
		"handleCategoryPointerUp",
		"resolveCategoryPointerTarget",
		"document.elementsFromPoint",
		"getBoundingClientRect",
		"setPointerCapture",
		"dropCategoryAtPosition(primary, target.position)",
		"data-primary-id",
		"data-secondary-position",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings robust pointer drag missing %q", want)
		}
	}
	if strings.Contains(src, "@dragstart=\"startCategoryDrag(secondary)\"") {
		t.Fatalf("secondary category sorting must not depend on native HTML5 dragstart/drop")
	}
}

func TestProductSettingsVueSupportsCategoryDelete(t *testing.T) {
	settings, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(settings)
	for _, want := range []string{
		"deleteCategory(primary)",
		"deleteCategory(secondary)",
		"async function deleteCategory(category)",
		"method: 'DELETE'",
		"`/api/product-settings/categories/${category.id}`",
		"删除分类后，分类内商品会回到未分类",
		"danger-text",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings category delete UI missing %q", want)
		}
	}
}

func TestProductSettingsRepositorySoftDeletesCategoriesAndUnassignsProducts(t *testing.T) {
	repo, err := os.ReadFile(filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(repo)
	for _, want := range []string{
		"func (r Repository) DeleteProductCategory",
		"SET active=false, updated_at=now()",
		"SET product_category_id=NULL, product_category_position=0",
		"COALESCE(parent_id,0)=$1",
		"normalizeCategoryPositions",
		"normalizeProductPositions(ctx, tx, r.schema, 0, customerID)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings repository delete behavior missing %q", want)
		}
	}
}
