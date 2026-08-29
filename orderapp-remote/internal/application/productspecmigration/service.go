// Package productspecmigration coordinates the gradual replacement of legacy
// child-SKU business identity with parent product + BOM spec identity.
package productspecmigration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type MigrationState string

const (
	StateLegacy    MigrationState = "legacy"
	StatePreparing MigrationState = "preparing"
	StateReady     MigrationState = "ready"
	StateCutover   MigrationState = "cutover"
)

type ResolveMode string

const (
	ResolveRead  ResolveMode = "read"
	ResolveWrite ResolveMode = "write"
)

var (
	ErrProductRequired     = errors.New("product required")
	ErrActorRequired       = errors.New("actor required")
	ErrMigrationNotFound   = errors.New("product BOM spec migration not found")
	ErrLegacyWriteRejected = errors.New("legacy child SKU write rejected after BOM spec cutover")
	ErrBomSpecRequired     = errors.New("bom_spec_id required after BOM spec cutover")
	ErrBomSpecUnavailable  = errors.New("BOM spec is not published for product")
)

type Blocker struct {
	Code    string `json:"code"`
	Count   int64  `json:"count"`
	Message string `json:"message"`
}

type Readiness struct {
	Ready                              bool      `json:"ready"`
	ActiveSpecCount                    int64     `json:"active_spec_count"`
	PublishedSpecCount                 int64     `json:"published_spec_count"`
	UnpublishedSpecCount               int64     `json:"unpublished_spec_count"`
	InvalidSpecTemplateProvenanceCount int64     `json:"invalid_spec_template_provenance_count"`
	InactiveMainInputMaterialCount     int64     `json:"inactive_main_input_material_count"`
	AmbiguousLegacySpecCount           int64     `json:"ambiguous_legacy_spec_count"`
	LegacyUnitMismatchCount            int64     `json:"legacy_unit_mismatch_count"`
	LegacyStockCount                   int64     `json:"legacy_stock_count"`
	LegacyReservationCount             int64     `json:"legacy_reservation_count"`
	UnfinishedOrderCount               int64     `json:"unfinished_order_count"`
	UnfinishedPlanCount                int64     `json:"unfinished_plan_count"`
	UnfinishedWorkOrderCount           int64     `json:"unfinished_work_order_count"`
	UnfinishedFulfillmentCount         int64     `json:"unfinished_fulfillment_count"`
	Blockers                           []Blocker `json:"blockers"`
	CheckedAt                          time.Time `json:"checked_at,omitempty"`
}

type LegacyMapping struct {
	ID                   int64      `json:"id"`
	ParentProductID      int64      `json:"parent_product_id"`
	LegacyChildProductID int64      `json:"legacy_child_product_id"`
	LegacySpecKey        string     `json:"legacy_spec_key"`
	LegacySpecName       string     `json:"legacy_spec_name"`
	LegacySalesUnit      string     `json:"legacy_sales_unit"`
	LegacySpecG          int64      `json:"legacy_spec_g"`
	BomSpecID            int64      `json:"bom_spec_id"`
	BomVariantID         *int64     `json:"bom_variant_id,omitempty"`
	MetadataSnapshot     string     `json:"metadata_snapshot,omitempty"`
	TombstonedAt         *time.Time `json:"tombstoned_at,omitempty"`
}

type ProductMigration struct {
	ProductID            int64           `json:"product_id"`
	State                MigrationState  `json:"state"`
	MigrationState       MigrationState  `json:"migration_state"`
	LegacyCatalogProduct bool            `json:"legacy_catalog_product"`
	SpecIdentityMode     string          `json:"spec_identity_mode"`
	BomSpecAuthoritative bool            `json:"bom_spec_authoritative"`
	Readiness            Readiness       `json:"readiness"`
	Mappings             []LegacyMapping `json:"mappings"`
	PreparedAt           *time.Time      `json:"prepared_at,omitempty"`
	ReadyAt              *time.Time      `json:"ready_at,omitempty"`
	CutoverAt            *time.Time      `json:"cutover_at,omitempty"`
	UpdatedAt            time.Time       `json:"updated_at,omitempty"`
}

type PrepareCommand struct {
	ProductID int64
	Actor     string
}

type AssessCommand struct {
	ProductID int64
	Actor     string
}

type CutoverCommand struct {
	ProductID int64
	Actor     string
}

