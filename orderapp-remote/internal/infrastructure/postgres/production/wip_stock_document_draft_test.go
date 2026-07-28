package production

import (
	"os"
	"strings"
	"testing"
)

func TestWorkOrderStockDocumentDraftLookupCanTargetOneDraftID(t *testing.T) {
	src, err := os.ReadFile("wip_reservation.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	if !strings.Contains(text, "stockDocumentID int64") {
		t.Fatal("draft lookup must accept the selected stock document id")
	}
	if !strings.Contains(text, "AND ($4=0 OR id=$4)") {
		t.Fatal("draft lookup must constrain a selected id while preserving the no-id quick-entry lookup")
	}
	if !strings.Contains(text, "workOrderID, purpose, isReturn, stockDocumentID") {
		t.Fatal("draft lookup must bind work order, action purpose, return direction and selected id together")
	}
}
