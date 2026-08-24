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

//go:embed pr606_production_manifest.json
var pr606ProductionManifestJSON []byte

const pr606CutoverLockKey = "pr606-roasted-bean-semi-finished-cutover"

type PR606CutoverManifest struct {
	ManifestID                    string                              `json:"manifest_id"`
	ProductionGroupID             int64                               `json:"production_group_id"`
	MaterialGroupID               int64                               `json:"material_group_id"`
	MaterialGroupItemID           int64                               `json:"material_group_item_id"`
	ComponentMaterialReplacements []PR606ComponentMaterialReplacement `json:"component_material_replacements"`
	Entries                       []PR606CutoverManifestEntry         `json:"entries"`
}

type PR606ComponentMaterialReplacement struct {
	SourceMaterialID int64  `json:"source_material_id"`
	SourceName       string `json:"source_name"`
	TargetMaterialID int64  `json:"target_material_id"`
	TargetName       string `json:"target_name"`
}

type PR606CutoverManifestEntry struct {
	SourceBomID         int64  `json:"source_bom_id"`
	SourceName          string `json:"source_name"`
	SourceProductID     int64  `json:"source_product_id"`
	GroupItemID         int64  `json:"group_item_id"`
	SourceVersionID     int64  `json:"source_version_id"`
	SourceVersionNo     string `json:"source_version_no"`
	SourceVersionStatus string `json:"source_version_status"`
	SourceRecipeHash    string `json:"source_recipe_hash"`
	Publish             bool   `json:"publish"`
	ExistingMaterialID  int64  `json:"existing_material_id,omitempty"`
	RecipeOverride      string `json:"recipe_override,omitempty"`
}

type PR606CutoverEntryResult struct {
	SourceBomID          int64  `json:"source_bom_id"`
	SourceName           string `json:"source_name"`
	SourceVersionID      int64  `json:"source_version_id"`
	MaterialID           int64  `json:"material_id,omitempty"`
	MaterialCode         string `json:"material_code,omitempty"`
	ReplacementBomID     int64  `json:"replacement_bom_id,omitempty"`
	ReplacementVersionID int64  `json:"replacement_version_id,omitempty"`
	Status               string `json:"status"`
}

type PR606CutoverSummary struct {
	ManifestID          string                    `json:"manifest_id"`
	Mode                string                    `json:"mode"`
	State               string                    `json:"state"`
	SourceCount         int                       `json:"source_count"`
	NewMaterialCount    int                       `json:"new_material_count"`
	ReusedMaterialCount int                       `json:"reused_material_count"`
	ReplacementBomCount int                       `json:"replacement_bom_count"`
	PublishedCount      int                       `json:"published_count"`
	DraftCount          int                       `json:"draft_count"`
	DependencyCount     int                       `json:"dependency_count"`
	Entries             []PR606CutoverEntryResult `json:"entries,omitempty"`
	Message             string                    `json:"message,omitempty"`
}

type pr606ProductBinding struct {
	ProductID int64 `json:"product_id"`
	BomID     int64 `json:"bom_id"`
	VersionID int64 `json:"version_id"`
}

type pr606OutputBinding struct {
	OutputType string `json:"output_type"`
	OutputID   int64  `json:"output_id"`
	BomID      int64  `json:"bom_id"`
	VersionID  int64  `json:"version_id"`
	IsDefault  bool   `json:"is_default"`
}

type pr606ProductConfig struct {
	ProductID int64 `json:"product_id"`
	BomID     int64 `json:"bom_id"`
	VersionID int64 `json:"version_id"`
}

type pr606MaterialAssignment struct {
	GroupID     int64 `json:"group_id"`
	GroupItemID int64 `json:"group_item_id"`
}

type pr606RollbackSnapshot struct {
	ProductBindings            []pr606ProductBinding     `json:"product_bindings"`
	OutputBindings             []pr606OutputBinding      `json:"output_bindings"`
	ProductConfigs             []pr606ProductConfig      `json:"product_configs"`
	InitialMaterialAssignments []pr606MaterialAssignment `json:"initial_material_assignments"`
}

