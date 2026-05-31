package bom

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRepo struct {
	savedItem     SaveItemCommand
	deletedID     int64
	deactivatedID int64
	activated     int64
	versionFor    int64
	listRows      []ListItem
	productRows   []Option
}

func (r *fakeRepo) List(ctx context.Context) ([]ListItem, error) { return r.listRows, nil }
func (r *fakeRepo) Detail(ctx context.Context, productID int64) (Detail, error) {
	return Detail{}, nil
}
func (r *fakeRepo) Products(ctx context.Context) ([]Option, error)  { return r.productRows, nil }
func (r *fakeRepo) Materials(ctx context.Context) ([]Option, error) { return nil, nil }
func (r *fakeRepo) BagSpecMappings(ctx context.Context) ([]BagSpecMapping, error) {
	return nil, nil
}
func (r *fakeRepo) SyncProductYield(ctx context.Context, cmd SyncProductYieldCommand) error {
	return nil
}
func (r *fakeRepo) DeactivateBom(ctx context.Context, cmd DeactivateBomCommand) error {
	r.deactivatedID = cmd.ProductID
	return nil
}
func (r *fakeRepo) SaveItem(ctx context.Context, cmd SaveItemCommand) error {
	r.savedItem = cmd
	return nil
}
func (r *fakeRepo) DeleteItem(ctx context.Context, cmd DeleteItemCommand) error {
	r.deletedID = cmd.ID
	return nil
}
func (r *fakeRepo) SaveBagSpecMapping(ctx context.Context, cmd SaveBagSpecMappingCommand) error {
	return nil
}
func (r *fakeRepo) DeleteBagSpecMapping(ctx context.Context, cmd DeleteBagSpecMappingCommand) error {
	return nil
}
func (r *fakeRepo) ListVersions(ctx context.Context, productID int64) ([]Version, error) {
	r.versionFor = productID
	return nil, nil
}
func (r *fakeRepo) CreateVersion(ctx context.Context, cmd CreateVersionCommand) (Version, error) {
	return Version{ID: 9, ProductID: cmd.ProductID, VersionNo: "V001"}, nil
}
func (r *fakeRepo) ActivateVersion(ctx context.Context, cmd ActivateVersionCommand) error {
	r.activated = cmd.VersionID
	return nil
}
func (r *fakeRepo) SetBomSource(context.Context, SetBomSourceCommand) (Detail, error) {
	return Detail{}, nil
}
func (r *fakeRepo) DeriveOwned(context.Context, DeriveOwnedCommand) (Detail, error) {
	return Detail{}, nil
}
func (r *fakeRepo) ListProductionBomGroups(context.Context, bool) ([]ProductionBomGroup, error) {
	return nil, nil
}
func (r *fakeRepo) CreateProductionBomGroup(context.Context, CreateProductionBomGroupCommand) (ProductionBomGroup, error) {
	return ProductionBomGroup{}, nil
}
func (r *fakeRepo) UpdateProductionBomGroup(context.Context, UpdateProductionBomGroupCommand) (ProductionBomGroup, error) {
	return ProductionBomGroup{}, nil
}
func (r *fakeRepo) DisableProductionBomGroup(context.Context, DisableProductionBomGroupCommand) error {
	return nil
}
func (r *fakeRepo) ListProductionBoms(context.Context) ([]ProductionBomSummary, error) {
	return nil, nil
}
func (r *fakeRepo) GetProductionBomDetail(context.Context, int64) (ProductionBomDetail, error) {
	return ProductionBomDetail{}, nil
}
func (r *fakeRepo) CreateProductionBom(context.Context, CreateProductionBomCommand) (ProductionBomSummary, error) {
	return ProductionBomSummary{}, nil
}
func (r *fakeRepo) UpdateProductionBom(context.Context, UpdateProductionBomCommand) (ProductionBomSummary, error) {
	return ProductionBomSummary{}, nil
}
func (r *fakeRepo) CopyProductionBom(context.Context, CopyProductionBomCommand) (ProductionBomSummary, error) {
	return ProductionBomSummary{}, nil
}
func (r *fakeRepo) CreateProductionBomVersion(context.Context, CreateProductionBomVersionCommand) (ProductionBomVersion, error) {
	return ProductionBomVersion{}, nil
}
func (r *fakeRepo) UpdateProductionBomVersionDraft(context.Context, UpdateProductionBomVersionDraftCommand) (ProductionBomVersion, error) {
	return ProductionBomVersion{}, nil
}
func (r *fakeRepo) PublishProductionBomVersion(context.Context, PublishProductionBomVersionCommand) error {
	return nil
}
func (r *fakeRepo) BindProductProductionBom(context.Context, BindProductProductionBomCommand) (ProductProductionBomBinding, error) {
	return ProductProductionBomBinding{}, nil
}

