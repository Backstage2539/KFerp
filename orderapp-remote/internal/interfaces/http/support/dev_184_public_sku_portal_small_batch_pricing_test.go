package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicSKUPortalSmallBatchPricingEvidenceExists(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	repoTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository_source_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"portalPublishedPricingTx",
		"ResolvePublishedPricingForPublicationWithUnit",
		"portalPublishedPriceSourceSnapshot",
		"portalLineTotalFromPriceUnit",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("customerportal repository missing published snapshot pricing marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCustomerPortalFulfillmentPricingUsesPublishedSnapshotsOnly",
		"published_price_snapshot",
		"source_price_record_id",
	} {
		if !strings.Contains(repoTest, want) {
			t.Fatalf("customerportal repository test missing published snapshot pricing marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING",
		"TestCreateFulfillmentOrderUsesSmallBatchWeightTierForNon454Spec",
		"公共 SKU 小批量小程序订单计价",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing public SKU pricing marker %q", want)
		}
	}
}

func TestPublicSKUPortalSmallBatchPricingRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING",
		"DEV-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING",
		"UT-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING",
		"API-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING",
		"REV-184-PUBLIC-SKU-PORTAL-SMALL-BATCH-PRICING",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestPublicSKUPortalSmallBatchPricingManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"公共 SKU 小批量",
			"已发布商品价格表快照",
			"库存换算",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing public SKU snapshot pricing marker %q", path, want)
			}
		}
	}
}
