package production

import (
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestRecheckedPlanCannotBeSubmittedWithoutNewSplit(t *testing.T) {
	item := app.ProductionPlanItem{ID: 77, PlannedG: 100000, ProcessSnapshotJSON: `{"operations":[{"seq":1,"operation_id":77,"operation":"分选","execution_json":"{\"reporting_type\":\"incoming\"}"}]}`}
	if err := validateProductionPlanOperationSplitCoverage(item, nil); err == nil || !strings.Contains(err.Error(), "尚未选择") {
		t.Fatalf("missing replan was accepted: %v", err)
	}
}

func TestRecheckReportingTypesOnlyChangesReportingType(t *testing.T) {
	old := `{"id":5,"operations":[{"seq":1,"operation_id":12,"operation":"排序","execution_json":"{\"version\":1,\"node_key\":\"s1\",\"source_warehouse\":\"wip\",\"output_unit\":\"kg\"}"}]}`
	current := `{"id":5,"operations":[{"seq":1,"operation_id":12,"operation":"排序","execution_json":"{\"version\":1,\"node_key\":\"s1\",\"source_warehouse\":\"elsewhere\",\"output_unit\":\"g\",\"reporting_type\":\"incoming\"}"}]}`
	got, changes, err := mergeReportingTypes(old, current)
	if err != nil || len(changes) != 1 || changes[0].To != "incoming" || !strings.Contains(got, `incoming`) || strings.Contains(got, `elsewhere`) {
		t.Fatalf("result=%s changes=%+v err=%v", got, changes, err)
	}
}
func TestRecheckRejectsChangedRouteStructure(t *testing.T) {
	old := `{"operations":[{"seq":1,"operation_id":12,"execution_json":"{\"node_key\":\"s1\"}"}]}`
	current := `{"operations":[{"seq":1,"operation_id":13,"execution_json":"{\"node_key\":\"s1\",\"reporting_type\":\"incoming\"}"}]}`
	if _, _, err := mergeReportingTypes(old, current); err == nil {
		t.Fatal("changed operation must be rejected")
	}
}
func TestRecheckMultiOperationPreservesUnchangedNode(t *testing.T) {
	old := `{"operations":[{"seq":1,"operation_id":12,"operation":"烘焙","execution_json":"{\"node_key\":\"a\",\"reporting_type\":\"batch\"}"},{"seq":2,"operation_id":13,"operation":"分选","execution_json":"{\"node_key\":\"b\",\"source_warehouse\":\"wip\"}"}]}`
	current := `{"operations":[{"seq":1,"operation_id":12,"operation":"烘焙","execution_json":"{\"node_key\":\"a\",\"reporting_type\":\"batch\"}"},{"seq":2,"operation_id":13,"operation":"分选","execution_json":"{\"node_key\":\"b\",\"source_warehouse\":\"wrong\",\"reporting_type\":\"incoming\"}"}]}`
	got, changes, err := mergeReportingTypes(old, current)
	if err != nil || len(changes) != 1 || changes[0].Seq != 2 || strings.Contains(got, "wrong") || !strings.Contains(got, "batch") {
		t.Fatalf("result %s changes %+v err %v", got, changes, err)
	}
}
