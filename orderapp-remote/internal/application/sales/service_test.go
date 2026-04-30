package sales

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	saveCmd         SaveOrderCommand
	inlineCmd       InlineUpdateCommand
	saveCalls       int
	outsourceSaved  SaveOutsourceTemplateCommand
	trackingCmd     FillTrackingPairsCommand
	shipMethodCmd   SetShipMethodCommand
	shipmentCmd     CreateOrderShipmentCommand
	trackingItems   FillShipmentTrackingCommand
	orderNoTracking FillShipmentTrackingByOrderNoCommand
	settingsCmd     SaveSalesOrderSettingsCommand
	generateCmd     GenerateSalesOrderDocumentCommand
	previewOrderID  int64
}

func (r *fakeRepo) SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error) {
	r.saveCmd = cmd
	r.saveCalls++
	return SaveOrderResult{OrderID: 7, OrderNo: "SO-TEST", Edited: cmd.EditID > 0}, nil
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

func (r *fakeRepo) Unvoid(ctx context.Context, id int64, actor string) error {
	return nil
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

func (r *fakeRepo) SaveSalesOrderPaymentCode(ctx context.Context, cmd SaveSalesOrderPaymentCodeCommand) (SalesOrderPaymentCode, error) {
	return SalesOrderPaymentCode{ID: 4, Label: cmd.Label, Description: cmd.Description, AssetID: cmd.AssetID, Sort: cmd.Sort, Active: cmd.Active}, nil
}

func (r *fakeRepo) DeleteSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	return nil
}

func (r *fakeRepo) SetSalesOrderSealAsset(ctx context.Context, assetID int64, actor string) error {
	return nil
}

func (r *fakeRepo) ListSalesOrderDocuments(ctx context.Context, orderID int64) ([]SalesOrderDocument, error) {
	return []SalesOrderDocument{{ID: 9, OrderID: orderID, OrderNo: "SO-TEST", VersionNo: 2, IsLatest: true}}, nil
}

func (r *fakeRepo) LoadSalesOrderContext(ctx context.Context, orderID int64) (SalesOrderContext, error) {
	return SalesOrderContext{OrderID: orderID, OrderNo: "SO-TEST", Customer: SalesOrderCustomerInfo{ID: 3, Name: "测试客户"}}, nil
}

func (r *fakeRepo) GenerateSalesOrderDocument(ctx context.Context, cmd GenerateSalesOrderDocumentCommand) (GenerateSalesOrderDocumentResult, error) {
	r.generateCmd = cmd
	return GenerateSalesOrderDocumentResult{Document: SalesOrderDocument{ID: 10, OrderID: cmd.OrderID, OrderNo: "SO-TEST", VersionNo: 1, IsLatest: true}}, nil
}

func (r *fakeRepo) PreviewSalesOrderDocument(ctx context.Context, orderID int64) (SalesOrderPreview, error) {
	r.previewOrderID = orderID
	return SalesOrderPreview{OrderID: orderID, OrderNo: "SO-TEST", NextVersionNo: 3}, nil
}

func (r *fakeRepo) LoadSalesOrderDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (SalesOrderDocumentFile, error) {
	return SalesOrderDocumentFile{Document: SalesOrderDocument{ID: documentID, OrderID: orderID, OrderNo: "SO-TEST", VersionNo: 1}, Path: "/tmp/test.pdf", Filename: "SO-TEST-V1.pdf"}, nil
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
			{OrderID: 20, TrackingNo: "SF123"},
			{OrderID: 21, TrackingNo: ""},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Updated != 1 {
		t.Fatalf("tracking result = %+v", updated)
	}
	if repo.trackingItems.Actor != "shipping" || repo.trackingItems.ShipmentID != 12 || len(repo.trackingItems.Items) != 1 || repo.trackingItems.Items[0].TrackingNo != "SF123" {
		t.Fatalf("tracking command = %+v", repo.trackingItems)
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
	if first.OrderNo != "SO-20260428-0001" || first.TrackingNo != "SF5199040648127" {
		t.Fatalf("first normalized item = %+v", first)
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
	}); err != nil {
		t.Fatal(err)
	}
	if repo.settingsCmd.Actor != "tester" || repo.settingsCmd.CompanyName != "浅焙作坊咖啡" || repo.settingsCmd.Note != "请密封保存" || repo.settingsCmd.PaymentText != "微信或对公转账" {
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
