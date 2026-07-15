package costing

import (
	"math"
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

func TestPricingRuleTrialProductionCostUsesProductDefaultBomBeforeOutputFallback(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"product_production_configs ppc",
		"product_production_bom_bindings pbb",
		"COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id",
		"pb.output_product_id=selected.product_id",
		"LoadPricingRuleTrialProductionOptions",
		"pricing_rule_trial_bom_versions",
		"is_default",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("pricing trial production cost must use product default BOM before output fallback; missing %q", want)
		}
	}
}

func TestPricingRuleTrialProductionCostFallsBackToParentProductForDerivedSku(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"pricing_rule_trial_selected_products",
		"p.parent_product_id",
		"source_priority",
		"ppc.product_id=selected.product_id",
		"pbb.product_id=selected.product_id",
		"pb.output_product_id=selected.product_id",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("pricing trial production cost must try derived SKU BOM first and parent product BOM second; missing %q", want)
		}
	}
}

func TestLoadProductInputsUsesDerivedSkuNetContentForTrialUnitConversion(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"derived_sku_unit_factor",
		"p.net_content_qty",
		"p.net_content_unit",
		"/ 1000.0",
		"jsonb_build_object(p.derived_sales_unit",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing trial inputs must convert derived SKU sales unit from net content; missing %q", want)
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

func TestProductSalesUnitResolversPreferProductDirectUnitTemplateBeforeLegacyTemplateChain(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"LEFT JOIN %[1]s.product_unit_templates product_unit_template ON product_unit_template.id = p.unit_template_id AND product_unit_template.active = true",
		"product_unit_template_default_spec",
		"jsonb_array_elements(COALESCE(product_unit_template.sales_specs_json, '[]'::jsonb)) WITH ORDINALITY",
		"NULLIF(spec.row->>'spec_name','') AS sales_unit",
		"NULLIF(product_unit_template_default_spec.sales_unit,'')",
		"NULLIF(product_unit_template_default_spec.unit_conversion_json,'{}')",
		"NULLIF(product_unit_template.inventory_unit,'')",
		"NULLIF(product_unit_template.quote_unit,'')",
		"NULLIF(product_unit_template.unit_conversion_json::text,'{}')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing sales-unit resolver must read product direct unit template; missing %q", want)
		}
	}
	if strings.Contains(src, "COALESCE(NULLIF(spec.row->>'sales_unit',''), NULLIF(spec.row->>'spec_name','')) AS sales_unit") {
		t.Fatalf("sales spec default unit must use spec_name, not legacy generic sales_unit")
	}
	overrideIdx := strings.Index(src, "NULLIF(p.unit_rule_override_json->>'inventory_unit','')")
	templateIdx := strings.Index(src, "NULLIF(product_unit_template.inventory_unit,'')")
	legacyConfigIdx := strings.Index(src, "NULLIF(pct.inventory_unit,'')")
	if overrideIdx < 0 || templateIdx < 0 || legacyConfigIdx < 0 || !(overrideIdx < templateIdx && templateIdx < legacyConfigIdx) {
		t.Fatalf("product unit resolution priority must be product override -> direct unit template -> legacy config/template; indexes override=%d template=%d legacy=%d", overrideIdx, templateIdx, legacyConfigIdx)
	}
	loadProductInputsIdx := strings.Index(src, "func (r Repository) loadProductInputs")
	if loadProductInputsIdx < 0 {
		t.Fatalf("missing loadProductInputs")
	}
	loadProductInputsSrc := src[loadProductInputsIdx:]
	for _, want := range []string{
		"NULLIF(p.unit_rule_override_json->>'inventory_unit',''),\n\t\t\t           NULLIF(product_unit_template.inventory_unit,''),\n\t\t\t           NULLIF(p_config.inventory_unit,'')",
		"NULLIF(p.unit_rule_override_json->>'quote_unit',''),\n\t\t\t           NULLIF(product_unit_template_default_spec.sales_unit,''),\n\t\t\t           NULLIF(product_unit_template.quote_unit,''),",
		"NULLIF(p.unit_rule_override_json->>'unit_conversion_json',''),\n\t\t\t           NULLIF(p.unit_rule_override_json->>'conversion_json',''),\n\t\t\t           NULLIF(product_unit_template_default_spec.unit_conversion_json,'{}'),\n\t\t\t           NULLIF(product_unit_template.unit_conversion_json::text,'{}'),\n\t\t           NULLIF(p_config.unit_conversion_json::text,'{}')",
		"COALESCE(NULLIF(NULLIF(p.sku_name,''),'默认规格'), NULLIF(product_unit_template_default_spec.spec_name,''), '默认规格')",
		"NULLIF(product_unit_template_default_spec.unit_conversion_json,'{}')",
	} {
		if !strings.Contains(loadProductInputsSrc, want) {
			t.Fatalf("loadProductInputs must prefer product override and direct unit template before legacy config; missing %q", want)
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

func TestResolveProductSalesUnitRuleUsesProductMasterAndLegacyFallbacks(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) ResolveProductSalesUnitRule")
	if start < 0 {
		t.Fatalf("missing ResolveProductSalesUnitRule")
	}
	end := strings.Index(src[start:], "func productSalesUnitConversionMap")
	if end < 0 {
		t.Fatalf("missing ResolveProductSalesUnitRule end marker")
	}
	fn := src[start : start+end]
	for _, want := range []string{
		"NULLIF(p.unit_rule_override_json->>'inventory_unit','')",
		"NULLIF(p.unit_rule_override_json->>'default_sales_unit','')",
		"NULLIF(p.unit_rule_override_json->>'unit_conversion_json','')",
		"LEFT JOIN %[1]s.product_config_templates pct",
		"LEFT JOIN %[1]s.product_unit_templates put",
		"LEFT JOIN %[1]s.product_categories pc",
		"LEFT JOIN %[1]s.product_categories parent_pc",
		"NULLIF(pct.inventory_unit,'')",
		"NULLIF(put.inventory_unit,'')",
		"NULLIF(pc.inventory_unit,'')",
		"NULLIF(parent_pc.inventory_unit,'')",
		"NULLIF(pct.unit_conversion_json::text,'{}')",
		"NULLIF(put.unit_conversion_json::text,'{}')",
		"NULLIF(pc.unit_conversion_json::text,'{}')",
		"NULLIF(parent_pc.unit_conversion_json::text,'{}')",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("ResolveProductSalesUnitRule must use product unit master data and legacy fallbacks; missing %q", want)
		}
	}
	if !strings.Contains(fn, "productSalesUnitConversionMap(conversionJSON, inventoryUnit)") {
		t.Fatalf("ResolveProductSalesUnitRule must parse legacy flat conversion JSON against the resolved inventory unit")
	}
}