// AuthorityUpgradeMode is the whole-catalog, BOM-owned specification upgrade
// workflow.  It intentionally lives beside the per-product migration API so
// callers cannot accidentally mix a preview manifest with a different catalog
// snapshot.
type AuthorityUpgradeMode string

const (
	AuthorityUpgradePreview  AuthorityUpgradeMode = "preview"
	AuthorityUpgradePrepare  AuthorityUpgradeMode = "prepare"
	AuthorityUpgradeApply    AuthorityUpgradeMode = "apply"
	AuthorityUpgradeRollback AuthorityUpgradeMode = "rollback"
)

type AuthorityUpgradeProduct struct {
	ProductID             int64            `json:"product_id"`
	ProductName           string           `json:"product_name"`
	LegacyChildCount      int64            `json:"legacy_child_count"`
	BomID                 int64            `json:"bom_id"`
	BomVersionID          int64            `json:"bom_version_id"`
	BomVersionNo          string           `json:"bom_version_no,omitempty"`
	PublishedVariantCount int64            `json:"published_variant_count"`
	DefaultVariantCount   int64            `json:"default_variant_count"`
	InvalidUnitCount      int64            `json:"invalid_unit_count"`
	RecipeItemCount       int64            `json:"recipe_item_count"`
	ReferenceCounts       map[string]int64 `json:"reference_counts,omitempty"`
	Ready                 bool             `json:"ready"`
	Blockers              []string         `json:"blockers,omitempty"`
}

type AuthorityUpgradeReport struct {
	ManifestID          string                    `json:"manifest_id"`
	Mode                AuthorityUpgradeMode      `json:"mode"`
	State               string                    `json:"state"`
	GeneratedAt         time.Time                 `json:"generated_at"`
	ActiveProductCount  int64                     `json:"active_product_count"`
	ReadyProductCount   int64                     `json:"ready_product_count"`
	BlockedProductCount int64                     `json:"blocked_product_count"`
	LegacyChildCount    int64                     `json:"legacy_child_count"`
	DeletedChildCount   int64                     `json:"deleted_child_count,omitempty"`
	RewrittenRefCount   int64                     `json:"rewritten_reference_count,omitempty"`
	Products            []AuthorityUpgradeProduct `json:"products"`
	Message             string                    `json:"message,omitempty"`
}

type AuthorityUpgradeCommand struct {
	Actor      string
	ManifestID string
}

type AuthorityUpgradeRepository interface {
	PreviewAuthorityUpgrade(context.Context) (AuthorityUpgradeReport, error)
	PrepareAuthorityUpgrade(context.Context, AuthorityUpgradeCommand) (AuthorityUpgradeReport, error)
	ApplyAuthorityUpgrade(context.Context, AuthorityUpgradeCommand) (AuthorityUpgradeReport, error)
	RollbackAuthorityUpgrade(context.Context, AuthorityUpgradeCommand) (AuthorityUpgradeReport, error)
}

type AuthorityUpgradeService struct {
	repo AuthorityUpgradeRepository
}

func NewAuthorityUpgradeService(repo AuthorityUpgradeRepository) *AuthorityUpgradeService {
	return &AuthorityUpgradeService{repo: repo}
}

func (s *AuthorityUpgradeService) Preview(ctx context.Context) (AuthorityUpgradeReport, error) {
	if s == nil || s.repo == nil {
		return AuthorityUpgradeReport{}, errors.New("authority upgrade repository required")
	}
	return s.repo.PreviewAuthorityUpgrade(ctx)
}

func (s *AuthorityUpgradeService) Prepare(ctx context.Context, cmd AuthorityUpgradeCommand) (AuthorityUpgradeReport, error) {
	if err := normalizeUpgradeCommand(&cmd); err != nil {
		return AuthorityUpgradeReport{}, err
	}
	return s.repo.PrepareAuthorityUpgrade(ctx, cmd)
}

func (s *AuthorityUpgradeService) Apply(ctx context.Context, cmd AuthorityUpgradeCommand) (AuthorityUpgradeReport, error) {
	if err := normalizeUpgradeCommand(&cmd); err != nil {
		return AuthorityUpgradeReport{}, err
	}
	if cmd.ManifestID == "" {
		return AuthorityUpgradeReport{}, errors.New("manifest_id required for apply")
	}
	return s.repo.ApplyAuthorityUpgrade(ctx, cmd)
}

