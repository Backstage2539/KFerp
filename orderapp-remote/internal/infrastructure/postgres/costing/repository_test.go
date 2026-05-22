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

func TestLoadProductInputsUsesBomCostSnapshotForGreenBeanCost(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"unit_cost_snapshot",
		"p.product_kind",
		"green_bean",
		"bi.ratio_pct",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must price green beans from BOM cost snapshots; missing %q", want)
		}
	}
}

func TestLoadProductInputsPricesDripFromFinishedProductComponentCost(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"finished_product_cost",
		"finished_component_cost",
		"component_type,''),'material') = 'finished_product'",
		"finished_green_cost_per_kg",
		"p.product_kind",
		"drip_bag",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must price drip products from finished-product BOM components; missing %q", want)
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

func TestLoadProductInputsReadsSkuCategoryPathForCustomerBeanLists(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"p.product_category_position",
		"parent_pc.name",
		"parent_pc.position",
		"&input.CategoryPrimaryName",
		"&input.CategorySecondaryName",
		"&input.ProductCategoryPosition",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must load SKU category path for customer bean lists; missing %q", want)
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

func TestLoadProductInputsUsesGreenBeanBoundBomProductForCosting(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"green_bean_bom_product_id",
		"bom_product_id",
		"b.product_id = bom_product_id",
		"bi.product_id = bom_product_id",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("green bean costing must read BOM through bound roasted product; missing %q", want)
		}
	}
}

func TestLoadProductInputsDoesNotLoadGreenBeanDirectSaleTiers(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, forbidden := range []string{
		"loadGreenBeanSaleTiers",
		"green_bean_direct",
		"GreenBeanSaleTiers = tiers",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("green bean sale tiers must come from costing templates, not direct product price tiers; found %q", forbidden)
		}
	}
}

func TestLoadProductInputsReadsLatestPassedProductionQualityForBeanList(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"quality_inspections qi",
		"qi.result='pass'",
		"qi_work_order.product_id=p.bom_product_id",
		"qi_finished_batch.item_id=p.bom_product_id",
		"ORDER BY qi.created_at DESC, qi.id DESC",
		"factory_flavor_description",
		"moisture",
		"density",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean list QC must use latest passed production inspection; missing %q", want)
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
		"math.Floor",
		"dripBoxMinQty",
		"dripBoxMaxQty",
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