func LoadPR606ProductionManifest() (PR606CutoverManifest, error) {
	var manifest PR606CutoverManifest
	if err := json.Unmarshal(pr606ProductionManifestJSON, &manifest); err != nil {
		return manifest, err
	}
	if err := validatePR606Manifest(manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func validatePR606Manifest(manifest PR606CutoverManifest) error {
	if strings.TrimSpace(manifest.ManifestID) == "" || manifest.ProductionGroupID <= 0 || manifest.MaterialGroupID <= 0 || manifest.MaterialGroupItemID <= 0 {
		return fmt.Errorf("invalid PR-606 manifest header")
	}
	if len(manifest.Entries) != 31 {
		return fmt.Errorf("PR-606 manifest requires 31 entries, got %d", len(manifest.Entries))
	}
	if len(manifest.ComponentMaterialReplacements) != 2 {
		return fmt.Errorf("PR-606 manifest requires 2 canonical component material replacements")
	}
	seenReplacementSource := map[int64]bool{}
	for _, replacement := range manifest.ComponentMaterialReplacements {
		if replacement.SourceMaterialID <= 0 || replacement.TargetMaterialID <= 0 || replacement.SourceMaterialID == replacement.TargetMaterialID || strings.TrimSpace(replacement.SourceName) == "" || strings.TrimSpace(replacement.TargetName) == "" || seenReplacementSource[replacement.SourceMaterialID] {
			return fmt.Errorf("invalid PR-606 component material replacement %+v", replacement)
		}
		seenReplacementSource[replacement.SourceMaterialID] = true
	}
	seenBOM := map[int64]bool{}
	seenName := map[string]bool{}
	published := 0
	drafts := 0
	reused := 0
	for _, entry := range manifest.Entries {
		name := strings.TrimSpace(entry.SourceName)
		if entry.SourceBomID <= 0 || entry.SourceProductID <= 0 || entry.SourceVersionID <= 0 || name == "" || entry.GroupItemID <= 0 || entry.SourceRecipeHash == "" {
			return fmt.Errorf("invalid PR-606 manifest entry %+v", entry)
		}
		if seenBOM[entry.SourceBomID] || seenName[name] {
			return fmt.Errorf("duplicate PR-606 source BOM %d or name %s", entry.SourceBomID, name)
		}
		seenBOM[entry.SourceBomID], seenName[name] = true, true
		if entry.Publish {
			published++
		} else {
			drafts++
		}
		if entry.ExistingMaterialID > 0 {
			reused++
		}
	}
	if published != 26 || drafts != 5 || reused != 1 {
		return fmt.Errorf("PR-606 manifest counts must be 26 published, 5 draft, 1 reused; got %d/%d/%d", published, drafts, reused)
	}
	return nil
}

func (r Repository) PreviewPR606Cutover(ctx context.Context) (PR606CutoverSummary, error) {
	manifest, err := LoadPR606ProductionManifest()
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	return r.inspectPR606Cutover(ctx, tx, manifest, false)
}

func (r Repository) ApplyPR606Cutover(ctx context.Context, actor string) (PR606CutoverSummary, error) {
	manifest, err := LoadPR606ProductionManifest()
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":"+pr606CutoverLockKey); err != nil {
		return PR606CutoverSummary{}, err
	}
	preview, err := r.inspectPR606Cutover(ctx, tx, manifest, true)
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	if preview.State == "applied" {
		preview.Mode = "apply"
		preview.Message = "already applied; no changes written"
		return preview, nil
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system-pr606-cutover"
	}
	snapshot, err := r.capturePR606RollbackSnapshot(ctx, tx, manifest)
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	result := preview
	result.Mode, result.State, result.Entries = "apply", "applied", nil
	for _, entry := range manifest.Entries {
		row, err := r.applyPR606Entry(ctx, tx, manifest, entry, actor)
		if err != nil {
			return PR606CutoverSummary{}, fmt.Errorf("%s: %w", entry.SourceName, err)
		}
		result.Entries = append(result.Entries, row)
	}
	if err := r.cutOverPR606SourceBindings(ctx, tx, manifest, actor); err != nil {
		return PR606CutoverSummary{}, err
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "bom_semi_finished_cutover", nil, "apply", postgresinfra.StrPtr("manifest_id"), nil, postgresinfra.StrPtr(manifest.ManifestID), postgresinfra.AuditMeta{
		"manifest_id": manifest.ManifestID, "source_count": 31, "new_material_count": 30, "reused_material_count": 1,
		"replacement_bom_count": 31, "published_count": 26, "draft_count": 5, "rollback_snapshot": json.RawMessage(snapshotJSON),
	}); err != nil {
		return PR606CutoverSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PR606CutoverSummary{}, err
	}
	result.NewMaterialCount, result.ReusedMaterialCount, result.ReplacementBomCount = 30, 1, 31
	result.PublishedCount, result.DraftCount = 26, 5
	result.Message = "PR-606 cutover applied in one transaction"
	return result, nil
}

func (r Repository) inspectPR606Cutover(ctx context.Context, tx pgx.Tx, manifest PR606CutoverManifest, lock bool) (PR606CutoverSummary, error) {
	summary := PR606CutoverSummary{ManifestID: manifest.ManifestID, Mode: "preview", SourceCount: len(manifest.Entries), PublishedCount: 26, DraftCount: 5}
	sourceIDs := make([]int64, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		sourceIDs = append(sourceIDs, entry.SourceBomID)
	}
	var replacementCount, inactiveSourceCount int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE source_bom_id=ANY($1) AND output_type='material'`, r.schema), sourceIDs).Scan(&replacementCount); err != nil {
		return summary, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE id=ANY($1) AND status='inactive'`, r.schema), sourceIDs).Scan(&inactiveSourceCount); err != nil {
		return summary, err
	}
	if replacementCount == len(manifest.Entries) && inactiveSourceCount == len(manifest.Entries) {
		if err := r.validatePR606AppliedState(ctx, tx, manifest, &summary); err != nil {
			return summary, err
		}
		summary.State = "applied"
		return summary, nil
	}
	if replacementCount != 0 || inactiveSourceCount != 0 {
		return summary, fmt.Errorf("PR-606 partial state detected: replacements=%d inactive_sources=%d; aborting", replacementCount, inactiveSourceCount)
	}
	if err := r.validatePR606PendingState(ctx, tx, manifest, lock, &summary); err != nil {
		return summary, err
	}
	summary.State = "ready"
	summary.NewMaterialCount, summary.ReusedMaterialCount, summary.ReplacementBomCount = 30, 1, 31
	summary.Message = "membership, source versions, recipes, categories, materials and dependencies match the locked manifest"
	return summary, nil
}

