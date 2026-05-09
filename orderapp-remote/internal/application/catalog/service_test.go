package catalog

import (
	"context"
	"testing"
)

type fakeRepo struct {
	replace ReplacePriceTiersCommand
	update  UpdateProductBasicsCommand
	create  CreateProductCommand
}

func (r *fakeRepo) ListProducts(ctx context.Context) ([]Product, error) {
	return []Product{{ID: 1, Name: "A"}}, nil
}

func (r *fakeRepo) GetProduct(ctx context.Context, id int64) (*Product, error) {
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

func (r *fakeRepo) CreateProduct(ctx context.Context, cmd CreateProductCommand) (Product, error) {
	r.create = cmd
	return Product{ID: 11, Name: cmd.Name, RoastLevel: cmd.RoastLevel, YieldRate: cmd.YieldRate, Visibility: "public"}, nil
}

func (r *fakeRepo) ListProductCategories(ctx context.Context) ([]ProductCategory, error) {
	return []ProductCategory{{ID: 1, Name: "咖啡豆", Level: 1, Position: 1}}, nil
}

func (r *fakeRepo) SaveProductCategory(ctx context.Context, cmd SaveProductCategoryCommand) (ProductCategory, error) {
	return ProductCategory{ID: 2, Name: cmd.Name, ParentID: cmd.ParentID, Position: cmd.Position}, nil
}

func (r *fakeRepo) MoveProductCategory(ctx context.Context, cmd MoveProductCategoryCommand) error {
	return nil
}

func (r *fakeRepo) DeleteProductCategory(ctx context.Context, cmd DeleteProductCategoryCommand) error {
	return nil
}

func (r *fakeRepo) AssignProductCategory(ctx context.Context, cmd AssignProductCategoryCommand) error {
	return nil
}

func (r *fakeRepo) CreateCustomProduct(ctx context.Context, cmd CreateCustomProductCommand) (Product, error) {
	return Product{ID: 10, Name: cmd.Name, CustomerID: cmd.CustomerID, BaseProductID: cmd.BaseProductID, Visibility: "customer_only", CustomType: cmd.CustomType}, nil
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