func TestResolveProductSalesUnitRuleUsesDerivedSkuNetContentConversion(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) ResolveProductSalesUnitRule")
	if start < 0 {
		t.Fatalf("missing ResolveProductSalesUnitRule")
	}
	end := strings.Index(src[start:], "func (r Repository) ResolveCustomerProductSalesUnitRule")
	if end < 0 {
		t.Fatalf("missing ResolveProductSalesUnitRule end marker")
	}
	fn := src[start : start+end]
	for _, want := range []string{
		"derived_sku_unit_factor",
		"p.net_content_qty",
		"p.net_content_unit",
		"/ 1000.0",
		"jsonb_build_object(p.derived_sales_unit",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("derived SKU publication snapshots must convert net content to the parent inventory unit; missing %q", want)
		}
	}
	if strings.Contains(fn, "conversion[derivedSalesUnit] = map[string]float64{inventoryUnit: 1}") {
		t.Fatalf("derived SKU publication snapshots must not hard-code one parent inventory unit per sales unit")
	}
}

func TestProductSalesUnitConversionMapAcceptsLegacyFlatConversions(t *testing.T) {
	got := productSalesUnitConversionMap(`{"盒":0.2,"袋":"0.25"}`, "kg")
	if got["盒"]["kg"] != 0.2 {
		t.Fatalf("盒 conversion = %#v, want 0.2 kg", got["盒"])
	}
	if got["袋"]["kg"] != 0.25 {
		t.Fatalf("袋 conversion = %#v, want 0.25 kg", got["袋"])
	}

	nested := productSalesUnitConversionMap(`{"箱":{"盒":24}}`, "kg")
	if nested["箱"]["盒"] != 24 {
		t.Fatalf("nested conversion = %#v, want existing nested conversion preserved", nested["箱"])
	}
}

