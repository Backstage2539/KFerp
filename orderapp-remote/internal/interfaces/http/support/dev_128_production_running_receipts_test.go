package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev128ProductionRunningAndMaterialReceiptRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		"PR-128",
		"DEV-128-01",
		"DEV-128-02",
		"DEV-128-03",
		"UT-128-01",
		"API-128-01",
		"REV-128-01",
		"原料入库物料模糊搜索",
		"生产中投料和成品数合并编辑",
		"部分完工说明",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
