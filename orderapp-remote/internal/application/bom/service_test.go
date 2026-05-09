package bom

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct {
	savedItem     SaveItemCommand
	deletedID     int64
	deactivatedID int64
	activated     int64
	versionFor    int64
}

func (r *fakeRepo) List(ctx context.Context) ([]ListItem, error) { return nil, nil }
func (r *fakeRepo) Detail(ctx context.Context, productID int64) (Detail, error) {
	return Detail{}, nil
}
func (r *fakeRepo) Products(ctx context.Context) ([]Option, error)  { return nil, nil }
func (r *fakeRepo) Materials(ctx context.Context) ([]Option, error) { return nil, nil }
func (r *fakeRepo) BagSpecMappings(ctx context.Context) ([]BagSpecMapping, error) {
	return nil, nil
}
func (r *fakeRepo) SyncProductYield(ctx context.Context, productID int64) error { return nil }
func (r *fakeRepo) DeactivateBom(ctx context.Context, productID int64) error {
	r.deactivatedID = productID
	return nil
}
func (r *fakeRepo) SaveItem(ctx context.Context, cmd SaveItemCommand) error {
	r.savedItem = cmd
	return nil
}
func (r *fakeRepo) DeleteItem(ctx context.Context, cmd DeleteItemCommand) error {
	r.deletedID = cmd.ID
	return nil
}
func (r *fakeRepo) SaveBagSpecMapping(ctx context.Context, cmd SaveBagSpecMappingCommand) error {
	return nil
}
func (r *fakeRepo) DeleteBagSpecMapping(ctx context.Context, specG int64) error { return nil }
func (r *fakeRepo) ListVersions(ctx context.Context, productID int64) ([]Version, error) {
	r.versionFor = productID
	return nil, nil
}
func (r *fakeRepo) CreateVersion(ctx context.Context, cmd CreateVersionCommand) (Version, error) {
	return Version{ID: 9, ProductID: cmd.ProductID, VersionNo: "V001"}, nil
}
func (r *fakeRepo) ActivateVersion(ctx context.Context, versionID int64) error {
	r.activated = versionID
	return nil
}

func TestServiceValidatesSaveItem(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	cases := []SaveItemCommand{
		{ProductID: 0, MaterialID: 1, RatioPct: 10},
		{ProductID: 1, MaterialID: 0, RatioPct: 10},
		{ProductID: 1, MaterialID: 1, RatioPct: 0},
		{ProductID: 1, MaterialID: 1, RatioPct: 100.01},
	}
	for _, tc := range cases {
		if err := svc.SaveItem(ctx, tc); err == nil {
			t.Fatalf("SaveItem(%+v) succeeded, want validation error", tc)
		}
	}

	if err := svc.SaveItem(ctx, SaveItemCommand{ProductID: 1, MaterialID: 2, RatioPct: 25}); err != nil {
		t.Fatalf("SaveItem valid command: %v", err)
	}
	if repo.savedItem.MaterialID != 2 || repo.savedItem.RatioPct != 25 {
		t.Fatalf("savedItem = %+v, want material 2 ratio 25", repo.savedItem)
	}
}

func TestServicePropagatesRepositoryErrors(t *testing.T) {
	wantErr := errors.New("repo failed")
	svc := NewService(errorRepo{err: wantErr})
	if _, err := svc.List(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("List error = %v, want %v", err, wantErr)
	}
}

func TestServiceValidatesBOMVersions(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	if _, err := svc.ListVersions(ctx, 0); err == nil {
		t.Fatal("ListVersions should require product id")
	}
	if _, err := svc.CreateVersion(ctx, CreateVersionCommand{}); err == nil {
		t.Fatal("CreateVersion should require product id")
	}
	if err := svc.ActivateVersion(ctx, 0); err == nil {
		t.Fatal("ActivateVersion should require version id")
	}
	if err := svc.ActivateVersion(ctx, 7); err != nil {
		t.Fatalf("ActivateVersion valid command: %v", err)
	}
	if repo.activated != 7 {
		t.Fatalf("activated = %d, want 7", repo.activated)
	}
}

func TestServiceValidatesDeactivateBom(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	if err := svc.DeactivateBom(ctx, 0); err == nil {
		t.Fatal("DeactivateBom should require product id")
	}
	if err := svc.DeactivateBom(ctx, 7); err != nil {
		t.Fatalf("DeactivateBom valid command: %v", err)
	}
	if repo.deactivatedID != 7 {
		t.Fatalf("deactivatedID = %d, want 7", repo.deactivatedID)
	}
}

type errorRepo struct {
	err error
}

func (r errorRepo) List(ctx context.Context) ([]ListItem, error) { return nil, r.err }
func (r errorRepo) Detail(ctx context.Context, productID int64) (Detail, error) {
	return Detail{}, r.err
}
func (r errorRepo) Products(ctx context.Context) ([]Option, error)  { return nil, r.err }
func (r errorRepo) Materials(ctx context.Context) ([]Option, error) { return nil, r.err }
func (r errorRepo) BagSpecMappings(ctx context.Context) ([]BagSpecMapping, error) {
	return nil, r.err
}
func (r errorRepo) SyncProductYield(ctx context.Context, productID int64) error {
	return r.err
}
func (r errorRepo) DeactivateBom(ctx context.Context, productID int64) error {
	return r.err
}
func (r errorRepo) SaveItem(ctx context.Context, cmd SaveItemCommand) error {
	return r.err
}
func (r errorRepo) DeleteItem(ctx context.Context, cmd DeleteItemCommand) error {
	return r.err
}
func (r errorRepo) SaveBagSpecMapping(ctx context.Context, cmd SaveBagSpecMappingCommand) error {
	return r.err
}
func (r errorRepo) DeleteBagSpecMapping(ctx context.Context, specG int64) error {
	return r.err
}
func (r errorRepo) ListVersions(ctx context.Context, productID int64) ([]Version, error) {
	return nil, r.err
}
func (r errorRepo) CreateVersion(ctx context.Context, cmd CreateVersionCommand) (Version, error) {
	return Version{}, r.err
}
func (r errorRepo) ActivateVersion(ctx context.Context, versionID int64) error {
	return r.err
}
