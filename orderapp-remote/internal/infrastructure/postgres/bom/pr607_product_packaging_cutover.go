package bom

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	bomapp "orderapp/internal/application/bom"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

//go:embed pr607_production_manifest.json
var pr607ProductionManifestJSON []byte

const pr607ProductPackagingLockKey = "pr607-roasted-bean-product-packaging-cutover"

type PR607ProductPackagingManifest struct {
	ManifestID              string                       `json:"manifest_id"`
	SemiFinishedGroupID     int64                        `json:"semi_finished_group_id"`
	ProductGroupID          int64                        `json:"product_group_id"`
	ProductSingleItemID     int64                        `json:"product_single_item_id"`
	ProductBlendItemID      int64                        `json:"product_blend_item_id"`
	SpecTemplateID          int64                        `json:"spec_template_id"`
	SpecTemplateVersionID   int64                        `json:"spec_template_version_id"`
	SpecTemplateVersionNo   string                       `json:"spec_template_version_no"`
	SpecTemplateName        string                       `json:"spec_template_name"`
	SpecTemplateFingerprint string                       `json:"spec_template_fingerprint"`
	PackagingSpecs          []PR607PackagingSpec         `json:"packaging_specs"`
	Entries                 []PR607ProductPackagingEntry `json:"entries"`
}

type PR607PackagingSpec struct {
	SpecKey               string  `json:"spec_key"`
	Name                  string  `json:"name"`
	MainQtyKG             float64 `json:"main_qty_kg"`
	PackagingMaterialID   int64   `json:"packaging_material_id"`
	PackagingMaterialCode string  `json:"packaging_material_code"`
}

type PR607ProductPackagingEntry struct {
	Name                     string `json:"name"`
	SemiBomID                int64  `json:"semi_bom_id"`
	SemiVersionID            int64  `json:"semi_version_id"`
	SemiMaterialID           int64  `json:"semi_material_id"`
	SemiMaterialCode         string `json:"semi_material_code"`
	SourceGroupItemID        int64  `json:"source_group_item_id"`
	TargetGroupItemID        int64  `json:"target_group_item_id"`
	SourceProductID          int64  `json:"source_product_id"`
	TargetProductID          int64  `json:"target_product_id"`
	ExpectedProductName      string `json:"expected_product_name"`
	ExpectedProductActive    bool   `json:"expected_product_active"`
	ExpectedDefaultBomID     int64  `json:"expected_default_bom_id,omitempty"`
	ExpectedDefaultVersionID int64  `json:"expected_default_version_id,omitempty"`
	ExpectedDefaultBomStatus string `json:"expected_default_bom_status,omitempty"`
	Publish                  bool   `json:"publish"`
}

type PR607ProductPackagingEntryResult struct {
	Name           string `json:"name"`
	ProductID      int64  `json:"product_id"`
	SemiMaterialID int64  `json:"semi_material_id"`
	BomID          int64  `json:"bom_id,omitempty"`
	VersionID      int64  `json:"version_id,omitempty"`
	Status         string `json:"status"`
}

type PR607ProductPackagingSummary struct {
	ManifestID       string                             `json:"manifest_id"`
	Mode             string                             `json:"mode"`
	State            string                             `json:"state"`
	ProductCount     int                                `json:"product_count"`
	BomCount         int                                `json:"bom_count"`
	VariantCount     int                                `json:"variant_count"`
	PublishedCount   int                                `json:"published_count"`
	DraftCount       int                                `json:"draft_count"`
	ReactivatedCount int                                `json:"reactivated_count"`
	RenamedCount     int                                `json:"renamed_count"`
	DependencyCount  int                                `json:"dependency_count"`
	Entries          []PR607ProductPackagingEntryResult `json:"entries,omitempty"`
	Message          string                             `json:"message,omitempty"`
}

type pr607ProductState struct {
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
}

type pr607ProductBinding struct {
	ProductID int64 `json:"product_id"`
	BomID     int64 `json:"bom_id"`
	VersionID int64 `json:"version_id"`
}

type pr607OutputBinding struct {
	ProductID int64 `json:"product_id"`
	BomID     int64 `json:"bom_id"`
	VersionID int64 `json:"version_id"`
	IsDefault bool  `json:"is_default"`
}

type pr607ProductConfig struct {
	ProductID int64 `json:"product_id"`
	BomID     int64 `json:"bom_id"`
	VersionID int64 `json:"version_id"`
}

type pr607RollbackSnapshot struct {
	Products       []pr607ProductState   `json:"products"`
	Bindings       []pr607ProductBinding `json:"bindings"`
	OutputBindings []pr607OutputBinding  `json:"output_bindings"`
	Configs        []pr607ProductConfig  `json:"configs"`
}

