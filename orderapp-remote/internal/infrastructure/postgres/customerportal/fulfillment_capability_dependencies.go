package customerportal

import (
	"context"
	app "orderapp/internal/application/customerportal"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) effectiveFulfillmentCapabilitiesTx(ctx context.Context, q txQuerier, customerID int64, configured []app.Capability) ([]app.Capability, error) {
	return postgresinfra.EffectiveFulfillmentCapabilities(ctx, q, r.schema, customerID, configured)
}
