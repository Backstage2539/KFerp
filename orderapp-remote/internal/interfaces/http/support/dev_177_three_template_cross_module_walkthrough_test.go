package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestThreeTemplateCrossModuleWalkthroughEvidenceExists(t *testing.T) {
	walkthrough := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "businessaudit", "three_template_walkthrough_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"TestThreeTemplateBusinessWalkthroughAcrossModules",
		"customerportalapp.NewService",
		"customerfulfillmentapp.NewService",
		"productionapp.NewService",
		"financeapp.NewService",
		"processing_fulfillment",
		"public_sku_direct_ship",
		"retail_mall",
	} {
		if !strings.Contains(walkthrough, want) {
			t.Fatalf("cross-module walkthrough missing marker %q", want)
		}
	}
	for _, want := range []string{
		"跨模块业务走查矩阵",
		"Prompt-to-artifact checklist",
		"完成度审计",
		"当前结论：未完成",
		"真实环境全流程",
		"浏览器或者任何方法跑一遍",
		"PR-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH",
		"订单、生产、财务、客户履约工作台",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing marker %q", want)
		}
	}
}

func TestThreeTemplateCrossModuleWalkthroughRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH",
		"DEV-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH",
		"UT-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH",
		"API-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH",
		"REV-177-THREE-TEMPLATE-CROSS-MODULE-WALKTHROUGH",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
