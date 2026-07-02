package manufacturing

import (
	"os"
	"strings"
	"testing"
)

func TestProcessRouteRepositoryPersistsStandardCostDefaultCapacity(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"COALESCE(pro.standard_cost_capacity_id,0)",
		"sc.id=pro.standard_cost_capacity_id",
		"&op.StandardCostCapacityID",
		"standard_cost_capacity_id,operation",
		"op.StandardCostCapacityID",
		"old_standard_cost_capacity_ids",
		"new_standard_cost_capacity_ids",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("process route repository missing standard cost capacity marker %q", want)
		}
	}
}
