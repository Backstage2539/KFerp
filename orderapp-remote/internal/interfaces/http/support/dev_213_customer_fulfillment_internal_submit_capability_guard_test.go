package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerFulfillmentInternalSubmitCapabilityGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"requireCustomerCapability(ctx, customerID, \"processing\")",
		"requireCustomerCapability(ctx, customerID, \"direct_ship\")",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing internal submit capability marker %q", want)
		}
	}
	if strings.Contains(repository, "fromCustomerPortal") {
		t.Fatalf("customer fulfillment repository still scopes submit capability guard to customer portal path")
	}
	for _, want := range []string{
		"TestInternalCustomerFulfillmentSubmitRequiresCustomerCapability",
		"customer capability processing unavailable",
		"customer capability direct_ship unavailable",
		"customer_processing_work_orders",
		"customer_direct_ship_import_orders",
		"portal_service_code='direct_ship'",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing internal submit marker %q", want)
		}
	}
	for _, want := range []string{
		"TestInternalSubmitAPICapabilityUnavailableMapsToBadRequest",
		"customer capability processing unavailable",
		"customer capability direct_ship unavailable",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("customer fulfillment API test missing internal submit marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD",
		"TestInternalCustomerFulfillmentSubmitRequiresCustomerCapability",
		"手工提交",
		"CUSTOMER_INTERNAL_SUBMIT_CAPABILITY_REAL_API_OK",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing internal submit marker %q", want)
		}
	}
}

func TestCustomerFulfillmentInternalSubmitCapabilityGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD",
		"DEV-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD",
		"UT-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD",
		"API-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD",
		"REV-213-CUSTOMER-FULFILLMENT-INTERNAL-SUBMIT-CAPABILITY-GUARD",
		"CUSTOMER_INTERNAL_SUBMIT_CAPABILITY_REAL_API_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestCustomerFulfillmentInternalSubmitCapabilityGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"手工提交",
			"未开通对应能力",
			"customer capability",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing internal submit capability marker %q", path, want)
			}
		}
	}
}
