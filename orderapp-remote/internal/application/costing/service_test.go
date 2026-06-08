package costing

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"

	domain "orderapp/internal/domain/costing"
)

type fakeRepo struct {
	params              domain.Parameters
	inputs              []domain.ProductInput
	settings            []ParameterSetting
	customerInputs      []domain.ProductInput
	lastCustomerID      int64
	savedItems          []domain.ProductResult
	publishedID         int64
	savedDripTemplate   SaveDripPriceTemplateCommand
	deactivatedDripID   int64
	publishedBeanList   PublishBeanListCommand
	draftBeanList       PublishBeanListCommand
	beanListPublication *BeanListPublication
	beanListAsset       BeanListPublicationAsset
	savedBeanListAsset  BeanListPublicationAsset
	pricingRules        map[int64]ProductPricingRule
	costDetails         []PricingRuleTrialBaseCostDetail
}

func sliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (r *fakeRepo) LoadParameters(context.Context) (domain.Parameters, error) {
	if r.params.RoastYieldRate == 0 {
		return domain.DefaultParameters(), nil
	}
	return r.params, nil
}

func (r *fakeRepo) LoadProductInputs(context.Context, domain.Parameters) ([]domain.ProductInput, error) {
	return r.inputs, nil
}

func (r *fakeRepo) LoadProductInputsForCustomer(_ context.Context, _ domain.Parameters, customerID int64) ([]domain.ProductInput, error) {
	r.lastCustomerID = customerID
	return r.customerInputs, nil
}

func (r *fakeRepo) LoadProductPricingRule(_ context.Context, id int64) (ProductPricingRule, error) {
	if r.pricingRules != nil {
		if row, ok := r.pricingRules[id]; ok {
			return row, nil
		}
	}
	return ProductPricingRule{}, ErrProductPricingRuleNotFound
}

func (r *fakeRepo) LoadPricingRuleTrialBaseCostDetails(_ context.Context, _ domain.ProductInput) ([]PricingRuleTrialBaseCostDetail, error) {
	return r.costDetails, nil
}

func (r *fakeRepo) CreateRun(_ context.Context, actor string, items []domain.ProductResult) (*Run, error) {
	r.savedItems = items
	return &Run{ID: 42, Status: "draft", ProductCount: len(items), Items: items}, nil
}

func (r *fakeRepo) PublishRun(_ context.Context, actor string, runID int64) error {
	r.publishedID = runID
	return nil
}

func (r *fakeRepo) ListParameterSettings(context.Context) ([]ParameterSetting, error) {
	return r.settings, nil
}

func (r *fakeRepo) UpdateParameterSetting(context.Context, UpdateParameterCommand) (ParameterSetting, error) {
	return ParameterSetting{}, nil
}

func (r *fakeRepo) ListDripPriceTemplates(context.Context) ([]domain.DripPriceTemplate, error) {
	return nil, nil
}

func (r *fakeRepo) SaveDripPriceTemplate(_ context.Context, cmd SaveDripPriceTemplateCommand) (*domain.DripPriceTemplate, error) {
	r.savedDripTemplate = cmd
	includePackaging := true
	if cmd.IncludePackaging != nil {
		includePackaging = *cmd.IncludePackaging
	}
	return &domain.DripPriceTemplate{
		ID:               1,
		Name:             cmd.Name,
		Active:           cmd.Active == nil || *cmd.Active,
		BagGrams:         cmd.BagGrams,
		BoxBagCount:      cmd.BoxBagCount,
		IncludePackaging: includePackaging,
	}, nil
}

func (r *fakeRepo) DeactivateDripPriceTemplate(_ context.Context, cmd DeactivateDripPriceTemplateCommand) error {
	r.deactivatedDripID = cmd.ID
	return nil
}

func (r *fakeRepo) ListBeanListPublications(context.Context, BeanListPublicationQuery) ([]BeanListPublication, error) {
	return nil, nil
}

func (r *fakeRepo) PublishedBeanList(context.Context, BeanListPublicationQuery) (*BeanListPublication, error) {
	return nil, nil
}

func (r *fakeRepo) LoadBeanListPublication(context.Context, BeanListPublicationQuery, int64) (*BeanListPublication, error) {
	if r.beanListPublication != nil {
		return r.beanListPublication, nil
	}
	return nil, nil
}

func (r *fakeRepo) LoadBeanListPublicationAsset(context.Context, int64, string) (BeanListPublicationAsset, error) {
	if len(r.beanListAsset.Payload) > 0 {
		return r.beanListAsset, nil
	}
	return BeanListPublicationAsset{}, ErrBeanListPublicationNotFound
}

func (r *fakeRepo) SaveBeanListPublicationAsset(_ context.Context, asset BeanListPublicationAsset, _ string) (BeanListPublicationAsset, error) {
	r.savedBeanListAsset = asset
	r.beanListAsset = asset
	return asset, nil
}

func (r *fakeRepo) PublishBeanList(_ context.Context, cmd PublishBeanListCommand) (*BeanListPublication, error) {
	r.publishedBeanList = cmd
	return &BeanListPublication{ID: 1, ListType: cmd.ListType, Version: cmd.Version, Status: "published", OwnerType: cmd.OwnerType, OwnerKey: cmd.OwnerKey, PriceSourcePublicationID: cmd.PriceSourcePublicationID, StyleSourcePublicationID: cmd.StyleSourcePublicationID}, nil
}

func (r *fakeRepo) SaveBeanListDraft(_ context.Context, cmd PublishBeanListCommand) (*BeanListPublication, error) {
	r.draftBeanList = cmd
	return &BeanListPublication{ID: 2, ListType: cmd.ListType, Version: cmd.Version, Status: "draft", OwnerType: cmd.OwnerType, OwnerKey: cmd.OwnerKey, PriceSourcePublicationID: cmd.PriceSourcePublicationID, StyleSourcePublicationID: cmd.StyleSourcePublicationID}, nil
}

func (r *fakeRepo) WithdrawBeanList(context.Context, WithdrawBeanListCommand) error {
	return nil
}

func TestCalculateRejectsEmptyProducts(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if _, err := svc.Calculate(context.Background(), CalculateRequest{}); err == nil {
		t.Fatalf("expected products required error")
	}
}

func TestBeanListAllowsEmptyProductCatalog(t *testing.T) {
	resp, err := NewService(&fakeRepo{}).BeanList(context.Background(), BeanListQuery{})
	if err != nil {
		t.Fatalf("BeanList() error = %v, want empty response", err)
	}
	if resp == nil {
		t.Fatal("BeanList() response is nil")
	}
	if len(resp.Items) != 0 {
		t.Fatalf("items = %+v, want empty list", resp.Items)
	}
	if resp.Parameters.RoastYieldRate == 0 {
		t.Fatalf("parameters not populated: %+v", resp.Parameters)
	}
}

