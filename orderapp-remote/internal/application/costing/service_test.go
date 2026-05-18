package costing

import (
	"context"
	"reflect"
	"testing"

	domain "orderapp/internal/domain/costing"
)

type fakeRepo struct {
	params            domain.Parameters
	inputs            []domain.ProductInput
	savedItems        []domain.ProductResult
	publishedID       int64
	savedDripTemplate SaveDripPriceTemplateCommand
	deactivatedDripID int64
	publishedBeanList PublishBeanListCommand
	draftBeanList     PublishBeanListCommand
}

func (r *fakeRepo) LoadParameters(context.Context) (domain.Parameters, error) {
	if r.params.RoastYieldRate == 0 {
		return domain.DefaultParameters(), nil
	}
	return r.params, nil
}

func (r *fakeRepo) LoadProductInputs(context.Context, domain.Parameters) ([]domain.ProductInput, error) {
	return r.inputs, nil
}

func (r *fakeRepo) CreateRun(_ context.Context, actor string, items []domain.ProductResult) (*Run, error) {
	r.savedItems = items
	return &Run{ID: 42, Status: "draft", ProductCount: len(items), Items: items}, nil
}

func (r *fakeRepo) PublishRun(_ context.Context, actor string, runID int64) error {
	r.publishedID = runID
	return nil
}

func (r *fakeRepo) ListParameterSettings(context.Context) ([]ParameterSetting, error) {
	return nil, nil
}

func (r *fakeRepo) UpdateParameterSetting(context.Context, UpdateParameterCommand) (ParameterSetting, error) {
	return ParameterSetting{}, nil
}

func (r *fakeRepo) ListDripPriceTemplates(context.Context) ([]domain.DripPriceTemplate, error) {
	return nil, nil
}

func (r *fakeRepo) SaveDripPriceTemplate(_ context.Context, cmd SaveDripPriceTemplateCommand) (*domain.DripPriceTemplate, error) {
	r.savedDripTemplate = cmd
	includePackaging := true
	if cmd.IncludePackaging != nil {
		includePackaging = *cmd.IncludePackaging
	}
	return &domain.DripPriceTemplate{
		ID:               1,
		Name:             cmd.Name,
		Active:           cmd.Active == nil || *cmd.Active,
		BagGrams:         cmd.BagGrams,
		BoxBagCount:      cmd.BoxBagCount,
		IncludePackaging: includePackaging,
	}, nil
}

func (r *fakeRepo) DeactivateDripPriceTemplate(_ context.Context, cmd DeactivateDripPriceTemplateCommand) error {
	r.deactivatedDripID = cmd.ID
	return nil
}

func (r *fakeRepo) ListBeanListPublications(context.Context, BeanListPublicationQuery) ([]BeanListPublication, error) {
	return nil, nil
}

func (r *fakeRepo) PublishedBeanList(context.Context, BeanListPublicationQuery) (*BeanListPublication, error) {
	return nil, nil
}

func (r *fakeRepo) PublishBeanList(_ context.Context, cmd PublishBeanListCommand) (*BeanListPublication, error) {
	r.publishedBeanList = cmd
	return &BeanListPublication{ID: 1, ListType: cmd.ListType, Version: cmd.Version, Status: "published", OwnerType: cmd.OwnerType, OwnerKey: cmd.OwnerKey, PriceSourcePublicationID: cmd.PriceSourcePublicationID, StyleSourcePublicationID: cmd.StyleSourcePublicationID}, nil
}

func (r *fakeRepo) SaveBeanListDraft(_ context.Context, cmd PublishBeanListCommand) (*BeanListPublication, error) {
	r.draftBeanList = cmd
	return &BeanListPublication{ID: 2, ListType: cmd.ListType, Version: cmd.Version, Status: "draft", OwnerType: cmd.OwnerType, OwnerKey: cmd.OwnerKey, PriceSourcePublicationID: cmd.PriceSourcePublicationID, StyleSourcePublicationID: cmd.StyleSourcePublicationID}, nil
}

