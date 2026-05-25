package catalog

import (
	"context"
	"testing"
)

type fakeRepo struct {
	replace         ReplacePriceTiersCommand
	update          UpdateProductBasicsCommand
	create          CreateProductCommand
	custom          CreateCustomProductCommand
	derivedProduct  DeriveCustomerProductCommand
	derivedCategory DeriveProductCategoryCommand
	derivedTemplate DeriveGradientTemplateCommand
	derivedConfig   DeriveProductConfigTemplateCommand
	assigned        AssignProductCategoryCommand
	assignResult    AssignProductCategoryResult
	publicUsage     CustomerPublicUsageCommand
	ruleTemplate    SaveCustomerProductRuleTemplateCommand
	ruleOverride    SaveCustomerProductRuleOverrideCommand
	ruleBinding     CustomerProductRuleTemplateBindingCommand
	configTemplate  SaveProductConfigTemplateCommand
	unitDefinition  SaveProductUnitDefinitionCommand
	unitTemplate    SaveProductUnitTemplateCommand
	deactivate      DeactivateProductsCommand
	products        map[int64]Product
	deactivated     bool
	usageSaved      bool
}

func (r *fakeRepo) ListProducts(ctx context.Context) ([]Product, error) {
	return []Product{{ID: 1, Name: "A"}}, nil
}

func (r *fakeRepo) GetProduct(ctx context.Context, id int64) (*Product, error) {
	if r.products != nil {
		if product, ok := r.products[id]; ok {
			return &product, nil
		}
		return nil, nil
	}
	return &Product{ID: id, Name: "A"}, nil
}

func (r *fakeRepo) ReplacePriceTiers(ctx context.Context, cmd ReplacePriceTiersCommand) error {
	r.replace = cmd
	return nil
}

func (r *fakeRepo) UpdateProductBasics(ctx context.Context, cmd UpdateProductBasicsCommand) error {
	r.update = cmd
	return nil
}

func (r *fakeRepo) DeactivateProducts(ctx context.Context, cmd DeactivateProductsCommand) error {
	r.deactivate = cmd
	r.deactivated = true
	return nil
}

func (r *fakeRepo) CreateProduct(ctx context.Context, cmd CreateProductCommand) (Product, error) {
	r.create = cmd
	return Product{ID: 11, Name: cmd.Name, RoastLevel: cmd.RoastLevel, YieldRate: cmd.YieldRate, Visibility: "public"}, nil
}

func (r *fakeRepo) ListProductCategories(ctx context.Context) ([]ProductCategory, error) {
	return []ProductCategory{{ID: 1, Name: "咖啡豆", Level: 1, Position: 1}}, nil
}

func (r *fakeRepo) ListGradientTemplates(ctx context.Context) ([]GradientTemplate, error) {
	return nil, nil
}

func (r *fakeRepo) ListProductConfigTemplates(ctx context.Context) ([]ProductConfigTemplate, error) {
	return []ProductConfigTemplate{{
		ID:                  301,
		CustomerID:          0,
		Name:                "公共盒装配置",
		GradientTemplateID:  8,
		OperationTemplateID: 9,
		UnitTemplateID:      12,
		PriceListRuleJSON:   `{"pricing_mode":"inherit_gradient_template"}`,
		InventoryUnit:       "kg",
		QuoteUnit:           "盒",
		OrderUnit:           "盒",
		UnitConversionJSON:  `{"盒":{"kg":0.2}}`,
		IntegerUnit:         true,
		Active:              true,
	}}, nil
}

func (r *fakeRepo) ListProductUnitDefinitions(ctx context.Context) ([]ProductUnitDefinition, error) {
	return []ProductUnitDefinition{{Code: "kg", Name: "kg", UnitType: "weight", AllowDecimal: true, Active: true}, {Code: "盒", Name: "盒", UnitType: "package", Active: true}}, nil
}

