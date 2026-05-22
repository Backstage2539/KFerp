package sales

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	saveCmd                SaveOrderCommand
	stockPreviewCmd        OrderStockBatchPreviewCommand
	inlineCmd              InlineUpdateCommand
	saveCalls              int
	outsourceSaved         SaveOutsourceTemplateCommand
	trackingCmd            FillTrackingPairsCommand
	shipMethodCmd          SetShipMethodCommand
	shipmentCmd            CreateOrderShipmentCommand
	trackingItems          FillShipmentTrackingCommand
	orderNoTracking        FillShipmentTrackingByOrderNoCommand
	orderTracking          FillOrderTrackingCommand
	settingsCmd            SaveSalesOrderSettingsCommand
	salesOrderNoteCmd      SaveSalesOrderNoteCommand
	generateCmd            GenerateSalesOrderDocumentCommand
	imageCmd               GenerateSalesOrderImageCommand
	previewOrderID         int64
	deliveryFormCmd        SaveDeliveryNoteFormCommand
	generateDeliveryCmd    GenerateDeliveryNoteDocumentCommand
	previewDeliveryOrderID int64
	shareCmd               CreateExternalShareResourceCommand
	invoiceRequestCmd      RequestOrderInvoiceCommand
	invoiceFileCmd         SaveOrderInvoiceFileCommand
	sealAssets             []SalesOrderAsset
	voidManyIDs            []int64
	voidManyReason         string
}

func (r *fakeRepo) SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error) {
	r.saveCmd = cmd
	r.saveCalls++
	return SaveOrderResult{OrderID: 7, OrderNo: "SO-TEST", Edited: cmd.EditID > 0, StockBatchUsed: cmd.StockBatchDecision == "use_batch"}, nil
}

func (r *fakeRepo) PreviewOrderStockBatches(ctx context.Context, cmd OrderStockBatchPreviewCommand) (OrderStockBatchPreview, error) {
	r.stockPreviewCmd = cmd
	return OrderStockBatchPreview{Sufficient: true, HasBatchChoices: true}, nil
}

func (r *fakeRepo) UpdateHeader(ctx context.Context, id int64, cmd UpdateHeaderCommand) error {
	return nil
}

func (r *fakeRepo) InlineUpdate(ctx context.Context, id int64, actor string, cmd InlineUpdateCommand) error {
	r.inlineCmd = cmd
	return nil
}

func (r *fakeRepo) Void(ctx context.Context, id int64, actor, reason string) error {
	return nil
}

func (r *fakeRepo) VoidMany(ctx context.Context, ids []int64, actor, reason string) (int, error) {
	r.voidManyIDs = append([]int64(nil), ids...)
	r.voidManyReason = reason
	return len(ids), nil
}

func (r *fakeRepo) ListOrders(ctx context.Context, query OrderListQuery) (OrderListResult, error) {
	return OrderListResult{}, nil
}

func (r *fakeRepo) ListOrderAuditLogs(ctx context.Context, orderID int64, limit int) ([]AuditRow, error) {
	return nil, nil
}

func (r *fakeRepo) ListOutsourceTemplates(ctx context.Context) ([]OutsourceTemplate, error) {
	return []OutsourceTemplate{{ID: 1, Name: "默认", IsDefault: true, RoastUnitPrice: 2.5}}, nil
}

func (r *fakeRepo) SaveOutsourceTemplate(ctx context.Context, cmd SaveOutsourceTemplateCommand) error {
	r.outsourceSaved = cmd
	return nil
}

func (r *fakeRepo) FillTrackingPairs(ctx context.Context, cmd FillTrackingPairsCommand) (FillTrackingResult, error) {
	r.trackingCmd = cmd
	return FillTrackingResult{Updated: len(cmd.Pairs), Total: len(cmd.Pairs)}, nil
}

func (r *fakeRepo) SetShipMethod(ctx context.Context, cmd SetShipMethodCommand) error {
	r.shipMethodCmd = cmd
	return nil
}

func (r *fakeRepo) OrderForm(ctx context.Context, editID int64) (OrderFormData, error) {
	return OrderFormData{}, nil
}

func (r *fakeRepo) LoadSenderProfile(ctx context.Context) (SenderProfile, error) {
	return SenderProfile{Goods: "咖啡"}, nil
}

func (r *fakeRepo) LoadSenderProfileByID(ctx context.Context, id int64) (SenderProfile, error) {
	return SenderProfile{ID: id, Goods: "茶叶"}, nil
}

func (r *fakeRepo) ListSenderProfiles(ctx context.Context) ([]SenderProfile, error) {
	return []SenderProfile{{ID: 1, Label: "默认寄件人", Goods: "茶叶", IsDefault: true, Active: true}}, nil
}

func (r *fakeRepo) SaveSenderProfile(ctx context.Context, profile SenderProfile) error {
	return nil
}

func (r *fakeRepo) ListSFSmallShippingRows(ctx context.Context, query ShippingExportQuery) ([]ShippingExportRow, error) {
	return nil, nil
}

