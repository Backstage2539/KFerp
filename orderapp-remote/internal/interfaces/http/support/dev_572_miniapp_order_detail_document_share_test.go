package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev572MiniappOrderDetailDocumentShareContract(t *testing.T) {
	orderappRoot := findAncestorForTest(t, "go.mod")
	workspaceRoot := filepath.Dir(orderappRoot)

	assertSourceMarkers := func(relativePath string, markers ...string) string {
		t.Helper()
		path := filepath.Join(workspaceRoot, relativePath)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		source := string(content)
		for _, marker := range markers {
			if !strings.Contains(source, marker) {
				t.Errorf("%s missing %q", relativePath, marker)
			}
		}
		return source
	}

	assertSourceMarkers("miniapp/src/pages.json", "pages/employee-order-detail/employee-order-detail")
	assertSourceMarkers("miniapp/src/pages/employee-orders/employee-orders.vue", "openOrderDetail", "pages/employee-order-detail/employee-order-detail?id=")
	detailPage := assertSourceMarkers(
		"miniapp/src/pages/employee-order-detail/employee-order-detail.vue",
		"收件信息", "物流信息", "订单状态", "费用明细", "商品明细", "报价来源", "生产来源",
		"销售单 PDF", "销售单图片", "发货单 PDF", "发货单图片",
	)
	if !strings.Contains(detailPage, "shareMiniappFileOutput") {
		t.Error("employee order detail does not use the shared WeChat file output helper")
	}

	entryPage := assertSourceMarkers("miniapp/src/pages/employee-order-entry/employee-order-entry.vue", "@tap=\"addItem\"", "class=\"order-total\"")
	addIndex := strings.Index(entryPage, "@tap=\"addItem\"")
	itemIndex := strings.Index(entryPage, "v-for=\"(item, index) in form.items\"")
	totalIndex := strings.Index(entryPage, "class=\"order-total\"")
	if addIndex < itemIndex || addIndex > totalIndex {
		t.Errorf("新增商品按钮必须位于商品明细循环之后、订单合计之前: item=%d add=%d total=%d", itemIndex, addIndex, totalIndex)
	}

	assertSourceMarkers(
		"miniapp/src/api/customerPortal.ts",
		"buildEmployeeOrderDetailPath", "fetchEmployeeOrderDetail", "sales-order.pdf",
		"sales-order.png", "delivery-note.pdf", "delivery-note.png",
	)
	assertSourceMarkers("miniapp/src/utils/fileOutput.ts", "shareMiniappFileOutput", "shareFileMessage", "showShareImageMenu", "showMenu: true")

	assertSourceMarkers(
		"orderapp-remote/internal/interfaces/http/customerportal/mini_employee_api.go",
		"/api/mini/employee/orders/:id", "sales-order.pdf", "sales-order.png",
		"delivery-note.pdf", "delivery-note.png", "ensureMiniEmployeeOrderAccess",
	)
	assertSourceMarkers("orderapp-remote/internal/application/sales/service.go", "DeliveryNoteImageFile", "LoadDeliveryNoteImageFile")
	assertSourceMarkers("orderapp-remote/internal/infrastructure/pdf/delivery_note_png.go", "RenderPNG")

	for _, relativePath := range []string{
		"REQUIREMENTS.md",
		"ACCEPTANCE_TESTS.md",
		"orderapp-remote/docs/REQUIREMENTS.md",
		"orderapp-remote/docs/ACCEPTANCE_TESTS.md",
		"orderapp-remote/docs/OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md",
		"orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md",
		"orderapp-remote/internal/interfaces/http/support/req_store.go",
	} {
		assertSourceMarkers(relativePath, "PR-572-MINIAPP-ORDER-DETAIL-DOCUMENT-SHARE")
	}
}
