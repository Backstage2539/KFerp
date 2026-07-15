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
	params               domain.Parameters
	inputs               []domain.ProductInput
	settings             []ParameterSetting
	customerInputs       []domain.ProductInput
	lastCustomerID       int64
	savedItems           []domain.ProductResult
	publishedID          int64
	savedDripTemplate    SaveDripPriceTemplateCommand
	deactivatedDripID    int64
	publishedBeanList    PublishBeanListCommand
	draftBeanList        PublishBeanListCommand
	beanListPublications []BeanListPublication
	lastBeanListQuery    BeanListPublicationQuery
	archivedBeanLists    ArchiveBeanListPublicationsCommand
	unarchivedBeanLists  ArchiveBeanListPublicationsCommand
	beanListPublication  *BeanListPublication
	beanListAsset        BeanListPublicationAsset
	savedBeanListAsset   BeanListPublicationAsset
	pricingRules         map[int64]ProductPricingRule
	costDetails          []PricingRuleTrialBaseCostDetail
	costDetailsByBom     map[int64][]PricingRuleTrialBaseCostDetail
	productionOptions    PricingRuleTrialProductionOptions
	lastDetailInput      domain.ProductInput
	defaultTaxRate       PricingRuleTrialDefaultTaxRate
	productUnitRules     map[int64]ProductSalesUnitRule
	customerUnitRules    map[int64]ProductSalesUnitRule
	lastCustomerAliasID  int64
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

func (r *fakeRepo) LoadPricingRuleTrialBaseCostDetails(_ context.Context, input domain.ProductInput) ([]PricingRuleTrialBaseCostDetail, error) {
	r.lastDetailInput = input
	if r.costDetailsByBom != nil {
		if rows, ok := r.costDetailsByBom[input.BomVersionID]; ok {
			return rows, nil
		}
	}
	return r.costDetails, nil
}

func (r *fakeRepo) LoadPricingRuleTrialProductionOptions(_ context.Context, _ domain.ProductInput) (PricingRuleTrialProductionOptions, error) {
	return r.productionOptions, nil
}

func (r *fakeRepo) LoadPricingRuleTrialDefaultTaxRate(context.Context) (PricingRuleTrialDefaultTaxRate, error) {
	return r.defaultTaxRate, nil
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

func (r *fakeRepo) ListBeanListPublications(_ context.Context, query BeanListPublicationQuery) ([]BeanListPublication, error) {
	r.lastBeanListQuery = query
	return r.beanListPublications, nil
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

func (r *fakeRepo) ResolveProductSalesUnitRule(_ context.Context, productID int64, priceUnit string) (ProductSalesUnitRule, error) {
	if r.productUnitRules == nil {
		return ProductSalesUnitRule{}, nil
	}
	rule, ok := r.productUnitRules[productID]
	if !ok {
		return ProductSalesUnitRule{}, ErrProductSalesUnitRuleNotFound
	}
	if _, ok := rule.Conversion[priceUnit]; !ok {
		return ProductSalesUnitRule{}, ErrProductSalesUnitRuleNotFound
	}
	return rule, nil
}

func (r *fakeRepo) ResolveCustomerProductSalesUnitRule(_ context.Context, productID int64, customerProductAliasID int64, priceUnit string) (ProductSalesUnitRule, error) {
	r.lastCustomerAliasID = customerProductAliasID
	if r.customerUnitRules == nil {
		return r.ResolveProductSalesUnitRule(context.Background(), productID, priceUnit)
	}
	rule, ok := r.customerUnitRules[customerProductAliasID]
	if !ok {
		return ProductSalesUnitRule{}, ErrProductSalesUnitRuleNotFound
	}
	if _, ok := rule.Conversion[priceUnit]; !ok {
		return ProductSalesUnitRule{}, ErrProductSalesUnitRuleNotFound
	}
	return rule, nil
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

func (r *fakeRepo) ArchiveBeanListPublications(_ context.Context, cmd ArchiveBeanListPublicationsCommand) error {
	r.archivedBeanLists = cmd
	return nil
}

func (r *fakeRepo) UnarchiveBeanListPublications(_ context.Context, cmd ArchiveBeanListPublicationsCommand) error {
	r.unarchivedBeanLists = cmd
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
			BomUsageMode:             "production_bom_output",
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
	if got.BaseCost != 60 || got.OtherCostTotal != 2.5 || got.CostAfterYield != 62.5 {
		t.Fatalf("trial costs = base %.2f other %.2f after yield %.2f", got.BaseCost, got.OtherCostTotal, got.CostAfterYield)
	}
	if len(got.OtherCostDetails) != 1 {
		t.Fatalf("other cost details = %+v, want one template row", got.OtherCostDetails)
	}
	if row := got.OtherCostDetails[0]; row.Name != "包装贴标" || row.Amount != 2.5 || row.Unit != "kg" || row.Source != "pricing_rule" || !strings.Contains(row.SettingLocation, "价格计算模板编辑区") {
		t.Fatalf("other cost detail = %+v, want pricing rule source and setting location", row)
	}
	if got.BomCostTotal != 50 || got.OperationCostTotal != 10 || len(got.BaseCostDetails) != 2 {
		t.Fatalf("base details = bom %.2f operation %.2f rows %+v", got.BomCostTotal, got.OperationCostTotal, got.BaseCostDetails)
	}
	if got.CostSource != "standard_manufacturing_cost" || got.MaterialUnitCost != 50 || got.OperationUnitCost != 10 || got.StandardManufacturingUnitCost != 60 {
		t.Fatalf("standard manufacturing cost fields = source %q material %.2f operation %.2f total %.2f", got.CostSource, got.MaterialUnitCost, got.OperationUnitCost, got.StandardManufacturingUnitCost)
	}
	if got.BomSnapshot.VersionID != 3315 || got.BomSnapshot.VersionNo != "BOM-v1" || got.ProcessRouteSnapshot.ID != 0 {
		t.Fatalf("standard manufacturing snapshots missing BOM/process source: bom=%+v process=%+v", got.BomSnapshot, got.ProcessRouteSnapshot)
	}
	if got.WorkstationCostSnapshot.MaterialUnitCost != 50 || got.WorkstationCostSnapshot.OperationUnitCost != 10 || got.WorkstationCostSnapshot.StandardManufacturingUnitCost != 60 {
		t.Fatalf("workstation cost snapshot = %+v, want material/operation/standard costs", got.WorkstationCostSnapshot)
	}
	if got.CostBaseTotal != 62.5 || got.YieldLossAmount != 0 || got.ProfitMarkupAmount != 20.83 || got.TaxInPriceAmount != 5 || got.FinalBeforeRounding != 88.33 || got.RoundingAdjustment != -0.03 {
		t.Fatalf("waterfall = base %.2f loss %.2f profit %.2f taxInPrice %.2f finalBefore %.2f rounding %.2f", got.CostBaseTotal, got.YieldLossAmount, got.ProfitMarkupAmount, got.TaxInPriceAmount, got.FinalBeforeRounding, got.RoundingAdjustment)
	}
	if got.PreTaxPrice != 83.33 || got.TaxAmount != 5 || got.FinalUnitPrice != 88.3 {
		t.Fatalf("trial prices = preTax %.2f tax %.2f final %.2f", got.PreTaxPrice, got.TaxAmount, got.FinalUnitPrice)
	}
	if got.ProfitExplanation.Method != "gross_margin" || got.ProfitExplanation.MethodLabel != "毛利率" || got.ProfitExplanation.Rate != 0.25 || got.ProfitExplanation.Source != "pricing_rule" {
		t.Fatalf("profit explanation = %+v, want gross margin from pricing rule", got.ProfitExplanation)
	}
	if got.ProfitExplanation.CostAfterYield != got.CostAfterYield || got.ProfitExplanation.MarkupAmount != got.ProfitMarkupAmount || got.ProfitExplanation.PreTaxPrice != got.PreTaxPrice {
		t.Fatalf("profit explanation amounts = %+v, want current waterfall amounts", got.ProfitExplanation)
	}
	if !strings.Contains(got.ProfitExplanation.Formula, "损耗后成本 / (1 - 毛利率 25%)") {
		t.Fatalf("profit explanation formula = %q, want gross margin formula", got.ProfitExplanation.Formula)
	}
	waterfallTotal := got.CostBaseTotal + got.YieldLossAmount + got.ProfitMarkupAmount + got.TaxInPriceAmount + got.RoundingAdjustment
	if math.Abs(waterfallTotal-got.FinalUnitPrice) > 0.001 {
		t.Fatalf("waterfall total %.4f must equal final unit price %.4f", waterfallTotal, got.FinalUnitPrice)
	}
	if got.FormulaExpression == "" || !sliceContains(got.FormulaExpressionLines, "最终售价 = 88.3/kg") {
		t.Fatalf("formula expression = %q lines = %+v, want final price line", got.FormulaExpression, got.FormulaExpressionLines)
	}
	for _, want := range []string{"(标准制造成本 60/kg + 其他成本 2.5/kg)", "/ (1 - 毛利率 25%)", "* (1 + 税率 6%)"} {
		if !strings.Contains(got.FormulaExpression, want) {
			t.Fatalf("formula expression = %q, want %q", got.FormulaExpression, want)
		}
	}
	if strings.Contains(got.FormulaExpression, "/ (1 - 损耗率 20%)") {
		t.Fatalf("formula expression = %q, should not add default product/BOM loss on actual BOM detail cost", got.FormulaExpression)
	}
	for _, key := range []string{"standard_manufacturing_cost", "other_cost_total", "expected_loss_rate", "profit_method", "tax_rate", "rounding_rule", "final_unit_price"} {
		if !pricingRuleTrialHasStep(got.Steps, key) {
			t.Fatalf("steps missing %q: %+v", key, got.Steps)
		}
	}
}

func TestPricingRuleTrialUsesOperationMasterStandardCost(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:        549,
			Name:             "PR516 试算商品",
			InventoryUnit:    "kg",
			QuoteUnit:        "kg",
			BomVersionID:     3315,
			BomVersionNo:     "BOM-v1",
			BomStatus:        "active",
			ProcessRouteID:   77,
			ProcessRouteName: "标准烘焙",
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{{
			Key:                     "operation:standard:1",
			Type:                    "operation",
			TypeLabel:               "标准工序",
			Name:                    "烘焙",
			ConsumeUnit:             "per_inventory_unit",
			Unit:                    "kg",
			CostUnit:                "kg",
			UnitCost:                8.5,
			CostUnitCost:            8.5,
			AmountPerUnit:           8.5,
			CapacitySelectionSource: "operation_master",
			Description:             "标准工序成本来自工序列表：8.5000/kg",
		}},
		pricingRules: map[int64]ProductPricingRule{
			10: {
				ID:             10,
				Name:           "PR516 毛利",
				CostSourceMode: "bom_current_cost",
				MarginRate:     0.2,
				FormulaVersion: "v2",
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
		ProductID:     549,
		QuoteUnit:     "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if pricingRuleTrialWarningsContain(got.Warnings, "标准成本默认产能") {
		t.Fatalf("warnings = %+v, should not mention retired route default capacity", got.Warnings)
	}
	if len(got.BaseCostDetails) != 1 || got.BaseCostDetails[0].CapacitySelectionSource != "operation_master" {
		t.Fatalf("base cost detail source not preserved: %+v", got.BaseCostDetails)
	}
	if got.OperationUnitCost != 8.5 {
		t.Fatalf("operation unit cost = %.2f, want 8.5", got.OperationUnitCost)
	}
}

func TestPricingRuleTrialUsesFinanceTaxRateWhenPricingRuleTaxRateUnset(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:        612,
			Name:             "PR512 财务税率商品",
			InventoryUnit:    "kg",
			QuoteUnit:        "kg",
			BomVersionID:     7612,
			BomUsageMode:     "production_bom_output",
			BomStatus:        "active",
			ExpectedLossRate: 0.2,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:612", Type: "material", TypeLabel: "物料", Name: "PR512 原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 100, AmountPerKg: 100, Unit: "kg"},
		},
		defaultTaxRate: PricingRuleTrialDefaultTaxRate{Rate: 0.13, Source: "finance_settings"},
		pricingRules: map[int64]ProductPricingRule{
			512: {
				ID:           512,
				Name:         "PR512 税率回退",
				MarginRate:   0,
				TaxRate:      0,
				RoundingMode: "none",
				Active:       true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "bom_or_product",
					"profit_method":   "markup",
					"tax_mode":        "tax_included",
				},
			},
		},
	}

	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 512,
		ProductID:     612,
		QuoteUnit:     "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.YieldLossAmount != 0 {
		t.Fatalf("BOM detail cost already includes BOM loss, default product loss must not be applied again: %+v", got)
	}
	if got.TaxRateSource != "finance_settings" || got.TaxAmount != 13 || got.FinalUnitPrice != 113 {
		t.Fatalf("tax fallback = source %q amount %.2f final %.2f, want finance settings 13%%", got.TaxRateSource, got.TaxAmount, got.FinalUnitPrice)
	}
	if !pricingRuleTrialHasStepSource(got.Steps, "tax_rate", "finance_settings") {
		t.Fatalf("steps must record finance tax source: %+v", got.Steps)
	}
}

