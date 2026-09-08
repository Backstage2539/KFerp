package costing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	appcosting "orderapp/internal/application/costing"
)

type batchAPIService struct {
	fakeService
	calls int
	last  appcosting.BeanListBatchCommand
}

func (s *batchAPIService) PublishBeanListBatch(_ context.Context, cmd appcosting.BeanListBatchCommand) (*appcosting.BeanListBatchResult, error) {
	s.calls++
	s.last = cmd
	return &appcosting.BeanListBatchResult{ReleaseID: "batch", Version: "V3.0.6", Tables: []appcosting.BeanListPublication{{ID: 21, Version: "V3.0.6"}, {ID: 22, Version: "V3.0.6"}, {ID: 23, Version: "V3.0.6"}}}, nil
}
func (s *batchAPIService) SaveBeanListDraftBatch(ctx context.Context, cmd appcosting.BeanListBatchCommand) (*appcosting.BeanListBatchResult, error) {
	return s.PublishBeanListBatch(ctx, cmd)
}
func (s *batchAPIService) GenerateBeanListPublicationPDF(_ context.Context, cmd appcosting.BeanListPublicationPDFCommand, _ func(appcosting.BeanListPublication) ([]byte, error)) (appcosting.BeanListPublicationPDFFile, error) {
	if cmd.PublicationID == 22 {
		return appcosting.BeanListPublicationPDFFile{}, fmt.Errorf("renderer temporarily unavailable")
	}
	return appcosting.BeanListPublicationPDFFile{}, nil
}

func TestNamedPriceTableBatchAPIAuthScopeAndPartialPDF(t *testing.T) {
	for _, path := range []string{"publication-batches", "draft-batches"} {
		t.Run(path, func(t *testing.T) {
			svc := &batchAPIService{}
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error {
					c.Set("basic_auth_admin", true)
					c.Set("employee_id", int64(7))
					return next(c)
				}
			})
			RegisterRoutes(e, Dependencies{Costing: svc})
			req := httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/"+path, strings.NewReader(`{"list_type":"commercial","version":"V3.0.6","scope":"customer","customer_id":42,"owner_type":"official","owner_key":"evil","default_table_key":"a","tables":[{"key":"a","name":"227g"},{"key":"b","name":"454g"},{"key":"c","name":"1kg"}]}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != 200 || svc.calls != 1 {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, svc.calls, rec.Body.String())
			}
			if svc.last.OwnerType != "customer" || svc.last.OwnerKey != "42" || len(svc.last.Tables) != 3 {
				t.Fatalf("scope=%+v", svc.last)
			}
			if path == "publication-batches" && (!strings.Contains(rec.Body.String(), `"pdf_errors"`) || !strings.Contains(rec.Body.String(), `"22"`)) {
				t.Fatalf("must return committed publication and retriable PDF error: %s", rec.Body.String())
			}
		})
	}
	svc := &batchAPIService{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: svc})
	req := httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publication-batches", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == 200 || svc.calls != 0 {
		t.Fatalf("unauthorized publish status=%d calls=%d", rec.Code, svc.calls)
	}
}
