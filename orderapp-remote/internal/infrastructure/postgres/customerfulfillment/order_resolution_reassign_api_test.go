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

type reassignAPIAuth struct{ support.AuthzService }

func (reassignAPIAuth) ActorByEmployeeID(_ context.Context, id int64) (authzapp.Actor, error) {
	return authzapp.Actor{EmployeeID: id, Permissions: []string{map[int64]string{7: "orders.resolve", 8: "finance.resolve", 9: "production.resolve"}[id]}}, nil
}
func TestOrderResolutionProductionReassignHTTPPermissionsAndRetry(t *testing.T) {
	pool, schema := productionReassignFixture(t)
	r := resolutionpg.NewRepository(pool, schema)
	e := echo.New()
	auth := reassignAPIAuth{}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id, _ := strconv.ParseInt(c.Request().Header.Get("X-Test-Employee"), 10, 64)
			c.Set("employee_id", id)
			c.Set("operator_employee", "reassign-api")
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
			Reassignments []struct {
				ID            int64  `json:"id"`
				WorkOrderID   int64  `json:"work_order_id"`
				SourceOrderNo string `json:"source_order_no"`
				TargetOrderNo string `json:"target_order_no"`
				Quantity      int64  `json:"quantity"`
				Unit          string `json:"unit"`
			} `json:"reassignments"`
		} `json:"history"`
	}
	if err := json.Unmarshal(call(7, "GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	cmd := productionReassignCommand(loaded.Context.Version)
	missing := cmd
	missing.Dispositions = nil
	call(7, "POST", endpoint+"/preview", missing, 400)
	call(7, "POST", endpoint+"/preview", cmd, 200)
	var result resolutionpg.Result
	if err := json.Unmarshal(call(7, "POST", endpoint+"/submit", cmd, 200), &result); err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		body := resolutionpg.StepCommand{Choice: step.Choice, Note: "明确原工单投入及目标需求", Reference: fmt.Sprint("APPROVE-", step.ID), IdempotencyKey: fmt.Sprint("step-", step.ID)}
		path := fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, step.ID)
		call(7, "POST", path, body, 403)
		employee := int64(9)
		call(8, "POST", path, body, 403)
		missingProof := body
		missingProof.Reference = ""
		call(employee, "POST", path, missingProof, 400)
		call(employee, "POST", path, body, 200)
		call(employee, "POST", path, body, 200)
	}
	if err := json.Unmarshal(call(7, "GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.History) != 1 || len(loaded.History[0].Reassignments) != 1 {
		t.Fatalf("missing posted transfer evidence: %+v", loaded.History)
	}
	proof := loaded.History[0].Reassignments[0]
	if proof.WorkOrderID != 78400 || proof.SourceOrderNo != "ALLOC-SOURCE" || proof.TargetOrderNo != "ALLOC-TARGET" || proof.Quantity != 5 || proof.Unit != "袋" {
		t.Fatalf("incorrect transfer proof: %+v", proof)
	}
	if loaded.Context.Status != "closed" {
		t.Fatal("source not closed")
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_production_reassignments", "status='applied'", 1)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "action='resolution_production_reassign'", 1)
}
