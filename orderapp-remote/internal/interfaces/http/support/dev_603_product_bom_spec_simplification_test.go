package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev603ProductBomSpecSimplificationContracts(t *testing.T) {
	bomView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, want := range []string{
		"isMaterialOutputBom",
		"versionMaterialLossRateEnabled",
		"handleVersionMaterialLossToggle",
		"outputChangedToMaterial",
		"variants: []",
		"规格组将随保存一起删除",
	} {
		if !strings.Contains(bomView, want) {
			t.Fatalf("BomView missing PR-603 marker %q", want)
		}
	}
	if !strings.Contains(bomView, `v-if="isMaterialOutputBom" class="material-loss-control`) {
		t.Fatal("material-output BOMs must scope the loss control via isMaterialOutputBom")
	}

	productSettings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, banned := range []string{
		"batchProductUnitTemplateID",
		"saveSelectedProductUnitTemplate",
		"openProductUnitTemplateManagement",
		"productProductionConfigForm.unit_template_id",
		"setDefaultProductSalesSpec",
		"createChildSkuForProduct",
		"productProductionSalesSpecRows",
	} {
		if strings.Contains(productSettings, banned) {
			t.Fatalf("ProductSettingsView still exposes retired surface %q", banned)
		}
	}
	for _, want := range []string{
		"productProductionBomSpecsSummary",
		"BOM 规格（只读）",
		"尚未绑定默认制造 BOM",
	} {
		if !strings.Contains(productSettings, want) {
			t.Fatalf("ProductSettingsView missing PR-603 marker %q", want)
		}
	}

	repoSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go")))
	if !strings.Contains(repoSrc, "identityChangedToMaterial") {
		t.Fatal("UpdateProductionBom must track identityChangedToMaterial to clear draft spec groups")
	}
}
