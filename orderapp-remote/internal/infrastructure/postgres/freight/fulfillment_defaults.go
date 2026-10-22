package freight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/freight"
)

// Shared by employee and customer quote paths. Stored policies remain editable
// independently; only fields explicitly opted into inheritance are replaced.
func (r *Repository) loadFulfillmentPolicy(ctx context.Context, q Queryer, id int64, scene string, lock bool) (domain.Policy, error) {
	p := domain.Policy{CustomerID: id, Scene: scene}
	if p.Scene != "stock" && p.Scene != "on_demand" {
		return r.LoadPolicy(ctx, q, id, scene, lock)
	}
	var raw []byte
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT modes_json FROM %s.customer_fulfillment_settings WHERE customer_id=$1%s`, r.schema, suffix), p.CustomerID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return r.LoadPolicy(ctx, q, id, scene, lock)
	}
	if err != nil {
		return p, err
	}
	var modes []app.FulfillmentModeSettings
	if err = json.Unmarshal(raw, &modes); err != nil {
		return p, err
	}
	for _, m := range modes {
		if m.Mode != p.Scene || len(m.InheritedFields) == 0 {
			continue
		}
		var d app.FulfillmentDefaults
		if err = q.QueryRow(ctx, fmt.Sprintf(`SELECT revision,modes_json FROM %s.fulfillment_defaults WHERE singleton=true%s`, r.schema, suffix)).Scan(&d.Revision, &raw); err != nil {
			return p, err
		}
		if err = json.Unmarshal(raw, &d.Modes); err != nil {
			return p, err
		}
		p, err = r.LoadPolicy(ctx, q, id, scene, lock)
		if err != nil {
			return p, err
		}
		m.ShippingPolicy = &app.FulfillmentShippingPolicy{Revision: p.Revision, TemplateID: p.TemplateID, SurchargeCents: p.SurchargeCents, FreeShipping: p.FreeShipping}
		resolved := app.ResolveFulfillmentModeDefaults(m, app.FulfillmentModeDefault(d, m.Mode))
		p.TemplateID, p.SurchargeCents, p.FreeShipping = resolved.ShippingPolicy.TemplateID, resolved.ShippingPolicy.SurchargeCents, resolved.ShippingPolicy.FreeShipping
		p.FulfillmentDefaultsRevision = d.Revision
		return p, nil
	}
	return r.LoadPolicy(ctx, q, id, scene, lock)
}
