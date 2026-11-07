package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
)

// These fixtures explicitly choose cancellation and waiver of their unpaid
// test fees. They never provide default dispositions to application callers.
func cancelOrderResolutionFixture(ctx context.Context, pool *pgxpool.Pool, schema string, orderID int64, reason, actor string) error {
	repo := resolutionpg.NewRepository(pool, schema)
	current, err := repo.Context(ctx, orderID)
	if err != nil {
		return err
	}
	cmd := domain.Command{Action: "cancel", Reason: reason, Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: current.Version, IdempotencyKey: fmt.Sprintf("fixture-cancel-%d", orderID)}
	for _, d := range current.Dependencies {
		if d.Status == "cancelled" || d.Status == "released" {
			continue
		}
		switch d.Kind {
		case "fee":
			cmd.Dispositions = append(cmd.Dispositions, domain.Disposition{Kind: d.Kind, ID: d.ID, Choice: "waive"})
		case "direct_ship":
			cmd.Dispositions = append(cmd.Dispositions, domain.Disposition{Kind: d.Kind, ID: d.ID, Choice: "cancel"})
		default:
			return fmt.Errorf("fixture requires explicit handling for %s", d.Kind)
		}
	}
	result, err := repo.Submit(ctx, orderID, cmd, actor)
	if err != nil {
		return err
	}
	for _, s := range result.Steps {
		if _, err = repo.CompleteStep(ctx, orderID, result.ID, s.ID, resolutionpg.StepCommand{Choice: s.Choice, Note: "隔离样本明确取消并免除未收测试费用", Reference: "fixture-only", IdempotencyKey: fmt.Sprintf("fixture-step-%d", s.ID)}, actor); err != nil {
			return err
		}
	}
	return nil
}
func cancelDirectShipResolutionFixture(ctx context.Context, pool *pgxpool.Pool, schema string, customerID, requestID int64, reason, actor string) (app.MiniDirectShipRequest, error) {
	r := NewRepository(pool, schema)
	request, err := r.GetMiniDirectShipRequest(ctx, customerID, requestID)
	if err != nil {
		return request, err
	}
	for _, p := range request.Packages {
		if err = cancelOrderResolutionFixture(ctx, pool, schema, p.OrderID, reason, actor); err != nil {
			return request, err
		}
	}
	return r.GetMiniDirectShipRequest(ctx, customerID, requestID)
}