func TestProductSalesUnitConversionMapAddsStandardWeightConversions(t *testing.T) {
	got := productSalesUnitConversionMap(`{}`, "kg")
	if math.Abs(got["lb"]["kg"]-0.45359237) > 0.00000001 {
		t.Fatalf("lb conversion = %#v, want 0.45359237 kg", got["lb"])
	}
	if math.Abs(got["磅"]["kg"]-0.45359237) > 0.00000001 {
		t.Fatalf("磅 conversion = %#v, want 0.45359237 kg", got["磅"])
	}
	if got["kg"]["kg"] != 1 {
		t.Fatalf("kg conversion = %#v, want 1 kg", got["kg"])
	}

	toGrams := productSalesUnitConversionMap(`{}`, "g")
	if math.Abs(toGrams["kg"]["g"]-1000) > 0.00000001 {
		t.Fatalf("kg conversion to g = %#v, want 1000 g", toGrams["kg"])
	}
	if math.Abs(toGrams["lb"]["g"]-453.59237) > 0.00000001 {
		t.Fatalf("lb conversion to g = %#v, want 453.59237 g", toGrams["lb"])
	}

	custom := productSalesUnitConversionMap(`{"lb":{"kg":0.454}}`, "kg")
	if custom["lb"]["kg"] != 0.454 {
		t.Fatalf("explicit lb conversion = %#v, want explicit 0.454 kg", custom["lb"])
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
		"pricing_rule_trial_selected_products",
		"production_boms",
		"output_product_id=selected.product_id",
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

func TestPricingRuleTrialDetailsConvertGramBomItemsToKgCost(t *testing.T) {
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
		"WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct')='g'",
		"THEN COALESCE(bi.qty_per_unit,0) / 1000.0 * COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0)",
		"WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct')='kg'",
		"THEN COALESCE(bi.qty_per_unit,0) * COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0)",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("pricing rule trial BOM detail cost must convert generic g/kg quantities with kg material cost; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') IN ('unit_per_bag','unit_per_box','fixed_qty','unit','g','kg','length','area')",
	} {
		if strings.Contains(fn, forbidden) {
			t.Fatalf("generic g/kg BOM detail costs must not be priced as raw quantity without kg conversion; found %q", forbidden)
		}
	}
}

func TestPricingRuleTrialDetailsGrossRatioCostsByMaterialLossRate(t *testing.T) {
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
		"pbi.material_loss_rate",
		"COALESCE(bi.material_loss_rate,0)::float8",
		"&row.MaterialLossRate",
		"row.RecipeRatioPct = row.RatioPct",
		"row.EffectiveRatioPct = row.RatioPct / (1 - row.MaterialLossRate)",
		"row.RatioPct / (1 - row.MaterialLossRate)",
		"THEN COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0) * COALESCE(bi.ratio_pct,0) / 100.0 / (1 - LEAST(GREATEST(COALESCE(bi.material_loss_rate,0),0),0.9999))",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("pricing rule trial BOM detail material loss cost missing marker %q", want)
		}
	}
}

func TestPricingRuleTrialProductionOptionsExposeProcessRoutes(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	fnStart := strings.Index(src, "func (r Repository) LoadPricingRuleTrialProductionOptions")
	if fnStart < 0 {
		t.Fatal("LoadPricingRuleTrialProductionOptions not found")
	}
	fnEnd := strings.Index(src[fnStart:], "func (r Repository) LoadPricingRuleTrialBaseCostDetails")
	if fnEnd < 0 {
		t.Fatal("LoadPricingRuleTrialBaseCostDetails not found after LoadPricingRuleTrialProductionOptions")
	}
	fn := src[fnStart : fnStart+fnEnd]
	for _, want := range []string{
		"v.process_route_id",
		"process_route_name",
		"process_routes",
		"out.ProcessRoutes",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("pricing rule trial production options must expose current process routes; missing %q", want)
		}
	}
}

func TestPricingRuleTrialDetailsDoNotUseProcessRoutePlannedOperationCost(t *testing.T) {
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
	if !strings.Contains(fn, "input.ProcessRouteID") {
		t.Fatalf("pricing rule trial details must still respect selected process route boundary")
	}
	for _, forbidden := range []string{
		"planned_operation_cost",
		"工艺路线计划工序成本",
		"process_route:%d",
	} {
		if strings.Contains(fn, forbidden) {
			t.Fatalf("pricing rule trial details must not read route template operation cost; found %q", forbidden)
		}
	}
}

