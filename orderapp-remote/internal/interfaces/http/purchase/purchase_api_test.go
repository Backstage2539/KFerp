package purchase

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	purchaseapp "orderapp/internal/application/purchase"
	stockapp "orderapp/internal/application/stock"

	"github.com/labstack/echo/v4"
)

type apiRepo struct {
	receipt purchaseapp.CreatePurchaseReceiptCommand
}

func (r *apiRepo) ListSuppliers(ctx context.Context) ([]purchaseapp.Supplier, error) {
	return []purchaseapp.Supplier{{ID: 2, Name: "生豆供应商", Active: true}}, nil
}

func (r *apiRepo) SaveSupplier(ctx context.Context, cmd purchaseapp.SaveSupplierCommand) (purchaseapp.Supplier, error) {
	return purchaseapp.Supplier{ID: 2, Name: cmd.Name, Active: true}, nil
}

func (r *apiRepo) ListPurchaseOrders(ctx context.Context) ([]purchaseapp.PurchaseOrder, error) {
	return nil, nil
}

func (r *apiRepo) CreatePurchaseOrder(ctx context.Context, cmd purchaseapp.CreatePurchaseOrderCommand) (purchaseapp.PurchaseOrder, error) {
	return purchaseapp.PurchaseOrder{ID: 3, OrderNo: "PO-20260428-0001"}, nil
}

func (r *apiRepo) ListPurchaseReceipts(ctx context.Context) ([]purchaseapp.PurchaseReceipt, error) {
	return nil, nil
}

func (r *apiRepo) CreatePurchaseReceipt(ctx context.Context, cmd purchaseapp.CreatePurchaseReceiptCommand, stockResult stockapp.MaterialReceiptResult) (purchaseapp.PurchaseReceipt, error) {
	r.receipt = cmd
	return purchaseapp.PurchaseReceipt{ID: 4, ReceiptNo: "PRC-20260428-0001", StockBatchCode: stockResult.BatchCode}, nil
}

func (r *apiRepo) UpdateMaterialPurchasePrice(ctx context.Context, materialID int64, unitCost float64) error {
	return nil
}

type apiStock struct{}

func (s apiStock) ReceiveMaterial(ctx context.Context, cmd stockapp.MaterialReceiptCommand) (stockapp.MaterialReceiptResult, error) {
	return stockapp.MaterialReceiptResult{ReceiptID: 9, BatchID: 10, BatchCode: "MB-0000000009"}, nil
}

func TestPurchaseReceiptAPIReceivesMaterial(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("operator_employee", "仓库")
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{Purchase: purchaseapp.NewService(repo, apiStock{})})

	body := bytes.NewBufferString(`{"purchase_order_id":3,"supplier_id":2,"supplier_name":"生豆供应商","material_id":7,"qty_g":2500,"unit_cost":48}`)
	req := httptest.NewRequest(http.MethodPost, "/api/purchase/receipts", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.receipt.Operator != "仓库" || repo.receipt.MaterialID != 7 || repo.receipt.QtyG != 2500 {
		t.Fatalf("receipt command=%+v", repo.receipt)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"stock_batch_code":"MB-0000000009"`)) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}
