package customerportal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type fakeEmployeeShareSettingsStore struct {
	value    string
	hasValue bool
	setActor string
	setKey   string
	setValue string
	setCalls int
	getCalls int
	getErr   error
	setErr   error
}

func (f *fakeEmployeeShareSettingsStore) Get(context.Context, string) (string, bool, error) {
	f.getCalls++
	return f.value, f.hasValue, f.getErr
}

func (f *fakeEmployeeShareSettingsStore) Set(_ context.Context, actor, key, value string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.setActor = actor
	f.setKey = key
	f.setValue = value
	f.setCalls++
	f.value = value
	f.hasValue = true
	return nil
}

func employeeShareSettingsContext(role string, permissions ...string) customerportalapp.CurrentContext {
	return customerportalapp.CurrentContext{
		AccountType:  "employee",
		EmployeeID:   17,
		EmployeeName: "管理员甲",
		Roles:        []string{role},
		Permissions:  permissions,
	}
}

func TestMiniEmployeeShareSettingsDefaultsToExistingEntranceBehavior(t *testing.T) {
	e := echo.New()
	store := &fakeEmployeeShareSettingsStore{}
	portal := fakeService{me: employeeShareSettingsContext("sales", "orders.read", "orders.write")}
	registerMiniEmployeeShareSettingsAPI(e, portal, store)

	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/share-settings", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer employee-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"image_need_show_entrance":true`) || !strings.Contains(rec.Body.String(), `"can_manage":false`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestMiniEmployeeShareSettingsAdminCanSaveAndSalesCannot(t *testing.T) {
	adminStore := &fakeEmployeeShareSettingsStore{
		value:    "true",
		hasValue: true,
		getErr:   errors.New("post-commit read must not run"),
	}
	adminAPI := echo.New()
	admin := fakeService{me: employeeShareSettingsContext("admin", "orders.read", "orders.write", "settings.write")}
	registerMiniEmployeeShareSettingsAPI(adminAPI, admin, adminStore)

	req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/share-settings", strings.NewReader(`{"image_need_show_entrance":false}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer admin-token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	adminAPI.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("admin status=%d body=%s", rec.Code, rec.Body.String())
	}
	if adminStore.setCalls != 1 || adminStore.setKey != miniappShareImageNeedShowEntranceKey || adminStore.setValue != "false" {
		t.Fatalf("admin store=%+v", adminStore)
	}
	if adminStore.setActor != "mini-employee:17:管理员甲" {
		t.Fatalf("actor=%q", adminStore.setActor)
	}
	if adminStore.getCalls != 0 {
		t.Fatalf("successful PUT must not perform a fallible read after commit; get calls=%d", adminStore.getCalls)
	}
	if !strings.Contains(rec.Body.String(), `"image_need_show_entrance":false`) || !strings.Contains(rec.Body.String(), `"can_manage":true`) {
		t.Fatalf("body=%s", rec.Body.String())
	}

	salesStore := &fakeEmployeeShareSettingsStore{value: "true", hasValue: true}
	salesAPI := echo.New()
	sales := fakeService{me: employeeShareSettingsContext("sales", "orders.read", "orders.write")}
	registerMiniEmployeeShareSettingsAPI(salesAPI, sales, salesStore)

	salesReq := httptest.NewRequest(http.MethodPut, "/api/mini/employee/share-settings", strings.NewReader(`{"image_need_show_entrance":false}`))
	salesReq.Header.Set(echo.HeaderAuthorization, "Bearer sales-token")
	salesReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	salesRec := httptest.NewRecorder()
	salesAPI.ServeHTTP(salesRec, salesReq)

	if salesRec.Code != http.StatusForbidden {
		t.Fatalf("sales status=%d body=%s", salesRec.Code, salesRec.Body.String())
	}
	if salesStore.setCalls != 0 {
		t.Fatalf("sales write reached store: %+v", salesStore)
	}
}

func TestMiniEmployeeShareSettingsTreatsInvalidStoredValueAsReadFailure(t *testing.T) {
	e := echo.New()
	store := &fakeEmployeeShareSettingsStore{value: "not-a-boolean", hasValue: true}
	portal := fakeService{me: employeeShareSettingsContext("sales", "orders.read")}
	registerMiniEmployeeShareSettingsAPI(e, portal, store)

	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/share-settings", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer employee-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniEmployeeShareSettingsWriteRequiresAdminRoleAndSettingsPermission(t *testing.T) {
	tests := []struct {
		name        string
		role        string
		permissions []string
	}{
		{name: "sales granted settings permission", role: "sales", permissions: []string{"settings.write"}},
		{name: "admin missing settings permission", role: "admin", permissions: []string{"orders.read"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			store := &fakeEmployeeShareSettingsStore{}
			portal := fakeService{me: employeeShareSettingsContext(tc.role, tc.permissions...)}
			registerMiniEmployeeShareSettingsAPI(e, portal, store)

			req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/share-settings", strings.NewReader(`{"image_need_show_entrance":false}`))
			req.Header.Set(echo.HeaderAuthorization, "Bearer employee-token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if store.setCalls != 0 {
				t.Fatalf("forbidden request reached store: %+v", store)
			}
		})
	}
}

func TestMiniEmployeeShareSettingsStoreFailuresDoNotReportSuccess(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		e := echo.New()
		store := &fakeEmployeeShareSettingsStore{getErr: errors.New("read failed")}
		portal := fakeService{me: employeeShareSettingsContext("sales", "orders.read")}
		registerMiniEmployeeShareSettingsAPI(e, portal, store)

		req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/share-settings", nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer employee-token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("write", func(t *testing.T) {
		e := echo.New()
		store := &fakeEmployeeShareSettingsStore{setErr: errors.New("write failed")}
		portal := fakeService{me: employeeShareSettingsContext("admin", "settings.write")}
		registerMiniEmployeeShareSettingsAPI(e, portal, store)

		req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/share-settings", strings.NewReader(`{"image_need_show_entrance":false}`))
		req.Header.Set(echo.HeaderAuthorization, "Bearer employee-token")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if store.setCalls != 0 {
			t.Fatalf("failed write must not be counted as committed: %+v", store)
		}
	})
}

func TestMiniEmployeeShareSettingsRejectsCustomerMissingTokenAndMissingValue(t *testing.T) {
	tests := []struct {
		name       string
		portal     fakeService
		token      string
		body       string
		wantStatus int
	}{
		{
			name: "customer",
			portal: fakeService{me: customerportalapp.CurrentContext{
				AccountType: "customer",
			}},
			token:      "customer-token",
			body:       `{"image_need_show_entrance":false}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing token",
			portal:     fakeService{me: employeeShareSettingsContext("admin", "settings.write")},
			body:       `{"image_need_show_entrance":false}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "expired token",
			portal:     fakeService{err: customerportalapp.ErrMiniSessionNotFound},
			token:      "expired-token",
			body:       `{"image_need_show_entrance":false}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing value",
			portal:     fakeService{me: employeeShareSettingsContext("admin", "orders.read", "settings.write")},
			token:      "admin-token",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			store := &fakeEmployeeShareSettingsStore{}
			registerMiniEmployeeShareSettingsAPI(e, tc.portal, store)
			req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/share-settings", strings.NewReader(tc.body))
			if tc.token != "" {
				req.Header.Set(echo.HeaderAuthorization, "Bearer "+tc.token)
			}
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if store.setCalls != 0 {
				t.Fatalf("rejected request reached store: %+v", store)
			}
		})
	}
}
