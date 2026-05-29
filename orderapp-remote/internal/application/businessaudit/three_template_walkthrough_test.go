package businessaudit

import (
	"context"
	"errors"
	"fmt"
	"testing"

	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"
	financeapp "orderapp/internal/application/finance"
	productionapp "orderapp/internal/application/production"
	financedomain "orderapp/internal/domain/finance"
)

const (
	walkthroughProcessingTemplateKey = "processing_fulfillment"
	walkthroughPublicSKUTemplateKey  = "public_sku_direct_ship"
	walkthroughRetailMallTemplateKey = "retail_mall"
)

func TestThreeTemplateBusinessWalkthroughAcrossModules(t *testing.T) {
	ctx := context.Background()
	store := newThreeTemplateWalkthroughStore(t)
	customerPortal := customerportalapp.NewService(store, walkthroughIdentityProvider{})
	customerFulfillment := customerfulfillmentapp.NewService(store)
	production := productionapp.NewService(store)
	finance := financeapp.NewService(store)

	for _, customer := range store.customers {
		detail, err := customerPortal.ApplyCapabilityTemplate(ctx, customerportalapp.ApplyCapabilityTemplateCommand{
			CustomerID:  customer.ID,
			TemplateKey: customer.TemplateKey,
			UpdatedBy:   "Codex",
		})
		if err != nil {
			t.Fatalf("ApplyCapabilityTemplate(%s) err=%v", customer.TemplateKey, err)
		}
		if detail.Customer.CapabilityTemplateKey != customer.TemplateKey || len(detail.Capabilities) == 0 {
			t.Fatalf("template detail for %s = %+v", customer.TemplateKey, detail)
		}
		if customer.WorkbenchEnabled {
			if _, err := customerPortal.UpsertPortalERPBinding(ctx, customerportalapp.UpsertPortalERPBindingCommand{
				CustomerID: customer.ID,
				EmployeeID: customer.EmployeeID,
				Status:     "active",
				UpdatedBy:  "Codex",
			}); err != nil {
				t.Fatalf("UpsertPortalERPBinding(%s) err=%v", customer.TemplateKey, err)
			}
		}
	}

	processing := store.mustCustomer(customerportalapp.CapabilityTemplateProcessingFulfillment)
	processingMe, err := customerPortal.Me(ctx, processing.Token)
	if err != nil {
		t.Fatalf("processing Me err=%v", err)
	}
	if !processingMe.HasCapability(customerportalapp.CapabilityProcessing) || !processingMe.HasCapability(customerportalapp.CapabilitySettlement) {
		t.Fatalf("processing capabilities = %+v", processingMe.Capabilities)
	}
	if _, err := customerPortal.CreateProcessingRequest(ctx, processing.Token, customerportalapp.CreateProcessingRequestCommand{
		InputMaterialID: 4,
		InputQtyG:       30000,
		TargetProductID: 8,
		TargetSpecG:     250,
		TargetQty:       80,
		Note:            "客户小程序代加工申请",
	}); err != nil {
		t.Fatalf("processing mini processing request err=%v", err)
	}
	if _, err := customerPortal.CreateFulfillmentOrder(ctx, processing.Token, walkthroughFulfillmentOrder(customerportalapp.PortalServiceProcessingShipment)); err != nil {
		t.Fatalf("processing mini processing shipment err=%v", err)
	}
	if _, err := customerPortal.CreateFulfillmentOrder(ctx, processing.Token, walkthroughFulfillmentOrder(customerportalapp.PortalServiceDirectShip)); err != nil {
		t.Fatalf("processing mini direct ship order err=%v", err)
	}
	if _, err := customerPortal.CreateDirectShipBatch(ctx, processing.Token, customerportalapp.CreateDirectShipBatchCommand{SourceName: "客户批量代发", TotalRows: 12}); err != nil {
		t.Fatalf("processing mini direct ship batch err=%v", err)
	}
	if _, err := customerFulfillment.SubmitCustomerProcessingWorkOrder(ctx, customerfulfillmentapp.SubmitCustomerProcessingWorkOrderCommand{
		EmployeeID:         processing.EmployeeID,
		ProductID:          8,
		ProductName:        "客户代加工豆",
		RawBeanItemID:      4,
		RawBeanName:        "客户托管生豆",
		InputQuantityG:     15000,
		PlannedOutputUnits: 40,
		ExpectedDate:       "2026-05-18",
	}); err != nil {
		t.Fatalf("processing workbench processing order err=%v", err)
	}
	if _, err := customerFulfillment.AdjustCustodyInventory(ctx, customerfulfillmentapp.AdjustCustodyInventoryCommand{
		CustomerID:     processing.ID,
		ItemType:       "raw_bean",
		ItemName:       "客户托管生豆",
		QuantityGDelta: 50000,
		Actor:          "Codex",
	}); err != nil {
		t.Fatalf("processing custody adjustment err=%v", err)
	}

	publicSKU := store.mustCustomer(customerportalapp.CapabilityTemplatePublicSKUDirectShip)
	if _, err := customerPortal.CreateFulfillmentOrder(ctx, publicSKU.Token, walkthroughFulfillmentOrder(customerportalapp.PortalServiceDirectShip)); err != nil {
		t.Fatalf("public SKU mini direct ship order err=%v", err)
	}
	if _, err := customerPortal.CreateFulfillmentOrder(ctx, publicSKU.Token, walkthroughFulfillmentOrder(customerportalapp.PortalServiceProductOrder)); err != nil {
		t.Fatalf("public SKU mini product order err=%v", err)
	}
	if _, err := customerPortal.CreateDirectShipBatch(ctx, publicSKU.Token, customerportalapp.CreateDirectShipBatchCommand{SourceName: "公共 SKU 小批量", TotalRows: 6}); err != nil {
		t.Fatalf("public SKU mini direct ship batch err=%v", err)
	}
	if _, err := customerFulfillment.SubmitCustomerDirectShipOrder(ctx, customerfulfillmentapp.SubmitCustomerDirectShipOrderCommand{
		EmployeeID:      publicSKU.EmployeeID,
		ReceiverName:    "张三",
		ReceiverPhone:   "13800138000",
		ReceiverAddress: "浙江杭州",
		ProductID:       8,
		ProductName:     "公共 SKU 冷萃豆",
		Spec:            "250g",
		QuantityUnits:   2,
	}); err != nil {
		t.Fatalf("public SKU workbench direct ship order err=%v", err)
	}

	retail := store.mustCustomer(customerportalapp.CapabilityTemplateRetailMall)
	if _, err := customerPortal.GetMallPage(ctx, retail.Token); err != nil {
		t.Fatalf("retail mall page err=%v", err)
	}
	if _, err := customerPortal.CreateMallOrder(ctx, retail.Token, customerportalapp.CreateMallOrderCommand{
		RecipientName:    "李四",
		RecipientPhone:   "13900139000",
		RecipientAddress: "上海市",
		Items:            []customerportalapp.MallOrderItemCommand{{MallProductID: 11, Qty: 2}},
	}); err != nil {
		t.Fatalf("retail mall order err=%v", err)
	}
	if _, err := customerFulfillment.CustomerPortalOverview(ctx, retail.EmployeeID); !errors.Is(err, customerfulfillmentapp.ErrCustomerERPBindingNotFound) {
		t.Fatalf("retail fulfillment workbench err=%v, want ErrCustomerERPBindingNotFound", err)
	}

	for _, customer := range []*walkthroughCustomer{processing, publicSKU} {
		overview, err := customerFulfillment.CustomerPortalOverview(ctx, customer.EmployeeID)
		if err != nil {
			t.Fatalf("CustomerPortalOverview(%s) err=%v", customer.TemplateKey, err)
		}
		if len(overview.Capabilities) == 0 || len(overview.DirectShipOrders) == 0 || len(overview.Fees) == 0 {
			t.Fatalf("overview(%s) = %+v", customer.TemplateKey, overview)
		}
		if customer.TemplateKey == customerportalapp.CapabilityTemplateProcessingFulfillment && len(overview.ProcessingOrders) == 0 {
			t.Fatalf("processing overview missing processing orders: %+v", overview)
		}
		settlement, err := customerFulfillment.CreateSettlement(ctx, customerfulfillmentapp.CreateSettlementCommand{
			CustomerID: customer.ID,
			PeriodFrom: "2026-05-01",
			PeriodTo:   "2026-05-31",
			CreatedBy:  "Codex",
		})
		if err != nil {
			t.Fatalf("CreateSettlement(%s) err=%v", customer.TemplateKey, err)
		}
		if settlement.FeeItems == 0 || settlement.TotalAmountCents <= 0 {
			t.Fatalf("settlement(%s) = %+v", customer.TemplateKey, settlement)
		}
	}

	for _, customer := range []*walkthroughCustomer{processing, publicSKU, retail} {
		plan, err := production.PlanSummary(ctx, productionapp.PlanSummaryQuery{
			From:       "2026-05-01",
			To:         "2026-05-31",
			CustomerID: customer.ID,
			Plan:       true,
		})
		if err != nil {
			t.Fatalf("PlanSummary(%s) err=%v", customer.TemplateKey, err)
		}
		if len(plan.Rows) == 0 {
			t.Fatalf("PlanSummary(%s) rows empty", customer.TemplateKey)
		}
		started := startWalkthroughProduction(t, ctx, production, plan, customer.ID)
		if started.BatchID == "" {
			t.Fatalf("Start(%s) returned empty batch", customer.TemplateKey)
		}
	}

	firstPublicOrder := store.firstOrderForCustomer(publicSKU.ID)
	if _, err := finance.CreateExpense(ctx, financeapp.CreateExpenseCommand{
		Date:          "2026-05-20",
		Category:      "客户代发耗材",
		Amount:        120,
		Allocation:    financeapp.AllocationPeriodExpense,
		CustomerID:    publicSKU.ID,
		OrderID:       firstPublicOrder.ID,
		ProductID:     firstPublicOrder.ProductID,
		BatchNo:       "PB-AUDIT-001",
		DimensionNote: "公共 SKU 代发耗材",
		Actor:         "Codex",
	}); err != nil {
		t.Fatalf("finance CreateExpense err=%v", err)
	}
	dashboard, err := finance.Dashboard(ctx, "2026-05")
	if err != nil {
		t.Fatalf("finance Dashboard err=%v", err)
	}
	if dashboard.Report.RevenueTaxInclusive <= 0 || dashboard.Report.MainBusinessCost <= 0 || dashboard.Report.PeriodExpenses <= 0 {
		t.Fatalf("dashboard report missing revenue/cost/expense: %+v", dashboard.Report)
	}
	drilldown, err := finance.ReportDrilldown(ctx, financeapp.ReportFilter{Month: "2026-05"})
	if err != nil {
		t.Fatalf("finance ReportDrilldown err=%v", err)
	}
	if drilldown.SectionTotal("revenue") <= 0 || drilldown.SectionTotal("main_cost") <= 0 || drilldown.SectionTotal("period_expense") <= 0 {
		t.Fatalf("drilldown missing revenue/cost/expense sections: %+v", drilldown.Sections)
	}
	for _, serviceCode := range []string{customerportalapp.PortalServiceProcessingShipment, customerportalapp.PortalServiceDirectShip, customerportalapp.PortalServiceProductOrder, customerportalapp.PortalServiceMall} {
		if got := store.orderCountByService(serviceCode); got == 0 {
			t.Fatalf("missing order service code %s in walkthrough orders", serviceCode)
		}
	}
}