func (r Repository) validatePR606PendingState(ctx context.Context, tx pgx.Tx, manifest PR606CutoverManifest, lock bool, summary *PR606CutoverSummary) error {
	lockSuffix := ""
	if lock {
		lockSuffix = " FOR UPDATE OF pb,v"
	}
	actualMembers := make([]int64, 0)
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT pb.id
		FROM %s.production_boms pb
		JOIN %s.business_group_assignments a ON lower(a.usage_key)='production_bom' AND lower(a.object_key)='production_bom' AND a.object_id=pb.id AND a.object_ref=''
		WHERE a.group_id=$1 AND pb.status='active'
		ORDER BY pb.id
	`, r.schema, r.schema), manifest.ProductionGroupID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		actualMembers = append(actualMembers, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	expectedMembers := make([]int64, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		expectedMembers = append(expectedMembers, entry.SourceBomID)
	}
	sort.Slice(expectedMembers, func(i, j int) bool { return expectedMembers[i] < expectedMembers[j] })
	if fmt.Sprint(actualMembers) != fmt.Sprint(expectedMembers) {
		return fmt.Errorf("production group membership drift: got %v want %v", actualMembers, expectedMembers)
	}
	for _, entry := range manifest.Entries {
		var name, bomStatus, versionNo, versionStatus, hash string
		var productID, groupID, groupItemID int64
		var itemCount int
		query := fmt.Sprintf(`
			SELECT pb.name,pb.status,COALESCE(pb.output_product_id,0),a.group_id,a.group_item_id,
			       v.version_no,v.status,
			       (SELECT COUNT(*) FROM %s.production_bom_version_items i WHERE i.version_id=v.id),
			       md5(COALESCE((SELECT string_agg(concat_ws('|',i.material_id,i.component_type,i.component_product_id,i.component_bom_spec_id,i.component_spec_g,i.consume_unit,i.qty_per_unit,i.ratio_pct,i.material_loss_rate),';' ORDER BY i.id) FROM %s.production_bom_version_items i WHERE i.version_id=v.id),'') || concat_ws('|',v.output_qty,v.output_unit,v.material_loss_rate,v.process_route_id))
			FROM %s.production_boms pb
			JOIN %s.production_bom_versions v ON v.bom_id=pb.id AND v.id=$2
			JOIN %s.business_group_assignments a ON lower(a.usage_key)='production_bom' AND lower(a.object_key)='production_bom' AND a.object_id=pb.id AND a.object_ref=''
			WHERE pb.id=$1%s
		`, r.schema, r.schema, r.schema, r.schema, r.schema, lockSuffix)
		if err := tx.QueryRow(ctx, query, entry.SourceBomID, entry.SourceVersionID).Scan(&name, &bomStatus, &productID, &groupID, &groupItemID, &versionNo, &versionStatus, &itemCount, &hash); err != nil {
			return fmt.Errorf("source BOM/version missing for %s: %w", entry.SourceName, err)
		}
		if name != entry.SourceName || bomStatus != "active" || productID != entry.SourceProductID || groupID != manifest.ProductionGroupID || groupItemID != entry.GroupItemID || versionNo != entry.SourceVersionNo || versionStatus != entry.SourceVersionStatus || hash != entry.SourceRecipeHash {
			return fmt.Errorf("source drift for %s: name=%q status=%s product=%d group=%d/%d version=%s/%s hash=%s", entry.SourceName, name, bomStatus, productID, groupID, groupItemID, versionNo, versionStatus, hash)
		}
		if entry.Publish && entry.RecipeOverride != "initial-screenshot" && itemCount == 0 {
			return fmt.Errorf("published replacement source %s has empty recipe", entry.SourceName)
		}
		if !entry.Publish && itemCount != 0 {
			return fmt.Errorf("draft-only source %s unexpectedly has %d components", entry.SourceName, itemCount)
		}
	}
	if err := r.validatePR606MaterialTargets(ctx, tx, manifest, lock); err != nil {
		return err
	}
	dependencyCount, err := r.countPR606OpenDependencies(ctx, tx, manifest)
	if err != nil {
		return err
	}
	summary.DependencyCount = dependencyCount
	if dependencyCount != 0 {
		return fmt.Errorf("PR-606 source versions gained %d unfinished production references; aborting", dependencyCount)
	}
	return nil
}

func (r Repository) validatePR606MaterialTargets(ctx context.Context, tx pgx.Tx, manifest PR606CutoverManifest, lock bool) error {
	var validCategory bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s.business_groups g
			JOIN %s.business_group_items i ON i.group_id=g.id
			JOIN %s.business_group_usages u ON u.group_id=g.id
			WHERE g.id=$1 AND i.id=$2 AND g.active=true AND i.active=true AND u.active=true AND lower(u.usage_key)='material_catalog'
		)
	`, r.schema, r.schema, r.schema), manifest.MaterialGroupID, manifest.MaterialGroupItemID).Scan(&validCategory); err != nil {
		return err
	}
	if !validCategory {
		return fmt.Errorf("material target category drift")
	}
	for _, replacement := range manifest.ComponentMaterialReplacements {
		var sourceName, sourceUnit, targetName, targetUnit string
		var sourceDeprecated, targetDeprecated *string
		query := fmt.Sprintf(`
			SELECT source.name,source.unit,source.deprecated_at::text,target.name,target.unit,target.deprecated_at::text
			FROM %s.materials source
			JOIN %s.materials target ON target.id=$2
			WHERE source.id=$1
		`, r.schema, r.schema)
		if lock {
			query += " FOR UPDATE OF source,target"
		}
		if err := tx.QueryRow(ctx, query, replacement.SourceMaterialID, replacement.TargetMaterialID).Scan(&sourceName, &sourceUnit, &sourceDeprecated, &targetName, &targetUnit, &targetDeprecated); err != nil {
			return fmt.Errorf("component material replacement missing for %d->%d: %w", replacement.SourceMaterialID, replacement.TargetMaterialID, err)
		}
		if sourceName != replacement.SourceName || sourceUnit != "kg" || sourceDeprecated == nil || targetName != replacement.TargetName || targetUnit != "kg" || targetDeprecated != nil {
			return fmt.Errorf("component material replacement drift for %d->%d", replacement.SourceMaterialID, replacement.TargetMaterialID)
		}
	}
	for _, entry := range manifest.Entries {
		name := entry.SourceName + "-半成品"
		if entry.ExistingMaterialID > 0 {
			var code, storedName, unit, costUnit string
			var semi bool
			query := fmt.Sprintf(`SELECT code,name,unit,cost_unit,COALESCE(is_semi_finished,false) FROM %s.materials WHERE id=$1 AND deprecated_at IS NULL`, r.schema)
			if lock {
				query += " FOR UPDATE"
			}
			if err := tx.QueryRow(ctx, query, entry.ExistingMaterialID).Scan(&code, &storedName, &unit, &costUnit, &semi); err != nil {
				return fmt.Errorf("existing material %d missing: %w", entry.ExistingMaterialID, err)
			}
			if code != "MAT-000106" || storedName != name || unit != "kg" || costUnit != "kg" || !semi {
				return fmt.Errorf("existing material %d drift", entry.ExistingMaterialID)
			}
			continue
		}
		var count int
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.materials WHERE (name=$1 OR code=$2) AND deprecated_at IS NULL`, r.schema), name, fmt.Sprintf("PR606-SOURCE-%d", entry.SourceBomID)).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("material target conflict for %s", name)
		}
	}
	for materialID, expectedName := range map[int64]string{56: "黄波旁水洗", 85: "孟连水洗5T批次", 10: "生豆：耶加雪菲G2", 47: "生豆-巴布亚之光-石光"} {
		var name string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.materials WHERE id=$1 AND deprecated_at IS NULL`, r.schema), materialID).Scan(&name); err != nil || name != expectedName {
			return fmt.Errorf("初晓配方物料 %d drift: got %q", materialID, name)
		}
	}
	return nil
}

