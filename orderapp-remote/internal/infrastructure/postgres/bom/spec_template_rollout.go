package bom

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	bomapp "orderapp/internal/application/bom"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) rolloutTemplate(ctx context.Context, tx pgx.Tx, id int64, lock bool) (bomapp.TemplateRolloutPreview, error) {
	out := bomapp.TemplateRolloutPreview{TemplateVersionID: id, Rows: []bomapp.TemplateRolloutRow{}}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT template_id,version_no FROM %s.production_bom_spec_template_versions WHERE id=$1`, r.schema), id).Scan(&out.TemplateID, &out.TemplateVersionNo); err != nil {
		return out, fmt.Errorf("规格模板版本不存在")
	}
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	var active bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.production_bom_spec_templates WHERE id=$1`, r.schema)+suffix, out.TemplateID).Scan(&active); err != nil || !active {
		return out, fmt.Errorf("规格模板已停用或不存在")
	}
	var current int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_spec_template_versions WHERE template_id=$1 AND status='published' ORDER BY id DESC LIMIT 1`, r.schema), out.TemplateID).Scan(&current); err != nil || current != id {
		return out, fmt.Errorf("模板已更新，请刷新后选择最新已发布版本")
	}
	return out, nil
}

func (r Repository) PreviewSpecTemplateRollout(ctx context.Context, id int64) (bomapp.TemplateRolloutPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return bomapp.TemplateRolloutPreview{}, err
	}
	defer tx.Rollback(ctx)
	out, err := r.rolloutTemplate(ctx, tx, id, false)
	if err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT b.id FROM %[1]s.production_boms b JOIN %[1]s.production_bom_versions v ON v.bom_id=b.id AND v.status='published' LEFT JOIN %[1]s.production_bom_spec_template_versions t ON t.id=v.source_spec_template_version_id WHERE b.status='active' AND b.specification_mode='spec_group' AND (t.template_id=$1 OR (COALESCE(v.source_spec_template_version_id,0)=0 AND EXISTS(SELECT 1 FROM %[1]s.production_bom_version_variants x JOIN %[1]s.production_bom_specs s ON s.id=x.bom_spec_id WHERE x.version_id=v.id AND s.source_spec_template_id=$1))) ORDER BY b.id`, r.schema), out.TemplateID)
	if err != nil {
		return out, err
	}
	ids := []int64{}
	for rows.Next() {
		var bid int64
		if err = rows.Scan(&bid); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, bid)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, bid := range ids {
		row, _, _, e := r.prepareTemplateRollout(ctx, tx, out, bid)
		if e != nil {
			return out, e
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

func (r Repository) currentRolloutSpecs(ctx context.Context, tx pgx.Tx, versionID, templateID int64) ([]bomapp.TemplateRolloutSpec, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT v.id,v.bom_spec_id,s.spec_key,s.barcode,v.spec_name_snapshot,v.inventory_unit,v.is_default,v.sort_order,v.material_loss_rate::float8,v.process_route_id,v.logistics_weight_g::float8,CASE WHEN s.source_spec_template_id=$2 THEN s.source_spec_template_key WHEN s.source_spec_template_id=0 AND EXISTS(SELECT 1 FROM %s.production_bom_spec_template_variants legacy JOIN %s.production_bom_versions source ON source.id=v.version_id WHERE legacy.version_id=source.source_spec_template_version_id AND lower(legacy.spec_key)=lower(s.spec_key)) THEN s.spec_key ELSE 'local:'||s.id END FROM %s.production_bom_version_variants v JOIN %s.production_bom_specs s ON s.id=v.bom_spec_id WHERE v.version_id=$1 ORDER BY v.sort_order,v.id`, r.schema, r.schema, r.schema, r.schema), versionID, templateID)
	if err != nil {
		return nil, err
	}
	out := []bomapp.TemplateRolloutSpec{}
	for rows.Next() {
		var row bomapp.TemplateRolloutSpec
		v := &row.Variant
		if err = rows.Scan(&v.BomVariantID, &v.BomSpecID, &v.SpecKey, &v.Barcode, &v.Name, &v.InventoryUnit, &v.IsDefault, &v.SortOrder, &v.MaterialLossRate, &v.ProcessRouteID, &v.LogisticsWeightG, &row.Key); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		var raw []byte
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(i) ORDER BY i.id),'[]') FROM %s.production_bom_version_items i WHERE version_id=$1 AND variant_id=$2`, r.schema), versionID, out[i].Variant.BomVariantID).Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &out[i].Variant.Items); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func expandRolloutTemplate(rows []bomapp.ProductionBomSpecTemplateVariant, main bomapp.ProductionBomMainInputComponent, unit string) []bomapp.TemplateRolloutSpec {
	out := []bomapp.TemplateRolloutSpec{}
	for _, row := range rows {
		v := bomapp.ProductionBomDraftVariant{SpecKey: row.SpecKey, Name: row.Name, InventoryUnit: row.InventoryUnit, IsDefault: row.IsDefault, SortOrder: row.SortOrder, MaterialLossRate: row.MaterialLossRate, ProcessRouteID: row.ProcessRouteID, LogisticsWeightG: row.LogisticsWeightG, Items: []bomapp.ProductionBomDraftItem{}}
		for _, item := range row.Items {
			it := item.ProductionBomDraftItem
			if item.IsMainInput {
				it.MaterialID = main.MaterialID
				it.ComponentType = main.ComponentType
				it.ComponentProductID = main.ComponentProductID
				it.ComponentBomSpecID = main.ComponentBomSpecID
				if it.ConsumeUnit == "main_input_unit" {
					it.ConsumeUnit = unit
				}
			}
			it.MaterialLossRate = v.MaterialLossRate
			v.Items = append(v.Items, it)
		}
		out = append(out, bomapp.TemplateRolloutSpec{Key: row.SpecKey, Variant: v})
	}
	return out
}