func (r *fakeRepo) WithdrawBeanList(context.Context, WithdrawBeanListCommand) error {
	return nil
}

func TestCalculateRejectsEmptyProducts(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if _, err := svc.Calculate(context.Background(), CalculateRequest{}); err == nil {
		t.Fatalf("expected products required error")
	}
}

func TestCalculateReturnsCostingItems(t *testing.T) {
	svc := NewService(&fakeRepo{})
	resp, err := svc.Calculate(context.Background(), CalculateRequest{Products: []domain.ProductInput{{
		ProductID:          1,
		Name:               "金色山脉",
		GreenBeanCostPerKg: 62,
		YieldRate:          0.8,
	}}})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Name != "金色山脉" {
		t.Fatalf("items = %+v", resp.Items)
	}
	if resp.Items[0].Retail227gPrice <= 0 || len(resp.Items[0].WholesaleKgPrices) == 0 {
		t.Fatalf("missing calculated prices: %+v", resp.Items[0])
	}
}

func TestCreateRunCalculatesAndPersistsDatabaseInputs(t *testing.T) {
	repo := &fakeRepo{inputs: []domain.ProductInput{{
		ProductID:          7,
		Name:               "孟连水洗",
		GreenBeanCostPerKg: 62,
		YieldRate:          0.8,
	}}}
	svc := NewService(repo)

	run, err := svc.CreateRun(context.Background(), "JJ")
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if run.ID != 42 || run.ProductCount != 1 || len(repo.savedItems) != 1 {
		t.Fatalf("run = %+v, saved = %+v", run, repo.savedItems)
	}
}

func TestBeanListOrdersItemsByExcelCommercialCode(t *testing.T) {
	repo := &fakeRepo{inputs: []domain.ProductInput{
		{Name: "Uraga乌拉嘎", GreenBeanCostPerKg: 108, YieldRate: 0.8},
		{Name: "曲奇拼配", GreenBeanCostPerKg: 51.75, YieldRate: 0.8},
		{Name: "金色山脉", GreenBeanCostPerKg: 62, YieldRate: 0.8},
	}}
	svc := NewService(repo)

	resp, err := svc.BeanList(context.Background())
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}

	got := []string{
		resp.Items[0].CommercialBeanList.Code,
		resp.Items[1].CommercialBeanList.Code,
		resp.Items[2].CommercialBeanList.Code,
	}
	want := []string{"1.1", "3.1", "5.2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("codes = %+v, want %+v", got, want)
		}
	}
}

func TestBeanListAppliesCategoryGradientTemplateAndLeavesUnboundDefaults(t *testing.T) {
	repo := &fakeRepo{inputs: []domain.ProductInput{
		{
			ProductID:          501,
			Name:               "模板拼配",
			GreenBeanCostPerKg: 51.75,
			YieldRate:          0.8,
			GradientTemplate: &domain.GradientTemplate{
				ID:          9,
				Name:        "工厂量单模板",
				DisplayUnit: domain.GradientDisplayUnitKg,
				Tiers: []domain.GradientTemplateTier{{
					ID: 91, Label: "大客户量单", MinWeightG: 24000, MaxWeightG: floatPtr(49000), MarginRate: 0.175, Position: 1,
				}},
			},
		},
		{
			ProductID:          502,
			Name:               "金色山脉",
			GreenBeanCostPerKg: 62,
			YieldRate:          0.8,
		},
	}}
	svc := NewService(repo)

	resp, err := svc.BeanList(context.Background())
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}
	var templated, unbound domain.ProductResult
	for _, item := range resp.Items {
		if item.ProductID == 501 {
			templated = item
		}
		if item.ProductID == 502 {
			unbound = item
		}
	}
	if len(templated.CommercialWholesaleTiers) != 1 || templated.CommercialWholesaleTiers[0].Label != "大客户量单" || templated.CommercialWholesaleTiers[0].DisplayUnit != domain.GradientDisplayUnitKg {
		t.Fatalf("templated tiers = %+v", templated.CommercialWholesaleTiers)
	}
	if len(unbound.CommercialWholesaleTiers) != 4 || unbound.CommercialWholesaleTiers[0].Label != "2包-13包" {
		t.Fatalf("unbound tiers = %+v", unbound.CommercialWholesaleTiers)
	}
}

