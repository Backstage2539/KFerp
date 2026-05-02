package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderNumberEditDrawerRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-142",
		"DEV-142-01",
		"UT-142-01",
		"API-142-01",
		"REV-142-01",
		"订单号编辑抽屉",
		"点击订单号直接在抽屉编辑订单",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("order number edit drawer requirement seed missing %q", want)
		}
	}
}

func TestOrdersViewMergesOrderNumberAndEditIntoDrawer(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		"OrderEntryView",
		"order-edit-panel",
		"openOrderDetailDrawer(row)",
		`@click.prevent="openOrderDetailDrawer(row)"`,
		`@saved="handleOrderEditSaved"`,
		":edit-id=\"activeOrderDetail.id\"",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrdersView missing order edit drawer marker %q", want)
		}
	}
	for _, old := range []string{
		":href=\"`/order?edit_id=${row.id}`\"",
		":href=\"`/order?edit_id=${activeOrderDetail.id}`\"",
		">编辑</a>",
	} {
		if strings.Contains(src, old) {
			t.Fatalf("OrdersView should not keep separate edit navigation marker %q", old)
		}
	}
}

func TestOrderEntryViewSupportsEmbeddedEditWithoutRedirect(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"defineProps",
		"editId",
		"embedded",
		"defineEmits",
		"emit('saved'",
		"props.editId",
		"!props.embedded && data.redirect_url",
		"watch(",
		"emit('close')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrderEntryView missing embedded edit marker %q", want)
		}
	}
}
