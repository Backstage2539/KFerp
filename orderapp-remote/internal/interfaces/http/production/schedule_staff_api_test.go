package production

import (
	"context"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

type staffAPIRepo struct {
	workOrderAPIRepo
	batch   app.ScheduleBatchCommand
	preview bool
}

func (r *staffAPIRepo) ScheduleBatch(_ context.Context, cmd app.ScheduleBatchCommand, preview bool) (app.ScheduleBatchResult, error) {
	r.batch = cmd
	r.preview = preview
	return app.ScheduleBatchResult{Saved: !preview, PreviewToken: "confirmed-preview", Rows: []app.ScheduleTask{}, Conflicts: []app.StaffScheduleConflict{}}, nil
}
func (r *staffAPIRepo) ScheduleOptions(context.Context) (app.ScheduleOptions, error) {
	return app.ScheduleOptions{Employees: []app.ScheduleEmployee{{ID: 3, Name: "包装员工", Active: true}}}, nil
}
func TestScheduleStaffBatchAPI(t *testing.T) {
	e := echo.New()
	repo := &staffAPIRepo{}
	RegisterRoutes(e, Dependencies{Production: app.NewService(repo)})
	body := `{"request_id":"one-save","preview_token":"reviewed","items":[{"work_order_id":2,"job_card_id":4,"expected_version":7,"assigned_employee_id":11}]}`
	for _, path := range []string{"/api/production-schedule/preview", "/api/production-schedule/batch"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		if repo.batch.Operator == "" || len(repo.batch.Items) != 1 || repo.batch.Items[0].PlannedStartAt != nil {
			t.Fatalf("patch fields lost %+v", repo.batch)
		}
		if repo.preview != strings.HasSuffix(path, "preview") {
			t.Fatal("preview became a write")
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/production-schedule/preview", strings.NewReader(`{"request_id":"retired-collaborator","items":[{"work_order_id":2,"job_card_id":4,"expected_version":7,"assigned_employee_id":11,"collaborator_employee_ids":[12]}]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "协作人员功能已停用") {
		t.Fatalf("nonempty collaborator payload accepted: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/production-schedule/preview", strings.NewReader(`{"items":[{"work_order_id":2,"job_card_id":4,"assigned_employee_id":11}]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("missing version accepted %s", rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/production-schedule/options", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var data app.ScheduleOptions
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil || len(data.Employees) != 1 {
		t.Fatal("options missing employees")
	}
}
func TestScheduleSingleAPIKeepsFieldPresence(t *testing.T) {
	e := echo.New()
	repo := &workOrderAPIRepo{}
	RegisterRoutes(e, Dependencies{Production: app.NewService(repo)})
	req := httptest.NewRequest(http.MethodPost, "/api/production-schedule/assign", strings.NewReader(`{"work_order_id":2,"job_card_id":4,"assigned_employee_id":11}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 || repo.scheduleAssignment.Patch == nil {
		t.Fatalf("missing partial patch %d %s", rec.Code, rec.Body.String())
	}
	p := repo.scheduleAssignment.Patch
	if p.PlannedStartAt != nil || p.ShiftCode != nil || p.Note != nil || p.AssignedEmployeeID == nil {
		t.Fatalf("personnel update sends empty schedule fields %+v", p)
	}
}

func TestRetiredProductionOverviewKeepsTaskContext(t *testing.T) {
	e := echo.New()
	registerProductionWorkstationAPI(e, nil)
	req := httptest.NewRequest(http.MethodGet, "/production-overview?work_order_id=88&job_card_id=91&focus=workstation_task", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "vue-shell?view=workstationView&work_order_id=88&job_card_id=91&focus=workstation_task" {
		t.Fatalf("redirect lost task: %d %s", rec.Code, rec.Header().Get("Location"))
	}
}