func (r *fakeRepo) ListProductUnitTemplates(ctx context.Context) ([]ProductUnitTemplate, error) {
	return []ProductUnitTemplate{{ID: 12, Name: "盒装200g", InventoryUnit: "kg", QuoteUnit: "盒", OrderUnit: "盒", UnitConversionJSON: `{"盒":{"kg":0.2}}`, IntegerUnit: true, Active: true}}, nil
}

func (r *fakeRepo) SaveProductConfigTemplate(ctx context.Context, cmd SaveProductConfigTemplateCommand) (ProductConfigTemplate, error) {
	r.configTemplate = cmd
	return ProductConfigTemplate{
		ID:                  701,
		CustomerID:          cmd.CustomerID,
		Name:                cmd.Name,
		GradientTemplateID:  cmd.GradientTemplateID,
		OperationTemplateID: cmd.OperationTemplateID,
		UnitTemplateID:      cmd.UnitTemplateID,
		PriceListRuleJSON:   cmd.PriceListRuleJSON,
		InventoryUnit:       cmd.InventoryUnit,
		QuoteUnit:           cmd.QuoteUnit,
		OrderUnit:           cmd.OrderUnit,
		UnitConversionJSON:  cmd.UnitConversionJSON,
		IntegerUnit:         cmd.IntegerUnit,
		Active:              true,
	}, nil
}

func (r *fakeRepo) SaveProductUnitDefinition(ctx context.Context, cmd SaveProductUnitDefinitionCommand) (ProductUnitDefinition, error) {
	r.unitDefinition = cmd
	return ProductUnitDefinition{Code: cmd.Code, Name: cmd.Name, UnitType: cmd.UnitType, AllowDecimal: cmd.AllowDecimal, Active: true}, nil
}

func (r *fakeRepo) SaveProductUnitTemplate(ctx context.Context, cmd SaveProductUnitTemplateCommand) (ProductUnitTemplate, error) {
	r.unitTemplate = cmd
	return ProductUnitTemplate{ID: 12, Name: cmd.Name, InventoryUnit: cmd.InventoryUnit, QuoteUnit: cmd.QuoteUnit, OrderUnit: cmd.OrderUnit, UnitConversionJSON: cmd.UnitConversionJSON, IntegerUnit: cmd.IntegerUnit, Active: true}, nil
}

func (r *fakeRepo) DeriveProductConfigTemplate(ctx context.Context, cmd DeriveProductConfigTemplateCommand) (ProductConfigTemplate, error) {
	r.derivedConfig = cmd
	return ProductConfigTemplate{ID: 702, CustomerID: cmd.CustomerID, SourceTemplateID: cmd.SourceTemplateID, TemplateState: TemplateStateDerived, Name: cmd.Name, Active: true}, nil
}

func (r *fakeRepo) SaveGradientTemplate(ctx context.Context, cmd SaveGradientTemplateCommand) (GradientTemplate, error) {
	return GradientTemplate{ID: 1, Name: cmd.Name, DisplayUnit: cmd.DisplayUnit, Active: true, Tiers: cmd.Tiers}, nil
}

func (r *fakeRepo) DeactivateGradientTemplate(ctx context.Context, cmd DeactivateGradientTemplateCommand) error {
	return nil
}

func (r *fakeRepo) BindCategoryGradientTemplate(ctx context.Context, cmd BindCategoryGradientTemplateCommand) error {
	return nil
}

func (r *fakeRepo) SaveProductCategory(ctx context.Context, cmd SaveProductCategoryCommand) (ProductCategory, error) {
	return ProductCategory{ID: 2, Name: cmd.Name, ParentID: cmd.ParentID, CustomerID: cmd.CustomerID, Position: cmd.Position, ProductConfigTemplateID: cmd.ProductConfigTemplateID}, nil
}

func (r *fakeRepo) MoveProductCategory(ctx context.Context, cmd MoveProductCategoryCommand) error {
	return nil
}

func (r *fakeRepo) DeleteProductCategory(ctx context.Context, cmd DeleteProductCategoryCommand) error {
	return nil
}