func (r *fakeRepo) LoadOrderShippingExportData(ctx context.Context, orderID int64) (OrderShippingExportData, error) {
	return OrderShippingExportData{OrderID: orderID}, nil
}

func (r *fakeRepo) CreateOrderShipment(ctx context.Context, cmd CreateOrderShipmentCommand) (OrderShipmentResult, error) {
	r.shipmentCmd = cmd
	return OrderShipmentResult{ShipmentID: 12, ShipmentNo: "SHIP-20260428-0001"}, nil
}

func (r *fakeRepo) FillShipmentTracking(ctx context.Context, cmd FillShipmentTrackingCommand) (FillShipmentTrackingResult, error) {
	r.trackingItems = cmd
	return FillShipmentTrackingResult{Updated: len(cmd.Items)}, nil
}

func (r *fakeRepo) FillShipmentTrackingByOrderNo(ctx context.Context, cmd FillShipmentTrackingByOrderNoCommand) (FillShipmentTrackingResult, error) {
	r.orderNoTracking = cmd
	return FillShipmentTrackingResult{Updated: len(cmd.Items), Total: len(cmd.Items)}, nil
}

func (r *fakeRepo) FillOrderTracking(ctx context.Context, cmd FillOrderTrackingCommand) (FillShipmentTrackingResult, error) {
	r.orderTracking = cmd
	return FillShipmentTrackingResult{Updated: 1, Total: 1}, nil
}

func (r *fakeRepo) LoadSalesOrderSettings(ctx context.Context) (SalesOrderSettings, error) {
	return SalesOrderSettings{CompanyName: "浅焙作坊咖啡", PaymentText: "微信或对公转账"}, nil
}

func (r *fakeRepo) SaveSalesOrderSettings(ctx context.Context, cmd SaveSalesOrderSettingsCommand) error {
	r.settingsCmd = cmd
	return nil
}

func (r *fakeRepo) SaveSalesOrderAsset(ctx context.Context, cmd SaveSalesOrderAssetCommand) (SalesOrderAsset, error) {
	return SalesOrderAsset{ID: 3, Kind: cmd.Kind, Filename: cmd.Filename, ContentType: cmd.ContentType, ObjectKey: cmd.ObjectKey}, nil
}

func (r *fakeRepo) DeleteSalesOrderAsset(ctx context.Context, id int64, actor string) error {
	return nil
}

func (r *fakeRepo) SaveSalesOrderPaymentCode(ctx context.Context, cmd SaveSalesOrderPaymentCodeCommand) (SalesOrderPaymentCode, error) {
	return SalesOrderPaymentCode{ID: 4, Label: cmd.Label, Description: cmd.Description, AssetID: cmd.AssetID, Sort: cmd.Sort, Active: cmd.Active}, nil
}

func (r *fakeRepo) DeleteSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	return nil
}

func (r *fakeRepo) SetSalesOrderSealAsset(ctx context.Context, assetID int64, actor string) error {
	return nil
}

func (r *fakeRepo) ListSalesOrderSealAssets(ctx context.Context) ([]SalesOrderAsset, error) {
	return append([]SalesOrderAsset(nil), r.sealAssets...), nil
}

func (r *fakeRepo) ListSalesOrderDocuments(ctx context.Context, orderID int64) ([]SalesOrderDocument, error) {
	return []SalesOrderDocument{{ID: 9, OrderID: orderID, OrderNo: "SO-TEST", VersionNo: 2, IsLatest: true}}, nil
}

func (r *fakeRepo) ListSalesOrderImageDocuments(ctx context.Context, orderID int64) ([]SalesOrderImageDocument, error) {
	return []SalesOrderImageDocument{{ID: 19, OrderID: orderID, OrderNo: "SO-TEST", VersionNo: 2, IsLatest: true}}, nil
}

func (r *fakeRepo) LoadSalesOrderContext(ctx context.Context, orderID int64) (SalesOrderContext, error) {
	return SalesOrderContext{OrderID: orderID, OrderNo: "SO-TEST", Customer: SalesOrderCustomerInfo{ID: 3, Name: "测试客户"}}, nil
}

func (r *fakeRepo) SaveSalesOrderNote(ctx context.Context, cmd SaveSalesOrderNoteCommand) error {
	r.salesOrderNoteCmd = cmd
	return nil
}

func (r *fakeRepo) GenerateSalesOrderDocument(ctx context.Context, cmd GenerateSalesOrderDocumentCommand) (GenerateSalesOrderDocumentResult, error) {
	r.generateCmd = cmd
	return GenerateSalesOrderDocumentResult{Document: SalesOrderDocument{ID: 10, OrderID: cmd.OrderID, OrderNo: "SO-TEST", VersionNo: 1, IsLatest: true}}, nil
}

func (r *fakeRepo) GenerateSalesOrderImage(ctx context.Context, cmd GenerateSalesOrderImageCommand) (GenerateSalesOrderImageResult, error) {
	r.imageCmd = cmd
	return GenerateSalesOrderImageResult{Document: SalesOrderImageDocument{ID: 20, OrderID: cmd.OrderID, OrderNo: "SO-TEST", VersionNo: 1, IsLatest: true}}, nil
}

