package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev132SalesOrderImageRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		"PR-132",
		"DEV-132-01",
		"DEV-132-02",
		"DEV-132-03",
		"UT-132-01",
		"API-132-01",
		"REV-132-01",
		"销售单支持生成图片",
		"PNG 图片版本",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDev132SalesOrderImageAPIRoutesAndVueControls(t *testing.T) {
	httpSrc, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "sales", "sales_order_documents.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`/api/orders/:id/sales-order-images`,
		`/orders/:id/sales-order-image-latest.png`,
		`image/png`,
		`GenerateSalesOrderImage`,
	} {
		if !strings.Contains(string(httpSrc), want) {
			t.Fatalf("sales_order_documents.go missing %q", want)
		}
	}

	viewSrc, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"确认生成图片",
		"下载最新版图片",
		"图片版本",
		"salesOrderImageDownloadUrl",
	} {
		if !strings.Contains(string(viewSrc), want) {
			t.Fatalf("SalesOrderView.vue missing %q", want)
		}
	}
}
