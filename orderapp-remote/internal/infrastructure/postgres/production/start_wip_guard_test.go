package production

import (
	"os"
	"strings"
	"testing"
)

func TestStartChecksWIPAvailabilityBeforeCreatingRunningItem(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	check := strings.Index(text, "ensureWIPStockForRunningItemTx")
	insert := strings.Index(text, "INSERT INTO %s.produce_running_items")
	if check < 0 {
		t.Fatal("Repository.Start must check WIP availability before creating a running item")
	}
	if insert < 0 {
		t.Fatal("Repository.Start missing running item insert")
	}
	if check > insert {
		t.Fatal("Repository.Start checks WIP after creating a running item; start must fail before work is opened")
	}
}
