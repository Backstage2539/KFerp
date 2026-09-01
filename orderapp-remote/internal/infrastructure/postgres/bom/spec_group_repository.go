package bom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	bomapp "orderapp/internal/application/bom"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func (r Repository) ListProductionBomSpecTemplates(ctx context.Context) ([]bomapp.ProductionBomSpecTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, code, name, active,
		       to_char(created_at,'YYYY-MM-DD HH24:MI'),
		       to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.production_bom_spec_templates
		WHERE active=true
		ORDER BY name,id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	out := make([]bomapp.ProductionBomSpecTemplate, 0)
	for rows.Next() {
		var row bomapp.ProductionBomSpecTemplate
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.Active, &row.CreatedAt, &row.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range out {
		versions, err := r.listProductionBomSpecTemplateVersions(ctx, out[index].ID)
		if err != nil {
			return nil, err
		}
		out[index].Versions = versions
	}
	return out, nil
}

func (r Repository) ProductionBomSpecInventoryUnits(ctx context.Context, specIDs []int64) (map[int64]string, error) {
	units := make(map[int64]string)
	if len(specIDs) == 0 {
		return units, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,COALESCE(NULLIF(inventory_unit,''),'unit')
		FROM %s.production_bom_specs
		WHERE id=ANY($1::bigint[])
	`, r.schema), specIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var unit string
		if err := rows.Scan(&id, &unit); err != nil {
			return nil, err
		}
		units[id] = unit
	}
	return units, rows.Err()
}

func (r Repository) GetProductionBomSpecTemplate(ctx context.Context, id int64, versionID int64) (bomapp.ProductionBomSpecTemplate, error) {
	var out bomapp.ProductionBomSpecTemplate
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,code,name,active,to_char(created_at,'YYYY-MM-DD HH24:MI'),to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.production_bom_spec_templates WHERE id=$1
	`, r.schema), id).Scan(&out.ID, &out.Code, &out.Name, &out.Active, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return out, err
	}
	versions, err := r.listProductionBomSpecTemplateVersions(ctx, id)
	if err != nil {
		return out, err
	}
	out.Versions = versions
	selected := versionID
	if selected <= 0 {
		for _, version := range versions {
			if version.Status == "draft" {
				selected = version.ID
				break
			}
			if selected <= 0 && version.Status == "published" {
				selected = version.ID
			}
		}
	}
	if selected > 0 {
		out.Variants, err = r.listProductionBomSpecTemplateVariants(ctx, selected)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func (r Repository) listProductionBomSpecTemplateVersions(ctx context.Context, templateID int64) ([]bomapp.ProductionBomSpecTemplateVersion, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT v.id,v.template_id,v.version_no,v.status,v.note,
		       (SELECT count(*) FROM %[1]s.production_bom_spec_template_variants x WHERE x.version_id=v.id),
		       to_char(v.created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(v.published_at,'YYYY-MM-DD HH24:MI'),'')
		FROM %[1]s.production_bom_spec_template_versions v
		WHERE v.template_id=$1
		ORDER BY v.id DESC
	`, r.schema), templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]bomapp.ProductionBomSpecTemplateVersion, 0)
	for rows.Next() {
		var row bomapp.ProductionBomSpecTemplateVersion
		if err := rows.Scan(&row.ID, &row.TemplateID, &row.VersionNo, &row.Status, &row.Note, &row.VariantCount, &row.CreatedAt, &row.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listProductionBomSpecTemplateVariants(ctx context.Context, versionID int64) ([]bomapp.ProductionBomSpecTemplateVariant, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,spec_key,name,inventory_unit,is_default,sort_order,material_loss_rate::float8,process_route_id
		FROM %s.production_bom_spec_template_variants
		WHERE version_id=$1 ORDER BY sort_order,id
	`, r.schema), versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]bomapp.ProductionBomSpecTemplateVariant, 0)
	for rows.Next() {
		var variant bomapp.ProductionBomSpecTemplateVariant
		if err := rows.Scan(&variant.ID, &variant.SpecKey, &variant.Name, &variant.InventoryUnit, &variant.IsDefault, &variant.SortOrder, &variant.MaterialLossRate, &variant.ProcessRouteID); err != nil {
			return nil, err
		}
		itemRows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT is_main_input,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,
			       qty_per_unit::float8,ratio_pct::float8,material_loss_rate::float8,sort_order
			FROM %s.production_bom_spec_template_variant_items
			WHERE variant_id=$1 ORDER BY sort_order,id
		`, r.schema), variant.ID)
		if err != nil {
			return nil, err
		}
		for itemRows.Next() {
			var item bomapp.ProductionBomSpecTemplateVariantDraftItem
			if err := itemRows.Scan(&item.IsMainInput, &item.MaterialID, &item.ComponentType, &item.ComponentProductID, &item.ComponentBomSpecID, &item.ComponentSpecG, &item.ConsumeUnit, &item.QtyPerUnit, &item.RatioPct, &item.MaterialLossRate, &item.SortOrder); err != nil {
				itemRows.Close()
				return nil, err
			}
			variant.Items = append(variant.Items, item)
		}
		if err := itemRows.Err(); err != nil {
			itemRows.Close()
			return nil, err
		}
		itemRows.Close()
		out = append(out, variant)
	}
	return out, rows.Err()
}

func (r Repository) CreateProductionBomSpecTemplate(ctx context.Context, cmd bomapp.CreateProductionBomSpecTemplateCommand) (bomapp.ProductionBomSpecTemplate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomSpecTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var templateID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_spec_templates(code,name,created_by,updated_by)
		VALUES($1,$2,$3,$3) RETURNING id
	`, r.schema), fmt.Sprintf("PENDING-%s", strings.TrimSpace(cmd.Actor)), strings.TrimSpace(cmd.Name), strings.TrimSpace(cmd.Actor)).Scan(&templateID); err != nil {
		return bomapp.ProductionBomSpecTemplate{}, err
	}
	code := fmt.Sprintf("BOM-SPEC-TPL-%06d", templateID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_spec_templates SET code=$2 WHERE id=$1`, r.schema), templateID, code); err != nil {
		return bomapp.ProductionBomSpecTemplate{}, err
	}
	var versionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_spec_template_versions(template_id,version_no,status,note,created_by)
		VALUES($1,'V001','draft','初始版本',$2) RETURNING id
	`, r.schema), templateID, strings.TrimSpace(cmd.Actor)).Scan(&versionID); err != nil {
		return bomapp.ProductionBomSpecTemplate{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_spec_template", &templateID, "create", postgresinfra.StrPtr("code"), nil, postgresinfra.StrPtr(code), postgresinfra.AuditMeta{"template_id": templateID, "version_id": versionID, "name": strings.TrimSpace(cmd.Name)}); err != nil {
		return bomapp.ProductionBomSpecTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomSpecTemplate{}, err
	}
	return r.GetProductionBomSpecTemplate(ctx, templateID, versionID)
}

func (r Repository) CreateProductionBomSpecTemplateVersion(ctx context.Context, cmd bomapp.CreateProductionBomSpecTemplateVersionCommand) (bomapp.ProductionBomSpecTemplateVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var sourceID int64
	if cmd.SourceVersionID > 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_spec_template_versions WHERE id=$1 AND template_id=$2`, r.schema), cmd.SourceVersionID, cmd.TemplateID).Scan(&sourceID)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_spec_template_versions WHERE template_id=$1 AND status='published' ORDER BY id DESC LIMIT 1`, r.schema), cmd.TemplateID).Scan(&sourceID)
	}
	if err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, fmt.Errorf("published specification template version not found")
	}
	var next int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)+1 FROM %s.production_bom_spec_template_versions WHERE template_id=$1`, r.schema), cmd.TemplateID).Scan(&next); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	versionNo := fmt.Sprintf("V%03d", next)
	var versionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_spec_template_versions(template_id,version_no,status,note,created_by)
		VALUES($1,$2,'draft',$3,$4) RETURNING id
	`, r.schema), cmd.TemplateID, versionNo, strings.TrimSpace(cmd.Note), strings.TrimSpace(cmd.Actor)).Scan(&versionID); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	if err := copyProductionBomSpecTemplateVariantsTx(ctx, tx, r.schema, sourceID, versionID); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_spec_template_version", &versionID, "create", postgresinfra.StrPtr("version_no"), nil, postgresinfra.StrPtr(versionNo), postgresinfra.AuditMeta{"template_id": cmd.TemplateID, "source_version_id": sourceID}); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	return r.productionBomSpecTemplateVersionByID(ctx, versionID)
}

