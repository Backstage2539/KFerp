package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev500UnitModelConsolidationContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":          filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"productSettings":   filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"),
		"productSettingsJS": filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"),
		"materialsView":     filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"),
		"receiptsView":      filepath.Join("frontend-vue-shell", "src", "views", "MaterialReceiptsView.vue"),
		"stockAdjustments":  filepath.Join("frontend-vue-shell", "src", "views", "StockAdjustmentsView.vue"),
		"bomView":           filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"),
		"stockAPI":          filepath.Join("internal", "interfaces", "http", "stock", "stock_api.go"),
		"stockService":      filepath.Join("internal", "application", "stock", "service.go"),
		"stockRepository":   filepath.Join("internal", "infrastructure", "postgres", "stock", "repository.go"),
		"stockSchema":       filepath.Join("internal", "infrastructure", "postgres", "stock", "schema.go"),
		"bomService":        filepath.Join("internal", "application", "bom", "service.go"),
		"requirements":      filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":        filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manualInventory":   filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"),
		"manualProduction":  filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"manualCosting":     filepath.Join("docs", "OP_MANUAL_COSTING.md"),
		"evidence":          filepath.Join("docs", "acceptance", "2026-06-26-unit-model-consolidation.md"),
	}
	contents := map[string]string{}
	for key, rel := range files {
		contents[key] = string(readOrderAppFileForTest(t, rel))
	}

	for _, marker := range []string{
		"PR-500-UNIT-MODEL-CONSOLIDATION",
		"DEV-500-UNIT-LANGUAGE-CONTRACT",
		"DEV-500-STOCK-UNIT-FLOWS",
		"DEV-500-BOM-UNIT-DERIVATION",
		"DEV-500-SALES-UNIT-COMPAT",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store.go missing %s", marker)
		}
	}

	frontend := contents["productSettings"] + contents["productSettingsJS"] + contents["materialsView"] + contents["receiptsView"] + contents["stockAdjustments"] + contents["bomView"]
	for _, marker := range []string{
		"销售单位",
		"库存单位",
		"单位转换",
		"sales_unit",
		"qty",
		"unit_code",
		"outputUnitDisplay",
		"componentStockUnitLabel",
	} {
		if !strings.Contains(frontend, marker) {
			t.Fatalf("frontend missing %s", marker)
		}
	}
	for _, forbidden := range []string{
		"数量(g)",
		"物料单位",
		"成品库存单位",
		"报价单位</span>",
		"录单单位</span>",
		`v-model.trim="bomForm.output_unit"`,
	} {
		if strings.Contains(frontend, forbidden) {
			t.Fatalf("frontend still exposes legacy unit wording or editable field %s", forbidden)
		}
	}

	for _, marker := range []string{"Qty", "UnitCode", "qty", "unit_code"} {
		if !strings.Contains(contents["stockAPI"], marker) {
			t.Fatalf("stock API missing material receipt %s contract", marker)
		}
	}
	stockBackend := contents["stockService"] + contents["stockRepository"] + contents["stockSchema"]
	for _, marker := range []string{"QtyUnits", "qty_units", "remaining_units", "material_batch_locations"} {
		if !strings.Contains(stockBackend, marker) {
			t.Fatalf("stock backend missing inventory-unit receipt marker %s", marker)
		}
	}
	for _, marker := range []string{"deriveProductionBomOutputUnit", "outputProductInventoryUnit"} {
		if !strings.Contains(contents["bomService"], marker) {
			t.Fatalf("BOM service missing %s", marker)
		}
	}

	docs := contents["requirements"] + contents["acceptance"] + contents["manualInventory"] + contents["manualProduction"] + contents["manualCosting"] + contents["evidence"]
	for _, marker := range []string{
		"PR-500-UNIT-MODEL-CONSOLIDATION",
		"库存单位 / 销售单位 / 单位转换",
		"物料单位不作为新业务概念",
		"报价单位和录单单位只作为历史兼容字段",
		"BOM 产出单位自动取产出商品库存单位",
		"原料入库按库存单位录入",
	} {
		if !strings.Contains(docs, marker) {
			t.Fatalf("docs missing %s", marker)
		}
	}
}
