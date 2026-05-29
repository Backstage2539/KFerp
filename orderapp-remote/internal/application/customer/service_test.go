package customer

import (
	"context"
	"strings"
	"testing"
)

type fakeRepo struct {
	upsert             UpsertCommand
	asset              SaveAssetCommand
	listQuery          ListQuery
	customerTypeCreate CreateCustomerTypeCommand
	orderTypeCreate    CreateOrderTypeCommand
}

func (r *fakeRepo) Upsert(ctx context.Context, actor string, id *int64, cmd UpsertCommand) (int64, error) {
	r.upsert = cmd
	return 9, nil
}

func (r *fakeRepo) Prefs(ctx context.Context, id int64) (*Prefs, error) {
	return &Prefs{ID: id}, nil
}

func (r *fakeRepo) SaveAsset(ctx context.Context, cmd SaveAssetCommand) (SaveAssetResult, error) {
	r.asset = cmd
	return SaveAssetResult{CustomerID: cmd.CustomerID, ObjectKey: "obj", Bytes: 3, SHA256: "sha"}, nil
}

func (r *fakeRepo) DeleteAsset(ctx context.Context, actor string, assetID int64) (DeleteAssetResult, error) {
	return DeleteAssetResult{CustomerID: 1, ObjectKey: "obj"}, nil
}

func (r *fakeRepo) InlineUpdate(ctx context.Context, actor string, id int64, cmd InlineUpdateCommand) error {
	return nil
}

func (r *fakeRepo) Delete(ctx context.Context, actor string, id int64) error {
	return nil
}

func (r *fakeRepo) CreateOrderType(ctx context.Context, actor string, name string) (Option, error) {
	return Option{ID: 22, Name: name}, nil
}

func (r *fakeRepo) List(ctx context.Context, query ListQuery) (ListResult, error) {
	r.listQuery = query
	return ListResult{}, nil
}

func (r *fakeRepo) Editor(ctx context.Context, id int64) (*EditorData, error) {
	return &EditorData{}, nil
}

func (r *fakeRepo) AssetObject(ctx context.Context, assetID int64) (AssetObject, error) {
	return AssetObject{}, nil
}

func (r *fakeRepo) ListCustomerTypeOptions(ctx context.Context) ([]CustomerTypeOption, error) {
	return DefaultCustomerTypeOptions(), nil
}

func (r *fakeRepo) CreateCustomerTypeOption(ctx context.Context, actor string, cmd CreateCustomerTypeCommand) (CustomerTypeOption, error) {
	r.customerTypeCreate = cmd
	return CustomerTypeOption{Value: "partner_store", Label: cmd.Label}, nil
}

func (r *fakeRepo) CreateOrderTypeOption(ctx context.Context, actor string, cmd CreateOrderTypeCommand) (Option, error) {
	r.orderTypeCreate = cmd
	return Option{ID: 88, Name: cmd.Name}, nil
}

