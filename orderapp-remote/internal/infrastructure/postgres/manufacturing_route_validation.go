package postgres

import (
	"context"
	"fmt"
)

// ValidateManufacturingRoute checks configuration only. Stock readiness belongs
// to production preparation/start, so a new zero-stock product can be created.
func ValidateManufacturingRoute(ctx context.Context, q standardCostCapacityIssueQuerier, schema string, routeID int64) error {
	var name, status string
	var count, distinct, invalid int
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT r.name,r.status,COUNT(o.id)::int,COUNT(DISTINCT o.seq)::int,COUNT(o.id) FILTER(WHERE o.seq<=0 OR trim(o.operation)='')::int FROM %[1]s.process_routes r LEFT JOIN %[1]s.process_route_operations o ON o.route_id=r.id WHERE r.id=$1 GROUP BY r.id,r.name,r.status`, schema), routeID).Scan(&name, &status, &count, &distinct, &invalid)
	if err != nil {
		return fmt.Errorf("工艺路线 #%d 不存在或无法读取", routeID)
	}
	if status != "active" {
		return fmt.Errorf("工艺路线「%s」已停用", name)
	}
	if count == 0 {
		return fmt.Errorf("工艺路线「%s」没有工序，请先补齐路线", name)
	}
	if count != distinct || invalid > 0 {
		return fmt.Errorf("工艺路线「%s」的工序名称或顺序不完整", name)
	}
	return nil
}
