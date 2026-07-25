package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev552StockEntryConvergenceContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-552-STOCK-ENTRY-CONVERGENCE",
			"DEV-552-STOCK-DOCUMENT-LIFECYCLE",
			"DEV-552-AUTHORITATIVE-POSTING",
			"DEV-552-LEGACY-COMPATIBILITY",
			"DEV-552-WORKORDER-PREVIEW",
			"DEV-552-INVENTORY-UI",
			"DEV-552-DOCS-DELIVERY",
			"REV-552-STOCK-ENTRY-CONVERGENCE",
		},
		filepath.Join("internal", "application", "stock", "service.go"): {
			"PurposeMaterialReceipt",
			"PurposeMaterialIssue",
			"PurposeMaterialTransfer",
			"PurposeMaterialTransferForManufacture",
			"PurposeMaterialConsumption",
			"PurposeManufacture",
			"legacyPurposeMaterialReturn",
			"CreateStockDocumentDraft",
			"SubmitStockDocument",
			"CancelStockDocument",
		},
		filepath.Join("internal", "infrastructure", "postgres", "stock", "stock_document.go"): {
			"stock_entry_batch_allocations",
			"FOR UPDATE",
			"stock_entry_cancel",
			"AuditInsertTx",
			"material_receipts",
			"material_transfers",
			"finished_product_transfers",
		},
		filepath.Join("internal", "interfaces", "http", "production", "stock_entry_api.go"): {
			`POST("/api/stock-documents"`,
			`PUT("/api/stock-documents/:id"`,
			`POST("/api/stock-documents/:id/submit"`,
			`POST("/api/stock-documents/:id/cancel"`,
			`GET("/api/stock-documents"`,
		},
		filepath.Join("internal", "interfaces", "http", "production", "work_order_api.go"): {
			`POST("/api/produce/work-orders/:id/stock-document-preview"`,
		},
		filepath.Join("frontend-vue-shell", "src", "views", "StockOperationsView.vue"): {
			"库存单据",
			"盘点调整",
			"StockEntriesView",
			"StockAdjustmentsView",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "production-execution-hub.js"): {
			"productionIssue",
			"productionSupplement",
			"productionReturn",
			"productionConsume",
			"finishedReceipt",
			"stockEntries",
		},
		filepath.Join("internal", "interfaces", "http", "support", "audit_page.go"): {
			`case "stock_entry"`,
			"创建库存单据草稿",
			"提交库存单据",
			"取消库存单据",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "AuditView.vue"): {
			`value="stock_entry"`,
			"库存单据",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-552-STOCK-ENTRY-CONVERGENCE",
			"Stock Entry",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-552-STOCK-ENTRY-CONVERGENCE",
			"原料入库",
			"余料退回",
			"完工入库",
		},
	}
	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-552 marker %q", rel, want)
			}
		}
	}
}

func TestDev552InventoryWorkspaceDoesNotExposeParallelWriterTabs(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "StockOperationsView.vue")))
	for _, forbidden := range []string{
		"WIP领退/转仓",
		"成品转仓",
		"MaterialReceiptsView",
		"WipMaterialsView",
		"FinishedTransfersView",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("StockOperationsView.vue must not expose parallel writer %q", forbidden)
		}
	}
}
