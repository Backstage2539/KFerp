package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	catalogapp "orderapp/internal/application/catalog"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type productSettingsRepo struct {
	products                            []catalogapp.Product
	categories                          []catalogapp.ProductCategory
	gradientTemplates                   []catalogapp.GradientTemplate
	productConfigTemplates              []catalogapp.ProductConfigTemplate
	productUnitDefinitions              []catalogapp.ProductUnitDefinition
	productUnitTemplates                []catalogapp.ProductUnitTemplate
	productProductionConfigs            []catalogapp.ProductProductionConfig
	savedCategory                       catalogapp.SaveProductCategoryCommand
	movedCategory                       catalogapp.MoveProductCategoryCommand
	deletedCategory                     catalogapp.DeleteProductCategoryCommand
	assigned                            catalogapp.AssignProductCategoryCommand
	assignResult                        catalogapp.AssignProductCategoryResult
	derivedProduct                      catalogapp.DeriveCustomerProductCommand
	derivedCategory                     catalogapp.DeriveProductCategoryCommand
	derivedTemplate                     catalogapp.DeriveGradientTemplateCommand
	derivedConfig                       catalogapp.DeriveProductConfigTemplateCommand
	savedTemplate                       catalogapp.SaveGradientTemplateCommand
	savedConfigTemplate                 catalogapp.SaveProductConfigTemplateCommand
	savedUnitDefinition                 catalogapp.SaveProductUnitDefinitionCommand
	savedUnitTemplate                   catalogapp.SaveProductUnitTemplateCommand
	savedProductionConfig               catalogapp.SaveProductProductionConfigCommand
	deactivatedTemplate                 catalogapp.DeactivateGradientTemplateCommand
	boundTemplate                       catalogapp.BindCategoryGradientTemplateCommand
	updated                             catalogapp.UpdateProductBasicsCommand
	updateErr                           error
	deactivated                         catalogapp.DeactivateProductsCommand
	createdPublic                       catalogapp.CreateProductCommand
	createdSKU                          catalogapp.CreateSKUCommand
	copiedProduct                       catalogapp.CopyProductCommand
	publicUsage                         catalogapp.CustomerPublicUsageCommand
	publicUsages                        []catalogapp.CustomerPublicUsage
	customerProductAliases              []catalogapp.CustomerProductAlias
	customerAliasQuery                  catalogapp.CustomerProductAliasQuery
	savedCustomerAlias                  catalogapp.CustomerProductAliasCommand
	disabledCustomerAlias               catalogapp.DisableCustomerProductAliasCommand
	batchDisabledCustomerAliases        catalogapp.BatchDisableCustomerProductAliasesCommand
	batchCustomerAliases                catalogapp.BatchCustomerProductAliasesCommand
	aliasIndustryQuery                  catalogapp.CustomerProductAliasIndustryFieldQuery
	savedAliasIndustryFields            catalogapp.SaveCustomerProductAliasIndustryFieldsCommand
	aliasCandidates                     []catalogapp.CustomerProductAliasMigrationCandidate
	aliasCandidateQuery                 catalogapp.CustomerProductAliasMigrationCandidateQuery
	ruleTemplates                       []catalogapp.CustomerProductRuleTemplate
	ruleOverrides                       []catalogapp.CustomerProductRuleOverride
	customerRuleBindings                []catalogapp.CustomerProductRuleBinding
	classificationTemplates             []catalogapp.ProductClassificationTemplate
	productClassificationTemplateUsages []catalogapp.ProductClassificationTemplateUsage
	aliasClassificationTemplateUsages   []catalogapp.CustomerProductAliasClassificationTemplateUsage
	savedRuleTemplate                   catalogapp.SaveCustomerProductRuleTemplateCommand
	savedRuleOverride                   catalogapp.SaveCustomerProductRuleOverrideCommand
	savedRuleBinding                    catalogapp.CustomerProductRuleTemplateBindingCommand
	savedClassificationTemplate         catalogapp.SaveProductClassificationTemplateCommand
	savedClassificationCategory         catalogapp.SaveProductClassificationCategoryCommand
	savedClassificationAssignment       catalogapp.SaveProductClassificationAssignmentCommand
	savedAliasClassificationAssignment  catalogapp.SaveCustomerProductAliasClassificationAssignmentCommand
	savedProductClassificationUsage     catalogapp.SaveProductClassificationTemplateUsageCommand
	savedAliasClassificationUsage       catalogapp.SaveCustomerProductAliasClassificationTemplateUsageCommand
	createErr                           error
	createdProduct                      catalogapp.CreateCustomProductCommand
	categoryCreated                     bool
	categoryMoved                       bool
	categoryDeleted                     bool
	productAssigned                     bool
	productDerived                      bool
	categoryDerived                     bool
	templateDerived                     bool
	configTemplateDerived               bool
	templateSaved                       bool
	configTemplateSaved                 bool
	unitDefinitionSaved                 bool
	unitTemplateSaved                   bool
	productionConfigSaved               bool
	templateDeactivated                 bool
	templateBound                       bool
	productUpdated                      bool
	productsDeactivated                 bool
	publicCreated                       bool
	skuCreated                          bool
	productCopied                       bool
	productCreated                      bool
	publicUsageSaved                    bool
	customerAliasesListed               bool
	customerAliasSaved                  bool
	customerAliasDisabled               bool
	customerAliasesBatchDisabled        bool
	customerAliasBatchSaved             bool
	aliasIndustryFieldsListed           bool
	aliasIndustryFieldsSaved            bool
	aliasCandidatesListed               bool
	ruleTemplateSaved                   bool
	ruleOverrideSaved                   bool
	ruleBindingSaved                    bool
	classificationTemplateSaved         bool
	classificationTemplateDeleted       bool
	classificationCategorySaved         bool
	classificationCategoryDeleted       bool
	classificationAssignmentSaved       bool
	aliasClassificationAssignmentSaved  bool
	productClassificationUsageSaved     bool
	aliasClassificationUsageSaved       bool
}

func (r *productSettingsRepo) ListProducts(ctx context.Context) ([]catalogapp.Product, error) {
	return r.products, nil
}

func (r *productSettingsRepo) GetProduct(ctx context.Context, id int64) (*catalogapp.Product, error) {
	for i := range r.products {
		if r.products[i].ID == id {
			return &r.products[i], nil
		}
	}
	return nil, nil
}

func (r *productSettingsRepo) ReplacePriceTiers(ctx context.Context, cmd catalogapp.ReplacePriceTiersCommand) error {
	r.updated = catalogapp.UpdateProductBasicsCommand{ProductID: cmd.ProductID, ProductKind: cmd.ProductKind}
	return nil
}

func (r *productSettingsRepo) UpdateProductBasics(ctx context.Context, cmd catalogapp.UpdateProductBasicsCommand) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updated = cmd
	r.productUpdated = true
	for i := range r.products {
		if r.products[i].ID == cmd.ProductID {
			if cmd.Name != "" {
				r.products[i].Name = cmd.Name
			}
			r.products[i].ProductKind = cmd.ProductKind
			r.products[i].Remark = cmd.Remark
			r.products[i].GreenBeanType = cmd.GreenBeanType
			r.products[i].GreenBeanBomProductID = cmd.GreenBeanBomProductID
			r.products[i].RoastLevel = cmd.RoastLevel
			r.products[i].DefaultPrice = cmd.DefaultPrice
			r.products[i].RetailPrice100G = cmd.RetailPrice100G
			r.products[i].RetailPrice200G = cmd.RetailPrice200G
			r.products[i].RetailPrice227G = cmd.RetailPrice227G
			r.products[i].RetailPrice250G = cmd.RetailPrice250G
			r.products[i].YieldRate = cmd.YieldRate
			r.products[i].MarginRateOverride = cmd.MarginRateOverride
			r.products[i].GradientTemplateIDOverride = cmd.GradientTemplateIDOverride
			r.products[i].OperationTemplateIDOverride = cmd.OperationTemplateIDOverride
			r.products[i].UnitRuleOverrideJSON = cmd.UnitRuleOverrideJSON
		}
	}
	return nil
}

func (r *productSettingsRepo) DeactivateProducts(ctx context.Context, cmd catalogapp.DeactivateProductsCommand) error {
	r.deactivated = cmd
	r.productsDeactivated = true
	return nil
}

func (r *productSettingsRepo) CreateProduct(ctx context.Context, cmd catalogapp.CreateProductCommand) (catalogapp.Product, error) {
	if r.createErr != nil {
		return catalogapp.Product{}, r.createErr
	}
	r.createdPublic = cmd
	r.publicCreated = true
	return catalogapp.Product{
		ID:                    77,
		Name:                  cmd.Name,
		Remark:                cmd.Remark,
		RoastLevel:            cmd.RoastLevel,
		ProductKind:           cmd.ProductKind,
		GreenBeanType:         cmd.GreenBeanType,
		GreenBeanBomProductID: cmd.GreenBeanBomProductID,
		DripBagGrams:          cmd.DripBagGrams,
		DripBoxBagCount:       cmd.DripBoxBagCount,
		AllowFulfillmentOrder: cmd.AllowFulfillmentOrder,
		AllowMallOrder:        cmd.AllowMallOrder,
		SalesUnits:            cmd.SalesUnits,
		DefaultPrice:          cmd.DefaultPrice,
		YieldRate:             cmd.YieldRate,
		Visibility:            "public",
		BomItemCount:          0,
		CustomerID:            0,
		BaseProductID:         0,
	}, nil
}

func (r *productSettingsRepo) CopyProduct(ctx context.Context, cmd catalogapp.CopyProductCommand) (catalogapp.Product, error) {
	r.copiedProduct = cmd
	r.productCopied = true
	return catalogapp.Product{
		ID:                      913,
		Name:                    "速溶10条盒装 复制",
		Remark:                  "复制配置",
		ProductConfigTemplateID: 301,
		ProductionBomID:         11,
		ProductionBomVersionID:  22,
		ProcessRouteID:          33,
		ExpectedLossRate:        0.08,
		Visibility:              "public",
		Active:                  true,
	}, nil
}

func (r *productSettingsRepo) CreateSKU(ctx context.Context, cmd catalogapp.CreateSKUCommand) (catalogapp.Product, error) {
	r.createdSKU = cmd
	r.skuCreated = true
	visibility := "public"
	if cmd.CustomerID > 0 {
		visibility = "customer_only"
	}
	return catalogapp.Product{
		ID:                912,
		Name:              cmd.Name,
		Remark:            cmd.Remark,
		CustomerID:        cmd.CustomerID,
		ProductCategoryID: cmd.ProductSubtypeCategoryID,
		SpecialAttrsJSON:  cmd.SpecialAttrsJSON,
		Visibility:        visibility,
	}, nil
}

func (r *productSettingsRepo) ListProductCategories(ctx context.Context) ([]catalogapp.ProductCategory, error) {
	return r.categories, nil
}

func (r *productSettingsRepo) ListProductProductionConfigs(ctx context.Context) ([]catalogapp.ProductProductionConfig, error) {
	return r.productProductionConfigs, nil
}

func (r *productSettingsRepo) GetProductProductionConfig(ctx context.Context, productID int64) (catalogapp.ProductProductionConfig, error) {
	for _, row := range r.productProductionConfigs {
		if row.ProductID == productID {
			return row, nil
		}
	}
	return catalogapp.ProductProductionConfig{ProductID: productID}, nil
}

func (r *productSettingsRepo) SaveProductProductionConfig(ctx context.Context, cmd catalogapp.SaveProductProductionConfigCommand) (catalogapp.ProductProductionConfig, error) {
	r.savedProductionConfig = cmd
	r.productionConfigSaved = true
	row := catalogapp.ProductProductionConfig{
		ProductID:               cmd.ProductID,
		ProductionBomID:         cmd.ProductionBomID,
		ProductionBomVersionID:  cmd.ProductionBomVersionID,
		ProcessRouteID:          cmd.ProcessRouteID,
		IndustryFieldTemplateID: cmd.IndustryFieldTemplateID,
		ExpectedLossRate:        cmd.ExpectedLossRate,
		Note:                    cmd.Note,
		Fields:                  cmd.Fields,
	}
	r.productProductionConfigs = append(r.productProductionConfigs, row)
	return row, nil
}

func (r *productSettingsRepo) ListGradientTemplates(ctx context.Context) ([]catalogapp.GradientTemplate, error) {
	return r.gradientTemplates, nil
}