func TestServiceDelegatesCustomerUpsert(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	id, err := svc.Upsert(context.Background(), "actor", nil, UpsertCommand{
		Name:               "Ada",
		CustomerType:       "wholesale",
		DefaultSourceID:    "1",
		DefaultOrderTypeID: "2",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if id != 9 || repo.upsert.Name != "Ada" {
		t.Fatalf("Upsert result/id = %d command = %+v", id, repo.upsert)
	}
}

func TestServiceDelegatesAssetSave(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	res, err := svc.SaveAsset(context.Background(), SaveAssetCommand{
		CustomerID:  4,
		Kind:        "logo",
		Reader:      strings.NewReader("abc"),
		ContentType: "image/png",
		Filename:    "logo.png",
	})
	if err != nil {
		t.Fatalf("SaveAsset() error = %v", err)
	}
	if res.CustomerID != 4 || repo.asset.Kind != "logo" {
		t.Fatalf("SaveAsset result = %+v command = %+v", res, repo.asset)
	}
}

func TestServiceListNormalizesQuery(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	active := true
	_, err := svc.List(context.Background(), ListQuery{
		Query:         "rock",
		CustomerType:  "bad-type",
		SortBy:        "weird",
		SortDirection: "weird",
		Limit:         0,
		Offset:        -7,
		Active:        &active,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.listQuery.Query != "rock" {
		t.Fatalf("list query = %q, want rock", repo.listQuery.Query)
	}
	if repo.listQuery.CustomerType != "bad-type" {
		t.Fatalf("normalized customer_type = %q, want bad-type", repo.listQuery.CustomerType)
	}
	if repo.listQuery.SortBy != "name" {
		t.Fatalf("normalized sort_by = %q, want name", repo.listQuery.SortBy)
	}
	if repo.listQuery.SortDirection != "asc" {
		t.Fatalf("normalized sort_direction = %q, want asc", repo.listQuery.SortDirection)
	}
	if repo.listQuery.Limit != 10 {
		t.Fatalf("normalized limit = %d, want 10", repo.listQuery.Limit)
	}
	if repo.listQuery.Offset != 0 {
		t.Fatalf("normalized offset = %d, want 0", repo.listQuery.Offset)
	}
	if repo.listQuery.Active == nil || *repo.listQuery.Active != true {
		t.Fatalf("normalized active = %+v, want true", repo.listQuery.Active)
	}
}

func TestServiceSupportsChannelCustomerType(t *testing.T) {
	if got := NormalizeCustomerType("channel"); got != "channel" {
		t.Fatalf("NormalizeCustomerType(channel)=%q, want channel", got)
	}
	if got := NormalizeCustomerTypeFilter("channel"); got != "channel" {
		t.Fatalf("NormalizeCustomerTypeFilter(channel)=%q, want channel", got)
	}
}

func TestServiceSupportsCustomCustomerTypeOptions(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	_, err := svc.Upsert(context.Background(), "actor", nil, UpsertCommand{
		Name:               "联名门店A",
		CustomerType:       "partner_store",
		DefaultSourceID:    "1",
		DefaultOrderTypeID: "2",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v, want custom customer type accepted", err)
	}
	if repo.upsert.CustomerType != "partner_store" {
		t.Fatalf("customer_type=%q, want partner_store", repo.upsert.CustomerType)
	}
	if got := NormalizeCustomerTypeFilter("partner_store"); got != "partner_store" {
		t.Fatalf("custom filter = %q, want partner_store", got)
	}
}

func TestServiceCreatesCustomerAndOrderTypeOptions(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	customerType, err := svc.CreateCustomerTypeOption(context.Background(), "actor", CreateCustomerTypeCommand{Label: " 联名门店 "})
	if err != nil {
		t.Fatalf("CreateCustomerTypeOption() error = %v", err)
	}
	if customerType.Value != "partner_store" || repo.customerTypeCreate.Label != "联名门店" {
		t.Fatalf("customer type option = %+v command=%+v", customerType, repo.customerTypeCreate)
	}
	orderType, err := svc.CreateOrderTypeOption(context.Background(), "actor", CreateOrderTypeCommand{Name: " 补货订单 "})
	if err != nil {
		t.Fatalf("CreateOrderTypeOption() error = %v", err)
	}
	if orderType.ID != 88 || orderType.Name != "补货订单" || repo.orderTypeCreate.Name != "补货订单" {
		t.Fatalf("order type option = %+v command=%+v", orderType, repo.orderTypeCreate)
	}
}

func TestServiceAllowsPortalSwitchWithoutCapabilityTemplate(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	enabled := true
	_, err := svc.Upsert(context.Background(), "actor", nil, UpsertCommand{
		Name:                  "渠道客户",
		CustomerType:          "channel",
		DefaultSourceID:       "1",
		DefaultOrderTypeID:    "2",
		PortalEnabled:         &enabled,
		CapabilityTemplateKey: "channel_direct_ship",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v, want portal switch without template accepted", err)
	}
	if repo.upsert.PortalEnabled == nil || !*repo.upsert.PortalEnabled {
		t.Fatalf("portal_enabled not delegated: %+v", repo.upsert)
	}
	if repo.upsert.CapabilityTemplateKey != "" {
		t.Fatalf("capability_template_key=%q, want customer profile not to bind templates", repo.upsert.CapabilityTemplateKey)
	}
}

func TestServiceListRespectsExplicitSort(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	_, err := svc.List(context.Background(), ListQuery{
		SortBy:        "updated",
		SortDirection: "desc",
		Limit:         15,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.listQuery.SortBy != "updated" {
		t.Fatalf("sort_by = %q, want updated", repo.listQuery.SortBy)
	}
	if repo.listQuery.SortDirection != "desc" {
		t.Fatalf("sort_direction = %q, want desc", repo.listQuery.SortDirection)
	}
	if repo.listQuery.Limit != 15 {
		t.Fatalf("limit = %d, want 15", repo.listQuery.Limit)
	}
}