func TestPricingRuleTrialRuleTaxRateOverridesFinanceDefault(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:     613,
			Name:          "PR512 模板税率商品",
			InventoryUnit: "kg",
			QuoteUnit:     "kg",
			BomVersionID:  7613,
			BomUsageMode:  "production_bom_output",
			BomStatus:     "active",
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:613", Type: "material", TypeLabel: "物料", Name: "PR512 原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 100, AmountPerKg: 100, Unit: "kg"},
		},
		defaultTaxRate: PricingRuleTrialDefaultTaxRate{Rate: 0.13, Source: "finance_settings"},
		pricingRules: map[int64]ProductPricingRule{
			513: {
				ID:           513,
				Name:         "PR512 模板税率",
				MarginRate:   0,
				TaxRate:      0.06,
				RoundingMode: "none",
				Active:       true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "none",
					"profit_method":   "markup",
					"tax_mode":        "tax_included",
				},
			},
		},
	}

	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 513,
		ProductID:     613,
		QuoteUnit:     "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.TaxRateSource != "pricing_rule" || got.TaxAmount != 6 || got.FinalUnitPrice != 106 {
		t.Fatalf("tax override = source %q amount %.2f final %.2f, want pricing rule 6%%", got.TaxRateSource, got.TaxAmount, got.FinalUnitPrice)
	}
}

func TestPricingRuleTrialTaxModeNoneForcesZeroTax(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:     614,
			Name:          "PR512 不计税商品",
			InventoryUnit: "kg",
			QuoteUnit:     "kg",
			BomVersionID:  7614,
			BomUsageMode:  "production_bom_output",
			BomStatus:     "active",
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:614", Type: "material", TypeLabel: "物料", Name: "PR512 原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 100, AmountPerKg: 100, Unit: "kg"},
		},
		defaultTaxRate: PricingRuleTrialDefaultTaxRate{Rate: 0.13, Source: "finance_settings"},
		pricingRules: map[int64]ProductPricingRule{
			514: {
				ID:           514,
				Name:         "PR512 不计税",
				MarginRate:   0,
				TaxRate:      0.06,
				RoundingMode: "none",
				Active:       true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "none",
					"profit_method":   "markup",
					"tax_mode":        "none",
				},
			},
		},
	}

	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 514,
		ProductID:     614,
		QuoteUnit:     "kg",
		Overrides: PricingRuleTrialOverrides{
			TaxRate: floatPtr(0.2),
		},
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.TaxRateSource != "tax_disabled" || got.TaxAmount != 0 || got.FinalUnitPrice != 100 {
		t.Fatalf("tax disabled = source %q amount %.2f final %.2f, want forced zero tax", got.TaxRateSource, got.TaxAmount, got.FinalUnitPrice)
	}
}