func TestBeanListKeepsGreenBeanProductsOnDirectSaleTiers(t *testing.T) {
	repo := &fakeRepo{inputs: []domain.ProductInput{{
		ProductID:   909,
		Name:        "埃塞瑰夏生豆",
		ProductKind: "green_bean",
		GreenBeanSaleTiers: []domain.CommercialWholesaleTier{{
			Label:        "1kg+",
			SpecG:        1000,
			MinQty:       1,
			PricePerUnit: 128,
			DisplayUnit:  domain.GradientDisplayUnitKg,
		}},
	}}}
	svc := NewService(repo)

	resp, err := svc.BeanList(context.Background())
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %+v", resp.Items)
	}
	item := resp.Items[0]
	if item.ProductKind != "green_bean" {
		t.Fatalf("product_kind = %q", item.ProductKind)
	}
	if item.GreenBeanCostPerKg != 0 || item.RoastedBeanCostPerKg != 0 {
		t.Fatalf("green sales item must not run roasted costing, got green/roasted costs %.2f/%.2f", item.GreenBeanCostPerKg, item.RoastedBeanCostPerKg)
	}
	if item.GreenBeanList.Code == "" || item.GreenBeanList.DisplayName != "埃塞瑰夏生豆" {
		t.Fatalf("green bean list metadata = %+v", item.GreenBeanList)
	}
	if len(item.GreenBeanSaleTiers) != 1 || item.GreenBeanSaleTiers[0].PricePerUnit != 128 {
		t.Fatalf("green bean sale tiers = %+v", item.GreenBeanSaleTiers)
	}
}

func TestBeanListAppliesProductMarginOverrideBeforeCategoryTemplateMargin(t *testing.T) {
	input := domain.ProductInput{
		ProductID:          501,
		Name:               "模板拼配",
		GreenBeanCostPerKg: 51.75,
		YieldRate:          0.8,
		GradientTemplate: &domain.GradientTemplate{
			ID:          9,
			Name:        "工厂量单模板",
			DisplayUnit: domain.GradientDisplayUnitKg,
			Tiers: []domain.GradientTemplateTier{{
				ID: 91, Label: "大客户量单", MinWeightG: 24000, MaxWeightG: floatPtr(49000), MarginRate: 0.175, Position: 1,
			}},
		},
	}
	setDomainProductInputFloat64PtrField(t, &input, "MarginRateOverride", 0.30)
	svc := NewService(&fakeRepo{inputs: []domain.ProductInput{input}})

	resp, err := svc.BeanList(context.Background())
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}
	if len(resp.Items) != 1 || len(resp.Items[0].CommercialWholesaleTiers) != 1 {
		t.Fatalf("bean list items = %+v", resp.Items)
	}
	tier := resp.Items[0].CommercialWholesaleTiers[0]
	if tier.MarginRate != 0.30 || tier.PricePerUnit != 91 {
		t.Fatalf("tier should use product margin override before category template margin, got %+v", tier)
	}
}

func TestPublishRunRequiresPositiveID(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if err := svc.PublishRun(context.Background(), "JJ", 0); err == nil {
		t.Fatalf("expected invalid id error")
	}
}