func copyProductionBomSpecTemplateVariantsTx(ctx context.Context, tx pgx.Tx, schema string, sourceVersionID, targetVersionID int64) error {
	type sourceVariant struct {
		id, routeID     int64
		key, name, unit string
		isDefault       bool
		sortOrder       int
		loss            float64
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,spec_key,name,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id FROM %s.production_bom_spec_template_variants WHERE version_id=$1 ORDER BY id`, schema), sourceVersionID)
	if err != nil {
		return err
	}
	variants := make([]sourceVariant, 0)
	for rows.Next() {
		var variant sourceVariant
		if err := rows.Scan(&variant.id, &variant.key, &variant.name, &variant.unit, &variant.isDefault, &variant.sortOrder, &variant.loss, &variant.routeID); err != nil {
			rows.Close()
			return err
		}
		variants = append(variants, variant)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, variant := range variants {
		var newID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_spec_template_variants(version_id,spec_key,name,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, schema), targetVersionID, variant.key, variant.name, variant.unit, variant.isDefault, variant.sortOrder, variant.loss, variant.routeID).Scan(&newID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_spec_template_variant_items(variant_id,is_main_input,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,sort_order) SELECT $1,is_main_input,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,sort_order FROM %s.production_bom_spec_template_variant_items WHERE variant_id=$2 ORDER BY sort_order,id`, schema, schema), newID, variant.id); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) UpdateProductionBomSpecTemplateVersionDraft(ctx context.Context, cmd bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand) (bomapp.ProductionBomSpecTemplateVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_bom_spec_template_versions WHERE id=$1 FOR UPDATE`, r.schema), cmd.VersionID).Scan(&status); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, fmt.Errorf("specification template version not found")
	}
	if status != "draft" {
		return bomapp.ProductionBomSpecTemplateVersion{}, fmt.Errorf("published specification template version is read-only")
	}
	if err := validateSpecVariantUnitsTx(ctx, tx, r.schema, cmd.Variants); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	if err := validateProductionBomSpecTemplateDraftComponentUnitsTx(ctx, tx, r.schema, cmd.Variants); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	previousVariants, err := r.listProductionBomSpecTemplateVariantsTx(ctx, tx, cmd.VersionID)
	if err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_spec_template_variant_items WHERE variant_id IN (SELECT id FROM %s.production_bom_spec_template_variants WHERE version_id=$1)`, r.schema, r.schema), cmd.VersionID); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_spec_template_variants WHERE version_id=$1`, r.schema), cmd.VersionID); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	for _, variant := range cmd.Variants {
		var variantID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_spec_template_variants(version_id,spec_key,name,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, r.schema), cmd.VersionID, variant.SpecKey, variant.Name, variant.InventoryUnit, variant.IsDefault, variant.SortOrder, variant.MaterialLossRate, variant.ProcessRouteID).Scan(&variantID); err != nil {
			return bomapp.ProductionBomSpecTemplateVersion{}, err
		}
		for _, item := range variant.Items {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_spec_template_variant_items(variant_id,is_main_input,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, r.schema), variantID, item.IsMainInput, item.MaterialID, item.ComponentType, item.ComponentProductID, item.ComponentBomSpecID, item.ComponentSpecG, item.ConsumeUnit, item.QtyPerUnit, item.RatioPct, item.MaterialLossRate, item.SortOrder); err != nil {
				return bomapp.ProductionBomSpecTemplateVersion{}, err
			}
		}
	}
	storedVariants, err := r.listProductionBomSpecTemplateVariantsTx(ctx, tx, cmd.VersionID)
	if err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	previousVariantsJSON, err := productionBomSpecTemplateVariantsAuditJSON(previousVariants)
	if err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	storedVariantsJSON, err := productionBomSpecTemplateVariantsAuditJSON(storedVariants)
	if err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	if previousVariantsJSON != storedVariantsJSON {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_spec_template_version", &cmd.VersionID, "update_specifications", postgresinfra.StrPtr("variants"), postgresinfra.StrPtr(previousVariantsJSON), postgresinfra.StrPtr(storedVariantsJSON), postgresinfra.AuditMeta{"old_variant_count": len(previousVariants), "new_variant_count": len(storedVariants)}); err != nil {
			return bomapp.ProductionBomSpecTemplateVersion{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_spec_template_version", &cmd.VersionID, "update_draft", postgresinfra.StrPtr("version_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.VersionID)), postgresinfra.AuditMeta{"variant_count": len(cmd.Variants)}); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomSpecTemplateVersion{}, err
	}
	return r.productionBomSpecTemplateVersionByID(ctx, cmd.VersionID)
}

func validateProductionBomSpecTemplateDraftComponentUnitsTx(ctx context.Context, tx pgx.Tx, schema string, variants []bomapp.ProductionBomSpecTemplateVariant) error {
	for _, variant := range variants {
		for _, templateItem := range variant.Items {
			item := templateItem.ProductionBomDraftItem
			componentType := strings.ToLower(strings.TrimSpace(item.ComponentType))
			if componentType == "finished_product" {
				componentType = "product"
			}
			if componentType != "product" || item.ComponentBomSpecID <= 0 {
				continue
			}
			if err := validateBomSpecConsumeUnit(ctx, tx, schema, item.ComponentBomSpecID, item.ConsumeUnit); err != nil {
				return fmt.Errorf("variant %s: %w", variant.SpecKey, err)
			}
		}
	}
	return nil
}

func validateBomSpecConsumeUnit(ctx context.Context, q bomQueryer, schema string, componentBomSpecID int64, consumeUnit string) error {
	var specInventoryUnit string
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(inventory_unit,''),'unit')
		FROM %s.production_bom_specs
		WHERE id=$1
		FOR SHARE
	`, schema), componentBomSpecID).Scan(&specInventoryUnit)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("component BOM specification not found: %d", componentBomSpecID)
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(consumeUnit), strings.TrimSpace(specInventoryUnit)) {
		return fmt.Errorf("consume_unit must match component BOM specification inventory_unit %s", specInventoryUnit)
	}
	return nil
}

