package bom

import (
	"context"
	"errors"
	"reflect"
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
	materialRows                  []Option
	createdProductionBomCommand   CreateProductionBomCommand
	updatedProductionBomCommand   UpdateProductionBomCommand
	copiedProductionBomCommand    CopyProductionBomCommand
	updatedProductionDraftCommand UpdateProductionBomVersionDraftCommand
	publishValidationErr          error
	usageProductID                int64
	usageRows                     []ProductionBomUsedByBom
	syncedYield                   SyncProductYieldCommand
	productionBomFilter           ProductionBomFilter
	boundOutput                   BindProductionBomOutputCommand
}

func TestNormalizeProductionBomDraftWorkspaceRecipePayloadBySpecificationMode(t *testing.T) {
	items := []ProductionBomDraftItem{{MaterialID: 1}}
	variants := []ProductionBomDraftVariant{{SpecKey: "bag-454"}}

	materialItems, materialVariants := NormalizeProductionBomDraftWorkspaceRecipePayload("material", "single", items, variants)
	if !reflect.DeepEqual(materialItems, items) || materialVariants != nil {
		t.Fatalf("material payload = items %+v variants %+v, want items preserved and variants nil", materialItems, materialVariants)
	}

	singleItems, singleVariants := NormalizeProductionBomDraftWorkspaceRecipePayload("product", "single", items, variants)
	if !reflect.DeepEqual(singleItems, items) || singleVariants != nil {
		t.Fatalf("single product payload = items %+v variants %+v, want items preserved and variants nil", singleItems, singleVariants)
	}

	groupItems, groupVariants := NormalizeProductionBomDraftWorkspaceRecipePayload("product", "spec_group", items, variants)
	if groupItems != nil || !reflect.DeepEqual(groupVariants, variants) {
		t.Fatalf("spec-group product payload = items %+v variants %+v, want items nil and variants preserved", groupItems, groupVariants)
	}
}

func TestCreateProductionBomRequiresProductSpecificationGroup(t *testing.T) {
	repo := &fakeRepo{productRows: []Option{{ID: 88, Name: "盒装挂耳", InventoryUnit: "盒"}}}
	svc := NewService(repo)

	_, err := svc.CreateProductionBom(context.Background(), CreateProductionBomCommand{
		Name: "盒装挂耳 BOM", OutputProductID: 88, OutputQty: 1, SpecificationMode: "single",
	})
	if !errors.Is(err, ErrProductOutputRequiresBOMSpec) {
		t.Fatalf("single product BOM error=%v want ErrProductOutputRequiresBOMSpec", err)
	}
	if repo.createdProductionBomCommand.Name != "" {
		t.Fatal("invalid single product BOM must not reach repository")
	}

	variants := []ProductionBomDraftVariant{{SpecKey: "default", Name: "默认规格", InventoryUnit: "袋", IsDefault: true}}
	if _, err := svc.CreateProductionBom(context.Background(), CreateProductionBomCommand{
		Name: "盒装挂耳 BOM", OutputProductID: 88, OutputQty: 1, Variants: variants,
	}); err != nil {
		t.Fatalf("manual spec-group CreateProductionBom: %v", err)
	}
	if repo.createdProductionBomCommand.SpecificationMode != ProductionBomSpecificationModeSpecGroup {
		t.Fatalf("specification_mode=%q want spec_group", repo.createdProductionBomCommand.SpecificationMode)
	}
	if !reflect.DeepEqual(repo.createdProductionBomCommand.Variants, variants) {
		t.Fatalf("variants=%+v want %+v", repo.createdProductionBomCommand.Variants, variants)
	}
	if repo.createdProductionBomCommand.OutputUnit != "袋" {
		t.Fatalf("output_unit=%q want BOM default specification unit 袋", repo.createdProductionBomCommand.OutputUnit)
	}
}

func TestCreateProductionBomKeepsMaterialSingleOutputUnit(t *testing.T) {
	repo := &fakeRepo{materialRows: []Option{{ID: 95, Name: "咖啡粉", InventoryUnit: "kg"}}}
	svc := NewService(repo)
	if _, err := svc.CreateProductionBom(context.Background(), CreateProductionBomCommand{
		Name: "咖啡粉", OutputType: "material", OutputMaterialID: 95, OutputQty: 1,
	}); err != nil {
		t.Fatalf("material CreateProductionBom: %v", err)
	}
	if repo.createdProductionBomCommand.SpecificationMode != ProductionBomSpecificationModeSingle || repo.createdProductionBomCommand.OutputUnit != "kg" {
		t.Fatalf("material command=%+v", repo.createdProductionBomCommand)
	}
}

