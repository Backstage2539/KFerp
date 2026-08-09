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

func expectLegacyClassificationWriteGone(t *testing.T, rec *httptest.ResponseRecorder, label string) {
	t.Helper()
	if rec.Code != http.StatusGone || !bytes.Contains(rec.Body.Bytes(), []byte("classification write APIs are legacy readonly")) {
		t.Fatalf("%s should be legacy readonly, status=%d body=%s", label, rec.Code, rec.Body.String())
	}
}

type productSettingsRepo struct {
	products                            []catalogapp.Product
	categories                          []catalogapp.ProductCategory
	gradientTemplates                   []catalogapp.GradientTemplate
	productConfigTemplates              []catalogapp.ProductConfigTemplate
	productUnitDefinitions              []catalogapp.ProductUnitDefinition
	productUnitTemplates                []catalogapp.ProductUnitTemplate
	productPriceGroups                  []catalogapp.ProductPriceGroup
	businessGroups                      []catalogapp.BusinessGroup
	savedBusinessGroup                  catalogapp.BusinessGroup
	featureSelection                    catalogapp.BusinessGroupFeatureSelection
	savedFeatureSelection               catalogapp.SaveBusinessGroupFeatureSelectionCommand
	deletedBusinessGroup                catalogapp.DeleteBusinessGroupCommand
	savedBusinessGroupItem              catalogapp.BusinessGroupItem
	deletedBusinessGroupItem            catalogapp.DeleteBusinessGroupItemCommand
	movedBusinessGroupItem              catalogapp.MoveBusinessGroupItemCommand
	ensuredBusinessGroupID              int64
	ensuredBusinessGroupUsageKey        string
	ensuredBusinessGroupActor           string
	productPriceRecords                 []catalogapp.ProductPriceRecord
	productPriceRecordByID              map[int64]catalogapp.ProductPriceRecord
	productTierPriceSchemes             []catalogapp.ProductTierPriceScheme
	productPricingRules                 []catalogapp.ProductPricingRule
	deletedPriceTierTemplateID          int64
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
	deletedConfigTemplate               catalogapp.DeleteProductConfigTemplateCommand
	savedUnitDefinition                 catalogapp.SaveProductUnitDefinitionCommand
	savedUnitTemplate                   catalogapp.SaveProductUnitTemplateCommand
	savedProductPriceGroup              catalogapp.SaveProductPriceGroupCommand
	savedProductPriceRecord             catalogapp.SaveProductPriceRecordCommand
	savedProductTierPriceScheme         catalogapp.SaveProductTierPriceSchemeCommand
	deletedUnitDefinition               catalogapp.DeleteProductUnitDefinitionCommand
	deletedUnitTemplate                 catalogapp.DeleteProductUnitTemplateCommand
	savedProductionConfig               catalogapp.SaveProductProductionConfigCommand
	deactivatedTemplate                 catalogapp.DeactivateGradientTemplateCommand
	boundTemplate                       catalogapp.BindCategoryGradientTemplateCommand
	updated                             catalogapp.UpdateProductBasicsCommand
	updateErr                           error
	deactivated                         catalogapp.DeactivateProductsCommand
	createdPublic                       catalogapp.CreateProductCommand
	createdSKU                          catalogapp.CreateSKUCommand
	defaultSKU                          catalogapp.SetProductDefaultSKUCommand
	defaultSKUErr                       error
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
	configTemplateDeleted               bool
	unitDefinitionSaved                 bool
	unitTemplateSaved                   bool
	productPriceGroupSaved              bool
	productPriceRecordSaved             bool
	productTierPriceSchemeSaved         bool
	unitDefinitionDeleted               bool
	unitTemplateDeleted                 bool
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
			r.products[i].UnitTemplateID = cmd.UnitTemplateID
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
		UnitTemplateID:        cmd.UnitTemplateID,
		UnitRuleOverrideJSON:  cmd.UnitRuleOverrideJSON,
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
		ID:                       912,
		SKUID:                    912,
		ParentProductID:          cmd.ParentProductID,
		EffectiveParentProductID: cmd.ParentProductID,
		SKUName:                  cmd.SKUName,
		SKUCode:                  cmd.SKUCode,
		Barcode:                  cmd.Barcode,
		SpecLabel:                cmd.SpecLabel,
		NetContentQty:            cmd.NetContentQty,
		NetContentUnit:           cmd.NetContentUnit,
		IsDefaultSKU:             cmd.IsDefaultSKU,
		Name:                     cmd.Name,
		Remark:                   cmd.Remark,
		CustomerID:               cmd.CustomerID,
		ProductCategoryID:        cmd.ProductSubtypeCategoryID,
		SpecialAttrsJSON:         cmd.SpecialAttrsJSON,
		UnitTemplateID:           cmd.UnitTemplateID,
		UnitRuleOverrideJSON:     cmd.UnitRuleOverrideJSON,
		Visibility:               visibility,
	}, nil
}

