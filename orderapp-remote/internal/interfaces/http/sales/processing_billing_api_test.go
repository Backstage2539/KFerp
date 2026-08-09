package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type fakeBillingSalesRepository struct {
	salesapp.Repository
	previewCmd salesapp.PreviewProcessingBillingCommand
	confirmCmd salesapp.ConfirmProcessingBillingCommand
	runsQuery  salesapp.ProcessingBillingRunsQuery
	payCmd     salesapp.PayProcessingBillingCommand
	reverseCmd salesapp.ReverseProcessingBillingCommand
	adjustCmd  salesapp.AdjustProcessingBillingCommand
}

func (r *fakeBillingSalesRepository) ListOutsourceTemplates(context.Context) ([]salesapp.OutsourceTemplate, error) {
	return []salesapp.OutsourceTemplate{{
		ID: 7, Name: "实际费用模板", CurrentVersionID: 13, CurrentVersionNo: 2, CurrentVersionStatus: "published",
		Rules: []salesapp.OutsourceTemplateRule{{ID: 31, VersionID: 13, FeeType: "processing", Name: "加工费", Basis: salesapp.BillingBasisActualOutputKG, UnitPrice: 2}},
	}}, nil
}

func (r *fakeBillingSalesRepository) ListProcessingBillingCustomerOptions(context.Context) ([]salesapp.ProcessingBillingCustomerOption, error) {
	return []salesapp.ProcessingBillingCustomerOption{{CustomerID: 19, CustomerName: "9.9 COFFEE LAB"}}, nil
}

func (r *fakeBillingSalesRepository) ListProcessingBillingCandidates(_ context.Context, customerID int64) ([]salesapp.ProcessingBillingCandidate, error) {
	return []salesapp.ProcessingBillingCandidate{{WorkOrderID: 91, CustomerID: customerID, Status: "completed"}}, nil
}

func (r *fakeBillingSalesRepository) PreviewProcessingBilling(_ context.Context, cmd salesapp.PreviewProcessingBillingCommand) (salesapp.ProcessingBillingPreview, error) {
	r.previewCmd = cmd
	return salesapp.ProcessingBillingPreview{CustomerID: cmd.CustomerID, TemplateVersionID: 13, TotalAmount: 88.5}, nil
}

func (r *fakeBillingSalesRepository) ConfirmProcessingBilling(_ context.Context, cmd salesapp.ConfirmProcessingBillingCommand) (salesapp.ProcessingBillingConfirmation, error) {
	r.confirmCmd = cmd
	return salesapp.ProcessingBillingConfirmation{SettlementBatchID: 41, SettlementNo: "CPB-41", TotalAmount: 88.5}, nil
}

func (r *fakeBillingSalesRepository) ListProcessingBillingRuns(_ context.Context, query salesapp.ProcessingBillingRunsQuery) ([]salesapp.ProcessingBillingRun, error) {
	r.runsQuery = query
	return []salesapp.ProcessingBillingRun{{ID: 41, CustomerID: query.CustomerID, Status: salesapp.ProcessingBillingStatusConfirmed}}, nil
}

func (r *fakeBillingSalesRepository) PayProcessingBilling(_ context.Context, cmd salesapp.PayProcessingBillingCommand) (salesapp.ProcessingBillingLifecycleResult, error) {
	r.payCmd = cmd
	return salesapp.ProcessingBillingLifecycleResult{BillingRunID: cmd.BillingRunID, Status: salesapp.ProcessingBillingStatusPaid}, nil
}

func (r *fakeBillingSalesRepository) ReverseProcessingBilling(_ context.Context, cmd salesapp.ReverseProcessingBillingCommand) (salesapp.ProcessingBillingLifecycleResult, error) {
	r.reverseCmd = cmd
	return salesapp.ProcessingBillingLifecycleResult{BillingRunID: 42, SourceBillingRunID: cmd.BillingRunID, Status: salesapp.ProcessingBillingStatusConfirmed}, nil
}

func (r *fakeBillingSalesRepository) AdjustProcessingBilling(_ context.Context, cmd salesapp.AdjustProcessingBillingCommand) (salesapp.ProcessingBillingLifecycleResult, error) {
	r.adjustCmd = cmd
	return salesapp.ProcessingBillingLifecycleResult{BillingRunID: 43, SourceBillingRunID: cmd.BillingRunID, Status: salesapp.ProcessingBillingStatusConfirmed}, nil
}

