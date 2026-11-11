package production

import (
	"context"
	"encoding/json"
	"testing"
)

func TestFlexibleCapacityInputAcrossPlanAndWorkOrderPaths(t *testing.T) {
	for _, path := range []string{"preview", "splits", "draft", "work_order"} {
		for _, tc := range []struct {
			name    string
			custom  string
			batches int
			valid   bool
		}{{"custom", "18.7", 3, true}, {"three_decimals", "18.123", 3, true}, {"precision", "18.1234", 3, false}, {"default", "null", 3, true}, {"zero", "0", 3, false}, {"negative", "-1", 3, false}, {"too_large", "10000000000", 3, false}, {"empty_count", "18.7", 0, false}} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				var row ProductionPlanOperationSplit
				if err := json.Unmarshal([]byte(`{"production_plan_item_id":11,"workstation_capacity_id":7,"operation_seq":1,"custom_batch_size_qty":`+tc.custom+`}`), &row); err != nil {
					t.Fatal(err)
				}
				row.PlannedBatchCount = tc.batches
				svc := NewService(&fakeRepo{})
				var err error
				switch path {
				case "preview":
					_, err = svc.PreviewProductionPlanOperationSplits(context.Background(), PreviewProductionPlanOperationSplitsCommand{ID: 1, Items: []ProductionPlanOperationSplit{row}})
				case "splits":
					_, err = svc.SaveProductionPlanOperationSplits(context.Background(), SaveProductionPlanOperationSplitsCommand{ID: 1, Items: []ProductionPlanOperationSplit{row}})
				case "draft":
					_, err = svc.SaveProductionPlanDraft(context.Background(), SaveProductionPlanDraftCommand{ID: 1, DraftToken: "token", OperationSplits: []ProductionPlanOperationSplit{row}})
				case "work_order":
					_, err = svc.SaveWorkOrderOperationSplits(context.Background(), SaveWorkOrderOperationSplitsCommand{ID: 1, Items: []ProductionPlanOperationSplit{row}})
				}
				if (err == nil) != tc.valid {
					t.Fatalf("valid=%v: %v", tc.valid, err)
				}
			})
		}
	}
}