func TestPricingRuleTrialExplicitTemporaryLossStillAppliesToBomCost(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:        551,
			Name:             "PR511 显式临时损耗商品",
			InventoryUnit:    "kg",
			QuoteUnit:        "kg",
			ExpectedLossRate: 0.2,
			BomVersionID:     5511,
			BomVersionNo:     "V002",
			BomUsageMode:     "production_bom_output",
			BomStatus:        "active",
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:551", Type: "material", TypeLabel: "物料", Name: "已含原料损耗物料", ConsumeUnit: "ratio_pct", RatioPct: 125, MaterialLossRate: 0.2, UnitCost: 80, AmountPerKg: 100, Unit: "kg"},
		},
		pricingRules: map[int64]ProductPricingRule{
			12: {
				ID:             12,
				Name:           "PR511 毛利",
				MarginRate:     0.25,
				TaxRate:        0,
				RoundingMode:   "none",
				FormulaVersion: "v1",
				Active:         true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "bom_or_product",
					"profit_method":   "gross_margin",
					"tax_mode":        "none",
				},
			},
		},
	}
	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 12,
		ProductID:     551,
		Overrides: PricingRuleTrialOverrides{
			ExpectedLossRate: floatPtr(0.2),
		},
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.CostBaseTotal != 100 || got.CostAfterYield != 125 || got.YieldLossAmount != 25 {
		t.Fatalf("explicit temporary loss waterfall = base %.2f after %.2f loss %.2f, want 100 -> 125 with 25 loss", got.CostBaseTotal, got.CostAfterYield, got.YieldLossAmount)
	}
	if got.FinalUnitPrice != 166.67 {
		t.Fatalf("final unit price = %.2f, want explicit loss to affect gross-margin price", got.FinalUnitPrice)
	}
	if !strings.Contains(got.FormulaExpression, "/ (1 - 损耗率 20%)") {
		t.Fatalf("formula expression = %q, want explicit temporary loss formula", got.FormulaExpression)
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
	if !pricingRuleTrialWarningsContain(got.Warnings, "该商品暂无可试算的标准制造成本") {
		t.Fatalf("warnings = %+v, want missing BOM warning", got.Warnings)
	}
	if strings.Contains(strings.Join(got.Warnings, "\n"), "反推") {
		t.Fatalf("warnings must not mention snapshot inference: %+v", got.Warnings)
	}
}

func TestPricingRuleTrialIgnoresLegacySummaryCostWithoutOutputBomDetails(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:          540,
			Name:               "旧商品成本汇总商品",
			InventoryUnit:      "kg",
			QuoteUnit:          "kg",
			GreenBeanCostPerKg: 999,
			OperationCostPerKg: 1,
			YieldRate:          1,
			BomUsageMode:       "legacy_product_summary",
			BomStatus:          "missing",
		}},
		pricingRules: map[int64]ProductPricingRule{
			10: {
				ID:           10,
				Name:         "PR460 不读旧商品汇总",
				MarginRate:   0.2,
				TaxRate:      0,
				RoundingMode: "none",
				Active:       true,
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
		ProductID:     540,
		QuoteUnit:     "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.BaseCost != 0 || got.BomCostTotal != 0 || got.OperationCostTotal != 0 || got.FinalUnitPrice != 0 || len(got.BaseCostDetails) != 0 {
		t.Fatalf("trial must not use legacy product summary costs: base %.2f bom %.2f op %.2f final %.2f details %+v", got.BaseCost, got.BomCostTotal, got.OperationCostTotal, got.FinalUnitPrice, got.BaseCostDetails)
	}
	if !pricingRuleTrialWarningsContain(got.Warnings, "该商品暂无可试算的标准制造成本") {
		t.Fatalf("warnings = %+v, want missing BOM/operation cost warning", got.Warnings)
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
	if pricingRuleTrialWarningsContain(got.Warnings, "该商品暂无可试算的标准制造成本") {
		t.Fatalf("warnings should not claim missing cost when details exist: %+v", got.Warnings)
	}
}

func TestPricingRuleTrialUsesSelectedOutputBomVersionAndOperationTemplate(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:           539,
			Name:                "PR439-20260606182321 工厂量单商品",
			InventoryUnit:       "kg",
			QuoteUnit:           "kg",
			GreenBeanCostPerKg:  999,
			OperationCostPerKg:  999,
			YieldRate:           0.8,
			ExpectedLossRate:    0.2,
			BomStatus:           "active",
			BomVersionID:        5391,
			BomVersionNo:        "V001",
			BomUsageMode:        "production_bom_output",
			OperationTemplateID: 7,
		}},
		productionOptions: PricingRuleTrialProductionOptions{
			BomVersions: []PricingRuleTrialBomVersionOption{
				{BomID: 539, BomCode: "BOM-000539", BomName: "PR439-20260606182321 工厂量单商品 生产 BOM", VersionID: 5391, VersionNo: "V001", Status: "published", IsDefault: false},
				{BomID: 539, BomCode: "BOM-000539", BomName: "PR439-20260606182321 工厂量单商品 生产 BOM", VersionID: 5392, VersionNo: "V002", Status: "published", IsDefault: true},
			},
			OperationTemplates: []PricingRuleTrialOperationTemplateOption{
				{ID: 7, Name: "旧工序", IsDefault: true},
				{ID: 9, Name: "新版工序", IsDefault: false},
			},
		},
		costDetailsByBom: map[int64][]PricingRuleTrialBaseCostDetail{
			5392: {
				{Key: "material:1001", Type: "material", TypeLabel: "物料", Name: "V002 原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 42, AmountPerKg: 42, Unit: "kg"},
				{Key: "operation:9001", Type: "operation", TypeLabel: "工序", Name: "新版工序", ConsumeUnit: "per_kg", UnitCost: 8, AmountPerKg: 8, Unit: "kg"},
			},
		},
		pricingRules: map[int64]ProductPricingRule{
			10: {
				ID:             10,
				Name:           "PR460 输出 BOM 试算",
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
		PricingRuleID:       10,
		ProductID:           539,
		BomVersionID:        5392,
		OperationTemplateID: 9,
		QuoteUnit:           "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if repo.lastDetailInput.BomVersionID != 5392 || repo.lastDetailInput.OperationTemplateID != 9 {
		t.Fatalf("detail input = %+v, want selected BOM version 5392 and operation template 9", repo.lastDetailInput)
	}
	if got.BomVersionID != 5392 || got.BomVersionNo != "V002" || got.OperationTemplateID != 9 || got.OperationTemplateName != "新版工序" {
		t.Fatalf("selected production sources = %+v", got)
	}
	if got.BaseCost != 50 || got.BomCostTotal != 42 || got.OperationCostTotal != 8 || got.FinalUnitPrice != 60 {
		t.Fatalf("trial must price from selected detail rows, got base %.2f bom %.2f op %.2f final %.2f", got.BaseCost, got.BomCostTotal, got.OperationCostTotal, got.FinalUnitPrice)
	}
	if len(got.BomVersionOptions) != 2 || !got.BomVersionOptions[1].IsDefault || len(got.OperationTemplateOptions) != 2 {
		t.Fatalf("options missing = %+v / %+v", got.BomVersionOptions, got.OperationTemplateOptions)
	}
}

func TestPricingRuleTrialUsesSelectedProcessRouteBeforeLegacyOperationTemplate(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:           615,
			Name:                "PR512 工艺路线商品",
			InventoryUnit:       "kg",
			QuoteUnit:           "kg",
			BomStatus:           "active",
			BomVersionID:        7151,
			BomVersionNo:        "V001",
			BomUsageMode:        "production_bom_output",
			ProcessRouteID:      21,
			ProcessRouteName:    "默认烘焙路线",
			OperationTemplateID: 7,
		}},
		productionOptions: PricingRuleTrialProductionOptions{
			BomVersions: []PricingRuleTrialBomVersionOption{
				{BomID: 615, BomCode: "BOM-000615", BomName: "PR512 工艺路线商品 生产 BOM", VersionID: 7151, VersionNo: "V001", Status: "published", IsDefault: true, ProcessRouteID: 21, ProcessRouteName: "默认烘焙路线"},
			},
			ProcessRoutes: []PricingRuleTrialProcessRouteOption{
				{ID: 21, Name: "默认烘焙路线", IsDefault: true},
				{ID: 42, Name: "手选包装路线", IsDefault: false},
			},
			OperationTemplates: []PricingRuleTrialOperationTemplateOption{
				{ID: 7, Name: "旧工序模板", IsDefault: true},
			},
		},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:615", Type: "material", TypeLabel: "物料", Name: "路线原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 30, AmountPerKg: 30, Unit: "kg"},
			{Key: "operation:42", Type: "operation", TypeLabel: "工艺路线", Name: "手选包装路线", ConsumeUnit: "process_route", UnitCost: 5, AmountPerKg: 5, Unit: "kg"},
		},
		pricingRules: map[int64]ProductPricingRule{
			515: {
				ID:           515,
				Name:         "PR512 工艺路线试算",
				MarginRate:   0.2,
				RoundingMode: "none",
				Active:       true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "none",
					"profit_method":   "markup",
					"tax_mode":        "none",
				},
			},
		},
	}

	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID:  515,
		ProductID:      615,
		ProcessRouteID: 42,
		QuoteUnit:      "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if repo.lastDetailInput.ProcessRouteID != 42 || repo.lastDetailInput.OperationTemplateID != 0 {
		t.Fatalf("detail input = %+v, want selected process route 42 and no legacy operation template", repo.lastDetailInput)
	}
	if got.ProcessRouteID != 42 || got.ProcessRouteName != "手选包装路线" || len(got.ProcessRouteOptions) != 2 {
		t.Fatalf("selected process route missing in result: %+v", got)
	}
	if got.OperationTemplateID != 0 || len(got.OperationTemplateOptions) != 1 {
		t.Fatalf("legacy operation template must remain compatibility-only: %+v", got)
	}
	if got.BaseCost != 35 || got.OperationCostTotal != 5 || got.FinalUnitPrice != 42 {
		t.Fatalf("trial must include process route planned cost, got base %.2f op %.2f final %.2f", got.BaseCost, got.OperationCostTotal, got.FinalUnitPrice)
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
				BomVersionID:       453011,
				GreenBeanCostPerKg: 67.5, // 物料成本!C4 / 生产项目!H3 = 54 / 0.8
				YieldRate:          1,
			},
			{
				ProductID:          45302,
				Name:               "单品：孟连红果厌氧慢速日晒",
				InventoryUnit:      "kg",
				QuoteUnit:          "kg",
				BomVersionID:       453022,
				GreenBeanCostPerKg: 131.25, // 物料成本!C5 / 生产项目!H3 = 105 / 0.8
				YieldRate:          1,
			},
		},
		costDetailsByBom: map[int64][]PricingRuleTrialBaseCostDetail{
			453011: {
				{Key: "material:excel:45301", Type: "material", TypeLabel: "物料", Name: "测试用原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 67.5, AmountPerKg: 67.5, Unit: "kg"},
			},
			453022: {
				{Key: "material:excel:45302", Type: "material", TypeLabel: "物料", Name: "孟连红果厌氧慢速日晒原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 131.25, AmountPerKg: 131.25, Unit: "kg"},
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
			BomVersionID:       5501,
			GreenBeanCostPerKg: 90,
			OperationCostPerKg: 10,
			YieldRate:          1,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:550", Type: "material", TypeLabel: "物料", Name: "固定加价原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 90, AmountPerKg: 90, Unit: "kg"},
			{Key: "operation:550", Type: "operation", TypeLabel: "工序", Name: "固定加价工序", ConsumeUnit: "per_kg", UnitCost: 10, AmountPerKg: 10, Unit: "kg"},
		},
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

func TestPricingRuleTrialRejectsUnresolvableQuoteUnit(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:          552,
			Name:               "PR507 无盒换算商品",
			InventoryUnit:      "kg",
			QuoteUnit:          "kg",
			UnitConversionJSON: "{}",
			GreenBeanCostPerKg: 20,
			OperationCostPerKg: 5,
			YieldRate:          1,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:552", Type: "material", TypeLabel: "物料", Name: "原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 20, AmountPerKg: 20, Unit: "kg"},
			{Key: "operation:552", Type: "operation", TypeLabel: "工序", Name: "包装", ConsumeUnit: "per_kg", UnitCost: 5, AmountPerKg: 5, Unit: "kg"},
		},
		pricingRules: map[int64]ProductPricingRule{
			12: {
				ID:             12,
				Name:           "PR507 单位校验",
				MarginRate:     0.2,
				TaxRate:        0,
				RoundingMode:   "none",
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

	_, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 12,
		ProductID:     552,
		QuoteUnit:     "盒",
	})
	if err == nil {
		t.Fatal("expected missing unit conversion error")
	}
	if !strings.Contains(err.Error(), "销售单位") || !strings.Contains(err.Error(), "单位换算") {
		t.Fatalf("error = %v, want sales unit conversion message", err)
	}
}

