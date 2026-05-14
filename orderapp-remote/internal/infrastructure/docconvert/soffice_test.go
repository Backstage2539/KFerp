package docconvert

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLibreOfficeConverterRunsHeadlessDOCXToPDFCommand(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "合同.docx")
	if err := os.WriteFile(source, []byte("PK\x03\x04docx"), 0644); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	var gotName string
	var gotArgs []string
	converter := NewLibreOfficeConverter("custom-soffice", WithCommandRunner(func(ctx context.Context, name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return os.WriteFile(filepath.Join(outputDir, "合同.pdf"), []byte("%PDF-1.4\nconverted"), 0644)
	}))

	pdfPath, err := converter.ConvertDOCXToPDF(context.Background(), source, outputDir)
	if err != nil {
		t.Fatalf("ConvertDOCXToPDF: %v", err)
	}
	if gotName != "custom-soffice" {
		t.Fatalf("command name = %q", gotName)
	}
	wantArgs := []string{"--headless", "--convert-to", "pdf", "--outdir", outputDir, source}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
	if pdfPath != filepath.Join(outputDir, "合同.pdf") {
		t.Fatalf("pdf path = %q", pdfPath)
	}
}
