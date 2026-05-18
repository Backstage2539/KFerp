package catalog

import (
	"context"
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
	ID                      int64
	Name                    string
	ProductKind             string
	GreenBeanType           string
	GreenBeanBomProductID   int64
	RoastLevel              string
	DripBagGrams            float64
	DripBoxBagCount         int
	AllowFulfillmentOrder   bool
	AllowMallOrder          bool
	SalesUnits              []string
	DefaultPrice            float64
	RetailPrice100G         float64
	RetailPrice200G         float64
	RetailPrice227G         float64
	RetailPrice250G         float64
	YieldRate               float64
	ProductCategoryID       int64
	ProductCategoryPosition int
	CustomerID              int64
	BaseProductID           int64
	Visibility              string
	CustomType              string
	MarginRateOverride      *float64
	BomItemCount            int
	BomStatus               string
	Tiers                   []PriceTier
}

type ProductCategory struct {
	ID                 int64  `json:"id"`
	ParentID           int64  `json:"parent_id"`
	CustomerID         int64  `json:"customer_id"`
	Name               string `json:"name"`
	Level              int    `json:"level"`
	Position           int    `json:"position"`
	Number             int    `json:"number"`
	GradientTemplateID int64  `json:"gradient_template_id"`
}

type ProductSettingsProduct struct {
	ID                      int64    `json:"id"`
	Name                    string   `json:"name"`
	ProductKind             string   `json:"product_kind"`
	GreenBeanType           string   `json:"green_bean_type"`
	GreenBeanBomProductID   int64    `json:"green_bean_bom_product_id"`
	RoastLevel              string   `json:"roast_level"`
	DripBagGrams            float64  `json:"drip_bag_grams"`
	DripBoxBagCount         int      `json:"drip_box_bag_count"`
	AllowFulfillmentOrder   bool     `json:"allow_fulfillment_order"`
	AllowMallOrder          bool     `json:"allow_mall_order"`
	SalesUnits              []string `json:"sales_units"`
	DefaultPrice            float64  `json:"default_price"`
	RetailPrice100G         float64  `json:"retail_price_100g"`
	RetailPrice200G         float64  `json:"retail_price_200g"`
	RetailPrice227G         float64  `json:"retail_price_227g"`
	RetailPrice250G         float64  `json:"retail_price_250g"`
	YieldRate               float64  `json:"yield_rate"`
	ProductCategoryID       int64    `json:"product_category_id"`
	ProductCategoryPosition int      `json:"product_category_position"`
	CustomerID              int64    `json:"customer_id"`
	BaseProductID           int64    `json:"base_product_id"`
	Visibility              string   `json:"visibility"`
	CustomType              string   `json:"custom_type"`
	MarginRateOverride      *float64 `json:"margin_rate_override"`
	BomItemCount            int      `json:"bom_item_count"`
	BomStatus               string   `json:"bom_status"`
	Number                  int      `json:"number"`
}

type ProductCategoryNode struct {
	ProductCategory
	Children []ProductCategoryNode    `json:"children"`
	Products []ProductSettingsProduct `json:"products"`
}

type ProductSettingsData struct {
	Categories        []ProductCategoryNode    `json:"categories"`
	Products          []ProductSettingsProduct `json:"products"`
	GradientTemplates []GradientTemplate       `json:"gradient_templates"`
}

type GradientTemplate struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	DisplayUnit string                 `json:"display_unit"`
	Active      bool                   `json:"active"`
	Tiers       []GradientTemplateTier `json:"tiers"`
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
	Actor                 string
	ProductID             int64
	ProductKind           string
	GreenBeanType         string
	GreenBeanBomProductID int64
	DefaultPrice          float64
	RoastLevel            string
	DripBagGrams          float64
	DripBoxBagCount       int
	AllowFulfillmentOrder bool
	AllowMallOrder        bool
	SalesUnits            []string
	RetailPrice100G       float64
	RetailPrice200G       float64
	RetailPrice227G       float64
	RetailPrice250G       float64
	YieldRate             float64
	MarginRateOverride    *float64
}

