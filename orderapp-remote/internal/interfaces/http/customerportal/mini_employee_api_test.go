package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	customerapp "orderapp/internal/application/customer"
	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

func TestMiniEmployeeCurrentCatalogProductRejectsMissingAliasWhenProductHasMultipleCustomerAliases(t *testing.T) {
	products := []salesapp.ProductOption{
		{ID: 551, CustomerProductAliasID: 201, Name: "客户别名一"},
		{ID: 551, CustomerProductAliasID: 202, Name: "客户别名二"},
	}
	if _, found := miniEmployeeCurrentCatalogProduct(products, 551, 0); found {
		t.Fatal("missing alias_id must not silently select one of multiple customer aliases")
	}
	got, found := miniEmployeeCurrentCatalogProduct(products, 551, 202)
	if !found || got.CustomerProductAliasID != 202 {
		t.Fatalf("explicit alias selection = %+v, found=%v", got, found)
	}
	got, found = miniEmployeeCurrentCatalogProduct(products[:1], 551, 0)
	if !found || got.CustomerProductAliasID != 201 {
		t.Fatalf("single alias fallback = %+v, found=%v", got, found)
	}
}

type miniEmployeeSalesFake struct {
	listQuery                salesapp.OrderListQuery
	listResult               *salesapp.OrderListResult
	save                     salesapp.SaveOrderCommand
	orderFormEditID          int64
	orderFormResult          *salesapp.OrderFormData
	orderFormCalls           int
	listCalls                int
	saveCalls                int
	saveError                error
	editability              salesapp.OrderEditability
	editabilityError         error
	editabilityCalls         int
	editabilityOrderID       int64
	draft                    *salesapp.EmployeeOrderDraft
	draftSave                salesapp.SaveEmployeeOrderDraftCommand
	draftSaveError           error
	draftDeleteID            int64
	salesOrderDocuments      []salesapp.SalesOrderDocument
	salesOrderImages         []salesapp.SalesOrderImageDocument
	deliveryNoteDocuments    []salesapp.DeliveryNoteDocument
	salesOrderPDF            salesapp.SalesOrderDocumentFile
	salesOrderPNG            salesapp.SalesOrderImageFile
	deliveryNotePDF          salesapp.DeliveryNoteDocumentFile
	deliveryNotePNG          salesapp.DeliveryNoteImageFile
	documentLoadError        error
	documentListCalls        int
	documentLoadCalls        int
	documentLoadOrderID      int64
	documentLoadID           int64
	documentLoadLatest       bool
	generateSalesOrderPDF    salesapp.GenerateSalesOrderDocumentCommand
	generateSalesOrderPNG    salesapp.GenerateSalesOrderImageCommand
	generateDeliveryNotePDF  salesapp.GenerateDeliveryNoteDocumentCommand
	generateSalesOrderResult salesapp.GenerateSalesOrderDocumentResult
	generateSalesImageResult salesapp.GenerateSalesOrderImageResult
	generateDeliveryResult   salesapp.GenerateDeliveryNoteDocumentResult
	generateCalls            int
}