func LoadPR607ProductionManifest() (PR607ProductPackagingManifest, error) {
	var manifest PR607ProductPackagingManifest
	if err := json.Unmarshal(pr607ProductionManifestJSON, &manifest); err != nil {
		return manifest, err
	}
	if err := validatePR607ProductPackagingManifest(manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func validatePR607ProductPackagingManifest(manifest PR607ProductPackagingManifest) error {
	if strings.TrimSpace(manifest.ManifestID) == "" || manifest.SemiFinishedGroupID <= 0 || manifest.ProductGroupID <= 0 || manifest.ProductSingleItemID <= 0 || manifest.ProductBlendItemID <= 0 || manifest.SpecTemplateID <= 0 || manifest.SpecTemplateVersionID <= 0 || manifest.SpecTemplateVersionNo != "V001" || strings.TrimSpace(manifest.SpecTemplateFingerprint) == "" {
		return fmt.Errorf("invalid PR-607 manifest header")
	}
	if len(manifest.Entries) != 31 || len(manifest.PackagingSpecs) != 7 {
		return fmt.Errorf("PR-607 manifest requires 31 entries and 7 packaging specifications")
	}
	seenNames, seenProducts, seenSemiBOMs, seenSpecs := map[string]bool{}, map[int64]bool{}, map[int64]bool{}, map[string]bool{}
	published, drafts, reactivated, renamed := 0, 0, 0, 0
	for _, spec := range manifest.PackagingSpecs {
		if strings.TrimSpace(spec.SpecKey) == "" || strings.TrimSpace(spec.Name) == "" || spec.MainQtyKG <= 0 || spec.PackagingMaterialID <= 0 || strings.TrimSpace(spec.PackagingMaterialCode) == "" || seenSpecs[spec.SpecKey] {
			return fmt.Errorf("invalid PR-607 packaging specification %+v", spec)
		}
		seenSpecs[spec.SpecKey] = true
	}
	for _, entry := range manifest.Entries {
		if strings.TrimSpace(entry.Name) == "" || entry.SemiBomID <= 0 || entry.SemiVersionID <= 0 || entry.SemiMaterialID <= 0 || entry.SourceGroupItemID <= 0 || entry.TargetGroupItemID <= 0 || entry.SourceProductID <= 0 || entry.TargetProductID <= 0 || strings.TrimSpace(entry.ExpectedProductName) == "" || (entry.ExpectedDefaultBomID == 0) != (entry.ExpectedDefaultVersionID == 0) || (entry.ExpectedDefaultBomID == 0) != (strings.TrimSpace(entry.ExpectedDefaultBomStatus) == "") || seenNames[entry.Name] || seenProducts[entry.TargetProductID] || seenSemiBOMs[entry.SemiBomID] {
			return fmt.Errorf("invalid or duplicate PR-607 entry %+v", entry)
		}
		wantTarget := manifest.ProductSingleItemID
		if entry.SourceGroupItemID == 890 || entry.SourceGroupItemID == 891 {
			wantTarget = manifest.ProductBlendItemID
		}
		if entry.TargetGroupItemID != wantTarget {
			return fmt.Errorf("invalid target category for %s", entry.Name)
		}
		seenNames[entry.Name], seenProducts[entry.TargetProductID], seenSemiBOMs[entry.SemiBomID] = true, true, true
		if entry.Publish {
			published++
		} else {
			drafts++
		}
		if !entry.ExpectedProductActive {
			reactivated++
		}
		if entry.ExpectedProductName != entry.Name {
			renamed++
		}
	}
	if published != 26 || drafts != 5 || reactivated != 11 || renamed != 3 {
		return fmt.Errorf("PR-607 counts must be 26 published, 5 draft, 11 reactivated and 3 renamed; got %d/%d/%d/%d", published, drafts, reactivated, renamed)
	}
	return nil
}

func (r Repository) PreviewPR607ProductPackagingCutover(ctx context.Context) (PR607ProductPackagingSummary, error) {
	manifest, err := LoadPR607ProductionManifest()
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	return r.inspectPR607ProductPackagingCutover(ctx, tx, manifest, false)
}

func (r Repository) ApplyPR607ProductPackagingCutover(ctx context.Context, actor string) (PR607ProductPackagingSummary, error) {
	manifest, err := LoadPR607ProductionManifest()
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":"+pr607ProductPackagingLockKey); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	if err := lockProductionBomDefaultGraphTx(ctx, tx, r.schema); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	preview, err := r.inspectPR607ProductPackagingCutover(ctx, tx, manifest, true)
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	if preview.State == "applied" {
		preview.Mode = "apply"
		preview.Message = "already applied; no changes written"
		return preview, nil
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system-pr607-product-packaging"
	}
	snapshot, err := r.capturePR607RollbackSnapshot(ctx, tx, manifest)
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	result := preview
	result.Mode, result.State, result.Entries = "apply", "applied", nil
	for _, entry := range manifest.Entries {
		row, err := r.applyPR607ProductPackagingEntry(ctx, tx, manifest, entry, actor)
		if err != nil {
			return PR607ProductPackagingSummary{}, fmt.Errorf("%s: %w", entry.Name, err)
		}
		result.Entries = append(result.Entries, row)
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "bom_product_packaging_cutover", nil, "apply", postgresinfra.StrPtr("manifest_id"), nil, postgresinfra.StrPtr(manifest.ManifestID), postgresinfra.AuditMeta{
		"manifest_id": manifest.ManifestID, "product_count": 31, "bom_count": 31, "variant_count": 217, "published_count": 26, "draft_count": 5, "reactivated_count": 11, "renamed_count": 3, "rollback_snapshot": json.RawMessage(snapshotJSON),
	}); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	result.BomCount, result.VariantCount = 31, 217
	result.Message = "PR-607 product packaging BOM cutover applied in one transaction"
	return result, nil
}

func (r Repository) inspectPR607ProductPackagingCutover(ctx context.Context, tx pgx.Tx, manifest PR607ProductPackagingManifest, lock bool) (PR607ProductPackagingSummary, error) {
	summary := PR607ProductPackagingSummary{ManifestID: manifest.ManifestID, Mode: "preview", ProductCount: 31, PublishedCount: 26, DraftCount: 5, ReactivatedCount: 11, RenamedCount: 3}
	semiBOMIDs := pr607SemiBOMIDs(manifest)
	var count int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE source_bom_id=ANY($1) AND output_type='product' AND COALESCE(legacy_product_id,0)=0`, r.schema), semiBOMIDs).Scan(&count); err != nil {
		return summary, err
	}
	if count == 31 {
		if err := r.validatePR607AppliedState(ctx, tx, manifest, &summary); err != nil {
			return summary, err
		}
		summary.State = "applied"
		summary.Message = "all 31 product packaging BOMs and bindings match the locked manifest"
		return summary, nil
	}
	if count != 0 {
		return summary, fmt.Errorf("PR-607 partial state detected: product_boms=%d; aborting", count)
	}
	if err := r.validatePR607PendingState(ctx, tx, manifest, lock, &summary); err != nil {
		return summary, err
	}
	summary.State, summary.BomCount, summary.VariantCount = "ready", 31, 217
	summary.Message = "31 semi-finished BOMs, products, template, packaging materials, categories, defaults and dependencies match the locked manifest"
	return summary, nil
}

func (r Repository) validatePR607PendingState(ctx context.Context, tx pgx.Tx, manifest PR607ProductPackagingManifest, lock bool, summary *PR607ProductPackagingSummary) error {
	if err := r.validatePR607TemplateAndCategories(ctx, tx, manifest, lock); err != nil {
		return err
	}
	memberIDs := []int64{}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT pb.id FROM %s.production_boms pb JOIN %s.business_group_assignments a ON lower(a.usage_key)='production_bom' AND lower(a.object_key)='production_bom' AND a.object_id=pb.id AND a.object_ref='' WHERE a.group_id=$1 AND pb.status='active' ORDER BY pb.id`, r.schema, r.schema), manifest.SemiFinishedGroupID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		memberIDs = append(memberIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	wantMembers := pr607SemiBOMIDs(manifest)
	sort.Slice(wantMembers, func(i, j int) bool { return wantMembers[i] < wantMembers[j] })
	if fmt.Sprint(memberIDs) != fmt.Sprint(wantMembers) {
		return fmt.Errorf("PR-607 semi-finished group membership drift: got %v want %v", memberIDs, wantMembers)
	}
	lockSuffix := ""
	if lock {
		lockSuffix = " FOR UPDATE OF pb,v,m,p"
	}
	for _, entry := range manifest.Entries {
		var bomName, outputType, bomStatus, versionStatus, materialCode, materialName, unit, costUnit, productName string
		var outputMaterialID, sourceGroupItemID int64
		var semi, active bool
		query := fmt.Sprintf(`
			SELECT pb.name,pb.output_type,pb.status,pb.output_material_id,v.status,m.code,m.name,m.unit,m.cost_unit,COALESCE(m.is_semi_finished,false),a.group_item_id,p.name,COALESCE(p.active,true)
			FROM %s.production_boms pb
			JOIN %s.production_bom_versions v ON v.id=$2 AND v.bom_id=pb.id
			JOIN %s.materials m ON m.id=pb.output_material_id AND m.deprecated_at IS NULL
			JOIN %s.business_group_assignments a ON lower(a.usage_key)='production_bom' AND lower(a.object_key)='production_bom' AND a.object_id=pb.id AND a.object_ref=''
			JOIN %s.products p ON p.id=$3 AND COALESCE(p.parent_product_id,0)=0 AND COALESCE(p.customer_id,0)=0
			WHERE pb.id=$1%s`, r.schema, r.schema, r.schema, r.schema, r.schema, lockSuffix)
		if err := tx.QueryRow(ctx, query, entry.SemiBomID, entry.SemiVersionID, entry.TargetProductID).Scan(&bomName, &outputType, &bomStatus, &outputMaterialID, &versionStatus, &materialCode, &materialName, &unit, &costUnit, &semi, &sourceGroupItemID, &productName, &active); err != nil {
			return fmt.Errorf("locked source/product missing for %s: %w", entry.Name, err)
		}
		wantVersionStatus := "draft"
		if entry.Publish {
			wantVersionStatus = "published"
		}
		if bomName != entry.Name || outputType != "material" || bomStatus != "active" || outputMaterialID != entry.SemiMaterialID || versionStatus != wantVersionStatus || materialCode != entry.SemiMaterialCode || materialName != entry.Name+"-半成品" || unit != "kg" || costUnit != "kg" || !semi || sourceGroupItemID != entry.SourceGroupItemID {
			return fmt.Errorf("semi-finished source drift for %s", entry.Name)
		}
		if productName != entry.ExpectedProductName || active != entry.ExpectedProductActive {
			return fmt.Errorf("product identity drift for %s: product %d is %q active=%v", entry.Name, entry.TargetProductID, productName, active)
		}
		var desiredActiveDuplicates int
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.products WHERE id<>$1 AND parent_product_id=0 AND customer_id=0 AND active=true AND name=$2`, r.schema), entry.TargetProductID, entry.Name).Scan(&desiredActiveDuplicates); err != nil {
			return err
		}
		if desiredActiveDuplicates != 0 {
			return fmt.Errorf("duplicate active factory product name %s", entry.Name)
		}
		if err := r.validatePR607ExpectedDefault(ctx, tx, entry); err != nil {
			return err
		}
	}
	productIDs := pr607ProductIDs(manifest)
	deps, err := r.countPR607ProductDependencies(ctx, tx, productIDs)
	if err != nil {
		return err
	}
	summary.DependencyCount = deps
	if deps != 0 {
		return fmt.Errorf("PR-607 target products gained %d unfinished production references; aborting", deps)
	}
	return nil
}

func (r Repository) validatePR607ExpectedDefault(ctx context.Context, tx pgx.Tx, entry PR607ProductPackagingEntry) error {
	var activeBOMs, bindings, outputs, configs int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE output_type='product' AND output_product_id=$1 AND status='active'`, r.schema), entry.TargetProductID).Scan(&activeBOMs); err != nil {
		return err
	}
	if entry.ExpectedDefaultBomID == 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT COUNT(*) FROM %s.product_production_bom_bindings WHERE product_id=$1)+(SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=$1)+(SELECT COUNT(*) FROM %s.product_production_configs WHERE product_id=$1 AND (production_bom_id<>0 OR production_bom_version_id<>0))`, r.schema, r.schema, r.schema), entry.TargetProductID).Scan(&bindings); err != nil {
			return err
		}
		if activeBOMs != 0 || bindings != 0 {
			return fmt.Errorf("unexpected target product BOM/default for %s: active_boms=%d default_rows=%d", entry.Name, activeBOMs, bindings)
		}
		return nil
	}
	if activeBOMs != 0 {
		return fmt.Errorf("unexpected active target BOM for %s, got %d", entry.Name, activeBOMs)
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT COUNT(*) FROM %s.product_production_bom_bindings WHERE product_id=$1 AND bom_id=$2 AND bom_version_id=$3),(SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=$1 AND bom_id=$2 AND bom_version_id=$3 AND is_default=true),(SELECT COUNT(*) FROM %s.product_production_configs WHERE product_id=$1 AND production_bom_id=$2),(SELECT COUNT(*) FROM %s.production_boms pb JOIN %s.production_bom_versions v ON v.bom_id=pb.id WHERE pb.id=$2 AND pb.output_type='product' AND pb.output_product_id=$1 AND pb.status=$4 AND v.id=$3 AND v.status='published')`, r.schema, r.schema, r.schema, r.schema, r.schema), entry.TargetProductID, entry.ExpectedDefaultBomID, entry.ExpectedDefaultVersionID, entry.ExpectedDefaultBomStatus).Scan(&bindings, &outputs, &configs, &activeBOMs); err != nil {
		return err
	}
	if bindings != 1 || outputs != 1 || configs != 1 || activeBOMs != 1 {
		return fmt.Errorf("locked current default drift for %s: binding=%d output=%d config=%d bom=%d", entry.Name, bindings, outputs, configs, activeBOMs)
	}
	return nil
}