func (r *productSettingsRepo) ListProductConfigTemplates(ctx context.Context) ([]catalogapp.ProductConfigTemplate, error) {
	return r.productConfigTemplates, nil
}

func (r *productSettingsRepo) ListProductUnitDefinitions(ctx context.Context) ([]catalogapp.ProductUnitDefinition, error) {
	return r.productUnitDefinitions, nil
}

func (r *productSettingsRepo) ListProductUnitTemplates(ctx context.Context) ([]catalogapp.ProductUnitTemplate, error) {
	return r.productUnitTemplates, nil
}

func (r *productSettingsRepo) ListCustomerPublicUsages(ctx context.Context) ([]catalogapp.CustomerPublicUsage, error) {
	return r.publicUsages, nil
}

func (r *productSettingsRepo) ListCustomerProductRuleTemplates(ctx context.Context) ([]catalogapp.CustomerProductRuleTemplate, error) {
	return r.ruleTemplates, nil
}

func (r *productSettingsRepo) ListCustomerProductRuleOverrides(ctx context.Context) ([]catalogapp.CustomerProductRuleOverride, error) {
	return r.ruleOverrides, nil
}

func (r *productSettingsRepo) ListCustomerProductRuleBindings(ctx context.Context) ([]catalogapp.CustomerProductRuleBinding, error) {
	return r.customerRuleBindings, nil
}

func (r *productSettingsRepo) SaveGradientTemplate(ctx context.Context, cmd catalogapp.SaveGradientTemplateCommand) (catalogapp.GradientTemplate, error) {
	r.savedTemplate = cmd
	r.templateSaved = true
	return catalogapp.GradientTemplate{ID: 77, Name: cmd.Name, DisplayUnit: cmd.DisplayUnit, Active: true, Tiers: cmd.Tiers}, nil
}