func TestPricingRuleTrialScalesBomAndOperationCostsByQuoteUnit(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:          573,
			Name:               "榛巧拼配 227g袋装",
			InventoryUnit:      "kg",
			QuoteUnit:          "227g袋装",
			UnitConversionJSON: `{"227g袋装":{"kg":0.227}}`,
			GreenBeanCostPerKg: 50,
			OperationCostPerKg: 10,
			YieldRate:          1,
			ExpectedLossRate:   0,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:573", Type: "material", TypeLabel: "物料", Name: "熟豆原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 50, Amount: 50, Unit: "kg"},
			{Key: "operation:573", Type: "operation", TypeLabel: "工序", Name: "包装", ConsumeUnit: "per_kg", UnitCost: 10, Amount: 10, Unit: "kg"},
		},
		pricingRules: map[int64]ProductPricingRule{
			13: {
				ID:             13,
				Name:           "PR508 规格单位成本折算",
				MarginRate:     0,
				TaxRate:        0,
				RoundingMode:   "none",
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
		PricingRuleID: 13,
		ProductID:     573,
		QuoteUnit:     "227g袋装",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.QuoteUnit != "227g袋装" {
		t.Fatalf("quote unit = %q, want 227g袋装", got.QuoteUnit)
	}
	if got.BomCostTotal != 11.35 || got.OperationCostTotal != 2.27 || got.BaseCost != 13.62 {
		t.Fatalf("base costs = bom %.2f operation %.2f total %.2f, want 11.35 + 2.27 = 13.62", got.BomCostTotal, got.OperationCostTotal, got.BaseCost)
	}
	if got.FinalUnitPrice != 13.62 {
		t.Fatalf("final price = %.2f, want 13.62", got.FinalUnitPrice)
	}
	if len(got.BaseCostDetails) != 2 {
		t.Fatalf("base cost details = %+v, want 2 rows", got.BaseCostDetails)
	}
	if got.BaseCostDetails[0].Amount != 11.35 || got.BaseCostDetails[0].Unit != "227g袋装" {
		t.Fatalf("material detail = %+v, want 11.35/227g袋装", got.BaseCostDetails[0])
	}
	if got.BaseCostDetails[1].Amount != 2.27 || got.BaseCostDetails[1].Unit != "227g袋装" {
		t.Fatalf("operation detail = %+v, want 2.27/227g袋装", got.BaseCostDetails[1])
	}
	if !sliceContains(got.FormulaExpressionLines, "最终售价 = 13.62/227g袋装") {
		t.Fatalf("formula lines = %+v, want final price in quote unit", got.FormulaExpressionLines)
	}
}

func TestPricingRuleTrialScalesPerUnitBomCostsBetweenQuoteUnits(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:          574,
			Name:               "榛巧拼配 227g袋装",
			InventoryUnit:      "g",
			QuoteUnit:          "袋",
			OrderUnit:          "袋",
			UnitConversionJSON: `{"袋":{"g":227}}`,
			NetContentQty:      227,
			NetContentUnit:     "g",
			YieldRate:          1,
			ExpectedLossRate:   0,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:574", Type: "material", TypeLabel: "物料", Name: "熟豆原料", ConsumeUnit: "g", Quantity: 227, UnitCost: 50, AmountPerUnit: 11.35},
		},
		pricingRules: map[int64]ProductPricingRule{
			14: {
				ID:             14,
				Name:           "PR508 每袋成本折算",
				MarginRate:     0,
				TaxRate:        0,
				RoundingMode:   "none",
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

	bag, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 14,
		ProductID:     574,
		QuoteUnit:     "袋",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial(袋) error = %v", err)
	}
	kg, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 14,
		ProductID:     574,
		QuoteUnit:     "kg",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial(kg) error = %v", err)
	}
	if bag.BaseCost != 11.35 || bag.BomCostTotal != 11.35 || bag.FinalUnitPrice != 11.35 {
		t.Fatalf("bag trial = base %.2f bom %.2f final %.2f, want 11.35", bag.BaseCost, bag.BomCostTotal, bag.FinalUnitPrice)
	}
	if kg.BaseCost != 50 || kg.BomCostTotal != 50 || kg.FinalUnitPrice != 50 {
		t.Fatalf("kg trial = base %.2f bom %.2f final %.2f, want 50.00", kg.BaseCost, kg.BomCostTotal, kg.FinalUnitPrice)
	}
	if len(kg.OtherCostDetails) != 0 {
		t.Fatalf("kg other cost details = %+v, want empty when no other costs configured", kg.OtherCostDetails)
	}
	if len(kg.BaseCostDetails) != 1 || kg.BaseCostDetails[0].Amount != 50 || kg.BaseCostDetails[0].Unit != "kg" {
		t.Fatalf("kg detail = %+v, want 50/kg", kg.BaseCostDetails)
	}
}