func (r Repository) validatePR607TemplateAndCategories(ctx context.Context, tx pgx.Tx, manifest PR607ProductPackagingManifest, lock bool) error {
	var templateName, versionNo, versionStatus string
	query := fmt.Sprintf(`SELECT t.name,v.version_no,v.status FROM %s.production_bom_spec_templates t JOIN %s.production_bom_spec_template_versions v ON v.template_id=t.id WHERE t.id=$1 AND v.id=$2 AND t.active=true`, r.schema, r.schema)
	if lock {
		query += " FOR SHARE OF t,v"
	}
	if err := tx.QueryRow(ctx, query, manifest.SpecTemplateID, manifest.SpecTemplateVersionID).Scan(&templateName, &versionNo, &versionStatus); err != nil {
		return fmt.Errorf("locked specification template missing: %w", err)
	}
	if templateName != manifest.SpecTemplateName || versionNo != manifest.SpecTemplateVersionNo || versionStatus != "published" {
		return fmt.Errorf("specification template identity drift")
	}
	for specIndex, spec := range manifest.PackagingSpecs {
		var name, inventoryUnit, mainUnit, packageUnit, materialCode, materialUnit, materialCostUnit string
		var mainQty, packageQty, loss float64
		var packageMaterialID int64
		var itemCount, sortOrder int
		var isDefault bool
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT v.name,v.inventory_unit,v.is_default,v.sort_order,v.material_loss_rate::float8,
			       COALESCE((SELECT i.qty_per_unit::float8 FROM %s.production_bom_spec_template_variant_items i WHERE i.variant_id=v.id AND i.is_main_input=true LIMIT 1),0),
			       COALESCE((SELECT i.consume_unit FROM %s.production_bom_spec_template_variant_items i WHERE i.variant_id=v.id AND i.is_main_input=true LIMIT 1),''),
			       COALESCE((SELECT i.material_id FROM %s.production_bom_spec_template_variant_items i WHERE i.variant_id=v.id AND i.is_main_input=false LIMIT 1),0),
			       COALESCE((SELECT i.consume_unit FROM %s.production_bom_spec_template_variant_items i WHERE i.variant_id=v.id AND i.is_main_input=false LIMIT 1),''),
			       COALESCE((SELECT i.qty_per_unit::float8 FROM %s.production_bom_spec_template_variant_items i WHERE i.variant_id=v.id AND i.is_main_input=false LIMIT 1),0),
			       (SELECT COUNT(*) FROM %s.production_bom_spec_template_variant_items i WHERE i.variant_id=v.id),m.code,m.unit,m.cost_unit
			FROM %s.production_bom_spec_template_variants v
			JOIN %s.materials m ON m.id=$3 AND m.deprecated_at IS NULL
			WHERE v.version_id=$1 AND v.spec_key=$2`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema), manifest.SpecTemplateVersionID, spec.SpecKey, spec.PackagingMaterialID).Scan(&name, &inventoryUnit, &isDefault, &sortOrder, &loss, &mainQty, &mainUnit, &packageMaterialID, &packageUnit, &packageQty, &itemCount, &materialCode, &materialUnit, &materialCostUnit)
		if err != nil {
			return fmt.Errorf("packaging specification %s missing: %w", spec.SpecKey, err)
		}
		if name != spec.Name || inventoryUnit != "袋" || isDefault != (specIndex == 0) || sortOrder != specIndex+1 || loss != 0 || mainQty != spec.MainQtyKG || mainUnit != "main_input_unit" || packageMaterialID != spec.PackagingMaterialID || packageUnit != "pic" || packageQty != 1 || itemCount != 2 || materialCode != spec.PackagingMaterialCode || materialUnit != "pic" || materialCostUnit != "pic" {
			return fmt.Errorf("packaging specification drift for %s", spec.SpecKey)
		}
	}
	for _, itemID := range []int64{manifest.ProductSingleItemID, manifest.ProductBlendItemID} {
		var ok bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_groups g JOIN %s.business_group_usages u ON u.group_id=g.id JOIN %s.business_group_items i ON i.group_id=g.id WHERE g.id=$1 AND i.id=$2 AND g.active=true AND i.active=true AND u.active=true AND lower(u.usage_key)='production_bom')`, r.schema, r.schema, r.schema), manifest.ProductGroupID, itemID).Scan(&ok); err != nil || !ok {
			return fmt.Errorf("product BOM target category drift for item %d", itemID)
		}
	}
	return nil
}

