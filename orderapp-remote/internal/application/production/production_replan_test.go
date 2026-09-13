package production

import (
	"context"
	"testing"
)

type replanFakeRepo struct {
	fakeRepo
	previewCmd ProductionReplanPreviewCommand
	commitCmd  ProductionReplanCommand
}

func (r *replanFakeRepo) PreviewProductionReplan(_ context.Context, cmd ProductionReplanPreviewCommand) (ProductionReplanPreview, error) {
	r.previewCmd = cmd
	return ProductionReplanPreview{ProductionPlanID: cmd.ProductionPlanID, Revision: cmd.Revision, CanReplan: true}, nil
}

func (r *replanFakeRepo) ReplanProduction(_ context.Context, cmd ProductionReplanCommand) (ProductionReplanResult, error) {
	r.commitCmd = cmd
	return ProductionReplanResult{PreviousPlanID: cmd.ProductionPlanID, NewPlan: ProductionPlanDetail{ID: 99, Status: "draft"}}, nil
}

func TestProductionReplanRequiresVersionScopeAndIdempotency(t *testing.T) {
	repo := &replanFakeRepo{}
	svc := NewService(repo)
	if _, err := svc.PreviewProductionReplan(context.Background(), ProductionReplanPreviewCommand{ProductionPlanID: 41, Revision: 2, ProductionPlanItemIDs: []int64{7, 7}}); err == nil {
		t.Fatal("duplicate replan item accepted")
	}
	preview, err := svc.PreviewProductionReplan(context.Background(), ProductionReplanPreviewCommand{ProductionPlanID: 41, Revision: 2, ProductionPlanItemIDs: []int64{7}})
	if err != nil || !preview.CanReplan || repo.previewCmd.ProductionPlanItemIDs[0] != 7 {
		t.Fatalf("preview = %+v err=%v cmd=%+v", preview, err, repo.previewCmd)
	}
	if _, err := svc.ReplanProduction(context.Background(), ProductionReplanCommand{ProductionReplanPreviewCommand: ProductionReplanPreviewCommand{ProductionPlanID: 41, Revision: 2, ProductionPlanItemIDs: []int64{7}}, Operator: "计划员"}); err == nil {
		t.Fatal("replan without request id accepted")
	}
	result, err := svc.ReplanProduction(context.Background(), ProductionReplanCommand{ProductionReplanPreviewCommand: ProductionReplanPreviewCommand{ProductionPlanID: 41, Revision: 2, ProductionPlanItemIDs: []int64{7}}, RequestID: "rp-1", Operator: "计划员"})
	if err != nil || result.NewPlan.ID != 99 || repo.commitCmd.RequestID != "rp-1" {
		t.Fatalf("result = %+v err=%v cmd=%+v", result, err, repo.commitCmd)
	}
}
