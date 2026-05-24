package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	catalogdomain "orderapp/internal/domain/catalog"
	"sort"
	"strings"
)

type PriceTier struct {
	ID        int64
	SpecG     int64
	MinQty    float64
	MaxQty    *float64
	UnitPrice float64
}

type Product struct {
	ID                          int64
	Name                        string
	Remark                      string
	ProductKind                 string
	GreenBeanType               string
	GreenBeanBomProductID       int64
	RoastLevel                  string
	DripBagGrams                float64
	DripBoxBagCount             int
	AllowFulfillmentOrder       bool
	AllowMallOrder              bool
	SalesUnits                  []string
	DefaultPrice                float64
	RetailPrice100G             float64
	RetailPrice200G             float64
	RetailPrice227G             float64
	RetailPrice250G             float64
	YieldRate                   float64
	ProductCategoryID           int64
	ProductCategoryPosition     int
	CustomerID                  int64
	BaseProductID               int64
	Visibility                  string
	CustomType                  string
	MarginRateOverride          *float64
	GradientTemplateIDOverride  int64
	OperationTemplateIDOverride int64
	UnitRuleOverrideJSON        string
	BomItemCount                int
	BomStatus                   string
	OrderUsageCount             int
	Tiers                       []PriceTier
}

const (
	TemplateStatePublic   = "public_template"
	TemplateStateDerived  = "derived_from_public"
	TemplateStateCustomer = "customer_owned"
)

type ProductCategory struct {
	ID                  int64  `json:"id"`
	ParentID            int64  `json:"parent_id"`
	CustomerID          int64  `json:"customer_id"`
	SourceCategoryID    int64  `json:"source_category_id"`
	Name                string `json:"name"`
	Level               int    `json:"level"`
	Position            int    `json:"position"`
	Number              int    `json:"number"`
	GradientTemplateID  int64  `json:"gradient_template_id"`
	OperationTemplateID int64  `json:"operation_template_id"`
	PriceListRuleJSON   string `json:"price_list_rule_json"`
	InventoryUnit       string `json:"inventory_unit"`
	QuoteUnit           string `json:"quote_unit"`
	OrderUnit           string `json:"order_unit"`
	UnitConversionJSON  string `json:"unit_conversion_json"`
	IntegerUnit         bool   `json:"integer_unit"`
	TemplateState       string `json:"template_state"`
}

type ProductSettingsProduct struct {
	ID                          int64    `json:"id"`
	Name                        string   `json:"name"`
	Remark                      string   `json:"remark"`
	ProductKind                 string   `json:"product_kind"`
	GreenBeanType               string   `json:"green_bean_type"`
	GreenBeanBomProductID       int64    `json:"green_bean_bom_product_id"`
	RoastLevel                  string   `json:"roast_level"`
	DripBagGrams                float64  `json:"drip_bag_grams"`
	DripBoxBagCount             int      `json:"drip_box_bag_count"`
	AllowFulfillmentOrder       bool     `json:"allow_fulfillment_order"`
	AllowMallOrder              bool     `json:"allow_mall_order"`
	SalesUnits                  []string `json:"sales_units"`
	DefaultPrice                float64  `json:"default_price"`
	RetailPrice100G             float64  `json:"retail_price_100g"`
	RetailPrice200G             float64  `json:"retail_price_200g"`
	RetailPrice227G             float64  `json:"retail_price_227g"`
	RetailPrice250G             float64  `json:"retail_price_250g"`
	YieldRate                   float64  `json:"yield_rate"`
	ProductCategoryID           int64    `json:"product_category_id"`
	ProductCategoryPosition     int      `json:"product_category_position"`
	CustomerID                  int64    `json:"customer_id"`
	BaseProductID               int64    `json:"base_product_id"`
	Visibility                  string   `json:"visibility"`
	CustomType                  string   `json:"custom_type"`
	MarginRateOverride          *float64 `json:"margin_rate_override"`
	GradientTemplateIDOverride  int64    `json:"gradient_template_id_override"`
	OperationTemplateIDOverride int64    `json:"operation_template_id_override"`
	UnitRuleOverrideJSON        string   `json:"unit_rule_override_json"`
	BomItemCount                int      `json:"bom_item_count"`
	BomStatus                   string   `json:"bom_status"`
	OrderUsageCount             int      `json:"order_usage_count"`
	Number                      int      `json:"number"`
}

type ProductCategoryNode struct {
	ProductCategory
	Children []ProductCategoryNode    `json:"children"`
	Products []ProductSettingsProduct `json:"products"`
}

type ProductSettingsData struct {
	Categories                   []ProductCategoryNode         `json:"categories"`
	Products                     []ProductSettingsProduct      `json:"products"`
	GradientTemplates            []GradientTemplate            `json:"gradient_templates"`
	CustomerPublicUsages         []CustomerPublicUsage         `json:"customer_public_usages"`
	CustomerProductRuleTemplates []CustomerProductRuleTemplate `json:"customer_product_rule_templates"`
	CustomerProductRuleOverrides []CustomerProductRuleOverride `json:"customer_product_rule_overrides"`
	CustomerProductRuleBindings  []CustomerProductRuleBinding  `json:"customer_product_rule_bindings"`
}

type GradientTemplate struct {
	ID               int64                  `json:"id"`
	Name             string                 `json:"name"`
	CustomerID       int64                  `json:"customer_id"`
	SourceTemplateID int64                  `json:"source_template_id"`
	TemplateState    string                 `json:"template_state"`
	DisplayUnit      string                 `json:"display_unit"`
	Active           bool                   `json:"active"`
	Tiers            []GradientTemplateTier `json:"tiers"`
}