func walkthroughFulfillmentOrder(serviceCode string) customerportalapp.CreateFulfillmentOrderCommand {
	return customerportalapp.CreateFulfillmentOrderCommand{
		PortalServiceCode: serviceCode,
		RecipientName:     "张三",
		RecipientPhone:    "13800138000",
		RecipientAddress:  "浙江杭州",
		ProductID:         8,
		ProductName:       "审计冷萃豆",
		SpecG:             250,
		Qty:               2,
		UnitPrice:         68,
		ShippingAmount:    12,
	}
}

func startWalkthroughProduction(t *testing.T, ctx context.Context, svc *productionapp.Service, plan productionapp.PlanSummaryData, customerID int64) productionapp.StartResult {
	t.Helper()
	selected := map[string]bool{}
	inputByKey := map[string]int64{}
	for _, row := range plan.Rows {
		key := fmt.Sprintf("%d-%d", row.ProductID, row.SpecG)
		selected[key] = true
		if row.NeedG > 0 {
			inputByKey[key] = row.NeedG
		} else {
			inputByKey[key] = 1000
		}
	}
	started, err := svc.Start(ctx, productionapp.StartCommand{
		From:       plan.From,
		To:         plan.To,
		CustomerID: customerID,
		Selected:   selected,
		InputByKey: inputByKey,
		Operator:   "Codex",
	})
	if err != nil {
		t.Fatalf("Start(customer=%d) err=%v", customerID, err)
	}
	return started
}

type walkthroughIdentityProvider struct{}

func (walkthroughIdentityProvider) Resolve(context.Context, string) (customerportalapp.MiniIdentity, error) {
	return customerportalapp.MiniIdentity{OpenID: "openid"}, nil
}

type walkthroughCustomer struct {
	ID                  int64
	EmployeeID          int64
	Token               string
	Name                string
	TemplateKey         string
	ThemeKey            string
	MiniappEntryMode    string
	Capabilities        []customerportalapp.CapabilityOption
	WorkbenchEnabled    bool
	ProcessingWarehouse string
}

type walkthroughOrder struct {
	ID              int64
	CustomerID      int64
	OrderNo         string
	OrderDate       string
	PortalService   string
	SourceWarehouse string
	ProductID       int64
	ProductName     string
	SpecG           int64
	Qty             int64
	Amount          financedomain.Money
}

type walkthroughDemand struct {
	CustomerID  int64
	RequestNo   string
	ProductID   int64
	ProductName string
	SpecG       int64
	Qty         int64
}

type walkthroughProductionCost struct {
	ID          int64
	CustomerID  int64
	BatchID     string
	ProductName string
	Amount      financedomain.Money
}