func TestBeanListPreservesCustomerAliasAndProductSnapshots(t *testing.T) {
	repo := &fakeRepo{customerInputs: []domain.ProductInput{{
		ProductID:                  10,
		ProductCode:                "K001",
		ProductName:                "工厂拼配",
		Name:                       "Karen 贴牌拼配",
		CustomerID:                 42,
		CustomerProductAliasID:     101,
		CustomerProductDisplayName: "Karen 贴牌拼配",
		CustomerItemCode:           "KA-001",
		DisplayCategoryName:        "Karen 批发",
		BomVersionID:               5,
		BomUsageMode:               "inherit_current",
		GreenBeanCostPerKg:         62,
		YieldRate:                  domain.DefaultParameters().RoastYieldRate,
	}}}

	resp, err := NewService(repo).BeanList(context.Background(), BeanListQuery{CustomerID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastCustomerID != 42 {
		t.Fatalf("customer id = %d, want 42", repo.lastCustomerID)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %+v", resp.Items)
	}
	item := resp.Items[0]
	if item.CustomerProductAliasID != 101 ||
		item.CustomerProductDisplayName != "Karen 贴牌拼配" ||
		item.ProductCode != "K001" ||
		item.ProductName != "工厂拼配" ||
		item.BomVersionID != 5 ||
		item.BomUsageMode != "inherit_current" {
		t.Fatalf("alias/product snapshots = %+v", item)
	}
}

func TestPricingRuleTrialUsesBomCostTemplateFormula(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:                549,
			Name:                     "PR452 试算商品",
			InventoryUnit:            "kg",
			QuoteUnit:                "kg",
			GreenBeanCostPerKg:       50,
			OperationCostPerKg:       10,
			YieldRate:                0.8,
			ExpectedLossRate:         0.2,
			BomVersionID:             3315,
			BomVersionNo:             "BOM-v1",
			BomUsageMode:             "product_production_config",
			BomStatus:                "active",
			OperationTemplateID:      7,
			ProductSubtypeName:       "商用拼配",
			ProductTypeName:          "咖啡豆",
			ProductCategoryID:        91,
			ProductSubtypeCategoryID: 92,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:1", Type: "material", TypeLabel: "物料", Name: "拼配熟豆原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 50, Amount: 50, Unit: "kg", Description: "物料成本 50/kg"},
			{Key: "operation:7:1", Type: "operation", TypeLabel: "工序", Name: "烘焙", ConsumeUnit: "per_kg", UnitCost: 10, Amount: 10, Unit: "kg", Description: "工序成本 10/kg"},
		},
		pricingRules: map[int64]ProductPricingRule{
			10: {
				ID:             10,
				Name:           "PR452 毛利含税",
				Code:           "PR452-GM",
				CostSourceMode: "bom_current_cost",
				MarginRate:     0.25,
				TaxRate:        0.06,
				RoundingMode:   "jiao",
				FormulaVersion: "v2",
				Active:         true,
				CalculationJSON: map[string]any{
					"yield_loss_mode":     "bom_or_product",
					"profit_method":       "gross_margin",
					"tax_mode":            "tax_included",
					"minimum_margin_rate": 0.18,
					"other_costs": map[string]any{
						"包装贴标": 2.5,
					},
				},
			},
		},
	}

	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 10,
		ProductID:     549,
		QuoteUnit:     "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.PricingRuleID != 10 || got.ProductID != 549 || got.BomVersionID != 3315 {
		t.Fatalf("trial identity = %+v", got)
	}
	if got.BaseCost != 60 || got.OtherCostTotal != 2.5 || got.CostAfterYield != 78.13 {
		t.Fatalf("trial costs = base %.2f other %.2f after yield %.2f", got.BaseCost, got.OtherCostTotal, got.CostAfterYield)
	}
	if got.BomCostTotal != 50 || got.OperationCostTotal != 10 || len(got.BaseCostDetails) != 2 {
		t.Fatalf("base details = bom %.2f operation %.2f rows %+v", got.BomCostTotal, got.OperationCostTotal, got.BaseCostDetails)
	}
	if got.CostBaseTotal != 62.5 || got.YieldLossAmount != 15.63 || got.ProfitMarkupAmount != 26.04 || got.TaxInPriceAmount != 6.25 || got.FinalBeforeRounding != 110.42 || got.RoundingAdjustment != -0.02 {
		t.Fatalf("waterfall = base %.2f loss %.2f profit %.2f taxInPrice %.2f finalBefore %.2f rounding %.2f", got.CostBaseTotal, got.YieldLossAmount, got.ProfitMarkupAmount, got.TaxInPriceAmount, got.FinalBeforeRounding, got.RoundingAdjustment)
	}
	if got.PreTaxPrice != 104.17 || got.TaxAmount != 6.25 || got.FinalUnitPrice != 110.4 {
		t.Fatalf("trial prices = preTax %.2f tax %.2f final %.2f", got.PreTaxPrice, got.TaxAmount, got.FinalUnitPrice)
	}
	waterfallTotal := got.CostBaseTotal + got.YieldLossAmount + got.ProfitMarkupAmount + got.TaxInPriceAmount + got.RoundingAdjustment
	if math.Abs(waterfallTotal-got.FinalUnitPrice) > 0.001 {
		t.Fatalf("waterfall total %.4f must equal final unit price %.4f", waterfallTotal, got.FinalUnitPrice)
	}
	if got.FormulaExpression == "" || !sliceContains(got.FormulaExpressionLines, "最终售价 = 110.4/kg") {
		t.Fatalf("formula expression = %q lines = %+v, want final price line", got.FormulaExpression, got.FormulaExpressionLines)
	}
	for _, want := range []string{"(BOM+工序成本 60/kg + 其他成本 2.5/kg)", "/ (1 - 损耗率 20%)", "/ (1 - 毛利率 25%)", "* (1 + 税率 6%)"} {
		if !strings.Contains(got.FormulaExpression, want) {
			t.Fatalf("formula expression = %q, want %q", got.FormulaExpression, want)
		}
	}
	for _, key := range []string{"bom_operation_cost", "other_cost_total", "expected_loss_rate", "profit_method", "tax_rate", "rounding_rule", "final_unit_price"} {
		if !pricingRuleTrialHasStep(got.Steps, key) {
			t.Fatalf("steps missing %q: %+v", key, got.Steps)
		}
	}
}

