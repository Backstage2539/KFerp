package production

import "testing"

func TestFirstFrozenRouteSequence(t *testing.T) {
	route := processTemplateSnapshot{Operations: []processSnapshotOperation{
		{Seq: 20, Operation: "烘焙"}, {Seq: 10, Operation: "预处理"},
	}}
	got, err := firstFrozenRouteSequence(route)
	if err != nil || got != 10 {
		t.Fatalf("first frozen route sequence = %d, %v", got, err)
	}
	for _, invalid := range []processTemplateSnapshot{
		{},
		{Operations: []processSnapshotOperation{{Seq: 1}, {Seq: 1}}},
		{Operations: []processSnapshotOperation{{Seq: 0}}},
	} {
		if _, err := firstFrozenRouteSequence(invalid); err == nil {
			t.Fatalf("invalid frozen route accepted: %+v", invalid)
		}
	}
}
