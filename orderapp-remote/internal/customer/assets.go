package customer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AssetKind string

const (
	AssetKindLogo AssetKind = "logo"
)

type Asset struct {
	CustomerID  ID
	Kind        AssetKind
	ObjectKey   string // relative path under base dir
	ContentType string
	Bytes       int64
	SHA256      string
	CreatedAt   time.Time
}

var (
	ErrAssetTooLarge     = errors.New("asset too large")
	ErrAssetTypeRejected = errors.New("asset content-type rejected")
)

type AssetStore interface {
	Save(ctx context.Context, customerID ID, kind AssetKind, filename string, r io.Reader, maxBytes int64, allowedTypes []string) (Asset, error)
	Open(ctx context.Context, customerID ID, kind AssetKind) (asset *Asset, rc io.ReadSeekCloser, err error)
}

// LocalDiskAssetStore stores customer assets on local filesystem.
// Pair with a DB table that maps (customer_id,kind) -> object_key.
// For now we rely on deterministic object keys and filesystem existence.
// This is intentionally simple scaffolding.
type LocalDiskAssetStore struct {
	BaseDir string
}

func (s LocalDiskAssetStore) Save(ctx context.Context, customerID ID, kind AssetKind, filename string, r io.Reader, maxBytes int64, allowedTypes []string) (Asset, error) {
	_ = ctx
	if maxBytes <= 0 {
		maxBytes = 2 << 20 // 2MB default
	}
	// sniff type (read up to 512)
	buf := make([]byte, 512)
	n, _ := io.ReadFull(r, buf)
	sniff := http.DetectContentType(buf[:n])
	if len(allowedTypes) > 0 {
		ok := false
		for _, t := range allowedTypes {
			if strings.EqualFold(strings.TrimSpace(t), sniff) {
				ok = true
				break
			}
		}
		if !ok {
			return Asset{}, fmt.Errorf("%w: %s", ErrAssetTypeRejected, sniff)
		}
	}
	ext := extFrom(sniff, filename)
	objKey := filepath.ToSlash(filepath.Join("customers", fmt.Sprintf("%d", customerID), string(kind)+ext))
	full := filepath.Join(s.BaseDir, objKey)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return Asset{}, err
	}
	f, err := os.Create(full)
	if err != nil {
		return Asset{}, err
	}
	defer f.Close()

	h := sha256.New()
	mw := io.MultiWriter(f, h)
	// write sniffed bytes first
	var written int64
	if n > 0 {
		wn, err := mw.Write(buf[:n])
		written += int64(wn)
		if err != nil {
			return Asset{}, err
		}
	}
	lr := &io.LimitedReader{R: r, N: maxBytes - written + 1}
	n2, err := io.Copy(mw, lr)
	written += n2
	if err != nil {
		return Asset{}, err
	}
	if written > maxBytes {
		_ = os.Remove(full)
		return Asset{}, ErrAssetTooLarge
	}

	return Asset{
		CustomerID:  customerID,
		Kind:        kind,
		ObjectKey:   objKey,
		ContentType: sniff,
		Bytes:       written,
		SHA256:      hex.EncodeToString(h.Sum(nil)),
		CreatedAt:   time.Now(),
	}, nil
}

func (s LocalDiskAssetStore) Open(ctx context.Context, customerID ID, kind AssetKind) (*Asset, io.ReadSeekCloser, error) {
	_ = ctx
	// best-effort: try common image types
	candidates := []string{".png", ".jpg", ".jpeg", ".webp"}
	for _, ext := range candidates {
		objKey := filepath.ToSlash(filepath.Join("customers", fmt.Sprintf("%d", customerID), string(kind)+ext))
		full := filepath.Join(s.BaseDir, objKey)
		f, err := os.Open(full)
		if err != nil {
			continue
		}
		ct := mime.TypeByExtension(ext)
		if ct == "" {
			ct = "application/octet-stream"
		}
		st, _ := f.Stat()
		return &Asset{CustomerID: customerID, Kind: kind, ObjectKey: objKey, ContentType: ct, Bytes: st.Size()}, f, nil
	}
	return nil, nil, os.ErrNotExist
}

func extFrom(contentType, filename string) string {
	// prefer extension from original filename
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" {
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp":
			return ext
		}
	}
	// fallback by type
	switch strings.ToLower(contentType) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}
