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

func (r Repository) ListManufacturingOperations(ctx context.Context) ([]manufacturingapp.ManufacturingOperation, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,code,name,status,default_minutes,COALESCE(standard_operation_cost,0)::float8,note,
		       to_char(created_at,'YYYY-MM-DD HH24:MI'),
		       to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.manufacturing_operations
		ORDER BY CASE WHEN status='active' THEN 0 ELSE 1 END, name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]manufacturingapp.ManufacturingOperation, 0)
	for rows.Next() {
		var row manufacturingapp.ManufacturingOperation
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.Status, &row.DefaultMinutes, &row.StandardOperationCost, &row.Note, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveManufacturingOperation(ctx context.Context, cmd manufacturingapp.SaveManufacturingOperationCommand) (manufacturingapp.ManufacturingOperation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return manufacturingapp.ManufacturingOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	action := "create"
	var id int64
	if cmd.ID > 0 {
		action = "update"
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.manufacturing_operations
			SET code=$2,name=$3,status=$4,default_minutes=$5,standard_operation_cost=$6,note=$7,updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Code, cmd.Name, cmd.Status, cmd.DefaultMinutes, cmd.StandardOperationCost, cmd.Note).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.manufacturing_operations(code,name,status,default_minutes,standard_operation_cost,note,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,now(),now())
			RETURNING id
		`, r.schema), cmd.Code, cmd.Name, cmd.Status, cmd.DefaultMinutes, cmd.StandardOperationCost, cmd.Note).Scan(&id)
	}
	if err != nil {
		return manufacturingapp.ManufacturingOperation{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "manufacturing_operation", &id, action, postgresinfra.StrPtr("operation"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"code": cmd.Code, "status": cmd.Status, "standard_operation_cost": cmd.StandardOperationCost}); err != nil {
		return manufacturingapp.ManufacturingOperation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return manufacturingapp.ManufacturingOperation{}, err
	}
	return r.manufacturingOperationByID(ctx, id)
}

func (r Repository) DeactivateManufacturingOperation(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.manufacturing_operations SET status='inactive', updated_at=now() WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "manufacturing_operation", &cmd.ID, "deactivate", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"id": cmd.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) manufacturingOperationByID(ctx context.Context, id int64) (manufacturingapp.ManufacturingOperation, error) {
	rows, err := r.ListManufacturingOperations(ctx)
	if err != nil {
		return manufacturingapp.ManufacturingOperation{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return manufacturingapp.ManufacturingOperation{}, pgx.ErrNoRows
}

func (r Repository) ListManufacturingWorkstations(ctx context.Context) ([]manufacturingapp.ManufacturingWorkstation, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,code,name,status,default_minutes,
		       COALESCE(machine_hourly_cost,0)::float8,
		       COALESCE(labor_hourly_cost,0)::float8,
		       COALESCE(overhead_hourly_cost,0)::float8,
		       COALESCE(hourly_rate,0)::float8,note,
		       to_char(created_at,'YYYY-MM-DD HH24:MI'),
		       to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.manufacturing_workstations
		ORDER BY CASE WHEN status='active' THEN 0 ELSE 1 END, name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]manufacturingapp.ManufacturingWorkstation, 0)
	for rows.Next() {
		var row manufacturingapp.ManufacturingWorkstation
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.Status, &row.DefaultMinutes, &row.MachineHourlyCost, &row.LaborHourlyCost, &row.OverheadHourlyCost, &row.HourlyRate, &row.Note, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachWorkstationOperations(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) attachWorkstationOperations(ctx context.Context, rows []manufacturingapp.ManufacturingWorkstation) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	index := map[int64]int{}
	for i := range rows {
		ids = append(ids, rows[i].ID)
		index[rows[i].ID] = i
	}
	opRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT wo.workstation_id,
		       o.id,o.code,o.name,o.status,o.default_minutes,COALESCE(o.standard_operation_cost,0)::float8,o.note,
		       to_char(o.created_at,'YYYY-MM-DD HH24:MI'),
		       to_char(o.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.manufacturing_workstation_operations wo
		JOIN %s.manufacturing_operations o ON o.id=wo.operation_id
		WHERE wo.workstation_id = ANY($1)
		ORDER BY wo.workstation_id, o.name, o.id
	`, r.schema, r.schema), ids)
	if err != nil {
		return err
	}
	defer opRows.Close()
	for opRows.Next() {
		var workstationID int64
		var op manufacturingapp.ManufacturingOperation
		if err := opRows.Scan(&workstationID, &op.ID, &op.Code, &op.Name, &op.Status, &op.DefaultMinutes, &op.StandardOperationCost, &op.Note, &op.CreatedAt, &op.UpdatedAt); err != nil {
			return err
		}
		i, ok := index[workstationID]
		if !ok {
			continue
		}
		rows[i].ApplicableOperationIDs = append(rows[i].ApplicableOperationIDs, op.ID)
		rows[i].ApplicableOperations = append(rows[i].ApplicableOperations, op)
	}
	return opRows.Err()
}

