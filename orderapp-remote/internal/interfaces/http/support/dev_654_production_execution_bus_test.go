package support

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPR654ProductionExecutionBusDeliveryContract(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test source path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	checks := map[string][]string{
		filepath.Join(repoRoot, "internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-654-PRODUCTION-EXECUTION-BUS",
			"DEV-654-TASK-EXECUTION",
			"REV-654-PRODUCTION-EXECUTION-BUS",
		},
		filepath.Join(repoRoot, "docs", "OP_MANUAL_PRODUCTION.md"): {
			"当前工单与工序操作（PR-729）",
			"开始、暂停用于计时",
			"生产记录留在工单详情内",
		},
		filepath.Join(repoRoot, "docs", "OP_MANUAL_STOCK.md"): {
			"当前生产库存动作（PR-729）",
			"不会跨仓寻找库存，也不会挪用其他工单预留",
		},
	}
	for path, markers := range checks {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, marker := range markers {
			if !strings.Contains(string(body), marker) {
				t.Errorf("%s missing %q", path, marker)
			}
		}
	}
}
