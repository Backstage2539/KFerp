package customerportal

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

func TestMiniProcessingPreviewAndRequestReadAPIsUseMultiTargetContract(t *testing.T) {
	var cmd customerportalapp.CreateProcessingRequestCommand
	row := customerportalapp.ProcessingRequest{
		ID: 9, RequestNo: "PJ-0000000009", Status: "awaiting_schedule",
		Items: []customerportalapp.ProcessingRequestItem{{ID: 10, ProductID: 101, ProductName: "目标一", SpecG: 227, Qty: 3}},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		processing:     row,
		processingRows: []customerportalapp.ProcessingRequest{row},
		processingPreview: customerportalapp.ProcessingRequestPreview{
			CanSubmit: true,
			Items:     []customerportalapp.ProcessingRequestItem{{ProductID: 101, SpecG: 227, Qty: 3, BomVersionID: 7}},
		},
		processingCmd: &cmd,
	}})

	previewReq := httptest.NewRequest(http.MethodPost, "/api/mini/processing-requests/preview", strings.NewReader(`{"items":[{"product_id":101,"spec_g":227,"qty":3},{"product_id":102,"spec_g":454,"qty":2}]}`))
	previewReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	previewReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	previewRec := httptest.NewRecorder()
	e.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK || !strings.Contains(previewRec.Body.String(), `"can_submit":true`) || !strings.Contains(previewRec.Body.String(), `"bom_version_id":7`) {
		t.Fatalf("preview status=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	if len(cmd.Items) != 2 || cmd.InputMaterialID != 0 || cmd.InputQtyG != 0 {
		t.Fatalf("preview command=%+v, want only target items", cmd)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/mini/processing-requests", nil)
	listReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), `"rows"`) || !strings.Contains(listRec.Body.String(), `"PJ-0000000009"`) {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/mini/processing-requests/9", nil)
	detailReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	detailRec := httptest.NewRecorder()
	e.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK || !strings.Contains(detailRec.Body.String(), `"request"`) || !strings.Contains(detailRec.Body.String(), `"目标一"`) {
		t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}
}

func TestMiniProcessingRequestDetailReturnsNotFoundForAnotherCustomerOrMissingRequest(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: fmt.Errorf("processing request not found")}})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/processing-requests/99", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniProcessingSubmitReturnsConflictWithLatestPreviewWhenInventoryChanged(t *testing.T) {
	preview := customerportalapp.ProcessingRequestPreview{
		CanSubmit: false,
		Materials: []customerportalapp.ProcessingMaterialPreview{{MaterialID: 7, RequiredG: 1000, AvailableG: 500, ShortageG: 500}},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: &customerportalapp.ProcessingMaterialsUnavailableError{Preview: preview}}})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/processing-requests", strings.NewReader(`{"items":[{"product_id":101,"spec_g":227,"qty":3}]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"shortage_g":500`) || !strings.Contains(rec.Body.String(), `"processing materials unavailable"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
