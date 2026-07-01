package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev452PricingRuleTrialContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-452-PRICING-RULE-TRIAL",
			"DEV-452-PRICING-RULE-TRIAL-API",
			"DEV-452-PRICING-RULE-TRIAL-UI",
			"DEV-452-PRICING-RULE-TRIAL-DOCS",
			"REV-452-PRICING-RULE-TRIAL",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"type PricingRuleTrialCommand struct",
			"type PricingRuleTrialResult struct",
			"func (s *Service) PricingRuleTrial",
			"LoadProductPricingRule",
			"试算毛利率低于最低毛利",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"func (r Repository) LoadProductPricingRule",
			"product_pricing_rules",
			"calculation_json",
			"formula_version",
		},
		filepath.Join("internal", "interfaces", "http", "costing", "costing_api.go"): {
			"/api/costing/pricing-rule-trial",
			"PricingRuleTrialCommand",
			"PricingRuleTrial",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildPricingRuleTrialPayload",
			"pricing_rule_id",
			"other_costs",
			"expected_loss_rate",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"价格计算模板试算",
			"openPricingRuleTrial",
			"pricingRuleTrialDrawerOpen",
			"/api/costing/pricing-rule-trial",
			"试算商品",
			"公式步骤",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-452-PRICING-RULE-TRIAL",
			"价格计算模板试算",
			"不保存到模板、商品价格表、发布快照或订单",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-452-PRICING-RULE-TRIAL",
			"模板行显示 `试算`",
			"试算结果不回写商品价格表",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"价格计算模板试算",
			"选择商品",
			"自动试算",
			"只读结果",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-pricing-rule-trial.md"): {
			"PR-452",
			"商品价格管理模板试算",
			"POST /api/costing/pricing-rule-trial",
			"浏览器验收",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-452 marker %q", rel, want)
			}
		}
	}
}

func TestDev454PricingRuleTrialExcelParityContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-454-PRICING-RULE-TRIAL-EXCEL-PARITY",
			"DEV-454-EXCEL-PARITY-ANALYSIS",
			"DEV-454-TRIAL-FORMULA-PARITY",
			"DEV-454-TRIAL-FORMULA-NODES",
			"REV-454-PRICING-RULE-TRIAL-EXCEL-PARITY",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"PostMarkupCosts",
			"supplier_tier_markup",
			"price_after_markup",
			"post_markup_cost_total",
			"加价附加成本",
		},
		filepath.Join("internal", "application", "costing", "service_test.go"): {
			"TestPricingRuleTrialMatchesExcelSupplierPriceSamples",
			"测试用 1kg-2磅",
			"单品：孟连红果厌氧慢速日晒 24kg-48磅",
			"116.70915065789475",
			"177.11505625",
		},
		filepath.Join("internal", "interfaces", "http", "costing", "costing_api_test.go"): {
			"PostMarkupCosts",
			"PriceAfterMarkup",
			"PostMarkupCostTotal",
			"post_markup_cost_total",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"加价后价格",
			"pricingRuleTrialQuoteUnitOptions",
			"schedulePricingRuleTrial",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-454-PRICING-RULE-TRIAL-EXCEL-PARITY",
			"物料成本",
			"生产项目",
			"供应售价",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-454 商品价格管理试算 Excel 对账",
			"2 个产品 × 2 个供应售价档位",
			"生产项目",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"Excel 供应售价对账",
			"加价前生产项目成本",
			"前端试算抽屉不再提供售价后附加成本录入口",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-pricing-rule-trial-excel-parity.md"): {
			"PR-453",
			"刘豪-成本核算3",
			"测试用 1kg-2磅",
			"单品：孟连红果厌氧慢速日晒 24kg-48磅",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-453 marker %q", rel, want)
			}
		}
	}
}

