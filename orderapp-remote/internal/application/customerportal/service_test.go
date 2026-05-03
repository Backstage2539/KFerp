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
	_, err := svc.GetServicePage(context.Background(), "mini-token", "directShip")
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
	got, err := svc.GetServicePage(context.Background(), "mini-token", ServiceKeyShipping)
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