func (r Repository) countPR606OpenDependencies(ctx context.Context, tx pgx.Tx, manifest PR606CutoverManifest) (int, error) {
	versionIDs := make([]int64, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		versionIDs = append(versionIDs, entry.SourceVersionID)
	}
	var planCount, workOrderCount int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.production_plan_items item
		JOIN %s.production_plans plan ON plan.id=item.production_plan_id
		WHERE item.bom_version_id=ANY($1) AND lower(COALESCE(plan.status,'')) NOT IN ('completed','cancelled','canceled')
	`, r.schema, r.schema), versionIDs).Scan(&planCount); err != nil {
		return 0, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.work_orders WHERE bom_version_id=ANY($1) AND lower(COALESCE(status,'')) NOT IN ('completed','cancelled','canceled')`, r.schema), versionIDs).Scan(&workOrderCount); err != nil {
		return 0, err
	}
	return planCount + workOrderCount, nil
}

func (r Repository) validatePR606AppliedState(ctx context.Context, tx pgx.Tx, manifest PR606CutoverManifest, summary *PR606CutoverSummary) error {
	results := make([]PR606CutoverEntryResult, 0, len(manifest.Entries))
	newMaterials, reused, published, drafts := 0, 0, 0, 0
	for _, entry := range manifest.Entries {
		var bomID, versionID, materialID int64
		var bomStatus, versionStatus, materialCode, materialName, unit, costUnit string
		var semi bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT pb.id,pb.status,pb.output_material_id,v.id,v.status,m.code,m.name,m.unit,m.cost_unit,COALESCE(m.is_semi_finished,false)
			FROM %s.production_boms pb
			JOIN %s.production_bom_versions v ON v.bom_id=pb.id AND v.version_no='V001'
			JOIN %s.materials m ON m.id=pb.output_material_id AND m.deprecated_at IS NULL
			WHERE pb.source_bom_id=$1 AND pb.source_bom_version_id=$2 AND pb.output_type='material'
		`, r.schema, r.schema, r.schema), entry.SourceBomID, entry.SourceVersionID).Scan(&bomID, &bomStatus, &materialID, &versionID, &versionStatus, &materialCode, &materialName, &unit, &costUnit, &semi); err != nil {
			return fmt.Errorf("applied replacement missing for %s: %w", entry.SourceName, err)
		}
		expectedVersionStatus := "draft"
		if entry.Publish {
			expectedVersionStatus = "published"
			published++
		} else {
			drafts++
		}
		if bomStatus != "active" || versionStatus != expectedVersionStatus || materialName != entry.SourceName+"-半成品" || unit != "kg" || costUnit != "kg" || !semi {
			return fmt.Errorf("applied replacement drift for %s", entry.SourceName)
		}
		if entry.ExistingMaterialID > 0 {
			if materialID != entry.ExistingMaterialID {
				return fmt.Errorf("reused material drift for %s", entry.SourceName)
			}
			reused++
		} else {
			newMaterials++
		}
		if entry.Publish {
			var bound bool
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_bom_output_bindings WHERE output_type='material' AND output_id=$1 AND bom_id=$2 AND bom_version_id=$3 AND is_default=true)`, r.schema), materialID, bomID, versionID).Scan(&bound); err != nil || !bound {
				return fmt.Errorf("default material BOM binding drift for %s", entry.SourceName)
			}
		}
		results = append(results, PR606CutoverEntryResult{SourceBomID: entry.SourceBomID, SourceName: entry.SourceName, SourceVersionID: entry.SourceVersionID, MaterialID: materialID, MaterialCode: materialCode, ReplacementBomID: bomID, ReplacementVersionID: versionID, Status: versionStatus})
	}
	var staleBindings, staleConfigs int
	sourceBOMIDs, productIDs := pr606IDs(manifest)
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT COUNT(*) FROM %s.product_production_bom_bindings WHERE bom_id=ANY($1) OR product_id=ANY($2)) + (SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='product' AND (bom_id=ANY($1) OR output_id=ANY($2)))`, r.schema, r.schema), sourceBOMIDs, productIDs).Scan(&staleBindings); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.product_production_configs WHERE product_id=ANY($1) AND (production_bom_id<>0 OR production_bom_version_id<>0)`, r.schema), productIDs).Scan(&staleConfigs); err != nil {
		return err
	}
	if staleBindings != 0 || staleConfigs != 0 {
		return fmt.Errorf("old product manufacturing bindings remain: bindings=%d configs=%d", staleBindings, staleConfigs)
	}
	summary.Entries, summary.NewMaterialCount, summary.ReusedMaterialCount = results, newMaterials, reused
	summary.ReplacementBomCount, summary.PublishedCount, summary.DraftCount = len(results), published, drafts
	return nil
}

