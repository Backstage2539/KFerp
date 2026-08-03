package support

import (
	"os"
	"strings"
	"testing"
)

func TestDev572MiniappOrderDetailDocumentShareContract(t *testing.T) {
	assertSourceMarkers := func(relativePath string, markers ...string) string {
		t.Helper()
		path := repoFilePath(t, relativePath)
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
	listPage := assertSourceMarkers(
		"miniapp/src/pages/employee-orders/employee-orders.vue",
		"<template v-for=\"row in rows\"", "<navigator v-if=\"row.detail_url\"", ":url=\"row.detail_url\"",
		"<view v-else class=\"card card-disabled\">", "订单编号异常，无法查看", "employeeOrderNavigationRows", "rememberListQuery",
	)
	if strings.Contains(listPage, "@tap=\"openOrderDetail(row)\"") {
		t.Error("employee order cards must navigate natively instead of relying on a dynamic tap handler")
	}
	if strings.Contains(listPage, ":url=\"employeeOrderDetailPagePath(row.id)\"") {
		t.Error("employee order navigator must not render a runtime path that can become an empty url")
	}
	assertSourceMarkers(
		"miniapp/src/utils/employeeOrderDetail.ts",
		"employeeOrderDetailPagePath", "employeeOrderNavigationRows", "/pages/employee-order-detail/employee-order-detail?id=",
	)
	detailPage := assertSourceMarkers(
		"miniapp/src/pages/employee-order-detail/employee-order-detail.vue",
		"收件信息", "物流信息", "订单状态", "费用明细", "商品明细", "报价来源", "生产来源",
		"销售单 PDF", "销售单图片", "发货单 PDF", "发货单图片",
	)
	documentSectionIndex := strings.Index(detailPage, `<view class="section document-section">`)
	heroSectionIndex := strings.Index(detailPage, `<view class="hero-card">`)
	if documentSectionIndex < 0 || heroSectionIndex < 0 || documentSectionIndex > heroSectionIndex {
		t.Errorf("导出并微信分享必须是订单加载成功后的首个业务卡片: document=%d hero=%d", documentSectionIndex, heroSectionIndex)
	}
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