func TestPricingRuleTrialDetailsReadBomOperationCostSnapshots(t *testing.T) {
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
		"production_bom_version_operation_costs",
		"workstation_capacity_id",
		"operation_unit_cost",
		"bom_operation_snapshot",
		"per_inventory_unit",
		"标准工序成本",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("pricing trial details must read frozen BOM operation cost snapshots; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"standard_operation_cost",
		"operation_master",
		"标准工序成本来自工序列表",
	} {
		if strings.Contains(fn, forbidden) {
			t.Fatalf("pricing trial details must not read operation master standard cost; found %q", forbidden)
		}
	}
}

func TestPricingRuleTrialDetailsWarnWhenBomOperationCostSnapshotMissing(t *testing.T) {
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
		"bom_operation_snapshot_missing",
		"请先发布包含标准成本产能档快照的 BOM",
		`CapacitySelectionSource: "bom_operation_snapshot_missing"`,
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("pricing trial details must warn on missing BOM operation cost snapshots; missing %q", want)
		}
	}
}

func TestPricingRuleTrialRepositoryReadsFinanceDefaultTaxRate(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"LoadPricingRuleTrialDefaultTaxRate",
		"finance_settings",
		"taxpayer_type",
		"general_output_vat_rate",
		"small_scale_vat_rate",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("pricing rule trial repository must read finance default tax rate; missing %q", want)
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

func TestLoadProductInputsReadsChildSKUMetadataForPriceListRows(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"p.id AS sku_id",
		"COALESCE(p.parent_product_id,0) AS parent_product_id",
		"effective_parent_product_id",
		"COALESCE(p.sku_code,'') AS sku_code",
		"COALESCE(p.spec_label,'') AS spec_label",
		"COALESCE(NULLIF(p.net_content_qty,0), NULLIF(product_unit_template_default_spec.net_content_qty,0), 0)::float8 AS net_content_qty",
		"COALESCE(NULLIF(p.net_content_unit,''), NULLIF(product_unit_template_default_spec.net_content_unit,''), '') AS net_content_unit",
		"&input.SKUID",
		"&input.ParentProductID",
		"&input.SKUName",
		"&input.SpecLabel",
		"&input.NetContentQty",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing repository must load child SKU metadata for price list rows; missing %q", want)
		}
	}
}

func TestLoadProductInputsHideTemplateRemovedDerivedSKUsFromPriceListCandidates(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"COALESCE(p.auto_derived_sku,false)",
		"derived_spec_status",
		"template_removed",
		"COALESCE(NULLIF(p.derived_spec_status,''),'active')<>'template_removed'",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("costing price-list candidates must hide template-removed derived SKUs; missing %q", want)
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

func TestBeanListPublicationArchiveWritesStatusAndAudit(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	archiveStart := strings.Index(src, "func (r Repository) ArchiveBeanListPublications")
	unarchiveStart := strings.Index(src, "func (r Repository) UnarchiveBeanListPublications")
	if archiveStart < 0 || unarchiveStart <= archiveStart {
		t.Fatalf("archive and unarchive repository functions not found")
	}
	archiveBody := src[archiveStart:unarchiveStart]
	for _, want := range []string{
		"SET status='archived'",
		"archived_from_status",
		"jsonb_set",
		"WHERE id=ANY($1)",
		"status<>'archived'",
		`"archive"`,
		"postgresinfra.AuditInsertTx",
	} {
		if !strings.Contains(archiveBody, want) {
			t.Fatalf("ArchiveBeanListPublications must write archived status and audit log; missing %q", want)
		}
	}
	unarchiveEnd := strings.Index(src[unarchiveStart:], "func (r Repository) CreateRun")
	if unarchiveEnd < 0 {
		t.Fatalf("unarchive function end not found")
	}
	unarchiveBody := src[unarchiveStart : unarchiveStart+unarchiveEnd]
	for _, want := range []string{
		"archived_from_status",
		"restored_status",
		"SET status=s.restored_status",
		"config_json = b.config_json - 'archived_from_status'",
		"WHERE id=ANY($1)",
		"status='archived'",
		`"unarchive"`,
		"postgresinfra.StrPtr(row.newStatus)",
		"postgresinfra.AuditInsertTx",
	} {
		if !strings.Contains(unarchiveBody, want) {
			t.Fatalf("UnarchiveBeanListPublications must restore published status and audit log; missing %q", want)
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
