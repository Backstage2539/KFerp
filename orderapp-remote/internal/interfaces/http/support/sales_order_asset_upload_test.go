package support

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
)

type uploadAssetSaverFake struct {
	cmd   salesapp.SaveSalesOrderAssetCommand
	calls int
	err   error
}

func (f *uploadAssetSaverFake) SaveSalesOrderAsset(_ context.Context, cmd salesapp.SaveSalesOrderAssetCommand) (salesapp.SalesOrderAsset, error) {
	f.cmd, f.calls = cmd, f.calls+1
	return salesapp.SalesOrderAsset{ID: 9, Kind: cmd.Kind, Filename: cmd.Filename}, f.err
}

func assetUploadTestContext(t *testing.T, data []byte) echo.Context {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("file", "receipt.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set(echo.HeaderContentType, form.FormDataContentType())
	return echo.New().NewContext(req, httptest.NewRecorder())
}

func TestSaveUploadedSalesOrderAssetPreservesDataAndExplicitActor(t *testing.T) {
	dir := t.TempDir()
	saver := &uploadAssetSaverFake{}
	data := []byte("receipt bytes")
	asset, err := SaveUploadedSalesOrderAsset(assetUploadTestContext(t, data), saver, dir, "payment_voucher", "mini-employee:7:销售甲")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(saver.cmd.ObjectKey)))
	if err != nil {
		t.Fatal(err)
	}
	if asset.ID != 9 || !bytes.Equal(stored, data) || saver.cmd.Actor != "mini-employee:7:销售甲" || saver.cmd.Bytes != int64(len(data)) || saver.cmd.SHA256 == "" {
		t.Fatalf("asset=%+v cmd=%+v stored=%q", asset, saver.cmd, stored)
	}
}

func TestSaveUploadedSalesOrderAssetRejectsOversizeAndCleansFailedMetadata(t *testing.T) {
	for _, mode := range []string{"oversize", "metadata"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			saver := &uploadAssetSaverFake{}
			data := []byte("receipt bytes")
			if mode == "oversize" {
				data = make([]byte, (8<<20)+1)
			} else {
				saver.err = errors.New("metadata failed")
			}
			if _, err := SaveUploadedSalesOrderAsset(assetUploadTestContext(t, data), saver, dir, "payment_voucher", "employee"); err == nil {
				t.Fatal("expected failure")
			}
			if mode == "oversize" && saver.calls != 0 {
				t.Fatal("oversized file reached metadata save")
			}
			_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					t.Fatal(err)
				}
				if !entry.IsDir() {
					t.Fatalf("orphan upload retained: %s", path)
				}
				return nil
			})
		})
	}
}
