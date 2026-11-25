package customerportal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customer"
	web "orderapp/internal/interfaces/http/customer"
)

type customerContactWriteProbe struct {
	app.Repository
	app.ManagedRepository
	writes int
}

func (r *customerContactWriteProbe) Upsert(context.Context, string, *int64, app.UpsertCommand) (int64, error) {
	r.writes++
	return 0, errors.New("unexpected write")
}

func (r *customerContactWriteProbe) UpsertManaged(context.Context, app.MaintenancePrincipal, *int64, app.UpsertCommand) (int64, error) {
	r.writes++
	return 0, errors.New("unexpected write")
}

func TestEmployeeCreateCustomerRequiresContactsOnBothAPIs(t *testing.T) {
	repo := &customerContactWriteProbe{}
	svc := app.NewService(repo)
	e := echo.New()
	web.RegisterRoutes(e, web.Dependencies{Customer: svc})
	registerMiniEmployeeAPI(e, employeePortalService(), nil, Dependencies{CustomerMaintenance: svc})
	for _, path := range []string{"/api/customers", "/api/mini/employee/customers"} {
		for field, label := range map[string]string{"contact": "联系人", "phone": "联系电话", "address": "联系地址"} {
			t.Run(path+"/"+field, func(t *testing.T) {
				data := map[string]any{"name": "测试客户", "contact": "张三", "phone": "021-12345678", "address": "测试路一号", "company_phone": "13800000000", "company_address": "测试公司地址", "customer_type": "retail", "default_source_id": 1, "default_order_type_id": 2}
				data[field] = "　 "
				raw, err := json.Marshal(data)
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
				req.Header.Set(echo.HeaderAuthorization, "Bearer token")
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				if rec.Code != 400 || !strings.Contains(rec.Body.String(), label) || repo.writes != 0 {
					t.Fatalf("status=%d body=%s writes=%d", rec.Code, rec.Body.String(), repo.writes)
				}
			})
		}
	}
}
