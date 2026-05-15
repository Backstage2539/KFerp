package docconvert

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type GotenbergConverter struct {
	endpoint string
	client   HTTPDoer
}

type GotenbergOption func(*GotenbergConverter)

func WithHTTPClient(client HTTPDoer) GotenbergOption {
	return func(c *GotenbergConverter) {
		if client != nil {
			c.client = client
		}
	}
}

func NewGotenbergConverter(endpoint string, opts ...GotenbergOption) GotenbergConverter {
	c := GotenbergConverter{
		endpoint: strings.TrimSpace(endpoint),
		client:   http.DefaultClient,
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

func (c GotenbergConverter) ConvertDOCXToPDF(ctx context.Context, sourcePath, outputDir string) (string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	outputDir = strings.TrimSpace(outputDir)
	if sourcePath == "" {
		return "", fmt.Errorf("source path required")
	}
	if outputDir == "" {
		return "", fmt.Errorf("output dir required")
	}
	if c.endpoint == "" {
		return "", fmt.Errorf("gotenberg endpoint required")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", filepath.Base(sourcePath))
	if err != nil {
		return "", err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	if _, err := io.Copy(part, source); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(limited))
		if msg == "" {
			msg = resp.Status
		}
		return "", fmt.Errorf("gotenberg convert failed: %s", msg)
	}

	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath)) + ".pdf"
	pdfPath := filepath.Join(outputDir, base)
	out, err := os.Create(pdfPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}
	return pdfPath, nil
}
