package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	resolutionhttp "orderapp/internal/interfaces/http/orderresolution"
	"orderapp/internal/interfaces/http/support"
)

func TestOrderResolutionOwnershipHTTPRefreshAfterWarehouseReceipt(t *testing.T) {
	pool, schema := ownershipReturnFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("operator_employee", "return-api")
			return next(c)
		}
	})
	e.Use(support.AuthorizationMiddleware(nil))
	resolutionhttp.RegisterRoutes(e, r, nil)
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: %d expected %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	endpoint := "/api/orders/78510/resolution"
	var loaded struct {
		Context domain.Context        `json:"context"`
		History []resolutionpg.Result `json:"history"`
	}
	if err := json.Unmarshal(call("GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	cmd := returnCommand(loaded.Context.Version)
	source := loaded.Context.ReturnSources[0]
	cmd.Scope = "selected"
	cmd.Lines = []domain.Selection{{ItemID: 78510, Quantity: 3}}
	cmd.ReturnSelections = []domain.ReturnSelection{{SourceKind: "ownership", SourceID: source.ID, BatchID: source.BatchID, Quantity: 3, Ownership: "company", Warehouse: "finished_goods", Quality: "quarantine"}}
	cmd.Dispositions = []domain.Disposition{{Kind: "customer_stock", ID: source.EntitlementID, Choice: "ownership_return"}}
	missingOwnership := cmd
	missingOwnership.Dispositions = nil
	call("POST", endpoint+"/preview", missingOwnership, 400)

	missing := cmd
	missing.ReturnSettlement = ""
	call("POST", endpoint+"/preview", missing, 400)
	call("POST", endpoint+"/preview", cmd, 200)
	var result resolutionpg.Result
	if err := json.Unmarshal(call("POST", endpoint+"/submit", cmd, 200), &result); err != nil {
		t.Fatal(err)
	}
	var warehouse, finance int64
	for _, s := range result.Steps {
		if s.Kind == "return_stock" {
			warehouse = s.ID
		}
		if s.Kind == "return_finance" {
			finance = s.ID
		}
	}
	stockURL := fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, warehouse)
	call("POST", stockURL, resolutionpg.StepCommand{Choice: "receive", Note: "missing receipt", IdempotencyKey: "bad-receipt"}, 400)
	confirm := resolutionpg.StepCommand{Choice: "receive", Note: "核实原批次与实物", Reference: "收货验收-API", PostingDate: "2026-11-08", IdempotencyKey: "api-return-stock"}
	call("POST", stockURL, confirm, 200)
	call("POST", stockURL, confirm, 200)
	if err := json.Unmarshal(call("GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Context.Lines[0].Returned != 3 || loaded.Context.CreditCents != 0 {
		t.Fatalf("stock and money mixed: %+v", loaded.Context)
	}
	for _, d := range loaded.Context.Dependencies {
		if d.Kind == "customer_stock" && d.ID == source.EntitlementID && (d.Quantity != 0 || d.Status != "released") {
			t.Fatalf("fully returned ownership still requires an active stock disposition: %+v", d)
		}
	}
	cmd.Version = loaded.Context.Version
	cmd.IdempotencyKey = "refresh-posted-return"
	call("POST", endpoint+"/preview", cmd, 200)
	call("POST", fmt.Sprintf("%s/%d/refresh", endpoint, result.ID), cmd, 200)
	altered := cmd
	altered.ReturnSelections = append([]domain.ReturnSelection(nil), cmd.ReturnSelections...)
	altered.ReturnSelections[0].Quality = "restock"
	call("POST", endpoint+"/preview", altered, 409)
	call("POST", fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, finance), resolutionpg.StepCommand{Choice: "keep_receivable", Note: "保留商品应收", IdempotencyKey: "api-return-finance"}, 200)
	current, err := r.Context(ctx, 78510)
	if err != nil {
		t.Fatal(err)
	}
	if current.Lines[0].Returned != 3 || current.CreditCents != 0 {
		t.Fatalf("duplicate quantity or unintended credit: %+v", current)
	}
	var receipts, logs int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.order_resolution_stock_returns),(SELECT count(*) FROM %[1]s.audit_logs WHERE action='resolution_return_stock' AND actor='return-api')`, schema)).Scan(&receipts, &logs); err != nil || receipts != 1 || logs != 1 {
		t.Fatalf("receipts=%d logs=%d error=%v", receipts, logs, err)
	}
}
