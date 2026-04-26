package appmain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerAsset struct {
	ID          int64
	CustomerID  int64
	Kind        string
	ObjectKey   string
	ContentType string
	Bytes       int64
	Sha256      string
	CreatedAt   string
}

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// fetchCustomerAssets returns all assets for a customer.
func fetchCustomerAssets(ctx context.Context, pool *pgxpool.Pool, schema string, customerID int64) ([]CustomerAsset, error) {
	q := fmt.Sprintf(`
		SELECT id, customer_id, kind, object_key, content_type, bytes, sha256,
			to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_assets
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
	`, schema)
	rows, err := pool.Query(ctx, q, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CustomerAsset, 0)
	for rows.Next() {
		var a CustomerAsset
		if err := rows.Scan(&a.ID, &a.CustomerID, &a.Kind, &a.ObjectKey, &a.ContentType, &a.Bytes, &a.Sha256, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func kindLabel(kind string) string {
	switch kind {
	case "label_front":
		return "标签-正面"
	case "label_back":
		return "标签-反面"
	case "bag":
		return "豆袋"
	case "drip_box":
		return "挂耳盒"
	case "print_requirement":
		return "印刷需求"
	default:
		return kind
	}
}

func extByContentType(ct string) string {
	s, _ := mime.ExtensionsByType(ct)
	if len(s) > 0 {
		// prefer common
		for _, e := range s {
			if e == ".jpg" || e == ".jpeg" || e == ".png" || e == ".webp" {
				return e
			}
		}
		return s[0]
	}
	if ct == "image/jpeg" {
		return ".jpg"
	}
	if ct == "image/png" {
		return ".png"
	}
	if ct == "image/webp" {
		return ".webp"
	}
	return ""
}

func saveCustomerAssetFile(assetDir string, customerID int64, kind string, r io.Reader, contentType string, maxBytes int64, filename string) (objectKey string, size int64, sha string, err error) {
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
	if !allowedImageTypes[contentType] {
		fn := strings.ToLower(strings.TrimSpace(filename))
		if strings.HasSuffix(fn, ".heic") || strings.HasSuffix(fn, ".heif") {
			return "", 0, "", fmt.Errorf("不支持 HEIC 图片：请在 iPhone 分享→选项→格式选“最兼容”(JPEG)，或先转换成 JPG/PNG 后再上传")
		}
		return "", 0, "", fmt.Errorf("不支持的图片格式（仅支持 JPG/PNG/WebP）")
	}
	ext := extByContentType(contentType)
	if ext == "" {
		return "", 0, "", fmt.Errorf("unknown file type")
	}

	base := fmt.Sprintf("customers/%d/%s/%d%s", customerID, kind, time.Now().UnixNano(), ext)
	path := filepath.Join(assetDir, base)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", 0, "", err
	}

	f, err := os.Create(path)
	if err != nil {
		return "", 0, "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	mw := io.MultiWriter(f, h)
	lr := &io.LimitedReader{R: r, N: maxBytes + 1}
	n, err := io.Copy(mw, lr)
	if err != nil {
		_ = os.Remove(path)
		return "", 0, "", err
	}
	if n > maxBytes {
		_ = os.Remove(path)
		return "", 0, "", fmt.Errorf("file too large")
	}

	sha = hex.EncodeToString(h.Sum(nil))
	return base, n, sha, nil
}

func insertCustomerAsset(ctx context.Context, pool *pgxpool.Pool, schema, actor string, customerID int64, kind string, objectKey string, contentType string, bytes int64, sha string) (int64, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return 0, fmt.Errorf("kind required")
	}
	q := fmt.Sprintf(`
		INSERT INTO %s.customer_assets(customer_id, kind, object_key, content_type, bytes, sha256, created_at, created_by)
		VALUES ($1,$2,$3,$4,$5,$6, now(), $7)
		RETURNING id
	`, schema)
	var id int64
	if err := pool.QueryRow(ctx, q, customerID, kind, objectKey, contentType, bytes, sha, actor).Scan(&id); err != nil {
		return 0, err
	}
	auditInsert(ctx, pool, schema, actor, "customer_asset", &customerID, "upload", strPtrStr("kind"), nil, strPtrStr(kind), AuditMeta{"asset_id": id, "object_key": objectKey, "bytes": bytes, "content_type": contentType})
	return id, nil
}

func deleteCustomerAssetByID(ctx context.Context, pool *pgxpool.Pool, schema, actor string, assetID int64) (customerID int64, kind string, objectKey string, err error) {
	q0 := fmt.Sprintf(`SELECT customer_id, kind, object_key FROM %s.customer_assets WHERE id=$1`, schema)
	if err := pool.QueryRow(ctx, q0, assetID).Scan(&customerID, &kind, &objectKey); err != nil {
		return 0, "", "", err
	}
	q := fmt.Sprintf(`DELETE FROM %s.customer_assets WHERE id=$1`, schema)
	if _, err := pool.Exec(ctx, q, assetID); err != nil {
		return 0, "", "", err
	}
	auditInsert(ctx, pool, schema, actor, "customer_asset", &customerID, "delete", strPtrStr("asset_id"), strPtrStr(fmt.Sprintf("%d", assetID)), nil, AuditMeta{"kind": kind})
	return customerID, kind, objectKey, nil
}

// serve asset by objectKey relative to assetDir
func serveAssetFile(w http.ResponseWriter, assetDir string, objectKey string, contentType string) error {
	objectKey = strings.TrimPrefix(objectKey, "/")
	path := filepath.Join(assetDir, objectKey)
	bs, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=60")
	_, _ = w.Write(bs)
	return nil
}

// (intentionally no extra types)