type threeTemplateWalkthroughStore struct {
	customers          []*walkthroughCustomer
	customerByID       map[int64]*walkthroughCustomer
	customerByToken    map[string]*walkthroughCustomer
	customerByEmployee map[int64]*walkthroughCustomer
	orders             []walkthroughOrder
	demands            []walkthroughDemand
	processingRows     map[int64][]customerfulfillmentapp.ProcessingOrderSummary
	directRows         map[int64][]customerfulfillmentapp.DirectShipOrderSummary
	custodyRows        map[int64][]customerfulfillmentapp.CustodyBalance
	fees               map[int64][]customerfulfillmentapp.FeeItemSummary
	settlements        map[int64][]customerfulfillmentapp.SettlementSummary
	productionCosts    []walkthroughProductionCost
	expenses           []financeapp.Expense
	taxLedger          []financeapp.TaxLedgerEntry
	nextOrderID        int64
	nextBatchID        int64
	nextFeeID          int64
	nextExpenseID      int64
}

func newThreeTemplateWalkthroughStore(t *testing.T) *threeTemplateWalkthroughStore {
	t.Helper()
	store := &threeTemplateWalkthroughStore{
		customerByID:       map[int64]*walkthroughCustomer{},
		customerByToken:    map[string]*walkthroughCustomer{},
		customerByEmployee: map[int64]*walkthroughCustomer{},
		processingRows:     map[int64][]customerfulfillmentapp.ProcessingOrderSummary{},
		directRows:         map[int64][]customerfulfillmentapp.DirectShipOrderSummary{},
		custodyRows:        map[int64][]customerfulfillmentapp.CustodyBalance{},
		fees:               map[int64][]customerfulfillmentapp.FeeItemSummary{},
		settlements:        map[int64][]customerfulfillmentapp.SettlementSummary{},
		nextOrderID:        1000,
		nextBatchID:        1,
		nextFeeID:          1,
		nextExpenseID:      1,
	}
	store.addCustomer(t, 101, 201, "processing-token", "客户代加工履约", customerportalapp.CapabilityTemplateProcessingFulfillment, true)
	store.addCustomer(t, 102, 202, "public-token", "公共 SKU 小批量代发", customerportalapp.CapabilityTemplatePublicSKUDirectShip, true)
	store.addCustomer(t, 103, 203, "retail-token", "零售商城客户", customerportalapp.CapabilityTemplateRetailMall, false)
	return store
}

func (s *threeTemplateWalkthroughStore) addCustomer(t *testing.T, id, employeeID int64, token, name, templateKey string, workbench bool) {
	t.Helper()
	template, ok := customerportalapp.CustomerCapabilityTemplateByKey(templateKey)
	if !ok {
		t.Fatalf("template %s missing", templateKey)
	}
	customer := &walkthroughCustomer{
		ID:                  id,
		EmployeeID:          employeeID,
		Token:               token,
		Name:                name,
		TemplateKey:         templateKey,
		ThemeKey:            template.ThemeKey,
		MiniappEntryMode:    template.MiniappEntryMode,
		Capabilities:        template.Capabilities,
		WorkbenchEnabled:    workbench,
		ProcessingWarehouse: fmt.Sprintf("cust_%d_processing", id),
	}
	s.customers = append(s.customers, customer)
	s.customerByID[id] = customer
	s.customerByToken[token] = customer
	if workbench {
		s.customerByEmployee[employeeID] = customer
	}
}

func (s *threeTemplateWalkthroughStore) mustCustomer(templateKey string) *walkthroughCustomer {
	for _, customer := range s.customers {
		if customer.TemplateKey == templateKey {
			return customer
		}
	}
	panic("missing walkthrough customer " + templateKey)
}

func (s *threeTemplateWalkthroughStore) currentContext(customer *walkthroughCustomer) customerportalapp.CurrentContext {
	capabilities := make([]customerportalapp.Capability, 0, len(customer.Capabilities))
	for _, capability := range customer.Capabilities {
		capabilities = append(capabilities, customerportalapp.Capability{Code: capability.Code, Enabled: capability.Enabled, Config: capability.Config})
	}
	return customerportalapp.CurrentContext{
		MiniUserID:          customer.EmployeeID,
		CurrentCustomerID:   customer.ID,
		CurrentCustomerName: customer.Name,
		ThemeKey:            customer.ThemeKey,
		MiniappEntryMode:    customer.MiniappEntryMode,
		Capabilities:        capabilities,
	}
}

func (s *threeTemplateWalkthroughStore) addOrder(customerID int64, serviceCode, productName string, productID, specG, qty int64, amount financedomain.Money, sourceWarehouse string) walkthroughOrder {
	s.nextOrderID++
	order := walkthroughOrder{
		ID:              s.nextOrderID,
		CustomerID:      customerID,
		OrderNo:         fmt.Sprintf("SO-AUDIT-%04d", s.nextOrderID),
		OrderDate:       "2026-05-18",
		PortalService:   serviceCode,
		SourceWarehouse: sourceWarehouse,
		ProductID:       productID,
		ProductName:     productName,
		SpecG:           specG,
		Qty:             qty,
		Amount:          amount,
	}
	s.orders = append(s.orders, order)
	if serviceCode == customerportalapp.PortalServiceDirectShip {
		s.addDirectShipSummary(customerID, order)
		s.addFee(customerID, "direct_ship_service", "代发服务费", 900)
	}
	return order
}

func (s *threeTemplateWalkthroughStore) addDirectShipSummary(customerID int64, order walkthroughOrder) {
	s.directRows[customerID] = append(s.directRows[customerID], customerfulfillmentapp.DirectShipOrderSummary{
		OrderID:         order.ID,
		OrderNo:         order.OrderNo,
		OrderDate:       order.OrderDate,
		ReceiverAddress: "浙江杭州",
		Status:          "submitted",
		ItemCount:       1,
	})
}

func (s *threeTemplateWalkthroughStore) addFee(customerID int64, feeType, feeName string, amountCents int64) {
	s.nextFeeID++
	s.fees[customerID] = append(s.fees[customerID], customerfulfillmentapp.FeeItemSummary{
		FeeType:     feeType,
		FeeName:     feeName,
		AmountCents: amountCents,
		Source:      "walkthrough",
	})
}

func (s *threeTemplateWalkthroughStore) firstOrderForCustomer(customerID int64) walkthroughOrder {
	for _, order := range s.orders {
		if order.CustomerID == customerID {
			return order
		}
	}
	panic(fmt.Sprintf("missing order for customer %d", customerID))
}

func (s *threeTemplateWalkthroughStore) orderCountByService(serviceCode string) int {
	count := 0
	for _, order := range s.orders {
		if order.PortalService == serviceCode {
			count++
		}
	}
	return count
}

func (s *threeTemplateWalkthroughStore) CreateLoginSession(context.Context, customerportalapp.CreateLoginSessionCommand) (customerportalapp.LoginResult, error) {
	return customerportalapp.LoginResult{}, nil
}

func (s *threeTemplateWalkthroughStore) CreatePhoneVerifiedLoginSession(context.Context, customerportalapp.CreatePhoneVerifiedLoginSessionCommand) (customerportalapp.LoginResult, error) {
	return customerportalapp.LoginResult{}, nil
}

func (s *threeTemplateWalkthroughStore) CreatePasswordLoginSession(context.Context, customerportalapp.CreatePasswordLoginSessionCommand) (customerportalapp.LoginResult, error) {
	return customerportalapp.LoginResult{}, nil
}

func (s *threeTemplateWalkthroughStore) CurrentContextByToken(_ context.Context, token string) (customerportalapp.CurrentContext, error) {
	customer := s.customerByToken[token]
	if customer == nil {
		return customerportalapp.CurrentContext{}, customerportalapp.ErrMiniSessionNotFound
	}
	return s.currentContext(customer), nil
}

