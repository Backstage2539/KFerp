package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceModuleIsWiredIntoAppmainAndPermissions(t *testing.T) {
	for _, tc := range []struct {
		path  string
		wants []string
	}{
		{
			path: filepath.Join("internal", "appmain", "app_routes.go"),
			wants: []string{
				`financeapp "orderapp/internal/application/finance"`,
				`postgresfinance "orderapp/internal/infrastructure/postgres/finance"`,
				`financehttp "orderapp/internal/interfaces/http/finance"`,
				"financehttp.RegisterRoutes",
			},
		},
		{
			path: filepath.Join("internal", "appmain", "schema_setup.go"),
			wants: []string{
				`postgresfinance "orderapp/internal/infrastructure/postgres/finance"`,
				`Name: "finance"`,
				"postgresfinance.EnsureSchema",
			},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "authz", "schema.go"),
			wants: []string{
				`{Code: "finance.read"`,
				`{Code: "finance.write"`,
				`{Code: "finance.close"`,
				`{Code: "finance.close_mode.manage"`,
				`"financeDashboard":`,
				`"financeClosing":`,
			},
		},
		{
			path: filepath.Join("internal", "interfaces", "http", "support", "authz_middleware.go"),
			wants: []string{
				`strings.HasPrefix(path, "/api/finance/settings/closing-mode")`,
				`finance.close_mode.manage`,
				`strings.HasPrefix(path, "/api/finance/")`,
			},
		},
	} {
		src := readSupportTestFile(t, tc.path)
		for _, want := range tc.wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
	}
}