func TestPricingRuleTrialDoesNotInferCostFromPublishedPriceSnapshotWhenBomCostMissing(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:        538,
			Name:             "PR439-20260606182321 熟豆下单商品",
			InventoryUnit:    "kg",
			QuoteUnit:        "kg",
			YieldRate:        0.8,
			ExpectedLossRate: 0.2,
			BomStatus:        "missing",
			ProductPriceSnapshots: []domain.ProductPriceSnapshot{{
				SourcePriceRecordID: 1,
				Label:               "1kg+",
				MinQty:              1,
				FinalUnitPrice:      88.5,
				PriceUnit:           "kg",
				Currency:            "CNY",
				PriceGroupName:      "PR439 常规批发",
				InventoryUnit:       "kg",
			}},
		}},
		pricingRules: map[int64]ProductPricingRule{
			10: {
				ID:             10,
				Name:           "PR455 快照反推",
				CostSourceMode: "bom_current_cost",
				MarginRate:     0.2,
				TaxRate:        0.13,
				RoundingMode:   "cent",
				FormulaVersion: "v1",
				Active:         true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "bom_or_product",
					"profit_method":   "gross_margin",
					"tax_mode":        "tax_included",
				},
			},
		},
	}

	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 10,
		ProductID:     538,
		QuoteUnit:     "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.BaseCost != 0 || got.BomCostTotal != 0 || got.OperationCostTotal != 0 || got.CostAfterYield != 0 || got.PreTaxPrice != 0 || got.TaxAmount != 0 || got.FinalUnitPrice != 0 {
		t.Fatalf("trial must not infer from snapshot = base %.2f bom %.2f operation %.2f after yield %.2f preTax %.2f tax %.2f final %.2f", got.BaseCost, got.BomCostTotal, got.OperationCostTotal, got.CostAfterYield, got.PreTaxPrice, got.TaxAmount, got.FinalUnitPrice)
	}
	if strings.Contains(got.FormulaExpression, "发布售价快照反推") || strings.Contains(strings.Join(got.FormulaExpressionLines, "\n"), "发布售价快照反推") {
		t.Fatalf("formula expression should not mention published snapshot inference: %q lines=%+v", got.FormulaExpression, got.FormulaExpressionLines)
	}
	if pricingRuleTrialHasStep(got.Steps, "published_price_snapshot") {
		t.Fatalf("steps must not include published price snapshot source: %+v", got.Steps)
	}
	if !pricingRuleTrialWarningsContain(got.Warnings, "该商品暂无可试算的 BOM/工序成本") {
		t.Fatalf("warnings = %+v, want missing BOM warning", got.Warnings)
	}
	if strings.Contains(strings.Join(got.Warnings, "\n"), "反推") {
		t.Fatalf("warnings must not mention snapshot inference: %+v", got.Warnings)
	}
}

func TestPricingRuleTrialUsesBaseCostDetailsWhenProductInputSummaryMissing(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:        539,
			Name:             "PR439-20260606182321 工厂量单商品",
			InventoryUnit:    "kg",
			QuoteUnit:        "kg",
			YieldRate:        0.8,
			ExpectedLossRate: 0.2,
			BomStatus:        "active",
			BomVersionID:     5392,
			BomVersionNo:     "V002",
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:1001", Type: "material", TypeLabel: "物料", Name: "BOM-000539 原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 42, AmountPerKg: 42, Unit: "kg"},
			{Key: "operation:2001", Type: "operation", TypeLabel: "工序", Name: "工厂量单工序", ConsumeUnit: "per_kg", UnitCost: 8, AmountPerKg: 8, Unit: "kg"},
		},
		pricingRules: map[int64]ProductPricingRule{
			10: {
				ID:             10,
				Name:           "PR459 明细补成本",
				CostSourceMode: "bom_current_cost",
				MarginRate:     0.2,
				TaxRate:        0,
				RoundingMode:   "cent",
				FormulaVersion: "v1",
				Active:         true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "none",
					"profit_method":   "markup",
					"tax_mode":        "none",
				},
			},
		},
	}

	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 10,
		ProductID:     539,
		QuoteUnit:     "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.BaseCost != 50 || got.BomCostTotal != 42 || got.OperationCostTotal != 8 {
		t.Fatalf("trial must use BOM detail costs when summary is missing: base %.2f bom %.2f operation %.2f rows %+v", got.BaseCost, got.BomCostTotal, got.OperationCostTotal, got.BaseCostDetails)
	}
	if got.FinalUnitPrice != 60 {
		t.Fatalf("final price = %.2f, want 60", got.FinalUnitPrice)
	}
	if pricingRuleTrialWarningsContain(got.Warnings, "该商品暂无可试算的 BOM/工序成本") {
		t.Fatalf("warnings should not claim missing cost when details exist: %+v", got.Warnings)
	}
}

