package customer

import (
	"context"
	"net/http"
	"net/http/httptest"
	customerapp "orderapp/internal/application/customer"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type fakeCustomerRepo struct {
	upsert customerapp.UpsertCommand
}

func (r *fakeCustomerRepo) Upsert(ctx context.Context, actor string, id *int64, cmd customerapp.UpsertCommand) (int64, error) {
	r.upsert = cmd
	return 9, nil
}

func (r *fakeCustomerRepo) List(ctx context.Context, query customerapp.ListQuery) (customerapp.ListResult, error) {
	return customerapp.ListResult{}, nil
}

func (r *fakeCustomerRepo) Editor(ctx context.Context, id int64) (*customerapp.EditorData, error) {
	return &customerapp.EditorData{
		Customer: customerapp.CustomerEditData{
			ID:             id,
			Name:           r.upsert.Name,
			CompanyName:    r.upsert.CompanyName,
			CompanyAddress: r.upsert.CompanyAddress,
			CompanyPhone:   r.upsert.CompanyPhone,
			Active:         true,
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
	return customerapp.SaveAssetResult{}, nil
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

	body := strings.NewReader(`{"name":"张三","company_name":"张三咖啡公司","company_address":"上海市徐汇区","company_phone":"021-12345678","active":true}`)
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
