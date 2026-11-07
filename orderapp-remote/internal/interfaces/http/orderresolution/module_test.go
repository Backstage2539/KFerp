package orderresolution

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	domain "orderapp/internal/domain/orderresolution"
	repo "orderapp/internal/infrastructure/postgres/orderresolution"
	"orderapp/internal/interfaces/http/support"
	"strings"
	"testing"
)

type fakeBackend struct {
	Backend
	id    int64
	actor string
	cmd   domain.Command
}

func (f *fakeBackend) Context(context.Context, int64) (domain.Context, error) {
	return domain.Context{OrderID: 7, Version: "v1", Lines: []domain.Line{{ID: 71, Quantity: 2, AmountCents: 10000}}}, nil
}
func (f *fakeBackend) Submit(_ context.Context, id int64, c domain.Command, actor string) (repo.Result, error) {
	f.id = id
	f.cmd = c
	f.actor = actor
	return repo.Result{ID: 10, Status: "completed"}, nil
}
func TestResolutionAPIRequiresSessionAndUsesPathAndActor(t *testing.T) {
	for _, admin := range []bool{false, true} {
		f := &fakeBackend{}
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				if admin {
					c.Set("basic_auth_admin", true)
					c.Set("operator_employee", "staff")
				}
				return next(c)
			}
		})
		RegisterRoutes(e, f, nil)
		req := httptest.NewRequest("POST", "/api/orders/7/resolution/submit", strings.NewReader(`{"order_id":999,"actor":"spoof","action":"close","version":"v1","idempotency_key":"k"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if !admin {
			if rec.Code != 401 || f.id != 0 {
				t.Fatalf("unauthenticated %d %+v", rec.Code, f)
			}
		} else if rec.Code != 200 || f.id != 7 || f.actor != "staff" {
			t.Fatalf("status=%d id=%d actor=%s body=%s", rec.Code, f.id, f.actor, rec.Body.String())
		}
	}
}
func TestPreviewRejectsMissingChoicesWithoutWrite(t *testing.T) {
	f := &fakeBackend{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("basic_auth_admin", true); return next(c) }
	})
	RegisterRoutes(e, f, nil)
	req := httptest.NewRequest("POST", "/api/orders/7/resolution/preview", strings.NewReader(`{"action":"close"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 400 || f.id != 0 {
		t.Fatalf("status=%d wrote=%d", rec.Code, f.id)
	}
}

type fakeAuth struct {
	support.AuthzService
	permissions []string
}

func (f fakeAuth) ActorByEmployeeID(context.Context, int64) (authzapp.Actor, error) {
	return authzapp.Actor{EmployeeID: 7, Permissions: f.permissions}, nil
}
func (f *fakeBackend) History(context.Context, int64) ([]repo.Result, error) {
	return []repo.Result{{ID: 10, Steps: []repo.Step{{ID: 11, Task: domain.Task{Kind: "funds", Permission: "finance.resolve"}}}}}, nil
}
func (f *fakeBackend) CompleteStep(_ context.Context, order, request, step int64, cmd repo.StepCommand, actor string) (repo.Result, error) {
	f.id = order
	return repo.Result{ID: request, Status: "completed"}, nil
}
func TestResolutionStepEnforcesRequiredRole(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		f := &fakeBackend{}
		permissions := []string{"orders.resolve"}
		if allowed {
			permissions = []string{"finance.resolve"}
		}
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { c.Set("employee_id", int64(7)); return next(c) }
		})
		e.Use(support.AuthorizationMiddleware(fakeAuth{permissions: permissions}))
		RegisterRoutes(e, f, fakeAuth{permissions: permissions})
		req := httptest.NewRequest("POST", "/api/orders/7/resolution/10/steps/11", strings.NewReader(`{"choice":"refund"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if allowed && rec.Code != 200 || !allowed && (rec.Code != 403 || f.id != 0) {
			t.Fatalf("allowed=%v status=%d write=%d body=%s", allowed, rec.Code, f.id, rec.Body.String())
		}
	}
}

func (f *fakeBackend) Policy(context.Context) (repo.PolicyRecord, error) {
	return repo.PolicyRecord{Revision: 1}, nil
}
func (f *fakeBackend) SavePolicy(_ context.Context, revision int64, p domain.Policy, actor string) (repo.PolicyRecord, error) {
	f.id = revision
	f.actor = actor
	return repo.PolicyRecord{Revision: revision + 1, Policy: p}, nil
}
func TestResolutionPolicyAPIRequiresSettingsPermission(t *testing.T) {
	for _, allow := range []bool{false, true} {
		f := &fakeBackend{}
		permissions := []string{"orders.resolve"}
		if allow {
			permissions = []string{"settings.write"}
		}
		auth := fakeAuth{permissions: permissions}
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("employee_id", int64(7))
				c.Set("operator_employee", "reviewer")
				return next(c)
			}
		})
		e.Use(support.AuthorizationMiddleware(auth))
		RegisterRoutes(e, f, auth)
		req := httptest.NewRequest("POST", "/api/settings/order-resolution", strings.NewReader(`{"revision":1,"policy":{"disabled":{"action":["cancel"]}}}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if allow {
			if rec.Code != 200 || f.id != 1 || f.actor != "reviewer" {
				t.Fatalf("%d %s %+v", rec.Code, rec.Body.String(), f)
			}
		} else if rec.Code != 403 || f.id != 0 {
			t.Fatalf("unauthorized: %d %+v", rec.Code, f)
		}
	}
}

func (f *fakeBackend) Refresh(_ context.Context, order, request int64, cmd domain.Command, actor string) (repo.Result, error) {
	f.id = order
	f.cmd = cmd
	f.actor = actor
	return repo.Result{ID: request, Status: "pending"}, nil
}
func TestResolutionRefreshUsesSalesPermissionAndBoundIDs(t *testing.T) {
	for _, allow := range []bool{false, true} {
		f := &fakeBackend{}
		permissions := []string{"finance.resolve"}
		if allow {
			permissions = []string{"orders.resolve"}
		}
		auth := fakeAuth{permissions: permissions}
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { c.Set("employee_id", int64(7)); return next(c) }
		})
		e.Use(support.AuthorizationMiddleware(auth))
		RegisterRoutes(e, f, auth)
		req := httptest.NewRequest("POST", "/api/orders/7/resolution/10/refresh", strings.NewReader(`{"version":"v2","idempotency_key":"refresh"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if allow {
			if rec.Code != 200 || f.id != 7 || f.cmd.Version != "v2" {
				t.Fatalf("%d %s %+v", rec.Code, rec.Body.String(), f)
			}
		} else if rec.Code != 403 || f.id != 0 {
			t.Fatalf("unauthorized: %d %+v", rec.Code, f)
		}
	}
}

func (f *fakeBackend) UseBalance(_ context.Context, order int64, cmd repo.BalanceCommand, actor string) error {
	f.id = order
	f.actor = actor
	return nil
}
func TestResolutionBalanceAPIOnlyFinanceMaySpend(t *testing.T) {
	for _, allow := range []bool{false, true} {
		f := &fakeBackend{}
		permissions := []string{"orders.resolve"}
		if allow {
			permissions = []string{"finance.resolve"}
		}
		auth := fakeAuth{permissions: permissions}
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { c.Set("employee_id", int64(7)); return next(c) }
		})
		e.Use(support.AuthorizationMiddleware(auth))
		RegisterRoutes(e, f, auth)
		req := httptest.NewRequest("POST", "/api/orders/7/resolution/balance", strings.NewReader(`{"source_entry_id":1,"choice":"offset","amount_cents":100}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if allow {
			if rec.Code != 200 || f.id != 7 {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
		} else if rec.Code != 403 || f.id != 0 {
			t.Fatalf("unauthorized balance use: %d %+v", rec.Code, f)
		}
	}
}
