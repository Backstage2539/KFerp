package support

import (
	"net/http"
	"testing"
)

func TestProcessingBillingPermissionsSeparateTemplatePreviewAndConfirm(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/outsource/templates", "settings.write"},
		{http.MethodPost, "/api/outsource/templates", "settings.write"},
		{http.MethodGet, "/api/finance/customer-processing-billing/options", "finance.read"},
		{http.MethodGet, "/api/finance/customer-processing-billing/candidates", "finance.read"},
		{http.MethodPost, "/api/finance/customer-processing-billing/preview", "finance.read"},
		{http.MethodPost, "/api/finance/customer-processing-billing/confirm", "finance.write"},
		{http.MethodGet, "/api/finance/customer-processing-billing/runs", "finance.read"},
		{http.MethodPost, "/api/finance/customer-processing-billing/runs/41/pay", "finance.write"},
		{http.MethodPost, "/api/finance/customer-processing-billing/runs/41/reverse", "finance.write"},
		{http.MethodPost, "/api/finance/customer-processing-billing/runs/41/adjustments", "finance.write"},
	}
	for _, tc := range cases {
		if got := requiredPermissionForRequest(tc.method, tc.path); got != tc.want {
			t.Errorf("requiredPermissionForRequest(%s,%s)=%q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}
