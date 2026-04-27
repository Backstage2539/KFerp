package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWarehouseInventoryFollowupRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		`code: "DEV-087-03", title: "旧成品库存改造：成品库存从手工覆盖改成成品入库、盘点、转仓单据驱动，并支持多成品仓", status: "done"`,
		`code: "DEV-087-04", title: "旧生产追溯改造：工单冻结 BOM/原料快照，贯通原料入库、转仓、生产消耗和成品批次链路", status: "done"`,
		"UT-087-02",
		"API-087-02",
		"REV-087-02",
		"finished_product_transfers",
		"material_snapshot",
		"/api/stock/trace",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements follow-up seed missing %q", want)
		}
	}
}
