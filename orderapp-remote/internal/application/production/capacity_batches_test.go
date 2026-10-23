package production

import (
	"context"
	"testing"
)

func TestCapacityBatchInputAllPlanPaths(t *testing.T) {
	for _, path := range []string{"preview", "splits", "draft"} {
		t.Run(path, func(t *testing.T) {
			svc := NewService(&fakeRepo{})
			for _, tc := range []struct {
				qty     float64
				batches int
				valid   bool
			}{{0, 6, true}, {3.797, 0, true}, {3.797, 1, true}, {0, 0, false}, {0, -1, false}, {-1, 2, false}} {
				row := ProductionPlanOperationSplit{ProductionPlanItemID: 11, WorkstationCapacityID: 7, OperationSeq: 1, PlannedQty: tc.qty, PlannedBatchCount: tc.batches}
				var err error
				switch path {
				case "preview":
					_, err = svc.PreviewProductionPlanOperationSplits(context.Background(), PreviewProductionPlanOperationSplitsCommand{ID: 1, Items: []ProductionPlanOperationSplit{row}})
				case "splits":
					_, err = svc.SaveProductionPlanOperationSplits(context.Background(), SaveProductionPlanOperationSplitsCommand{ID: 1, Items: []ProductionPlanOperationSplit{row}})
				case "draft":
					_, err = svc.SaveProductionPlanDraft(context.Background(), SaveProductionPlanDraftCommand{ID: 1, DraftToken: "token", OperationSplits: []ProductionPlanOperationSplit{row}})
				}
				if (err == nil) != tc.valid {
					t.Errorf("qty=%v batches=%d: err=%v valid=%v", tc.qty, tc.batches, err, tc.valid)
				}
			}
		})
	}
}
