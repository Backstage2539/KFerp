package docconvert

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGotenbergConverterPostsDOCXAndWritesPDF(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "合同.docx")
	if err := os.WriteFile(source, []byte("docx content"), 0644); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(dir, "out")

	converter := NewGotenbergConverter("http://docconvert:3000/forms/libreoffice/convert", WithHTTPClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s", req.Method)
		}
		if req.URL.String() != "http://docconvert:3000/forms/libreoffice/convert" {
			t.Fatalf("url = %s", req.URL.String())
		}
		reader, err := req.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		if part.FormName() != "files" || part.FileName() != "合同.docx" {
			t.Fatalf("multipart field/file = %q/%q", part.FormName(), part.FileName())
		}
		uploaded, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(uploaded) != "docx content" {
			t.Fatalf("uploaded = %q", string(uploaded))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("%PDF-1.4\nconverted")),
			Header:     make(http.Header),
		}, nil
	})))

	pdfPath, err := converter.ConvertDOCXToPDF(context.Background(), source, outputDir)
	if err != nil {
		t.Fatalf("ConvertDOCXToPDF: %v", err)
	}
	if pdfPath != filepath.Join(outputDir, "合同.pdf") {
		t.Fatalf("pdf path = %q", pdfPath)
	}
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "%PDF-1.4\nconverted" {
		t.Fatalf("pdf data = %q", string(data))
	}
}

func TestGotenbergConverterReturnsResponseError(t *testing.T) {
	converter := NewGotenbergConverter("http://docconvert:3000/forms/libreoffice/convert", WithHTTPClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.Close()
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Status:     "400 Bad Request",
			Body:       io.NopCloser(strings.NewReader("invalid document")),
			Header:     make(http.Header),
		}, nil
	})))
	source := filepath.Join(t.TempDir(), "bad.docx")
	if err := os.WriteFile(source, []byte("bad"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := converter.ConvertDOCXToPDF(context.Background(), source, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "invalid document") {
		t.Fatalf("error = %v", err)
	}
}
