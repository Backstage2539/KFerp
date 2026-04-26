package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployScriptTargetsDevelopAndVueShellOnly(t *testing.T) {
	body, err := os.ReadFile("../deploy_orderapp.sh")
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("deploy_orderapp.sh is outside the orderapp Docker build context")
		}
		t.Fatal(err)
	}
	src := string(body)
	for _, want := range []string{
		`BRANCH" != "develop"`,
		"origin/develop",
		"frontend-vue-shell",
		"docker compose build orderapp",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("deploy script missing %q", want)
		}
	}
	for _, bad := range []string{
		`BRANCH" != "main"`,
		"origin/main",
		"cd orderapp-remote/frontend\n",
		"Building frontend (React)",
	} {
		if strings.Contains(src, bad) {
			t.Fatalf("deploy script still contains obsolete deployment concern %q", bad)
		}
	}
}

func TestVueShellDoesNotExposeLegacyFallback(t *testing.T) {
	for _, path := range []string{
		"frontend-vue-shell/src/App.vue",
		"frontend-vue-shell/src/views/LegacyMigrationView.vue",
	} {
		if _, err := os.Stat(path); err == nil && strings.HasSuffix(path, "LegacyMigrationView.vue") {
			t.Fatalf("%s should be removed after migrated pages no longer link to legacy templates", path)
		} else if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, bad := range []string{"legacyUrl", "LegacyMigrationView", "打开旧页面"} {
		if strings.Contains(src, bad) {
			t.Fatalf("App.vue still exposes legacy fallback concern %q", bad)
		}
	}
}

func TestMigratedLegacyTemplatesAreRemoved(t *testing.T) {
	for _, path := range []string{
		"templates/audit.html",
		"templates/bom.html",
		"templates/company_departments.html",
		"templates/company_employees.html",
		"templates/customer_edit.html",
		"templates/customers.html",
		"templates/finished_inventory.html",
		"templates/materials.html",
		"templates/order.html",
		"templates/orders.html",
		"templates/produce_machines.html",
		"templates/produce_plan.html",
		"templates/produce_running.html",
		"templates/product_edit.html",
		"templates/products.html",
		"templates/products_print.html",
		"templates/production_logs.html",
		"templates/req_api.html",
		"templates/req_dev.html",
		"templates/req_product.html",
		"templates/req_review.html",
		"templates/req_unit.html",
		"templates/sender_settings.html",
		"templates/unprod_summary.html",
	} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("migrated legacy template should be removed: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

func TestGoRoutesDoNotKeepLegacyTemplateBranches(t *testing.T) {
	err := filepath.WalkDir("internal/interfaces/http", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == "docs" || path == "frontend-vue-shell" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(body)
		for _, bad := range []string{`QueryParam("legacy")`, `c.Render(http.StatusOK, "audit.html"`, `c.Render(http.StatusOK, "orders.html"`, `c.Render(http.StatusOK, "bom.html"`} {
			if strings.Contains(src, bad) {
				t.Fatalf("%s still contains legacy template branch %q", path, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompanyStaffAPIUsesApplicationService(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/company/company_staff.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, want := range []string{"companyapp.NewService", "postgresCompanyRepository"} {
		if !strings.Contains(src, want) {
			t.Fatalf("company_staff.go missing application boundary %q", want)
		}
	}
	start := strings.Index(src, "func registerCompanyStaffAPI")
	if start < 0 {
		t.Fatal("registerCompanyStaffAPI missing")
	}
	handlerSrc := src[start:]
	for _, bad := range []string{"pool.Query(", "pool.QueryRow(", "pool.Exec("} {
		if strings.Contains(handlerSrc, bad) {
			t.Fatalf("registerCompanyStaffAPI still owns persistence concern %q", bad)
		}
	}
}

func TestProduceBatchAPIUsesApplicationServiceForReadModels(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/production/produce_batch_api.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, want := range []string{"productionSvc.ListBatches", "productionSvc.Detail"} {
		if !strings.Contains(src, want) {
			t.Fatalf("produce_batch_api.go missing service call %q", want)
		}
	}
	for _, bad := range []string{"pool.Query(", "pool.QueryRow("} {
		if strings.Contains(src, bad) {
			t.Fatalf("produce_batch_api.go still owns persistence concern %q", bad)
		}
	}
}

func TestDeductPreviewContractTestIsNotSkipped(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/production/produce_batch_deduct_preview_test.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, bad := range []string{"t.Skip", "TODO"} {
		if strings.Contains(src, bad) {
			t.Fatalf("deduct preview contract test still contains %q", bad)
		}
	}
	for _, want := range []string{"/api/produce/batch/", "deduct-preview", "warning_low_stock"} {
		if !strings.Contains(src, want) {
			t.Fatalf("deduct preview contract test missing %q", want)
		}
	}
}
