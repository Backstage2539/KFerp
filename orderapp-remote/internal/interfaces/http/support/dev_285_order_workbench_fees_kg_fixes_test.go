package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev285OrderWorkbenchFeesKgRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-285-ORDER-WORKBENCH-FEES-KG-FIXES",
		"DEV-285-CUSTOMER-WORKBENCH-ORDER-DETAIL-AUTH",
		"DEV-285-ERP-ORDER-FEE-RECIPIENT-SNAPSHOTS",
		"DEV-285-KG-EXACT-TIER-PRICING",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 285 order workbench fees kg seed missing %q", want)
		}
	}
}

func TestDev285OrderWorkbenchFeesKgSourceWiring(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "api", "customer-fulfillment.js"): {
			"/api/orders/${Number(orderId)}/detail",
		},
		filepath.Join("internal", "interfaces", "http", "support", "authz_middleware.go"): {
			"isFulfillmentOrderDetailRequest",
			"customer_processing.read",
		},
		filepath.Join("internal", "interfaces", "http", "sales", "order_api.go"): {
			"/api/orders/:id/detail",
			"ensureFulfillmentOrderDetailAccess",
			"receiver_name",
			"outsource_total_fee",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue"): {
			"orderFeeLines(row)",
			"收件信息",
			"费用明细",
			"outsource_total_fee",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"): {
			"tierQuantityUnitLabel(tier)",
			"rowQuantityKg(row)",
			"exactQuantity",
		},
		filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go"): {
			"wholesaleTierQuantityForSpec",
		},
	}

	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 285 marker %q", rel, want)
			}
		}
	}
}

func TestDev285OrderWorkbenchFeesKgManualsAndAcceptanceDocs(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"按 KG 设梯度",
			"快递费说明",
			"委外合计",
			"收件人、电话、地址、公司",
		},
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"): {
			"按 KG 设梯度",
			"快递费说明",
			"委外合计",
			"收件人、电话、地址、公司",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"): {
			"不需要 ERP 内部 `orders.write` 权限",
			"订单详情读取走客户工作台专用详情接口",
		},
		filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"): {
			"不需要 ERP 内部 `orders.write` 权限",
			"订单详情读取走客户工作台专用详情接口",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"KG 规格如果存在专属梯度",
			"ERP 订单列表金额列必须展示费用明细",
			"具备 `customer_processing.read` 的客户工作台账号",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"KG 规格如果存在专属梯度",
			"ERP 订单列表金额列必须展示费用明细",
			"具备 `customer_processing.read` 的客户工作台账号",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"2.5kg、数量 10 袋时按 25kg 匹配对应梯度",
			"ERP 订单列表金额列展示商品金额、运费、优惠、应收",
			"不要求 `orders.write` 权限",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"2.5kg、数量 10 袋时按 25kg 匹配对应梯度",
			"ERP 订单列表金额列展示商品金额、运费、优惠、应收",
			"不要求 `orders.write` 权限",
		},
	}

	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 285 manual marker %q", rel, want)
			}
		}
	}
}
