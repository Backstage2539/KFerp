package customer

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	customerapp "orderapp/internal/application/customer"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type fakeCustomerRepo struct {
	upsert    customerapp.UpsertCommand
	listQuery customerapp.ListQuery
	assetCmd  customerapp.SaveAssetCommand
}

func (r *fakeCustomerRepo) Upsert(ctx context.Context, actor string, id *int64, cmd customerapp.UpsertCommand) (int64, error) {
	r.upsert = cmd
	return 9, nil
}

func (r *fakeCustomerRepo) List(ctx context.Context, query customerapp.ListQuery) (customerapp.ListResult, error) {
	r.listQuery = query
	return customerapp.ListResult{Total: 37}, nil
}

func (r *fakeCustomerRepo) Editor(ctx context.Context, id int64) (*customerapp.EditorData, error) {
	return &customerapp.EditorData{
		Customer: customerapp.CustomerEditData{
			ID:                    id,
			Name:                  r.upsert.Name,
			CustomerType:          r.upsert.CustomerType,
			CompanyName:           r.upsert.CompanyName,
			CompanyAddress:        r.upsert.CompanyAddress,
			CompanyPhone:          r.upsert.CompanyPhone,
			DefaultSourceID:       r.upsert.DefaultSourceID,
			DefaultOrderTypeID:    r.upsert.DefaultOrderTypeID,
			PortalEnabled:         r.upsert.PortalEnabled != nil && *r.upsert.PortalEnabled,
			CapabilityTemplateKey: r.upsert.CapabilityTemplateKey,
			Active:                true,
		},
	}, nil
}

func (r *fakeCustomerRepo) Prefs(ctx context.Context, id int64) (*customerapp.Prefs, error) {
	return &customerapp.Prefs{ID: id}, nil
}

func (r *fakeCustomerRepo) AssetObject(ctx context.Context, assetID int64) (customerapp.AssetObject, error) {
	return customerapp.AssetObject{}, nil
}

func (r *fakeCustomerRepo) SaveAsset(ctx context.Context, cmd customerapp.SaveAssetCommand) (customerapp.SaveAssetResult, error) {
	r.assetCmd = cmd
	return customerapp.SaveAssetResult{CustomerID: cmd.CustomerID, ObjectKey: "customers/9/logo.png", Bytes: 68, SHA256: "sha"}, nil
}

func (r *fakeCustomerRepo) DeleteAsset(ctx context.Context, actor string, assetID int64) (customerapp.DeleteAssetResult, error) {
	return customerapp.DeleteAssetResult{}, nil
}

func (r *fakeCustomerRepo) InlineUpdate(ctx context.Context, actor string, id int64, cmd customerapp.InlineUpdateCommand) error {
	return nil
}

func (r *fakeCustomerRepo) Delete(ctx context.Context, actor string, id int64) error {
	return nil
}

func TestCustomerAPIStoresCompanyContactFields(t *testing.T) {
	repo := &fakeCustomerRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Customer: customerapp.NewService(repo)})

	body := strings.NewReader(`{"name":"张三","customer_type":"wholesale","company_name":"张三咖啡公司","company_address":"上海市徐汇区","company_phone":"021-12345678","default_source_id":1,"default_order_type_id":2,"active":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/customers", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/customers status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"company_name":"张三咖啡公司"`,
		`"company_address":"上海市徐汇区"`,
		`"company_phone":"021-12345678"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing %s: %s", want, rec.Body.String())
		}
	}
	if repo.upsert.CompanyName != "张三咖啡公司" || repo.upsert.CompanyAddress != "上海市徐汇区" || repo.upsert.CompanyPhone != "021-12345678" {
		t.Fatalf("upsert company fields = %+v", repo.upsert)
	}
}

func TestCustomerAPIRoundTripsCustomerType(t *testing.T) {
	repo := &fakeCustomerRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Customer: customerapp.NewService(repo)})

	body := strings.NewReader(`{"name":"岩师傅","customer_type":"wholesale","default_source_id":1,"default_order_type_id":2,"active":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/customers", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/customers status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.upsert.CustomerType != customerapp.CustomerTypeWholesale {
		t.Fatalf("upsert customer_type=%q, want wholesale", repo.upsert.CustomerType)
	}
	if !strings.Contains(rec.Body.String(), `"customer_type":"wholesale"`) {
		t.Fatalf("response missing customer_type: %s", rec.Body.String())
	}
}

