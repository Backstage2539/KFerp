package support

import (
	"net/http"
	"testing"
)

func TestScheduleStaffUsesExistingProductionPermissions(t *testing.T) {
	for _, path := range []string{"/api/production-schedule", "/api/production-schedule/options", "/api/production-capacity-calendar"} {
		if got := requiredPermissionForRequest(http.MethodGet, path); got != "production.read" {
			t.Fatalf("%s read permission %s", path, got)
		}
	}
	for _, path := range []string{"/api/production-schedule/preview", "/api/production-schedule/batch", "/api/production-schedule/assign", "/api/production-capacity-calendar"} {
		if got := requiredPermissionForRequest(http.MethodPost, path); got != "production.run" {
			t.Fatalf("%s write permission %s", path, got)
		}
	}
}