func (r Repository) prepareTemplateRollout(ctx context.Context, tx pgx.Tx, target bomapp.TemplateRolloutPreview, bomID int64) (bomapp.TemplateRolloutRow, bomapp.TemplateRolloutMerge, bomapp.ProductionBomMainInputComponent, error) {
	row := bomapp.TemplateRolloutRow{BomID: bomID, Status: "blocked", Changes: []bomapp.TemplateRolloutChange{}}
	merge := bomapp.TemplateRolloutMerge{}
	main := bomapp.ProductionBomMainInputComponent{}
	var state, mode, oldState string
	var sourceTemplate int64
	var draft, wasPublished bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.name,COALESCE(p.name,m.name,''),b.status,b.specification_mode,v.id,v.version_no,COALESCE(v.source_spec_template_version_id,0),COALESCE(t.version_no,''),COALESCE(t.template_id,0),COALESCE(t.status,''),COALESCE(t.published_at IS NOT NULL,false),EXISTS(SELECT 1 FROM %[1]s.production_bom_versions d WHERE d.bom_id=b.id AND d.status='draft'),COALESCE(NULLIF(v.main_input_component_type,''),'material'),COALESCE(v.main_input_material_id,0),COALESCE(v.main_input_product_id,0),COALESCE(v.main_input_bom_spec_id,0) FROM %[1]s.production_boms b JOIN LATERAL(SELECT * FROM %[1]s.production_bom_versions WHERE bom_id=b.id AND status='published' ORDER BY published_at DESC NULLS LAST,id DESC LIMIT 1)v ON true LEFT JOIN %[1]s.production_bom_spec_template_versions t ON t.id=v.source_spec_template_version_id LEFT JOIN %[1]s.products p ON b.output_type='product' AND p.id=b.output_product_id LEFT JOIN %[1]s.materials m ON b.output_type='material' AND m.id=b.output_material_id WHERE b.id=$1`, r.schema), bomID).Scan(&row.BomName, &row.ProductName, &state, &mode, &row.CurrentVersionID, &row.CurrentVersionNo, &row.SourceTemplateVersionID, &row.SourceTemplateVersionNo, &sourceTemplate, &oldState, &wasPublished, &draft, &main.ComponentType, &main.MaterialID, &main.ComponentProductID, &main.ComponentBomSpecID)
	if err == pgx.ErrNoRows {
		row.Reason = "BOM不存在或没有已发布版本"
		return row, merge, main, nil
	}
	if err != nil {
		return row, merge, main, err
	}
	if state != "active" || mode != "spec_group" {
		row.Reason = "BOM已停用或不是规格组"
		return row, merge, main, nil
	}
	if sourceTemplate != target.TemplateID || !wasPublished || (oldState != "published" && oldState != "archived") {
		row.Reason = "模板来源不完整或不属于该模板"
		return row, merge, main, nil
	}
	if row.SourceTemplateVersionID == target.TemplateVersionID {
		row.Status = "already_updated"
		row.Reason = "已使用该模板版本"
		return row, merge, main, nil
	}
	if draft {
		row.Reason = "存在未发布草稿，请先处理草稿"
		return row, merge, main, nil
	}
	if valid, e := productBOMSpecTemplateProvenanceMode(row.SourceTemplateVersionID, main); e != nil || !valid {
		row.Reason = "规格主体组件来源不完整"
		return row, merge, main, nil
	}
	var unit string
	if main.ComponentType == "product" {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT s.inventory_unit FROM %s.production_bom_specs s JOIN %s.production_boms b ON b.id=s.bom_id JOIN %s.products p ON p.id=b.output_product_id AND p.active=true WHERE s.id=$1 AND b.output_product_id=$2`, r.schema, r.schema, r.schema), main.ComponentBomSpecID, main.ComponentProductID).Scan(&unit)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(unit,''),'unit') FROM %s.materials WHERE id=$1 AND deprecated_at IS NULL`, r.schema), main.MaterialID).Scan(&unit)
	}
	if err == pgx.ErrNoRows {
		row.Reason = "规格主体组件不可用"
		return row, merge, main, nil
	}
	if err != nil {
		return row, merge, main, err
	}
	before, err := r.listProductionBomSpecTemplateVariantsTx(ctx, tx, row.SourceTemplateVersionID)
	if err != nil {
		return row, merge, main, err
	}
	after, err := r.listProductionBomSpecTemplateVariantsTx(ctx, tx, target.TemplateVersionID)
	if err != nil {
		return row, merge, main, err
	}
	current, err := r.currentRolloutSpecs(ctx, tx, row.CurrentVersionID, target.TemplateID)
	if err != nil {
		return row, merge, main, err
	}
	merge = bomapp.MergeTemplateRollout(expandRolloutTemplate(before, main, unit), expandRolloutTemplate(after, main, unit), current)
	row.Changes = merge.Changes
	if len(merge.Conflicts) > 0 {
		row.Reason = strings.Join(merge.Conflicts, "；")
		return row, merge, main, nil
	}
	row.Status = "ready"
	return row, merge, main, nil
}

func (r Repository) ApplySpecTemplateRollout(ctx context.Context, cmd bomapp.TemplateRolloutCommand) (bomapp.TemplateRolloutResult, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return bomapp.TemplateRolloutResult{}, err
	}
	result := bomapp.TemplateRolloutResult{BatchID: hex.EncodeToString(random[:]), Rows: []bomapp.TemplateRolloutRow{}}
	for _, target := range cmd.Targets {
		row, err := r.applyTemplateRolloutItem(ctx, cmd.VersionID, target, cmd.Actor, result.BatchID)
		if err != nil {
			row.Status = "failed"
			row.Reason = err.Error()
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}
func (r Repository) applyTemplateRolloutItem(ctx context.Context, templateVersionID int64, selected bomapp.TemplateRolloutTarget, actor, batchID string) (bomapp.TemplateRolloutRow, error) {
	row := bomapp.TemplateRolloutRow{BomID: selected.BomID}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return row, err
	}
	defer tx.Rollback(ctx)
	if err = lockProductionBomDefaultGraphTx(ctx, tx, r.schema); err != nil {
		return row, err
	}
	var outputType string
	var outputID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT output_type,CASE WHEN output_type='product' THEN output_product_id ELSE output_material_id END FROM %s.production_boms WHERE id=$1`, r.schema), selected.BomID).Scan(&outputType, &outputID); err != nil {
		return row, fmt.Errorf("BOM不存在")
	}
	if outputType == "product" {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, outputID); err != nil {
			return row, err
		}
	}
	var lockedType string
	var lockedOutput int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT output_type,CASE WHEN output_type='product' THEN output_product_id ELSE output_material_id END FROM %s.production_boms WHERE id=$1 FOR UPDATE`, r.schema), selected.BomID).Scan(&lockedType, &lockedOutput); err != nil {
		return row, err
	}
	if lockedType != outputType || lockedOutput != outputID {
		return row, fmt.Errorf("BOM产出对象已变化，请刷新后重试")
	}
	target, err := r.rolloutTemplate(ctx, tx, templateVersionID, true)
	if err != nil {
		return row, err
	}
	row, merge, _, err := r.prepareTemplateRollout(ctx, tx, target, selected.BomID)
	if err != nil {
		return row, err
	}
	if row.Status != "ready" {
		return row, nil
	}
	if row.CurrentVersionID != selected.ExpectedVersionID {
		row.Status = "blocked"
		row.Reason = "BOM已被修改，请刷新下发清单"
		return row, nil
	}
	bound := postgresinfra.WithTransaction(ctx, tx)
	version, err := r.CreateProductionBomVersion(bound, bomapp.CreateProductionBomVersionCommand{BomID: selected.BomID, SourceVersionID: row.CurrentVersionID, Note: "规格模板下发 " + target.TemplateVersionNo, Actor: actor})
	if err != nil {
		return row, err
	}
	variants := []bomapp.ProductionBomDraftVariant{}
	for _, spec := range merge.Specs {
		v := spec.Variant
		if !strings.HasPrefix(spec.Key, "local:") {
			id, key, barcode, e := resolveTemplateBomSpecTx(ctx, tx, r.schema, selected.BomID, target.TemplateID, spec.Key, v.Name, v.InventoryUnit, actor)
			if e != nil {
				return row, e
			}
			v.BomSpecID = id
			v.SpecKey = key
			v.Barcode = barcode
		}
		variants = append(variants, v)
	}
	if err = saveProductionBomDraftVariantsTx(ctx, tx, r.schema, selected.BomID, version.ID, variants, actor); err != nil {
		return row, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET source_spec_template_version_id=$2 WHERE id=$1`, r.schema), version.ID, templateVersionID); err != nil {
		return row, err
	}
	for _, v := range variants {
		if v.IsDefault {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET output_unit=$2,material_loss_rate=$3,process_route_id=$4 WHERE id=$1`, r.schema), version.ID, v.InventoryUnit, v.MaterialLossRate, v.ProcessRouteID); err != nil {
				return row, err
			}
		}
	}
	if err = r.ValidateAndPublishProductionBomVersion(bound, bomapp.PublishProductionBomVersionCommand{VersionID: version.ID, Actor: actor}); err != nil {
		return row, err
	}
	if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "production_bom", &selected.BomID, "rollout_spec_template", postgresinfra.StrPtr("source_spec_template_version_id"), postgresinfra.StrPtr(fmt.Sprint(row.SourceTemplateVersionID)), postgresinfra.StrPtr(fmt.Sprint(templateVersionID)), postgresinfra.AuditMeta{"batch_id": batchID, "template_id": target.TemplateID, "old_bom_version_id": row.CurrentVersionID, "new_bom_version_id": version.ID, "changes": merge.Changes}); err != nil {
		return row, err
	}
	if err = tx.Commit(ctx); err != nil {
		return row, err
	}
	row.Status = "updated"
	row.NewVersionID = version.ID
	row.NewVersionNo = version.VersionNo
	return row, nil
}