type GradientTemplateTier struct {
	ID            int64    `json:"id"`
	TemplateID    int64    `json:"template_id,omitempty"`
	Label         string   `json:"label"`
	MinDisplayQty *float64 `json:"min_display_qty,omitempty"`
	MaxDisplayQty *float64 `json:"max_display_qty,omitempty"`
	MinWeightG    float64  `json:"min_weight_g"`
	MaxWeightG    *float64 `json:"max_weight_g,omitempty"`
	MarginRate    float64  `json:"margin_rate"`
	Position      int      `json:"position"`
}

type ReplacePriceTiersCommand struct {
	Actor                 string
	ProductID             int64
	ProductKind           string
	GreenBeanType         string
	GreenBeanBomProductID int64
	DefaultPrice          float64
	RoastLevel            string
	RetailPrice100G       float64
	RetailPrice200G       float64
	RetailPrice227G       float64
	RetailPrice250G       float64
	YieldRate             float64
	Tiers                 []PriceTier
}

type UpdateProductBasicsCommand struct {
	Actor                       string
	ProductID                   int64
	Name                        string
	ProductKind                 string
	Remark                      string
	GreenBeanType               string
	GreenBeanBomProductID       int64
	DefaultPrice                float64
	RoastLevel                  string
	DripBagGrams                float64
	DripBoxBagCount             int
	AllowFulfillmentOrder       bool
	AllowMallOrder              bool
	SalesUnits                  []string
	RetailPrice100G             float64
	RetailPrice200G             float64
	RetailPrice227G             float64
	RetailPrice250G             float64
	YieldRate                   float64
	MarginRateOverride          *float64
	GradientTemplateIDOverride  int64
	OperationTemplateIDOverride int64
	UnitRuleOverrideJSON        string
}

type CreateProductCommand struct {
	Actor                    string
	Name                     string
	Remark                   string
	ProductKind              string
	GreenBeanType            string
	GreenBeanBomProductID    int64
	RoastLevel               string
	DripBagGrams             float64
	DripBoxBagCount          int
	AllowFulfillmentOrder    bool
	AllowFulfillmentOrderSet bool
	AllowMallOrder           bool
	SalesUnits               []string
	DefaultPrice             float64
	RetailPrice100G          float64
	RetailPrice200G          float64
	RetailPrice227G          float64
	RetailPrice250G          float64
	YieldRate                float64
	Tiers                    []PriceTier
}

type DeactivateProductsCommand struct {
	Actor      string
	ProductIDs []int64
}

type CreateCustomProductCommand struct {
	Actor                 string
	CustomerID            int64
	BaseProductID         int64
	Name                  string
	Remark                string
	ProductKind           string
	GreenBeanType         string
	GreenBeanBomProductID int64
	RoastLevel            string
	DripBagGrams          float64
	DripBoxBagCount       int
	CustomType            string
	CopyBOM               bool
	CopyPriceTiers        bool
}

type CustomerPublicUsage struct {
	CustomerID                 int64 `json:"customer_id"`
	UsePublicSKU               bool  `json:"use_public_sku"`
	UsePublicCategories        bool  `json:"use_public_categories"`
	UsePublicGradientTemplates bool  `json:"use_public_gradient_templates"`
}

type CustomerPublicUsageCommand struct {
	Actor                      string
	CustomerID                 int64
	UsePublicSKU               bool
	UsePublicCategories        bool
	UsePublicGradientTemplates bool
}

type CustomerProductRuleTemplateItem struct {
	ID                       int64  `json:"id,omitempty"`
	TemplateID               int64  `json:"template_id,omitempty"`
	ProductSubtypeCategoryID int64  `json:"product_subtype_category_id"`
	GradientTemplateID       int64  `json:"gradient_template_id"`
	OperationTemplateID      int64  `json:"operation_template_id"`
	PriceListRuleJSON        string `json:"price_list_rule_json"`
	UnitRuleJSON             string `json:"unit_rule_json"`
}

type CustomerProductRuleTemplate struct {
	ID         int64                             `json:"id"`
	CustomerID int64                             `json:"customer_id"`
	Name       string                            `json:"name"`
	Active     bool                              `json:"active"`
	Items      []CustomerProductRuleTemplateItem `json:"items"`
}

type CustomerProductRuleOverride struct {
	ID                       int64  `json:"id"`
	CustomerID               int64  `json:"customer_id"`
	ProductSubtypeCategoryID int64  `json:"product_subtype_category_id"`
	GradientTemplateID       int64  `json:"gradient_template_id"`
	OperationTemplateID      int64  `json:"operation_template_id"`
	PriceListRuleJSON        string `json:"price_list_rule_json"`
	UnitRuleJSON             string `json:"unit_rule_json"`
	Active                   bool   `json:"active"`
}

type CustomerProductRuleBinding struct {
	CustomerID int64 `json:"customer_id"`
	TemplateID int64 `json:"template_id"`
}

type SaveCustomerProductRuleTemplateCommand struct {
	Actor      string
	ID         int64
	CustomerID int64
	Name       string
	Active     *bool
	Items      []CustomerProductRuleTemplateItem
}

type SaveCustomerProductRuleOverrideCommand struct {
	Actor                    string
	ID                       int64
	CustomerID               int64
	ProductSubtypeCategoryID int64
	GradientTemplateID       int64
	OperationTemplateID      int64
	PriceListRuleJSON        string
	UnitRuleJSON             string
	Active                   *bool
}

type CustomerProductRuleTemplateBindingCommand struct {
	Actor      string
	CustomerID int64
	TemplateID int64
}

type SaveProductCategoryCommand struct {
	Actor               string
	ID                  int64
	ParentID            int64
	CustomerID          int64
	Name                string
	Position            int
	GradientTemplateID  int64
	OperationTemplateID int64
	PriceListRuleJSON   string
	InventoryUnit       string
	QuoteUnit           string
	OrderUnit           string
	UnitConversionJSON  string
	IntegerUnit         bool
}

