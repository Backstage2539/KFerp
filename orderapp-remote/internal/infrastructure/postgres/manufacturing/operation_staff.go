package manufacturing

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/manufacturing"
)

func (r Repository) attachOperationStaff(ctx context.Context, operations []app.ManufacturingOperation) error {
	if len(operations) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(operations))
	indexes := map[int64]int{}
	for i := range operations {
		ids = append(ids, operations[i].ID)
		indexes[operations[i].ID] = i
		operations[i].EligibleEmployeeIDs = []int64{}
		operations[i].DefaultCollaboratorIDs = []int64{}
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT operation_id,employee_id,default_role FROM %s.manufacturing_operation_employees WHERE operation_id=ANY($1) ORDER BY operation_id,employee_id`, r.schema), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var op, id int64
		var role string
		if err := rows.Scan(&op, &id, &role); err != nil {
			return err
		}
		row := &operations[indexes[op]]
		row.EligibleEmployeeIDs = append(row.EligibleEmployeeIDs, id)
		if role == "lead" {
			row.DefaultEmployeeID = id
		}
		if role == "collaborator" {
			row.DefaultCollaboratorIDs = append(row.DefaultCollaboratorIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	// Old installations may not have employee data yet; configuration remains visibly incomplete.
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, r.schema+".company_employees").Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	active := map[int64]bool{}
	staff, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.company_employees WHERE active=true`, r.schema))
	if err != nil {
		return err
	}
	defer staff.Close()
	for staff.Next() {
		var id int64
		if err := staff.Scan(&id); err != nil {
			return err
		}
		active[id] = true
	}
	if err := staff.Err(); err != nil {
		return err
	}
	for i := range operations {
		operations[i].StaffingReady = active[operations[i].DefaultEmployeeID]
	}
	return nil
}

func saveOperationStaffTx(ctx context.Context, tx pgx.Tx, schema string, id int64, cmd app.SaveManufacturingOperationCommand) error {
	// Use the same lock as dispatch so a qualification cannot disappear between validation and commit.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, schema+":production_schedule"); err != nil {
		return err
	}
	roles := map[int64]string{}
	for _, employeeID := range cmd.EligibleEmployeeIDs {
		roles[employeeID] = ""
	}
	if cmd.DefaultEmployeeID > 0 {
		roles[cmd.DefaultEmployeeID] = "lead"
	}
	for _, employeeID := range cmd.DefaultCollaboratorIDs {
		roles[employeeID] = "collaborator"
	}
	for employeeID := range roles {
		var active bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.company_employees WHERE id=$1 FOR SHARE`, schema), employeeID).Scan(&active); err != nil || !active {
			return fmt.Errorf("所选员工已停用或不存在，请重新选择")
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.manufacturing_operation_employees WHERE operation_id=$1`, schema), id); err != nil {
		return err
	}
	for employeeID, role := range roles {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.manufacturing_operation_employees(operation_id,employee_id,default_role) VALUES($1,$2,$3)`, schema), id, employeeID, role); err != nil {
			return err
		}
	}
	return nil
}