func (s *threeTemplateWalkthroughStore) SwitchCurrentCustomer(_ context.Context, token string, customerID int64) (customerportalapp.CurrentContext, error) {
	customer := s.customerByToken[token]
	if customer == nil || customer.ID != customerID {
		return customerportalapp.CurrentContext{}, customerportalapp.ErrCustomerBindingNotFound
	}
	return s.currentContext(customer), nil
}

func (s *threeTemplateWalkthroughStore) LoadServicePage(_ context.Context, query customerportalapp.ServicePageQuery) (customerportalapp.ServicePage, error) {
	page := customerportalapp.ServicePage{Key: query.Key}
	for _, order := range s.orders {
		if order.CustomerID != query.CustomerID {
			continue
		}
		page.Orders = append(page.Orders, customerportalapp.CustomerOrderSummary{
			ID:         order.ID,
			OrderNo:    order.OrderNo,
			OrderDate:  order.OrderDate,
			GrandTotal: fmt.Sprintf("%.2f", order.Amount),
			Items: []customerportalapp.CustomerOrderItemSummary{{
				ItemName:  order.ProductName,
				Spec:      fmt.Sprintf("%dg", order.SpecG),
				Qty:       fmt.Sprintf("%d", order.Qty),
				UnitPrice: "68.00",
				LineTotal: fmt.Sprintf("%.2f", order.Amount),
			}},
		})
	}
	for _, row := range s.processingRows[query.CustomerID] {
		page.ProcessingRequests = append(page.ProcessingRequests, customerportalapp.ProcessingRequest{
			RequestNo:         row.WorkOrderNo,
			TargetProductName: row.ProductName,
			TargetQty:         int(row.Units),
			Status:            row.Status,
		})
	}
	for _, fee := range s.fees[query.CustomerID] {
		page.FeeItems = append(page.FeeItems, customerportalapp.FeeItem{
			FeeType: fee.FeeType,
			Amount:  fmt.Sprintf("%.2f", float64(fee.AmountCents)/100),
			Status:  "pending",
		})
	}
	for _, row := range s.settlements[query.CustomerID] {
		page.SettlementBatches = append(page.SettlementBatches, customerportalapp.SettlementBatch{
			ID:          row.BatchID,
			PeriodFrom:  row.PeriodFrom,
			PeriodTo:    row.PeriodTo,
			Status:      row.Status,
			TotalAmount: fmt.Sprintf("%.2f", float64(row.TotalAmountCents)/100),
		})
	}
	for _, row := range s.custodyRows[query.CustomerID] {
		page.Inventory = append(page.Inventory, customerportalapp.InventoryItem{
			ItemType: row.ItemType,
			ItemName: row.ItemName,
			QtyG:     row.QuantityG,
		})
	}
	return page, nil
}

func (s *threeTemplateWalkthroughStore) LoadBeanListPublication(context.Context, int64, int64) (customerportalapp.BeanListSummary, error) {
	return customerportalapp.BeanListSummary{}, nil
}

func (s *threeTemplateWalkthroughStore) LoadBeanListPublicationAsset(context.Context, int64, string) (customerportalapp.BeanListPublicationAsset, error) {
	return customerportalapp.BeanListPublicationAsset{}, customerportalapp.ErrBeanListPublicationNotFound
}

func (s *threeTemplateWalkthroughStore) SaveBeanListPublicationAsset(_ context.Context, asset customerportalapp.BeanListPublicationAsset, _ string) (customerportalapp.BeanListPublicationAsset, error) {
	return asset, nil
}

func (s *threeTemplateWalkthroughStore) AcknowledgeBeanListPublication(context.Context, int64, int64, string) error {
	return nil
}

func (s *threeTemplateWalkthroughStore) ListPortalAdminCustomers(context.Context, customerportalapp.PortalAdminCustomerQuery) ([]customerportalapp.PortalAdminCustomer, error) {
	rows := make([]customerportalapp.PortalAdminCustomer, 0, len(s.customers))
	for _, customer := range s.customers {
		rows = append(rows, customerportalapp.PortalAdminCustomer{ID: customer.ID, Name: customer.Name, DisplayName: customer.Name, CapabilityTemplateKey: customer.TemplateKey})
	}
	return rows, nil
}

func (s *threeTemplateWalkthroughStore) PortalAdminDetail(_ context.Context, customerID int64) (customerportalapp.PortalAdminDetail, error) {
	customer := s.customerByID[customerID]
	if customer == nil {
		return customerportalapp.PortalAdminDetail{}, customerportalapp.ErrPortalCustomerNotFound
	}
	return s.portalDetail(customer), nil
}

func (s *threeTemplateWalkthroughStore) UpdatePortalVisibility(_ context.Context, cmd customerportalapp.UpdatePortalVisibilityCommand) (customerportalapp.PortalAdminDetail, error) {
	customer := s.customerByID[cmd.CustomerID]
	if customer == nil {
		return customerportalapp.PortalAdminDetail{}, customerportalapp.ErrPortalCustomerNotFound
	}
	customer.TemplateKey = cmd.CapabilityTemplateKey
	customer.ThemeKey = cmd.ThemeKey
	customer.MiniappEntryMode = cmd.MiniappEntryMode
	customer.Capabilities = cmd.Capabilities
	return s.portalDetail(customer), nil
}

func (s *threeTemplateWalkthroughStore) ListCapabilityTemplates(context.Context) ([]customerportalapp.CapabilityTemplate, error) {
	return customerportalapp.DefaultCapabilityTemplates(), nil
}

func (s *threeTemplateWalkthroughStore) SaveCapabilityTemplate(_ context.Context, cmd customerportalapp.SaveCapabilityTemplateCommand) (customerportalapp.CapabilityTemplate, error) {
	return cmd.Template, nil
}

func (s *threeTemplateWalkthroughStore) ApplyCapabilityTemplate(_ context.Context, cmd customerportalapp.ApplyCapabilityTemplateCommand) (customerportalapp.PortalAdminDetail, error) {
	customer := s.customerByID[cmd.CustomerID]
	if customer == nil {
		return customerportalapp.PortalAdminDetail{}, customerportalapp.ErrPortalCustomerNotFound
	}
	customer.TemplateKey = cmd.Template.Key
	customer.ThemeKey = cmd.Template.ThemeKey
	customer.MiniappEntryMode = cmd.Template.MiniappEntryMode
	customer.Capabilities = cmd.Template.Capabilities
	return s.portalDetail(customer), nil
}

func (s *threeTemplateWalkthroughStore) UpsertPortalERPBinding(_ context.Context, cmd customerportalapp.UpsertPortalERPBindingCommand) (customerportalapp.PortalAdminDetail, error) {
	customer := s.customerByID[cmd.CustomerID]
	if customer == nil {
		return customerportalapp.PortalAdminDetail{}, customerportalapp.ErrPortalCustomerNotFound
	}
	customer.EmployeeID = cmd.EmployeeID
	customer.WorkbenchEnabled = cmd.Status == "active"
	if customer.WorkbenchEnabled {
		s.customerByEmployee[cmd.EmployeeID] = customer
	}
	return s.portalDetail(customer), nil
}