type MoveProductCategoryCommand struct {
	Actor    string
	ID       int64
	ParentID int64
	Position int
}

type DeleteProductCategoryCommand struct {
	Actor string
	ID    int64
}

type AssignProductCategoryCommand struct {
	Actor                string
	ProductID            int64
	CategoryID           int64
	CustomerID           int64
	Position             int
	DerivePublicCategory bool
	DerivePublicProduct  bool
}

type AssignProductCategoryResult struct {
	ProductID          int64 `json:"product_id"`
	CategoryID         int64 `json:"category_id"`
	DerivedProductID   int64 `json:"derived_product_id"`
	DerivedCategoryID  int64 `json:"derived_category_id"`
	UsedPublicProduct  bool  `json:"used_public_product"`
	UsedPublicCategory bool  `json:"used_public_category"`
}

type DeriveProductCategoryCommand struct {
	Actor            string
	CustomerID       int64
	SourceCategoryID int64
}

type DeriveCustomerProductCommand struct {
	Actor          string
	CustomerID     int64
	BaseProductID  int64
	CategoryID     int64
	Position       int
	Name           string
	CopyBOM        bool
	CopyPriceTiers bool
}

type DeriveGradientTemplateCommand struct {
	Actor            string
	CustomerID       int64
	SourceTemplateID int64
	Name             string
}

type SaveGradientTemplateCommand struct {
	Actor       string
	ID          int64
	CustomerID  int64
	Name        string
	DisplayUnit string
	Tiers       []GradientTemplateTier
}

type DeactivateGradientTemplateCommand struct {
	Actor string
	ID    int64
}

type BindCategoryGradientTemplateCommand struct {
	Actor              string
	CategoryID         int64
	GradientTemplateID int64
}

type Repository interface {
	ListProducts(ctx context.Context) ([]Product, error)
	GetProduct(ctx context.Context, id int64) (*Product, error)
	ReplacePriceTiers(ctx context.Context, cmd ReplacePriceTiersCommand) error
	UpdateProductBasics(ctx context.Context, cmd UpdateProductBasicsCommand) error
	DeactivateProducts(ctx context.Context, cmd DeactivateProductsCommand) error
	CreateProduct(ctx context.Context, cmd CreateProductCommand) (Product, error)
	ListProductCategories(ctx context.Context) ([]ProductCategory, error)
	ListGradientTemplates(ctx context.Context) ([]GradientTemplate, error)
	SaveGradientTemplate(ctx context.Context, cmd SaveGradientTemplateCommand) (GradientTemplate, error)
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
	ListCustomerProductRuleTemplates(ctx context.Context) ([]CustomerProductRuleTemplate, error)
	ListCustomerProductRuleOverrides(ctx context.Context) ([]CustomerProductRuleOverride, error)
	ListCustomerProductRuleBindings(ctx context.Context) ([]CustomerProductRuleBinding, error)
	SaveCustomerProductRuleTemplate(ctx context.Context, cmd SaveCustomerProductRuleTemplateCommand) (CustomerProductRuleTemplate, error)
	SaveCustomerProductRuleOverride(ctx context.Context, cmd SaveCustomerProductRuleOverrideCommand) (CustomerProductRuleOverride, error)
	BindCustomerProductRuleTemplate(ctx context.Context, cmd CustomerProductRuleTemplateBindingCommand) (CustomerProductRuleBinding, error)
}

