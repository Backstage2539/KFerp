package billingseal

import (
	"context"
	sales "orderapp/internal/application/sales"
	"os"
	"path/filepath"
	"testing"
)

type sealProvider []sales.SalesOrderAsset

func (p sealProvider) ListSalesOrderSealAssets(context.Context) ([]sales.SalesOrderAsset, error) {
	return p, nil
}
func TestReadUsesOnlySelectedConfinedSeal(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sales_order_assets")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "seal.png"), []byte("seal"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "private"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "private"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "large"), make([]byte, (8<<20)+1), 0600); err != nil {
		t.Fatal(err)
	}
	p := sealProvider{{ID: 1, Kind: "seal", ObjectKey: "sales_order_assets/seal.png"}, {ID: 2, Kind: "seal", ObjectKey: "sales_order_assets/../private"}, {ID: 3, Kind: "seal", ObjectKey: "sales_order_assets/link"}, {ID: 4, Kind: "image", ObjectKey: "sales_order_assets/seal.png"}, {ID: 5, Kind: "seal", ObjectKey: "sales_order_assets/large"}}
	got, err := Read(context.Background(), p, root, 1)
	if err != nil || string(got) != "seal" {
		t.Fatalf("read=%q err=%v", got, err)
	}
	for _, id := range []int64{2, 3, 4, 5, 6} {
		if _, err := Read(context.Background(), p, root, id); err == nil {
			t.Fatalf("id=%d accepted", id)
		}
	}
	if got, err := Read(context.Background(), nil, root, 0); err != nil || got != nil {
		t.Fatalf("empty=%q %v", got, err)
	}
}