func TestDev456PricingRuleTrialPr439UnitContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-456-PRICING-RULE-TRIAL-PR439-UNIT",
			"DEV-456-TRIAL-AUTO-UNIT-UI",
			"DEV-456-TRIAL-PUBLISHED-SNAPSHOT-FALLBACK",
			"REV-456-PRICING-RULE-TRIAL-PR439-UNIT",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"PricingRuleTrialBaseCostDetail",
			"base_cost_details",
			"该商品暂无可试算的 BOM/工序成本",
		},
		filepath.Join("internal", "application", "costing", "service_test.go"): {
			"TestPricingRuleTrialDoesNotInferCostFromPublishedPriceSnapshotWhenBomCostMissing",
			"PR439-20260606182321 熟豆下单商品",
			"must not infer from snapshot",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildPricingRuleTrialPayload",
			"other_costs",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"pricingRuleTrialQuoteUnitOptions",
			"schedulePricingRuleTrial",
			"activeProductUnitDefinitions",
			"试算中...",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-456-PRICING-RULE-TRIAL-PR439-UNIT",
			"全局单位字典",
			"当前缺 BOM/工序成本时不再反推",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-456-PRICING-RULE-TRIAL-PR439-UNIT",
			"不显示 `重新试算`",
			"不显示 `售价后附加成本`",
			"不再反推 `88.5/kg`",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-456-PRICING-RULE-TRIAL-PR439-UNIT",
			"全局单位字典",
			"自动试算",
			"不再按已发布售价反推成本",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-pricing-rule-trial-pr439-unit.md"): {
			"PR-456-PRICING-RULE-TRIAL-PR439-UNIT",
			"PR-460",
			"不再反推",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-455 marker %q", rel, want)
			}
		}
	}

	for rel, forbidden := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"重新试算",
			"售价后附加成本",
			"post_markup_cost_rows",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"post_markup_costs",
			"postMarkupCosts",
			"post_markup_cost_rows",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, value := range forbidden {
			if strings.Contains(src, value) {
				t.Fatalf("%s should not expose PR-455 removed marker %q", rel, value)
			}
		}
	}
}

func TestDev457PricingRuleTrialFormulaExpressionContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-457-PRICING-RULE-TRIAL-FORMULA-EXPRESSION",
			"DEV-457-TRIAL-FORMULA-API",
			"DEV-457-TRIAL-FORMULA-UI",
			"DEV-457-DOCS-ACCEPTANCE",
			"REV-457-PRICING-RULE-TRIAL-FORMULA-EXPRESSION",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"FormulaExpression",
			"formula_expression_lines",
			"pricingRuleTrialFormulaExpression",
			"最终售价 =",
		},
		filepath.Join("internal", "application", "costing", "service_test.go"): {
			"FormulaExpression",
			"FormulaExpressionLines",
			"最终售价 = 88.3/kg",
			"(BOM+工序成本 60/kg + 其他成本 2.5/kg)",
		},
		filepath.Join("internal", "interfaces", "http", "costing", "costing_api_test.go"): {
			"FormulaExpression",
			"FormulaExpressionLines",
			"formula_expression",
			"formula_expression_lines",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"计算公式",
			"pricing-rule-trial-formula",
			"formula_expression",
			"formula_expression_lines",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.test.js"): {
			"计算公式",
			"formula_expression_lines",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-457-PRICING-RULE-TRIAL-FORMULA-EXPRESSION",
			"计算公式",
			"最终售价串起来",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-457-PRICING-RULE-TRIAL-FORMULA-EXPRESSION",
			"`计算公式`",
			"逐节点公式行",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-457-PRICING-RULE-TRIAL-FORMULA-EXPRESSION",
			"计算公式",
			"逐节点公式行",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-pricing-rule-trial-formula-expression.md"): {
			"PR-457-PRICING-RULE-TRIAL-FORMULA-EXPRESSION",
			"计算公式",
			"逐节点公式行",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-457 marker %q", rel, want)
			}
		}
	}
}

