package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	"strings"
)

func (r Repository) CreateQualityInspection(ctx context.Context, cmd productionapp.QualityInspectionCommand) (productionapp.QualityInspectionRow, error) {
	metricsJSON := strings.TrimSpace(cmd.MetricsJSON)
	if metricsJSON == "" {
		metricsJSON = "{}"
	}
	var row productionapp.QualityInspectionRow
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
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
	return row, nil
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
