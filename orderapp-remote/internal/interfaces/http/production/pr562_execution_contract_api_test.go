package production

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestPR562JobCardRequirementAndExecutionCommandAPIContracts(t *testing.T) {
	repo := &workOrderAPIRepo{
		rows: []productionapp.WorkOrderRow{{
			ID:          88,
			WorkOrderNo: "WO-PR562-001",
			ProductName: "PR562 商品",
			Status:      "released",
		}},
		jobCards: []productionapp.JobCardRow{{
			ID:          91,
			WorkOrderID: 88,
			WorkOrderNo: "WO-PR562-001",
			ProductName: "PR562 商品",
			SequenceNo:  2,
			Operation:   "包装",
			Status:      "pending",
			ProcessSnapshotJSON: `{
				"operations":[
					{"seq":1,"operation":"烘焙","process_requirement":"按曲线烘焙"},
					{"seq":2,"operation":"包装","process_requirement":"封口完整并核对标签"}
				]
			}`,
		}},
	}
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/produce/job-cards", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET job cards status=%d body=%s", rec.Code, rec.Body.String())
	}
	var cards struct {
		Rows []productionapp.JobCardRow `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cards); err != nil {
		t.Fatal(err)
	}
	if len(cards.Rows) != 1 || cards.Rows[0].ProcessRequirement != "封口完整并核对标签" {
		t.Fatalf("job-card API rows=%+v", cards.Rows)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/produce/work-orders/88", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET execution hub status=%d body=%s", rec.Code, rec.Body.String())
	}
	var detail productionapp.WorkOrderDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	var start *productionapp.ProductionContextAction
	for i := range detail.ExecutionHub.ContextActions {
		action := &detail.ExecutionHub.ContextActions[i]
		if action.Key == "startProduction" {
			start = action
			break
		}
	}
	if start == nil {
		t.Fatalf("execution hub has no start action: %+v", detail.ExecutionHub.ContextActions)
	}
	if start.ActionType != "command" || start.Endpoint != "/api/produce/work-orders/88/start" || start.View != "" {
		t.Fatalf("start action=%+v", *start)
	}
	for _, action := range detail.ExecutionHub.ContextActions {
		if action.Key == "startProduction" {
			continue
		}
		if action.ActionType != "navigate" || action.View == "" || action.Endpoint != "" {
			t.Fatalf("navigation action=%+v", action)
		}
	}
}