func TestDraftWorkspaceRejectsExplicitMixedSpecificationRecipes(t *testing.T) {
	repo := &workspaceNormalizationRepo{fakeRepo: &fakeRepo{productRows: []Option{{ID: 88, InventoryUnit: "盒"}}}}
	svc := NewService(repo)
	item := ProductionBomDraftItem{ComponentType: "material", MaterialID: 7, ConsumeUnit: "kg", QtyPerUnit: 1}
	variant := ProductionBomDraftVariant{SpecKey: "bag", Name: "袋装", InventoryUnit: "袋", IsDefault: true}

	for _, testCase := range []struct {
		mode     string
		items    []ProductionBomDraftItem
		variants []ProductionBomDraftVariant
		want     string
	}{
		{mode: ProductionBomSpecificationModeSingle, items: []ProductionBomDraftItem{item}, variants: []ProductionBomDraftVariant{variant}, want: ErrProductOutputRequiresBOMSpec.Error()},
		{mode: ProductionBomSpecificationModeSpecGroup, items: []ProductionBomDraftItem{item}, variants: []ProductionBomDraftVariant{variant}, want: "spec_group specification_mode cannot include flat recipe items"},
	} {
		_, err := svc.UpdateProductionBomDraftWorkspace(context.Background(), ProductionBomDraftWorkspaceCommand{
			Bom:            UpdateProductionBomCommand{ID: 11, Name: "盒装", OutputType: "product", OutputProductID: 88, SpecificationMode: testCase.mode, UpdateOutputBinding: true},
			Version:        UpdateProductionBomVersionDraftCommand{VersionID: 103, OutputQty: 1, OutputUnit: "盒", Items: testCase.items, Variants: testCase.variants},
			SpecTemplateID: 9, MainInputMaterialID: 7,
		})
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("mode %s mixed recipe error=%v, want %q", testCase.mode, err, testCase.want)
		}
	}
	if repo.workspaceCommand.Bom.ID != 0 {
		t.Fatal("invalid mixed recipe must not reach repository")
	}
}

type atomicPublishRepo struct {
	fakeRepo
	atomicCalls int
	atomicErr   error
}

type componentSpecUnitFakeRepo struct {
	*fakeRepo
	specUnits map[int64]string
}

type workspaceNormalizationRepo struct {
	*fakeRepo
	workspaceCommand   ProductionBomDraftWorkspaceCommand
	replacementCommand CreateProductionBomReplacementDraftCommand
}

func (r *workspaceNormalizationRepo) UpdateProductionBomDraftWorkspace(_ context.Context, cmd ProductionBomDraftWorkspaceCommand) (ProductionBomDetail, error) {
	r.workspaceCommand = cmd
	return ProductionBomDetail{ProductionBomSummary: ProductionBomSummary{ID: cmd.Bom.ID}}, nil
}

func (r *workspaceNormalizationRepo) CreateProductionBomReplacementDraft(_ context.Context, cmd CreateProductionBomReplacementDraftCommand) (ProductionBomDetail, error) {
	r.replacementCommand = cmd
	return ProductionBomDetail{ProductionBomSummary: ProductionBomSummary{ID: 88}}, nil
}

func TestDraftWorkspaceReusesVersionNormalizationAndLossOverride(t *testing.T) {
	repo := &workspaceNormalizationRepo{fakeRepo: &fakeRepo{materialRows: []Option{{ID: 95, Name: "初晓-半成品", InventoryUnit: "kg"}}}}
	svc := NewService(repo)
	loss := 0.195
	_, err := svc.UpdateProductionBomDraftWorkspace(context.Background(), ProductionBomDraftWorkspaceCommand{
		Bom: UpdateProductionBomCommand{ID: 11, Name: " 初晓 ", OutputType: "MATERIAL", OutputID: 95, UpdateOutputBinding: true, Actor: " VA "},
		Version: UpdateProductionBomVersionDraftCommand{VersionID: 103, OutputQty: 1, OutputUnit: "lb", MaterialLossRate: &loss, Items: []ProductionBomDraftItem{
			{MaterialID: 56, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 50, MaterialLossRate: 0.01},
			{MaterialID: 85, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 15},
		}},
	})
	if err != nil {
		t.Fatalf("UpdateProductionBomDraftWorkspace: %v", err)
	}
	got := repo.workspaceCommand
	if got.Bom.Name != "初晓" || got.Bom.OutputType != "material" || got.Bom.OutputMaterialID != 95 || got.Bom.OutputProductID != 0 {
		t.Fatalf("normalized BOM = %+v", got.Bom)
	}
	if got.Bom.OutputUnit != "kg" || got.Version.OutputUnit != "kg" {
		t.Fatalf("workspace output units = bom %q version %q, want kg", got.Bom.OutputUnit, got.Version.OutputUnit)
	}
	for index, item := range got.Version.Items {
		if item.MaterialLossRate != loss {
			t.Fatalf("item %d loss = %v, want %v", index, item.MaterialLossRate, loss)
		}
	}
}

