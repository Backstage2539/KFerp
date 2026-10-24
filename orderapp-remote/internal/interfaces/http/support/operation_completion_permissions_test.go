package support

import "testing"

func TestOperationCompletionStockPermission(t *testing.T) {
	for _, row := range []struct{ method, path, want string }{{"GET", "/api/job-cards/1/completion-preview", "stock.read"}, {"POST", "/api/job-cards/1/complete", "stock.write"}, {"POST", "/api/job-cards/1/progress", ""}, {"POST", "/api/produce/job-cards/1/correct-output", "stock.write"}, {"GET", "/app/api/production/process-balances", "stock.read"}, {"POST", "/api/production/legacy-review/submit", "stock.write"}} {
		if got := operationCompletionStockPermission(row.method, row.path); got != row.want {
			t.Errorf("%s %s got %s want %s", row.method, row.path, got, row.want)
		}
	}
}