func TestPricingRuleTrialMatchesExcelSupplierPriceSamples(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{
			{
				ProductID:          45301,
				Name:               "测试用",
				InventoryUnit:      "kg",
				QuoteUnit:          "kg",
				GreenBeanCostPerKg: 67.5, // 物料成本!C4 / 生产项目!H3 = 54 / 0.8
				YieldRate:          1,
			},
			{
				ProductID:          45302,
				Name:               "单品：孟连红果厌氧慢速日晒",
				InventoryUnit:      "kg",
				QuoteUnit:          "kg",
				GreenBeanCostPerKg: 131.25, // 物料成本!C5 / 生产项目!H3 = 105 / 0.8
				YieldRate:          1,
			},
		},
		pricingRules: map[int64]ProductPricingRule{
			453: {
				ID:             453,
				Name:           "PR453 Excel 供应售价",
				CostSourceMode: "bom_current_cost",
				MarginRate:     0,
				TaxRate:        0,
				RoundingMode:   "none",
				FormulaVersion: "excel-202604-v3",
				Active:         true,
				CalculationJSON: map[string]any{
					"formula_mode":    "supplier_tier_markup",
					"yield_loss_mode": "none",
					"profit_method":   "markup",
					"tax_mode":        "none",
				},
			},
			454: {
				ID:             454,
				Name:           "PR453 Excel 供应售价自动口径",
				CostSourceMode: "bom_current_cost",
				MarginRate:     0,
				TaxRate:        0,
				RoundingMode:   "none",
				FormulaVersion: "excel-202604-v3",
				Active:         true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "none",
					"profit_method":   "markup",
					"tax_mode":        "none",
				},
			},
		},
	}
	cases := []struct {
		name         string
		productID    int64
		tierRate     float64
		preCosts     map[string]float64
		postCosts    map[string]float64
		wantFinal    float64
		wantCostBase float64
	}{
		{
			name:      "测试用 1kg-2磅",
			productID: 45301,
			tierRate:  0.5421052631578949,
			preCosts: map[string]float64{
				"电力":     1.875,
				"租金":     0.1375,
				"生产损耗":   0.5,
				"人力（智烘）": 3.75,
			},
			postCosts: map[string]float64{
				"包装":   1.7,
				"产品损耗": 0.06,
				"利润税额": 1.1996111842105266,
			},
			wantCostBase: 73.7625,
			wantFinal:    116.70915065789475,
		},
		{
			name:      "测试用 100kg-200磅",
			productID: 45301,
			tierRate:  0.045,
			preCosts: map[string]float64{
				"燃气":     1.025,
				"租金":     0.1375,
				"生产损耗":   0.5,
				"人力（布勒）": 1.5,
			},
			postCosts: map[string]float64{
				"包装":   1.7,
				"产品损耗": 0.06,
			},
			wantCostBase: 70.6625,
			wantFinal:    75.6023125,
		},
		{
			name:      "单品：孟连红果厌氧慢速日晒 1kg-2磅",
			productID: 45302,
			tierRate:  0.8131578947368422,
			preCosts: map[string]float64{
				"电力":     1.875,
				"租金":     0.1375,
				"生产损耗":   0.5,
				"人力（智烘）": 3.75,
			},
			postCosts: map[string]float64{
				"包装":   1.7,
				"产品损耗": 0.06,
				"利润税额": 2.2363875,
			},
			wantCostBase: 137.5125,
			wantFinal:    253.3282625,
		},
		{
			name:      "单品：孟连红果厌氧慢速日晒 24kg-48磅",
			productID: 45302,
			tierRate:  0.3,
			preCosts: map[string]float64{
				"燃气":     1.025,
				"租金":     0.1375,
				"生产损耗":   0.5,
				"人力（布勒）": 1.5,
			},
			postCosts: map[string]float64{
				"包装":   1.7,
				"产品损耗": 0.06,
				"利润税额": 0.61880625,
			},
			wantCostBase: 134.4125,
			wantFinal:    177.11505625,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
				PricingRuleID: 453,
				ProductID:     tc.productID,
				QuoteUnit:     "kg",
				Overrides: PricingRuleTrialOverrides{
					MarginRate:      floatPtr(tc.tierRate),
					OtherCosts:      tc.preCosts,
					PostMarkupCosts: tc.postCosts,
				},
			})
			if err != nil {
				t.Fatalf("PricingRuleTrial() error = %v", err)
			}
			if math.Abs(got.CostAfterYield-tc.wantCostBase) > 0.0001 {
				t.Fatalf("cost base = %.10f, want %.10f", got.CostAfterYield, tc.wantCostBase)
			}
			if math.Abs(got.FinalUnitPrice-tc.wantFinal) > 0.0001 {
				t.Fatalf("final unit price = %.10f, want Excel %.10f", got.FinalUnitPrice, tc.wantFinal)
			}
			if got.PostMarkupCostTotal <= 0 || got.PriceAfterMarkup <= 0 {
				t.Fatalf("supplier formula nodes missing totals: %+v", got)
			}
			for _, key := range []string{"material_cost", "other_cost_total", "price_after_markup", "post_markup_cost_total", "final_unit_price"} {
				if !pricingRuleTrialHasStep(got.Steps, key) {
					t.Fatalf("steps missing %q: %+v", key, got.Steps)
				}
			}
		})
	}
	t.Run("post markup override activates supplier formula", func(t *testing.T) {
		got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
			PricingRuleID: 454,
			ProductID:     45301,
			QuoteUnit:     "kg",
			Overrides: PricingRuleTrialOverrides{
				MarginRate: floatPtr(0.5421052631578949),
				OtherCosts: map[string]float64{
					"电力":     1.875,
					"租金":     0.1375,
					"生产损耗":   0.5,
					"人力（智烘）": 3.75,
				},
				PostMarkupCosts: map[string]float64{
					"包装":   1.7,
					"产品损耗": 0.06,
					"利润税额": 1.1996111842105266,
				},
			},
		})
		if err != nil {
			t.Fatalf("PricingRuleTrial() error = %v", err)
		}
		if math.Abs(got.FinalUnitPrice-116.70915065789475) > 0.0001 {
			t.Fatalf("final unit price = %.10f, want Excel %.10f", got.FinalUnitPrice, 116.70915065789475)
		}
		if !pricingRuleTrialHasStep(got.Steps, "post_markup_cost_total") {
			t.Fatalf("steps missing post markup total: %+v", got.Steps)
		}
	})
}

func TestPricingRuleTrialSupportsOverridesAndMinimumMarginWarning(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:          550,
			Name:               "PR452 固定加价商品",
			InventoryUnit:      "kg",
			QuoteUnit:          "kg",
			GreenBeanCostPerKg: 90,
			OperationCostPerKg: 10,
			YieldRate:          1,
		}},
		pricingRules: map[int64]ProductPricingRule{
			11: {
				ID:             11,
				Name:           "PR452 固定加价",
				MarginRate:     10,
				TaxRate:        0,
				RoundingMode:   "none",
				FormulaVersion: "v1",
				Active:         false,
				CalculationJSON: map[string]any{
					"yield_loss_mode":     "none",
					"profit_method":       "fixed_add",
					"tax_mode":            "none",
					"minimum_margin_rate": 0.3,
				},
			},
		},
	}
	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 11,
		ProductID:     550,
		Overrides: PricingRuleTrialOverrides{
			MarginRate: floatPtr(10),
		},
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.FinalUnitPrice != 110 || got.GrossMarginRate >= 0.3 {
		t.Fatalf("trial = %+v, want fixed add final price and low margin", got)
	}
	if !sliceContains(got.Warnings, "停用模板：试算仅供查看，不能作为新发布价格来源") {
		t.Fatalf("warnings = %+v, want inactive template warning", got.Warnings)
	}
	if !sliceContains(got.Warnings, "试算毛利率低于最低毛利") {
		t.Fatalf("warnings = %+v, want minimum margin warning", got.Warnings)
	}
}

func TestPricingRuleTrialSupportsMarkupTaxExcludedAndYuanRounding(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:          551,
			Name:               "PR452 加价未税商品",
			InventoryUnit:      "kg",
			QuoteUnit:          "kg",
			GreenBeanCostPerKg: 30,
			OperationCostPerKg: 5,
			YieldRate:          1,
		}},
		pricingRules: map[int64]ProductPricingRule{
			12: {
				ID:           12,
				Name:         "PR452 加价未税",
				MarginRate:   0.2,
				TaxRate:      0.1,
				RoundingMode: "yuan",
				Active:       true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "none",
					"profit_method":   "markup",
					"tax_mode":        "tax_excluded",
					"other_costs": map[string]any{
						"包装": 4,
					},
				},
			},
		},
	}
	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 12,
		ProductID:     551,
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.BaseCost != 35 || got.OtherCostTotal != 4 || got.CostAfterYield != 39 {
		t.Fatalf("trial costs = base %.2f other %.2f after yield %.2f", got.BaseCost, got.OtherCostTotal, got.CostAfterYield)
	}
	if got.PreTaxPrice != 46.8 || got.TaxAmount != 4.68 || got.FinalUnitPrice != 47 {
		t.Fatalf("trial prices = preTax %.2f tax %.2f final %.2f", got.PreTaxPrice, got.TaxAmount, got.FinalUnitPrice)
	}
	if got.TaxInPriceAmount != 0 || got.FinalBeforeRounding != 46.8 || got.RoundingAdjustment != 0.2 {
		t.Fatalf("tax excluded waterfall = taxInPrice %.2f finalBefore %.2f rounding %.2f", got.TaxInPriceAmount, got.FinalBeforeRounding, got.RoundingAdjustment)
	}
	waterfallTotal := got.CostBaseTotal + got.YieldLossAmount + got.ProfitMarkupAmount + got.TaxInPriceAmount + got.RoundingAdjustment
	if math.Abs(waterfallTotal-got.FinalUnitPrice) > 0.001 {
		t.Fatalf("waterfall total %.4f must equal final unit price %.4f", waterfallTotal, got.FinalUnitPrice)
	}
}