func (r *fakeRepo) AssignProductCategory(ctx context.Context, cmd AssignProductCategoryCommand) (AssignProductCategoryResult, error) {
	r.assigned = cmd
	if r.assignResult.ProductID == 0 {
		r.assignResult = AssignProductCategoryResult{ProductID: cmd.ProductID, CategoryID: cmd.CategoryID}
	}
	return r.assignResult, nil
}

func (r *fakeRepo) CreateCustomProduct(ctx context.Context, cmd CreateCustomProductCommand) (Product, error) {
	r.custom = cmd
	return Product{ID: 10, Name: cmd.Name, ProductKind: cmd.ProductKind, GreenBeanType: cmd.GreenBeanType, GreenBeanBomProductID: cmd.GreenBeanBomProductID, CustomerID: cmd.CustomerID, BaseProductID: cmd.BaseProductID, Visibility: "customer_only", CustomType: cmd.CustomType}, nil
}

func (r *fakeRepo) DeriveCustomerProduct(ctx context.Context, cmd DeriveCustomerProductCommand) (Product, error) {
	r.derivedProduct = cmd
	return Product{ID: 420, Name: cmd.Name, CustomerID: cmd.CustomerID, BaseProductID: cmd.BaseProductID, Visibility: "customer_only", CustomType: "public_sku_alias", ProductCategoryID: cmd.CategoryID}, nil
}

func (r *fakeRepo) DeriveProductCategory(ctx context.Context, cmd DeriveProductCategoryCommand) (ProductCategory, error) {
	r.derivedCategory = cmd
	return ProductCategory{ID: 117, CustomerID: cmd.CustomerID, SourceCategoryID: cmd.SourceCategoryID, TemplateState: "derived_from_public", Name: "客户定制"}, nil
}

func (r *fakeRepo) DeriveGradientTemplate(ctx context.Context, cmd DeriveGradientTemplateCommand) (GradientTemplate, error) {
	r.derivedTemplate = cmd
	return GradientTemplate{ID: 202, CustomerID: cmd.CustomerID, SourceTemplateID: cmd.SourceTemplateID, TemplateState: "derived_from_public", Name: cmd.Name, Active: true}, nil
}

func (r *fakeRepo) ListCustomerPublicUsages(ctx context.Context) ([]CustomerPublicUsage, error) {
	return nil, nil
}

func (r *fakeRepo) SaveCustomerPublicUsage(ctx context.Context, cmd CustomerPublicUsageCommand) (CustomerPublicUsage, error) {
	r.publicUsage = cmd
	r.usageSaved = true
	return CustomerPublicUsage{CustomerID: cmd.CustomerID, UsePublicSKU: cmd.UsePublicSKU, UsePublicCategories: cmd.UsePublicCategories, UsePublicGradientTemplates: cmd.UsePublicGradientTemplates}, nil
}

func (r *fakeRepo) ListCustomerProductRuleTemplates(ctx context.Context) ([]CustomerProductRuleTemplate, error) {
	return nil, nil
}

func (r *fakeRepo) ListCustomerProductRuleOverrides(ctx context.Context) ([]CustomerProductRuleOverride, error) {
	return nil, nil
}

func (r *fakeRepo) ListCustomerProductRuleBindings(ctx context.Context) ([]CustomerProductRuleBinding, error) {
	return nil, nil
}

func (r *fakeRepo) SaveCustomerProductRuleTemplate(ctx context.Context, cmd SaveCustomerProductRuleTemplateCommand) (CustomerProductRuleTemplate, error) {
	r.ruleTemplate = cmd
	return CustomerProductRuleTemplate{ID: 501, CustomerID: cmd.CustomerID, Name: cmd.Name, Active: true, Items: cmd.Items}, nil
}

func (r *fakeRepo) SaveCustomerProductRuleOverride(ctx context.Context, cmd SaveCustomerProductRuleOverrideCommand) (CustomerProductRuleOverride, error) {
	r.ruleOverride = cmd
	return CustomerProductRuleOverride{
		ID:                       601,
		CustomerID:               cmd.CustomerID,
		ProductSubtypeCategoryID: cmd.ProductSubtypeCategoryID,
		GradientTemplateID:       cmd.GradientTemplateID,
		OperationTemplateID:      cmd.OperationTemplateID,
		PriceListRuleJSON:        cmd.PriceListRuleJSON,
		UnitRuleJSON:             cmd.UnitRuleJSON,
		Active:                   true,
	}, nil
}

