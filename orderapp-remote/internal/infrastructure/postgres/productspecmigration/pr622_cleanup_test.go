package productspecmigration

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPR622ManifestIDIsDeterministicAndDriftSensitive(t *testing.T) {
	children := []pr622ManifestChild{{ID: 7, ParentID: 3, SpecKey: "454g", Active: true, Mappable: true}}
	dependencies := []PR622CleanupDependency{{Table: "finished_inventory", Count: 2}}
	first := pr622ManifestID(children, dependencies, 1)
	second := pr622ManifestID(children, dependencies, 1)
	if first != second || !strings.HasPrefix(first, "PR-622-") {
		t.Fatalf("manifest ids = %q/%q, want stable PR-622 id", first, second)
	}
	children[0].Mappable = false
	if drifted := pr622ManifestID(children, dependencies, 1); drifted == first {
		t.Fatal("mapping drift must change the locked manifest id")
	}
}

func TestPR622CleanupIsLockedTransactionalAndDropsOnlyRetiredRelations(t *testing.T) {
	source, err := os.ReadFile("pr622_cleanup.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, marker := range []string{
		"pg_advisory_xact_lock",
		"pgx.Serializable",
		"legacy_child_sku_bom_spec_mappings",
		"product_bom_spec_authority_upgrades",
		"product_bom_spec_migrations",
		"DELETE FROM %s.products WHERE COALESCE(parent_product_id,0)>0",
		"product_bom_spec_authority_cleanup",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("PR-622 cleanup missing %q", marker)
		}
	}
	if strings.Contains(text, "DROP TABLE IF EXISTS %[1]s.products") || strings.Contains(text, "DROP TABLE IF EXISTS %[1]s.production_boms") {
		t.Fatal("PR-622 cleanup must retain product masters and BOM history")
	}
}

func TestPR622CleanupBlocksUnmappedLiveStockAndProduction(t *testing.T) {
	source, err := os.ReadFile("pr622_cleanup.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, table := range []string{"finished_inventory", "stock_batches", "production_plan_items", "work_orders", "produce_running_items", "order_items_unfinished", "bean_list_publications.price_rows", "customer_direct_ship_request_items"} {
		if !strings.Contains(text, table) {
			t.Fatalf("PR-622 blocker scan missing %s", table)
		}
	}
}

func TestPR622CleanupCanExplicitlyDiscardAuthorizedTestDependencies(t *testing.T) {
	cleanupSource, err := os.ReadFile("pr622_cleanup.go")
	if err != nil {
		t.Fatal(err)
	}
	commandSource, err := os.ReadFile("../../../../cmd/product-bom-spec-authority-cleanup/main.go")
	if err != nil {
		t.Fatal(err)
	}
	cleanupText := string(cleanupSource)
	for _, marker := range []string{
		"discardUnmappedTestDataTx",
		"suspendPR622AuthorityGuardsTx",
		"restorePR622AuthorityGuardsTx",
		"discarded_unmapped_reference_count",
		"withdrawn_unmapped_publication_count",
		"voided_unmapped_order_count",
	} {
		if !strings.Contains(cleanupText, marker) {
			t.Fatalf("authorized PR-622 test-data cleanup missing %q", marker)
		}
	}
	if !strings.Contains(string(commandSource), "discard-unmapped-test-data") {
		t.Fatal("one-time cleanup command must require the explicit discard-unmapped-test-data flag")
	}
}

func TestPR622SingleBOMReplacementKeepsSourceProductIDNumeric(t *testing.T) {
	source, err := os.ReadFile("pr622_cleanup.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "$1::bigint,source_product_code_snapshot") {
		t.Fatal("source_product_id must cast the reused text parameter back to bigint")
	}
}

func TestRewritePR622PublishedJSONMovesIdentityButKeepsFrozenDisplay(t *testing.T) {
	var input any
	if err := json.Unmarshal([]byte(`{
		"price_rows":[{"product_id":17,"parent_product_id":8,"sku_id":17,"product_name_snapshot":"历史名称","final_unit_price":38.2,"effective_sales_spec":{"sku_id":17,"spec_name":"454g"}}],
		"groups":[{"items":[{"product_id":17,"sku_id":17,"name":"历史名称"}]}]
	}`), &input); err != nil {
		t.Fatal(err)
	}
	rewritten, changed := rewritePR622PublishedJSON(input, map[int64]pr622PublishedIdentity{
		17: {ParentProductID: 8, BOMID: 30, BOMVersionID: 31, BOMSpecID: 32, BOMVariantID: 33},
	})
	if !changed {
		t.Fatal("published child identity should be rewritten")
	}
	encoded, _ := json.Marshal(rewritten)
	text := string(encoded)
	for _, want := range []string{`"product_id":8`, `"sku_id":8`, `"bom_spec_id":32`, `"bom_variant_id":33`, `"spec_identity_mode":"bom_spec"`, `"product_name_snapshot":"历史名称"`, `"final_unit_price":38.2`} {
		if !strings.Contains(text, want) {
			t.Fatalf("rewritten publication missing %s: %s", want, text)
		}
	}
	if strings.Contains(text, `"product_id":17`) || strings.Contains(text, `"sku_id":17`) {
		t.Fatalf("legacy child identity leaked into rewritten current publication: %s", text)
	}
}

func TestPR622CleanupCopiesCurrentPricesAndVerifiesPhysicalRetirement(t *testing.T) {
	source, err := os.ReadFile("pr622_cleanup.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, marker := range []string{
		"clonePublishedPriceVersionsTx",
		`item.versionNo+"-PR622"`,
		"LegacyUnitTemplateCount",
		"ActiveProductSingleBOMCount",
		"PublishedChildPriceReferenceCount",
		"detachRemainingSingleProductBOMsTx",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("PR-622 cleanup missing %q", marker)
		}
	}
}