func (r *fakeRepo) PreviewSalesOrderDocument(ctx context.Context, orderID int64) (SalesOrderPreview, error) {
	r.previewOrderID = orderID
	return SalesOrderPreview{OrderID: orderID, OrderNo: "SO-TEST", NextVersionNo: 3}, nil
}

func (r *fakeRepo) PreviewSalesOrderPDF(ctx context.Context, orderID int64) (SalesOrderPreviewPDF, error) {
	r.previewOrderID = orderID
	return SalesOrderPreviewPDF{
		Preview:  SalesOrderPreview{OrderID: orderID, OrderNo: "SO-TEST", NextVersionNo: 3},
		Data:     []byte("%PDF-preview"),
		Filename: "SO-TEST-preview.pdf",
	}, nil
}

func (r *fakeRepo) LoadSalesOrderDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (SalesOrderDocumentFile, error) {
	return SalesOrderDocumentFile{Document: SalesOrderDocument{ID: documentID, OrderID: orderID, OrderNo: "SO-TEST", VersionNo: 1}, Path: "/tmp/test.pdf", Filename: "SO-TEST-V1.pdf"}, nil
}

func (r *fakeRepo) LoadSalesOrderImageFile(ctx context.Context, orderID, imageID int64, latest bool) (SalesOrderImageFile, error) {
	return SalesOrderImageFile{Document: SalesOrderImageDocument{ID: imageID, OrderID: orderID, OrderNo: "SO-TEST", VersionNo: 1}, Path: "/tmp/test.png", Filename: "SO-TEST-V1.png"}, nil
}

func (r *fakeRepo) LoadDeliveryNoteContext(ctx context.Context, orderID int64) (DeliveryNoteContext, error) {
	return DeliveryNoteContext{OrderID: orderID, OrderNo: "SO-TEST", ShipStatus: "已发货", Customer: SalesOrderCustomerInfo{ID: 3, Name: "测试客户"}}, nil
}

func (r *fakeRepo) LoadDeliveryNoteForm(ctx context.Context, orderID int64) (DeliveryNoteForm, error) {
	return DeliveryNoteForm{OrderID: orderID, OrderNo: "SO-TEST", SourceWarehouse: "finished_goods"}, nil
}

func (r *fakeRepo) SaveDeliveryNoteForm(ctx context.Context, cmd SaveDeliveryNoteFormCommand) (DeliveryNoteForm, error) {
	r.deliveryFormCmd = cmd
	return DeliveryNoteForm{OrderID: cmd.OrderID, OrderNo: "SO-TEST", PostingDate: cmd.PostingDate, SourceWarehouse: cmd.SourceWarehouse, DeliveryMethod: cmd.DeliveryMethod, TrackingNo: cmd.TrackingNo, Note: cmd.Note}, nil
}

func (r *fakeRepo) ListDeliveryNoteDocuments(ctx context.Context, orderID int64) ([]DeliveryNoteDocument, error) {
	return []DeliveryNoteDocument{{ID: 9, OrderID: orderID, OrderNo: "SO-TEST", VersionNo: 1, IsLatest: true}}, nil
}

func (r *fakeRepo) PreviewDeliveryNoteDocument(ctx context.Context, orderID int64) (DeliveryNotePreview, error) {
	r.previewDeliveryOrderID = orderID
	return DeliveryNotePreview{OrderID: orderID, OrderNo: "SO-TEST", NextVersionNo: 2}, nil
}

func (r *fakeRepo) PreviewDeliveryNotePDF(ctx context.Context, orderID int64) (DeliveryNotePreviewPDF, error) {
	r.previewDeliveryOrderID = orderID
	return DeliveryNotePreviewPDF{
		Preview:  DeliveryNotePreview{OrderID: orderID, OrderNo: "SO-TEST", NextVersionNo: 2},
		Data:     []byte("%PDF-delivery-preview"),
		Filename: "SO-TEST-delivery-note-preview.pdf",
	}, nil
}

func (r *fakeRepo) GenerateDeliveryNoteDocument(ctx context.Context, cmd GenerateDeliveryNoteDocumentCommand) (GenerateDeliveryNoteDocumentResult, error) {
	r.generateDeliveryCmd = cmd
	return GenerateDeliveryNoteDocumentResult{Document: DeliveryNoteDocument{ID: 10, OrderID: cmd.OrderID, OrderNo: "SO-TEST", VersionNo: 1, IsLatest: true}}, nil
}

func (r *fakeRepo) LoadDeliveryNoteDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (DeliveryNoteDocumentFile, error) {
	return DeliveryNoteDocumentFile{Document: DeliveryNoteDocument{ID: documentID, OrderID: orderID, OrderNo: "SO-TEST", VersionNo: 1}, Path: "/tmp/test.pdf", Filename: "SO-TEST-DN-V1.pdf"}, nil
}

func (r *fakeRepo) CreateExternalShareResource(ctx context.Context, cmd CreateExternalShareResourceCommand) (ExternalShareResource, error) {
	r.shareCmd = cmd
	return ExternalShareResource{Token: "token", ResourceType: cmd.ResourceType, OrderID: cmd.OrderID, ResourceID: cmd.DocumentID, ShareURL: "/share/token", FileURL: "/share/token/file"}, nil
}

