package catalog

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type fakeRepo struct {
	replace                ReplacePriceTiersCommand
	update                 UpdateProductBasicsCommand
	create                 CreateProductCommand
	copyProduct            CopyProductCommand
	skuCreate              CreateSKUCommand
	defaultSKU             SetProductDefaultSKUCommand
	defaultSKUResult       Product
	defaultSKUErr          error
	custom                 CreateCustomProductCommand
	derivedProduct         DeriveCustomerProductCommand
	derivedCategory        DeriveProductCategoryCommand
	derivedTemplate        DeriveGradientTemplateCommand
	derivedConfig          DeriveProductConfigTemplateCommand
	assigned               AssignProductCategoryCommand
	assignResult           AssignProductCategoryResult
	publicUsage            CustomerPublicUsageCommand
	aliasQuery             CustomerProductAliasQuery
	aliasCommand           CustomerProductAliasCommand
	aliasBatch             BatchCustomerProductAliasesCommand
	aliasBatchDisable      BatchDisableCustomerProductAliasesCommand
	disabledAlias          DisableCustomerProductAliasCommand
	aliasIndustryQuery     CustomerProductAliasIndustryFieldQuery
	aliasIndustrySave      SaveCustomerProductAliasIndustryFieldsCommand
	aliasCandidates        CustomerProductAliasMigrationCandidateQuery
	ruleTemplate           SaveCustomerProductRuleTemplateCommand
	ruleOverride           SaveCustomerProductRuleOverrideCommand
	ruleBinding            CustomerProductRuleTemplateBindingCommand
	configTemplate         SaveProductConfigTemplateCommand
	productionConfig       SaveProductProductionConfigCommand
	businessGroup          BusinessGroup
	featureSelection       BusinessGroupFeatureSelection
	savedFeatureSelection  SaveBusinessGroupFeatureSelectionCommand
	deleteConfig           DeleteProductConfigTemplateCommand
	priceGroup             SaveProductPriceGroupCommand
	deleteGroup            DeleteBusinessGroupCommand
	groupAssignment        BusinessGroupAssignment
	deleteAssignment       DeleteBusinessGroupAssignmentCommand
	priceRecord            SaveProductPriceRecordCommand
	tierPriceScheme        SaveProductTierPriceSchemeCommand
	classTemplate          SaveProductClassificationTemplateCommand
	classCategory          SaveProductClassificationCategoryCommand
	classAssign            SaveProductClassificationAssignmentCommand
	aliasClassAssign       SaveCustomerProductAliasClassificationAssignmentCommand
	unitDefinition         SaveProductUnitDefinitionCommand
	unitTemplate           SaveProductUnitTemplateCommand
	deactivate             DeactivateProductsCommand
	priceGroups            []ProductPriceGroup
	priceRecords           []ProductPriceRecord
	priceRecordByID        map[int64]ProductPriceRecord
	tierPriceSchemes       []ProductTierPriceScheme
	pricingRules           []ProductPricingRule
	products               map[int64]Product
	publicUsages           []CustomerPublicUsage
	deactivated            bool
	usageSaved             bool
	skuCreated             bool
	productCopied          bool
	priceGroupSaved        bool
	groupAssigned          bool
	groupAssignmentDeleted bool
	priceRecordSaved       bool
	tierSchemeSaved        bool
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
	return Product{ID: 11, Name: cmd.Name, RoastLevel: cmd.RoastLevel, YieldRate: cmd.YieldRate, Visibility: "public", ProductConfigTemplateID: cmd.ProductConfigTemplateID}, nil
}

func (r *fakeRepo) CopyProduct(ctx context.Context, cmd CopyProductCommand) (Product, error) {
	r.copyProduct = cmd
	r.productCopied = true
	return Product{ID: 13, Name: "商品复制", Active: true}, nil
}

func (r *fakeRepo) CreateSKU(ctx context.Context, cmd CreateSKUCommand) (Product, error) {
	r.skuCreate = cmd
	r.skuCreated = true
	visibility := "public"
	if cmd.CustomerID > 0 {
		visibility = "customer_only"
	}
	return Product{ID: 12, SKUID: 12, ParentProductID: cmd.ParentProductID, EffectiveParentProductID: cmd.ParentProductID, SKUName: cmd.SKUName, SKUCode: cmd.SKUCode, Barcode: cmd.Barcode, SpecLabel: cmd.SpecLabel, NetContentQty: cmd.NetContentQty, NetContentUnit: cmd.NetContentUnit, IsDefaultSKU: cmd.IsDefaultSKU, Name: cmd.Name, Remark: cmd.Remark, CustomerID: cmd.CustomerID, ProductCategoryID: cmd.ProductSubtypeCategoryID, Visibility: visibility, SpecialAttrsJSON: cmd.SpecialAttrsJSON, ProductConfigTemplateID: cmd.ProductConfigTemplateID}, nil
}

func (r *fakeRepo) SetProductDefaultSKU(ctx context.Context, cmd SetProductDefaultSKUCommand) (Product, error) {
	r.defaultSKU = cmd
	if r.defaultSKUErr != nil {
		return Product{}, r.defaultSKUErr
	}
	return r.defaultSKUResult, nil
}

func (r *fakeRepo) ListProductCategories(ctx context.Context) ([]ProductCategory, error) {
	return []ProductCategory{{ID: 1, Name: "咖啡豆", Level: 1, Position: 1}}, nil
}

func (r *fakeRepo) ListProductProductionConfigs(ctx context.Context) ([]ProductProductionConfig, error) {
	return nil, nil
}

func (r *fakeRepo) GetProductProductionConfig(ctx context.Context, productID int64) (ProductProductionConfig, error) {
	return ProductProductionConfig{ProductID: productID}, nil
}

func (r *fakeRepo) SaveProductProductionConfig(ctx context.Context, cmd SaveProductProductionConfigCommand) (ProductProductionConfig, error) {
	r.productionConfig = cmd
	return ProductProductionConfig{
		ProductID:                cmd.ProductID,
		ProductionBomID:          cmd.ProductionBomID,
		ProductionBomVersionID:   cmd.ProductionBomVersionID,
		ProcessRouteID:           cmd.ProcessRouteID,
		ExpectedLossRate:         cmd.ExpectedLossRate,
		IndustryFieldTemplateID:  cmd.IndustryFieldTemplateID,
		IndustryFieldTemplateIDs: cmd.IndustryFieldTemplateIDs,
		Note:                     cmd.Note,
		Fields:                   cmd.Fields,
	}, nil
}

