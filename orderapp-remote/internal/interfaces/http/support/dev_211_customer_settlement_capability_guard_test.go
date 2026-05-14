package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSettlementCapabilityGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"requireCustomerCapability(ctx, cmd.CustomerID, \"settlement\")",
		"customer capability settlement unavailable",
	} {
		if !strings.Contains(repository, want) && !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment settlement capability guard missing marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateSettlementRejectsCustomerWithoutSettlementCapability",
		"customer_settlement_batches",
		"status='unsettled'",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing settlement capability marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateSettlementAPICapabilityUnavailableMapsToBadRequest",
		"customer capability settlement unavailable",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("customer fulfillment API test missing settlement capability marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD",
		"TestCreateSettlementRejectsCustomerWithoutSettlementCapability",
		"未开通结算能力",
		"CUSTOMER_SETTLEMENT_CAPABILITY_REAL_API_OK",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing settlement capability marker %q", want)
		}
	}
}

func TestCustomerSettlementCapabilityGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD",
		"DEV-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD",
		"UT-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD",
		"API-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD",
		"REV-211-CUSTOMER-SETTLEMENT-CAPABILITY-GUARD",
		"CUSTOMER_SETTLEMENT_CAPABILITY_REAL_API_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestCustomerSettlementCapabilityGuardManualsAndRequirementDocs(t *testing.T) {
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
			"结算能力",
			"未开通结算",
			"不能生成结算批次",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing settlement capability marker %q", path, want)
			}
		}
	}
}
