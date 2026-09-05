package production

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

const PR629CutoverVersion = "pr629-warehouse-source-v1"

type PR629BackupEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type PR629CutoverReport struct {
	Mode                         string              `json:"mode"`
	ManifestID                   string              `json:"manifest_id"`
	ProductReferenceSourceColumn bool                `json:"product_reference_source_column"`
	ProductReferenceSourceRows   int64               `json:"product_reference_source_rows"`
	HistoricalOrderItemRows      int64               `json:"historical_order_item_rows"`
	UnmigratedCustomerDemands    int64               `json:"unmigrated_customer_demands"`
	MigratedCustomerDemands      int64               `json:"migrated_customer_demands"`
	LegacyDraftPlanIDs           []int64             `json:"legacy_draft_plan_ids"`
	ActiveLegacyWorkOrderIDs     []int64             `json:"active_legacy_work_order_ids"`
	Blockers                     []string            `json:"blockers"`
	MaterialLocationG            int64               `json:"material_location_g"`
	MaterialLocationUnits        int64               `json:"material_location_units"`
	FinishedBatchG               int64               `json:"finished_batch_g"`
	FinishedBatchUnits           int64               `json:"finished_batch_units"`
	ReservedG                    int64               `json:"reserved_g"`
	ReservedUnits                int64               `json:"reserved_units"`
	Backup                       PR629BackupEvidence `json:"backup,omitempty"`
}

func (r Repository) PreviewPR629Cutover(ctx context.Context) (PR629CutoverReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return PR629CutoverReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	report, err := r.previewPR629CutoverTx(ctx, tx, "preview")
	if err != nil {
		return PR629CutoverReport{}, err
	}
	return report, tx.Commit(ctx)
}

func (r Repository) VerifyPR629Cutover(ctx context.Context) (PR629CutoverReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return PR629CutoverReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	report, err := r.previewPR629CutoverTx(ctx, tx, "verify")
	if err != nil {
		return PR629CutoverReport{}, err
	}
	if report.ProductReferenceSourceColumn || report.UnmigratedCustomerDemands > 0 || len(report.LegacyDraftPlanIDs) > 0 || len(report.Blockers) > 0 {
		return report, fmt.Errorf("PR-629 cutover verification failed")
	}
	return report, tx.Commit(ctx)
}

