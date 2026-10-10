package officialaccount

import (
	"context"
	"strings"
	"testing"
)

func TestLegacyOrderActionsNeverQueryCustomerData(t *testing.T) {
	h := &Handler{}
	for _, key := range []string{"ORDERS_RECENT_1", "ORDERS_RECENT_3"} {
		// No repository/identity provider: guidance must not read bindings or orders.
		text := h.dispatch(context.Background(), incoming{MsgType: "event", Event: "CLICK", Key: key, From: "any-user"})
		if !strings.Contains(text, "小程序") || strings.Contains(text, "金额") {
			t.Fatal(text)
		}
	}
}