func (r *fakeRepo) BindCustomerProductRuleTemplate(ctx context.Context, cmd CustomerProductRuleTemplateBindingCommand) (CustomerProductRuleBinding, error) {
	r.ruleBinding = cmd
	return CustomerProductRuleBinding{CustomerID: cmd.CustomerID, TemplateID: cmd.TemplateID}, nil
}

func TestServiceDelegatesCatalogOperations(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	ps, err := svc.ListProducts(context.Background())
	if err != nil || len(ps) != 1 {
		t.Fatalf("ListProducts() = %+v, %v", ps, err)
	}
	p, err := svc.GetProduct(context.Background(), 9)
	if err != nil || p.ID != 9 {
		t.Fatalf("GetProduct() = %+v, %v", p, err)
	}
	if err := svc.ReplacePriceTiers(context.Background(), ReplacePriceTiersCommand{ProductID: 9, Tiers: []PriceTier{{SpecG: 454, MinQty: 1, UnitPrice: 2}}}); err != nil {
		t.Fatalf("ReplacePriceTiers() error = %v", err)
	}
	if repo.replace.ProductID != 9 || len(repo.replace.Tiers) != 1 {
		t.Fatalf("replace command = %+v", repo.replace)
	}
	if err := svc.UpdateProductBasics(context.Background(), UpdateProductBasicsCommand{ProductID: 9, RoastLevel: "中烘"}); err != nil {
		t.Fatalf("UpdateProductBasics() error = %v", err)
	}
	if repo.update.ProductID != 9 {
		t.Fatalf("update command = %+v", repo.update)
	}
	if err := svc.DeactivateProducts(context.Background(), DeactivateProductsCommand{Actor: "tester", ProductIDs: []int64{9, 10}}); err != nil {
		t.Fatalf("DeactivateProducts() error = %v", err)
	}
	if !repo.deactivated || repo.deactivate.Actor != "tester" || len(repo.deactivate.ProductIDs) != 2 || repo.deactivate.ProductIDs[0] != 9 || repo.deactivate.ProductIDs[1] != 10 {
		t.Fatalf("deactivate command = %+v deactivated=%v", repo.deactivate, repo.deactivated)
	}
	product, err := svc.CreateProduct(context.Background(), CreateProductCommand{Actor: "tester", Name: "新拼配", RoastLevel: "中烘", YieldRate: 0.81})
	if err != nil || product.ID != 11 {
		t.Fatalf("CreateProduct() = %+v, %v", product, err)
	}
	if repo.create.Name != "新拼配" || repo.create.RoastLevel != "中烘" || repo.create.Actor != "tester" {
		t.Fatalf("create command = %+v", repo.create)
	}
	settings, err := svc.ProductSettings(context.Background())
	if err != nil || len(settings.Categories) != 1 || len(settings.Products) != 1 {
		t.Fatalf("ProductSettings() = %+v, %v", settings, err)
	}
}

func TestCreateCustomProductAcceptsPublicSKUAliasType(t *testing.T) {
	svc := NewService(&fakeRepo{})
	got, err := svc.CreateCustomProduct(context.Background(), CreateCustomProductCommand{
		CustomerID:     3,
		BaseProductID:  7,
		Name:           "客户A-自定义货品名",
		RoastLevel:     "中深烘",
		CustomType:     "public_sku_alias",
		CopyPriceTiers: true,
		CopyBOM:        true,
	})
	if err != nil {
		t.Fatalf("CreateCustomProduct() err=%v", err)
	}
	if got.CustomType != "public_sku_alias" || got.CustomerID != 3 || got.BaseProductID != 7 {
		t.Fatalf("custom product=%+v", got)
	}
}