func TestReplacementDraftNormalizesFullWorkspaceAndRequiresSourceVersion(t *testing.T) {
	repo := &workspaceNormalizationRepo{fakeRepo: &fakeRepo{materialRows: []Option{{ID: 95, Name: "初晓-半成品", InventoryUnit: "kg"}}}}
	svc := NewService(repo)
	if _, err := svc.CreateProductionBomReplacementDraft(context.Background(), CreateProductionBomReplacementDraftCommand{SourceBomID: 11}); err == nil || !strings.Contains(err.Error(), "source_version_id") {
		t.Fatalf("missing source version error = %v", err)
	}
	loss := 0.195
	row, err := svc.CreateProductionBomReplacementDraft(context.Background(), CreateProductionBomReplacementDraftCommand{
		SourceBomID: 11, SourceVersionID: 102,
		Workspace: ProductionBomDraftWorkspaceCommand{
			Bom:     UpdateProductionBomCommand{Name: "初晓", OutputType: "material", OutputID: 95, UpdateOutputBinding: true, Actor: "VA", GroupID: 165, GroupCategoryID: 885, UpdateGroupAssignment: true},
			Version: UpdateProductionBomVersionDraftCommand{OutputQty: 1, OutputUnit: "lb", MaterialLossRate: &loss, Items: []ProductionBomDraftItem{{MaterialID: 56, ConsumeUnit: "ratio_pct", RatioPct: 50}}},
		},
	})
	if err != nil {
		t.Fatalf("CreateProductionBomReplacementDraft: %v", err)
	}
	if row.ID != 88 || repo.replacementCommand.Workspace.Bom.OutputMaterialID != 95 || repo.replacementCommand.Workspace.Version.OutputUnit != "kg" {
		t.Fatalf("replacement = row %+v command %+v", row, repo.replacementCommand)
	}
	if got := repo.replacementCommand.Workspace.Version.Items[0].MaterialLossRate; got != loss {
		t.Fatalf("replacement item loss = %v, want %v", got, loss)
	}
}

