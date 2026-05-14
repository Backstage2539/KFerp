package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestThreeTemplateBusinessContractEvidenceExists(t *testing.T) {
	serviceTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service_test.go")))
	miniAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"TestDefaultCapabilityTemplatesRuntimeBusinessContract",
		"CapabilityTemplateProcessingFulfillment",
		"CapabilityTemplatePublicSKUDirectShip",
		"CapabilityTemplateRetailMall",
		"CreateMallOrder",
		"CreateDirectShipBatch",
		"CreateProcessingRequest",
		"CreateFulfillmentOrder",
	} {
		if !strings.Contains(serviceTest, want) {
			t.Fatalf("service contract test missing marker %q", want)
		}
	}
	if !strings.Contains(miniAPITest, "TestMiniAPITemplateBusinessContract") {
		t.Fatal("mini API contract test missing TestMiniAPITemplateBusinessContract")
	}
	for _, want := range []string{
		"三模板业务契约矩阵",
		"processing_fulfillment",
		"public_sku_direct_ship",
		"retail_mall",
		"DB/E2E",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing marker %q", want)
		}
	}
}

func TestThreeTemplateBusinessContractRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-176-THREE-TEMPLATE-BUSINESS-CONTRACT",
		"DEV-176-THREE-TEMPLATE-BUSINESS-CONTRACT",
		"UT-176-THREE-TEMPLATE-BUSINESS-CONTRACT",
		"API-176-THREE-TEMPLATE-BUSINESS-CONTRACT",
		"REV-176-THREE-TEMPLATE-BUSINESS-CONTRACT",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
