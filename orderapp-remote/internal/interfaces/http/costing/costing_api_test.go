package costing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appcosting "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"

	"github.com/labstack/echo/v4"
)

type fakeService struct{}

func (fakeService) Parameters(context.Context) (domain.Parameters, error) {
	return domain.DefaultParameters(), nil
}

func (fakeService) Calculate(ctx context.Context, req appcosting.CalculateRequest) (*appcosting.CalculateResponse, error) {
	items, err := appcosting.NewService(&fakeRepo{}).Calculate(ctx, req)
	return items, err
}

func (fakeService) BeanList(context.Context) (*appcosting.CalculateResponse, error) {
	return &appcosting.CalculateResponse{Parameters: domain.DefaultParameters()}, nil
}

func (fakeService) CreateRun(context.Context, string) (*appcosting.Run, error) {
	return &appcosting.Run{ID: 1, Status: "draft"}, nil
}

func (fakeService) PublishRun(context.Context, string, int64) error {
	return nil
}

func (fakeService) Settings(context.Context) ([]appcosting.ParameterSetting, error) {
	return []appcosting.ParameterSetting{{Key: "roast_yield_rate", Label: "生豆到熟豆转化率", Value: 0.8, Unit: "ratio"}}, nil
}

func (fakeService) UpdateSetting(context.Context, appcosting.UpdateParameterCommand) (appcosting.ParameterSetting, error) {
	return appcosting.ParameterSetting{Key: "roast_yield_rate", Label: "生豆到熟豆转化率", Value: 0.81, Unit: "ratio"}, nil
}

type fakeRepo struct{}

func (fakeRepo) LoadParameters(context.Context) (domain.Parameters, error) {
	return domain.DefaultParameters(), nil
}

func (fakeRepo) LoadProductInputs(context.Context, domain.Parameters) ([]domain.ProductInput, error) {
	return nil, nil
}

func (fakeRepo) CreateRun(context.Context, string, []domain.ProductResult) (*appcosting.Run, error) {
	return nil, nil
}

func (fakeRepo) PublishRun(context.Context, string, int64) error {
	return nil
}

func (fakeRepo) ListParameterSettings(context.Context) ([]appcosting.ParameterSetting, error) {
	return nil, nil
}

func (fakeRepo) UpdateParameterSetting(context.Context, appcosting.UpdateParameterCommand) (appcosting.ParameterSetting, error) {
	return appcosting.ParameterSetting{}, nil
}

func TestCostingCalculateAPI(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{{
		ProductID:          1,
		Name:               "金色山脉",
		GreenBeanCostPerKg: 62,
		YieldRate:          0.8,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/calculate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got appcosting.CalculateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "金色山脉" {
		t.Fatalf("items = %+v", got.Items)
	}
}

func TestCostingCalculateAPIReturnsExcelTierSchemeMetadata(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{{
		ProductID:           1,
		Name:                "白月光-瑰夏",
		GreenBeanCostPerKg:  362.5,
		YieldRate:           0.8,
		WholesaleTierScheme: domain.WholesaleTierScheme227GTwo,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/calculate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got appcosting.CalculateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(got.Items) != 1 || len(got.Items[0].CommercialWholesaleTiers) != 2 {
		t.Fatalf("items = %+v", got.Items)
	}
	tier := got.Items[0].CommercialWholesaleTiers[0]
	if tier.SpecG != 227 || tier.Label != "2包-7包" || tier.PricePerUnit <= 0 {
		t.Fatalf("tier = %+v", tier)
	}
}

func TestRoutesAreRegistered(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})
	seen := map[string]bool{}
	for _, route := range e.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /api/costing/parameters",
		"GET /api/costing/settings",
		"POST /api/costing/settings/:key",
		"POST /api/costing/calculate",
		"GET /api/costing/bean-list",
		"POST /api/costing/runs",
		"POST /api/costing/runs/:id/publish",
	} {
		if !seen[want] {
			t.Fatalf("missing route %s; got %+v", want, seen)
		}
	}
}

func TestCostingSettingsAPI(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	body := bytes.NewBufferString(`{"value":0.81}`)
	req = httptest.NewRequest(http.MethodPost, "/api/costing/settings/roast_yield_rate", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got appcosting.ParameterSetting
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Key != "roast_yield_rate" || got.Value != 0.81 {
		t.Fatalf("setting = %+v", got)
	}
}
