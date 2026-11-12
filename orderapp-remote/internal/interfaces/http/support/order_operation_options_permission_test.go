package support

import (
	"net/http"
	"testing"
)

func TestOrderOperationOptionsRequireOrderEntryPermission(t *testing.T) {
	if got := requiredPermissionForRequest(http.MethodGet, "/api/order-operation-options"); got != "orders.write" {
		t.Fatalf("operation options permission=%q, want orders.write", got)
	}
}
