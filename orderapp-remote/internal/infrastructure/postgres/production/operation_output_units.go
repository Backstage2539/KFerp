package production

import (
	"context"
	"fmt"

	app "orderapp/internal/application/production"
)

// JobCardOutputUnits reads the units recorded on posted completion events.
// Cards without events have no entry; missing or mixed units must never be
// substituted with the destination work order's inventory unit.
func (r Repository) JobCardOutputUnits(ctx context.Context, ids []int64) (map[int64]app.PredecessorOutputUnitInfo, error) {
	result := map[int64]app.PredecessorOutputUnitInfo{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT job_card_id,COUNT(*) FILTER (WHERE TRIM(COALESCE(output_unit,''))='')::int,COUNT(DISTINCT NULLIF(TRIM(output_unit),''))::int,COALESCE(MIN(NULLIF(TRIM(output_unit),'')),'') FROM %s.production_operation_events WHERE job_card_id=ANY($1::bigint[]) AND kind='complete' AND status='posted' GROUP BY job_card_id`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var missing, count int
		var unit string
		if err = rows.Scan(&id, &missing, &count, &unit); err != nil {
			return nil, err
		}
		info := app.PredecessorOutputUnitInfo{State: "missing"}
		if count > 1 {
			info.State = "mixed"
		} else if missing == 0 && count == 1 {
			info.State = "known"
			info.Unit = unit
		}
		result[id] = info
	}
	return result, rows.Err()
}
