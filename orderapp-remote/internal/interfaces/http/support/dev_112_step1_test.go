package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerBeanListSnapshotRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-112",
		"DEV-112-01",
		"DEV-112-02",
		"DEV-112-03",
		"UT-112-01",
		"API-112-01",
		"REV-112-01",
		"客户豆单发布后锁定为自己的快照",
		"复制官方价格来源",
		"复制自己的历史样式配置",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer bean-list snapshot requirement seed missing %q", want)
		}
	}
}
