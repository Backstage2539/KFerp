package costing

import (
	domain "orderapp/internal/domain/costing"
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

func TestPricingRuleTrialProductionCostUsesOutputProductBomOnly(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"pb.output_product_id=p.id",
		"pb.output_product_id=$2",
		"LoadPricingRuleTrialProductionOptions",
		"pricing_rule_trial_bom_versions",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("pricing trial production cost must use output product BOM lookup; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"product_production_bom_bindings pbb",
		"product_production_bom_bindings b",
		"pbb.bom_version_id",
		"pbb.bom_id",
		"COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("pricing trial must not reserve product-bound BOM lookup; found %q", forbidden)
		}
	}
}

func TestLoadProductInputsPrefersProductProductionConfigFields(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"product_production_configs ppc",
		"product_production_config_fields ppcf",
		"production_config_attrs_json",
		"production_config_attrs_schema_json",
		"COALESCE(NULLIF(ppc.expected_loss_rate,0)",
		"1 - COALESCE(NULLIF(ppc.expected_loss_rate,0)",
		"ppcf.show_in_price_list=true",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must prefer product production config before legacy fallback; missing %q", want)
		}
	}
}

func TestLoadProductInputsUsesCustomerAliasIndustryFieldOverridesWithoutClassificationAmbiguity(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"alias_config_attrs",
		"customer_product_alias_industry_field_values",
		"current_classification_template_id",
		"current_classification_category_id",
		"CASE WHEN $2 > 0 THEN NULLIF(alias_attrs.alias_attrs_json::text,'{}')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must use alias industry overrides and qualified classification aliases; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"COALESCE(p.classification_template_id,0)",
		"COALESCE(p.classification_category_id,0)",
		"GROUP BY p.id, p.name, p.customer_product_alias_id, p.customer_product_display_name, p.customer_item_code, p.brand_name, p.display_category_id, p.display_category_name, p.classification_template_id",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("costing repository still references ambiguous product_scope classification column %q", forbidden)
		}
	}
}

func TestLoadProductInputsUsesCustomerAliasRenameAsCustomerDisplayName(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"COALESCE(NULLIF(cpa.brand_name,''), NULLIF(cpa.display_name,''), p.name) AS customer_product_display_name",
		"CASE WHEN $2 > 0 THEN COALESCE(NULLIF(p.customer_product_display_name,''), p.name) ELSE p.name END",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer price lists must use alias rename before customer product name; missing %q", want)
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

func TestLoadProductInputsDoesNotFallbackToCategoryGradientTemplates(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	gradientExpr := effectiveGradientTemplateExpr(t, src)
	for _, want := range []string{
		"NULLIF(alias_config.gradient_template_id,0)",
		"NULLIF(p_config.gradient_template_id,0)",
		"NULLIF(p.customer_product_alias_gradient_template_id,0)",
		"NULLIF(cpro.gradient_template_id,0)",
		"NULLIF(cpti.gradient_template_id,0)",
		"NULLIF(p.gradient_template_id_override,0)",
	} {
		if !strings.Contains(gradientExpr, want) {
			t.Fatalf("costing repository must keep explicit gradient template sources; missing %q in %s", want, gradientExpr)
		}
	}
	for _, forbidden := range []string{
		"NULLIF(pc.gradient_template_id,0)",
		"NULLIF(parent_pc.gradient_template_id,0)",
	} {
		if strings.Contains(gradientExpr, forbidden) {
			t.Fatalf("costing repository must not use product/category template gradient as price source; found %q in %s", forbidden, gradientExpr)
		}
	}
	for _, want := range []string{
		"pricing_gradient_templates",
		"pricing_gradient_template_tiers",
		"GradientTemplate = template",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must still load explicit gradient template details; missing %q", want)
		}
	}
}

func TestLoadProductInputsReadsFinalPriceTierSchemes(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"product_tier_price_schemes",
		"product_tier_price_scheme_tiers",
		"tier_label",
		"min_qty",
		"source_price_record_id",
		"inventory_conversion_json",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must project final price tier schemes into price snapshots; missing %q", want)
		}
	}
}

func TestBeanListPublicationQueriesFallbackToLegacyListTypeRows(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"COALESCE(product_type_category_id,0)=$2",
		"COALESCE(product_type_category_id,0)=0 AND list_type=$5",
		"COALESCE(product_type_category_id,0)=$3",
		"COALESCE(product_type_category_id,0)=0 AND list_type=$6",
		"CASE WHEN COALESCE(product_type_category_id,0)=",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean-list publication queries must fall back to legacy list_type rows; missing %q", want)
		}
	}
}