func TestCreateCustomProductAcceptsCustomerGreenBeanWithoutRoastLevel(t *testing.T) {
	repo := &fakeRepo{products: map[int64]Product{
		8: {ID: 8, ProductKind: "roasted"},
	}}
	svc := NewService(repo)

	got, err := svc.CreateCustomProduct(context.Background(), CreateCustomProductCommand{
		CustomerID:            3,
		Name:                  "客户A-巴拿马生豆",
		ProductKind:           "green_bean",
		GreenBeanType:         "blend",
		GreenBeanBomProductID: 8,
		CustomType:            "public_sku_alias",
		CopyPriceTiers:        true,
	})
	if err != nil {
		t.Fatalf("CreateCustomProduct() err=%v", err)
	}
	if got.ProductKind != "green_bean" || repo.custom.RoastLevel != "" || repo.custom.GreenBeanType != "blend" || repo.custom.GreenBeanBomProductID != 8 {
		t.Fatalf("green custom product=%+v command=%+v", got, repo.custom)
	}
	if got.BaseProductID != 0 || repo.custom.BaseProductID != 0 || repo.custom.CopyPriceTiers {
		t.Fatalf("green custom product should not require base product or copied price tiers: got=%+v command=%+v", got, repo.custom)
	}
}

func TestSaveCustomerPublicUsageAllowsIndependentReferenceSwitches(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.SaveCustomerPublicUsage(context.Background(), CustomerPublicUsageCommand{
		Actor:                      "tester",
		CustomerID:                 42,
		UsePublicSKU:               false,
		UsePublicCategories:        true,
		UsePublicGradientTemplates: true,
	})
	if err != nil {
		t.Fatalf("SaveCustomerPublicUsage() err=%v", err)
	}
	if !repo.usageSaved || repo.publicUsage.Actor != "tester" || repo.publicUsage.CustomerID != 42 || repo.publicUsage.UsePublicSKU || !repo.publicUsage.UsePublicCategories || !repo.publicUsage.UsePublicGradientTemplates {
		t.Fatalf("public usage command = %+v saved=%v", repo.publicUsage, repo.usageSaved)
	}
	if got.CustomerID != 42 || got.UsePublicSKU || !got.UsePublicCategories || !got.UsePublicGradientTemplates {
		t.Fatalf("public usage result = %+v", got)
	}
}

func TestServiceDelegatesCustomerProductRuleConfiguration(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	template, err := svc.SaveCustomerProductRuleTemplate(context.Background(), SaveCustomerProductRuleTemplateCommand{
		Actor:      "tester",
		CustomerID: 42,
		Name:       "贴牌客户规则",
		Items: []CustomerProductRuleTemplateItem{{
			ProductSubtypeCategoryID: 7,
			GradientTemplateID:       8,
			OperationTemplateID:      9,
			PriceListRuleJSON:        `{"mode":"by_subtype"}`,
			UnitRuleJSON:             `{"quote_unit":"元/kg"}`,
		}},
	})
	if err != nil {
		t.Fatalf("SaveCustomerProductRuleTemplate() err=%v", err)
	}
	if template.ID != 501 || repo.ruleTemplate.CustomerID != 42 || len(repo.ruleTemplate.Items) != 1 || repo.ruleTemplate.Items[0].ProductSubtypeCategoryID != 7 {
		t.Fatalf("rule template result=%+v command=%+v", template, repo.ruleTemplate)
	}

	override, err := svc.SaveCustomerProductRuleOverride(context.Background(), SaveCustomerProductRuleOverrideCommand{
		Actor:                    "tester",
		CustomerID:               42,
		ProductSubtypeCategoryID: 7,
		GradientTemplateID:       18,
		OperationTemplateID:      19,
		PriceListRuleJSON:        `{"mode":"customer"}`,
		UnitRuleJSON:             `{"order_unit":"kg"}`,
	})
	if err != nil {
		t.Fatalf("SaveCustomerProductRuleOverride() err=%v", err)
	}
	if override.ID != 601 || repo.ruleOverride.CustomerID != 42 || repo.ruleOverride.ProductSubtypeCategoryID != 7 || repo.ruleOverride.GradientTemplateID != 18 {
		t.Fatalf("rule override result=%+v command=%+v", override, repo.ruleOverride)
	}

	binding, err := svc.BindCustomerProductRuleTemplate(context.Background(), CustomerProductRuleTemplateBindingCommand{
		Actor:      "tester",
		CustomerID: 42,
		TemplateID: 501,
	})
	if err != nil {
		t.Fatalf("BindCustomerProductRuleTemplate() err=%v", err)
	}
	if binding.CustomerID != 42 || binding.TemplateID != 501 || repo.ruleBinding.CustomerID != 42 || repo.ruleBinding.TemplateID != 501 {
		t.Fatalf("rule binding result=%+v command=%+v", binding, repo.ruleBinding)
	}
}

