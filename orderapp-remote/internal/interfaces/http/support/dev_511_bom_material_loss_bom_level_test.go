package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev511BomMaterialLossBomLevelContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"DEV-511-BOM-LEVEL-MATERIAL-LOSS",
			"DEV-511-BOM-LEVEL-MATERIAL-LOSS-UI",
			"DEV-511-DOCS-ACCEPTANCE",
			"API-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"REV-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"原料损耗比",
			"比例 %",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"开启后组件消耗单位只能使用比例 %",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"production_bom_versions.material_loss_rate",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"40%",
			"20%",
			"50%",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"实际原料需求 = 计划投料基准 × 配方比例 ÷ (1 - 原料损耗率)",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"净配方数量 ÷ (1 - 原料损耗率)",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-592-BOM-LOSS-GROSS-INPUT",
			"损耗后用量 = BOM组成 ÷ (1 - 原料损耗率)",
		},
		filepath.Join("docs", "acceptance", "2026-06-30-bom-material-loss-bom-level.md"): {
			"PR-511-BOM-MATERIAL-LOSS-BOM-LEVEL",
			"BOM 版本级",
			"比例 %",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-511 marker %q", rel, want)
			}
		}
	}
}
