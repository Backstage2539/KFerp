package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	resolutionhttp "orderapp/internal/interfaces/http/orderresolution"
	"orderapp/internal/interfaces/http/support"
	"strconv"
	"testing"
)

type scrapAPIAuth struct{ support.AuthzService }

func (scrapAPIAuth) ActorByEmployeeID(_ context.Context, id int64) (authzapp.Actor, error) {
	return authzapp.Actor{EmployeeID: id, Permissions: []string{map[int64]string{7: "orders.resolve", 8: "finance.resolve", 9: "stock.resolve"}[id]}}, nil
}
func TestOrderResolutionCompanyScrapHTTPPermissionsAndRetry(t *testing.T) {
	pool, schema := companyScrapFixture(t)
	r := resolutionpg.NewRepository(pool, schema)
	e := echo.New()
	auth := scrapAPIAuth{}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id, _ := strconv.ParseInt(c.Request().Header.Get("X-Test-Employee"), 10, 64)
			c.Set("employee_id", id)
			c.Set("operator_employee", "scrap-api")
			return next(c)
		}
	})
	e.Use(support.AuthorizationMiddleware(auth))
	resolutionhttp.RegisterRoutes(e, r, auth)
	call := func(employee int64, method, path string, body any, want int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set("X-Test-Employee", fmt.Sprint(employee))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s employee=%d: %d want %d %s", method, path, employee, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	endpoint := "/api/orders/78400/resolution"
	var loaded struct {
		Context domain.Context             `json:"context"`
		Options map[string][]domain.Option `json:"options"`
		History []struct {
			Scraps []struct {
				Unit      string `json:"unit"`
				ID        int64  `json:"id"`
				Quantity  int64  `json:"quantity"`
				CostCents int64  `json:"cost_cents"`
				BatchCode string `json:"batch_code"`
			} `json:"scraps"`
		} `json:"history"`
	}
	if err := json.Unmarshal(call(7, "GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Options["scrap_cost"]) != 2 {
		t.Fatal("cost choices not exposed")
	}
	cmd := companyScrapCommand(loaded.Context.Version, "retain_cost")
	missing := cmd
	missing.Dispositions = nil
	call(7, "POST", endpoint+"/preview", missing, 400)
	call(7, "POST", endpoint+"/preview", cmd, 200)
	var result resolutionpg.Result
	if err := json.Unmarshal(call(7, "POST", endpoint+"/submit", cmd, 200), &result); err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		body := resolutionpg.StepCommand{Choice: step.Choice, Note: "明确原批次及成本", Reference: fmt.Sprint("APPROVE-", step.ID), IdempotencyKey: fmt.Sprint("step-", step.ID)}
		path := fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, step.ID)
		call(7, "POST", path, body, 403)
		employee := int64(9)
		if step.Permission == "finance.resolve" {
			employee = 8
			call(9, "POST", path, body, 403)
		} else {
			call(8, "POST", path, body, 403)
		}
		call(employee, "POST", path, body, 200)
		call(employee, "POST", path, body, 200)
	}
	if err := json.Unmarshal(call(7, "GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.History) != 1 || len(loaded.History[0].Scraps) != 1 || loaded.History[0].Scraps[0].Unit != "袋" || loaded.History[0].Scraps[0].Quantity != 3 || loaded.History[0].Scraps[0].CostCents != 3000 || loaded.History[0].Scraps[0].BatchCode != "ALLOC-COMPANY-BATCH" {
		t.Fatalf("missing posted loss evidence: %+v", loaded.History)
	}
	if loaded.Context.Status != "closed" {
		t.Fatal("order not closed")
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_company_scraps", "status='posted'", 1)
	assertMiniDirectShipCount(t, pool, schema, "stock_ledger_entries", "source_doc_type='order_resolution_company_scrap'", 1)
}