type CreateProductCommand struct {
	Actor                    string
	Name                     string
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
	Actor          string
	CustomerID     int64
	BaseProductID  int64
	Name           string
	RoastLevel     string
	CustomType     string
	CopyBOM        bool
	CopyPriceTiers bool
}

type SaveProductCategoryCommand struct {
	Actor      string
	ID         int64
	ParentID   int64
	CustomerID int64
	Name       string
	Position   int
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
	Actor      string
	ProductID  int64
	CategoryID int64
	Position   int
}

type SaveGradientTemplateCommand struct {
	Actor       string
	ID          int64
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
	AssignProductCategory(ctx context.Context, cmd AssignProductCategoryCommand) error
	CreateCustomProduct(ctx context.Context, cmd CreateCustomProductCommand) (Product, error)
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
	} else {
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
	data := BuildProductSettings(categories, products)
	data.GradientTemplates = templates
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
	return s.repo.SaveProductCategory(ctx, cmd)
}

func (s *Service) MoveProductCategory(ctx context.Context, cmd MoveProductCategoryCommand) error {
	return s.repo.MoveProductCategory(ctx, cmd)
}

func (s *Service) DeleteProductCategory(ctx context.Context, cmd DeleteProductCategoryCommand) error {
	return s.repo.DeleteProductCategory(ctx, cmd)
}

func (s *Service) AssignProductCategory(ctx context.Context, cmd AssignProductCategoryCommand) error {
	return s.repo.AssignProductCategory(ctx, cmd)
}

func (s *Service) CreateCustomProduct(ctx context.Context, cmd CreateCustomProductCommand) (Product, error) {
	if cmd.CustomerID <= 0 {
		return Product{}, fmt.Errorf("customer_id required")
	}
	if cmd.BaseProductID <= 0 {
		return Product{}, fmt.Errorf("base_product_id required")
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return Product{}, fmt.Errorf("name required")
	}
	cmd.RoastLevel = catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	if cmd.RoastLevel == "" {
		return Product{}, fmt.Errorf("invalid roast_level")
	}
	cmd.CustomType = strings.TrimSpace(cmd.CustomType)
	if cmd.CustomType != "custom_blend" && cmd.CustomType != "custom_roast" && cmd.CustomType != "public_sku_alias" {
		return Product{}, fmt.Errorf("invalid custom_type")
	}
	return s.repo.CreateCustomProduct(ctx, cmd)
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
	return ProductSettingsProduct{
		ID:                      p.ID,
		Name:                    p.Name,
		GreenBeanType:           p.GreenBeanType,
		GreenBeanBomProductID:   p.GreenBeanBomProductID,
		RoastLevel:              p.RoastLevel,
		ProductKind:             productKind,
		DripBagGrams:            dripBagGrams,
		DripBoxBagCount:         dripBoxBagCount,
		AllowFulfillmentOrder:   p.AllowFulfillmentOrder,
		AllowMallOrder:          p.AllowMallOrder,
		SalesUnits:              salesUnits,
		DefaultPrice:            p.DefaultPrice,
		RetailPrice100G:         p.RetailPrice100G,
		RetailPrice200G:         p.RetailPrice200G,
		RetailPrice227G:         p.RetailPrice227G,
		RetailPrice250G:         p.RetailPrice250G,
		YieldRate:               p.YieldRate,
		ProductCategoryID:       p.ProductCategoryID,
		ProductCategoryPosition: p.ProductCategoryPosition,
		CustomerID:              p.CustomerID,
		BaseProductID:           p.BaseProductID,
		Visibility:              productVisibility(p.Visibility, p.CustomerID),
		CustomType:              p.CustomType,
		MarginRateOverride:      p.MarginRateOverride,
		BomItemCount:            p.BomItemCount,
		BomStatus:               productBomStatus(p.BomStatus, p.BomItemCount),
	}
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
