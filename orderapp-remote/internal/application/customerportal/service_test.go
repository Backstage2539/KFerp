package customerportal

import (
	"context"
	"errors"
	"testing"
)

type fakeIdentityProvider struct {
	identity MiniIdentity
	err      error
}

func (p fakeIdentityProvider) Resolve(ctx context.Context, code string) (MiniIdentity, error) {
	if p.err != nil {
		return MiniIdentity{}, p.err
	}
	return p.identity, nil
}

type fakeRepository struct {
	loginResult       LoginResult
	loginCommand      CreateLoginSessionCommand
	context           CurrentContext
	session           string
	serviceQuery      ServicePageQuery
	servicePage       ServicePage
	beanList          BeanListSummary
	portalCustomers   []PortalAdminCustomer
	portalDetail      PortalAdminDetail
	visibilityCommand UpdatePortalVisibilityCommand
	directShipCommand CreateDirectShipBatchCommand
	directShipBatch   DirectShipBatch
	processingCommand CreateProcessingRequestCommand
	processingRequest ProcessingRequest
	err               error
	switchErr         error
}

func (r *fakeRepository) CreateLoginSession(ctx context.Context, cmd CreateLoginSessionCommand) (LoginResult, error) {
	r.loginCommand = cmd
	if r.err != nil {
		return LoginResult{}, r.err
	}
	return r.loginResult, nil
}

func (r *fakeRepository) CurrentContextByToken(ctx context.Context, token string) (CurrentContext, error) {
	r.session = token
	if r.err != nil {
		return CurrentContext{}, r.err
	}
	return r.context, nil
}

func (r *fakeRepository) SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error) {
	r.session = token
	if r.switchErr != nil {
		return CurrentContext{}, r.switchErr
	}
	r.context.CurrentCustomerID = customerID
	return r.context, nil
}

func (r *fakeRepository) LoadServicePage(ctx context.Context, query ServicePageQuery) (ServicePage, error) {
	r.serviceQuery = query
	if r.err != nil {
		return ServicePage{}, r.err
	}
	return r.servicePage, nil
}

func (r *fakeRepository) LoadBeanListPublication(ctx context.Context, customerID, publicationID int64) (BeanListSummary, error) {
	if r.err != nil {
		return BeanListSummary{}, r.err
	}
	return r.beanList, nil
}

func (r *fakeRepository) ListPortalAdminCustomers(ctx context.Context, query PortalAdminCustomerQuery) ([]PortalAdminCustomer, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.portalCustomers, nil
}

func (r *fakeRepository) PortalAdminDetail(ctx context.Context, customerID int64) (PortalAdminDetail, error) {
	if r.err != nil {
		return PortalAdminDetail{}, r.err
	}
	r.portalDetail.Customer.ID = customerID
	return r.portalDetail, nil
}

func (r *fakeRepository) UpdatePortalVisibility(ctx context.Context, cmd UpdatePortalVisibilityCommand) (PortalAdminDetail, error) {
	r.visibilityCommand = cmd
	if r.err != nil {
		return PortalAdminDetail{}, r.err
	}
	r.portalDetail.Customer.ID = cmd.CustomerID
	r.portalDetail.Customer.DisplayName = cmd.DisplayName
	r.portalDetail.Customer.PortalEnabled = cmd.Enabled
	return r.portalDetail, nil
}

func (r *fakeRepository) CreateDirectShipBatch(ctx context.Context, cmd CreateDirectShipBatchCommand) (DirectShipBatch, error) {
	r.directShipCommand = cmd
	if r.err != nil {
		return DirectShipBatch{}, r.err
	}
	return r.directShipBatch, nil
}

func (r *fakeRepository) CreateProcessingRequest(ctx context.Context, cmd CreateProcessingRequestCommand) (ProcessingRequest, error) {
	r.processingCommand = cmd
	if r.err != nil {
		return ProcessingRequest{}, r.err
	}
	return r.processingRequest, nil
}

