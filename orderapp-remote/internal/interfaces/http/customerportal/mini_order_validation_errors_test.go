package customerportal

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestMiniOrderItemValidationDoesNotRequestShippingAddress(t *testing.T) {
	for _, message := range []string{"product required", "spec required", "bom_spec_id required", "bom_spec_id required for product output", "canonical BOM spec must not use spec_g", "bom_variant_id mismatch for BOM spec", "quantity required", "items required"} {
		t.Run(message, func(t *testing.T) {
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodPost, "/api/mini/product-orders/preview", nil), rec)
			if err := miniCustomerFulfillmentError(c, fmt.Errorf("%s", message)); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "商品规格或数量不完整，请重新选择商品与规格") {
				t.Fatalf("item validation misleading status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	for _, message := range []string{"recipient_name required", "recipient_phone required", "detail_address required", "receiver_name required", "receiver_phone required", "receiver_address required"} {
		t.Run(message, func(t *testing.T) {
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodPost, "/api/mini/product-orders/preview", nil), rec)
			if err := miniCustomerFulfillmentError(c, fmt.Errorf("%s", message)); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "发货信息不完整，请检查收件信息和商品数量") {
				t.Fatalf("recipient validation changed status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMiniOrderCapabilityRevocationIsForbidden(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodPost, "/api/mini/product-orders", nil), rec)
	if err := miniCustomerFulfillmentError(c, fmt.Errorf("customer capability product_order unavailable")); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "当前客户尚未开通该服务") {
		t.Fatalf("capability error misleading status=%d body=%s", rec.Code, rec.Body.String())
	}
}
