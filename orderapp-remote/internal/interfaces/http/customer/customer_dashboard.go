package customer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// customer dashboard (counts) using existing pay/ship statuses + optional process status.
func fetchCustomerDashboard(ctx context.Context, pool *pgxpool.Pool, schema string, customerID int64) (CustomerDashboard, error) {
	var d CustomerDashboard

	// process status ids (optional)
	var pidProd, pidShip int64
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT id FROM %s.order_process_statuses WHERE name='生产中' LIMIT 1),0)`, schema)).Scan(&pidProd)
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT id FROM %s.order_process_statuses WHERE name='发货中' LIMIT 1),0)`, schema)).Scan(&pidShip)

	// pay_status_id: 2=已收款; ship_status_id: 3=已发货, 4=不发货
	q := fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			SUM(CASE WHEN COALESCE(o.pay_status_id,0) <> 2 THEN 1 ELSE 0 END) AS unpaid,
			SUM(CASE WHEN COALESCE(o.ship_status_id,0) IN (0,1,2) THEN 1 ELSE 0 END) AS unshipped,
			SUM(CASE WHEN $2>0 AND COALESCE(o.process_status_id,0) = $2 THEN 1 ELSE 0 END) AS in_prod,
			SUM(CASE WHEN $3>0 AND COALESCE(o.process_status_id,0) = $3 THEN 1 ELSE 0 END) AS in_ship,
			SUM(CASE WHEN COALESCE(o.pay_status_id,0)=2 AND COALESCE(o.ship_status_id,0) IN (3,4) THEN 1 ELSE 0 END) AS completed
		FROM %s.orders o
		WHERE o.customer_id=$1 AND COALESCE(o.is_void,false)=false
	`, schema)

	var total, unpaid, unshipped, inProd, inShip, completed int
	if err := pool.QueryRow(ctx, q, customerID, pidProd, pidShip).Scan(&total, &unpaid, &unshipped, &inProd, &inShip, &completed); err != nil {
		return d, err
	}
	d.TotalOrders = total
	d.UnpaidOrders = unpaid
	d.UnshippedOrders = unshipped
	d.InProduction = inProd
	d.InShipping = inShip
	d.Completed = completed
	return d, nil
}
