package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestThreeTemplateBrowserClickSmokeEvidenceExists(t *testing.T) {
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"PR-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE",
		"CLICK_LEVEL_SMOKE_OK app=http://127.0.0.1:18085",
		"SO-20260513-PAGE",
		"点击级财务费用",
		"E2E代加工成品",
		"已提交工单 CP-",
		"已提交代发 CDS-",
		"E2E公共SKU代发客户",
		"EXTENDED_DOM_SMOKE_OK app=http://127.0.0.1:18081 pg=55435",
		"customerPortalSettings",
		"financeDashboard",
		"EXTENDED_CLICK_LEVEL_SMOKE_OK app=http://127.0.0.1:18090 pg=55448",
		"SO-20260513-FINISHCLICK",
		"E2E完工点击生豆",
		"生产已完成",
		"月度经营报告",
		"来源明细",
		"点击级月结调整",
		"running_done=1",
		"adjustments=1",
		"monthly_status=adjusted",
		"已新增调整",
		"微信开发者工具",
		"Service Port 已开启",
		"miniapp/dist/build/mp-weixin",
		"WECHAT_GUI_CLICK_OK",
		"template=processing_fulfillment",
		"product_picker_default_price_only",
		"WECHAT_GUI_PUBLIC_SKU_CLICK_OK",
		"template=public_sku_direct_ship",
		"SO-20260514-0007",
		"no_unit_price_input",
		"WECHAT_GUI_RETAIL_MALL_CLICK_OK",
		"template=retail_mall",
		"SO-20260514-0006",
		"当前结论：未完成",
		"2026-05-14 收尾审计",
		"收尾验证命令",
		"go test ./...",
		"npm run build:mp-weixin",
		"ERP_MENU_RENDER_MATRIX_SMOKE_OK",
		"views=29",
		"workOrders,jobCards,qualityInspections,produceLogs,productionCosts,stockOperations,stockOutboundLogs,purchase,materials,productSettings,mallSettings,costing,bom,order,customers,salesOrderSettings,senderSettings,orderInvoice,salesOrder,deliveryNote,financeSettings,customerPortalSettings,customerCapabilityTemplates,companyProfile,machines,userPermissions,employees,departments,audit",
		"port_18159_free",
		"剩余 ERP 点击矩阵目标",
		"workOrders",
		"qualityInspections",
		"stockOperations",
		"productSettings",
		"mallSettings",
		"financeSettings",
		"customerCapabilityTemplates",
		"合并 develop",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing marker %q", want)
		}
	}
}

func TestThreeTemplateBrowserClickSmokeRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE",
		"DEV-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE",
		"UT-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE",
		"API-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE",
		"REV-180-THREE-TEMPLATE-BROWSER-CLICK-SMOKE",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
