package customerportal

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeIdentityProvider struct {
	identity MiniIdentity
	phone    MiniPhoneNumber
	err      error
}

func (p fakeIdentityProvider) Resolve(ctx context.Context, code string) (MiniIdentity, error) {
	if p.err != nil {
		return MiniIdentity{}, p.err
	}
	return p.identity, nil
}

func (p fakeIdentityProvider) ResolvePhoneNumber(ctx context.Context, code string) (MiniPhoneNumber, error) {
	if p.err != nil {
		return MiniPhoneNumber{}, p.err
	}
	return p.phone, nil
}

type fakeRepository struct {
	loginResult              LoginResult
	loginCommand             CreateLoginSessionCommand
	phoneVerifiedLoginResult LoginResult
	phoneVerifiedCommand     CreatePhoneVerifiedLoginSessionCommand
	passwordCommand          CreatePasswordLoginSessionCommand
	context                  CurrentContext
	session                  string
	serviceQuery             ServicePageQuery
	servicePage              ServicePage
	beanList                 BeanListSummary
	portalCustomers          []PortalAdminCustomer
	portalDetail             PortalAdminDetail
	visibilityCommand        UpdatePortalVisibilityCommand
	templates                []CapabilityTemplate
	templateSaveCommand      SaveCapabilityTemplateCommand
	templateCommand          ApplyCapabilityTemplateCommand
	erpBindingCommand        UpsertPortalERPBindingCommand
	mallProducts             []MallProduct
	mallProductOptions       []MallProductOption
	mallProductCommand       SaveMallProductCommand
	mallImageCommand         UpdateMallProductImageCommand
	mallPage                 MallPage
	mallOrderCommand         CreateMallOrderCommand
	mallOrder                FulfillmentOrder
	directShipCommand        CreateDirectShipBatchCommand
	directShipBatch          DirectShipBatch
	processingCommand        CreateProcessingRequestCommand
	processingRequest        ProcessingRequest
	fulfillmentCommand       CreateFulfillmentOrderCommand
	fulfillmentOrder         FulfillmentOrder
	orderAccessCustomer      int64
	orderAccessOrder         int64
	orderAccessOK            bool
	err                      error
	switchErr                error
	phoneVerifiedErr         error
}

func (r *fakeRepository) CreateLoginSession(ctx context.Context, cmd CreateLoginSessionCommand) (LoginResult, error) {
	r.loginCommand = cmd
	if r.err != nil {
		return LoginResult{}, r.err
	}
	return r.loginResult, nil
}

func (r *fakeRepository) CreatePhoneVerifiedLoginSession(ctx context.Context, cmd CreatePhoneVerifiedLoginSessionCommand) (LoginResult, error) {
	r.phoneVerifiedCommand = cmd
	if r.phoneVerifiedErr != nil {
		return LoginResult{}, r.phoneVerifiedErr
	}
	return r.phoneVerifiedLoginResult, nil
}