func (f *miniEmployeeSalesFake) ListOrders(_ context.Context, query salesapp.OrderListQuery) (salesapp.OrderListResult, error) {
	f.listCalls++
	f.listQuery = query
	if f.listResult != nil {
		return *f.listResult, nil
	}
	return salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 1, OrderNo: "SO-1"}}}, nil
}
func (f *miniEmployeeSalesFake) OrderForm(_ context.Context, editID int64) (salesapp.OrderFormData, error) {
	f.orderFormCalls++
	f.orderFormEditID = editID
	if f.orderFormResult != nil {
		result := *f.orderFormResult
		if editID > 0 && result.EditData == nil {
			result.EditData = &salesapp.OrderEditData{ID: editID, CustomerID: 8}
		}
		return result, nil
	}
	result := salesapp.OrderFormData{
		Today: "2026-07-30",
		Customers: []salesapp.CustomerOption{{
			ID: 8, Name: "客户A", Contact: "收货人A", Phone: "13800000000",
			Address: "上海市测试路1号", CompanyName: "客户A公司", ResponsibleEmployeeID: 7,
		}},
		Products: []salesapp.ProductOption{{
			ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎",
			Name: "乌拉嘎 227g", SKUName: "227g袋装", SKUCode: "WLG-227", SpecLabel: "227g",
			NetContentQty: 227, NetContentUnit: "g", ProductKind: "roasted_bean",
			Tiers: []salesapp.ProductTierOption{{
				ID: 11, UnitPrice: 68, PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial",
				PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`, QuantityBasis: "sales_spec_count",
				EffectiveSalesSpec: map[string]any{"sku_id": float64(551), "spec_label": "227g", "sales_unit": "袋"},
			}},
		}},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
	}
	if editID > 0 {
		result.EditData = &salesapp.OrderEditData{ID: editID, CustomerID: 8}
	}
	return result, nil
}
func (f *miniEmployeeSalesFake) SaveOrder(_ context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	f.saveCalls++
	f.save = cmd
	if f.saveError != nil {
		return salesapp.SaveOrderResult{}, f.saveError
	}
	return salesapp.SaveOrderResult{OrderID: 9, OrderNo: "SO-9"}, nil
}

func (f *miniEmployeeSalesFake) OrderEditability(_ context.Context, orderID int64) (salesapp.OrderEditability, error) {
	f.editabilityCalls++
	f.editabilityOrderID = orderID
	if f.editabilityError != nil {
		return salesapp.OrderEditability{}, f.editabilityError
	}
	if f.editability.BlockReason == "" && !f.editability.CanEdit {
		return salesapp.OrderEditability{CanEdit: true}, nil
	}
	return f.editability, nil
}

func (f *miniEmployeeSalesFake) GetEmployeeOrderDraft(_ context.Context, employeeID int64) (*salesapp.EmployeeOrderDraft, error) {
	if f.draft == nil || f.draft.EmployeeID != employeeID {
		return nil, nil
	}
	copyDraft := *f.draft
	copyDraft.Payload = append(json.RawMessage(nil), f.draft.Payload...)
	return &copyDraft, nil
}

func (f *miniEmployeeSalesFake) SaveEmployeeOrderDraft(_ context.Context, cmd salesapp.SaveEmployeeOrderDraftCommand) (salesapp.EmployeeOrderDraft, error) {
	f.draftSave = cmd
	if f.draftSaveError != nil {
		return salesapp.EmployeeOrderDraft{}, f.draftSaveError
	}
	return salesapp.EmployeeOrderDraft{ID: 12, EmployeeID: cmd.EmployeeID, Payload: cmd.Payload, UpdatedAt: time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)}, nil
}

func (f *miniEmployeeSalesFake) DeleteEmployeeOrderDraft(_ context.Context, employeeID int64, _ string) (bool, error) {
	f.draftDeleteID = employeeID
	return true, nil
}

func (f *miniEmployeeSalesFake) ListSalesOrderDocuments(context.Context, int64) ([]salesapp.SalesOrderDocument, error) {
	f.documentListCalls++
	return f.salesOrderDocuments, nil
}

func (f *miniEmployeeSalesFake) ListSalesOrderImageDocuments(context.Context, int64) ([]salesapp.SalesOrderImageDocument, error) {
	f.documentListCalls++
	return f.salesOrderImages, nil
}

func (f *miniEmployeeSalesFake) ListDeliveryNoteDocuments(context.Context, int64) ([]salesapp.DeliveryNoteDocument, error) {
	f.documentListCalls++
	return f.deliveryNoteDocuments, nil
}

func (f *miniEmployeeSalesFake) LoadSalesOrderDocumentFile(_ context.Context, orderID, documentID int64, latest bool) (salesapp.SalesOrderDocumentFile, error) {
	f.recordDocumentLoad(orderID, documentID, latest)
	if f.documentLoadError != nil {
		return salesapp.SalesOrderDocumentFile{}, f.documentLoadError
	}
	return f.salesOrderPDF, nil
}

func (f *miniEmployeeSalesFake) LoadSalesOrderImageFile(_ context.Context, orderID, documentID int64, latest bool) (salesapp.SalesOrderImageFile, error) {
	f.recordDocumentLoad(orderID, documentID, latest)
	if f.documentLoadError != nil {
		return salesapp.SalesOrderImageFile{}, f.documentLoadError
	}
	return f.salesOrderPNG, nil
}

func (f *miniEmployeeSalesFake) LoadDeliveryNoteDocumentFile(_ context.Context, orderID, documentID int64, latest bool) (salesapp.DeliveryNoteDocumentFile, error) {
	f.recordDocumentLoad(orderID, documentID, latest)
	if f.documentLoadError != nil {
		return salesapp.DeliveryNoteDocumentFile{}, f.documentLoadError
	}
	return f.deliveryNotePDF, nil
}

func (f *miniEmployeeSalesFake) LoadDeliveryNoteImageFile(_ context.Context, orderID, documentID int64, latest bool) (salesapp.DeliveryNoteImageFile, error) {
	f.recordDocumentLoad(orderID, documentID, latest)
	if f.documentLoadError != nil {
		return salesapp.DeliveryNoteImageFile{}, f.documentLoadError
	}
	return f.deliveryNotePNG, nil
}

func (f *miniEmployeeSalesFake) recordDocumentLoad(orderID, documentID int64, latest bool) {
	f.documentLoadCalls++
	f.documentLoadOrderID = orderID
	f.documentLoadID = documentID
	f.documentLoadLatest = latest
}

func (f *miniEmployeeSalesFake) GenerateSalesOrderDocument(_ context.Context, cmd salesapp.GenerateSalesOrderDocumentCommand) (salesapp.GenerateSalesOrderDocumentResult, error) {
	f.generateCalls++
	f.generateSalesOrderPDF = cmd
	return f.generateSalesOrderResult, nil
}

func (f *miniEmployeeSalesFake) GenerateSalesOrderImage(_ context.Context, cmd salesapp.GenerateSalesOrderImageCommand) (salesapp.GenerateSalesOrderImageResult, error) {
	f.generateCalls++
	f.generateSalesOrderPNG = cmd
	return f.generateSalesImageResult, nil
}

func (f *miniEmployeeSalesFake) GenerateDeliveryNoteDocument(_ context.Context, cmd salesapp.GenerateDeliveryNoteDocumentCommand) (salesapp.GenerateDeliveryNoteDocumentResult, error) {
	f.generateCalls++
	f.generateDeliveryNotePDF = cmd
	return f.generateDeliveryResult, nil
}

type miniEmployeeCustomersFake struct {
	listActor        customerapp.MaintenancePrincipal
	editorActor      customerapp.MaintenancePrincipal
	upsertActor      customerapp.MaintenancePrincipal
	upsertID         *int64
	upsertCmd        customerapp.UpsertCommand
	upsertCalls      int
	upsertCompleted  bool
	listResult       customerapp.ListResult
	editorData       *customerapp.EditorData
	afterUpsertError error
	error            error
}

func (f *miniEmployeeCustomersFake) ListManaged(_ context.Context, actor customerapp.MaintenancePrincipal, _ customerapp.ListQuery) (customerapp.ListResult, error) {
	f.listActor = actor
	return f.listResult, f.error
}

func (f *miniEmployeeCustomersFake) EditorManaged(_ context.Context, actor customerapp.MaintenancePrincipal, _ int64) (*customerapp.EditorData, error) {
	f.editorActor = actor
	if f.upsertCompleted && f.afterUpsertError != nil {
		return nil, f.afterUpsertError
	}
	return f.editorData, f.error
}

func (f *miniEmployeeCustomersFake) UpsertManaged(_ context.Context, actor customerapp.MaintenancePrincipal, id *int64, cmd customerapp.UpsertCommand) (int64, error) {
	f.upsertCalls++
	f.upsertActor = actor
	f.upsertID = id
	f.upsertCmd = cmd
	if f.error != nil {
		return 0, f.error
	}
	f.upsertCompleted = true
	if id != nil {
		return *id, nil
	}
	return 99, nil
}

func employeePortalService() fakeService {
	return fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 7, EmployeeName: "销售甲",
		Roles: []string{"sales"}, Permissions: []string{"orders.read", "orders.write", "customers.read", "customers.write"},
	}}
}

func TestMiniEmployeeOrderListScopesSalesToOwnOrders(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders?q=SO", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"order_no":"SO-1"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sales.listQuery.Scope != "mine" || sales.listQuery.EmployeeID != 7 || sales.listQuery.Q != "SO" {
		t.Fatalf("query=%+v", sales.listQuery)
	}
}

func TestMiniEmployeeOrderListKeepsAdministratorScopeUnrestricted(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	portal := fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 1, Roles: []string{"admin"}, Permissions: []string{"orders.read"},
	}}
	registerMiniEmployeeAPI(e, portal, sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || sales.listQuery.Scope != "" || sales.listQuery.EmployeeID != 0 {
		t.Fatalf("status=%d query=%+v body=%s", rec.Code, sales.listQuery, rec.Body.String())
	}
}

func TestMiniEmployeeOrderDetailReturnsExplicitWebParityDTOAfterScopedLookup(t *testing.T) {
	e := echo.New()
	row := salesapp.OrderRow{
		ID: 42, OrderNo: "SO-42", DocumentDate: "2026-08-01", OrderDate: "2026-07-31",
		CustomerID: 8, Customer: "客户A", ResponsibleType: "employee", ResponsibleID: 7, ResponsibleName: "销售甲",
		TotalAmount: "136.00", ShippingAmount: "12.00", DiscountAmount: "6.00", GrandTotal: "142.00", ExpressFee: "顺丰到付",
		OutsourceMaterialFee: "1.00", OutsourceRoastFee: "2.00", OutsourcePackagingFee: "3.00", OutsourceManualFee: "4.00",
		OutsourceTaxFee: "5.00", OutsourceOtherFee: "6.00", OutsourceTotalFee: "21.00",
		OrderTypeID: 2, OrderType: "批发", PayStatusID: 3, PayStatus: "已收款", PaymentMethod: "银行转账",
		ShipStatusID: 4, ShipStatus: "已发货", ProcessStatusID: 5, ProcessStatus: "生产完成", ShipTrackingNo: "SF123",
		ReceiverName: "收货人A", ReceiverPhone: "13800000000", ReceiverAddress: "上海市测试路1号", ReceiverCompany: "客户A公司",
		PortalServiceCode: "employee", SourceWarehouse: "finished_goods", ProductKindSummary: "roasted_bean",
		CreatedByEmployee: "销售甲", Notes: "订单备注", InvoiceStatus: "uploaded", InvoiceFilename: "invoice.pdf", InvoiceFileURL: "/assets/invoice.pdf",
	}
	form := salesapp.OrderFormData{EditData: &salesapp.OrderEditData{
		ID: 42, EditRevision: "rev-42", OrderNo: "SO-42", DocumentDate: "2026-08-01", OrderDate: "2026-07-31", CustomerID: 8,
		SourceID: 1, OrderTypeID: 2, PayStatusID: 3, PaymentMethod: "银行转账", ShipStatusID: 4,
		ShipMethod: "顺丰", ShipTrackingNo: "SF123", LogisticsCompanyID: 10, LogisticsProductID: 11,
		PaymentGoodsAmount: "136.00", PaymentShippingAmount: "12.00", ResponsibleType: "employee", ResponsibleID: 7, ResponsibleName: "销售甲",
		ReceiverName: "收货人A", ReceiverPhone: "13800000000", ReceiverAddress: "上海市测试路1号", ReceiverCompany: "客户A公司",
		PortalServiceCode: "employee", SourceWarehouse: "finished_goods", BeanListPublicationID: 91, BeanListVersionNo: "V1.2", Notes: "订单备注",
		TotalAmount: "136.00", ShippingAmount: "12.00", DiscountAmount: "6.00", RoundToInt: true, RoundingAmount: "0.00", GrandTotal: "142.00",
		ExpressFee: "顺丰到付", OutsourceMaterialFee: "1.00", OutsourceRoastFee: "2.00", OutsourcePackagingFee: "3.00", OutsourceManualFee: "4.00",
		OutsourceTaxFee: "5.00", OutsourceOtherFee: "6.00", OutsourceTotalFee: "21.00",
		Items: []salesapp.OrderEditItem{{
			ItemID: 51, LineNo: 1, ProductID: 551, Product: "乌拉嘎", CustomerProductAliasID: 201,
			CustomerProductDisplayNameSnapshot: "客户乌拉嘎", CustomerItemCodeSnapshot: "C-551", BrandNameSnapshot: "品牌A",
			ProductCodeSnapshot: "WLG-227", ProductNameSnapshot: "乌拉嘎 227g", Note: "研磨",
			Spec: "227g", Qty: "2", Unit: "袋", UnitPrice: "68.00", LineTotal: "136.00", PriceTierID: 12,
			BeanListPublicationID: 91, BeanListVersionNo: "V1.2", DiscountType: "unit_amount", DiscountValue: "3.00", DiscountAmount: "6.00",
			ProductKind: "roasted_bean", SalesUnit: "bag", UnitBeanG: "227", MatchedPriceQty: "2", UnitConversionLabel: "227g/袋",
			PriceSourceJSON: `{"publication_id":91,"version":"V1.2","tier_label":"2袋+","price_unit":"元/袋","final_unit_price":68,"pricing_rule_version":"R3","manual_adjusted":false,"cost_source_snapshot":{"work_order_no":"WO-42","bom_version_no":"BOM-V2","process_route_name":"标准烘焙","process_card_no":"PC-9","material_batch_no":"MB-1"}}`,
		}},
	}}
	sales := &miniEmployeeSalesFake{
		listResult:      &salesapp.OrderListResult{Rows: []salesapp.OrderRow{row}},
		orderFormResult: &form,
	}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)

	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders/42", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"can_edit":true`, `"edit_block_reason":""`,
		`"order":{"id":42,"edit_revision":"rev-42","order_no":"SO-42"`, `"customer":"客户A"`, `"source_id":1`, `"order_type":"批发"`,
		`"payment_goods_amount":"136.00"`, `"receiver_address":"上海市测试路1号"`, `"outsource_total_fee":"21.00"`,
		`"discount_amount":"6.00"`, `"order_discount_amount":"0.00"`,
		`"items":[{"item_id":51,"line_no":1,"product_id":551`, `"customer_product_alias_id":201`, `"line_total":"136.00"`, `"price_override":false`,
		`"quote_source_trace":[{"product_id":551`, `"price_list_publication_id":91`, `"source_label":"已发布商品价格表快照"`,
		`"production_source_trace":[{"product_id":551`, `"work_order_no":"WO-42"`, `"bom_version_no":"BOM-V2"`,
		`"documents":{`, `"sales_order_pdf"`, `"sales_order_png"`, `"delivery_note_pdf"`, `"delivery_note_png"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body=%s missing %s", rec.Body.String(), want)
		}
	}
	if sales.listQuery.OrderID != 42 || sales.listQuery.Void != "all" || sales.listQuery.Scope != "mine" || sales.listQuery.EmployeeID != 7 || sales.listQuery.Limit != 1 {
		t.Fatalf("scope query=%+v", sales.listQuery)
	}
	if sales.orderFormCalls != 1 || sales.orderFormEditID != 42 {
		t.Fatalf("order form calls=%d edit_id=%d", sales.orderFormCalls, sales.orderFormEditID)
	}
	if sales.editabilityCalls != 1 || sales.editabilityOrderID != 42 {
		t.Fatalf("editability calls=%d order_id=%d", sales.editabilityCalls, sales.editabilityOrderID)
	}
}

func TestMiniEmployeeHeaderDiscountAmountSubtractsLineDiscounts(t *testing.T) {
	ed := &salesapp.OrderEditData{
		DiscountAmount: "12.50",
		Items: []salesapp.OrderEditItem{
			{DiscountAmount: "2.25"},
			{DiscountAmount: "3.25"},
		},
	}
	if got := miniEmployeeHeaderDiscountAmount(ed); got != "7.00" {
		t.Fatalf("header discount=%q want 7.00", got)
	}
	ed.DiscountAmount = "4.00"
	if got := miniEmployeeHeaderDiscountAmount(ed); got != "0.00" {
		t.Fatalf("legacy inconsistent discount must clamp at zero, got %q", got)
	}
}

func TestMiniEmployeeOrderDetailReturnsPreProductionEditBlockReason(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", ProcessStatus: "生产中"}}}
	form := salesapp.OrderFormData{EditData: &salesapp.OrderEditData{ID: 42, OrderNo: "SO-42"}}
	sales := &miniEmployeeSalesFake{
		listResult: &result, orderFormResult: &form,
		editability: salesapp.OrderEditability{CanEdit: false, BlockReason: "订单已进入生产流程，不能再编辑"},
	}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders/42", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"can_edit":false`) || !strings.Contains(rec.Body.String(), `"edit_block_reason":"订单已进入生产流程，不能再编辑"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniEmployeeOrderDetailReturnsNotFoundBeforeLoadingOutOfScopeOrder(t *testing.T) {
	e := echo.New()
	empty := salesapp.OrderListResult{Rows: []salesapp.OrderRow{}}
	sales := &miniEmployeeSalesFake{listResult: &empty}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)

	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders/42", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound || sales.orderFormCalls != 0 {
		t.Fatalf("status=%d detail_calls=%d body=%s", rec.Code, sales.orderFormCalls, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) || strings.Contains(rec.Body.String(), "forbidden") {
		t.Fatalf("out-of-scope response=%s", rec.Body.String())
	}
}

func TestMiniEmployeeOrderDetailKeepsAdministratorScopeUnrestricted(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42"}}}
	form := salesapp.OrderFormData{EditData: &salesapp.OrderEditData{ID: 42, OrderNo: "SO-42"}}
	sales := &miniEmployeeSalesFake{listResult: &result, orderFormResult: &form}
	portal := fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 1, Roles: []string{"admin"}, Permissions: []string{"orders.read"},
	}}
	registerMiniEmployeeAPI(e, portal, sales)

	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders/42", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || sales.listQuery.Scope != "" || sales.listQuery.EmployeeID != 0 || sales.orderFormCalls != 1 {
		t.Fatalf("status=%d query=%+v detail_calls=%d body=%s", rec.Code, sales.listQuery, sales.orderFormCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderDetailDocumentAvailabilityUsesLatestFormalVersions(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42"}}}
	form := salesapp.OrderFormData{EditData: &salesapp.OrderEditData{ID: 42, OrderNo: "SO-42"}}
	sales := &miniEmployeeSalesFake{
		listResult: &result, orderFormResult: &form,
		salesOrderPDF: salesapp.SalesOrderDocumentFile{
			Document: salesapp.SalesOrderDocument{VersionNo: 3},
			Path:     miniEmployeeDocumentTestFile(t, "available-sales-order.pdf", "%PDF"),
			Filename: "SO-42-V3.pdf",
		},
		salesOrderPNG: salesapp.SalesOrderImageFile{
			Document: salesapp.SalesOrderImageDocument{VersionNo: 2},
			Path:     miniEmployeeDocumentTestFile(t, "available-sales-order.png", "\x89PNG\r\n\x1a\n"),
			Filename: "SO-42-V2.png",
		},
		deliveryNotePDF: salesapp.DeliveryNoteDocumentFile{
			Document: salesapp.DeliveryNoteDocument{VersionNo: 5},
			Path:     miniEmployeeDocumentTestFile(t, "available-delivery-note.pdf", "%PDF"),
			Filename: "SO-42-DN-V5.pdf",
		},
		salesOrderDocuments: []salesapp.SalesOrderDocument{{ID: 2, OrderID: 42, PDFAssetID: 20, IsLatest: true}},
		salesOrderImages:    []salesapp.SalesOrderImageDocument{{ID: 3, OrderID: 42, ImageAssetID: 30, IsLatest: true}},
		deliveryNoteDocuments: []salesapp.DeliveryNoteDocument{
			{ID: 5, OrderID: 42, PDFAssetID: 50, ImageAssetID: 0, IsLatest: true},
			{ID: 4, OrderID: 42, PDFAssetID: 40, ImageAssetID: 41, IsLatest: false},
		},
	}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)

	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders/42", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response miniEmployeeOrderDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Documents.SalesOrderPDF.Available || !response.Documents.SalesOrderPNG.Available || !response.Documents.DeliveryNotePDF.Available || response.Documents.DeliveryNotePNG.Available {
		t.Fatalf("documents=%+v", response.Documents)
	}
	if response.Documents.SalesOrderPDF.Filename != "SO-42-V3.pdf" || response.Documents.SalesOrderPDF.VersionNo != 3 ||
		response.Documents.SalesOrderPNG.Filename != "SO-42-V2.png" || response.Documents.SalesOrderPNG.VersionNo != 2 ||
		response.Documents.DeliveryNotePDF.Filename != "SO-42-DN-V5.pdf" || response.Documents.DeliveryNotePDF.VersionNo != 5 {
		t.Fatalf("document filenames and versions=%+v", response.Documents)
	}
}

func TestMiniEmployeeOrderDetailDoesNotAdvertiseMissingFormalFiles(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42"}}}
	form := salesapp.OrderFormData{EditData: &salesapp.OrderEditData{ID: 42, OrderNo: "SO-42"}}
	sales := &miniEmployeeSalesFake{
		listResult: &result, orderFormResult: &form,
		salesOrderDocuments:   []salesapp.SalesOrderDocument{{ID: 2, OrderID: 42, PDFAssetID: 20, IsLatest: true}},
		salesOrderImages:      []salesapp.SalesOrderImageDocument{{ID: 3, OrderID: 42, ImageAssetID: 30, IsLatest: true}},
		deliveryNoteDocuments: []salesapp.DeliveryNoteDocument{{ID: 5, OrderID: 42, PDFAssetID: 50, ImageAssetID: 51, IsLatest: true}},
	}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)

	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders/42", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response miniEmployeeOrderDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Documents.SalesOrderPDF.Available || response.Documents.SalesOrderPNG.Available || response.Documents.DeliveryNotePDF.Available || response.Documents.DeliveryNotePNG.Available {
		t.Fatalf("missing files advertised as available: %+v", response.Documents)
	}
}

func TestMiniEmployeeOrderDetailDoesNotReuseInvalidatedDocumentVersions(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42"}}}
	form := salesapp.OrderFormData{EditData: &salesapp.OrderEditData{ID: 42, OrderNo: "SO-42"}}
	sales := &miniEmployeeSalesFake{
		listResult: &result, orderFormResult: &form,
		salesOrderDocuments:   []salesapp.SalesOrderDocument{{ID: 2, OrderID: 42, PDFAssetID: 20, IsLatest: false}},
		salesOrderImages:      []salesapp.SalesOrderImageDocument{{ID: 3, OrderID: 42, ImageAssetID: 30, IsLatest: false}},
		deliveryNoteDocuments: []salesapp.DeliveryNoteDocument{{ID: 5, OrderID: 42, PDFAssetID: 50, ImageAssetID: 51, IsLatest: false}},
	}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders/42", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response miniEmployeeOrderDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Documents.SalesOrderPDF.Available || response.Documents.SalesOrderPNG.Available || response.Documents.DeliveryNotePDF.Available || response.Documents.DeliveryNotePNG.Available || sales.documentLoadCalls != 0 {
		t.Fatalf("invalidated documents reused: %+v loads=%d", response.Documents, sales.documentLoadCalls)
	}
}

func TestMiniEmployeeOrderDocumentDownloadsRevalidateScopeAndReturnWechatFiles(t *testing.T) {
	pdfPath := miniEmployeeDocumentTestFile(t, "sales-order.pdf", "%PDF-1.7 sales")
	pngPath := miniEmployeeDocumentTestFile(t, "sales-order.png", "\x89PNG\r\n\x1a\nimage")
	deliveryPath := miniEmployeeDocumentTestFile(t, "delivery-note.pdf", "%PDF-1.7 delivery")
	deliveryPNGPath := miniEmployeeDocumentTestFile(t, "delivery-note.png", "\x89PNG\r\n\x1a\ndelivery")

	for _, tc := range []struct {
		name        string
		path        string
		contentType string
		filename    string
		body        string
		configure   func(*miniEmployeeSalesFake)
	}{
		{
			name: "sales order pdf", path: "/api/mini/employee/orders/42/documents/sales-order.pdf",
			contentType: "application/pdf", filename: "SO-42-V3.pdf", body: "%PDF-1.7 sales",
			configure: func(sales *miniEmployeeSalesFake) {
				sales.salesOrderPDF = salesapp.SalesOrderDocumentFile{Path: pdfPath, Filename: "SO-42-V3.pdf"}
			},
		},
		{
			name: "sales order png", path: "/api/mini/employee/orders/42/documents/sales-order.png",
			contentType: "image/png", filename: "SO-42-V2.png", body: "\x89PNG\r\n\x1a\nimage",
			configure: func(sales *miniEmployeeSalesFake) {
				sales.salesOrderPNG = salesapp.SalesOrderImageFile{Path: pngPath, Filename: "SO-42-V2.png"}
			},
		},
		{
			name: "delivery note pdf", path: "/api/mini/employee/orders/42/documents/delivery-note.pdf",
			contentType: "application/pdf", filename: "SO-42-DN-V4.pdf", body: "%PDF-1.7 delivery",
			configure: func(sales *miniEmployeeSalesFake) {
				sales.deliveryNotePDF = salesapp.DeliveryNoteDocumentFile{Path: deliveryPath, Filename: "SO-42-DN-V4.pdf"}
			},
		},
		{
			name: "delivery note png", path: "/api/mini/employee/orders/42/documents/delivery-note.png",
			contentType: "image/png", filename: "SO-42-DN-V4.png", body: "\x89PNG\r\n\x1a\ndelivery",
			configure: func(sales *miniEmployeeSalesFake) {
				sales.deliveryNotePNG = salesapp.DeliveryNoteImageFile{Path: deliveryPNGPath, Filename: "SO-42-DN-V4.png"}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42"}}}
			sales := &miniEmployeeSalesFake{listResult: &result}
			tc.configure(sales)
			registerMiniEmployeeAPI(e, employeePortalService(), sales)

			for attempt := 1; attempt <= 2; attempt++ {
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				req.Header.Set(echo.HeaderAuthorization, "Bearer token")
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK || rec.Header().Get(echo.HeaderContentType) != tc.contentType || rec.Body.String() != tc.body {
					t.Fatalf("attempt=%d status=%d content_type=%q body=%q", attempt, rec.Code, rec.Header().Get(echo.HeaderContentType), rec.Body.String())
				}
				_, params, err := mime.ParseMediaType(rec.Header().Get(echo.HeaderContentDisposition))
				if err != nil || params["filename"] != tc.filename {
					t.Fatalf("attempt=%d disposition=%q filename=%q err=%v", attempt, rec.Header().Get(echo.HeaderContentDisposition), params["filename"], err)
				}
			}
			if sales.listCalls != 2 || sales.documentLoadCalls != 2 || sales.documentLoadOrderID != 42 || sales.documentLoadID != 0 || !sales.documentLoadLatest {
				t.Fatalf("scope checks=%d file loads=%d order=%d document=%d latest=%v", sales.listCalls, sales.documentLoadCalls, sales.documentLoadOrderID, sales.documentLoadID, sales.documentLoadLatest)
			}
		})
	}
}

func TestMiniEmployeeOrderDocumentReturnsNotFoundWithoutLoadingOutOfScopeFile(t *testing.T) {
	for _, path := range []string{
		"/api/mini/employee/orders/42/documents/sales-order.pdf",
		"/api/mini/employee/orders/42/documents/sales-order.png",
		"/api/mini/employee/orders/42/documents/delivery-note.pdf",
		"/api/mini/employee/orders/42/documents/delivery-note.png",
	} {
		e := echo.New()
		empty := salesapp.OrderListResult{Rows: []salesapp.OrderRow{}}
		sales := &miniEmployeeSalesFake{listResult: &empty}
		registerMiniEmployeeAPI(e, employeePortalService(), sales)

		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound || sales.documentLoadCalls != 0 {
			t.Fatalf("path=%s status=%d file_loads=%d body=%s", path, rec.Code, sales.documentLoadCalls, rec.Body.String())
		}
	}
}

func TestMiniEmployeeOrderDocumentGenerateReturnsNotFoundBeforeLoadingOutOfScopeFile(t *testing.T) {
	for _, path := range []string{
		"/api/mini/employee/orders/42/documents/sales-order.pdf",
		"/api/mini/employee/orders/42/documents/sales-order.png",
		"/api/mini/employee/orders/42/documents/delivery-note.pdf",
		"/api/mini/employee/orders/42/documents/delivery-note.png",
	} {
		e := echo.New()
		empty := salesapp.OrderListResult{Rows: []salesapp.OrderRow{}}
		sales := &miniEmployeeSalesFake{listResult: &empty}
		registerMiniEmployeeAPI(e, employeePortalService(), sales)

		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound || sales.documentLoadCalls != 0 || sales.generateCalls != 0 {
			t.Fatalf("path=%s status=%d file_loads=%d generate=%d body=%s", path, rec.Code, sales.documentLoadCalls, sales.generateCalls, rec.Body.String())
		}
	}
}

func TestMiniEmployeeOrderDocumentGenerateRequiresWritePermissionBeforeScopeOrFiles(t *testing.T) {
	e := echo.New()
	portal := fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 7, EmployeeName: "销售甲",
		Roles: []string{"sales"}, Permissions: []string{"orders.read"},
	}}
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, portal, sales)

	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders/42/documents/sales-order.pdf", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || sales.listCalls != 0 || sales.documentLoadCalls != 0 || sales.generateCalls != 0 {
		t.Fatalf("status=%d scope=%d load=%d generate=%d body=%s", rec.Code, sales.listCalls, sales.documentLoadCalls, sales.generateCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderDocumentMapsMissingLatestFileToNotFound(t *testing.T) {
	for _, path := range []string{
		"/api/mini/employee/orders/42/documents/sales-order.pdf",
		"/api/mini/employee/orders/42/documents/sales-order.png",
		"/api/mini/employee/orders/42/documents/delivery-note.pdf",
		"/api/mini/employee/orders/42/documents/delivery-note.png",
	} {
		e := echo.New()
		result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42"}}}
		sales := &miniEmployeeSalesFake{listResult: &result, documentLoadError: errors.New("private storage path missing")}
		registerMiniEmployeeAPI(e, employeePortalService(), sales)

		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound || !json.Valid(rec.Body.Bytes()) || strings.Contains(rec.Body.String(), "private storage") {
			t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestMiniEmployeeOrderDocumentGenerateUsesMiniEmployeeActorAndReusesLatest(t *testing.T) {
	for _, tc := range []struct {
		name             string
		path             string
		generated        string
		expectedFilename string
		configureResult  func(*miniEmployeeSalesFake)
		assertCmd        func(*testing.T, *miniEmployeeSalesFake)
	}{
		{
			name: "sales order pdf", path: "/api/mini/employee/orders/42/documents/sales-order.pdf", generated: `"path":"/api/mini/employee/orders/42/documents/sales-order.pdf"`, expectedFilename: "销售单-SO-42-V4.pdf",
			configureResult: func(sales *miniEmployeeSalesFake) { sales.generateSalesOrderResult.Document.VersionNo = 4 },
			assertCmd: func(t *testing.T, sales *miniEmployeeSalesFake) {
				if sales.generateSalesOrderPDF.OrderID != 42 || sales.generateSalesOrderPDF.Actor != "mini-employee:7:销售甲" {
					t.Fatalf("command=%+v", sales.generateSalesOrderPDF)
				}
			},
		},
		{
			name: "sales order png", path: "/api/mini/employee/orders/42/documents/sales-order.png", generated: `"path":"/api/mini/employee/orders/42/documents/sales-order.png"`, expectedFilename: "销售单-SO-42-V4.png",
			configureResult: func(sales *miniEmployeeSalesFake) { sales.generateSalesImageResult.Document.VersionNo = 4 },
			assertCmd: func(t *testing.T, sales *miniEmployeeSalesFake) {
				if sales.generateSalesOrderPNG.OrderID != 42 || sales.generateSalesOrderPNG.Actor != "mini-employee:7:销售甲" {
					t.Fatalf("command=%+v", sales.generateSalesOrderPNG)
				}
			},
		},
		{
			name: "delivery note pdf", path: "/api/mini/employee/orders/42/documents/delivery-note.pdf", generated: `"path":"/api/mini/employee/orders/42/documents/delivery-note.pdf"`, expectedFilename: "发货单-SO-42-V4.pdf",
			configureResult: func(sales *miniEmployeeSalesFake) { sales.generateDeliveryResult.Document.VersionNo = 4 },
			assertCmd: func(t *testing.T, sales *miniEmployeeSalesFake) {
				if sales.generateDeliveryNotePDF.OrderID != 42 || sales.generateDeliveryNotePDF.Actor != "mini-employee:7:销售甲" {
					t.Fatalf("command=%+v", sales.generateDeliveryNotePDF)
				}
			},
		},
		{
			name: "delivery note png", path: "/api/mini/employee/orders/42/documents/delivery-note.png", generated: `"path":"/api/mini/employee/orders/42/documents/delivery-note.png"`, expectedFilename: "发货单-SO-42-V4.png",
			configureResult: func(sales *miniEmployeeSalesFake) { sales.generateDeliveryResult.Document.VersionNo = 4 },
			assertCmd: func(t *testing.T, sales *miniEmployeeSalesFake) {
				if sales.generateDeliveryNotePDF.OrderID != 42 || sales.generateDeliveryNotePDF.Actor != "mini-employee:7:销售甲" {
					t.Fatalf("command=%+v", sales.generateDeliveryNotePDF)
				}
			},
		},
	} {
		t.Run(tc.name+" generates when missing", func(t *testing.T) {
			e := echo.New()
			result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42"}}}
			sales := &miniEmployeeSalesFake{listResult: &result, documentLoadError: errors.New("not found")}
			tc.configureResult(sales)
			registerMiniEmployeeAPI(e, employeePortalService(), sales)

			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.generated) ||
				!strings.Contains(rec.Body.String(), `"filename":"`+tc.expectedFilename+`"`) ||
				!strings.Contains(rec.Body.String(), `"version_no":4`) || !strings.Contains(rec.Body.String(), `"generated":true`) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if sales.listCalls != 1 || sales.documentLoadCalls != 1 || sales.generateCalls != 1 {
				t.Fatalf("scope=%d load=%d generate=%d", sales.listCalls, sales.documentLoadCalls, sales.generateCalls)
			}
			tc.assertCmd(t, sales)
		})

		t.Run(tc.name+" reuses latest", func(t *testing.T) {
			e := echo.New()
			result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42"}}}
			sales := &miniEmployeeSalesFake{
				listResult: &result,
				salesOrderPDF: salesapp.SalesOrderDocumentFile{
					Document: salesapp.SalesOrderDocument{VersionNo: 3}, Path: miniEmployeeDocumentTestFile(t, "existing-sales-order.pdf", "%PDF"), Filename: "销售单-SO-42-V3.pdf",
				},
				salesOrderPNG: salesapp.SalesOrderImageFile{
					Document: salesapp.SalesOrderImageDocument{VersionNo: 3}, Path: miniEmployeeDocumentTestFile(t, "existing-sales-order.png", "\x89PNG"), Filename: "销售单-SO-42-V3.png",
				},
				deliveryNotePDF: salesapp.DeliveryNoteDocumentFile{
					Document: salesapp.DeliveryNoteDocument{VersionNo: 3}, Path: miniEmployeeDocumentTestFile(t, "existing-delivery-note.pdf", "%PDF"), Filename: "发货单-SO-42-V3.pdf",
				},
				deliveryNotePNG: salesapp.DeliveryNoteImageFile{
					Document: salesapp.DeliveryNoteDocument{VersionNo: 3}, Path: miniEmployeeDocumentTestFile(t, "existing-delivery-note.png", "\x89PNG"), Filename: "发货单-SO-42-V3.png",
				},
			}
			registerMiniEmployeeAPI(e, employeePortalService(), sales)

			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version_no":3`) ||
				!strings.Contains(rec.Body.String(), `-V3.`) || !strings.Contains(rec.Body.String(), `"generated":false`) || sales.generateCalls != 0 {
				t.Fatalf("status=%d generate=%d body=%s", rec.Code, sales.generateCalls, rec.Body.String())
			}
		})
	}
}

func miniEmployeeDocumentTestFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestMiniEmployeeOrderFormSeparatesProductAndSpecsAndReturnsShippingDefaults(t *testing.T) {
	e := echo.New()
	registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form?customer_id=8", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
	for _, want := range []string{
		`"name":"乌拉嘎"`,
		`"specs":[`,
		`"spec_label":"227g"`,
		`"sku_code":"WLG-227"`,
		`"products":[`,
		`"receiver_name":"收货人A"`,
		`"receiver_phone":"13800000000"`,
		`"receiver_address":"上海市测试路1号"`,
		`"receiver_company":"客户A公司"`,
		`"can_maintain":true`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("status=%d body=%s missing %s", rec.Code, body, want)
		}
	}
	if strings.Contains(body, `"name":"乌拉嘎 227g"`) {
		var response struct {
			ProductFamilies []struct {
				Name string `json:"name"`
			} `json:"product_families"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		for _, family := range response.ProductFamilies {
			if family.Name == "乌拉嘎 227g" {
				t.Fatalf("family product name must not carry specification: %s", body)
			}
		}
	}
	var response struct {
		Customers []struct {
			Py  string `json:"py"`
			Pyi string `json:"pyi"`
		} `json:"customers"`
		ProductFamilies []struct {
			Py    string `json:"py"`
			Pyi   string `json:"pyi"`
			Specs []struct {
				Py  string `json:"py"`
				Pyi string `json:"pyi"`
			} `json:"specs"`
		} `json:"product_families"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Customers) != 1 || response.Customers[0].Py == "" || response.Customers[0].Pyi == "" {
		t.Fatalf("customer search fields missing: %#v", response.Customers)
	}
	if len(response.ProductFamilies) != 1 || response.ProductFamilies[0].Py == "" || response.ProductFamilies[0].Pyi == "" || len(response.ProductFamilies[0].Specs) != 1 || response.ProductFamilies[0].Specs[0].Py == "" || response.ProductFamilies[0].Specs[0].Pyi == "" {
		t.Fatalf("product search fields missing: %#v", response.ProductFamilies)
	}
}

func TestMiniEmployeeOrderFormRequiresCustomerBeforeReturningProducts(t *testing.T) {
	e := echo.New()
	registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Products        []salesapp.ProductOption `json:"products"`
		ProductFamilies []map[string]any         `json:"product_families"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Products) != 0 || len(response.ProductFamilies) != 0 {
		t.Fatalf("unscoped form leaked products: %+v", response)
	}
}

func TestMiniEmployeeOrderFormRejectsUnknownCustomer(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form?customer_id=999", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "客户不存在") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniEmployeeOrderFormKeepsOnlyCurrentDefaultPublishedCatalog(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{orderFormResult: &salesapp.OrderFormData{
		Today:     "2026-08-02",
		Customers: []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products: []salesapp.ProductOption{
			{ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 227g", ProductKind: "roasted_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{
				{ID: 11, PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`},
				{ID: 10, PublicationID: 900, PublicationVersionNo: "V9.0", ListType: "commercial", UnitPrice: 66, PriceSourceJSON: `{"publication_id":900,"list_type":"commercial"}`},
			}},
			{ID: 626, SKUID: 626, ParentProductID: 626, ParentProductName: "红岩", Name: "红岩", ProductKind: "roasted_bean", Visibility: "public"},
			{ID: 888, SKUID: 888, ParentProductID: 626, ParentProductName: "红岩", Name: "红岩 227g", ProductKind: "roasted_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{{ID: 20, PublicationID: 900, PublicationVersionNo: "V9.0", ListType: "commercial", UnitPrice: 60, PriceSourceJSON: `{"publication_id":900,"list_type":"commercial"}`}}},
			{ID: 777, SKUID: 777, ParentProductID: 777, ParentProductName: "旧生豆", Name: "旧生豆", ProductKind: "green_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{{ID: 30, PublicationID: 300, PublicationVersionNo: "G3", ListType: "green", UnitPrice: 40, PriceSourceJSON: `{"publication_id":300,"list_type":"green"}`}}},
		},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{
			{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true},
			{CustomerID: 8, ListType: "commercial", ID: 900, VersionNo: "V9.0", IsDefault: false},
		},
		CustomerPublicUsages: []salesapp.CustomerPublicUsageOption{{CustomerID: 8, UsePublicSKU: true}},
	}}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form?customer_id=8", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", got)
	}
	var response struct {
		Products        []salesapp.ProductOption `json:"products"`
		ProductFamilies []struct {
			Name  string           `json:"name"`
			Specs []map[string]any `json:"specs"`
		} `json:"product_families"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Products) != 1 || response.Products[0].ID != 551 || len(response.Products[0].Tiers) != 1 || response.Products[0].Tiers[0].PublicationID != 901 {
		t.Fatalf("products=%+v", response.Products)
	}
	if len(response.ProductFamilies) != 1 || response.ProductFamilies[0].Name != "乌拉嘎" || len(response.ProductFamilies[0].Specs) != 1 || strings.Contains(rec.Body.String(), "红岩") || strings.Contains(rec.Body.String(), "旧生豆") {
		t.Fatalf("families=%+v body=%s", response.ProductFamilies, rec.Body.String())
	}
}

func TestMiniEmployeeOrderFormUsesCutoverBOMSpecsAndHidesLegacyChildren(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{orderFormResult: &salesapp.OrderFormData{
		Today:     "2026-08-17",
		Customers: []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products: []salesapp.ProductOption{
			{ID: 10, SKUID: 10, ParentProductID: 10, ParentProductName: "商品A", Name: "商品A", ProductKind: "roasted_bean", Visibility: "public"},
			{ID: 11, SKUID: 11, ParentProductID: 10, ParentProductName: "商品A", Name: "商品A 227g", SKUName: "旧227g", SpecLabel: "227g", ProductKind: "roasted_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{{ID: 11, PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68}}},
			{ID: 12, SKUID: 12, ParentProductID: 10, ParentProductName: "商品A", Name: "商品A 454g", SKUName: "旧454g", SpecLabel: "454g", ProductKind: "roasted_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{{ID: 12, PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 98}}},
		},
		ProductBOMSpecOptions: []salesapp.ProductBOMSpecOption{
			{
				ParentProductID: 10, LegacyChildProductID: 11, BomID: 100, BomSpecID: 101, BomVariantID: 1001,
				SpecCode: "BOM-SPEC-000101", Barcode: "6900000000101", SpecKey: "bag-227", SpecName: "227g袋", InventoryUnit: "袋", Published: true, IsDefault: true, SortOrder: 1, MigrationState: "cutover",
				WriteProductID: 10, WriteBomSpecID: 101, WriteBomVariantID: 1001,
				Tiers: []salesapp.ProductTierOption{{ID: 11, PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, ParentProductID: 10, BomSpecID: 101, BomVariantID: 1001}},
			},
			{
				ParentProductID: 10, BomID: 100, BomSpecID: 102, BomVariantID: 1002,
				SpecCode: "BOM-SPEC-000102", Barcode: "6900000000102", SpecKey: "gift-2", SpecName: "双袋礼盒", InventoryUnit: "盒", Published: true, SortOrder: 2, MigrationState: "cutover",
				WriteProductID: 10, WriteBomSpecID: 102, WriteBomVariantID: 1002,
				Tiers: []salesapp.ProductTierOption{{ID: 22, PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 138, ParentProductID: 10, BomSpecID: 102, BomVariantID: 1002}},
			},
		},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
		CustomerPublicUsages:   []salesapp.CustomerPublicUsageOption{{CustomerID: 8, UsePublicSKU: true}},
	}}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form?customer_id=8", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Products []salesapp.ProductOption `json:"products"`
		Families []struct {
			ParentProductID int64  `json:"parent_product_id"`
			MigrationState  string `json:"migration_state"`
			Specs           []struct {
				ProductID     int64                        `json:"product_id"`
				BomSpecID     int64                        `json:"bom_spec_id"`
				BomVariantID  int64                        `json:"bom_variant_id"`
				SpecCode      string                       `json:"spec_code"`
				Barcode       string                       `json:"barcode"`
				SpecKey       string                       `json:"spec_key"`
				SpecLabel     string                       `json:"spec_label"`
				InventoryUnit string                       `json:"inventory_unit"`
				Tiers         []salesapp.ProductTierOption `json:"tiers"`
			} `json:"specs"`
		} `json:"product_families"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Products) != 0 {
		t.Fatalf("cutover legacy products leaked: %+v", response.Products)
	}
	if len(response.Families) != 1 || response.Families[0].ParentProductID != 10 || response.Families[0].MigrationState != "cutover" {
		t.Fatalf("families=%+v body=%s", response.Families, rec.Body.String())
	}
	if len(response.Families[0].Specs) != 2 {
		t.Fatalf("specs=%+v body=%s", response.Families[0].Specs, rec.Body.String())
	}
	first, second := response.Families[0].Specs[0], response.Families[0].Specs[1]
	if first.ProductID != 10 || first.BomSpecID != 101 || first.BomVariantID != 1001 || first.SpecCode != "BOM-SPEC-000101" || first.Barcode != "6900000000101" || first.SpecKey != "bag-227" || first.InventoryUnit != "袋" || len(first.Tiers) != 1 || first.Tiers[0].UnitPrice != 68 {
		t.Fatalf("first spec=%+v", first)
	}
	if second.ProductID != 10 || second.BomSpecID != 102 || second.BomVariantID != 1002 || second.SpecKey != "gift-2" || second.SpecLabel != "双袋礼盒" || second.InventoryUnit != "盒" || len(second.Tiers) != 1 || second.Tiers[0].UnitPrice != 138 {
		t.Fatalf("new unmapped spec=%+v", second)
	}
}

