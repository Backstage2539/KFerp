package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerFulfillmentManualVisibleInVueShell(t *testing.T) {
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	account := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerFulfillmentView.vue")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))

	for _, want := range []string{
		"import OperationManualView from './views/OperationManualView.vue'",
		"customerFulfillmentManual: OperationManualView",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}
	for _, want := range []string{
		"customerFulfillmentManual",
		"客户履约手册",
	} {
		if !strings.Contains(menu, want) {
			t.Fatalf("menu-ia.js missing %q", want)
		}
	}
	for _, want := range []string{
		"openManual",
		"客户履约手册",
		"kferp:navigate-view",
		"customerFulfillmentManual",
		"提交加工工单",
		"提交代发信息",
		"submitCustomerFulfillmentProcessingWorkOrder",
		"submitCustomerFulfillmentDirectShipOrder",
	} {
		if !strings.Contains(account, want) {
			t.Fatalf("CustomerFulfillmentView.vue missing manual entry %q", want)
		}
	}
	for _, want := range []string{
		"操作手册：客户履约账户",
		"解析导入",
		"应用最新批次",
		"代发清单",
		"月结",
		"常见问题",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("OP_MANUAL_CUSTOMER_FULFILLMENT.md missing %q", want)
		}
	}
}

func TestCustomerFulfillmentManualPermissionAndRequirementSeeds(t *testing.T) {
	authz := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "authz", "schema.go")))
	if !strings.Contains(authz, `"customerFulfillmentManual": "stock.read"`) {
		t.Fatal("customer fulfillment manual should be visible to stock readers")
	}
	req := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{"PR-158", "DEV-158-01", "UT-158-01", "API-158-01", "REV-158-01", "客户履约手册"} {
		if !strings.Contains(req, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestCustomerFulfillmentManualUsesConsolidatedOperationManual(t *testing.T) {
	path := filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	for _, want := range []string{"操作手册：客户履约账户", "三类 Excel", "结果校验"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("%s missing %q", path, want)
		}
	}
}