func productionBomSpecTemplateVariantsAuditJSON(variants []bomapp.ProductionBomSpecTemplateVariant) (string, error) {
	normalized := make([]bomapp.ProductionBomSpecTemplateVariant, len(variants))
	for index := range variants {
		normalized[index] = variants[index]
		normalized[index].ID = 0
		normalized[index].Items = append([]bomapp.ProductionBomSpecTemplateVariantDraftItem(nil), variants[index].Items...)
		for itemIndex := range normalized[index].Items {
			normalized[index].Items[itemIndex].BomVariantID = 0
		}
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (r Repository) PublishProductionBomSpecTemplateVersion(ctx context.Context, cmd bomapp.PublishProductionBomSpecTemplateVersionCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var templateID, publishedVersionBeforeLock int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT candidate.template_id,COALESCE((
			SELECT published.id
			FROM %[1]s.production_bom_spec_template_versions published
			WHERE published.template_id=candidate.template_id AND published.status='published'
			ORDER BY published.id DESC LIMIT 1
		),0)
		FROM %[1]s.production_bom_spec_template_versions candidate
		WHERE candidate.id=$1
	`, r.schema), cmd.VersionID).Scan(&templateID, &publishedVersionBeforeLock); err != nil {
		return fmt.Errorf("specification template version not found")
	}
	var lockedTemplateID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_spec_templates WHERE id=$1 FOR UPDATE`, r.schema), templateID).Scan(&lockedTemplateID); err != nil {
		return fmt.Errorf("specification template not found")
	}
	var publishedVersionAfterLock int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE((
			SELECT id
			FROM %s.production_bom_spec_template_versions
			WHERE template_id=$1 AND status='published'
			ORDER BY id DESC LIMIT 1
		),0)
	`, r.schema), templateID).Scan(&publishedVersionAfterLock); err != nil {
		return err
	}
	if publishedVersionAfterLock != publishedVersionBeforeLock {
		return fmt.Errorf("specification template draft was superseded by a newer published specification template version")
	}
	var lockedVersionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_spec_template_versions WHERE id=$1 AND template_id=$2 AND status='draft' FOR UPDATE`, r.schema), cmd.VersionID, templateID).Scan(&lockedVersionID); err != nil {
		return fmt.Errorf("specification template version not found")
	}
	variants, err := r.listProductionBomSpecTemplateVariantsTx(ctx, tx, cmd.VersionID)
	if err != nil {
		return err
	}
	if len(variants) == 0 {
		return fmt.Errorf("at least one specification is required")
	}
	defaultCount := 0
	for _, variant := range variants {
		if variant.IsDefault {
			defaultCount++
		}
		if len(variant.Items) == 0 {
			return fmt.Errorf("variant %s requires components", variant.SpecKey)
		}
		mainCount := 0
		for _, item := range variant.Items {
			if item.IsMainInput {
				mainCount++
			}
		}
		if mainCount != 1 {
			return fmt.Errorf("variant %s requires exactly one main input", variant.SpecKey)
		}
		if err := validateProductionBomSpecTemplateVariantItemsForPublish(ctx, tx, r.schema, variant); err != nil {
			return fmt.Errorf("variant %s: %w", variant.SpecKey, err)
		}
	}
	if defaultCount != 1 {
		return fmt.Errorf("exactly one default specification is required")
	}
	if err := validateSpecVariantUnitsTx(ctx, tx, r.schema, variants); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_spec_template_versions SET status='archived' WHERE template_id=$1 AND status='published' AND id<>$2`, r.schema), templateID, cmd.VersionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_spec_template_versions SET status='published',published_at=now(),published_by=$2 WHERE id=$1`, r.schema), cmd.VersionID, strings.TrimSpace(cmd.Actor)); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_spec_template_version", &cmd.VersionID, "publish", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("draft"), postgresinfra.StrPtr("published"), postgresinfra.AuditMeta{"template_id": templateID, "variant_count": len(variants)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func validateProductionBomSpecTemplateVariantItemsForPublish(ctx context.Context, tx pgx.Tx, schema string, variant bomapp.ProductionBomSpecTemplateVariant) error {
	items := make([]bomapp.ProductionBomDraftItem, 0, len(variant.Items))
	for _, templateItem := range variant.Items {
		item, err := normalizeProductionBomSpecTemplateItemForPublish(templateItem)
		if err != nil {
			return err
		}
		items = append(items, item)
		if templateItem.IsMainInput {
			// A template main input is deliberately unresolved until a concrete
			// material is selected while copying the template into a product BOM.
			// Its quantity/mode is still validated above, but its placeholder ID
			// must not be looked up as a material master record here.
			continue
		}
		switch item.ComponentType {
		case "material":
			var inventoryUnit string
			var active bool
			err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(NULLIF(unit,''),'kg'),deprecated_at IS NULL
				FROM %s.materials WHERE id=$1
				FOR SHARE
			`, schema), item.MaterialID).Scan(&inventoryUnit, &active)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("component material not found: %d", item.MaterialID)
			}
			if err != nil {
				return err
			}
			if !active {
				return fmt.Errorf("component material is inactive: %d", item.MaterialID)
			}
			if bomapp.ProductionBomConsumeUnitRequiresInventoryMatch(item.ConsumeUnit) &&
				!strings.EqualFold(strings.TrimSpace(item.ConsumeUnit), strings.TrimSpace(inventoryUnit)) {
				return fmt.Errorf("consume_unit must match component inventory_unit")
			}
		case "product":
			var active bool
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.products WHERE id=$1 FOR SHARE`, schema), item.ComponentProductID).Scan(&active); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("component product not found: %d", item.ComponentProductID)
				}
				return err
			}
			if !active {
				return fmt.Errorf("component product is inactive: %d", item.ComponentProductID)
			}
			if item.ComponentBomSpecID <= 0 {
				return fmt.Errorf("product component requires component_bom_spec_id")
			}
			var specOutputType, bomStatus, specInventoryUnit string
			var specProductID int64
			err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(NULLIF(bom.output_type,''),'product'),
				       COALESCE(bom.output_product_id,0),
				       COALESCE(NULLIF(bom.status,''),'active'),
				       COALESCE(NULLIF(spec.inventory_unit,''),'unit')
				FROM %s.production_bom_specs spec
				JOIN %s.production_boms bom ON bom.id=spec.bom_id
				WHERE spec.id=$1
				FOR SHARE OF spec,bom
			`, schema, schema), item.ComponentBomSpecID).Scan(&specOutputType, &specProductID, &bomStatus, &specInventoryUnit)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("component BOM specification not found: %d", item.ComponentBomSpecID)
			}
			if err != nil {
				return err
			}
			if specOutputType != "product" || specProductID != item.ComponentProductID {
				return fmt.Errorf("component_bom_spec_id does not belong to component product %d", item.ComponentProductID)
			}
			if !strings.EqualFold(strings.TrimSpace(bomStatus), "active") {
				return fmt.Errorf("component BOM specification belongs to an inactive BOM: %d", item.ComponentBomSpecID)
			}
			if !strings.EqualFold(strings.TrimSpace(item.ConsumeUnit), strings.TrimSpace(specInventoryUnit)) {
				return fmt.Errorf("consume_unit must match component BOM specification inventory_unit %s", specInventoryUnit)
			}
			var published bool
			if err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT EXISTS(
					SELECT 1
					FROM %[1]s.production_bom_version_variants variant
					JOIN %[1]s.production_bom_versions version
					  ON version.id=variant.version_id AND version.status='published'
					WHERE variant.bom_spec_id=$1
				)
			`, schema), item.ComponentBomSpecID).Scan(&published); err != nil {
				return err
			}
			if !published {
				return fmt.Errorf("component BOM specification has no published version: %d", item.ComponentBomSpecID)
			}
		}
	}
	return bomapp.ValidateProductionBomRecipeMode(variant.MaterialLossRate, items)
}

func validateProductComponentBomSpecConsumeUnit(ctx context.Context, q bomQueryer, schema string, componentProductID, componentBomSpecID int64, consumeUnit string) error {
	var specInventoryUnit string
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(spec.inventory_unit,''),'unit')
		FROM %s.production_bom_specs spec
		JOIN %s.production_boms bom ON bom.id=spec.bom_id
		WHERE spec.id=$1
		  AND COALESCE(NULLIF(bom.output_type,''),'product')='product'
		  AND bom.output_product_id=$2
		FOR SHARE OF spec,bom
	`, schema, schema), componentBomSpecID, componentProductID).Scan(&specInventoryUnit)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("component BOM specification not found or does not belong to component product %d", componentProductID)
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(consumeUnit), strings.TrimSpace(specInventoryUnit)) {
		return fmt.Errorf("consume_unit must match component BOM specification inventory_unit %s", specInventoryUnit)
	}
	return nil
}

func normalizeProductionBomSpecTemplateItemForPublish(templateItem bomapp.ProductionBomSpecTemplateVariantDraftItem) (bomapp.ProductionBomDraftItem, error) {
	item := templateItem.ProductionBomDraftItem
	componentType := strings.TrimSpace(item.ComponentType)
	if componentType == "" {
		componentType = "material"
	}
	if componentType == "finished_product" {
		componentType = "product"
	}
	if componentType != "material" && componentType != "product" {
		return item, fmt.Errorf("invalid component_type")
	}
	if templateItem.IsMainInput && componentType != "material" {
		return item, fmt.Errorf("main input must be material")
	}
	consumeUnit := strings.TrimSpace(item.ConsumeUnit)
	if consumeUnit == "" {
		if componentType == "product" {
			consumeUnit = "unit_per_box"
		} else {
			consumeUnit = "ratio_pct"
		}
	}
	if len(consumeUnit) > 64 {
		return item, fmt.Errorf("invalid consume_unit")
	}
	switch componentType {
	case "material":
		if item.ComponentBomSpecID > 0 {
			return item, fmt.Errorf("component_bom_spec_id requires product component")
		}
		if !templateItem.IsMainInput && item.MaterialID <= 0 {
			return item, fmt.Errorf("material_id required")
		}
	case "product":
		if item.ComponentProductID <= 0 {
			return item, fmt.Errorf("component_product_id required")
		}
		if consumeUnit == "ratio_pct" {
			return item, fmt.Errorf("product consume_unit must not be ratio_pct")
		}
	}
	if consumeUnit == "ratio_pct" {
		if item.RatioPct <= 0 || item.RatioPct > 100 {
			return item, fmt.Errorf("ratio must be (0,100]")
		}
	} else if item.QtyPerUnit <= 0 {
		return item, fmt.Errorf("qty_per_unit required")
	}
	item.ComponentType = componentType
	item.ConsumeUnit = consumeUnit
	return item, nil
}

func (r Repository) productionBomSpecTemplateVersionByID(ctx context.Context, id int64) (bomapp.ProductionBomSpecTemplateVersion, error) {
	var row bomapp.ProductionBomSpecTemplateVersion
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT v.id,v.template_id,v.version_no,v.status,v.note,(SELECT count(*) FROM %s.production_bom_spec_template_variants x WHERE x.version_id=v.id),to_char(v.created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(v.published_at,'YYYY-MM-DD HH24:MI'),'') FROM %s.production_bom_spec_template_versions v WHERE v.id=$1`, r.schema, r.schema), id).Scan(&row.ID, &row.TemplateID, &row.VersionNo, &row.Status, &row.Note, &row.VariantCount, &row.CreatedAt, &row.PublishedAt)
	return row, err
}

