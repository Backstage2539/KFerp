package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (r Repository) CreateQualityInspection(ctx context.Context, cmd productionapp.QualityInspectionCommand) (productionapp.QualityInspectionRow, error) {
	metricsJSON := strings.TrimSpace(cmd.MetricsJSON)
	if metricsJSON == "" {
		metricsJSON = "{}"
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.QualityInspectionRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var row productionapp.QualityInspectionRow
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.quality_inspections(
			scope,reference_type,reference_no,item_name,result,metrics_json,note,operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,$8,now())
		RETURNING id,scope,reference_type,reference_no,item_name,result,metrics_json::text,note,operator,to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema),
		cmd.Scope, cmd.ReferenceType, cmd.ReferenceNo, cmd.ItemName, cmd.Result, metricsJSON, cmd.Note, cmd.Operator,
	).Scan(&row.ID, &row.Scope, &row.ReferenceType, &row.ReferenceNo, &row.ItemName, &row.Result, &row.MetricsJSON, &row.Note, &row.Operator, &row.CreatedAt)
	if err != nil {
		return productionapp.QualityInspectionRow{}, err
	}
	if err := applyQualityInspectionStatusTx(ctx, tx, r.schema, cmd); err != nil {
		return productionapp.QualityInspectionRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.QualityInspectionRow{}, err
	}
	return row, nil
}

func applyQualityInspectionStatusTx(ctx context.Context, tx pgx.Tx, schema string, cmd productionapp.QualityInspectionCommand) error {
	status := qualityStatusForResult(cmd.Result)
	if status == "" {
		return nil
	}
	referenceNo := strings.TrimSpace(cmd.ReferenceNo)
	referenceType := strings.TrimSpace(cmd.ReferenceType)
	scope := strings.TrimSpace(cmd.Scope)
	if referenceNo == "" {
		return nil
	}

	switch {
	case referenceType == "work_order" || scope == "work_order":
		_, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.stock_batches b
			SET quality_status=$2
			FROM %s.work_orders wo
			WHERE wo.work_order_no=$1
			  AND b.item_type=$3
			  AND b.source_doc_id=wo.running_item_id
		`, schema, schema), referenceNo, status, stockItemTypeFinishedProduct)
		return ignoreMissingQualityTarget(err)
	case referenceType == "finished_batch" || scope == "finished_batch":
		_, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.stock_batches
			SET quality_status=$2
			WHERE batch_code=$1 AND item_type=$3
		`, schema), referenceNo, status, stockItemTypeFinishedProduct)
		return ignoreMissingQualityTarget(err)
	case referenceType == "raw_material" || scope == "raw_material" || referenceType == "material_batch" || scope == "material_batch":
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.material_batches
			SET quality_status=$2
			WHERE batch_code=$1
		`, schema), referenceNo, status); err != nil {
			if !isMissingQualityTableOrColumn(err) {
				return err
			}
		}
		_, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.stock_batches
			SET quality_status=$2
			WHERE batch_code=$1 AND item_type=$3
		`, schema), referenceNo, status, stockItemTypeMaterial)
		return ignoreMissingQualityTarget(err)
	default:
		return nil
	}
}

func qualityStatusForResult(result string) string {
	switch strings.TrimSpace(result) {
	case "pass":
		return "pass"
	case "hold":
		return "hold"
	case "reject":
		return "reject"
	default:
		return ""
	}
}

func ignoreMissingQualityTarget(err error) error {
	if err != nil && !isMissingQualityTableOrColumn(err) {
		return err
	}
	return nil
}

func isMissingQualityTableOrColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined_column") || strings.Contains(msg, "undefined_table")
}

