package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeanListPDFRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-086",
		"DEV-086-01",
		"DEV-086-02",
		"UT-086-01",
		"API-086-01",
		"REV-086-01",
		"生成PDF格式豆单",
		"V3.0.5",
		"背景上传",
		"手机查看",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}

func TestBeanListPDFPreviewFixRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-088",
		"DEV-088-01",
		"DEV-088-02",
		"UT-088-01",
		"API-088-01",
		"REV-088-01",
		"生成前预览",
		"报价、风味、特点、出品建议",
		"避免只生成曲奇",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("preview fix requirements seed missing %q", want)
		}
	}
}