func (r Repository) applyPR607ProductPackagingEntry(ctx context.Context, tx pgx.Tx, manifest PR607ProductPackagingManifest, entry PR607ProductPackagingEntry, actor string) (PR607ProductPackagingEntryResult, error) {
	if entry.ExpectedProductName != entry.Name {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET name=$2 WHERE id=$1`, r.schema), entry.TargetProductID, entry.Name); err != nil {
			return PR607ProductPackagingEntryResult{}, err
		}
		var childCount int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`WITH updated AS (UPDATE %s.products SET name=trim($2 || ' ' || COALESCE(NULLIF(spec_label,''),NULLIF(derived_spec_name,''),NULLIF(sku_name,''))) WHERE parent_product_id=$1 AND auto_derived_sku=true RETURNING id) SELECT COUNT(*) FROM updated`, r.schema), entry.TargetProductID, entry.Name).Scan(&childCount); err != nil {
			return PR607ProductPackagingEntryResult{}, err
		}
		id := entry.TargetProductID
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product", &id, "rename_for_product_bom_packaging", postgresinfra.StrPtr("name"), postgresinfra.StrPtr(entry.ExpectedProductName), postgresinfra.StrPtr(entry.Name), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "derived_sku_count": childCount}); err != nil {
			return PR607ProductPackagingEntryResult{}, err
		}
	}
	if !entry.ExpectedProductActive {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET active=true WHERE id=$1`, r.schema), entry.TargetProductID); err != nil {
			return PR607ProductPackagingEntryResult{}, err
		}
		id := entry.TargetProductID
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product", &id, "reactivate_for_product_bom_packaging", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("false"), postgresinfra.StrPtr("true"), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID}); err != nil {
			return PR607ProductPackagingEntryResult{}, err
		}
	}
	var bomID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_boms(code,name,output_type,output_product_id,output_material_id,status,legacy_product_id,source_bom_id,source_bom_version_id,created_by,updated_by) VALUES($1,$2,'product',$3,0,'active',0,$4,$5,$6,$6) RETURNING id`, r.schema), fmt.Sprintf("PR607-PENDING-%d", entry.SemiBomID), entry.Name, entry.TargetProductID, entry.SemiBomID, entry.SemiVersionID, actor).Scan(&bomID); err != nil {
		return PR607ProductPackagingEntryResult{}, err
	}
	code, err := nextPR607ProductionBomCodeTx(ctx, tx, r.schema)
	if err != nil {
		return PR607ProductPackagingEntryResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET code=$2 WHERE id=$1`, r.schema), bomID, code); err != nil {
		return PR607ProductPackagingEntryResult{}, err
	}
	var versionID int64
	note := "PR-607 商品分装 BOM"
	if !entry.Publish {
		note += "；待补半成品配方"
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_versions(bom_id,version_no,status,yield_rate,output_qty,output_unit,note,source_spec_template_version_id,main_input_material_id,created_at,created_by) VALUES($1,'V001','draft',1,1,'袋',$2,$3,$4,now(),$5) RETURNING id`, r.schema), bomID, note, manifest.SpecTemplateVersionID, entry.SemiMaterialID, actor).Scan(&versionID); err != nil {
		return PR607ProductPackagingEntryResult{}, err
	}
	if err := copySpecTemplateToProductionBomTx(ctx, tx, r.schema, bomID, versionID, manifest.SpecTemplateVersionID, entry.SemiMaterialID, actor); err != nil {
		return PR607ProductPackagingEntryResult{}, err
	}
	if err := saveBusinessGroupAssignmentForProductionBomTx(ctx, tx, r.schema, actor, bomID, manifest.ProductGroupID, entry.TargetGroupItemID); err != nil {
		return PR607ProductPackagingEntryResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "production_bom", &bomID, "create_product_packaging_bom", postgresinfra.StrPtr("code"), nil, postgresinfra.StrPtr(code), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "bom_version_id": versionID, "output_product_id": entry.TargetProductID, "main_input_material_id": entry.SemiMaterialID, "source_bom_id": entry.SemiBomID, "source_bom_version_id": entry.SemiVersionID, "spec_template_version_id": manifest.SpecTemplateVersionID, "group_id": manifest.ProductGroupID, "group_item_id": entry.TargetGroupItemID}); err != nil {
		return PR607ProductPackagingEntryResult{}, err
	}
	status := "draft"
	if entry.Publish {
		cmd := bomapp.PublishProductionBomVersionCommand{VersionID: versionID, Actor: actor}
		if err := r.validateProductionBomVersionForPublish(ctx, tx, cmd); err != nil {
			return PR607ProductPackagingEntryResult{}, err
		}
		if err := r.publishProductionBomVersionTx(ctx, tx, cmd); err != nil {
			return PR607ProductPackagingEntryResult{}, err
		}
		if err := r.bindPR607ProductDefault(ctx, tx, entry.TargetProductID, bomID, versionID, actor, manifest.ManifestID); err != nil {
			return PR607ProductPackagingEntryResult{}, err
		}
		status = "published"
	}
	return PR607ProductPackagingEntryResult{Name: entry.Name, ProductID: entry.TargetProductID, SemiMaterialID: entry.SemiMaterialID, BomID: bomID, VersionID: versionID, Status: status}, nil
}

func nextPR607ProductionBomCodeTx(ctx context.Context, tx pgx.Tx, schema string) (string, error) {
	var next int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(MAX(substring(code from 5)::bigint),0)+1 FROM %s.production_boms WHERE code ~ '^BOM-[0-9]{6}$'`, schema)).Scan(&next); err != nil {
		return "", err
	}
	return fmt.Sprintf("BOM-%06d", next), nil
}