func backfillQualityStatusesFromInspections(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	queries := []string{
		fmt.Sprintf(`
			WITH latest AS (
				SELECT DISTINCT ON (reference_type,reference_no)
				       reference_type,scope,reference_no,result
				FROM %s.quality_inspections
				WHERE reference_no <> ''
				  AND result IN ('pass','hold','reject')
				ORDER BY reference_type,reference_no,created_at DESC,id DESC
			)
			UPDATE %s.stock_batches b
			SET quality_status=latest.result
			FROM latest
			JOIN %s.work_orders wo ON wo.work_order_no=latest.reference_no
			WHERE (latest.reference_type='work_order' OR latest.scope='work_order')
			  AND b.item_type=$1
			  AND b.source_doc_id=wo.running_item_id
		`, schema, schema, schema),
		fmt.Sprintf(`
			WITH latest AS (
				SELECT DISTINCT ON (reference_type,reference_no)
				       reference_type,scope,reference_no,result
				FROM %s.quality_inspections
				WHERE reference_no <> ''
				  AND result IN ('pass','hold','reject')
				ORDER BY reference_type,reference_no,created_at DESC,id DESC
			)
			UPDATE %s.stock_batches b
			SET quality_status=latest.result
			FROM latest
			WHERE (latest.reference_type='finished_batch' OR latest.scope='finished_batch')
			  AND b.item_type=$1
			  AND b.batch_code=latest.reference_no
		`, schema, schema),
		fmt.Sprintf(`
			WITH latest AS (
				SELECT DISTINCT ON (reference_type,reference_no)
				       reference_type,scope,reference_no,result
				FROM %s.quality_inspections
				WHERE reference_no <> ''
				  AND result IN ('pass','hold','reject')
				ORDER BY reference_type,reference_no,created_at DESC,id DESC
			)
			UPDATE %s.material_batches b
			SET quality_status=latest.result
			FROM latest
			WHERE (latest.reference_type IN ('raw_material','material_batch') OR latest.scope IN ('raw_material','material_batch'))
			  AND b.batch_code=latest.reference_no
		`, schema, schema),
		fmt.Sprintf(`
			WITH latest AS (
				SELECT DISTINCT ON (reference_type,reference_no)
				       reference_type,scope,reference_no,result
				FROM %s.quality_inspections
				WHERE reference_no <> ''
				  AND result IN ('pass','hold','reject')
				ORDER BY reference_type,reference_no,created_at DESC,id DESC
			)
			UPDATE %s.stock_batches b
			SET quality_status=latest.result
			FROM latest
			WHERE (latest.reference_type IN ('raw_material','material_batch') OR latest.scope IN ('raw_material','material_batch'))
			  AND b.item_type=$1
			  AND b.batch_code=latest.reference_no
		`, schema, schema),
	}
	args := [][]any{
		{stockItemTypeFinishedProduct},
		{stockItemTypeFinishedProduct},
		nil,
		{stockItemTypeMaterial},
	}
	for i, q := range queries {
		if _, err := pool.Exec(ctx, q, args[i]...); err != nil && !isMissingQualityTableOrColumn(err) {
			return err
		}
	}
	return nil
}

func (r Repository) ListQualityInspections(ctx context.Context, query productionapp.QualityInspectionQuery) ([]productionapp.QualityInspectionRow, error) {
	args := []any{}
	where := "1=1"
	if strings.TrimSpace(query.Scope) != "" {
		args = append(args, query.Scope)
		where += fmt.Sprintf(" AND scope=$%d", len(args))
	}
	if strings.TrimSpace(query.Result) != "" {
		args = append(args, query.Result)
		where += fmt.Sprintf(" AND result=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,scope,reference_type,reference_no,item_name,result,metrics_json::text,note,operator,to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.quality_inspections
		WHERE %s
		ORDER BY created_at DESC,id DESC
		LIMIT $%d
	`, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.QualityInspectionRow, 0)
	for rows.Next() {
		var row productionapp.QualityInspectionRow
		if err := rows.Scan(&row.ID, &row.Scope, &row.ReferenceType, &row.ReferenceNo, &row.ItemName, &row.Result, &row.MetricsJSON, &row.Note, &row.Operator, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
