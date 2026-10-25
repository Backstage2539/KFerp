package production

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/production"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type completionGroupAPIRepo struct {
	*workOrderAPIRepo
	previewIDs []int64
	command    app.OperationCompletionGroupCommand
}

func (r *completionGroupAPIRepo) PreviewOperationCompletionGroup(_ context.Context, ids []int64) (app.OperationCompletionGroupPreview, error) {
	r.previewIDs = append([]int64(nil), ids...)
	return app.OperationCompletionGroupPreview{GroupKey: "pack:test", Version: "v1", Items: []app.OperationCompletionGroupItem{{JobCardID: 1, WorkOrderNo: "WO-1"}, {JobCardID: 2, WorkOrderNo: "WO-2"}}, PlannedQty: 2, RemainingQty: 2, MaxOutputQty: 2, OutputUnit: "袋", WarehouseName: "成品仓"}, nil
}

func (r *completionGroupAPIRepo) CompleteOperationGroup(_ context.Context, cmd app.OperationCompletionGroupCommand) (app.OperationCompletionGroupResult, error) {
	r.command = cmd
	return app.OperationCompletionGroupResult{RequestedQty: cmd.ActualOutputQty, CompletedQty: cmd.ActualOutputQty, OutputUnit: cmd.OutputUnit}, nil
}

func TestWorkstationCompletionGroupPreviewAndSubmitAPI(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(17))
			c.Set("operator_employee", "包装员工")
			c.Set("actor", "包装员工")
			return next(c)
		}
	})
	repo := &completionGroupAPIRepo{workOrderAPIRepo: &workOrderAPIRepo{}}
	registerProductionWorkstationAPI(e, app.NewService(repo))

	previewRequest := httptest.NewRequest(http.MethodPost, "/api/production/workstation/completion/preview", strings.NewReader(`{"job_card_ids":[2,1]}`))
	previewRequest.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	previewResponse := httptest.NewRecorder()
	e.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", previewResponse.Code, previewResponse.Body.String())
	}
	if !reflect.DeepEqual(repo.previewIDs, []int64{1, 2}) {
		t.Fatalf("preview ids not normalized: %v", repo.previewIDs)
	}
	var preview app.OperationCompletionGroupPreview
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil || preview.MaxOutputQty != 2 || preview.WarehouseName != "成品仓" {
		t.Fatalf("preview response=%s err=%v", previewResponse.Body.String(), err)
	}

	payload := `{"job_card_ids":[1,2],"actual_output_qty":2,"output_unit":"袋","actual_minutes":5,"difference_reason":"现场核数","finish_group":true,"expected_version":"v1","request_id":"group-req"}`
	submitRequest := httptest.NewRequest(http.MethodPost, "/api/production/workstation/completion/submit", strings.NewReader(payload))
	submitRequest.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	submitResponse := httptest.NewRecorder()
	e.ServeHTTP(submitResponse, submitRequest)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", submitResponse.Code, submitResponse.Body.String())
	}
	if repo.command.EmployeeID != 17 || repo.command.Operator != "包装员工" || repo.command.ActualOutputQty != 2 || repo.command.RequestID != "group-req" || !repo.command.FinishGroup {
		t.Fatalf("submit command lost user or quantity data: %+v", repo.command)
	}

	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/production/workstation/completion/preview", strings.NewReader(`{"job_card_ids":[1]}`))
	invalidRequest.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	invalidResponse := httptest.NewRecorder()
	e.ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest || !strings.Contains(invalidResponse.Body.String(), "2 至 100 张") {
		t.Fatalf("single-card group should be rejected: %d %s", invalidResponse.Code, invalidResponse.Body.String())
	}
}
