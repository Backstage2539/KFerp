package costing

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/costing"
	"strings"
	"testing"
)

type orderabilityAPIService struct {
	fakeService
	last app.PublishBeanListCommand
	fail bool
}

func (s *orderabilityAPIService) ValidateBeanListOrderability(_ context.Context, cmd app.PublishBeanListCommand) error {
	s.last = cmd
	if s.fail {
		return fmt.Errorf("商品「测试生豆」未配置可用于录单的默认已发布 BOM 规格")
	}
	return nil
}

func TestPublicationOrderabilityPreflightRejectsBeforeGeneration(t *testing.T) {
	for _, fail := range []bool{true, false} {
		svc := &orderabilityAPIService{fail: fail}
		e := echo.New()
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { c.Set("basic_auth_admin", true); return next(c) }
		})
		RegisterRoutes(e, Dependencies{Costing: svc})
		req := httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/validate-orderability", strings.NewReader(`{"scope":"customer","customer_id":42,"owner_type":"official","owner_key":"forged","list_type":"green","content":{"price_rows":[{"product_id":97}]}}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		want := 200
		if fail {
			want = 400
		}
		if rec.Code != want || svc.last.OwnerType != "customer" || svc.last.OwnerKey != "42" {
			t.Fatalf("status=%d scope=%s/%s body=%s", rec.Code, svc.last.OwnerType, svc.last.OwnerKey, rec.Body.String())
		}
		if fail && !strings.Contains(rec.Body.String(), "测试生豆") {
			t.Fatal(rec.Body.String())
		}
	}
}
