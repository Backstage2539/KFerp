package materials

import (
	"context"
	"testing"
)

type fakeRepo struct {
	list   ListCommand
	update UpdateCommand
}

func (r *fakeRepo) List(ctx context.Context, cmd ListCommand) ([]Material, error) {
	r.list = cmd
	return []Material{{ID: 3, Code: "BAG-227", Name: "227g豆袋"}}, nil
}

func (r *fakeRepo) Update(ctx context.Context, cmd UpdateCommand) (Material, error) {
	r.update = cmd
	return Material{ID: cmd.ID, Code: cmd.Input.Code, Name: cmd.Input.Name}, nil
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
}