func TestCommercialTiersForPublishDoesNotInventDefaultTiers(t *testing.T) {
	item := domain.ProductResult{
		ProductID:         434,
		Name:              "初晓2.5kg装",
		WholesaleKgPrices: []float64{132, 119, 106, 99},
		WholesaleLbPrices: []float64{61, 55, 49, 46},
	}

	if tiers := commercialTiersForPublish(item); len(tiers) != 0 {
		t.Fatalf("publish tiers = %+v, want none when result has no commercial tiers", tiers)
	}
}

func effectiveGradientTemplateExpr(t *testing.T, src string) string {
	t.Helper()
	marker := ") AS effective_gradient_template_id"
	end := strings.Index(src, marker)
	if end < 0 {
		t.Fatalf("missing effective_gradient_template_id expression")
	}
	start := strings.LastIndex(src[:end], "COALESCE(")
	if start < 0 {
		t.Fatalf("missing COALESCE for effective_gradient_template_id expression")
	}
	return src[start : end+len(marker)]
}

func TestLoadProductInputsResolvesCustomerProductRuleTemplates(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"LoadProductInputsForCustomer",
		"customer_product_rule_overrides cpro",
		"customer_product_rule_template_items cpti",
		"customer_product_rule_template_id",
		"alias_config",
		"p.gradient_template_id_override",
		"effective_gradient_template_id",
		"NULLIF(alias_config.gradient_template_id,0)",
		"NULLIF(p_config.gradient_template_id,0)",
		"NULLIF(cpro.gradient_template_id,0)",
		"NULLIF(cpti.gradient_template_id,0)",
		"NULLIF(p.gradient_template_id_override,0)",
		"&input.InventoryUnit",
		"&input.QuoteUnit",
		"&input.OrderUnit",
		"&input.UnitConversionJSON",
		"&input.IntegerUnit",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must resolve customer product rule templates and unit rules; missing %q", want)
		}
	}
}

func TestLoadProductInputsForCustomerUsesCustomerProductAliasesAsPriceListSource(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"customer_product_aliases cpa",
		"COALESCE(cpa.product_config_template_id,0) AS customer_product_alias_product_config_template_id",
		"COALESCE(cpa.gradient_template_id,0) AS customer_product_alias_gradient_template_id",
		"COALESCE(cpa.unit_template_id,0) AS customer_product_alias_unit_template_id",
		"cpa.include_in_price_list=true",
		"cpa.active=true",
		"cpa.id IS NOT NULL",
		"&input.CustomerProductAliasID",
		"&input.CustomerProductDisplayName",
		"&input.CustomerItemCode",
		"&input.ProductCode",
		"&input.ProductName",
		"&input.BomVersionID",
		"&input.BomUsageMode",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer price lists must load products through customer_product_aliases; missing %q", want)
		}
	}
}

func TestLoadProductInputsForCustomerAliasConfigTemplateOverridesProductTemplateAndKeepsLegacyFallback(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	gradientExpr := effectiveGradientTemplateExpr(t, src)
	firstAliasConfig := strings.Index(gradientExpr, "NULLIF(alias_config.gradient_template_id,0)")
	firstProductConfig := strings.Index(gradientExpr, "NULLIF(p_config.gradient_template_id,0)")
	legacyAlias := strings.Index(gradientExpr, "NULLIF(p.customer_product_alias_gradient_template_id,0)")
	if firstAliasConfig < 0 {
		t.Fatalf("customer alias product config template must be part of effective gradient expression: %s", gradientExpr)
	}
	if firstProductConfig < 0 || firstAliasConfig > firstProductConfig {
		t.Fatalf("customer alias product config template must override product archive config template: %s", gradientExpr)
	}
	if legacyAlias < 0 || firstProductConfig > legacyAlias {
		t.Fatalf("legacy customer alias direct gradient must only fallback after product config template: %s", gradientExpr)
	}
	for _, want := range []string{
		"LEFT JOIN %[1]s.product_config_templates alias_config",
		"LEFT JOIN %[1]s.product_unit_templates alias_legacy_unit",
		"NULLIF(alias_config.inventory_unit,'')",
		"NULLIF(p_config.inventory_unit,'')",
		"NULLIF(alias_legacy_unit.inventory_unit,'')",
		"NULLIF(alias_config.quote_unit,'')",
		"NULLIF(p_config.quote_unit,'')",
		"NULLIF(alias_legacy_unit.quote_unit,'')",
		"NULLIF(alias_config.order_unit,'')",
		"NULLIF(p_config.order_unit,'')",
		"NULLIF(alias_legacy_unit.order_unit,'')",
		"NULLIF(alias_config.unit_conversion_json::text,'{}')",
		"NULLIF(p_config.unit_conversion_json::text,'{}')",
		"NULLIF(alias_legacy_unit.unit_conversion_json::text,'{}')",
		"alias_config.integer_unit",
		"p_config.integer_unit",
		"alias_legacy_unit.integer_unit",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer alias product config template must drive units before legacy fallback; missing %q", want)
		}
	}
}

