package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev326SalesOrderPaymentCodeSoftDisableWiring(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "sales_order_repository.go")))
	for _, want := range []string{
		"DeactivateSalesOrderPaymentCode",
		"UPDATE %s.sales_order_payment_codes SET active=false",
		`"deactivate"`,
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("sales order payment code soft-disable wiring missing %q", want)
		}
	}
	if strings.Contains(repository, `actor, "sales_order_payment_code", &id, "delete"`) {
		t.Fatalf("sales order payment code disable must not record delete audit action")
	}

	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "sales", "service.go")))
	for _, want := range []string{
		"DeactivateSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error",
		"return s.repo.DeactivateSalesOrderPaymentCode(ctx, id, actor)",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("sales service soft-disable wiring missing %q", want)
		}
	}

	handler := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_settings.go")))
	for _, want := range []string{
		"e.DELETE(\"/api/settings/sales-order/payment-codes/:id\", h.deactivatePaymentCode)",
		"DeactivateSalesOrderPaymentCode",
	} {
		if !strings.Contains(handler, want) {
			t.Fatalf("sales order settings handler soft-disable wiring missing %q", want)
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderSettingsView.vue")))
	for _, want := range []string{
		"deactivatePaymentCode(code)",
		"收款码已停用",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("sales order settings view soft-disable wiring missing %q", want)
		}
	}

	auditPage := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "audit_page.go")))
	for _, want := range []string{
		`case "deactivate":`,
		"停用收款二维码",
		"停用了",
	} {
		if !strings.Contains(auditPage, want) {
			t.Fatalf("audit page soft-disable marker missing %q", want)
		}
	}
}

func TestDev326SalesOrderPaymentCodeSoftDisableRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-326-SALES-ORDER-PAYMENT-CODE-SOFT-DISABLE",
		"DEV-326-SALES-ORDER-PAYMENT-CODE-SOFT-DISABLE",
		"UT-326-SALES-ORDER-PAYMENT-CODE-SOFT-DISABLE",
		"API-326-SALES-ORDER-PAYMENT-CODE-SOFT-DISABLE",
		"REV-326-SALES-ORDER-PAYMENT-CODE-SOFT-DISABLE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 326 requirement seed missing %q", want)
		}
	}
}

func TestDev326SalesOrderPaymentCodeSoftDisableDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-22-sales-order-payment-code-soft-disable.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-326",
			"停用",
			"不删除",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 326 documentation marker %q", rel, want)
			}
		}
	}
}
