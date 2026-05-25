package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerFulfillmentCustodyAdjustmentCapabilityGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"func (r *Repository) AdjustCustodyInventory",
		"requireCustomerCapability(ctx, cmd.CustomerID, \"inventory_custody\")",
		"customer_custody_ledger_entries",
		"customer_custody_balances",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing custody capability marker %q", want)
		}
	}
	for _, want := range []string{
		"TestAdjustCustodyInventoryRequiresCustomerInventoryCapability",
		"customer capability inventory_custody unavailable",
		"customer_custody_items",
		"customer_custody_ledger_entries",
		"customer_custody_balances",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing custody capability marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCustodyAdjustmentAPICapabilityUnavailableMapsToBadRequest",
		"customer capability inventory_custody unavailable",
		"custodyAdjustmentErr",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("customer fulfillment API test missing custody capability marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD",
		"TestAdjustCustodyInventoryRequiresCustomerInventoryCapability",
		"托管库存",
		"CUSTOMER_CUSTODY_ADJUSTMENT_CAPABILITY_REAL_API_OK",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing custody capability marker %q", want)
		}
	}
}

func TestCustomerFulfillmentCustodyAdjustmentCapabilityGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD",
		"DEV-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD",
		"UT-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD",
		"API-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD",
		"REV-214-CUSTOMER-FULFILLMENT-CUSTODY-ADJUSTMENT-CAPABILITY-GUARD",
		"CUSTOMER_CUSTODY_ADJUSTMENT_CAPABILITY_REAL_API_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestCustomerFulfillmentCustodyAdjustmentCapabilityGuardManualsAndRequirementDocs(t *testing.T) {
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
			"inventory_custody",
			"托管库存",
			"customer capability inventory_custody unavailable",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing custody adjustment capability marker %q", path, want)
			}
		}
	}
}