func TestLoadProductInputsUsesClassificationConfigTemplatesBeforeLegacyFallback(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	gradientExpr := effectiveGradientTemplateExpr(t, src)
	for _, want := range []string{
		"LEFT JOIN %[1]s.product_config_templates classification_category_config",
		"LEFT JOIN %[1]s.product_config_templates classification_template_config",
		"NULLIF(classification_category_config.gradient_template_id,0)",
		"NULLIF(classification_template_config.gradient_template_id,0)",
		"NULLIF(classification_category_config.inventory_unit,'')",
		"NULLIF(classification_template_config.inventory_unit,'')",
		"NULLIF(classification_category_config.price_list_rule_json::text,'{}')",
		"NULLIF(classification_template_config.price_list_rule_json::text,'{}')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("classification config template inheritance missing marker %q", want)
		}
	}
	productConfig := strings.Index(gradientExpr, "NULLIF(p_config.gradient_template_id,0)")
	categoryConfig := strings.Index(gradientExpr, "NULLIF(classification_category_config.gradient_template_id,0)")
	templateConfig := strings.Index(gradientExpr, "NULLIF(classification_template_config.gradient_template_id,0)")
	legacyAlias := strings.Index(gradientExpr, "NULLIF(p.customer_product_alias_gradient_template_id,0)")
	if productConfig < 0 || categoryConfig < 0 || templateConfig < 0 || legacyAlias < 0 {
		t.Fatalf("effective gradient expression missing expected source: %s", gradientExpr)
	}
	if !(productConfig < categoryConfig && categoryConfig < templateConfig && templateConfig < legacyAlias) {
		t.Fatalf("classification config template precedence must be product > category > template > legacy alias: %s", gradientExpr)
	}
}

func TestLoadProductInputsReadsCurrentClassificationAssignments(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"product_classification_assignments product_class",
		"customer_product_alias_classification_assignments alias_class",
		"product_classification_templates product_ct",
		"product_classification_templates alias_ct",
		"product_classification_template_categories product_cc",
		"product_classification_template_categories alias_cc",
		"p.current_classification_template_id",
		"&input.ClassificationTemplateID",
		"&input.ClassificationCategoryName",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must load current classification assignments for price list candidates; missing %q", want)
		}
	}
}

func TestLoadProductInputsReadsComposablePriceRulesAndBomUnitCosts(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"bom_unit_cost",
		"unit_per_box",
		"unit_per_bag",
		"g_per_bag",
		"price_list_rule_json",
		"&input.BomCostPerUnit",
		"&input.PriceListRuleJSON",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must load composable price rules and BOM unit costs; missing %q", want)
		}
	}
}

func TestLoadProductInputsReadsOperationTemplateCosts(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"operation_template_steps",
		"operation_unit_cost",
		"effective_operation_template_id",
		"&input.OperationCostPerUnit",
		"&input.OperationCostPerKg",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must load operation template cost into product inputs; missing %q", want)
		}
	}
}

func TestLoadProductInputsUsesProductionBomOutputProductFallback(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"output_bom",
		"pb.output_product_id=p.id",
		"output_bom.bom_version_id",
		"NULLIF(output_bom.bom_version_id,0)",
		"NULLIF(output_bom.bom_id,0)",
		"production_bom_output",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must treat production BOM output product as an effective BOM source; missing %q", want)
		}
	}
}

func TestPricingRuleTrialDetailsUseProductionBomOutputProductFallback(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	fnStart := strings.Index(src, "func (r Repository) LoadPricingRuleTrialBaseCostDetails")
	if fnStart < 0 {
		t.Fatal("LoadPricingRuleTrialBaseCostDetails not found")
	}
	fnEnd := strings.Index(src[fnStart:], "func (r Repository) loadProductInputs")
	if fnEnd < 0 {
		t.Fatal("loadProductInputs not found after LoadPricingRuleTrialBaseCostDetails")
	}
	fn := src[fnStart : fnStart+fnEnd]
	for _, want := range []string{
		"production_boms",
		"output_product_id=$2",
		"production_bom_versions",
		"production_bom_version_items",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("pricing rule trial BOM details must fall back to production BOM output product; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"product_bom_sources",
		"product_bom_items",
		"source_product_id",
		"inherit_current",
		"inherit_version",
	} {
		if strings.Contains(fn, forbidden) {
			t.Fatalf("pricing rule trial BOM details must not reserve product-bound BOM fallback; found %q", forbidden)
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
		"effective_bom_items",
		"bi.product_id = p.id",
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
		`whereClause := "publication_purpose=$1 AND list_type=$2 AND owner_type=$3 AND owner_key=$4"`,
		`orderClause := "published_at DESC, id DESC"`,
		"COALESCE(product_type_category_id,0)=0 AND list_type=$5",
		"WHERE %s AND status='published'",
		"ORDER BY %s",
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
		"DROP INDEX IF EXISTS %[1]s.bean_list_publications_one_published_owner_idx",
		"bean_list_publications_owner_status_idx",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean list publication schema must support owned locked snapshots; missing %q", want)
		}
	}
	if strings.Contains(src, "CREATE UNIQUE INDEX IF NOT EXISTS bean_list_publications_one_published_owner_idx") {
		t.Fatalf("bean list publication schema must not enforce one published snapshot per owner; withdraw is manual only")
	}
}

func TestBeanListPublicationSchemaSupportsProductPriceListGeneralization(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"product_type_category_id BIGINT NOT NULL DEFAULT 0",
		"product_type_name TEXT NOT NULL DEFAULT ''",
		"WHEN list_type IN ('commercial','retail') THEN '熟豆'",
		"WHEN list_type='green' THEN '生豆'",
		"WHEN list_type='drip' THEN '挂耳'",
		"bean_list_publications_product_type_owner_status_idx",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean list publication schema must support product price list generalization; missing %q", want)
		}
	}
}

