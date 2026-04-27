package catalog

import (
	"reflect"
	"testing"
)

func TestInsertIDAtPositionReordersWithoutDuplicatePositionTie(t *testing.T) {
	cases := []struct {
		name     string
		ids      []int64
		movedID  int64
		position int
		want     []int64
	}{
		{name: "move to first", ids: []int64{1, 3}, movedID: 2, position: 1, want: []int64{2, 1, 3}},
		{name: "move to middle", ids: []int64{1, 3}, movedID: 2, position: 2, want: []int64{1, 2, 3}},
		{name: "append when position is too large", ids: []int64{1, 3}, movedID: 2, position: 99, want: []int64{1, 3, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := insertIDAtPosition(tc.ids, tc.movedID, tc.position)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("insertIDAtPosition() = %v, want %v", got, tc.want)
			}
		})
	}
}