func TestServiceValidatesSaveItem(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	cases := []SaveItemCommand{
		{ProductID: 0, MaterialID: 1, RatioPct: 10},
		{ProductID: 1, MaterialID: 0, RatioPct: 10},
		{ProductID: 1, MaterialID: 1, RatioPct: 0},
		{ProductID: 1, MaterialID: 1, RatioPct: 100.01},
	}
	for _, tc := range cases {
		if err := svc.SaveItem(ctx, tc); err == nil {
			t.Fatalf("SaveItem(%+v) succeeded, want validation error", tc)
		}
	}

	if err := svc.SaveItem(ctx, SaveItemCommand{ProductID: 1, MaterialID: 2, RatioPct: 25}); err != nil {
		t.Fatalf("SaveItem valid command: %v", err)
	}
	if repo.savedItem.MaterialID != 2 || repo.savedItem.RatioPct != 25 {
		t.Fatalf("savedItem = %+v, want material 2 ratio 25", repo.savedItem)
	}
}

func TestServiceHidesGreenBeanProductsFromBomMaintenance(t *testing.T) {
	repo := &fakeRepo{
		listRows: []ListItem{
			{ProductID: 1, Product: "岩师傅熟豆", CustomerID: 152, ProductKind: "roasted_bean"},
			{ProductID: 2, Product: "兰卡拼配生豆", CustomerID: 152, ProductKind: "green_bean"},
			{ProductID: 3, Product: "岩师傅挂耳", CustomerID: 152, ProductKind: "drip_bag"},
		},
		productRows: []Option{
			{ID: 1, Name: "岩师傅熟豆", CustomerID: 152, ProductKind: "roasted_bean"},
			{ID: 2, Name: "兰卡拼配生豆", CustomerID: 152, ProductKind: "green_bean"},
			{ID: 3, Name: "岩师傅挂耳", CustomerID: 152, ProductKind: "drip_bag"},
		},
	}
	svc := NewService(repo)

	listRows, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listRows) != 2 || listRows[0].ProductID != 1 || listRows[1].ProductID != 3 {
		t.Fatalf("List rows = %+v, want only non-green products", listRows)
	}

	productRows, err := svc.Products(context.Background())
	if err != nil {
		t.Fatalf("Products: %v", err)
	}
	if len(productRows) != 2 || productRows[0].ID != 1 || productRows[1].ID != 3 {
		t.Fatalf("Product rows = %+v, want only non-green products", productRows)
	}
}

func TestSaveFinishedProductComponentRequiresComponentProduct(t *testing.T) {
	svc := NewService(&fakeRepo{})
	err := svc.SaveItem(context.Background(), SaveItemCommand{
		ProductID:     1,
		ComponentType: "finished_product",
		ConsumeUnit:   "g_per_bag",
		QtyPerUnit:    10,
	})
	if err == nil || !strings.Contains(err.Error(), "component_product_id required") {
		t.Fatalf("expected component_product_id error, got %v", err)
	}
}

func TestSaveMaterialComponentKeepsLegacyMaterialValidation(t *testing.T) {
	svc := NewService(&fakeRepo{})
	err := svc.SaveItem(context.Background(), SaveItemCommand{
		ProductID:     1,
		ComponentType: "material",
		MaterialID:    2,
		ConsumeUnit:   "unit_per_bag",
		QtyPerUnit:    1,
	})
	if err != nil {
		t.Fatalf("SaveItem: %v", err)
	}
}

func TestSaveFinishedProductComponentRejectsRatioPct(t *testing.T) {
	svc := NewService(&fakeRepo{})
	err := svc.SaveItem(context.Background(), SaveItemCommand{
		ProductID:          1,
		ComponentType:      "finished_product",
		ComponentProductID: 2,
		ConsumeUnit:        "ratio_pct",
		RatioPct:           10,
	})
	if err == nil || !strings.Contains(err.Error(), "finished_product consume_unit must not be ratio_pct") {
		t.Fatalf("expected finished product ratio error, got %v", err)
	}
}

func TestDeleteItemRequiresProductID(t *testing.T) {
	svc := NewService(&fakeRepo{})
	err := svc.DeleteItem(context.Background(), DeleteItemCommand{ID: 7})
	if err == nil || !strings.Contains(err.Error(), "product_id required") {
		t.Fatalf("expected product_id error, got %v", err)
	}
}

func TestServicePropagatesRepositoryErrors(t *testing.T) {
	wantErr := errors.New("repo failed")
	svc := NewService(errorRepo{err: wantErr})
	if _, err := svc.List(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("List error = %v, want %v", err, wantErr)
	}
}

func TestServiceValidatesBOMVersions(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	if _, err := svc.ListVersions(ctx, 0); err == nil {
		t.Fatal("ListVersions should require product id")
	}
	if _, err := svc.CreateVersion(ctx, CreateVersionCommand{}); err == nil {
		t.Fatal("CreateVersion should require product id")
	}
	if err := svc.ActivateVersion(ctx, ActivateVersionCommand{}); err == nil {
		t.Fatal("ActivateVersion should require version id")
	}
	if err := svc.ActivateVersion(ctx, ActivateVersionCommand{VersionID: 7}); err != nil {
		t.Fatalf("ActivateVersion valid command: %v", err)
	}
	if repo.activated != 7 {
		t.Fatalf("activated = %d, want 7", repo.activated)
	}
}