func TestServiceDelegatesProductConfigTemplates(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	template, err := svc.SaveProductConfigTemplate(context.Background(), SaveProductConfigTemplateCommand{
		Actor:               "tester",
		CustomerID:          42,
		Name:                "客户盒装商品配置",
		GradientTemplateID:  8,
		OperationTemplateID: 9,
		PriceListRuleJSON:   `{"pricing_mode":"fixed_unit_price"}`,
		InventoryUnit:       "kg",
		QuoteUnit:           "盒",
		OrderUnit:           "盒",
		UnitConversionJSON:  `{"盒":{"kg":0.2}}`,
		IntegerUnit:         true,
	})
	if err != nil {
		t.Fatalf("SaveProductConfigTemplate() err=%v", err)
	}
	if template.ID != 701 || repo.configTemplate.CustomerID != 42 || repo.configTemplate.Name != "客户盒装商品配置" || repo.configTemplate.QuoteUnit != "盒" || !repo.configTemplate.IntegerUnit {
		t.Fatalf("product config template result=%+v command=%+v", template, repo.configTemplate)
	}

	derived, err := svc.DeriveProductConfigTemplate(context.Background(), DeriveProductConfigTemplateCommand{
		Actor:            "tester",
		CustomerID:       42,
		SourceTemplateID: 301,
		Name:             "客户复制盒装配置",
	})
	if err != nil {
		t.Fatalf("DeriveProductConfigTemplate() err=%v", err)
	}
	if derived.ID != 702 || repo.derivedConfig.CustomerID != 42 || repo.derivedConfig.SourceTemplateID != 301 || repo.derivedConfig.Name != "客户复制盒装配置" {
		t.Fatalf("derived product config template=%+v command=%+v", derived, repo.derivedConfig)
	}
}

func TestDeriveProductCategoryRequiresCustomerAndSource(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.DeriveProductCategory(context.Background(), DeriveProductCategoryCommand{
		Actor:            "tester",
		CustomerID:       42,
		SourceCategoryID: 17,
	})
	if err != nil {
		t.Fatalf("DeriveProductCategory() err=%v", err)
	}
	if repo.derivedCategory.Actor != "tester" || repo.derivedCategory.CustomerID != 42 || repo.derivedCategory.SourceCategoryID != 17 {
		t.Fatalf("derive category command = %+v", repo.derivedCategory)
	}
	if got.ID != 117 || got.CustomerID != 42 || got.SourceCategoryID != 17 || got.TemplateState != "derived_from_public" {
		t.Fatalf("derived category = %+v", got)
	}
	if _, err := svc.DeriveProductCategory(context.Background(), DeriveProductCategoryCommand{CustomerID: 42}); err == nil {
		t.Fatalf("DeriveProductCategory() should require source_category_id")
	}
}