func TestProcessingBillingPreviewAndConfirmAPIContracts(t *testing.T) {
	repo := &fakeBillingSalesRepository{}
	svc := salesapp.NewService(repo)
	e := echo.New()
	registerOutsourceSettingsRoutes(e, svc)

	previewBody := []byte(`{"customer_id":19,"template_id":7,"work_order_ids":[91,92]}`)
	previewReq := httptest.NewRequest(http.MethodPost, "/api/finance/customer-processing-billing/preview", bytes.NewReader(previewBody))
	previewReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	previewRec := httptest.NewRecorder()
	e.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	var preview struct {
		OK      bool                              `json:"ok"`
		Preview salesapp.ProcessingBillingPreview `json:"preview"`
	}
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.OK || preview.Preview.TemplateVersionID != 13 || repo.previewCmd.CustomerID != 19 {
		t.Fatalf("preview response=%+v command=%+v", preview, repo.previewCmd)
	}

	confirmBody := []byte(`{"customer_id":19,"template_version_id":13,"work_order_ids":[91,92]}`)
	confirmReq := httptest.NewRequest(http.MethodPost, "/api/finance/customer-processing-billing/confirm", bytes.NewReader(confirmBody))
	confirmReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	confirmRec := httptest.NewRecorder()
	e.ServeHTTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusOK {
		t.Fatalf("confirm status=%d body=%s", confirmRec.Code, confirmRec.Body.String())
	}
	var confirm struct {
		OK     bool                                   `json:"ok"`
		Result salesapp.ProcessingBillingConfirmation `json:"result"`
	}
	if err := json.Unmarshal(confirmRec.Body.Bytes(), &confirm); err != nil {
		t.Fatal(err)
	}
	if !confirm.OK || confirm.Result.SettlementBatchID != 41 || repo.confirmCmd.Actor == "" {
		t.Fatalf("confirm response=%+v command=%+v", confirm, repo.confirmCmd)
	}
}

func TestProcessingBillingCandidatesRequireCustomerID(t *testing.T) {
	repo := &fakeBillingSalesRepository{}
	e := echo.New()
	registerOutsourceSettingsRoutes(e, salesapp.NewService(repo))
	req := httptest.NewRequest(http.MethodGet, "/api/finance/customer-processing-billing/candidates", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("candidates status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProcessingBillingOptionsAPIUsesFinanceSafeDirectory(t *testing.T) {
	repo := &fakeBillingSalesRepository{}
	e := echo.New()
	registerOutsourceSettingsRoutes(e, salesapp.NewService(repo))
	req := httptest.NewRequest(http.MethodGet, "/api/finance/customer-processing-billing/options", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"customer_name":"9.9 COFFEE LAB"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"current_version_id":13`)) {
		t.Fatalf("options status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProcessingBillingLifecycleAPIContracts(t *testing.T) {
	repo := &fakeBillingSalesRepository{}
	e := echo.New()
	registerOutsourceSettingsRoutes(e, salesapp.NewService(repo))

	listReq := httptest.NewRequest(http.MethodGet, "/api/finance/customer-processing-billing/runs?customer_id=19", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || repo.runsQuery.CustomerID != 19 || !bytes.Contains(listRec.Body.Bytes(), []byte(`"status":"confirmed"`)) {
		t.Fatalf("list status=%d body=%s query=%+v", listRec.Code, listRec.Body.String(), repo.runsQuery)
	}

	postCases := []struct {
		name   string
		path   string
		body   string
		assert func(*testing.T)
	}{
		{name: "pay", path: "/api/finance/customer-processing-billing/runs/41/pay", body: `{"note":"微信收款"}`, assert: func(t *testing.T) {
			if repo.payCmd.BillingRunID != 41 || repo.payCmd.Actor == "" || repo.payCmd.Note != "微信收款" {
				t.Fatalf("pay cmd=%+v", repo.payCmd)
			}
		}},
		{name: "reverse", path: "/api/finance/customer-processing-billing/runs/41/reverse", body: `{"reason":"重复计费"}`, assert: func(t *testing.T) {
			if repo.reverseCmd.BillingRunID != 41 || repo.reverseCmd.Actor == "" || repo.reverseCmd.Reason != "重复计费" {
				t.Fatalf("reverse cmd=%+v", repo.reverseCmd)
			}
		}},
		{name: "adjust", path: "/api/finance/customer-processing-billing/runs/41/adjustments", body: `{"reason":"补收人工费","lines":[{"work_order_id":91,"fee_type":"labor","fee_name":"补收人工","amount":12.5}]}`, assert: func(t *testing.T) {
			if repo.adjustCmd.BillingRunID != 41 || repo.adjustCmd.Actor == "" || len(repo.adjustCmd.Lines) != 1 || repo.adjustCmd.RequestKey == "" {
				t.Fatalf("adjust cmd=%+v", repo.adjustCmd)
			}
		}},
	}
	for _, tc := range postCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"ok":true`)) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			tc.assert(t)
		})
	}
}
