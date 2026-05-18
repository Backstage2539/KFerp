package costing

import (
	"os"
	"strings"
	"testing"
)

func TestLoadProductInputsReadsBeanMetadataFromProfileTable(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "material_bean_profiles") {
		t.Fatalf("costing repository must join material_bean_profiles for bean-list metadata")
	}
	for _, forbidden := range []string{"m.flavor", "m.origin", "m.processing_station", "m.variety", "m.process_method", "m.grade", "m.altitude", "m.bean_list_note"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("costing repository still reads %s from materials", forbidden)
		}
	}
}

func TestLoadProductInputsDoesNotUsePublishedDefaultPriceAsBeanCost(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if strings.Contains(src, "NULLIF(p.default_price") {
		t.Fatalf("costing repository must not reuse published product default_price as green bean cost")
	}
}

func TestLoadProductInputsUsesAvailableBatchWeightedAverageBeanCost(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"material_valuation",
		"material_batch_locations",
		"material_batches",
		"SUM(l.qty_g::numeric * COALESCE(b.unit_cost,0)) / NULLIF(SUM(l.qty_g),0)",
		"COALESCE(mv.weighted_unit_cost, m.purchase_price, 0)",
		"COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must price BOM beans from available batch weighted average; missing %q", want)
		}
	}
}

func TestLoadProductInputsReadsCategoryGradientTemplates(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"pc.gradient_template_id",
		"product_categories pc",
		"pricing_gradient_templates",
		"pricing_gradient_template_tiers",
		"GradientTemplate = template",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must load category gradient templates; missing %q", want)
		}
	}
}

func TestLoadProductInputsReadsProductMarginOverrideForTemplatePricing(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"p.margin_rate_override::float8",
		"&input.MarginRateOverride",
		"margin_rate_override",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must load product margin override; missing %q", want)
		}
	}
}

func TestPublishBeanListUsesQueryRowBeforeAuditToAvoidBusyConnection(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) PublishBeanList")
	end := strings.Index(src, "func (r Repository) WithdrawBeanList")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("PublishBeanList function not found")
	}
	body := src[start:end]
	if !strings.Contains(body, "tx.QueryRow(ctx, fmt.Sprintf(`") {
		t.Fatalf("PublishBeanList must use QueryRow for INSERT ... RETURNING before audit writes")
	}
	if strings.Contains(body, "tx.Query(ctx, fmt.Sprintf(`\n\t\tINSERT INTO %s.bean_list_publications") {
		t.Fatalf("PublishBeanList must not leave pgx Rows open before AuditInsertTx; it causes conn busy")
	}
}

func TestPublishedBeanListReadsOnlyCurrentPublishedSnapshot(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) PublishedBeanList")
	end := strings.Index(src, "func (r Repository) PublishBeanList")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("PublishedBeanList function not found")
	}
	body := src[start:end]
	for _, want := range []string{
		"WHERE list_type=$1 AND owner_type=$2 AND owner_key=$3 AND status='published'",
		"ORDER BY published_at DESC, id DESC",
		"LIMIT 1",
		"pgx.ErrNoRows",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PublishedBeanList must read current published snapshot; missing %q", want)
		}
	}
}

func TestBeanListPublicationSchemaSupportsOwnedLockedSnapshots(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"owner_type TEXT NOT NULL DEFAULT 'official'",
		"owner_key TEXT NOT NULL DEFAULT ''",
		"price_source_publication_id BIGINT",
		"style_source_publication_id BIGINT",
		"source_version_no TEXT NOT NULL DEFAULT ''",
		"bean_list_publications_one_published_owner_idx",
		"ON %[1]s.bean_list_publications(list_type, owner_type, owner_key)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean list publication schema must support owned locked snapshots; missing %q", want)
		}
	}
}

func TestPublishBeanListWithdrawsOnlySameOwnerSnapshot(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) PublishBeanList")
	end := strings.Index(src, "func (r Repository) WithdrawBeanList")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("PublishBeanList function not found")
	}
	body := src[start:end]
	for _, want := range []string{
		"WHERE list_type=$1 AND owner_type=$2 AND owner_key=$3 AND status='published'",
		"price_source_publication_id",
		"style_source_publication_id",
		"source_version_no",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PublishBeanList must lock customer snapshots independently; missing %q", want)
		}
	}
}

func TestPublishRunPublishesDripPriceTiersAsUnitAndBoxSnapshots(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) PublishRun")
	end := strings.Index(src, "func loadRunItems")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("PublishRun function not found")
	}
	body := src[start:end]
	for _, want := range []string{
		"ProductKind",
		"drip_bag",
		"product_kind",
		"price_basis",
		"sales_unit",
		"unit_bag_count",
		"price_source_json",
		"bag",
		"box",
		"math.Ceil",
		"PackedPricePerBag",
		"LoosePricePerBag",
		"TemplateID",
		"TemplateTierID",
		"BagGrams",
		"BoxBagCount",
		"Multiplier",
		"TaxRate",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PublishRun must publish drip bag/box price snapshots; missing %q", want)
		}
	}
}

func TestDefaultDripPriceTemplateSchemaAndSeed(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"drip_price_templates",
		"drip_price_template_tiers",
		"默认挂耳供应价",
		"product_kind TEXT NOT NULL DEFAULT 'roasted_bean'",
		"price_basis TEXT NOT NULL DEFAULT 'weight'",
		"sales_unit TEXT NOT NULL DEFAULT ''",
		"unit_bag_count INT NOT NULL DEFAULT 0",
		"price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"100袋",
		"1000袋",
		"5000袋",
		"10000袋",
		"2.2",
		"1.8",
		"1.6",
		"1.35",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing schema must create and seed drip price templates; missing %q", want)
		}
	}
}

func TestSaveBeanListDraftInsertsCustomerDraftWithoutPublishing(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) SaveBeanListDraft")
	end := strings.Index(src, "func (r Repository) WithdrawBeanList")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("SaveBeanListDraft function not found before WithdrawBeanList")
	}
	body := src[start:end]
	for _, want := range []string{
		"VALUES($1,$2,'draft'",
		"owner_type",
		"owner_key",
		"price_source_publication_id",
		"style_source_publication_id",
		"source_version_no",
		"save_draft",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("SaveBeanListDraft must insert owned draft snapshots; missing %q", want)
		}
	}
	if strings.Contains(body, "SET status='withdrawn'") {
		t.Fatalf("SaveBeanListDraft must not withdraw published rows")
	}
}
