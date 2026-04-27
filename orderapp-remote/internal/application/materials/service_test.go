package materials

import (
	"context"
	"testing"
)

type fakeRepo struct {
	list      ListCommand
	create    CreateCommand
	update    UpdateCommand
	deprecate DeprecateCommand
}

func (r *fakeRepo) List(ctx context.Context, cmd ListCommand) ([]Material, error) {
	r.list = cmd
	return []Material{{ID: 3, Code: "BAG-227", Name: "227g豆袋"}}, nil
}

func (r *fakeRepo) Update(ctx context.Context, cmd UpdateCommand) (Material, error) {
	r.update = cmd
	return Material{ID: cmd.ID, Code: cmd.Input.Code, Name: cmd.Input.Name}, nil
}

func (r *fakeRepo) Create(ctx context.Context, cmd CreateCommand) (Material, error) {
	r.create = cmd
	return Material{ID: 4, Code: cmd.Input.Code, Name: cmd.Input.Name}, nil
}

func (r *fakeRepo) Deprecate(ctx context.Context, cmd DeprecateCommand) (Material, error) {
	r.deprecate = cmd
	return Material{ID: cmd.ID, DeprecatedAt: "2026-04-27 13:30"}, nil
}

func TestServiceOwnsMaterialUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	rows, err := svc.List(ctx, ListCommand{Query: "豆袋", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "BAG-227" || repo.list.Limit != 50 {
		t.Fatalf("List() rows=%+v repo=%+v", rows, repo.list)
	}

	row, err := svc.Update(ctx, UpdateCommand{Actor: "测试员", ID: 3, Input: MaterialInput{Code: "BAG-227", Name: "227g豆袋"}})
	if err != nil {
		t.Fatal(err)
	}
	if row.ID != 3 || repo.update.Actor != "测试员" || repo.update.Input.Name != "227g豆袋" {
		t.Fatalf("Update() row=%+v repo=%+v", row, repo.update)
	}

	created, err := svc.Create(ctx, CreateCommand{Actor: "测试员", Input: MaterialInput{Code: "BAG-228", Name: "228g豆袋"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 4 || repo.create.Input.Code != "BAG-228" {
		t.Fatalf("Create() row=%+v repo=%+v", created, repo.create)
	}

	deprecated, err := svc.Deprecate(ctx, DeprecateCommand{Actor: "测试员", ID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if deprecated.DeprecatedAt == "" || repo.deprecate.ID != 3 {
		t.Fatalf("Deprecate() row=%+v repo=%+v", deprecated, repo.deprecate)
	}
}
