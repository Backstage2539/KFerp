package catalog

import (
	"context"
	"sort"
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
	RoastLevel              string
	DefaultPrice            float64
	RetailPrice100G         float64
	RetailPrice200G         float64
	RetailPrice227G         float64
	RetailPrice250G         float64
	YieldRate               float64
	ProductCategoryID       int64
	ProductCategoryPosition int
	Tiers                   []PriceTier
}

type ProductCategory struct {
	ID       int64  `json:"id"`
	ParentID int64  `json:"parent_id"`
	Name     string `json:"name"`
	Level    int    `json:"level"`
	Position int    `json:"position"`
	Number   int    `json:"number"`
}

type ProductSettingsProduct struct {
	ID                      int64   `json:"id"`
	Name                    string  `json:"name"`
	RoastLevel              string  `json:"roast_level"`
	DefaultPrice            float64 `json:"default_price"`
	RetailPrice100G         float64 `json:"retail_price_100g"`
	RetailPrice200G         float64 `json:"retail_price_200g"`
	RetailPrice227G         float64 `json:"retail_price_227g"`
	RetailPrice250G         float64 `json:"retail_price_250g"`
	YieldRate               float64 `json:"yield_rate"`
	ProductCategoryID       int64   `json:"product_category_id"`
	ProductCategoryPosition int     `json:"product_category_position"`
	Number                  int     `json:"number"`
}

type ProductCategoryNode struct {
	ProductCategory
	Children []ProductCategoryNode    `json:"children"`
	Products []ProductSettingsProduct `json:"products"`
}

type ProductSettingsData struct {
	Categories []ProductCategoryNode    `json:"categories"`
	Products   []ProductSettingsProduct `json:"products"`
}

type ReplacePriceTiersCommand struct {
	Actor           string
	ProductID       int64
	RoastLevel      string
	RetailPrice100G float64
	RetailPrice200G float64
	RetailPrice227G float64
	RetailPrice250G float64
	Tiers           []PriceTier
}

type UpdateProductBasicsCommand struct {
	Actor           string
	ProductID       int64
	RoastLevel      string
	RetailPrice100G float64
	RetailPrice200G float64
	RetailPrice227G float64
	RetailPrice250G float64
	YieldRate       float64
}

type SaveProductCategoryCommand struct {
	Actor    string
	ID       int64
	ParentID int64
	Name     string
	Position int
}

type MoveProductCategoryCommand struct {
	Actor    string
	ID       int64
	ParentID int64
	Position int
}

type AssignProductCategoryCommand struct {
	Actor      string
	ProductID  int64
	CategoryID int64
	Position   int
}

type Repository interface {
	ListProducts(ctx context.Context) ([]Product, error)
	GetProduct(ctx context.Context, id int64) (*Product, error)
	ReplacePriceTiers(ctx context.Context, cmd ReplacePriceTiersCommand) error
	UpdateProductBasics(ctx context.Context, cmd UpdateProductBasicsCommand) error
	ListProductCategories(ctx context.Context) ([]ProductCategory, error)
	SaveProductCategory(ctx context.Context, cmd SaveProductCategoryCommand) (ProductCategory, error)
	MoveProductCategory(ctx context.Context, cmd MoveProductCategoryCommand) error
	AssignProductCategory(ctx context.Context, cmd AssignProductCategoryCommand) error
}

type Service struct {
	repo Repository
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
	return s.repo.ReplacePriceTiers(ctx, cmd)
}

func (s *Service) UpdateProductBasics(ctx context.Context, cmd UpdateProductBasicsCommand) error {
	return s.repo.UpdateProductBasics(ctx, cmd)
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
	return BuildProductSettings(categories, products), nil
}

func (s *Service) SaveProductCategory(ctx context.Context, cmd SaveProductCategoryCommand) (ProductCategory, error) {
	return s.repo.SaveProductCategory(ctx, cmd)
}

func (s *Service) MoveProductCategory(ctx context.Context, cmd MoveProductCategoryCommand) error {
	return s.repo.MoveProductCategory(ctx, cmd)
}

func (s *Service) AssignProductCategory(ctx context.Context, cmd AssignProductCategoryCommand) error {
	return s.repo.AssignProductCategory(ctx, cmd)
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
	return ProductSettingsProduct{
		ID:                      p.ID,
		Name:                    p.Name,
		RoastLevel:              p.RoastLevel,
		DefaultPrice:            p.DefaultPrice,
		RetailPrice100G:         p.RetailPrice100G,
		RetailPrice200G:         p.RetailPrice200G,
		RetailPrice227G:         p.RetailPrice227G,
		RetailPrice250G:         p.RetailPrice250G,
		YieldRate:               p.YieldRate,
		ProductCategoryID:       p.ProductCategoryID,
		ProductCategoryPosition: p.ProductCategoryPosition,
	}
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