func (r Repository) ApplyPR629Cutover(ctx context.Context, confirm, actor string, backup PR629BackupEvidence) (PR629CutoverReport, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PR629CutoverReport{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, PR629CutoverVersion); err != nil {
		return PR629CutoverReport{}, err
	}
	before, err := r.previewPR629CutoverTx(ctx, tx, "apply")
	if err != nil {
		return PR629CutoverReport{}, err
	}
	if strings.TrimSpace(confirm) == "" || confirm != before.ManifestID {
		return PR629CutoverReport{}, fmt.Errorf("--confirm %s is required", before.ManifestID)
	}
	if len(before.Blockers) > 0 {
		return before, fmt.Errorf("PR-629 cutover blocked: %s", strings.Join(before.Blockers, "; "))
	}
	if !before.ProductReferenceSourceColumn && before.UnmigratedCustomerDemands == 0 && len(before.LegacyDraftPlanIDs) == 0 {
		var priorBackup PR629BackupEvidence
		var auditTableExists bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, r.schema+".pr629_warehouse_source_cutover_runs").Scan(&auditTableExists); err != nil {
			return before, err
		}
		if auditTableExists {
			_ = tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT backup_path,backup_sha256,backup_size
				FROM %s.pr629_warehouse_source_cutover_runs ORDER BY id DESC LIMIT 1
			`, r.schema)).Scan(&priorBackup.Path, &priorBackup.SHA256, &priorBackup.Size)
		}
		if priorBackup.Path != "" {
			before.Backup = priorBackup
			if err := tx.Commit(ctx); err != nil {
				return PR629CutoverReport{}, err
			}
			return before, nil
		}
	}
	if strings.TrimSpace(backup.Path) == "" || len(strings.TrimSpace(backup.SHA256)) != 64 || backup.Size <= 0 {
		return before, fmt.Errorf("verified full database backup evidence is required")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %[1]s.order_items ALTER COLUMN material_source_mode DROP NOT NULL;
		ALTER TABLE %[1]s.order_items ALTER COLUMN material_source_mode DROP DEFAULT;
		ALTER TABLE %[1]s.customer_order_production_demands ADD COLUMN IF NOT EXISTS migrated_at TIMESTAMPTZ;
		ALTER TABLE %[1]s.customer_order_production_demands ADD COLUMN IF NOT EXISTS migration_note TEXT NOT NULL DEFAULT '';
		CREATE TABLE IF NOT EXISTS %[1]s.pr629_warehouse_source_cutover_runs(
			id BIGSERIAL PRIMARY KEY,manifest_id TEXT NOT NULL UNIQUE,actor TEXT NOT NULL DEFAULT '',
			backup_path TEXT NOT NULL,backup_sha256 TEXT NOT NULL,backup_size BIGINT NOT NULL,
			report_json JSONB NOT NULL DEFAULT '{}'::jsonb,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`, r.schema)); err != nil {
		return PR629CutoverReport{}, err
	}
	for _, planID := range before.LegacyDraftPlanIDs {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET status='cancelled',cancelled_at=now() WHERE id=$1 AND status='draft'`, r.schema), planID); err != nil {
			return PR629CutoverReport{}, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "production_plan", &planID, "pr629_reenter_unified_demand",
			postgresinfra.StrPtr("status"), postgresinfra.StrPtr("draft"), postgresinfra.StrPtr("cancelled"),
			postgresinfra.AuditMeta{"migration": PR629CutoverVersion, "reason": "重新进入统一需求并等待逐组件选择来源仓"}); err != nil {
			return PR629CutoverReport{}, err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_order_production_demands
		SET migration_note=concat_ws(';',NULLIF(migration_note,''),'legacy_status='||status,'cutover='||$1),
		    status='migrated',migrated_at=now(),updated_at=now()
		WHERE migrated_at IS NULL
	`, r.schema), PR629CutoverVersion); err != nil {
		return PR629CutoverReport{}, err
	}
	if before.ProductReferenceSourceColumn {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.product_customer_references DROP COLUMN material_source_mode`, r.schema)); err != nil {
			return PR629CutoverReport{}, err
		}
	}
	after, err := r.previewPR629CutoverTx(ctx, tx, "apply")
	if err != nil {
		return PR629CutoverReport{}, err
	}
	after.Backup = backup
	if before.MaterialLocationG != after.MaterialLocationG || before.MaterialLocationUnits != after.MaterialLocationUnits ||
		before.FinishedBatchG != after.FinishedBatchG || before.FinishedBatchUnits != after.FinishedBatchUnits ||
		before.ReservedG != after.ReservedG || before.ReservedUnits != after.ReservedUnits {
		return PR629CutoverReport{}, fmt.Errorf("inventory or reservation totals changed during PR-629 cutover")
	}
	encoded, _ := json.Marshal(after)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.pr629_warehouse_source_cutover_runs(manifest_id,actor,backup_path,backup_sha256,backup_size,report_json)
		VALUES($1,$2,$3,$4,$5,$6::jsonb)
		ON CONFLICT(manifest_id) DO UPDATE SET report_json=excluded.report_json
	`, r.schema), before.ManifestID, strings.TrimSpace(actor), backup.Path, strings.ToLower(backup.SHA256), backup.Size, encoded); err != nil {
		return PR629CutoverReport{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PR629CutoverReport{}, err
	}
	return after, nil
}

