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
	loginResult         LoginResult
	loginCommand        CreateLoginSessionCommand
	context             CurrentContext
	session             string
	serviceQuery        ServicePageQuery
	servicePage         ServicePage
	beanList            BeanListSummary
	portalCustomers     []PortalAdminCustomer
	portalDetail        PortalAdminDetail
	visibilityCommand   UpdatePortalVisibilityCommand
	templates           []CapabilityTemplate
	templateSaveCommand SaveCapabilityTemplateCommand
	templateCommand     ApplyCapabilityTemplateCommand
	erpBindingCommand   UpsertPortalERPBindingCommand
	mallProducts        []MallProduct
	mallProductOptions  []MallProductOption
	mallProductCommand  SaveMallProductCommand
	mallImageCommand    UpdateMallProductImageCommand
	mallPage            MallPage
	mallOrderCommand    CreateMallOrderCommand
	mallOrder           FulfillmentOrder
	directShipCommand   CreateDirectShipBatchCommand
	directShipBatch     DirectShipBatch
	processingCommand   CreateProcessingRequestCommand
	processingRequest   ProcessingRequest
	fulfillmentCommand  CreateFulfillmentOrderCommand
	fulfillmentOrder    FulfillmentOrder
	err                 error
	switchErr           error
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

func capabilityOptionEnabled(options []CapabilityOption, code string) bool {
	for _, option := range options {
		if option.Code == code {
			return option.Enabled
		}
	}
	return false
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
		Items:            []MallOrderItemCommand{{MallProductID: 11, Qty: 2}},
	})
	if err != nil {
		t.Fatalf("CreateMallOrder() err=%v", err)
	}
	if order.OrderNo != "SO-20260507-0088" || repo.mallOrderCommand.CustomerID != 7 || repo.mallOrderCommand.CreatedByMiniUserID != 3 || repo.mallOrderCommand.RecipientName != "张三" || repo.mallOrderCommand.Items[0].Qty != 2 {
		t.Fatalf("mall order=%+v command=%+v", order, repo.mallOrderCommand)
	}

	repo.context.Capabilities = []Capability{{Code: CapabilityProductOrder, Enabled: true}}
	_, err = svc.GetMallPage(context.Background(), "mini-token")
	if !errors.Is(err, ErrCapabilityNotEnabled) {
		t.Fatalf("GetMallPage without mall capability err=%v, want ErrCapabilityNotEnabled", err)
	}
}
