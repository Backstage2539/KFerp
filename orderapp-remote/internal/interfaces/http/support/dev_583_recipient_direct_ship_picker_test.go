package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev583RequirementManualAndMiniappContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-583-RECIPIENT-COMPACT-ADDRESS-DIRECT-SHIP-PICKER",
			"DEV-583-COMPACT-RECIPIENT-PARSE",
			"DEV-583-SHARED-PRODUCT-PICKER",
			"DEV-583-DIRECT-SHIP-LINES",
			"DEV-583-DOCS-ACCEPTANCE-DEPLOY",
			"REV-583-RECIPIENT-COMPACT-ADDRESS-DIRECT-SHIP-PICKER",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-583-RECIPIENT-COMPACT-ADDRESS-DIRECT-SHIP-PICKER",
			"低置信度",
			"商品、规格、数量",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-583-RECIPIENT-COMPACT-ADDRESS-DIRECT-SHIP-PICKER",
			"景谷傣族彝族自治县",
			"共享商品选择弹层",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"): {
			"连写收货信息",
			"商品、规格、数量",
			"无法可靠识别姓名",
		},
	}

	for path, wants := range checks {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(body)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}
}
