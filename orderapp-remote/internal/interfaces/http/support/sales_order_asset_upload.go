package support

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
)

type SalesOrderAssetSaver interface {
	SaveSalesOrderAsset(context.Context, salesapp.SaveSalesOrderAssetCommand) (salesapp.SalesOrderAsset, error)
}

// SaveUploadedSalesOrderAsset shares file persistence across authenticated order
// entry points. Callers authorize the order and supply the authenticated actor.
func SaveUploadedSalesOrderAsset(c echo.Context, saver SalesOrderAssetSaver, assetDir, kind, actor string) (salesapp.SalesOrderAsset, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return salesapp.SalesOrderAsset{}, echo.NewHTTPError(http.StatusBadRequest, "请选择收款凭证文件")
	}
	const maxBytes = 8 << 20
	if file.Size > maxBytes {
		return salesapp.SalesOrderAsset{}, echo.NewHTTPError(http.StatusBadRequest, "收款凭证不能超过8MB")
	}
	src, err := file.Open()
	if err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxBytes+1))
	if err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	if len(data) == 0 {
		return salesapp.SalesOrderAsset{}, echo.NewHTTPError(http.StatusBadRequest, "收款凭证文件为空")
	}
	if len(data) > maxBytes {
		return salesapp.SalesOrderAsset{}, echo.NewHTTPError(http.StatusBadRequest, "收款凭证不能超过8MB")
	}
	filename := filepath.Base(file.Filename)
	objectKey := filepath.ToSlash(filepath.Join("sales_order_assets", kind, fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)))
	path := filepath.Join(assetDir, filepath.FromSlash(objectKey))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	sum := sha256.Sum256(data)
	asset, err := saver.SaveSalesOrderAsset(c.Request().Context(), salesapp.SaveSalesOrderAssetCommand{
		Actor: actor, Kind: kind, Filename: filename, ContentType: file.Header.Get("Content-Type"),
		Bytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), ObjectKey: objectKey,
	})
	if err != nil {
		_ = os.Remove(path)
		return salesapp.SalesOrderAsset{}, err
	}
	return asset, nil
}