func (r *fakeRepo) LoadExternalShareResourceFile(ctx context.Context, token string) (ExternalShareResourceFile, error) {
	return ExternalShareResourceFile{Resource: ExternalShareResource{Token: token, ContentType: "application/pdf", Filename: "SO-TEST-V1.pdf"}, Path: "/tmp/test.pdf"}, nil
}

func (r *fakeRepo) LoadOrderInvoice(ctx context.Context, orderID int64) (OrderInvoice, error) {
	return OrderInvoice{OrderID: orderID, OrderNo: "SO-TEST", Status: "requested"}, nil
}

func (r *fakeRepo) RequestOrderInvoice(ctx context.Context, cmd RequestOrderInvoiceCommand) (OrderInvoice, error) {
	r.invoiceRequestCmd = cmd
	return OrderInvoice{OrderID: cmd.OrderID, OrderNo: "SO-TEST", Status: "requested", RequestedBy: cmd.Actor}, nil
}

func (r *fakeRepo) SaveOrderInvoiceFile(ctx context.Context, cmd SaveOrderInvoiceFileCommand) (OrderInvoice, error) {
	r.invoiceFileCmd = cmd
	return OrderInvoice{
		OrderID: cmd.OrderID,
		OrderNo: "SO-TEST",
		Status:  "uploaded",
		Asset: &SalesOrderAsset{
			ID:          33,
			Kind:        "order_invoice",
			Filename:    cmd.Filename,
			ContentType: cmd.ContentType,
			Bytes:       cmd.Bytes,
			SHA256:      cmd.SHA256,
			ObjectKey:   cmd.ObjectKey,
			URL:         "/assets/" + cmd.ObjectKey,
		},
	}, nil
}

func TestServiceDelegatesSaveOrder(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	res, err := svc.SaveOrder(context.Background(), SaveOrderCommand{
		EditID:     10,
		OrderDate:  time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
		CustomerID: 3,
		Items: []OrderItemCommand{{
			ProductID: int64Ptr(11),
			Name:      "橘皮乌龙",
			Units:     2,
			SpecG:     227,
		}},
	})
	if err != nil {
		t.Fatalf("SaveOrder() error = %v", err)
	}
	if !res.Edited || res.OrderID != 7 || res.OrderNo != "SO-TEST" {
		t.Fatalf("SaveOrder() result = %+v", res)
	}
	if repo.saveCmd.CustomerID != 3 {
		t.Fatalf("repo command = %+v", repo.saveCmd)
	}
	if len(repo.saveCmd.Items) != 1 || repo.saveCmd.Items[0].SpecG != 227 {
		t.Fatalf("repo items = %+v", repo.saveCmd.Items)
	}
}

func TestServiceVoidManyDeduplicatesAndRejectsInvalidIDs(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	count, err := svc.VoidMany(context.Background(), []int64{9, 9, 10}, "tester", "批量失效")
	if err != nil {
		t.Fatalf("VoidMany() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("VoidMany() count = %d, want 2", count)
	}
	if len(repo.voidManyIDs) != 2 || repo.voidManyIDs[0] != 9 || repo.voidManyIDs[1] != 10 {
		t.Fatalf("VoidMany() ids = %v, want [9 10]", repo.voidManyIDs)
	}
	if repo.voidManyReason != "批量失效" {
		t.Fatalf("VoidMany() reason = %q", repo.voidManyReason)
	}
	if _, err := svc.VoidMany(context.Background(), []int64{9, 0}, "tester", "bad"); err == nil {
		t.Fatal("VoidMany() invalid id error = nil")
	}
}

func TestServiceValidatesAndDelegatesStockBatchPreview(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	productID := int64(7)

	preview, err := svc.PreviewOrderStockBatches(context.Background(), OrderStockBatchPreviewCommand{
		EditID: 9,
		Items:  []OrderItemCommand{{ProductID: &productID, Name: "橘皮乌龙", Units: 2, Unit: "件", SpecG: 454}},
	})
	if err != nil {
		t.Fatalf("PreviewOrderStockBatches() error = %v", err)
	}
	if !preview.Sufficient || len(repo.stockPreviewCmd.Items) != 1 {
		t.Fatalf("preview=%+v cmd=%+v, want delegated sufficient preview", preview, repo.stockPreviewCmd)
	}
	if repo.stockPreviewCmd.EditID != 9 {
		t.Fatalf("preview edit id = %d, want 9", repo.stockPreviewCmd.EditID)
	}

	_, err = svc.PreviewOrderStockBatches(context.Background(), OrderStockBatchPreviewCommand{
		Items: []OrderItemCommand{{ProductID: &productID, Units: 0, SpecG: 454}},
	})
	if err == nil {
		t.Fatal("PreviewOrderStockBatches invalid quantity error = nil")
	}
}

func TestServiceOwnsOrderInvoiceUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	requested, err := svc.RequestOrderInvoice(context.Background(), RequestOrderInvoiceCommand{Actor: "财务", OrderID: 9})
	if err != nil {
		t.Fatalf("RequestOrderInvoice: %v", err)
	}
	if requested.Status != "requested" || repo.invoiceRequestCmd.Actor != "财务" || repo.invoiceRequestCmd.OrderID != 9 {
		t.Fatalf("requested invoice=%+v cmd=%+v", requested, repo.invoiceRequestCmd)
	}

	uploaded, err := svc.SaveOrderInvoiceFile(context.Background(), SaveOrderInvoiceFileCommand{
		Actor:       "财务",
		OrderID:     9,
		Filename:    "invoice.pdf",
		ContentType: "application/pdf",
		Bytes:       128,
		SHA256:      "abc123",
		ObjectKey:   "sales_order_assets/order_invoices/SO-TEST/invoice.pdf",
	})
	if err != nil {
		t.Fatalf("SaveOrderInvoiceFile: %v", err)
	}
	if uploaded.Status != "uploaded" || uploaded.Asset == nil || uploaded.Asset.Filename != "invoice.pdf" {
		t.Fatalf("uploaded invoice=%+v", uploaded)
	}

	if _, err := svc.RequestOrderInvoice(context.Background(), RequestOrderInvoiceCommand{OrderID: 0}); err == nil {
		t.Fatal("RequestOrderInvoice invalid order id error = nil")
	}
	if _, err := svc.SaveOrderInvoiceFile(context.Background(), SaveOrderInvoiceFileCommand{OrderID: 9, Filename: "invoice.txt", ContentType: "text/plain", Bytes: 1, SHA256: "x", ObjectKey: "x"}); err == nil {
		t.Fatal("SaveOrderInvoiceFile unsupported content type error = nil")
	}
}

func TestServiceOwnsExternalShareResourceUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	share, err := svc.CreateExternalShareResource(context.Background(), CreateExternalShareResourceCommand{
		Actor:        " 销售 ",
		ResourceType: ExternalShareSalesOrderPDF,
		OrderID:      18,
		Latest:       true,
	})
	if err != nil {
		t.Fatalf("CreateExternalShareResource: %v", err)
	}
	if share.ShareURL != "/share/token" || repo.shareCmd.Actor != "销售" || repo.shareCmd.ResourceType != ExternalShareSalesOrderPDF || repo.shareCmd.OrderID != 18 || !repo.shareCmd.Latest {
		t.Fatalf("share=%+v cmd=%+v", share, repo.shareCmd)
	}

	if _, err := svc.CreateExternalShareResource(context.Background(), CreateExternalShareResourceCommand{ResourceType: "unknown", OrderID: 18, Latest: true}); err == nil {
		t.Fatal("invalid resource type error = nil")
	}
	if _, err := svc.CreateExternalShareResource(context.Background(), CreateExternalShareResourceCommand{ResourceType: ExternalShareDeliveryNotePDF, OrderID: 0, Latest: true}); err == nil {
		t.Fatal("invalid order id error = nil")
	}
	if _, err := svc.CreateExternalShareResource(context.Background(), CreateExternalShareResourceCommand{ResourceType: ExternalShareSalesOrderImage, OrderID: 18}); err == nil {
		t.Fatal("missing explicit document id for non-latest share error = nil")
	}

	file, err := svc.LoadExternalShareResourceFile(context.Background(), " token ")
	if err != nil {
		t.Fatalf("LoadExternalShareResourceFile: %v", err)
	}
	if file.Resource.Token != "token" {
		t.Fatalf("share file=%+v", file)
	}
	if _, err := svc.LoadExternalShareResourceFile(context.Background(), ""); err == nil {
		t.Fatal("empty token error = nil")
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestSaveOrderCommandUsesTypedFields(t *testing.T) {
	cmd := SaveOrderCommand{
		ShippingAmount:        9.5,
		DiscountAmount:        1.25,
		RoundToInt:            true,
		OutsourceMaterialFee:  1,
		OutsourceRoastFee:     2,
		OutsourcePackagingFee: 3,
		OutsourceManualFee:    4,
		OutsourceTaxFee:       5,
		OutsourceOtherFee:     6,
	}
	if cmd.ShippingAmount != 9.5 || cmd.DiscountAmount != 1.25 || !cmd.RoundToInt {
		t.Fatalf("unexpected amount fields: %+v", cmd)
	}
	if cmd.OutsourceMaterialFee+cmd.OutsourceRoastFee+cmd.OutsourcePackagingFee+cmd.OutsourceManualFee+cmd.OutsourceTaxFee+cmd.OutsourceOtherFee != 21 {
		t.Fatalf("unexpected outsource fields: %+v", cmd)
	}
}

func TestServiceValidatesSaveOrderBeforeRepository(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.SaveOrder(context.Background(), SaveOrderCommand{
		OrderDate:  time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
		CustomerID: 3,
		Items: []OrderItemCommand{{
			Name:  "missing spec",
			Units: 2,
		}},
	})
	if err == nil {
		t.Fatal("SaveOrder() error = nil, want validation error")
	}
	if repo.saveCalls != 0 {
		t.Fatalf("repository was called %d times for invalid command", repo.saveCalls)
	}
}

func TestServiceOwnsOutsourceTemplateUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	rows, err := svc.ListOutsourceTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "默认" {
		t.Fatalf("ListOutsourceTemplates() = %+v", rows)
	}

	err = svc.SaveOutsourceTemplate(context.Background(), SaveOutsourceTemplateCommand{
		Name:              " 默认外包 ",
		IsDefault:         true,
		RoastUnitPrice:    1.5,
		BeanPackUnitPrice: 2.5,
		DripPackUnitPrice: 3.5,
		SCUnitPrice:       4.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.outsourceSaved.Name != "默认外包" || !repo.outsourceSaved.IsDefault {
		t.Fatalf("SaveOutsourceTemplate normalized command = %+v", repo.outsourceSaved)
	}

	if err := svc.SaveOutsourceTemplate(context.Background(), SaveOutsourceTemplateCommand{Name: ""}); err == nil {
		t.Fatal("SaveOutsourceTemplate empty name error = nil")
	}
	if err := svc.SaveOutsourceTemplate(context.Background(), SaveOutsourceTemplateCommand{Name: "坏价格", RoastUnitPrice: -1}); err == nil {
		t.Fatal("SaveOutsourceTemplate negative price error = nil")
	}
}

func TestServiceOwnsShippingWriteUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	res, err := svc.FillTrackingPairs(context.Background(), FillTrackingPairsCommand{
		Actor: " tester ",
		Pairs: []TrackingPair{
			{Phone: " 138 0000 0000 ", Tracking: " SF123 "},
			{Phone: "", Tracking: "ignored"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || repo.trackingCmd.Actor != "tester" || repo.trackingCmd.Pairs[0].Phone != "13800000000" || repo.trackingCmd.Pairs[0].Tracking != "SF123" {
		t.Fatalf("tracking result=%+v command=%+v", res, repo.trackingCmd)
	}

	if err := svc.SetShipMethod(context.Background(), SetShipMethodCommand{
		Actor:    "tester",
		OrderIDs: []int64{7, 7, 8, 0},
		Method:   " sf_large ",
	}); err != nil {
		t.Fatal(err)
	}
	if len(repo.shipMethodCmd.OrderIDs) != 2 || repo.shipMethodCmd.OrderIDs[0] != 7 || repo.shipMethodCmd.OrderIDs[1] != 8 || repo.shipMethodCmd.Method != "sf_large" {
		t.Fatalf("ship method command = %+v", repo.shipMethodCmd)
	}
}

func TestServiceOwnsShipmentClosureUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	shipment, err := svc.CreateOrderShipment(context.Background(), CreateOrderShipmentCommand{
		Actor:    " tester ",
		SenderID: 3,
		FileURL:  " /ship/order_exports/a.xlsx ",
		Orders: []OrderShipmentOrderCommand{
			{OrderID: 20, SenderID: 3},
			{OrderID: 20, SenderID: 3},
			{OrderID: 21, SenderID: 4},
			{OrderID: 0, SenderID: 4},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if shipment.ShipmentID != 12 || shipment.ShipmentNo == "" {
		t.Fatalf("shipment result = %+v", shipment)
	}
	if repo.shipmentCmd.Actor != "tester" || repo.shipmentCmd.FileURL != "/ship/order_exports/a.xlsx" {
		t.Fatalf("shipment command = %+v", repo.shipmentCmd)
	}
	if len(repo.shipmentCmd.Orders) != 2 || repo.shipmentCmd.Orders[0].OrderID != 20 || repo.shipmentCmd.Orders[1].SenderID != 4 {
		t.Fatalf("shipment orders = %+v", repo.shipmentCmd.Orders)
	}

	updated, err := svc.FillShipmentTracking(context.Background(), FillShipmentTrackingCommand{
		Actor:      "",
		ShipmentID: 12,
		Items: []ShipmentTrackingItemCommand{
			{OrderID: 20, TrackingNo: " SF123 "},
			{OrderID: 20, TrackingNo: "SF124"},
			{OrderID: 20, TrackingNo: "SF124"},
			{OrderID: 21, TrackingNo: ""},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Updated != 1 {
		t.Fatalf("tracking result = %+v", updated)
	}
	if repo.trackingItems.Actor != "shipping" || repo.trackingItems.ShipmentID != 12 || len(repo.trackingItems.Items) != 1 || repo.trackingItems.Items[0].TrackingNo != "SF123\nSF124" {
		t.Fatalf("tracking command = %+v", repo.trackingItems)
	}
}

func TestNormalizeTrackingNumbersKeepsMultipleUniqueNumbers(t *testing.T) {
	got := NormalizeTrackingNumbers(" SF123, SF124\nSF123；SF125 ")
	want := []string{"SF123", "SF124", "SF125"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeTrackingNumbers len=%d want=%d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeTrackingNumbers[%d]=%q want %q: %#v", i, got[i], want[i], got)
		}
	}
	if summary := TrackingNumbersSummary(got); summary != "SF123\nSF124\nSF125" {
		t.Fatalf("TrackingNumbersSummary=%q", summary)
	}
}

func TestServiceNormalizesShipmentTrackingByOrderNo(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	updated, err := svc.FillShipmentTrackingByOrderNo(context.Background(), FillShipmentTrackingByOrderNoCommand{
		Items: []ShipmentTrackingByOrderNoItemCommand{
			{OrderNo: " SO-20260428-0001 ", TrackingNo: " SF5199040648127 "},
			{OrderNo: "SO-20260428-0001", TrackingNo: "SF-DUP"},
			{OrderNo: "SO-20260428-0002", TrackingNo: ""},
			{OrderNo: "", TrackingNo: "SF-MISSING"},
			{OrderNo: " SO-20260428-0003 ", TrackingNo: " SF0222363353152 "},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Updated != 2 || updated.Total != 2 {
		t.Fatalf("tracking result = %+v", updated)
	}
	if repo.orderNoTracking.Actor != "shipping" || len(repo.orderNoTracking.Items) != 2 {
		t.Fatalf("order-no tracking command = %+v", repo.orderNoTracking)
	}
	first := repo.orderNoTracking.Items[0]
	if first.OrderNo != "SO-20260428-0001" || first.TrackingNo != "SF5199040648127\nSF-DUP" {
		t.Fatalf("first normalized item = %+v", first)
	}
}

func TestServiceNormalizesSingleOrderTracking(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	updated, err := svc.FillOrderTracking(context.Background(), FillOrderTrackingCommand{
		Actor:      "",
		OrderID:    33,
		TrackingNo: " SF-DRAWER-001，SF-DRAWER-002 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Updated != 1 || updated.Total != 1 {
		t.Fatalf("tracking result = %+v", updated)
	}
	if repo.orderTracking.Actor != "shipping" || repo.orderTracking.OrderID != 33 || repo.orderTracking.TrackingNo != "SF-DRAWER-001\nSF-DRAWER-002" {
		t.Fatalf("single order tracking command = %+v", repo.orderTracking)
	}
}

func TestServiceOwnsSalesOrderSettingsUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if err := svc.SaveSalesOrderSettings(context.Background(), SaveSalesOrderSettingsCommand{
		Actor:       " tester ",
		CompanyName: " 浅焙作坊咖啡 ",
		Note:        " 请密封保存 ",
		PaymentText: " 微信或对公转账 ",
		SealXMM:     32.4,
		SealYMM:     22.5,
		SealWidthMM: 40,
	}); err != nil {
		t.Fatal(err)
	}
	if repo.settingsCmd.Actor != "tester" || repo.settingsCmd.CompanyName != "浅焙作坊咖啡" || repo.settingsCmd.Note != "请密封保存" || repo.settingsCmd.PaymentText != "微信或对公转账" || repo.settingsCmd.SealXMM != 32.4 || repo.settingsCmd.SealYMM != 22.5 || repo.settingsCmd.SealWidthMM != 40 {
		t.Fatalf("settings command = %+v", repo.settingsCmd)
	}
	if err := svc.SaveSalesOrderSettings(context.Background(), SaveSalesOrderSettingsCommand{CompanyName: ""}); err != nil {
		t.Fatalf("SaveSalesOrderSettings empty company error = %v", err)
	}
	if repo.settingsCmd.Actor != "sales" || repo.settingsCmd.CompanyName != "" {
		t.Fatalf("empty settings command = %+v", repo.settingsCmd)
	}

	got, err := svc.LoadSalesOrderSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.CompanyName != "浅焙作坊咖啡" {
		t.Fatalf("LoadSalesOrderSettings() = %+v", got)
	}
}

func TestServiceOwnsSalesOrderDocumentUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if err := svc.SaveSalesOrderNote(context.Background(), SaveSalesOrderNoteCommand{
		Actor:   " sales ",
		OrderID: 18,
		Note:    "  末行备注：随货附赠杯测样  ",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.salesOrderNoteCmd.Actor != "sales" || repo.salesOrderNoteCmd.OrderID != 18 || repo.salesOrderNoteCmd.Note != "末行备注：随货附赠杯测样" {
		t.Fatalf("sales order note command = %+v", repo.salesOrderNoteCmd)
	}
	if err := svc.SaveSalesOrderNote(context.Background(), SaveSalesOrderNoteCommand{OrderID: 0, Note: "bad"}); err == nil {
		t.Fatal("SaveSalesOrderNote invalid order error = nil")
	}

	doc, err := svc.GenerateSalesOrderDocument(context.Background(), GenerateSalesOrderDocumentCommand{Actor: " sales ", OrderID: 18})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Document.VersionNo != 1 || repo.generateCmd.Actor != "sales" || repo.generateCmd.OrderID != 18 {
		t.Fatalf("document=%+v command=%+v", doc, repo.generateCmd)
	}
	if _, err := svc.GenerateSalesOrderDocument(context.Background(), GenerateSalesOrderDocumentCommand{OrderID: 0}); err == nil {
		t.Fatal("GenerateSalesOrderDocument invalid order error = nil")
	}
	preview, err := svc.PreviewSalesOrderDocument(context.Background(), 18)
	if err != nil {
		t.Fatal(err)
	}
	if preview.NextVersionNo != 3 || repo.previewOrderID != 18 {
		t.Fatalf("preview=%+v orderID=%d", preview, repo.previewOrderID)
	}
	if _, err := svc.PreviewSalesOrderDocument(context.Background(), 0); err == nil {
		t.Fatal("PreviewSalesOrderDocument invalid order error = nil")
	}

	docs, err := svc.ListSalesOrderDocuments(context.Background(), 18)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].OrderID != 18 {
		t.Fatalf("ListSalesOrderDocuments() = %+v", docs)
	}
}

func TestServiceOwnsSalesOrderImageUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	generated, err := svc.GenerateSalesOrderImage(context.Background(), GenerateSalesOrderImageCommand{OrderID: 18})
	if err != nil {
		t.Fatal(err)
	}
	if generated.Document.VersionNo != 1 || repo.imageCmd.Actor != "sales" || repo.imageCmd.OrderID != 18 {
		t.Fatalf("image=%+v command=%+v", generated, repo.imageCmd)
	}
	if _, err := svc.GenerateSalesOrderImage(context.Background(), GenerateSalesOrderImageCommand{OrderID: 0}); err == nil {
		t.Fatal("GenerateSalesOrderImage invalid order error = nil")
	}

	images, err := svc.ListSalesOrderImageDocuments(context.Background(), 18)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || images[0].OrderID != 18 {
		t.Fatalf("ListSalesOrderImageDocuments() = %+v", images)
	}
	if _, err := svc.ListSalesOrderImageDocuments(context.Background(), 0); err == nil {
		t.Fatal("ListSalesOrderImageDocuments invalid order error = nil")
	}
	if _, err := svc.LoadSalesOrderImageFile(context.Background(), 18, 0, false); err == nil {
		t.Fatal("LoadSalesOrderImageFile should require image id unless latest")
	}
	if _, err := svc.LoadSalesOrderImageFile(context.Background(), 18, 0, true); err != nil {
		t.Fatalf("LoadSalesOrderImageFile latest error = %v", err)
	}
}

func TestServiceListsSalesOrderSealAssets(t *testing.T) {
	repo := &fakeRepo{sealAssets: []SalesOrderAsset{
		{ID: 3, Kind: "seal", Filename: "公章A.png", ObjectKey: "sales_order_assets/seal/a.png"},
		{ID: 4, Kind: "seal", Filename: "公章B.png", ObjectKey: "sales_order_assets/seal/b.png"},
	}}
	svc := NewService(repo)
	got, err := svc.ListSalesOrderSealAssets(context.Background())
	if err != nil {
		t.Fatalf("ListSalesOrderSealAssets: %v", err)
	}
	if len(got) != 2 || got[0].URL == "" || got[1].Filename != "公章B.png" {
		t.Fatalf("seal assets = %+v", got)
	}
}

func TestServiceOwnsDeliveryNoteUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if err := svc.SaveDeliveryNoteForm(context.Background(), SaveDeliveryNoteFormCommand{
		Actor:           " warehouse ",
		OrderID:         18,
		PostingDate:     " 2026-05-02 ",
		SourceWarehouse: " finished_goods ",
		DeliveryMethod:  " 顺丰 ",
		TrackingNo:      " SF123 ",
		Note:            " 随货附单 ",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.deliveryFormCmd.Actor != "warehouse" || repo.deliveryFormCmd.OrderID != 18 || repo.deliveryFormCmd.SourceWarehouse != "finished_goods" || repo.deliveryFormCmd.TrackingNo != "SF123" {
		t.Fatalf("delivery form command = %+v", repo.deliveryFormCmd)
	}

	doc, err := svc.GenerateDeliveryNoteDocument(context.Background(), GenerateDeliveryNoteDocumentCommand{Actor: " stock ", OrderID: 18})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Document.VersionNo != 1 || repo.generateDeliveryCmd.Actor != "stock" || repo.generateDeliveryCmd.OrderID != 18 {
		t.Fatalf("document=%+v command=%+v", doc, repo.generateDeliveryCmd)
	}
	if _, err := svc.GenerateDeliveryNoteDocument(context.Background(), GenerateDeliveryNoteDocumentCommand{OrderID: 0}); err == nil {
		t.Fatal("GenerateDeliveryNoteDocument invalid order error = nil")
	}
	preview, err := svc.PreviewDeliveryNoteDocument(context.Background(), 18)
	if err != nil {
		t.Fatal(err)
	}
	if preview.NextVersionNo != 2 || repo.previewDeliveryOrderID != 18 {
		t.Fatalf("preview=%+v orderID=%d", preview, repo.previewDeliveryOrderID)
	}
	if _, err := svc.PreviewDeliveryNoteDocument(context.Background(), 0); err == nil {
		t.Fatal("PreviewDeliveryNoteDocument invalid order error = nil")
	}

	docs, err := svc.ListDeliveryNoteDocuments(context.Background(), 18)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].OrderID != 18 {
		t.Fatalf("ListDeliveryNoteDocuments() = %+v", docs)
	}
}
