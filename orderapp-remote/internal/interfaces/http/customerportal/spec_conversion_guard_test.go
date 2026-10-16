package customerportal

import (
	"github.com/labstack/echo/v4"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCustomerConversionGuardRejectsUnknownEmployeeInstructions(t *testing.T) {
	for _, body := range []string{`{"price_selection_mode":"converted_tier"}`, `{"items":[{"spec_conversion":{"group_id":1}}]}`, `{"items":[{"price_selection_mode":"converted_tier"}]}`} {
		c := echo.New().NewContext(httptest.NewRequest("POST", "/", strings.NewReader(body)), httptest.NewRecorder())
		if err := rejectCustomerSpecConversion(c); err == nil {
			t.Fatal("accepted employee conversion", body)
		}
	}
	body := `{"items":[{"product_id":7,"qty":2}]}`
	c := echo.New().NewContext(httptest.NewRequest("POST", "/", strings.NewReader(body)), httptest.NewRecorder())
	if err := rejectCustomerSpecConversion(c); err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(c.Request().Body)
	if string(raw) != body {
		t.Fatal("body changed")
	}
}
