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
			"按任务执行与完工入库（PR-654）",
			"开始本任务",
			"第 N 批/共 N 批",
		},
		filepath.Join(repoRoot, "docs", "OP_MANUAL_STOCK.md"): {
			"工位任务领料、耗料与完工入库（PR-654）",
			"任务的预留不能被另一个批次重复使用",
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
