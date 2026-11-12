package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/catalog"
	infra "orderapp/internal/infrastructure/postgres"
	"strings"
)

func (r Repository) ListSpecPricingGroups(ctx context.Context) ([]app.SpecPricingGroup, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT g.id,g.name,g.spec_template_id,g.spec_template_version_id,g.revision,g.active,COALESCE(jsonb_agg(jsonb_build_object('spec_key',i.spec_key,'inventory_unit',i.inventory_unit,'pricing_rule_id',i.pricing_rule_id,'sort_order',i.sort_order) ORDER BY i.sort_order,i.id) FILTER (WHERE i.id IS NOT NULL),'[]'::jsonb) FROM %s.spec_pricing_groups g LEFT JOIN %s.spec_pricing_group_items i ON i.group_id=g.id GROUP BY g.id ORDER BY g.active DESC,g.name,g.id`, r.schema, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.SpecPricingGroup{}
	for rows.Next() {
		var g app.SpecPricingGroup
		var data []byte
		if err = rows.Scan(&g.ID, &g.Name, &g.SpecTemplateID, &g.SpecTemplateVersionID, &g.Revision, &g.Active, &data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &g.Items); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
func (r Repository) SaveSpecPricingGroup(ctx context.Context, g app.SpecPricingGroup) (app.SpecPricingGroup, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return g, err
	}
	defer tx.Rollback(ctx)
	if g.ID == 0 || g.Active {
		var valid bool
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_bom_spec_template_versions v JOIN %s.production_bom_spec_templates t ON t.id=v.template_id WHERE v.id=$1 AND v.template_id=$2 AND v.status='published' AND t.active)`, r.schema, r.schema), g.SpecTemplateVersionID, g.SpecTemplateID).Scan(&valid)
		if err != nil {
			return g, err
		}
		if !valid {
			return g, app.ValidationError{Message: "规格模板版本不存在、未发布或已停用"}
		}
		for _, i := range g.Items {
			err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_bom_spec_template_variants v JOIN %s.product_pricing_rules p ON p.id=$4 AND p.active WHERE v.version_id=$1 AND lower(v.spec_key)=lower($2) AND lower(v.inventory_unit)=lower($3))`, r.schema, r.schema), g.SpecTemplateVersionID, strings.TrimSpace(i.SpecKey), strings.TrimSpace(i.InventoryUnit), i.PricingRuleID).Scan(&valid)
			if err != nil {
				return g, err
			}
			if !valid {
				return g, app.ValidationError{Message: "规格或单位不属于所选模板版本，或价格模板已失效"}
			}
		}
	}
	var oldJSON []byte
	if g.ID > 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT row_to_json(g) FROM %s.spec_pricing_groups g WHERE id=$1 FOR UPDATE`, r.schema), g.ID).Scan(&oldJSON)
		if errors.Is(err, pgx.ErrNoRows) {
			return g, app.ValidationError{Message: "计价组不存在"}
		}
		if err != nil {
			return g, err
		}
		var oldItems []byte
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(jsonb_build_object('spec_key',spec_key,'inventory_unit',inventory_unit,'pricing_rule_id',pricing_rule_id,'sort_order',sort_order) ORDER BY sort_order,id),'[]'::jsonb) FROM %s.spec_pricing_group_items WHERE group_id=$1`, r.schema), g.ID).Scan(&oldItems)
		if err != nil {
			return g, err
		}
		var oldObject map[string]any
		if err = json.Unmarshal(oldJSON, &oldObject); err != nil {
			return g, err
		}
		var beforeItems []any
		if err = json.Unmarshal(oldItems, &beforeItems); err != nil {
			return g, err
		}
		oldObject["items"] = beforeItems
		oldJSON, err = json.Marshal(oldObject)
		if err != nil {
			return g, err
		}
		var old struct {
			Revision int64 `json:"revision"`
		}
		if err = json.Unmarshal(oldJSON, &old); err != nil {
			return g, err
		}
		if g.Revision != old.Revision {
			return g, app.ValidationError{Message: "计价组已被修改，请刷新后重试"}
		}
		if !g.Active {
			var previous app.SpecPricingGroup
			if err = json.Unmarshal(oldJSON, &previous); err != nil {
				return g, err
			}
			if previous.SpecTemplateID != g.SpecTemplateID || previous.SpecTemplateVersionID != g.SpecTemplateVersionID || len(previous.Items) != len(g.Items) {
				return g, app.ValidationError{Message: "停用计价组时不能同时更改规格配置"}
			}
			for idx, item := range g.Items {
				before := previous.Items[idx]
				if before.SpecKey != item.SpecKey || before.InventoryUnit != item.InventoryUnit || before.PricingRuleID != item.PricingRuleID {
					return g, app.ValidationError{Message: "停用计价组时不能同时更改规格配置"}
				}
			}
		}
		err = tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.spec_pricing_groups SET name=$2,spec_template_id=$3,spec_template_version_id=$4,active=$5,revision=revision+1,updated_by=$6,updated_at=now() WHERE id=$1 RETURNING revision`, r.schema), g.ID, g.Name, g.SpecTemplateID, g.SpecTemplateVersionID, g.Active, g.Actor).Scan(&g.Revision)
		if err != nil {
			return g, err
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.spec_pricing_group_items WHERE group_id=$1`, r.schema), g.ID)
		if err != nil {
			return g, err
		}
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.spec_pricing_groups(name,spec_template_id,spec_template_version_id,active,created_by,updated_by) VALUES($1,$2,$3,$4,$5,$5) RETURNING id,revision`, r.schema), g.Name, g.SpecTemplateID, g.SpecTemplateVersionID, g.Active, g.Actor).Scan(&g.ID, &g.Revision)
		if err != nil {
			return g, err
		}
	}
	for n, i := range g.Items {
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.spec_pricing_group_items(group_id,spec_key,inventory_unit,pricing_rule_id,sort_order) VALUES($1,$2,$3,$4,$5)`, r.schema), g.ID, strings.TrimSpace(i.SpecKey), strings.TrimSpace(i.InventoryUnit), i.PricingRuleID, n)
		if err != nil {
			return g, err
		}
		g.Items[n].SortOrder = n
	}
	newJSON, err := json.Marshal(g)
	if err != nil {
		return g, err
	}
	old := string(oldJSON)
	now := string(newJSON)
	if err = infra.AuditInsertTx(ctx, tx, r.schema, g.Actor, "spec_pricing_group", &g.ID, "save_spec_pricing_group", nil, infra.StrPtr(old), infra.StrPtr(now), infra.AuditMeta{"revision": g.Revision, "item_count": len(g.Items), "active": g.Active}); err != nil {
		return g, err
	}
	if err = tx.Commit(ctx); err != nil {
		return g, err
	}
	return g, nil
}
func (r Repository) SpecPricingGroupOptions(ctx context.Context) (app.SpecPricingOptions, error) {
	out := app.SpecPricingOptions{Templates: []app.SpecPricingOptionTemplate{}, PricingRules: []app.SpecPricingOptionRule{}}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT t.id,t.name,v.id,v.version_no,s.spec_key,s.name,s.inventory_unit FROM %s.production_bom_spec_templates t JOIN %s.production_bom_spec_template_versions v ON v.template_id=t.id AND v.status='published' JOIN %s.production_bom_spec_template_variants s ON s.version_id=v.id WHERE t.active ORDER BY t.name,t.id,v.id DESC,s.sort_order,s.id`, r.schema, r.schema, r.schema))
	if err != nil {
		return out, err
	}
	tids := map[int64]int{}
	vids := map[int64]int{}
	for rows.Next() {
		var tid, vid int64
		var tn, vn string
		var s app.SpecPricingOptionVariant
		if err = rows.Scan(&tid, &tn, &vid, &vn, &s.SpecKey, &s.Name, &s.InventoryUnit); err != nil {
			rows.Close()
			return out, err
		}
		ti, ok := tids[tid]
		if !ok {
			ti = len(out.Templates)
			tids[tid] = ti
			out.Templates = append(out.Templates, app.SpecPricingOptionTemplate{ID: tid, Name: tn, Versions: []app.SpecPricingOptionVersion{}})
		}
		vi, ok := vids[vid]
		if !ok {
			vi = len(out.Templates[ti].Versions)
			vids[vid] = vi
			out.Templates[ti].Versions = append(out.Templates[ti].Versions, app.SpecPricingOptionVersion{ID: vid, VersionNo: vn, Variants: []app.SpecPricingOptionVariant{}})
		}
		out.Templates[ti].Versions[vi].Variants = append(out.Templates[ti].Versions[vi].Variants, s)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = r.pool.Query(ctx, fmt.Sprintf(`SELECT id,name,active FROM %s.product_pricing_rules WHERE active ORDER BY name,id`, r.schema))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var rule app.SpecPricingOptionRule
		if err = rows.Scan(&rule.ID, &rule.Name, &rule.Active); err != nil {
			return out, err
		}
		out.PricingRules = append(out.PricingRules, rule)
	}
	return out, rows.Err()
}