func TestPricingRuleTrialScalesPerUnitCostsForDerivedBoxQuoteUnit(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:          576,
			Name:               "初晓挂耳 盒（10袋）",
			InventoryUnit:      "袋",
			QuoteUnit:          "盒（10袋）",
			OrderUnit:          "盒（10袋）",
			UnitConversionJSON: `{"盒（10袋）":{"袋":10}}`,
			YieldRate:          1,
			ExpectedLossRate:   0,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:576", Type: "material", TypeLabel: "物料", Name: "挂耳物料", ConsumeUnit: "per_unit", UnitCost: 1.10, AmountPerUnit: 1.10, Unit: "袋"},
			{Key: "operation:576", Type: "operation", TypeLabel: "工序", Name: "挂耳包装", ConsumeUnit: "per_inventory_unit", UnitCost: 0.24, AmountPerUnit: 0.24, Unit: "袋"},
		},
		pricingRules: map[int64]ProductPricingRule{
			15: {
				ID:             15,
				Name:           "派生盒装成本折算",
				MarginRate:     0,
				TaxRate:        0,
				RoundingMode:   "none",
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

	bag, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 15,
		ProductID:     576,
		QuoteUnit:     "袋",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial(袋) error = %v", err)
	}
	box, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 15,
		ProductID:     576,
		QuoteUnit:     "盒（10袋）",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial(盒（10袋）) error = %v", err)
	}
	if bag.BaseCost != 1.34 || bag.BomCostTotal != 1.10 || bag.OperationCostTotal != 0.24 {
		t.Fatalf("bag trial = base %.2f bom %.2f operation %.2f, want 1.34 = 1.10 + 0.24", bag.BaseCost, bag.BomCostTotal, bag.OperationCostTotal)
	}
	if box.BaseCost != 13.40 || box.BomCostTotal != 11.00 || box.OperationCostTotal != 2.40 {
		t.Fatalf("box trial = base %.2f bom %.2f operation %.2f, want 13.40 = 11.00 + 2.40", box.BaseCost, box.BomCostTotal, box.OperationCostTotal)
	}
	if len(box.BaseCostDetails) != 2 || box.BaseCostDetails[0].Unit != "盒（10袋）" || box.BaseCostDetails[1].Unit != "盒（10袋）" {
		t.Fatalf("box details = %+v, want both rows converted to 盒（10袋）", box.BaseCostDetails)
	}
}

func TestPricingRuleTrialPreservesMaterialCompositionAndCostUnitWhenQuoteUnitChanges(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:          575,
			Name:               "曜石2.0 磅装",
			InventoryUnit:      "kg",
			QuoteUnit:          "lb",
			OrderUnit:          "lb",
			UnitConversionJSON: `{"lb":{"kg":0.45359237}}`,
			YieldRate:          1,
			ExpectedLossRate:   0,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:575", Type: "material", TypeLabel: "物料", Name: "卡蒂姆红酒日晒", ConsumeUnit: "ratio_pct", RatioPct: 12.5, RecipeRatioPct: 10, EffectiveRatioPct: 12.5, MaterialLossRate: 0.2, UnitCost: 67, AmountPerKg: 8.375, Unit: "kg"},
		},
		pricingRules: map[int64]ProductPricingRule{
			15: {
				ID:             15,
				Name:           "PR512 物料单位展示",
				MarginRate:     0,
				TaxRate:        0,
				RoundingMode:   "none",
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
		PricingRuleID: 15,
		ProductID:     575,
		QuoteUnit:     "lb",
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial(lb) error = %v", err)
	}
	if len(got.BaseCostDetails) != 1 {
		t.Fatalf("base cost details = %+v, want one material row", got.BaseCostDetails)
	}
	row := got.BaseCostDetails[0]
	if row.Unit != "lb" || math.Abs(row.Amount-3.8) > 0.005 {
		t.Fatalf("quote amount = %.4f/%s, want 3.8/lb", row.Amount, row.Unit)
	}
	if row.UnitCost == 67 {
		t.Fatalf("unit_cost should remain the compatibility quote-unit value, got %.4f", row.UnitCost)
	}
	if got.BomCostTotal != row.Amount || got.BaseCost != row.Amount || got.FinalUnitPrice != row.Amount {
		t.Fatalf("trial totals = base %.4f bom %.4f final %.4f row %.4f, want quote-unit totals", got.BaseCost, got.BomCostTotal, got.FinalUnitPrice, row.Amount)
	}
	if row.RatioPct != 12.5 {
		t.Fatalf("compat ratio pct = %.4f, want effective 12.5", row.RatioPct)
	}
	if row.RecipeRatioPct != 10 {
		t.Fatalf("recipe ratio pct = %.4f, want original BOM composition 10", row.RecipeRatioPct)
	}
	if row.EffectiveRatioPct != 12.5 {
		t.Fatalf("effective ratio pct = %.4f, want loss-adjusted 12.5", row.EffectiveRatioPct)
	}
	if row.CostUnit != "kg" {
		t.Fatalf("cost unit = %q, want kg", row.CostUnit)
	}
	if row.CostUnitCost != 67 {
		t.Fatalf("cost unit cost = %.4f, want 67/kg", row.CostUnitCost)
	}
	if !strings.Contains(row.Description, "原比例 10%") || !strings.Contains(row.Description, "有效比例 12.5%") || !strings.Contains(row.Description, "单位成本 67/kg") || !strings.Contains(row.Description, "折算金额") || !strings.Contains(row.Description, "/lb") {
		t.Fatalf("description = %q, want material cost unit and quote amount", row.Description)
	}
}

func TestPricingRuleTrialSupportsMarkupTaxExcludedAndYuanRounding(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:          551,
			Name:               "PR452 加价未税商品",
			InventoryUnit:      "kg",
			QuoteUnit:          "kg",
			BomVersionID:       5511,
			GreenBeanCostPerKg: 30,
			OperationCostPerKg: 5,
			YieldRate:          1,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:551", Type: "material", TypeLabel: "物料", Name: "未税加价原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 30, AmountPerKg: 30, Unit: "kg"},
			{Key: "operation:551", Type: "operation", TypeLabel: "工序", Name: "未税加价工序", ConsumeUnit: "per_kg", UnitCost: 5, AmountPerKg: 5, Unit: "kg"},
		},
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
	if len(got.OtherCostDetails) != 1 || got.OtherCostDetails[0].Name != "包装" || got.OtherCostDetails[0].Amount != 4 || got.OtherCostDetails[0].Source != "pricing_rule" {
		t.Fatalf("other cost details = %+v, want pricing rule packaging row", got.OtherCostDetails)
	}
	if got.PreTaxPrice != 46.8 || got.TaxAmount != 4.68 || got.FinalUnitPrice != 47 {
		t.Fatalf("trial prices = preTax %.2f tax %.2f final %.2f", got.PreTaxPrice, got.TaxAmount, got.FinalUnitPrice)
	}
	if got.ProfitExplanation.Method != "markup" || got.ProfitExplanation.MethodLabel != "加价率" || got.ProfitExplanation.Rate != 0.2 {
		t.Fatalf("profit explanation = %+v, want markup 20%%", got.ProfitExplanation)
	}
	if !strings.Contains(got.ProfitExplanation.Formula, "损耗后成本 * (1 + 加价率 20%)") {
		t.Fatalf("profit explanation formula = %q, want markup formula", got.ProfitExplanation.Formula)
	}
	if got.TaxInPriceAmount != 0 || got.FinalBeforeRounding != 46.8 || got.RoundingAdjustment != 0.2 {
		t.Fatalf("tax excluded waterfall = taxInPrice %.2f finalBefore %.2f rounding %.2f", got.TaxInPriceAmount, got.FinalBeforeRounding, got.RoundingAdjustment)
	}
	waterfallTotal := got.CostBaseTotal + got.YieldLossAmount + got.ProfitMarkupAmount + got.TaxInPriceAmount + got.RoundingAdjustment
	if math.Abs(waterfallTotal-got.FinalUnitPrice) > 0.001 {
		t.Fatalf("waterfall total %.4f must equal final unit price %.4f", waterfallTotal, got.FinalUnitPrice)
	}
}