func (r *productSettingsRepo) SetProductDefaultSKU(ctx context.Context, cmd catalogapp.SetProductDefaultSKUCommand) (catalogapp.Product, error) {
	r.defaultSKU = cmd
	if r.defaultSKUErr != nil {
		return catalogapp.Product{}, r.defaultSKUErr
	}
	for i := range r.products {
		if r.products[i].ID == cmd.ParentProductID {
			r.products[i].DefaultSKUID = cmd.SKUID
			r.products[i].EffectiveDefaultSKUID = cmd.SKUID
			return r.products[i], nil
		}
	}
	return catalogapp.Product{ID: cmd.ParentProductID, DefaultSKUID: cmd.SKUID, EffectiveDefaultSKUID: cmd.SKUID}, nil
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
		ProductID:                cmd.ProductID,
		ProductionBomID:          cmd.ProductionBomID,
		ProductionBomVersionID:   cmd.ProductionBomVersionID,
		ProcessRouteID:           cmd.ProcessRouteID,
		IndustryFieldTemplateID:  cmd.IndustryFieldTemplateID,
		IndustryFieldTemplateIDs: cmd.IndustryFieldTemplateIDs,
		ExpectedLossRate:         cmd.ExpectedLossRate,
		Note:                     cmd.Note,
		Fields:                   cmd.Fields,
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

func (r *productSettingsRepo) ListProductPriceGroups(ctx context.Context) ([]catalogapp.ProductPriceGroup, error) {
	return r.productPriceGroups, nil
}

func (r *productSettingsRepo) SaveProductPriceGroup(ctx context.Context, cmd catalogapp.SaveProductPriceGroupCommand) (catalogapp.ProductPriceGroup, error) {
	r.savedProductPriceGroup = cmd
	r.productPriceGroupSaved = true
	id := cmd.ID
	if id == 0 {
		id = 31
	}
	return catalogapp.ProductPriceGroup{ID: id, Name: cmd.Name, SortOrder: cmd.SortOrder, Active: true}, nil
}

func (r *productSettingsRepo) ListBusinessGroups(ctx context.Context) ([]catalogapp.BusinessGroup, error) {
	return r.businessGroups, nil
}

func (r *productSettingsRepo) SaveBusinessGroup(ctx context.Context, cmd catalogapp.BusinessGroup) (catalogapp.BusinessGroup, error) {
	r.savedBusinessGroup = cmd
	if cmd.ID == 0 {
		cmd.ID = 61
	}
	return cmd, nil
}

func TestPR584BusinessGroupAPIIgnoresCachedTemplateOwnedUsageWrites(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	legacyReq := httptest.NewRequest(http.MethodPut, "/api/business-groups/61", strings.NewReader(`{"name":"商品分类模板","usages":[]}`))
	legacyReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	legacyRec := httptest.NewRecorder()
	e.ServeHTTP(legacyRec, legacyReq)
	if legacyRec.Code != http.StatusOK {
		t.Fatalf("legacy empty usages status=%d body=%s", legacyRec.Code, legacyRec.Body.String())
	}
	if repo.savedBusinessGroup.Usages != nil {
		t.Fatalf("legacy empty usages=%#v, want no-op nil", repo.savedBusinessGroup.Usages)
	}

	clearReq := httptest.NewRequest(http.MethodPut, "/api/business-groups/61", strings.NewReader(`{"name":"商品分类模板","replace_usages":true,"usages":[]}`))
	clearReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	clearRec := httptest.NewRecorder()
	e.ServeHTTP(clearRec, clearReq)
	if clearRec.Code != http.StatusOK {
		t.Fatalf("explicit clear usages status=%d body=%s", clearRec.Code, clearRec.Body.String())
	}
	if repo.savedBusinessGroup.ReplaceUsages || repo.savedBusinessGroup.Usages != nil {
		t.Fatalf("cached template-owned clear must not overwrite feature selection: %+v", repo.savedBusinessGroup)
	}
}

func (r *productSettingsRepo) DeleteBusinessGroup(ctx context.Context, cmd catalogapp.DeleteBusinessGroupCommand) error {
	r.deletedBusinessGroup = cmd
	return nil
}

func (r *productSettingsRepo) SaveBusinessGroupItem(ctx context.Context, cmd catalogapp.BusinessGroupItem) (catalogapp.BusinessGroupItem, error) {
	r.savedBusinessGroupItem = cmd
	if cmd.ID == 0 {
		cmd.ID = 67
	}
	if cmd.Active == false {
		cmd.Active = true
	}
	return cmd, nil
}

func (r *productSettingsRepo) DeleteBusinessGroupItem(ctx context.Context, cmd catalogapp.DeleteBusinessGroupItemCommand) error {
	r.deletedBusinessGroupItem = cmd
	return nil
}

func (r *productSettingsRepo) MoveBusinessGroupItem(ctx context.Context, cmd catalogapp.MoveBusinessGroupItemCommand) (catalogapp.BusinessGroupItem, error) {
	r.movedBusinessGroupItem = cmd
	return catalogapp.BusinessGroupItem{ID: cmd.ID, ParentID: cmd.ParentID, SortOrder: cmd.Position * 10, Active: true}, nil
}

func (r *productSettingsRepo) EnsureBusinessGroupUsage(ctx context.Context, groupID int64, usageKey string, actor string) error {
	r.ensuredBusinessGroupID = groupID
	r.ensuredBusinessGroupUsageKey = usageKey
	r.ensuredBusinessGroupActor = actor
	return nil
}

func (r *productSettingsRepo) GetBusinessGroupFeatureSelection(ctx context.Context, featureKey string) (catalogapp.BusinessGroupFeatureSelection, error) {
	return r.featureSelection, nil
}

func (r *productSettingsRepo) SaveBusinessGroupFeatureSelection(ctx context.Context, cmd catalogapp.SaveBusinessGroupFeatureSelectionCommand) (catalogapp.BusinessGroupFeatureSelection, error) {
	r.savedFeatureSelection = cmd
	r.featureSelection = catalogapp.BusinessGroupFeatureSelection{FeatureKey: cmd.FeatureKey, GroupTemplateIDs: append([]int64(nil), cmd.GroupTemplateIDs...)}
	return r.featureSelection, nil
}

func TestPR584BusinessGroupFeatureSelectionAPIUsesFeatureOwnedOrderedTemplates(t *testing.T) {
	repo := &productSettingsRepo{featureSelection: catalogapp.BusinessGroupFeatureSelection{
		FeatureKey:       catalogapp.BusinessGroupUsageProductCatalog,
		GroupTemplateIDs: []int64{72, 71},
	}}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	getReq := httptest.NewRequest(http.MethodGet, "/api/business-group-feature-selections/product_catalog", nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK || !bytes.Contains(getRec.Body.Bytes(), []byte(`"group_template_ids":[72,71]`)) {
		t.Fatalf("feature selection GET status=%d body=%s", getRec.Code, getRec.Body.String())
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/business-group-feature-selections/product_catalog", strings.NewReader(`{"group_template_ids":[71,72,71]}`))
	putReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	putReq.Header.Set("X-Actor", "feature-owner")
	putRec := httptest.NewRecorder()
	e.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("feature selection PUT status=%d body=%s", putRec.Code, putRec.Body.String())
	}
	if got, want := repo.savedFeatureSelection.GroupTemplateIDs, []int64{71, 72}; !reflect.DeepEqual(got, want) {
		t.Fatalf("saved ordered template ids=%v, want %v", got, want)
	}
	if repo.savedFeatureSelection.FeatureKey != catalogapp.BusinessGroupUsageProductCatalog {
		t.Fatalf("saved feature key=%q", repo.savedFeatureSelection.FeatureKey)
	}

	clearReq := httptest.NewRequest(http.MethodPut, "/api/business-group-feature-selections/product_catalog", strings.NewReader(`{"group_template_ids":[]}`))
	clearReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	clearRec := httptest.NewRecorder()
	e.ServeHTTP(clearRec, clearReq)
	if clearRec.Code != http.StatusOK || repo.savedFeatureSelection.GroupTemplateIDs == nil || len(repo.savedFeatureSelection.GroupTemplateIDs) != 0 {
		t.Fatalf("explicit clear status=%d saved=%#v body=%s", clearRec.Code, repo.savedFeatureSelection.GroupTemplateIDs, clearRec.Body.String())
	}

	for name, tc := range map[string][2]string{
		"missing ids": {"/api/business-group-feature-selections/product_catalog", `{}`},
		"null ids":    {"/api/business-group-feature-selections/product_catalog", `{"group_template_ids":null}`},
		"price list":  {"/api/business-group-feature-selections/price_list", `{"group_template_ids":[71]}`},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, tc[0], strings.NewReader(tc[1]))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func (r *productSettingsRepo) ListBusinessGroupAssignments(ctx context.Context, query catalogapp.BusinessGroupAssignmentQuery) ([]catalogapp.BusinessGroupAssignment, error) {
	return []catalogapp.BusinessGroupAssignment{}, nil
}

func (r *productSettingsRepo) SaveBusinessGroupAssignment(ctx context.Context, cmd catalogapp.BusinessGroupAssignment) (catalogapp.BusinessGroupAssignment, error) {
	if cmd.ID == 0 {
		cmd.ID = 65
	}
	return cmd, nil
}

func (r *productSettingsRepo) DeleteBusinessGroupAssignment(ctx context.Context, cmd catalogapp.DeleteBusinessGroupAssignmentCommand) error {
	return nil
}

func TestBusinessGroupsAPIUsageFilterHidesInactiveGroups(t *testing.T) {
	repo := &productSettingsRepo{
		businessGroups: []catalogapp.BusinessGroup{
			{
				ID:     63,
				Name:   "PR442-SCENARIO-20260607-H4Z5JC Group",
				Active: false,
				Usages: []catalogapp.BusinessGroupUsage{{
					UsageKey: catalogapp.BusinessGroupUsageWarehouseInventory,
					Active:   true,
				}},
			},
			{
				ID:     64,
				Name:   "仓库库存默认分组",
				Active: true,
				Usages: []catalogapp.BusinessGroupUsage{{
					UsageKey: catalogapp.BusinessGroupUsageWarehouseInventory,
					Active:   true,
				}},
			},
			{
				ID:     65,
				Name:   "停用用途分组",
				Active: true,
				Usages: []catalogapp.BusinessGroupUsage{{
					UsageKey: catalogapp.BusinessGroupUsageWarehouseInventory,
					Active:   false,
				}},
			},
			{
				ID:     66,
				Name:   "商品档案分组",
				Active: true,
				Usages: []catalogapp.BusinessGroupUsage{{
					UsageKey: catalogapp.BusinessGroupUsageProductCatalog,
					Active:   true,
				}},
			},
		},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/business-groups?usage_key=warehouse_inventory", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("business groups status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.Bytes()
	if !bytes.Contains(body, []byte(`"name":"仓库库存默认分组"`)) {
		t.Fatalf("active warehouse group missing: %s", rec.Body.String())
	}
	for _, unexpected := range []string{"PR442-SCENARIO-20260607-H4Z5JC Group", "停用用途分组", "商品档案分组"} {
		if bytes.Contains(body, []byte(unexpected)) {
			t.Fatalf("business group usage filter leaked %q: %s", unexpected, rec.Body.String())
		}
	}
}

func TestBusinessGroupItemsAPIWritesGenericGroupItems(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/business-group-items", strings.NewReader(`{"group_id":66,"parent_id":0,"name":"新大类","sort_order":10}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("business group item create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.savedBusinessGroupItem.GroupID != 66 || repo.savedBusinessGroupItem.Name != "新大类" || repo.savedBusinessGroupItem.SortOrder != 10 {
		t.Fatalf("unexpected saved business group item: %+v", repo.savedBusinessGroupItem)
	}

	moveReq := httptest.NewRequest(http.MethodPost, "/api/business-group-items/67/move", strings.NewReader(`{"parent_id":68,"position":2}`))
	moveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	moveRec := httptest.NewRecorder()
	e.ServeHTTP(moveRec, moveReq)
	if moveRec.Code != http.StatusOK {
		t.Fatalf("business group item move status=%d body=%s", moveRec.Code, moveRec.Body.String())
	}
	if repo.movedBusinessGroupItem.ID != 67 || repo.movedBusinessGroupItem.ParentID != 68 || repo.movedBusinessGroupItem.Position != 2 {
		t.Fatalf("unexpected moved business group item: %+v", repo.movedBusinessGroupItem)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/business-group-items/67", nil)
	deleteRec := httptest.NewRecorder()
	e.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("business group item delete status=%d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	if repo.deletedBusinessGroupItem.ID != 67 {
		t.Fatalf("unexpected deleted business group item: %+v", repo.deletedBusinessGroupItem)
	}
}

func TestBusinessGroupsAPIDeletesTemplate(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodDelete, "/api/business-groups/66", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("business group delete status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.deletedBusinessGroup.ID != 66 {
		t.Fatalf("unexpected deleted business group: %+v", repo.deletedBusinessGroup)
	}
}

func TestPR584LegacyBusinessGroupUsageAPIRejectsTemplateOwnedMutation(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/business-groups/72/usages", strings.NewReader(`{"usage_key":"production_bom"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("legacy business group usage status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.ensuredBusinessGroupID != 0 {
		t.Fatalf("legacy endpoint must not mutate feature selection: id=%d key=%q", repo.ensuredBusinessGroupID, repo.ensuredBusinessGroupUsageKey)
	}
}

func (r *productSettingsRepo) ListProductCustomerReferences(ctx context.Context, productID int64) ([]catalogapp.ProductCustomerReference, error) {
	return []catalogapp.ProductCustomerReference{}, nil
}

func (r *productSettingsRepo) SaveProductCustomerReference(ctx context.Context, cmd catalogapp.ProductCustomerReference) (catalogapp.ProductCustomerReference, error) {
	if cmd.ID == 0 {
		cmd.ID = 62
	}
	return cmd, nil
}

func (r *productSettingsRepo) ListProductPricingRules(ctx context.Context) ([]catalogapp.ProductPricingRule, error) {
	return r.productPricingRules, nil
}

func (r *productSettingsRepo) SaveProductPricingRule(ctx context.Context, cmd catalogapp.ProductPricingRule) (catalogapp.ProductPricingRule, error) {
	if cmd.ID == 0 {
		cmd.ID = 63
	}
	return cmd, nil
}

func (r *productSettingsRepo) ListPriceTierTemplates(ctx context.Context) ([]catalogapp.PriceTierTemplate, error) {
	return []catalogapp.PriceTierTemplate{}, nil
}

func (r *productSettingsRepo) SavePriceTierTemplate(ctx context.Context, cmd catalogapp.PriceTierTemplate) (catalogapp.PriceTierTemplate, error) {
	if cmd.ID == 0 {
		cmd.ID = 64
	}
	return cmd, nil
}

func (r *productSettingsRepo) DeletePriceTierTemplate(ctx context.Context, id int64, actor string) error {
	r.deletedPriceTierTemplateID = id
	return nil
}

func (r *productSettingsRepo) ListProductPriceRecords(ctx context.Context, query catalogapp.ProductPriceRecordQuery) ([]catalogapp.ProductPriceRecord, error) {
	if r.productPriceRecords != nil {
		return r.productPriceRecords, nil
	}
	out := make([]catalogapp.ProductPriceRecord, 0, len(r.productPriceRecordByID))
	for _, row := range r.productPriceRecordByID {
		out = append(out, row)
	}
	return out, nil
}

func (r *productSettingsRepo) GetProductPriceRecord(ctx context.Context, id int64) (catalogapp.ProductPriceRecord, error) {
	if r.productPriceRecordByID != nil {
		if row, ok := r.productPriceRecordByID[id]; ok {
			return row, nil
		}
	}
	return catalogapp.ProductPriceRecord{}, nil
}

func (r *productSettingsRepo) SaveProductPriceRecord(ctx context.Context, cmd catalogapp.SaveProductPriceRecordCommand) (catalogapp.ProductPriceRecord, error) {
	r.savedProductPriceRecord = cmd
	r.productPriceRecordSaved = true
	id := cmd.ID
	if id == 0 {
		id = 41
	}
	return catalogapp.ProductPriceRecord{
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

func (r *productSettingsRepo) ListProductTierPriceSchemes(ctx context.Context, query catalogapp.ProductTierPriceSchemeQuery) ([]catalogapp.ProductTierPriceScheme, error) {
	return r.productTierPriceSchemes, nil
}

func (r *productSettingsRepo) SaveProductTierPriceScheme(ctx context.Context, cmd catalogapp.SaveProductTierPriceSchemeCommand) (catalogapp.ProductTierPriceScheme, error) {
	r.savedProductTierPriceScheme = cmd
	r.productTierPriceSchemeSaved = true
	id := cmd.ID
	if id == 0 {
		id = 51
	}
	return catalogapp.ProductTierPriceScheme{ID: id, Name: cmd.Name, ProductID: cmd.ProductID, CustomerProductAliasID: cmd.CustomerProductAliasID, PriceGroupID: cmd.PriceGroupID, Active: true, Tiers: cmd.Tiers}, nil
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

func (r *productSettingsRepo) DeleteProductConfigTemplate(ctx context.Context, cmd catalogapp.DeleteProductConfigTemplateCommand) error {
	r.deletedConfigTemplate = cmd
	r.configTemplateDeleted = true
	return nil
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
		SalesUnit:          cmd.SalesUnit,
		DefaultSalesUnit:   cmd.DefaultSalesUnit,
		SalesUnits:         cmd.SalesUnits,
		SalesSpecs:         cmd.SalesSpecs,
		QuoteUnit:          cmd.QuoteUnit,
		OrderUnit:          cmd.OrderUnit,
		UnitConversionJSON: cmd.UnitConversionJSON,
		IntegerUnit:        cmd.IntegerUnit,
		Active:             true,
	}, nil
}

func (r *productSettingsRepo) DeleteProductUnitDefinition(ctx context.Context, cmd catalogapp.DeleteProductUnitDefinitionCommand) error {
	r.deletedUnitDefinition = cmd
	r.unitDefinitionDeleted = true
	return nil
}

func (r *productSettingsRepo) DeleteProductUnitTemplate(ctx context.Context, cmd catalogapp.DeleteProductUnitTemplateCommand) error {
	r.deletedUnitTemplate = cmd
	r.unitTemplateDeleted = true
	return nil
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

func TestCustomerProductAliasAPIsReadLegacyRowsAndRejectNewWrites(t *testing.T) {
	repo := &productSettingsRepo{
		customerProductAliases: []catalogapp.CustomerProductAlias{{
			ID:                      11,
			CustomerID:              42,
			CustomerName:            "Karen",
			ProductID:               88,
			ProductName:             "精品意式拼配",
			ProductCode:             "SKU-000088",
			ProductActive:           false,
			DisplayName:             "Karen 精品拼配",
			CustomerItemCode:        "KAREN-ESP",
			BrandName:               "",
			DisplayCategoryID:       7,
			DisplayCategoryName:     "商用批发",
			ProductConfigTemplateID: 301,
			GradientTemplateID:      18,
			UnitTemplateID:          22,
			SortOrder:               20,
			IncludeInPriceList:      true,
			Active:                  true,
			Remark:                  "贴牌只改名字",
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
	for _, want := range []string{`"rows"`, `"display_name":"Karen 精品拼配"`, `"customer_item_code":"KAREN-ESP"`, `"product_id":88`, `"product_code":"SKU-000088"`, `"product_active":false`, `"product_config_template_id":301`, `"gradient_template_id":18`, `"unit_template_id":22`, `"include_in_price_list":true`} {
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
		"customer_item_code":"KAREN-ESP-001",
		"brand_name":"",
		"display_category_id":7,
		"product_config_template_id":301,
		"sort_order":20,
		"include_in_price_list":true,
		"active":true,
		"remark":"贴牌只改名字"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("customer products are legacy readonly")) {
		t.Fatalf("POST legacy alias should be readonly, status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.customerAliasSaved {
		t.Fatalf("legacy alias write should not reach repo")
	}

	req = httptest.NewRequest(http.MethodPut, "/api/customer-product-aliases/11", bytes.NewBufferString(`{
		"customer_id":42,
		"product_id":88,
		"display_name":"Karen 改名拼配",
		"customer_item_code":"KAREN-ESP-002",
		"product_config_template_id":302,
		"include_in_price_list":false,
		"active":true
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("customer products are legacy readonly")) {
		t.Fatalf("PUT legacy alias should be readonly, status=%d body=%s", rec.Code, rec.Body.String())
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

func TestProductCustomerReferenceAPIReplacesCustomerProductMasterWrites(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	req := httptest.NewRequest(http.MethodPost, "/api/product-customer-references", bytes.NewBufferString(`{
		"product_id":88,
		"customer_id":42,
		"customer_item_code":"KAREN-ESP-001",
		"customer_display_name":"Karen 精品拼配",
		"active":true,
		"remark":"打印和搜索使用"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST product customer reference status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"product_id":88`, `"customer_id":42`, `"customer_item_code":"KAREN-ESP-001"`, `"customer_display_name":"Karen 精品拼配"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("customer reference response missing %s: %s", want, rec.Body.String())
		}
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

func TestCustomerProductAliasBatchAPIRejectsLegacyCustomerProductWrites(t *testing.T) {
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

	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("customer products are legacy readonly")) {
		t.Fatalf("POST legacy batch alias should be readonly, status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.customerAliasBatchSaved {
		t.Fatalf("legacy batch alias write should not reach repo")
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
	expectLegacyClassificationWriteGone(t, rec, "POST product template usage")

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
	expectLegacyClassificationWriteGone(t, rec, "POST alias template usage")
	if repo.productClassificationUsageSaved || repo.aliasClassificationUsageSaved {
		t.Fatalf("legacy classification usage writes should not reach repo: product=%v alias=%v", repo.productClassificationUsageSaved, repo.aliasClassificationUsageSaved)
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

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-templates", bytes.NewBufferString(`{"customer_id":42,"name":"客户侧价格表分类","sort_order":2,"product_config_template_id":701}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "POST classification template")

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-template-categories", bytes.NewBufferString(`{"template_id":501,"name":"新品","level":1,"sort_order":3,"product_config_template_id":702}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "POST classification category")

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-assignments/products", bytes.NewBufferString(`{"product_id":88,"template_id":501,"category_id":502,"sort_order":4}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "POST product classification assignment")

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-assignments/products", bytes.NewBufferString(`{"product_id":88,"template_id":601,"category_id":0,"sort_order":6}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "POST product classification reassignment")

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-assignments/customer-aliases", bytes.NewBufferString(`{"alias_id":77,"template_id":501,"category_id":502,"sort_order":5}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "POST alias classification assignment")

	req = httptest.NewRequest(http.MethodPost, "/api/product-classification-assignments/customer-aliases", bytes.NewBufferString(`{"alias_id":77,"template_id":602,"category_id":0,"sort_order":6}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "POST alias classification reassignment")

	req = httptest.NewRequest(http.MethodDelete, "/api/product-classification-template-categories/502?template_id=501", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "DELETE classification category")
	if repo.classificationTemplateSaved || repo.classificationCategorySaved || repo.classificationAssignmentSaved || repo.aliasClassificationAssignmentSaved || repo.classificationCategoryDeleted {
		t.Fatalf("legacy classification writes should not reach repo: template=%v category=%v product=%v alias=%v deleted=%v", repo.classificationTemplateSaved, repo.classificationCategorySaved, repo.classificationAssignmentSaved, repo.aliasClassificationAssignmentSaved, repo.classificationCategoryDeleted)
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
			PriceSummary: catalogapp.PriceSummary{FinalPrice: 88.5, PriceUnit: "kg", TierLabel: "1kg+", PriceTableVersion: "PR439-PRICE", SourcePriceRecordID: 901},
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
	for _, want := range []string{`"categories"`, `"children"`, `"products"`, `"gradient_templates"`, `"customer_public_usages"`, `"use_public_sku":true`, `"use_public_categories":false`, `"use_public_gradient_templates":true`, `"gradient_template_id":9`, `"name":"工厂量单模板"`, `"display_unit":"kg"`, `"template_state":"public_template"`, `"number":1`, `"name":"咖啡豆"`, `"name":"意式拼配"`, `"name":"客户A分类"`, `"customer_id":3`, `"name":"曲奇拼配"`, `"remark":"奶咖主推"`, `"yield_rate":1`, `"expected_loss_rate":0`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing %s: %s", want, rec.Body.String())
		}
	}
	for _, want := range []string{`"customer_id":0`, `"base_product_id":0`, `"visibility":"public"`, `"custom_type":""`, `"bom_item_count":2`, `"production_bom_code":"BOM-001"`, `"production_bom_version_no":"V002"`, `"latest_bom_version_no":"V003"`, `"is_latest_bom_version":false`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing ownership field %s: %s", want, rec.Body.String())
		}
	}
	for _, want := range []string{`"price_summary"`, `"final_price":88.5`, `"price_unit":"kg"`, `"tier_label":"1kg+"`, `"price_table_version":"PR439-PRICE"`, `"source_price_record_id":901`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing price summary field %s: %s", want, rec.Body.String())
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
	expectLegacyClassificationWriteGone(t, rec, "POST category")

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/categories/2/move", bytes.NewBufferString(`{"parent_id":1,"position":1}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "POST move category")

	req = httptest.NewRequest(http.MethodDelete, "/api/product-settings/categories/2", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "DELETE category")

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/products/7/category", bytes.NewBufferString(`{"category_id":2,"position":3}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	expectLegacyClassificationWriteGone(t, rec, "POST assign product")
	if repo.categoryCreated || repo.categoryMoved || repo.categoryDeleted || repo.productAssigned {
		t.Fatalf("legacy product category writes should not reach repo: created=%v moved=%v deleted=%v assigned=%v", repo.categoryCreated, repo.categoryMoved, repo.categoryDeleted, repo.productAssigned)
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
	expectLegacyClassificationWriteGone(t, rec, "POST category config")
	if repo.categoryCreated {
		t.Fatalf("legacy category config write should not reach repo: %+v", repo.savedCategory)
	}
}

func TestProductInventoryUnitAPIContract(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID:                   91,
			Name:                 "盒装速溶",
			ProductKind:          "instant_coffee",
			UnitRuleOverrideJSON: `{"inventory_unit":"kg","integer_unit":false,"default_sales_unit":"盒","unit_conversion_json":{"盒":{"kg":0.2}},"sales_unit_rules":{"盒":{"integer":true}},"order_unit":"箱","legacy_key":"keep"}`,
			Active:               true,
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
	for _, want := range []string{`"inventory_unit":"kg"`, `"integer_inventory_unit":false`, `"default_sales_unit":"盒"`, `"unit_conversion_json":"{\"盒\":{\"kg\":0.2}}"`, `"sales_unit_rules":"{\"盒\":{\"integer\":true}}"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing inventory unit field %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPut, "/api/products/91", bytes.NewBufferString(`{
		"name":"盒装速溶",
		"product_kind":"instant_coffee",
		"yield_rate":0.8,
		"inventory_unit":"盒",
		"integer_inventory_unit":true,
		"default_sales_unit":"袋",
		"unit_conversion_json":{"袋":{"盒":6}},
		"sales_unit_rules":{"袋":{"integer":true}}
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product inventory unit status=%d body=%s", rec.Code, rec.Body.String())
	}
	var updatedRule map[string]any
	if err := json.Unmarshal([]byte(repo.updated.UnitRuleOverrideJSON), &updatedRule); err != nil {
		t.Fatalf("updated unit rule json invalid: %v raw=%s", err, repo.updated.UnitRuleOverrideJSON)
	}
	if updatedRule["inventory_unit"] != "盒" || updatedRule["integer_inventory_unit"] != true || updatedRule["default_sales_unit"] != "袋" || updatedRule["order_unit"] != "箱" || updatedRule["legacy_key"] != "keep" {
		t.Fatalf("updated unit rule should preserve existing keys and write inventory/sales fields: %#v", updatedRule)
	}
	if conversion, ok := updatedRule["unit_conversion_json"].(map[string]any); !ok || conversion["袋"].(map[string]any)["盒"] != float64(6) {
		t.Fatalf("updated unit conversion = %#v", updatedRule["unit_conversion_json"])
	}
	if salesRules, ok := updatedRule["sales_unit_rules"].(map[string]any); !ok || salesRules["袋"].(map[string]any)["integer"] != true {
		t.Fatalf("updated sales unit rules = %#v", updatedRule["sales_unit_rules"])
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(`{
		"name":"新盒装",
		"product_kind":"instant_coffee",
		"yield_rate":0.8,
		"inventory_unit":"盒",
		"integer_inventory_unit":true,
		"default_sales_unit":"袋",
		"unit_conversion_json":{"袋":{"盒":6}},
		"sales_unit_rules":{"袋":{"integer":true}}
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST product inventory unit status=%d body=%s", rec.Code, rec.Body.String())
	}
	var createdRule map[string]any
	if err := json.Unmarshal([]byte(repo.createdPublic.UnitRuleOverrideJSON), &createdRule); err != nil {
		t.Fatalf("created unit rule json invalid: %v raw=%s", err, repo.createdPublic.UnitRuleOverrideJSON)
	}
	if createdRule["inventory_unit"] != "盒" || createdRule["integer_inventory_unit"] != true || createdRule["default_sales_unit"] != "袋" {
		t.Fatalf("created product unit rule = %#v, want inventory and sales unit fields", createdRule)
	}
	if conversion, ok := createdRule["unit_conversion_json"].(map[string]any); !ok || conversion["袋"].(map[string]any)["盒"] != float64(6) {
		t.Fatalf("created unit conversion = %#v", createdRule["unit_conversion_json"])
	}
	if salesRules, ok := createdRule["sales_unit_rules"].(map[string]any); !ok || salesRules["袋"].(map[string]any)["integer"] != true {
		t.Fatalf("created sales unit rules = %#v", createdRule["sales_unit_rules"])
	}
}

func TestProductUnitTemplateReferenceAPIContract(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID:                   101,
			Name:                 "模板咖啡豆",
			ProductKind:          "roasted",
			UnitTemplateID:       7,
			UnitTemplateName:     "咖啡豆单位",
			InventoryUnit:        "kg",
			IntegerInventoryUnit: false,
			DefaultSalesUnit:     "袋",
			UnitConversionJSON:   `{"袋":{"kg":0.25}}`,
			SalesUnitRulesJSON:   `{"袋":{"integer":true}}`,
			UnitRuleOverrideJSON: `{"legacy_key":"keep"}`,
			UnitRuleSource:       "product_unit_template",
			Active:               true,
		}},
		productUnitTemplates: []catalogapp.ProductUnitTemplate{{
			ID:                 7,
			Name:               "咖啡豆单位",
			InventoryUnit:      "kg",
			SalesUnit:          "袋",
			QuoteUnit:          "袋",
			OrderUnit:          "袋",
			UnitConversionJSON: `{"袋":{"kg":0.25}}`,
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
		t.Fatalf("GET product settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"unit_template_id":7`,
		`"unit_template_name":"咖啡豆单位"`,
		`"unit_rule_source":"product_unit_template"`,
		`"inventory_unit":"kg"`,
		`"default_sales_unit":"袋"`,
		`"unit_conversion_json":"{\"袋\":{\"kg\":0.25}}"`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing unit-template field %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPut, "/api/products/101", bytes.NewBufferString(`{
		"name":"模板咖啡豆",
		"product_kind":"roasted",
		"yield_rate":0.8,
		"unit_template_id":7
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product unit template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.updated.UnitTemplateID != 7 {
		t.Fatalf("PUT product should pass unit_template_id to service command, got %d", repo.updated.UnitTemplateID)
	}
	if repo.updated.UnitRuleOverrideJSON != `{"legacy_key":"keep"}` {
		t.Fatalf("PUT product should keep existing product override json when only template changes, got %s", repo.updated.UnitRuleOverrideJSON)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(`{
		"name":"新模板商品",
		"product_kind":"roasted",
		"yield_rate":0.8,
		"unit_template_id":7
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST product unit template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdPublic.UnitTemplateID != 7 {
		t.Fatalf("POST product should pass unit_template_id to service command, got %d", repo.createdPublic.UnitTemplateID)
	}
	if repo.createdPublic.UnitRuleOverrideJSON != "{}" {
		t.Fatalf("POST product with template but no advanced override should not write effective units as product override, got %s", repo.createdPublic.UnitRuleOverrideJSON)
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
	if repo.createdPublic.RoastLevel != "" || repo.createdPublic.YieldRate != 1 {
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
			SalesUnit:          "盒",
			DefaultSalesUnit:   "盒",
			SalesUnits:         []string{"kg", "盒", "磅"},
			QuoteUnit:          "盒",
			OrderUnit:          "盒",
			UnitConversionJSON: `{"kg":{"kg":1},"盒":{"kg":0.2},"磅":{"kg":0.453592}}`,
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
		`"default_sales_unit":"盒"`,
		`"sales_units":["kg","盒","磅"]`,
		`"unit_conversion_json":"{\"kg\":{\"kg\":1},\"盒\":{\"kg\":0.2},\"磅\":{\"kg\":0.453592}}"`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/product-settings/units", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings/units status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"盒"`)) || bytes.Contains(rec.Body.Bytes(), []byte(`"product_unit_templates"`)) {
		t.Fatalf("unit dictionary response should only include unit definitions: %s", rec.Body.String())
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
		"default_sales_unit":"盒",
		"sales_units":["盒","磅"],
		"unit_conversion_json":"{\"盒\":{\"kg\":0.2},\"磅\":{\"kg\":0.453592}}",
		"integer_unit":true,
		"active":true
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST unit template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.unitTemplateSaved || repo.savedUnitTemplate.Name != "盒装200g" || repo.savedUnitTemplate.SalesUnit != "盒" || repo.savedUnitTemplate.DefaultSalesUnit != "盒" || !reflect.DeepEqual(repo.savedUnitTemplate.SalesUnits, []string{"kg", "盒", "磅"}) || repo.savedUnitTemplate.QuoteUnit != "盒" || repo.savedUnitTemplate.OrderUnit != "盒" || !strings.Contains(repo.savedUnitTemplate.UnitConversionJSON, `"磅":{"kg":0.453592}`) || !repo.savedUnitTemplate.IntegerUnit {
		t.Fatalf("saved unit template = %+v saved=%v", repo.savedUnitTemplate, repo.unitTemplateSaved)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"sales_units":["kg","盒","磅"]`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"default_sales_unit":"盒"`)) {
		t.Fatalf("unit template response missing semantic fields: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIDeletesGlobalUnitsAndUnitTemplates(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodDelete, "/api/product-settings/units/%E7%9B%92", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE unit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.unitDefinitionDeleted || repo.deletedUnitDefinition.Code != "盒" {
		t.Fatalf("deleted unit definition = %+v deleted=%v", repo.deletedUnitDefinition, repo.unitDefinitionDeleted)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"ok":true`)) {
		t.Fatalf("DELETE unit response should include ok=true: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/product-settings/unit-templates/12", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE unit template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.unitTemplateDeleted || repo.deletedUnitTemplate.ID != 12 {
		t.Fatalf("deleted unit template = %+v deleted=%v", repo.deletedUnitTemplate, repo.unitTemplateDeleted)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"ok":true`)) {
		t.Fatalf("DELETE unit template response should include ok=true: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIDeletesProductConfigTemplate(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodDelete, "/api/product-settings/product-config-templates/377", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE product config template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.configTemplateDeleted || repo.deletedConfigTemplate.ID != 377 {
		t.Fatalf("deleted product config template = %+v deleted=%v", repo.deletedConfigTemplate, repo.configTemplateDeleted)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"ok":true`)) {
		t.Fatalf("DELETE product config template response should include ok=true: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIUpdatesProductIndustryFieldsWithoutLegacyTemplateWrites(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID: 91, Name: "旧SKU名", ProductKind: "roasted", RoastLevel: "中烘", Remark: "旧备注", YieldRate: 0.8,
		}},
		productProductionConfigs: []catalogapp.ProductProductionConfig{{
			ProductID:                91,
			ProductionBomID:          12,
			ProductionBomVersionID:   1203,
			ProcessRouteID:           5,
			IndustryFieldTemplateID:  3001,
			IndustryFieldTemplateIDs: []int64{3001},
			ExpectedLossRate:         0.18,
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
	for _, want := range []string{`"product_production_configs"`, `"industry_field_template_id":3001`, `"industry_field_template_ids":[3001]`, `"template_field_key":"roast_level"`, `"required":true`, `"options_json":"[\"浅烘\",\"中烘\",\"深烘\"]"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing %s: %s", want, rec.Body.String())
		}
	}

	body := bytes.NewBufferString(`{
		"name":"新SKU名",
		"product_kind":"roasted",
		"yield_rate":0.81,
		"remark":"门店常用奶咖",
		"product_config_template_id":301,
		"classification_template_id":401,
		"gradient_template_id_override":18,
		"operation_template_id_override":19,
		"unit_rule_override_json":"{\"order_unit\":\"盒\"}"
	}`)
	req = httptest.NewRequest(http.MethodPut, "/api/products/91", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productUpdated || repo.updated.ProductConfigTemplateID != 0 || repo.updated.ClassificationTemplateID != 0 || repo.updated.GradientTemplateIDOverride != 0 || repo.updated.OperationTemplateIDOverride != 0 || repo.updated.UnitRuleOverrideJSON != "{}" {
		t.Fatalf("updated product template command=%+v updated=%v", repo.updated, repo.productUpdated)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/product-production-configs/91", bytes.NewBufferString(`{
		"production_bom_id":12,
		"production_bom_version_id":1203,
		"process_route_id":5,
		"industry_field_template_id":-9999,
		"industry_field_template_ids":[3002,3001,3002],
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
	if !repo.productionConfigSaved || repo.savedProductionConfig.IndustryFieldTemplateID != 3002 || !reflect.DeepEqual(repo.savedProductionConfig.IndustryFieldTemplateIDs, []int64{3002, 3001}) || len(repo.savedProductionConfig.Fields) != 1 {
		t.Fatalf("saved production config=%+v saved=%v", repo.savedProductionConfig, repo.productionConfigSaved)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"industry_field_template_id":3002`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"industry_field_template_ids":[3002,3001]`)) {
		t.Fatalf("response should expose ordered template ids and legacy first item: %s", rec.Body.String())
	}
	field := repo.savedProductionConfig.Fields[0]
	if field.TemplateFieldKey != "roast_level" || !field.Required || field.OptionsJSON != `["浅烘","中烘","深烘"]` || field.FieldType != "select" {
		t.Fatalf("saved production config field=%+v", field)
	}
}

func TestProductSettingsAPIClearsIndustryFieldsWithoutTemplate(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/product-production-configs/91", bytes.NewBufferString(`{
		"industry_field_template_id":0,
		"expected_loss_rate":0.2,
		"fields":[
			{"field_key":"roast_level","label":"roast_level","value_text":"深烘"},
			{"field_key":"   ","label":"stale_field","value_text":"legacy"}
		]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product production config status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productionConfigSaved {
		t.Fatal("product production config repository was not called")
	}
	if repo.savedProductionConfig.ProductID != 91 {
		t.Fatalf("saved product_id=%d, want 91", repo.savedProductionConfig.ProductID)
	}
	if repo.savedProductionConfig.Fields == nil {
		t.Fatal("saved fields=nil, want non-nil empty fields without industry template")
	}
	if len(repo.savedProductionConfig.Fields) != 0 {
		t.Fatalf("saved fields=%+v, want none without industry template", repo.savedProductionConfig.Fields)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"fields":[]`)) {
		t.Fatalf("response should expose empty fields: %s", rec.Body.String())
	}
}

func TestPR584ProductProductionConfigAPIRejectsNullTemplateIDs(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/product-production-configs/91", bytes.NewBufferString(`{
		"industry_field_template_id":3001,
		"industry_field_template_ids":null,
		"fields":[]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("industry_field_template_ids must be an array")) {
		t.Fatalf("null industry_field_template_ids status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.productionConfigSaved {
		t.Fatal("null industry_field_template_ids must not reach repository")
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

func TestProductSettingsAPICreatesChildSKUUnderParentProduct(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{ID: 88, Name: "埃塞俄比亚 水洗", UnitTemplateID: 12, Active: true}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/skus", bytes.NewBufferString(`{
		"parent_product_id":88,
		"name":"埃塞俄比亚 水洗 227g袋装",
		"sku_name":"227g袋装",
		"sku_code":"ETH-227",
		"barcode":"690000000227",
		"spec_label":"227g",
		"net_content_qty":227,
		"net_content_unit":"g",
		"unit_template_id":12,
		"active":true
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST child sku status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.skuCreated || repo.createdSKU.ParentProductID != 88 || repo.createdSKU.SKUName != "227g袋装" || repo.createdSKU.SKUCode != "ETH-227" || repo.createdSKU.NetContentQty != 227 || repo.createdSKU.NetContentUnit != "g" {
		t.Fatalf("created child SKU command=%+v created=%v", repo.createdSKU, repo.skuCreated)
	}
	for _, want := range []string{`"sku_id":912`, `"parent_product_id":88`, `"sku_name":"227g袋装"`, `"spec_label":"227g"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("child sku response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPISavesSalesSpecTemplateContract(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/unit-templates", bytes.NewBufferString(`{
		"name":"咖啡袋装销售规格",
		"sales_specs":[
			{"spec_key":"bag-227g","spec_name":"227g袋装","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g","active":true},
			{"spec_key":"bag-100g","spec_name":"100g袋装","sales_unit":"袋","net_content_qty":100,"net_content_unit":"g","default":true,"active":true}
		],
		"active":true
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST sales spec template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.unitTemplateSaved || len(repo.savedUnitTemplate.SalesSpecs) != 2 {
		t.Fatalf("saved sales specs command=%+v saved=%v", repo.savedUnitTemplate, repo.unitTemplateSaved)
	}
	if repo.savedUnitTemplate.DefaultSalesUnit != "100g袋装" || !repo.savedUnitTemplate.SalesSpecs[1].Default || repo.savedUnitTemplate.SalesSpecs[1].SalesUnit != "100g袋装" {
		t.Fatalf("selected default sales spec not normalized through API: %+v", repo.savedUnitTemplate)
	}
	if repo.savedUnitTemplate.InventoryUnit != "kg" || repo.savedUnitTemplate.UnitConversionJSON != "{}" {
		t.Fatalf("sales spec template should only use kg as legacy storage fallback and no conversion, got inventory=%q conversion=%q", repo.savedUnitTemplate.InventoryUnit, repo.savedUnitTemplate.UnitConversionJSON)
	}
	for _, want := range []string{`"default_sales_unit":"100g袋装"`, `"sales_specs"`, `"spec_key":"bag-227g"`, `"spec_name":"100g袋装"`, `"sales_unit":"100g袋装"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("sales spec template response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductOptionFromCatalogIncludesDerivedSKUMetadata(t *testing.T) {
	got := productOptionFromCatalog(catalogapp.Product{
		ID:                    91,
		ParentProductID:       88,
		SKUName:               "227g袋装",
		AutoDerivedSKU:        true,
		DerivedUnitTemplateID: 12,
		DerivedSpecKey:        "bag-227g",
		DerivedSpecName:       "227g袋装",
		DerivedSalesUnit:      "袋",
		DerivedSpecStatus:     "active",
	})
	if !got.AutoDerivedSKU || got.DerivedUnitTemplateID != 12 || got.DerivedSpecKey != "bag-227g" || got.DerivedSalesUnit != "袋" || got.DerivedSpecStatus != "active" {
		t.Fatalf("derived SKU metadata lost in API mapping: %+v", got)
	}
}

func TestProductOptionFromCatalogIncludesPerProductDefaultSKUProjection(t *testing.T) {
	parent := productOptionFromCatalog(catalogapp.Product{
		ID: 88, DefaultSKUID: 91, EffectiveDefaultSKUID: 91, DefaultSpecLabel: "1磅", IsDefaultSKU: false,
	})
	if parent.DefaultSKUID != 91 || parent.EffectiveDefaultSKUID != 91 || parent.DefaultSpecLabel != "1磅" || parent.IsDefaultSKU {
		t.Fatalf("parent projection = %+v", parent)
	}
	child := productOptionFromCatalog(catalogapp.Product{
		ID: 91, ParentProductID: 88, DefaultSKUID: 0, EffectiveDefaultSKUID: 91, DefaultSpecLabel: "1磅", IsDefaultSKU: true,
	})
	if child.DefaultSKUID != 0 || child.EffectiveDefaultSKUID != 91 || child.DefaultSpecLabel != "1磅" || !child.IsDefaultSKU {
		t.Fatalf("child projection = %+v", child)
	}
}

func TestProductSettingsAPIUpdatesPerProductDefaultSKU(t *testing.T) {
	repo := &productSettingsRepo{products: []catalogapp.Product{{ID: 88, Name: "初晓", DefaultSpecLabel: "1磅"}}}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("operator_employee", "刘祎泊")
			return next(c)
		}
	})
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/product-settings/products/88/default-sku", strings.NewReader(`{"sku_id":91}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.defaultSKU.Actor != "刘祎泊" || repo.defaultSKU.ParentProductID != 88 || repo.defaultSKU.SKUID != 91 {
		t.Fatalf("command=%+v", repo.defaultSKU)
	}
	for _, want := range []string{`"default_sku_id":91`, `"effective_default_sku_id":91`, `"default_spec_label":"1磅"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPIRejectsInvalidPerProductDefaultSKU(t *testing.T) {
	tests := []struct {
		path string
		body string
	}{
		{path: "/api/product-settings/products/0/default-sku", body: `{"sku_id":91}`},
		{path: "/api/product-settings/products/88/default-sku", body: `{"sku_id":0}`},
		{path: "/api/product-settings/products/88/default-sku", body: `{`},
	}
	for _, tt := range tests {
		e := echo.New()
		registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))
		req := httptest.NewRequest(http.MethodPut, tt.path, strings.NewReader(tt.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s body=%s status=%d, want 400", tt.path, tt.body, rec.Code)
		}
	}

	repo := &productSettingsRepo{defaultSKUErr: catalogapp.ValidationError{Message: "sku does not belong to parent product"}}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))
	req := httptest.NewRequest(http.MethodPut, "/api/product-settings/products/88/default-sku", strings.NewReader(`{"sku_id":99}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "sku does not belong") {
		t.Fatalf("validation status=%d body=%s", rec.Code, rec.Body.String())
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
	expectLegacyClassificationWriteGone(t, rec, "POST assign with context")
	if repo.productAssigned {
		t.Fatalf("legacy product category assignment should not reach repo: %+v", repo.assigned)
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

	body := `{"name":"新公共拼配","remark":"奶咖主推","roast_level":"中深烘","default_price":88,"yield_rate":0.805,"product_config_template_id":301,"classification_template_id":401,"tiers":[{"spec_g":227,"min_qty":1,"unit_price":88}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST public product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicCreated || repo.createdPublic.Name != "新公共拼配" || repo.createdPublic.RoastLevel != "中深烘" || repo.createdPublic.DefaultPrice != 88 || repo.createdPublic.RetailPrice227G != 88 || repo.createdPublic.YieldRate != 1 {
		t.Fatalf("public product command = %+v created=%v", repo.createdPublic, repo.publicCreated)
	}
	if repo.createdPublic.Remark != "奶咖主推" {
		t.Fatalf("public product remark not passed: %+v", repo.createdPublic)
	}
	if repo.createdPublic.ProductConfigTemplateID != 0 || repo.createdPublic.ClassificationTemplateID != 0 || len(repo.createdPublic.Tiers) != 0 {
		t.Fatalf("public product create should not carry legacy template or price tiers, command=%+v", repo.createdPublic)
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
	if !repo.publicCreated || repo.createdPublic.ProductKind != "instant_coffee" || repo.createdPublic.RoastLevel != "" || repo.createdPublic.YieldRate != 1 || repo.createdPublic.DefaultPrice != 39 {
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

func TestProductSettingsAPIIgnoresRetiredProductYieldRate(t *testing.T) {
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
	if !repo.productUpdated || repo.updated.ProductID != 7 || repo.updated.YieldRate != 1 || repo.updated.DefaultPrice != 99 || repo.updated.RetailPrice227G != 48 {
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

func TestProductSettingsAPIReturnsLegacyProductMarginOverrideButIgnoresWrites(t *testing.T) {
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
	if got := float64PtrField(t, repo.updated, "MarginRateOverride"); got != nil {
		t.Fatalf("product margin override write should be ignored, got %v in %+v", *got, repo.updated)
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

func TestProductSettingsAPIReturnsLegacyCustomerSkuMarginOverrideButIgnoresWrites(t *testing.T) {
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
	if got := float64PtrField(t, repo.updated, "MarginRateOverride"); got != nil {
		t.Fatalf("customer SKU margin override write should be ignored, got %v in %+v", *got, repo.updated)
	}
}

func TestProductPriceRecordAPIReadsLegacyRowsAndRejectsWrites(t *testing.T) {
	repo := &productSettingsRepo{
		productPriceGroups: []catalogapp.ProductPriceGroup{{ID: 3, Name: "常规批发", SortOrder: 10, Active: true}},
		productPriceRecords: []catalogapp.ProductPriceRecord{{
			ID: 9, ProductID: 7, FinalUnitPrice: 88.5, PriceUnit: "kg", Currency: "CNY",
			PriceGroupID: 3, PriceGroupName: "常规批发", InventoryUnit: "kg", InventoryConversionJSON: "{}", Status: "published", Active: true,
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-price-records", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET product price records status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"final_unit_price":88.5`, `"price_unit":"kg"`, `"price_group_name":"常规批发"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("price records response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-price-records", bytes.NewBufferString(`{
		"product_id":7,
		"final_unit_price":91.25,
		"price_unit":"kg",
		"currency":"CNY",
		"price_group_name":"客户A常规",
		"inventory_unit":"kg",
		"inventory_conversion_json":{"kg":{"kg":1}},
		"status":"published",
		"remark":"最终价"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("product price records are legacy readonly")) {
		t.Fatalf("POST legacy product price record should be readonly, status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.productPriceRecordSaved {
		t.Fatalf("legacy product price record write should not reach repo")
	}
}

func TestProductPricingRuleAPIReplacesFinalPriceRecordMasterData(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	req := httptest.NewRequest(http.MethodPost, "/api/product-pricing-rules", bytes.NewBufferString(`{
		"name":"成本加成模板",
		"code":"RULE-001",
		"cost_source_mode":"",
		"margin_rate":0.18,
		"tax_rate":0.06,
		"rounding_mode":"",
		"remark":"价格表引用"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST pricing rule status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"name":"成本加成模板"`, `"code":"RULE-001"`, `"cost_source_mode":"bom_current_cost"`, `"rounding_mode":"none"`, `"profit_method":"markup"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("pricing rule response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductPricingRuleAPISavesCalculationTemplateWithoutQuantityTiers(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	req := httptest.NewRequest(http.MethodPost, "/api/product-pricing-rules", bytes.NewBufferString(`{
		"name":"通用 BOM 成本含税模板",
		"code":"RULE-BOM-GENERIC",
		"cost_source_mode":"bom_current_cost",
		"margin_rate":0.35,
		"tax_rate":0.13,
		"rounding_mode":"jiao",
		"formula_version":"v2",
		"calculation_json":{
			"cost_components":["material","operation","labor","equipment_energy","packaging","logistics"],
			"yield_loss_mode":"bom_or_product",
			"profit_method":"gross_margin",
			"tax_mode":"tax_included",
			"minimum_margin_rate":0.18,
			"other_costs":{"包装贴标":1.25,"认证费":2.5},
			"trial_note":"选择商品、报价单位后试算"
		},
		"min_qty":10,
		"max_qty":60,
		"tier_label":"10kg+",
		"tiers":[{"label":"10kg+"}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST pricing rule calculation template status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"cost_source_mode":"bom_current_cost"`,
		`"formula_version":"v2"`,
		`"yield_loss_mode":"bom_or_product"`,
		`"profit_method":"markup"`,
		`"tax_mode":"tax_included"`,
		`"minimum_margin_rate":0.18`,
		`"other_costs":{"包装贴标":1.25,"认证费":2.5}`,
		`"trial_note":"选择商品、报价单位后试算"`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("pricing rule calculation response missing %s: %s", want, rec.Body.String())
		}
	}
	for _, forbidden := range []string{`"cost_components"`, `"min_qty"`, `"max_qty"`, `"tier_label"`, `"tiers"`} {
		if bytes.Contains(rec.Body.Bytes(), []byte(forbidden)) {
			t.Fatalf("pricing rule response must not carry removed field %s: %s", forbidden, rec.Body.String())
		}
	}
}

func TestProductPricingRuleAPINormalizesLegacyWholePercentToMarkup(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	req := httptest.NewRequest(http.MethodPost, "/api/product-pricing-rules", bytes.NewBufferString(`{
		"name":"旧80%毛利模板",
		"margin_rate":80,
		"calculation_json":{"profit_method":"gross_margin","tax_mode":"none"}
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST legacy pricing rule status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"margin_rate":0.8`, `"profit_method":"markup"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("legacy pricing rule response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-pricing-rules", bytes.NewBufferString(`{
		"name":"旧固定加价模板",
		"margin_rate":3,
		"calculation_json":{"profit_method":"fixed_add"}
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("only markup rate is supported")) {
		t.Fatalf("POST fixed-add pricing rule status=%d body=%s, want markup-only validation", rec.Code, rec.Body.String())
	}
}

func TestProductPricingRuleAPIRejectsCleanUpdateOverQuarantinedExistingTemplate(t *testing.T) {
	repo := &productSettingsRepo{productPricingRules: []catalogapp.ProductPricingRule{{
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
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/product-pricing-rules/77", bytes.NewBufferString(`{
		"name":"试图覆盖隔离模板",
		"margin_rate":0.8,
		"active":true,
		"calculation_json":{"profit_method":"markup"}
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("quarantined legacy pricing rule")) {
		t.Fatalf("PUT clean update over quarantined pricing rule status=%d body=%s, want replacement validation", rec.Code, rec.Body.String())
	}
}

func TestProductPricingRuleAPICanDeactivateExistingTemplate(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	req := httptest.NewRequest(http.MethodPut, "/api/product-pricing-rules/63", bytes.NewBufferString(`{
		"name":"停用模板",
		"code":"RULE-INACTIVE",
		"cost_source_mode":"bom_current_cost",
		"active":false
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"active":false`)) {
		t.Fatalf("PUT pricing rule inactive status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProductPricingRuleAPICopyCreateActivatesCopiedTemplate(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	req := httptest.NewRequest(http.MethodPost, "/api/product-pricing-rules", bytes.NewBufferString(`{
		"name":"停用模板 复制",
		"code":"RULE-INACTIVE-COPY",
		"cost_source_mode":"bom_current_cost",
		"margin_rate":0.2,
		"tax_rate":0.13,
		"rounding_mode":"jiao",
		"formula_version":"v2",
		"active":false,
		"calculation_json":{
			"yield_loss_mode":"manual",
			"profit_method":"markup",
			"tax_mode":"tax_included",
			"other_costs":{"包装":1.2},
			"trial_note":"复制模板后再试算"
		}
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST pricing rule copy status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"name":"停用模板 复制"`,
		`"code":"RULE-INACTIVE-COPY"`,
		`"active":true`,
		`"formula_version":"v2"`,
		`"other_costs":{"包装":1.2}`,
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("pricing rule copy response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductPricingRuleAPIRejectsQuantityTierFieldsInsideCalculationTemplate(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	req := httptest.NewRequest(http.MethodPost, "/api/product-pricing-rules", bytes.NewBufferString(`{
		"name":"错误档位模板",
		"code":"RULE-BAD-TIER",
		"calculation_json":{"tiers":[{"label":"10kg+","min_qty":10}]}
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("pricing rule must not contain quantity tiers")) {
		t.Fatalf("POST pricing rule with tier fields should fail, status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProductTierPriceSchemeAPIRejectsLegacyWrites(t *testing.T) {
	repo := &productSettingsRepo{productPriceRecordByID: map[int64]catalogapp.ProductPriceRecord{
		11: {ID: 11, ProductID: 7, FinalUnitPrice: 88, PriceUnit: "kg", Currency: "CNY", Active: true},
	}}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-tier-price-schemes", bytes.NewBufferString(`{
		"name":"批发阶梯",
		"product_id":7,
		"price_group_id":3,
		"tiers":[{"label":"1kg+","min_qty":1,"source_price_record_id":11,"final_unit_price":999,"price_unit":"箱","currency":"USD","position":1}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("product tier price schemes are legacy readonly")) {
		t.Fatalf("POST legacy tier price scheme should be readonly, status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.productTierPriceSchemeSaved {
		t.Fatalf("legacy tier price scheme write should not reach repo")
	}
}

func TestPriceTierTemplateAPIUsesReusableQuantityTiers(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	req := httptest.NewRequest(http.MethodPost, "/api/price-tier-templates", bytes.NewBufferString(`{
		"name":"批发档位",
		"tiers":[
			{"label":"10kg+","min_qty":10,"quantity_unit":"kg","pricing_rule_id":20,"position":2},
			{"label":"1kg+","min_qty":1,"max_qty":10,"pricing_rule_id":10,"position":1}
		]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST price tier template status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"name":"批发档位"`, `"label":"1kg+"`, `"pricing_rule_id":10`, `"quantity_unit":"kg"`, `"label":"10kg+"`, `"pricing_rule_id":20`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("price tier template response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/price-tier-templates", bytes.NewBufferString(`{
		"name":"缺少计算模板",
		"tiers":[{"label":"1kg+","min_qty":1,"quantity_unit":"kg","position":1}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST price tier template without pricing_rule_id status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPriceTierTemplateAPISoftDeletesTemplate(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodDelete, "/api/price-tier-templates/64", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE price tier template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.deletedPriceTierTemplateID != 64 {
		t.Fatalf("deleted price tier template id=%d", repo.deletedPriceTierTemplateID)
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
