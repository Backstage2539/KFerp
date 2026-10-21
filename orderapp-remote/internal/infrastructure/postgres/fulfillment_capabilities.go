package postgres

import (
	"context"
	"fmt"

	app "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5"
)

type FulfillmentCapabilityQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// EffectiveFulfillmentCapabilities keeps read and submit authorization consistent
// without persisting derived capabilities into customer settings or templates.
func EffectiveFulfillmentCapabilities(ctx context.Context, q FulfillmentCapabilityQuerier, schema string, customerID int64, configured []app.Capability) ([]app.Capability, error) {
	var installed bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".customer_fulfillment_settings").Scan(&installed); err != nil {
		return nil, err
	}
	modes := []string{}
	if installed {
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT m->>'mode' FROM %s.customer_fulfillment_settings s CROSS JOIN LATERAL jsonb_array_elements(s.modes_json) m WHERE customer_id=$1 AND m->>'enabled'='true' AND m->>'mode' IN ('stock','on_demand')`, schema), customerID)
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
