package manufacturing

import (
	"context"
	"fmt"
	"strings"

	manufacturingapp "orderapp/internal/application/manufacturing"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) ListIndustryTemplates(ctx context.Context) ([]manufacturingapp.IndustryFieldTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,name,industry_key,description,status,
		       to_char(created_at,'YYYY-MM-DD HH24:MI'),
		       to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.industry_field_templates
		ORDER BY CASE WHEN status='active' THEN 0 ELSE 1 END, updated_at DESC, id DESC
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]manufacturingapp.IndustryFieldTemplate, 0)
	for rows.Next() {
		var row manufacturingapp.IndustryFieldTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.IndustryKey, &row.Description, &row.Status, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachIndustryFields(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) attachIndustryFields(ctx context.Context, templates []manufacturingapp.IndustryFieldTemplate) error {
	if len(templates) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(templates))
	index := map[int64]int{}
	for i, row := range templates {
		ids = append(ids, row.ID)
		index[row.ID] = i
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,template_id,field_key,label,field_type,unit,required,
		       COALESCE(options_json,'[]'::jsonb)::text,sort_order
		FROM %s.industry_field_definitions
		WHERE template_id = ANY($1)
		ORDER BY template_id, sort_order, id
	`, r.schema), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var field manufacturingapp.IndustryFieldDefinition
		if err := rows.Scan(&field.ID, &field.TemplateID, &field.FieldKey, &field.Label, &field.FieldType, &field.Unit, &field.Required, &field.OptionsJSON, &field.SortOrder); err != nil {
			return err
		}
		if i, ok := index[field.TemplateID]; ok {
			templates[i].Fields = append(templates[i].Fields, field)
		}
	}
	return rows.Err()
}

func (r Repository) SaveIndustryTemplate(ctx context.Context, cmd manufacturingapp.SaveIndustryTemplateCommand) (manufacturingapp.IndustryFieldTemplate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return manufacturingapp.IndustryFieldTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	action := "create"
	var id int64
	if cmd.ID > 0 {
		action = "update"
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.industry_field_templates
			SET name=$2, industry_key=$3, description=$4, status=$5, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.IndustryKey, cmd.Description, cmd.Status).Scan(&id)
		if err != nil {
			return manufacturingapp.IndustryFieldTemplate{}, err
		}
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.industry_field_templates(name,industry_key,description,status,created_at,updated_at)
			VALUES($1,$2,$3,$4,now(),now())
			RETURNING id
		`, r.schema), cmd.Name, cmd.IndustryKey, cmd.Description, cmd.Status).Scan(&id)
		if err != nil {
			return manufacturingapp.IndustryFieldTemplate{}, err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.industry_field_definitions WHERE template_id=$1`, r.schema), id); err != nil {
		return manufacturingapp.IndustryFieldTemplate{}, err
	}
	for _, field := range cmd.Fields {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.industry_field_definitions(
				template_id,field_key,label,field_type,unit,required,options_json,sort_order,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8,now(),now())
		`, r.schema), id, field.FieldKey, field.Label, field.FieldType, field.Unit, field.Required, field.OptionsJSON, field.SortOrder); err != nil {
			return manufacturingapp.IndustryFieldTemplate{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "industry_field_template", &id, action, postgresinfra.StrPtr("template"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"industry_key": cmd.IndustryKey, "status": cmd.Status, "field_count": len(cmd.Fields)}); err != nil {
		return manufacturingapp.IndustryFieldTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return manufacturingapp.IndustryFieldTemplate{}, err
	}
	return r.industryTemplateByID(ctx, id)
}

