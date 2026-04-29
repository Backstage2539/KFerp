package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeanListPublishAdminOnlyRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-113",
		"DEV-113-01",
		"DEV-113-02",
		"DEV-113-03",
		"UT-113-01",
		"API-113-01",
		"REV-113-01",
		"只有管理员才可以发布豆单",
		"客户登录后只能保存修改和下载豆单",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("admin-only bean-list publishing requirement seed missing %q", want)
		}
	}
}
