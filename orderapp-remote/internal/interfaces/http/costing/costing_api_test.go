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

func TestRoutesAreRegistered(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})
	seen := map[string]bool{}
	for _, route := range e.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /api/costing/parameters",
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
