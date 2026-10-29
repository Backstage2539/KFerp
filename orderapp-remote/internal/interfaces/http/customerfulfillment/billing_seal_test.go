package customerfulfillment

import (
	"context"
	salesapp "orderapp/internal/application/sales"
	"os"
	"path/filepath"
	"testing"
)

type testBillingSeals struct{ assets []salesapp.SalesOrderAsset }

func (s testBillingSeals) SaveOrder(context.Context, salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	return salesapp.SaveOrderResult{}, nil
}
func (s testBillingSeals) ListSalesOrderSealAssets(context.Context) ([]salesapp.SalesOrderAsset, error) {
	return s.assets, nil
}
func TestBillingSealReadsOnlySelectedAssetInsideSealDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sales_order_assets"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sales_order_assets", "seal.png"), []byte("seal"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "private"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	a := api{assetDir: root, sales: testBillingSeals{assets: []salesapp.SalesOrderAsset{{ID: 1, Kind: "seal", ObjectKey: "sales_order_assets/seal.png"}, {ID: 2, Kind: "seal", ObjectKey: "sales_order_assets/../private"}}}}
	got, err := a.readBillingSeal(context.Background(), 1)
	if err != nil || string(got) != "seal" {
		t.Fatalf("read seal=%q err=%v", got, err)
	}
	if _, err = a.readBillingSeal(context.Background(), 2); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err = a.readBillingSeal(context.Background(), 3); err == nil {
		t.Fatal("missing seal accepted")
	}
}