func (s *threeTemplateWalkthroughStore) portalDetail(customer *walkthroughCustomer) customerportalapp.PortalAdminDetail {
	return customerportalapp.PortalAdminDetail{
		Customer: customerportalapp.PortalAdminCustomer{
			ID:                    customer.ID,
			Name:                  customer.Name,
			DisplayName:           customer.Name,
			PortalEnabled:         true,
			ThemeKey:              customer.ThemeKey,
			MiniappEntryMode:      customer.MiniappEntryMode,
			CapabilityTemplateKey: customer.TemplateKey,
			ERPBinding:            &customerportalapp.PortalERPBinding{CustomerID: customer.ID, EmployeeID: customer.EmployeeID, Status: "active"},
		},
		Capabilities: customer.Capabilities,
	}
}

func (s *threeTemplateWalkthroughStore) ListMallProducts(context.Context) ([]customerportalapp.MallProduct, []customerportalapp.MallProductOption, error) {
	return []customerportalapp.MallProduct{{ID: 11, ProductID: 8, Title: "审计冷萃豆", SpecG: 250, UnitPrice: 68, Status: customerportalapp.MallProductStatusPublished}}, nil, nil
}

func (s *threeTemplateWalkthroughStore) SaveMallProduct(_ context.Context, cmd customerportalapp.SaveMallProductCommand) (customerportalapp.MallProduct, error) {
	return customerportalapp.MallProduct{ID: 11, ProductID: cmd.ProductID, Title: cmd.Title, SpecG: cmd.SpecG, UnitPrice: cmd.UnitPrice, Status: cmd.Status}, nil
}

func (s *threeTemplateWalkthroughStore) UpdateMallProductImage(_ context.Context, cmd customerportalapp.UpdateMallProductImageCommand) (customerportalapp.MallProduct, error) {
	return customerportalapp.MallProduct{ID: cmd.ID, ImageURL: cmd.ImageURL}, nil
}

func (s *threeTemplateWalkthroughStore) LoadMallPage(_ context.Context, customerID int64) (customerportalapp.MallPage, error) {
	customer := s.customerByID[customerID]
	return customerportalapp.MallPage{
		ThemeKey:            customer.ThemeKey,
		MiniappEntryMode:    customer.MiniappEntryMode,
		CurrentCustomerID:   customer.ID,
		CurrentCustomerName: customer.Name,
		Products:            []customerportalapp.MallProduct{{ID: 11, ProductID: 8, Title: "审计冷萃豆", SpecG: 250, UnitPrice: 68, Status: customerportalapp.MallProductStatusPublished}},
	}, nil
}

func (s *threeTemplateWalkthroughStore) CustomerOwnsOrder(_ context.Context, customerID, orderID int64) (bool, error) {
	for _, order := range s.orders {
		if order.ID == orderID && order.CustomerID == customerID {
			return true, nil
		}
	}
	return false, nil
}

func (s *threeTemplateWalkthroughStore) CreateMallOrder(_ context.Context, cmd customerportalapp.CreateMallOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	order := s.addOrder(cmd.CustomerID, customerportalapp.PortalServiceMall, "审计冷萃豆", 8, 250, cmd.Items[0].Qty, financedomain.Money(cmd.Items[0].Qty)*68, "finished_goods")
	return customerportalapp.FulfillmentOrder{OrderID: order.ID, OrderNo: order.OrderNo, PortalServiceCode: customerportalapp.PortalServiceMall, SourceWarehouse: order.SourceWarehouse}, nil
}

func (s *threeTemplateWalkthroughStore) CreateDirectShipBatch(_ context.Context, cmd customerportalapp.CreateDirectShipBatchCommand) (customerportalapp.DirectShipBatch, error) {
	return customerportalapp.DirectShipBatch{ID: s.nextBatchID, BatchNo: fmt.Sprintf("DS-AUDIT-%03d", s.nextBatchID), SourceName: cmd.SourceName, Status: "submitted", TotalRows: cmd.TotalRows}, nil
}

func (s *threeTemplateWalkthroughStore) CreateProcessingRequest(_ context.Context, cmd customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequest, error) {
	requestNo := fmt.Sprintf("PJ-AUDIT-%03d", len(s.demands)+1)
	demand := walkthroughDemand{CustomerID: cmd.CustomerID, RequestNo: requestNo, ProductID: cmd.TargetProductID, ProductName: "客户代加工豆", SpecG: cmd.TargetSpecG, Qty: int64(cmd.TargetQty)}
	s.demands = append(s.demands, demand)
	s.processingRows[cmd.CustomerID] = append(s.processingRows[cmd.CustomerID], customerfulfillmentapp.ProcessingOrderSummary{
		WorkOrderNo: requestNo,
		ProductName: demand.ProductName,
		Status:      "planned",
		QuantityG:   int64(cmd.TargetQty) * cmd.TargetSpecG,
		Units:       int64(cmd.TargetQty),
	})
	s.addFee(cmd.CustomerID, "roasting", "代加工费", 8000)
	return customerportalapp.ProcessingRequest{ID: int64(len(s.demands)), RequestNo: requestNo, TargetProductID: cmd.TargetProductID, TargetProductName: demand.ProductName, TargetSpecG: cmd.TargetSpecG, TargetQty: cmd.TargetQty, Status: "submitted"}, nil
}

func (s *threeTemplateWalkthroughStore) CreateFulfillmentOrder(_ context.Context, cmd customerportalapp.CreateFulfillmentOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	sourceWarehouse := "finished_goods"
	if cmd.PortalServiceCode == customerportalapp.PortalServiceProcessingShipment {
		sourceWarehouse = s.customerByID[cmd.CustomerID].ProcessingWarehouse
	}
	amount := financedomain.Money(cmd.Qty)*financedomain.Money(cmd.UnitPrice) + financedomain.Money(cmd.ShippingAmount)
	order := s.addOrder(cmd.CustomerID, cmd.PortalServiceCode, cmd.ProductName, cmd.ProductID, cmd.SpecG, cmd.Qty, amount, sourceWarehouse)
	return customerportalapp.FulfillmentOrder{OrderID: order.ID, OrderNo: order.OrderNo, PortalServiceCode: order.PortalService, SourceWarehouse: order.SourceWarehouse}, nil
}

func (s *threeTemplateWalkthroughStore) ImportBatch(context.Context, int64) (customerfulfillmentapp.ImportBatch, error) {
	return customerfulfillmentapp.ImportBatch{}, nil
}

func (s *threeTemplateWalkthroughStore) StoreParsedImport(context.Context, customerfulfillmentapp.StoreParsedImportCommand) (customerfulfillmentapp.ImportBatch, error) {
	return customerfulfillmentapp.ImportBatch{}, nil
}

func (s *threeTemplateWalkthroughStore) ListImportRows(context.Context, customerfulfillmentapp.ListImportRowsQuery) ([]customerfulfillmentapp.ImportRow, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) ApplyImport(context.Context, customerfulfillmentapp.ApplyImportCommand) (customerfulfillmentapp.ApplyResult, error) {
	return customerfulfillmentapp.ApplyResult{}, nil
}