type Service struct {
	repo Repository
}

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func IsValidationError(err error) bool {
	var validationErr ValidationError
	return errors.As(err, &validationErr)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListProducts(ctx context.Context) ([]Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *Service) GetProduct(ctx context.Context, id int64) (*Product, error) {
	return s.repo.GetProduct(ctx, id)
}

func (s *Service) ReplacePriceTiers(ctx context.Context, cmd ReplacePriceTiersCommand) error {
	cmd.ProductKind = catalogdomain.NormalizeProductKind(cmd.ProductKind)
	if err := normalizeProductSalesShape(&cmd.ProductKind, &cmd.GreenBeanType, &cmd.GreenBeanBomProductID, &cmd.RoastLevel, &cmd.DefaultPrice, &cmd.RetailPrice100G, &cmd.RetailPrice200G, &cmd.RetailPrice227G, &cmd.RetailPrice250G, &cmd.YieldRate, &cmd.Tiers); err != nil {
		return err
	}
	if cmd.ProductKind == catalogdomain.ProductKindGreenBean {
		if err := s.validateGreenBeanBomProduct(ctx, cmd.GreenBeanBomProductID); err != nil {
			return err
		}
	}
	return s.repo.ReplacePriceTiers(ctx, cmd)
}

func (s *Service) UpdateProductBasics(ctx context.Context, cmd UpdateProductBasicsCommand) error {
	var err error
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount, cmd.SalesUnits, err = normalizeProductKindSettings(cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount)
	if err != nil {
		return err
	}
	if err := normalizeProductSalesShape(&cmd.ProductKind, &cmd.GreenBeanType, &cmd.GreenBeanBomProductID, &cmd.RoastLevel, &cmd.DefaultPrice, &cmd.RetailPrice100G, &cmd.RetailPrice200G, &cmd.RetailPrice227G, &cmd.RetailPrice250G, &cmd.YieldRate, nil); err != nil {
		return err
	}
	if cmd.ProductKind == catalogdomain.ProductKindGreenBean {
		if err := s.validateGreenBeanBomProduct(ctx, cmd.GreenBeanBomProductID); err != nil {
			return err
		}
	}
	if cmd.GradientTemplateIDOverride < 0 || cmd.OperationTemplateIDOverride < 0 {
		return ValidationError{Message: "invalid template override"}
	}
	unitRuleOverrideJSON, err := normalizeJSONText(cmd.UnitRuleOverrideJSON)
	if err != nil {
		return ValidationError{Message: "invalid unit_rule_override_json"}
	}
	cmd.UnitRuleOverrideJSON = unitRuleOverrideJSON
	return s.repo.UpdateProductBasics(ctx, cmd)
}

func (s *Service) DeactivateProducts(ctx context.Context, cmd DeactivateProductsCommand) error {
	ids := make([]int64, 0, len(cmd.ProductIDs))
	seen := map[int64]bool{}
	for _, id := range cmd.ProductIDs {
		if id <= 0 || seen[id] {
			continue
		}
		ids = append(ids, id)
		seen[id] = true
	}
	if len(ids) == 0 {
		return fmt.Errorf("product_ids required")
	}
	cmd.ProductIDs = ids
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.DeactivateProducts(ctx, cmd)
}

func (s *Service) CreateProduct(ctx context.Context, cmd CreateProductCommand) (Product, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.Name == "" {
		return Product{}, ValidationError{Message: "name required"}
	}
	var err error
	cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount, cmd.SalesUnits, err = normalizeProductKindSettings(cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount)
	if err != nil {
		return Product{}, err
	}
	if !cmd.AllowFulfillmentOrderSet {
		cmd.AllowFulfillmentOrder = true
	}
	if cmd.DefaultPrice < 0 || cmd.RetailPrice100G < 0 || cmd.RetailPrice200G < 0 || cmd.RetailPrice227G < 0 || cmd.RetailPrice250G < 0 {
		return Product{}, ValidationError{Message: "price must not be negative"}
	}
	if cmd.RetailPrice227G <= 0 && cmd.DefaultPrice > 0 {
		cmd.RetailPrice227G = cmd.DefaultPrice
	}
	if cmd.ProductKind == catalogdomain.ProductKindGreenBean {
		if err := normalizeProductSalesShape(&cmd.ProductKind, &cmd.GreenBeanType, &cmd.GreenBeanBomProductID, &cmd.RoastLevel, &cmd.DefaultPrice, &cmd.RetailPrice100G, &cmd.RetailPrice200G, &cmd.RetailPrice227G, &cmd.RetailPrice250G, &cmd.YieldRate, &cmd.Tiers); err != nil {
			return Product{}, err
		}
		if err := s.validateGreenBeanBomProduct(ctx, cmd.GreenBeanBomProductID); err != nil {
			return Product{}, err
		}
	} else if catalogdomain.ProductKindRequiresRoast(cmd.ProductKind) {
		cmd.RoastLevel = catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
		if cmd.RoastLevel == "" {
			return Product{}, ValidationError{Message: "invalid roast_level"}
		}
		if cmd.YieldRate <= 0 {
			cmd.YieldRate = catalogdomain.ResolveYieldRate(cmd.RoastLevel, 0.8)
		}
		if cmd.YieldRate <= 0 || cmd.YieldRate > 1 {
			return Product{}, ValidationError{Message: "invalid yield_rate"}
		}
	} else if err := normalizeProductSalesShape(&cmd.ProductKind, &cmd.GreenBeanType, &cmd.GreenBeanBomProductID, &cmd.RoastLevel, &cmd.DefaultPrice, &cmd.RetailPrice100G, &cmd.RetailPrice200G, &cmd.RetailPrice227G, &cmd.RetailPrice250G, &cmd.YieldRate, &cmd.Tiers); err != nil {
		return Product{}, err
	}
	return s.repo.CreateProduct(ctx, cmd)
}

func (s *Service) validateGreenBeanBomProduct(ctx context.Context, productID int64) error {
	product, err := s.repo.GetProduct(ctx, productID)
	if err != nil {
		return err
	}
	if product == nil || product.ID <= 0 {
		return ValidationError{Message: "green_bean_bom_product_id not found"}
	}
	if catalogdomain.NormalizeProductKind(product.ProductKind) == catalogdomain.ProductKindGreenBean {
		return ValidationError{Message: "green_bean_bom_product_id must reference roasted product"}
	}
	return nil
}

func normalizeProductSalesShape(productKind *string, greenBeanType *string, greenBeanBomProductID *int64, roastLevel *string, defaultPrice *float64, retailPrice100G *float64, retailPrice200G *float64, retailPrice227G *float64, retailPrice250G *float64, yieldRate *float64, tiers *[]PriceTier) error {
	kind := catalogdomain.NormalizeProductKind(*productKind)
	*productKind = kind
	if kind != catalogdomain.ProductKindGreenBean {
		*greenBeanType = ""
		*greenBeanBomProductID = 0
		if !catalogdomain.ProductKindRequiresRoast(kind) {
			*roastLevel = ""
			*yieldRate = 0
		}
		return nil
	}
	*greenBeanType = normalizeGreenBeanType(*greenBeanType)
	if *greenBeanBomProductID <= 0 {
		return fmt.Errorf("green_bean_bom_product_id required")
	}
	*roastLevel = ""
	*defaultPrice = 0
	*retailPrice100G = 0
	*retailPrice200G = 0
	*retailPrice227G = 0
	*retailPrice250G = 0
	*yieldRate = 0
	if tiers != nil {
		*tiers = nil
	}
	return nil
}

func normalizeGreenBeanType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "blend", "拼配":
		return "blend"
	case "single_origin", "single", "单品":
		return "single_origin"
	default:
		return "single_origin"
	}
}