func (r Repository) previewPR629CutoverTx(ctx context.Context, tx pgx.Tx, mode string) (PR629CutoverReport, error) {
	report := PR629CutoverReport{Mode: mode, LegacyDraftPlanIDs: []int64{}, ActiveLegacyWorkOrderIDs: []int64{}, Blockers: []string{}}
	var err error
	report.ProductReferenceSourceColumn, err = schemaColumnExistsTx(ctx, tx, r.schema, "product_customer_references", "material_source_mode")
	if err != nil {
		return report, err
	}
	if report.ProductReferenceSourceColumn {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::bigint FROM %s.product_customer_references WHERE COALESCE(material_source_mode,'')<>''`, r.schema)).Scan(&report.ProductReferenceSourceRows); err != nil {
			return report, err
		}
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::bigint FROM %s.order_items WHERE material_source_mode IS NOT NULL`, r.schema)).Scan(&report.HistoricalOrderItemRows); err != nil {
		return report, err
	}
	hasMigratedAt, err := schemaColumnExistsTx(ctx, tx, r.schema, "customer_order_production_demands", "migrated_at")
	if err != nil {
		return report, err
	}
	if hasMigratedAt {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FILTER(WHERE migrated_at IS NULL)::bigint,COUNT(*) FILTER(WHERE migrated_at IS NOT NULL)::bigint FROM %s.customer_order_production_demands`, r.schema)).Scan(&report.UnmigratedCustomerDemands, &report.MigratedCustomerDemands)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::bigint,0::bigint FROM %s.customer_order_production_demands`, r.schema)).Scan(&report.UnmigratedCustomerDemands, &report.MigratedCustomerDemands)
	}
	if err != nil {
		return report, err
	}
	legacyPlanFilter := "true"
	legacyWorkOrderFilter := "true"
	if hasMigratedAt {
		legacyPlanFilter = "(d.migrated_at IS NULL OR plan.created_at <= d.migrated_at)"
		legacyWorkOrderFilter = "(d.migrated_at IS NULL OR wo.created_at <= d.migrated_at)"
	}
	planRows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT plan.id
		FROM %s.production_plans plan
		JOIN %s.production_plan_items item ON item.production_plan_id=plan.id
		JOIN %s.customer_order_production_demands d ON true
		JOIN %s.orders o ON o.id=d.order_id
		WHERE plan.status='draft' AND o.order_no=ANY(regexp_split_to_array(COALESCE(item.order_nos,''),'\\s*[,，;；\\n]+\\s*'))
		  AND %s
		ORDER BY plan.id
	`, r.schema, r.schema, r.schema, r.schema, legacyPlanFilter))
	if err != nil {
		return report, err
	}
	for planRows.Next() {
		var id int64
		if err := planRows.Scan(&id); err != nil {
			planRows.Close()
			return report, err
		}
		report.LegacyDraftPlanIDs = append(report.LegacyDraftPlanIDs, id)
	}
	if err := planRows.Err(); err != nil {
		planRows.Close()
		return report, err
	}
	planRows.Close()
	workRows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT wo.id
		FROM %s.work_orders wo
		JOIN %s.production_plan_items item ON item.id=wo.production_plan_item_id
		JOIN %s.customer_order_production_demands d ON true
		JOIN %s.orders o ON o.id=d.order_id
		WHERE wo.status IN ('released','running','partially_completed','paused')
		  AND o.order_no=ANY(regexp_split_to_array(COALESCE(item.order_nos,''),'\\s*[,，;；\\n]+\\s*'))
		  AND %s
		ORDER BY wo.id
	`, r.schema, r.schema, r.schema, r.schema, legacyWorkOrderFilter))
	if err != nil {
		return report, err
	}
	for workRows.Next() {
		var id int64
		if err := workRows.Scan(&id); err != nil {
			workRows.Close()
			return report, err
		}
		report.ActiveLegacyWorkOrderIDs = append(report.ActiveLegacyWorkOrderIDs, id)
		report.Blockers = append(report.Blockers, fmt.Sprintf("执行中旧客户工单 %d 无法从商品供料方式唯一还原逐组件来源仓、货主和批次", id))
	}
	if err := workRows.Err(); err != nil {
		workRows.Close()
		return report, err
	}
	workRows.Close()
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0)::bigint,COALESCE(SUM(qty_units),0)::bigint FROM %s.material_batch_locations`, r.schema)).Scan(&report.MaterialLocationG, &report.MaterialLocationUnits); err != nil {
		return report, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(remaining_g),0)::bigint,COALESCE(SUM(remaining_units),0)::bigint FROM %s.stock_batches WHERE item_type='finished_product'`, r.schema)).Scan(&report.FinishedBatchG, &report.FinishedBatchUnits); err != nil {
		return report, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g)),0)::bigint,COALESCE(SUM(GREATEST(0,reserved_units-consumed_units-returned_units)),0)::bigint FROM %s.work_order_material_reservations WHERE status='reserved'`, r.schema)).Scan(&report.ReservedG, &report.ReservedUnits); err != nil {
		return report, err
	}
	sort.Slice(report.LegacyDraftPlanIDs, func(i, j int) bool { return report.LegacyDraftPlanIDs[i] < report.LegacyDraftPlanIDs[j] })
	report.ManifestID = fmt.Sprintf("%s-r%d-o%d-d%d-p%d-w%d", PR629CutoverVersion, report.ProductReferenceSourceRows,
		report.HistoricalOrderItemRows, report.UnmigratedCustomerDemands, len(report.LegacyDraftPlanIDs), len(report.ActiveLegacyWorkOrderIDs))
	return report, nil
}