func TestCustomerAPISupportsChannelPortalSwitchWithoutTemplateBinding(t *testing.T) {
	repo := &fakeCustomerRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Customer: customerapp.NewService(repo)})

	body := strings.NewReader(`{"name":"渠道伙伴","customer_type":"channel","default_source_id":1,"default_order_type_id":2,"portal_enabled":true,"capability_template_key":"channel_direct_ship","active":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/customers", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/customers status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"customer_type":"channel"`,
		`"portal_enabled":true`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing %s: %s", want, rec.Body.String())
		}
	}
	if repo.upsert.CapabilityTemplateKey != "" {
		t.Fatalf("customer API should not bind capability template from customer profile, got %+v", repo.upsert)
	}
}

func TestCustomerAPIRequiresCustomerTypeSourceAndOrderType(t *testing.T) {
	repo := &fakeCustomerRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Customer: customerapp.NewService(repo)})

	body := strings.NewReader(`{"name":"历史客户","active":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/customers", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/customers status=%d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"客户类型", "来源", "订单类型"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing required %s error: %s", want, rec.Body.String())
		}
	}
	if repo.upsert.Name != "" {
		t.Fatalf("repo should not be called for invalid customer defaults, got %+v", repo.upsert)
	}
}

func TestCustomerAPIIndexSupportsFiltersAndSortParams(t *testing.T) {
	repo := &fakeCustomerRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Customer: customerapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/customers?customer_type=wholesale&active=false&sort_by=updated&sort_direction=desc&page=3&q=王", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/customers status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.listQuery.CustomerType != customerapp.CustomerTypeWholesale {
		t.Fatalf("list customer_type=%q, want wholesale", repo.listQuery.CustomerType)
	}
	if repo.listQuery.Active == nil || *repo.listQuery.Active != false {
		t.Fatalf("list active=%v, want false pointer", repo.listQuery.Active)
	}
	if repo.listQuery.SortBy != "updated" {
		t.Fatalf("list sort_by=%q, want updated", repo.listQuery.SortBy)
	}
	if repo.listQuery.SortDirection != "desc" {
		t.Fatalf("list sort_direction=%q, want desc", repo.listQuery.SortDirection)
	}
	if repo.listQuery.Limit != 10 {
		t.Fatalf("list limit=%d, want 10", repo.listQuery.Limit)
	}
	if repo.listQuery.Offset != 20 {
		t.Fatalf("list offset=%d, want 20", repo.listQuery.Offset)
	}
	if repo.listQuery.Query != "王" {
		t.Fatalf("list query=%q, want 王", repo.listQuery.Query)
	}
	for _, want := range []string{`"total":37`, `"total_pages":4`, `"page":3`, `"limit":10`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("GET /api/customers missing pagination marker %s: %s", want, rec.Body.String())
		}
	}
}

func TestCustomerAssetUploadUsesImageSizeLimit(t *testing.T) {
	repo := &fakeCustomerRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Customer: customerapp.NewService(repo)})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("kind", "logo"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/customers/9/assets/upload", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	req.Header.Set(echo.HeaderAccept, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("asset upload status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.assetCmd.MaxBytes != 8<<20 {
		t.Fatalf("asset upload max bytes=%d, want 8MB image limit", repo.assetCmd.MaxBytes)
	}
	if repo.assetCmd.ContentType != "image/png" {
		t.Fatalf("asset upload content type=%q, want image/png", repo.assetCmd.ContentType)
	}
}
