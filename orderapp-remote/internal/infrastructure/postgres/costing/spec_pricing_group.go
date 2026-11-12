package costing

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/costing"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"sort"
)

func (r Repository) ResolveSpecPricingGroupForProduct(ctx context.Context, groupID, parentID int64) (app.SpecPricingGroupResolved, error) {
	out := app.SpecPricingGroupResolved{}
	rows, err := r.readDB(ctx).Query(ctx, fmt.Sprintf(`SELECT g.id,g.revision,(g.active AND t.active AND v.status='published'),i.spec_key,COALESCE(s.id,0),COALESCE(bv.id,0),CASE WHEN COALESCE(rule.active,false) THEN i.pricing_rule_id ELSE 0 END,COALESCE(md5(jsonb_build_array(rule.cost_source_mode,rule.margin_rate,rule.tax_rate,rule.rounding_mode,rule.calculation_json,rule.formula_version)::text),'') FROM %[1]s.spec_pricing_groups g JOIN %[1]s.spec_pricing_group_items i ON i.group_id=g.id JOIN %[1]s.production_bom_spec_templates t ON t.id=g.spec_template_id JOIN %[1]s.production_bom_spec_template_versions v ON v.id=g.spec_template_version_id AND v.template_id=t.id LEFT JOIN %[1]s.product_pricing_rules rule ON rule.id=i.pricing_rule_id LEFT JOIN %[1]s.production_bom_output_bindings binding ON binding.output_type='product' AND binding.output_id=$2 AND binding.is_default=true LEFT JOIN %[1]s.production_bom_versions bver ON bver.id=binding.bom_version_id AND bver.bom_id=binding.bom_id AND bver.status='published' LEFT JOIN %[1]s.production_bom_specs s ON s.bom_id=binding.bom_id AND s.source_spec_template_id=g.spec_template_id AND lower(s.source_spec_template_key)=lower(i.spec_key) AND lower(s.inventory_unit)=lower(i.inventory_unit) LEFT JOIN %[1]s.production_bom_version_variants bv ON bv.version_id=bver.id AND bv.bom_spec_id=s.id WHERE g.id=$1 ORDER BY i.sort_order,i.id`, r.schema), groupID, parentID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var i app.SpecPricingGroupResolvedItem
		if err = rows.Scan(&out.ID, &out.Revision, &out.Active, &i.SpecKey, &i.BomSpecID, &i.BomVariantID, &i.PricingRuleID, &i.PricingRuleFingerprint); err != nil {
			return out, err
		}
		out.Items = append(out.Items, i)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if out.ID == 0 {
		return out, fmt.Errorf("规格计价组不存在或未配置规格")
	}
	return out, nil
}

func (r Repository) lockAndValidateSpecPricingGroups(ctx context.Context, tx pgx.Tx, cmd app.PublishBeanListCommand) error {
	config, ok := cmd.Config["price_list_template_selection"].(map[string]any)
	if !ok || config["product_pricing_scope"] != "spec_pricing_group_aware" {
		return nil
	}
	ids := map[int64]bool{}
	parents := map[int64]bool{}
	rules := map[int64]bool{}
	specs := map[int64]bool{}
	variants := map[int64]bool{}
	numeric := func(v any) int64 {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		case json.Number:
			i, _ := n.Int64()
			return i
		}
		return 0
	}
	for _, raw := range arrayAny(cmd.Content["price_rows"]) {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := numeric(row["spec_pricing_group_id"])
		if id <= 0 {
			continue
		}
		ids[id] = true
		parents[numeric(row["parent_product_id"])] = true
		rules[numeric(row["pricing_rule_id"])] = true
		specs[numeric(row["bom_spec_id"])] = true
		variants[numeric(row["bom_variant_id"])] = true
	}
	toIDs := func(m map[int64]bool) []int64 {
		v := []int64{}
		for id := range m {
			if id > 0 {
				v = append(v, id)
			}
		}
		sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
		return v
	}
	for _, v := range []struct {
		table, key string
		ids        []int64
		where      string
	}{
		{"spec_pricing_groups", "id", toIDs(ids), ""},
		{"product_pricing_rules", "id", toIDs(rules), ""},
		{"production_bom_output_bindings", "output_id", toIDs(parents), " AND output_type='product' AND is_default=true"},
		{"production_bom_specs", "id", toIDs(specs), ""},
		{"production_bom_version_variants", "id", toIDs(variants), ""},
	} {
		if len(v.ids) == 0 {
			continue
		}
		rows, err := tx.Query(ctx, fmt.Sprintf("SELECT %s FROM %s.%s WHERE %s=ANY($1::bigint[])%s ORDER BY %s FOR SHARE", v.key, r.schema, v.table, v.key, v.where, v.key), v.ids)
		if err != nil {
			return err
		}
		for rows.Next() {
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	if groupIDs := toIDs(ids); len(groupIDs) > 0 {
		rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT t.id FROM %[1]s.spec_pricing_groups g JOIN %[1]s.production_bom_spec_templates t ON t.id=g.spec_template_id JOIN %[1]s.production_bom_spec_template_versions v ON v.id=g.spec_template_version_id WHERE g.id=ANY($1::bigint[]) ORDER BY t.id FOR SHARE OF t,v`, r.schema), groupIDs)
		if err != nil {
			return err
		}
		for rows.Next() {
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	txctx := postgresinfra.WithTransaction(ctx, tx)
	return app.NewService(r).ValidateSpecPricingGroups(txctx, &cmd)
}
func arrayAny(v any) []any {
	switch a := v.(type) {
	case []any:
		return a
	case []map[string]any:
		out := make([]any, len(a))
		for i := range a {
			out[i] = a[i]
		}
		return out
	}
	return nil
}
