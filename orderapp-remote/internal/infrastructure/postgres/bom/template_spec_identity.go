package bom

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Template keys (often spec-1) are local to their template. A changed inventory
// unit must receive a fresh stock identity, never mutate an already used one.
func resolveTemplateBomSpecTx(ctx context.Context, tx pgx.Tx, schema string, bomID, templateID int64, templateKey, name, unit, actor string) (id int64, specKey, barcode string, err error) {
	templateKey = strings.TrimSpace(templateKey)
	unit = strings.TrimSpace(unit)
	var locked int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_boms WHERE id=$1 FOR UPDATE`, schema), bomID).Scan(&locked); err != nil {
		return
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,spec_key,barcode FROM %s.production_bom_specs
		WHERE bom_id=$1 AND source_spec_template_id=$2 AND lower(source_spec_template_key)=lower($3) AND lower(inventory_unit)=lower($4)
		FOR UPDATE
	`, schema), bomID, templateID, templateKey, unit).Scan(&id, &specKey, &barcode)
	if err == nil {
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return
	}
	// Adopt an unmapped legacy identity only when its version provenance proves
	// it belongs to this template, with no conflicting template origin.
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT s.id,s.spec_key,s.barcode FROM %[1]s.production_bom_specs s
		WHERE s.bom_id=$1 AND s.source_spec_template_id=0 AND lower(s.spec_key)=lower($3) AND lower(s.inventory_unit)=lower($4)
		AND EXISTS(SELECT 1 FROM %[1]s.production_bom_version_variants v JOIN %[1]s.production_bom_versions b ON b.id=v.version_id
			JOIN %[1]s.production_bom_spec_template_versions t ON t.id=b.source_spec_template_version_id WHERE v.bom_spec_id=s.id AND t.template_id=$2)
		AND NOT EXISTS(SELECT 1 FROM %[1]s.production_bom_version_variants v JOIN %[1]s.production_bom_versions b ON b.id=v.version_id
			JOIN %[1]s.production_bom_spec_template_versions t ON t.id=b.source_spec_template_version_id WHERE v.bom_spec_id=s.id AND t.template_id<>$2)
		FOR UPDATE OF s
	`, schema), bomID, templateID, templateKey, unit).Scan(&id, &specKey, &barcode)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		base := templateKey
		unitHash := sha256.Sum256([]byte(strings.ToLower(unit)))
		for n := 0; ; n++ {
			specKey = base
			if n > 0 {
				specKey = fmt.Sprintf("%s-t%d-u%x-%d", templateKey, templateID, unitHash[:4], n)
			}
			var exists bool
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_bom_specs WHERE bom_id=$1 AND lower(spec_key)=lower($2))`, schema), bomID, specKey).Scan(&exists); err != nil {
				return
			}
			if !exists {
				break
			}
		}
		id, err = upsertProductionBomSpecTx(ctx, tx, schema, bomID, specKey, name, unit, "", actor)
		if err != nil {
			return
		}
		barcode = ""
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_specs SET source_spec_template_id=$2,source_spec_template_key=$3 WHERE id=$1`, schema), id, templateID, templateKey)
	return
}