func (r Repository) listProductionBomSpecTemplateVariantsTx(ctx context.Context, tx pgx.Tx, versionID int64) ([]bomapp.ProductionBomSpecTemplateVariant, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,spec_key,name,inventory_unit,is_default,sort_order,material_loss_rate::float8,process_route_id FROM %s.production_bom_spec_template_variants WHERE version_id=$1 ORDER BY sort_order,id`, r.schema), versionID)
	if err != nil {
		return nil, err
	}
	out := make([]bomapp.ProductionBomSpecTemplateVariant, 0)
	for rows.Next() {
		var variant bomapp.ProductionBomSpecTemplateVariant
		if err := rows.Scan(&variant.ID, &variant.SpecKey, &variant.Name, &variant.InventoryUnit, &variant.IsDefault, &variant.SortOrder, &variant.MaterialLossRate, &variant.ProcessRouteID); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, variant)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for variantIndex := range out {
		variant := &out[variantIndex]
		itemRows, err := tx.Query(ctx, fmt.Sprintf(`SELECT is_main_input,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit::float8,ratio_pct::float8,material_loss_rate::float8,sort_order FROM %s.production_bom_spec_template_variant_items WHERE variant_id=$1 ORDER BY sort_order,id`, r.schema), variant.ID)
		if err != nil {
			return nil, err
		}
		for itemRows.Next() {
			var item bomapp.ProductionBomSpecTemplateVariantDraftItem
			if err := itemRows.Scan(&item.IsMainInput, &item.MaterialID, &item.ComponentType, &item.ComponentProductID, &item.ComponentBomSpecID, &item.ComponentSpecG, &item.ConsumeUnit, &item.QtyPerUnit, &item.RatioPct, &item.MaterialLossRate, &item.SortOrder); err != nil {
				itemRows.Close()
				return nil, err
			}
			variant.Items = append(variant.Items, item)
		}
		if err := itemRows.Err(); err != nil {
			itemRows.Close()
			return nil, err
		}
		itemRows.Close()
	}
	return out, nil
}

func validateSpecVariantUnitsTx(ctx context.Context, tx pgx.Tx, schema string, variants []bomapp.ProductionBomSpecTemplateVariant) error {
	seen := make(map[string]struct{})
	for _, variant := range variants {
		unit := strings.TrimSpace(variant.InventoryUnit)
		if unit == "" {
			return fmt.Errorf("variant inventory_unit required")
		}
		seen[strings.ToLower(unit)] = struct{}{}
	}
	for unit := range seen {
		var ok bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_unit_definitions WHERE lower(btrim(code))=$1 AND active=true AND deleted_at IS NULL)`, schema), unit).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("inventory_unit is not an active unit: %s", unit)
		}
	}
	return nil
}

func validateProductionBomDraftVariantUnitsTx(ctx context.Context, tx pgx.Tx, schema string, variants []bomapp.ProductionBomDraftVariant) error {
	converted := make([]bomapp.ProductionBomSpecTemplateVariant, len(variants))
	for i, variant := range variants {
		converted[i].InventoryUnit = variant.InventoryUnit
	}
	return validateSpecVariantUnitsTx(ctx, tx, schema, converted)
}

