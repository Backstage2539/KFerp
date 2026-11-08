package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	resolutionhttp "orderapp/internal/interfaces/http/orderresolution"
	"orderapp/internal/interfaces/http/support"
	"testing"
)

func TestOrderResolutionProductionStopHTTPRefreshRequiresNewConfirmations(t *testing.T) {
	pool, schema := productionStopFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("operator_employee", "stop-api")
			return next(c)
		}
	})
	e.Use(support.AuthorizationMiddleware(nil))
	resolutionhttp.RegisterRoutes(e, r, nil)
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: %d wanted %d %s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	endpoint := "/api/orders/78400/resolution"
	var loaded struct {
		Context domain.Context `json:"context"`
	}
	if err := json.Unmarshal(call("GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	cmd := productionStopCommand(loaded.Context.Version)
	missing := cmd
	missing.Dispositions = nil
	call("POST", endpoint+"/preview", missing, 400)
	call("POST", endpoint+"/preview", cmd, 200)
	var result resolutionpg.Result
	if err := json.Unmarshal(call("POST", endpoint+"/submit", cmd, 200), &result); err != nil {
		t.Fatal(err)
	}
	first := result.Steps[0]
	call("POST", fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, first.ID), resolutionpg.StepCommand{Choice: "stop", Note: "初次核对", IdempotencyKey: "initial"}, 200)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_balances SET remaining_qty=1,remaining_cost=12.5 WHERE id=78400`, schema)); err != nil {
		t.Fatal(err)
	}
	call("POST", fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, result.Steps[1].ID), resolutionpg.StepCommand{Choice: "stop", Note: "过期核对", IdempotencyKey: "stale"}, 409)
	for i := 0; i < 2; i++ {
		if err := json.Unmarshal(call("GET", endpoint, nil, 200), &loaded); err != nil {
			t.Fatal(err)
		}
		cmd.Version = loaded.Context.Version
		cmd.IdempotencyKey = fmt.Sprint("refresh-", i)
		call("POST", endpoint+"/preview", cmd, 200)
		if err := json.Unmarshal(call("POST", fmt.Sprintf("%s/%d/refresh", endpoint, result.ID), cmd, 200), &result); err != nil {
			t.Fatal(err)
		}
		pending := 0
		for _, step := range result.Steps {
			if step.Status == "pending" {
				pending++
			}
			if step.ID == first.ID && step.Status != "superseded" {
				t.Fatalf("obsolete confirmation retained after facts changed: %+v", step)
			}
		}
		if pending != 2 || result.Applied {
			t.Fatalf("both roles must review changed facts: %+v", result)
		}
	}
	for _, step := range result.Steps {
		if step.Status != "pending" {
			continue
		}
		body := resolutionpg.StepCommand{Choice: "stop", Note: "重新核对剩余一公斤", IdempotencyKey: fmt.Sprint("fresh-", step.ID)}
		call("POST", fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, step.ID), body, 200)
		call("POST", fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, step.ID), body, 200)
	}
	if err := json.Unmarshal(call("GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Context.Status != "closed" || loaded.Context.Dependencies[0].Status != "stopped" {
		t.Fatalf("missing independent order/production states: %+v", loaded.Context)
	}
}