func TestPricingRuleTrialValidatesRuleAndProduct(t *testing.T) {
	svc := NewService(&fakeRepo{pricingRules: map[int64]ProductPricingRule{}})
	if _, err := svc.PricingRuleTrial(context.Background(), PricingRuleTrialCommand{PricingRuleID: 0, ProductID: 1}); err == nil {
		t.Fatal("expected pricing_rule_id required error")
	}
	if _, err := svc.PricingRuleTrial(context.Background(), PricingRuleTrialCommand{PricingRuleID: 99, ProductID: 1}); err == nil {
		t.Fatal("expected missing pricing rule error")
	}
	svc = NewService(&fakeRepo{
		inputs: []domain.ProductInput{},
		pricingRules: map[int64]ProductPricingRule{
			10: {ID: 10, Name: "PR452"},
		},
	})
	if _, err := svc.PricingRuleTrial(context.Background(), PricingRuleTrialCommand{PricingRuleID: 10, ProductID: 999}); err == nil {
		t.Fatal("expected missing product error")
	}
}

func pricingRuleTrialHasStep(steps []domain.PriceExplanationStep, key string) bool {
	for _, step := range steps {
		if step.Key == key {
			return true
		}
	}
	return false
}

func pricingRuleTrialWarningsContain(warnings []string, want string) bool {
	for _, warning := range warnings {
		if warning == want {
			return true
		}
	}
	return false
}

func TestGenerateBeanListPublicationPDFSavesAndReusesAsset(t *testing.T) {
	repo := &fakeRepo{beanListPublication: &BeanListPublication{
		ID:        7,
		ListType:  "commercial",
		Version:   "V3.0.5",
		Status:    "published",
		OwnerType: "official",
		Config:    map[string]any{},
		Content:   map[string]any{"groups": []any{}},
	}}
	svc := NewService(repo)
	renderCalls := 0
	render := func(BeanListPublication) ([]byte, error) {
		renderCalls++
		return []byte("%PDF-1.4"), nil
	}
	cmd := BeanListPublicationPDFCommand{PublicationID: 7, Query: BeanListPublicationQuery{ListType: "commercial", OwnerType: "official"}}

	first, err := svc.GenerateBeanListPublicationPDF(context.Background(), cmd, render)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.GenerateBeanListPublicationPDF(context.Background(), cmd, render)
	if err != nil {
		t.Fatal(err)
	}

	if renderCalls != 1 {
		t.Fatalf("render calls = %d", renderCalls)
	}
	if repo.savedBeanListAsset.PublicationID != 7 || repo.savedBeanListAsset.AssetType != "pdf" || repo.savedBeanListAsset.CacheKey != "bean-list-preview-style-v4:7:V3.0.5" {
		t.Fatalf("saved asset = %+v", repo.savedBeanListAsset)
	}
	if first.Filename != "bean-list-commercial-V3.0.5.pdf" || first.Bytes != len("%PDF-1.4") || second.Bytes != first.Bytes {
		t.Fatalf("pdf files = first %+v second %+v", first, second)
	}
}

func TestGenerateBeanListPublicationPDFRegeneratesStaleCacheKey(t *testing.T) {
	repo := &fakeRepo{
		beanListPublication: &BeanListPublication{
			ID:        7,
			ListType:  "commercial",
			Version:   "V3.0.5",
			Status:    "published",
			OwnerType: "official",
			Config:    map[string]any{},
			Content:   map[string]any{"groups": []any{}},
		},
		beanListAsset: BeanListPublicationAsset{
			PublicationID: 7,
			AssetType:     "pdf",
			ContentType:   "application/pdf",
			CacheKey:      "bean-list-preview-style-v1:7:V3.0.5",
			Payload:       []byte("%PDF-old-text-style"),
		},
	}
	svc := NewService(repo)
	renderCalls := 0
	render := func(BeanListPublication) ([]byte, error) {
		renderCalls++
		return []byte("%PDF-preview-style"), nil
	}
	cmd := BeanListPublicationPDFCommand{PublicationID: 7, Query: BeanListPublicationQuery{ListType: "commercial", OwnerType: "official"}}

	file, err := svc.GenerateBeanListPublicationPDF(context.Background(), cmd, render)
	if err != nil {
		t.Fatal(err)
	}

	if renderCalls != 1 {
		t.Fatalf("render calls = %d", renderCalls)
	}
	if repo.savedBeanListAsset.CacheKey != "bean-list-preview-style-v4:7:V3.0.5" || string(repo.savedBeanListAsset.Payload) != "%PDF-preview-style" {
		t.Fatalf("saved asset = %+v", repo.savedBeanListAsset)
	}
	if file.CacheKey != "bean-list-preview-style-v4:7:V3.0.5" || string(file.Payload) != "%PDF-preview-style" {
		t.Fatalf("file = %+v", file)
	}
}

func TestSettingsHidesDeprecatedYieldAndMarginParameters(t *testing.T) {
	repo := &fakeRepo{settings: []ParameterSetting{
		{Key: "roast_yield_rate", Label: "生豆到熟豆转化率", Value: 0.8, Unit: "ratio"},
		{Key: "kg_to_lb_factor", Label: "kg 到 lb 换算", Value: 0.454, Unit: "lb/kg"},
		{Key: "retail_bean_margin_rate", Label: "零售熟豆利润系数", Value: 0.6, Unit: "ratio"},
		{Key: "retail_tax_rate", Label: "零售税率", Value: 0.03, Unit: "ratio"},
		{Key: "retail_drip_multiplier", Label: "零售挂耳利润系数", Value: 2.5, Unit: "ratio"},
		{Key: "wholesale_kg_margin_rate_2", Label: "商用熟豆 14包-23包 利润系数", Value: 0.38, Unit: "ratio"},
		{Key: "wholesale_drip_multiplier_1", Label: "商用挂耳 100包 利润系数", Value: 2.2, Unit: "ratio"},
	}}

	rows, err := NewService(repo).Settings(context.Background())
	if err != nil {
		t.Fatalf("Settings() error = %v", err)
	}

	got := map[string]bool{}
	for _, row := range rows {
		got[row.Key] = true
	}
	for _, removed := range []string{
		"roast_yield_rate",
		"retail_bean_margin_rate",
		"retail_drip_multiplier",
		"wholesale_kg_margin_rate_2",
		"wholesale_drip_multiplier_1",
	} {
		if got[removed] {
			t.Fatalf("Settings() exposed deprecated quick setting %q in %+v", removed, rows)
		}
	}
	for _, kept := range []string{"kg_to_lb_factor", "retail_tax_rate"} {
		if !got[kept] {
			t.Fatalf("Settings() missing editable quick setting %q in %+v", kept, rows)
		}
	}
}

