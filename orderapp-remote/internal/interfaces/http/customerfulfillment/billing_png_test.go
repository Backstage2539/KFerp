package customerfulfillment

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
)

func TestIssuedBillingPNGDownload(t *testing.T) {
	for _, name := range []string{"pdftoppm", "pdfinfo"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s is installed in the release image", name)
		}
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: &fakeCustomerFulfillmentService{}})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/149/billing/bills/88.png", nil))
	if rec.Code != http.StatusOK || rec.Header().Get(echo.HeaderContentType) != "image/png" || !bytes.HasPrefix(rec.Body.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("PNG download status=%d type=%q", rec.Code, rec.Header().Get(echo.HeaderContentType))
	}
}

func TestIssuedBillingPNGRejectsUnavailableSeal(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: &fakeCustomerFulfillmentService{billingPDFSettings: app.BillingPDFSettings{SealAssetID: 123}}})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/149/billing/bills/88.png", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("missing seal status=%d, expected error", rec.Code)
	}
}
