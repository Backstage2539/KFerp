package production

import (
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	"testing"
)

func TestPR659ProductionLogsPaginationAndInvalidDates(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	registerProductionLogPages(e, productionapp.NewService(repo))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/produce/logs?page=2&limit=20&operator=A&batch_id=PB-77&running_item_id=77&q=coffee", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if repo.productionLogsQuery.Limit != 20 {
		t.Fatalf("requested 20 records, got query %+v", repo.productionLogsQuery)
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"total", "page", "limit", "total_pages"} {
		if _, ok := result[key]; !ok {
			t.Errorf("missing pagination %s", key)
		}
	}
	for _, query := range []string{"from=2026-02-30", "from=2026-09-13&to=2026-09-01", "to=not-a-date"} {
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/produce/logs?"+query, nil))
		if rec.Code != 400 {
			t.Errorf("%s: got %d, want 400", query, rec.Code)
		}
	}
}
