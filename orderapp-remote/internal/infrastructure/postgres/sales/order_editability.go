package sales

import (
	"context"
	"fmt"

	salesapp "orderapp/internal/application/sales"

	"github.com/jackc/pgx/v5"
)

type orderEditabilityQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func orderEditStateQuery(schema string, lock bool) string {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF o"
	}
	return fmt.Sprintf(`
		SELECT COALESCE(o.is_void,false),
		       COALESCE(ss.name,''),
		       COALESCE(ops.name,''),
		       EXISTS (
		         SELECT 1
		         FROM %[1]s.production_plan_items pi
		         JOIN %[1]s.production_plans pp ON pp.id=pi.production_plan_id
		         WHERE o.order_no = ANY(string_to_array(replace(COALESCE(pi.order_nos,''),' ',''), ','))
		           AND COALESCE(pp.status,'') <> 'cancelled'
		       ),
		       EXISTS (
		         SELECT 1
		         FROM %[1]s.work_orders wo
		         WHERE o.order_no = ANY(string_to_array(replace(COALESCE(wo.order_nos,''),' ',''), ','))
		           AND COALESCE(wo.status,'') <> 'cancelled'
		       ) OR EXISTS (
		         SELECT 1
		         FROM %[1]s.produce_running_items ri
		         WHERE o.order_no = ANY(string_to_array(replace(COALESCE(ri.order_nos,''),' ',''), ','))
		           AND COALESCE(ri.status,'') <> 'cancelled'
		       ),
		       EXISTS (
		         SELECT 1
		         FROM %[1]s.produce_batch_order_items pbo
		         JOIN %[1]s.produce_batches pb ON pb.batch_id=pbo.batch_id
		         WHERE pbo.order_id=o.id
		           AND COALESCE(pb.status,'') <> 'cancelled'
		       ),
		       EXISTS (SELECT 1 FROM %[1]s.order_shipment_orders oso WHERE oso.order_id=o.id),
		       EXISTS (SELECT 1 FROM %[1]s.order_stock_batch_allocations osa WHERE osa.order_id=o.id),
		       EXISTS (SELECT 1 FROM %[1]s.order_stock_deductions osd WHERE osd.order_id=o.id)
		FROM %[1]s.orders o
		LEFT JOIN %[1]s.ship_statuses ss ON ss.id=o.ship_status_id
		LEFT JOIN %[1]s.order_process_statuses ops ON ops.id=o.process_status_id
		WHERE o.id=$1%[2]s
	`, schema, lockClause)
}

func loadOrderEditState(ctx context.Context, queryer orderEditabilityQueryer, schema string, orderID int64, lock bool) (salesapp.OrderEditState, error) {
	var state salesapp.OrderEditState
	err := queryer.QueryRow(ctx, orderEditStateQuery(schema, lock), orderID).Scan(
		&state.IsVoid,
		&state.ShipStatus,
		&state.ProcessStatus,
		&state.HasProductionPlan,
		&state.HasWorkOrder,
		&state.HasProduceBatch,
		&state.HasShipment,
		&state.HasStockAllocation,
		&state.HasStockDeduction,
	)
	return state, err
}

func (r Repository) OrderEditability(ctx context.Context, orderID int64) (salesapp.OrderEditability, error) {
	state, err := loadOrderEditState(ctx, r.pool, r.schema, orderID, false)
	if err != nil {
		return salesapp.OrderEditability{}, err
	}
	return salesapp.EvaluateOrderEditability(state), nil
}