func TestBeanListPublicationRepositoryQueriesByProductType(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"product_type_category_id",
		"product_type_name",
		"query.ProductTypeCategoryID",
		"cmd.ProductTypeCategoryID",
		"cmd.ProductTypeName",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean list publication repository must preserve product price list fields; missing %q", want)
		}
	}
	if !strings.Contains(src, "COALESCE(product_type_category_id,0)=$2 OR (COALESCE(product_type_category_id,0)=0 AND list_type=$5)") {
		t.Fatalf("ListBeanListPublications must allow product_type_category_id lookup while legacy list_type remains available")
	}
}

func TestPublishBeanListDoesNotWithdrawExistingPublishedSnapshots(t *testing.T) {
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
		"price_source_publication_id",
		"style_source_publication_id",
		"source_version_no",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PublishBeanList must lock customer snapshots independently; missing %q", want)
		}
	}
	if strings.Contains(body, "SET status='withdrawn'") || strings.Contains(body, "UPDATE %s.bean_list_publications") {
		t.Fatalf("PublishBeanList must not withdraw any existing published bean list; withdraw must stay a manual action")
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

func TestDripPriceTemplateSchemaIsLegacyOnlyAndDoesNotSeedDefaultTemplate(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"drip_price_templates",
		"drip_price_template_tiers",
		"product_kind TEXT NOT NULL DEFAULT 'roasted_bean'",
		"price_basis TEXT NOT NULL DEFAULT 'weight'",
		"sales_unit TEXT NOT NULL DEFAULT ''",
		"unit_bag_count INT NOT NULL DEFAULT 0",
		"price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing schema must keep legacy drip snapshot structure; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"seedDefaultDripPriceTemplate",
		"默认挂耳供应价",
		"10000袋",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("costing schema must not seed default drip template; found %q", forbidden)
		}
	}
}

func TestBeanListProductScopeAllowsPR440CustomerPriceRowsForPublicProducts(t *testing.T) {
	content := map[string]any{
		"groups": []any{
			map[string]any{
				"items": []any{
					map[string]any{"productId": float64(540), "name": "PR-440 商品档案"},
				},
			},
		},
		"price_rows": []any{
			map[string]any{
				"product_id":                  float64(540),
				"final_unit_price":            float64(88),
				"price_unit":                  "kg",
				"customer_reference_snapshot": map[string]any{"customer_id": float64(170), "customer_display_name": "客户展示名"},
				"pricing_rule_version":        "PR440/v1",
				"tier_template_source":        "product",
				"pricing_rule_source":         "product",
				"cost_source_snapshot":        map[string]any{"material_id": float64(46)},
				"group_snapshot":              map[string]any{"group_id": float64(1), "group_item_id": float64(2)},
				"manual_adjusted":             false,
				"inventory_conversion_json":   map[string]any{"kg": float64(1)},
				"inventory_unit":              "kg",
				"tier_template_id":            float64(1),
				"pricing_rule_id":             float64(1),
				"original_final_unit_price":   float64(88),
			},
		},
	}

	if !beanListContentHasPR440FlatRowsForProducts(content, []int64{540}) {
		t.Fatalf("PR-440 customer price list should allow public product archive rows when flat price rows freeze product pricing metadata")
	}

	missing := map[string]any{
		"groups":     []any{map[string]any{"items": []any{map[string]any{"productId": float64(541)}}}},
		"price_rows": []any{map[string]any{"product_id": float64(540)}},
	}
	if beanListContentHasPR440FlatRowsForProducts(missing, []int64{541}) {
		t.Fatalf("public product should still be rejected when the customer price list lacks a matching PR-440 flat row")
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
		"publication_purpose",
		"classification_template_id",
		"classification_category_id",
		"VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft'",
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
