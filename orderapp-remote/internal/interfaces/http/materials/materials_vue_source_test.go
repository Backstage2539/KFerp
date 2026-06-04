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
		"material-detail-panel",
		"selectMaterial(row)",
		"deprecateSelectedMaterial",
		"新建物料",
		"全部分类",
		"未分类",
		"增加分类",
		"移动到分类",
		"移动到小分类",
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
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("MaterialsView.vue still contains old material marker %q", forbidden)
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