func copySpecTemplateToProductionBomTx(ctx context.Context, tx pgx.Tx, schema string, bomID, versionID, templateVersionID, mainInputMaterialID int64, actor string) error {
	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_bom_spec_template_versions WHERE id=$1`, schema), templateVersionID).Scan(&status); err != nil || status != "published" {
		return fmt.Errorf("published specification template version not found")
	}
	var mainInputUnit string
	var mainInputActive bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(unit,''),'unit'),deprecated_at IS NULL FROM %s.materials WHERE id=$1`, schema), mainInputMaterialID).Scan(&mainInputUnit, &mainInputActive); err != nil || !mainInputActive {
		return fmt.Errorf("main input material not found or inactive")
	}
	type sourceVariant struct {
		id, routeID         int64
		specKey, name, unit string
		isDefault           bool
		sortOrder           int
		loss                float64
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,spec_key,name,inventory_unit,is_default,sort_order,material_loss_rate::float8,process_route_id FROM %s.production_bom_spec_template_variants WHERE version_id=$1 ORDER BY sort_order,id`, schema), templateVersionID)
	if err != nil {
		return err
	}
	variants := make([]sourceVariant, 0)
	for rows.Next() {
		var variant sourceVariant
		if err := rows.Scan(&variant.id, &variant.specKey, &variant.name, &variant.unit, &variant.isDefault, &variant.sortOrder, &variant.loss, &variant.routeID); err != nil {
			rows.Close()
			return err
		}
		variants = append(variants, variant)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	defaultUnit := ""
	defaultLoss := 0.0
	defaultRouteID := int64(0)
	for _, variant := range variants {
		bomSpecID, err := upsertProductionBomSpecTx(ctx, tx, schema, bomID, variant.specKey, variant.name, variant.unit, "", actor)
		if err != nil {
			return err
		}
		var bomVariantID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, schema), versionID, bomSpecID, variant.name, variant.unit, variant.isDefault, variant.sortOrder, variant.loss, variant.routeID).Scan(&bomVariantID); err != nil {
			return err
		}
		type sourceItem struct {
			isMain                                                    bool
			materialID, productID, componentBomSpecID, componentSpecG int64
			componentType, consumeUnit                                string
			qty, ratio                                                float64
		}
		itemRows, err := tx.Query(ctx, fmt.Sprintf(`SELECT is_main_input,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit::float8,ratio_pct::float8 FROM %s.production_bom_spec_template_variant_items WHERE variant_id=$1 ORDER BY sort_order,id`, schema), variant.id)
		if err != nil {
			return err
		}
		items := make([]sourceItem, 0)
		for itemRows.Next() {
			var item sourceItem
			if err := itemRows.Scan(&item.isMain, &item.materialID, &item.componentType, &item.productID, &item.componentBomSpecID, &item.componentSpecG, &item.consumeUnit, &item.qty, &item.ratio); err != nil {
				itemRows.Close()
				return err
			}
			items = append(items, item)
		}
		if err := itemRows.Err(); err != nil {
			itemRows.Close()
			return err
		}
		itemRows.Close()
		for _, item := range items {
			if item.isMain {
				item.materialID = mainInputMaterialID
				item.componentType = "material"
				item.productID = 0
				item.componentBomSpecID = 0
				if item.consumeUnit == "main_input_unit" {
					item.consumeUnit = strings.TrimSpace(mainInputUnit)
				}
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_items(version_id,variant_id,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,COALESCE((SELECT purchase_price FROM %s.materials WHERE id=$3),0))`, schema, schema), versionID, bomVariantID, item.materialID, item.componentType, item.productID, item.componentBomSpecID, item.componentSpecG, item.consumeUnit, item.qty, item.ratio, variant.loss); err != nil {
				return err
			}
		}
		if variant.isDefault {
			defaultUnit = variant.unit
			defaultLoss = variant.loss
			defaultRouteID = variant.routeID
		}
	}
	if len(variants) == 0 {
		return fmt.Errorf("specification template has no variants")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET source_spec_template_version_id=$2,main_input_material_id=$3,output_unit=COALESCE(NULLIF($4,''),output_unit),material_loss_rate=$5,process_route_id=$6 WHERE id=$1`, schema), versionID, templateVersionID, mainInputMaterialID, defaultUnit, defaultLoss, defaultRouteID); err != nil {
		return err
	}
	return nil
}

func (r Repository) ReapplyProductionBomSpecTemplateVersion(ctx context.Context, cmd bomapp.ReapplyProductionBomSpecTemplateVersionCommand) (bomapp.ProductionBomVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var prelockBomID, prelockOutputProductID int64
	var prelockOutputType, prelockSpecificationMode string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT version.bom_id,
		       COALESCE(NULLIF(bom.output_type,''),'product'),
		       COALESCE(NULLIF(bom.specification_mode,''),'single'),
		       COALESCE(bom.output_product_id,0)
		FROM %s.production_bom_versions version
		JOIN %s.production_boms bom ON bom.id=version.bom_id
		WHERE version.id=$1
	`, r.schema, r.schema), cmd.VersionID).Scan(&prelockBomID, &prelockOutputType, &prelockSpecificationMode, &prelockOutputProductID); err != nil {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("production BOM version not found")
	}
	if prelockOutputType != "product" || prelockSpecificationMode != bomapp.ProductionBomSpecificationModeSpecGroup {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("specification template replacement requires a multi-specification product BOM")
	}
	var bomID, outputProductID int64
	var status, outputType, specificationMode string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT version.bom_id,
		       version.status,
		       COALESCE(NULLIF(bom.output_type,''),'product'),
		       COALESCE(NULLIF(bom.specification_mode,''),'single'),
		       COALESCE(bom.output_product_id,0)
		FROM %s.production_bom_versions version
		JOIN %s.production_boms bom ON bom.id=version.bom_id
		WHERE version.id=$1
		FOR UPDATE OF version,bom
	`, r.schema, r.schema), cmd.VersionID).Scan(&bomID, &status, &outputType, &specificationMode, &outputProductID); err != nil {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("production BOM version not found")
	}
	if status != "draft" {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("published production BOM version is read-only")
	}
	if bomID != prelockBomID || outputType != prelockOutputType || specificationMode != prelockSpecificationMode || outputProductID != prelockOutputProductID {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("production BOM output changed concurrently; retry specification template replacement")
	}
	if outputType != "product" || outputProductID <= 0 {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("specification template requires product output")
	}
	if cmd.SpecTemplateVersionID <= 0 || cmd.MainInputMaterialID <= 0 {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("published specification template version and main_input_material_id are required")
	}
	var templateStatus string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT status FROM %s.production_bom_spec_template_versions
		WHERE id=$1
		FOR SHARE
	`, r.schema), cmd.SpecTemplateVersionID).Scan(&templateStatus); err != nil || templateStatus != "published" {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("published specification template version not found")
	}
	var mainInputUnit string
	var mainInputActive bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(unit,''),'unit'),deprecated_at IS NULL
		FROM %s.materials WHERE id=$1
		FOR SHARE
	`, r.schema), cmd.MainInputMaterialID).Scan(&mainInputUnit, &mainInputActive); err != nil || !mainInputActive {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("main input material not found or inactive")
	}

	templateVariants, err := r.listProductionBomSpecTemplateVariantsTx(ctx, tx, cmd.SpecTemplateVersionID)
	if err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if len(templateVariants) == 0 {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("specification template has no variants")
	}
	variants := make([]bomapp.ProductionBomDraftVariant, 0, len(templateVariants))
	for _, source := range templateVariants {
		variant := bomapp.ProductionBomDraftVariant{
			SpecKey: source.SpecKey, Name: source.Name, InventoryUnit: source.InventoryUnit,
			IsDefault: source.IsDefault, SortOrder: source.SortOrder,
			MaterialLossRate: source.MaterialLossRate, ProcessRouteID: source.ProcessRouteID,
		}
		// Reapplying a template replaces the version snapshot, not the BOM-owned
		// specification identity. Preserve user-maintained stable fields that the
		// reusable template does not own, notably barcode and generated code.
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id,barcode
			FROM %s.production_bom_specs
			WHERE bom_id=$1 AND lower(spec_key)=lower($2)
			FOR UPDATE
		`, r.schema), bomID, source.SpecKey).Scan(&variant.BomSpecID, &variant.Barcode)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return bomapp.ProductionBomVersion{}, err
		}
		for _, sourceItem := range source.Items {
			item := sourceItem.ProductionBomDraftItem
			if sourceItem.IsMainInput {
				item.MaterialID = cmd.MainInputMaterialID
				item.ComponentType = "material"
				item.ComponentProductID = 0
				item.ComponentBomSpecID = 0
				item.ComponentSpecG = 0
				if item.ConsumeUnit == "main_input_unit" {
					item.ConsumeUnit = strings.TrimSpace(mainInputUnit)
				}
			}
			if source.MaterialLossRate > 0 && item.ComponentType == "material" && item.ConsumeUnit == "ratio_pct" {
				item.MaterialLossRate = source.MaterialLossRate
			} else {
				item.MaterialLossRate = 0
			}
			variant.Items = append(variant.Items, item)
		}
		if err := bomapp.ValidateProductionBomRecipeMode(variant.MaterialLossRate, variant.Items); err != nil {
			return bomapp.ProductionBomVersion{}, fmt.Errorf("variant %s: %w", variant.SpecKey, err)
		}
		variants = append(variants, variant)
	}
	if err := validateProductionBomDraftVariantUnitsTx(ctx, tx, r.schema, variants); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	oldGroupJSON, err := productionBomVersionVariantGroupAuditJSONTx(ctx, tx, r.schema, cmd.VersionID)
	if err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if err := saveProductionBomDraftVariantsTx(ctx, tx, r.schema, bomID, cmd.VersionID, variants, cmd.Actor); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_versions version
		SET source_spec_template_version_id=$2,
		    main_input_material_id=$3,
		    output_unit=defaults.inventory_unit,
		    material_loss_rate=defaults.material_loss_rate,
		    process_route_id=defaults.process_route_id
		FROM %s.production_bom_version_variants defaults
		WHERE version.id=$1 AND defaults.version_id=version.id AND defaults.is_default=true
	`, r.schema, r.schema), cmd.VersionID, cmd.SpecTemplateVersionID, cmd.MainInputMaterialID); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	newGroupJSON, err := productionBomVersionVariantGroupAuditJSONTx(ctx, tx, r.schema, cmd.VersionID)
	if err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_version", &cmd.VersionID, "reapply_spec_template", postgresinfra.StrPtr("variants"), postgresinfra.StrPtr(oldGroupJSON), postgresinfra.StrPtr(newGroupJSON), postgresinfra.AuditMeta{
		"bom_id": bomID, "spec_template_version_id": cmd.SpecTemplateVersionID,
		"main_input_material_id": cmd.MainInputMaterialID, "old_variant_count": strings.Count(oldGroupJSON, `"spec_key"`),
		"new_variant_count": len(variants),
	}); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	return r.productionBomVersionByID(ctx, cmd.VersionID)
}

func productionBomVersionVariantGroupAuditJSONTx(ctx context.Context, tx pgx.Tx, schema string, versionID int64) (string, error) {
	var payload string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(jsonb_agg(
			jsonb_build_object(
				'bom_spec_id',variant.bom_spec_id,
				'spec_key',spec.spec_key,
				'name',variant.spec_name_snapshot,
				'inventory_unit',variant.inventory_unit,
				'is_default',variant.is_default,
				'sort_order',variant.sort_order,
				'material_loss_rate',variant.material_loss_rate,
				'process_route_id',variant.process_route_id,
				'items',COALESCE((
					SELECT jsonb_agg(jsonb_build_object(
						'material_id',item.material_id,
						'component_type',item.component_type,
						'component_product_id',item.component_product_id,
						'component_bom_spec_id',item.component_bom_spec_id,
						'consume_unit',item.consume_unit,
						'qty_per_unit',item.qty_per_unit,
						'ratio_pct',item.ratio_pct
					) ORDER BY item.id)
					FROM %s.production_bom_version_items item
					WHERE item.variant_id=variant.id
				),'[]'::jsonb)
			) ORDER BY variant.sort_order,variant.id
		),'[]'::jsonb)::text
		FROM %s.production_bom_version_variants variant
		JOIN %s.production_bom_specs spec ON spec.id=variant.bom_spec_id
		WHERE variant.version_id=$1
	`, schema, schema, schema), versionID).Scan(&payload)
	return payload, err
}

