package production

import (
	"context"
	app "orderapp/internal/application/production"
)

func (r Repository) saveTaskAssignmentCompat(ctx context.Context, cmd app.ScheduleAssignmentCommand) (app.ScheduleAssignmentResult, error) {
	p := app.ScheduleTaskPatch{WorkOrderID: cmd.WorkOrderID, JobCardID: cmd.JobCardID, Claim: cmd.Claim}
	if cmd.Patch != nil {
		p = *cmd.Patch
		p.Claim = cmd.Claim
	} else {
		// Pre-existing service callers send only the fields they intend to update.
		if cmd.AssignedEmployeeID > 0 {
			p.AssignedEmployeeID = &cmd.AssignedEmployeeID
		} else if cmd.AssignedTo != "" {
			p.AssignedTo = &cmd.AssignedTo
		}
		if cmd.PlannedStartAt != "" {
			p.PlannedStartAt = &cmd.PlannedStartAt
		}
		if cmd.PlannedEndAt != "" {
			p.PlannedEndAt = &cmd.PlannedEndAt
		}
		if cmd.ShiftCode != "" {
			p.ShiftCode = &cmd.ShiftCode
		}
		if cmd.Note != "" {
			p.Note = &cmd.Note
		}
		if cmd.Priority > 0 {
			p.Priority = &cmd.Priority
		}
	}
	result, err := r.ScheduleBatch(ctx, app.ScheduleBatchCommand{Items: []app.ScheduleTaskPatch{p}, Operator: cmd.Operator, PreviewToken: cmd.PreviewToken, RequestID: cmd.RequestID}, false)
	if err != nil {
		return app.ScheduleAssignmentResult{}, err
	}
	row := result.Rows[0]
	wo, err := loadScheduledWorkOrderTx(ctx, r.pool, r.schema, row.WorkOrderID)
	if err != nil {
		return app.ScheduleAssignmentResult{}, err
	}
	return app.ScheduleAssignmentResult{JobCard: row.JobCardRow, WorkOrder: wo, Conflicts: []app.ScheduleConflict{}}, nil
}
