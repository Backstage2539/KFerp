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

func TestServiceKeepsBOMSpecFinishedInventoryAsWholeSpecificationUnits(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.AdjustFinished(context.Background(), AdjustFinishedInventoryCommand{
		ProductID: 7, BomSpecID: 91, BomVariantID: 191, Units: 12, Operator: " tester ",
	}); err != nil {
		t.Fatalf("AdjustFinished BOM spec: %v", err)
	}
	if repo.adjust.ProductID != 7 || repo.adjust.BomSpecID != 91 || repo.adjust.BomVariantID != 191 || repo.adjust.SpecG != 0 || repo.adjust.Units != 12 || repo.adjust.LooseG != 0 {
		t.Fatalf("adjust=%+v", repo.adjust)
	}
}

func TestServiceAllowsRepositoryToResolveCurrentBOMVariant(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.AdjustFinished(context.Background(), AdjustFinishedInventoryCommand{
		ProductID: 7, BomSpecID: 91, Units: 12, Operator: " tester ",
	}); err != nil {
		t.Fatalf("AdjustFinished without client variant: %v", err)
	}
	if repo.adjust.BomSpecID != 91 || repo.adjust.BomVariantID != 0 || repo.adjust.SpecG != 0 || repo.adjust.Units != 12 {
		t.Fatalf("adjust=%+v, want repository-resolved variant placeholder", repo.adjust)
	}
}

func TestServiceKeepsDirectProductFinishedInventoryAsNamedWholeUnits(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.AdjustFinished(context.Background(), AdjustFinishedInventoryCommand{
		ProductID: 8, SpecG: 0, BomSpecID: 0, BomVariantID: 0,
		UnitCode: " 盒 ", Units: 3, LooseG: 0, Operator: " tester ",
	}); err != nil {
		t.Fatalf("AdjustFinished direct product: %v", err)
	}
	if repo.adjust.ProductID != 8 || repo.adjust.SpecG != 0 || repo.adjust.BomSpecID != 0 ||
		repo.adjust.BomVariantID != 0 || repo.adjust.UnitCode != "盒" || repo.adjust.Units != 3 || repo.adjust.LooseG != 0 {
		t.Fatalf("direct product adjust=%+v", repo.adjust)
	}
}