func TestLoginRejectsEmptyCode(t *testing.T) {
	svc := NewService(&fakeRepository{}, fakeIdentityProvider{})
	_, err := svc.Login(context.Background(), LoginCommand{})
	if err == nil || err.Error() != "code required" {
		t.Fatalf("Login() err=%v, want code required", err)
	}
}

func TestLoginCreatesSessionFromResolvedIdentity(t *testing.T) {
	repo := &fakeRepository{loginResult: LoginResult{Token: "mini-token", MiniUserID: 9}}
	svc := NewService(repo, fakeIdentityProvider{identity: MiniIdentity{OpenID: " openid-1 ", UnionID: " union-1 "}})
	got, err := svc.Login(context.Background(), LoginCommand{Code: "wx-code", Phone: " 13800138000 ", Nickname: " 客户 "})
	if err != nil {
		t.Fatalf("Login() err=%v", err)
	}
	if got.Token != "mini-token" || got.MiniUserID != 9 {
		t.Fatalf("Login()=%+v", got)
	}
	if repo.loginCommand.OpenID != "openid-1" || repo.loginCommand.UnionID != "union-1" || repo.loginCommand.Phone != "13800138000" || repo.loginCommand.Nickname != "客户" {
		t.Fatalf("CreateLoginSession() cmd=%+v", repo.loginCommand)
	}
}

func TestMeRequiresTokenAndReturnsBoundCapabilities(t *testing.T) {
	repo := &fakeRepository{context: CurrentContext{
		MiniUserID:          8,
		CurrentCustomerID:   7,
		CurrentCustomerName: "品牌客户",
		Bindings:            []CustomerBinding{{CustomerID: 7, CustomerName: "品牌客户", Role: "owner", Status: "approved"}},
		Capabilities:        []Capability{{Code: CapabilityDirectShip, Enabled: true}, {Code: CapabilitySettlement, Enabled: true}},
	}}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.Me(context.Background(), "mini-token")
	if err != nil {
		t.Fatalf("Me() err=%v", err)
	}
	if got.CurrentCustomerID != 7 || !got.HasCapability(CapabilityDirectShip) || !got.HasCapability(CapabilitySettlement) {
		t.Fatalf("Me()=%+v", got)
	}
}

func TestSwitchCustomerRejectsUnauthorizedBinding(t *testing.T) {
	repo := &fakeRepository{switchErr: ErrCustomerBindingNotFound}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.SwitchCurrentCustomer(context.Background(), "mini-token", 99)
	if !errors.Is(err, ErrCustomerBindingNotFound) {
		t.Fatalf("SwitchCurrentCustomer() err=%v", err)
	}
}

func TestServicePreservesMiniSessionNotFoundSentinel(t *testing.T) {
	repo := &fakeRepository{err: ErrMiniSessionNotFound}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.Me(context.Background(), "missing-token")
	if !errors.Is(err, ErrMiniSessionNotFound) {
		t.Fatalf("Me() err=%v, want ErrMiniSessionNotFound", err)
	}
}

func TestGetServicePageRequiresEnabledCapability(t *testing.T) {
	repo := &fakeRepository{context: CurrentContext{
		CurrentCustomerID:   7,
		CurrentCustomerName: "客户A",
		Capabilities:        []Capability{{Code: CapabilityDirectShip, Enabled: false}},
	}}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.GetServicePage(context.Background(), "mini-token", "directShip", ServicePageFilter{})
	if !errors.Is(err, ErrCapabilityNotEnabled) {
		t.Fatalf("GetServicePage() err=%v, want ErrCapabilityNotEnabled", err)
	}
}