func (s *Service) ProductSettings(ctx context.Context) (ProductSettingsData, error) {
	categories, err := s.repo.ListProductCategories(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	products, err := s.repo.ListProducts(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	templates, err := s.repo.ListGradientTemplates(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	usages, err := s.repo.ListCustomerPublicUsages(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	ruleTemplates, err := s.repo.ListCustomerProductRuleTemplates(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	ruleOverrides, err := s.repo.ListCustomerProductRuleOverrides(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	ruleBindings, err := s.repo.ListCustomerProductRuleBindings(ctx)
	if err != nil {
		return ProductSettingsData{}, err
	}
	data := BuildProductSettings(categories, products)
	data.GradientTemplates = templates
	data.CustomerPublicUsages = usages
	data.CustomerProductRuleTemplates = ruleTemplates
	data.CustomerProductRuleOverrides = ruleOverrides
	data.CustomerProductRuleBindings = ruleBindings
	return data, nil
}

func (s *Service) ListGradientTemplates(ctx context.Context) ([]GradientTemplate, error) {
	return s.repo.ListGradientTemplates(ctx)
}

func (s *Service) SaveGradientTemplate(ctx context.Context, cmd SaveGradientTemplateCommand) (GradientTemplate, error) {
	normalized, err := normalizeGradientTemplateCommand(cmd)
	if err != nil {
		return GradientTemplate{}, err
	}
	return s.repo.SaveGradientTemplate(ctx, normalized)
}

func (s *Service) DeactivateGradientTemplate(ctx context.Context, cmd DeactivateGradientTemplateCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("invalid id")
	}
	return s.repo.DeactivateGradientTemplate(ctx, cmd)
}

func (s *Service) BindCategoryGradientTemplate(ctx context.Context, cmd BindCategoryGradientTemplateCommand) error {
	if cmd.CategoryID <= 0 {
		return fmt.Errorf("invalid category")
	}
	if cmd.GradientTemplateID < 0 {
		return fmt.Errorf("invalid gradient_template_id")
	}
	return s.repo.BindCategoryGradientTemplate(ctx, cmd)
}

func (s *Service) SaveProductCategory(ctx context.Context, cmd SaveProductCategoryCommand) (ProductCategory, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return ProductCategory{}, ValidationError{Message: "name required"}
	}
	if cmd.GradientTemplateID < 0 || cmd.OperationTemplateID < 0 {
		return ProductCategory{}, ValidationError{Message: "invalid template id"}
	}
	priceRuleJSON, err := normalizeJSONText(cmd.PriceListRuleJSON)
	if err != nil {
		return ProductCategory{}, ValidationError{Message: "invalid price_list_rule_json"}
	}
	unitRule := catalogdomain.NormalizeProductUnitRule(catalogdomain.ProductUnitRule{
		InventoryUnit:  cmd.InventoryUnit,
		QuoteUnit:      cmd.QuoteUnit,
		OrderUnit:      cmd.OrderUnit,
		ConversionJSON: cmd.UnitConversionJSON,
		IntegerUnit:    cmd.IntegerUnit,
	})
	unitConversionJSON, err := normalizeJSONText(unitRule.ConversionJSON)
	if err != nil {
		return ProductCategory{}, ValidationError{Message: "invalid unit_conversion_json"}
	}
	cmd.PriceListRuleJSON = priceRuleJSON
	cmd.InventoryUnit = unitRule.InventoryUnit
	cmd.QuoteUnit = unitRule.QuoteUnit
	cmd.OrderUnit = unitRule.OrderUnit
	cmd.UnitConversionJSON = unitConversionJSON
	cmd.IntegerUnit = unitRule.IntegerUnit
	return s.repo.SaveProductCategory(ctx, cmd)
}

func (s *Service) MoveProductCategory(ctx context.Context, cmd MoveProductCategoryCommand) error {
	return s.repo.MoveProductCategory(ctx, cmd)
}

func (s *Service) DeleteProductCategory(ctx context.Context, cmd DeleteProductCategoryCommand) error {
	return s.repo.DeleteProductCategory(ctx, cmd)
}

func (s *Service) AssignProductCategory(ctx context.Context, cmd AssignProductCategoryCommand) (AssignProductCategoryResult, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.AssignProductCategory(ctx, cmd)
}

func (s *Service) CreateCustomProduct(ctx context.Context, cmd CreateCustomProductCommand) (Product, error) {
	if cmd.CustomerID <= 0 {
		return Product{}, fmt.Errorf("customer_id required")
	}
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Remark = strings.TrimSpace(cmd.Remark)
	if cmd.Name == "" {
		return Product{}, fmt.Errorf("name required")
	}
	cmd.CustomType = strings.TrimSpace(cmd.CustomType)
	if cmd.CustomType != "custom_blend" && cmd.CustomType != "custom_roast" && cmd.CustomType != "public_sku_alias" {
		return Product{}, fmt.Errorf("invalid custom_type")
	}
	requestedKind := strings.TrimSpace(cmd.ProductKind)
	cmd.ProductKind = catalogdomain.NormalizeProductKind(cmd.ProductKind)
	var base *Product
	var err error
	requiresBaseProduct := cmd.CustomType != "custom_roast" && (requestedKind == "" || cmd.ProductKind != catalogdomain.ProductKindGreenBean)
	if requiresBaseProduct {
		if cmd.BaseProductID <= 0 {
			return Product{}, fmt.Errorf("base_product_id required")
		}
		base, err = s.repo.GetProduct(ctx, cmd.BaseProductID)
		if err != nil {
			return Product{}, err
		}
		if base == nil || base.ID <= 0 {
			return Product{}, fmt.Errorf("base product not found")
		}
		baseKind := catalogdomain.NormalizeProductKind(base.ProductKind)
		if requestedKind == "" {
			cmd.ProductKind = baseKind
		} else if cmd.ProductKind != baseKind {
			return Product{}, fmt.Errorf("product_kind must match base product")
		}
	}
	if cmd.CustomType == "custom_roast" {
		cmd.BaseProductID = 0
		cmd.CopyBOM = false
		cmd.CopyPriceTiers = false
	}
	if cmd.ProductKind == catalogdomain.ProductKindGreenBean {
		cmd.BaseProductID = 0
		cmd.RoastLevel = ""
		cmd.GreenBeanType = normalizeGreenBeanType(cmd.GreenBeanType)
		if cmd.GreenBeanBomProductID <= 0 && base != nil {
			cmd.GreenBeanBomProductID = base.GreenBeanBomProductID
		}
		if cmd.GreenBeanBomProductID <= 0 {
			return Product{}, ValidationError{Message: "green_bean_bom_product_id required"}
		}
		if err := s.validateGreenBeanBomProduct(ctx, cmd.GreenBeanBomProductID); err != nil {
			return Product{}, err
		}
		cmd.CopyBOM = false
		cmd.CopyPriceTiers = false
	} else if catalogdomain.ProductKindRequiresRoast(cmd.ProductKind) {
		cmd.RoastLevel = catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
		if cmd.RoastLevel == "" {
			return Product{}, fmt.Errorf("invalid roast_level")
		}
		cmd.GreenBeanType = ""
		cmd.GreenBeanBomProductID = 0
	} else {
		cmd.RoastLevel = ""
		cmd.GreenBeanType = ""
		cmd.GreenBeanBomProductID = 0
		cmd.CopyBOM = false
	}
	if cmd.ProductKind == catalogdomain.ProductKindDripBag {
		if cmd.DripBagGrams <= 0 && base != nil {
			cmd.DripBagGrams = base.DripBagGrams
		}
		if cmd.DripBoxBagCount <= 0 && base != nil {
			cmd.DripBoxBagCount = base.DripBoxBagCount
		}
		var salesUnits []string
		cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount, salesUnits, err = normalizeProductKindSettings(cmd.ProductKind, cmd.DripBagGrams, cmd.DripBoxBagCount)
		if err != nil {
			return Product{}, err
		}
		_ = salesUnits
		cmd.CopyBOM = false
	}
	return s.repo.CreateCustomProduct(ctx, cmd)
}

func (s *Service) DeriveProductCategory(ctx context.Context, cmd DeriveProductCategoryCommand) (ProductCategory, error) {
	if cmd.CustomerID <= 0 {
		return ProductCategory{}, fmt.Errorf("customer_id required")
	}
	if cmd.SourceCategoryID <= 0 {
		return ProductCategory{}, fmt.Errorf("source_category_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.DeriveProductCategory(ctx, cmd)
}

func (s *Service) DeriveCustomerProduct(ctx context.Context, cmd DeriveCustomerProductCommand) (Product, error) {
	if cmd.CustomerID <= 0 {
		return Product{}, fmt.Errorf("customer_id required")
	}
	if cmd.BaseProductID <= 0 {
		return Product{}, fmt.Errorf("base_product_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	return s.repo.DeriveCustomerProduct(ctx, cmd)
}

func (s *Service) DeriveGradientTemplate(ctx context.Context, cmd DeriveGradientTemplateCommand) (GradientTemplate, error) {
	if cmd.CustomerID <= 0 {
		return GradientTemplate{}, fmt.Errorf("customer_id required")
	}
	if cmd.SourceTemplateID <= 0 {
		return GradientTemplate{}, fmt.Errorf("source_template_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	return s.repo.DeriveGradientTemplate(ctx, cmd)
}

func (s *Service) SaveCustomerPublicUsage(ctx context.Context, cmd CustomerPublicUsageCommand) (CustomerPublicUsage, error) {
	if cmd.CustomerID <= 0 {
		return CustomerPublicUsage{}, fmt.Errorf("customer_id required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.SaveCustomerPublicUsage(ctx, cmd)
}

func (s *Service) SaveCustomerProductRuleTemplate(ctx context.Context, cmd SaveCustomerProductRuleTemplateCommand) (CustomerProductRuleTemplate, error) {
	normalized, err := normalizeCustomerProductRuleTemplateCommand(cmd)
	if err != nil {
		return CustomerProductRuleTemplate{}, err
	}
	return s.repo.SaveCustomerProductRuleTemplate(ctx, normalized)
}

func (s *Service) SaveCustomerProductRuleOverride(ctx context.Context, cmd SaveCustomerProductRuleOverrideCommand) (CustomerProductRuleOverride, error) {
	normalized, err := normalizeCustomerProductRuleOverrideCommand(cmd)
	if err != nil {
		return CustomerProductRuleOverride{}, err
	}
	return s.repo.SaveCustomerProductRuleOverride(ctx, normalized)
}

func (s *Service) BindCustomerProductRuleTemplate(ctx context.Context, cmd CustomerProductRuleTemplateBindingCommand) (CustomerProductRuleBinding, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.CustomerID <= 0 {
		return CustomerProductRuleBinding{}, ValidationError{Message: "customer_id required"}
	}
	if cmd.TemplateID < 0 {
		return CustomerProductRuleBinding{}, ValidationError{Message: "invalid template_id"}
	}
	return s.repo.BindCustomerProductRuleTemplate(ctx, cmd)
}

func normalizeCustomerProductRuleTemplateCommand(cmd SaveCustomerProductRuleTemplateCommand) (SaveCustomerProductRuleTemplateCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.ID < 0 {
		return SaveCustomerProductRuleTemplateCommand{}, ValidationError{Message: "invalid id"}
	}
	if cmd.CustomerID < 0 {
		return SaveCustomerProductRuleTemplateCommand{}, ValidationError{Message: "invalid customer_id"}
	}
	if cmd.Name == "" {
		return SaveCustomerProductRuleTemplateCommand{}, ValidationError{Message: "name required"}
	}
	if len(cmd.Items) == 0 {
		return SaveCustomerProductRuleTemplateCommand{}, ValidationError{Message: "items required"}
	}
	for i := range cmd.Items {
		item, err := normalizeCustomerProductRuleTemplateItem(cmd.Items[i])
		if err != nil {
			return SaveCustomerProductRuleTemplateCommand{}, err
		}
		cmd.Items[i] = item
	}
	return cmd, nil
}

func normalizeCustomerProductRuleTemplateItem(item CustomerProductRuleTemplateItem) (CustomerProductRuleTemplateItem, error) {
	if item.ProductSubtypeCategoryID <= 0 {
		return CustomerProductRuleTemplateItem{}, ValidationError{Message: "product_subtype_category_id required"}
	}
	if item.GradientTemplateID < 0 || item.OperationTemplateID < 0 {
		return CustomerProductRuleTemplateItem{}, ValidationError{Message: "invalid template id"}
	}
	priceRule, err := normalizeJSONText(item.PriceListRuleJSON)
	if err != nil {
		return CustomerProductRuleTemplateItem{}, ValidationError{Message: "invalid price_list_rule_json"}
	}
	unitRule, err := normalizeJSONText(item.UnitRuleJSON)
	if err != nil {
		return CustomerProductRuleTemplateItem{}, ValidationError{Message: "invalid unit_rule_json"}
	}
	item.PriceListRuleJSON = priceRule
	item.UnitRuleJSON = unitRule
	return item, nil
}

func normalizeCustomerProductRuleOverrideCommand(cmd SaveCustomerProductRuleOverrideCommand) (SaveCustomerProductRuleOverrideCommand, error) {
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ID < 0 {
		return SaveCustomerProductRuleOverrideCommand{}, ValidationError{Message: "invalid id"}
	}
	if cmd.CustomerID <= 0 {
		return SaveCustomerProductRuleOverrideCommand{}, ValidationError{Message: "customer_id required"}
	}
	item, err := normalizeCustomerProductRuleTemplateItem(CustomerProductRuleTemplateItem{
		ProductSubtypeCategoryID: cmd.ProductSubtypeCategoryID,
		GradientTemplateID:       cmd.GradientTemplateID,
		OperationTemplateID:      cmd.OperationTemplateID,
		PriceListRuleJSON:        cmd.PriceListRuleJSON,
		UnitRuleJSON:             cmd.UnitRuleJSON,
	})
	if err != nil {
		return SaveCustomerProductRuleOverrideCommand{}, err
	}
	cmd.ProductSubtypeCategoryID = item.ProductSubtypeCategoryID
	cmd.GradientTemplateID = item.GradientTemplateID
	cmd.OperationTemplateID = item.OperationTemplateID
	cmd.PriceListRuleJSON = item.PriceListRuleJSON
	cmd.UnitRuleJSON = item.UnitRuleJSON
	return cmd, nil
}

func normalizeGradientTemplateCommand(cmd SaveGradientTemplateCommand) (SaveGradientTemplateCommand, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return cmd, fmt.Errorf("name required")
	}
	cmd.DisplayUnit = normalizeGradientDisplayUnit(cmd.DisplayUnit)
	if len(cmd.Tiers) == 0 {
		return cmd, fmt.Errorf("tiers required")
	}
	normalized := make([]GradientTemplateTier, 0, len(cmd.Tiers))
	for i, tier := range cmd.Tiers {
		tier.Label = strings.TrimSpace(tier.Label)
		if tier.Label == "" {
			return cmd, fmt.Errorf("tier label required")
		}
		tier = normalizeGradientTemplateTierWeights(cmd.DisplayUnit, tier)
		if tier.MinWeightG <= 0 {
			return cmd, fmt.Errorf("tier min_weight_g must be > 0")
		}
		if tier.MaxWeightG != nil && *tier.MaxWeightG <= tier.MinWeightG {
			return cmd, fmt.Errorf("tier max_weight_g must be greater than min_weight_g")
		}
		if tier.MarginRate < 0 {
			return cmd, fmt.Errorf("tier margin_rate must be >= 0")
		}
		if tier.Position <= 0 {
			tier.Position = i + 1
		}
		normalized = append(normalized, tier)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].Position != normalized[j].Position {
			return normalized[i].Position < normalized[j].Position
		}
		return normalized[i].MinWeightG < normalized[j].MinWeightG
	})
	cmd.Tiers = normalized
	return cmd, nil
}

func normalizeGradientDisplayUnit(unit string) string {
	switch strings.TrimSpace(unit) {
	case "kg":
		return "kg"
	case "g227":
		return "g227"
	case "g100":
		return "g100"
	case "g250":
		return "g250"
	case "lb":
		return "lb"
	default:
		return "lb"
	}
}

func normalizeJSONText(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	if !json.Valid([]byte(raw)) {
		return "", fmt.Errorf("invalid json")
	}
	return raw, nil
}

func normalizeGradientTemplateTierWeights(displayUnit string, tier GradientTemplateTier) GradientTemplateTier {
	specG := gradientDisplayUnitSpecG(displayUnit)
	if tier.MinDisplayQty != nil {
		tier.MinWeightG = roundTemplateWeightG(*tier.MinDisplayQty * float64(specG))
	}
	if tier.MaxDisplayQty != nil {
		v := roundTemplateWeightG(*tier.MaxDisplayQty * float64(specG))
		tier.MaxWeightG = &v
	}
	return tier
}

func gradientDisplayUnitSpecG(unit string) int {
	switch normalizeGradientDisplayUnit(unit) {
	case "kg":
		return 1000
	case "g227":
		return 227
	case "g100":
		return 100
	case "g250":
		return 250
	default:
		return 454
	}
}

func roundTemplateWeightG(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func BuildProductSettings(categories []ProductCategory, products []Product) ProductSettingsData {
	roots := make([]ProductCategory, 0)
	children := map[int64][]ProductCategory{}
	for _, category := range categories {
		if category.ParentID == 0 {
			roots = append(roots, category)
			continue
		}
		children[category.ParentID] = append(children[category.ParentID], category)
	}
	sortCategories(roots)
	for parentID := range children {
		sortCategories(children[parentID])
	}

	productsByCategory := map[int64][]ProductSettingsProduct{}
	allProducts := make([]ProductSettingsProduct, 0, len(products))
	for _, product := range products {
		row := productSettingsProduct(product)
		allProducts = append(allProducts, row)
		productsByCategory[product.ProductCategoryID] = append(productsByCategory[product.ProductCategoryID], row)
	}
	for categoryID := range productsByCategory {
		sortProducts(productsByCategory[categoryID])
		for i := range productsByCategory[categoryID] {
			productsByCategory[categoryID][i].Number = i + 1
		}
	}
	sortProducts(allProducts)

	out := ProductSettingsData{Products: allProducts, Categories: make([]ProductCategoryNode, 0, len(roots))}
	for i, root := range roots {
		root.Number = i + 1
		node := ProductCategoryNode{
			ProductCategory: root,
			Children:        make([]ProductCategoryNode, 0),
			Products:        productsByCategory[root.ID],
		}
		if node.Products == nil {
			node.Products = make([]ProductSettingsProduct, 0)
		}
		for j, child := range children[root.ID] {
			child.Number = j + 1
			childProducts := productsByCategory[child.ID]
			if childProducts == nil {
				childProducts = make([]ProductSettingsProduct, 0)
			}
			childNode := ProductCategoryNode{
				ProductCategory: child,
				Children:        make([]ProductCategoryNode, 0),
				Products:        childProducts,
			}
			node.Children = append(node.Children, childNode)
		}
		out.Categories = append(out.Categories, node)
	}
	return out
}

func productSettingsProduct(p Product) ProductSettingsProduct {
	productKind, dripBagGrams, dripBoxBagCount, salesUnits, _ := normalizeProductKindSettings(p.ProductKind, p.DripBagGrams, p.DripBoxBagCount)
	if !catalogdomain.ProductKindRequiresRoast(productKind) {
		p.RoastLevel = ""
		p.YieldRate = 0
	}
	return ProductSettingsProduct{
		ID:                          p.ID,
		Name:                        p.Name,
		Remark:                      p.Remark,
		GreenBeanType:               p.GreenBeanType,
		GreenBeanBomProductID:       p.GreenBeanBomProductID,
		RoastLevel:                  p.RoastLevel,
		ProductKind:                 productKind,
		DripBagGrams:                dripBagGrams,
		DripBoxBagCount:             dripBoxBagCount,
		AllowFulfillmentOrder:       p.AllowFulfillmentOrder,
		AllowMallOrder:              p.AllowMallOrder,
		SalesUnits:                  salesUnits,
		DefaultPrice:                p.DefaultPrice,
		RetailPrice100G:             p.RetailPrice100G,
		RetailPrice200G:             p.RetailPrice200G,
		RetailPrice227G:             p.RetailPrice227G,
		RetailPrice250G:             p.RetailPrice250G,
		YieldRate:                   p.YieldRate,
		ProductCategoryID:           p.ProductCategoryID,
		ProductCategoryPosition:     p.ProductCategoryPosition,
		CustomerID:                  p.CustomerID,
		BaseProductID:               p.BaseProductID,
		Visibility:                  productVisibility(p.Visibility, p.CustomerID),
		CustomType:                  p.CustomType,
		MarginRateOverride:          p.MarginRateOverride,
		GradientTemplateIDOverride:  p.GradientTemplateIDOverride,
		OperationTemplateIDOverride: p.OperationTemplateIDOverride,
		UnitRuleOverrideJSON:        productJSONOrDefault(p.UnitRuleOverrideJSON),
		BomItemCount:                p.BomItemCount,
		BomStatus:                   productBomStatus(p.BomStatus, p.BomItemCount),
		OrderUsageCount:             p.OrderUsageCount,
	}
}

func productJSONOrDefault(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	return raw
}

func normalizeProductKindSettings(productKind string, dripBagGrams float64, dripBoxBagCount int) (string, float64, int, []string, error) {
	productKind = catalogdomain.NormalizeProductKind(productKind)
	if productKind != catalogdomain.ProductKindDripBag {
		return productKind, dripBagGrams, dripBoxBagCount, []string{}, nil
	}
	if dripBagGrams == 0 {
		dripBagGrams = 10
	}
	if dripBoxBagCount == 0 {
		dripBoxBagCount = 10
	}
	if dripBagGrams <= 0 {
		return productKind, dripBagGrams, dripBoxBagCount, nil, ValidationError{Message: "drip_bag_grams must be > 0"}
	}
	if dripBoxBagCount <= 0 {
		return productKind, dripBagGrams, dripBoxBagCount, nil, ValidationError{Message: "drip_box_bag_count must be > 0"}
	}
	return productKind, dripBagGrams, dripBoxBagCount, []string{"bag", "box"}, nil
}

func productBomStatus(status string, itemCount int) string {
	status = strings.TrimSpace(status)
	if status != "" {
		return status
	}
	if itemCount > 0 {
		return "active"
	}
	return "missing"
}

func productVisibility(visibility string, customerID int64) string {
	visibility = strings.TrimSpace(visibility)
	if visibility != "" {
		return visibility
	}
	if customerID > 0 {
		return "customer_only"
	}
	return "public"
}

func sortCategories(categories []ProductCategory) {
	sort.SliceStable(categories, func(i, j int) bool {
		if categories[i].Position != categories[j].Position {
			return categories[i].Position < categories[j].Position
		}
		if categories[i].Name != categories[j].Name {
			return categories[i].Name < categories[j].Name
		}
		return categories[i].ID < categories[j].ID
	})
}

func sortProducts(products []ProductSettingsProduct) {
	sort.SliceStable(products, func(i, j int) bool {
		if products[i].ProductCategoryPosition != products[j].ProductCategoryPosition {
			return products[i].ProductCategoryPosition < products[j].ProductCategoryPosition
		}
		if products[i].Name != products[j].Name {
			return products[i].Name < products[j].Name
		}
		return products[i].ID < products[j].ID
	})
}
