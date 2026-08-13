package stock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVueShellRedirectsLegacyStockWritersToUnifiedStockOperations(t *testing.T) {
	app, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "App.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(app)
	for _, want := range []string{
		"materialReceipts: 'stockOperations'",
		"wipMaterials: 'stockOperations'",
		"requestedViewParam === 'materialReceipts'",
		"requestedViewParam === 'wipMaterials'",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}

	operations, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "StockOperationsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	operationsSrc := string(operations)
	for _, want := range []string{"库存单据", "盘点调整", "StockEntriesView", "StockAdjustmentsView"} {
		if !strings.Contains(operationsSrc, want) {
			t.Fatalf("StockOperationsView.vue missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"WipMaterialsView",
		"MaterialReceiptsView",
		"FinishedTransfersView",
		"WIP领退/转仓",
		"成品转仓",
	} {
		if strings.Contains(operationsSrc, forbidden) {
			t.Fatalf("StockOperationsView.vue must not expose legacy writer %q", forbidden)
		}
	}
}

func TestVueStockWorkspaceUsesUnifiedTransferAndKeepsTraceLookup(t *testing.T) {
	operations, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "StockOperationsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	operationsSrc := string(operations)
	for _, want := range []string{
		"库存单据",
		"StockEntriesView",
	} {
		if !strings.Contains(operationsSrc, want) {
			t.Fatalf("StockOperationsView.vue missing %q", want)
		}
	}

	entryView, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "StockEntriesView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	transferSrc := string(entryView)
	for _, want := range []string{
		"material_transfer",
		"from_warehouse",
		"to_warehouse",
		"item_type",
	} {
		if !strings.Contains(transferSrc, want) {
			t.Fatalf("StockEntriesView.vue missing unified transfer marker %q", want)
		}
	}

	warehouse, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	warehouseSrc := string(warehouse)
	for _, want := range []string{
		"/api/stock/trace",
		"traceDrawerOpen",
		"追溯",
		"material_batch",
		"LEGACY-MAT",
		"quality_status",
		"qualityLabel",
		"质检",
	} {
		if !strings.Contains(warehouseSrc, want) {
			t.Fatalf("WarehouseInventoryView.vue missing trace lookup %q", want)
		}
	}

	materialBatches, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialBatchesView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	stockBatches, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "StockBatchesView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	for path, src := range map[string]string{
		"MaterialBatchesView.vue": string(materialBatches),
		"StockBatchesView.vue":    string(stockBatches),
	} {
		for _, want := range []string{"quality_status", "qualityLabel", "质检"} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}
}

func TestVueMaterialReceiptsAndQualityExposeGreenBeanInboundFields(t *testing.T) {
	receipts, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialReceiptsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	receiptSrc := string(receipts)
	for _, want := range []string{
		"crop_season",
		"origin",
		"producer_flavor_description",
		"产季",
		"产地",
		"产家风味描述",
	} {
		if !strings.Contains(receiptSrc, want) {
			t.Fatalf("MaterialReceiptsView.vue missing %q", want)
		}
	}

	quality, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "QualityInspectionsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	qualitySrc := string(quality)
	for _, want := range []string{
		"factory_flavor_description",
		"moisture",
		"density",
		"工厂风味描述",
		"水分",
		"密度",
	} {
		if !strings.Contains(qualitySrc, want) {
			t.Fatalf("QualityInspectionsView.vue missing %q", want)
		}
	}

	batches, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialBatchesView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	batchSrc := string(batches)
	for _, want := range []string{"crop_season", "origin", "producer_flavor_description", "产季", "产地", "产家风味描述"} {
		if !strings.Contains(batchSrc, want) {
			t.Fatalf("MaterialBatchesView.vue missing %q", want)
		}
	}
}

func TestVueMaterialReceiptUsesInventoryUnitQuantity(t *testing.T) {
	receipts, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialReceiptsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(receipts)
	for _, want := range []string{
		"库存单位",
		"入库数量",
		"unit_code",
		"qty",
		"selectedMaterialUnitLabel",
		":value=\"selectedMaterialUnitLabel\" readonly",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("MaterialReceiptsView.vue missing inventory unit receipt marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"数量(g)",
		"form.qty_g",
		"qty_g",
		"/api/product-settings",
		"form.unit_code",
		"<select v-model=\"form.unit_code\"",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("MaterialReceiptsView.vue still exposes legacy gram-only receipt marker %q", forbidden)
		}
	}
}

func TestVueStockAdjustmentsExposeMaterialCostAdjustment(t *testing.T) {
	view, err := readStockWorkspaceFile(filepath.Join("frontend-vue-shell", "src", "views", "StockAdjustmentsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(view)
	for _, want := range []string{
		"adjustment_type",
		"material_cost",
		"material_batch_id",
		"target_unit_cost",
		"/api/stock/material-batches",
		"批次成本",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("StockAdjustmentsView.vue must expose material cost adjustment; missing %q", want)
		}
	}
}

func readStockWorkspaceFile(path string) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		return b, nil
	}
	return os.ReadFile(filepath.Join("..", "..", "..", "..", path))
}
