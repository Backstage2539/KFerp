package samples

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	portal "orderapp/internal/application/customerportal"
	app "orderapp/internal/application/samples"
	"strings"
	"testing"
)

type fakePortal struct{}

func (fakePortal) Me(_ context.Context, token string) (portal.CurrentContext, error) {
	switch token {
	case "visitor":
		return portal.CurrentContext{MiniUserID: 7, RegistrationComplete: true}, nil
	case "sales":
		return portal.CurrentContext{EmployeeID: 8, AccountType: "employee", Roles: []string{"sales"}}, nil
	case "warehouse":
		return portal.CurrentContext{EmployeeID: 8, AccountType: "employee", Roles: []string{"warehouse"}}, nil
	}
	return portal.CurrentContext{}, portal.ErrMiniSessionNotFound
}

type fakeRepo struct {
	app.Repository
	called bool
}

func (f *fakeRepo) Submit(_ context.Context, p app.Principal, c app.SubmitCommand) (app.Request, error) {
	f.called = true
	if p.MiniUserID != 7 {
		panic("wrong identity")
	}
	return app.Request{ID: 12, Status: "pending_assignment"}, nil
}
func TestSampleAPIIdentityBoundary(t *testing.T) {
	r := &fakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Service: app.NewService(r), Portal: fakePortal{}})
	body := `{"request_id":"request-1","entry_key":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","publication_id":3,"product_keys":["3:0:0"],"receiver":{"name":"甲","phone":"13800000000","address":"上海测试路"}}`
	for _, tc := range []struct {
		path, token string
		status      int
	}{{"/api/mini/sample-requests", "", 401}, {"/api/mini/sample-requests", "warehouse", 403}, {"/api/mini/sample-requests", "visitor", 201}, {"/api/mini/employee/sample-requests/12/assign", "sales", 403}} {
		req := httptest.NewRequest("POST", tc.path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tc.token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.path, tc.token, rec.Code, rec.Body.String())
		}
	}
	if !r.called {
		t.Fatal("request not saved")
	}
}