func pr606IDs(manifest PR606CutoverManifest) ([]int64, []int64) {
	bomIDs := make([]int64, 0, len(manifest.Entries))
	productIDs := make([]int64, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		bomIDs = append(bomIDs, entry.SourceBomID)
		productIDs = append(productIDs, entry.SourceProductID)
	}
	return bomIDs, productIDs
}

func pr606FixedScale(outputQty float64, outputUnit string) float64 {
	if outputQty <= 0 {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(outputUnit)) {
	case "lb", "磅":
		return 1 / (outputQty * 0.45359237)
	case "g", "克":
		return 1000 / outputQty
	case "kg", "千克", "公斤":
		return 1 / outputQty
	default:
		return 1 / outputQty
	}
}

func (r Repository) capturePR606RollbackSnapshot(ctx context.Context, tx pgx.Tx, manifest PR606CutoverManifest) (pr606RollbackSnapshot, error) {
	var snapshot pr606RollbackSnapshot
	bomIDs, productIDs := pr606IDs(manifest)
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT product_id,bom_id,bom_version_id FROM %s.product_production_bom_bindings WHERE bom_id=ANY($1) OR product_id=ANY($2) ORDER BY product_id`, r.schema), bomIDs, productIDs)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var row pr606ProductBinding
		if err := rows.Scan(&row.ProductID, &row.BomID, &row.VersionID); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.ProductBindings = append(snapshot.ProductBindings, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snapshot, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT output_type,output_id,bom_id,bom_version_id,is_default FROM %s.production_bom_output_bindings WHERE output_type='product' AND (bom_id=ANY($1) OR output_id=ANY($2)) ORDER BY output_id`, r.schema), bomIDs, productIDs)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var row pr606OutputBinding
		if err := rows.Scan(&row.OutputType, &row.OutputID, &row.BomID, &row.VersionID, &row.IsDefault); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.OutputBindings = append(snapshot.OutputBindings, row)
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
		var row pr606ProductConfig
		if err := rows.Scan(&row.ProductID, &row.BomID, &row.VersionID); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.ProductConfigs = append(snapshot.ProductConfigs, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snapshot, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT group_id,group_item_id FROM %s.business_group_assignments WHERE lower(usage_key)='material_catalog' AND lower(object_key)='material' AND object_id=106 AND object_ref='' ORDER BY id`, r.schema))
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var row pr606MaterialAssignment
		if err := rows.Scan(&row.GroupID, &row.GroupItemID); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.InitialMaterialAssignments = append(snapshot.InitialMaterialAssignments, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snapshot, err
	}
	rows.Close()
	return snapshot, nil
}

func (r Repository) applyPR606Entry(ctx context.Context, tx pgx.Tx, manifest PR606CutoverManifest, entry PR606CutoverManifestEntry, actor string) (PR606CutoverEntryResult, error) {
	materialID := entry.ExistingMaterialID
	materialName := entry.SourceName + "-半成品"
	if materialID > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET name=$2,kind='other',is_semi_finished=true,unit='kg',cost_unit='kg',purchase_price=0,updated_at=now() WHERE id=$1 AND deprecated_at IS NULL`, r.schema), materialID, materialName); err != nil {
			return PR606CutoverEntryResult{}, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "material", &materialID, "update_for_bom_cutover", postgresinfra.StrPtr("is_semi_finished"), nil, postgresinfra.StrPtr("true"), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "source_bom_id": entry.SourceBomID, "material_name": materialName}); err != nil {
			return PR606CutoverEntryResult{}, err
		}
	} else {
		tempCode := fmt.Sprintf("PR606-SOURCE-%d", entry.SourceBomID)
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,is_semi_finished,unit,cost_unit,purchase_price,sale_price,updated_at) VALUES($1,$2,'other',true,'kg','kg',0,0,now()) RETURNING id`, r.schema), tempCode, materialName).Scan(&materialID); err != nil {
			return PR606CutoverEntryResult{}, err
		}
		code := fmt.Sprintf("MAT-%06d", materialID)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET code=$1 WHERE id=$2`, r.schema), code, materialID); err != nil {
			return PR606CutoverEntryResult{}, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "material", &materialID, "create", postgresinfra.StrPtr("code"), nil, postgresinfra.StrPtr(code), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "source_bom_id": entry.SourceBomID, "name": materialName, "supply_mode": "manufacture", "unit": "kg", "cost_unit": "kg", "purchase_price": 0}); err != nil {
			return PR606CutoverEntryResult{}, err
		}
	}
	if err := r.savePR606MaterialAssignment(ctx, tx, manifest, materialID, actor); err != nil {
		return PR606CutoverEntryResult{}, err
	}
	var lossRate, sourceOutputQty float64
	var sourceOutputUnit, attrsSchema, attrs string
	var processRouteID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(material_loss_rate,0)::float8,COALESCE(output_qty,1)::float8,COALESCE(NULLIF(output_unit,''),'kg'),COALESCE(special_attrs_schema_json::text,'[]'),COALESCE(special_attrs_json::text,'{}'),COALESCE(process_route_id,0) FROM %s.production_bom_versions WHERE id=$1 AND bom_id=$2`, r.schema), entry.SourceVersionID, entry.SourceBomID).Scan(&lossRate, &sourceOutputQty, &sourceOutputUnit, &attrsSchema, &attrs, &processRouteID); err != nil {
		return PR606CutoverEntryResult{}, err
	}
	if entry.RecipeOverride == "initial-screenshot" {
		lossRate = 0.195
	}
	var newBomID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_boms(code,name,output_type,output_product_id,output_material_id,status,source_bom_id,source_bom_version_id,created_by,updated_by) VALUES($1,$2,'material',0,$3,'active',$4,$5,$6,$6) RETURNING id`, r.schema), fmt.Sprintf("PENDING-%d", entry.SourceBomID), entry.SourceName, materialID, entry.SourceBomID, entry.SourceVersionID, actor).Scan(&newBomID); err != nil {
		return PR606CutoverEntryResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET code=$1 WHERE id=$2`, r.schema), fmt.Sprintf("BOM-%06d", newBomID), newBomID); err != nil {
		return PR606CutoverEntryResult{}, err
	}
	var newVersionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_versions(bom_id,version_no,status,yield_rate,output_qty,output_unit,material_loss_rate,note,special_attrs_schema_json,special_attrs_json,process_route_id,created_at,created_by) VALUES($1,'V001','draft',1,1,'kg',$2,'PR-606 半成品替代草稿',$3::jsonb,$4::jsonb,$5,now(),$6) RETURNING id`, r.schema), newBomID, lossRate, attrsSchema, attrs, processRouteID, actor).Scan(&newVersionID); err != nil {
		return PR606CutoverEntryResult{}, err
	}
	if entry.RecipeOverride == "initial-screenshot" {
		for _, item := range pr606InitialRecipe() {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_items(version_id,material_id,component_type,consume_unit,ratio_pct,material_loss_rate,unit_cost_snapshot) VALUES($1,$2,'material','ratio_pct',$3,$4,COALESCE((SELECT purchase_price FROM %s.materials WHERE id=$2),0))`, r.schema, r.schema), newVersionID, item.id, item.ratio, lossRate); err != nil {
				return PR606CutoverEntryResult{}, err
			}
		}
	} else {
		scale := pr606FixedScale(sourceOutputQty, sourceOutputUnit)
		replacementSources, replacementTargets := pr606ComponentMaterialReplacementIDs(manifest)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.production_bom_version_items(version_id,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot)
			SELECT $1,COALESCE(replacement.target_id,i.material_id),i.component_type,i.component_product_id,i.component_bom_spec_id,i.component_spec_g,i.consume_unit,
			       CASE WHEN i.consume_unit='ratio_pct' THEN i.qty_per_unit ELSE i.qty_per_unit*$3 END,
			       i.ratio_pct,CASE WHEN i.component_type='material' AND i.consume_unit='ratio_pct' AND $4>0 THEN $4 ELSE 0 END,i.unit_cost_snapshot
			FROM %s.production_bom_version_items i
			LEFT JOIN unnest($5::bigint[],$6::bigint[]) replacement(source_id,target_id) ON replacement.source_id=i.material_id
			WHERE i.version_id=$2 AND COALESCE(i.variant_id,0)=0
			ORDER BY i.id
		`, r.schema, r.schema), newVersionID, entry.SourceVersionID, scale, lossRate, replacementSources, replacementTargets); err != nil {
			return PR606CutoverEntryResult{}, err
		}
	}
	if err := saveBusinessGroupAssignmentForProductionBomTx(ctx, tx, r.schema, actor, newBomID, manifest.ProductionGroupID, entry.GroupItemID); err != nil {
		return PR606CutoverEntryResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "production_bom", &newBomID, "create_replacement_draft", postgresinfra.StrPtr("source_bom_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", entry.SourceBomID)), postgresinfra.StrPtr(fmt.Sprintf("%d", newBomID)), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "source_bom_id": entry.SourceBomID, "source_bom_version_id": entry.SourceVersionID, "target_bom_version_id": newVersionID, "output_type": "material", "output_material_id": materialID}); err != nil {
		return PR606CutoverEntryResult{}, err
	}
	status := "draft"
	if entry.Publish {
		cmd := bomapp.PublishProductionBomVersionCommand{VersionID: newVersionID, Actor: actor}
		if err := lockProductionBomPublishTargetsTx(ctx, tx, r.schema, newVersionID); err != nil {
			return PR606CutoverEntryResult{}, err
		}
		if err := r.validateProductionBomVersionForPublish(ctx, tx, cmd); err != nil {
			return PR606CutoverEntryResult{}, err
		}
		if err := r.publishProductionBomVersionTx(ctx, tx, cmd); err != nil {
			return PR606CutoverEntryResult{}, err
		}
		status = "published"
	}
	var materialCode string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT code FROM %s.materials WHERE id=$1`, r.schema), materialID).Scan(&materialCode); err != nil {
		return PR606CutoverEntryResult{}, err
	}
	return PR606CutoverEntryResult{SourceBomID: entry.SourceBomID, SourceName: entry.SourceName, SourceVersionID: entry.SourceVersionID, MaterialID: materialID, MaterialCode: materialCode, ReplacementBomID: newBomID, ReplacementVersionID: newVersionID, Status: status}, nil
}

type pr606InitialRecipeItem struct {
	id    int64
	ratio float64
}

func pr606InitialRecipe() []pr606InitialRecipeItem {
	return []pr606InitialRecipeItem{{56, 50}, {85, 15}, {10, 20}, {47, 15}}
}

func pr606ComponentMaterialReplacementIDs(manifest PR606CutoverManifest) ([]int64, []int64) {
	sources := make([]int64, 0, len(manifest.ComponentMaterialReplacements))
	targets := make([]int64, 0, len(manifest.ComponentMaterialReplacements))
	for _, replacement := range manifest.ComponentMaterialReplacements {
		sources = append(sources, replacement.SourceMaterialID)
		targets = append(targets, replacement.TargetMaterialID)
	}
	return sources, targets
}

func (r Repository) savePR606MaterialAssignment(ctx context.Context, tx pgx.Tx, manifest PR606CutoverManifest, materialID int64, actor string) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_group_assignments WHERE lower(usage_key)='material_catalog' AND lower(object_key)='material' AND object_id=$1 AND object_ref=''`, r.schema), materialID); err != nil {
		return err
	}
	var assignmentID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.business_group_assignments(group_id,group_item_id,usage_key,object_key,object_id,object_ref,sort_order,created_by,updated_by) VALUES($1,$2,'material_catalog','material',$3,'',100,$4,$4) RETURNING id`, r.schema), manifest.MaterialGroupID, manifest.MaterialGroupItemID, materialID, actor).Scan(&assignmentID); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "business_group_assignment", &assignmentID, "save_business_group_assignment", postgresinfra.StrPtr("material_catalog"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", manifest.MaterialGroupItemID)), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "material_id": materialID, "group_id": manifest.MaterialGroupID, "group_item_id": manifest.MaterialGroupItemID, "usage_key": "material_catalog", "object_key": "material"})
}

func (r Repository) cutOverPR606SourceBindings(ctx context.Context, tx pgx.Tx, manifest PR606CutoverManifest, actor string) error {
	bomIDs, productIDs := pr606IDs(manifest)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_production_bom_bindings WHERE bom_id=ANY($1) OR product_id=ANY($2)`, r.schema), bomIDs, productIDs); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_output_bindings WHERE output_type='product' AND (bom_id=ANY($1) OR output_id=ANY($2))`, r.schema), bomIDs, productIDs); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_production_configs SET production_bom_id=0,production_bom_version_id=0,updated_at=now(),updated_by=$2 WHERE product_id=ANY($1) OR production_bom_id=ANY($3)`, r.schema), productIDs, actor, bomIDs); err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET status='inactive',updated_at=now(),updated_by=$2 WHERE id=$1`, r.schema), entry.SourceBomID, actor); err != nil {
			return err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "production_bom", &entry.SourceBomID, "deactivate_after_replacement", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("active"), postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "source_bom_id": entry.SourceBomID, "source_version_id": entry.SourceVersionID, "source_product_id": entry.SourceProductID}); err != nil {
			return err
		}
		productID := entry.SourceProductID
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product", &productID, "clear_direct_manufacturing_bom", postgresinfra.StrPtr("default_production_bom_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", entry.SourceBomID)), postgresinfra.StrPtr("0"), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "source_bom_id": entry.SourceBomID, "source_version_id": entry.SourceVersionID}); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) RollbackPR606Cutover(ctx context.Context, actor string) (PR606CutoverSummary, error) {
	manifest, err := LoadPR606ProductionManifest()
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":"+pr606CutoverLockKey); err != nil {
		return PR606CutoverSummary{}, err
	}
	bomIDs, _ := pr606IDs(manifest)
	var activeSources, inactiveReplacements int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE id=ANY($1) AND status='active'`, r.schema), bomIDs).Scan(&activeSources); err != nil {
		return PR606CutoverSummary{}, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE source_bom_id=ANY($1) AND output_type='material' AND status='inactive'`, r.schema), bomIDs).Scan(&inactiveReplacements); err != nil {
		return PR606CutoverSummary{}, err
	}
	if activeSources == 31 && inactiveReplacements == 31 {
		return PR606CutoverSummary{ManifestID: manifest.ManifestID, Mode: "rollback", State: "rolled_back", SourceCount: 31, ReplacementBomCount: 31, Message: "already rolled back; no changes written"}, nil
	}
	preview, err := r.inspectPR606Cutover(ctx, tx, manifest, true)
	if err != nil {
		return PR606CutoverSummary{}, err
	}
	if preview.State != "applied" {
		return PR606CutoverSummary{}, fmt.Errorf("PR-606 rollback requires fully applied state")
	}
	var snapshotJSON []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT meta->'rollback_snapshot' FROM %s.audit_logs WHERE entity_type='bom_semi_finished_cutover' AND action='apply' AND meta->>'manifest_id'=$1 ORDER BY id DESC LIMIT 1`, r.schema), manifest.ManifestID).Scan(&snapshotJSON); err != nil {
		return PR606CutoverSummary{}, fmt.Errorf("rollback snapshot not found: %w", err)
	}
	var snapshot pr606RollbackSnapshot
	if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
		return PR606CutoverSummary{}, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system-pr606-rollback"
	}
	for _, entry := range manifest.Entries {
		var replacementBomID, materialID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,output_material_id FROM %s.production_boms WHERE source_bom_id=$1 AND source_bom_version_id=$2 AND output_type='material' FOR UPDATE`, r.schema), entry.SourceBomID, entry.SourceVersionID).Scan(&replacementBomID, &materialID); err != nil {
			return PR606CutoverSummary{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET status='inactive',updated_at=now(),updated_by=$2 WHERE id=$1`, r.schema), replacementBomID, actor); err != nil {
			return PR606CutoverSummary{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_output_bindings WHERE output_type='material' AND output_id=$1 AND bom_id=$2`, r.schema), materialID, replacementBomID); err != nil {
			return PR606CutoverSummary{}, err
		}
		if entry.ExistingMaterialID == 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET deprecated_at=COALESCE(deprecated_at,now()),updated_at=now() WHERE id=$1`, r.schema), materialID); err != nil {
				return PR606CutoverSummary{}, err
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET status='active',updated_at=now(),updated_by=$2 WHERE id=$1`, r.schema), entry.SourceBomID, actor); err != nil {
			return PR606CutoverSummary{}, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "production_bom", &replacementBomID, "deactivate_replacement_on_rollback", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("active"), postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "source_bom_id": entry.SourceBomID, "material_id": materialID}); err != nil {
			return PR606CutoverSummary{}, err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_group_assignments WHERE lower(usage_key)='material_catalog' AND lower(object_key)='material' AND object_id=106 AND object_ref=''`, r.schema)); err != nil {
		return PR606CutoverSummary{}, err
	}
	for _, assignment := range snapshot.InitialMaterialAssignments {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.business_group_assignments(group_id,group_item_id,usage_key,object_key,object_id,object_ref,sort_order,created_by,updated_by) VALUES($1,$2,'material_catalog','material',106,'',100,$3,$3)`, r.schema), assignment.GroupID, assignment.GroupItemID, actor); err != nil {
			return PR606CutoverSummary{}, err
		}
	}
	for _, row := range snapshot.ProductBindings {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_at,bound_by) VALUES($1,$2,$3,now(),$4) ON CONFLICT(product_id) DO UPDATE SET bom_id=excluded.bom_id,bom_version_id=excluded.bom_version_id,bound_at=excluded.bound_at,bound_by=excluded.bound_by`, r.schema), row.ProductID, row.BomID, row.VersionID, actor); err != nil {
			return PR606CutoverSummary{}, err
		}
	}
	for _, row := range snapshot.OutputBindings {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_at,updated_by) VALUES($1,$2,$3,$4,$5,now(),$6) ON CONFLICT(output_type,output_id) DO UPDATE SET bom_id=excluded.bom_id,bom_version_id=excluded.bom_version_id,is_default=excluded.is_default,updated_at=excluded.updated_at,updated_by=excluded.updated_by`, r.schema), row.OutputType, row.OutputID, row.BomID, row.VersionID, row.IsDefault, actor); err != nil {
			return PR606CutoverSummary{}, err
		}
	}
	for _, row := range snapshot.ProductConfigs {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_production_configs SET production_bom_id=$2,production_bom_version_id=$3,updated_at=now(),updated_by=$4 WHERE product_id=$1`, r.schema), row.ProductID, row.BomID, row.VersionID, actor); err != nil {
			return PR606CutoverSummary{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "bom_semi_finished_cutover", nil, "rollback", postgresinfra.StrPtr("manifest_id"), postgresinfra.StrPtr(manifest.ManifestID), postgresinfra.StrPtr(manifest.ManifestID), postgresinfra.AuditMeta{"manifest_id": manifest.ManifestID, "source_count": 31, "replacement_bom_count": 31, "new_materials_deprecated": 30}); err != nil {
		return PR606CutoverSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PR606CutoverSummary{}, err
	}
	return PR606CutoverSummary{ManifestID: manifest.ManifestID, Mode: "rollback", State: "rolled_back", SourceCount: 31, NewMaterialCount: 30, ReusedMaterialCount: 1, ReplacementBomCount: 31, PublishedCount: 26, DraftCount: 5, Message: "PR-606 rollback applied without deleting history"}, nil
}
