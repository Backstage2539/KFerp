package customer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDiskAssetStore_SaveAndOpen(t *testing.T) {
	dir := t.TempDir()
	s := LocalDiskAssetStore{BaseDir: dir}
	// minimal PNG header
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	asset, err := s.Save(context.Background(), 123, AssetKindLogo, "logo.png", bytes.NewReader(png), 1024, []string{"image/png"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if asset.ObjectKey == "" {
		t.Fatalf("expected object key")
	}
	full := filepath.Join(dir, asset.ObjectKey)
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	_, rc, err := s.Open(context.Background(), 123, AssetKindLogo)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = rc.Close()
}