func upsertProductionBomSpecTx(ctx context.Context, tx pgx.Tx, schema string, bomID int64, specKey, name, unit, barcode, actor string) (int64, error) {
	pendingCode := fmt.Sprintf("BOM-SPEC-PENDING-%d-%s", bomID, strings.ToLower(strings.TrimSpace(specKey)))
	var id int64
	var code string
	created := false
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_specs(bom_id,code,spec_key,name,inventory_unit,barcode,created_by,updated_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$7)
		ON CONFLICT (bom_id, lower(spec_key)) DO NOTHING
		RETURNING id,code
	`, schema), bomID, pendingCode, strings.TrimSpace(specKey), strings.TrimSpace(name), strings.TrimSpace(unit), strings.TrimSpace(barcode), strings.TrimSpace(actor)).Scan(&id, &code)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id,code FROM %s.production_bom_specs
			WHERE bom_id=$1 AND lower(spec_key)=lower($2)
			FOR UPDATE
		`, schema), bomID, strings.TrimSpace(specKey)).Scan(&id, &code); err != nil {
			return 0, err
		}
		if err := updateProductionBomSpecTx(ctx, tx, schema, id, name, unit, barcode, actor); err != nil {
			return 0, err
		}
	} else if err != nil {
		return 0, err
	} else {
		created = true
	}
	if strings.HasPrefix(code, "BOM-SPEC-PENDING-") {
		code = fmt.Sprintf("BOM-SPEC-%06d", id)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_specs SET code=$2 WHERE id=$1`, schema), id, code); err != nil {
			return 0, err
		}
	}
	if created {
		fields := []struct {
			name  string
			value string
		}{
			{name: "code", value: code},
			{name: "spec_key", value: strings.TrimSpace(specKey)},
			{name: "name", value: strings.TrimSpace(name)},
			{name: "inventory_unit", value: strings.TrimSpace(unit)},
			{name: "barcode", value: strings.TrimSpace(barcode)},
		}
		for _, field := range fields {
			if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "production_bom_spec", &id, "create", postgresinfra.StrPtr(field.name), nil, postgresinfra.StrPtr(field.value), postgresinfra.AuditMeta{"bom_id": bomID, "spec_key": strings.TrimSpace(specKey), "code": code}); err != nil {
				return 0, err
			}
		}
	}
	return id, nil
}

func updateProductionBomSpecTx(ctx context.Context, tx pgx.Tx, schema string, bomSpecID int64, name, unit, barcode, actor string) error {
	var bomID int64
	var code, specKey, currentName, currentUnit, currentBarcode string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT bom_id,code,spec_key,name,inventory_unit,barcode FROM %s.production_bom_specs WHERE id=$1 FOR UPDATE`, schema), bomSpecID).Scan(&bomID, &code, &specKey, &currentName, &currentUnit, &currentBarcode); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	unit = strings.TrimSpace(unit)
	barcode = strings.TrimSpace(barcode)
	if !strings.EqualFold(strings.TrimSpace(currentUnit), unit) {
		var historicallyPublished bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT EXISTS(
				SELECT 1
				FROM %s.production_bom_version_variants variant
				JOIN %s.production_bom_versions version ON version.id=variant.version_id
				WHERE variant.bom_spec_id=$1 AND lower(version.status) IN ('published','archived')
			)
		`, schema, schema), bomSpecID).Scan(&historicallyPublished); err != nil {
			return err
		}
		if historicallyPublished {
			return fmt.Errorf("BOM specification inventory_unit cannot be changed after publication; create a new specification")
		}
		for _, table := range []string{
			"finished_inventory",
			"stock_batches",
			"stock_ledger_entries",
			"stock_entry_items",
			"stock_adjustment_items",
			"finished_product_transfer_items",
			"product_price_records",
			"product_price_tiers",
			"product_tier_price_scheme_tiers",
			"order_items",
			"order_stock_batch_allocations",
			"order_stock_deductions",
		} {
			referenced, err := productionBomSpecReferencedByTableTx(ctx, tx, schema, table, bomSpecID)
			if err != nil {
				return err
			}
			if referenced {
				return fmt.Errorf("BOM specification inventory_unit cannot be changed after inventory, price, or order use; create a new specification")
			}
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_specs
		SET name=$2,inventory_unit=$3,barcode=$4,updated_at=now(),updated_by=$5
		WHERE id=$1
	`, schema), bomSpecID, name, unit, barcode, strings.TrimSpace(actor)); err != nil {
		return err
	}
	changes := []struct {
		field string
		old   string
		new   string
	}{
		{field: "name", old: currentName, new: name},
		{field: "inventory_unit", old: currentUnit, new: unit},
		{field: "barcode", old: currentBarcode, new: barcode},
	}
	for _, change := range changes {
		if change.old == change.new {
			continue
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "production_bom_spec", &bomSpecID, "update", postgresinfra.StrPtr(change.field), postgresinfra.StrPtr(change.old), postgresinfra.StrPtr(change.new), postgresinfra.AuditMeta{"bom_id": bomID, "spec_key": specKey, "code": code}); err != nil {
			return err
		}
	}
	return nil
}

func productionBomSpecReferencedByTableTx(ctx context.Context, tx pgx.Tx, schema, table string, bomSpecID int64) (bool, error) {
	var hasColumn bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2 AND column_name='bom_spec_id'
		)
	`, schema, table).Scan(&hasColumn); err != nil {
		return false, err
	}
	if !hasColumn {
		return false, nil
	}
	var referenced bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.%s WHERE bom_spec_id=$1)`, schema, table), bomSpecID).Scan(&referenced); err != nil {
		return false, err
	}
	return referenced, nil
}

func copyProductionBomVersionVariantsTx(ctx context.Context, tx pgx.Tx, schema string, sourceVersionID, targetVersionID int64) error {
	type sourceVariant struct {
		id, bomSpecID, routeID int64
		name, unit             string
		isDefault              bool
		sortOrder              int
		loss                   float64
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id FROM %s.production_bom_version_variants WHERE version_id=$1 ORDER BY sort_order,id`, schema), sourceVersionID)
	if err != nil {
		return err
	}
	variants := make([]sourceVariant, 0)
	for rows.Next() {
		var variant sourceVariant
		if err := rows.Scan(&variant.id, &variant.bomSpecID, &variant.name, &variant.unit, &variant.isDefault, &variant.sortOrder, &variant.loss, &variant.routeID); err != nil {
			rows.Close()
			return err
		}
		variants = append(variants, variant)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, variant := range variants {
		var targetVariantID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, schema), targetVersionID, variant.bomSpecID, variant.name, variant.unit, variant.isDefault, variant.sortOrder, variant.loss, variant.routeID).Scan(&targetVariantID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_items(version_id,variant_id,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot) SELECT $1,$2,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot FROM %s.production_bom_version_items WHERE version_id=$3 AND variant_id=$4 ORDER BY id`, schema, schema), targetVersionID, targetVariantID, sourceVersionID, variant.id); err != nil {
			return err
		}
	}
	return nil
}