func TestMiniEmployeePrepareCurrentCatalogAcceptsCutoverParentAndBOMSpec(t *testing.T) {
	parentID := int64(10)
	form := salesapp.OrderFormData{
		Customers: []salesapp.CustomerOption{{ID: 8}},
		Products: []salesapp.ProductOption{
			{ID: 10, ParentProductID: 10, CustomerID: 8, Name: "初晓", ProductKind: "roasted_bean"},
			{ID: 11, SKUID: 11, ParentProductID: 10, CustomerID: 8, Name: "初晓旧规格", ProductKind: "roasted_bean"},
		},
		ProductBOMSpecOptions: []salesapp.ProductBOMSpecOption{{
			ParentProductID: 10, BomID: 100, BomVersionID: 1000, BomSpecID: 101, BomVariantID: 1001,
			SpecCode: "BOM-SPEC-000101", SpecKey: "bag-227", SpecName: "227g袋", InventoryUnit: "袋",
			Published: true, IsDefault: true, MigrationState: "cutover", WriteProductID: 10, WriteBomSpecID: 101, WriteBomVariantID: 1001,
			Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"source":"price_list"}`}},
		}},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
	}
	cmd := salesapp.SaveOrderCommand{CustomerID: 8, Items: []salesapp.OrderItemCommand{{
		ProductID: &parentID, ParentProductID: 10, BomSpecID: 101, BomVariantID: 1001,
		Units: 2, Unit: "袋", SalesUnit: "袋", BeanListPublicationID: 901, BeanListVersionNo: "V9.1",
	}}}

	message, err := miniEmployeePrepareCurrentCatalogFromForm(form, &cmd)
	if err != nil || message != "" {
		t.Fatalf("prepare cutover current catalog message=%q err=%v", message, err)
	}
	item := cmd.Items[0]
	if item.ProductID == nil || *item.ProductID != 10 || item.ParentProductID != 10 || item.BomSpecID != 101 || item.BomVariantID != 1001 {
		t.Fatalf("prepared cutover identity = %+v", item)
	}
	if item.SpecG != 0 || item.Unit != "袋" || item.SalesUnit != "袋" || item.ProductKind != "roasted_bean" || item.PriceSourceJSON == "" {
		t.Fatalf("prepared direct specification pricing = %+v", item)
	}
}

func TestMiniEmployeeSaveOrderCommandKeepsBOMSpecIdentity(t *testing.T) {
	cmd, err := miniEmployeeSaveOrderCommand(miniEmployeeOrderRequest{
		OrderDate:  "2026-08-17",
		CustomerID: 8,
		Items: []miniEmployeeOrderItemRequest{{
			ProductID: 10, ParentProductID: 10, BomSpecID: 101, BomVariantID: 1001,
			Qty: 2, Unit: "袋", SalesUnit: "袋",
		}},
	}, "employee:7", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Items) != 1 || cmd.Items[0].BomSpecID != 101 || cmd.Items[0].BomVariantID != 1001 {
		t.Fatalf("mini order command lost BOM spec identity: %+v", cmd.Items)
	}
}

func TestMiniEmployeeOrderDetailReturnsFrozenBOMSpecIdentity(t *testing.T) {
	detail := miniEmployeeOrderDetail(salesapp.OrderRow{ID: 42}, salesapp.OrderFormData{EditData: &salesapp.OrderEditData{
		ID: 42,
		Items: []salesapp.OrderEditItem{{
			ItemID: 1, ProductID: 10, BomSpecID: 101, BomVariantID: 1001, BomSpecKey: "bag-227", BomSpecName: "227g袋", Unit: "袋",
		}},
	}})
	if len(detail.Items) != 1 || detail.Items[0].ParentProductID != 10 || detail.Items[0].BomSpecID != 101 || detail.Items[0].BomVariantID != 1001 || detail.Items[0].BomSpecName != "227g袋" {
		t.Fatalf("mini detail BOM spec identity = %+v", detail.Items)
	}
}

func TestMiniEmployeeProductFamiliesFallBackToParentAndSKUFields(t *testing.T) {
	families := salesapp.BuildOrderProductFamilies([]salesapp.ProductOption{
		{ID: 550, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎"},
		{ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 227g", SpecLabel: "227g", NetContentQty: 227, NetContentUnit: "g"},
		{ID: 552, SKUID: 552, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 454g", SpecLabel: "454g", NetContentQty: 454, NetContentUnit: "g"},
	})
	if len(families) != 1 || families[0]["name"] != "乌拉嘎" {
		t.Fatalf("families=%#v", families)
	}
	specs, _ := families[0]["specs"].([]map[string]any)
	if len(specs) != 2 || specs[0]["spec_label"] != "227g" || specs[1]["spec_label"] != "454g" {
		t.Fatalf("specs=%#v", specs)
	}
}

func TestMiniEmployeeOrderFormWithoutTokenDoesNotCallSalesOrLeakMasterData(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || sales.orderFormCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, sales.orderFormCalls, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("unauthorized response must be written exactly once: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "客户A") || strings.Contains(rec.Body.String(), "乌拉嘎") || strings.Contains(rec.Body.String(), "product_families") {
		t.Fatalf("unauthorized response leaked order form data: %s", rec.Body.String())
	}
}

func TestMiniEmployeeOrderListAndCreateWithoutTokenStopBeforeSales(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/mini/employee/orders"},
		{method: http.MethodGet, path: "/api/mini/employee/orders/42"},
		{method: http.MethodGet, path: "/api/mini/employee/orders/42/documents/sales-order.pdf"},
		{method: http.MethodGet, path: "/api/mini/employee/orders/42/documents/delivery-note.png"},
		{method: http.MethodPost, path: "/api/mini/employee/orders/42/documents/sales-order.png"},
		{method: http.MethodPost, path: "/api/mini/employee/orders/42/documents/delivery-note.pdf"},
		{method: http.MethodPost, path: "/api/mini/employee/orders", body: `{}`},
		{method: http.MethodPut, path: "/api/mini/employee/orders/42", body: `{}`},
	} {
		req := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || !json.Valid(rec.Body.Bytes()) {
			t.Fatalf("%s %s status=%d body=%s", request.method, request.path, rec.Code, rec.Body.String())
		}
	}
	if sales.listCalls != 0 || sales.saveCalls != 0 || sales.orderFormCalls != 0 || sales.documentLoadCalls != 0 || sales.generateCalls != 0 {
		t.Fatalf("sales called after auth failure: form=%d list=%d save=%d document_load=%d generate=%d", sales.orderFormCalls, sales.listCalls, sales.saveCalls, sales.documentLoadCalls, sales.generateCalls)
	}
}

func TestMiniEmployeeOrderFormWithExpiredTokenDoesNotCallSalesOrLeakMasterData(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, fakeService{err: customerportalapp.ErrMiniSessionNotFound}, sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer expired")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || sales.orderFormCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, sales.orderFormCalls, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("expired-token response must be written exactly once: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "客户A") || strings.Contains(rec.Body.String(), "乌拉嘎") || strings.Contains(rec.Body.String(), "product_families") {
		t.Fatalf("expired-token response leaked order form data: %s", rec.Body.String())
	}
}

func TestMiniEmployeeOrderFormWithoutPermissionDoesNotCallSales(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	portal := fakeService{me: customerportalapp.CurrentContext{AccountType: "employee", Roles: []string{"sales"}}}
	registerMiniEmployeeAPI(e, portal, sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || sales.orderFormCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, sales.orderFormCalls, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) || strings.Contains(rec.Body.String(), "product_families") {
		t.Fatalf("forbidden response must be one error object without master data: %s", rec.Body.String())
	}
}

func TestMiniEmployeeOrderFormAllowsAdministratorWithPermission(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	portal := fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", Roles: []string{"admin"}, Permissions: []string{"orders.write"},
	}}
	registerMiniEmployeeAPI(e, portal, sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || sales.orderFormCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, sales.orderFormCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderFormHidesMaintainActionWithoutCustomerPermissions(t *testing.T) {
	e := echo.New()
	portal := fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 7, Roles: []string{"sales"}, Permissions: []string{"orders.write"},
	}}
	registerMiniEmployeeAPI(e, portal, &miniEmployeeSalesFake{})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"can_maintain":false`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniEmployeeOrderCreateLeavesResponsibilityToCustomerArchive(t *testing.T) {
	e := echo.New()
	form := salesapp.OrderFormData{
		Customers:              []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products:               []salesapp.ProductOption{{ID: 3, ProductKind: "roasted_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`}}}},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
	}
	sales := &miniEmployeeSalesFake{orderFormResult: &form}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-07-30","customer_id":8,"items":[{"product_id":3,"name":"咖啡豆","qty":2,"spec_g":454,"unit_price":68}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"order_no":"SO-9"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sales.save.ResponsibleID != 0 || sales.save.ResponsibleType != "" || sales.save.DraftEmployeeID != 7 || !sales.save.RequireCurrentDefaultPublications || !sales.save.OrderDate.Equal(time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("save=%+v", sales.save)
	}
}

func TestMiniEmployeeOrderCreateAllowsBlankTrackingNumberForUnshippedDraft(t *testing.T) {
	e := echo.New()
	form := salesapp.OrderFormData{
		Customers:              []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products:               []salesapp.ProductOption{{ID: 3, ProductKind: "roasted_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`}}}},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
	}
	sales := &miniEmployeeSalesFake{orderFormResult: &form}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-08-09","customer_id":8,"ship_status_id":1,"ship_tracking_no":"","items":[{"product_id":3,"qty":2,"spec_g":454,"unit_price":68}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sales.saveCalls != 1 || sales.save.ShipStatusID != 1 || sales.save.ShipTrackingNo != "" || sales.save.DraftEmployeeID != 7 {
		t.Fatalf("save calls=%d command=%+v", sales.saveCalls, sales.save)
	}
}

func TestMiniEmployeeOrderCreateMapsEverySubmittedItem(t *testing.T) {
	e := echo.New()
	form := salesapp.OrderFormData{
		Customers: []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products: []salesapp.ProductOption{
			{ID: 3, CustomerID: 8, CustomerProductAliasID: 201, ProductKind: "roasted_bean", Visibility: "customer_alias", Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`}}},
			{ID: 4, CustomerID: 8, CustomerProductAliasID: 202, ProductKind: "roasted_bean", Visibility: "customer_alias", Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 88, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`}}},
		},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
	}
	sales := &miniEmployeeSalesFake{orderFormResult: &form}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-08-01","customer_id":8,"items":[{"product_id":3,"customer_product_alias_id":201,"name":"咖啡豆A","qty":2,"spec_g":227,"unit_price":68,"price_override":true},{"product_id":4,"customer_product_alias_id":202,"name":"咖啡豆B","qty":3,"spec_g":454,"unit_price":88,"price_override":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sales.saveCalls != 1 || len(sales.save.Items) != 2 || sales.save.Items[0].Units != 2 || sales.save.Items[1].Units != 3 || sales.save.Items[0].CustomerProductAliasID != 201 || sales.save.Items[1].CustomerProductAliasID != 202 || sales.save.Items[0].ManualPrice == nil || *sales.save.Items[0].ManualPrice != 68 || sales.save.Items[1].ManualPrice == nil || *sales.save.Items[1].ManualPrice != 88 {
		t.Fatalf("save calls=%d items=%+v", sales.saveCalls, sales.save.Items)
	}
}

func TestMiniEmployeeOrderCreateRejectsProductOutsideCurrentDefaultCatalog(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-08-02","customer_id":8,"items":[{"product_id":626,"parent_product_id":626,"name":"红岩","qty":1,"spec_g":227,"unit_price":68,"price_override":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || sales.saveCalls != 0 || !strings.Contains(rec.Body.String(), "当前价格表") {
		t.Fatalf("status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderCreateDoesNotExposeUnknownPersistenceError(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{saveError: errors.New(`ERROR: relation private_table does not exist (SQLSTATE 42P01)`)}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-08-02","customer_id":8,"items":[{"product_id":551,"parent_product_id":550,"qty":1,"spec_g":227,"bean_list_publication_id":901,"bean_list_version_no":"V9.1","price_source_json":"{\"publication_id\":901,\"list_type\":\"commercial\"}"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || sales.saveCalls != 1 || strings.Contains(rec.Body.String(), "private_table") || strings.Contains(rec.Body.String(), "SQLSTATE") {
		t.Fatalf("status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderCreateMapsInvalidEditableAmountsToSafeBadRequest(t *testing.T) {
	form := salesapp.OrderFormData{
		Customers:              []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products:               []salesapp.ProductOption{{ID: 551, ParentProductID: 550, ProductKind: "roasted_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`}}}},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
	}
	tests := []struct {
		name      string
		field     string
		saveError error
		want      string
	}{
		{name: "negative shipping", field: `"shipping_amount":-0.01`, saveError: salesapp.ErrInvalidShippingAmount, want: "运费金额不能小于 0"},
		{name: "negative discount", field: `"discount_amount":-0.01`, saveError: salesapp.ErrInvalidDiscountAmount, want: "整单优惠不能小于 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			sales := &miniEmployeeSalesFake{orderFormResult: &form, saveError: tt.saveError}
			registerMiniEmployeeAPI(e, employeePortalService(), sales)
			body := `{"order_date":"2026-08-02","customer_id":8,` + tt.field + `,"items":[{"product_id":551,"parent_product_id":550,"qty":1,"spec_g":227,"bean_list_publication_id":901,"bean_list_version_no":"V9.1"}]}`
			req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders", strings.NewReader(body))
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || sales.saveCalls != 1 || !strings.Contains(rec.Body.String(), tt.want) {
				t.Fatalf("status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
			}
		})
	}
}

func TestMiniEmployeeOrderUpdateMapsFullPreProductionCommand(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", CustomerID: 8}}}
	form := salesapp.OrderFormData{
		Customers: []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products: []salesapp.ProductOption{
			{ID: 551, SKUID: 551, ParentProductID: 550, CustomerID: 8, CustomerProductAliasID: 201, ProductKind: "roasted_bean", Visibility: "customer_alias", Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`}}},
			{ID: 552, SKUID: 552, ParentProductID: 550, ProductKind: "roasted_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 88, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`}}},
		},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
	}
	sales := &miniEmployeeSalesFake{listResult: &result, orderFormResult: &form}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{
		"order_date":"2026-08-02","customer_id":8,"source_id":1,"order_type_id":2,"pay_status_id":3,"ship_status_id":4,
		"shipping_amount":12.5,"discount_amount":3.5,
		"receiver_name":"新收件人","receiver_phone":"13900000000","receiver_address":"杭州市新地址","receiver_company":"新公司","notes":"修改备注",
		"items":[
			{"product_id":551,"parent_product_id":550,"customer_product_alias_id":201,"name":"客户乌拉嘎","qty":2,"unit":"袋","spec_g":227,"product_kind":"roasted_bean","sales_unit":"bag","unit_price":68,"bean_list_publication_id":901,"bean_list_version_no":"V9.1","price_source_json":"{\"publication_id\":901,\"list_type\":\"commercial\"}","price_override":false},
			{"product_id":552,"parent_product_id":550,"name":"乌拉嘎 454g","qty":1,"unit":"袋","spec_g":454,"product_kind":"roasted_bean","sales_unit":"bag","unit_price":88,"bean_list_publication_id":901,"bean_list_version_no":"V9.1","price_override":true}
		]
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"order_id":9`) || !strings.Contains(rec.Body.String(), `"order_no":"SO-9"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	cmd := sales.save
	if sales.saveCalls != 1 || cmd.EditID != 42 || !cmd.RequirePreProductionEdit || !cmd.RequireCurrentDefaultPublications || cmd.DraftEmployeeID != 0 || cmd.Actor != "mini-employee:7:销售甲" {
		t.Fatalf("save calls=%d command=%+v", sales.saveCalls, cmd)
	}
	if cmd.ShippingAmount != 12.5 || cmd.DiscountAmount != 3.5 || cmd.ReceiverName != "新收件人" || cmd.ReceiverPhone != "13900000000" || cmd.ReceiverAddress != "杭州市新地址" || cmd.ReceiverCompany != "新公司" || cmd.Notes != "修改备注" {
		t.Fatalf("header command=%+v", cmd)
	}
	if len(cmd.Items) != 2 || cmd.Items[0].ProductID == nil || *cmd.Items[0].ProductID != 551 || cmd.Items[0].ParentProductID != 550 || cmd.Items[0].CustomerProductAliasID != 201 || cmd.Items[0].BeanListPublicationID != 901 || cmd.Items[0].BeanListVersionNo != "V9.1" || cmd.Items[0].PriceSourceJSON == "" {
		t.Fatalf("first item=%+v", cmd.Items)
	}
	if cmd.Items[0].ManualPrice != nil || cmd.Items[1].ManualPrice == nil || *cmd.Items[1].ManualPrice != 88 {
		t.Fatalf("price override mapping=%+v", cmd.Items)
	}
}

func TestMiniEmployeeOrderUpdatePreservesEveryHiddenHeaderAndItemField(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", CustomerID: 8}}}
	form := salesapp.OrderFormData{
		Customers: []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products: []salesapp.ProductOption{{
			ID: 551, SKUID: 551, ParentProductID: 550, ProductKind: "roasted_bean", Visibility: "public",
			Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`}},
		}},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
		EditData: &salesapp.OrderEditData{
			ID: 42, EditRevision: "rev-hidden", CustomerID: 8, DocumentDate: "2026-07-31", OrderDate: "2026-07-30", SourceID: 11, OrderTypeID: 12,
			PayStatusID: 13, PaymentMethod: "bank_transfer", ShipStatusID: 14,
			ShipMethod: "sf_large", ShipTrackingNo: "SF123456", LogisticsCompanyID: 15, LogisticsProductID: 16,
			PaymentGoodsAmount: "321.50", PaymentShippingAmount: "18.25", PaymentVoucherAssetID: 17,
			ResponsibleType: "employee", ResponsibleID: 18, ResponsibleName: "原负责人",
			PortalServiceCode: "product_order", SourceWarehouse: "finished_goods",
			BeanListPublicationID: 901, BeanListVersionNo: "V9.1",
			RoundToInt: true, ExpressFee: "23.80",
			OutsourceMaterialFee: "1.10", OutsourceRoastFee: "2.20", OutsourcePackagingFee: "3.30",
			OutsourceManualFee: "4.40", OutsourceTaxFee: "5.50", OutsourceOtherFee: "6.60",
			Items: []salesapp.OrderEditItem{{
				ItemID: 51, ProductID: 552, CustomerProductAliasID: 0, BeanListPublicationID: 901, BeanListVersionNo: "V9.1",
				Note: "保留行备注", DiscountType: "percent", DiscountValue: "7.5",
				CustomerProductDisplayNameSnapshot: "旧客户商品名", CustomerItemCodeSnapshot: "OLD-551", BrandNameSnapshot: "旧品牌",
				ProductCodeSnapshot: "SKU-551", ProductNameSnapshot: "乌拉嘎 227g",
			}},
		},
	}
	sales := &miniEmployeeSalesFake{listResult: &result, orderFormResult: &form}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{
		"edit_revision":"rev-hidden","order_date":"2026-08-02","customer_id":8,
		"shipping_amount":12.5,"discount_amount":3.5,
		"receiver_name":"新收件人","receiver_phone":"13900000000","receiver_address":"杭州市新地址","receiver_company":"新公司","notes":"新订单备注",
		"items":[{"product_id":551,"parent_product_id":550,"qty":2,"unit":"袋","spec_g":227,"sales_unit":"bag","unit_price":68,"bean_list_publication_id":901,"bean_list_version_no":"V9.1","price_source_json":"{\"publication_id\":901,\"list_type\":\"commercial\"}"}]
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	cmd := sales.save
	if cmd.ExpectedEditRevision != "rev-hidden" {
		t.Fatalf("repository edit revision=%q", cmd.ExpectedEditRevision)
	}
	if cmd.DocumentDate.Format("2006-01-02") != "2026-07-31" || cmd.OrderDate.Format("2006-01-02") != "2026-08-02" {
		t.Fatalf("document/order dates were not separated: document=%s order=%s", cmd.DocumentDate.Format("2006-01-02"), cmd.OrderDate.Format("2006-01-02"))
	}
	if cmd.SourceID != 11 || cmd.OrderTypeID != 12 || cmd.PayStatusID != 13 || cmd.PaymentMethod != "bank_transfer" || cmd.ShipStatusID != 14 || cmd.ShipMethod != "sf_large" || cmd.ShipTrackingNo != "SF123456" {
		t.Fatalf("status/payment/shipping fields lost: %+v", cmd)
	}
	if cmd.LogisticsCompanyID != 15 || cmd.LogisticsProductID != 16 || cmd.PaymentGoodsAmount != 321.5 || cmd.PaymentShippingAmount != 18.25 || cmd.PaymentVoucherAssetID != 17 {
		t.Fatalf("logistics/payment fields lost: %+v", cmd)
	}
	if !cmd.RoundToInt || cmd.ExpressFee != "23.80" || cmd.OutsourceMaterialFee != 1.1 || cmd.OutsourceRoastFee != 2.2 || cmd.OutsourcePackagingFee != 3.3 || cmd.OutsourceManualFee != 4.4 || cmd.OutsourceTaxFee != 5.5 || cmd.OutsourceOtherFee != 6.6 {
		t.Fatalf("rounding/fee fields lost: %+v", cmd)
	}
	if cmd.ResponsibleType != "employee" || cmd.ResponsibleID != 18 || cmd.ResponsibleName != "原负责人" || !cmd.PreserveResponsibleSnapshot || cmd.PortalServiceCode != "product_order" || cmd.SourceWarehouse != "finished_goods" || !cmd.PreserveFulfillmentSnapshot || cmd.BeanListPublicationID != 901 || cmd.CommercialBeanListPublicationID != 901 {
		t.Fatalf("responsible/portal/publication fields lost: %+v", cmd)
	}
	if cmd.ShippingAmount != 12.5 || cmd.DiscountAmount != 3.5 || cmd.ReceiverName != "新收件人" || cmd.ReceiverPhone != "13900000000" || cmd.ReceiverAddress != "杭州市新地址" || cmd.ReceiverCompany != "新公司" || cmd.Notes != "新订单备注" {
		t.Fatalf("explicit editable fields not authoritative: %+v", cmd)
	}
	if len(cmd.Items) != 1 || cmd.Items[0].Note != "保留行备注" || cmd.Items[0].DiscountType != "percent" || cmd.Items[0].DiscountValue != 7.5 {
		t.Fatalf("hidden item fields lost: %+v", cmd.Items)
	}
}

func TestMiniEmployeeOrderUpdateUsesExplicitEditableZeroAndEmptyValues(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", CustomerID: 8}}}
	form := salesapp.OrderFormData{
		Customers:              []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products:               []salesapp.ProductOption{{ID: 551, ParentProductID: 550, ProductKind: "roasted_bean", Visibility: "public", Tiers: []salesapp.ProductTierOption{{PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", UnitPrice: 68, PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`}}}},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true}},
		EditData: &salesapp.OrderEditData{
			ID: 42, CustomerID: 8, ShippingAmount: "99.00", DiscountAmount: "10.00",
			ReceiverName: "旧收件人", ReceiverPhone: "13800000000", ReceiverAddress: "旧地址", ReceiverCompany: "旧公司", Notes: "旧备注",
			Items: []salesapp.OrderEditItem{{ItemID: 51, ProductID: 551, BeanListPublicationID: 901}},
		},
	}
	sales := &miniEmployeeSalesFake{listResult: &result, orderFormResult: &form}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-08-02","customer_id":8,"shipping_amount":0,"discount_amount":0,"receiver_name":"","receiver_phone":"","receiver_address":"","receiver_company":"","notes":"","items":[{"product_id":551,"parent_product_id":550,"qty":1,"unit":"袋","spec_g":227,"sales_unit":"bag","unit_price":68,"bean_list_publication_id":901,"bean_list_version_no":"V9.1"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	cmd := sales.save
	if cmd.ShippingAmount != 0 || cmd.DiscountAmount != 0 || cmd.ReceiverName != "" || cmd.ReceiverPhone != "" || cmd.ReceiverAddress != "" || cmd.ReceiverCompany != "" || cmd.Notes != "" {
		t.Fatalf("explicit zero/empty values were replaced by old values: %+v", cmd)
	}
}

func TestMiniEmployeeOrderUpdatePreservesHiddenLineFieldsByItemIDAfterDeleteAndProductChange(t *testing.T) {
	productID := int64(551)
	requests := []miniEmployeeOrderItemRequest{{ItemID: 52, ProductID: productID}}
	commands := []salesapp.OrderItemCommand{{ProductID: &productID}}
	existing := []salesapp.OrderEditItem{
		{ItemID: 51, ProductID: 550, Note: "已删除行备注", DiscountType: "fixed", DiscountValue: "1"},
		{ItemID: 52, ProductID: 552, Note: "应保留行备注", DiscountType: "percent", DiscountValue: "8"},
	}
	if err := miniEmployeePreserveHiddenOrderItemFields(requests, commands, existing); err != nil {
		t.Fatal(err)
	}
	if commands[0].Note != "应保留行备注" || commands[0].DiscountType != "percent" || commands[0].DiscountValue != 8 {
		t.Fatalf("hidden line fields matched the wrong original item: %+v", commands[0])
	}
}

func TestMiniEmployeeOrderUpdateRejectsCustomerChange(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", CustomerID: 8}}}
	sales := &miniEmployeeSalesFake{listResult: &result}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(`{"order_date":"2026-08-02","customer_id":9,"items":[{"product_id":551,"qty":1}]}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || sales.saveCalls != 0 || !strings.Contains(rec.Body.String(), "不能更换客户") {
		t.Fatalf("status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderUpdateRejectsStalePageRevisionBeforeSaving(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", CustomerID: 8}}}
	form := salesapp.OrderFormData{EditData: &salesapp.OrderEditData{ID: 42, CustomerID: 8, EditRevision: "current-revision"}}
	sales := &miniEmployeeSalesFake{listResult: &result, orderFormResult: &form}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(`{"edit_revision":"stale-revision","order_date":"2026-08-02","customer_id":8,"items":[]}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || sales.saveCalls != 0 || !strings.Contains(rec.Body.String(), "重新打开") {
		t.Fatalf("status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderUpdateRejectsHistoricalPublication(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", CustomerID: 8}}}
	form := salesapp.OrderFormData{
		Customers: []salesapp.CustomerOption{{ID: 8, Name: "客户A"}},
		Products: []salesapp.ProductOption{{
			ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", ProductKind: "roasted_bean", Visibility: "public",
			Tiers: []salesapp.ProductTierOption{
				{ID: 11, PublicationID: 901, PublicationVersionNo: "V9.1", ListType: "commercial", PriceSourceJSON: `{"publication_id":901,"list_type":"commercial"}`},
				{ID: 10, PublicationID: 900, PublicationVersionNo: "V9.0", ListType: "commercial", PriceSourceJSON: `{"publication_id":900,"list_type":"commercial"}`},
			},
		}},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{
			{CustomerID: 8, ListType: "commercial", ID: 901, VersionNo: "V9.1", IsDefault: true},
			{CustomerID: 8, ListType: "commercial", ID: 900, VersionNo: "V9.0", IsDefault: false},
		},
	}
	sales := &miniEmployeeSalesFake{listResult: &result, orderFormResult: &form}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-08-02","customer_id":8,"items":[{"product_id":551,"parent_product_id":550,"name":"乌拉嘎 227g","qty":1,"bean_list_publication_id":900,"bean_list_version_no":"V9.0","price_source_json":"{\"publication_id\":900,\"list_type\":\"commercial\"}"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || sales.saveCalls != 0 || !strings.Contains(rec.Body.String(), "当前价格表") {
		t.Fatalf("status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderUpdateHidesOutOfScopeOrder(t *testing.T) {
	e := echo.New()
	empty := salesapp.OrderListResult{}
	sales := &miniEmployeeSalesFake{listResult: &empty}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(`{"order_date":"2026-08-02","customer_id":8,"items":[]}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || sales.saveCalls != 0 {
		t.Fatalf("status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderUpdateMapsProductionConflictTo409(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", CustomerID: 8}}}
	sales := &miniEmployeeSalesFake{
		listResult: &result,
		saveError:  salesapp.NewOrderEditConflictError("订单已进入生产流程，不能再编辑"),
	}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(`{"order_date":"2026-08-02","customer_id":8,"items":[{"product_id":551,"qty":1,"spec_g":227,"unit_price":68,"price_override":true}]}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || sales.saveCalls != 1 || !strings.Contains(rec.Body.String(), "订单已进入生产流程，不能再编辑") {
		t.Fatalf("status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderUpdateMapsUnknownPersistenceErrorToGeneric500(t *testing.T) {
	e := echo.New()
	result := salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", CustomerID: 8}}}
	sales := &miniEmployeeSalesFake{
		listResult: &result,
		saveError:  errors.New(`ERROR: column oi.private_schema_detail does not exist (SQLSTATE 42703)`),
	}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(`{"order_date":"2026-08-02","customer_id":8,"items":[{"product_id":551,"parent_product_id":550,"qty":1,"spec_g":227,"bean_list_publication_id":901,"bean_list_version_no":"V9.1","price_source_json":"{\"publication_id\":901,\"list_type\":\"commercial\"}"}]}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || sales.saveCalls != 1 {
		t.Fatalf("status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "private_schema_detail") || strings.Contains(rec.Body.String(), "SQLSTATE") {
		t.Fatalf("persistence details leaked: %s", rec.Body.String())
	}
}

func TestMiniEmployeeOrderUpdateRequiresWritePermission(t *testing.T) {
	e := echo.New()
	portal := fakeService{me: customerportalapp.CurrentContext{AccountType: "employee", EmployeeID: 7, Roles: []string{"sales"}, Permissions: []string{"orders.read"}}}
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, portal, sales)
	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || sales.listCalls != 0 || sales.saveCalls != 0 {
		t.Fatalf("status=%d list_calls=%d save_calls=%d body=%s", rec.Code, sales.listCalls, sales.saveCalls, rec.Body.String())
	}
}

func TestMiniEmployeeCustomerMaintenanceContractsAndPrincipal(t *testing.T) {
	e := echo.New()
	customers := &miniEmployeeCustomersFake{
		listResult: customerapp.ListResult{
			Rows:                []customerapp.CustomerRow{{ID: 8, Name: "客户A", Active: true}},
			Sources:             []customerapp.Option{{ID: 1, Name: "小程序"}},
			OrderTypes:          []customerapp.Option{{ID: 2, Name: "零售"}},
			Employees:           []customerapp.Option{{ID: 7, Name: "销售甲"}},
			CustomerTypeOptions: customerapp.DefaultCustomerTypeOptions(),
			Total:               1,
		},
		editorData: &customerapp.EditorData{Customer: customerapp.CustomerEditData{
			ID: 8, Name: "客户A", CustomerType: customerapp.CustomerTypeRetail,
			DefaultSourceID: "1", DefaultOrderTypeID: "2", ResponsibleEmployeeID: "7",
			ResponsibleEmployeeName: "销售甲", Active: true,
		}},
	}
	registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{}, customers)

	for _, request := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{method: http.MethodGet, path: "/api/mini/employee/customers", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/mini/employee/customers/8", status: http.StatusOK},
		{method: http.MethodPut, path: "/api/mini/employee/customers/8", body: `{"name":"客户A","customer_type":"retail","default_source_id":1,"default_order_type_id":2,"responsible_employee_id":999,"active":false,"portal_enabled":true}`, status: http.StatusOK},
	} {
		req := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body))
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != request.status {
			t.Fatalf("%s %s status=%d body=%s", request.method, request.path, rec.Code, rec.Body.String())
		}
		if request.path == "/api/mini/employee/customers" {
			for _, want := range []string{`"rows"`, `"sources":[{"id":1,"name":"小程序"}]`, `"order_types":[{"id":2,"name":"零售"}]`, `"employees":[{"id":7,"name":"销售甲"}]`, `"customer_type_options"`, `"is_admin":false`, `"total":1`, `"has_next":false`} {
				if !strings.Contains(rec.Body.String(), want) {
					t.Fatalf("list body=%s missing %s", rec.Body.String(), want)
				}
			}
		} else if !strings.Contains(rec.Body.String(), `"customer"`) {
			t.Fatalf("detail/upsert body=%s", rec.Body.String())
		}
	}
	if customers.listActor.EmployeeID != 7 || customers.listActor.IsAdmin || customers.editorActor.EmployeeID != 7 || customers.upsertActor.EmployeeID != 7 || customers.upsertID == nil || *customers.upsertID != 8 {
		t.Fatalf("principals list=%+v editor=%+v upsert=%+v id=%v", customers.listActor, customers.editorActor, customers.upsertActor, customers.upsertID)
	}
}

func TestMiniEmployeeCustomerMaintenanceMapsForbiddenAndNotFound(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{err: customerapp.ErrCustomerMaintenanceForbidden, status: http.StatusForbidden},
		{err: customerapp.ErrCustomerNotFound, status: http.StatusNotFound},
	} {
		e := echo.New()
		customers := &miniEmployeeCustomersFake{error: tc.err}
		registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{}, customers)
		req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/customers/88", nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("error=%v status=%d body=%s", tc.err, rec.Code, rec.Body.String())
		}
	}
}

func TestMiniEmployeeCustomerWritesRequireReadAndWritePermissions(t *testing.T) {
	for _, permissions := range [][]string{
		{"customers.read"},
		{"customers.write"},
	} {
		e := echo.New()
		portal := fakeService{me: customerportalapp.CurrentContext{
			AccountType: "employee", EmployeeID: 7, EmployeeName: "销售甲",
			Roles: []string{"sales"}, Permissions: permissions,
		}}
		customers := &miniEmployeeCustomersFake{}
		registerMiniEmployeeAPI(e, portal, &miniEmployeeSalesFake{}, customers)
		req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/customers", strings.NewReader(`{"name":"客户A","customer_type":"retail","default_source_id":1,"default_order_type_id":2}`))
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || customers.upsertActor.EmployeeID != 0 {
			t.Fatalf("permissions=%v status=%d upsert actor=%+v body=%s", permissions, rec.Code, customers.upsertActor, rec.Body.String())
		}
	}
}

func TestMiniEmployeeCustomerWritesExposeOnlySafeValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		status     int
		want       string
		notContain string
	}{
		{name: "validation", err: customerapp.NewMaintenanceValidationError("来源不存在，请重新选择"), status: http.StatusBadRequest, want: "来源不存在，请重新选择"},
		{name: "repository failure", err: errors.New("pq: secret_schema.audit_logs unavailable"), status: http.StatusInternalServerError, notContain: "secret_schema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			customers := &miniEmployeeCustomersFake{error: tc.err}
			registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{}, customers)
			req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/customers", strings.NewReader(`{"name":"客户A","customer_type":"retail","default_source_id":1,"default_order_type_id":2}`))
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status || (tc.want != "" && !strings.Contains(rec.Body.String(), tc.want)) || (tc.notContain != "" && strings.Contains(rec.Body.String(), tc.notContain)) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMiniEmployeeCustomerCreateDoesNotReportFailureAfterCommittedPostReadRace(t *testing.T) {
	e := echo.New()
	customers := &miniEmployeeCustomersFake{afterUpsertError: customerapp.ErrCustomerMaintenanceForbidden}
	registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{}, customers)
	body := `{"name":"客户A","customer_type":"retail","company_name":"客户A公司","contact":"收货人A","phone":"021-12345678-801","address":"上海市测试路1号","default_source_id":1,"default_order_type_id":2,"responsible_employee_id":999,"active":false,"portal_enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/customers", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated || customers.upsertCalls != 1 {
		t.Fatalf("status=%d upsert calls=%d body=%s", rec.Code, customers.upsertCalls, rec.Body.String())
	}
	for _, want := range []string{
		`"id":99`, `"name":"客户A"`, `"responsible_employee_id":7`, `"active":true`, `"portal_enabled":false`,
		`"receiver_name":"收货人A"`, `"receiver_phone":"021-12345678-801"`, `"receiver_address":"上海市测试路1号"`, `"receiver_company":"客户A公司"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body=%s missing %s", rec.Body.String(), want)
		}
	}
}

func TestMiniEmployeeOrderDraftContractsUseTokenEmployeeOnly(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{draft: &salesapp.EmployeeOrderDraft{
		ID: 5, EmployeeID: 7, Payload: json.RawMessage(`{"customer_id":8}`), UpdatedAt: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
	}}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)

	requests := []struct {
		method string
		body   string
		want   string
	}{
		{method: http.MethodGet, want: `"draft":{"id":5`},
		{method: http.MethodPut, body: `{"payload":{"customer_id":9,"items":[]}}`, want: `"draft":{"id":12`},
		{method: http.MethodDelete, want: `"deleted":true`},
	}
	for _, request := range requests {
		req := httptest.NewRequest(request.method, "/api/mini/employee/order-draft", strings.NewReader(request.body))
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), request.want) {
			t.Fatalf("%s status=%d body=%s want=%s", request.method, rec.Code, rec.Body.String(), request.want)
		}
	}
	if sales.draftSave.EmployeeID != 7 || sales.draftDeleteID != 7 || strings.Contains(string(sales.draftSave.Payload), "employee_id") {
		t.Fatalf("draft save=%+v delete employee=%d", sales.draftSave, sales.draftDeleteID)
	}
}

func TestMiniEmployeeOrderDraftSaveExposesOnlySafeValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		status     int
		want       string
		notContain string
	}{
		{name: "validation", err: salesapp.NewEmployeeOrderDraftValidationError("草稿内容不正确"), status: http.StatusBadRequest, want: "草稿内容不正确"},
		{name: "repository failure", err: errors.New("pq: secret_schema.employee_order_drafts unavailable"), status: http.StatusInternalServerError, want: `"error":"internal error"`, notContain: "secret_schema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			sales := &miniEmployeeSalesFake{draftSaveError: tc.err}
			registerMiniEmployeeAPI(e, employeePortalService(), sales)
			req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/order-draft", strings.NewReader(`{"payload":{"customer_id":9,"items":[]}}`))
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.want) || (tc.notContain != "" && strings.Contains(rec.Body.String(), tc.notContain)) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMiniEmployeeAPIRejectsCustomerAccount(t *testing.T) {
	e := echo.New()
	registerMiniEmployeeAPI(e, fakeService{me: customerportalapp.CurrentContext{AccountType: "customer"}}, &miniEmployeeSalesFake{})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
