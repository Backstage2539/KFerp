package sales

import (
	"errors"
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func TestOrderResponsiblePartyFallbackOnlyHandlesMissingCustomerResponsibleEmployee(t *testing.T) {
	cmd := salesapp.SaveOrderCommand{
		AllowResponsibleEmployeeFallback: true,
		FallbackResponsibleEmployeeID:    2,
		FallbackResponsibleEmployeeName:  "刘祎泊",
	}
	type want struct {
		message string
		ok      bool
	}
	for _, tc := range []want{
		{message: "customer responsible employee required", ok: true},
		{message: "customer responsible employee not found", ok: true},
		{message: "customer not found", ok: false},
	} {
		gotType, gotID, gotName, ok := orderResponsiblePartyFallback(cmd, errors.New(tc.message))
		if ok != tc.ok {
			t.Fatalf("message=%q ok=%v want %v", tc.message, ok, tc.ok)
		}
		if tc.ok && (gotType != "employee" || gotID != 2 || gotName != "刘祎泊") {
			t.Fatalf("message=%q fallback=(%q,%d,%q)", tc.message, gotType, gotID, gotName)
		}
	}
}