func TestCalculateReturnsCostingItems(t *testing.T) {
	svc := NewService(&fakeRepo{})
	resp, err := svc.Calculate(context.Background(), CalculateRequest{Products: []domain.ProductInput{{
		ProductID:          1,
		Name:               "金色山脉",
		GreenBeanCostPerKg: 62,
		YieldRate:          0.8,
	}}})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Name != "金色山脉" {
		t.Fatalf("items = %+v", resp.Items)
	}
	if resp.Items[0].Retail227gPrice <= 0 || len(resp.Items[0].WholesaleKgPrices) == 0 {
		t.Fatalf("missing calculated prices: %+v", resp.Items[0])
	}
}

func TestCreateRunCalculatesAndPersistsDatabaseInputs(t *testing.T) {
	repo := &fakeRepo{inputs: []domain.ProductInput{{
		ProductID:          7,
		Name:               "孟连水洗",
		GreenBeanCostPerKg: 62,
		YieldRate:          0.8,
	}}}
	svc := NewService(repo)

	run, err := svc.CreateRun(context.Background(), "JJ")
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if run.ID != 42 || run.ProductCount != 1 || len(repo.savedItems) != 1 {
		t.Fatalf("run = %+v, saved = %+v", run, repo.savedItems)
	}
}

func TestBeanListOrdersItemsByExcelCommercialCode(t *testing.T) {
	repo := &fakeRepo{inputs: []domain.ProductInput{
		{Name: "Uraga乌拉嘎", GreenBeanCostPerKg: 108, YieldRate: 0.8},
		{Name: "曲奇拼配", GreenBeanCostPerKg: 51.75, YieldRate: 0.8},
		{Name: "金色山脉", GreenBeanCostPerKg: 62, YieldRate: 0.8},
	}}
	svc := NewService(repo)

	resp, err := svc.BeanList(context.Background(), BeanListQuery{})
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}

	got := []string{
		resp.Items[0].CommercialBeanList.Code,
		resp.Items[1].CommercialBeanList.Code,
		resp.Items[2].CommercialBeanList.Code,
	}
	want := []string{"1.1", "3.1", "5.2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("codes = %+v, want %+v", got, want)
		}
	}
}

func TestBeanListRequiresExplicitGradientTemplateForCommercialTiers(t *testing.T) {
	repo := &fakeRepo{inputs: []domain.ProductInput{
		{
			ProductID:          501,
			Name:               "模板拼配",
			GreenBeanCostPerKg: 51.75,
			YieldRate:          0.8,
			GradientTemplate: &domain.GradientTemplate{
				ID:          9,
				Name:        "工厂量单模板",
				DisplayUnit: domain.GradientDisplayUnitKg,
				Tiers: []domain.GradientTemplateTier{{
					ID: 91, Label: "大客户量单", MinWeightG: 24000, MaxWeightG: floatPtr(49000), MarginRate: 0.175, Position: 1,
				}},
			},
		},
		{
			ProductID:          502,
			Name:               "金色山脉",
			GreenBeanCostPerKg: 62,
			YieldRate:          0.8,
		},
	}}
	svc := NewService(repo)

	resp, err := svc.BeanList(context.Background(), BeanListQuery{})
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}
	var templated, unbound domain.ProductResult
	for _, item := range resp.Items {
		if item.ProductID == 501 {
			templated = item
		}
		if item.ProductID == 502 {
			unbound = item
		}
	}
	if len(templated.CommercialWholesaleTiers) != 1 || templated.CommercialWholesaleTiers[0].Label != "大客户量单" || templated.CommercialWholesaleTiers[0].DisplayUnit != domain.GradientDisplayUnitKg {
		t.Fatalf("templated tiers = %+v", templated.CommercialWholesaleTiers)
	}
	if len(unbound.CommercialWholesaleTiers) != 0 {
		t.Fatalf("unbound tiers = %+v", unbound.CommercialWholesaleTiers)
	}
	if !sliceContains(unbound.Warnings, domain.MissingPricingMethodWarning) {
		t.Fatalf("unbound warnings = %+v", unbound.Warnings)
	}
}

func TestBeanListKeepsGreenBeanProductsOnDirectSaleTiers(t *testing.T) {
	repo := &fakeRepo{inputs: []domain.ProductInput{{
		ProductID:   909,
		Name:        "埃塞瑰夏生豆",
		ProductKind: "green_bean",
		GreenBeanSaleTiers: []domain.CommercialWholesaleTier{{
			Label:        "1kg+",
			SpecG:        1000,
			MinQty:       1,
			PricePerUnit: 128,
			DisplayUnit:  domain.GradientDisplayUnitKg,
		}},
	}}}
	svc := NewService(repo)

	resp, err := svc.BeanList(context.Background(), BeanListQuery{})
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %+v", resp.Items)
	}
	item := resp.Items[0]
	if item.ProductKind != "green_bean" {
		t.Fatalf("product_kind = %q", item.ProductKind)
	}
	if item.GreenBeanCostPerKg != 0 || item.RoastedBeanCostPerKg != 0 {
		t.Fatalf("green sales item must not run roasted costing, got green/roasted costs %.2f/%.2f", item.GreenBeanCostPerKg, item.RoastedBeanCostPerKg)
	}
	if item.GreenBeanList.Code == "" || item.GreenBeanList.DisplayName != "埃塞瑰夏生豆" {
		t.Fatalf("green bean list metadata = %+v", item.GreenBeanList)
	}
	if len(item.GreenBeanSaleTiers) != 1 || item.GreenBeanSaleTiers[0].PricePerUnit != 128 {
		t.Fatalf("green bean sale tiers = %+v", item.GreenBeanSaleTiers)
	}
}

func TestBeanListGreenBeanTemplateTiersDefaultToBomCostWithoutMargin(t *testing.T) {
	input := domain.ProductInput{
		ProductID:          910,
		Name:               "兰卡拼配生豆",
		ProductKind:        "green_bean",
		GreenBeanCostPerKg: 60,
		GradientTemplate: &domain.GradientTemplate{
			ID:          18,
			Name:        "生豆磅价模板",
			DisplayUnit: domain.GradientDisplayUnitLb,
			Tiers: []domain.GradientTemplateTier{{
				ID: 1801, Label: "24-49lb", MinWeightG: 24000, MaxWeightG: floatPtr(49000), MarginRate: 0.5, Position: 1,
			}},
		},
	}
	svc := NewService(&fakeRepo{inputs: []domain.ProductInput{input}})

	resp, err := svc.BeanList(context.Background(), BeanListQuery{})
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}
	if len(resp.Items) != 1 || len(resp.Items[0].GreenBeanSaleTiers) != 1 {
		t.Fatalf("green bean list items = %+v", resp.Items)
	}
	tier := resp.Items[0].GreenBeanSaleTiers[0]
	if tier.PricePerKg != 60 || tier.PricePerLb != 27.24 || tier.PricePerUnit != 27.24 || tier.MarginRate != 0 {
		t.Fatalf("green tier should default to BOM cost without margin, got %+v", tier)
	}
}