func TestDev460PricingRuleTrialWaterfallBomDetailContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-460-PRICING-RULE-TRIAL-WATERFALL-BOM-DETAIL",
			"DEV-460-TRIAL-WATERFALL-API",
			"DEV-460-TRIAL-BOM-OPERATION-DETAILS",
			"DEV-460-TRIAL-WATERFALL-UI",
			"DEV-460-DOCS-ACCEPTANCE",
			"REV-460-PRICING-RULE-TRIAL-WATERFALL-BOM-DETAIL",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"PricingRuleTrialBaseCostDetail",
			"BaseCostDetails",
			"YieldLossAmount",
			"ProfitMarkupAmount",
			"TaxInPriceAmount",
			"RoundingAdjustment",
			"LoadPricingRuleTrialBaseCostDetails",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"LoadPricingRuleTrialBaseCostDetails",
			"production_bom_version_items",
			"operation_template_steps",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"BOM+工序成本折算明细",
			"物料成本明细",
			"工序成本明细",
			"BOM组成",
			"原料损耗",
			"损耗后用量",
			"折算成本",
			"损耗增加",
			"加价增加",
			"tax_in_price_amount",
			"pricing-rule-trial-waterfall",
			"pricing-rule-trial-operator",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-460-PRICING-RULE-TRIAL-WATERFALL-BOM-DETAIL",
			"价格瀑布",
			"BOM+工序成本折算明细",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-460-PRICING-RULE-TRIAL-WATERFALL-BOM-DETAIL",
			"不再按发布售价快照反推",
			"BOM+工序成本折算明细",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-460-PRICING-RULE-TRIAL-WATERFALL-BOM-DETAIL",
			"价格瀑布",
			"BOM+工序成本折算明细",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-pricing-rule-trial-waterfall-bom-detail.md"): {
			"PR-460-PRICING-RULE-TRIAL-WATERFALL-BOM-DETAIL",
			"BOM+工序成本明细",
			"不反推",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
	}

	trialDrawer := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	trialDrawer = trialDrawer[strings.Index(trialDrawer, `pricingRuleTrialDrawerOpen`):strings.Index(trialDrawer, `customerAliasCreateDrawerOpen`)]
	for _, forbidden := range []string{"状态：", "发布售价快照反推"} {
		if strings.Contains(trialDrawer, forbidden) {
			t.Fatalf("pricing rule trial drawer should not expose %q", forbidden)
		}
	}
}

func TestDev462PricingRuleTrialOutputBomOperationSelectionContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-462-PRICING-RULE-TRIAL-OUTPUT-BOM-OPERATION-SELECT",
			"DEV-462-TRIAL-OUTPUT-BOM-OPTIONS",
			"DEV-462-TRIAL-NO-PRODUCT-BOM-FALLBACK",
			"DEV-462-TRIAL-OPERATION-OPTIONS",
			"DEV-462-TRIAL-SELECTION-UI",
			"DEV-462-DOCS-ACCEPTANCE",
			"REV-462-PRICING-RULE-TRIAL-OUTPUT-BOM-OPERATION-SELECT",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"BomVersionOptions",
			"ProcessRouteOptions",
			"OperationTemplateOptions",
			"pricingRuleTrialApplyProductionSelection",
			"pricingRuleTrialDefaultBomVersionOption",
			"production BOM version not found for product",
			"process route not found",
			"operation template not found",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"LoadPricingRuleTrialProductionOptions",
			"pricing_rule_trial_selected_products",
			"pricing_rule_trial_bom_versions",
			"pb.output_product_id=selected.product_id",
			"process_routes",
			"process_route_operations",
			"operation_templates",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"bom_version_id",
			"process_route_id",
			"operation_template_id",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"试算BOM版本",
			"工艺路线",
			"pricingRuleTrialBomVersionOptions",
			"pricingRuleTrialProcessRouteOptions",
			"pricingRuleTrialForm.bom_version_id",
			"pricingRuleTrialForm.process_route_id",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-462-PRICING-RULE-TRIAL-OUTPUT-BOM-OPERATION-SELECT",
			"production_boms.output_product_id=product_id",
			"product_bom_sources",
			"product_bom_items",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-462-PRICING-RULE-TRIAL-OUTPUT-BOM-OPERATION-SELECT",
			"试算BOM版本",
			"PR439-20260606182321 工厂量单商品",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-462-PRICING-RULE-TRIAL-OUTPUT-BOM-OPERATION-SELECT",
			"试算BOM版本",
			"工艺路线",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-pricing-rule-trial-output-bom-operation-select.md"): {
			"PR-462-PRICING-RULE-TRIAL-OUTPUT-BOM-OPERATION-SELECT",
			"BOM-000539",
			"不读商品绑定 BOM",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", rel, want)
			}
		}
	}

	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go")))
	fnStart := strings.Index(src, "func (r Repository) LoadPricingRuleTrialBaseCostDetails")
	if fnStart < 0 {
		t.Fatal("LoadPricingRuleTrialBaseCostDetails not found")
	}
	fnEnd := strings.Index(src[fnStart:], "func (r Repository) loadProductInputs")
	if fnEnd < 0 {
		t.Fatal("loadProductInputs not found after LoadPricingRuleTrialBaseCostDetails")
	}
	fn := src[fnStart : fnStart+fnEnd]
	for _, forbidden := range []string{
		"product_bom_sources",
		"product_bom_items",
		"inherit_current",
		"inherit_version",
	} {
		if strings.Contains(fn, forbidden) {
			t.Fatalf("pricing trial details must not keep product-bound BOM fallback %q", forbidden)
		}
	}
}
