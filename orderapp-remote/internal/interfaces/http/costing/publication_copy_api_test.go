package costing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	appcosting "orderapp/internal/application/costing"
)

type copyAPIService struct {
	fakeService
	previewCalls int
	copyCalls    int
	last         appcosting.BeanListPublicationCopyCommand
}

func (s *copyAPIService) PreviewBeanListPublicationCopy(_ context.Context, cmd appcosting.BeanListPublicationCopyCommand) (appcosting.BeanListPublicationCopyStats, error) {
	s.previewCalls++
	s.last = cmd
	return appcosting.BeanListPublicationCopyStats{SourcePublicationID: cmd.SourcePublicationID, SourceVersion: "V5.1", CopyProductCount: 1, CopySpecCount: 2, SkipProductCount: 2}, nil
}

func (s *copyAPIService) CopyBeanListPublicationToDraft(_ context.Context, cmd appcosting.BeanListPublicationCopyCommand) (appcosting.BeanListPublicationCopyResult, error) {
	s.copyCalls++
	s.last = cmd
	return appcosting.BeanListPublicationCopyResult{Stats: appcosting.BeanListPublicationCopyStats{SourcePublicationID: cmd.SourcePublicationID, CopyProductCount: 1}}, nil
}

func TestPublicationCopyAPIUsesScopedSourceAndServerOwnedCustomerTarget(t *testing.T) {
	for _, endpoint := range []string{"copy-preview", "copy-to-draft"} {
		t.Run(endpoint, func(t *testing.T) {
			svc := &copyAPIService{}
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error {
					c.Set("basic_auth_admin", true)
					c.Set("employee_id", int64(7))
					return next(c)
				}
			})
			RegisterRoutes(e, Dependencies{Costing: svc})
			requestID := "copy-request-1"
			requestBody := `{"customer_id":42,"target_table_key":"a","copy_request_id":"` + requestID + `","batch":{"list_type":"commercial","version":"V2.0","scope":"official","owner_type":"official","owner_key":"forged","default_table_key":"a","tables":[{"key":"a","name":"客户价格表"}]}}`
			req := httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications/8/"+endpoint+"?list_type=commercial&scope=official", strings.NewReader(requestBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if svc.last.SourcePublicationID != 8 || svc.last.SourceQuery.OwnerType != "official" || svc.last.CustomerID != 42 || svc.last.Batch.OwnerType != "customer" || svc.last.Batch.OwnerKey != "42" || svc.last.Batch.Actor == "" || svc.last.CopyRequestID != requestID {
				t.Fatalf("copy scope was not derived safely: %+v", svc.last)
			}
			if endpoint == "copy-preview" && (!strings.Contains(rec.Body.String(), `"copy_spec_count":2`) || svc.previewCalls != 1 || svc.copyCalls != 0) {
				t.Fatalf("preview result = %s, calls=%d/%d", rec.Body.String(), svc.previewCalls, svc.copyCalls)
			}
			if endpoint == "copy-to-draft" && (svc.copyCalls != 1 || svc.previewCalls != 0) {
				t.Fatalf("copy calls=%d/%d", svc.previewCalls, svc.copyCalls)
			}
		})
	}
}

func TestPublicationCopyAPIRejectsUnauthorizedSourcesAndActors(t *testing.T) {
	for _, test := range []struct {
		name  string
		url   string
		admin bool
		want  int
	}{
		{name: "resale source", url: "/api/costing/bean-list/publications/8/copy-preview?list_type=commercial&scope=official&publication_purpose=customer_resale", admin: true, want: http.StatusBadRequest},
		{name: "missing copy request id", url: "/api/costing/bean-list/publications/8/copy-to-draft?list_type=commercial&scope=official", admin: true, want: http.StatusBadRequest},
		{name: "non-admin actor", url: "/api/costing/bean-list/publications/8/copy-to-draft?list_type=commercial&scope=official", admin: false, want: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := &copyAPIService{}
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error {
					if test.admin {
						c.Set("basic_auth_admin", true)
					}
					return next(c)
				}
			})
			RegisterRoutes(e, Dependencies{Costing: svc})
			req := httptest.NewRequest(http.MethodPost, test.url, strings.NewReader(`{"customer_id":42,"target_table_key":"a"}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != test.want || svc.previewCalls+svc.copyCalls != 0 {
				t.Fatalf("status=%d want=%d calls=%d body=%s", rec.Code, test.want, svc.previewCalls+svc.copyCalls, rec.Body.String())
			}
		})
	}
}