func TestSaveDripPriceTemplatePreservesOmittedBooleanFields(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.SaveDripPriceTemplate(context.Background(), SaveDripPriceTemplateCommand{
		Name:        "挂耳供应价",
		BagGrams:    10,
		BoxBagCount: 10,
		Tiers: []SaveDripPriceTemplateTierRow{{
			Label:      "100袋",
			MinBags:    100,
			Multiplier: 2.2,
		}},
	})
	if err != nil {
		t.Fatalf("SaveDripPriceTemplate() error = %v", err)
	}
	if repo.savedDripTemplate.Active != nil {
		t.Fatalf("active should stay nil when omitted, got %v", *repo.savedDripTemplate.Active)
	}
	if repo.savedDripTemplate.IncludePackaging != nil {
		t.Fatalf("include_packaging should stay nil when omitted, got %v", *repo.savedDripTemplate.IncludePackaging)
	}
}

func setDomainProductInputFloat64PtrField(t *testing.T, target any, fieldName string, value float64) {
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

func TestPublishBeanListValidatesVersionAndListType(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if _, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial"}); err == nil {
		t.Fatalf("expected version required")
	}
	row, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{ListType: "commercial", Version: "V3.0.5"})
	if err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}
	if row.Status != "published" {
		t.Fatalf("row = %+v", row)
	}
	if _, err := svc.ListBeanListPublications(context.Background(), BeanListPublicationQuery{ListType: "bad"}); err == nil {
		t.Fatalf("expected invalid list type")
	}
	if _, err := svc.SaveBeanListDraft(context.Background(), PublishBeanListCommand{ListType: "green", Version: "VGREEN-1"}); err != nil {
		t.Fatalf("green bean list type should be publishable: %v", err)
	}
	if err := svc.WithdrawBeanList(context.Background(), WithdrawBeanListCommand{}); err == nil {
		t.Fatalf("expected invalid id")
	}
}

func TestPublishBeanListKeepsCustomerSnapshotOwnerAndSources(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	row, err := svc.PublishBeanList(context.Background(), PublishBeanListCommand{
		ListType:                 "commercial",
		Version:                  "V3.0.7",
		OwnerType:                "actor",
		OwnerKey:                 "employee:7",
		PriceSourcePublicationID: 11,
		StyleSourcePublicationID: 5,
		Content: map[string]any{
			"totalItems": float64(25),
		},
	})
	if err != nil {
		t.Fatalf("PublishBeanList() error = %v", err)
	}
	if row.OwnerType != "actor" || row.OwnerKey != "employee:7" {
		t.Fatalf("row owner = %s/%s", row.OwnerType, row.OwnerKey)
	}
	if repo.publishedBeanList.OwnerType != "actor" || repo.publishedBeanList.OwnerKey != "employee:7" {
		t.Fatalf("cmd owner = %+v", repo.publishedBeanList)
	}
	if repo.publishedBeanList.PriceSourcePublicationID != 11 || repo.publishedBeanList.StyleSourcePublicationID != 5 {
		t.Fatalf("source ids = %+v", repo.publishedBeanList)
	}
}

func TestSaveBeanListDraftValidatesAndKeepsCustomerOwner(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	row, err := svc.SaveBeanListDraft(context.Background(), PublishBeanListCommand{
		ListType:  "retail",
		Version:   " V3.0.8 ",
		OwnerType: "actor",
		OwnerKey:  "employee:7",
		Config:    map[string]any{"layoutStyle": "card"},
		Content:   map[string]any{"totalItems": float64(1)},
	})
	if err != nil {
		t.Fatalf("SaveBeanListDraft() error = %v", err)
	}
	if row.Status != "draft" || row.ListType != "retail" || row.Version != "V3.0.8" {
		t.Fatalf("draft row = %+v", row)
	}
	if repo.draftBeanList.OwnerType != "actor" || repo.draftBeanList.OwnerKey != "employee:7" {
		t.Fatalf("draft owner = %+v", repo.draftBeanList)
	}
	if repo.draftBeanList.Config == nil || repo.draftBeanList.Content == nil {
		t.Fatalf("draft should normalize empty config/content maps: %+v", repo.draftBeanList)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
