package costing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	appcosting "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"
	pgcosting "orderapp/internal/infrastructure/postgres/costing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type catalogGuardAPIRepo struct {
	fakeRepo
	guard pgcosting.Repository
}

func (r catalogGuardAPIRepo) FindUnknownCostProductRefs(ctx context.Context, refs []domain.ProductInput) ([]bool, error) {
	return r.guard.FindUnknownCostProductRefs(ctx, refs)
}

type catalogGuardAPIService struct {
	fakeService
	repo catalogGuardAPIRepo
}

func (s catalogGuardAPIService) Calculate(ctx context.Context, req appcosting.CalculateRequest) (*appcosting.CalculateResponse, error) {
	return appcosting.NewService(s.repo).Calculate(ctx, req)
}

func TestCalculateAPIRejectsForgedCatalogIdentityPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("quote_identity_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.products(id bigint primary key,parent_product_id bigint not null default 0);
 CREATE TABLE %[1]s.production_boms(id bigint primary key,output_product_id bigint not null);
 CREATE TABLE %[1]s.production_bom_specs(id bigint primary key,bom_id bigint not null);
 CREATE TABLE %[1]s.production_bom_version_variants(id bigint primary key,bom_spec_id bigint not null);
 INSERT INTO %[1]s.products VALUES(1,0),(2,0),(3,1);
 INSERT INTO %[1]s.production_boms VALUES(10,1),(20,2);
 INSERT INTO %[1]s.production_bom_specs VALUES(101,10),(201,20);
 INSERT INTO %[1]s.production_bom_version_variants VALUES(1001,101),(2001,201);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: catalogGuardAPIService{repo: catalogGuardAPIRepo{guard: pgcosting.NewRepository(pool, schema)}}})
	cases := []struct {
		name    string
		product domain.ProductInput
		status  int
	}{
		{"forged parent", domain.ProductInput{ProductID: 3, ParentProductID: 2, BomSpecID: 201}, 400},
		{"foreign spec", domain.ProductInput{ProductID: 1, BomSpecID: 201}, 400},
		{"foreign variant", domain.ProductInput{ProductID: 1, BomSpecID: 101, BomVariantID: 2001}, 400},
		{"valid sales SKU", domain.ProductInput{ProductID: 3, ParentProductID: 1, BomSpecID: 101, BomVariantID: 1001, Name: "正常规格", GreenBeanCostPerKg: 10, YieldRate: 0.8}, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{tc.product}})
			req := httptest.NewRequest(http.MethodPost, "/api/costing/calculate", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}
