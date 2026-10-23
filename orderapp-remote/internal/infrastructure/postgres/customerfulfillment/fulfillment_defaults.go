package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	freight "orderapp/internal/domain/freight"
	"strings"
)

func (r *Repository) loadFulfillmentDefaults(ctx context.Context, q miniDirectShipQuerier, lock bool) (app.FulfillmentDefaults, error) {
	d := app.FulfillmentDefaults{Modes: defaultFulfillmentSettings(0).Modes}
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	var raw []byte
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT revision,modes_json FROM %s.fulfillment_defaults WHERE singleton=true%s`, r.schema, suffix)).Scan(&d.Revision, &raw)
	if err != nil {
		return d, err
	}
	if err = json.Unmarshal(raw, &d.Modes); err != nil {
		return d, err
	}
	d.Modes = completeFulfillmentModes(d.Modes)
	return d, nil
}
func (r *Repository) LoadFulfillmentDefaults(ctx context.Context) (app.FulfillmentDefaults, error) {
	d, err := r.loadFulfillmentDefaults(ctx, r.pool, false)
	if err != nil {
		return d, err
	}
	err = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.customer_fulfillment_settings s WHERE EXISTS(SELECT 1 FROM jsonb_array_elements(s.modes_json) m WHERE (m->>'enabled')::boolean=true AND jsonb_array_length(COALESCE(NULLIF(m->'inherited_fields','null'::jsonb),'[]'::jsonb))>0)`, r.schema)).Scan(&d.InheritingCustomers)
	return d, err
}
func (r *Repository) SaveFulfillmentDefaults(ctx context.Context, c app.SaveFulfillmentDefaultsCommand) (app.FulfillmentDefaults, error) {
	d := c.Defaults
	if strings.TrimSpace(c.Actor) == "" {
		return d, fmt.Errorf("操作人不能为空")
	}
	if len(d.Modes) != 2 {
		return d, fmt.Errorf("请完整配置库存代发与现做现发默认规则")
	}
	seen := map[string]bool{}
	for i, m := range d.Modes {
		if (m.Mode != "stock" && m.Mode != "on_demand") || seen[m.Mode] {
			return d, fmt.Errorf("履约场景无效或重复")
		}
		seen[m.Mode] = true
		if !m.FeeConfigured || m.FixedFeeCents < 0 || m.PerPackageFeeCents < 0 || m.FixedFeeCents > freight.MaxCents || m.PerPackageFeeCents > freight.MaxCents {
			return d, fmt.Errorf("请明确填写默认服务费，免费请填写 0")
		}
		policy := freight.Policy{CustomerID: 1, Scene: m.Mode, TemplateID: m.ShippingTemplateID, FreeShipping: m.ShippingFree}
		if m.ShippingPolicy != nil {
			policy.SurchargeCents = m.ShippingPolicy.SurchargeCents
		}
		if err := freight.ValidatePolicyOverride(policy); err != nil {
			return d, err
		}
		d.Modes[i] = app.FulfillmentModeSettings{Mode: m.Mode, FeeConfigured: true, FixedFeeCents: m.FixedFeeCents, PerPackageFeeCents: m.PerPackageFeeCents, ShippingTemplateID: policy.TemplateID, ShippingFree: policy.FreeShipping, ShippingPolicy: fulfillmentPolicyProjection(policy)}
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT singleton FROM %s.fulfillment_defaults WHERE singleton=true FOR UPDATE`, r.schema)); err != nil {
		return d, err
	}
	old, err := r.loadFulfillmentDefaults(ctx, tx, false)
	if err != nil {
		return d, err
	}
	if old.Revision != c.ExpectedRevision {
		return d, fmt.Errorf("默认配置已变化，请刷新后重试")
	}
	for _, m := range d.Modes {
		if m.ShippingTemplateID > 0 {
			tm, e := r.loadShippingFeeTemplate(ctx, tx, m.ShippingTemplateID, true)
			if e != nil || !tm.Active {
				return d, fmt.Errorf("运费模板不存在或已停用")
			}
		}
	}
	d.Revision = old.Revision + 1
	raw, err := json.Marshal(d.Modes)
	if err != nil {
		return d, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.fulfillment_defaults SET revision=$1,modes_json=$2,updated_by=$3,updated_at=now() WHERE singleton=true`, r.schema), d.Revision, raw, c.Actor); err != nil {
		return d, err
	}
	if err = r.auditFulfillmentConfig(ctx, tx, c.Actor, "fulfillment_defaults", 0, "save", old, d); err != nil {
		return d, err
	}
	return d, tx.Commit(ctx)
}
