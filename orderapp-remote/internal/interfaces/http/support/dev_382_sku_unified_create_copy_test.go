package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev382SKUUnifiedCreateCopyRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-382-SKU-UNIFIED-CREATE-COPY",
		"DEV-382-SKU-UNIFIED-CREATE-COPY",
		"UT-382-SKU-UNIFIED-CREATE-COPY",
		"API-382-SKU-UNIFIED-CREATE-COPY",
		"REV-382-SKU-UNIFIED-CREATE-COPY",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU unified create/copy seed missing %q", want)
		}
	}
}

func TestDev382SKUUnifiedCreateCopySourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"创建新商品档案",
			">复制</button>",
			"copyProductArchive",
			"批量添加商品档案",
			"增加分类",
			"未分类商品",
			"分类模板",
			"productClassificationTabs",
			"/api/product-settings/products/${row.id}/copy",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildSkuCreatePayload",
			"classificationTemplateTabs",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"CreateSKU(ctx context.Context, cmd CreateSKUCommand)",
			"CopyProduct(ctx context.Context, cmd CopyProductCommand)",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"func (r Repository) CreateSKU",
			"func (r Repository) CopyProduct",
			"nextProductArchiveCopyNameTx",
			"product_production_configs",
			"copyProductPriceTiersTx",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			"/api/product-settings/products",
			"/api/product-settings/products/:id/copy",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing SKU unified create/copy marker %q", rel, want)
			}
		}
	}
}

func TestDev382SKUUnifiedCreateCopyDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-382-SKU-UNIFIED-CREATE-COPY",
			"新增SKU",
			"SKU复制",
			"同名同归属同分类时覆盖并保留目标 SKU ID",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-382-SKU-UNIFIED-CREATE-COPY",
			"选择分类和产品 X/Y 款",
			"商品分类管理",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-382",
			"历史 SKU 复制抽屉",
			"生产 BOM",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-382-SKU-UNIFIED-CREATE-COPY",
			"新增 X 款、覆盖 Y 款、跳过 Z 款",
			"SKU复制",
		},
		filepath.Join("docs", "acceptance", "2026-05-26-sku-unified-create-copy.md"): {
			"PR-382",
			"统一新增",
			"批量复制",
			"浏览器验收",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing SKU unified create/copy doc marker %q", rel, want)
			}
		}
	}
}
