package materials

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

type MaterialOwnershipCutoverCandidate struct {
	SourceMaterialID int64   `json:"source_material_id"`
	SourceCode       string  `json:"source_code"`
	SourceName       string  `json:"source_name"`
	OwnerCustomerID  int64   `json:"owner_customer_id"`
	OwnerName        string  `json:"owner_name"`
	BatchCount       int64   `json:"batch_count"`
	RemainingG       int64   `json:"remaining_g"`
	RemainingUnits   int64   `json:"remaining_units"`
	InventoryAmount  float64 `json:"inventory_amount"`
}

type MaterialOwnershipCutoverMapping struct {
	SourceMaterialID int64  `json:"source_material_id"`
	TargetMaterialID int64  `json:"target_material_id"`
	OwnerCustomerID  int64  `json:"owner_customer_id"`
	TargetCode       string `json:"target_code"`
	Status           string `json:"status"`
}

type MaterialOwnershipCutoverReport struct {
	ManifestID string                              `json:"manifest_id"`
	Candidates []MaterialOwnershipCutoverCandidate `json:"candidates"`
	Conflicts  []string                            `json:"conflicts"`
	Mappings   []MaterialOwnershipCutoverMapping   `json:"mappings"`
	Applied    int                                 `json:"applied"`
	RolledBack int                                 `json:"rolled_back"`
}

func (r Repository) PreviewMaterialOwnershipCutover(ctx context.Context) (MaterialOwnershipCutoverReport, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return MaterialOwnershipCutoverReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	report, err := r.previewMaterialOwnershipCutoverTx(ctx, tx)
	if err != nil {
		return MaterialOwnershipCutoverReport{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MaterialOwnershipCutoverReport{}, err
	}
	return report, nil
}

func (r Repository) previewMaterialOwnershipCutoverTx(ctx context.Context, tx pgx.Tx) (MaterialOwnershipCutoverReport, error) {
	report := MaterialOwnershipCutoverReport{Candidates: []MaterialOwnershipCutoverCandidate{}, Conflicts: []string{}, Mappings: []MaterialOwnershipCutoverMapping{}}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		WITH active_customer_batches AS (
			SELECT b.id,b.material_id,COALESCE(b.owner_customer_id,0) owner_customer_id,
			       COALESCE(b.remaining_g,0) remaining_g,COALESCE(b.remaining_units,0) remaining_units,
			       COALESCE(b.unit_cost,0)::float8 unit_cost
			FROM %[1]s.material_batches b
			WHERE COALESCE(b.owner_customer_id,0)>0
			  AND ((COALESCE(b.remaining_g,0)<>0 OR COALESCE(b.remaining_units,0)<>0)
			    OR EXISTS(SELECT 1 FROM %[1]s.material_batch_locations l WHERE l.material_batch_id=b.id AND (l.qty_g<>0 OR l.qty_units<>0)))
		)
		SELECT m.id,m.code,m.name,b.owner_customer_id,COALESCE(c.name,'客户 #' || b.owner_customer_id::text),
		       COUNT(*)::bigint,SUM(b.remaining_g)::bigint,SUM(b.remaining_units)::bigint,
		       SUM((b.remaining_g::numeric/1000+b.remaining_units::numeric)*b.unit_cost)::float8
		FROM active_customer_batches b
		JOIN %[1]s.materials m ON m.id=b.material_id
		LEFT JOIN %[1]s.customers c ON c.id=b.owner_customer_id
		WHERE COALESCE(m.owner_customer_id,0)<>b.owner_customer_id
		GROUP BY m.id,m.code,m.name,b.owner_customer_id,c.name
		ORDER BY m.id,b.owner_customer_id
	`, r.schema))
	if err != nil {
		return report, err
	}
	for rows.Next() {
		var c MaterialOwnershipCutoverCandidate
		if err := rows.Scan(&c.SourceMaterialID, &c.SourceCode, &c.SourceName, &c.OwnerCustomerID, &c.OwnerName, &c.BatchCount, &c.RemainingG, &c.RemainingUnits, &c.InventoryAmount); err != nil {
			rows.Close()
			return report, err
		}
		report.Candidates = append(report.Candidates, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return report, err
	}
	rows.Close()
	for _, candidate := range report.Candidates {
		if ok, err := materialCutoverTableExistsTx(ctx, tx, r.schema, "work_order_material_reservations"); err != nil {
			return report, err
		} else if ok {
			var count int64
			query := fmt.Sprintf(`SELECT COUNT(*) FROM %s.work_order_material_reservations wr`, r.schema)
			if wo, _ := materialCutoverTableExistsTx(ctx, tx, r.schema, "work_orders"); wo {
				query += fmt.Sprintf(` JOIN %s.work_orders wo ON wo.id=wr.work_order_id WHERE wr.material_id=$1 AND lower(COALESCE(wo.status,'')) NOT IN ('completed','cancelled','canceled','closed')`, r.schema)
			} else {
				query += ` WHERE wr.material_id=$1 AND lower(COALESCE(wr.status,'')) NOT IN ('completed','cancelled','canceled','closed')`
			}
			if err := tx.QueryRow(ctx, query, candidate.SourceMaterialID).Scan(&count); err != nil {
				return report, err
			}
			if count > 0 {
				report.Conflicts = append(report.Conflicts, fmt.Sprintf("物料 %d 存在 %d 条未完成生产预占/任务", candidate.SourceMaterialID, count))
			}
		}
		if entries, _ := materialCutoverTableExistsTx(ctx, tx, r.schema, "stock_entries"); entries {
			if items, _ := materialCutoverTableExistsTx(ctx, tx, r.schema, "stock_entry_items"); items {
				var count int64
				if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE i.material_id=$1 AND COALESCE(i.owner_customer_id,0)=$2 AND e.status='draft'`, r.schema), candidate.SourceMaterialID, candidate.OwnerCustomerID).Scan(&count); err != nil {
					return report, err
				}
				if count > 0 {
					report.Conflicts = append(report.Conflicts, fmt.Sprintf("物料 %d / 客户 %d 存在 %d 条未提交库存单据", candidate.SourceMaterialID, candidate.OwnerCustomerID, count))
				}
			}
		}
	}
	mappingRows, err := tx.Query(ctx, fmt.Sprintf(`SELECT source_material_id,target_material_id,owner_customer_id,target_code,status FROM %s.material_owner_migrations ORDER BY source_material_id,owner_customer_id`, r.schema))
	if err != nil {
		return report, err
	}
	for mappingRows.Next() {
		var m MaterialOwnershipCutoverMapping
		if err := mappingRows.Scan(&m.SourceMaterialID, &m.TargetMaterialID, &m.OwnerCustomerID, &m.TargetCode, &m.Status); err != nil {
			mappingRows.Close()
			return report, err
		}
		report.Mappings = append(report.Mappings, m)
	}
	if err := mappingRows.Err(); err != nil {
		mappingRows.Close()
		return report, err
	}
	mappingRows.Close()
	payload, _ := json.Marshal(struct {
		Candidates []MaterialOwnershipCutoverCandidate `json:"candidates"`
		Conflicts  []string                            `json:"conflicts"`
	}{report.Candidates, report.Conflicts})
	sum := sha256.Sum256(payload)
	report.ManifestID = hex.EncodeToString(sum[:])[:16]
	return report, nil
}

