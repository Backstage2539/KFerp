package company

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	companyapp "orderapp/internal/application/company"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestCompanyProfileAPI(t *testing.T) {
	pool, schema := newCompanyProfileAPITestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Company: companyapp.NewService(postgrescompany.NewRepository(pool, schema))})

	body := strings.NewReader(`{"company_name":" 棵凡咖啡 ","company_address":" 昆明市人民东路 ","company_phone":" 0871-12345678 "}`)
	req := httptest.NewRequest(http.MethodPost, "/api/company/profile", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST profile status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"company_name":"棵凡咖啡"`,
		`"company_address":"昆明市人民东路"`,
		`"company_phone":"0871-12345678"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("POST profile response missing %s: %s", want, rec.Body.String())
		}
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/company/profile", nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK || !strings.Contains(getRec.Body.String(), `"company_name":"棵凡咖啡"`) {
		t.Fatalf("GET profile status=%d body=%s", getRec.Code, getRec.Body.String())
	}
}

func newCompanyProfileAPITestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for company profile API tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_company_profile_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}
