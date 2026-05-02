package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderDrawerDirectSenderRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-143",
		"DEV-143-01",
		"UT-143-01",
		"API-143-01",
		"REV-143-01",
		"订单抽屉寄件人直接修改",
		"去掉加入本次快递录单",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("order drawer direct sender requirement seed missing %q", want)
		}
	}
}

func TestOrdersViewDrawerSenderIsDirectAndFollowsGlobalDefault(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		"globalSenderLabel",
		"跟随本次寄件人",
		`v-model.number="orderSenderIDs[Number(activeOrderDetail.id)]"`,
		`if (orderSenderIDs[id] === undefined) orderSenderIDs[id] = 0`,
		"if (overrideID > 0) return senderProfileLabel(overrideID)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrdersView missing direct sender marker %q", want)
		}
	}
	for _, old := range []string{
		"加入本次快递录单",
		"drawer-check",
		`v-if="selectedOrderIDs.includes(Number(activeOrderDetail.id)) && isProductionComplete(activeOrderDetail)"`,
		`<input v-else :value="senderDisplay(activeOrderDetail)" disabled />`,
	} {
		if strings.Contains(src, old) {
			t.Fatalf("OrdersView should remove drawer sender gate marker %q", old)
		}
	}
}