func (r Repository) ApplyMaterialOwnershipCutover(ctx context.Context, actor, expectedManifestID string) (MaterialOwnershipCutoverReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return MaterialOwnershipCutoverReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":material-owner-cutover"); err != nil {
		return MaterialOwnershipCutoverReport{}, err
	}
	report, err := r.previewMaterialOwnershipCutoverTx(ctx, tx)
	if err != nil {
		return report, err
	}
	if strings.TrimSpace(expectedManifestID) == "" || expectedManifestID != report.ManifestID {
		return report, fmt.Errorf("迁移清单已变化：请重新预检查并使用 manifest_id %s", report.ManifestID)
	}
	if len(report.Conflicts) > 0 {
		return report, fmt.Errorf("迁移预检查发现冲突：%s", strings.Join(report.Conflicts, "；"))
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "material-owner-cutover"
	}
	for _, candidate := range report.Candidates {
		var migrationID, targetID int64
		var status, targetCode string
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,target_material_id,status,target_code FROM %s.material_owner_migrations WHERE source_material_id=$1 AND owner_customer_id=$2 FOR UPDATE`, r.schema), candidate.SourceMaterialID, candidate.OwnerCustomerID).Scan(&migrationID, &targetID, &status, &targetCode)
		if err != nil && err != pgx.ErrNoRows {
			return report, err
		}
		if err == pgx.ErrNoRows {
			targetCode = fmt.Sprintf("%s-C%d", candidate.SourceCode, candidate.OwnerCustomerID)
			var codeUsed bool
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.materials WHERE code=$1)`, r.schema), targetCode).Scan(&codeUsed); err != nil {
				return report, err
			}
			if codeUsed {
				targetCode = fmt.Sprintf("%s-C%d-M%d", candidate.SourceCode, candidate.OwnerCustomerID, candidate.SourceMaterialID)
			}
			if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.materials(code,name,kind,is_semi_finished,unit,cost_unit,batch_no,purchase_price,sale_price,onhand_g,onhand_units,min_level_g,min_level_units,industry_field_template_id,owner_customer_id,deprecated_at,updated_at) SELECT $2,name,kind,is_semi_finished,unit,cost_unit,batch_no,purchase_price,sale_price,0,0,min_level_g,min_level_units,industry_field_template_id,$3,NULL,now() FROM %[1]s.materials WHERE id=$1 RETURNING id`, r.schema), candidate.SourceMaterialID, targetCode, candidate.OwnerCustomerID).Scan(&targetID); err != nil {
				return report, err
			}
			if err := copyMaterialOwnershipProfilesTx(ctx, tx, r.schema, candidate.SourceMaterialID, targetID, actor); err != nil {
				return report, err
			}
			if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.material_owner_migrations(source_material_id,owner_customer_id,target_material_id,source_code,target_code,status,manifest_id,applied_by,applied_at,updated_at) VALUES($1,$2,$3,$4,$5,'applied',$6,$7,now(),now()) RETURNING id`, r.schema), candidate.SourceMaterialID, candidate.OwnerCustomerID, targetID, candidate.SourceCode, targetCode, report.ManifestID, actor).Scan(&migrationID); err != nil {
				return report, err
			}
		} else {
			if status == "applied" {
				continue
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET owner_customer_id=$2,deprecated_at=NULL,updated_at=now() WHERE id=$1`, r.schema), targetID, candidate.OwnerCustomerID); err != nil {
				return report, err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_owner_migrations SET status='applied',manifest_id=$2,applied_by=$3,applied_at=now(),rolled_back_by='',rolled_back_at=NULL,updated_at=now() WHERE id=$1`, r.schema), migrationID, report.ManifestID, actor); err != nil {
				return report, err
			}
		}
		batchRows, err := tx.Query(ctx, fmt.Sprintf(`SELECT b.id,b.batch_code,b.remaining_g,b.remaining_units,COALESCE(b.unit_cost,0)::float8 FROM %[1]s.material_batches b WHERE b.material_id=$1 AND COALESCE(b.owner_customer_id,0)=$2 AND ((b.remaining_g<>0 OR b.remaining_units<>0) OR EXISTS(SELECT 1 FROM %[1]s.material_batch_locations l WHERE l.material_batch_id=b.id AND (l.qty_g<>0 OR l.qty_units<>0))) FOR UPDATE`, r.schema), candidate.SourceMaterialID, candidate.OwnerCustomerID)
		if err != nil {
			return report, err
		}
		type batchSnapshot struct {
			id, remainingG, remainingUnits int64
			code                           string
			cost                           float64
		}
		batchSnapshots := []batchSnapshot{}
		for batchRows.Next() {
			var batch batchSnapshot
			if err := batchRows.Scan(&batch.id, &batch.code, &batch.remainingG, &batch.remainingUnits, &batch.cost); err != nil {
				batchRows.Close()
				return report, err
			}
			batchSnapshots = append(batchSnapshots, batch)
		}
		if err := batchRows.Err(); err != nil {
			batchRows.Close()
			return report, err
		}
		batchRows.Close()
		batchIDs := make([]int64, 0, len(batchSnapshots))
		for _, batch := range batchSnapshots {
			batchIDs = append(batchIDs, batch.id)
			if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.material_owner_migration_batches(migration_id,material_batch_id,batch_code,source_material_id,target_material_id,owner_customer_id,remaining_g,remaining_units,unit_cost) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(material_batch_id) DO NOTHING`, r.schema), migrationID, batch.id, batch.code, candidate.SourceMaterialID, targetID, candidate.OwnerCustomerID, batch.remainingG, batch.remainingUnits, batch.cost); err != nil {
				return report, err
			}
		}
		if len(batchIDs) > 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET material_id=$2 WHERE id=ANY($1)`, r.schema), batchIDs, targetID); err != nil {
				return report, err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batch_locations SET material_id=$2,updated_at=now() WHERE material_batch_id=ANY($1)`, r.schema), batchIDs, targetID); err != nil {
				return report, err
			}
		}
		if err := recomputeCutoverMaterialOnhandTx(ctx, tx, r.schema, candidate.SourceMaterialID); err != nil {
			return report, err
		}
		if err := recomputeCutoverMaterialOnhandTx(ctx, tx, r.schema, targetID); err != nil {
			return report, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "material_owner_migration", &migrationID, "split_customer_inventory", postgresinfra.StrPtr("material_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", candidate.SourceMaterialID)), postgresinfra.StrPtr(fmt.Sprintf("%d", targetID)), postgresinfra.AuditMeta{"source_material_id": candidate.SourceMaterialID, "target_material_id": targetID, "owner_customer_id": candidate.OwnerCustomerID, "batch_ids": batchIDs, "manifest_id": report.ManifestID, "financial_document_created": false}); err != nil {
			return report, err
		}
		report.Applied++
	}
	if err := tx.Commit(ctx); err != nil {
		return report, err
	}
	return report, nil
}

func (r Repository) RollbackMaterialOwnershipCutover(ctx context.Context, actor string) (MaterialOwnershipCutoverReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return MaterialOwnershipCutoverReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":material-owner-cutover"); err != nil {
		return MaterialOwnershipCutoverReport{}, err
	}
	report, err := r.previewMaterialOwnershipCutoverTx(ctx, tx)
	if err != nil {
		return report, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "material-owner-cutover"
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,source_material_id,target_material_id,owner_customer_id,target_code FROM %s.material_owner_migrations WHERE status='applied' ORDER BY id DESC FOR UPDATE`, r.schema))
	if err != nil {
		return report, err
	}
	type mapping struct {
		id, source, target, owner int64
		code                      string
	}
	mappings := []mapping{}
	for rows.Next() {
		var m mapping
		if err := rows.Scan(&m.id, &m.source, &m.target, &m.owner, &m.code); err != nil {
			rows.Close()
			return report, err
		}
		mappings = append(mappings, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return report, err
	}
	rows.Close()
	for _, m := range mappings {
		var extraBatches, bomRefs, draftRefs int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.material_batches b WHERE b.material_id=$1 AND NOT EXISTS(SELECT 1 FROM %s.material_owner_migration_batches mb WHERE mb.material_batch_id=b.id AND mb.migration_id=$2)`, r.schema, r.schema), m.target, m.id).Scan(&extraBatches); err != nil {
			return report, err
		}
		if itemsOK, _ := materialCutoverTableExistsTx(ctx, tx, r.schema, "production_bom_version_items"); itemsOK {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_version_items WHERE material_id=$1`, r.schema), m.target).Scan(&bomRefs); err != nil {
				return report, err
			}
		}
		if bomsOK, _ := materialCutoverTableExistsTx(ctx, tx, r.schema, "production_boms"); bomsOK {
			var directBomRefs int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE output_material_id=$1 OR main_input_material_id=$1`, r.schema), m.target).Scan(&directBomRefs); err != nil {
				return report, err
			}
			bomRefs += directBomRefs
		}
		if ok, _ := materialCutoverTableExistsTx(ctx, tx, r.schema, "stock_entry_items"); ok {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE i.material_id=$1 AND e.status='draft'`, r.schema), m.target).Scan(&draftRefs); err != nil {
				return report, err
			}
		}
		if extraBatches+bomRefs+draftRefs > 0 {
			return report, fmt.Errorf("目标物料 %d 已产生新批次或业务引用，不能回滚", m.target)
		}
		var batchIDs []int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(array_agg(material_batch_id ORDER BY material_batch_id),'{}'::bigint[]) FROM %s.material_owner_migration_batches WHERE migration_id=$1`, r.schema), m.id).Scan(&batchIDs); err != nil {
			return report, err
		}
		if len(batchIDs) > 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET material_id=$2 WHERE id=ANY($1)`, r.schema), batchIDs, m.source); err != nil {
				return report, err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batch_locations SET material_id=$2,updated_at=now() WHERE material_batch_id=ANY($1)`, r.schema), batchIDs, m.source); err != nil {
				return report, err
			}
		}
		if err := recomputeCutoverMaterialOnhandTx(ctx, tx, r.schema, m.source); err != nil {
			return report, err
		}
		if err := recomputeCutoverMaterialOnhandTx(ctx, tx, r.schema, m.target); err != nil {
			return report, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET deprecated_at=now(),updated_at=now() WHERE id=$1`, r.schema), m.target); err != nil {
			return report, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_owner_migrations SET status='rolled_back',rolled_back_by=$2,rolled_back_at=now(),updated_at=now() WHERE id=$1`, r.schema), m.id, actor); err != nil {
			return report, err
		}
		if ok, _ := materialCutoverTableExistsTx(ctx, tx, r.schema, "business_group_assignments"); ok {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_group_assignments WHERE lower(usage_key)='material_catalog' AND lower(object_key)='material' AND object_id=$1 AND object_ref=''`, r.schema), m.target); err != nil {
				return report, err
			}
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "material_owner_migration", &m.id, "rollback_customer_inventory_split", postgresinfra.StrPtr("material_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", m.target)), postgresinfra.StrPtr(fmt.Sprintf("%d", m.source)), postgresinfra.AuditMeta{"source_material_id": m.source, "target_material_id": m.target, "owner_customer_id": m.owner, "batch_ids": batchIDs}); err != nil {
			return report, err
		}
		report.RolledBack++
	}
	if err := tx.Commit(ctx); err != nil {
		return report, err
	}
	return report, nil
}

