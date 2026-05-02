package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestOrderInvoiceFileContentTypeAllowsPDFAndImagesOnly(t *testing.T) {
	cases := []struct {
		name        string
		filename    string
		headerType  string
		data        []byte
		want        string
		wantErrText string
	}{
		{name: "pdf", filename: "invoice.pdf", headerType: "application/pdf", data: []byte("%PDF-1.7\nbody"), want: "application/pdf"},
		{name: "png", filename: "invoice.png", headerType: "image/png", data: []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), want: "image/png"},
		{name: "jpeg", filename: "invoice.jpg", headerType: "image/jpeg", data: []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00"), want: "image/jpeg"},
		{name: "text", filename: "invoice.txt", headerType: "text/plain", data: []byte("hello"), wantErrText: "only PDF and image files are allowed"},
		{name: "svg", filename: "invoice.svg", headerType: "image/svg+xml", data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), wantErrText: "only PDF and image files are allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := classifyOrderInvoiceFile(tc.filename, tc.headerType, tc.data)
			if tc.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrText) {
					t.Fatalf("classifyOrderInvoiceFile error=%v, want %q", err, tc.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("classifyOrderInvoiceFile: %v", err)
			}
			if got != tc.want {
				t.Fatalf("content type=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestOrderInvoiceAPIRequestsAndUploadsPDFAndImage(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (35, 'SO-INVOICE-0001', '2026-05-02', 3, 1, 2, 1, 1, 88, false);
	`, schema))

	assetDir := t.TempDir()
	e := newOrderInvoiceAPITestEcho(pool, schema, assetDir)

	req := httptest.NewRequest(http.MethodPost, "/api/orders/35/invoice-request", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"requested"`) {
		t.Fatalf("invoice request status=%d body=%s", rec.Code, rec.Body.String())
	}

	body, contentType := multipartInvoiceFileBody(t, "invoice.pdf", "application/pdf", []byte("%PDF-1.7\ninvoice"))
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/orders/35/invoice-file", body)
	uploadReq.Header.Set(echo.HeaderContentType, contentType)
	uploadRec := httptest.NewRecorder()
	e.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusOK {
		t.Fatalf("invoice upload status=%d body=%s", uploadRec.Code, uploadRec.Body.String())
	}
	var resp salesapp.OrderInvoice
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode invoice response: %v", err)
	}
	if resp.Status != "uploaded" || resp.Asset == nil || resp.Asset.ContentType != "application/pdf" || resp.Asset.URL == "" {
		t.Fatalf("invoice upload response=%+v body=%s", resp, uploadRec.Body.String())
	}
	if _, err := osStatInvoiceAsset(assetDir, resp.Asset.ObjectKey); err != nil {
		t.Fatalf("uploaded invoice file missing: %v object_key=%q", err, resp.Asset.ObjectKey)
	}

	var status, filename, contentTypeDB string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT oi.status, a.filename, a.content_type
		FROM %s.order_invoices oi
		JOIN %s.sales_order_assets a ON a.id=oi.invoice_asset_id
		WHERE oi.order_id=35
	`, schema, schema)).Scan(&status, &filename, &contentTypeDB); err != nil {
		t.Fatalf("query order invoice: %v", err)
	}
	if status != "uploaded" || filename != "invoice.pdf" || contentTypeDB != "application/pdf" {
		t.Fatalf("invoice row status=%q filename=%q content_type=%q", status, filename, contentTypeDB)
	}

	imageBody, imageContentType := multipartInvoiceFileBody(t, "invoice.png", "image/png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
	imageReq := httptest.NewRequest(http.MethodPost, "/api/orders/35/invoice-file", imageBody)
	imageReq.Header.Set(echo.HeaderContentType, imageContentType)
	imageRec := httptest.NewRecorder()
	e.ServeHTTP(imageRec, imageReq)
	if imageRec.Code != http.StatusOK {
		t.Fatalf("invoice image upload status=%d body=%s", imageRec.Code, imageRec.Body.String())
	}
	var imageResp salesapp.OrderInvoice
	if err := json.Unmarshal(imageRec.Body.Bytes(), &imageResp); err != nil {
		t.Fatalf("decode image invoice response: %v", err)
	}
	if imageResp.Status != "uploaded" || imageResp.Asset == nil || imageResp.Asset.ContentType != "image/png" || imageResp.Asset.Filename != "invoice.png" {
		t.Fatalf("invoice image response=%+v body=%s", imageResp, imageRec.Body.String())
	}
}

func TestOrderInvoiceAPIRejectsTextUpload(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (36, 'SO-INVOICE-0002', '2026-05-02', 3, 1, 2, 1, 1, 88, false);
	`, schema))

	e := newOrderInvoiceAPITestEcho(pool, schema, t.TempDir())
	body, contentType := multipartInvoiceFileBody(t, "invoice.txt", "text/plain", []byte("not an invoice pdf or image"))
	req := httptest.NewRequest(http.MethodPost, "/api/orders/36/invoice-file", body)
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "only PDF and image files are allowed") {
		t.Fatalf("text upload status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func newOrderInvoiceAPITestEcho(pool *pgxpool.Pool, schema string, assetDir string) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	svc := salesapp.NewService(postgressales.NewRepository(pool, schema, postgressales.WithSalesOrderAssetDir(assetDir)))
	registerOrderInvoiceRoutes(e, svc, assetDir)
	return e
}

func multipartInvoiceFileBody(t *testing.T, filename, contentType string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename)},
		"Content-Type":        {contentType},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func osStatInvoiceAsset(assetDir, objectKey string) (string, error) {
	path := filepath.Join(assetDir, filepath.FromSlash(objectKey))
	_, err := os.Stat(path)
	return path, err
}