func (r Repository) bindPR607ProductDefault(ctx context.Context, tx pgx.Tx, productID, bomID, versionID int64, actor, manifestID string) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_production_configs(product_id,production_bom_id,production_bom_version_id,process_route_id,industry_field_template_id,expected_loss_rate,note,created_by,updated_by) VALUES($1,$2,0,0,0,0,'',$3,$3) ON CONFLICT(product_id) DO UPDATE SET production_bom_id=excluded.production_bom_id,production_bom_version_id=0,updated_at=now(),updated_by=excluded.updated_by`, r.schema), productID, bomID, actor); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_at,bound_by) VALUES($1,$2,$3,now(),$4) ON CONFLICT(product_id) DO UPDATE SET bom_id=excluded.bom_id,bom_version_id=excluded.bom_version_id,bound_at=now(),bound_by=excluded.bound_by`, r.schema), productID, bomID, versionID, actor); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_at,updated_by) VALUES('product',$1,$2,$3,true,now(),$4) ON CONFLICT(output_type,output_id) DO UPDATE SET bom_id=excluded.bom_id,bom_version_id=excluded.bom_version_id,is_default=true,updated_at=now(),updated_by=excluded.updated_by`, r.schema), productID, bomID, versionID, actor); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product", &productID, "set_default_production_bom", postgresinfra.StrPtr("default_production_bom_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", bomID)), postgresinfra.AuditMeta{"manifest_id": manifestID, "product_id": productID, "production_bom_id": bomID, "production_bom_version_id": versionID})
}

func (r Repository) validatePR607AppliedState(ctx context.Context, tx pgx.Tx, manifest PR607ProductPackagingManifest, summary *PR607ProductPackagingSummary) error {
	results := make([]PR607ProductPackagingEntryResult, 0, 31)
	for _, entry := range manifest.Entries {
		var bomID, versionID, groupID, groupItemID, mainMaterialID int64
		var bomName, bomStatus, versionStatus, productName string
		var productActive bool
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT pb.id,pb.name,pb.status,v.id,v.status,v.main_input_material_id,a.group_id,a.group_item_id,p.name,COALESCE(p.active,true) FROM %s.production_boms pb JOIN %s.production_bom_versions v ON v.bom_id=pb.id AND v.version_no='V001' JOIN %s.business_group_assignments a ON lower(a.usage_key)='production_bom' AND lower(a.object_key)='production_bom' AND a.object_id=pb.id AND a.object_ref='' JOIN %s.products p ON p.id=pb.output_product_id WHERE pb.source_bom_id=$1 AND pb.source_bom_version_id=$2 AND pb.output_type='product' AND pb.output_product_id=$3 AND COALESCE(pb.legacy_product_id,0)=0`, r.schema, r.schema, r.schema, r.schema), entry.SemiBomID, entry.SemiVersionID, entry.TargetProductID).Scan(&bomID, &bomName, &bomStatus, &versionID, &versionStatus, &mainMaterialID, &groupID, &groupItemID, &productName, &productActive)
		if err != nil {
			return fmt.Errorf("applied product BOM missing for %s: %w", entry.Name, err)
		}
		wantStatus := "draft"
		if entry.Publish {
			wantStatus = "published"
		}
		if bomName != entry.Name || bomStatus != "active" || versionStatus != wantStatus || mainMaterialID != entry.SemiMaterialID || groupID != manifest.ProductGroupID || groupItemID != entry.TargetGroupItemID || productName != entry.Name || !productActive {
			return fmt.Errorf("applied identity drift for %s", entry.Name)
		}
		var variants, items int
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT COUNT(*) FROM %s.production_bom_version_variants WHERE version_id=$1),(SELECT COUNT(*) FROM %s.production_bom_version_items WHERE version_id=$1)`, r.schema, r.schema), versionID).Scan(&variants, &items); err != nil {
			return err
		}
		if variants != 7 || items != 14 {
			return fmt.Errorf("applied packaging snapshot drift for %s: variants=%d items=%d", entry.Name, variants, items)
		}
		for specIndex, spec := range manifest.PackagingSpecs {
			var mainQty, packageQty, loss float64
			var packageMaterialID int64
			var mainUnit, packageUnit, specName, inventoryUnit string
			var isDefault bool
			var sortOrder int
			err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT v.spec_name_snapshot,v.inventory_unit,v.is_default,v.sort_order,v.material_loss_rate::float8,COALESCE((SELECT i.qty_per_unit::float8 FROM %s.production_bom_version_items i WHERE i.version_id=$1 AND i.variant_id=v.id AND i.material_id=$3),0),COALESCE((SELECT i.consume_unit FROM %s.production_bom_version_items i WHERE i.version_id=$1 AND i.variant_id=v.id AND i.material_id=$3),''),COALESCE((SELECT i.material_id FROM %s.production_bom_version_items i WHERE i.version_id=$1 AND i.variant_id=v.id AND i.material_id<>$3 LIMIT 1),0),COALESCE((SELECT i.consume_unit FROM %s.production_bom_version_items i WHERE i.version_id=$1 AND i.variant_id=v.id AND i.material_id<>$3 LIMIT 1),''),COALESCE((SELECT i.qty_per_unit::float8 FROM %s.production_bom_version_items i WHERE i.version_id=$1 AND i.variant_id=v.id AND i.material_id<>$3 LIMIT 1),0) FROM %s.production_bom_version_variants v JOIN %s.production_bom_specs s ON s.id=v.bom_spec_id WHERE v.version_id=$1 AND s.spec_key=$2`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema), versionID, spec.SpecKey, entry.SemiMaterialID).Scan(&specName, &inventoryUnit, &isDefault, &sortOrder, &loss, &mainQty, &mainUnit, &packageMaterialID, &packageUnit, &packageQty)
			if err != nil || specName != spec.Name || inventoryUnit != "袋" || isDefault != (specIndex == 0) || sortOrder != specIndex+1 || loss != 0 || mainQty != spec.MainQtyKG || mainUnit != "kg" || packageMaterialID != spec.PackagingMaterialID || packageUnit != "pic" || packageQty != 1 {
				return fmt.Errorf("applied variant %s drift for %s", spec.SpecKey, entry.Name)
			}
		}
		var defaults int
		if entry.Publish {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT COUNT(*) FROM %s.product_production_bom_bindings WHERE product_id=$1 AND bom_id=$2 AND bom_version_id=$3)+(SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=$1 AND bom_id=$2 AND bom_version_id=$3 AND is_default=true)+(SELECT COUNT(*) FROM %s.product_production_configs WHERE product_id=$1 AND production_bom_id=$2 AND production_bom_version_id=0)`, r.schema, r.schema, r.schema), entry.TargetProductID, bomID, versionID).Scan(&defaults); err != nil {
				return err
			}
			if defaults != 3 {
				return fmt.Errorf("published product default drift for %s: %d/3", entry.Name, defaults)
			}
		} else {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT COUNT(*) FROM %s.product_production_bom_bindings WHERE product_id=$1)+(SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=$1)+(SELECT COUNT(*) FROM %s.product_production_configs WHERE product_id=$1 AND (production_bom_id<>0 OR production_bom_version_id<>0))`, r.schema, r.schema, r.schema), entry.TargetProductID).Scan(&defaults); err != nil {
				return err
			}
			if defaults != 0 {
				return fmt.Errorf("draft-only product gained a default BOM for %s", entry.Name)
			}
		}
		var oldSourceActive bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(source.status='active',false) FROM %s.production_boms semi JOIN %s.production_boms source ON source.id=semi.source_bom_id WHERE semi.id=$1`, r.schema, r.schema), entry.SemiBomID).Scan(&oldSourceActive); err != nil || oldSourceActive {
			return fmt.Errorf("old product-output BOM must remain inactive for %s", entry.Name)
		}
		results = append(results, PR607ProductPackagingEntryResult{Name: entry.Name, ProductID: entry.TargetProductID, SemiMaterialID: entry.SemiMaterialID, BomID: bomID, VersionID: versionID, Status: versionStatus})
	}
	summary.Entries, summary.BomCount, summary.VariantCount = results, 31, 217
	return nil
}

func (r Repository) capturePR607RollbackSnapshot(ctx context.Context, tx pgx.Tx, manifest PR607ProductPackagingManifest) (pr607RollbackSnapshot, error) {
	var snapshot pr607RollbackSnapshot
	for _, entry := range manifest.Entries {
		snapshot.Products = append(snapshot.Products, pr607ProductState{ProductID: entry.TargetProductID, Name: entry.ExpectedProductName, Active: entry.ExpectedProductActive})
	}
	productIDs := pr607ProductIDs(manifest)
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT product_id,bom_id,bom_version_id FROM %s.product_production_bom_bindings WHERE product_id=ANY($1) ORDER BY product_id`, r.schema), productIDs)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var v pr607ProductBinding
		if err := rows.Scan(&v.ProductID, &v.BomID, &v.VersionID); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.Bindings = append(snapshot.Bindings, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snapshot, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT output_id,bom_id,bom_version_id,is_default FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=ANY($1) ORDER BY output_id`, r.schema), productIDs)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var v pr607OutputBinding
		if err := rows.Scan(&v.ProductID, &v.BomID, &v.VersionID, &v.IsDefault); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.OutputBindings = append(snapshot.OutputBindings, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snapshot, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT product_id,production_bom_id,production_bom_version_id FROM %s.product_production_configs WHERE product_id=ANY($1) ORDER BY product_id`, r.schema), productIDs)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var v pr607ProductConfig
		if err := rows.Scan(&v.ProductID, &v.BomID, &v.VersionID); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.Configs = append(snapshot.Configs, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snapshot, err
	}
	rows.Close()
	return snapshot, nil
}