func TestPricingRuleTrialExplanationFieldsUseTemporaryOtherCostsAndFixedAdd(t *testing.T) {
	repo := &fakeRepo{
		inputs: []domain.ProductInput{{
			ProductID:     552,
			Name:          "PR510 固定加价商品",
			InventoryUnit: "kg",
			QuoteUnit:     "kg",
			BomVersionID:  5521,
			YieldRate:     1,
		}},
		costDetails: []PricingRuleTrialBaseCostDetail{
			{Key: "material:552", Type: "material", TypeLabel: "物料", Name: "固定加价原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 20, AmountPerKg: 20, Unit: "kg"},
		},
		pricingRules: map[int64]ProductPricingRule{
			13: {
				ID:           13,
				Name:         "PR510 固定加价",
				MarginRate:   3,
				TaxRate:      0,
				RoundingMode: "none",
				Active:       true,
				CalculationJSON: map[string]any{
					"yield_loss_mode": "none",
					"profit_method":   "fixed_add",
					"tax_mode":        "none",
					"other_costs": map[string]any{
						"模板包装": 1,
					},
				},
			},
		},
	}
	got, err := NewService(repo).PricingRuleTrial(context.Background(), PricingRuleTrialCommand{
		PricingRuleID: 13,
		ProductID:     552,
		Overrides: PricingRuleTrialOverrides{
			OtherCosts: map[string]float64{
				"临时包装": 2,
				"临时贴标": 0.5,
			},
			MarginRate: floatPtr(4),
		},
	})
	if err != nil {
		t.Fatalf("PricingRuleTrial() error = %v", err)
	}
	if got.OtherCostTotal != 2.5 || len(got.OtherCostDetails) != 2 {
		t.Fatalf("other costs = total %.2f details %+v, want temporary rows", got.OtherCostTotal, got.OtherCostDetails)
	}
	if got.OtherCostDetails[0].Name != "临时包装" || got.OtherCostDetails[1].Name != "临时贴标" {
		t.Fatalf("other cost details order = %+v, want stable name order", got.OtherCostDetails)
	}
	for _, row := range got.OtherCostDetails {
		if row.Source != "temporary_override" || !strings.Contains(row.SettingLocation, "本次试算抽屉") {
			t.Fatalf("other cost detail = %+v, want temporary source and trial drawer location", row)
		}
	}
	if got.ProfitExplanation.Method != "fixed_add" || got.ProfitExplanation.MethodLabel != "固定加价" || got.ProfitExplanation.Rate != 4 || got.ProfitExplanation.Source != "temporary_override" {
		t.Fatalf("profit explanation = %+v, want fixed add temporary override", got.ProfitExplanation)
	}
	if got.ProfitExplanation.CostAfterYield != 22.5 || got.ProfitExplanation.MarkupAmount != 4 || got.ProfitExplanation.PreTaxPrice != 26.5 {
		t.Fatalf("profit explanation amounts = %+v, want fixed add waterfall amounts", got.ProfitExplanation)
	}
	if !strings.Contains(got.ProfitExplanation.Formula, "损耗后成本 + 固定加价 4/kg") {
		t.Fatalf("profit explanation formula = %q, want fixed add formula", got.ProfitExplanation.Formula)
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

func pricingRuleTrialHasStepSource(steps []domain.PriceExplanationStep, key string, source string) bool {
	for _, step := range steps {
		if step.Key == key && step.Source == source {
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

func TestPublishBeanListAutoIncrementsDuplicatePublicationVersion(t *testing.T) {
	repo := &fakeRepo{beanListPublications: []BeanListPublication{
		{ID: 76, ListType: "commercial", ProductTypeCategoryID: 12, Version: "V3.0.5", Status: "published", OwnerType: "official"},
		{ID: 78, ListType: "commercial", ProductTypeCategoryID: 12, Version: "V3.0.6", Status: "published", OwnerType: "official"},
	}}
	svc := NewService(repo)

	row, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{
		ListType:              "commercial",
		ProductTypeCategoryID: 12,
		ProductTypeName:       "熟豆",
		Version:               "V3.0.6",
	})
	if err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}

	if row.Version != "V3.0.7" || repo.publishedBeanList.Version != "V3.0.7" {
		t.Fatalf("published version = row %q cmd %q, want V3.0.7", row.Version, repo.publishedBeanList.Version)
	}
	if repo.lastBeanListQuery.ProductTypeCategoryID != 12 || repo.lastBeanListQuery.ListType != "commercial" || repo.lastBeanListQuery.OwnerType != "official" {
		t.Fatalf("version lookup query = %+v", repo.lastBeanListQuery)
	}
}

func TestPublishBeanListKeepsStandardVersionAheadOfSmokeVersions(t *testing.T) {
	repo := &fakeRepo{beanListPublications: []BeanListPublication{
		{ID: 78, ListType: "commercial", ProductTypeCategoryID: 12, Version: "V3.0.6", Status: "published", OwnerType: "official"},
		{ID: 79, ListType: "commercial", ProductTypeCategoryID: 12, Version: "codex-smoke-brand", Status: "published", OwnerType: "official"},
	}}
	svc := NewService(repo)

	row, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{
		ListType:              "commercial",
		ProductTypeCategoryID: 12,
		ProductTypeName:       "熟豆",
		Version:               "V3.0.6",
	})
	if err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}

	if row.Version != "V3.0.7" || repo.publishedBeanList.Version != "V3.0.7" {
		t.Fatalf("published version = row %q cmd %q, want V3.0.7", row.Version, repo.publishedBeanList.Version)
	}
}

func TestArchiveBeanListPublicationsValidatesIDsAndOwner(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if err := svc.ArchiveBeanListPublications(context.Background(), ArchiveBeanListPublicationsCommand{}); err == nil {
		t.Fatalf("expected ids required")
	}
	err := svc.ArchiveBeanListPublications(context.Background(), ArchiveBeanListPublicationsCommand{
		IDs:                []int64{7, 8},
		PublicationPurpose: BeanListPublicationPurposeFactorySupply,
		OwnerType:          "customer",
		OwnerKey:           "42",
		Actor:              "tester",
	})
	if err != nil {
		t.Fatalf("ArchiveBeanListPublications() error = %v", err)
	}
	if !reflect.DeepEqual(repo.archivedBeanLists.IDs, []int64{7, 8}) || repo.archivedBeanLists.OwnerType != "customer" || repo.archivedBeanLists.OwnerKey != "42" || repo.archivedBeanLists.Actor != "tester" {
		t.Fatalf("archive command = %+v", repo.archivedBeanLists)
	}

	err = svc.UnarchiveBeanListPublications(context.Background(), ArchiveBeanListPublicationsCommand{
		IDs:                []int64{7},
		PublicationPurpose: BeanListPublicationPurposeFactorySupply,
		OwnerType:          "customer",
		OwnerKey:           "42",
		Actor:              "tester",
	})
	if err != nil {
		t.Fatalf("UnarchiveBeanListPublications() error = %v", err)
	}
	if !reflect.DeepEqual(repo.unarchivedBeanLists.IDs, []int64{7}) || repo.unarchivedBeanLists.OwnerType != "customer" || repo.unarchivedBeanLists.OwnerKey != "42" {
		t.Fatalf("unarchive command = %+v", repo.unarchivedBeanLists)
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
	} else if !strings.Contains(err.Error(), "旧价格档") || strings.Contains(err.Error(), "来源价格记录") {
		t.Fatalf("legacy price snapshot error should explain the business fix, got %q", err.Error())
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

func TestPublishBeanListUsesFlatPriceRowsInsteadOfLegacySourceRecordForPR440Snapshots(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	content := map[string]any{
		"price_rows": []any{
			map[string]any{
				"product_id":                float64(414),
				"product_name":              "曲奇拼配",
				"tier_label":                "基础价",
				"min_qty":                   float64(0),
				"final_unit_price":          float64(88),
				"price_unit":                "kg",
				"currency":                  "CNY",
				"inventory_unit":            "kg",
				"inventory_conversion_json": map[string]any{"kg": map[string]any{"kg": float64(1)}},
				"group_snapshot":            map[string]any{"group_id": float64(3), "group_name": "商品价格表分组", "group_item_id": float64(101), "group_item_name": "大客户"},
				"group_source":              "product_catalog",
				"pricing_mode":              "pricing_rule",
				"pricing_mode_source":       "product",
				"pricing_rule_id":           float64(90),
				"pricing_rule_source":       "product",
				"pricing_rule_version":      "PR-COST/v3",
				"cost_source_snapshot":      map[string]any{"bom_version_no": "BOM-A1/V002", "process_route_name": "标准烘焙"},
				"customer_reference_snapshot": map[string]any{
					"customer_id":           float64(5),
					"customer_display_name": "Karen 拼配",
				},
				"manual_adjusted": false,
			},
		},
		"groups": []any{
			map[string]any{
				"items": []any{
					map[string]any{
						"productId": float64(414),
						"name":      "曲奇拼配",
						"commercial_wholesale_tiers": []any{
							map[string]any{
								"label":                     "基础价",
								"min_qty":                   float64(0),
								"price_per_unit":            float64(88),
								"final_unit_price":          float64(88),
								"price_unit":                "kg",
								"currency":                  "CNY",
								"inventory_unit":            "kg",
								"inventory_conversion_json": map[string]any{"kg": map[string]any{"kg": float64(1)}},
								"pricing_mode":              "pricing_rule",
								"pricing_rule_id":           float64(90),
								"pricing_rule_source":       "product",
								"pricing_rule_version":      "PR-COST/v3",
							},
						},
					},
				},
			},
		},
	}

	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.0.2", Content: content}); err != nil {
		t.Fatalf("PublishBeanList() with PR-440 flat price row snapshot error = %v", err)
	}
}

func TestPublishBeanListRewritesFlatRowUnitSnapshotFromProductMaster(t *testing.T) {
	repo := &fakeRepo{
		productUnitRules: map[int64]ProductSalesUnitRule{
			414: {
				ProductID:     414,
				InventoryUnit: "kg",
				Conversion: map[string]map[string]float64{
					"盒": {"kg": 0.2},
				},
			},
		},
	}
	svc := NewService(repo)
	row := map[string]any{
		"product_id":                float64(414),
		"sku_id":                    float64(514),
		"sku_name":                  "100g袋装",
		"sku_code":                  "SKU-000514",
		"product_name":              "盒装速溶",
		"tier_label":                "基础价",
		"min_qty":                   float64(0),
		"final_unit_price":          float64(18),
		"price_unit":                "盒",
		"currency":                  "CNY",
		"inventory_unit":            "g",
		"inventory_conversion_json": map[string]any{"盒": map[string]any{"g": float64(999)}},
		"group_snapshot":            map[string]any{"group_id": float64(3), "group_name": "商品价格表分组", "group_item_id": float64(101), "group_item_name": "盒装"},
		"group_source":              "product_catalog",
		"pricing_mode":              "pricing_rule",
		"pricing_mode_source":       "product",
		"pricing_rule_id":           float64(90),
		"pricing_rule_source":       "product",
		"pricing_rule_version":      "PR-COST/v3",
		"cost_source_snapshot":      map[string]any{"bom_version_no": "BOM-A1/V002"},
		"customer_reference_snapshot": map[string]any{
			"customer_id":           float64(5),
			"customer_display_name": "Karen 盒装",
		},
		"manual_adjusted": false,
	}
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.0.3", Content: map[string]any{"price_rows": []any{row}}}); err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}
	got := repo.publishedBeanList.Content["price_rows"].([]any)[0].(map[string]any)
	if got["inventory_unit"] != "kg" {
		t.Fatalf("inventory_unit = %#v, want kg", got["inventory_unit"])
	}
	conversion := got["inventory_conversion_json"].(map[string]any)
	if conversion["盒"].(map[string]any)["kg"] != float64(0.2) {
		t.Fatalf("inventory_conversion_json = %#v, want product master conversion", conversion)
	}

	row["price_unit"] = "袋"
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.0.4", Content: map[string]any{"price_rows": []any{row}}}); err == nil {
		t.Fatalf("expected missing product UOM conversion error")
	} else {
		for _, want := range []string{
			"商品档案缺少价格单位到库存单位换算：第1行",
			"商品：盒装速溶",
			"SKU：100g袋装（SKU-000514）",
			"价格单位：袋",
			"库存单位：kg",
			"销售规格模板",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("missing error detail %q in %q", want, err.Error())
			}
		}
	}
}

func TestPublishBeanListSnapshotsSkuIdentityForFlatRows(t *testing.T) {
	repo := &fakeRepo{
		productUnitRules: map[int64]ProductSalesUnitRule{
			414: {
				ProductID:     414,
				InventoryUnit: "袋",
				Conversion: map[string]map[string]float64{
					"袋": {"袋": 1},
				},
			},
		},
	}
	svc := NewService(repo)
	row := map[string]any{
		"product_id":                float64(414),
		"parent_product_id":         float64(88),
		"product_name":              "埃塞俄比亚 水洗 227g袋装",
		"sku_name":                  "227g袋装",
		"sku_code":                  "ETH-227",
		"spec_label":                "227g",
		"tier_label":                "基础价",
		"final_unit_price":          float64(36),
		"price_unit":                "袋",
		"inventory_unit":            "袋",
		"inventory_conversion_json": map[string]any{"袋": map[string]any{"袋": float64(1)}},
		"group_snapshot":            map[string]any{"group_id": float64(3), "group_name": "商品价格表分组", "group_item_id": float64(101), "group_item_name": "袋装"},
		"group_source":              "product_catalog",
		"pricing_mode":              "pricing_rule",
		"pricing_mode_source":       "product",
		"pricing_rule_id":           float64(90),
		"pricing_rule_source":       "product",
		"pricing_rule_version":      "PR-COST/v3",
		"cost_source_snapshot":      map[string]any{"bom_version_no": "BOM-SKU/V001"},
		"customer_reference_snapshot": map[string]any{
			"customer_id":           float64(5),
			"customer_display_name": "Karen 227g袋装",
		},
		"manual_adjusted": false,
	}
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.0.6", Content: map[string]any{"price_rows": []any{row}}}); err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}
	got := repo.publishedBeanList.Content["price_rows"].([]any)[0].(map[string]any)
	if got["sku_id"] != float64(414) || got["parent_product_id"] != float64(88) {
		t.Fatalf("sku identity snapshot = %#v", got)
	}
	snapshot, ok := got["sku_snapshot"].(map[string]any)
	if !ok || snapshot["sku_name"] != "227g袋装" || snapshot["sku_code"] != "ETH-227" || snapshot["spec_label"] != "227g" {
		t.Fatalf("sku_snapshot = %#v", got["sku_snapshot"])
	}
}

func TestPublishBeanListResolvesDerivedSKUUnitSnapshotsBySKUIdentity(t *testing.T) {
	repo := &fakeRepo{
		productUnitRules: map[int64]ProductSalesUnitRule{
			1001: {
				ProductID:     1001,
				InventoryUnit: "kg",
				Conversion: map[string]map[string]float64{
					"227g": {"kg": 0.227},
				},
			},
			1002: {
				ProductID:     1002,
				InventoryUnit: "袋",
				Conversion: map[string]map[string]float64{
					"盒": {"袋": 10},
				},
			},
			1003: {
				ProductID:     1003,
				InventoryUnit: "袋",
				Conversion: map[string]map[string]float64{
					"袋": {"袋": 1},
				},
			},
		},
	}
	svc := NewService(repo)
	row := func(parentID, skuID int64, productName, priceUnit string) map[string]any {
		return map[string]any{
			"product_id":        float64(parentID),
			"sku_id":            float64(skuID),
			"parent_product_id": float64(parentID),
			"product_name":      productName,
			"sku_name":          priceUnit,
			"tier_label":        "基础价",
			"final_unit_price":  float64(36),
			"price_unit":        priceUnit,
			"currency":          "CNY",
			"group_snapshot": map[string]any{
				"group_id": float64(3), "group_name": "商品价格表分组",
				"group_item_id": float64(101), "group_item_name": "派生规格",
			},
			"group_source":         "product_catalog",
			"pricing_mode":         "pricing_rule",
			"pricing_mode_source":  "product",
			"pricing_rule_id":      float64(90),
			"pricing_rule_source":  "product",
			"pricing_rule_version": "PR-COST/v3",
			"cost_source_snapshot": map[string]any{"bom_version_no": "BOM-DERIVED/V001"},
			"customer_reference_snapshot": map[string]any{
				"customer_id": float64(5),
			},
			"manual_adjusted": false,
		}
	}
	rows := []any{
		row(539, 1001, "熟豆 227g", "227g"),
		row(640, 1002, "挂耳 盒", "盒"),
		row(640, 1003, "挂耳 袋", "袋"),
	}
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{
		ListType: "commercial",
		Version:  "V4.0.7",
		Content:  map[string]any{"price_rows": rows},
	}); err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}

	gotRows := repo.publishedBeanList.Content["price_rows"].([]any)
	assertConversion := func(index int, priceUnit, inventoryUnit string, want float64) {
		t.Helper()
		got := gotRows[index].(map[string]any)
		if got["inventory_unit"] != inventoryUnit {
			t.Fatalf("row %d inventory_unit = %#v, want %q", index+1, got["inventory_unit"], inventoryUnit)
		}
		conversion := got["inventory_conversion_json"].(map[string]any)
		if value := conversion[priceUnit].(map[string]any)[inventoryUnit]; value != want {
			t.Fatalf("row %d conversion = %#v, want %s -> %s = %v", index+1, conversion, priceUnit, inventoryUnit, want)
		}
	}
	assertConversion(0, "227g", "kg", 0.227)
	assertConversion(1, "盒", "袋", 10)
	assertConversion(2, "袋", "袋", 1)
}

