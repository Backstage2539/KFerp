package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev592BomLossGrossInputContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-592-BOM-LOSS-GROSS-INPUT",
			"DEV-592-COSTING-GROSS-INPUT-LOSS",
			"DEV-592-PRODUCTION-GROSS-INPUT-LOSS",
			"DEV-592-DOCS-DEVELOPMENT-DELIVERY",
			"REV-592-BOM-LOSS-GROSS-INPUT",
		},
		filepath.Join("internal", "domain", "production", "yield.go"): {
			"float64(needG) / (1 - lossRate)",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "production_bom_cost.go"): {
			"item.RatioPct / 100 / (1 - lossRate)",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"row.RatioPct / (1 - row.MaterialLossRate)",
			"/ (1 - LEAST(GREATEST(COALESCE(bi.material_loss_rate,0),0),0.9999))",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "material_consumption.go"): {
			`yieldDenominatorMaterialLossCalculationMode = "yield_denominator"`,
			"return 1 / (1 - rate)",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"materialLossAdjustedRatioPct",
			"配方比例 ÷ (1 - 原料损耗率)",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "bom.js"): {
			"ratioPct / (1 - rate)",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-592-BOM-LOSS-GROSS-INPUT",
			"25% ÷ (1 - 19.5%) = 31.0559%",
			"31.0559% × 78 = 24.22元",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"6356 ÷ 0.82 ≈ 7751.22g",
			"向上取整为7752g",
		},
		filepath.Join("docs", "acceptance", "2026-08-10-bom-loss-gross-input.md"): {
			"初晓",
			"熟豆24磅模板-正常-418",
			"24.22元",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-592 marker %q", rel, want)
			}
		}
	}

	for rel, forbidden := range map[string]string{
		filepath.Join("internal", "domain", "production", "yield.go"):      "float64(needG) * (1 + lossRate)",
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): "配方比例 × (1 + 原料损耗比)",
	} {
		src := string(readOrderAppFileForTest(t, rel))
		if strings.Contains(src, forbidden) {
			t.Fatalf("%s retains additive BOM-loss marker %q", rel, forbidden)
		}
	}
}