func copyProductionBomVariantsToNewBomTx(ctx context.Context, tx pgx.Tx, schema string, sourceVersionID, targetBomID, targetVersionID int64, actor string) error {
	type sourceVariant struct {
		id, routeID         int64
		specKey, name, unit string
		isDefault           bool
		sortOrder           int
		loss                float64
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT v.id,s.spec_key,v.spec_name_snapshot,v.inventory_unit,v.is_default,v.sort_order,v.material_loss_rate,v.process_route_id
		FROM %s.production_bom_version_variants v
		JOIN %s.production_bom_specs s ON s.id=v.bom_spec_id
		WHERE v.version_id=$1 ORDER BY v.sort_order,v.id
	`, schema, schema), sourceVersionID)
	if err != nil {
		return err
	}
	variants := make([]sourceVariant, 0)
	for rows.Next() {
		var variant sourceVariant
		if err := rows.Scan(&variant.id, &variant.specKey, &variant.name, &variant.unit, &variant.isDefault, &variant.sortOrder, &variant.loss, &variant.routeID); err != nil {
			rows.Close()
			return err
		}
		variants = append(variants, variant)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, variant := range variants {
		bomSpecID, err := upsertProductionBomSpecTx(ctx, tx, schema, targetBomID, variant.specKey, variant.name, variant.unit, "", actor)
		if err != nil {
			return err
		}
		var targetVariantID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, schema), targetVersionID, bomSpecID, variant.name, variant.unit, variant.isDefault, variant.sortOrder, variant.loss, variant.routeID).Scan(&targetVariantID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_items(version_id,variant_id,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot) SELECT $1,$2,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot FROM %s.production_bom_version_items WHERE version_id=$3 AND variant_id=$4 ORDER BY id`, schema, schema), targetVersionID, targetVariantID, sourceVersionID, variant.id); err != nil {
			return err
		}
	}
	return nil
}

func saveProductionBomDraftVariantsTx(ctx context.Context, tx pgx.Tx, schema string, bomID, versionID int64, variants []bomapp.ProductionBomDraftVariant, actor string) error {
	type previousSpec struct {
		id                  int64
		code, specKey, name string
	}
	previousRows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT spec.id,spec.code,spec.spec_key,spec.name
		FROM %s.production_bom_version_variants variant
		JOIN %s.production_bom_specs spec ON spec.id=variant.bom_spec_id
		WHERE variant.version_id=$1
		ORDER BY spec.id
	`, schema, schema), versionID)
	if err != nil {
		return err
	}
	previousSpecs := make([]previousSpec, 0)
	for previousRows.Next() {
		var spec previousSpec
		if err := previousRows.Scan(&spec.id, &spec.code, &spec.specKey, &spec.name); err != nil {
			previousRows.Close()
			return err
		}
		previousSpecs = append(previousSpecs, spec)
	}
	if err := previousRows.Err(); err != nil {
		previousRows.Close()
		return err
	}
	previousRows.Close()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_version_items WHERE version_id=$1 AND variant_id<>0`, schema), versionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_version_variants WHERE version_id=$1`, schema), versionID); err != nil {
		return err
	}
	retainedSpecIDs := make(map[int64]struct{}, len(variants))
	for _, variant := range variants {
		bomSpecID := variant.BomSpecID
		if bomSpecID > 0 {
			var belongs bool
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_bom_specs WHERE id=$1 AND bom_id=$2 AND lower(spec_key)=lower($3))`, schema), bomSpecID, bomID, variant.SpecKey).Scan(&belongs); err != nil {
				return err
			}
			if !belongs {
				return fmt.Errorf("bom_spec_id does not belong to BOM")
			}
			if err := updateProductionBomSpecTx(ctx, tx, schema, bomSpecID, variant.Name, variant.InventoryUnit, variant.Barcode, actor); err != nil {
				return err
			}
		} else {
			var err error
			bomSpecID, err = upsertProductionBomSpecTx(ctx, tx, schema, bomID, variant.SpecKey, variant.Name, variant.InventoryUnit, variant.Barcode, actor)
			if err != nil {
				return err
			}
		}
		retainedSpecIDs[bomSpecID] = struct{}{}
		var bomVariantID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, schema), versionID, bomSpecID, variant.Name, variant.InventoryUnit, variant.IsDefault, variant.SortOrder, variant.MaterialLossRate, variant.ProcessRouteID).Scan(&bomVariantID); err != nil {
			return err
		}
		for _, item := range variant.Items {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_items(version_id,variant_id,material_id,component_type,component_product_id,component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,material_loss_rate,unit_cost_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,COALESCE((SELECT purchase_price FROM %s.materials WHERE id=$3),0))`, schema, schema), versionID, bomVariantID, item.MaterialID, item.ComponentType, item.ComponentProductID, item.ComponentBomSpecID, item.ComponentSpecG, item.ConsumeUnit, item.QtyPerUnit, item.RatioPct, item.MaterialLossRate); err != nil {
				return err
			}
		}
	}
	for _, spec := range previousSpecs {
		if _, retained := retainedSpecIDs[spec.id]; retained {
			continue
		}
		oldVersionID := fmt.Sprintf("%d", versionID)
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "production_bom_spec", &spec.id, "remove_from_draft", postgresinfra.StrPtr("bom_version_id"), postgresinfra.StrPtr(oldVersionID), nil, postgresinfra.AuditMeta{"bom_id": bomID, "bom_version_id": versionID, "spec_key": spec.specKey, "code": spec.code, "name": spec.name, "stable_identity_retained": true}); err != nil {
			return err
		}
	}
	return nil
}

func productBOMRequiresSpecGroupTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (bool, error) {
	if productID <= 0 {
		return false, nil
	}
	// Serialize product-BOM authority decisions. Every product is governed by a
	// BOM specification group; no migration-state lookup or direct-product
	// fallback remains.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, productID); err != nil {
		return false, err
	}
	return true, nil
}

func validateProductBOMDraftSpecTemplateProvenanceTx(ctx context.Context, tx pgx.Tx, schema string, bomID int64) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.production_bom_versions
		WHERE bom_id=$1 AND status='draft'
		ORDER BY id
		FOR SHARE
	`, schema), bomID)
	if err != nil {
		return err
	}
	versionIDs := make([]int64, 0)
	for rows.Next() {
		var versionID int64
		if err := rows.Scan(&versionID); err != nil {
			rows.Close()
			return err
		}
		versionIDs = append(versionIDs, versionID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(versionIDs) == 0 {
		return fmt.Errorf("product BOM requires a draft specification group copied from a published specification template with an active main input material")
	}
	for _, versionID := range versionIDs {
		if err := validateGovernedProductBOMVersionSpecGroupTx(ctx, tx, schema, versionID); err != nil {
			return err
		}
	}
	return nil
}

func requireProductBOMSpecTemplateTx(ctx context.Context, tx pgx.Tx, schema string, specTemplateVersionID, mainInputMaterialID int64) error {
	if specTemplateVersionID <= 0 || mainInputMaterialID <= 0 {
		return fmt.Errorf("product BOM requires a published specification template version and main_input_material_id")
	}
	var templateStatus string
	var templateWasPublished bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT status,published_at IS NOT NULL
		FROM %s.production_bom_spec_template_versions
		WHERE id=$1
		FOR SHARE
	`, schema), specTemplateVersionID).Scan(&templateStatus, &templateWasPublished); err != nil || !templateWasPublished || templateStatus != "published" {
		return fmt.Errorf("product BOM requires a published specification template version and main_input_material_id")
	}
	var mainInputActive bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT deprecated_at IS NULL
		FROM %s.materials
		WHERE id=$1
		FOR SHARE
	`, schema), mainInputMaterialID).Scan(&mainInputActive); err != nil || !mainInputActive {
		return fmt.Errorf("product BOM requires a published specification template version and main_input_material_id")
	}
	return nil
}

func validateProductBOMVersionSpecGroupTx(ctx context.Context, tx pgx.Tx, schema string, productID, versionID int64) error {
	if productID <= 0 || versionID <= 0 {
		return nil
	}
	return validateGovernedProductBOMVersionSpecGroupTx(ctx, tx, schema, versionID)
}