func (s *threeTemplateWalkthroughStore) CustomerPortalContext(_ context.Context, employeeID int64) (customerfulfillmentapp.CustomerERPContext, error) {
	customer := s.customerByEmployee[employeeID]
	if customer == nil || !customer.WorkbenchEnabled {
		return customerfulfillmentapp.CustomerERPContext{}, customerfulfillmentapp.ErrCustomerERPBindingNotFound
	}
	return customerfulfillmentapp.CustomerERPContext{EmployeeID: employeeID, CustomerID: customer.ID, CustomerName: customer.Name, BindingRole: "customer", BindingStatus: "active"}, nil
}

func (s *threeTemplateWalkthroughStore) CustomerPortalOverview(ctx context.Context, employeeID int64) (customerfulfillmentapp.CustomerPortalOverview, error) {
	current, err := s.CustomerPortalContext(ctx, employeeID)
	if err != nil {
		return customerfulfillmentapp.CustomerPortalOverview{}, err
	}
	return s.buildOverview(current.CustomerID, current.CustomerName), nil
}

func (s *threeTemplateWalkthroughStore) InternalCustomerPortalOverview(ctx context.Context, customerID int64) (customerfulfillmentapp.CustomerPortalOverview, error) {
	if c, ok := s.customerByID[customerID]; ok && c != nil {
		return s.buildOverview(customerID, c.Name), nil
	}
	return customerfulfillmentapp.CustomerPortalOverview{}, fmt.Errorf("customer not found")
}

func (s *threeTemplateWalkthroughStore) InternalCustomerPortalOptions(ctx context.Context, customerID int64) (customerfulfillmentapp.CustomerFulfillmentOptions, error) {
	if c, ok := s.customerByID[customerID]; ok && c != nil {
		return customerfulfillmentapp.CustomerFulfillmentOptions{
			CustomerSKUs: s.skuRows[customerID],
			CustodyItems: s.custodyItemRows[customerID],
		}, nil
	}
	return customerfulfillmentapp.CustomerFulfillmentOptions{}, fmt.Errorf("customer not found")
}

func (s *threeTemplateWalkthroughStore) buildOverview(customerID int64, customerName string) customerfulfillmentapp.CustomerPortalOverview {
	return customerfulfillmentapp.CustomerPortalOverview{
		CustomerID:       customerID,
		CustomerName:     customerName,
		Capabilities:     s.enabledCapabilityCodes(s.customerByID[customerID]),
		CustodyBalances:  s.custodyRows[customerID],
		ProcessingOrders: s.processingRows[customerID],
		DirectShipOrders: s.directRows[current.CustomerID],
		Fees:             s.fees[current.CustomerID],
		Settlements:      s.settlements[current.CustomerID],
	}, nil
}

func (s *threeTemplateWalkthroughStore) SubmitCustomerProcessingWorkOrder(_ context.Context, cmd customerfulfillmentapp.SubmitCustomerProcessingWorkOrderCommand) (customerfulfillmentapp.ProcessingOrderSummary, error) {
	customerID := cmd.CustomerID
	if customerID == 0 {
		customerID = s.customerByEmployee[cmd.EmployeeID].ID
	}
	row := customerfulfillmentapp.ProcessingOrderSummary{WorkOrderNo: fmt.Sprintf("CP-AUDIT-%03d", len(s.processingRows[customerID])+1), ProductName: cmd.ProductName, Status: "submitted", QuantityG: cmd.InputQuantityG, Units: cmd.PlannedOutputUnits}
	s.processingRows[customerID] = append(s.processingRows[customerID], row)
	s.demands = append(s.demands, walkthroughDemand{CustomerID: customerID, RequestNo: row.WorkOrderNo, ProductID: cmd.ProductID, ProductName: cmd.ProductName, SpecG: 250, Qty: cmd.PlannedOutputUnits})
	s.addFee(customerID, "roasting", "代加工费", 4500)
	return row, nil
}

func (s *threeTemplateWalkthroughStore) SubmitCustomerDirectShipOrder(_ context.Context, cmd customerfulfillmentapp.SubmitCustomerDirectShipOrderCommand) (customerfulfillmentapp.DirectShipOrderSummary, error) {
	customerID := cmd.CustomerID
	if customerID == 0 {
		customerID = s.customerByEmployee[cmd.EmployeeID].ID
	}
	s.addOrder(customerID, customerportalapp.PortalServiceDirectShip, cmd.ProductName, cmd.ProductID, 250, cmd.QuantityUnits, financedomain.Money(cmd.QuantityUnits)*68, "finished_goods")
	return s.directRows[customerID][len(s.directRows[customerID])-1], nil
}

func (s *threeTemplateWalkthroughStore) AdjustCustodyInventory(_ context.Context, cmd customerfulfillmentapp.AdjustCustodyInventoryCommand) (customerfulfillmentapp.CustodyBalance, error) {
	row := customerfulfillmentapp.CustodyBalance{ItemType: cmd.ItemType, ItemName: cmd.ItemName, QuantityG: cmd.QuantityGDelta, QuantityUnits: cmd.QuantityUnitsDelta}
	s.custodyRows[cmd.CustomerID] = append(s.custodyRows[cmd.CustomerID], row)
	return row, nil
}

func (s *threeTemplateWalkthroughStore) UpsertCustomerERPBinding(_ context.Context, cmd customerfulfillmentapp.UpsertCustomerERPBindingCommand) (customerfulfillmentapp.CustomerERPBinding, error) {
	customer := s.customerByID[cmd.CustomerID]
	customer.EmployeeID = cmd.EmployeeID
	customer.WorkbenchEnabled = cmd.Status == "active"
	s.customerByEmployee[cmd.EmployeeID] = customer
	return customerfulfillmentapp.CustomerERPBinding{CustomerID: cmd.CustomerID, EmployeeID: cmd.EmployeeID, Role: cmd.Role, Status: cmd.Status}, nil
}

func (s *threeTemplateWalkthroughStore) ListCustomerERPBindings(context.Context, int64) ([]customerfulfillmentapp.CustomerERPBinding, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) CustomerERPWorkbenchAvailable(_ context.Context, customerID int64) (bool, error) {
	customer := s.customerByID[customerID]
	return customer != nil && customer.WorkbenchEnabled, nil
}

func (s *threeTemplateWalkthroughStore) CreateExternalUser(_ context.Context, cmd customerfulfillmentapp.CreateExternalUserCommand) (customerfulfillmentapp.CustomerExternalUser, error) {
	return customerfulfillmentapp.CustomerExternalUser{CustomerID: cmd.CustomerID, Name: cmd.Name, Phone: cmd.Phone, LoginEnabled: true, HasPassword: cmd.Password != "", BindingStatus: "active"}, nil
}

func (s *threeTemplateWalkthroughStore) ListExternalUsers(context.Context, int64) ([]customerfulfillmentapp.CustomerExternalUser, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) ResetExternalUserPassword(_ context.Context, cmd customerfulfillmentapp.ResetExternalUserPasswordCommand) (customerfulfillmentapp.CustomerExternalUser, error) {
	return customerfulfillmentapp.CustomerExternalUser{CustomerID: cmd.CustomerID, EmployeeID: cmd.EmployeeID, LoginEnabled: true, HasPassword: cmd.Password != "", BindingStatus: "active"}, nil
}

