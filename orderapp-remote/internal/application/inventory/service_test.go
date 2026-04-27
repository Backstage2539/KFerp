package inventory

import (
	"context"
	"testing"
)

type fakeRepo struct {
	adjust AdjustFinishedInventoryCommand
}

func (r *fakeRepo) ListFinished(ctx context.Context, query FinishedInventoryQuery) (FinishedInventoryResult, error) {
	return FinishedInventoryResult{}, nil
}

func (r *fakeRepo) AdjustFinished(ctx context.Context, cmd AdjustFinishedInventoryCommand) error {
	r.adjust = cmd
	return nil
}

func (r *fakeRepo) ListAllocations(ctx context.Context, query AllocationLogQuery) (AllocationLogResult, error) {
	return AllocationLogResult{}, nil
}

func TestServiceNormalizesFinishedInventoryAdjustment(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if err := svc.AdjustFinished(context.Background(), AdjustFinishedInventoryCommand{
		ProductID: 1,
		SpecG:     454,
		Units:     1,
		LooseG:    500,
		Operator:  " tester ",
	}); err != nil {
		t.Fatal(err)
	}

	if repo.adjust.Units != 2 || repo.adjust.LooseG != 46 {
		t.Fatalf("adjust = %+v, want 2 units + 46g", repo.adjust)
	}
	if repo.adjust.Operator != "tester" {
		t.Fatalf("operator = %q", repo.adjust.Operator)
	}
}

func TestServiceRejectsInvalidFinishedInventoryAdjustment(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if err := svc.AdjustFinished(context.Background(), AdjustFinishedInventoryCommand{SpecG: 454, Units: 1}); err == nil {
		t.Fatal("expected missing product to fail")
	}
	if err := svc.AdjustFinished(context.Background(), AdjustFinishedInventoryCommand{ProductID: 1, SpecG: 454, Units: -1}); err == nil {
		t.Fatal("expected negative quantity to fail")
	}
}
