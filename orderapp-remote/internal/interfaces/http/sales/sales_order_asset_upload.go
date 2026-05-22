package sales

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

func saveUploadedSalesOrderAsset(c echo.Context, salesSvc *salesapp.Service, assetDir, kind string) (salesapp.SalesOrderAsset, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return salesapp.SalesOrderAsset{}, fmt.Errorf("file required")
	}
	src, err := file.Open()
	if err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, 8<<20))
	if err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	if len(data) == 0 {
		return salesapp.SalesOrderAsset{}, fmt.Errorf("empty file")
	}
	filename := filepath.Base(file.Filename)
	objectKey := filepath.ToSlash(filepath.Join("sales_order_assets", kind, fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)))
	if err := os.MkdirAll(filepath.Dir(filepath.Join(assetDir, objectKey)), 0755); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	if err := os.WriteFile(filepath.Join(assetDir, objectKey), data, 0644); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	sum := sha256.Sum256(data)
	return salesSvc.SaveSalesOrderAsset(c.Request().Context(), salesapp.SaveSalesOrderAssetCommand{
		Actor:       support.ActorOf(c),
		Kind:        kind,
		Filename:    filename,
		ContentType: file.Header.Get("Content-Type"),
		Bytes:       int64(len(data)),
		SHA256:      hex.EncodeToString(sum[:]),
		ObjectKey:   objectKey,
	})
}