func (s *threeTemplateWalkthroughStore) SetExternalUserLoginEnabled(_ context.Context, cmd customerfulfillmentapp.SetExternalUserLoginEnabledCommand) (customerfulfillmentapp.CustomerExternalUser, error) {
	return customerfulfillmentapp.CustomerExternalUser{CustomerID: cmd.CustomerID, EmployeeID: cmd.EmployeeID, LoginEnabled: cmd.LoginEnabled, HasPassword: true, BindingStatus: "active"}, nil
}

func (s *threeTemplateWalkthroughStore) CustomerFulfillmentOptions(context.Context, int64) (customerfulfillmentapp.CustomerFulfillmentOptions, error) {
	return customerfulfillmentapp.CustomerFulfillmentOptions{}, nil
}

func (s *threeTemplateWalkthroughStore) CreateSettlement(_ context.Context, cmd customerfulfillmentapp.CreateSettlementCommand) (customerfulfillmentapp.SettlementResult, error) {
	fees := s.fees[cmd.CustomerID]
	total := int64(0)
	for _, fee := range fees {
		total += fee.AmountCents
	}
	s.nextBatchID++
	row := customerfulfillmentapp.SettlementSummary{BatchID: s.nextBatchID, PeriodFrom: cmd.PeriodFrom, PeriodTo: cmd.PeriodTo, Status: "created", TotalAmountCents: total}
	s.settlements[cmd.CustomerID] = append(s.settlements[cmd.CustomerID], row)
	return customerfulfillmentapp.SettlementResult{BatchID: row.BatchID, CustomerID: cmd.CustomerID, PeriodFrom: cmd.PeriodFrom, PeriodTo: cmd.PeriodTo, FeeItems: len(fees), TotalAmountCents: total}, nil
}

func (s *threeTemplateWalkthroughStore) Overview(context.Context, customerfulfillmentapp.OverviewQuery) (customerfulfillmentapp.Overview, error) {
	return customerfulfillmentapp.Overview{}, nil
}

func (s *threeTemplateWalkthroughStore) ListImports(context.Context, customerfulfillmentapp.ListImportsQuery) ([]customerfulfillmentapp.ImportBatch, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) enabledCapabilityCodes(customer *walkthroughCustomer) []string {
	out := []string{}
	for _, capability := range customer.Capabilities {
		if capability.Enabled {
			out = append(out, capability.Code)
		}
	}
	return out
}

func (s *threeTemplateWalkthroughStore) CreateBatch(context.Context, productionapp.CreateBatchCommand) (productionapp.CreateBatchResult, error) {
	return productionapp.CreateBatchResult{}, nil
}

func (s *threeTemplateWalkthroughStore) ListBatches(context.Context, productionapp.ListBatchesCommand) ([]productionapp.BatchListItem, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) Detail(context.Context, string) (productionapp.BatchDetail, error) {
	return productionapp.BatchDetail{}, nil
}

func (s *threeTemplateWalkthroughStore) PreviewDeduct(context.Context, string) (productionapp.DeductPreview, error) {
	return productionapp.DeductPreview{}, nil
}

func (s *threeTemplateWalkthroughStore) ConfirmDeduct(context.Context, string, string) (productionapp.DeductConfirmResult, error) {
	return productionapp.DeductConfirmResult{}, nil
}

func (s *threeTemplateWalkthroughStore) ListRunning(context.Context) ([]productionapp.RunningItem, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) ListStartNeeds(_ context.Context, cmd productionapp.StartCommand) ([]productionapp.StartNeed, error) {
	rows := s.productionNeeds(cmd.CustomerID)
	out := make([]productionapp.StartNeed, 0, len(rows))
	for _, row := range rows {
		out = append(out, productionapp.StartNeed{ProductID: row.ProductID, ProductName: row.Product, SpecG: row.SpecG, GapG: row.GapG, OrderNos: row.OrderNos})
	}
	return out, nil
}

func (s *threeTemplateWalkthroughStore) Start(_ context.Context, cmd productionapp.StartExecutionCommand) (productionapp.StartResult, error) {
	s.nextBatchID++
	batchID := fmt.Sprintf("PB-AUDIT-%03d", s.nextBatchID)
	for _, need := range cmd.Needs {
		s.productionCosts = append(s.productionCosts, walkthroughProductionCost{ID: int64(len(s.productionCosts) + 1), BatchID: batchID, ProductName: need.ProductName, Amount: financedomain.Money(cmd.InputByKey[fmt.Sprintf("%d-%d", need.ProductID, need.SpecG)]) * 0.04})
	}
	return productionapp.StartResult{BatchID: batchID}, nil
}

func (s *threeTemplateWalkthroughStore) Finish(_ context.Context, cmd productionapp.FinishCommand) (productionapp.FinishResult, error) {
	return productionapp.FinishResult{RunningItemID: cmd.ID}, nil
}

func (s *threeTemplateWalkthroughStore) Cancel(context.Context, productionapp.CancelCommand) error {
	return nil
}

func (s *threeTemplateWalkthroughStore) ListMachines(context.Context, bool) ([]productionapp.RoastMachine, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) SaveMachine(context.Context, productionapp.RoastMachineCommand) error {
	return nil
}

func (s *threeTemplateWalkthroughStore) PlanSummary(_ context.Context, query productionapp.PlanSummaryQuery) (productionapp.PlanSummaryData, error) {
	rows := s.productionNeeds(query.CustomerID)
	return productionapp.PlanSummaryData{From: query.From, To: query.To, CustomerID: query.CustomerID, Rows: rows, Selected: map[string]bool{}, PlanReady: len(rows) > 0}, nil
}

func (s *threeTemplateWalkthroughStore) productionNeeds(customerID int64) []productionapp.UnprodNeedRow {
	rows := []productionapp.UnprodNeedRow{}
	for _, demand := range s.demands {
		if demand.CustomerID != customerID {
			continue
		}
		rows = append(rows, productionapp.UnprodNeedRow{ProductID: demand.ProductID, Product: demand.ProductName, OrderNos: demand.RequestNo, SpecG: demand.SpecG, NeedUnits: demand.Qty, NeedG: demand.Qty * demand.SpecG, GapG: demand.Qty * demand.SpecG})
	}
	for _, order := range s.orders {
		if order.CustomerID != customerID {
			continue
		}
		rows = append(rows, productionapp.UnprodNeedRow{ProductID: order.ProductID, Product: order.ProductName, OrderNos: order.OrderNo, SpecG: order.SpecG, NeedUnits: order.Qty, NeedG: order.Qty * order.SpecG, GapG: order.Qty * order.SpecG})
	}
	return rows
}

func (s *threeTemplateWalkthroughStore) ListProductionLogs(context.Context, productionapp.ProductionLogsQuery) (productionapp.ProductionLogsResult, error) {
	return productionapp.ProductionLogsResult{}, nil
}

func (s *threeTemplateWalkthroughStore) ListWorkOrders(context.Context, productionapp.WorkOrderQuery) ([]productionapp.WorkOrderRow, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) ListJobCards(context.Context, productionapp.JobCardQuery) ([]productionapp.JobCardRow, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) UpdateJobCardActuals(context.Context, productionapp.JobCardActualsCommand) error {
	return nil
}

func (s *threeTemplateWalkthroughStore) ListBatchCosts(context.Context, productionapp.BatchCostQuery) ([]productionapp.BatchCostRow, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) MaterialPlan(context.Context, productionapp.MaterialPlanQuery) (productionapp.MaterialPlanResult, error) {
	return productionapp.MaterialPlanResult{}, nil
}

