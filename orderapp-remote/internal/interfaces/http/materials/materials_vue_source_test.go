package materials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialsViewUsesClassificationAndIndustryFields(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"materials-layout",
		"material-list-panel",
		"data-material-detail-drawer",
		"material-name-button",
		"selectMaterial(row)",
		"deprecateSelectedMaterials",
		"新建物料",
		"BusinessGroupInlineWorkspace",
		"collapsedMaterialCategoryKeys",
		"materialCategoryMoveActive",
		`@target="handleMaterialCategoryMoveTarget"`,
		"material_catalog",
		"MATERIAL_OBJECT_KEY = 'material'",
		"/api/business-group-assignments",
		"groupRowsByBusinessGroupTemplates(",
		"businessGroupInlineListState",
		"paginatedMaterialGroups",
		"handleMaterialGroupPaginationChange",
		"industry_field_template_id",
		"materialIndustryFields",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("MaterialsView.vue missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"copySelectedMaterial",
		"咖啡生豆属性",
		"销售价",
		"基础档案字段锁定",
		"profile-modal",
		"物料类型",
		"增加分类",
		"新增小分类",
		"移动到小分类",
		"/api/material-classification-groups",
		"/api/material-classification-assignments",
		"groupRowsByBusinessGroupTemplate(",
		"BusinessGroupControls",
		"BusinessGroupWorkspace",
		"selectedMaterialCategoryKey",
		"businessGroupGroupsForCategorySelection",
		"visibleMaterialDisplayGroups",
		"material-detail-panel",
		"selectedMaterialGroupTemplateID",
		"selectedMaterialMoveGroupItemID",
		"skuGroupHiddenByCollapsedAncestor",
		"renderedMaterialDisplayGroups",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("MaterialsView.vue still contains old material marker %q", forbidden)
		}
	}
}

func TestMaterialsViewListLayoutSupportsBulkSelection(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"material-list-toolbar",
		"全选物料",
		"toggleMaterialRows",
		"group.rows",
		"PaginationControls",
		`data-auto-pagination="off"`,
		"deprecateSelectedMaterials",
		"批量失效",
		"table-layout: fixed",
		"min-width: 660px",
		"col.name-col) { width: 130px; }",
		"max-width: 130px",
		"overflow-x: auto",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("MaterialsView.vue missing list layout marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"min-width: 920px",
		"col.name-col) { width: 390px; }",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("MaterialsView.vue still contains old list width marker %q", forbidden)
		}
	}
	compactStart := strings.Index(src, `<section class="panel compact-head">`)
	layoutStart := strings.Index(src, `<div class="materials-layout">`)
	if compactStart < 0 || layoutStart < 0 || compactStart >= layoutStart {
		t.Fatalf("MaterialsView.vue missing expected compact/list layout markers")
	}
	compact := src[compactStart:layoutStart]
	for _, forbidden := range []string{`v-model.trim="q"`, `v-model="filters.active"`} {
		if strings.Contains(compact, forbidden) {
			t.Fatalf("compact head still contains material list filter %q", forbidden)
		}
	}
}

func TestMaterialsViewDisallowsInlineStockAndUsesBackfill(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"库存补录",
		"补录说明",
		"stockBackfill",
		"openStockBackfill",
		"/api/stock/adjustments",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("MaterialsView.vue missing stock backfill marker %q", want)
		}
	}
	for _, forbidden := range []string{
		`v-model.number="draft.onhand_g"`,
		`v-model.number="draft.onhand_units"`,
		`stockBackfill.target_g`,
		`stockBackfill.target_units`,
		"目标库存(g)",
		"目标库存(个)",
		"保存库存/属性",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("MaterialsView.vue still allows inline stock editing through %q", forbidden)
		}
	}
	for _, want := range []string{
		`stockBackfill.target_qty`,
		`target_qty`,
		`unit_code`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("MaterialsView.vue missing single quantity marker %q", want)
		}
	}
}