func TestBeanListAppliesProductMarginOverrideBeforeCategoryTemplateMargin(t *testing.T) {
	input := domain.ProductInput{
		ProductID:          501,
		Name:               "模板拼配",
		GreenBeanCostPerKg: 51.75,
		YieldRate:          0.8,
		GradientTemplate: &domain.GradientTemplate{
			ID:          9,
			Name:        "工厂量单模板",
			DisplayUnit: domain.GradientDisplayUnitKg,
			Tiers: []domain.GradientTemplateTier{{
				ID: 91, Label: "大客户量单", MinWeightG: 24000, MaxWeightG: floatPtr(49000), MarginRate: 0.175, Position: 1,
			}},
		},
	}
	setDomainProductInputFloat64PtrField(t, &input, "MarginRateOverride", 0.30)
	svc := NewService(&fakeRepo{inputs: []domain.ProductInput{input}})

	resp, err := svc.BeanList(context.Background(), BeanListQuery{})
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}
	if len(resp.Items) != 1 || len(resp.Items[0].CommercialWholesaleTiers) != 1 {
		t.Fatalf("bean list items = %+v", resp.Items)
	}
	tier := resp.Items[0].CommercialWholesaleTiers[0]
	if tier.MarginRate != 0.30 || tier.PricePerUnit != 91 {
		t.Fatalf("tier should use product margin override before category template margin, got %+v", tier)
	}
}

func TestPublishRunRequiresPositiveID(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if err := svc.PublishRun(context.Background(), "JJ", 0); err == nil {
		t.Fatalf("expected invalid id error")
	}
}

func TestSaveDripPriceTemplatePreservesOmittedBooleanFields(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.SaveDripPriceTemplate(context.Background(), SaveDripPriceTemplateCommand{
		Name:        "挂耳供应价",
		BagGrams:    10,
		BoxBagCount: 10,
		Tiers: []SaveDripPriceTemplateTierRow{{
			Label:      "100袋",
			MinBags:    100,
			Multiplier: 2.2,
		}},
	})
	if err != nil {
		t.Fatalf("SaveDripPriceTemplate() error = %v", err)
	}
	if repo.savedDripTemplate.Active != nil {
		t.Fatalf("active should stay nil when omitted, got %v", *repo.savedDripTemplate.Active)
	}
	if repo.savedDripTemplate.IncludePackaging != nil {
		t.Fatalf("include_packaging should stay nil when omitted, got %v", *repo.savedDripTemplate.IncludePackaging)
	}
}

func setDomainProductInputFloat64PtrField(t *testing.T, target any, fieldName string, value float64) {
	t.Helper()
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("missing %s field", fieldName)
	}
	if field.Kind() != reflect.Ptr || field.Type().Elem().Kind() != reflect.Float64 {
		t.Fatalf("%s field type = %s, want *float64", fieldName, field.Type())
	}
	field.Set(reflect.ValueOf(&value))
}

func TestPublishBeanListValidatesVersionAndListType(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial"}); err == nil {
		t.Fatalf("expected version required")
	}
	row, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V3.0.5"})
	if err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}
	if row.Status != "published" {
		t.Fatalf("row = %+v", row)
	}
	if _, err := svc.ListBeanListPublications(context.Background(), BeanListPublicationQuery{ListType: "bad"}); err == nil {
		t.Fatalf("expected invalid list type")
	}
	if _, err := svc.SaveBeanListDraft(context.Background(), PublishBeanListCommand{ListType: "green", Version: "VGREEN-1"}); err != nil {
		t.Fatalf("green bean list type should be publishable: %v", err)
	}
	if err := svc.WithdrawBeanList(context.Background(), WithdrawBeanListCommand{}); err == nil {
		t.Fatalf("expected invalid id")
	}
}

func TestPublishBeanListRequiresFinalPriceSnapshotOnPriceTiers(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	content := map[string]any{
		"groups": []any{
			map[string]any{
				"items": []any{
					map[string]any{
						"productId": float64(414),
						"name":      "曲奇拼配",
						"commercial_wholesale_tiers": []any{
							map[string]any{
								"label":          "1kg+",
								"spec_g":         float64(1000),
								"min_qty":        float64(1),
								"price_per_unit": float64(88),
								"display_unit":   "kg",
								"price_unit":     "kg",
							},
						},
					},
				},
			},
		},
	}

	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.0.0", Content: content}); err == nil {
		t.Fatalf("expected publish to reject price tiers without source price snapshot")
	}
	if _, err := svc.SaveBeanListDraft(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.0.0", Content: content}); err != nil {
		t.Fatalf("draft should allow incomplete price snapshots: %v", err)
	}

	tier := content["groups"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)["commercial_wholesale_tiers"].([]any)[0].(map[string]any)
	tier["source_price_record_id"] = float64(901)
	tier["final_unit_price"] = float64(88)
	tier["currency"] = "CNY"
	tier["inventory_unit"] = "kg"
	tier["inventory_conversion_json"] = map[string]any{"kg": map[string]any{"kg": float64(1)}}

	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.0.1", Content: content}); err != nil {
		t.Fatalf("PublishBeanList() with final price snapshot error = %v", err)
	}
}

func TestPublishBeanListRequiresPR440PriceListSnapshotMetadata(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	content := map[string]any{
		"price_rows": []any{
			map[string]any{
				"product_id":                float64(414),
				"product_name":              "曲奇拼配",
				"tier_label":                "24kg+",
				"min_qty":                   float64(24),
				"final_unit_price":          float64(82),
				"price_unit":                "kg",
				"currency":                  "CNY",
				"inventory_unit":            "kg",
				"inventory_conversion_json": map[string]any{"kg": map[string]any{"kg": float64(1)}},
				"source_price_record_id":    float64(901),
			},
		},
	}

	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.1.0", Content: content}); err == nil {
		t.Fatalf("expected publish to reject price rows without PR-440 snapshot metadata")
	}

	row := content["price_rows"].([]any)[0].(map[string]any)
	row["group_snapshot"] = map[string]any{
		"group_id":                float64(3),
		"group_name":              "商品价格表分组",
		"group_item_id":           float64(101),
		"group_item_name":         "大客户",
		"parent_group_item_id":    float64(100),
		"parent_group_item_name":  "商用豆",
		"classification_snapshot": "PR-440",
	}
	row["pricing_mode"] = "tier_template"
	row["pricing_mode_source"] = "subgroup"
	row["tier_template_id"] = float64(9)
	row["tier_template_source"] = "subgroup"
	row["template_tier_id"] = float64(91)
	row["pricing_rule_id"] = float64(90)
	row["pricing_rule_source"] = "product"
	row["pricing_rule_version"] = "PR-COST/v3"
	row["tier_pricing_rule_id"] = float64(90)
	row["tier_pricing_rule_version"] = "PR-COST/v3"
	row["cost_source_snapshot"] = map[string]any{"bom_version_no": "BOM-A1/V002", "process_route_name": "标准烘焙"}
	row["customer_reference_snapshot"] = map[string]any{"customer_id": float64(5), "customer_display_name": "Karen 拼配", "customer_item_code": "K-ESP"}
	row["manual_adjusted"] = true

	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.1.1", Content: content}); err != nil {
		t.Fatalf("PublishBeanList() with PR-440 snapshot metadata error = %v", err)
	}
}