func copyMaterialOwnershipProfilesTx(ctx context.Context, tx pgx.Tx, schema string, sourceID, targetID int64, actor string) error {
	statements := []struct {
		sql  string
		args []any
	}{
		{fmt.Sprintf(`INSERT INTO %[1]s.material_bean_profiles(material_id,origin,processing_station,variety,process_method,grade,altitude,flavor,bean_list_note,updated_at) SELECT $2,origin,processing_station,variety,process_method,grade,altitude,flavor,bean_list_note,now() FROM %[1]s.material_bean_profiles WHERE material_id=$1`, schema), []any{sourceID, targetID}},
		{fmt.Sprintf(`INSERT INTO %[1]s.material_pack_profiles(material_id,size_spec,dimensions,material_texture,capacity,color,note,updated_at) SELECT $2,size_spec,dimensions,material_texture,capacity,color,note,now() FROM %[1]s.material_pack_profiles WHERE material_id=$1`, schema), []any{sourceID, targetID}},
		{fmt.Sprintf(`INSERT INTO %[1]s.material_industry_field_values(material_id,field_key,value_text,created_at,updated_at,updated_by) SELECT $2,field_key,value_text,now(),now(),$3 FROM %[1]s.material_industry_field_values WHERE material_id=$1`, schema), []any{sourceID, targetID, actor}},
		{fmt.Sprintf(`INSERT INTO %[1]s.material_classification_assignments(material_id,group_id,category_id,updated_at,updated_by) SELECT $2,group_id,category_id,now(),$3 FROM %[1]s.material_classification_assignments WHERE material_id=$1`, schema), []any{sourceID, targetID, actor}},
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			return err
		}
	}
	if ok, err := materialCutoverTableExistsTx(ctx, tx, schema, "business_group_assignments"); err != nil {
		return err
	} else if ok {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %[1]s.business_group_assignments(
				group_id,group_item_id,usage_key,object_key,object_id,object_ref,sort_order,created_by,updated_by
			)
			SELECT group_id,group_item_id,usage_key,object_key,$2,object_ref,sort_order,$3,$3
			FROM %[1]s.business_group_assignments
			WHERE lower(usage_key)='material_catalog' AND lower(object_key)='material' AND object_id=$1 AND object_ref=''
			ON CONFLICT DO NOTHING
		`, schema), sourceID, targetID, actor); err != nil {
			return err
		}
	}
	return nil
}

func recomputeCutoverMaterialOnhandTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.materials m SET onhand_g=COALESCE(x.qty_g,0),onhand_units=COALESCE(x.qty_units,0),updated_at=now() FROM (SELECT $1::bigint material_id,COALESCE(SUM(qty_g),0)::bigint qty_g,COALESCE(SUM(qty_units),0)::bigint qty_units FROM %[1]s.material_batch_locations WHERE material_id=$1) x WHERE m.id=x.material_id`, schema), materialID)
	return err
}

func materialCutoverTableExistsTx(ctx context.Context, tx pgx.Tx, schema, table string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+table).Scan(&exists)
	return exists, err
}