func (r Repository) RollbackPR607ProductPackagingCutover(ctx context.Context, actor string) (PR607ProductPackagingSummary, error) {
	manifest, err := LoadPR607ProductionManifest()
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":"+pr607ProductPackagingLockKey); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	if err := lockProductionBomDefaultGraphTx(ctx, tx, r.schema); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	semiBOMIDs := pr607SemiBOMIDs(manifest)
	var inactive int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE source_bom_id=ANY($1) AND output_type='product' AND COALESCE(legacy_product_id,0)=0 AND status='inactive'`, r.schema), semiBOMIDs).Scan(&inactive); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	if inactive == 31 {
		return PR607ProductPackagingSummary{ManifestID: manifest.ManifestID, Mode: "rollback", State: "rolled_back", ProductCount: 31, BomCount: 31, PublishedCount: 26, DraftCount: 5, Message: "already rolled back; no changes written"}, nil
	}
	preview, err := r.inspectPR607ProductPackagingCutover(ctx, tx, manifest, true)
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	if preview.State != "applied" {
		return PR607ProductPackagingSummary{}, fmt.Errorf("PR-607 rollback requires fully applied state")
	}
	versionIDs := []int64{}
	for _, row := range preview.Entries {
		versionIDs = append(versionIDs, row.VersionID)
	}
	deps, err := r.countPR607VersionDependencies(ctx, tx, versionIDs)
	if err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	if deps != 0 {
		return PR607ProductPackagingSummary{}, fmt.Errorf("PR-607 rollback blocked by %d production references; manual handling required", deps)
	}
	var snapshotJSON []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT meta->'rollback_snapshot' FROM %s.audit_logs WHERE entity_type='bom_product_packaging_cutover' AND action='apply' AND meta->>'manifest_id'=$1 ORDER BY id DESC LIMIT 1`, r.schema), manifest.ManifestID).Scan(&snapshotJSON); err != nil {
		return PR607ProductPackagingSummary{}, fmt.Errorf("rollback snapshot not found: %w", err)
	}
	var snapshot pr607RollbackSnapshot
	if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system-pr607-product-packaging-rollback"
	}
	for _, entry := range manifest.Entries {
		var bomID int64
		var productName string
		var productActive bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT pb.id,p.name,COALESCE(p.active,true) FROM %s.production_boms pb JOIN %s.products p ON p.id=pb.output_product_id WHERE pb.source_bom_id=$1 AND pb.source_bom_version_id=$2 AND pb.output_type='product' AND pb.output_product_id=$3 FOR UPDATE OF pb,p`, r.schema, r.schema), entry.SemiBomID, entry.SemiVersionID, entry.TargetProductID).Scan(&bomID, &productName, &productActive); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
		if productName != entry.Name || !productActive {
			return PR607ProductPackagingSummary{}, fmt.Errorf("product %s changed after PR-607; manual rollback required", entry.Name)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET status='inactive',updated_at=now(),updated_by=$2 WHERE id=$1`, r.schema), bomID, actor); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_production_bom_bindings WHERE product_id=$1 AND bom_id=$2`, r.schema), entry.TargetProductID, bomID); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=$1 AND bom_id=$2`, r.schema), entry.TargetProductID, bomID); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_production_configs SET production_bom_id=0,production_bom_version_id=0,updated_at=now(),updated_by=$2 WHERE product_id=$1 AND production_bom_id=$3`, r.schema), entry.TargetProductID, actor, bomID); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
		if entry.ExpectedProductName != entry.Name {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET name=$2 WHERE id=$1`, r.schema), entry.TargetProductID, entry.ExpectedProductName); err != nil {
				return PR607ProductPackagingSummary{}, err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET name=trim($2 || ' ' || COALESCE(NULLIF(spec_label,''),NULLIF(derived_spec_name,''),NULLIF(sku_name,''))) WHERE parent_product_id=$1 AND auto_derived_sku=true`, r.schema), entry.TargetProductID, entry.ExpectedProductName); err != nil {
				return PR607ProductPackagingSummary{}, err
			}
			id := entry.TargetProductID
			if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product", &id, "restore_name_after_product_packaging_rollback", postgresinfra.StrPtr("name"), postgresinfra.StrPtr(entry.Name), postgresinfra.StrPtr(entry.ExpectedProductName), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID}); err != nil {
				return PR607ProductPackagingSummary{}, err
			}
		}
		if !entry.ExpectedProductActive {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET active=false WHERE id=$1`, r.schema), entry.TargetProductID); err != nil {
				return PR607ProductPackagingSummary{}, err
			}
			id := entry.TargetProductID
			if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product", &id, "restore_active_after_product_packaging_rollback", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID}); err != nil {
				return PR607ProductPackagingSummary{}, err
			}
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "production_bom", &bomID, "deactivate_product_packaging_on_rollback", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("active"), postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "product_id": entry.TargetProductID}); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
	}
	for _, row := range snapshot.Bindings {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_at,bound_by) VALUES($1,$2,$3,now(),$4) ON CONFLICT(product_id) DO UPDATE SET bom_id=excluded.bom_id,bom_version_id=excluded.bom_version_id,bound_at=now(),bound_by=excluded.bound_by`, r.schema), row.ProductID, row.BomID, row.VersionID, actor); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
		id := row.ProductID
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product", &id, "restore_default_after_product_packaging_rollback", postgresinfra.StrPtr("default_production_bom_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", row.BomID)), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "production_bom_version_id": row.VersionID}); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
	}
	for _, row := range snapshot.OutputBindings {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_at,updated_by) VALUES('product',$1,$2,$3,$4,now(),$5) ON CONFLICT(output_type,output_id) DO UPDATE SET bom_id=excluded.bom_id,bom_version_id=excluded.bom_version_id,is_default=excluded.is_default,updated_at=now(),updated_by=excluded.updated_by`, r.schema), row.ProductID, row.BomID, row.VersionID, row.IsDefault, actor); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
	}
	for _, row := range snapshot.Configs {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_production_configs SET production_bom_id=$2,production_bom_version_id=$3,updated_at=now(),updated_by=$4 WHERE product_id=$1`, r.schema), row.ProductID, row.BomID, row.VersionID, actor); err != nil {
			return PR607ProductPackagingSummary{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "bom_product_packaging_cutover", nil, "rollback", postgresinfra.StrPtr("manifest_id"), postgresinfra.StrPtr(manifest.ManifestID), postgresinfra.StrPtr(manifest.ManifestID), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "product_count": 31, "bom_count": 31}); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PR607ProductPackagingSummary{}, err
	}
	return PR607ProductPackagingSummary{ManifestID: manifest.ManifestID, Mode: "rollback", State: "rolled_back", ProductCount: 31, BomCount: 31, PublishedCount: 26, DraftCount: 5, ReactivatedCount: 11, RenamedCount: 3, Message: "PR-607 rollback applied without deleting history"}, nil
}

func (r Repository) countPR607ProductDependencies(ctx context.Context, tx pgx.Tx, productIDs []int64) (int, error) {
	var plans, orders int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_plan_items i JOIN %s.production_plans p ON p.id=i.production_plan_id WHERE (i.product_id=ANY($1) OR i.parent_product_id=ANY($1)) AND lower(COALESCE(p.status,'')) NOT IN ('completed','cancelled','canceled')`, r.schema, r.schema), productIDs).Scan(&plans); err != nil {
		return 0, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.work_orders WHERE (product_id=ANY($1) OR parent_product_id=ANY($1)) AND lower(COALESCE(status,'')) NOT IN ('completed','cancelled','canceled')`, r.schema), productIDs).Scan(&orders); err != nil {
		return 0, err
	}
	return plans + orders, nil
}

func (r Repository) countPR607VersionDependencies(ctx context.Context, tx pgx.Tx, versionIDs []int64) (int, error) {
	var plans, orders int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_plan_items i JOIN %s.production_plans p ON p.id=i.production_plan_id WHERE i.bom_version_id=ANY($1) AND lower(COALESCE(p.status,'')) NOT IN ('completed','cancelled','canceled')`, r.schema, r.schema), versionIDs).Scan(&plans); err != nil {
		return 0, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.work_orders WHERE bom_version_id=ANY($1) AND lower(COALESCE(status,'')) NOT IN ('completed','cancelled','canceled')`, r.schema), versionIDs).Scan(&orders); err != nil {
		return 0, err
	}
	return plans + orders, nil
}

func pr607SemiBOMIDs(manifest PR607ProductPackagingManifest) []int64 {
	ids := make([]int64, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		ids = append(ids, entry.SemiBomID)
	}
	return ids
}
func pr607ProductIDs(manifest PR607ProductPackagingManifest) []int64 {
	ids := make([]int64, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		ids = append(ids, entry.TargetProductID)
	}
	return ids
}