func TestPublishBeanListAcceptsPricingRuleAndFixedPriceModes(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	base := func(mode string) map[string]any {
		return map[string]any{
			"product_id":                float64(414),
			"product_name":              "曲奇拼配",
			"pricing_mode":              mode,
			"pricing_mode_source":       "product",
			"tier_label":                "基础价",
			"min_qty":                   float64(0),
			"final_unit_price":          float64(82),
			"price_unit":                "kg",
			"currency":                  "CNY",
			"inventory_unit":            "kg",
			"inventory_conversion_json": map[string]any{"kg": map[string]any{"kg": float64(1)}},
			"group_snapshot":            map[string]any{"group_id": float64(3), "group_name": "商品价格表分组", "group_item_id": float64(101), "group_item_name": "大客户"},
			"cost_source_snapshot":      map[string]any{"cost_source_mode": mode},
			"customer_reference_snapshot": map[string]any{
				"customer_id":           float64(5),
				"customer_display_name": "Karen 拼配",
			},
			"manual_adjusted": false,
		}
	}

	pricingRuleRow := base("pricing_rule")
	pricingRuleRow["pricing_rule_id"] = float64(90)
	pricingRuleRow["pricing_rule_source"] = "product"
	pricingRuleRow["pricing_rule_version"] = "PR-COST/v3"
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.2.0", Content: map[string]any{"price_rows": []any{pricingRuleRow}}}); err != nil {
		t.Fatalf("PublishBeanList() pricing_rule mode error = %v", err)
	}

	fixedRow := base("fixed_price")
	fixedRow["tier_label"] = "固定价"
	fixedRow["fixed_unit_price"] = float64(82)
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.2.1", Content: map[string]any{"price_rows": []any{fixedRow}}}); err != nil {
		t.Fatalf("PublishBeanList() fixed_price mode error = %v", err)
	}

	badFixedRow := base("fixed_price")
	badFixedRow["fixed_unit_price"] = float64(0)
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.2.2", Content: map[string]any{"price_rows": []any{badFixedRow}}}); err == nil {
		t.Fatalf("PublishBeanList() fixed_price mode must reject missing fixed_unit_price")
	}
}

func TestPublishBeanListKeepsCustomerSnapshotOwnerAndSources(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	row, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{
		ListType:                 "commercial",
		Version:                  "V3.0.7",
		OwnerType:                "actor",
		OwnerKey:                 "employee:7",
		PriceSourcePublicationID: 11,
		StyleSourcePublicationID: 5,
		Content: map[string]any{
			"totalItems": float64(25),
		},
	})
	if err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}
	if row.OwnerType != "actor" || row.OwnerKey != "employee:7" {
		t.Fatalf("row owner = %s/%s", row.OwnerType, row.OwnerKey)
	}
	if repo.publishedBeanList.OwnerType != "actor" || repo.publishedBeanList.OwnerKey != "employee:7" {
		t.Fatalf("cmd owner = %+v", repo.publishedBeanList)
	}
	if repo.publishedBeanList.PriceSourcePublicationID != 11 || repo.publishedBeanList.StyleSourcePublicationID != 5 {
		t.Fatalf("source ids = %+v", repo.publishedBeanList)
	}
}

func TestPublishGreenBeanListAppliesManualKgPriceOverridesToKgContentSnapshot(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{
		ListType: "green",
		Version:  "VGREEN-1",
		Config: map[string]any{
			"customizers": map[string]any{
				"414": map[string]any{
					"greenPriceOverrides": map[string]any{
						"51": float64(62),
					},
				},
			},
		},
		Content: map[string]any{
			"groups": []any{
				map[string]any{
					"items": []any{
						map[string]any{
							"productId": float64(414),
							"name":      "兰卡拼配生豆",
							"prices": []any{
								map[string]any{"label": "60kg+", "price": float64(51.75), "unit": "kg"},
							},
							"green_bean_sale_tiers": []any{
								map[string]any{
									"label":                     "60kg+",
									"source_price_record_id":    float64(905),
									"final_unit_price":          float64(51.75),
									"currency":                  "CNY",
									"inventory_unit":            "kg",
									"inventory_conversion_json": map[string]any{"kg": map[string]any{"kg": float64(1)}},
									"template_tier_id":          float64(51),
									"spec_g":                    float64(1000),
									"min_qty":                   float64(60),
									"price_per_unit":            float64(51.75),
									"price_per_lb":              float64(23.49),
									"display_unit":              "kg",
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}
	groups := repo.publishedBeanList.Content["groups"].([]any)
	item := groups[0].(map[string]any)["items"].([]any)[0].(map[string]any)
	price := item["prices"].([]any)[0].(map[string]any)
	if price["price"] != float64(62) || price["unit"] != "kg" {
		t.Fatalf("price row = %#v, want 62/kg", price)
	}
	tier := item["green_bean_sale_tiers"].([]any)[0].(map[string]any)
	if tier["price_per_lb"] != float64(28.15) || tier["price_per_unit"] != float64(62) || tier["price_unit"] != "kg" || tier["display_unit"] != "kg" {
		t.Fatalf("tier = %#v, want kg range with 62/kg", tier)
	}
	if tier["price_per_kg"] != float64(62) {
		t.Fatalf("price_per_kg = %#v, want 62", tier["price_per_kg"])
	}
}

func TestSaveBeanListDraftValidatesAndKeepsCustomerOwner(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	row, err := svc.SaveBeanListDraft(context.Background(), PublishBeanListCommand{
		ListType:  "retail",
		Version:   " V3.0.8 ",
		OwnerType: "actor",
		OwnerKey:  "employee:7",
		Config:    map[string]any{"layoutStyle": "card"},
		Content:   map[string]any{"totalItems": float64(1)},
	})
	if err != nil {
		t.Fatalf("SaveBeanListDraft() error = %v", err)
	}
	if row.Status != "draft" || row.ListType != "retail" || row.Version != "V3.0.8" {
		t.Fatalf("draft row = %+v", row)
	}
	if repo.draftBeanList.OwnerType != "actor" || repo.draftBeanList.OwnerKey != "employee:7" {
		t.Fatalf("draft owner = %+v", repo.draftBeanList)
	}
	if repo.draftBeanList.Config == nil || repo.draftBeanList.Content == nil {
		t.Fatalf("draft should normalize empty config/content maps: %+v", repo.draftBeanList)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
