package support

import "testing"

func TestOperationCompletionStockPermission(t *testing.T) {
	for _, row := range []struct{ method, path, want string }{{"GET", "/api/job-cards/1/completion-preview", "stock.read"}, {"POST", "/api/job-cards/1/complete", "stock.write"}, {"POST", "/api/job-cards/1/execution-batches/complete-all", "stock.write"}, {"POST", "/api/job-cards/1/progress", ""}, {"POST", "/api/produce/job-cards/1/correct-output", "stock.write"}, {"GET", "/app/api/production/process-balances", "stock.read"}, {"POST", "/api/production/legacy-review/submit", "stock.write"}} {
		if got := operationCompletionStockPermission(row.method, row.path); got != row.want {
			t.Errorf("%s %s got %s want %s", row.method, row.path, got, row.want)
		}
	}
}

func TestBOMStartStockPermissions(t *testing.T) {
	for _, r := range []struct{ method, path, want string }{
		{"GET", "/api/job-cards/1/start-preview", "stock.read"}, {"POST", "/api/job-cards/1/start", "stock.write"}, {"POST", "/api/job-cards/1/material-adjustments", "stock.write"}, {"POST", "/api/job-cards/1/start-reversal", "stock.write"}, {"POST", "/api/production/workstation/start/preview", "stock.read"}, {"POST", "/api/production/workstation/start/submit", "stock.write"},
	} {
		if got := operationCompletionStockPermission(r.method, r.path); got != r.want {
			t.Errorf("%s: %s != %s", r.path, got, r.want)
		}
	}
}
