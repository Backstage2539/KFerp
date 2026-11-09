package billingseal

import (
	"context"
	"fmt"
	"io"
	salesapp "orderapp/internal/application/sales"
	"os"
	"path/filepath"
	"strings"
)

type billingSealProvider interface {
	ListSalesOrderSealAssets(context.Context) ([]salesapp.SalesOrderAsset, error)
}

// Read resolves only the configured seal asset under the seal directory.
func Read(ctx context.Context, sales any, assetDir string, id int64) ([]byte, error) {
	if id == 0 {
		return nil, nil
	}
	provider, ok := sales.(billingSealProvider)
	if !ok {
		return nil, fmt.Errorf("账单公章资源不可用")
	}
	assets, err := provider.ListSalesOrderSealAssets(ctx)
	if err != nil {
		return nil, err
	}
	var key string
	for _, asset := range assets {
		if asset.ID == id && asset.Kind == "seal" {
			key = asset.ObjectKey
			break
		}
	}
	if !strings.HasPrefix(key, "sales_order_assets/") {
		return nil, fmt.Errorf("账单公章不存在")
	}
	root, err := filepath.EvalSymlinks(filepath.Join(assetDir, "sales_order_assets"))
	if err != nil {
		return nil, fmt.Errorf("账单公章资源不可用")
	}
	path, err := filepath.EvalSymlinks(filepath.Join(assetDir, filepath.FromSlash(key)))
	if err != nil {
		return nil, fmt.Errorf("账单公章不存在")
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return nil, fmt.Errorf("账单公章路径无效")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("账单公章文件过大")
	}
	return data, nil
}