func (s *AuthorityUpgradeService) Rollback(ctx context.Context, cmd AuthorityUpgradeCommand) (AuthorityUpgradeReport, error) {
	if err := normalizeUpgradeCommand(&cmd); err != nil {
		return AuthorityUpgradeReport{}, err
	}
	if cmd.ManifestID == "" {
		return AuthorityUpgradeReport{}, errors.New("manifest_id required for rollback")
	}
	return s.repo.RollbackAuthorityUpgrade(ctx, cmd)
}

func normalizeUpgradeCommand(cmd *AuthorityUpgradeCommand) error {
	if cmd == nil {
		return ErrActorRequired
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.ManifestID = strings.TrimSpace(cmd.ManifestID)
	if cmd.Actor == "" {
		return ErrActorRequired
	}
	return nil
}

type ResolveIdentityCommand struct {
	ProductID        int64
	BomSpecID        *int64
	BomVariantID     *int64
	LegacySpecG      int64
	RequireBomSpecID bool
	Mode             ResolveMode
}

type BusinessIdentity struct {
	ProductID            int64          `json:"product_id"`
	BomSpecID            *int64         `json:"bom_spec_id,omitempty"`
	BomVariantID         *int64         `json:"bom_variant_id,omitempty"`
	LegacyProductID      *int64         `json:"legacy_product_id,omitempty"`
	LegacySpecG          int64          `json:"legacy_spec_g,omitempty"`
	MigrationState       MigrationState `json:"migration_state"`
	SpecIdentityMode     string         `json:"spec_identity_mode"`
	BomSpecAuthoritative bool           `json:"bom_spec_authoritative"`
	LegacyCompatible     bool           `json:"legacy_compatible"`
}

func IsBOMSpecAuthoritative(state MigrationState, legacyCatalogProduct bool) bool {
	return state == StateCutover || !legacyCatalogProduct
}

func SpecIdentityMode(state MigrationState, legacyCatalogProduct bool) string {
	if IsBOMSpecAuthoritative(state, legacyCatalogProduct) {
		return "bom_spec"
	}
	return "legacy_sku"
}

const (
	SpecIdentityModeLegacySKU = "legacy_sku"
	SpecIdentityModeProduct   = "product"
	SpecIdentityModeBOMSpec   = "bom_spec"
)

// ResolveSpecIdentityMode prefers the persisted business identity selected by
// the default BOM. Empty/older rows retain the pre-PR-619 migration behavior.
func ResolveSpecIdentityMode(stored string, state MigrationState, legacyCatalogProduct bool) string {
	switch strings.ToLower(strings.TrimSpace(stored)) {
	case SpecIdentityModeLegacySKU, SpecIdentityModeProduct, SpecIdentityModeBOMSpec:
		return strings.ToLower(strings.TrimSpace(stored))
	default:
		return SpecIdentityMode(state, legacyCatalogProduct)
	}
}

func IsBOMSpecAuthoritativeWithMode(stored string, state MigrationState, legacyCatalogProduct bool) bool {
	return ResolveSpecIdentityMode(stored, state, legacyCatalogProduct) == SpecIdentityModeBOMSpec
}

// ProductSpecOption is the compatibility contract consumed by catalog,
// pricing, order entry and mini-program clients during gradual cutover.
// Legacy identity is never overloaded with bom_spec_id.
type ProductSpecOption struct {
	ParentProductID      int64          `json:"parent_product_id"`
	LegacyChildProductID int64          `json:"legacy_child_product_id,omitempty"`
	BomID                int64          `json:"bom_id"`
	BomVersionID         int64          `json:"bom_version_id"`
	BomVersionNo         string         `json:"bom_version_no"`
	BomSpecID            int64          `json:"bom_spec_id"`
	BomVariantID         int64          `json:"bom_variant_id,omitempty"`
	SpecCode             string         `json:"spec_code"`
	Barcode              string         `json:"barcode,omitempty"`
	SpecKey              string         `json:"spec_key"`
	SpecName             string         `json:"spec_name"`
	InventoryUnit        string         `json:"inventory_unit"`
	Published            bool           `json:"published"`
	IsDefault            bool           `json:"is_default"`
	SortOrder            int            `json:"sort_order"`
	MigrationState       MigrationState `json:"migration_state"`
	SpecIdentityMode     string         `json:"spec_identity_mode"`
	BomSpecAuthoritative bool           `json:"bom_spec_authoritative"`
	WriteProductID       int64          `json:"write_product_id"`
	WriteBomSpecID       int64          `json:"write_bom_spec_id,omitempty"`
	WriteBomVariantID    int64          `json:"write_bom_variant_id,omitempty"`
}

type CutoverBlockedError struct {
	Readiness Readiness
}

func (e *CutoverBlockedError) Error() string {
	if e == nil {
		return "BOM spec cutover blocked"
	}
	return fmt.Sprintf("BOM spec cutover blocked by %d condition(s)", len(e.Readiness.Blockers))
}

// DefaultBOMSwitchBlockedError prevents a cutover product from abandoning its
// current BOM spec group while mutable business records still reference that
// group. Historical snapshots are intentionally excluded.
type DefaultBOMSwitchBlockedError struct {
	ProductID             int64     `json:"product_id"`
	CurrentBOMID          int64     `json:"current_bom_id"`
	CurrentBOMVersionID   int64     `json:"current_bom_version_id"`
	CandidateBOMID        int64     `json:"candidate_bom_id"`
	CandidateBOMVersionID int64     `json:"candidate_bom_version_id"`
	Blockers              []Blocker `json:"blockers"`
}

func (e *DefaultBOMSwitchBlockedError) Error() string {
	if e == nil {
		return "default BOM switch blocked"
	}
	return fmt.Sprintf("default BOM switch blocked by %d condition(s)", len(e.Blockers))
}

type Repository interface {
	Get(ctx context.Context, productID int64) (ProductMigration, error)
	Prepare(ctx context.Context, cmd PrepareCommand) (ProductMigration, error)
	Assess(ctx context.Context, cmd AssessCommand) (ProductMigration, error)
	Cutover(ctx context.Context, cmd CutoverCommand) (ProductMigration, error)
	ResolveIdentity(ctx context.Context, cmd ResolveIdentityCommand) (BusinessIdentity, error)
	ListOptions(ctx context.Context, productID int64) ([]ProductSpecOption, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, productID int64) (ProductMigration, error) {
	if productID <= 0 {
		return ProductMigration{}, ErrProductRequired
	}
	return s.repo.Get(ctx, productID)
}

func (s *Service) Prepare(ctx context.Context, cmd PrepareCommand) (ProductMigration, error) {
	if err := normalizeWrite(&cmd.ProductID, &cmd.Actor); err != nil {
		return ProductMigration{}, err
	}
	return s.repo.Prepare(ctx, cmd)
}

func (s *Service) Assess(ctx context.Context, cmd AssessCommand) (ProductMigration, error) {
	if err := normalizeWrite(&cmd.ProductID, &cmd.Actor); err != nil {
		return ProductMigration{}, err
	}
	return s.repo.Assess(ctx, cmd)
}

func (s *Service) Cutover(ctx context.Context, cmd CutoverCommand) (ProductMigration, error) {
	if err := normalizeWrite(&cmd.ProductID, &cmd.Actor); err != nil {
		return ProductMigration{}, err
	}
	return s.repo.Cutover(ctx, cmd)
}

func (s *Service) ResolveForRead(ctx context.Context, cmd ResolveIdentityCommand) (BusinessIdentity, error) {
	if cmd.ProductID <= 0 {
		return BusinessIdentity{}, ErrProductRequired
	}
	cmd.Mode = ResolveRead
	return s.repo.ResolveIdentity(ctx, cmd)
}

func (s *Service) ResolveForWrite(ctx context.Context, cmd ResolveIdentityCommand) (BusinessIdentity, error) {
	if cmd.ProductID <= 0 {
		return BusinessIdentity{}, ErrProductRequired
	}
	cmd.Mode = ResolveWrite
	return s.repo.ResolveIdentity(ctx, cmd)
}

func (s *Service) ListOptions(ctx context.Context, productID int64) ([]ProductSpecOption, error) {
	if productID <= 0 {
		return nil, ErrProductRequired
	}
	return s.repo.ListOptions(ctx, productID)
}

func normalizeWrite(productID *int64, actor *string) error {
	if productID == nil || *productID <= 0 {
		return ErrProductRequired
	}
	if actor == nil {
		return ErrActorRequired
	}
	*actor = strings.TrimSpace(*actor)
	if *actor == "" {
		return ErrActorRequired
	}
	return nil
}