func (r *productSettingsRepo) SaveProductConfigTemplate(ctx context.Context, cmd catalogapp.SaveProductConfigTemplateCommand) (catalogapp.ProductConfigTemplate, error) {
	r.savedConfigTemplate = cmd
	r.configTemplateSaved = true
	return catalogapp.ProductConfigTemplate{
		ID:                     377,
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

func (r *productSettingsRepo) SaveProductUnitDefinition(ctx context.Context, cmd catalogapp.SaveProductUnitDefinitionCommand) (catalogapp.ProductUnitDefinition, error) {
	r.savedUnitDefinition = cmd
	r.unitDefinitionSaved = true
	return catalogapp.ProductUnitDefinition{Code: cmd.Code, Name: cmd.Name, UnitType: cmd.UnitType, AllowDecimal: cmd.AllowDecimal, Active: true}, nil
}

func (r *productSettingsRepo) SaveProductUnitTemplate(ctx context.Context, cmd catalogapp.SaveProductUnitTemplateCommand) (catalogapp.ProductUnitTemplate, error) {
	r.savedUnitTemplate = cmd
	r.unitTemplateSaved = true
	return catalogapp.ProductUnitTemplate{
		ID:                 912,
		Name:               cmd.Name,
		InventoryUnit:      cmd.InventoryUnit,
		QuoteUnit:          cmd.QuoteUnit,
		OrderUnit:          cmd.OrderUnit,
		UnitConversionJSON: cmd.UnitConversionJSON,
		IntegerUnit:        cmd.IntegerUnit,
		Active:             true,
	}, nil
}

func (r *productSettingsRepo) DeactivateGradientTemplate(ctx context.Context, cmd catalogapp.DeactivateGradientTemplateCommand) error {
	r.deactivatedTemplate = cmd
	r.templateDeactivated = true
	return nil
}

func (r *productSettingsRepo) BindCategoryGradientTemplate(ctx context.Context, cmd catalogapp.BindCategoryGradientTemplateCommand) error {
	r.boundTemplate = cmd
	r.templateBound = true
	return nil
}

func (r *productSettingsRepo) SaveProductCategory(ctx context.Context, cmd catalogapp.SaveProductCategoryCommand) (catalogapp.ProductCategory, error) {
	r.savedCategory = cmd
	r.categoryCreated = true
	return catalogapp.ProductCategory{
		ID:                      99,
		ParentID:                cmd.ParentID,
		CustomerID:              cmd.CustomerID,
		Name:                    cmd.Name,
		Position:                cmd.Position,
		ProductConfigTemplateID: cmd.ProductConfigTemplateID,
		GradientTemplateID:      cmd.GradientTemplateID,
		OperationTemplateID:     cmd.OperationTemplateID,
		PriceListRuleJSON:       cmd.PriceListRuleJSON,
		InventoryUnit:           cmd.InventoryUnit,
		QuoteUnit:               cmd.QuoteUnit,
		OrderUnit:               cmd.OrderUnit,
		UnitConversionJSON:      cmd.UnitConversionJSON,
		IntegerUnit:             cmd.IntegerUnit,
	}, nil
}

func (r *productSettingsRepo) MoveProductCategory(ctx context.Context, cmd catalogapp.MoveProductCategoryCommand) error {
	r.movedCategory = cmd
	r.categoryMoved = true
	return nil
}

func (r *productSettingsRepo) DeleteProductCategory(ctx context.Context, cmd catalogapp.DeleteProductCategoryCommand) error {
	r.deletedCategory = cmd
	r.categoryDeleted = true
	return nil
}

func (r *productSettingsRepo) AssignProductCategory(ctx context.Context, cmd catalogapp.AssignProductCategoryCommand) (catalogapp.AssignProductCategoryResult, error) {
	r.assigned = cmd
	r.productAssigned = true
	if r.assignResult.ProductID == 0 {
		r.assignResult = catalogapp.AssignProductCategoryResult{ProductID: cmd.ProductID, CategoryID: cmd.CategoryID}
	}
	return r.assignResult, nil
}

func (r *productSettingsRepo) CreateCustomProduct(ctx context.Context, cmd catalogapp.CreateCustomProductCommand) (catalogapp.Product, error) {
	r.createdProduct = cmd
	r.productCreated = true
	return catalogapp.Product{
		ID:                    88,
		Name:                  cmd.Name,
		Remark:                cmd.Remark,
		ProductKind:           cmd.ProductKind,
		GreenBeanType:         cmd.GreenBeanType,
		GreenBeanBomProductID: cmd.GreenBeanBomProductID,
		RoastLevel:            cmd.RoastLevel,
		DripBagGrams:          cmd.DripBagGrams,
		DripBoxBagCount:       cmd.DripBoxBagCount,
		CustomerID:            cmd.CustomerID,
		BaseProductID:         cmd.BaseProductID,
		Visibility:            "customer_only",
		CustomType:            cmd.CustomType,
	}, nil
}

func (r *productSettingsRepo) DeriveCustomerProduct(ctx context.Context, cmd catalogapp.DeriveCustomerProductCommand) (catalogapp.Product, error) {
	r.derivedProduct = cmd
	r.productDerived = true
	return catalogapp.Product{
		ID:                188,
		Name:              cmd.Name,
		CustomerID:        cmd.CustomerID,
		BaseProductID:     cmd.BaseProductID,
		ProductCategoryID: cmd.CategoryID,
		Visibility:        "customer_only",
		CustomType:        "public_sku_alias",
	}, nil
}

func (r *productSettingsRepo) DeriveProductCategory(ctx context.Context, cmd catalogapp.DeriveProductCategoryCommand) (catalogapp.ProductCategory, error) {
	r.derivedCategory = cmd
	r.categoryDerived = true
	return catalogapp.ProductCategory{
		ID:               199,
		CustomerID:       cmd.CustomerID,
		SourceCategoryID: cmd.SourceCategoryID,
		Name:             "岩师傅定制",
		Level:            2,
		Position:         1,
		TemplateState:    "derived_from_public",
	}, nil
}

func (r *productSettingsRepo) DeriveGradientTemplate(ctx context.Context, cmd catalogapp.DeriveGradientTemplateCommand) (catalogapp.GradientTemplate, error) {
	r.derivedTemplate = cmd
	r.templateDerived = true
	return catalogapp.GradientTemplate{
		ID:               288,
		Name:             cmd.Name,
		CustomerID:       cmd.CustomerID,
		SourceTemplateID: cmd.SourceTemplateID,
		TemplateState:    "derived_from_public",
		Active:           true,
	}, nil
}

func (r *productSettingsRepo) DeriveProductConfigTemplate(ctx context.Context, cmd catalogapp.DeriveProductConfigTemplateCommand) (catalogapp.ProductConfigTemplate, error) {
	r.derivedConfig = cmd
	r.configTemplateDerived = true
	return catalogapp.ProductConfigTemplate{
		ID:               388,
		Name:             cmd.Name,
		CustomerID:       cmd.CustomerID,
		SourceTemplateID: cmd.SourceTemplateID,
		TemplateState:    "derived_from_public",
		Active:           true,
	}, nil
}

func (r *productSettingsRepo) ListProductClassificationTemplates(ctx context.Context) ([]catalogapp.ProductClassificationTemplate, error) {
	if r.classificationTemplates != nil {
		return r.classificationTemplates, nil
	}
	return []catalogapp.ProductClassificationTemplate{{
		ID:         501,
		CustomerID: 0,
		Name:       "默认分类模板",
		Active:     true,
		Categories: []catalogapp.ProductClassificationCategory{{
			ID:         502,
			TemplateID: 501,
			Name:       "未分类",
			SortOrder:  0,
			Active:     true,
		}},
	}}, nil
}

func (r *productSettingsRepo) ListProductClassificationTemplateUsages(ctx context.Context) ([]catalogapp.ProductClassificationTemplateUsage, error) {
	if r.productClassificationTemplateUsages != nil {
		return r.productClassificationTemplateUsages, nil
	}
	return []catalogapp.ProductClassificationTemplateUsage{}, nil
}

func (r *productSettingsRepo) SaveProductClassificationTemplateUsage(ctx context.Context, cmd catalogapp.SaveProductClassificationTemplateUsageCommand) (catalogapp.ProductClassificationTemplateUsage, error) {
	r.productClassificationUsageSaved = true
	r.savedProductClassificationUsage = cmd
	return catalogapp.ProductClassificationTemplateUsage{ClassificationTemplateID: cmd.ClassificationTemplateID, Active: true, SortOrder: cmd.SortOrder}, nil
}

func (r *productSettingsRepo) DeleteProductClassificationTemplateUsage(ctx context.Context, cmd catalogapp.DeleteProductClassificationTemplateUsageCommand) error {
	return nil
}

func (r *productSettingsRepo) ListCustomerProductAliasClassificationTemplateUsages(ctx context.Context, customerID int64) ([]catalogapp.CustomerProductAliasClassificationTemplateUsage, error) {
	if r.aliasClassificationTemplateUsages == nil {
		return []catalogapp.CustomerProductAliasClassificationTemplateUsage{}, nil
	}
	if customerID <= 0 {
		return r.aliasClassificationTemplateUsages, nil
	}
	out := []catalogapp.CustomerProductAliasClassificationTemplateUsage{}
	for _, row := range r.aliasClassificationTemplateUsages {
		if row.CustomerID == customerID {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *productSettingsRepo) SaveCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd catalogapp.SaveCustomerProductAliasClassificationTemplateUsageCommand) (catalogapp.CustomerProductAliasClassificationTemplateUsage, error) {
	r.aliasClassificationUsageSaved = true
	r.savedAliasClassificationUsage = cmd
	return catalogapp.CustomerProductAliasClassificationTemplateUsage{CustomerID: cmd.CustomerID, ClassificationTemplateID: cmd.ClassificationTemplateID, Active: true, SortOrder: cmd.SortOrder}, nil
}

func (r *productSettingsRepo) DeleteCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd catalogapp.DeleteCustomerProductAliasClassificationTemplateUsageCommand) error {
	return nil
}

func (r *productSettingsRepo) SaveProductClassificationTemplate(ctx context.Context, cmd catalogapp.SaveProductClassificationTemplateCommand) (catalogapp.ProductClassificationTemplate, error) {
	r.savedClassificationTemplate = cmd
	r.classificationTemplateSaved = true
	id := cmd.ID
	if id == 0 {
		id = 501
	}
	return catalogapp.ProductClassificationTemplate{ID: id, CustomerID: cmd.CustomerID, Name: cmd.Name, Active: true}, nil
}

func (r *productSettingsRepo) DeleteProductClassificationTemplate(ctx context.Context, cmd catalogapp.DeleteProductClassificationTemplateCommand) error {
	r.classificationTemplateDeleted = true
	return nil
}

func (r *productSettingsRepo) SaveProductClassificationCategory(ctx context.Context, cmd catalogapp.SaveProductClassificationCategoryCommand) (catalogapp.ProductClassificationCategory, error) {
	r.savedClassificationCategory = cmd
	r.classificationCategorySaved = true
	id := cmd.ID
	if id == 0 {
		id = 502
	}
	return catalogapp.ProductClassificationCategory{ID: id, TemplateID: cmd.TemplateID, Name: cmd.Name, SortOrder: cmd.SortOrder, Active: true}, nil
}

func (r *productSettingsRepo) DeleteProductClassificationCategory(ctx context.Context, cmd catalogapp.DeleteProductClassificationCategoryCommand) error {
	r.classificationCategoryDeleted = true
	return nil
}

func (r *productSettingsRepo) SaveProductClassificationAssignment(ctx context.Context, cmd catalogapp.SaveProductClassificationAssignmentCommand) (catalogapp.ProductClassificationAssignment, error) {
	r.savedClassificationAssignment = cmd
	r.classificationAssignmentSaved = true
	return catalogapp.ProductClassificationAssignment{TemplateID: cmd.TemplateID, CategoryID: cmd.CategoryID, ProductID: cmd.ProductID}, nil
}

func (r *productSettingsRepo) SaveCustomerProductAliasClassificationAssignment(ctx context.Context, cmd catalogapp.SaveCustomerProductAliasClassificationAssignmentCommand) (catalogapp.CustomerProductAliasClassificationAssignment, error) {
	r.savedAliasClassificationAssignment = cmd
	r.aliasClassificationAssignmentSaved = true
	return catalogapp.CustomerProductAliasClassificationAssignment{TemplateID: cmd.TemplateID, CategoryID: cmd.CategoryID, AliasID: cmd.AliasID}, nil
}

func (r *productSettingsRepo) SaveCustomerPublicUsage(ctx context.Context, cmd catalogapp.CustomerPublicUsageCommand) (catalogapp.CustomerPublicUsage, error) {
	r.publicUsage = cmd
	r.publicUsageSaved = true
	return catalogapp.CustomerPublicUsage{CustomerID: cmd.CustomerID, UsePublicSKU: cmd.UsePublicSKU, UsePublicCategories: cmd.UsePublicCategories, UsePublicGradientTemplates: cmd.UsePublicGradientTemplates}, nil
}

func (r *productSettingsRepo) EnsureFactoryCustomer(ctx context.Context, actor string) (int64, error) {
	return 9001, nil
}

func (r *productSettingsRepo) ListCustomerProductAliases(ctx context.Context, query catalogapp.CustomerProductAliasQuery) ([]catalogapp.CustomerProductAlias, error) {
	r.customerAliasQuery = query
	r.customerAliasesListed = true
	out := make([]catalogapp.CustomerProductAlias, 0, len(r.customerProductAliases))
	for _, row := range r.customerProductAliases {
		if query.CustomerID > 0 && row.CustomerID != query.CustomerID {
			continue
		}
		activeMode := query.ActiveMode
		if activeMode == "" {
			if query.ActiveOnly {
				activeMode = "active"
			} else {
				activeMode = "all"
			}
		}
		if activeMode == "active" && !row.Active {
			continue
		}
		if activeMode == "inactive" && row.Active {
			continue
		}
		if query.SearchQuery != "" {
			haystack := strings.ToLower(strings.Join([]string{row.DisplayName, row.CustomerItemCode, row.ProductCode, row.ProductName}, " "))
			if !strings.Contains(haystack, strings.ToLower(query.SearchQuery)) {
				continue
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func (r *productSettingsRepo) SaveCustomerProductAlias(ctx context.Context, cmd catalogapp.CustomerProductAliasCommand) (catalogapp.CustomerProductAlias, error) {
	r.savedCustomerAlias = cmd
	r.customerAliasSaved = true
	id := cmd.ID
	if id == 0 {
		id = 912
	}
	customerItemCode := cmd.CustomerItemCode
	if customerItemCode == "" {
		customerItemCode = "CPA-000912"
	}
	return catalogapp.CustomerProductAlias{
		ID:                 id,
		CustomerID:         cmd.CustomerID,
		ProductID:          cmd.ProductID,
		DisplayName:        cmd.DisplayName,
		CustomerItemCode:   customerItemCode,
		BrandName:          cmd.BrandName,
		DisplayCategoryID:  cmd.DisplayCategoryID,
		GradientTemplateID: cmd.GradientTemplateID,
		UnitTemplateID:     cmd.UnitTemplateID,
		SortOrder:          cmd.SortOrder,
		IncludeInPriceList: cmd.IncludeInPriceList,
		Active:             cmd.Active,
		Remark:             cmd.Remark,
		ProductName:        "绑定商品档案",
		CustomerName:       "Karen",
	}, nil
}

func (r *productSettingsRepo) DisableCustomerProductAlias(ctx context.Context, cmd catalogapp.DisableCustomerProductAliasCommand) error {
	r.disabledCustomerAlias = cmd
	r.customerAliasDisabled = true
	return nil
}

func (r *productSettingsRepo) BatchDisableCustomerProductAliases(ctx context.Context, cmd catalogapp.BatchDisableCustomerProductAliasesCommand) (catalogapp.BatchDisableCustomerProductAliasesResult, error) {
	r.batchDisabledCustomerAliases = cmd
	r.customerAliasesBatchDisabled = true
	return catalogapp.BatchDisableCustomerProductAliasesResult{DisabledCount: len(cmd.IDs), Disabled: cmd.IDs}, nil
}

func (r *productSettingsRepo) ListCustomerProductAliasIndustryFields(ctx context.Context, query catalogapp.CustomerProductAliasIndustryFieldQuery) ([]catalogapp.ProductProductionConfigField, error) {
	r.aliasIndustryQuery = query
	r.aliasIndustryFieldsListed = true
	return []catalogapp.ProductProductionConfigField{{
		FieldKey:        "roast_level",
		Label:           "烘焙度",
		FieldType:       "select",
		ValueText:       "深烘",
		OptionsJSON:     `["浅烘","中烘","深烘"]`,
		ShowInPriceList: true,
		SortOrder:       1,
	}}, nil
}

func (r *productSettingsRepo) SaveCustomerProductAliasIndustryFields(ctx context.Context, cmd catalogapp.SaveCustomerProductAliasIndustryFieldsCommand) ([]catalogapp.ProductProductionConfigField, error) {
	r.savedAliasIndustryFields = cmd
	r.aliasIndustryFieldsSaved = true
	return cmd.Fields, nil
}

func (r *productSettingsRepo) BatchCreateCustomerProductAliases(ctx context.Context, cmd catalogapp.BatchCustomerProductAliasesCommand) (catalogapp.BatchCustomerProductAliasesResult, error) {
	r.batchCustomerAliases = cmd
	r.customerAliasBatchSaved = true
	created := make([]catalogapp.CustomerProductAlias, 0)
	skipped := make([]catalogapp.CustomerProductAliasBatchSkipped, 0)
	seenExisting := map[int64]bool{}
	for _, alias := range r.customerProductAliases {
		if alias.CustomerID == cmd.CustomerID && alias.Active {
			seenExisting[alias.ProductID] = true
		}
	}
	for _, productID := range cmd.ProductIDs {
		if seenExisting[productID] {
			skipped = append(skipped, catalogapp.CustomerProductAliasBatchSkipped{ProductID: productID, Reason: "exists"})
			continue
		}
		created = append(created, catalogapp.CustomerProductAlias{
			ID:                 int64(1000 + len(created)),
			CustomerID:         cmd.CustomerID,
			ProductID:          productID,
			DisplayName:        "商品档案",
			CustomerItemCode:   "CPA-001000",
			BrandName:          cmd.BrandName,
			DisplayCategoryID:  cmd.DisplayCategoryID,
			IncludeInPriceList: cmd.IncludeInPriceList,
			Active:             true,
		})
	}
	return catalogapp.BatchCustomerProductAliasesResult{
		CreatedCount: len(created),
		SkippedCount: len(skipped),
		Created:      created,
		Skipped:      skipped,
	}, nil
}

func (r *productSettingsRepo) ListCustomerProductAliasMigrationCandidates(ctx context.Context, query catalogapp.CustomerProductAliasMigrationCandidateQuery) ([]catalogapp.CustomerProductAliasMigrationCandidate, error) {
	r.aliasCandidateQuery = query
	r.aliasCandidatesListed = true
	out := make([]catalogapp.CustomerProductAliasMigrationCandidate, 0, len(r.aliasCandidates))
	for _, row := range r.aliasCandidates {
		if query.CustomerID > 0 && row.CustomerID != query.CustomerID {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func (r *productSettingsRepo) SaveCustomerProductRuleTemplate(ctx context.Context, cmd catalogapp.SaveCustomerProductRuleTemplateCommand) (catalogapp.CustomerProductRuleTemplate, error) {
	r.savedRuleTemplate = cmd
	r.ruleTemplateSaved = true
	return catalogapp.CustomerProductRuleTemplate{ID: 501, CustomerID: cmd.CustomerID, Name: cmd.Name, Active: true, Items: cmd.Items}, nil
}

func (r *productSettingsRepo) SaveCustomerProductRuleOverride(ctx context.Context, cmd catalogapp.SaveCustomerProductRuleOverrideCommand) (catalogapp.CustomerProductRuleOverride, error) {
	r.savedRuleOverride = cmd
	r.ruleOverrideSaved = true
	return catalogapp.CustomerProductRuleOverride{
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

func (r *productSettingsRepo) BindCustomerProductRuleTemplate(ctx context.Context, cmd catalogapp.CustomerProductRuleTemplateBindingCommand) (catalogapp.CustomerProductRuleBinding, error) {
	r.savedRuleBinding = cmd
	r.ruleBindingSaved = true
	return catalogapp.CustomerProductRuleBinding{CustomerID: cmd.CustomerID, TemplateID: cmd.TemplateID}, nil
}

func TestCustomerProductAliasAPIsListSaveAndDisableCustomerNames(t *testing.T) {
	repo := &productSettingsRepo{
		customerProductAliases: []catalogapp.CustomerProductAlias{{
			ID:                  11,
			CustomerID:          42,
			CustomerName:        "Karen",
			ProductID:           88,
			ProductName:         "精品意式拼配",
			ProductCode:         "SKU-000088",
			ProductActive:       false,
			DisplayName:         "Karen 精品拼配",
			CustomerItemCode:    "KAREN-ESP",
			BrandName:           "",
			DisplayCategoryID:   7,
			DisplayCategoryName: "商用批发",
			GradientTemplateID:  18,
			UnitTemplateID:      22,
			SortOrder:           20,
			IncludeInPriceList:  true,
			Active:              true,
			Remark:              "贴牌只改名字",
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/customer-product-aliases?customer_id=42&active=all", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET aliases status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"rows"`, `"display_name":"Karen 精品拼配"`, `"customer_item_code":"KAREN-ESP"`, `"product_id":88`, `"product_code":"SKU-000088"`, `"product_active":false`, `"gradient_template_id":18`, `"unit_template_id":22`, `"include_in_price_list":true`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("customer product aliases response missing %s: %s", want, rec.Body.String())
		}
	}
	if !repo.customerAliasesListed || repo.customerAliasQuery.CustomerID != 42 || repo.customerAliasQuery.ActiveOnly {
		t.Fatalf("alias query = %+v listed=%v", repo.customerAliasQuery, repo.customerAliasesListed)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-product-aliases", bytes.NewBufferString(`{
		"customer_id":42,
		"product_id":88,
		"display_name":"Karen 精品拼配",
		"brand_name":"",
		"display_category_id":7,
		"gradient_template_id":18,
		"unit_template_id":22,
		"sort_order":20,
		"include_in_price_list":true,
		"active":true,
		"remark":"贴牌只改名字"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST alias status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.customerAliasSaved || repo.savedCustomerAlias.CustomerID != 42 || repo.savedCustomerAlias.ProductID != 88 || repo.savedCustomerAlias.DisplayName != "Karen 精品拼配" || repo.savedCustomerAlias.CustomerItemCode != "" || repo.savedCustomerAlias.GradientTemplateID != 18 || repo.savedCustomerAlias.UnitTemplateID != 22 || !repo.savedCustomerAlias.IncludeInPriceList {
		t.Fatalf("save alias command = %+v saved=%v", repo.savedCustomerAlias, repo.customerAliasSaved)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"customer_item_code":"CPA-000912"`)) {
		t.Fatalf("POST alias response should include generated customer item code: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/customer-product-aliases/11", bytes.NewBufferString(`{
		"customer_id":42,
		"product_id":88,
		"display_name":"Karen 改名拼配",
		"gradient_template_id":19,
		"unit_template_id":23,
		"include_in_price_list":false,
		"active":true
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT alias status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.savedCustomerAlias.ID != 11 || repo.savedCustomerAlias.CustomerItemCode != "" || repo.savedCustomerAlias.GradientTemplateID != 19 || repo.savedCustomerAlias.UnitTemplateID != 23 || repo.savedCustomerAlias.IncludeInPriceList {
		t.Fatalf("update alias command = %+v", repo.savedCustomerAlias)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-product-aliases/11/disable", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable alias status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.customerAliasDisabled || repo.disabledCustomerAlias.ID != 11 {
		t.Fatalf("disable alias command = %+v disabled=%v", repo.disabledCustomerAlias, repo.customerAliasDisabled)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/customer-product-aliases?customer_id=42&active=inactive&q=甜感", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET inactive aliases status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.customerAliasQuery.ActiveMode != "inactive" || repo.customerAliasQuery.SearchQuery != "甜感" {
		t.Fatalf("alias query should carry active mode and search query: %+v", repo.customerAliasQuery)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-product-aliases/batch-disable", bytes.NewBufferString(`{"ids":[11,12,12,0]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch disable alias status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.customerAliasesBatchDisabled || !reflect.DeepEqual(repo.batchDisabledCustomerAliases.IDs, []int64{11, 12}) {
		t.Fatalf("batch disable command = %+v disabled=%v", repo.batchDisabledCustomerAliases, repo.customerAliasesBatchDisabled)
	}
}

func TestCustomerProductAliasIndustryFieldAPIsListAndSaveOverrides(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/customer-product-aliases/11/industry-fields", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET alias industry fields status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.aliasIndustryFieldsListed || repo.aliasIndustryQuery.AliasID != 11 {
		t.Fatalf("alias industry query = %+v listed=%v", repo.aliasIndustryQuery, repo.aliasIndustryFieldsListed)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"field_key":"roast_level"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"value_text":"深烘"`)) {
		t.Fatalf("alias industry field response missing field values: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/customer-product-aliases/11/industry-fields", bytes.NewBufferString(`{"fields":[{"field_key":"roast_level","value_text":"中烘"}]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT alias industry fields status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.aliasIndustryFieldsSaved || repo.savedAliasIndustryFields.AliasID != 11 || len(repo.savedAliasIndustryFields.Fields) != 1 || repo.savedAliasIndustryFields.Fields[0].ValueText != "中烘" {
		t.Fatalf("saved alias industry fields = %+v saved=%v", repo.savedAliasIndustryFields, repo.aliasIndustryFieldsSaved)
	}
}

func TestCustomerProductAliasBatchAPICreatesAndSkipsExistingCustomerNames(t *testing.T) {
	repo := &productSettingsRepo{
		customerProductAliases: []catalogapp.CustomerProductAlias{{
			ID:         11,
			CustomerID: 42,
			ProductID:  88,
			Active:     true,
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/customer-product-aliases/batch", bytes.NewBufferString(`{
		"customer_id":42,
		"product_ids":[88,89,89,90],
		"include_in_price_list":true,
		"brand_name":"",
		"display_category_id":7,
		"classification_template_id":501
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST batch alias status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.customerAliasBatchSaved || repo.batchCustomerAliases.CustomerID != 42 || !reflect.DeepEqual(repo.batchCustomerAliases.ProductIDs, []int64{88, 89, 90}) || !repo.batchCustomerAliases.IncludeInPriceList || repo.batchCustomerAliases.ClassificationTemplateID != 0 {
		t.Fatalf("batch alias command=%+v saved=%v", repo.batchCustomerAliases, repo.customerAliasBatchSaved)
	}
	for _, want := range []string{`"created_count":2`, `"skipped_count":1`, `"reason":"exists"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("batch alias response missing %s: %s", want, rec.Body.String())
		}
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"customer_item_code":"SKU"`)) {
		t.Fatalf("batch alias response must not copy product code as customer item code: %s", rec.Body.String())
	}
}

func TestClassificationTemplateUsageAPIsExposePageLevelTabs(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-classification-template-usages/products", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET product template usages status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-template-usages/products", bytes.NewBufferString(`{"classification_template_id":501,"sort_order":20}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST product template usage status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/product-classification-template-usages/customer-aliases?customer_id=42", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET alias template usages status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-template-usages/customer-aliases", bytes.NewBufferString(`{"customer_id":42,"classification_template_id":501,"sort_order":30}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST alias template usage status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProductClassificationTemplateAPIsSaveCategoriesAndAssignments(t *testing.T) {
	repo := &productSettingsRepo{
		classificationTemplates: []catalogapp.ProductClassificationTemplate{{
			ID:         501,
			CustomerID: 42,
			Name:       "Karen 分类模板",
			Active:     true,
			Categories: []catalogapp.ProductClassificationCategory{{
				ID:         502,
				TemplateID: 501,
				Name:       "精品拼配",
				SortOrder:  1,
				Active:     true,
			}},
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-classification-templates", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"Karen 分类模板"`)) {
		t.Fatalf("GET classification templates status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-templates", bytes.NewBufferString(`{"customer_id":42,"name":"客户侧价格表分类","sort_order":2,"gradient_template_id":91,"unit_template_id":92}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !repo.classificationTemplateSaved || repo.savedClassificationTemplate.CustomerID != 0 || repo.savedClassificationTemplate.Name != "客户侧价格表分类" || repo.savedClassificationTemplate.GradientTemplateID != 91 || repo.savedClassificationTemplate.UnitTemplateID != 92 {
		t.Fatalf("POST classification template status=%d body=%s cmd=%+v saved=%v", rec.Code, rec.Body.String(), repo.savedClassificationTemplate, repo.classificationTemplateSaved)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-template-categories", bytes.NewBufferString(`{"template_id":501,"name":"新品","level":1,"sort_order":3,"gradient_template_id":93,"unit_template_id":94}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !repo.classificationCategorySaved || repo.savedClassificationCategory.TemplateID != 501 || repo.savedClassificationCategory.Name != "新品" || repo.savedClassificationCategory.GradientTemplateID != 93 || repo.savedClassificationCategory.UnitTemplateID != 94 {
		t.Fatalf("POST classification category status=%d body=%s cmd=%+v saved=%v", rec.Code, rec.Body.String(), repo.savedClassificationCategory, repo.classificationCategorySaved)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-assignments/products", bytes.NewBufferString(`{"product_id":88,"template_id":501,"category_id":502,"sort_order":4}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !repo.classificationAssignmentSaved || repo.savedClassificationAssignment.ProductID != 88 || repo.savedClassificationAssignment.CategoryID != 502 {
		t.Fatalf("POST product classification assignment status=%d body=%s cmd=%+v saved=%v", rec.Code, rec.Body.String(), repo.savedClassificationAssignment, repo.classificationAssignmentSaved)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-assignments/products", bytes.NewBufferString(`{"product_id":88,"template_id":601,"category_id":0,"sort_order":6}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.savedClassificationAssignment.TemplateID != 601 || repo.savedClassificationAssignment.CategoryID != 0 {
		t.Fatalf("POST product classification reassignment should overwrite without move-out status=%d body=%s cmd=%+v", rec.Code, rec.Body.String(), repo.savedClassificationAssignment)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-assignments/customer-aliases", bytes.NewBufferString(`{"alias_id":77,"template_id":501,"category_id":502,"sort_order":5}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !repo.aliasClassificationAssignmentSaved || repo.savedAliasClassificationAssignment.AliasID != 77 || repo.savedAliasClassificationAssignment.CategoryID != 502 {
		t.Fatalf("POST alias classification assignment status=%d body=%s cmd=%+v saved=%v", rec.Code, rec.Body.String(), repo.savedAliasClassificationAssignment, repo.aliasClassificationAssignmentSaved)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-assignments/customer-aliases", bytes.NewBufferString(`{"alias_id":77,"template_id":602,"category_id":0,"sort_order":6}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.savedAliasClassificationAssignment.TemplateID != 602 || repo.savedAliasClassificationAssignment.CategoryID != 0 {
		t.Fatalf("POST alias classification reassignment should overwrite without move-out status=%d body=%s cmd=%+v", rec.Code, rec.Body.String(), repo.savedAliasClassificationAssignment)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/product-classification-template-categories/502?template_id=501", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !repo.classificationCategoryDeleted {
		t.Fatalf("DELETE classification category status=%d body=%s deleted=%v", rec.Code, rec.Body.String(), repo.classificationCategoryDeleted)
	}
}

func TestCustomerProductAliasMigrationCandidatesAPIIsReadOnly(t *testing.T) {
	repo := &productSettingsRepo{
		aliasCandidates: []catalogapp.CustomerProductAliasMigrationCandidate{{
			CustomerID:          42,
			ProductID:           88,
			ProductCode:         "SKU-000088",
			ProductName:         "Karen 贴牌意式",
			BaseProductID:       7,
			BaseProductCode:     "SKU-000007",
			BaseProductName:     "精品意式拼配",
			BomSourceType:       "inherit_current",
			SuggestedAction:     "convert_to_customer_product_alias",
			SuggestedReason:     "仅名称/编号/价格差异，生产定义跟随来源商品档案",
			CanAutoRecommend:    true,
			HasOwnBom:           false,
			HasProductionRecord: false,
			HasInventoryRecord:  false,
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/customer-product-aliases/migration-candidates?customer_id=42", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("migration candidates status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"rows"`,
		`"suggested_action":"convert_to_customer_product_alias"`,
		`"base_product_code":"SKU-000007"`,
		`"can_auto_recommend":true`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("migration candidates response missing %s: %s", want, rec.Body.String())
		}
	}
	if !repo.aliasCandidatesListed || repo.aliasCandidateQuery.CustomerID != 42 {
		t.Fatalf("alias candidate query=%+v listed=%v", repo.aliasCandidateQuery, repo.aliasCandidatesListed)
	}
	if repo.customerAliasSaved || repo.productCreated || repo.productCopied {
		t.Fatalf("migration candidates endpoint must be read-only: aliasSaved=%v productCreated=%v productCopied=%v", repo.customerAliasSaved, repo.productCreated, repo.productCopied)
	}
}

func TestProductSettingsAPISupportsCategoryTreeAndDragAssignments(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID: 7, Name: "曲奇拼配", Remark: "奶咖主推", ProductCategoryID: 2, ProductCategoryPosition: 1, YieldRate: 0.82, BomItemCount: 2,
			ProductionBomID: 11, ProductionBomCode: "BOM-001", ProductionBomName: "精品拼配", ProductionBomVersionID: 100, ProductionBomVersionNo: "V002", LatestBomVersionID: 101, LatestBomVersionNo: "V003", IsLatestBomVersion: false,
		}, {
			ID: 8, Name: "埃塞瑰夏生豆", ProductKind: "green_bean", ProductCategoryID: 2, ProductCategoryPosition: 2, YieldRate: 1,
		}},
		categories: []catalogapp.ProductCategory{
			{ID: 1, Name: "咖啡豆", Level: 1, Position: 1, TemplateState: "public_template"},
			{ID: 2, ParentID: 1, Name: "意式拼配", Level: 2, Position: 1, GradientTemplateID: 9, TemplateState: "public_template"},
			{ID: 10, Name: "客户A分类", Level: 1, Position: 1, CustomerID: 3},
			{ID: 11, ParentID: 10, Name: "客户A二级", Level: 2, Position: 1, CustomerID: 3},
		},
		gradientTemplates: []catalogapp.GradientTemplate{{
			ID: 9, Name: "工厂量单模板", DisplayUnit: "kg", Active: true, TemplateState: "public_template",
			Tiers: []catalogapp.GradientTemplateTier{{ID: 91, Label: "24-49kg", MinWeightG: 24000, MaxWeightG: f64(49000), MarginRate: 0.175, Position: 1}},
		}},
		publicUsages: []catalogapp.CustomerPublicUsage{{CustomerID: 3, UsePublicSKU: true, UsePublicCategories: false, UsePublicGradientTemplates: true}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"categories"`, `"children"`, `"products"`, `"gradient_templates"`, `"customer_public_usages"`, `"use_public_sku":true`, `"use_public_categories":false`, `"use_public_gradient_templates":true`, `"gradient_template_id":9`, `"name":"工厂量单模板"`, `"display_unit":"kg"`, `"template_state":"public_template"`, `"number":1`, `"name":"咖啡豆"`, `"name":"意式拼配"`, `"name":"客户A分类"`, `"customer_id":3`, `"name":"曲奇拼配"`, `"remark":"奶咖主推"`, `"yield_rate":0.82`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing %s: %s", want, rec.Body.String())
		}
	}
	for _, want := range []string{`"customer_id":0`, `"base_product_id":0`, `"visibility":"public"`, `"custom_type":""`, `"bom_item_count":2`, `"production_bom_code":"BOM-001"`, `"production_bom_version_no":"V002"`, `"latest_bom_version_no":"V003"`, `"is_latest_bom_version":false`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing ownership field %s: %s", want, rec.Body.String())
		}
	}
	for _, want := range []string{`"name":"埃塞瑰夏生豆"`, `"product_kind":"green_bean"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response must preserve green bean field %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/categories", bytes.NewBufferString(`{"name":"单品豆","parent_id":1,"position":2,"customer_id":3}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST category status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.categoryCreated || repo.savedCategory.Name != "单品豆" || repo.savedCategory.ParentID != 1 || repo.savedCategory.Position != 2 || repo.savedCategory.CustomerID != 3 {
		t.Fatalf("category command = %+v created=%v", repo.savedCategory, repo.categoryCreated)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/categories/2/move", bytes.NewBufferString(`{"parent_id":1,"position":1}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST move category status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.categoryMoved || repo.movedCategory.ID != 2 || repo.movedCategory.ParentID != 1 || repo.movedCategory.Position != 1 {
		t.Fatalf("move category command = %+v moved=%v", repo.movedCategory, repo.categoryMoved)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/product-settings/categories/2", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE category status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.categoryDeleted || repo.deletedCategory.ID != 2 {
		t.Fatalf("delete category command = %+v deleted=%v", repo.deletedCategory, repo.categoryDeleted)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/products/7/category", bytes.NewBufferString(`{"category_id":2,"position":3}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST assign product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productAssigned || repo.assigned.ProductID != 7 || repo.assigned.CategoryID != 2 || repo.assigned.Position != 3 {
		t.Fatalf("assign product command = %+v assigned=%v", repo.assigned, repo.productAssigned)
	}
}

func TestProductSettingsAPIExposesAndSavesSubtypeConfigAndUnitRules(t *testing.T) {
	repo := &productSettingsRepo{
		categories: []catalogapp.ProductCategory{
			{ID: 1, Name: "速溶咖啡", Level: 1, Position: 1, TemplateState: "public_template"},
			{
				ID: 2, ParentID: 1, Name: "冻干速溶", Level: 2, Position: 1, GradientTemplateID: 9,
				OperationTemplateID: 19, PriceListRuleJSON: `{"generator":"instant"}`,
				InventoryUnit: "kg", QuoteUnit: "盒", OrderUnit: "盒", UnitConversionJSON: `{"盒":{"kg":0.2}}`, IntegerUnit: true,
				TemplateState: "public_template",
			},
		},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"operation_template_id":19`,
		`"price_list_rule_json":"{\"generator\":\"instant\"}"`,
		`"inventory_unit":"kg"`,
		`"quote_unit":"盒"`,
		`"order_unit":"盒"`,
		`"unit_conversion_json":"{\"盒\":{\"kg\":0.2}}"`,
		`"integer_unit":true`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing subtype config %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/categories", bytes.NewBufferString(`{
		"name":"冻干速溶",
		"parent_id":1,
		"position":2,
		"gradient_template_id":9,
		"operation_template_id":19,
		"price_list_rule_json":"{\"generator\":\"instant\"}",
		"inventory_unit":"kg",
		"quote_unit":"盒",
		"order_unit":"盒",
		"unit_conversion_json":"{\"盒\":{\"kg\":0.2}}",
		"integer_unit":true
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST category config status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.categoryCreated || repo.savedCategory.GradientTemplateID != 9 || repo.savedCategory.OperationTemplateID != 19 || repo.savedCategory.PriceListRuleJSON != `{"generator":"instant"}` {
		t.Fatalf("category config command = %+v created=%v", repo.savedCategory, repo.categoryCreated)
	}
	if repo.savedCategory.InventoryUnit != "kg" || repo.savedCategory.QuoteUnit != "盒" || repo.savedCategory.OrderUnit != "盒" || repo.savedCategory.UnitConversionJSON != `{"盒":{"kg":0.2}}` || !repo.savedCategory.IntegerUnit {
		t.Fatalf("category unit rule command = %+v", repo.savedCategory)
	}
}

func TestProductSettingsAPIExposesAndSavesCustomerProductRules(t *testing.T) {
	repo := &productSettingsRepo{
		ruleTemplates: []catalogapp.CustomerProductRuleTemplate{{
			ID: 501, CustomerID: 42, Name: "大客户速溶规则模板", Active: true,
			Items: []catalogapp.CustomerProductRuleTemplateItem{{
				ID: 701, ProductSubtypeCategoryID: 12, GradientTemplateID: 9, OperationTemplateID: 19,
				PriceListRuleJSON: `{"generator":"instant"}`, UnitRuleJSON: `{"order_unit":"盒","integer_unit":true}`,
			}},
		}},
		ruleOverrides: []catalogapp.CustomerProductRuleOverride{{
			ID: 601, CustomerID: 42, ProductSubtypeCategoryID: 12, GradientTemplateID: 10,
			PriceListRuleJSON: `{"generator":"customer-instant"}`, UnitRuleJSON: `{"order_unit":"箱","integer_unit":true}`, Active: true,
		}},
		customerRuleBindings: []catalogapp.CustomerProductRuleBinding{{CustomerID: 42, TemplateID: 501}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"customer_product_rule_templates"`,
		`"name":"大客户速溶规则模板"`,
		`"product_subtype_category_id":12`,
		`"gradient_template_id":9`,
		`"operation_template_id":19`,
		`"customer_product_rule_overrides"`,
		`"gradient_template_id":10`,
		`"customer_product_rule_bindings"`,
		`"template_id":501`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing customer product rule field %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/customer-rule-templates", bytes.NewBufferString(`{
		"customer_id":42,
		"name":"大客户速溶规则模板",
		"items":[{
			"product_subtype_category_id":12,
			"gradient_template_id":9,
			"operation_template_id":19,
			"price_list_rule_json":"{\"generator\":\"instant\"}",
			"unit_rule_json":"{\"order_unit\":\"盒\",\"integer_unit\":true}"
		}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST customer-rule-templates status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.ruleTemplateSaved || repo.savedRuleTemplate.CustomerID != 42 || len(repo.savedRuleTemplate.Items) != 1 || repo.savedRuleTemplate.Items[0].GradientTemplateID != 9 {
		t.Fatalf("saved customer rule template = %+v saved=%v", repo.savedRuleTemplate, repo.ruleTemplateSaved)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/customer-rule-overrides", bytes.NewBufferString(`{
		"customer_id":42,
		"product_subtype_category_id":12,
		"gradient_template_id":10,
		"operation_template_id":20,
		"price_list_rule_json":"{\"generator\":\"customer-instant\"}",
		"unit_rule_json":"{\"order_unit\":\"箱\",\"integer_unit\":true}"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST customer-rule-overrides status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.ruleOverrideSaved || repo.savedRuleOverride.CustomerID != 42 || repo.savedRuleOverride.ProductSubtypeCategoryID != 12 || repo.savedRuleOverride.GradientTemplateID != 10 {
		t.Fatalf("saved customer rule override = %+v saved=%v", repo.savedRuleOverride, repo.ruleOverrideSaved)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/customers/42/rule-template", bytes.NewBufferString(`{"template_id":501}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST customer rule binding status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.ruleBindingSaved || repo.savedRuleBinding.CustomerID != 42 || repo.savedRuleBinding.TemplateID != 501 {
		t.Fatalf("saved rule binding = %+v saved=%v", repo.savedRuleBinding, repo.ruleBindingSaved)
	}
}

func TestProductSettingsAPICreatesGreenBeanProductWithBomBinding(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{ID: 7, Name: "埃塞瑰夏熟豆", ProductKind: "roasted"}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := bytes.NewBufferString(`{
		"name":"埃塞瑰夏生豆",
		"product_kind":"green_bean",
		"green_bean_type":"single_origin",
		"green_bean_bom_product_id":7
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/product-settings/products status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicCreated || repo.createdPublic.ProductKind != "green_bean" {
		t.Fatalf("created command = %+v", repo.createdPublic)
	}
	if repo.createdPublic.GreenBeanType != "single_origin" || repo.createdPublic.GreenBeanBomProductID != 7 {
		t.Fatalf("green bean binding command = %+v", repo.createdPublic)
	}
	if repo.createdPublic.DefaultPrice != 0 || len(repo.createdPublic.Tiers) != 0 {
		t.Fatalf("green bean create should not carry direct sale price fields, got default=%.2f tiers=%+v", repo.createdPublic.DefaultPrice, repo.createdPublic.Tiers)
	}
	if repo.createdPublic.RoastLevel != "" || repo.createdPublic.YieldRate != 0 {
		t.Fatalf("green bean create should not require roasted defaults, got roast=%q yield=%.2f", repo.createdPublic.RoastLevel, repo.createdPublic.YieldRate)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"product_kind":"green_bean"`)) {
		t.Fatalf("response missing product_kind: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"green_bean_type":"single_origin"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"green_bean_bom_product_id":7`)) {
		t.Fatalf("response missing green bean binding fields: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIUpdatesGreenBeanBomBinding(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{
			{
				ID:                    91,
				Name:                  "埃塞瑰夏生豆",
				ProductKind:           "green_bean",
				GreenBeanType:         "single_origin",
				GreenBeanBomProductID: 7,
			},
			{ID: 7, Name: "原绑定熟豆", ProductKind: "roasted"},
			{ID: 8, Name: "新绑定熟豆", ProductKind: "roasted"},
		},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := bytes.NewBufferString(`{
		"product_kind":"green_bean",
		"green_bean_type":"blend",
		"green_bean_bom_product_id":8,
		"default_price":135,
		"tiers":[{"spec_g":1000,"min_qty":1,"unit_price":135}]
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/products/91", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/products/91 status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productUpdated || repo.updated.ProductKind != "green_bean" {
		t.Fatalf("updated command = %+v", repo.updated)
	}
	if repo.updated.GreenBeanType != "blend" || repo.updated.GreenBeanBomProductID != 8 {
		t.Fatalf("updated green bean binding = %+v", repo.updated)
	}
	if repo.updated.DefaultPrice != 0 || repo.updated.RetailPrice227G != 0 {
		t.Fatalf("green bean update should ignore direct price fields, got %+v", repo.updated)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"product_kind":"green_bean"`)) {
		t.Fatalf("response missing product_kind: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"green_bean_type":"blend"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"green_bean_bom_product_id":8`)) {
		t.Fatalf("response missing updated green bean binding: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIUpdatesProductRemark(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID: 91, Name: "暖阳拼配", ProductKind: "roasted", RoastLevel: "中烘", Remark: "旧备注", YieldRate: 0.8,
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := bytes.NewBufferString(`{"product_kind":"roasted","roast_level":"中烘","yield_rate":0.81,"remark":"门店常用奶咖"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/products/91", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/products/91 status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productUpdated || repo.updated.Remark != "门店常用奶咖" {
		t.Fatalf("updated remark command = %+v updated=%v", repo.updated, repo.productUpdated)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"remark":"门店常用奶咖"`)) {
		t.Fatalf("response missing updated remark: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIUpdatesProductName(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID: 91, Name: "旧SKU名", ProductKind: "roasted", RoastLevel: "中烘", Remark: "旧备注", YieldRate: 0.8,
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := bytes.NewBufferString(`{"name":"芬纳定制-红酒日晒-中深烘","product_kind":"roasted","roast_level":"中烘","yield_rate":0.81,"remark":"门店常用奶咖"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/products/91", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/products/91 status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productUpdated || repo.updated.Name != "芬纳定制-红酒日晒-中深烘" {
		t.Fatalf("updated name command = %+v updated=%v", repo.updated, repo.productUpdated)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"name":"芬纳定制-红酒日晒-中深烘"`)) {
		t.Fatalf("response missing updated name: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIRejectsGreenBeanAsBomBinding(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{ID: 8, Name: "生豆 SKU", ProductKind: "green_bean"}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := bytes.NewBufferString(`{
		"name":"错误绑定生豆",
		"product_kind":"green_bean",
		"green_bean_type":"single_origin",
		"green_bean_bom_product_id":8
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/product-settings/products status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.publicCreated {
		t.Fatalf("green bean BOM binding should reject non-roasted products, created=%+v", repo.createdPublic)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("green_bean_bom_product_id must reference roasted product")) {
		t.Fatalf("response should explain roasted binding requirement: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIManagesGradientTemplatesAndCategoryBinding(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"name":"工厂量单模板","display_unit":"g227","tiers":[{"label":"2-7份","min_display_qty":2,"max_display_qty":7,"margin_rate":0.175,"position":1},{"label":"8份+","min_display_qty":8,"margin_rate":0.12,"position":2}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/pricing-gradient-templates", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.templateSaved || repo.savedTemplate.Name != "工厂量单模板" || repo.savedTemplate.DisplayUnit != "g227" || len(repo.savedTemplate.Tiers) != 2 {
		t.Fatalf("template command = %+v saved=%v", repo.savedTemplate, repo.templateSaved)
	}
	if repo.savedTemplate.Tiers[0].MinWeightG != 454 || repo.savedTemplate.Tiers[0].MaxWeightG == nil || *repo.savedTemplate.Tiers[0].MaxWeightG != 1589 || repo.savedTemplate.Tiers[0].MarginRate != 0.175 {
		t.Fatalf("template tiers = %+v", repo.savedTemplate.Tiers)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/categories/2/gradient-template", bytes.NewBufferString(`{"gradient_template_id":77}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST category template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.templateBound || repo.boundTemplate.CategoryID != 2 || repo.boundTemplate.GradientTemplateID != 77 {
		t.Fatalf("bind command = %+v bound=%v", repo.boundTemplate, repo.templateBound)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/pricing-gradient-templates/77/deactivate", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST deactivate template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.templateDeactivated || repo.deactivatedTemplate.ID != 77 {
		t.Fatalf("deactivate command = %+v deactivated=%v", repo.deactivatedTemplate, repo.templateDeactivated)
	}
}

func TestProductSettingsAPIExposesSavesAndDerivesProductConfigTemplates(t *testing.T) {
	repo := &productSettingsRepo{
		productConfigTemplates: []catalogapp.ProductConfigTemplate{{
			ID:                     301,
			CustomerID:             0,
			Name:                   "公共盒装商品配置",
			GradientTemplateID:     8,
			OperationTemplateID:    9,
			UnitTemplateID:         12,
			PriceListRuleJSON:      `{"pricing_mode":"inherit_gradient_template","display_unit":"inherit_quote_unit"}`,
			SpecialAttrsSchemaJSON: `[{"key":"roast_level","label":"烘焙度","show_in_price_list":true}]`,
			InventoryUnit:          "kg",
			QuoteUnit:              "盒",
			OrderUnit:              "盒",
			UnitConversionJSON:     `{"盒":{"kg":0.2}}`,
			IntegerUnit:            true,
			TemplateState:          "public_template",
			Active:                 true,
		}},
		categories: []catalogapp.ProductCategory{{
			ID:            1,
			Name:          "速溶咖啡",
			Level:         1,
			TemplateState: "public_template",
		}, {
			ID:                      12,
			ParentID:                1,
			Name:                    "冻干速溶",
			Level:                   2,
			ProductConfigTemplateID: 301,
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"product_config_templates"`,
		`"name":"公共盒装商品配置"`,
		`"product_config_template_id":301`,
		`"unit_template_id":12`,
		`"quote_unit":"盒"`,
		`"price_list_rule_json":"{\"pricing_mode\":\"inherit_gradient_template\",\"display_unit\":\"inherit_quote_unit\"}"`,
		`"special_attrs_schema_json":"[{\"key\":\"roast_level\",\"label\":\"烘焙度\",\"show_in_price_list\":true}]"`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/product-config-templates", bytes.NewBufferString(`{
		"customer_id":42,
		"name":"客户盒装商品配置",
		"gradient_template_id":18,
		"operation_template_id":19,
		"unit_template_id":12,
		"price_list_rule_json":"{\"pricing_mode\":\"fixed_unit_price\",\"fixed_unit_price\":15}",
		"special_attrs_schema_json":"[{\"key\":\"roast_level\",\"label\":\"烘焙度\",\"show_in_price_list\":true}]",
		"active":true
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST product config template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.configTemplateSaved || repo.savedConfigTemplate.CustomerID != 42 || repo.savedConfigTemplate.Name != "客户盒装商品配置" || repo.savedConfigTemplate.GradientTemplateID != 18 || repo.savedConfigTemplate.UnitTemplateID != 12 {
		t.Fatalf("saved config template = %+v saved=%v", repo.savedConfigTemplate, repo.configTemplateSaved)
	}
	if repo.savedConfigTemplate.PriceListRuleJSON != `{"pricing_mode":"fixed_unit_price","fixed_unit_price":15}` {
		t.Fatalf("saved price list rule json = %q", repo.savedConfigTemplate.PriceListRuleJSON)
	}
	if repo.savedConfigTemplate.SpecialAttrsSchemaJSON == "" || repo.savedConfigTemplate.SpecialAttrsSchemaJSON == "[]" {
		t.Fatalf("saved special attrs schema json = %q", repo.savedConfigTemplate.SpecialAttrsSchemaJSON)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/product-config-templates/derive", bytes.NewBufferString(`{"customer_id":42,"source_template_id":301,"name":"客户复制盒装商品配置"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST derive product config template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.configTemplateDerived || repo.derivedConfig.CustomerID != 42 || repo.derivedConfig.SourceTemplateID != 301 || repo.derivedConfig.Name != "客户复制盒装商品配置" {
		t.Fatalf("derived config template = %+v derived=%v", repo.derivedConfig, repo.configTemplateDerived)
	}
	if !repo.publicUsageSaved || repo.publicUsage.CustomerID != 42 || !repo.publicUsage.UsePublicSKU || !repo.publicUsage.UsePublicCategories {
		t.Fatalf("derive product config should enable public SKU/category reference, usage=%+v saved=%v", repo.publicUsage, repo.publicUsageSaved)
	}
	for _, want := range []string{`"template"`, `"id":388`, `"customer_id":42`, `"source_template_id":301`, `"template_state":"derived_from_public"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("derive product config response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPISupportsGlobalUnitDefinitionsAndTemplates(t *testing.T) {
	repo := &productSettingsRepo{
		productUnitDefinitions: []catalogapp.ProductUnitDefinition{{
			Code:         "盒",
			Name:         "盒",
			UnitType:     "package",
			AllowDecimal: false,
			Active:       true,
		}},
		productUnitTemplates: []catalogapp.ProductUnitTemplate{{
			ID:                 12,
			Name:               "盒装200g",
			InventoryUnit:      "kg",
			QuoteUnit:          "盒",
			OrderUnit:          "盒",
			UnitConversionJSON: `{"盒":{"kg":0.2}}`,
			IntegerUnit:        true,
			Active:             true,
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"product_unit_definitions"`,
		`"code":"盒"`,
		`"product_unit_templates"`,
		`"name":"盒装200g"`,
		`"unit_conversion_json":"{\"盒\":{\"kg\":0.2}}"`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/units", bytes.NewBufferString(`{"code":"盒","name":"盒","unit_type":"package","allow_decimal":false,"active":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST unit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.unitDefinitionSaved || repo.savedUnitDefinition.Code != "盒" || repo.savedUnitDefinition.UnitType != "package" || repo.savedUnitDefinition.AllowDecimal {
		t.Fatalf("saved unit definition = %+v saved=%v", repo.savedUnitDefinition, repo.unitDefinitionSaved)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/unit-templates", bytes.NewBufferString(`{
		"name":"盒装200g",
		"inventory_unit":"kg",
		"quote_unit":"盒",
		"order_unit":"盒",
		"unit_conversion_json":"{\"盒\":{\"kg\":0.2}}",
		"integer_unit":true,
		"active":true
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST unit template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.unitTemplateSaved || repo.savedUnitTemplate.Name != "盒装200g" || repo.savedUnitTemplate.QuoteUnit != "盒" || repo.savedUnitTemplate.UnitConversionJSON != `{"盒":{"kg":0.2}}` || !repo.savedUnitTemplate.IntegerUnit {
		t.Fatalf("saved unit template = %+v saved=%v", repo.savedUnitTemplate, repo.unitTemplateSaved)
	}
}

func TestProductSettingsAPIUpdatesProductTemplateAndProductionConfigIndustryFields(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID: 91, Name: "旧SKU名", ProductKind: "roasted", RoastLevel: "中烘", Remark: "旧备注", YieldRate: 0.8,
		}},
		productProductionConfigs: []catalogapp.ProductProductionConfig{{
			ProductID:               91,
			ProductionBomID:         12,
			ProductionBomVersionID:  1203,
			ProcessRouteID:          5,
			IndustryFieldTemplateID: 3001,
			ExpectedLossRate:        0.18,
			Fields: []catalogapp.ProductProductionConfigField{{
				FieldKey:         "roast_level",
				TemplateFieldKey: "roast_level",
				Label:            "烘焙度",
				FieldType:        "select",
				ValueText:        "深烘",
				Required:         true,
				OptionsJSON:      `["浅烘","中烘","深烘"]`,
				ShowInPriceList:  true,
				SortOrder:        1,
			}},
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET product settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"product_production_configs"`, `"industry_field_template_id":3001`, `"template_field_key":"roast_level"`, `"required":true`, `"options_json":"[\"浅烘\",\"中烘\",\"深烘\"]"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing %s: %s", want, rec.Body.String())
		}
	}

	body := bytes.NewBufferString(`{"name":"新SKU名","product_kind":"roasted","yield_rate":0.81,"remark":"门店常用奶咖","product_config_template_id":301}`)
	req = httptest.NewRequest(http.MethodPut, "/api/products/91", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productUpdated || repo.updated.ProductConfigTemplateID != 301 {
		t.Fatalf("updated product template command=%+v updated=%v", repo.updated, repo.productUpdated)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/product-production-configs/91", bytes.NewBufferString(`{
		"production_bom_id":12,
		"production_bom_version_id":1203,
		"process_route_id":5,
		"industry_field_template_id":3001,
		"expected_loss_rate":0.18,
		"note":"深烘参数",
		"fields":[{
			"field_key":"roast_level",
			"template_field_key":"roast_level",
			"label":"烘焙度",
			"field_type":"select",
			"value_text":"深烘",
			"required":true,
			"options_json":"[\"浅烘\",\"中烘\",\"深烘\"]",
			"show_in_price_list":true,
			"sort_order":1
		}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product production config status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productionConfigSaved || repo.savedProductionConfig.IndustryFieldTemplateID != 3001 || len(repo.savedProductionConfig.Fields) != 1 {
		t.Fatalf("saved production config=%+v saved=%v", repo.savedProductionConfig, repo.productionConfigSaved)
	}
	field := repo.savedProductionConfig.Fields[0]
	if field.TemplateFieldKey != "roast_level" || !field.Required || field.OptionsJSON != `["浅烘","中烘","深烘"]` || field.FieldType != "select" {
		t.Fatalf("saved production config field=%+v", field)
	}
}

func TestProductSettingsAPICreatesCustomerCustomProduct(t *testing.T) {
	repo := &productSettingsRepo{products: []catalogapp.Product{{ID: 7, Name: "橘皮乌龙", ProductKind: "roasted"}}}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"customer_id":3,"base_product_id":7,"name":"测试客户-橘皮乌龙-中深烘","remark":"客户指定口味","roast_level":"中深烘","custom_type":"public_sku_alias","copy_bom":true,"copy_price_tiers":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/custom-products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST custom product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productCreated || repo.createdProduct.CustomerID != 3 || repo.createdProduct.BaseProductID != 7 || !repo.createdProduct.CopyBOM || !repo.createdProduct.CopyPriceTiers {
		t.Fatalf("custom product command = %+v created=%v", repo.createdProduct, repo.productCreated)
	}
	if repo.createdProduct.Remark != "客户指定口味" {
		t.Fatalf("custom product remark not passed: %+v", repo.createdProduct)
	}
	for _, want := range []string{`"product"`, `"customer_id":3`, `"base_product_id":7`, `"visibility":"customer_only"`, `"custom_type":"public_sku_alias"`, `"remark":"客户指定口味"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("custom product response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPICreatesUnifiedSKUWithoutLegacyFields(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/skus", bytes.NewBufferString(`{
		"customer_id":42,
		"name":"客户盒装速溶",
		"remark":"10g/条，10条/盒",
		"product_type_category_id":7,
		"product_subtype_category_id":17,
		"product_kind":"instant_coffee",
		"custom_type":"public_sku_alias",
		"base_product_id":99,
		"special_attrs_json":"{\"roast_level\":\"中深烘\"}",
		"active":true
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/product-settings/skus status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.skuCreated || repo.createdSKU.CustomerID != 42 || repo.createdSKU.ProductTypeCategoryID != 7 || repo.createdSKU.ProductSubtypeCategoryID != 17 {
		t.Fatalf("created SKU command=%+v created=%v", repo.createdSKU, repo.skuCreated)
	}
	if repo.createdSKU.SpecialAttrsJSON != `{"roast_level":"中深烘"}` {
		t.Fatalf("created SKU special attrs=%q", repo.createdSKU.SpecialAttrsJSON)
	}
}

func TestProductSettingsAPICopiesProductArchiveAndRemovesLegacySKUCopyRoutes(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{
			{ID: 7, Name: "速溶10条盒装", ProductConfigTemplateID: 301, ProductionBomID: 11, ProductionBomVersionID: 22, ProcessRouteID: 33, ExpectedLossRate: 0.08, Active: true},
		},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products/7/copy", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("product copy status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productCopied || repo.copiedProduct.SourceProductID != 7 {
		t.Fatalf("product copy command=%+v copied=%v", repo.copiedProduct, repo.productCopied)
	}
	if !strings.Contains(rec.Body.String(), `"name":"速溶10条盒装 复制"`) || !strings.Contains(rec.Body.String(), `"production_bom_id":11`) {
		t.Fatalf("product copy response missing copied config: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/product-settings/skus/copy-options?target_customer_id=42&source_customer_id=0", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("legacy copy options route status=%d body=%s, want 404", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/skus/copy", bytes.NewBufferString(`{"target_customer_id":42,"source_customer_id":0,"source_sku_ids":[7]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("legacy SKU copy route status=%d body=%s, want 404", rec.Code, rec.Body.String())
	}
}

func TestProductSettingsAPICreatesCustomerCustomRoastWithoutBaseProduct(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"customer_id":3,"name":"测试客户-专属深烘","product_kind":"roasted","roast_level":"深烘","custom_type":"custom_roast","copy_bom":true,"copy_price_tiers":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/custom-products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST custom roast product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productCreated || repo.createdProduct.CustomerID != 3 || repo.createdProduct.BaseProductID != 0 || repo.createdProduct.CopyBOM || repo.createdProduct.CopyPriceTiers {
		t.Fatalf("custom roast command = %+v created=%v", repo.createdProduct, repo.productCreated)
	}
	for _, want := range []string{`"product"`, `"customer_id":3`, `"base_product_id":0`, `"product_kind":"roasted"`, `"custom_type":"custom_roast"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("custom roast response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPICreatesCustomerGreenBeanCustomProduct(t *testing.T) {
	repo := &productSettingsRepo{products: []catalogapp.Product{
		{ID: 8, Name: "巴拿马熟豆", ProductKind: "roasted"},
	}}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"customer_id":3,"name":"测试客户-巴拿马生豆","product_kind":"green_bean","green_bean_type":"blend","green_bean_bom_product_id":8,"custom_type":"public_sku_alias","copy_price_tiers":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/custom-products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST custom green product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productCreated || repo.createdProduct.ProductKind != "green_bean" || repo.createdProduct.GreenBeanType != "blend" || repo.createdProduct.GreenBeanBomProductID != 8 {
		t.Fatalf("custom green command = %+v created=%v", repo.createdProduct, repo.productCreated)
	}
	if repo.createdProduct.BaseProductID != 0 || repo.createdProduct.CopyPriceTiers {
		t.Fatalf("custom green command should not require base product or copied price tiers: %+v", repo.createdProduct)
	}
	for _, want := range []string{`"base_product_id":0`, `"product_kind":"green_bean"`, `"green_bean_type":"blend"`, `"green_bean_bom_product_id":8`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("custom green response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPIDerivesPublicCategoryAndProductTemplates(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/customer-categories/derive", bytes.NewBufferString(`{"customer_id":42,"source_category_id":17}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST derive category status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.categoryDerived || repo.derivedCategory.CustomerID != 42 || repo.derivedCategory.SourceCategoryID != 17 {
		t.Fatalf("derive category command = %+v derived=%v", repo.derivedCategory, repo.categoryDerived)
	}
	if !repo.publicUsageSaved || repo.publicUsage.CustomerID != 42 || !repo.publicUsage.UsePublicSKU || !repo.publicUsage.UsePublicCategories {
		t.Fatalf("derive category should enable public SKU/category reference, usage=%+v saved=%v", repo.publicUsage, repo.publicUsageSaved)
	}
	for _, want := range []string{`"category"`, `"id":199`, `"customer_id":42`, `"source_category_id":17`, `"template_state":"derived_from_public"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("derive category response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/customer-products/derive", bytes.NewBufferString(`{"customer_id":42,"base_product_id":21,"category_id":199,"name":"岩师傅初晓","copy_bom":true,"copy_price_tiers":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST derive product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productDerived || repo.derivedProduct.CustomerID != 42 || repo.derivedProduct.BaseProductID != 21 || repo.derivedProduct.CategoryID != 199 || !repo.derivedProduct.CopyBOM || !repo.derivedProduct.CopyPriceTiers {
		t.Fatalf("derive product command = %+v derived=%v", repo.derivedProduct, repo.productDerived)
	}
	for _, want := range []string{`"product"`, `"id":188`, `"customer_id":42`, `"base_product_id":21`, `"visibility":"customer_only"`, `"custom_type":"public_sku_alias"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("derive product response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPIAssignProductCategoryCarriesCustomerContext(t *testing.T) {
	repo := &productSettingsRepo{assignResult: catalogapp.AssignProductCategoryResult{
		ProductID:          188,
		CategoryID:         199,
		DerivedProductID:   188,
		DerivedCategoryID:  199,
		UsedPublicProduct:  true,
		UsedPublicCategory: true,
	}}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products/21/category", bytes.NewBufferString(`{"category_id":17,"position":3,"customer_id":42,"derive_public_category":true,"derive_public_product":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST assign with context status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productAssigned || repo.assigned.ProductID != 21 || repo.assigned.CategoryID != 17 || repo.assigned.CustomerID != 42 || !repo.assigned.DerivePublicCategory || !repo.assigned.DerivePublicProduct || repo.assigned.Position != 3 {
		t.Fatalf("assign command = %+v assigned=%v", repo.assigned, repo.productAssigned)
	}
	for _, want := range []string{`"assignment"`, `"product_id":188`, `"category_id":199`, `"derived_product_id":188`, `"derived_category_id":199`, `"used_public_product":true`, `"used_public_category":true`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("assign response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPIDerivesGradientTemplates(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/customer-gradient-templates/derive", bytes.NewBufferString(`{"customer_id":42,"source_template_id":9,"name":"岩师傅 - 工厂量单模板"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST derive template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.templateDerived || repo.derivedTemplate.CustomerID != 42 || repo.derivedTemplate.SourceTemplateID != 9 || repo.derivedTemplate.Name != "岩师傅 - 工厂量单模板" {
		t.Fatalf("derive template command = %+v derived=%v", repo.derivedTemplate, repo.templateDerived)
	}
	for _, want := range []string{`"template"`, `"id":288`, `"customer_id":42`, `"source_template_id":9`, `"template_state":"derived_from_public"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("derive template response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPISavesCustomerPublicUsage(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/customer-public-usage", bytes.NewBufferString(`{"customer_id":42,"use_public_sku":false,"use_public_categories":true,"use_public_gradient_templates":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/product-settings/customer-public-usage status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicUsageSaved || repo.publicUsage.CustomerID != 42 || repo.publicUsage.UsePublicSKU || !repo.publicUsage.UsePublicCategories || !repo.publicUsage.UsePublicGradientTemplates {
		t.Fatalf("public usage command = %+v saved=%v", repo.publicUsage, repo.publicUsageSaved)
	}
	for _, want := range []string{`"usage"`, `"customer_id":42`, `"use_public_sku":false`, `"use_public_categories":true`, `"use_public_gradient_templates":true`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("public usage response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPICreatesPublicProduct(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"name":"新公共拼配","remark":"奶咖主推","roast_level":"中深烘","default_price":88,"yield_rate":0.805}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST public product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicCreated || repo.createdPublic.Name != "新公共拼配" || repo.createdPublic.RoastLevel != "中深烘" || repo.createdPublic.DefaultPrice != 88 || repo.createdPublic.RetailPrice227G != 88 || repo.createdPublic.YieldRate != 0.805 {
		t.Fatalf("public product command = %+v created=%v", repo.createdPublic, repo.publicCreated)
	}
	if repo.createdPublic.Remark != "奶咖主推" {
		t.Fatalf("public product remark not passed: %+v", repo.createdPublic)
	}
	for _, want := range []string{`"product"`, `"name":"新公共拼配"`, `"remark":"奶咖主推"`, `"customer_id":0`, `"visibility":"public"`, `"bom_item_count":0`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("public product response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPICreatesDripBagProduct(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"name":"耶加雪菲挂耳","product_kind":"drip_bag","drip_bag_grams":10,"drip_box_bag_count":10,"allow_fulfillment_order":true,"allow_mall_order":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST drip product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicCreated || repo.createdPublic.ProductKind != "drip_bag" || repo.createdPublic.DripBagGrams != 10 || repo.createdPublic.DripBoxBagCount != 10 || !repo.createdPublic.AllowFulfillmentOrder || !repo.createdPublic.AllowMallOrder {
		t.Fatalf("drip product command = %+v created=%v", repo.createdPublic, repo.publicCreated)
	}
	var payload struct {
		Product struct {
			ProductKind     string   `json:"product_kind"`
			DripBagGrams    float64  `json:"drip_bag_grams"`
			DripBoxBagCount int      `json:"drip_box_bag_count"`
			SalesUnits      []string `json:"sales_units"`
		} `json:"product"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode product response: %v body=%s", err, rec.Body.String())
	}
	if payload.Product.ProductKind != "drip_bag" || payload.Product.DripBagGrams != 10 || payload.Product.DripBoxBagCount != 10 {
		t.Fatalf("drip product response = %+v body=%s", payload.Product, rec.Body.String())
	}
	if !reflect.DeepEqual(payload.Product.SalesUnits, []string{"bag", "box"}) {
		t.Fatalf("sales_units = %#v, want bag/box body=%s", payload.Product.SalesUnits, rec.Body.String())
	}
}

func TestProductSettingsAPICreatesInstantCoffeeProductWithoutRoastLevel(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"name":"速溶美式","product_kind":"instant_coffee","default_price":39}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST instant coffee product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicCreated || repo.createdPublic.ProductKind != "instant_coffee" || repo.createdPublic.RoastLevel != "" || repo.createdPublic.YieldRate != 0 || repo.createdPublic.DefaultPrice != 39 {
		t.Fatalf("instant coffee product command = %+v created=%v", repo.createdPublic, repo.publicCreated)
	}
	var payload struct {
		Product struct {
			ProductKind string   `json:"product_kind"`
			RoastLevel  string   `json:"roast_level"`
			SalesUnits  []string `json:"sales_units"`
		} `json:"product"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode product response: %v body=%s", err, rec.Body.String())
	}
	if payload.Product.ProductKind != "instant_coffee" || payload.Product.RoastLevel != "" || len(payload.Product.SalesUnits) != 0 {
		t.Fatalf("instant coffee response = %+v body=%s", payload.Product, rec.Body.String())
	}
}

func TestProductSettingsAPIDefaultsOmittedDripBagConfig(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"name":"耶加雪菲挂耳","product_kind":"drip_bag"}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST drip product with omitted config status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicCreated || repo.createdPublic.ProductKind != "drip_bag" || repo.createdPublic.DripBagGrams != 10 || repo.createdPublic.DripBoxBagCount != 10 {
		t.Fatalf("omitted drip config command = %+v created=%v", repo.createdPublic, repo.publicCreated)
	}
	var payload struct {
		Product struct {
			DripBagGrams    float64  `json:"drip_bag_grams"`
			DripBoxBagCount int      `json:"drip_box_bag_count"`
			SalesUnits      []string `json:"sales_units"`
		} `json:"product"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode product response: %v body=%s", err, rec.Body.String())
	}
	if payload.Product.DripBagGrams != 10 || payload.Product.DripBoxBagCount != 10 || !reflect.DeepEqual(payload.Product.SalesUnits, []string{"bag", "box"}) {
		t.Fatalf("omitted drip config response = %+v body=%s", payload.Product, rec.Body.String())
	}
}

func TestProductSettingsAPIReturnsInternalErrorForProductCreatePersistenceFailure(t *testing.T) {
	repo := &productSettingsRepo{createErr: errors.New("database unavailable")}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(`{"name":"新公共拼配","roast_level":"中深烘"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST product persistence failure status=%d, want 500 body=%s", rec.Code, rec.Body.String())
	}
	if repo.publicCreated {
		t.Fatalf("failed persistence should not mark product created, command=%+v", repo.createdPublic)
	}
}

func TestProductSettingsAPIReturnsBadRequestForCreateValidationFailures(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing name",
			body: `{"roast_level":"中深烘"}`,
			want: "name required",
		},
		{
			name: "negative price",
			body: `{"name":"新公共拼配","roast_level":"中深烘","default_price":-1}`,
			want: "price must not be negative",
		},
		{
			name: "invalid roast level",
			body: `{"name":"X","roast_level":"not-a-level"}`,
			want: "invalid roast_level",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &productSettingsRepo{}
			e := echo.New()
			registerProductRoutes(e, catalogapp.NewService(repo))

			req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("POST product validation failure status=%d, want 400 body=%s", rec.Code, rec.Body.String())
			}
			if repo.publicCreated {
				t.Fatalf("invalid create should not reach repo, command=%+v", repo.createdPublic)
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(tc.want)) {
				t.Fatalf("validation response should mention %s: %s", tc.want, rec.Body.String())
			}
		})
	}
}

func TestProductSettingsAPIRejectsExplicitZeroDripBagCreate(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "zero grams",
			body: `{"name":"耶加雪菲挂耳","product_kind":"drip_bag","drip_bag_grams":0,"drip_box_bag_count":10}`,
			want: "drip_bag_grams",
		},
		{
			name: "zero box count",
			body: `{"name":"耶加雪菲挂耳","product_kind":"drip_bag","drip_bag_grams":10,"drip_box_bag_count":0}`,
			want: "drip_box_bag_count",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &productSettingsRepo{}
			e := echo.New()
			registerProductRoutes(e, catalogapp.NewService(repo))

			req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("POST explicit zero drip product status=%d, want 400 body=%s", rec.Code, rec.Body.String())
			}
			if repo.publicCreated {
				t.Fatalf("invalid drip create should not reach repo, command=%+v", repo.createdPublic)
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(tc.want)) {
				t.Fatalf("invalid drip create response should mention %s: %s", tc.want, rec.Body.String())
			}
		})
	}
}

func TestProductSettingsAPIUpdatesProductYieldRate(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{ID: 7, Name: "曲奇拼配", RoastLevel: "中烘", DefaultPrice: 99, RetailPrice100G: 22, RetailPrice200G: 43, RetailPrice227G: 48, RetailPrice250G: 52, YieldRate: 0.82}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","yield_rate":0.835}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productUpdated || repo.updated.ProductID != 7 || repo.updated.YieldRate != 0.835 || repo.updated.DefaultPrice != 99 || repo.updated.RetailPrice227G != 48 {
		t.Fatalf("update command = %+v updated=%v", repo.updated, repo.productUpdated)
	}
}

func TestProductSettingsAPIRejectsInvalidUpdateRoastLevel(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{ID: 7, Name: "曲奇拼配", RoastLevel: "中烘", YieldRate: 0.82}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"not-a-level"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT invalid roast level status=%d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	if repo.productUpdated {
		t.Fatalf("invalid roast level update should not reach repo, command=%+v", repo.updated)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("invalid roast_level")) {
		t.Fatalf("invalid roast level response should mention invalid roast_level: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIRejectsExplicitZeroDripBagUpdate(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "zero grams",
			body: `{"roast_level":"中烘","product_kind":"drip_bag","drip_bag_grams":0,"drip_box_bag_count":10}`,
			want: "drip_bag_grams",
		},
		{
			name: "zero box count",
			body: `{"roast_level":"中烘","product_kind":"drip_bag","drip_bag_grams":10,"drip_box_bag_count":0}`,
			want: "drip_box_bag_count",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &productSettingsRepo{
				products: []catalogapp.Product{{
					ID:                    7,
					Name:                  "耶加雪菲挂耳",
					RoastLevel:            "中烘",
					ProductKind:           "drip_bag",
					DripBagGrams:          10,
					DripBoxBagCount:       10,
					AllowFulfillmentOrder: true,
					AllowMallOrder:        true,
					YieldRate:             0.82,
				}},
			}
			e := echo.New()
			registerProductRoutes(e, catalogapp.NewService(repo))

			req := httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("PUT explicit zero drip product status=%d, want 400 body=%s", rec.Code, rec.Body.String())
			}
			if repo.productUpdated {
				t.Fatalf("invalid drip update should not reach repo, command=%+v", repo.updated)
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(tc.want)) {
				t.Fatalf("invalid drip update response should mention %s: %s", tc.want, rec.Body.String())
			}
		})
	}
}

func TestProductSettingsAPIRejectsInvalidDripBagUpdate(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID:                    7,
			Name:                  "耶加雪菲挂耳",
			RoastLevel:            "中烘",
			ProductKind:           "drip_bag",
			DripBagGrams:          10,
			DripBoxBagCount:       10,
			AllowFulfillmentOrder: true,
			AllowMallOrder:        true,
			YieldRate:             0.82,
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","product_kind":"drip_bag","drip_bag_grams":-1,"drip_box_bag_count":10}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT invalid drip product status=%d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	if repo.productUpdated {
		t.Fatalf("invalid drip update should not reach repo, command=%+v", repo.updated)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("drip_bag_grams")) {
		t.Fatalf("invalid drip response should mention drip_bag_grams: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIReturnsInternalErrorForProductUpdatePersistenceFailure(t *testing.T) {
	repo := &productSettingsRepo{
		products:  []catalogapp.Product{{ID: 7, Name: "曲奇拼配", RoastLevel: "中烘", YieldRate: 0.82}},
		updateErr: errors.New("database unavailable"),
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","yield_rate":0.835}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("PUT product persistence failure status=%d, want 500 body=%s", rec.Code, rec.Body.String())
	}
	if repo.productUpdated {
		t.Fatalf("failed persistence should not mark product updated, command=%+v", repo.updated)
	}
}

func TestProductSettingsAPISavesAndReturnsProductMarginOverride(t *testing.T) {
	margin := 0.235
	product := catalogapp.Product{ID: 7, Name: "曲奇拼配", RoastLevel: "中烘", YieldRate: 0.82}
	setFloat64PtrField(t, &product, "MarginRateOverride", margin)
	repo := &productSettingsRepo{
		products: []catalogapp.Product{product},
		categories: []catalogapp.ProductCategory{
			{ID: 1, Name: "咖啡豆", Level: 1, Position: 1},
			{ID: 2, ParentID: 1, Name: "意式拼配", Level: 2, Position: 1, GradientTemplateID: 9},
		},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"margin_rate_override":0.235`)) {
		t.Fatalf("product settings response missing product margin override: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","yield_rate":0.82,"margin_rate_override":0.31}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := float64PtrField(t, repo.updated, "MarginRateOverride")
	if got == nil || *got != 0.31 {
		t.Fatalf("margin override command = %+v, got %v", repo.updated, got)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","yield_rate":0.82,"margin_rate_override":null}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product clear status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := float64PtrField(t, repo.updated, "MarginRateOverride"); got != nil {
		t.Fatalf("margin override should clear to nil, got %v in %+v", *got, repo.updated)
	}
}

func TestProductSettingsAPISavesCustomerSkuMarginOverride(t *testing.T) {
	margin := 0.275
	product := catalogapp.Product{
		ID: 17, Name: "芬纳咖啡-曲奇拼配-深烘", ProductKind: "roasted", RoastLevel: "深烘", YieldRate: 0.8,
		CustomerID: 74, BaseProductID: 7, Visibility: "customer_only", CustomType: "custom_roast",
	}
	setFloat64PtrField(t, &product, "MarginRateOverride", margin)
	repo := &productSettingsRepo{products: []catalogapp.Product{product}}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"customer_id":74`, `"custom_type":"custom_roast"`, `"margin_rate_override":0.275`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("customer SKU product settings response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPut, "/api/products/17", bytes.NewBufferString(`{"product_kind":"roasted","roast_level":"深烘","yield_rate":0.8,"margin_rate_override":0.33}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT customer SKU product status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := float64PtrField(t, repo.updated, "MarginRateOverride")
	if got == nil || *got != 0.33 {
		t.Fatalf("customer SKU margin override command = %+v, got %v", repo.updated, got)
	}
}

func TestProductSettingsAPIDeactivatesMultipleProducts(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products/deactivate", bytes.NewBufferString(`{"product_ids":[7,8]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST deactivate products status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productsDeactivated || len(repo.deactivated.ProductIDs) != 2 || repo.deactivated.ProductIDs[0] != 7 || repo.deactivated.ProductIDs[1] != 8 {
		t.Fatalf("deactivate command = %+v deactivated=%v", repo.deactivated, repo.productsDeactivated)
	}
}

func TestProductSettingsAPIReturnsEmptyArraysForEmptyCategories(t *testing.T) {
	repo := &productSettingsRepo{
		categories: []catalogapp.ProductCategory{
			{ID: 1, Name: "咖啡豆", Level: 1, Position: 1},
			{ID: 2, ParentID: 1, Name: "意式拼配", Level: 2, Position: 1},
			{ID: 3, Name: "挂耳", Level: 1, Position: 2},
		},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Categories []struct {
			Name     string `json:"name"`
			Children []struct {
				Name     string `json:"name"`
				Products []any  `json:"products"`
			} `json:"children"`
			Products []any `json:"products"`
		} `json:"categories"`
		Products []any `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode product settings: %v body=%s", err, rec.Body.String())
	}
	if payload.Categories == nil || payload.Products == nil {
		t.Fatalf("top-level arrays must not be nil: %+v", payload)
	}
	if len(payload.Categories) != 2 {
		t.Fatalf("categories = %+v", payload.Categories)
	}
	if payload.Categories[0].Children == nil || payload.Categories[0].Children[0].Products == nil {
		t.Fatalf("empty category children/products must encode as [] not null: %s", rec.Body.String())
	}
	if payload.Categories[1].Children == nil || payload.Categories[1].Products == nil {
		t.Fatalf("empty root category arrays must encode as [] not null: %s", rec.Body.String())
	}
}

func TestLegacyProductAndCostingRoutesRedirectToProductSettings(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	cases := []struct {
		path string
		want string
	}{
		{path: "/products", want: "/vue-shell?view=productSettings"},
		{path: "/products/7", want: "/vue-shell?view=productSettings&edit_id=7"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		loc, err := url.Parse(rec.Header().Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		base, err := url.Parse("https://example.test" + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusFound || base.ResolveReference(loc).RequestURI() != tc.want {
			t.Fatalf("GET %s status=%d location=%q want %s", tc.path, rec.Code, rec.Header().Get("Location"), tc.want)
		}
	}
}

func setFloat64PtrField(t *testing.T, target any, fieldName string, value float64) {
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

func float64PtrField(t *testing.T, target any, fieldName string) *float64 {
	t.Helper()
	field := reflect.ValueOf(target).FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("missing %s field", fieldName)
	}
	if field.Kind() != reflect.Ptr || field.Type().Elem().Kind() != reflect.Float64 {
		t.Fatalf("%s field type = %s, want *float64", fieldName, field.Type())
	}
	if field.IsNil() {
		return nil
	}
	v := field.Elem().Float()
	return &v
}

func f64(v float64) *float64 {
	return &v
}
