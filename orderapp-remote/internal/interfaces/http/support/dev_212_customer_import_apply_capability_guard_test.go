package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerImportApplyCapabilityGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"capabilityForImportType",
		"ImportTypeProcessingWorkbook",
		"ImportTypeDirectShipWorkbook",
		"ImportTypeSettlementWorkbook",
		"requireCustomerCapability(ctx, customerID, capability)",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing import apply capability marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyDirectShipImportRejectsCustomerWithoutDirectShipCapability",
		"customer capability direct_ship unavailable",
		"customer_direct_ship_import_orders",
		"portal_service_code='direct_ship'",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing import apply capability marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyImportAPICapabilityUnavailableMapsToBadRequest",
		"customer capability direct_ship unavailable",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("customer fulfillment API test missing import apply capability marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD",
		"TestApplyDirectShipImportRejectsCustomerWithoutDirectShipCapability",
		"应用导入批次",
		"CUSTOMER_IMPORT_APPLY_CAPABILITY_REAL_API_OK",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing import apply capability marker %q", want)
		}
	}
}

func TestCustomerImportApplyCapabilityGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD",
		"DEV-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD",
		"UT-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD",
		"API-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD",
		"REV-212-CUSTOMER-IMPORT-APPLY-CAPABILITY-GUARD",
		"CUSTOMER_IMPORT_APPLY_CAPABILITY_REAL_API_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestCustomerImportApplyCapabilityGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"应用导入批次",
			"未开通对应能力",
			"customer capability",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing import apply capability marker %q", path, want)
			}
		}
	}
}