func TestAssignProductCategoryCarriesCustomerDerivationContext(t *testing.T) {
	repo := &fakeRepo{assignResult: AssignProductCategoryResult{
		ProductID:          420,
		CategoryID:         117,
		DerivedProductID:   420,
		DerivedCategoryID:  117,
		UsedPublicCategory: true,
		UsedPublicProduct:  true,
	}}
	svc := NewService(repo)

	got, err := svc.AssignProductCategory(context.Background(), AssignProductCategoryCommand{
		Actor:                "tester",
		ProductID:            21,
		CategoryID:           17,
		CustomerID:           42,
		Position:             3,
		DerivePublicCategory: true,
		DerivePublicProduct:  true,
	})
	if err != nil {
		t.Fatalf("AssignProductCategory() err=%v", err)
	}
	if repo.assigned.CustomerID != 42 || !repo.assigned.DerivePublicCategory || !repo.assigned.DerivePublicProduct || repo.assigned.Position != 3 {
		t.Fatalf("assign command = %+v", repo.assigned)
	}
	if got.ProductID != 420 || got.CategoryID != 117 || got.DerivedProductID != 420 || got.DerivedCategoryID != 117 {
		t.Fatalf("assign result = %+v", got)
	}
}

func TestDeriveGradientTemplateRequiresCustomerAndSource(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.DeriveGradientTemplate(context.Background(), DeriveGradientTemplateCommand{
		Actor:            "tester",
		CustomerID:       42,
		SourceTemplateID: 2,
		Name:             "岩师傅 - 正常磅价模板",
	})
	if err != nil {
		t.Fatalf("DeriveGradientTemplate() err=%v", err)
	}
	if repo.derivedTemplate.Actor != "tester" || repo.derivedTemplate.CustomerID != 42 || repo.derivedTemplate.SourceTemplateID != 2 {
		t.Fatalf("derive template command = %+v", repo.derivedTemplate)
	}
	if got.ID != 202 || got.CustomerID != 42 || got.SourceTemplateID != 2 || got.TemplateState != "derived_from_public" {
		t.Fatalf("derived template = %+v", got)
	}
	if _, err := svc.DeriveGradientTemplate(context.Background(), DeriveGradientTemplateCommand{CustomerID: 42}); err == nil {
		t.Fatalf("DeriveGradientTemplate() should require source_template_id")
	}
}

func TestSaveCustomerPublicUsageRequiresCustomerButAllowsBothSwitchesOff(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if _, err := svc.SaveCustomerPublicUsage(context.Background(), CustomerPublicUsageCommand{UsePublicSKU: true}); err == nil {
		t.Fatalf("SaveCustomerPublicUsage() should require customer_id")
	}
	if _, err := svc.SaveCustomerPublicUsage(context.Background(), CustomerPublicUsageCommand{CustomerID: 42}); err != nil {
		t.Fatalf("SaveCustomerPublicUsage() should allow both public switches off, err=%v", err)
	}
}

func TestCreateProductDefaultsAllowFulfillmentOrderAtServiceBoundary(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if _, err := svc.CreateProduct(context.Background(), CreateProductCommand{Name: "默认履约", RoastLevel: "中烘"}); err != nil {
		t.Fatalf("CreateProduct() err=%v", err)
	}
	if !repo.create.AllowFulfillmentOrder {
		t.Fatalf("CreateProductCommand should default allow fulfillment to true: %+v", repo.create)
	}

	if _, err := svc.CreateProduct(context.Background(), CreateProductCommand{Name: "禁止履约", RoastLevel: "中烘", AllowFulfillmentOrder: false, AllowFulfillmentOrderSet: true}); err != nil {
		t.Fatalf("CreateProduct(explicit false) err=%v", err)
	}
	if repo.create.AllowFulfillmentOrder {
		t.Fatalf("explicit allow fulfillment false should be preserved: %+v", repo.create)
	}
}

func TestCreateProductAcceptsInstantCoffeeWithoutRoastLevel(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if _, err := svc.CreateProduct(context.Background(), CreateProductCommand{
		Name:        "速溶美式",
		ProductKind: "instant_coffee",
	}); err != nil {
		t.Fatalf("CreateProduct(instant_coffee) err=%v", err)
	}

	if repo.create.ProductKind != "instant_coffee" || repo.create.RoastLevel != "" || repo.create.YieldRate != 0 || repo.create.GreenBeanBomProductID != 0 {
		t.Fatalf("instant coffee product command = %+v", repo.create)
	}
}
