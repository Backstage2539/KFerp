package productspecmigration

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	gotPrepare PrepareCommand
	gotAssess  AssessCommand
	gotCutover CutoverCommand
	gotResolve ResolveIdentityCommand
	migration  ProductMigration
	err        error
	options    []ProductSpecOption
}

type fakeAuthorityUpgradeRepository struct {
	report  AuthorityUpgradeReport
	command AuthorityUpgradeCommand
	mode    AuthorityUpgradeMode
	err     error
}

func TestResolveSpecIdentityModeSupportsDirectProductAuthority(t *testing.T) {
	for _, testCase := range []struct {
		stored string
		state  MigrationState
		legacy bool
		want   string
	}{
		{stored: SpecIdentityModeProduct, state: StatePreparing, legacy: false, want: SpecIdentityModeProduct},
		{stored: SpecIdentityModeBOMSpec, state: StateLegacy, legacy: true, want: SpecIdentityModeBOMSpec},
		{state: StateCutover, legacy: true, want: SpecIdentityModeBOMSpec},
		{state: StateLegacy, legacy: true, want: SpecIdentityModeLegacySKU},
	} {
		if got := ResolveSpecIdentityMode(testCase.stored, testCase.state, testCase.legacy); got != testCase.want {
			t.Fatalf("ResolveSpecIdentityMode(%q,%q,%v)=%q want %q", testCase.stored, testCase.state, testCase.legacy, got, testCase.want)
		}
	}
	if IsBOMSpecAuthoritativeWithMode(SpecIdentityModeProduct, StateCutover, false) {
		t.Fatal("direct product identity must not be BOM-spec authoritative")
	}
}

func (r *fakeAuthorityUpgradeRepository) PreviewAuthorityUpgrade(context.Context) (AuthorityUpgradeReport, error) {
	r.mode = AuthorityUpgradePreview
	return r.report, r.err
}
func (r *fakeAuthorityUpgradeRepository) PrepareAuthorityUpgrade(_ context.Context, cmd AuthorityUpgradeCommand) (AuthorityUpgradeReport, error) {
	r.mode, r.command = AuthorityUpgradePrepare, cmd
	return r.report, r.err
}
func (r *fakeAuthorityUpgradeRepository) ApplyAuthorityUpgrade(_ context.Context, cmd AuthorityUpgradeCommand) (AuthorityUpgradeReport, error) {
	r.mode, r.command = AuthorityUpgradeApply, cmd
	return r.report, r.err
}
func (r *fakeAuthorityUpgradeRepository) RollbackAuthorityUpgrade(_ context.Context, cmd AuthorityUpgradeCommand) (AuthorityUpgradeReport, error) {
	r.mode, r.command = AuthorityUpgradeRollback, cmd
	return r.report, r.err
}

func (r *fakeRepository) Get(context.Context, int64) (ProductMigration, error) {
	return r.migration, r.err
}

func (r *fakeRepository) Prepare(_ context.Context, cmd PrepareCommand) (ProductMigration, error) {
	r.gotPrepare = cmd
	return r.migration, r.err
}

func (r *fakeRepository) Assess(_ context.Context, cmd AssessCommand) (ProductMigration, error) {
	r.gotAssess = cmd
	return r.migration, r.err
}

func (r *fakeRepository) Cutover(_ context.Context, cmd CutoverCommand) (ProductMigration, error) {
	r.gotCutover = cmd
	return r.migration, r.err
}

func (r *fakeRepository) ResolveIdentity(_ context.Context, cmd ResolveIdentityCommand) (BusinessIdentity, error) {
	r.gotResolve = cmd
	return BusinessIdentity{
		ProductID:    cmd.ProductID,
		BomSpecID:    cmd.BomSpecID,
		BomVariantID: cmd.BomVariantID,
	}, r.err
}

func (r *fakeRepository) ListOptions(context.Context, int64) ([]ProductSpecOption, error) {
	return r.options, r.err
}

func TestPrepareRequiresProductAndAuditableActor(t *testing.T) {
	repo := &fakeRepository{migration: ProductMigration{ProductID: 42, State: StatePreparing}}
	svc := NewService(repo)

	got, err := svc.Prepare(context.Background(), PrepareCommand{ProductID: 42, Actor: "  operator-1  "})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StatePreparing {
		t.Fatalf("state = %q, want %q", got.State, StatePreparing)
	}
	if repo.gotPrepare.Actor != "operator-1" {
		t.Fatalf("actor = %q, want trimmed auditable actor", repo.gotPrepare.Actor)
	}

	if _, err := svc.Prepare(context.Background(), PrepareCommand{Actor: "operator-1"}); !errors.Is(err, ErrProductRequired) {
		t.Fatalf("missing product err = %v, want ErrProductRequired", err)
	}
	if _, err := svc.Prepare(context.Background(), PrepareCommand{ProductID: 42}); !errors.Is(err, ErrActorRequired) {
		t.Fatalf("missing actor err = %v, want ErrActorRequired", err)
	}
}

