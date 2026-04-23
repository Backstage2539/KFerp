package customer

import (
	"context"
	"strings"
	"testing"
)

type fakeRepo struct {
	upsert UpsertCommand
	asset  SaveAssetCommand
}

func (r *fakeRepo) Upsert(ctx context.Context, actor string, id *int64, cmd UpsertCommand) (int64, error) {
	r.upsert = cmd
	return 9, nil
}

func (r *fakeRepo) Prefs(ctx context.Context, id int64) (*Prefs, error) {
	return &Prefs{ID: id}, nil
}

func (r *fakeRepo) SaveAsset(ctx context.Context, cmd SaveAssetCommand) (SaveAssetResult, error) {
	r.asset = cmd
	return SaveAssetResult{CustomerID: cmd.CustomerID, ObjectKey: "obj", Bytes: 3, SHA256: "sha"}, nil
}

func (r *fakeRepo) DeleteAsset(ctx context.Context, actor string, assetID int64) (DeleteAssetResult, error) {
	return DeleteAssetResult{CustomerID: 1, ObjectKey: "obj"}, nil
}

func (r *fakeRepo) InlineUpdate(ctx context.Context, actor string, id int64, cmd InlineUpdateCommand) error {
	return nil
}

func (r *fakeRepo) Delete(ctx context.Context, actor string, id int64) error {
	return nil
}

func TestServiceDelegatesCustomerUpsert(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	id, err := svc.Upsert(context.Background(), "actor", nil, UpsertCommand{Name: "Ada"})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if id != 9 || repo.upsert.Name != "Ada" {
		t.Fatalf("Upsert result/id = %d command = %+v", id, repo.upsert)
	}
}

func TestServiceDelegatesAssetSave(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	res, err := svc.SaveAsset(context.Background(), SaveAssetCommand{
		CustomerID:  4,
		Kind:        "logo",
		Reader:      strings.NewReader("abc"),
		ContentType: "image/png",
		Filename:    "logo.png",
	})
	if err != nil {
		t.Fatalf("SaveAsset() error = %v", err)
	}
	if res.CustomerID != 4 || repo.asset.Kind != "logo" {
		t.Fatalf("SaveAsset result = %+v command = %+v", res, repo.asset)
	}
}