func TestPublishBeanListUsesCustomerAliasUnitRuleWhenPresent(t *testing.T) {
	repo := &fakeRepo{
		productUnitRules: map[int64]ProductSalesUnitRule{
			414: {
				ProductID:     414,
				InventoryUnit: "kg",
				Conversion: map[string]map[string]float64{
					"盒": {"kg": 0.2},
				},
			},
		},
		customerUnitRules: map[int64]ProductSalesUnitRule{
			701: {
				ProductID:     414,
				InventoryUnit: "条",
				Conversion: map[string]map[string]float64{
					"盒": {"条": 10},
				},
			},
		},
	}
	svc := NewService(repo)
	row := map[string]any{
		"product_id":                float64(414),
		"customer_product_alias_id": float64(701),
		"product_name":              "客户盒装速溶",
		"tier_label":                "基础价",
		"final_unit_price":          float64(18),
		"price_unit":                "盒",
		"currency":                  "CNY",
		"inventory_unit":            "kg",
		"inventory_conversion_json": map[string]any{"盒": map[string]any{"kg": float64(0.2)}},
		"group_snapshot":            map[string]any{"group_id": float64(3), "group_name": "客户商品价格表分组", "group_item_id": float64(101), "group_item_name": "盒装"},
		"group_source":              "customer_product_alias",
		"pricing_mode":              "pricing_rule",
		"pricing_mode_source":       "product",
		"pricing_rule_id":           float64(90),
		"pricing_rule_source":       "product",
		"pricing_rule_version":      "PR-COST/v3",
		"cost_source_snapshot":      map[string]any{"bom_version_no": "BOM-A1/V002"},
		"manual_adjusted":           false,
		"customer_reference_snapshot": map[string]any{
			"customer_product_alias_id": float64(701),
			"customer_id":               float64(5),
			"customer_display_name":     "Karen 盒装",
		},
	}
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V4.0.5", Content: map[string]any{"price_rows": []any{row}}}); err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}
	if repo.lastCustomerAliasID != 701 {
		t.Fatalf("customer alias resolver id = %d, want 701", repo.lastCustomerAliasID)
	}
	got := repo.publishedBeanList.Content["price_rows"].([]any)[0].(map[string]any)
	if got["inventory_unit"] != "条" {
		t.Fatalf("inventory_unit = %#v, want customer alias inventory unit 条", got["inventory_unit"])
	}
	conversion := got["inventory_conversion_json"].(map[string]any)
	if conversion["盒"].(map[string]any)["条"] != float64(10) {
		t.Fatalf("inventory_conversion_json = %#v, want customer alias conversion", conversion)
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

func TestPublishBeanListDoesNotBlockRetiredStandardCostDefaultCapacityWarning(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	row := map[string]any{
		"product_id":                float64(414),
		"product_name":              "曲奇拼配",
		"tier_label":                "基础价",
		"min_qty":                   float64(0),
		"final_unit_price":          float64(88),
		"price_unit":                "kg",
		"currency":                  "CNY",
		"inventory_unit":            "kg",
		"inventory_conversion_json": map[string]any{"kg": map[string]any{"kg": float64(1)}},
		"group_snapshot":            map[string]any{"group_id": float64(3), "group_name": "商品价格表分组"},
		"group_source":              PriceListGroupSourceProductCatalog,
		"pricing_mode":              "pricing_rule",
		"pricing_mode_source":       "product",
		"pricing_rule_id":           float64(90),
		"pricing_rule_source":       "product",
		"pricing_rule_version":      "PR-COST/v3",
		"cost_source_snapshot": map[string]any{
			"pricing_rule_trial_warnings": []any{"请为工艺路线工序设置标准成本默认产能"},
			"pricing_rule_trial_base_cost_details": []any{
				map[string]any{"type": "operation", "capacity_selection_source": "missing_default"},
			},
		},
		"customer_reference_snapshot": map[string]any{},
		"manual_adjusted":             false,
	}
	_, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{
		ListType: "commercial",
		Version:  "V4.1.2",
		Content:  map[string]any{"price_rows": []any{row}},
	})
	if err != nil {
		t.Fatalf("PublishBeanList() error = %v, should not block retired standard cost default capacity warning", err)
	}
}

func TestPublishBeanListBlocksMissingBomOperationCostSnapshot(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	row := map[string]any{
		"product_id":                float64(414),
		"product_name":              "曲奇拼配",
		"tier_label":                "基础价",
		"min_qty":                   float64(0),
		"final_unit_price":          float64(88),
		"price_unit":                "kg",
		"currency":                  "CNY",
		"inventory_unit":            "kg",
		"inventory_conversion_json": map[string]any{"kg": map[string]any{"kg": float64(1)}},
		"group_snapshot":            map[string]any{"group_id": float64(3), "group_name": "商品价格表分组"},
		"group_source":              PriceListGroupSourceProductCatalog,
		"pricing_mode":              "pricing_rule",
		"pricing_mode_source":       "product",
		"pricing_rule_id":           float64(90),
		"pricing_rule_source":       "product",
		"pricing_rule_version":      "PR-COST/v3",
		"cost_source_snapshot": map[string]any{
			"pricing_rule_trial_warnings": []any{"请先发布包含标准成本产能档快照的 BOM"},
			"pricing_rule_trial_base_cost_details": []any{
				map[string]any{
					"type":                      "operation",
					"name":                      "BOM工序成本快照缺失",
					"capacity_selection_source": "bom_operation_snapshot_missing",
				},
			},
		},
		"customer_reference_snapshot": map[string]any{},
		"manual_adjusted":             false,
	}

	_, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{
		ListType: "commercial",
		Version:  "V4.1.3",
		Content:  map[string]any{"price_rows": []any{row}},
	})
	if err == nil {
		t.Fatalf("expected publish to reject rows with missing BOM operation cost snapshot")
	}
	if !strings.Contains(err.Error(), "请先发布包含标准成本产能档快照的 BOM") {
		t.Fatalf("publish error = %q, want BOM operation snapshot guidance", err.Error())
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
