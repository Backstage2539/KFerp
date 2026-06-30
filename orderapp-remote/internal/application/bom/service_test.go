package bom

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRepo struct {
	savedItem                     SaveItemCommand
	deletedID                     int64
	deactivatedID                 int64
	activated                     int64
	versionFor                    int64
	listRows                      []ListItem
	productRows                   []Option
	createdProductionBomCommand   CreateProductionBomCommand
	updatedProductionBomCommand   UpdateProductionBomCommand
	updatedProductionDraftCommand UpdateProductionBomVersionDraftCommand
	publishValidationErr          error
	usageProductID                int64
	usageRows                     []ProductionBomUsedByBom
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
func (r *fakeRepo) DeleteProductionBomGroup(context.Context, DeleteProductionBomGroupCommand) error {
	return nil
}
func (r *fakeRepo) MoveProductionBomGroup(context.Context, MoveProductionBomGroupCommand) error {
	return nil
}
func (r *fakeRepo) CreateProductionBomGroupCategory(context.Context, CreateProductionBomGroupCategoryCommand) (ProductionBomGroupCategory, error) {
	return ProductionBomGroupCategory{}, nil
}
func (r *fakeRepo) UpdateProductionBomGroupCategory(context.Context, UpdateProductionBomGroupCategoryCommand) (ProductionBomGroupCategory, error) {
	return ProductionBomGroupCategory{}, nil
}
func (r *fakeRepo) DeleteProductionBomGroupCategory(context.Context, DeleteProductionBomGroupCategoryCommand) error {
	return nil
}
func (r *fakeRepo) ListProductionBoms(context.Context) ([]ProductionBomSummary, error) {
	return nil, nil
}
func (r *fakeRepo) GetProductionBomDetail(context.Context, int64, int64) (ProductionBomDetail, error) {
	return ProductionBomDetail{}, nil
}
func (r *fakeRepo) ListProductionBomUsageByProduct(_ context.Context, productID int64) ([]ProductionBomUsedByBom, error) {
	r.usageProductID = productID
	return r.usageRows, nil
}
func (r *fakeRepo) CreateProductionBom(_ context.Context, cmd CreateProductionBomCommand) (ProductionBomSummary, error) {
	r.createdProductionBomCommand = cmd
	return ProductionBomSummary{ID: 11, Name: cmd.Name, OutputProductID: cmd.OutputProductID}, nil
}
func (r *fakeRepo) UpdateProductionBom(_ context.Context, cmd UpdateProductionBomCommand) (ProductionBomSummary, error) {
	r.updatedProductionBomCommand = cmd
	return ProductionBomSummary{ID: cmd.ID, Name: cmd.Name, OutputProductID: cmd.OutputProductID}, nil
}
func (r *fakeRepo) CopyProductionBom(context.Context, CopyProductionBomCommand) (ProductionBomSummary, error) {
	return ProductionBomSummary{}, nil
}
func (r *fakeRepo) CreateProductionBomVersion(context.Context, CreateProductionBomVersionCommand) (ProductionBomVersion, error) {
	return ProductionBomVersion{}, nil
}
func (r *fakeRepo) UpdateProductionBomVersionDraft(_ context.Context, cmd UpdateProductionBomVersionDraftCommand) (ProductionBomVersion, error) {
	r.updatedProductionDraftCommand = cmd
	return ProductionBomVersion{ID: cmd.VersionID, Status: "draft", OutputQty: cmd.OutputQty, OutputUnit: cmd.OutputUnit}, nil
}
func (r *fakeRepo) ValidateProductionBomVersionForPublish(context.Context, PublishProductionBomVersionCommand) error {
	return r.publishValidationErr
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

func TestCreateProductionBomRequiresOutputProduct(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	if _, err := svc.CreateProductionBom(ctx, CreateProductionBomCommand{Name: "10条速溶盒装"}); err == nil || !strings.Contains(err.Error(), "output_product_id required") {
		t.Fatalf("expected output_product_id validation error, got %v", err)
	}
	row, err := svc.CreateProductionBom(ctx, CreateProductionBomCommand{Name: "10条速溶盒装", OutputProductID: 88, OutputQty: 1, OutputUnit: "盒"})
	if err != nil {
		t.Fatalf("CreateProductionBom valid command: %v", err)
	}
	if row.OutputProductID != 88 || repo.createdProductionBomCommand.OutputProductID != 88 {
		t.Fatalf("output product not propagated, row=%+v cmd=%+v", row, repo.createdProductionBomCommand)
	}
	if repo.createdProductionBomCommand.OutputQty != 1 || repo.createdProductionBomCommand.OutputUnit != "盒" {
		t.Fatalf("output basis not propagated: %+v", repo.createdProductionBomCommand)
	}
}

func TestCreateProductionBomDerivesOutputUnitFromProductInventoryUnit(t *testing.T) {
	repo := &fakeRepo{productRows: []Option{{ID: 88, Name: "10条速溶盒装", ProductKind: "instant_coffee", InventoryUnit: "盒"}}}
	svc := NewService(repo)
	ctx := context.Background()

	_, err := svc.CreateProductionBom(ctx, CreateProductionBomCommand{Name: "10条速溶盒装", OutputProductID: 88, OutputQty: 1, OutputUnit: "kg"})
	if err != nil {
		t.Fatalf("CreateProductionBom: %v", err)
	}
	if repo.createdProductionBomCommand.OutputUnit != "盒" {
		t.Fatalf("OutputUnit = %q, want product inventory unit 盒", repo.createdProductionBomCommand.OutputUnit)
	}
}

func TestUpdateProductionBomDerivesOutputUnitFromProductInventoryUnit(t *testing.T) {
	repo := &fakeRepo{productRows: []Option{{ID: 88, Name: "10条速溶盒装", ProductKind: "instant_coffee", InventoryUnit: "盒"}}}
	svc := NewService(repo)
	ctx := context.Background()

	_, err := svc.UpdateProductionBom(ctx, UpdateProductionBomCommand{ID: 11, Name: "盒装新版", OutputProductID: 88, OutputUnit: "kg"})
	if err != nil {
		t.Fatalf("UpdateProductionBom: %v", err)
	}
	if repo.updatedProductionBomCommand.OutputUnit != "盒" {
		t.Fatalf("OutputUnit = %q, want product inventory unit 盒", repo.updatedProductionBomCommand.OutputUnit)
	}
}

func TestUpdateProductionBomDraftAcceptsProductComponentsAndOutputBasis(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	version, err := svc.UpdateProductionBomVersionDraft(ctx, UpdateProductionBomVersionDraftCommand{
		VersionID:  103,
		OutputQty:  1,
		OutputUnit: "盒",
		Items: []ProductionBomDraftItem{{
			ComponentType:      "product",
			ComponentProductID: 77,
			ConsumeUnit:        "unit_per_box",
			QtyPerUnit:         10,
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProductionBomVersionDraft: %v", err)
	}
	if version.OutputQty != 1 || version.OutputUnit != "盒" {
		t.Fatalf("version output basis = %+v, want 1 盒", version)
	}
	item := repo.updatedProductionDraftCommand.Items[0]
	if item.ComponentType != "product" || item.ComponentProductID != 77 || item.QtyPerUnit != 10 {
		t.Fatalf("product component not normalized/preserved: %+v", item)
	}

	_, err = svc.UpdateProductionBomVersionDraft(ctx, UpdateProductionBomVersionDraftCommand{
		VersionID:  103,
		OutputQty:  1,
		OutputUnit: "盒",
		Items:      []ProductionBomDraftItem{{ComponentType: "product", ComponentProductID: 77, ConsumeUnit: "ratio_pct", RatioPct: 10}},
	})
	if err == nil || !strings.Contains(err.Error(), "product consume_unit must not be ratio_pct") {
		t.Fatalf("expected product ratio validation error, got %v", err)
	}
}

func TestUpdateProductionBomDraftAppliesBomLevelMaterialLossAndRequiresRatioUnits(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()
	lossRate := 0.2

	_, err := svc.UpdateProductionBomVersionDraft(ctx, UpdateProductionBomVersionDraftCommand{
		VersionID:        103,
		OutputQty:        1,
		OutputUnit:       "kg",
		MaterialLossRate: &lossRate,
		Items: []ProductionBomDraftItem{{
			ComponentType:      "material",
			MaterialID:         7,
			ConsumeUnit:        "ratio_pct",
			RatioPct:           40,
			ComponentSpecG:     0,
			ComponentProductID: 0,
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProductionBomVersionDraft: %v", err)
	}
	items := repo.updatedProductionDraftCommand.Items
	if repo.updatedProductionDraftCommand.MaterialLossRate == nil || *repo.updatedProductionDraftCommand.MaterialLossRate != 0.2 {
		t.Fatalf("version material loss rate = %v, want 0.2", repo.updatedProductionDraftCommand.MaterialLossRate)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	if items[0].MaterialLossRate != 0.2 {
		t.Fatalf("ratio material inherited BOM loss rate = %.4f, want 0.2", items[0].MaterialLossRate)
	}

	_, err = svc.UpdateProductionBomVersionDraft(ctx, UpdateProductionBomVersionDraftCommand{
		VersionID:        103,
		OutputQty:        1,
		OutputUnit:       "kg",
		MaterialLossRate: &lossRate,
		Items: []ProductionBomDraftItem{{
			ComponentType: "material",
			MaterialID:    8,
			ConsumeUnit:   "kg",
			QtyPerUnit:    1,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "原料损耗比开启后，组件消耗单位只能使用比例 %") {
		t.Fatalf("expected BOM-level material loss ratio consume-unit error, got %v", err)
	}

	invalidLossRate := 1.0
	_, err = svc.UpdateProductionBomVersionDraft(ctx, UpdateProductionBomVersionDraftCommand{
		VersionID:        103,
		OutputQty:        1,
		OutputUnit:       "kg",
		MaterialLossRate: &invalidLossRate,
		Items: []ProductionBomDraftItem{{
			ComponentType: "material",
			MaterialID:    7,
			ConsumeUnit:   "ratio_pct",
			RatioPct:      40,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "material_loss_rate must be >= 0 and < 1") {
		t.Fatalf("expected version material loss validation error, got %v", err)
	}
}

func TestUpdateProductionBomDraftIgnoresLineLevelMaterialLossWithoutBomLevelSwitch(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	_, err := svc.UpdateProductionBomVersionDraft(ctx, UpdateProductionBomVersionDraftCommand{
		VersionID:  103,
		OutputQty:  1,
		OutputUnit: "kg",
		Items: []ProductionBomDraftItem{{
			ComponentType:    "material",
			MaterialID:       7,
			ConsumeUnit:      "ratio_pct",
			RatioPct:         40,
			MaterialLossRate: 0.5,
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProductionBomVersionDraft: %v", err)
	}
	if got := repo.updatedProductionDraftCommand.Items[0].MaterialLossRate; got != 0 {
		t.Fatalf("line-level material loss rate = %.4f, want ignored as 0", got)
	}
}

func TestPublishProductionBomVersionRunsOutputComponentAndCycleValidation(t *testing.T) {
	repo := &fakeRepo{publishValidationErr: errors.New("cycle detected")}
	svc := NewService(repo)
	err := svc.PublishProductionBomVersion(context.Background(), PublishProductionBomVersionCommand{VersionID: 103})
	if err == nil || !strings.Contains(err.Error(), "cycle detected") {
		t.Fatalf("expected publish validation error, got %v", err)
	}
}

func TestListProductionBomUsageByProductRequiresProductAndDelegates(t *testing.T) {
	repo := &fakeRepo{usageRows: []ProductionBomUsedByBom{{BomID: 8, BomName: "10条盒装速溶"}}}
	svc := NewService(repo)
	if _, err := svc.ListProductionBomUsageByProduct(context.Background(), 0); err == nil || !strings.Contains(err.Error(), "product_id required") {
		t.Fatalf("expected product_id validation error, got %v", err)
	}
	rows, err := svc.ListProductionBomUsageByProduct(context.Background(), 77)
	if err != nil {
		t.Fatalf("ListProductionBomUsageByProduct: %v", err)
	}
	if repo.usageProductID != 77 || len(rows) != 1 || rows[0].BomID != 8 {
		t.Fatalf("usage lookup = product %d rows %+v", repo.usageProductID, rows)
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
func (r errorRepo) DeleteProductionBomGroup(context.Context, DeleteProductionBomGroupCommand) error {
	return r.err
}
func (r errorRepo) MoveProductionBomGroup(context.Context, MoveProductionBomGroupCommand) error {
	return r.err
}
func (r errorRepo) CreateProductionBomGroupCategory(context.Context, CreateProductionBomGroupCategoryCommand) (ProductionBomGroupCategory, error) {
	return ProductionBomGroupCategory{}, r.err
}
func (r errorRepo) UpdateProductionBomGroupCategory(context.Context, UpdateProductionBomGroupCategoryCommand) (ProductionBomGroupCategory, error) {
	return ProductionBomGroupCategory{}, r.err
}
func (r errorRepo) DeleteProductionBomGroupCategory(context.Context, DeleteProductionBomGroupCategoryCommand) error {
	return r.err
}
func (r errorRepo) ListProductionBoms(context.Context) ([]ProductionBomSummary, error) {
	return nil, r.err
}
func (r errorRepo) GetProductionBomDetail(context.Context, int64, int64) (ProductionBomDetail, error) {
	return ProductionBomDetail{}, r.err
}
func (r errorRepo) ListProductionBomUsageByProduct(context.Context, int64) ([]ProductionBomUsedByBom, error) {
	return nil, r.err
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
func (r errorRepo) ValidateProductionBomVersionForPublish(context.Context, PublishProductionBomVersionCommand) error {
	return r.err
}
func (r errorRepo) PublishProductionBomVersion(context.Context, PublishProductionBomVersionCommand) error {
	return r.err
}
func (r errorRepo) BindProductProductionBom(context.Context, BindProductProductionBomCommand) (ProductProductionBomBinding, error) {
	return ProductProductionBomBinding{}, r.err
}
