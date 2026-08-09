package sales

import (
	"os"
	"strings"
	"testing"
)

func TestOrderWritesKeepBlankNotNullTextFieldsNonNull(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	text := string(src)
	for _, forbidden := range []string{
		"nullText(shipTrackingNo)",
		"nullText(shipMethod)",
		"nullText(cmd.Notes)",
		"nullText(cmd.ExpressFee)",
		"nullText(req.Notes)",
		"nullText(req.ExpressFee)",
		"nullText(req.ShipMethod)",
		"nullText(reason)",
		"var nextNotesPtr *string",
		"it.note, qtyAny, it.unit, it.spec, it.unitPrice",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("order writes must not use %s for a NOT NULL text column", forbidden)
		}
	}
	for binding, want := range map[string]int{
		"notNullText(shipTrackingNo)": 3,
		"notNullText(shipMethod)":     2,
		"notNullText(cmd.Notes)":      2,
		"notNullText(cmd.ExpressFee)": 2,
		"notNullText(req.Notes)":      1,
		"notNullText(req.ExpressFee)": 1,
		"notNullText(req.ShipMethod)": 1,
		"notNullText(reason)":         1,
		"nextNotesPtr := &nextNotes":  1,
		"notNullTextPtr(it.unit)":     1,
		"notNullTextPtr(it.spec)":     1,
	} {
		if got := strings.Count(text, binding); got != want {
			t.Fatalf("%s bindings = %d, want %d", binding, got, want)
		}
	}
	if !strings.Contains(text, "nextShip, nextProc, nextNotes); err != nil") {
		t.Fatal("inline order update must bind the normalized non-null notes string")
	}
}

func TestNotNullOrderTextBindingsNormalizeBlankValues(t *testing.T) {
	if got := notNullText("  "); got != "" {
		t.Fatalf("notNullText(blank) = %q, want empty string", got)
	}
	if got := notNullText("  SF123  "); got != "SF123" {
		t.Fatalf("notNullText(value) = %q, want trimmed value", got)
	}
	if got := notNullTextPtr(nil); got != "" {
		t.Fatalf("notNullTextPtr(nil) = %q, want empty string", got)
	}
	blank := "   "
	if got := notNullTextPtr(&blank); got != "" {
		t.Fatalf("notNullTextPtr(blank) = %q, want empty string", got)
	}
}
