package support

import (
	"net/http"
	"testing"
)

func TestWIPTopUpRequiresProductionAndStockPermissions(t *testing.T) {
	for _, prefix := range []string{"", "/app"} {
		path := prefix + "/api/job-cards/120/wip-top-up"
		if got := requiredPermissionForRequest(http.MethodGet, path); got != "production.read" {
			t.Fatalf("GET %s requires %s", path, got)
		}
		if got := operationCompletionStockPermission(http.MethodGet, path); got != "stock.read" {
			t.Fatalf("GET %s stock requires %s", path, got)
		}
		if got := requiredPermissionForRequest(http.MethodPost, path); got != "production.run" {
			t.Fatalf("POST %s requires %s", path, got)
		}
		if got := operationCompletionStockPermission(http.MethodPost, path); got != "stock.write" {
			t.Fatalf("POST %s stock requires %s", path, got)
		}
	}
}