func TestGetServicePageLoadsCustomerScopedData(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			CurrentCustomerID:   7,
			CurrentCustomerName: "客户A",
			Capabilities:        []Capability{{Code: CapabilityShippingQuery, Enabled: true}},
		},
		servicePage: ServicePage{
			Key:    ServiceKeyShipping,
			Title:  "物流查询",
			Orders: []CustomerOrderSummary{{OrderNo: "SO-1", ShipTrackingNo: "SF123"}},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.GetServicePage(context.Background(), "mini-token", ServiceKeyShipping, ServicePageFilter{})
	if err != nil {
		t.Fatalf("GetServicePage() err=%v", err)
	}
	if repo.session != "mini-token" || repo.serviceQuery.CustomerID != 7 || repo.serviceQuery.Key != ServiceKeyShipping || repo.serviceQuery.Limit != 20 {
		t.Fatalf("service query=%+v session=%q", repo.serviceQuery, repo.session)
	}
	if got.CurrentCustomerID != 7 || len(got.Orders) != 1 || got.Orders[0].ShipTrackingNo != "SF123" {
		t.Fatalf("GetServicePage()=%+v", got)
	}
}

func TestGetServicePageNormalizesOrderSearchFilters(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			CurrentCustomerID:   7,
			CurrentCustomerName: "客户A",
			Capabilities:        []Capability{{Code: CapabilityProductOrder, Enabled: true}},
		},
		servicePage: ServicePage{
			Key:    ServiceKeyOrders,
			Orders: []CustomerOrderSummary{{OrderNo: "SO-1"}},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.GetServicePage(context.Background(), "mini-token", ServiceKeyOrders, ServicePageFilter{
		Query:    "  乌拉嘎 上海 张三  ",
		DateFrom: "2026-05-01",
		DateTo:   "2026-05-03",
	})
	if err != nil {
		t.Fatalf("GetServicePage() err=%v", err)
	}
	if repo.serviceQuery.Query != "乌拉嘎 上海 张三" || repo.serviceQuery.DateFrom != "2026-05-01" || repo.serviceQuery.DateTo != "2026-05-03" {
		t.Fatalf("service query filters=%+v", repo.serviceQuery)
	}
	if repo.serviceQuery.Limit != 50 {
		t.Fatalf("orders limit=%d, want 50 for searchable history", repo.serviceQuery.Limit)
	}
}

func TestGetOrdersServicePageAllowsAnyOrderRelatedCapability(t *testing.T) {
	for _, capability := range []string{CapabilityProductOrder, CapabilityDirectShip, CapabilityShippingQuery} {
		repo := &fakeRepository{
			context: CurrentContext{
				CurrentCustomerID:   7,
				CurrentCustomerName: "客户A",
				Capabilities:        []Capability{{Code: capability, Enabled: true}},
			},
			servicePage: ServicePage{
				Key:    ServiceKeyOrders,
				Orders: []CustomerOrderSummary{{OrderNo: "SO-ORDER", GrandTotal: "128.00"}},
			},
		}
		svc := NewService(repo, fakeIdentityProvider{})
		got, err := svc.GetServicePage(context.Background(), "mini-token", ServiceKeyOrders, ServicePageFilter{})
		if err != nil {
			t.Fatalf("GetServicePage(orders) with %s err=%v", capability, err)
		}
		if repo.serviceQuery.Key != ServiceKeyOrders || got.Title != "我的订单" || got.Capability != CapabilityProductOrder || len(got.Orders) != 1 {
			t.Fatalf("capability=%s query=%+v page=%+v", capability, repo.serviceQuery, got)
		}
	}
}

func TestCreateDirectShipBatchRequiresCapabilityAndTrimsInput(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			CurrentCustomerID: 7,
			Capabilities:      []Capability{{Code: CapabilityDirectShip, Enabled: true}},
		},
		directShipBatch: DirectShipBatch{ID: 3, BatchNo: "DS-20260503-0003", Status: "submitted", TotalRows: 100},
	}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.CreateDirectShipBatch(context.Background(), "mini-token", CreateDirectShipBatchCommand{
		SourceName: "  5月直播订单  ",
		TotalRows:  100,
		Note:       "  客户一次发来100单  ",
	})
	if err != nil {
		t.Fatalf("CreateDirectShipBatch() err=%v", err)
	}
	if got.BatchNo != "DS-20260503-0003" || repo.directShipCommand.CustomerID != 7 || repo.directShipCommand.SourceName != "5月直播订单" || repo.directShipCommand.Note != "客户一次发来100单" {
		t.Fatalf("batch=%+v command=%+v", got, repo.directShipCommand)
	}
}