func TestServiceValidatesDeactivateBom(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	if err := svc.DeactivateBom(ctx, DeactivateBomCommand{}); err == nil {
		t.Fatal("DeactivateBom should require product id")
	}
	if err := svc.DeactivateBom(ctx, DeactivateBomCommand{ProductID: 7}); err != nil {
		t.Fatalf("DeactivateBom valid command: %v", err)
	}
	if repo.deactivatedID != 7 {
		t.Fatalf("deactivatedID = %d, want 7", repo.deactivatedID)
	}
}

type errorRepo struct {
	err error
}

func (r errorRepo) List(ctx context.Context) ([]ListItem, error) { return nil, r.err }
func (r errorRepo) Detail(ctx context.Context, productID int64) (Detail, error) {
	return Detail{}, r.err
}
func (r errorRepo) Products(ctx context.Context) ([]Option, error)  { return nil, r.err }
func (r errorRepo) Materials(ctx context.Context) ([]Option, error) { return nil, r.err }
func (r errorRepo) BagSpecMappings(ctx context.Context) ([]BagSpecMapping, error) {
	return nil, r.err
}
func (r errorRepo) SyncProductYield(ctx context.Context, cmd SyncProductYieldCommand) error {
	return r.err
}
func (r errorRepo) DeactivateBom(ctx context.Context, cmd DeactivateBomCommand) error {
	return r.err
}
func (r errorRepo) SaveItem(ctx context.Context, cmd SaveItemCommand) error {
	return r.err
}
func (r errorRepo) DeleteItem(ctx context.Context, cmd DeleteItemCommand) error {
	return r.err
}
func (r errorRepo) SaveBagSpecMapping(ctx context.Context, cmd SaveBagSpecMappingCommand) error {
	return r.err
}
func (r errorRepo) DeleteBagSpecMapping(ctx context.Context, cmd DeleteBagSpecMappingCommand) error {
	return r.err
}
func (r errorRepo) ListVersions(ctx context.Context, productID int64) ([]Version, error) {
	return nil, r.err
}
func (r errorRepo) CreateVersion(ctx context.Context, cmd CreateVersionCommand) (Version, error) {
	return Version{}, r.err
}
func (r errorRepo) ActivateVersion(ctx context.Context, cmd ActivateVersionCommand) error {
	return r.err
}
func (r errorRepo) SetBomSource(context.Context, SetBomSourceCommand) (Detail, error) {
	return Detail{}, r.err
}
func (r errorRepo) DeriveOwned(context.Context, DeriveOwnedCommand) (Detail, error) {
	return Detail{}, r.err
}
func (r errorRepo) ListProductionBomGroups(context.Context, bool) ([]ProductionBomGroup, error) {
	return nil, r.err
}
func (r errorRepo) CreateProductionBomGroup(context.Context, CreateProductionBomGroupCommand) (ProductionBomGroup, error) {
	return ProductionBomGroup{}, r.err
}
func (r errorRepo) UpdateProductionBomGroup(context.Context, UpdateProductionBomGroupCommand) (ProductionBomGroup, error) {
	return ProductionBomGroup{}, r.err
}
func (r errorRepo) DisableProductionBomGroup(context.Context, DisableProductionBomGroupCommand) error {
	return r.err
}
func (r errorRepo) ListProductionBoms(context.Context) ([]ProductionBomSummary, error) {
	return nil, r.err
}
func (r errorRepo) GetProductionBomDetail(context.Context, int64) (ProductionBomDetail, error) {
	return ProductionBomDetail{}, r.err
}
func (r errorRepo) CreateProductionBom(context.Context, CreateProductionBomCommand) (ProductionBomSummary, error) {
	return ProductionBomSummary{}, r.err
}
func (r errorRepo) UpdateProductionBom(context.Context, UpdateProductionBomCommand) (ProductionBomSummary, error) {
	return ProductionBomSummary{}, r.err
}
func (r errorRepo) CopyProductionBom(context.Context, CopyProductionBomCommand) (ProductionBomSummary, error) {
	return ProductionBomSummary{}, r.err
}
func (r errorRepo) CreateProductionBomVersion(context.Context, CreateProductionBomVersionCommand) (ProductionBomVersion, error) {
	return ProductionBomVersion{}, r.err
}
func (r errorRepo) UpdateProductionBomVersionDraft(context.Context, UpdateProductionBomVersionDraftCommand) (ProductionBomVersion, error) {
	return ProductionBomVersion{}, r.err
}
func (r errorRepo) PublishProductionBomVersion(context.Context, PublishProductionBomVersionCommand) error {
	return r.err
}
func (r errorRepo) BindProductProductionBom(context.Context, BindProductProductionBomCommand) (ProductProductionBomBinding, error) {
	return ProductProductionBomBinding{}, r.err
}