func (s *threeTemplateWalkthroughStore) CreateQualityInspection(context.Context, productionapp.QualityInspectionCommand) (productionapp.QualityInspectionRow, error) {
	return productionapp.QualityInspectionRow{}, nil
}

func (s *threeTemplateWalkthroughStore) ListQualityInspections(context.Context, productionapp.QualityInspectionQuery) ([]productionapp.QualityInspectionRow, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) ListWIPReservations(context.Context, productionapp.WIPReservationQuery) (productionapp.WIPReservationResult, error) {
	return productionapp.WIPReservationResult{}, nil
}

func (s *threeTemplateWalkthroughStore) AdjustWIPReservation(context.Context, productionapp.WIPReservationAdjustCommand) (productionapp.WIPReservationRow, error) {
	return productionapp.WIPReservationRow{}, nil
}

func (s *threeTemplateWalkthroughStore) ReleaseWIPReservations(context.Context, productionapp.WIPReservationReleaseCommand) (productionapp.WIPReservationReleaseResult, error) {
	return productionapp.WIPReservationReleaseResult{}, nil
}

func (s *threeTemplateWalkthroughStore) AcceptanceSmoke(context.Context) (productionapp.AcceptanceSmokeResult, error) {
	return productionapp.AcceptanceSmokeResult{}, nil
}

func (s *threeTemplateWalkthroughStore) LoadSettings(context.Context) (financeapp.SettingsSnapshot, error) {
	return financeapp.SettingsSnapshot{Settings: financedomain.DefaultSettings()}, nil
}

func (s *threeTemplateWalkthroughStore) SaveSettings(_ context.Context, snapshot financeapp.SettingsSnapshot, actor string) (financeapp.SettingsSnapshot, error) {
	return snapshot, nil
}

func (s *threeTemplateWalkthroughStore) MonthlySourceTotals(_ context.Context, filter financeapp.ReportFilter) (financedomain.MonthlySourceTotals, []financeapp.Exception, error) {
	totals := financedomain.MonthlySourceTotals{Month: filter.Month}
	for _, order := range s.orders {
		if filter.CustomerID > 0 && order.CustomerID != filter.CustomerID {
			continue
		}
		totals.RevenueTaxInclusive += order.Amount
	}
	if filter.CustomerID == 0 {
		for _, cost := range s.productionCosts {
			totals.MainBusinessCost += cost.Amount
		}
	}
	for _, expense := range s.expenses {
		if filter.CustomerID > 0 && expense.CustomerID != filter.CustomerID {
			continue
		}
		if expense.Allocation == financeapp.AllocationMainCost {
			totals.MainBusinessCost += expense.Amount
		} else {
			totals.PeriodExpenses += expense.Amount
		}
	}
	return totals, nil, nil
}

func (s *threeTemplateWalkthroughStore) ListAdjustments(context.Context, string) ([]financeapp.AdjustmentRecord, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) CreateExpense(_ context.Context, cmd financeapp.CreateExpenseCommand) (financeapp.Expense, error) {
	s.nextExpenseID++
	row := financeapp.Expense{ID: s.nextExpenseID, Date: cmd.Date, Month: cmd.Month, Category: cmd.Category, Amount: cmd.Amount, Allocation: cmd.Allocation, OrderID: cmd.OrderID, CustomerID: cmd.CustomerID, ProductID: cmd.ProductID, BatchNo: cmd.BatchNo, DimensionNote: cmd.DimensionNote, Actor: cmd.Actor}
	s.expenses = append(s.expenses, row)
	return row, nil
}

func (s *threeTemplateWalkthroughStore) ListExpenses(context.Context, financeapp.ExpenseFilter) ([]financeapp.Expense, error) {
	return s.expenses, nil
}

func (s *threeTemplateWalkthroughStore) ListExpenseEmployees(context.Context) ([]financeapp.ExpenseEmployee, error) {
	return nil, nil
}

func (s *threeTemplateWalkthroughStore) FinanceSourceDetails(_ context.Context, filter financeapp.ReportFilter) ([]financeapp.SourceDetail, error) {
	rows := []financeapp.SourceDetail{}
	for _, order := range s.orders {
		if filter.CustomerID > 0 && order.CustomerID != filter.CustomerID {
			continue
		}
		customer := s.customerByID[order.CustomerID]
		rows = append(rows, financeapp.SourceDetail{Section: "revenue", SourceType: "order_revenue", SourceID: order.ID, Date: order.OrderDate, Name: order.OrderNo, Counterparty: customer.Name, Amount: order.Amount, Link: "/app/vue-shell?view=orders"})
	}
	if filter.CustomerID == 0 {
		for _, cost := range s.productionCosts {
			rows = append(rows, financeapp.SourceDetail{Section: "main_cost", SourceType: "production_cost", SourceID: cost.ID, Date: "2026-05-19", Name: cost.BatchID + " " + cost.ProductName, Amount: cost.Amount, Link: "/app/vue-shell?view=productionCosts"})
		}
	}
	for _, expense := range s.expenses {
		if filter.CustomerID > 0 && expense.CustomerID != filter.CustomerID {
			continue
		}
		rows = append(rows, financeapp.SourceDetail{Section: expense.Allocation, SourceType: "expense", SourceID: expense.ID, Date: expense.Date, Name: expense.Category, Category: expense.Category, Amount: expense.Amount, Link: "/app/vue-shell?view=financeExpenses"})
	}
	return rows, nil
}

func (s *threeTemplateWalkthroughStore) ListTaxLedger(context.Context, string) ([]financeapp.TaxLedgerEntry, error) {
	return s.taxLedger, nil
}

func (s *threeTemplateWalkthroughStore) CreateTaxLedgerEntry(_ context.Context, cmd financeapp.CreateTaxLedgerCommand) (financeapp.TaxLedgerEntry, error) {
	row := financeapp.TaxLedgerEntry{ID: int64(len(s.taxLedger) + 1), Month: cmd.Month, Kind: cmd.Kind, InvoiceNo: cmd.InvoiceNo, Counterparty: cmd.Counterparty, TotalAmount: cmd.TotalAmount, TaxAmount: cmd.TaxAmount, Status: cmd.Status, Actor: cmd.Actor}
	s.taxLedger = append(s.taxLedger, row)
	return row, nil
}

func (s *threeTemplateWalkthroughStore) SaveMonthlyReport(_ context.Context, report financedomain.MonthlyReport, actor string) (financedomain.MonthlyReport, error) {
	return report, nil
}

func (s *threeTemplateWalkthroughStore) MonthlyReportStatus(context.Context, string) (string, error) {
	return financedomain.MonthStatusDraft, nil
}

func (s *threeTemplateWalkthroughStore) CreateAdjustment(context.Context, financeapp.CreateAdjustmentCommand) (financeapp.AdjustmentRecord, error) {
	return financeapp.AdjustmentRecord{}, nil
}

var _ customerportalapp.Repository = (*threeTemplateWalkthroughStore)(nil)
var _ customerfulfillmentapp.Repository = (*threeTemplateWalkthroughStore)(nil)
var _ productionapp.Repository = (*threeTemplateWalkthroughStore)(nil)
var _ financeapp.Repository = (*threeTemplateWalkthroughStore)(nil)
