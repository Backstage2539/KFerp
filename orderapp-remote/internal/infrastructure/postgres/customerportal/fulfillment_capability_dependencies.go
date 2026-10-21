package customerportal

import (
	"context"
	"fmt"

	app "orderapp/internal/application/customerportal"
)

func (r Repository) effectiveFulfillmentCapabilitiesTx(ctx context.Context, q txQuerier, customerID int64, configured []app.Capability) ([]app.Capability, error) {
	var installed bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, r.schema+".customer_fulfillment_settings").Scan(&installed); err != nil {
		return nil, err
	}
	modes := []string{}
	if installed {
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT m->>'mode' FROM %s.customer_fulfillment_settings s CROSS JOIN LATERAL jsonb_array_elements(s.modes_json) m WHERE customer_id=$1 AND m->>'enabled'='true' AND m->>'mode' IN ('stock','on_demand')`, r.schema), customerID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var mode string
			if err := rows.Scan(&mode); err != nil {
				rows.Close()
				return nil, err
			}
			modes = append(modes, mode)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return app.ApplyFulfillmentCapabilityDependencies(configured, modes), nil
}
