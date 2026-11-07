package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
	"reflect"
)

// Recompute against locked current facts before effects are posted. Completed
// steps satisfy their specific obligations, but cannot authorize new obligations.
func (r Repository) revalidatePlan(ctx context.Context, tx pgx.Tx, orderID, requestID int64, original domain.Context, cmd domain.Command, plan domain.Plan) error {
	current, err := r.load(ctx, tx, orderID)
	if err != nil {
		return err
	}
	current.Paused = original.Paused // The request's own pause is not a new business fact.
	record, err := r.result(ctx, tx, requestID)
	if err != nil {
		return err
	}
	for _, d := range cmd.Dispositions {
		if d.Kind != "direct_ship" || d.Choice != "replace_supply" {
			continue
		}
		for _, step := range record.Steps {
			if step.Kind != d.Kind || step.SourceID != d.ID || step.Choice != d.Choice || step.Status != "completed" {
				continue
			}
			if err = r.verifyCompletedSupply(ctx, tx, orderID, original, d); err != nil {
				return err
			}
			for n := range current.Dependencies {
				dep := &current.Dependencies[n]
				if dep.Kind == d.Kind && dep.ID == d.ID && dep.Relation == "supply" && dep.Quantity == 0 {
					dep.Status = "released"
				}
			}
		}
	}
	if current.ExternalVersion != original.ExternalVersion && (current.Invoiced || original.Invoiced) {
		return fmt.Errorf("%w：开票记录已变化，请重新核实对应凭据", ErrStale)
	}
	command := cmd
	command.Dispositions = nil
	for _, d := range cmd.Dispositions {
		if d.Kind != "funds" {
			command.Dispositions = append(command.Dispositions, d)
		}
	}
	fresh, err := domain.Preview(current, command)
	if err != nil {
		return fmt.Errorf("%w：%s", ErrStale, err)
	}
	if fresh.Status != plan.Status || fresh.CreditCents != plan.CreditCents || !reflect.DeepEqual(fresh.Lines, plan.Lines) || !reflect.DeepEqual(fresh.Fees, plan.Fees) {
		return ErrStale
	}
	for _, task := range fresh.Tasks {
		found := false
		for _, step := range record.Steps {
			if step.Status != "superseded" && step.Kind == task.Kind && step.SourceID == task.SourceID && step.Permission == task.Permission && (task.Kind == "funds" || task.Kind == "external" || task.Kind == "period" || step.Choice == task.Choice) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w：新增待办 %s", ErrStale, task.Label)
		}
	}
	return nil
}