func TestPR584SaveProductProductionConfigNormalizesOrderedIndustryTemplateIDs(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)

	result, err := service.SaveProductProductionConfig(context.Background(), SaveProductProductionConfigCommand{
		ProductID:                91,
		IndustryFieldTemplateID:  -9999,
		IndustryFieldTemplateIDs: []int64{3002, 3001, 3002},
		Fields: []ProductProductionConfigField{{
			FieldKey: " roast_level ",
			Label:    " Roast level ",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{3002, 3001}
	if !reflect.DeepEqual(repo.productionConfig.IndustryFieldTemplateIDs, want) {
		t.Fatalf("saved industry_field_template_ids=%v, want %v", repo.productionConfig.IndustryFieldTemplateIDs, want)
	}
	if repo.productionConfig.IndustryFieldTemplateID != 3002 {
		t.Fatalf("saved legacy industry_field_template_id=%d, want first selected id", repo.productionConfig.IndustryFieldTemplateID)
	}
	if !reflect.DeepEqual(result.IndustryFieldTemplateIDs, want) || result.IndustryFieldTemplateID != 3002 {
		t.Fatalf("result templates=%v legacy=%d, want %v and 3002", result.IndustryFieldTemplateIDs, result.IndustryFieldTemplateID, want)
	}
}

func TestPR584SaveProductProductionConfigExplicitEmptyTemplateIDsClearFields(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)

	_, err := service.SaveProductProductionConfig(context.Background(), SaveProductProductionConfigCommand{
		ProductID:                91,
		IndustryFieldTemplateID:  3001,
		IndustryFieldTemplateIDs: []int64{},
		Fields:                   []ProductProductionConfigField{{FieldKey: "roast_level"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.productionConfig.IndustryFieldTemplateID != 0 || len(repo.productionConfig.IndustryFieldTemplateIDs) != 0 {
		t.Fatalf("saved templates=%v legacy=%d, want explicit clear", repo.productionConfig.IndustryFieldTemplateIDs, repo.productionConfig.IndustryFieldTemplateID)
	}
	if repo.productionConfig.Fields == nil || len(repo.productionConfig.Fields) != 0 {
		t.Fatalf("saved fields=%v, want non-nil empty fields", repo.productionConfig.Fields)
	}
}

func TestPR584SaveProductProductionConfigOmittedTemplateIDsFallsBackToLegacyScalar(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)

	_, err := service.SaveProductProductionConfig(context.Background(), SaveProductProductionConfigCommand{
		ProductID:               91,
		IndustryFieldTemplateID: 3001,
		Fields:                  []ProductProductionConfigField{{FieldKey: "roast_level"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := repo.productionConfig.IndustryFieldTemplateIDs, []int64{3001}; !reflect.DeepEqual(got, want) {
		t.Fatalf("saved industry_field_template_ids=%v, want legacy fallback %v", got, want)
	}
	if repo.productionConfig.IndustryFieldTemplateID != 3001 || len(repo.productionConfig.Fields) != 1 {
		t.Fatalf("saved legacy config=%+v", repo.productionConfig)
	}
}

func TestSaveProductProductionConfigClearsFieldsWithoutIndustryTemplate(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)

	result, err := service.SaveProductProductionConfig(context.Background(), SaveProductProductionConfigCommand{
		ProductID:               91,
		IndustryFieldTemplateID: 0,
		Fields: []ProductProductionConfigField{{
			FieldKey:  "roast_level",
			Label:     "roast_level",
			ValueText: "深烘",
		}, {
			FieldKey:  "   ",
			Label:     "stale_field",
			ValueText: "legacy",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.productionConfig.ProductID != 91 {
		t.Fatalf("saved product_id=%d, want repository dispatch for product 91", repo.productionConfig.ProductID)
	}
	if repo.productionConfig.Fields == nil {
		t.Fatal("saved fields=nil, want non-nil empty fields without industry template")
	}
	if len(repo.productionConfig.Fields) != 0 {
		t.Fatalf("saved fields=%+v, want none without industry template", repo.productionConfig.Fields)
	}
	if len(result.Fields) != 0 {
		t.Fatalf("result fields=%+v, want none without industry template", result.Fields)
	}
}

func TestSaveProductProductionConfigRejectsBlankFieldKeyWithIndustryTemplate(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)

	_, err := service.SaveProductProductionConfig(context.Background(), SaveProductProductionConfigCommand{
		ProductID:               91,
		IndustryFieldTemplateID: 3001,
		Fields: []ProductProductionConfigField{{
			FieldKey: "   ",
		}},
	})
	if err == nil || !IsValidationError(err) || err.Error() != "field_key required" {
		t.Fatalf("err=%v, want field_key required", err)
	}
	if repo.productionConfig.ProductID != 0 {
		t.Fatalf("saved product_id=%d, want no repository dispatch after validation failure", repo.productionConfig.ProductID)
	}
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

func (r *fakeRepo) ListProductPriceGroups(ctx context.Context) ([]ProductPriceGroup, error) {
	return r.priceGroups, nil
}

func (r *fakeRepo) SaveProductPriceGroup(ctx context.Context, cmd SaveProductPriceGroupCommand) (ProductPriceGroup, error) {
	r.priceGroup = cmd
	r.priceGroupSaved = true
	id := cmd.ID
	if id == 0 {
		id = 31
	}
	return ProductPriceGroup{ID: id, Name: cmd.Name, SortOrder: cmd.SortOrder, Active: true}, nil
}

func (r *fakeRepo) ListBusinessGroups(ctx context.Context) ([]BusinessGroup, error) {
	return []BusinessGroup{}, nil
}

func (r *fakeRepo) SaveBusinessGroup(ctx context.Context, cmd BusinessGroup) (BusinessGroup, error) {
	r.businessGroup = cmd
	if cmd.ID == 0 {
		cmd.ID = 61
	}
	return cmd, nil
}

func TestPR584SaveBusinessGroupIgnoresLegacyTemplateOwnedUsageWrites(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)

	_, err := service.SaveBusinessGroup(context.Background(), BusinessGroup{
		ID:   61,
		Name: "商品分类模板",
		Usages: []BusinessGroupUsage{
			{UsageKey: " PRODUCT_CATALOG ", UsageLabel: " 商品档案 ", Active: true},
			{UsageKey: BusinessGroupUsageMaterialCatalog, UsageLabel: "物料档案", Active: true},
			{UsageKey: BusinessGroupUsageProductCatalog, UsageLabel: "重复", Active: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.businessGroup.Usages != nil || repo.businessGroup.ReplaceUsages {
		t.Fatalf("cached template-owned usage payload must be ignored, got %+v", repo.businessGroup)
	}

	_, err = service.SaveBusinessGroup(context.Background(), BusinessGroup{ID: 61, Name: "商品分类模板", Usages: []BusinessGroupUsage{}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.businessGroup.Usages != nil {
		t.Fatalf("legacy empty usages=%#v, want nil no-op for cached clients", repo.businessGroup.Usages)
	}

	_, err = service.SaveBusinessGroup(context.Background(), BusinessGroup{ID: 61, Name: "商品分类模板", ReplaceUsages: true, Usages: []BusinessGroupUsage{}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.businessGroup.Usages != nil || repo.businessGroup.ReplaceUsages {
		t.Fatalf("cached explicit clear must not overwrite feature selection, got %+v", repo.businessGroup)
	}
}

func TestPR584BusinessGroupFeatureSelectionIsOrderedFeatureOwnedAndRejectsPriceList(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)
	repo.featureSelection = BusinessGroupFeatureSelection{
		FeatureKey:       BusinessGroupUsageProductCatalog,
		GroupTemplateIDs: []int64{72, 71},
	}
	selection, err := service.GetBusinessGroupFeatureSelection(context.Background(), " PRODUCT_CATALOG ")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selection.GroupTemplateIDs, []int64{72, 71}) {
		t.Fatalf("ordered feature selection=%v", selection.GroupTemplateIDs)
	}

	selection, err = service.SaveBusinessGroupFeatureSelection(context.Background(), SaveBusinessGroupFeatureSelectionCommand{
		Actor:            " tester ",
		FeatureKey:       " PRODUCT_CATALOG ",
		GroupTemplateIDs: []int64{72, 71, 72},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := repo.savedFeatureSelection, (SaveBusinessGroupFeatureSelectionCommand{
		Actor:            "tester",
		FeatureKey:       BusinessGroupUsageProductCatalog,
		GroupTemplateIDs: []int64{72, 71},
	}); !reflect.DeepEqual(got, want) {
		t.Fatalf("saved feature selection=%+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(selection.GroupTemplateIDs, []int64{72, 71}) {
		t.Fatalf("saved ordered feature selection=%v", selection.GroupTemplateIDs)
	}

	_, err = service.SaveBusinessGroupFeatureSelection(context.Background(), SaveBusinessGroupFeatureSelectionCommand{
		FeatureKey:       BusinessGroupUsagePriceList,
		GroupTemplateIDs: []int64{72},
	})
	if err == nil || !IsValidationError(err) {
		t.Fatalf("price_list must not be an independent feature selection: %v", err)
	}
	if err := service.EnsureBusinessGroupUsage(context.Background(), 61, BusinessGroupUsageProductCatalog, "tester"); err == nil || !IsValidationError(err) {
		t.Fatalf("legacy template-owned usage mutation must be rejected: %v", err)
	}
}

func TestPR584BusinessGroupAssignmentNormalization(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)

	_, err := service.SaveBusinessGroupAssignment(context.Background(), BusinessGroupAssignment{
		GroupID:     61,
		GroupItemID: 62,
		UsageKey:    " PRODUCT_CATALOG ",
		ObjectKey:   " PRODUCT ",
		ObjectID:    91,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.groupAssignment.UsageKey != BusinessGroupUsageProductCatalog || repo.groupAssignment.ObjectKey != "product" {
		t.Fatalf("normalized assignment=%+v", repo.groupAssignment)
	}

	_, err = service.SaveBusinessGroupAssignment(context.Background(), BusinessGroupAssignment{
		GroupID: 61, GroupItemID: 62, UsageKey: "unknown_usage", ObjectKey: "product", ObjectID: 91,
	})
	if err == nil || !IsValidationError(err) || err.Error() != "invalid business group usage" {
		t.Fatalf("unsupported assignment usage err=%v", err)
	}
}

func (r *fakeRepo) DeleteBusinessGroup(ctx context.Context, cmd DeleteBusinessGroupCommand) error {
	r.deleteGroup = cmd
	return nil
}

func (r *fakeRepo) SaveBusinessGroupItem(ctx context.Context, cmd BusinessGroupItem) (BusinessGroupItem, error) {
	if cmd.ID == 0 {
		cmd.ID = 66
	}
	return cmd, nil
}

func (r *fakeRepo) DeleteBusinessGroupItem(ctx context.Context, cmd DeleteBusinessGroupItemCommand) error {
	return nil
}

func (r *fakeRepo) MoveBusinessGroupItem(ctx context.Context, cmd MoveBusinessGroupItemCommand) (BusinessGroupItem, error) {
	return BusinessGroupItem{ID: cmd.ID, ParentID: cmd.ParentID, SortOrder: cmd.Position * 10, Active: true}, nil
}

func (r *fakeRepo) EnsureBusinessGroupUsage(ctx context.Context, groupID int64, usageKey string, actor string) error {
	return nil
}

func (r *fakeRepo) GetBusinessGroupFeatureSelection(ctx context.Context, featureKey string) (BusinessGroupFeatureSelection, error) {
	return r.featureSelection, nil
}

func (r *fakeRepo) SaveBusinessGroupFeatureSelection(ctx context.Context, cmd SaveBusinessGroupFeatureSelectionCommand) (BusinessGroupFeatureSelection, error) {
	r.savedFeatureSelection = cmd
	r.featureSelection = BusinessGroupFeatureSelection{FeatureKey: cmd.FeatureKey, GroupTemplateIDs: append([]int64(nil), cmd.GroupTemplateIDs...)}
	return r.featureSelection, nil
}

func (r *fakeRepo) ListBusinessGroupAssignments(ctx context.Context, query BusinessGroupAssignmentQuery) ([]BusinessGroupAssignment, error) {
	return nil, nil
}

func (r *fakeRepo) SaveBusinessGroupAssignment(ctx context.Context, cmd BusinessGroupAssignment) (BusinessGroupAssignment, error) {
	r.groupAssignment = cmd
	r.groupAssigned = true
	if cmd.ID == 0 {
		cmd.ID = 65
	}
	return cmd, nil
}

func (r *fakeRepo) DeleteBusinessGroupAssignment(ctx context.Context, cmd DeleteBusinessGroupAssignmentCommand) error {
	r.deleteAssignment = cmd
	r.groupAssignmentDeleted = true
	return nil
}

func (r *fakeRepo) ListProductCustomerReferences(ctx context.Context, productID int64) ([]ProductCustomerReference, error) {
	return []ProductCustomerReference{}, nil
}

func (r *fakeRepo) SaveProductCustomerReference(ctx context.Context, cmd ProductCustomerReference) (ProductCustomerReference, error) {
	if cmd.ID == 0 {
		cmd.ID = 62
	}
	return cmd, nil
}

func (r *fakeRepo) ListProductPricingRules(ctx context.Context) ([]ProductPricingRule, error) {
	return r.pricingRules, nil
}

func (r *fakeRepo) SaveProductPricingRule(ctx context.Context, cmd ProductPricingRule) (ProductPricingRule, error) {
	if cmd.ID == 0 {
		cmd.ID = 63
	}
	return cmd, nil
}

func (r *fakeRepo) ListPriceTierTemplates(ctx context.Context) ([]PriceTierTemplate, error) {
	return []PriceTierTemplate{}, nil
}

func (r *fakeRepo) SavePriceTierTemplate(ctx context.Context, cmd PriceTierTemplate) (PriceTierTemplate, error) {
	if cmd.ID == 0 {
		cmd.ID = 64
	}
	return cmd, nil
}

func (r *fakeRepo) DeletePriceTierTemplate(ctx context.Context, id int64, actor string) error {
	return nil
}

func (r *fakeRepo) ListProductPriceRecords(ctx context.Context, query ProductPriceRecordQuery) ([]ProductPriceRecord, error) {
	if len(r.priceRecords) > 0 {
		return r.priceRecords, nil
	}
	out := make([]ProductPriceRecord, 0, len(r.priceRecordByID))
	for _, row := range r.priceRecordByID {
		out = append(out, row)
	}
	return out, nil
}

func (r *fakeRepo) GetProductPriceRecord(ctx context.Context, id int64) (ProductPriceRecord, error) {
	if r.priceRecordByID != nil {
		if row, ok := r.priceRecordByID[id]; ok {
			return row, nil
		}
	}
	return ProductPriceRecord{}, nil
}

func (r *fakeRepo) SaveProductPriceRecord(ctx context.Context, cmd SaveProductPriceRecordCommand) (ProductPriceRecord, error) {
	r.priceRecord = cmd
	r.priceRecordSaved = true
	id := cmd.ID
	if id == 0 {
		id = 41
	}
	return ProductPriceRecord{
		ID:                      id,
		ProductID:               cmd.ProductID,
		CustomerProductAliasID:  cmd.CustomerProductAliasID,
		FinalUnitPrice:          cmd.FinalUnitPrice,
		PriceUnit:               cmd.PriceUnit,
		Currency:                cmd.Currency,
		PriceGroupID:            cmd.PriceGroupID,
		PriceGroupName:          cmd.PriceGroupName,
		InventoryUnit:           cmd.InventoryUnit,
		InventoryConversionJSON: cmd.InventoryConversionJSON,
		Status:                  cmd.Status,
		Remark:                  cmd.Remark,
		Active:                  true,
	}, nil
}

func (r *fakeRepo) ListProductTierPriceSchemes(ctx context.Context, query ProductTierPriceSchemeQuery) ([]ProductTierPriceScheme, error) {
	return r.tierPriceSchemes, nil
}

func (r *fakeRepo) SaveProductTierPriceScheme(ctx context.Context, cmd SaveProductTierPriceSchemeCommand) (ProductTierPriceScheme, error) {
	r.tierPriceScheme = cmd
	r.tierSchemeSaved = true
	id := cmd.ID
	if id == 0 {
		id = 51
	}
	return ProductTierPriceScheme{ID: id, Name: cmd.Name, ProductID: cmd.ProductID, CustomerProductAliasID: cmd.CustomerProductAliasID, PriceGroupID: cmd.PriceGroupID, Active: true, Tiers: cmd.Tiers}, nil
}

func (r *fakeRepo) ListProductClassificationTemplates(ctx context.Context) ([]ProductClassificationTemplate, error) {
	return []ProductClassificationTemplate{{
		ID:         801,
		CustomerID: 0,
		Name:       "默认分类模板",
		Active:     true,
		Categories: []ProductClassificationCategory{{
			ID:         802,
			TemplateID: 801,
			Name:       "未分类",
			SortOrder:  0,
			Active:     true,
		}},
	}}, nil
}

func (r *fakeRepo) SaveProductClassificationTemplate(ctx context.Context, cmd SaveProductClassificationTemplateCommand) (ProductClassificationTemplate, error) {
	r.classTemplate = cmd
	id := cmd.ID
	if id == 0 {
		id = 801
	}
	return ProductClassificationTemplate{ID: id, CustomerID: cmd.CustomerID, Name: cmd.Name, Active: true}, nil
}

func (r *fakeRepo) ListProductClassificationTemplateUsages(ctx context.Context) ([]ProductClassificationTemplateUsage, error) {
	return []ProductClassificationTemplateUsage{}, nil
}

func (r *fakeRepo) SaveProductClassificationTemplateUsage(ctx context.Context, cmd SaveProductClassificationTemplateUsageCommand) (ProductClassificationTemplateUsage, error) {
	return ProductClassificationTemplateUsage{ClassificationTemplateID: cmd.ClassificationTemplateID, Active: true, SortOrder: cmd.SortOrder}, nil
}

func (r *fakeRepo) DeleteProductClassificationTemplateUsage(ctx context.Context, cmd DeleteProductClassificationTemplateUsageCommand) error {
	return nil
}

func (r *fakeRepo) ListCustomerProductAliasClassificationTemplateUsages(ctx context.Context, customerID int64) ([]CustomerProductAliasClassificationTemplateUsage, error) {
	return []CustomerProductAliasClassificationTemplateUsage{}, nil
}

func (r *fakeRepo) SaveCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd SaveCustomerProductAliasClassificationTemplateUsageCommand) (CustomerProductAliasClassificationTemplateUsage, error) {
	return CustomerProductAliasClassificationTemplateUsage{CustomerID: cmd.CustomerID, ClassificationTemplateID: cmd.ClassificationTemplateID, Active: true, SortOrder: cmd.SortOrder}, nil
}

func (r *fakeRepo) DeleteCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd DeleteCustomerProductAliasClassificationTemplateUsageCommand) error {
	return nil
}

func (r *fakeRepo) DeleteProductClassificationTemplate(ctx context.Context, cmd DeleteProductClassificationTemplateCommand) error {
	return nil
}

func (r *fakeRepo) SaveProductClassificationCategory(ctx context.Context, cmd SaveProductClassificationCategoryCommand) (ProductClassificationCategory, error) {
	r.classCategory = cmd
	id := cmd.ID
	if id == 0 {
		id = 802
	}
	return ProductClassificationCategory{ID: id, TemplateID: cmd.TemplateID, Name: cmd.Name, SortOrder: cmd.SortOrder, Active: true}, nil
}

func (r *fakeRepo) DeleteProductClassificationCategory(ctx context.Context, cmd DeleteProductClassificationCategoryCommand) error {
	return nil
}

func (r *fakeRepo) SaveProductClassificationAssignment(ctx context.Context, cmd SaveProductClassificationAssignmentCommand) (ProductClassificationAssignment, error) {
	r.classAssign = cmd
	return ProductClassificationAssignment{TemplateID: cmd.TemplateID, CategoryID: cmd.CategoryID, ProductID: cmd.ProductID}, nil
}

func (r *fakeRepo) SaveCustomerProductAliasClassificationAssignment(ctx context.Context, cmd SaveCustomerProductAliasClassificationAssignmentCommand) (CustomerProductAliasClassificationAssignment, error) {
	r.aliasClassAssign = cmd
	return CustomerProductAliasClassificationAssignment{TemplateID: cmd.TemplateID, CategoryID: cmd.CategoryID, AliasID: cmd.AliasID}, nil
}

func (r *fakeRepo) ListProductUnitDefinitions(ctx context.Context) ([]ProductUnitDefinition, error) {
	return []ProductUnitDefinition{{Code: "kg", Name: "kg", UnitType: "weight", AllowDecimal: true, Active: true}, {Code: "盒", Name: "盒", UnitType: "package", Active: true}}, nil
}

func (r *fakeRepo) ListProductUnitTemplates(ctx context.Context) ([]ProductUnitTemplate, error) {
	return []ProductUnitTemplate{{ID: 12, Name: "盒装200g", InventoryUnit: "kg", SalesUnit: "盒", QuoteUnit: "盒", OrderUnit: "盒", UnitConversionJSON: `{"盒":{"kg":0.2}}`, IntegerUnit: true, Active: true}}, nil
}

func (r *fakeRepo) SaveProductConfigTemplate(ctx context.Context, cmd SaveProductConfigTemplateCommand) (ProductConfigTemplate, error) {
	r.configTemplate = cmd
	return ProductConfigTemplate{
		ID:                     701,
		CustomerID:             cmd.CustomerID,
		Name:                   cmd.Name,
		GradientTemplateID:     cmd.GradientTemplateID,
		OperationTemplateID:    cmd.OperationTemplateID,
		UnitTemplateID:         cmd.UnitTemplateID,
		PriceListRuleJSON:      cmd.PriceListRuleJSON,
		SpecialAttrsSchemaJSON: cmd.SpecialAttrsSchemaJSON,
		InventoryUnit:          cmd.InventoryUnit,
		QuoteUnit:              cmd.QuoteUnit,
		OrderUnit:              cmd.OrderUnit,
		UnitConversionJSON:     cmd.UnitConversionJSON,
		IntegerUnit:            cmd.IntegerUnit,
		Active:                 true,
	}, nil
}

func (r *fakeRepo) DeleteProductConfigTemplate(ctx context.Context, cmd DeleteProductConfigTemplateCommand) error {
	r.deleteConfig = cmd
	return nil
}

func (r *fakeRepo) SaveProductUnitDefinition(ctx context.Context, cmd SaveProductUnitDefinitionCommand) (ProductUnitDefinition, error) {
	r.unitDefinition = cmd
	return ProductUnitDefinition{Code: cmd.Code, Name: cmd.Name, UnitType: cmd.UnitType, AllowDecimal: cmd.AllowDecimal, Active: true}, nil
}

func (r *fakeRepo) SaveProductUnitTemplate(ctx context.Context, cmd SaveProductUnitTemplateCommand) (ProductUnitTemplate, error) {
	r.unitTemplate = cmd
	return ProductUnitTemplate{ID: 12, Name: cmd.Name, InventoryUnit: cmd.InventoryUnit, SalesUnit: cmd.SalesUnit, DefaultSalesUnit: cmd.DefaultSalesUnit, SalesUnits: cmd.SalesUnits, SalesSpecs: cmd.SalesSpecs, QuoteUnit: cmd.QuoteUnit, OrderUnit: cmd.OrderUnit, UnitConversionJSON: cmd.UnitConversionJSON, IntegerUnit: cmd.IntegerUnit, Active: true}, nil
}

func (r *fakeRepo) DeleteProductUnitDefinition(ctx context.Context, cmd DeleteProductUnitDefinitionCommand) error {
	return nil
}

func (r *fakeRepo) DeleteProductUnitTemplate(ctx context.Context, cmd DeleteProductUnitTemplateCommand) error {
	return nil
}

func TestServiceSavesProductUnitTemplateWithMultipleSalesUnits(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.SaveProductUnitTemplate(context.Background(), SaveProductUnitTemplateCommand{
		Actor:              "tester",
		Name:               "咖啡豆单位",
		InventoryUnit:      "kg",
		DefaultSalesUnit:   "盒",
		SalesUnits:         []string{"盒", "磅"},
		UnitConversionJSON: `{"盒":{"kg":0.2},"磅":{"kg":0.453592}}`,
		IntegerUnit:        true,
	})
	if err != nil {
		t.Fatalf("SaveProductUnitTemplate() error = %v", err)
	}
	if repo.unitTemplate.SalesUnit != "盒" || repo.unitTemplate.QuoteUnit != "盒" || repo.unitTemplate.OrderUnit != "盒" {
		t.Fatalf("legacy/default sales units not dual-written: %+v", repo.unitTemplate)
	}
	for _, want := range []string{`"kg":{"kg":1}`, `"盒":{"kg":0.2}`, `"磅":{"kg":0.453592}`} {
		if !strings.Contains(repo.unitTemplate.UnitConversionJSON, want) {
			t.Fatalf("normalized conversion %q missing %q", repo.unitTemplate.UnitConversionJSON, want)
		}
	}
	if got.DefaultSalesUnit != "盒" || !reflect.DeepEqual(got.SalesUnits, []string{"kg", "盒", "磅"}) {
		t.Fatalf("returned sales unit fields = default %q units %+v", got.DefaultSalesUnit, got.SalesUnits)
	}
}

func TestServiceNormalizesLegacyUnitTemplateConversionToInventoryUnit(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.SaveProductUnitTemplate(context.Background(), SaveProductUnitTemplateCommand{
		Name:               "箱装咖啡豆",
		InventoryUnit:      "kg",
		DefaultSalesUnit:   "箱",
		UnitConversionJSON: `{"箱":{"盒":24},"盒":{"kg":0.2}}`,
	})
	if err != nil {
		t.Fatalf("SaveProductUnitTemplate() error = %v", err)
	}
	for _, want := range []string{`"箱":{"kg":4.8}`, `"盒":{"kg":0.2}`} {
		if !strings.Contains(repo.unitTemplate.UnitConversionJSON, want) {
			t.Fatalf("legacy conversion should be normalized to inventory unit, got %q missing %q", repo.unitTemplate.UnitConversionJSON, want)
		}
	}
}

func TestServiceRejectsDefaultSalesUnitWithoutInventoryConversion(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.SaveProductUnitTemplate(context.Background(), SaveProductUnitTemplateCommand{
		Name:               "错误模板",
		InventoryUnit:      "kg",
		DefaultSalesUnit:   "箱",
		UnitConversionJSON: `{"盒":{"kg":0.2}}`,
	})
	if err == nil || !strings.Contains(err.Error(), "default sales unit conversion required") {
		t.Fatalf("expected default sales unit conversion error, got %v", err)
	}
}

func TestServiceSavesSalesSpecTemplateWithoutInventoryConversion(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.SaveProductUnitTemplate(context.Background(), SaveProductUnitTemplateCommand{
		Actor: "tester",
		Name:  "咖啡袋装销售规格",
		SalesSpecs: []ProductSalesSpec{
			{SpecKey: "bag-227g", SpecName: "227g袋装", SalesUnit: "袋", NetContentQty: 227, NetContentUnit: "g", Default: true, Active: true},
			{SpecKey: "bag-100g", SpecName: "100g袋装", SalesUnit: "袋", NetContentQty: 100, NetContentUnit: "g", Active: true},
		},
	})
	if err != nil {
		t.Fatalf("SaveProductUnitTemplate() error = %v", err)
	}
	if repo.unitTemplate.InventoryUnit != "kg" {
		t.Fatalf("legacy inventory storage should only default for compatibility, got %q", repo.unitTemplate.InventoryUnit)
	}
	if repo.unitTemplate.UnitConversionJSON != "{}" {
		t.Fatalf("sales spec template should not require sales-unit inventory conversion, got %q", repo.unitTemplate.UnitConversionJSON)
	}
	if repo.unitTemplate.DefaultSalesUnit != "227g袋装" || repo.unitTemplate.SalesUnit != "227g袋装" || repo.unitTemplate.OrderUnit != "227g袋装" || repo.unitTemplate.QuoteUnit != "227g袋装" {
		t.Fatalf("default sales unit should come from default spec and still dual-write legacy fields: %+v", repo.unitTemplate)
	}
	if len(got.SalesSpecs) != 2 || got.SalesSpecs[0].SpecName != "227g袋装" || got.SalesSpecs[1].NetContentQty != 100 {
		t.Fatalf("returned sales specs = %+v", got.SalesSpecs)
	}
}

func TestServiceSavesSelectedDefaultSalesSpecTemplate(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.SaveProductUnitTemplate(context.Background(), SaveProductUnitTemplateCommand{
		Actor: "tester",
		Name:  "咖啡袋装销售规格",
		SalesSpecs: []ProductSalesSpec{
			{SpecKey: "bag-227g", SpecName: "227g袋装", SalesUnit: "袋", NetContentQty: 227, NetContentUnit: "g", Active: true},
			{SpecKey: "bag-100g", SpecName: "100g袋装", SalesUnit: "袋", NetContentQty: 100, NetContentUnit: "g", Default: true, Active: true},
		},
	})
	if err != nil {
		t.Fatalf("SaveProductUnitTemplate() error = %v", err)
	}
	if got.DefaultSalesUnit != "100g袋装" || repo.unitTemplate.DefaultSalesUnit != "100g袋装" {
		t.Fatalf("selected default spec should drive default sales unit, got returned=%+v repo=%+v", got, repo.unitTemplate)
	}
	if !got.SalesSpecs[1].Default || got.SalesSpecs[0].Default {
		t.Fatalf("selected default flags not preserved: %+v", got.SalesSpecs)
	}
	for _, spec := range got.SalesSpecs {
		if spec.SalesUnit != spec.SpecName {
			t.Fatalf("sales spec %q should use spec name as sales unit, got %+v", spec.SpecName, spec)
		}
	}
}

func TestServiceRejectsInactiveDefaultSalesSpecTemplate(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.SaveProductUnitTemplate(context.Background(), SaveProductUnitTemplateCommand{
		Name: "错误销售规格",
		SalesSpecs: []ProductSalesSpec{
			{SpecKey: "bag-227g", SpecName: "227g袋装", SalesUnit: "袋", NetContentQty: 227, NetContentUnit: "g", Default: true, Active: false},
			{SpecKey: "bag-100g", SpecName: "100g袋装", SalesUnit: "袋", NetContentQty: 100, NetContentUnit: "g", Active: true},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "default sales spec must be active") {
		t.Fatalf("expected inactive default sales spec error, got %v", err)
	}
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
	return r.publicUsages, nil
}

func (r *fakeRepo) SaveCustomerPublicUsage(ctx context.Context, cmd CustomerPublicUsageCommand) (CustomerPublicUsage, error) {
	r.publicUsage = cmd
	r.usageSaved = true
	return CustomerPublicUsage{CustomerID: cmd.CustomerID, UsePublicSKU: cmd.UsePublicSKU, UsePublicCategories: cmd.UsePublicCategories, UsePublicGradientTemplates: cmd.UsePublicGradientTemplates}, nil
}

func (r *fakeRepo) EnsureFactoryCustomer(ctx context.Context, actor string) (int64, error) {
	return 9001, nil
}

func (r *fakeRepo) ListCustomerProductAliases(ctx context.Context, query CustomerProductAliasQuery) ([]CustomerProductAlias, error) {
	r.aliasQuery = query
	return []CustomerProductAlias{{ID: 1, CustomerID: query.CustomerID, ProductID: 88, DisplayName: "客户商品名", Active: true}}, nil
}

func (r *fakeRepo) SaveCustomerProductAlias(ctx context.Context, cmd CustomerProductAliasCommand) (CustomerProductAlias, error) {
	r.aliasCommand = cmd
	return CustomerProductAlias{ID: 1, CustomerID: cmd.CustomerID, ProductID: cmd.ProductID, DisplayName: cmd.DisplayName, CustomerItemCode: cmd.CustomerItemCode, GradientTemplateID: cmd.GradientTemplateID, UnitTemplateID: cmd.UnitTemplateID, IncludeInPriceList: cmd.IncludeInPriceList, Active: cmd.Active}, nil
}

func (r *fakeRepo) BatchCreateCustomerProductAliases(ctx context.Context, cmd BatchCustomerProductAliasesCommand) (BatchCustomerProductAliasesResult, error) {
	r.aliasBatch = cmd
	created := make([]CustomerProductAlias, 0, len(cmd.ProductIDs))
	for idx, productID := range cmd.ProductIDs {
		created = append(created, CustomerProductAlias{
			ID:                 int64(idx + 1),
			CustomerID:         cmd.CustomerID,
			ProductID:          productID,
			DisplayName:        "商品档案",
			CustomerItemCode:   "SKU",
			BrandName:          cmd.BrandName,
			DisplayCategoryID:  cmd.DisplayCategoryID,
			IncludeInPriceList: cmd.IncludeInPriceList,
			Active:             true,
		})
	}
	return BatchCustomerProductAliasesResult{CreatedCount: len(created), Created: created}, nil
}

func (r *fakeRepo) DisableCustomerProductAlias(ctx context.Context, cmd DisableCustomerProductAliasCommand) error {
	r.disabledAlias = cmd
	return nil
}

func (r *fakeRepo) BatchDisableCustomerProductAliases(ctx context.Context, cmd BatchDisableCustomerProductAliasesCommand) (BatchDisableCustomerProductAliasesResult, error) {
	r.aliasBatchDisable = cmd
	disabled := append([]int64(nil), cmd.IDs...)
	return BatchDisableCustomerProductAliasesResult{DisabledCount: len(disabled), Disabled: disabled}, nil
}

func (r *fakeRepo) ListCustomerProductAliasIndustryFields(ctx context.Context, query CustomerProductAliasIndustryFieldQuery) ([]ProductProductionConfigField, error) {
	r.aliasIndustryQuery = query
	return []ProductProductionConfigField{{FieldKey: "roast_level", Label: "烘焙度", FieldType: "select", ValueText: "中烘"}}, nil
}

func (r *fakeRepo) SaveCustomerProductAliasIndustryFields(ctx context.Context, cmd SaveCustomerProductAliasIndustryFieldsCommand) ([]ProductProductionConfigField, error) {
	r.aliasIndustrySave = cmd
	return cmd.Fields, nil
}

func (r *fakeRepo) ListCustomerProductAliasMigrationCandidates(ctx context.Context, query CustomerProductAliasMigrationCandidateQuery) ([]CustomerProductAliasMigrationCandidate, error) {
	r.aliasCandidates = query
	return []CustomerProductAliasMigrationCandidate{{
		ProductID:        88,
		ProductName:      "Karen 贴牌意式",
		BaseProductID:    7,
		BaseProductName:  "精品意式拼配",
		SuggestedAction:  "convert_to_customer_product_alias",
		SuggestedReason:  "仅名称/编号/价格差异，生产定义跟随来源商品档案",
		CanAutoRecommend: true,
	}}, nil
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

func TestLegacyProductPriceRecordWritesAreReadonly(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.SaveProductPriceRecord(context.Background(), SaveProductPriceRecordCommand{
		Actor:                   " tester ",
		ProductID:               7,
		FinalUnitPrice:          88.5,
		PriceUnit:               " kg ",
		Currency:                "",
		PriceGroupName:          " 常规批发 ",
		InventoryUnit:           "kg",
		InventoryConversionJSON: `{"kg":{"kg":1}}`,
		Status:                  "published",
		Remark:                  " 直接最终价 ",
	})
	if err == nil || !strings.Contains(err.Error(), "product price records are legacy readonly") {
		t.Fatalf("SaveProductPriceRecord() err=%v, want legacy readonly", err)
	}
	if repo.priceRecordSaved {
		t.Fatalf("legacy price record write should not reach repo")
	}
}

func TestLegacyProductTierPriceSchemeWritesAreReadonly(t *testing.T) {
	repo := &fakeRepo{priceRecordByID: map[int64]ProductPriceRecord{
		11: {ID: 11, ProductID: 7, FinalUnitPrice: 88, PriceUnit: "kg", Currency: "CNY", Active: true},
		12: {ID: 12, ProductID: 7, FinalUnitPrice: 82, PriceUnit: "kg", Currency: "CNY", Active: true},
	}}
	svc := NewService(repo)

	_, err := svc.SaveProductTierPriceScheme(context.Background(), SaveProductTierPriceSchemeCommand{
		Actor:        "tester",
		Name:         "批发阶梯",
		ProductID:    7,
		PriceGroupID: 3,
		Tiers: []ProductTierPriceSchemeTier{
			{Label: "1kg+", MinQty: 1, SourcePriceRecordID: 11, FinalUnitPrice: 999, PriceUnit: "箱", Currency: "USD", Position: 2},
			{Label: "10kg+", MinQty: 10, SourcePriceRecordID: 12, FinalUnitPrice: 999, PriceUnit: "箱", Currency: "USD", Position: 1},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "product tier price schemes are legacy readonly") {
		t.Fatalf("SaveProductTierPriceScheme() err=%v, want legacy readonly", err)
	}
	if repo.tierSchemeSaved {
		t.Fatalf("legacy tier scheme write should not reach repo")
	}
}

func TestPricingRuleAndPriceTierTemplateServicesUseNewPriceListModel(t *testing.T) {
	svc := NewService(&fakeRepo{})

	rule, err := svc.SaveProductPricingRule(context.Background(), ProductPricingRule{
		Name:         " 成本加成模板 ",
		Code:         " RULE-001 ",
		MarginRate:   0.18,
		TaxRate:      0.06,
		RoundingMode: "",
		Remark:       " 后续价格表引用 ",
	})
	if err != nil {
		t.Fatalf("SaveProductPricingRule() err=%v", err)
	}
	if rule.ID <= 0 || rule.Name != "成本加成模板" || rule.Code != "RULE-001" || rule.CostSourceMode != "bom_current_cost" || rule.RoundingMode != "none" {
		t.Fatalf("pricing rule not normalized: %+v", rule)
	}

	maxQty := 10.0
	template, err := svc.SavePriceTierTemplate(context.Background(), PriceTierTemplate{
		Name: " 批发档位 ",
		Tiers: []PriceTierTemplateTier{
			{Label: "10kg+", MinQty: 10, QuantityUnit: " kg ", PricingRuleID: rule.ID, Position: 2},
			{Label: "1kg+", MinQty: 1, MaxQty: &maxQty, QuantityUnit: "", PricingRuleID: rule.ID, Position: 1},
		},
	})
	if err != nil {
		t.Fatalf("SavePriceTierTemplate() err=%v", err)
	}
	if template.ID <= 0 || template.Name != "批发档位" || len(template.Tiers) != 2 {
		t.Fatalf("price tier template = %+v", template)
	}
	if template.Tiers[0].Label != "1kg+" || template.Tiers[0].QuantityUnit != "kg" || template.Tiers[1].Label != "10kg+" || template.Tiers[1].QuantityUnit != "kg" {
		t.Fatalf("price tier template tiers not normalized/sorted: %+v", template.Tiers)
	}
	if _, err := svc.SavePriceTierTemplate(context.Background(), PriceTierTemplate{
		Name: " 缺少计算模板 ",
		Tiers: []PriceTierTemplateTier{
			{Label: "1kg+", MinQty: 1, QuantityUnit: "kg"},
		},
	}); err == nil {
		t.Fatalf("SavePriceTierTemplate() must reject enabled tiers without pricing_rule_id")
	}
}

func TestProductPricingRuleUsesMarkupAsTheOnlyPricingMethod(t *testing.T) {
	tests := []struct {
		name       string
		marginRate float64
		method     string
		wantRate   float64
	}{
		{name: "legacy gross margin keeps the entered percentage meaning", marginRate: 0.8, method: "gross_margin", wantRate: 0.8},
		{name: "legacy whole percent is normalized", marginRate: 80, method: "gross_margin", wantRate: 0.8},
		{name: "missing legacy method becomes markup", marginRate: 0.35, method: "", wantRate: 0.35},
		{name: "markup over one remains a valid rate", marginRate: 1.2, method: "markup", wantRate: 1.2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculation := map[string]any{}
			if tt.method != "" {
				calculation["profit_method"] = tt.method
			}
			rule, err := NewService(&fakeRepo{}).SaveProductPricingRule(context.Background(), ProductPricingRule{
				Name:            "加价率模板",
				MarginRate:      tt.marginRate,
				CalculationJSON: calculation,
			})
			if err != nil {
				t.Fatalf("SaveProductPricingRule() err=%v", err)
			}
			if rule.MarginRate != tt.wantRate {
				t.Fatalf("margin_rate=%v, want %v", rule.MarginRate, tt.wantRate)
			}
			got, ok := rule.CalculationJSON["profit_method"].(string)
			if !ok || strings.TrimSpace(got) != "markup" {
				t.Fatalf("profit_method=%v, want markup", rule.CalculationJSON["profit_method"])
			}
		})
	}

	_, err := NewService(&fakeRepo{}).SaveProductPricingRule(context.Background(), ProductPricingRule{
		Name:            "旧固定加价模板",
		MarginRate:      3,
		CalculationJSON: map[string]any{"profit_method": "fixed_add"},
	})
	if err == nil || !strings.Contains(err.Error(), "only markup rate is supported") {
		t.Fatalf("fixed_add err=%v, want explicit markup-only validation", err)
	}

	_, err = NewService(&fakeRepo{}).SaveProductPricingRule(context.Background(), ProductPricingRule{
		Name:       "已隔离旧固定加价模板",
		MarginRate: 0,
		CalculationJSON: map[string]any{
			"profit_method":        "markup",
			"legacy_profit_method": "fixed_add",
			"legacy_margin_rate":   3,
			"migration_warning":    "only markup rate is supported",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "quarantined legacy pricing rule") {
		t.Fatalf("quarantined legacy rule err=%v, want replacement validation", err)
	}
}

func TestProductPricingRuleRejectsCleanUpdateOverQuarantinedExistingTemplate(t *testing.T) {
	repo := &fakeRepo{pricingRules: []ProductPricingRule{{
		ID:         77,
		Name:       "已隔离旧固定加价模板",
		MarginRate: 0,
		Active:     false,
		CalculationJSON: map[string]any{
			"profit_method":        "markup",
			"legacy_profit_method": "fixed_add",
			"legacy_margin_rate":   3,
			"migration_warning":    "only markup rate is supported",
		},
	}}}

	_, err := NewService(repo).SaveProductPricingRule(context.Background(), ProductPricingRule{
		ID:              77,
		Name:            "试图覆盖隔离模板",
		MarginRate:      0.8,
		Active:          true,
		CalculationJSON: map[string]any{"profit_method": "markup"},
	})
	if err == nil || !strings.Contains(err.Error(), "quarantined legacy pricing rule") {
		t.Fatalf("clean update over quarantined row err=%v, want replacement validation", err)
	}
}

func TestListProductPricingRulesNormalizesLegacyMethodsWithoutChangingPublishedPrices(t *testing.T) {
	repo := &fakeRepo{pricingRules: []ProductPricingRule{
		{ID: 1, Name: "旧毛利率", MarginRate: 20, Active: true, CalculationJSON: map[string]any{"profit_method": "gross_margin"}},
		{ID: 2, Name: "旧固定加价", MarginRate: 3, Active: true, CalculationJSON: map[string]any{"profit_method": "fixed_add"}},
	}}
	rows, err := NewService(repo).ListProductPricingRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].MarginRate != 0.2 || rows[0].CalculationJSON["profit_method"] != "markup" || !rows[0].Active {
		t.Fatalf("legacy gross-margin row=%+v, want active markup 0.2", rows[0])
	}
	if rows[1].MarginRate != 0 || rows[1].CalculationJSON["profit_method"] != "markup" || rows[1].Active {
		t.Fatalf("legacy fixed-add row=%+v, want quarantined inactive markup row", rows[1])
	}
	if rows[1].CalculationJSON["legacy_profit_method"] != "fixed_add" || rows[1].CalculationJSON["legacy_margin_rate"] != float64(3) {
		t.Fatalf("legacy fixed-add evidence missing: %+v", rows[1].CalculationJSON)
	}
	settings, err := NewService(repo).ProductSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.ProductPricingRules) != 2 || settings.ProductPricingRules[0].MarginRate != 0.2 || settings.ProductPricingRules[0].CalculationJSON["profit_method"] != "markup" || settings.ProductPricingRules[1].Active {
		t.Fatalf("ProductSettings pricing rules bypassed markup-only normalization: %+v", settings.ProductPricingRules)
	}
}

func TestProductSettingsKeepsBomParamsOnNonGreenSKU(t *testing.T) {
	settings := BuildProductSettings(nil, []Product{{
		ID:          88,
		Name:        "速溶盒装",
		ProductKind: "instant_coffee",
		RoastLevel:  "中烘",
		YieldRate:   0.96,
	}})

	if len(settings.Products) != 1 {
		t.Fatalf("settings products = %+v", settings.Products)
	}
	got := settings.Products[0]
	if got.ProductKind != "instant_coffee" || got.RoastLevel != "中烘" || got.YieldRate != 0.96 {
		t.Fatalf("instant coffee product settings = %+v, want roast/yield from SKU", got)
	}
}

func TestSetProductDefaultSKUValidatesAndDelegates(t *testing.T) {
	repo := &fakeRepo{defaultSKUResult: Product{ID: 41, DefaultSKUID: 43, EffectiveDefaultSKUID: 43, DefaultSpecLabel: "1磅"}}
	svc := NewService(repo)

	for _, cmd := range []SetProductDefaultSKUCommand{
		{ParentProductID: 0, SKUID: 43},
		{ParentProductID: 41, SKUID: 0},
	} {
		if _, err := svc.SetProductDefaultSKU(context.Background(), cmd); err == nil || !IsValidationError(err) {
			t.Fatalf("command %+v should fail validation, got %v", cmd, err)
		}
	}

	got, err := svc.SetProductDefaultSKU(context.Background(), SetProductDefaultSKUCommand{Actor: "  刘祎泊  ", ParentProductID: 41, SKUID: 43})
	if err != nil {
		t.Fatal(err)
	}
	if repo.defaultSKU.Actor != "刘祎泊" || repo.defaultSKU.ParentProductID != 41 || repo.defaultSKU.SKUID != 43 {
		t.Fatalf("delegated command = %+v", repo.defaultSKU)
	}
	if got.DefaultSKUID != 43 || got.EffectiveDefaultSKUID != 43 || got.DefaultSpecLabel != "1磅" {
		t.Fatalf("result = %+v", got)
	}
}

func TestCreateProductKeepsBomParamsOnInstantCoffee(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.CreateProduct(context.Background(), CreateProductCommand{
		Actor:            "tester",
		Name:             "速溶盒装",
		ProductKind:      "instant_coffee",
		SpecialAttrsJSON: `{"roast_level":"中烘"}`,
		YieldRate:        0.96,
	})
	if err != nil {
		t.Fatalf("CreateProduct() err=%v", err)
	}
	if repo.create.ProductKind != "instant_coffee" || repo.create.RoastLevel != "" || repo.create.YieldRate != 0.96 || repo.create.SpecialAttrsJSON == "{}" {
		t.Fatalf("create command = %+v, want instant coffee yield and special attrs preserved without legacy roast", repo.create)
	}
}

func TestCreateSKUUsesUnifiedPayloadWithoutLegacyProductKindFields(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.CreateSKU(context.Background(), CreateSKUCommand{
		Actor:                    "tester",
		CustomerID:               42,
		Name:                     "客户盒装速溶",
		Remark:                   "10g/条，10条/盒",
		ProductTypeCategoryID:    7,
		ProductSubtypeCategoryID: 17,
		SpecialAttrsJSON:         `{"roast_level":"中深烘"}`,
		Active:                   true,
	})
	if err != nil {
		t.Fatalf("CreateSKU() err=%v", err)
	}
	if !repo.skuCreated || repo.skuCreate.CustomerID != 42 || repo.skuCreate.ProductSubtypeCategoryID != 17 || repo.skuCreate.SpecialAttrsJSON != `{"roast_level":"中深烘"}` {
		t.Fatalf("CreateSKU command=%+v created=%v", repo.skuCreate, repo.skuCreated)
	}
	if got.CustomerID != 42 || got.ProductCategoryID != 17 || got.BaseProductID != 0 || got.CustomType != "" {
		t.Fatalf("created SKU result = %+v", got)
	}
}

func TestCreateSKUSupportsParentProductAndSpecFields(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.CreateSKU(context.Background(), CreateSKUCommand{
		Actor:                "tester",
		ParentProductID:      88,
		Name:                 "埃塞俄比亚 水洗 227g袋装",
		SKUName:              "227g袋装",
		SKUCode:              "ETH-227",
		Barcode:              "690000000227",
		SpecLabel:            "227g",
		NetContentQty:        227,
		NetContentUnit:       "g",
		UnitTemplateID:       12,
		SpecialAttrsJSON:     `{}`,
		UnitRuleOverrideJSON: `{}`,
		Active:               true,
	})
	if err != nil {
		t.Fatalf("CreateSKU() err=%v", err)
	}
	if !repo.skuCreated || repo.skuCreate.ParentProductID != 88 || repo.skuCreate.SKUName != "227g袋装" || repo.skuCreate.SKUCode != "ETH-227" || repo.skuCreate.Barcode != "690000000227" {
		t.Fatalf("CreateSKU command=%+v created=%v", repo.skuCreate, repo.skuCreated)
	}
	if got.SKUID != got.ID || got.ParentProductID != 88 || got.SKUName != "227g袋装" || got.SpecLabel != "227g" || got.NetContentQty != 227 || got.NetContentUnit != "g" {
		t.Fatalf("created child SKU result = %+v", got)
	}
}

func TestCopyProductValidatesAndDelegates(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if _, err := svc.CopyProduct(context.Background(), CopyProductCommand{Actor: "tester"}); err == nil {
		t.Fatal("CopyProduct without source should fail")
	}
	got, err := svc.CopyProduct(context.Background(), CopyProductCommand{Actor: "tester", SourceProductID: 7})
	if err != nil {
		t.Fatalf("CopyProduct() err=%v", err)
	}
	if !repo.productCopied || repo.copyProduct.SourceProductID != 7 || got.ID != 13 {
		t.Fatalf("CopyProduct command=%+v copied=%v result=%+v", repo.copyProduct, repo.productCopied, got)
	}
}

func TestListCustomerProductAliasMigrationCandidatesValidatesAndDelegates(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if _, err := svc.ListCustomerProductAliasMigrationCandidates(context.Background(), CustomerProductAliasMigrationCandidateQuery{}); err == nil {
		t.Fatal("ListCustomerProductAliasMigrationCandidates without customer should fail")
	}
	got, err := svc.ListCustomerProductAliasMigrationCandidates(context.Background(), CustomerProductAliasMigrationCandidateQuery{CustomerID: 42})
	if err != nil {
		t.Fatalf("ListCustomerProductAliasMigrationCandidates err=%v", err)
	}
	if repo.aliasCandidates.CustomerID != 42 {
		t.Fatalf("alias candidate query = %+v, want customer 42", repo.aliasCandidates)
	}
	if len(got) != 1 || got[0].SuggestedAction != "convert_to_customer_product_alias" {
		t.Fatalf("migration candidates = %+v", got)
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
		Actor:                  "tester",
		CustomerID:             42,
		Name:                   "客户盒装商品配置",
		GradientTemplateID:     8,
		OperationTemplateID:    9,
		PriceListRuleJSON:      `{"pricing_mode":"fixed_unit_price","fixed_unit_price":15}`,
		SpecialAttrsSchemaJSON: `[{"key":"roast_level","label":"烘焙度","value_type":"select","options":["浅烘","中烘"],"show_in_price_list":true}]`,
		InventoryUnit:          "kg",
		QuoteUnit:              "盒",
		OrderUnit:              "盒",
		UnitConversionJSON:     `{"盒":{"kg":0.2}}`,
		IntegerUnit:            true,
	})
	if err != nil {
		t.Fatalf("SaveProductConfigTemplate() err=%v", err)
	}
	if template.ID != 701 || repo.configTemplate.CustomerID != 42 || repo.configTemplate.Name != "客户盒装商品配置" || repo.configTemplate.QuoteUnit != "盒" || !repo.configTemplate.IntegerUnit {
		t.Fatalf("product config template result=%+v command=%+v", template, repo.configTemplate)
	}
	if repo.configTemplate.SpecialAttrsSchemaJSON == "" || repo.configTemplate.SpecialAttrsSchemaJSON == "{}" {
		t.Fatalf("special attrs schema not carried: %+v", repo.configTemplate)
	}

	if _, err := svc.SaveProductConfigTemplate(context.Background(), SaveProductConfigTemplateCommand{
		Name:              "缺固定价",
		UnitTemplateID:    12,
		PriceListRuleJSON: `{"pricing_mode":"fixed_unit_price"}`,
	}); err == nil {
		t.Fatalf("SaveProductConfigTemplate() must reject fixed_unit_price without fixed_unit_price")
	}
	if _, err := svc.SaveProductConfigTemplate(context.Background(), SaveProductConfigTemplateCommand{
		Name:              "缺加成比例",
		UnitTemplateID:    12,
		PriceListRuleJSON: `{"pricing_mode":"cost_plus"}`,
	}); err == nil {
		t.Fatalf("SaveProductConfigTemplate() must reject cost_plus without cost_plus_rate")
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

func TestDeriveProductConfigTemplateEnablesPublicSKUReference(t *testing.T) {
	repo := &fakeRepo{
		publicUsages: []CustomerPublicUsage{{
			CustomerID:                 42,
			UsePublicSKU:               false,
			UsePublicCategories:        false,
			UsePublicGradientTemplates: true,
		}},
	}
	svc := NewService(repo)

	if _, err := svc.DeriveProductConfigTemplate(context.Background(), DeriveProductConfigTemplateCommand{
		Actor:            "tester",
		CustomerID:       42,
		SourceTemplateID: 301,
		Name:             "客户复制盒装配置",
	}); err != nil {
		t.Fatalf("DeriveProductConfigTemplate() err=%v", err)
	}

	if !repo.usageSaved {
		t.Fatalf("DeriveProductConfigTemplate() should save public SKU reference usage")
	}
	if repo.publicUsage.CustomerID != 42 || !repo.publicUsage.UsePublicSKU || !repo.publicUsage.UsePublicCategories {
		t.Fatalf("public usage should reference public SKU and categories, got %+v", repo.publicUsage)
	}
	if !repo.publicUsage.UsePublicGradientTemplates {
		t.Fatalf("public gradient template switch should be preserved, got %+v", repo.publicUsage)
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

func TestDeriveProductCategoryEnablesPublicSKUReference(t *testing.T) {
	repo := &fakeRepo{
		publicUsages: []CustomerPublicUsage{{
			CustomerID:                 42,
			UsePublicSKU:               false,
			UsePublicCategories:        false,
			UsePublicGradientTemplates: true,
		}},
	}
	svc := NewService(repo)

	if _, err := svc.DeriveProductCategory(context.Background(), DeriveProductCategoryCommand{
		Actor:            "tester",
		CustomerID:       42,
		SourceCategoryID: 17,
	}); err != nil {
		t.Fatalf("DeriveProductCategory() err=%v", err)
	}

	if !repo.usageSaved {
		t.Fatalf("DeriveProductCategory() should save public SKU reference usage")
	}
	if repo.publicUsage.CustomerID != 42 || !repo.publicUsage.UsePublicSKU || !repo.publicUsage.UsePublicCategories {
		t.Fatalf("public usage should reference public SKU and categories, got %+v", repo.publicUsage)
	}
	if !repo.publicUsage.UsePublicGradientTemplates {
		t.Fatalf("public gradient template switch should be preserved, got %+v", repo.publicUsage)
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

func TestCreateProductAcceptsInstantCoffeeWithDefaultBomParams(t *testing.T) {
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

func TestAutoDerivedSKUUsesSalesSpecNetContentForUnitConversion(t *testing.T) {
	defaultSalesUnit, conversionJSON, rulesJSON := productSalesUnitRuleFields(Product{
		AutoDerivedSKU:   true,
		DerivedSalesUnit: "袋",
		NetContentQty:    227,
		NetContentUnit:   "g",
	}, "kg")

	if defaultSalesUnit != "袋" {
		t.Fatalf("default sales unit = %q, want 袋", defaultSalesUnit)
	}
	if conversionJSON != `{"袋":{"kg":0.227}}` {
		t.Fatalf("conversionJSON = %s, want sales spec net content conversion", conversionJSON)
	}
	if rulesJSON != `{}` {
		t.Fatalf("rulesJSON = %s, want empty rules", rulesJSON)
	}
}

func TestAutoDerivedSKUUsesDirectSalesSpecUnitConversionWhenUnitsMatch(t *testing.T) {
	_, conversionJSON, _ := productSalesUnitRuleFields(Product{
		AutoDerivedSKU:   true,
		DerivedSalesUnit: "箱",
		NetContentQty:    12,
		NetContentUnit:   "袋",
	}, "袋")

	if conversionJSON != `{"箱":{"袋":12}}` {
		t.Fatalf("conversionJSON = %s, want direct sales spec conversion", conversionJSON)
	}
}

func TestResolvePriceTableTemplateInheritanceWalksAllAncestorsAndKeepsFixedPriceOnConcreteProduct(t *testing.T) {
	got := ResolvePriceTableTemplateInheritance(PriceTableTemplateResolutionInput{
		DefaultPricingMode:    "tier_template",
		DefaultTierTemplateID: 90,
		DefaultFixedUnitPrice: 999,
		GroupAssignments: []PriceTableGroupTemplateAssignment{
			{GroupItemID: 10, PricingMode: "fixed_price", FixedUnitPrice: 777},
			{GroupItemID: 20, ParentGroupItemID: 10},
			{GroupItemID: 30, ParentGroupItemID: 20},
			{GroupItemID: 40, ParentGroupItemID: 30},
		},
		ProductOverrides: []PriceTableProductTemplateOverride{{ProductID: 401, FixedUnitPrice: 86}},
		ProductID:        401,
		GroupItemID:      40,
	})

	if got.PricingMode != "fixed_price" || got.PricingModeSource != "parent_group" {
		t.Fatalf("mode = %q from %q, want inherited fixed_price from ancestor", got.PricingMode, got.PricingModeSource)
	}
	if got.FixedUnitPrice != 86 || got.FixedUnitPriceSource != "sku" {
		t.Fatalf("fixed price = %v from %q, want concrete product amount 86", got.FixedUnitPrice, got.FixedUnitPriceSource)
	}
}

func TestResolvePriceTableTemplateInheritanceSeparatesParentMethodFromSKUAmounts(t *testing.T) {
	input := PriceTableTemplateResolutionInput{
		DefaultPricingMode: "tier_template",
		ProductOverrides: []PriceTableProductTemplateOverride{
			{Scope: "parent_product", ProductID: 400, ParentProductID: 400, PricingMode: "fixed_price"},
			// Legacy fixed-only SKU rows had no explicit scope; they must not
			// erase the modern parent-product pricing method.
			{ProductID: 401, SKUID: 401, ParentProductID: 400, FixedUnitPrice: 86},
			{Scope: "sku", ProductID: 402, SKUID: 402, ParentProductID: 400, FixedUnitPrice: 118},
		},
		ParentProductID: 400,
	}

	input.ProductID = 401
	first := ResolvePriceTableTemplateInheritance(input)
	input.ProductID = 402
	second := ResolvePriceTableTemplateInheritance(input)

	if first.PricingMode != "fixed_price" || first.PricingModeSource != "product" || first.FixedUnitPrice != 86 {
		t.Fatalf("first resolution = %+v, want parent fixed method and SKU amount 86", first)
	}
	if second.PricingMode != "fixed_price" || second.PricingModeSource != "product" || second.FixedUnitPrice != 118 {
		t.Fatalf("second resolution = %+v, want parent fixed method and SKU amount 118", second)
	}
}

func TestResolvePriceTableTemplateInheritanceUsesNearestAncestorAndStopsAtCycles(t *testing.T) {
	got := ResolvePriceTableTemplateInheritance(PriceTableTemplateResolutionInput{
		DefaultPricingMode:   "tier_template",
		DefaultPricingRuleID: 91,
		GroupAssignments: []PriceTableGroupTemplateAssignment{
			{GroupItemID: 10, ParentGroupItemID: 20, PricingMode: "fixed_price"},
			{GroupItemID: 20, ParentGroupItemID: 10, PricingMode: "pricing_rule", PricingRuleID: 22},
			{GroupItemID: 30, ParentGroupItemID: 20},
		},
		ProductID:   401,
		GroupItemID: 30,
	})

	if got.PricingMode != "pricing_rule" || got.PricingRuleID != 22 || got.PricingModeSource != "parent_group" {
		t.Fatalf("resolution = %+v, want nearest ancestor pricing rule without looping", got)
	}
}

func TestResolvePriceTableTemplateInheritanceDoesNotBorrowMissingTemplateFromFartherLevel(t *testing.T) {
	got := ResolvePriceTableTemplateInheritance(PriceTableTemplateResolutionInput{
		DefaultPricingMode:   "pricing_rule",
		DefaultPricingRuleID: 88,
		GroupAssignments: []PriceTableGroupTemplateAssignment{
			{GroupItemID: 10, PricingMode: "pricing_rule", PricingRuleID: 77},
			{GroupItemID: 20, ParentGroupItemID: 10, PricingMode: "pricing_rule"},
		},
		ProductID:   401,
		GroupItemID: 20,
	})

	if got.PricingMode != "pricing_rule" || got.PricingRuleID != 0 || got.PricingRuleSource != "subgroup" {
		t.Fatalf("resolution = %+v, want missing rule at effective subgroup", got)
	}
}

func TestResolvePriceTableTemplateInheritanceDoesNotInferModeFromFixedAmount(t *testing.T) {
	got := ResolvePriceTableTemplateInheritance(PriceTableTemplateResolutionInput{
		GroupAssignments: []PriceTableGroupTemplateAssignment{
			{GroupItemID: 10},
			{GroupItemID: 20, ParentGroupItemID: 10},
		},
		ProductOverrides: []PriceTableProductTemplateOverride{{Scope: "sku", ProductID: 401, SKUID: 401, FixedUnitPrice: 86}},
		ProductID:        401,
		GroupItemID:      20,
	})

	if got.PricingMode != "" || got.FixedUnitPrice != 86 {
		t.Fatalf("resolution = %+v, want amount preserved without inventing fixed mode", got)
	}
}

func TestResolvePriceTableTemplateInheritanceInfersLegacyTemplateModeFromNearestLevel(t *testing.T) {
	got := ResolvePriceTableTemplateInheritance(PriceTableTemplateResolutionInput{
		GroupAssignments: []PriceTableGroupTemplateAssignment{
			{GroupItemID: 10, TierTemplateID: 9},
			{GroupItemID: 20, ParentGroupItemID: 10, PricingRuleID: 22},
		},
		ProductID:   401,
		GroupItemID: 20,
	})

	if got.PricingMode != "pricing_rule" || got.PricingModeSource != "subgroup" || got.PricingRuleID != 22 {
		t.Fatalf("resolution = %+v, want nearest legacy pricing-rule config", got)
	}
}