// validateGovernedProductBOMVersionSpecGroupTx validates one explicitly
// multi-specification version. The caller owns the surrounding product/default
// graph lock before this function locks the version and its provenance rows.
func validateGovernedProductBOMVersionSpecGroupTx(ctx context.Context, tx pgx.Tx, schema string, versionID int64) error {
	if versionID <= 0 {
		return nil
	}
	var lockedVersionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.production_bom_versions
		WHERE id=$1
		FOR SHARE
	`, schema), versionID).Scan(&lockedVersionID); err != nil {
		return fmt.Errorf("product BOM version not found")
	}
	var variantCount, defaultCount int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*),COUNT(*) FILTER (WHERE is_default)
		FROM %s.production_bom_version_variants
		WHERE version_id=$1
	`, schema), versionID).Scan(&variantCount, &defaultCount); err != nil {
		return err
	}
	if variantCount == 0 {
		return fmt.Errorf("product BOM version requires at least one specification")
	}
	if defaultCount != 1 {
		return fmt.Errorf("product BOM version requires exactly one default specification")
	}
	var templateVersionID, mainInputMaterialID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(source_spec_template_version_id,0),COALESCE(main_input_material_id,0)
		FROM %s.production_bom_versions
		WHERE id=$1
	`, schema), lockedVersionID).Scan(&templateVersionID, &mainInputMaterialID); err != nil {
		return fmt.Errorf("product BOM version not found")
	}
	if templateVersionID <= 0 || mainInputMaterialID <= 0 {
		return fmt.Errorf("product BOM version requires a specification group copied from a published specification template with an active main input material")
	}
	var templateStatus string
	var templateWasPublished bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT status,published_at IS NOT NULL FROM %s.production_bom_spec_template_versions
		WHERE id=$1
		FOR SHARE
	`, schema), templateVersionID).Scan(&templateStatus, &templateWasPublished); err != nil || !templateWasPublished || (templateStatus != "published" && templateStatus != "archived") {
		return fmt.Errorf("product BOM version requires a specification group copied from a published specification template with an active main input material")
	}
	var mainInputActive bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT deprecated_at IS NULL FROM %s.materials
		WHERE id=$1
		FOR SHARE
	`, schema), mainInputMaterialID).Scan(&mainInputActive); err != nil || !mainInputActive {
		return fmt.Errorf("product BOM version requires a specification group copied from a published specification template with an active main input material")
	}
	return nil
}

func (r Repository) listProductionBomVersionVariants(ctx context.Context, versionID int64) ([]bomapp.ProductionBomVersionVariant, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT v.id,v.bom_spec_id,s.code,s.barcode,s.spec_key,v.spec_name_snapshot,v.inventory_unit,v.is_default,v.sort_order,v.material_loss_rate::float8,v.process_route_id
		FROM %s.production_bom_version_variants v
		JOIN %s.production_bom_specs s ON s.id=v.bom_spec_id
		WHERE v.version_id=$1 ORDER BY v.sort_order,v.id
	`, r.schema, r.schema), versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]bomapp.ProductionBomVersionVariant, 0)
	for rows.Next() {
		var variant bomapp.ProductionBomVersionVariant
		if err := rows.Scan(&variant.ID, &variant.BomSpecID, &variant.Code, &variant.Barcode, &variant.SpecKey, &variant.Name, &variant.InventoryUnit, &variant.IsDefault, &variant.SortOrder, &variant.MaterialLossRate, &variant.ProcessRouteID); err != nil {
			return nil, err
		}
		itemRows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT i.id,i.variant_id,i.material_id,COALESCE(m.name,''),i.component_type,i.component_product_id,COALESCE(p.name,''),i.component_bom_spec_id,i.component_spec_g,i.consume_unit,i.qty_per_unit::float8,i.ratio_pct::float8,i.material_loss_rate::float8
			FROM %s.production_bom_version_items i
			LEFT JOIN %s.materials m ON m.id=i.material_id
			LEFT JOIN %s.products p ON p.id=i.component_product_id
			WHERE i.version_id=$1 AND i.variant_id=$2 ORDER BY i.id
		`, r.schema, r.schema, r.schema), versionID, variant.ID)
		if err != nil {
			return nil, err
		}
		for itemRows.Next() {
			var item bomapp.Item
			if err := itemRows.Scan(&item.ID, &item.BomVariantID, &item.MaterialID, &item.MaterialName, &item.ComponentType, &item.ComponentProductID, &item.ComponentProductName, &item.ComponentBomSpecID, &item.ComponentSpecG, &item.ConsumeUnit, &item.QtyPerUnit, &item.RatioPct, &item.MaterialLossRate); err != nil {
				itemRows.Close()
				return nil, err
			}
			variant.Items = append(variant.Items, item)
		}
		itemRows.Close()
		out = append(out, variant)
	}
	return out, rows.Err()
}

func validateProductionBomVersionVariantsForPublish(ctx context.Context, q bomQueryer, schema string, versionID, bomID int64) (bool, error) {
	type storedVariant struct {
		id, specBomID int64
		unit, specKey string
		isDefault     bool
		loss          float64
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT v.id,v.inventory_unit,v.is_default,v.material_loss_rate::float8,s.spec_key,s.bom_id
		FROM %s.production_bom_version_variants v
		JOIN %s.production_bom_specs s ON s.id=v.bom_spec_id
		WHERE v.version_id=$1 ORDER BY v.id
	`, schema, schema), versionID)
	if err != nil {
		return false, err
	}
	variants := make([]storedVariant, 0)
	for rows.Next() {
		var variant storedVariant
		if err := rows.Scan(&variant.id, &variant.unit, &variant.isDefault, &variant.loss, &variant.specKey, &variant.specBomID); err != nil {
			rows.Close()
			return false, err
		}
		variants = append(variants, variant)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	defaultCount := 0
	for _, variant := range variants {
		if variant.specBomID != bomID {
			return false, fmt.Errorf("bom_spec_id does not belong to BOM")
		}
		if variant.loss < 0 || variant.loss >= 1 {
			return false, fmt.Errorf("variant %s: material_loss_rate must be >= 0 and < 1", variant.specKey)
		}
		if variant.isDefault {
			defaultCount++
		}
		var unitOK bool
		if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_unit_definitions WHERE lower(btrim(code))=lower(btrim($1)) AND active=true AND deleted_at IS NULL)`, schema), variant.unit).Scan(&unitOK); err != nil {
			return false, err
		}
		if !unitOK {
			return false, fmt.Errorf("inventory_unit is not an active unit: %s", variant.unit)
		}
		itemRows, err := q.Query(ctx, fmt.Sprintf(`SELECT component_type,material_id,component_product_id,component_bom_spec_id,consume_unit,qty_per_unit::float8,ratio_pct::float8 FROM %s.production_bom_version_items WHERE version_id=$1 AND variant_id=$2 ORDER BY id`, schema), versionID, variant.id)
		if err != nil {
			return false, err
		}
		items := make([]bomapp.ProductionBomDraftItem, 0)
		for itemRows.Next() {
			var item bomapp.ProductionBomDraftItem
			if err := itemRows.Scan(&item.ComponentType, &item.MaterialID, &item.ComponentProductID, &item.ComponentBomSpecID, &item.ConsumeUnit, &item.QtyPerUnit, &item.RatioPct); err != nil {
				itemRows.Close()
				return false, err
			}
			if (item.ComponentType == "product" || item.ComponentType == "finished_product") && item.ConsumeUnit == "ratio_pct" {
				itemRows.Close()
				return false, fmt.Errorf("variant %s: product consume_unit must not be ratio_pct", variant.specKey)
			}
			items = append(items, item)
		}
		if err := itemRows.Err(); err != nil {
			itemRows.Close()
			return false, err
		}
		itemRows.Close()
		if len(items) == 0 {
			return false, fmt.Errorf("variant %s requires components", variant.specKey)
		}
		if err := bomapp.ValidateProductionBomRecipeMode(variant.loss, items); err != nil {
			return false, fmt.Errorf("variant %s: %w", variant.specKey, err)
		}
	}
	if len(variants) > 0 && defaultCount != 1 {
		return false, fmt.Errorf("exactly one default specification is required")
	}
	return len(variants) > 0, nil
}