func TestAuthorityUpgradeCommandsRequireActorAndManifest(t *testing.T) {
	repo := &fakeAuthorityUpgradeRepository{report: AuthorityUpgradeReport{ManifestID: "PR-608-abcd"}}
	svc := NewAuthorityUpgradeService(repo)
	if _, err := svc.Preview(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.mode != AuthorityUpgradePreview {
		t.Fatalf("preview mode = %q", repo.mode)
	}
	if _, err := svc.Prepare(context.Background(), AuthorityUpgradeCommand{Actor: " operator "}); err != nil {
		t.Fatal(err)
	}
	if repo.command.Actor != "operator" || repo.command.ManifestID != "" {
		t.Fatalf("prepare command = %+v", repo.command)
	}
	if _, err := svc.Apply(context.Background(), AuthorityUpgradeCommand{Actor: "operator"}); err == nil {
		t.Fatal("apply without manifest should fail")
	}
	if _, err := svc.Rollback(context.Background(), AuthorityUpgradeCommand{Actor: "operator", ManifestID: "PR-608-abcd"}); err != nil {
		t.Fatal(err)
	}
	if repo.mode != AuthorityUpgradeRollback || repo.command.ManifestID != "PR-608-abcd" {
		t.Fatalf("rollback command = %+v mode=%q", repo.command, repo.mode)
	}
}

func TestAssessAndCutoverDelegateAtomicReadinessToRepository(t *testing.T) {
	repo := &fakeRepository{migration: ProductMigration{
		ProductID: 42,
		State:     StatePreparing,
		Readiness: Readiness{Blockers: []Blocker{{Code: "legacy_stock", Count: 1}}},
	}}
	svc := NewService(repo)

	got, err := svc.Assess(context.Background(), AssessCommand{ProductID: 42, Actor: "operator-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Readiness.Ready || len(got.Readiness.Blockers) != 1 {
		t.Fatalf("readiness = %+v, want one blocker", got.Readiness)
	}

	repo.err = &CutoverBlockedError{Readiness: got.Readiness}
	_, err = svc.Cutover(context.Background(), CutoverCommand{ProductID: 42, Actor: "operator-1"})
	var blocked *CutoverBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("cutover err = %v, want CutoverBlockedError", err)
	}
	if len(blocked.Readiness.Blockers) != 1 || blocked.Readiness.Blockers[0].Code != "legacy_stock" {
		t.Fatalf("blocked readiness = %+v", blocked.Readiness)
	}
}

func TestResolveWriteRequiresBOMSpecAfterProductCutover(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)

	repo.err = ErrLegacyWriteRejected
	_, err := svc.ResolveForWrite(context.Background(), ResolveIdentityCommand{
		ProductID:        99,
		LegacySpecG:      227,
		RequireBomSpecID: true,
	})
	if !errors.Is(err, ErrLegacyWriteRejected) {
		t.Fatalf("legacy write err = %v, want ErrLegacyWriteRejected", err)
	}

	repo.err = nil
	specID := int64(501)
	got, err := svc.ResolveForWrite(context.Background(), ResolveIdentityCommand{
		ProductID:        42,
		BomSpecID:        &specID,
		RequireBomSpecID: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProductID != 42 || got.BomSpecID == nil || *got.BomSpecID != 501 {
		t.Fatalf("identity = %+v", got)
	}
	if repo.gotResolve.Mode != ResolveWrite {
		t.Fatalf("resolve mode = %q, want %q", repo.gotResolve.Mode, ResolveWrite)
	}
}

func TestListOptionsKeepsLegacyAndCanonicalWriteIdentityExplicit(t *testing.T) {
	repo := &fakeRepository{options: []ProductSpecOption{{
		ParentProductID:      42,
		LegacyChildProductID: 43,
		BomSpecID:            501,
		BomVariantID:         601,
		WriteProductID:       42,
		WriteBomSpecID:       501,
		MigrationState:       StateCutover,
	}}}
	got, err := NewService(repo).ListOptions(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].WriteProductID != 42 || got[0].WriteBomSpecID != 501 || got[0].LegacyChildProductID != 43 {
		t.Fatalf("options = %+v", got)
	}
}
