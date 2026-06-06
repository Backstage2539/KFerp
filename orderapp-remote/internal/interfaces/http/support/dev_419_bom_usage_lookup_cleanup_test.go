package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev419BomUsageLookupCleanupRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-419-BOM-USAGE-LOOKUP-CLEANUP",
		"DEV-419-BOM-USAGE-BACKEND-COMPONENT-ONLY",
		"DEV-419-BOM-USAGE-UI-CLEANUP",
		"UT-419-BOM-USAGE-LOOKUP-CLEANUP",
		"API-419-BOM-USAGE-LOOKUP-CLEANUP",
		"REV-419-BOM-USAGE-LOOKUP-CLEANUP",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-419 requirement seed missing %q", want)
		}
	}
}

func TestDev419BomUsageLookupCleanupSourceMarkers(t *testing.T) {
	checks := []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"),
			markers: []string{
				"listProductionBomUsageByProduct",
				"'output' AS relation_type",
				"WHERE pb.output_product_id=$1",
				"listProductionBomComponentUsedByBoms",
				"SELECT DISTINCT ON (pb.id)",
				"COALESCE(pb.output_product_id,0)<>$1",
				"component_type IN ('product','finished_product')",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"),
			markers: []string{
				"bomUsageRowKey",
				"isActiveBomUsageRow",
				"bomUsageRelationLabel",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"),
			markers: []string{
				"openBomRowPrimary",
				"openEditProductionBomRecord(bomRecordFromRow(row))",
				"referencedProductKey",
				"isActiveReferencedProduct",
			},
		},
	}
	for _, check := range checks {
		src := string(readOrderAppFileForTest(t, check.rel))
		for _, want := range check.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-419 source marker %q", check.rel, want)
			}
		}
	}
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go")))
	if !strings.Contains(repo, "usedByBoms, err := r.listProductionBomComponentUsedByBoms(ctx, summary.OutputProductID)") {
		t.Fatal("BOM detail should keep upper-BOM component lookup separate from product archive output usage")
	}
}

func TestDev419BomUsageLookupCleanupDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-419-BOM-USAGE-LOOKUP-CLEANUP",
			"产出该商品或把该商品作为组件消耗",
			"点击 BOM 名称直接打开该 BOM 的设置抽屉",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-419-BOM-USAGE-LOOKUP-CLEANUP",
			"BOM-000884 初晓拼配",
			"SKU-000518 初晓",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"产出该商品的有效生产 BOM",
			"点击 BOM 名称直接打开该 BOM 的设置抽屉",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-420",
			"SKU-000518 初晓",
		},
		filepath.Join("docs", "acceptance", "2026-06-05-bom-usage-lookup-cleanup.md"): {
			"PR-419",
			"商品档案 API 已由 PR-420 恢复返回 `relation_type=output`",
		},
	}
	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-419 documentation marker %q", rel, want)
			}
		}
	}
}