func (r *fakeRepository) CreatePasswordLoginSession(ctx context.Context, cmd CreatePasswordLoginSessionCommand) (LoginResult, error) {
	r.passwordCommand = cmd
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

func (r *fakeRepository) LoadBeanListPublicationAsset(ctx context.Context, publicationID int64, assetType string) (BeanListPublicationAsset, error) {
	return BeanListPublicationAsset{}, ErrBeanListPublicationNotFound
}

func (r *fakeRepository) SaveBeanListPublicationAsset(ctx context.Context, asset BeanListPublicationAsset, actor string) (BeanListPublicationAsset, error) {
	return asset, nil
}

func (r *fakeRepository) AcknowledgeBeanListPublication(ctx context.Context, customerID, publicationID int64, actor string) error {
	return nil
}

func TestBeanListDiffDetectsAddedRemovedAndChangedItems(t *testing.T) {
	oldList := BeanListSummary{Groups: []BeanListGroupSummary{{
		Category: "经典拼配",
		Items: []BeanListProductSummary{{
			Code: "1.1", Name: "曲奇拼配", Flavor: "坚果", Description: "均衡",
			Prices: []BeanListPriceSummary{{Label: "454g", Value: "88/包"}},
		}, {
			Code: "1.2", Name: "旧豆", Flavor: "可可",
			Prices: []BeanListPriceSummary{{Label: "454g", Value: "78/包"}},
		}},
	}}}
	newList := BeanListSummary{Groups: []BeanListGroupSummary{{
		Category: "经典拼配",
		Items: []BeanListProductSummary{{
			Code: "1.1", Name: "曲奇拼配", Flavor: "坚果/黄油", Description: "均衡",
			Prices: []BeanListPriceSummary{{Label: "454g", Value: "92/包"}},
		}, {
			Code: "1.3", Name: "新豆", Flavor: "柑橘",
			Prices: []BeanListPriceSummary{{Label: "454g", Value: "98/包"}},
		}},
	}}}

	diff := BeanListDiffBetween(oldList, newList)

	if len(diff.Added) != 1 || diff.Added[0].Name != "新豆" {
		t.Fatalf("added=%+v, want 新豆", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Name != "旧豆" {
		t.Fatalf("removed=%+v, want 旧豆", diff.Removed)
	}
	if len(diff.Changed) != 1 || diff.Changed[0].Code != "1.1" {
		t.Fatalf("changed=%+v, want 曲奇拼配", diff.Changed)
	}
	if !diff.Changed[0].HasField("prices") || !diff.Changed[0].HasField("flavor") {
		t.Fatalf("changed fields=%+v, want prices and flavor", diff.Changed[0].Fields)
	}
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
	r.portalDetail.Customer.MiniappEntryMode = cmd.MiniappEntryMode
	return r.portalDetail, nil
}

func (r *fakeRepository) ListCapabilityTemplates(ctx context.Context) ([]CapabilityTemplate, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.templates, nil
}

func (r *fakeRepository) SaveCapabilityTemplate(ctx context.Context, cmd SaveCapabilityTemplateCommand) (CapabilityTemplate, error) {
	r.templateSaveCommand = cmd
	if r.err != nil {
		return CapabilityTemplate{}, r.err
	}
	return cmd.Template, nil
}

func (r *fakeRepository) ApplyCapabilityTemplate(ctx context.Context, cmd ApplyCapabilityTemplateCommand) (PortalAdminDetail, error) {
	r.templateCommand = cmd
	if r.err != nil {
		return PortalAdminDetail{}, r.err
	}
	r.portalDetail.Customer.ID = cmd.CustomerID
	r.portalDetail.Customer.CapabilityTemplateKey = cmd.Template.Key
	r.portalDetail.Capabilities = cmd.Template.Capabilities
	return r.portalDetail, nil
}

func (r *fakeRepository) UpsertPortalERPBinding(ctx context.Context, cmd UpsertPortalERPBindingCommand) (PortalAdminDetail, error) {
	r.erpBindingCommand = cmd
	if r.err != nil {
		return PortalAdminDetail{}, r.err
	}
	r.portalDetail.Customer.ID = cmd.CustomerID
	r.portalDetail.Customer.ERPBinding = &PortalERPBinding{CustomerID: cmd.CustomerID, EmployeeID: cmd.EmployeeID, Status: cmd.Status}
	return r.portalDetail, nil
}

func (r *fakeRepository) ListMallProducts(ctx context.Context) ([]MallProduct, []MallProductOption, error) {
	if r.err != nil {
		return nil, nil, r.err
	}
	return r.mallProducts, r.mallProductOptions, nil
}

func (r *fakeRepository) SaveMallProduct(ctx context.Context, cmd SaveMallProductCommand) (MallProduct, error) {
	r.mallProductCommand = cmd
	if r.err != nil {
		return MallProduct{}, r.err
	}
	return MallProduct{
		ID:          9,
		ProductID:   cmd.ProductID,
		Title:       cmd.Title,
		Subtitle:    cmd.Subtitle,
		Description: cmd.Description,
		ImageURL:    cmd.ImageURL,
		SpecG:       cmd.SpecG,
		UnitPrice:   cmd.UnitPrice,
		TemplateKey: cmd.TemplateKey,
		Status:      cmd.Status,
		SortOrder:   cmd.SortOrder,
	}, nil
}

func (r *fakeRepository) UpdateMallProductImage(ctx context.Context, cmd UpdateMallProductImageCommand) (MallProduct, error) {
	r.mallImageCommand = cmd
	if r.err != nil {
		return MallProduct{}, r.err
	}
	return MallProduct{ID: cmd.ID, ImageURL: cmd.ImageURL}, nil
}

func (r *fakeRepository) LoadMallPage(ctx context.Context, customerID int64) (MallPage, error) {
	if r.err != nil {
		return MallPage{}, r.err
	}
	return r.mallPage, nil
}

func (r *fakeRepository) CreateMallOrder(ctx context.Context, cmd CreateMallOrderCommand) (FulfillmentOrder, error) {
	r.mallOrderCommand = cmd
	if r.err != nil {
		return FulfillmentOrder{}, r.err
	}
	return r.mallOrder, nil
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

func (r *fakeRepository) CreateFulfillmentOrder(ctx context.Context, cmd CreateFulfillmentOrderCommand) (FulfillmentOrder, error) {
	r.fulfillmentCommand = cmd
	if r.err != nil {
		return FulfillmentOrder{}, r.err
	}
	return r.fulfillmentOrder, nil
}

func (r *fakeRepository) CustomerOwnsOrder(ctx context.Context, customerID, orderID int64) (bool, error) {
	r.orderAccessCustomer = customerID
	r.orderAccessOrder = orderID
	if r.err != nil {
		return false, r.err
	}
	return r.orderAccessOK, nil
}

func TestLoginRejectsEmptyCode(t *testing.T) {
	svc := NewService(&fakeRepository{}, fakeIdentityProvider{})
	_, err := svc.Login(context.Background(), LoginCommand{})
	if err == nil || err.Error() != "code required" {
		t.Fatalf("Login() err=%v, want code required", err)
	}
}

func TestEnsureOrderAccessChecksCurrentCustomerOwnership(t *testing.T) {
	repo := &fakeRepository{
		context:       CurrentContext{MiniUserID: 9, CurrentCustomerID: 7},
		orderAccessOK: true,
	}
	svc := NewService(repo, fakeIdentityProvider{})

	if err := svc.EnsureOrderAccess(context.Background(), " mini-token ", 88); err != nil {
		t.Fatalf("EnsureOrderAccess() err=%v", err)
	}
	if repo.session != "mini-token" || repo.orderAccessCustomer != 7 || repo.orderAccessOrder != 88 {
		t.Fatalf("access session=%q customer=%d order=%d", repo.session, repo.orderAccessCustomer, repo.orderAccessOrder)
	}

	repo.orderAccessOK = false
	err := svc.EnsureOrderAccess(context.Background(), "mini-token", 99)
	if !errors.Is(err, ErrCustomerBindingNotFound) {
		t.Fatalf("EnsureOrderAccess() err=%v, want ErrCustomerBindingNotFound", err)
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

func TestLoginWithPhoneVerifyCreatesERPBoundSession(t *testing.T) {
	repo := &fakeRepository{phoneVerifiedLoginResult: LoginResult{Token: "mini-phone-token", MiniUserID: 11}}
	svc := NewService(repo, fakeIdentityProvider{
		identity: MiniIdentity{OpenID: " openid-verify ", UnionID: " union-verify "},
		phone:    MiniPhoneNumber{PhoneNumber: " 13800138000 ", PurePhoneNumber: "13800138000"},
	})
	got, err := svc.Login(context.Background(), LoginCommand{Mode: "phone_verify", Code: "wx-code", PhoneCode: " phone-code ", Nickname: " 渠道客户 "})
	if err != nil {
		t.Fatalf("Login() err=%v", err)
	}
	if got.Token != "mini-phone-token" || got.MiniUserID != 11 {
		t.Fatalf("Login()=%+v", got)
	}
	if repo.phoneVerifiedCommand.OpenID != "openid-verify" ||
		repo.phoneVerifiedCommand.UnionID != "union-verify" ||
		repo.phoneVerifiedCommand.Phone != "13800138000" ||
		repo.phoneVerifiedCommand.Nickname != "渠道客户" {
		t.Fatalf("CreatePhoneVerifiedLoginSession() cmd=%+v", repo.phoneVerifiedCommand)
	}
}

func TestLoginWithPasswordCreatesSessionForERPChannelAccount(t *testing.T) {
	repo := &fakeRepository{loginResult: LoginResult{
		Token:             "mini-token",
		MiniUserID:        11,
		CurrentCustomerID: 7,
		ThemeKey:          " premium_partner ",
		MiniappEntryMode:  " mall ",
	}}
	svc := NewService(repo, fakeIdentityProvider{})

	got, err := svc.LoginWithPassword(context.Background(), PasswordLoginCommand{Login: " 13800138075 ", Password: " secret "})
	if err != nil {
		t.Fatalf("LoginWithPassword() err=%v", err)
	}
	if got.Token != "mini-token" || got.MiniUserID != 11 || got.CurrentCustomerID != 7 {
		t.Fatalf("LoginWithPassword()=%+v", got)
	}
	if got.ThemeKey != PortalThemePremiumPartner || got.MiniappEntryMode != MiniappEntryModeMall {
		t.Fatalf("LoginWithPassword() theme=%q entry=%q, want normalized premium_partner/mall", got.ThemeKey, got.MiniappEntryMode)
	}
	if repo.passwordCommand.Login != "13800138075" || repo.passwordCommand.Password != "secret" {
		t.Fatalf("CreatePasswordLoginSession() cmd=%+v", repo.passwordCommand)
	}
}

func TestLoginWithPasswordRejectsMissingCredentials(t *testing.T) {
	svc := NewService(&fakeRepository{}, fakeIdentityProvider{})
	cases := []struct {
		name string
		cmd  PasswordLoginCommand
		want string
	}{
		{name: "login", cmd: PasswordLoginCommand{Password: "secret"}, want: "login required"},
		{name: "password", cmd: PasswordLoginCommand{Login: "13800138075"}, want: "password required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.LoginWithPassword(context.Background(), tc.cmd)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("LoginWithPassword() err=%v, want %s", err, tc.want)
			}
		})
	}
}

func TestMiniCustomerContextResponsesNormalizeThemeKey(t *testing.T) {
	repo := &fakeRepository{
		loginResult: LoginResult{Token: "mini-token", MiniUserID: 9},
		context:     CurrentContext{MiniUserID: 9, CurrentCustomerID: 7, ThemeKey: "  premium_partner  ", MiniappEntryMode: "  mall  "},
	}
	svc := NewService(repo, fakeIdentityProvider{identity: MiniIdentity{OpenID: "openid-1"}})

	login, err := svc.Login(context.Background(), LoginCommand{Code: "wx-code"})
	if err != nil {
		t.Fatalf("Login() err=%v", err)
	}
	if login.ThemeKey != PortalThemeCoffeeFactory {
		t.Fatalf("Login() theme_key=%q, want coffee_factory", login.ThemeKey)
	}

	me, err := svc.Me(context.Background(), "mini-token")
	if err != nil {
		t.Fatalf("Me() err=%v", err)
	}
	if me.ThemeKey != PortalThemePremiumPartner {
		t.Fatalf("Me() theme_key=%q, want premium_partner", me.ThemeKey)
	}
	if me.MiniappEntryMode != MiniappEntryModeMall {
		t.Fatalf("Me() miniapp_entry_mode=%q, want mall", me.MiniappEntryMode)
	}

	repo.context.ThemeKey = "unknown-theme"
	repo.context.MiniappEntryMode = "unknown-entry"
	switched, err := svc.SwitchCurrentCustomer(context.Background(), "mini-token", 8)
	if err != nil {
		t.Fatalf("SwitchCurrentCustomer() err=%v", err)
	}
	if switched.ThemeKey != PortalThemeCoffeeFactory {
		t.Fatalf("SwitchCurrentCustomer() theme_key=%q, want coffee_factory", switched.ThemeKey)
	}
	if switched.MiniappEntryMode != MiniappEntryModeServices {
		t.Fatalf("SwitchCurrentCustomer() miniapp_entry_mode=%q, want services", switched.MiniappEntryMode)
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
			ThemeKey:            PortalThemePremiumPartner,
			MiniappEntryMode:    MiniappEntryModeMall,
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
	if got.CurrentCustomerID != 7 || got.ThemeKey != PortalThemePremiumPartner || got.MiniappEntryMode != MiniappEntryModeMall ||
		len(got.Orders) != 1 || got.Orders[0].ShipTrackingNo != "SF123" {
		t.Fatalf("GetServicePage()=%+v", got)
	}
}

func TestGetSettlementServicePageSummaryCountsOrderBills(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			CurrentCustomerID:   152,
			CurrentCustomerName: "岩师傅",
			Capabilities:        []Capability{{Code: CapabilitySettlement, Enabled: true}},
		},
		servicePage: ServicePage{
			Key:    ServiceKeySettlement,
			Orders: []CustomerOrderSummary{{OrderNo: "SO-YAN-BILL", GrandTotal: "4559.00"}},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.GetServicePage(context.Background(), "mini-token", ServiceKeySettlement, ServicePageFilter{})
	if err != nil {
		t.Fatalf("GetServicePage(settlement) err=%v", err)
	}
	if len(got.Summary) < 3 || got.Summary[0].Label != "应收总额" || got.Summary[0].Value != "4559.00" ||
		got.Summary[2].Label != "未付款订单" || got.Summary[2].Value != "1" {
		t.Fatalf("settlement summary=%+v, want accounting bill totals", got.Summary)
	}
}

func TestGetSettlementServicePageSummaryShowsReceivableLedger(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			CurrentCustomerID:   152,
			CurrentCustomerName: "岩师傅",
			Capabilities:        []Capability{{Code: CapabilitySettlement, Enabled: true}},
		},
		servicePage: ServicePage{
			Key: ServiceKeySettlement,
			Orders: []CustomerOrderSummary{
				{OrderNo: "SO-UNPAID", GrandTotal: "2109.00", PayStatus: "未付款"},
				{OrderNo: "SO-PAID", GrandTotal: "128.50", PayStatus: "已付款"},
				{OrderNo: "SO-RECEIVED", GrandTotal: "20.50", PayStatus: "已收款"},
			},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.GetServicePage(context.Background(), "mini-token", ServiceKeySettlement, ServicePageFilter{})
	if err != nil {
		t.Fatalf("GetServicePage(settlement) err=%v", err)
	}
	if len(got.Summary) < 4 {
		t.Fatalf("settlement summary=%+v, want accounting metrics", got.Summary)
	}
	want := []ServiceMetric{
		{Label: "应收总额", Value: "2258.00"},
		{Label: "未付款金额", Value: "2109.00"},
		{Label: "未付款订单", Value: "1"},
		{Label: "已付款金额", Value: "149.00"},
	}
	for i, metric := range want {
		if got.Summary[i] != metric {
			t.Fatalf("summary[%d]=%+v, want %+v; full=%+v", i, got.Summary[i], metric, got.Summary)
		}
	}
}

func TestGetSettlementServicePageUsesLedgerLimitAndFilters(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			CurrentCustomerID:   152,
			CurrentCustomerName: "岩师傅",
			Capabilities:        []Capability{{Code: CapabilitySettlement, Enabled: true}},
		},
		servicePage: ServicePage{Key: ServiceKeySettlement},
	}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.GetServicePage(context.Background(), "mini-token", ServiceKeySettlement, ServicePageFilter{
		DateFrom:  "2026-05-01",
		DateTo:    "2026-05-31",
		PayStatus: "未付款",
	})
	if err != nil {
		t.Fatalf("GetServicePage(settlement) err=%v", err)
	}
	if repo.serviceQuery.Limit != 200 || repo.serviceQuery.DateFrom != "2026-05-01" || repo.serviceQuery.DateTo != "2026-05-31" || repo.serviceQuery.PayStatus != "未付款" {
		t.Fatalf("settlement query=%+v, want billing filters and ledger limit", repo.serviceQuery)
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
		Query:         "  乌拉嘎 上海 张三  ",
		DateFrom:      "2026-05-01",
		DateTo:        "2026-05-03",
		ProcessStatus: "  生产中  ",
		PayStatus:     "  已收款  ",
		ShipStatus:    "  待发货  ",
	})
	if err != nil {
		t.Fatalf("GetServicePage() err=%v", err)
	}
	if repo.serviceQuery.Query != "乌拉嘎 上海 张三" ||
		repo.serviceQuery.DateFrom != "2026-05-01" ||
		repo.serviceQuery.DateTo != "2026-05-03" ||
		repo.serviceQuery.ProcessStatus != "生产中" ||
		repo.serviceQuery.PayStatus != "已收款" ||
		repo.serviceQuery.ShipStatus != "待发货" {
		t.Fatalf("service query filters=%+v", repo.serviceQuery)
	}
	if repo.serviceQuery.Limit != 50 {
		t.Fatalf("orders limit=%d, want 50 for searchable history", repo.serviceQuery.Limit)
	}
}

func TestGetOrdersServicePageAllowsAnyOrderRelatedCapability(t *testing.T) {
	for _, capability := range []string{CapabilityProductOrder, CapabilityDirectShip, CapabilityShippingQuery, CapabilityMall} {
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

func TestCreateDirectShipBatchRejectsEmptyRows(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			CurrentCustomerID: 7,
			Capabilities:      []Capability{{Code: CapabilityDirectShip, Enabled: true}},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.CreateDirectShipBatch(context.Background(), "mini-token", CreateDirectShipBatchCommand{
		SourceName: "空批次",
		TotalRows:  0,
	})
	if err == nil || err.Error() != "total_rows invalid" {
		t.Fatalf("CreateDirectShipBatch empty rows err=%v, want total_rows invalid", err)
	}
	if repo.directShipCommand.CustomerID != 0 {
		t.Fatalf("empty direct ship batch should not reach repository: %+v", repo.directShipCommand)
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
		Customer: PortalAdminCustomer{ID: 147, Name: "13800138075", DisplayName: "测试客户", PortalEnabled: true, ProcessingWarehouseCode: "cust_147_processing", DefaultSenderID: 3},
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
	if got.Customer.ID != 147 || got.Customer.DisplayName != "测试客户" || got.Customer.ProcessingWarehouseCode != "cust_147_processing" || got.Customer.DefaultSenderID != 3 {
		t.Fatalf("detail customer=%+v", got.Customer)
	}
	if len(got.Capabilities) != len(DefaultCapabilityOptions()) {
		t.Fatalf("capabilities=%+v, want complete catalog", got.Capabilities)
	}
	for _, code := range []string{CapabilityBeanList, CapabilityProductOrder, CapabilityDirectShip, CapabilityProcessing, CapabilityInventoryCustody, CapabilitySettlement} {
		if !got.HasCapabilityOption(code) {
			t.Fatalf("capability catalog missing %s: %+v", code, got.Capabilities)
		}
	}
	if got.HasCapabilityOption(CapabilityShippingQuery) {
		t.Fatalf("capability catalog should not expose standalone logistics: %+v", got.Capabilities)
	}
}

func TestDefaultCapabilityTemplatesIncludePublicSKUDirectShipRules(t *testing.T) {
	templates := DefaultCapabilityTemplates()
	template, ok := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
	if !ok {
		t.Fatalf("public sku direct ship template missing from %+v", templates)
	}
	if template.MiniappEntryMode != MiniappEntryModeServices {
		t.Fatalf("miniapp entry mode=%q, want services", template.MiniappEntryMode)
	}
	if len(template.ERPRoleCodes) != 0 {
		t.Fatalf("erp roles=%+v, want no customer business roles in permission templates", template.ERPRoleCodes)
	}
	if !template.HasCapability(CapabilityDirectShip) || !template.HasCapability(CapabilityProductOrder) || template.HasCapability(CapabilityProcessing) {
		t.Fatalf("template capabilities not scoped to public sku direct ship: %+v", template.Capabilities)
	}
	rule := template.SmallBatchPriceRule()
	if !rule.Enabled || rule.ThresholdLB != 14 || rule.TierMinLB != 15 || rule.TierMaxLB != 28 {
		t.Fatalf("small batch rule=%+v, want <14lb uses 15-28lb tier", rule)
	}
}

func TestDefaultCapabilityTemplatesIncludeChannelDirectShipWorkbench(t *testing.T) {
	template, ok := CustomerCapabilityTemplateByKey("channel_direct_ship")
	if !ok {
		t.Fatal("missing channel direct ship capability template")
	}
	if template.Label != "渠道代发/现货下单" {
		t.Fatalf("channel template label=%q", template.Label)
	}
	if !template.ExposesERPWorkbench() {
		t.Fatalf("channel template should expose ERP customer workbench: %+v", template)
	}
	for _, code := range []string{CapabilityProductOrder, CapabilityDirectShip, CapabilitySettlement} {
		if !template.HasCapability(code) {
			t.Fatalf("channel template missing capability %s: %+v", code, template.Capabilities)
		}
	}
	rule := template.SmallBatchPriceRule()
	if !rule.Enabled {
		t.Fatalf("channel direct ship should keep small-batch price rule: %+v", rule)
	}
}

func TestDefaultCapabilityTemplatesIncludeRetailMallTemplate(t *testing.T) {
	template, ok := CustomerCapabilityTemplateByKey(CapabilityTemplateRetailMall)
	if !ok {
		t.Fatal("missing retail mall capability template")
	}
	if template.Label != "零售商城客户" {
		t.Fatalf("retail template label=%q", template.Label)
	}
	if template.MiniappEntryMode != MiniappEntryModeMall {
		t.Fatalf("retail template miniapp_entry_mode=%q, want mall", template.MiniappEntryMode)
	}
	if !template.HasCapability(CapabilityMall) {
		t.Fatalf("retail template capabilities=%+v, want mall enabled", template.Capabilities)
	}
	if template.HasCapability(CapabilityDirectShip) || template.HasCapability(CapabilityProcessing) {
		t.Fatalf("retail template should not enable wholesale fulfillment capabilities: %+v", template.Capabilities)
	}
	if got := NormalizeCapabilityTemplateKey(" retail_mall "); got != CapabilityTemplateRetailMall {
		t.Fatalf("NormalizeCapabilityTemplateKey(retail_mall)=%q", got)
	}
}

func TestOrdersServiceAllowsMallCapabilityForRetailOrderHistory(t *testing.T) {
	def, err := serviceDefinition(ServiceKeyOrders)
	if err != nil {
		t.Fatalf("serviceDefinition(orders): %v", err)
	}
	found := false
	for _, capability := range def.capabilities {
		if capability == CapabilityMall {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("orders service capabilities=%+v, want mall for retail order history", def.capabilities)
	}
}

func TestDefaultCapabilityTemplatesRuntimeBusinessContract(t *testing.T) {
	tests := []struct {
		templateKey       string
		entryMode         string
		themeKey          string
		erpPermissions    []string
		erpViewKeys       []string
		servicePages      map[string]bool
		mall              bool
		directShipBatch   bool
		directShipOrder   bool
		processingRequest bool
		processingOrder   bool
		productOrder      bool
	}{
		{
			templateKey:    CapabilityTemplateProcessingFulfillment,
			entryMode:      MiniappEntryModeServices,
			themeKey:       PortalThemeCoffeeFactory,
			erpPermissions: []string{"customer_processing.read", "customer_processing.submit"},
			erpViewKeys:    []string{"customerProcessingPortal"},
			servicePages: map[string]bool{
				ServiceKeyOrders:       true,
				ServiceKeyDirectShip:   true,
				ServiceKeyProcessing:   true,
				ServiceKeyInventory:    true,
				ServiceKeySettlement:   true,
				ServiceKeyProductOrder: false,
			},
			directShipBatch:   true,
			directShipOrder:   true,
			processingRequest: true,
			processingOrder:   true,
		},
		{
			templateKey:    CapabilityTemplatePublicSKUDirectShip,
			entryMode:      MiniappEntryModeServices,
			themeKey:       PortalThemeCleanOps,
			erpPermissions: []string{"customer_processing.read", "customer_processing.submit"},
			erpViewKeys:    []string{"customerProcessingPortal"},
			servicePages: map[string]bool{
				ServiceKeyOrders:       true,
				ServiceKeyDirectShip:   true,
				ServiceKeyProductOrder: true,
				ServiceKeySettlement:   true,
				ServiceKeyProcessing:   false,
				ServiceKeyInventory:    false,
			},
			directShipBatch: true,
			directShipOrder: true,
			productOrder:    true,
		},
		{
			templateKey: CapabilityTemplateRetailMall,
			entryMode:   MiniappEntryModeMall,
			themeKey:    PortalThemeCleanOps,
			servicePages: map[string]bool{
				ServiceKeyOrders:       true,
				ServiceKeyProductOrder: false,
				ServiceKeyDirectShip:   false,
				ServiceKeyProcessing:   false,
				ServiceKeyInventory:    false,
				ServiceKeySettlement:   false,
			},
			mall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.templateKey, func(t *testing.T) {
			template, ok := CustomerCapabilityTemplateByKey(tt.templateKey)
			if !ok {
				t.Fatalf("template %s missing", tt.templateKey)
			}
			if template.MiniappEntryMode != tt.entryMode || template.ThemeKey != tt.themeKey {
				t.Fatalf("template entry/theme=%q/%q, want %q/%q", template.MiniappEntryMode, template.ThemeKey, tt.entryMode, tt.themeKey)
			}
			assertStringSet(t, template.ERPPermissions, tt.erpPermissions, "erp permissions")
			assertStringSet(t, template.ERPViewKeys, tt.erpViewKeys, "erp view keys")
			if len(template.ERPRoleCodes) != 0 {
				t.Fatalf("template %s erp roles=%+v, want no customer business roles", tt.templateKey, template.ERPRoleCodes)
			}

			repo := &fakeRepository{
				context:           currentContextForTemplate(t, template),
				mallOrder:         FulfillmentOrder{OrderID: 101, OrderNo: "SO-MALL", PortalServiceCode: PortalServiceMall},
				directShipBatch:   DirectShipBatch{ID: 201, BatchNo: "DS-201", Status: "submitted"},
				processingRequest: ProcessingRequest{ID: 301, RequestNo: "PJ-301", Status: "submitted"},
				fulfillmentOrder:  FulfillmentOrder{OrderID: 401, OrderNo: "SO-FULFILLMENT"},
				servicePage:       ServicePage{Orders: []CustomerOrderSummary{{OrderNo: "SO-HISTORY"}}},
			}
			svc := NewService(repo, fakeIdentityProvider{})

			for serviceKey, allowed := range tt.servicePages {
				_, err := svc.GetServicePage(context.Background(), "mini-token", serviceKey, ServicePageFilter{})
				assertCapabilityAccess(t, tt.templateKey+" "+serviceKey, allowed, err)
			}

			_, err := svc.GetMallPage(context.Background(), "mini-token")
			assertCapabilityAccess(t, tt.templateKey+" mall page", tt.mall, err)
			_, err = svc.CreateMallOrder(context.Background(), "mini-token", validMallOrderCommand())
			assertCapabilityAccess(t, tt.templateKey+" mall order", tt.mall, err)
			_, err = svc.CreateDirectShipBatch(context.Background(), "mini-token", validDirectShipBatchCommand())
			assertCapabilityAccess(t, tt.templateKey+" direct ship batch", tt.directShipBatch, err)
			_, err = svc.CreateFulfillmentOrder(context.Background(), "mini-token", validFulfillmentOrderCommand(PortalServiceDirectShip))
			assertCapabilityAccess(t, tt.templateKey+" direct ship order", tt.directShipOrder, err)
			_, err = svc.CreateProcessingRequest(context.Background(), "mini-token", validProcessingRequestCommand())
			assertCapabilityAccess(t, tt.templateKey+" processing request", tt.processingRequest, err)
			_, err = svc.CreateFulfillmentOrder(context.Background(), "mini-token", validFulfillmentOrderCommand(PortalServiceProcessingShipment))
			assertCapabilityAccess(t, tt.templateKey+" processing order", tt.processingOrder, err)
			_, err = svc.CreateFulfillmentOrder(context.Background(), "mini-token", validFulfillmentOrderCommand(PortalServiceProductOrder))
			assertCapabilityAccess(t, tt.templateKey+" product order", tt.productOrder, err)
		})
	}
}

func TestSaveCapabilityTemplateNormalizesRulesAndDefaults(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.SaveCapabilityTemplate(context.Background(), SaveCapabilityTemplateCommand{
		Template: CapabilityTemplate{
			Key: CapabilityTemplatePublicSKUDirectShip,
			Capabilities: []CapabilityOption{
				{
					Code:    CapabilityDirectShip,
					Enabled: true,
					Config: map[string]any{
						"public_sku_aliases": true,
						"small_batch_price_rule": map[string]any{
							"enabled":      true,
							"threshold_lb": 0,
						},
					},
				},
			},
		},
		UpdatedBy: " admin ",
	})
	if err != nil {
		t.Fatalf("SaveCapabilityTemplate() err=%v", err)
	}
	if repo.templateSaveCommand.UpdatedBy != "admin" || repo.templateSaveCommand.Template.Key != CapabilityTemplatePublicSKUDirectShip {
		t.Fatalf("save command=%+v", repo.templateSaveCommand)
	}
	if got.Label != "公共 SKU 小批量代发" || len(got.ERPRoleCodes) != 0 {
		t.Fatalf("template defaults not filled: %+v", got)
	}
	if got.ThemeKey != PortalThemeCleanOps {
		t.Fatalf("theme_key=%q, want public SKU default clean_ops", got.ThemeKey)
	}
	rule := got.SmallBatchPriceRule()
	if !rule.Enabled || rule.ThresholdLB != 14 || rule.TierMinLB != 15 || rule.TierMaxLB != 28 {
		t.Fatalf("small batch rule=%+v, want normalized defaults", rule)
	}
	if got.HasCapability(CapabilityProcessing) {
		t.Fatalf("template should not enable processing from omitted capabilities: %+v", got.Capabilities)
	}
}

func TestSaveCapabilityTemplateRejectsRetailMallERPWorkbenchFields(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.SaveCapabilityTemplate(context.Background(), SaveCapabilityTemplateCommand{
		Template: CapabilityTemplate{
			Key:            CapabilityTemplateRetailMall,
			ERPPermissions: []string{"customer_processing.read"},
			ERPViewKeys:    []string{"customerProcessingPortal"},
		},
		UpdatedBy: "admin",
	})
	if !errors.Is(err, ErrCapabilityTemplateERPWorkbenchUnavailable) {
		t.Fatalf("SaveCapabilityTemplate retail ERP workbench err=%v, want ErrCapabilityTemplateERPWorkbenchUnavailable", err)
	}
	if repo.templateSaveCommand.Template.Key != "" {
		t.Fatalf("retail ERP workbench template should not be saved: %+v", repo.templateSaveCommand)
	}
}

func TestListCapabilityTemplatesKeepsManualChildrenAndInactiveRows(t *testing.T) {
	child, _ := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
	child.Key = "public_sku_direct_ship_b"
	child.ParentTemplateKey = CapabilityTemplatePublicSKUDirectShip
	child.Label = "公共 SKU 小批量代发 B"
	child.Active = true
	child.SortOrder = 30
	inactive, _ := CustomerCapabilityTemplateByKey(CapabilityTemplateRetailMall)
	inactive.Active = false
	repo := &fakeRepository{templates: []CapabilityTemplate{child, inactive}}
	svc := NewService(repo, fakeIdentityProvider{})

	rows, err := svc.ListCapabilityTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListCapabilityTemplates() err=%v", err)
	}
	var gotChild, gotRetail CapabilityTemplate
	for _, row := range rows {
		if row.Key == child.Key {
			gotChild = row
		}
		if row.Key == CapabilityTemplateRetailMall {
			gotRetail = row
		}
	}
	if gotChild.Key == "" || gotChild.ParentTemplateKey != CapabilityTemplatePublicSKUDirectShip || !gotChild.Active || gotChild.SortOrder != 30 {
		t.Fatalf("manual child template missing tree metadata: %+v", gotChild)
	}
	if gotRetail.Key == "" || gotRetail.Active {
		t.Fatalf("inactive saved template should stay visible to admin list: %+v", gotRetail)
	}
}

func TestSaveCapabilityTemplateAllowsManualChildTemplate(t *testing.T) {
	parent, _ := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
	child := parent
	child.Key = "public_sku_direct_ship_b"
	child.ParentTemplateKey = CapabilityTemplatePublicSKUDirectShip
	child.Label = "岩师傅模板"
	child.Active = true
	child.SortOrder = 10
	repo := &fakeRepository{}
	svc := NewService(repo, fakeIdentityProvider{})

	got, err := svc.SaveCapabilityTemplate(context.Background(), SaveCapabilityTemplateCommand{
		Template:  child,
		UpdatedBy: " admin ",
	})
	if err != nil {
		t.Fatalf("SaveCapabilityTemplate child err=%v", err)
	}
	if got.Key != "public_sku_direct_ship_b" || got.ParentTemplateKey != CapabilityTemplatePublicSKUDirectShip || !got.Active {
		t.Fatalf("child template not preserved: %+v", got)
	}
	if repo.templateSaveCommand.Template.ParentTemplateKey != CapabilityTemplatePublicSKUDirectShip || repo.templateSaveCommand.UpdatedBy != "admin" {
		t.Fatalf("save child command=%+v", repo.templateSaveCommand)
	}
}

func TestCopyCapabilityTemplateCreatesManualChildFromSource(t *testing.T) {
	parent, _ := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
	parent.Active = true
	repo := &fakeRepository{templates: []CapabilityTemplate{parent}}
	svc := NewService(repo, fakeIdentityProvider{})

	got, err := svc.CopyCapabilityTemplate(context.Background(), CopyCapabilityTemplateCommand{
		SourceKey: CapabilityTemplatePublicSKUDirectShip,
		NewKey:    "public_sku_direct_ship_b",
		Label:     "模板 B",
		UpdatedBy: " admin ",
	})
	if err != nil {
		t.Fatalf("CopyCapabilityTemplate() err=%v", err)
	}
	if got.Key != "public_sku_direct_ship_b" || got.ParentTemplateKey != CapabilityTemplatePublicSKUDirectShip || got.Label != "模板 B" || !got.Active {
		t.Fatalf("copy template result=%+v", got)
	}
	if repo.templateSaveCommand.Template.Key != "public_sku_direct_ship_b" || repo.templateSaveCommand.Template.ParentTemplateKey != CapabilityTemplatePublicSKUDirectShip {
		t.Fatalf("copy should save a manual child template: %+v", repo.templateSaveCommand)
	}
}

func TestCopyCapabilityTemplateGeneratesSafeKeyWhenOnlyLabelProvided(t *testing.T) {
	parent, _ := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
	parent.Active = true
	existing := parent
	existing.Key = "public_sku_direct_ship_copy"
	existing.ParentTemplateKey = CapabilityTemplatePublicSKUDirectShip
	existing.Label = "已有副本"
	repo := &fakeRepository{templates: []CapabilityTemplate{parent, existing}}
	svc := NewService(repo, fakeIdentityProvider{})

	got, err := svc.CopyCapabilityTemplate(context.Background(), CopyCapabilityTemplateCommand{
		SourceKey: CapabilityTemplatePublicSKUDirectShip,
		Label:     "岩师傅模板",
		UpdatedBy: " admin ",
	})
	if err != nil {
		t.Fatalf("CopyCapabilityTemplate() err=%v", err)
	}
	if got.Key != "public_sku_direct_ship_copy_2" || got.Label != "岩师傅模板" || got.ParentTemplateKey != CapabilityTemplatePublicSKUDirectShip {
		t.Fatalf("copy template result=%+v", got)
	}
	if repo.templateSaveCommand.Template.Key != "public_sku_direct_ship_copy_2" || repo.templateSaveCommand.UpdatedBy != "admin" {
		t.Fatalf("copy should save generated safe key: %+v", repo.templateSaveCommand)
	}
}

func TestUpdatePortalVisibilityRejectsInactiveTemplateKey(t *testing.T) {
	inactive, _ := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
	inactive.Key = "public_sku_direct_ship_b"
	inactive.ParentTemplateKey = CapabilityTemplatePublicSKUDirectShip
	inactive.Active = false
	repo := &fakeRepository{templates: []CapabilityTemplate{inactive}}
	svc := NewService(repo, fakeIdentityProvider{})

	_, err := svc.UpdatePortalVisibility(context.Background(), UpdatePortalVisibilityCommand{
		CustomerID:            147,
		DisplayName:           "客户B",
		Enabled:               true,
		CapabilityTemplateKey: " public_sku_direct_ship_b ",
	})
	if !errors.Is(err, ErrCapabilityTemplateInvalid) {
		t.Fatalf("UpdatePortalVisibility() err=%v, want ErrCapabilityTemplateInvalid", err)
	}
	if repo.visibilityCommand.CustomerID != 0 {
		t.Fatalf("inactive template should not save visibility command: %+v", repo.visibilityCommand)
	}
}

func TestPortalAdminDetailUsesLiveTemplateCapabilities(t *testing.T) {
	liveTemplate, _ := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
	liveTemplate.Key = "public_sku_direct_ship_b"
	liveTemplate.ParentTemplateKey = CapabilityTemplatePublicSKUDirectShip
	liveTemplate.Active = true
	for i := range liveTemplate.Capabilities {
		if liveTemplate.Capabilities[i].Code == CapabilityDirectShip {
			liveTemplate.Capabilities[i].Enabled = false
		}
	}
	repo := &fakeRepository{
		templates: []CapabilityTemplate{liveTemplate},
		portalDetail: PortalAdminDetail{
			Customer: PortalAdminCustomer{Name: "客户B", CapabilityTemplateKey: " public_sku_direct_ship_b "},
			Capabilities: []CapabilityOption{
				{Code: CapabilityDirectShip, Enabled: true},
				{Code: CapabilitySettlement, Enabled: false},
			},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	detail, err := svc.PortalAdminDetail(context.Background(), 147)
	if err != nil {
		t.Fatalf("PortalAdminDetail() err=%v", err)
	}
	if capabilityOptionEnabled(detail.Capabilities, CapabilityDirectShip) {
		t.Fatalf("detail should reflect live template capabilities, not stale customer_service_capabilities: %+v", detail.Capabilities)
	}
	if !capabilityOptionEnabled(detail.Capabilities, CapabilitySettlement) {
		t.Fatalf("detail should include live template settlement capability: %+v", detail.Capabilities)
	}
}

func TestApplyCapabilityTemplateUsesSavedTemplateSettings(t *testing.T) {
	template, _ := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
	for i := range template.Capabilities {
		if template.Capabilities[i].Code == CapabilityDirectShip {
			template.Capabilities[i].Enabled = false
		}
	}
	repo := &fakeRepository{
		templates: []CapabilityTemplate{template},
		portalDetail: PortalAdminDetail{
			Customer: PortalAdminCustomer{Name: "客户A"},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.ApplyCapabilityTemplate(context.Background(), ApplyCapabilityTemplateCommand{
		CustomerID:  147,
		TemplateKey: CapabilityTemplatePublicSKUDirectShip,
	})
	if err != nil {
		t.Fatalf("ApplyCapabilityTemplate() err=%v", err)
	}
	if repo.templateCommand.Template.HasCapability(CapabilityDirectShip) {
		t.Fatalf("apply command used default template instead of saved row: %+v", repo.templateCommand.Template.Capabilities)
	}
}

func TestApplyCapabilityTemplateNormalizesAndDelegates(t *testing.T) {
	repo := &fakeRepository{portalDetail: PortalAdminDetail{
		Customer: PortalAdminCustomer{Name: "客户A"},
	}}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.ApplyCapabilityTemplate(context.Background(), ApplyCapabilityTemplateCommand{
		CustomerID:  147,
		TemplateKey: " public_sku_direct_ship ",
		UpdatedBy:   " admin ",
	})
	if err != nil {
		t.Fatalf("ApplyCapabilityTemplate() err=%v", err)
	}
	if repo.templateCommand.CustomerID != 147 || repo.templateCommand.Template.Key != CapabilityTemplatePublicSKUDirectShip || repo.templateCommand.UpdatedBy != "admin" {
		t.Fatalf("template command=%+v", repo.templateCommand)
	}
	if got.Customer.CapabilityTemplateKey != CapabilityTemplatePublicSKUDirectShip || !got.HasCapabilityOption(CapabilityDirectShip) {
		t.Fatalf("detail=%+v", got)
	}
}

func TestApplyCapabilityTemplateRejectsInactiveTemplateKey(t *testing.T) {
	inactive, _ := CustomerCapabilityTemplateByKey(CapabilityTemplatePublicSKUDirectShip)
	inactive.Key = "public_sku_direct_ship_b"
	inactive.ParentTemplateKey = CapabilityTemplatePublicSKUDirectShip
	inactive.Active = false
	repo := &fakeRepository{templates: []CapabilityTemplate{inactive}}
	svc := NewService(repo, fakeIdentityProvider{})

	_, err := svc.ApplyCapabilityTemplate(context.Background(), ApplyCapabilityTemplateCommand{
		CustomerID:  147,
		TemplateKey: " public_sku_direct_ship_b ",
		UpdatedBy:   "admin",
	})
	if !errors.Is(err, ErrCapabilityTemplateInvalid) {
		t.Fatalf("ApplyCapabilityTemplate() err=%v, want ErrCapabilityTemplateInvalid", err)
	}
	if repo.templateCommand.CustomerID != 0 {
		t.Fatalf("inactive template should not be applied: %+v", repo.templateCommand)
	}
}

func TestUpsertPortalERPBindingRejectsRetailMallTemplate(t *testing.T) {
	repo := &fakeRepository{portalDetail: PortalAdminDetail{
		Customer: PortalAdminCustomer{Name: "零售客户", CapabilityTemplateKey: CapabilityTemplateRetailMall},
	}}
	svc := NewService(repo, fakeIdentityProvider{})

	_, err := svc.UpsertPortalERPBinding(context.Background(), UpsertPortalERPBindingCommand{
		CustomerID: 147,
		EmployeeID: 23,
		Status:     "active",
		UpdatedBy:  "admin",
	})
	if err == nil || !strings.Contains(err.Error(), "ERP workbench") {
		t.Fatalf("UpsertPortalERPBinding() err=%v, want ERP workbench template rejection", err)
	}
	if repo.erpBindingCommand.CustomerID != 0 {
		t.Fatalf("retail mall ERP binding should not delegate to repository: %+v", repo.erpBindingCommand)
	}
}

func TestUpsertPortalERPBindingRejectsUnknownTemplateKey(t *testing.T) {
	repo := &fakeRepository{portalDetail: PortalAdminDetail{
		Customer: PortalAdminCustomer{Name: "未知模板客户", CapabilityTemplateKey: "legacy_unknown_template"},
	}}
	svc := NewService(repo, fakeIdentityProvider{})

	_, err := svc.UpsertPortalERPBinding(context.Background(), UpsertPortalERPBindingCommand{
		CustomerID: 147,
		EmployeeID: 23,
		Status:     "active",
		UpdatedBy:  "admin",
	})
	if !errors.Is(err, ErrCapabilityTemplateERPWorkbenchUnavailable) {
		t.Fatalf("UpsertPortalERPBinding() err=%v, want ErrCapabilityTemplateERPWorkbenchUnavailable for unknown template", err)
	}
	if repo.erpBindingCommand.CustomerID != 0 {
		t.Fatalf("unknown template ERP binding should not delegate to repository: %+v", repo.erpBindingCommand)
	}
}

func TestUpsertPortalERPBindingRejectsSavedRetailMallTemplateWithERPWorkbench(t *testing.T) {
	repo := &fakeRepository{
		templates: []CapabilityTemplate{{
			Key:            CapabilityTemplateRetailMall,
			ERPPermissions: []string{"customer_processing.read"},
			ERPViewKeys:    []string{"customerProcessingPortal"},
			Capabilities:   []CapabilityOption{{Code: CapabilityMall, Enabled: true}},
		}},
		portalDetail: PortalAdminDetail{
			Customer: PortalAdminCustomer{Name: "零售客户", CapabilityTemplateKey: CapabilityTemplateRetailMall},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	_, err := svc.UpsertPortalERPBinding(context.Background(), UpsertPortalERPBindingCommand{
		CustomerID: 147,
		EmployeeID: 23,
		Status:     "active",
		UpdatedBy:  "admin",
	})
	if !errors.Is(err, ErrCapabilityTemplateERPWorkbenchUnavailable) {
		t.Fatalf("UpsertPortalERPBinding() err=%v, want ErrCapabilityTemplateERPWorkbenchUnavailable", err)
	}
	if repo.erpBindingCommand.CustomerID != 0 {
		t.Fatalf("saved retail mall ERP binding should not delegate to repository: %+v", repo.erpBindingCommand)
	}
}

func TestPortalAdminCustomerResponsesNormalizeThemeKey(t *testing.T) {
	repo := &fakeRepository{
		portalCustomers: []PortalAdminCustomer{
			{ID: 1, Name: "empty"},
			{ID: 2, Name: "clean", ThemeKey: "  clean_ops  ", MiniappEntryMode: "  mall  "},
			{ID: 3, Name: "unknown", ThemeKey: "unknown-theme", MiniappEntryMode: "unknown-entry"},
		},
		portalDetail: PortalAdminDetail{
			Customer: PortalAdminCustomer{Name: "detail", ThemeKey: "  premium_partner  ", MiniappEntryMode: "  mall  "},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	rows, err := svc.ListPortalAdminCustomers(context.Background(), PortalAdminCustomerQuery{})
	if err != nil {
		t.Fatalf("ListPortalAdminCustomers() err=%v", err)
	}
	if rows[0].ThemeKey != PortalThemeCoffeeFactory ||
		rows[1].ThemeKey != PortalThemeCleanOps ||
		rows[2].ThemeKey != PortalThemeCoffeeFactory {
		t.Fatalf("admin customer theme keys=%+v", rows)
	}
	if rows[0].MiniappEntryMode != MiniappEntryModeServices ||
		rows[1].MiniappEntryMode != MiniappEntryModeMall ||
		rows[2].MiniappEntryMode != MiniappEntryModeServices {
		t.Fatalf("admin customer entry modes=%+v", rows)
	}

	detail, err := svc.PortalAdminDetail(context.Background(), 147)
	if err != nil {
		t.Fatalf("PortalAdminDetail() err=%v", err)
	}
	if detail.Customer.ThemeKey != PortalThemePremiumPartner {
		t.Fatalf("PortalAdminDetail() theme_key=%q, want premium_partner", detail.Customer.ThemeKey)
	}
	if detail.Customer.MiniappEntryMode != MiniappEntryModeMall {
		t.Fatalf("PortalAdminDetail() miniapp_entry_mode=%q, want mall", detail.Customer.MiniappEntryMode)
	}
}

func TestPortalAdminCustomerResponsesPreserveUnknownTemplateKeyForCorrection(t *testing.T) {
	repo := &fakeRepository{
		portalCustomers: []PortalAdminCustomer{
			{ID: 1, Name: "known", CapabilityTemplateKey: " public_sku_direct_ship "},
			{ID: 2, Name: "unknown", CapabilityTemplateKey: " legacy_unknown_template "},
		},
		portalDetail: PortalAdminDetail{
			Customer: PortalAdminCustomer{Name: "detail", CapabilityTemplateKey: " legacy_unknown_template "},
		},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	rows, err := svc.ListPortalAdminCustomers(context.Background(), PortalAdminCustomerQuery{})
	if err != nil {
		t.Fatalf("ListPortalAdminCustomers() err=%v", err)
	}
	if rows[0].CapabilityTemplateKey != CapabilityTemplatePublicSKUDirectShip {
		t.Fatalf("known list capability_template_key=%q, want public_sku_direct_ship", rows[0].CapabilityTemplateKey)
	}
	if rows[1].CapabilityTemplateKey != "legacy_unknown_template" {
		t.Fatalf("unknown list capability_template_key=%q, want preserved dirty key", rows[1].CapabilityTemplateKey)
	}

	detail, err := svc.PortalAdminDetail(context.Background(), 147)
	if err != nil {
		t.Fatalf("PortalAdminDetail() err=%v", err)
	}
	if detail.Customer.CapabilityTemplateKey != "legacy_unknown_template" {
		t.Fatalf("detail capability_template_key=%q, want preserved dirty key", detail.Customer.CapabilityTemplateKey)
	}
}

func TestUpdatePortalVisibilityTrimsAndNormalizesCapabilities(t *testing.T) {
	repo := &fakeRepository{portalDetail: PortalAdminDetail{
		Customer: PortalAdminCustomer{Name: "13800138075"},
	}}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.UpdatePortalVisibility(context.Background(), UpdatePortalVisibilityCommand{
		CustomerID:              147,
		DisplayName:             "  测试客户  ",
		Enabled:                 true,
		ProcessingWarehouseCode: "  cust_147_processing  ",
		DefaultSenderID:         8,
		ThemeKey:                "  premium_partner  ",
		MiniappEntryMode:        "  mall  ",
		Capabilities:            []CapabilityOption{{Code: CapabilityDirectShip, Enabled: true}, {Code: CapabilityMall, Enabled: true}, {Code: "unknown", Enabled: true}, {Code: CapabilityBeanList, Enabled: false}},
	})
	if err != nil {
		t.Fatalf("UpdatePortalVisibility() err=%v", err)
	}
	if repo.visibilityCommand.CustomerID != 147 || repo.visibilityCommand.DisplayName != "测试客户" || !repo.visibilityCommand.Enabled {
		t.Fatalf("visibility command=%+v", repo.visibilityCommand)
	}
	if repo.visibilityCommand.ProcessingWarehouseCode != "cust_147_processing" || repo.visibilityCommand.DefaultSenderID != 8 {
		t.Fatalf("visibility warehouse/sender not normalized: %+v", repo.visibilityCommand)
	}
	if repo.visibilityCommand.ThemeKey != PortalThemePremiumPartner {
		t.Fatalf("visibility theme_key=%q, want premium_partner", repo.visibilityCommand.ThemeKey)
	}
	if repo.visibilityCommand.MiniappEntryMode != MiniappEntryModeMall {
		t.Fatalf("visibility miniapp_entry_mode=%q, want mall", repo.visibilityCommand.MiniappEntryMode)
	}
	if len(repo.visibilityCommand.Capabilities) != 3 {
		t.Fatalf("visibility capabilities=%+v, want only known codes from payload", repo.visibilityCommand.Capabilities)
	}
	if repo.visibilityCommand.Capabilities[0].Code != CapabilityBeanList || repo.visibilityCommand.Capabilities[1].Code != CapabilityMall || repo.visibilityCommand.Capabilities[2].Code != CapabilityDirectShip {
		t.Fatalf("capabilities sorted/normalized incorrectly: %+v", repo.visibilityCommand.Capabilities)
	}
}

func TestUpdatePortalVisibilityAppliesReferencedCapabilityTemplate(t *testing.T) {
	repo := &fakeRepository{portalDetail: PortalAdminDetail{
		Customer: PortalAdminCustomer{Name: "客户A"},
	}}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.UpdatePortalVisibility(context.Background(), UpdatePortalVisibilityCommand{
		CustomerID:            147,
		DisplayName:           " 客户A ",
		Enabled:               true,
		CapabilityTemplateKey: " public_sku_direct_ship ",
		Capabilities:          []CapabilityOption{{Code: CapabilityBeanList, Enabled: true}},
		ThemeKey:              PortalThemePremiumPartner,
		MiniappEntryMode:      MiniappEntryModeMall,
	})
	if err != nil {
		t.Fatalf("UpdatePortalVisibility() err=%v", err)
	}
	got := repo.visibilityCommand
	if got.CapabilityTemplateKey != CapabilityTemplatePublicSKUDirectShip || got.Template.Key != CapabilityTemplatePublicSKUDirectShip {
		t.Fatalf("template reference not applied: %+v", got)
	}
	if got.ThemeKey != PortalThemeCleanOps || got.MiniappEntryMode != MiniappEntryModeServices {
		t.Fatalf("template theme/entry not inherited: %+v", got)
	}
	if len(got.Capabilities) == 0 || !got.Template.HasCapability(CapabilityDirectShip) || !capabilityOptionEnabled(got.Capabilities, CapabilityDirectShip) {
		t.Fatalf("template capabilities not inherited: %+v", got.Capabilities)
	}
	if capabilityOptionEnabled(got.Capabilities, CapabilityBeanList) {
		t.Fatalf("inline capability payload should not override selected template: %+v", got.Capabilities)
	}
}

func TestUpdatePortalVisibilityRejectsUnknownTemplateKey(t *testing.T) {
	repo := &fakeRepository{portalDetail: PortalAdminDetail{
		Customer: PortalAdminCustomer{Name: "客户A"},
	}}
	svc := NewService(repo, fakeIdentityProvider{})

	_, err := svc.UpdatePortalVisibility(context.Background(), UpdatePortalVisibilityCommand{
		CustomerID:            147,
		DisplayName:           "客户A",
		Enabled:               true,
		CapabilityTemplateKey: " legacy_unknown_template ",
		Capabilities:          []CapabilityOption{{Code: CapabilityMall, Enabled: true}},
		ThemeKey:              PortalThemePremiumPartner,
		MiniappEntryMode:      MiniappEntryModeMall,
	})
	if err == nil || err.Error() != "capability template invalid" {
		t.Fatalf("UpdatePortalVisibility() err=%v, want capability template invalid", err)
	}
	if repo.visibilityCommand.CustomerID != 0 {
		t.Fatalf("unknown template should not save visibility command: %+v", repo.visibilityCommand)
	}
}

func capabilityOptionEnabled(options []CapabilityOption, code string) bool {
	for _, option := range options {
		if option.Code == code {
			return option.Enabled
		}
	}
	return false
}

func currentContextForTemplate(t *testing.T, template CapabilityTemplate) CurrentContext {
	t.Helper()
	capabilities := make([]Capability, 0, len(template.Capabilities))
	for _, capability := range template.Capabilities {
		capabilities = append(capabilities, Capability{
			Code:    capability.Code,
			Enabled: capability.Enabled,
			Config:  capability.Config,
		})
	}
	return CurrentContext{
		MiniUserID:          3,
		CurrentCustomerID:   7,
		CurrentCustomerName: template.Label,
		ThemeKey:            template.ThemeKey,
		MiniappEntryMode:    template.MiniappEntryMode,
		Capabilities:        capabilities,
	}
}

func assertCapabilityAccess(t *testing.T, name string, allowed bool, err error) {
	t.Helper()
	if allowed {
		if err != nil {
			t.Fatalf("%s err=%v, want allowed", name, err)
		}
		return
	}
	if !errors.Is(err, ErrCapabilityNotEnabled) {
		t.Fatalf("%s err=%v, want ErrCapabilityNotEnabled", name, err)
	}
}

func assertStringSet(t *testing.T, got, want []string, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s=%+v, want %+v", label, got, want)
	}
	seen := map[string]bool{}
	for _, value := range got {
		seen[value] = true
	}
	for _, value := range want {
		if !seen[value] {
			t.Fatalf("%s=%+v, want %+v", label, got, want)
		}
	}
}

func validMallOrderCommand() CreateMallOrderCommand {
	return CreateMallOrderCommand{
		RecipientName:    "张三",
		RecipientPhone:   "13800138000",
		RecipientAddress: "上海市",
		Items:            []MallOrderItemCommand{{MallProductID: 11, Qty: 2}},
	}
}

func validDirectShipBatchCommand() CreateDirectShipBatchCommand {
	return CreateDirectShipBatchCommand{
		SourceName: "客户批量代发",
		TotalRows:  12,
	}
}

func validProcessingRequestCommand() CreateProcessingRequestCommand {
	return CreateProcessingRequestCommand{
		InputMaterialID: 4,
		InputQtyG:       30000,
		TargetProductID: 5,
		TargetSpecG:     454,
		TargetQty:       50,
	}
}

func validFulfillmentOrderCommand(serviceCode string) CreateFulfillmentOrderCommand {
	return CreateFulfillmentOrderCommand{
		PortalServiceCode: serviceCode,
		RecipientName:     "张三",
		RecipientPhone:    "13800138000",
		RecipientAddress:  "上海市",
		ProductID:         8,
		ProductName:       "乌拉嘎",
		SpecG:             250,
		Qty:               2,
	}
}

func TestMallAdminSavesProductsAndNormalizesListing(t *testing.T) {
	repo := &fakeRepository{
		mallProducts:       []MallProduct{{ID: 1, ProductID: 7, Title: "  乌拉嘎  ", SpecG: 0, UnitPrice: 88, TemplateKey: "wide", Status: "published"}},
		mallProductOptions: []MallProductOption{{ID: 7, Name: "乌拉嘎", DefaultPrice: 88}},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	rows, options, err := svc.ListMallProducts(context.Background())
	if err != nil {
		t.Fatalf("ListMallProducts() err=%v", err)
	}
	if len(rows) != 1 || rows[0].Title != "乌拉嘎" || rows[0].SpecG != 454 || rows[0].TemplateKey != MallTemplateWide || rows[0].Status != MallProductStatusPublished {
		t.Fatalf("mall products normalized incorrectly: %+v", rows)
	}
	if len(options) != 1 || options[0].Name != "乌拉嘎" {
		t.Fatalf("mall product options=%+v", options)
	}

	got, err := svc.SaveMallProduct(context.Background(), SaveMallProductCommand{
		ProductID:   7,
		Title:       "  乌拉嘎  ",
		Subtitle:    "  柑橘莓果  ",
		Description: "  适合手冲  ",
		ImageURL:    "  /assets/mall_products/ulagaa.png  ",
		SpecG:       250,
		UnitPrice:   68,
		TemplateKey: "compact",
		Status:      "draft",
		SortOrder:   3,
		Actor:       "  测试员  ",
	})
	if err != nil {
		t.Fatalf("SaveMallProduct() err=%v", err)
	}
	if got.Title != "乌拉嘎" || repo.mallProductCommand.Subtitle != "柑橘莓果" || repo.mallProductCommand.TemplateKey != MallTemplateCompact || repo.mallProductCommand.Status != MallProductStatusDraft || repo.mallProductCommand.Actor != "测试员" {
		t.Fatalf("saved=%+v command=%+v", got, repo.mallProductCommand)
	}
}

func TestMallPageAndOrderRequireMallCapability(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			MiniUserID:        3,
			CurrentCustomerID: 7,
			Capabilities:      []Capability{{Code: CapabilityMall, Enabled: true}},
			ThemeKey:          PortalThemeCleanOps,
			MiniappEntryMode:  MiniappEntryModeMall,
		},
		mallPage: MallPage{
			ThemeKey:          PortalThemeCleanOps,
			MiniappEntryMode:  MiniappEntryModeMall,
			CurrentCustomerID: 7,
			Products:          []MallProduct{{ID: 11, ProductID: 8, Title: "冷萃拼配", SpecG: 250, UnitPrice: 59, Status: MallProductStatusPublished}},
		},
		mallOrder: FulfillmentOrder{OrderID: 88, OrderNo: "SO-20260507-0088", PortalServiceCode: PortalServiceMall, SourceWarehouse: "finished_goods"},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	page, err := svc.GetMallPage(context.Background(), "mini-token")
	if err != nil {
		t.Fatalf("GetMallPage() err=%v", err)
	}
	if page.CurrentCustomerID != 7 || page.ThemeKey != PortalThemeCleanOps || page.MiniappEntryMode != MiniappEntryModeMall || len(page.Products) != 1 {
		t.Fatalf("mall page=%+v", page)
	}

	order, err := svc.CreateMallOrder(context.Background(), "mini-token", CreateMallOrderCommand{
		RecipientName:    " 张三 ",
		RecipientPhone:   " 13800138000 ",
		RecipientAddress: " 上海市 ",
		ShippingAmount:   18,
		Items:            []MallOrderItemCommand{{MallProductID: 11, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("CreateMallOrder() err=%v", err)
	}
	if order.OrderNo != "SO-20260507-0088" || repo.mallOrderCommand.CustomerID != 7 || repo.mallOrderCommand.CreatedByMiniUserID != 3 || repo.mallOrderCommand.RecipientName != "张三" || repo.mallOrderCommand.Items[0].Qty != 2 {
		t.Fatalf("mall order=%+v command=%+v", order, repo.mallOrderCommand)
	}
	if repo.mallOrderCommand.ShippingAmount != 0 {
		t.Fatalf("mall order shipping amount=%v, want 0", repo.mallOrderCommand.ShippingAmount)
	}

	repo.context.Capabilities = []Capability{{Code: CapabilityProductOrder, Enabled: true}}
	_, err = svc.GetMallPage(context.Background(), "mini-token")
	if !errors.Is(err, ErrCapabilityNotEnabled) {
		t.Fatalf("GetMallPage without mall capability err=%v, want ErrCapabilityNotEnabled", err)
	}
}

func TestCreateFulfillmentOrderIgnoresCustomerSuppliedShippingAmount(t *testing.T) {
	repo := &fakeRepository{
		context: CurrentContext{
			MiniUserID:        3,
			CurrentCustomerID: 7,
			Capabilities:      []Capability{{Code: CapabilityDirectShip, Enabled: true}},
		},
		fulfillmentOrder: FulfillmentOrder{OrderID: 401, OrderNo: "SO-FULFILLMENT", PortalServiceCode: PortalServiceDirectShip},
	}
	svc := NewService(repo, fakeIdentityProvider{})

	order, err := svc.CreateFulfillmentOrder(context.Background(), "mini-token", CreateFulfillmentOrderCommand{
		PortalServiceCode: PortalServiceDirectShip,
		RecipientName:     " 张三 ",
		RecipientPhone:    " 13800138000 ",
		RecipientAddress:  " 上海市 ",
		ProductID:         8,
		ProductName:       " 乌拉嘎 ",
		SpecG:             250,
		Qty:               2,
		ShippingAmount:    16,
	})
	if err != nil {
		t.Fatalf("CreateFulfillmentOrder() err=%v", err)
	}
	if order.OrderNo != "SO-FULFILLMENT" {
		t.Fatalf("fulfillment order=%+v", order)
	}
	if repo.fulfillmentCommand.CustomerID != 7 || repo.fulfillmentCommand.CreatedByMiniUserID != 3 || repo.fulfillmentCommand.ProductName != "乌拉嘎" {
		t.Fatalf("fulfillment command=%+v", repo.fulfillmentCommand)
	}
	if repo.fulfillmentCommand.ShippingAmount != 0 {
		t.Fatalf("fulfillment shipping amount=%v, want 0", repo.fulfillmentCommand.ShippingAmount)
	}
}