func (r Repository) SaveManufacturingWorkstation(ctx context.Context, cmd manufacturingapp.SaveManufacturingWorkstationCommand) (manufacturingapp.ManufacturingWorkstation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return manufacturingapp.ManufacturingWorkstation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	action := "create"
	var id int64
	if cmd.ID > 0 {
		action = "update"
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.manufacturing_workstations
			SET code=$2,name=$3,status=$4,default_minutes=$5,
			    machine_hourly_cost=$6,labor_hourly_cost=$7,overhead_hourly_cost=$8,hourly_rate=$9,
			    note=$10,updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Code, cmd.Name, cmd.Status, cmd.DefaultMinutes, cmd.MachineHourlyCost, cmd.LaborHourlyCost, cmd.OverheadHourlyCost, cmd.HourlyRate, cmd.Note).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.manufacturing_workstations(code,name,status,default_minutes,machine_hourly_cost,labor_hourly_cost,overhead_hourly_cost,hourly_rate,note,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,now(),now())
			RETURNING id
		`, r.schema), cmd.Code, cmd.Name, cmd.Status, cmd.DefaultMinutes, cmd.MachineHourlyCost, cmd.LaborHourlyCost, cmd.OverheadHourlyCost, cmd.HourlyRate, cmd.Note).Scan(&id)
	}
	if err != nil {
		return manufacturingapp.ManufacturingWorkstation{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.manufacturing_workstation_operations WHERE workstation_id=$1`, r.schema), id); err != nil {
		return manufacturingapp.ManufacturingWorkstation{}, err
	}
	for _, operationID := range cmd.ApplicableOperationIDs {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.manufacturing_workstation_operations(workstation_id, operation_id, created_at)
			VALUES($1,$2,now())
			ON CONFLICT (workstation_id, operation_id) DO NOTHING
		`, r.schema), id, operationID); err != nil {
			return manufacturingapp.ManufacturingWorkstation{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "manufacturing_workstation", &id, action, postgresinfra.StrPtr("workstation"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"code": cmd.Code, "status": cmd.Status, "machine_hourly_cost": cmd.MachineHourlyCost, "labor_hourly_cost": cmd.LaborHourlyCost, "overhead_hourly_cost": cmd.OverheadHourlyCost, "hourly_rate": cmd.HourlyRate, "applicable_operation_ids": cmd.ApplicableOperationIDs}); err != nil {
		return manufacturingapp.ManufacturingWorkstation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return manufacturingapp.ManufacturingWorkstation{}, err
	}
	return r.manufacturingWorkstationByID(ctx, id)
}

func (r Repository) DeactivateManufacturingWorkstation(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.manufacturing_workstations SET status='inactive', updated_at=now() WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "manufacturing_workstation", &cmd.ID, "deactivate", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"id": cmd.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) manufacturingWorkstationByID(ctx context.Context, id int64) (manufacturingapp.ManufacturingWorkstation, error) {
	rows, err := r.ListManufacturingWorkstations(ctx)
	if err != nil {
		return manufacturingapp.ManufacturingWorkstation{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return manufacturingapp.ManufacturingWorkstation{}, pgx.ErrNoRows
}

func (r Repository) ListManufacturingWorkstationCapacities(ctx context.Context, query manufacturingapp.WorkstationCapacityQuery) ([]manufacturingapp.ManufacturingWorkstationCapacity, error) {
	args := []any{}
	where := "1=1"
	if query.WorkstationID > 0 {
		args = append(args, query.WorkstationID)
		where += fmt.Sprintf(" AND c.workstation_id=$%d", len(args))
	}
	if strings.TrimSpace(query.Status) != "" {
		args = append(args, strings.TrimSpace(query.Status))
		where += fmt.Sprintf(" AND c.status=$%d", len(args))
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT c.id,c.workstation_id,COALESCE(w.name,''),c.code,c.name,c.status,
		       COALESCE(c.batch_size_qty,0)::float8,
		       COALESCE(c.batch_size_unit,''),
		       COALESCE(c.standard_minutes,0),
		       COALESCE(NULLIF(w.hourly_rate,0), c.hourly_rate, 0)::float8,
		       COALESCE(c.production_capacity,1),
		       COALESCE(c.sort_order,0),
		       COALESCE(c.note,''),
		       to_char(c.created_at,'YYYY-MM-DD HH24:MI'),
		       to_char(c.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.manufacturing_workstation_capacities c
		LEFT JOIN %s.manufacturing_workstations w ON w.id=c.workstation_id
		WHERE %s
		ORDER BY CASE WHEN c.status='active' THEN 0 ELSE 1 END, c.workstation_id, c.sort_order, c.name, c.id
	`, r.schema, r.schema, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]manufacturingapp.ManufacturingWorkstationCapacity, 0)
	for rows.Next() {
		var row manufacturingapp.ManufacturingWorkstationCapacity
		if err := rows.Scan(&row.ID, &row.WorkstationID, &row.Workstation, &row.Code, &row.Name, &row.Status, &row.BatchSizeQty, &row.BatchSizeUnit, &row.StandardMinutes, &row.HourlyRate, &row.ProductionCapacity, &row.SortOrder, &row.Note, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachWorkstationCapacityOperations(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) attachWorkstationCapacityOperations(ctx context.Context, rows []manufacturingapp.ManufacturingWorkstationCapacity) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	seen := map[int64]bool{}
	index := map[int64][]int{}
	for i := range rows {
		if rows[i].WorkstationID <= 0 {
			continue
		}
		index[rows[i].WorkstationID] = append(index[rows[i].WorkstationID], i)
		if !seen[rows[i].WorkstationID] {
			seen[rows[i].WorkstationID] = true
			ids = append(ids, rows[i].WorkstationID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	opRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT workstation_id, operation_id
		FROM %s.manufacturing_workstation_operations
		WHERE workstation_id = ANY($1)
		ORDER BY workstation_id, operation_id
	`, r.schema), ids)
	if err != nil {
		return err
	}
	defer opRows.Close()
	for opRows.Next() {
		var workstationID int64
		var operationID int64
		if err := opRows.Scan(&workstationID, &operationID); err != nil {
			return err
		}
		indexes, ok := index[workstationID]
		if !ok {
			continue
		}
		for _, i := range indexes {
			rows[i].ApplicableOperationIDs = append(rows[i].ApplicableOperationIDs, operationID)
		}
	}
	return opRows.Err()
}

func (r Repository) SaveManufacturingWorkstationCapacity(ctx context.Context, cmd manufacturingapp.SaveWorkstationCapacityCommand) (manufacturingapp.ManufacturingWorkstationCapacity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return manufacturingapp.ManufacturingWorkstationCapacity{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	action := "create"
	var id int64
	if cmd.ID > 0 {
		action = "update"
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.manufacturing_workstation_capacities
			SET workstation_id=$2,code=$3,name=$4,status=$5,batch_size_qty=$6,batch_size_unit=$7,
			    standard_minutes=$8,hourly_rate=$9,production_capacity=$10,sort_order=$11,note=$12,updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.WorkstationID, cmd.Code, cmd.Name, cmd.Status, cmd.BatchSizeQty, cmd.BatchSizeUnit, cmd.StandardMinutes, cmd.HourlyRate, cmd.ProductionCapacity, cmd.SortOrder, cmd.Note).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.manufacturing_workstation_capacities(
				workstation_id,code,name,status,batch_size_qty,batch_size_unit,standard_minutes,hourly_rate,production_capacity,sort_order,note,created_at,updated_at
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),now())
			RETURNING id
		`, r.schema), cmd.WorkstationID, cmd.Code, cmd.Name, cmd.Status, cmd.BatchSizeQty, cmd.BatchSizeUnit, cmd.StandardMinutes, cmd.HourlyRate, cmd.ProductionCapacity, cmd.SortOrder, cmd.Note).Scan(&id)
	}
	if err != nil {
		return manufacturingapp.ManufacturingWorkstationCapacity{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "manufacturing_workstation_capacity", &id, action, postgresinfra.StrPtr("workstation_capacity"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"workstation_id": cmd.WorkstationID, "batch_size_qty": cmd.BatchSizeQty, "batch_size_unit": cmd.BatchSizeUnit, "standard_minutes": cmd.StandardMinutes, "status": cmd.Status}); err != nil {
		return manufacturingapp.ManufacturingWorkstationCapacity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return manufacturingapp.ManufacturingWorkstationCapacity{}, err
	}
	return r.manufacturingWorkstationCapacityByID(ctx, id)
}

func (r Repository) DeactivateManufacturingWorkstationCapacity(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.manufacturing_workstation_capacities SET status='inactive', updated_at=now() WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "manufacturing_workstation_capacity", &cmd.ID, "deactivate", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"id": cmd.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) manufacturingWorkstationCapacityByID(ctx context.Context, id int64) (manufacturingapp.ManufacturingWorkstationCapacity, error) {
	rows, err := r.ListManufacturingWorkstationCapacities(ctx, manufacturingapp.WorkstationCapacityQuery{})
	if err != nil {
		return manufacturingapp.ManufacturingWorkstationCapacity{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return manufacturingapp.ManufacturingWorkstationCapacity{}, pgx.ErrNoRows
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
		SELECT id,template_id,seq,operation_id,workstation_id,
		       COALESCE(workstation_capacity_id,0),
		       operation,workstation,COALESCE(workstation_capacity_name,''),
		       default_equipment,default_minutes,
		       COALESCE(batch_size_qty,0)::float8,
		       COALESCE(batch_size_unit,''),
		       COALESCE(standard_minutes,0),
		       COALESCE(hourly_rate,0)::float8,
		       COALESCE(planned_batch_count,0),
		       COALESCE(planned_minutes,0),
		       COALESCE(planned_operation_cost,0)::float8,
		       records_loss,
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
		if err := rows.Scan(
			&op.ID, &op.TemplateID, &op.Seq, &op.OperationID, &op.WorkstationID, &op.WorkstationCapacityID,
			&op.Operation, &op.Workstation, &op.WorkstationCapacityName, &op.DefaultEquipment, &op.DefaultMinutes,
			&op.BatchSizeQty, &op.BatchSizeUnit, &op.StandardMinutes, &op.HourlyRate, &op.PlannedBatchCount, &op.PlannedMinutes, &op.PlannedOperationCost,
			&op.RecordsLoss, &op.ParameterSchemaJSON, &op.QualityChecklistJSON,
		); err != nil {
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
				template_id,seq,operation_id,workstation_id,workstation_capacity_id,operation,workstation,workstation_capacity_name,
				default_equipment,default_minutes,batch_size_qty,batch_size_unit,standard_minutes,hourly_rate,
				planned_batch_count,planned_minutes,planned_operation_cost,records_loss,
				parameter_schema_json,quality_checklist_json,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19::jsonb,$20::jsonb,now(),now())
		`, r.schema), id, op.Seq, op.OperationID, op.WorkstationID, op.WorkstationCapacityID, op.Operation, op.Workstation, op.WorkstationCapacityName, op.DefaultEquipment, op.DefaultMinutes, op.BatchSizeQty, op.BatchSizeUnit, op.StandardMinutes, op.HourlyRate, op.PlannedBatchCount, op.PlannedMinutes, op.PlannedOperationCost, op.RecordsLoss, op.ParameterSchemaJSON, op.QualityChecklistJSON); err != nil {
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

func (r Repository) ListProcessRoutes(ctx context.Context, query manufacturingapp.ProcessRouteQuery) ([]manufacturingapp.ProcessRoute, error) {
	args := []any{}
	where := "1=1"
	if strings.TrimSpace(query.Status) != "" {
		args = append(args, strings.TrimSpace(query.Status))
		where += fmt.Sprintf(" AND pr.status=$%d", len(args))
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT pr.id,pr.name,pr.status,pr.default_equipment,pr.default_minutes,pr.note,
		       to_char(pr.created_at,'YYYY-MM-DD HH24:MI'),
		       to_char(pr.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %[1]s.process_routes pr
		WHERE %[2]s
		ORDER BY CASE WHEN pr.status='active' THEN 0 WHEN pr.status='draft' THEN 1 ELSE 2 END,
		         pr.updated_at DESC, pr.id DESC
	`, r.schema, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]manufacturingapp.ProcessRoute, 0)
	for rows.Next() {
		var row manufacturingapp.ProcessRoute
		if err := rows.Scan(&row.ID, &row.Name, &row.Status, &row.DefaultEquipment, &row.DefaultMinutes, &row.Note, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachProcessRouteOperations(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) attachProcessRouteOperations(ctx context.Context, routes []manufacturingapp.ProcessRoute) error {
	if len(routes) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(routes))
	index := map[int64]int{}
	for i, row := range routes {
		ids = append(ids, row.ID)
		index[row.ID] = i
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT pro.id,pro.route_id,pro.seq,pro.operation_id,pro.workstation_id,
		       COALESCE(pro.workstation_capacity_id,0),
		       pro.operation,pro.workstation,COALESCE(pro.workstation_capacity_name,''),
		       pro.default_equipment,pro.default_minutes,
		       COALESCE(pro.batch_size_qty,0)::float8,
		       COALESCE(pro.batch_size_unit,''),
		       COALESCE(pro.standard_minutes,0),
		       COALESCE(pro.hourly_rate,0)::float8,
		       COALESCE(pro.planned_batch_count,0),
		       COALESCE(pro.planned_minutes,0),
		       COALESCE(pro.planned_operation_cost,0)::float8,
		       pro.records_loss,
		       COALESCE(pro.quality_checklist_json,'[]'::jsonb)::text,
		       0::bigint,
		       ''::text,
		       ''::text,
		       0::float8,
		       ''::text,
		       0::float8,
		       0::float8
		FROM %s.process_route_operations pro
		WHERE pro.route_id = ANY($1)
		ORDER BY pro.route_id, pro.seq, pro.id
		`, r.schema), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var op manufacturingapp.ProcessRouteOperation
		var standardOutputQty float64
		var standardOutputUnit string
		var standardMinutes float64
		var hourlyRate float64
		if err := rows.Scan(
			&op.ID, &op.RouteID, &op.Seq, &op.OperationID, &op.WorkstationID, &op.WorkstationCapacityID,
			&op.Operation, &op.Workstation, &op.WorkstationCapacityName, &op.DefaultEquipment, &op.DefaultMinutes,
			&op.BatchSizeQty, &op.BatchSizeUnit, &op.StandardMinutes, &op.HourlyRate, &op.PlannedBatchCount, &op.PlannedMinutes, &op.PlannedOperationCost,
			&op.RecordsLoss, &op.QualityChecklistJSON,
			&op.StandardCostCapacityID, &op.StandardCostCapacityName, &op.StandardCostWorkstation,
			&standardOutputQty, &standardOutputUnit, &standardMinutes, &hourlyRate,
		); err != nil {
			return err
		}
		op.StandardCostCapacityID = 0
		op.StandardCostCapacityName = ""
		op.StandardCostWorkstation = ""
		op.StandardCostSummary = ""
		if i, ok := index[op.RouteID]; ok {
			routes[i].Operations = append(routes[i].Operations, op)
		}
	}
	return rows.Err()
}

func (r Repository) SaveProcessRoute(ctx context.Context, cmd manufacturingapp.SaveProcessRouteCommand) (manufacturingapp.ProcessRoute, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return manufacturingapp.ProcessRoute{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	action := "create"
	var id int64
	if cmd.ID > 0 {
		action = "update"
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.process_routes
			SET name=$2, status=$3, default_equipment=$4, default_minutes=$5, note=$6, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.Status, cmd.DefaultEquipment, cmd.DefaultMinutes, cmd.Note).Scan(&id)
		if err != nil {
			return manufacturingapp.ProcessRoute{}, err
		}
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.process_routes(name,status,default_equipment,default_minutes,note,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,now(),now())
			RETURNING id
		`, r.schema), cmd.Name, cmd.Status, cmd.DefaultEquipment, cmd.DefaultMinutes, cmd.Note).Scan(&id)
		if err != nil {
			return manufacturingapp.ProcessRoute{}, err
		}
	}
	oldStandardCostCapacityIDs := "{}"
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(jsonb_object_agg(seq, standard_cost_capacity_id ORDER BY seq), '{}'::jsonb)::text
			FROM %s.process_route_operations
			WHERE route_id=$1
		`, r.schema), id).Scan(&oldStandardCostCapacityIDs); err != nil {
			return manufacturingapp.ProcessRoute{}, err
		}
	}
	newStandardCostCapacityIDs := map[string]int64{}
	for _, op := range cmd.Operations {
		newStandardCostCapacityIDs[fmt.Sprintf("%d", op.Seq)] = op.StandardCostCapacityID
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.process_route_operations WHERE route_id=$1`, r.schema), id); err != nil {
		return manufacturingapp.ProcessRoute{}, err
	}
	for _, op := range cmd.Operations {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.process_route_operations(
				route_id,seq,operation_id,workstation_id,workstation_capacity_id,standard_cost_capacity_id,operation,workstation,workstation_capacity_name,
				default_equipment,default_minutes,batch_size_qty,batch_size_unit,standard_minutes,hourly_rate,
				planned_batch_count,planned_minutes,planned_operation_cost,records_loss,
				quality_checklist_json,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20::jsonb,now(),now())
		`, r.schema), id, op.Seq, op.OperationID, op.WorkstationID, op.WorkstationCapacityID, op.StandardCostCapacityID, op.Operation, op.Workstation, op.WorkstationCapacityName, op.DefaultEquipment, op.DefaultMinutes, op.BatchSizeQty, op.BatchSizeUnit, op.StandardMinutes, op.HourlyRate, op.PlannedBatchCount, op.PlannedMinutes, op.PlannedOperationCost, op.RecordsLoss, op.QualityChecklistJSON); err != nil {
			return manufacturingapp.ProcessRoute{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "process_route", &id, action, postgresinfra.StrPtr("route"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"status": cmd.Status, "operation_count": len(cmd.Operations), "old_standard_cost_capacity_ids": oldStandardCostCapacityIDs, "new_standard_cost_capacity_ids": newStandardCostCapacityIDs}); err != nil {
		return manufacturingapp.ProcessRoute{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return manufacturingapp.ProcessRoute{}, err
	}
	return r.processRouteByID(ctx, id)
}

func (r Repository) processRouteByID(ctx context.Context, id int64) (manufacturingapp.ProcessRoute, error) {
	rows, err := r.ListProcessRoutes(ctx, manufacturingapp.ProcessRouteQuery{})
	if err != nil {
		return manufacturingapp.ProcessRoute{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return manufacturingapp.ProcessRoute{}, pgx.ErrNoRows
}

func (r Repository) PublishProcessRoute(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.process_routes SET status='active', updated_at=now() WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "process_route", &cmd.ID, "publish", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("active"), postgresinfra.AuditMeta{"route_id": cmd.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) DeactivateProcessRoute(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.process_routes SET status='inactive', updated_at=now() WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "process_route", &cmd.ID, "deactivate", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"route_id": cmd.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