func TestReplacementDraftRequiresPositiveOutputQuantity(t *testing.T) {
	repo := &workspaceNormalizationRepo{fakeRepo: &fakeRepo{materialRows: []Option{{ID: 95, Name: "初晓-半成品", InventoryUnit: "kg"}}}}
	svc := NewService(repo)
	_, err := svc.CreateProductionBomReplacementDraft(context.Background(), CreateProductionBomReplacementDraftCommand{
		SourceBomID: 11, SourceVersionID: 103,
		Workspace: ProductionBomDraftWorkspaceCommand{
			Bom:     UpdateProductionBomCommand{Name: "初晓", OutputType: "material", OutputID: 95, Actor: "VA"},
			Version: UpdateProductionBomVersionDraftCommand{OutputQty: 0, Items: []ProductionBomDraftItem{{MaterialID: 56, ConsumeUnit: "ratio_pct", RatioPct: 100}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "output_qty must be positive") {
		t.Fatalf("err=%v", err)
	}
}

func (r *componentSpecUnitFakeRepo) ProductionBomSpecInventoryUnits(_ context.Context, specIDs []int64) (map[int64]string, error) {
	units := make(map[int64]string, len(specIDs))
	for _, specID := range specIDs {
		if unit, ok := r.specUnits[specID]; ok {
			units[specID] = unit
		}
	}
	return units, nil
}

func (r *atomicPublishRepo) ValidateAndPublishProductionBomVersion(_ context.Context, _ PublishProductionBomVersionCommand) error {
	r.atomicCalls++
	return r.atomicErr
}

func (r *fakeRepo) List(ctx context.Context) ([]ListItem, error) { return r.listRows, nil }
func (r *fakeRepo) Detail(ctx context.Context, productID int64) (Detail, error) {
	return Detail{}, nil
}
func (r *fakeRepo) Products(ctx context.Context) ([]Option, error)  { return r.productRows, nil }
func (r *fakeRepo) Materials(ctx context.Context) ([]Option, error) { return r.materialRows, nil }
func (r *fakeRepo) BagSpecMappings(ctx context.Context) ([]BagSpecMapping, error) {
	return nil, nil
}
func (r *fakeRepo) SyncProductYield(ctx context.Context, cmd SyncProductYieldCommand) error {
	r.syncedYield = cmd
	return nil
}

func TestCurrentBomResponsesNeutralizeLegacyOverallYieldAndLoss(t *testing.T) {
	listItem := ListItem{YieldRate: 0.8, ExpectedYieldRate: 0.8, ExpectedLossRate: 0.2}
	detail := Detail{YieldRate: 0.8, ExpectedYieldRate: 0.8, ExpectedLossRate: 0.2}
	version := Version{YieldRate: 0.8, ExpectedYieldRate: 0.8, ExpectedLossRate: 0.2}
	summary := ProductionBomSummary{ExpectedYieldRate: 0.8, ExpectedLossRate: 0.2}
	productionVersion := ProductionBomVersion{YieldRate: 0.8, ExpectedYieldRate: 0.8, ExpectedLossRate: 0.2}

	enrichListItemYield(&listItem)
	enrichDetailYield(&detail)
	enrichVersionYield(&version)
	enrichProductionBomSummaryYield(&summary)
	enrichProductionBomVersionYield(&productionVersion)

	for label, values := range map[string][2]float64{
		"list":               {listItem.ExpectedYieldRate, listItem.ExpectedLossRate},
		"detail":             {detail.ExpectedYieldRate, detail.ExpectedLossRate},
		"version":            {version.ExpectedYieldRate, version.ExpectedLossRate},
		"production summary": {summary.ExpectedYieldRate, summary.ExpectedLossRate},
		"production version": {productionVersion.ExpectedYieldRate, productionVersion.ExpectedLossRate},
	} {
		if values[0] != 1 || values[1] != 0 {
			t.Fatalf("%s compatibility yield/loss = %v, want [1 0]", label, values)
		}
	}
	if listItem.YieldRate != 1 || detail.YieldRate != 1 || version.YieldRate != 1 || productionVersion.YieldRate != 1 {
		t.Fatalf("legacy yield fields must also be neutralized")
	}
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
func (r *fakeRepo) ListProductionBomsFiltered(_ context.Context, filter ProductionBomFilter) ([]ProductionBomSummary, error) {
	r.productionBomFilter = filter
	return []ProductionBomSummary{{ID: 21, OutputType: filter.OutputType, OutputID: filter.OutputID}}, nil
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
	return ProductionBomSummary{ID: 11, Name: cmd.Name, OutputType: cmd.OutputType, OutputID: cmd.OutputID, OutputProductID: cmd.OutputProductID, OutputMaterialID: cmd.OutputMaterialID}, nil
}
func (r *fakeRepo) UpdateProductionBom(_ context.Context, cmd UpdateProductionBomCommand) (ProductionBomSummary, error) {
	r.updatedProductionBomCommand = cmd
	return ProductionBomSummary{ID: cmd.ID, Name: cmd.Name, OutputProductID: cmd.OutputProductID}, nil
}
func (r *fakeRepo) CopyProductionBom(_ context.Context, cmd CopyProductionBomCommand) (ProductionBomSummary, error) {
	r.copiedProductionBomCommand = cmd
	return ProductionBomSummary{Name: cmd.Name}, nil
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
func (r *fakeRepo) BindProductionBomOutput(_ context.Context, cmd BindProductionBomOutputCommand) (ProductionBomOutputBinding, error) {
	r.boundOutput = cmd
	return ProductionBomOutputBinding{OutputType: cmd.OutputType, OutputID: cmd.OutputID, BomID: cmd.BomID, BomVersionID: 501, IsDefault: true}, nil
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

func TestServiceKeepsGreenBeanProductsAvailableForProductionBomOutputs(t *testing.T) {
	repo := &fakeRepo{
		listRows: []ListItem{
			{ProductID: 1, Product: "岩师傅熟豆", CustomerID: 152, ProductKind: "roasted_bean"},
			{ProductID: 2, Product: "萨琪姆 生豆", CustomerID: 0, ProductKind: "green_bean"},
			{ProductID: 3, Product: "岩师傅挂耳", CustomerID: 152, ProductKind: "drip_bag"},
			{ProductID: 4, Product: "萨琪姆 生豆 Kg", CustomerID: 0, ProductKind: "green_bean"},
		},
		productRows: []Option{
			{ID: 1, Name: "岩师傅熟豆", CustomerID: 152, ProductKind: "roasted_bean"},
			{ID: 2, Name: "萨琪姆 生豆", CustomerID: 0, ProductKind: "green_bean"},
			{ID: 3, Name: "岩师傅挂耳", CustomerID: 152, ProductKind: "drip_bag"},
			{ID: 4, Name: "萨琪姆 生豆 Kg", CustomerID: 0, ProductKind: "green_bean"},
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
	if len(productRows) != 4 || productRows[0].ID != 1 || productRows[1].ID != 2 || productRows[2].ID != 3 || productRows[3].ID != 4 {
		t.Fatalf("Product rows = %+v, want green bean parent and child SKU available as production BOM outputs", productRows)
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

func TestServiceIgnoresLegacyOverallLossWrites(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	loss := 0.2
	if err := svc.SyncProductYield(context.Background(), SyncProductYieldCommand{ProductID: 7, ExpectedLossRate: &loss}); err != nil {
		t.Fatalf("SyncProductYield: %v", err)
	}
	if repo.syncedYield.ExpectedLossRate != nil || repo.syncedYield.ExpectedYieldRate != 1 {
		t.Fatalf("legacy product loss write must normalize to yield 1: %+v", repo.syncedYield)
	}

	if _, err := svc.CreateProductionBom(context.Background(), CreateProductionBomCommand{Name: "曲奇拼配", OutputProductID: 643, ExpectedLossRate: &loss, Variants: []ProductionBomDraftVariant{{SpecKey: "default", Name: "默认规格", InventoryUnit: "袋", IsDefault: true}}}); err != nil {
		t.Fatalf("CreateProductionBom: %v", err)
	}
	if repo.createdProductionBomCommand.ExpectedLossRate == nil || *repo.createdProductionBomCommand.ExpectedLossRate != 0 {
		t.Fatalf("legacy create loss must normalize to zero: %+v", repo.createdProductionBomCommand)
	}

	if _, err := svc.UpdateProductionBomVersionDraft(context.Background(), UpdateProductionBomVersionDraftCommand{VersionID: 11, ExpectedLossRate: &loss}); err != nil {
		t.Fatalf("UpdateProductionBomVersionDraft: %v", err)
	}
	if repo.updatedProductionDraftCommand.ExpectedLossRate == nil || *repo.updatedProductionDraftCommand.ExpectedLossRate != 0 {
		t.Fatalf("legacy draft loss must normalize to zero: %+v", repo.updatedProductionDraftCommand)
	}
}

func TestNormalizeProductionBomNameIsIdempotent(t *testing.T) {
	tests := map[string]string{
		"BOM-000659 ALO TOH#1 生产 BOM / V001":       "ALO TOH#1",
		"BOM000643 曲奇拼配 生产 BOM / V001":             "曲奇拼配",
		"BOM000643曲奇拼配 生产 BOM":                     "曲奇拼配",
		"BOM-003262 PR442-SCENARIO Production BOM": "PR442-SCENARIO",
		"GoalE2E 咖啡熟豆 BOM":                         "GoalE2E 咖啡熟豆",
		"ALO TOH#1 生产 BOM 副本 副本":                   "ALO TOH#1 副本 副本",
		"生产 BOM 校准配方":                              "生产 BOM 校准配方",
	}
	for input, want := range tests {
		got := NormalizeProductionBomName(input)
		if got != want {
			t.Errorf("NormalizeProductionBomName(%q) = %q, want %q", input, got, want)
		}
		if again := NormalizeProductionBomName(got); again != got {
			t.Errorf("NormalizeProductionBomName must be idempotent: first %q, second %q", got, again)
		}
	}
}

func TestProductionBomWritesNormalizeBusinessName(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if _, err := svc.CreateProductionBom(context.Background(), CreateProductionBomCommand{Name: "BOM-000659 ALO TOH#1 生产 BOM / V001", OutputProductID: 643, Variants: []ProductionBomDraftVariant{{SpecKey: "default", Name: "默认规格", InventoryUnit: "袋", IsDefault: true}}}); err != nil {
		t.Fatalf("CreateProductionBom: %v", err)
	}
	if repo.createdProductionBomCommand.Name != "ALO TOH#1" {
		t.Fatalf("create name = %q", repo.createdProductionBomCommand.Name)
	}

	if _, err := svc.UpdateProductionBom(context.Background(), UpdateProductionBomCommand{ID: 11, Name: "BOM000643 曲奇拼配 生产 BOM"}); err != nil {
		t.Fatalf("UpdateProductionBom: %v", err)
	}
	if repo.updatedProductionBomCommand.Name != "曲奇拼配" {
		t.Fatalf("update name = %q", repo.updatedProductionBomCommand.Name)
	}

	if _, err := svc.CopyProductionBom(context.Background(), CopyProductionBomCommand{ID: 11, Name: "BOM000643 曲奇拼配 生产 BOM / V001"}); err != nil {
		t.Fatalf("CopyProductionBom: %v", err)
	}
	if repo.copiedProductionBomCommand.Name != "曲奇拼配" {
		t.Fatalf("copy name = %q", repo.copiedProductionBomCommand.Name)
	}
}

func TestCopyProductionBomCanReapplyTemplateWhenKeepingSourceProductOutput(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.CopyProductionBom(context.Background(), CopyProductionBomCommand{
		ID:                    11,
		Name:                  "规格组新版",
		SpecificationMode:     ProductionBomSpecificationModeSpecGroup,
		SpecTemplateVersionID: 92,
		MainInputMaterialID:   7,
	})
	if err != nil {
		t.Fatalf("CopyProductionBom: %v", err)
	}
	if repo.copiedProductionBomCommand.SpecTemplateVersionID != 92 || repo.copiedProductionBomCommand.MainInputMaterialID != 7 {
		t.Fatalf("template replacement command not propagated: %+v", repo.copiedProductionBomCommand)
	}
}

func TestCreateProductionBomRequiresOutputProduct(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	if _, err := svc.CreateProductionBom(ctx, CreateProductionBomCommand{Name: "10条速溶盒装"}); err == nil || !strings.Contains(err.Error(), "output_product_id required") {
		t.Fatalf("expected output_product_id validation error, got %v", err)
	}
	row, err := svc.CreateProductionBom(ctx, CreateProductionBomCommand{Name: "10条速溶盒装", OutputProductID: 88, OutputQty: 1, Variants: []ProductionBomDraftVariant{{SpecKey: "box", Name: "盒装", InventoryUnit: "盒", IsDefault: true}}})
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

func TestCreateProductionBomSupportsTypedMaterialAndProductSpecificationOutputs(t *testing.T) {
	repo := &fakeRepo{
		productRows:  []Option{{ID: 88, Name: "盒装", InventoryUnit: "盒"}},
		materialRows: []Option{{ID: 95, Name: "烘焙豆", InventoryUnit: "kg"}},
	}
	svc := NewService(repo)
	ctx := context.Background()

	material, err := svc.CreateProductionBom(ctx, CreateProductionBomCommand{
		Name: "烘焙豆", OutputType: "material", OutputMaterialID: 95, OutputQty: 1, OutputUnit: "g",
	})
	if err != nil {
		t.Fatalf("CreateProductionBom material output: %v", err)
	}
	if material.OutputType != "material" || material.OutputID != 95 || material.OutputMaterialID != 95 {
		t.Fatalf("material output = %+v", material)
	}
	if repo.createdProductionBomCommand.OutputUnit != "kg" {
		t.Fatalf("material output unit = %q, want kg", repo.createdProductionBomCommand.OutputUnit)
	}

	product, err := svc.CreateProductionBom(ctx, CreateProductionBomCommand{Name: "盒装", OutputProductID: 88, OutputQty: 1, Variants: []ProductionBomDraftVariant{{SpecKey: "box", Name: "盒装", InventoryUnit: "盒", IsDefault: true}}})
	if err != nil {
		t.Fatalf("CreateProductionBom product specification: %v", err)
	}
	if product.OutputType != "product" || product.OutputID != 88 || repo.createdProductionBomCommand.OutputUnit != "盒" {
		t.Fatalf("product specification output = %+v command=%+v", product, repo.createdProductionBomCommand)
	}

	for _, cmd := range []CreateProductionBomCommand{
		{Name: "无产出", OutputType: "material"},
		{Name: "双产出", OutputType: "material", OutputMaterialID: 95, OutputProductID: 88},
		{Name: "类型错配", OutputType: "product", OutputMaterialID: 95},
	} {
		if _, err := svc.CreateProductionBom(ctx, cmd); err == nil {
			t.Fatalf("CreateProductionBom(%+v) should reject invalid output binding", cmd)
		}
	}
}

func TestListProductionBomsPassesUnifiedOutputAndComponentFilters(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	filter := ProductionBomFilter{OutputType: "material", OutputID: 95, ComponentType: "product", ComponentID: 88}
	rows, err := svc.ListProductionBoms(context.Background(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || repo.productionBomFilter != filter {
		t.Fatalf("ListProductionBoms() rows=%+v filter=%+v", rows, repo.productionBomFilter)
	}
}

func TestBindProductionBomOutputSupportsAuditableMaterialDefaultSwitch(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	row, err := svc.BindProductionBomOutput(context.Background(), BindProductionBomOutputCommand{
		OutputType: " material ", OutputID: 95, BomID: 21, Actor: "测试员",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.OutputType != "material" || row.OutputID != 95 || row.BomID != 21 || !row.IsDefault {
		t.Fatalf("binding = %+v", row)
	}
	if repo.boundOutput.OutputType != "material" || repo.boundOutput.Actor != "测试员" {
		t.Fatalf("binding command = %+v", repo.boundOutput)
	}
}

func TestCreateProductionBomDerivesOutputUnitFromDefaultBOMSpecification(t *testing.T) {
	repo := &fakeRepo{productRows: []Option{{ID: 88, Name: "10条速溶盒装", ProductKind: "instant_coffee", InventoryUnit: "盒"}}}
	svc := NewService(repo)
	ctx := context.Background()

	_, err := svc.CreateProductionBom(ctx, CreateProductionBomCommand{Name: "10条速溶盒装", OutputProductID: 88, OutputQty: 1, OutputUnit: "kg", Variants: []ProductionBomDraftVariant{{SpecKey: "box", Name: "盒装", InventoryUnit: "盒", IsDefault: true}}})
	if err != nil {
		t.Fatalf("CreateProductionBom: %v", err)
	}
	if repo.createdProductionBomCommand.OutputUnit != "盒" {
		t.Fatalf("OutputUnit = %q, want product inventory unit 盒", repo.createdProductionBomCommand.OutputUnit)
	}
}

func TestUpdateProductionBomDoesNotUseProductMasterInventoryUnit(t *testing.T) {
	repo := &fakeRepo{productRows: []Option{{ID: 88, Name: "10条速溶盒装", ProductKind: "instant_coffee", InventoryUnit: "盒"}}}
	svc := NewService(repo)
	ctx := context.Background()

	_, err := svc.UpdateProductionBom(ctx, UpdateProductionBomCommand{ID: 11, Name: "盒装新版", OutputProductID: 88, OutputUnit: "kg"})
	if err != nil {
		t.Fatalf("UpdateProductionBom: %v", err)
	}
	if repo.updatedProductionBomCommand.OutputUnit != "" {
		t.Fatalf("OutputUnit = %q, want BOM specification-owned unit", repo.updatedProductionBomCommand.OutputUnit)
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
			ComponentBomSpecID: 701,
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
	if item.ComponentType != "product" || item.ComponentProductID != 77 || item.ComponentBomSpecID != 701 || item.QtyPerUnit != 10 {
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

	_, err = svc.UpdateProductionBomVersionDraft(ctx, UpdateProductionBomVersionDraftCommand{
		VersionID: 103, OutputQty: 1, OutputUnit: "盒",
		Items: []ProductionBomDraftItem{{ComponentType: "material", MaterialID: 7, ComponentBomSpecID: 701, ConsumeUnit: "kg", QtyPerUnit: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "component_bom_spec_id requires product component") {
		t.Fatalf("material component specification identity error = %v", err)
	}
}

func TestProductionBomDraftUsesSelectedComponentSpecInventoryUnitInsteadOfParentProductUnit(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		specID     int64
		consume    string
		wantError  bool
		legacySpec bool
	}{
		{name: "selected box spec accepts box", specID: 701, consume: "盒"},
		{name: "selected box spec rejects parent kg", specID: 701, consume: "kg", wantError: true},
		{name: "selected bag spec accepts bag", specID: 702, consume: "袋"},
		{name: "different spec units stay isolated", specID: 702, consume: "盒", wantError: true},
		{name: "legacy component without spec keeps parent unit", consume: "kg", legacySpec: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			baseRepo := &fakeRepo{productRows: []Option{{ID: 77, Name: "父商品", InventoryUnit: "kg"}}}
			repo := &componentSpecUnitFakeRepo{fakeRepo: baseRepo, specUnits: map[int64]string{701: "盒", 702: "袋"}}
			svc := NewService(repo)
			specID := testCase.specID
			if testCase.legacySpec {
				specID = 0
			}
			_, err := svc.UpdateProductionBomVersionDraft(context.Background(), UpdateProductionBomVersionDraftCommand{
				VersionID: 103,
				Items: []ProductionBomDraftItem{{
					ComponentType:      "product",
					ComponentProductID: 77,
					ComponentBomSpecID: specID,
					ConsumeUnit:        testCase.consume,
					QtyPerUnit:         1,
				}},
			})
			if testCase.wantError {
				if err == nil || !strings.Contains(err.Error(), "component BOM specification inventory_unit") {
					t.Fatalf("consume_unit %q with spec %d error=%v", testCase.consume, specID, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("consume_unit %q with spec %d: %v", testCase.consume, specID, err)
			}
		})
	}
}

func TestUpdateProductionBomDraftAppliesBomLevelMaterialLossOnlyToRatioMaterials(t *testing.T) {
	repo := &fakeRepo{materialRows: []Option{
		{ID: 7, Name: "拼配原料", InventoryUnit: "kg"},
		{ID: 8, Name: "包装袋", InventoryUnit: "个"},
	}}
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
		Items: []ProductionBomDraftItem{
			{
				ComponentType: "material",
				MaterialID:    7,
				ConsumeUnit:   "ratio_pct",
				RatioPct:      40,
			},
			{
				ComponentType: "material",
				MaterialID:    8,
				ConsumeUnit:   "个",
				QtyPerUnit:    2,
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "material_loss_rate requires all components to use material ratio_pct") {
		t.Fatalf("loss recipe must reject a fixed component, got %v", err)
	}

	zeroLossRate := 0.0
	_, err = svc.UpdateProductionBomVersionDraft(ctx, UpdateProductionBomVersionDraftCommand{
		VersionID:        103,
		OutputQty:        1,
		OutputUnit:       "kg",
		MaterialLossRate: &zeroLossRate,
		Items: []ProductionBomDraftItem{{
			ComponentType: "material",
			MaterialID:    8,
			ConsumeUnit:   "盒",
			QtyPerUnit:    1,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "consume_unit must match component inventory_unit") {
		t.Fatalf("mismatched packaging unit must be rejected, got %v", err)
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

func TestUpdateProductionBomDraftRejectsMixedRatioAndFixedModeWithoutLoss(t *testing.T) {
	repo := &fakeRepo{materialRows: []Option{
		{ID: 7, Name: "拼配原料", InventoryUnit: "kg"},
		{ID: 8, Name: "包装袋", InventoryUnit: "个"},
	}}
	svc := NewService(repo)
	zero := 0.0

	_, err := svc.UpdateProductionBomVersionDraft(context.Background(), UpdateProductionBomVersionDraftCommand{
		VersionID:        103,
		OutputQty:        1,
		OutputUnit:       "盒",
		MaterialLossRate: &zero,
		Items: []ProductionBomDraftItem{
			{ComponentType: "material", MaterialID: 7, ConsumeUnit: "ratio_pct", RatioPct: 100},
			{ComponentType: "material", MaterialID: 8, ConsumeUnit: "个", QtyPerUnit: 1},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "ratio and fixed consume units cannot be mixed") {
		t.Fatalf("non-loss recipe must reject ratio/fixed mixing, got %v", err)
	}
}

func TestUpdateProductionBomDraftValidatesEverySpecificationVariant(t *testing.T) {
	repo := &fakeRepo{
		materialRows: []Option{{ID: 7, Name: "熟豆", InventoryUnit: "kg"}, {ID: 8, Name: "袋", InventoryUnit: "个"}},
		productRows:  []Option{{ID: 77, Name: "规格商品", InventoryUnit: "袋", SpecIdentityMode: "bom_spec"}, {ID: 78, Name: "袋装挂耳", InventoryUnit: "袋", SpecIdentityMode: "product"}},
	}
	svc := NewService(repo)

	_, err := svc.UpdateProductionBomVersionDraft(context.Background(), UpdateProductionBomVersionDraftCommand{
		VersionID: 103,
		Variants: []ProductionBomDraftVariant{
			{SpecKey: "227g", Name: "227g 袋装", InventoryUnit: "袋", IsDefault: true, MaterialLossRate: 0.1, Items: []ProductionBomDraftItem{{ComponentType: "material", MaterialID: 7, ConsumeUnit: "ratio_pct", RatioPct: 100}}},
			{SpecKey: "454g", Name: "454g 袋装", InventoryUnit: "袋", MaterialLossRate: 0.1, Items: []ProductionBomDraftItem{{ComponentType: "material", MaterialID: 7, ConsumeUnit: "ratio_pct", RatioPct: 90}, {ComponentType: "material", MaterialID: 8, ConsumeUnit: "个", QtyPerUnit: 1}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "variant 454g: material_loss_rate requires all components to use material ratio_pct") {
		t.Fatalf("invalid second variant must reject the whole draft, got %v", err)
	}
	if repo.updatedProductionDraftCommand.VersionID != 0 {
		t.Fatal("invalid specification group must not reach repository")
	}

	_, err = svc.UpdateProductionBomVersionDraft(context.Background(), UpdateProductionBomVersionDraftCommand{
		VersionID: 103,
		Variants: []ProductionBomDraftVariant{{
			SpecKey:       "box",
			Name:          "组合装",
			InventoryUnit: "盒",
			IsDefault:     true,
			Items: []ProductionBomDraftItem{{
				ComponentType:      "product",
				ComponentProductID: 77,
				ConsumeUnit:        "unit_per_box",
				QtyPerUnit:         1,
			}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "requires component_bom_spec_id") {
		t.Fatalf("product component without an explicit BOM specification must be rejected, got %v", err)
	}

	_, err = svc.UpdateProductionBomVersionDraft(context.Background(), UpdateProductionBomVersionDraftCommand{
		VersionID: 103,
		Items: []ProductionBomDraftItem{{
			ComponentType:      "product",
			ComponentProductID: 78,
			ConsumeUnit:        "袋",
			QtyPerUnit:         10,
		}},
	})
	if err != nil {
		t.Fatalf("direct product component without BOM specification should pass: %v", err)
	}
	if got := repo.updatedProductionDraftCommand.Items[0].ComponentBomSpecID; got != 0 {
		t.Fatalf("direct product component bom_spec_id=%d, want 0", got)
	}

	_, err = svc.UpdateProductionBomVersionDraft(context.Background(), UpdateProductionBomVersionDraftCommand{
		VersionID: 103,
		Variants: []ProductionBomDraftVariant{
			{SpecKey: "227g", Name: "227g 袋装", InventoryUnit: "袋", IsDefault: true, Items: []ProductionBomDraftItem{{ComponentType: "material", MaterialID: 7, ConsumeUnit: "kg", QtyPerUnit: .227}, {ComponentType: "material", MaterialID: 8, ConsumeUnit: "个", QtyPerUnit: 1}}},
			{SpecKey: "454g", Name: "454g 袋装", InventoryUnit: "袋", Items: []ProductionBomDraftItem{{ComponentType: "material", MaterialID: 7, ConsumeUnit: "kg", QtyPerUnit: .454}, {ComponentType: "material", MaterialID: 8, ConsumeUnit: "个", QtyPerUnit: 1}}},
		},
	})
	if err != nil {
		t.Fatalf("all-fixed specification group should pass: %v", err)
	}
	if len(repo.updatedProductionDraftCommand.Variants) != 2 {
		t.Fatalf("variants were not forwarded atomically: %+v", repo.updatedProductionDraftCommand.Variants)
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

func TestPublishProductionBomVersionUsesAtomicRepositoryCapability(t *testing.T) {
	repo := &atomicPublishRepo{}
	svc := NewService(repo)
	if err := svc.PublishProductionBomVersion(context.Background(), PublishProductionBomVersionCommand{VersionID: 103}); err != nil {
		t.Fatal(err)
	}
	if repo.atomicCalls != 1 {
		t.Fatalf("atomic publish calls = %d, want 1", repo.atomicCalls)
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