func TestCreateProcessingRequestValidatesAndRequiresCapability(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			CurrentCustomerID: 7,
			Capabilities:      []Capability{{Code: CapabilityProcessing, Enabled: true}},
		},
		processingRequest: ProcessingRequest{ID: 9, RequestNo: "PJ-20260503-0009", Status: "submitted"},
	}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.CreateProcessingRequest(context.Background(), "mini-token", CreateProcessingRequestCommand{
		InputMaterialID: 4,
		InputQtyG:       30000,
		TargetProductID: 5,
		TargetSpecG:     454,
		TargetQty:       50,
		Note:            "  做成454g  ",
	})
	if err != nil {
		t.Fatalf("CreateProcessingRequest() err=%v", err)
	}
	if got.RequestNo != "PJ-20260503-0009" || repo.processingCommand.CustomerID != 7 || repo.processingCommand.Note != "做成454g" {
		t.Fatalf("request=%+v command=%+v", got, repo.processingCommand)
	}
}

func TestPortalAdminDetailAlwaysReturnsCompleteCapabilityCatalog(t *testing.T) {
	repo := &fakeRepository{portalDetail: PortalAdminDetail{
		Customer: PortalAdminCustomer{ID: 147, Name: "13800138075", DisplayName: "测试客户", PortalEnabled: true},
		Capabilities: []CapabilityOption{
			{Code: CapabilityDirectShip, Enabled: true},
			{Code: CapabilitySettlement, Enabled: false},
		},
	}}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.PortalAdminDetail(context.Background(), 147)
	if err != nil {
		t.Fatalf("PortalAdminDetail() err=%v", err)
	}
	if got.Customer.ID != 147 || got.Customer.DisplayName != "测试客户" {
		t.Fatalf("detail customer=%+v", got.Customer)
	}
	if len(got.Capabilities) != len(DefaultCapabilityOptions()) {
		t.Fatalf("capabilities=%+v, want complete catalog", got.Capabilities)
	}
	for _, code := range []string{CapabilityBeanList, CapabilityProductOrder, CapabilityDirectShip, CapabilityProcessing, CapabilityInventoryCustody, CapabilityShippingQuery, CapabilitySettlement} {
		if !got.HasCapabilityOption(code) {
			t.Fatalf("capability catalog missing %s: %+v", code, got.Capabilities)
		}
	}
}

func TestUpdatePortalVisibilityTrimsAndNormalizesCapabilities(t *testing.T) {
	repo := &fakeRepository{portalDetail: PortalAdminDetail{
		Customer: PortalAdminCustomer{Name: "13800138075"},
	}}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.UpdatePortalVisibility(context.Background(), UpdatePortalVisibilityCommand{
		CustomerID:   147,
		DisplayName:  "  测试客户  ",
		Enabled:      true,
		Capabilities: []CapabilityOption{{Code: CapabilityDirectShip, Enabled: true}, {Code: "unknown", Enabled: true}, {Code: CapabilityBeanList, Enabled: false}},
	})
	if err != nil {
		t.Fatalf("UpdatePortalVisibility() err=%v", err)
	}
	if repo.visibilityCommand.CustomerID != 147 || repo.visibilityCommand.DisplayName != "测试客户" || !repo.visibilityCommand.Enabled {
		t.Fatalf("visibility command=%+v", repo.visibilityCommand)
	}
	if len(repo.visibilityCommand.Capabilities) != 2 {
		t.Fatalf("visibility capabilities=%+v, want only known codes from payload", repo.visibilityCommand.Capabilities)
	}
	if repo.visibilityCommand.Capabilities[0].Code != CapabilityBeanList || repo.visibilityCommand.Capabilities[1].Code != CapabilityDirectShip {
		t.Fatalf("capabilities sorted/normalized incorrectly: %+v", repo.visibilityCommand.Capabilities)
	}
}
