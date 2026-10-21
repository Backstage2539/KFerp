package support

import (
	"net/http"
	"testing"
)

func TestOrderCatalogRequiresTheSamePermissionAsOrderEntry(t *testing.T) {
	if got := requiredPermissionForRequest(http.MethodGet, "/api/order/catalog"); got != "orders.write" {
		t.Fatalf("catalog permission=%q, want orders.write", got)
	}
}