func (r Repository) industryTemplateByID(ctx context.Context, id int64) (manufacturingapp.IndustryFieldTemplate, error) {
	rows, err := r.ListIndustryTemplates(ctx)
	if err != nil {
		return manufacturingapp.IndustryFieldTemplate{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return manufacturingapp.IndustryFieldTemplate{}, pgx.ErrNoRows
}

func (r Repository) DeactivateIndustryTemplate(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.industry_field_templates SET status='inactive', updated_at=now() WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "industry_field_template", &cmd.ID, "deactivate", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"template_id": cmd.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListProcessTemplates(ctx context.Context, query manufacturingapp.ProcessTemplateQuery) ([]manufacturingapp.ProcessTemplate, error) {
	args := []any{}
	where := "1=1"
	if query.ProductID > 0 {
		args = append(args, query.ProductID)
		where += fmt.Sprintf(" AND pt.product_id=$%d", len(args))
	}
	if strings.TrimSpace(query.Status) != "" {
		args = append(args, strings.TrimSpace(query.Status))
		where += fmt.Sprintf(" AND pt.status=$%d", len(args))
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT pt.id,pt.name,pt.product_id,COALESCE(p.name,''),
		       pt.bom_version_id,COALESCE(bv.version_no,''),
		       pt.industry_template_id,COALESCE(ift.name,''),
		       pt.status,pt.default_equipment,pt.default_minutes,
		       COALESCE(pt.key_params_json,'{}'::jsonb)::text,pt.note,
		       to_char(pt.created_at,'YYYY-MM-DD HH24:MI'),
		       to_char(pt.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %[1]s.process_templates pt
		LEFT JOIN %[1]s.products p ON p.id=pt.product_id
		LEFT JOIN %[1]s.bom_versions bv ON bv.id=pt.bom_version_id
		LEFT JOIN %[1]s.industry_field_templates ift ON ift.id=pt.industry_template_id
		WHERE %[2]s
		ORDER BY CASE WHEN pt.status='active' THEN 0 WHEN pt.status='draft' THEN 1 ELSE 2 END,
		         pt.updated_at DESC, pt.id DESC
	`, r.schema, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]manufacturingapp.ProcessTemplate, 0)
	for rows.Next() {
		var row manufacturingapp.ProcessTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.ProductID, &row.ProductName, &row.BomVersionID, &row.BomVersionNo, &row.IndustryTemplateID, &row.IndustryTemplateName, &row.Status, &row.DefaultEquipment, &row.DefaultMinutes, &row.KeyParamsJSON, &row.Note, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachProcessOperations(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) attachProcessOperations(ctx context.Context, templates []manufacturingapp.ProcessTemplate) error {
	if len(templates) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(templates))
	index := map[int64]int{}
	for i, row := range templates {
		ids = append(ids, row.ID)
		index[row.ID] = i
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,template_id,seq,operation,workstation,default_equipment,default_minutes,records_loss,
		       COALESCE(parameter_schema_json,'{}'::jsonb)::text,
		       COALESCE(quality_checklist_json,'[]'::jsonb)::text
		FROM %s.process_template_operations
		WHERE template_id = ANY($1)
		ORDER BY template_id, seq, id
	`, r.schema), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var op manufacturingapp.ProcessTemplateOperation
		if err := rows.Scan(&op.ID, &op.TemplateID, &op.Seq, &op.Operation, &op.Workstation, &op.DefaultEquipment, &op.DefaultMinutes, &op.RecordsLoss, &op.ParameterSchemaJSON, &op.QualityChecklistJSON); err != nil {
			return err
		}
		if i, ok := index[op.TemplateID]; ok {
			templates[i].Operations = append(templates[i].Operations, op)
		}
	}
	return rows.Err()
}

func (r Repository) SaveProcessTemplate(ctx context.Context, cmd manufacturingapp.SaveProcessTemplateCommand) (manufacturingapp.ProcessTemplate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return manufacturingapp.ProcessTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	action := "create"
	var id int64
	if cmd.ID > 0 {
		action = "update"
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.process_templates
			SET name=$2, product_id=$3, bom_version_id=$4, industry_template_id=$5,
			    status=$6, default_equipment=$7, default_minutes=$8,
			    key_params_json=$9::jsonb, note=$10, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.ProductID, cmd.BomVersionID, cmd.IndustryTemplateID, cmd.Status, cmd.DefaultEquipment, cmd.DefaultMinutes, cmd.KeyParamsJSON, cmd.Note).Scan(&id)
		if err != nil {
			return manufacturingapp.ProcessTemplate{}, err
		}
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.process_templates(
				name,product_id,bom_version_id,industry_template_id,status,default_equipment,default_minutes,key_params_json,note,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,now(),now())
			RETURNING id
		`, r.schema), cmd.Name, cmd.ProductID, cmd.BomVersionID, cmd.IndustryTemplateID, cmd.Status, cmd.DefaultEquipment, cmd.DefaultMinutes, cmd.KeyParamsJSON, cmd.Note).Scan(&id)
		if err != nil {
			return manufacturingapp.ProcessTemplate{}, err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.process_template_operations WHERE template_id=$1`, r.schema), id); err != nil {
		return manufacturingapp.ProcessTemplate{}, err
	}
	for _, op := range cmd.Operations {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.process_template_operations(
				template_id,seq,operation,workstation,default_equipment,default_minutes,records_loss,
				parameter_schema_json,quality_checklist_json,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,now(),now())
		`, r.schema), id, op.Seq, op.Operation, op.Workstation, op.DefaultEquipment, op.DefaultMinutes, op.RecordsLoss, op.ParameterSchemaJSON, op.QualityChecklistJSON); err != nil {
			return manufacturingapp.ProcessTemplate{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "process_template", &id, action, postgresinfra.StrPtr("template"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "bom_version_id": cmd.BomVersionID, "industry_template_id": cmd.IndustryTemplateID, "status": cmd.Status, "operation_count": len(cmd.Operations)}); err != nil {
		return manufacturingapp.ProcessTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return manufacturingapp.ProcessTemplate{}, err
	}
	return r.processTemplateByID(ctx, id)
}

func (r Repository) processTemplateByID(ctx context.Context, id int64) (manufacturingapp.ProcessTemplate, error) {
	rows, err := r.ListProcessTemplates(ctx, manufacturingapp.ProcessTemplateQuery{})
	if err != nil {
		return manufacturingapp.ProcessTemplate{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return manufacturingapp.ProcessTemplate{}, pgx.ErrNoRows
}

func (r Repository) PublishProcessTemplate(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id FROM %s.process_templates WHERE id=$1 FOR UPDATE`, r.schema), cmd.ID).Scan(&productID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.process_templates
		SET status='draft', updated_at=now()
		WHERE product_id=$1 AND id<>$2 AND status='active'
	`, r.schema), productID, cmd.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.process_templates SET status='active', updated_at=now() WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "process_template", &cmd.ID, "publish", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("active"), postgresinfra.AuditMeta{"product_id": productID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) DeactivateProcessTemplate(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.process_templates SET status='inactive', updated_at=now() WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "process_template", &cmd.ID, "deactivate", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"template_id": cmd.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
