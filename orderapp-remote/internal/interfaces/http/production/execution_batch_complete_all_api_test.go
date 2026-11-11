package production

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

type completeAllAPIRepo struct {
	*workOrderAPIRepo
	command app.ExecutionBatchCompleteAllCommand
}

func (r *completeAllAPIRepo) CompleteExecutionBatches(_ context.Context, cmd app.ExecutionBatchCompleteAllCommand) (app.ExecutionBatchCompleteAllResult, error) {
	r.command = cmd
	return app.ExecutionBatchCompleteAllResult{}, nil
}
func TestExecutionBatchCompleteAllAPI(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(17)); c.Set("actor", "工人"); return next(c) }
	})
	repo := &completeAllAPIRepo{workOrderAPIRepo: &workOrderAPIRepo{}}
	registerWorkOrderAPI(e, app.NewService(repo))
	payload := `{"expected_version":"v1","request_id":"batch-key","batches":[{"execution_batch_id":7,"actual_input_qty":1.25,"actual_output_qty":1,"actual_minutes":0,"input_unit":"kg","output_unit":"kg"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/job-cards/5/execution-batches/complete-all", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	res := httptest.NewRecorder()
	e.ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if repo.command.ID != 5 || repo.command.EmployeeID != 17 || len(repo.command.Batches) != 1 || repo.command.Batches[0].ActualMinutes == nil || *repo.command.Batches[0].ActualMinutes != 0 {
		t.Fatalf("invalid command: %+v", repo.command)
	}
}
