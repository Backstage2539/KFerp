package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev510BomMaterialLossBomLevelContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"DEV-510-BOM-LEVEL-MATERIAL-LOSS",
			"DEV-510-BOM-LEVEL-MATERIAL-LOSS-UI",
			"DEV-510-DOCS-ACCEPTANCE",
			"API-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"REV-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"原料损耗比",
			"比例 %",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"开启后组件消耗单位只能使用比例 %",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"production_bom_versions.material_loss_rate",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"40%",
			"20%",
			"0.5kg",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"开启后组件消耗单位只能使用比例 %",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"损耗后需求量",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"ratio / (1 - 原料损耗比)",
		},
		filepath.Join("docs", "acceptance", "2026-06-30-bom-material-loss-bom-level.md"): {
			"PR-510-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"BOM 版本级",
			"比例 %",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-510 marker %q", rel, want)
			}
		}
	}
}
