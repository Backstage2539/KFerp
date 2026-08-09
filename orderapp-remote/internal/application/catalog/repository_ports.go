package catalog

import "context"

type Repository interface {
	ListProducts(ctx context.Context) ([]Product, error)
	GetProduct(ctx context.Context, id int64) (*Product, error)
	ReplacePriceTiers(ctx context.Context, cmd ReplacePriceTiersCommand) error
	UpdateProductBasics(ctx context.Context, cmd UpdateProductBasicsCommand) error
	DeactivateProducts(ctx context.Context, cmd DeactivateProductsCommand) error
	CreateProduct(ctx context.Context, cmd CreateProductCommand) (Product, error)
	CopyProduct(ctx context.Context, cmd CopyProductCommand) (Product, error)
	CreateSKU(ctx context.Context, cmd CreateSKUCommand) (Product, error)
	SetProductDefaultSKU(ctx context.Context, cmd SetProductDefaultSKUCommand) (Product, error)
	ListProductCategories(ctx context.Context) ([]ProductCategory, error)
	ListProductProductionConfigs(ctx context.Context) ([]ProductProductionConfig, error)
	GetProductProductionConfig(ctx context.Context, productID int64) (ProductProductionConfig, error)
	SaveProductProductionConfig(ctx context.Context, cmd SaveProductProductionConfigCommand) (ProductProductionConfig, error)
	ListProductClassificationTemplates(ctx context.Context) ([]ProductClassificationTemplate, error)
	SaveProductClassificationTemplate(ctx context.Context, cmd SaveProductClassificationTemplateCommand) (ProductClassificationTemplate, error)
	DeleteProductClassificationTemplate(ctx context.Context, cmd DeleteProductClassificationTemplateCommand) error
	SaveProductClassificationCategory(ctx context.Context, cmd SaveProductClassificationCategoryCommand) (ProductClassificationCategory, error)
	DeleteProductClassificationCategory(ctx context.Context, cmd DeleteProductClassificationCategoryCommand) error
	ListProductClassificationTemplateUsages(ctx context.Context) ([]ProductClassificationTemplateUsage, error)
	SaveProductClassificationTemplateUsage(ctx context.Context, cmd SaveProductClassificationTemplateUsageCommand) (ProductClassificationTemplateUsage, error)
	DeleteProductClassificationTemplateUsage(ctx context.Context, cmd DeleteProductClassificationTemplateUsageCommand) error
	ListCustomerProductAliasClassificationTemplateUsages(ctx context.Context, customerID int64) ([]CustomerProductAliasClassificationTemplateUsage, error)
	SaveCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd SaveCustomerProductAliasClassificationTemplateUsageCommand) (CustomerProductAliasClassificationTemplateUsage, error)
	DeleteCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd DeleteCustomerProductAliasClassificationTemplateUsageCommand) error
	SaveProductClassificationAssignment(ctx context.Context, cmd SaveProductClassificationAssignmentCommand) (ProductClassificationAssignment, error)
	SaveCustomerProductAliasClassificationAssignment(ctx context.Context, cmd SaveCustomerProductAliasClassificationAssignmentCommand) (CustomerProductAliasClassificationAssignment, error)
	ListGradientTemplates(ctx context.Context) ([]GradientTemplate, error)
	ListProductConfigTemplates(ctx context.Context) ([]ProductConfigTemplate, error)
	ListProductUnitDefinitions(ctx context.Context) ([]ProductUnitDefinition, error)
	ListProductUnitTemplates(ctx context.Context) ([]ProductUnitTemplate, error)
	ListProductPriceGroups(ctx context.Context) ([]ProductPriceGroup, error)
	SaveProductPriceGroup(ctx context.Context, cmd SaveProductPriceGroupCommand) (ProductPriceGroup, error)
	ListBusinessGroups(ctx context.Context) ([]BusinessGroup, error)
	SaveBusinessGroup(ctx context.Context, cmd BusinessGroup) (BusinessGroup, error)
	DeleteBusinessGroup(ctx context.Context, cmd DeleteBusinessGroupCommand) error
	SaveBusinessGroupItem(ctx context.Context, cmd BusinessGroupItem) (BusinessGroupItem, error)
	DeleteBusinessGroupItem(ctx context.Context, cmd DeleteBusinessGroupItemCommand) error
	MoveBusinessGroupItem(ctx context.Context, cmd MoveBusinessGroupItemCommand) (BusinessGroupItem, error)
	GetBusinessGroupFeatureSelection(ctx context.Context, featureKey string) (BusinessGroupFeatureSelection, error)
	SaveBusinessGroupFeatureSelection(ctx context.Context, cmd SaveBusinessGroupFeatureSelectionCommand) (BusinessGroupFeatureSelection, error)
	ListBusinessGroupAssignments(ctx context.Context, query BusinessGroupAssignmentQuery) ([]BusinessGroupAssignment, error)
	SaveBusinessGroupAssignment(ctx context.Context, cmd BusinessGroupAssignment) (BusinessGroupAssignment, error)
	DeleteBusinessGroupAssignment(ctx context.Context, cmd DeleteBusinessGroupAssignmentCommand) error
	ListProductCustomerReferences(ctx context.Context, productID int64) ([]ProductCustomerReference, error)
	SaveProductCustomerReference(ctx context.Context, cmd ProductCustomerReference) (ProductCustomerReference, error)
	ListProductPricingRules(ctx context.Context) ([]ProductPricingRule, error)
	SaveProductPricingRule(ctx context.Context, cmd ProductPricingRule) (ProductPricingRule, error)
	ListPriceTierTemplates(ctx context.Context) ([]PriceTierTemplate, error)
	SavePriceTierTemplate(ctx context.Context, cmd PriceTierTemplate) (PriceTierTemplate, error)
	DeletePriceTierTemplate(ctx context.Context, id int64, actor string) error
	ListProductPriceRecords(ctx context.Context, query ProductPriceRecordQuery) ([]ProductPriceRecord, error)
	GetProductPriceRecord(ctx context.Context, id int64) (ProductPriceRecord, error)
	SaveProductPriceRecord(ctx context.Context, cmd SaveProductPriceRecordCommand) (ProductPriceRecord, error)
	ListProductTierPriceSchemes(ctx context.Context, query ProductTierPriceSchemeQuery) ([]ProductTierPriceScheme, error)
	SaveProductTierPriceScheme(ctx context.Context, cmd SaveProductTierPriceSchemeCommand) (ProductTierPriceScheme, error)
	SaveGradientTemplate(ctx context.Context, cmd SaveGradientTemplateCommand) (GradientTemplate, error)
	SaveProductConfigTemplate(ctx context.Context, cmd SaveProductConfigTemplateCommand) (ProductConfigTemplate, error)
	DeleteProductConfigTemplate(ctx context.Context, cmd DeleteProductConfigTemplateCommand) error
	SaveProductUnitDefinition(ctx context.Context, cmd SaveProductUnitDefinitionCommand) (ProductUnitDefinition, error)
	SaveProductUnitTemplate(ctx context.Context, cmd SaveProductUnitTemplateCommand) (ProductUnitTemplate, error)
	DeleteProductUnitDefinition(ctx context.Context, cmd DeleteProductUnitDefinitionCommand) error
	DeleteProductUnitTemplate(ctx context.Context, cmd DeleteProductUnitTemplateCommand) error
	DeriveProductConfigTemplate(ctx context.Context, cmd DeriveProductConfigTemplateCommand) (ProductConfigTemplate, error)
	DeactivateGradientTemplate(ctx context.Context, cmd DeactivateGradientTemplateCommand) error
	BindCategoryGradientTemplate(ctx context.Context, cmd BindCategoryGradientTemplateCommand) error
	SaveProductCategory(ctx context.Context, cmd SaveProductCategoryCommand) (ProductCategory, error)
	MoveProductCategory(ctx context.Context, cmd MoveProductCategoryCommand) error
	DeleteProductCategory(ctx context.Context, cmd DeleteProductCategoryCommand) error
	AssignProductCategory(ctx context.Context, cmd AssignProductCategoryCommand) (AssignProductCategoryResult, error)
	CreateCustomProduct(ctx context.Context, cmd CreateCustomProductCommand) (Product, error)
	DeriveProductCategory(ctx context.Context, cmd DeriveProductCategoryCommand) (ProductCategory, error)
	DeriveCustomerProduct(ctx context.Context, cmd DeriveCustomerProductCommand) (Product, error)
	DeriveGradientTemplate(ctx context.Context, cmd DeriveGradientTemplateCommand) (GradientTemplate, error)
	ListCustomerPublicUsages(ctx context.Context) ([]CustomerPublicUsage, error)
	SaveCustomerPublicUsage(ctx context.Context, cmd CustomerPublicUsageCommand) (CustomerPublicUsage, error)
	EnsureFactoryCustomer(ctx context.Context, actor string) (int64, error)
	ListCustomerProductAliases(ctx context.Context, query CustomerProductAliasQuery) ([]CustomerProductAlias, error)
	SaveCustomerProductAlias(ctx context.Context, cmd CustomerProductAliasCommand) (CustomerProductAlias, error)
	BatchCreateCustomerProductAliases(ctx context.Context, cmd BatchCustomerProductAliasesCommand) (BatchCustomerProductAliasesResult, error)
	DisableCustomerProductAlias(ctx context.Context, cmd DisableCustomerProductAliasCommand) error
	BatchDisableCustomerProductAliases(ctx context.Context, cmd BatchDisableCustomerProductAliasesCommand) (BatchDisableCustomerProductAliasesResult, error)
	ListCustomerProductAliasIndustryFields(ctx context.Context, query CustomerProductAliasIndustryFieldQuery) ([]ProductProductionConfigField, error)
	SaveCustomerProductAliasIndustryFields(ctx context.Context, cmd SaveCustomerProductAliasIndustryFieldsCommand) ([]ProductProductionConfigField, error)
	ListCustomerProductAliasMigrationCandidates(ctx context.Context, query CustomerProductAliasMigrationCandidateQuery) ([]CustomerProductAliasMigrationCandidate, error)
	ListCustomerProductRuleTemplates(ctx context.Context) ([]CustomerProductRuleTemplate, error)
	ListCustomerProductRuleOverrides(ctx context.Context) ([]CustomerProductRuleOverride, error)
	ListCustomerProductRuleBindings(ctx context.Context) ([]CustomerProductRuleBinding, error)
	SaveCustomerProductRuleTemplate(ctx context.Context, cmd SaveCustomerProductRuleTemplateCommand) (CustomerProductRuleTemplate, error)
	SaveCustomerProductRuleOverride(ctx context.Context, cmd SaveCustomerProductRuleOverrideCommand) (CustomerProductRuleOverride, error)
	BindCustomerProductRuleTemplate(ctx context.Context, cmd CustomerProductRuleTemplateBindingCommand) (CustomerProductRuleBinding, error)
}
