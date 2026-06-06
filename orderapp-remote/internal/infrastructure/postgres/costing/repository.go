package costing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	appcosting "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) LoadParameters(ctx context.Context) (domain.Parameters, error) {
	params := domain.DefaultParameters()
	if r.pool == nil {
		return params, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT key, value::float8 FROM %s.cost_parameters`, r.schema))
	if err != nil {
		return params, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var value float64
		if err := rows.Scan(&key, &value); err != nil {
			return params, err
		}
		applyParameter(&params, key, value)
	}
	return params, rows.Err()
}

func (r Repository) LoadProductInputs(ctx context.Context, params domain.Parameters) ([]domain.ProductInput, error) {
	return r.loadProductInputs(ctx, params, 0)
}

func (r Repository) LoadProductInputsForCustomer(ctx context.Context, params domain.Parameters, customerID int64) ([]domain.ProductInput, error) {
	if customerID < 0 {
		customerID = 0
	}
	return r.loadProductInputs(ctx, params, customerID)
}

func (r Repository) loadProductInputs(ctx context.Context, params domain.Parameters, customerID int64) ([]domain.ProductInput, error) {
	q := fmt.Sprintf(`
		WITH material_valuation AS (
			SELECT l.material_id,
			       SUM(l.qty_g::numeric * COALESCE(b.unit_cost,0)) / NULLIF(SUM(l.qty_g),0) AS weighted_unit_cost
			FROM %[1]s.material_batch_locations l
			JOIN %[1]s.material_batches b ON b.id = l.material_batch_id
			WHERE l.qty_g > 0
			  AND b.status='active'
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			GROUP BY l.material_id
		),
		product_scope AS (
			SELECT p.*,
			       COALESCE(cpa.id,0) AS customer_product_alias_id,
			       COALESCE(NULLIF(cpa.brand_name,''), NULLIF(cpa.display_name,''), p.name) AS customer_product_display_name,
			       COALESCE(cpa.customer_item_code,'') AS customer_item_code,
			       COALESCE(cpa.brand_name,'') AS brand_name,
			       COALESCE(cpa.display_category_id,0) AS display_category_id,
			       COALESCE(alias_pc.name,'') AS display_category_name,
			       COALESCE(cpa.product_config_template_id,0) AS customer_product_alias_product_config_template_id,
			       COALESCE(cpa.gradient_template_id,0) AS customer_product_alias_gradient_template_id,
			       COALESCE(cpa.unit_template_id,0) AS customer_product_alias_unit_template_id,
			       COALESCE(CASE WHEN $2 > 0 THEN alias_class.template_id ELSE product_class.template_id END,0) AS current_classification_template_id,
			       COALESCE(CASE WHEN $2 > 0 THEN alias_ct.name ELSE product_ct.name END,'') AS current_classification_template_name,
			       COALESCE(CASE WHEN $2 > 0 THEN alias_class.category_id ELSE product_class.category_id END,0) AS current_classification_category_id,
			       COALESCE(CASE
			         WHEN $2 > 0 AND COALESCE(alias_class.template_id,0) > 0 AND COALESCE(alias_class.category_id,0)=0 THEN '未分类'
			         WHEN $2 > 0 THEN alias_cc.name
			         WHEN COALESCE(product_class.template_id,0) > 0 AND COALESCE(product_class.category_id,0)=0 THEN '未分类'
			         ELSE product_cc.name
			       END,'') AS current_classification_category_name,
			       COALESCE(CASE WHEN $2 > 0 THEN alias_cc.product_config_template_id ELSE product_cc.product_config_template_id END,0) AS current_classification_category_product_config_template_id,
			       COALESCE(CASE WHEN $2 > 0 THEN alias_ct.product_config_template_id ELSE product_ct.product_config_template_id END,0) AS current_classification_template_product_config_template_id,
			       CASE WHEN ppc.product_id IS NOT NULL THEN 'product_production_config' WHEN pbb.product_id IS NOT NULL THEN 'production_bom' ELSE COALESCE(NULLIF(bs.source_type,''), '') END AS bom_usage_mode,
			       COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id,0) AS production_bom_id,
			       COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id,0) AS production_bom_version_id,
			       COALESCE(NULLIF(ppc.expected_loss_rate,0), -1)::float8 AS expected_loss_rate,
			       CASE WHEN ppc.product_id IS NOT NULL THEN 1 - COALESCE(NULLIF(ppc.expected_loss_rate,0), 0) ELSE 0 END::float8 AS production_config_yield_rate,
			       COALESCE(NULLIF(ppc.process_route_id,0),0) AS process_route_id,
			       CASE
			         WHEN COALESCE(NULLIF(p.product_kind,''),'roasted')='green_bean'
			          AND COALESCE(p.green_bean_bom_product_id,0) > 0
			         THEN p.green_bean_bom_product_id
			         WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version')
			          AND COALESCE(bs.source_product_id,0) > 0
			         THEN bs.source_product_id
			         ELSE p.id
			       END AS bom_product_id
			FROM %[1]s.products p
			LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id = p.id
			LEFT JOIN %[1]s.product_bom_sources bs ON bs.product_id = p.id
			LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id = p.id
			LEFT JOIN %[1]s.customer_product_aliases cpa
			  ON $2 > 0
			 AND cpa.product_id = p.id
			 AND cpa.customer_id = $2
			 AND cpa.active=true
			 AND cpa.include_in_price_list=true
			LEFT JOIN %[1]s.product_categories alias_pc ON alias_pc.id=cpa.display_category_id AND alias_pc.active=true
			LEFT JOIN %[1]s.product_classification_assignments product_class
			  ON $2 <= 0
			 AND product_class.product_id = p.id
			LEFT JOIN %[1]s.product_classification_templates product_ct
			  ON product_ct.id = product_class.template_id
			 AND product_ct.active=true
			LEFT JOIN %[1]s.product_classification_template_categories product_cc
			  ON product_cc.id = product_class.category_id
			 AND product_cc.template_id = product_class.template_id
			 AND product_cc.active=true
			LEFT JOIN %[1]s.customer_product_alias_classification_assignments alias_class
			  ON $2 > 0
			 AND alias_class.alias_id = cpa.id
			LEFT JOIN %[1]s.product_classification_templates alias_ct
			  ON alias_ct.id = alias_class.template_id
			 AND alias_ct.active=true
			LEFT JOIN %[1]s.product_classification_template_categories alias_cc
			  ON alias_cc.id = alias_class.category_id
			 AND alias_cc.template_id = alias_class.template_id
			 AND alias_cc.active=true
			WHERE p.active = true
			  AND (($2 <= 0 AND COALESCE(p.customer_id,0)=0) OR ($2 > 0 AND cpa.id IS NOT NULL))
		),
		all_effective_bom_items AS (
			SELECT p.id AS product_id,
			       pbi.material_id, pbi.component_type, pbi.component_product_id, pbi.component_spec_g,
			       pbi.consume_unit, pbi.qty_per_unit, pbi.ratio_pct, pbi.unit_cost_snapshot
			FROM %[1]s.products p
			LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=p.id
			LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id=p.id
			JOIN %[1]s.production_bom_version_items pbi ON pbi.version_id=COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id)
			WHERE p.active=true
			  AND COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id,0) > 0
			UNION ALL
			SELECT p.id AS product_id,
			       bi.material_id, bi.component_type, bi.component_product_id, bi.component_spec_g,
			       bi.consume_unit, bi.qty_per_unit, bi.ratio_pct, bi.unit_cost_snapshot
			FROM %[1]s.products p
			LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=p.id
			LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id=p.id
			LEFT JOIN %[1]s.product_bom_sources bs ON bs.product_id=p.id
			JOIN %[1]s.product_bom_items bi ON bi.product_id=CASE
				WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
				ELSE p.id
			END
			WHERE p.active=true AND pbb.product_id IS NULL AND ppc.product_id IS NULL
		),
		effective_bom_items AS (
			SELECT p.id AS product_id,
			       pbi.material_id, pbi.component_type, pbi.component_product_id, pbi.component_spec_g,
			       pbi.consume_unit, pbi.qty_per_unit, pbi.ratio_pct, pbi.unit_cost_snapshot
			FROM product_scope p
			JOIN %[1]s.production_bom_version_items pbi ON pbi.version_id=p.production_bom_version_id
			UNION ALL
			SELECT p.id AS product_id,
			       bi.material_id, bi.component_type, bi.component_product_id, bi.component_spec_g,
			       bi.consume_unit, bi.qty_per_unit, bi.ratio_pct, bi.unit_cost_snapshot
			FROM product_scope p
			JOIN %[1]s.product_bom_items bi ON p.production_bom_version_id=0 AND bi.product_id=p.bom_product_id
		),
		finished_product_cost AS (
			SELECT p.id AS product_id,
			       COALESCE(SUM(COALESCE(mv.weighted_unit_cost, m.purchase_price, 0) * COALESCE(bi.ratio_pct,0) / 100.0),0) AS green_cost_per_kg
			FROM %[1]s.products p
			LEFT JOIN all_effective_bom_items bi ON bi.product_id = p.id
				AND COALESCE(NULLIF(bi.component_type,''),'material') = 'material'
				AND COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') = 'ratio_pct'
			LEFT JOIN %[1]s.materials m ON m.id = bi.material_id
			LEFT JOIN material_valuation mv ON mv.material_id = m.id
			WHERE p.active = true
			GROUP BY p.id
		),
		bom_unit_cost AS (
			SELECT p.id AS product_id,
			       COALESCE(SUM(CASE
			         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') = 'g_per_bag'
			         THEN COALESCE(bi.qty_per_unit,0) / 1000.0 * COALESCE(mv.weighted_unit_cost, m.purchase_price, 0)
			         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') IN ('unit_per_bag','unit_per_box')
			         THEN COALESCE(bi.qty_per_unit,0) * COALESCE(NULLIF(m.purchase_price,0), mv.weighted_unit_cost, 0)
			         ELSE 0
			       END),0) AS bom_cost_per_unit
			FROM product_scope p
			LEFT JOIN effective_bom_items bi ON bi.product_id = p.id
				AND COALESCE(NULLIF(bi.component_type,''),'material') = 'material'
			LEFT JOIN %[1]s.materials m ON m.id = bi.material_id
			LEFT JOIN material_valuation mv ON mv.material_id = m.id
			GROUP BY p.id
		),
		finished_component_cost AS (
			SELECT bi.product_id,
			       SUM(COALESCE(fpc.green_cost_per_kg,0) * COALESCE(NULLIF(bi.qty_per_unit,0), NULLIF(bi.component_spec_g,0), 1))
			       / NULLIF(SUM(COALESCE(NULLIF(bi.qty_per_unit,0), NULLIF(bi.component_spec_g,0), 1)),0) AS finished_green_cost_per_kg
			FROM all_effective_bom_items bi
			JOIN finished_product_cost fpc ON fpc.product_id = bi.component_product_id
			WHERE COALESCE(NULLIF(bi.component_type,''),'material') = 'finished_product'
			GROUP BY bi.product_id
		),
		operation_unit_cost AS (
			SELECT template_id,
			       COALESCE(SUM(CASE
			         WHEN COALESCE(NULLIF(cost_type,''),'fixed') IN ('fixed','per_unit','per_quote_unit')
			         THEN COALESCE(cost_rate,0)
			         ELSE 0
			       END),0) AS operation_cost_per_unit,
			       COALESCE(SUM(CASE
			         WHEN COALESCE(NULLIF(cost_type,''),'fixed') IN ('per_kg','per_kg_output','per_finished_kg')
			         THEN COALESCE(cost_rate,0)
			         ELSE 0
			       END),0) AS operation_cost_per_kg
			FROM %[1]s.operation_template_steps
			WHERE active=true
			GROUP BY template_id
		),
		production_config_attrs AS (
			SELECT ppcf.product_id,
			       COALESCE(jsonb_object_agg(ppcf.field_key,
			         COALESCE(to_jsonb(ppcf.value_text), to_jsonb(ppcf.value_number), to_jsonb(ppcf.value_bool), 'null'::jsonb)
			       ) FILTER (WHERE ppcf.show_in_price_list=true AND NULLIF(ppcf.field_key,'') IS NOT NULL), '{}'::jsonb) AS production_config_attrs_json,
			       COALESCE(jsonb_agg(jsonb_build_object(
			         'key', ppcf.field_key,
			         'label', ppcf.label,
			         'field_type', ppcf.field_type,
			         'unit', ppcf.unit,
			         'show_in_price_list', ppcf.show_in_price_list,
			         'sort_order', ppcf.sort_order
			       ) ORDER BY ppcf.sort_order, ppcf.id) FILTER (WHERE ppcf.show_in_price_list=true AND NULLIF(ppcf.field_key,'') IS NOT NULL), '[]'::jsonb) AS production_config_attrs_schema_json
			FROM %[1]s.product_production_config_fields ppcf
			GROUP BY ppcf.product_id
		),
		alias_config_attrs AS (
			SELECT cpa.id AS alias_id,
			       COALESCE(jsonb_object_agg(ppcf.field_key,
			         to_jsonb(COALESCE(NULLIF(cpaf.value_text,''), ppcf.value_text, ''))
			       ) FILTER (WHERE ppcf.show_in_price_list=true AND NULLIF(ppcf.field_key,'') IS NOT NULL), '{}'::jsonb) AS alias_attrs_json
			FROM %[1]s.customer_product_aliases cpa
			JOIN %[1]s.product_production_config_fields ppcf ON ppcf.product_id=cpa.product_id
			LEFT JOIN %[1]s.customer_product_alias_industry_field_values cpaf
			  ON cpaf.alias_id=cpa.id AND lower(cpaf.field_key)=lower(ppcf.field_key)
			WHERE $2 > 0
			  AND cpa.customer_id=$2
			  AND cpa.active=true
			  AND cpa.include_in_price_list=true
			GROUP BY cpa.id
		)
		SELECT p.id,
		       CASE WHEN $2 > 0 THEN COALESCE(NULLIF(p.customer_product_display_name,''), p.name) ELSE p.name END,
		       'SKU-' || p.id::text,
		       p.name,
		       COALESCE(p.customer_product_alias_id,0),
		       COALESCE(p.customer_product_display_name,''),
		       COALESCE(p.customer_item_code,''),
		       COALESCE(p.brand_name,''),
		       COALESCE(p.display_category_id,0),
		       COALESCE(p.display_category_name,''),
		       COALESCE(p.current_classification_template_id,0),
		       COALESCE(p.current_classification_template_name,''),
		       COALESCE(p.current_classification_category_id,0),
		       COALESCE(p.current_classification_category_name,''),
		       COALESCE(base_p.name, p.name),
		       COALESCE(p.roast_level, ''),
		       COALESCE(CASE WHEN $2 > 0 THEN NULLIF(alias_attrs.alias_attrs_json::text,'{}') ELSE NULL END, NULLIF(pca.production_config_attrs_json::text,'{}'), NULLIF(bound_bv.special_attrs_json::text,'{}'), NULLIF(p.special_attrs_json::text,'{}'), '{}'),
		       CASE WHEN $2 > 0 THEN $2 ELSE COALESCE(p.customer_id, 0) END,
		       COALESCE(p.base_product_id, 0),
		       COALESCE(NULLIF(p.visibility, ''), 'public'),
		       COALESCE(p.custom_type, ''),
		       COALESCE(NULLIF(p.product_kind,''), 'roasted'),
		       COALESCE(p.drip_bag_grams, 10)::float8,
		       COALESCE(p.drip_box_bag_count, 10),
		       COALESCE(p.product_category_id, 0),
		       COALESCE(p.product_category_position, 0),
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(parent_pc.id,0) ELSE COALESCE(pc.id,0) END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.id,0) ELSE 0 END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(parent_pc.name,'') ELSE COALESCE(pc.name,'') END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(parent_pc.position,0) ELSE COALESCE(pc.position,0) END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.name,'') ELSE '' END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.position,0) ELSE 0 END,
		       COALESCE(
			           NULLIF(alias_config.gradient_template_id,0),
			           NULLIF(p_config.gradient_template_id,0),
			           NULLIF(classification_category_config.gradient_template_id,0),
			           NULLIF(classification_template_config.gradient_template_id,0),
			           NULLIF(p.customer_product_alias_gradient_template_id,0),
			           NULLIF(cpro.gradient_template_id,0),
			           NULLIF(cpti.gradient_template_id,0),
			           NULLIF(p.gradient_template_id_override,0),
		           0
		       ) AS effective_gradient_template_id,
		       COALESCE(
			           NULLIF(alias_config.operation_template_id,0),
			           NULLIF(p_config.operation_template_id,0),
			           NULLIF(classification_category_config.operation_template_id,0),
			           NULLIF(classification_template_config.operation_template_id,0),
			           NULLIF(cpro.operation_template_id,0),
			           NULLIF(cpti.operation_template_id,0),
			           NULLIF(p.operation_template_id_override,0),
			           NULLIF(pc.operation_template_id,0),
		           NULLIF(parent_pc.operation_template_id,0),
		           0
		       ) AS effective_operation_template_id,
		       COALESCE(
			           NULLIF(alias_config.price_list_rule_json::text,'{}'),
			           NULLIF(p_config.price_list_rule_json::text,'{}'),
			           NULLIF(classification_category_config.price_list_rule_json::text,'{}'),
			           NULLIF(classification_template_config.price_list_rule_json::text,'{}'),
			           NULLIF(cpro.price_list_rule_json::text,'{}'),
			           NULLIF(cpti.price_list_rule_json::text,'{}'),
			           NULLIF(p.unit_rule_override_json->>'price_list_rule_json',''),
			           NULLIF(pc.price_list_rule_json::text,'{}'),
		           NULLIF(parent_pc.price_list_rule_json::text,'{}'),
		           '{}'
		       ) AS effective_price_list_rule_json,
		       COALESCE(
			           NULLIF(pca.production_config_attrs_schema_json::text,'[]'),
			           NULLIF(bound_bv.special_attrs_schema_json::text,'[]'),
			           NULLIF(alias_config.special_attrs_schema_json::text,'[]'),
			           NULLIF(p_config.special_attrs_schema_json::text,'[]'),
			           NULLIF(classification_category_config.special_attrs_schema_json::text,'[]'),
			           NULLIF(classification_template_config.special_attrs_schema_json::text,'[]'),
			           NULLIF(pc_config.special_attrs_schema_json::text,'[]'),
		           NULLIF(parent_pc_config.special_attrs_schema_json::text,'[]'),
		           '[]'
		       ) AS effective_special_attrs_schema_json,
		       COALESCE(
			           NULLIF(alias_config.inventory_unit,''),
			           NULLIF(p_config.inventory_unit,''),
			           NULLIF(classification_category_config.inventory_unit,''),
			           NULLIF(classification_template_config.inventory_unit,''),
			           NULLIF(alias_legacy_unit.inventory_unit,''),
			           NULLIF(cpro.unit_rule_json->>'inventory_unit',''),
			           NULLIF(cpti.unit_rule_json->>'inventory_unit',''),
			           NULLIF(p.unit_rule_override_json->>'inventory_unit',''),
			           NULLIF(pc.inventory_unit,''),
		           NULLIF(parent_pc.inventory_unit,''),
		           'kg'
		       ),
		       COALESCE(
			           NULLIF(alias_config.quote_unit,''),
			           NULLIF(p_config.quote_unit,''),
			           NULLIF(classification_category_config.quote_unit,''),
			           NULLIF(classification_template_config.quote_unit,''),
			           NULLIF(alias_legacy_unit.quote_unit,''),
			           NULLIF(cpro.unit_rule_json->>'quote_unit',''),
			           NULLIF(cpti.unit_rule_json->>'quote_unit',''),
			           NULLIF(p.unit_rule_override_json->>'quote_unit',''),
			           NULLIF(pc.quote_unit,''),
		           NULLIF(parent_pc.quote_unit,''),
		           'kg'
		       ),
		       COALESCE(
			           NULLIF(alias_config.order_unit,''),
			           NULLIF(p_config.order_unit,''),
			           NULLIF(classification_category_config.order_unit,''),
			           NULLIF(classification_template_config.order_unit,''),
			           NULLIF(alias_legacy_unit.order_unit,''),
			           NULLIF(cpro.unit_rule_json->>'order_unit',''),
			           NULLIF(cpti.unit_rule_json->>'order_unit',''),
			           NULLIF(p.unit_rule_override_json->>'order_unit',''),
			           NULLIF(pc.order_unit,''),
		           NULLIF(parent_pc.order_unit,''),
		           'kg'
		       ),
		       COALESCE(
		           NULLIF(alias_config.unit_conversion_json::text,'{}'),
		           NULLIF(p_config.unit_conversion_json::text,'{}'),
		           NULLIF(classification_category_config.unit_conversion_json::text,'{}'),
		           NULLIF(classification_template_config.unit_conversion_json::text,'{}'),
		           NULLIF(alias_legacy_unit.unit_conversion_json::text,'{}'),
		           NULLIF(cpro.unit_rule_json->>'unit_conversion_json',''),
		           NULLIF(cpro.unit_rule_json->>'conversion_json',''),
		           NULLIF(cpti.unit_rule_json->>'unit_conversion_json',''),
			           NULLIF(cpti.unit_rule_json->>'conversion_json',''),
			           NULLIF(p.unit_rule_override_json->>'unit_conversion_json',''),
			           NULLIF(p.unit_rule_override_json->>'conversion_json',''),
			           NULLIF(pc.unit_conversion_json::text,'{}'),
		           NULLIF(parent_pc.unit_conversion_json::text,'{}'),
		           '{}'
		       ),
		       COALESCE(
			           alias_config.integer_unit,
			           p_config.integer_unit,
			           classification_category_config.integer_unit,
			           classification_template_config.integer_unit,
			           alias_legacy_unit.integer_unit,
			           CASE WHEN lower(cpro.unit_rule_json->>'integer_unit') IN ('true','1','yes') THEN true WHEN lower(cpro.unit_rule_json->>'integer_unit') IN ('false','0','no') THEN false ELSE NULL END,
			           CASE WHEN lower(cpti.unit_rule_json->>'integer_unit') IN ('true','1','yes') THEN true WHEN lower(cpti.unit_rule_json->>'integer_unit') IN ('false','0','no') THEN false ELSE NULL END,
			           CASE WHEN lower(p.unit_rule_override_json->>'integer_unit') IN ('true','1','yes') THEN true WHEN lower(p.unit_rule_override_json->>'integer_unit') IN ('false','0','no') THEN false ELSE NULL END,
			           pc.integer_unit,
		           parent_pc.integer_unit,
		           false
		       ),
		       COALESCE(pps.product_price_snapshots_json, '[]'),
		       p.margin_rate_override::float8,
		       COALESCE(NULLIF(p.production_config_yield_rate,0), NULLIF(bound_bv.yield_rate,0), NULLIF(b.yield_rate,0), $1),
		       CASE
		           WHEN COALESCE(NULLIF(p.product_kind,''), 'roasted') = 'green_bean'
		           THEN COALESCE(SUM(COALESCE(NULLIF(bi.unit_cost_snapshot,0), m.purchase_price, 0) * COALESCE(bi.ratio_pct,0) / 100.0),0)
		           WHEN COALESCE(NULLIF(p.product_kind,''), 'roasted') = 'drip_bag' AND COALESCE(fcc.finished_green_cost_per_kg,0) > 0
		           THEN COALESCE(fcc.finished_green_cost_per_kg,0)
		           ELSE COALESCE(SUM(COALESCE(mv.weighted_unit_cost, m.purchase_price, 0) * COALESCE(bi.ratio_pct,0) / 100.0),0)
		       END,
		       COALESCE(MAX(buc.bom_cost_per_unit),0)::float8,
		       COALESCE(MAX(ouc.operation_cost_per_unit),0)::float8,
		       COALESCE(MAX(ouc.operation_cost_per_kg),0)::float8,
		       COALESCE(string_agg(DISTINCT NULLIF(bp.flavor, ''), ' / ') FILTER (WHERE NULLIF(bp.flavor, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.origin, ''), ' / ') FILTER (WHERE NULLIF(bp.origin, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.processing_station, ''), ' / ') FILTER (WHERE NULLIF(bp.processing_station, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.variety, ''), ' / ') FILTER (WHERE NULLIF(bp.variety, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.process_method, ''), ' / ') FILTER (WHERE NULLIF(bp.process_method, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.grade, ''), ' / ') FILTER (WHERE NULLIF(bp.grade, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.altitude, ''), ' / ') FILTER (WHERE NULLIF(bp.altitude, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.bean_list_note, ''), ' / ') FILTER (WHERE NULLIF(bp.bean_list_note, '') IS NOT NULL), ''),
		       COALESCE(NULLIF(bound_bom.status,''), NULLIF(b.status,''), CASE WHEN b.product_id IS NULL AND bound_bom.id IS NULL THEN 'missing' ELSE 'active' END),
		       COALESCE(bound_bv.id, active_bv.id, 0),
		       COALESCE(bound_bv.version_no, active_bv.version_no, ''),
		       COALESCE(NULLIF(p.bom_usage_mode,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'owned' END),
		       COALESCE(qc.factory_flavor_description, ''),
		       COALESCE(qc.moisture, ''),
		       COALESCE(qc.density, ''),
		       COALESCE(qc.inspection_created_at, ''),
		       COALESCE(qc.inspection_reference_no, '')
		FROM product_scope p
		LEFT JOIN %[1]s.product_bom b ON b.product_id = bom_product_id
		LEFT JOIN %[1]s.bom_versions active_bv ON active_bv.product_id=p.bom_product_id AND active_bv.status='active'
		LEFT JOIN %[1]s.production_boms bound_bom ON bound_bom.id=p.production_bom_id
		LEFT JOIN %[1]s.production_bom_versions bound_bv ON bound_bv.id=p.production_bom_version_id
		LEFT JOIN effective_bom_items bi ON bi.product_id = p.id
		LEFT JOIN %[1]s.materials m ON m.id = bi.material_id
		LEFT JOIN material_valuation mv ON mv.material_id = m.id
		LEFT JOIN %[1]s.material_bean_profiles bp ON bp.material_id = m.id
		LEFT JOIN %[1]s.products base_p ON base_p.id = p.base_product_id
			LEFT JOIN %[1]s.product_categories pc ON pc.id = p.product_category_id AND pc.active=true
			LEFT JOIN %[1]s.product_categories parent_pc ON parent_pc.id = pc.parent_id AND parent_pc.active=true
		LEFT JOIN %[1]s.product_config_templates p_config ON p_config.id = p.product_config_template_id AND p_config.active=true
		LEFT JOIN %[1]s.product_config_templates alias_config ON alias_config.id = p.customer_product_alias_product_config_template_id AND alias_config.active=true
		LEFT JOIN %[1]s.product_config_templates classification_category_config ON classification_category_config.id = p.current_classification_category_product_config_template_id AND classification_category_config.active=true
		LEFT JOIN %[1]s.product_config_templates classification_template_config ON classification_template_config.id = p.current_classification_template_product_config_template_id AND classification_template_config.active=true
			LEFT JOIN %[1]s.product_config_templates pc_config ON pc_config.id = pc.product_config_template_id AND pc_config.active=true
		LEFT JOIN %[1]s.product_config_templates parent_pc_config ON parent_pc_config.id = parent_pc.product_config_template_id AND parent_pc_config.active=true
		LEFT JOIN %[1]s.product_unit_templates alias_legacy_unit ON alias_legacy_unit.id=p.customer_product_alias_unit_template_id AND alias_legacy_unit.active=true
		LEFT JOIN production_config_attrs pca ON pca.product_id=p.id
		LEFT JOIN alias_config_attrs alias_attrs ON alias_attrs.alias_id=p.customer_product_alias_id
		LEFT JOIN %[1]s.customers rule_customer ON rule_customer.id = $2 AND rule_customer.active=true
		LEFT JOIN %[1]s.customer_product_rule_template_items cpti
		  ON cpti.active=true
		 AND cpti.template_id=COALESCE(rule_customer.customer_product_rule_template_id,0)
		 AND cpti.product_subtype_category_id=CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.id,0) ELSE 0 END
		LEFT JOIN %[1]s.customer_product_rule_overrides cpro
		  ON cpro.active=true
		 AND cpro.customer_id=$2
		 AND cpro.product_subtype_category_id=CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.id,0) ELSE 0 END
		LEFT JOIN operation_unit_cost ouc
		  ON ouc.template_id=COALESCE(
			           NULLIF(alias_config.operation_template_id,0),
			           NULLIF(p_config.operation_template_id,0),
			           NULLIF(classification_category_config.operation_template_id,0),
			           NULLIF(classification_template_config.operation_template_id,0),
			           NULLIF(cpro.operation_template_id,0),
			           NULLIF(cpti.operation_template_id,0),
			           NULLIF(p.operation_template_id_override,0),
			           NULLIF(pc.operation_template_id,0),
		           NULLIF(parent_pc.operation_template_id,0),
		           0
		       )
		LEFT JOIN finished_component_cost fcc ON fcc.product_id = p.id
		LEFT JOIN bom_unit_cost buc ON buc.product_id = p.id
		LEFT JOIN LATERAL (
			SELECT COALESCE(NULLIF(qi.metrics_json->>'factory_flavor_description',''), NULLIF(qi.metrics_json->>'factory_flavor',''), NULLIF(qi.metrics_json->>'工厂风味描述',''), '') AS factory_flavor_description,
			       COALESCE(NULLIF(qi.metrics_json->>'moisture',''), NULLIF(qi.metrics_json->>'水分',''), '') AS moisture,
			       COALESCE(NULLIF(qi.metrics_json->>'density',''), NULLIF(qi.metrics_json->>'密度',''), '') AS density,
			       to_char(qi.created_at,'YYYY-MM-DD HH24:MI') AS inspection_created_at,
			       qi.reference_no AS inspection_reference_no
			FROM %[1]s.quality_inspections qi
			LEFT JOIN %[1]s.work_orders qi_work_order
			  ON (qi.reference_type='work_order' OR qi.scope='work_order')
			 AND qi_work_order.work_order_no=qi.reference_no
			LEFT JOIN %[1]s.stock_batches qi_work_batch
			  ON (qi.reference_type='work_order' OR qi.scope='work_order')
			 AND qi_work_batch.item_type='finished_product'
			 AND qi_work_batch.source_doc_id=qi_work_order.running_item_id
			LEFT JOIN %[1]s.stock_batches qi_finished_batch
			  ON (qi.reference_type='finished_batch' OR qi.scope='finished_batch')
			 AND qi_finished_batch.item_type='finished_product'
			 AND qi_finished_batch.batch_code=qi.reference_no
			WHERE qi.result='pass'
			  AND (
			    ((qi.reference_type='work_order' OR qi.scope='work_order') AND (qi_work_order.product_id=p.bom_product_id OR qi_work_batch.item_id=p.bom_product_id))
			    OR ((qi.reference_type='finished_batch' OR qi.scope='finished_batch') AND qi_finished_batch.item_id=p.bom_product_id)
			  )
			ORDER BY qi.created_at DESC, qi.id DESC
			LIMIT 1
		) qc ON true
		LEFT JOIN LATERAL (
			SELECT COALESCE(jsonb_agg(jsonb_build_object(
				'source_price_record_id', ppr.id,
				'final_unit_price', ppr.final_unit_price,
				'price_unit', ppr.price_unit,
				'currency', ppr.currency,
				'price_group_id', COALESCE(ppr.price_group_id,0),
				'price_group_name', COALESCE(ppr.price_group_name,''),
				'inventory_unit', ppr.inventory_unit,
				'inventory_conversion_json', COALESCE(ppr.inventory_conversion_json, '{}'::jsonb),
				'product_id', COALESCE(ppr.product_id,0),
				'customer_product_alias_id', COALESCE(ppr.customer_product_alias_id,0)
			) ORDER BY
				CASE
					WHEN COALESCE(p.customer_product_alias_id,0)>0 AND COALESCE(ppr.customer_product_alias_id,0)=COALESCE(p.customer_product_alias_id,0) THEN 0
					WHEN COALESCE(ppr.product_id,0)=p.id THEN 1
					ELSE 2
				END,
				COALESCE(ppr.price_group_id,0),
				ppr.id
			), '[]'::jsonb)::text AS product_price_snapshots_json
			FROM %[1]s.product_price_records ppr
			WHERE ppr.active=true
			  AND ppr.status='published'
			  AND (
			    (COALESCE(p.customer_product_alias_id,0)>0 AND COALESCE(ppr.customer_product_alias_id,0)=COALESCE(p.customer_product_alias_id,0))
			    OR (COALESCE(ppr.customer_product_alias_id,0)=0 AND COALESCE(ppr.product_id,0)=p.id)
			  )
		) pps ON true
		WHERE p.active = true
			GROUP BY p.id, p.name, p.customer_product_alias_id, p.customer_product_display_name, p.customer_item_code, p.brand_name, p.display_category_id, p.display_category_name, p.customer_product_alias_product_config_template_id, p.customer_product_alias_gradient_template_id, p.customer_product_alias_unit_template_id, p.current_classification_template_id, p.current_classification_template_name, p.current_classification_category_id, p.current_classification_category_name, p.current_classification_category_product_config_template_id, p.current_classification_template_product_config_template_id, p.bom_usage_mode, p.production_bom_id, p.production_bom_version_id, p.production_config_yield_rate, base_p.name, p.roast_level, p.special_attrs_json, p.customer_id, p.base_product_id, p.visibility, p.custom_type, p.product_kind, p.drip_bag_grams, p.drip_box_bag_count, p.product_category_id, p.product_category_position, p.product_config_template_id, p.gradient_template_id_override, p.operation_template_id_override, p.unit_rule_override_json, alias_config.gradient_template_id, alias_config.operation_template_id, alias_config.price_list_rule_json, alias_config.inventory_unit, alias_config.quote_unit, alias_config.order_unit, alias_config.unit_conversion_json, alias_config.integer_unit, alias_config.special_attrs_schema_json, p_config.gradient_template_id, p_config.operation_template_id, p_config.price_list_rule_json, p_config.inventory_unit, p_config.quote_unit, p_config.order_unit, p_config.unit_conversion_json, p_config.integer_unit, p_config.special_attrs_schema_json, classification_category_config.gradient_template_id, classification_category_config.operation_template_id, classification_category_config.price_list_rule_json, classification_category_config.inventory_unit, classification_category_config.quote_unit, classification_category_config.order_unit, classification_category_config.unit_conversion_json, classification_category_config.integer_unit, classification_category_config.special_attrs_schema_json, classification_template_config.gradient_template_id, classification_template_config.operation_template_id, classification_template_config.price_list_rule_json, classification_template_config.inventory_unit, classification_template_config.quote_unit, classification_template_config.order_unit, classification_template_config.unit_conversion_json, classification_template_config.integer_unit, classification_template_config.special_attrs_schema_json, pc.id, pc.level, pc.name, pc.position, pc.gradient_template_id, pc.operation_template_id, pc.price_list_rule_json, pc.inventory_unit, pc.quote_unit, pc.order_unit, pc.unit_conversion_json, pc.integer_unit, pc_config.special_attrs_schema_json, parent_pc.id, parent_pc.name, parent_pc.position, parent_pc.gradient_template_id, parent_pc.operation_template_id, parent_pc.price_list_rule_json, parent_pc.inventory_unit, parent_pc.quote_unit, parent_pc.order_unit, parent_pc.unit_conversion_json, parent_pc.integer_unit, parent_pc_config.special_attrs_schema_json, alias_legacy_unit.inventory_unit, alias_legacy_unit.quote_unit, alias_legacy_unit.order_unit, alias_legacy_unit.unit_conversion_json, alias_legacy_unit.integer_unit, pca.production_config_attrs_json, pca.production_config_attrs_schema_json, alias_attrs.alias_attrs_json, cpti.gradient_template_id, cpti.operation_template_id, cpti.price_list_rule_json, cpti.unit_rule_json, cpro.gradient_template_id, cpro.operation_template_id, cpro.price_list_rule_json, cpro.unit_rule_json, pps.product_price_snapshots_json, p.margin_rate_override, p.bom_product_id, b.yield_rate, b.status, b.product_id, active_bv.id, active_bv.version_no, bound_bom.id, bound_bom.status, bound_bv.id, bound_bv.version_no, bound_bv.yield_rate, bound_bv.special_attrs_json, bound_bv.special_attrs_schema_json, fcc.finished_green_cost_per_kg, qc.factory_flavor_description, qc.moisture, qc.density, qc.inspection_created_at, qc.inspection_reference_no
		ORDER BY p.name
	`, r.schema)
	rows, err := r.pool.Query(ctx, q, params.RoastYieldRate, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.ProductInput, 0)
	templateIDs := map[int64]bool{}
	templateIDByProduct := map[int64]int64{}
	for rows.Next() {
		var input domain.ProductInput
		var roastLevel string
		var fallbackYield float64
		var gradientTemplateID int64
		var productPriceSnapshotsJSON string
		if err := rows.Scan(
			&input.ProductID,
			&input.Name,
			&input.ProductCode,
			&input.ProductName,
			&input.CustomerProductAliasID,
			&input.CustomerProductDisplayName,
			&input.CustomerItemCode,
			&input.BrandName,
			&input.DisplayCategoryID,
			&input.DisplayCategoryName,
			&input.ClassificationTemplateID,
			&input.ClassificationTemplateName,
			&input.ClassificationCategoryID,
			&input.ClassificationCategoryName,
			&input.BeanListTemplateName,
			&roastLevel,
			&input.SpecialAttrsJSON,
			&input.CustomerID,
			&input.BaseProductID,
			&input.Visibility,
			&input.CustomType,
			&input.ProductKind,
			&input.DripBagGrams,
			&input.DripBoxBagCount,
			&input.ProductCategoryID,
			&input.ProductCategoryPosition,
			&input.ProductTypeCategoryID,
			&input.ProductSubtypeCategoryID,
			&input.CategoryPrimaryName,
			&input.CategoryPrimaryPosition,
			&input.CategorySecondaryName,
			&input.CategorySecondaryPosition,
			&gradientTemplateID,
			&input.OperationTemplateID,
			&input.PriceListRuleJSON,
			&input.SpecialAttrsSchemaJSON,
			&input.InventoryUnit,
			&input.QuoteUnit,
			&input.OrderUnit,
			&input.UnitConversionJSON,
			&input.IntegerUnit,
			&productPriceSnapshotsJSON,
			&input.MarginRateOverride,
			&fallbackYield,
			&input.GreenBeanCostPerKg,
			&input.BomCostPerUnit,
			&input.OperationCostPerUnit,
			&input.OperationCostPerKg,
			&input.Flavor,
			&input.Origin,
			&input.ProcessingStation,
			&input.Variety,
			&input.ProcessMethod,
			&input.Grade,
			&input.Altitude,
			&input.BeanListNote,
			&input.BomStatus,
			&input.BomVersionID,
			&input.BomVersionNo,
			&input.BomUsageMode,
			&input.BeanListQuality.FactoryFlavorDescription,
			&input.BeanListQuality.Moisture,
			&input.BeanListQuality.Density,
			&input.BeanListQuality.InspectionCreatedAt,
			&input.BeanListQuality.InspectionReferenceNo,
		); err != nil {
			return nil, err
		}
		input.ProductTypeName = input.CategoryPrimaryName
		input.ProductSubtypeName = input.CategorySecondaryName
		if gradientTemplateID > 0 {
			templateIDs[gradientTemplateID] = true
			templateIDByProduct[input.ProductID] = gradientTemplateID
		}
		_ = roastLevel
		input.ProductPriceSnapshots = productPriceSnapshotsFromJSON(productPriceSnapshotsJSON)
		input.YieldRate = fallbackYield
		input.ExpectedLossRate = 1 - fallbackYield
		if strings.TrimSpace(input.BomStatus) == "inactive" {
			input.Warnings = append(input.Warnings, "BOM已失效：请重新启用 BOM 后再发布价格策略")
		}
		input = domain.ApplyExcelCommercialPricingProfile(params, input)
		out = append(out, input)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	templates, err := r.loadGradientTemplatesByID(ctx, templateIDs)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if templateID := templateIDByProduct[out[i].ProductID]; templateID > 0 {
			if template := templates[templateID]; template != nil {
				out[i].GradientTemplate = template
			}
		}
	}
	return out, nil
}

func productPriceSnapshotsFromJSON(raw string) []domain.ProductPriceSnapshot {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var rows []domain.ProductPriceSnapshot
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil
	}
	out := make([]domain.ProductPriceSnapshot, 0, len(rows))
	for _, row := range rows {
		row.PriceUnit = strings.TrimSpace(row.PriceUnit)
		row.Currency = strings.TrimSpace(row.Currency)
		row.PriceGroupName = strings.TrimSpace(row.PriceGroupName)
		row.InventoryUnit = strings.TrimSpace(row.InventoryUnit)
		if row.SourcePriceRecordID <= 0 || row.FinalUnitPrice <= 0 || row.PriceUnit == "" {
			continue
		}
		if row.Currency == "" {
			row.Currency = "CNY"
		}
		if len(row.InventoryConversionJSON) == 0 || string(row.InventoryConversionJSON) == "null" {
			row.InventoryConversionJSON = json.RawMessage(`{}`)
		}
		out = append(out, row)
	}
	return out
}

func (r Repository) loadGradientTemplatesByID(ctx context.Context, ids map[int64]bool) (map[int64]*domain.GradientTemplate, error) {
	out := map[int64]*domain.GradientTemplate{}
	if len(ids) == 0 {
		return out, nil
	}
	idList := make([]int64, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, display_unit, active
		FROM %s.pricing_gradient_templates
		WHERE id = ANY($1)
	`, r.schema), idList)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		template := &domain.GradientTemplate{}
		if err := rows.Scan(&template.ID, &template.Name, &template.DisplayUnit, &template.Active); err != nil {
			return nil, err
		}
		out[template.ID] = template
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	tierRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, label, min_weight_g::float8, max_weight_g::float8, margin_rate::float8, position
		FROM %s.pricing_gradient_template_tiers
		WHERE active=true AND template_id = ANY($1)
		ORDER BY template_id, position, min_weight_g, id
	`, r.schema), idList)
	if err != nil {
		return nil, err
	}
	defer tierRows.Close()
	for tierRows.Next() {
		var templateID int64
		var tier domain.GradientTemplateTier
		if err := tierRows.Scan(&tier.ID, &templateID, &tier.Label, &tier.MinWeightG, &tier.MaxWeightG, &tier.MarginRate, &tier.Position); err != nil {
			return nil, err
		}
		if template := out[templateID]; template != nil {
			template.Tiers = append(template.Tiers, tier)
		}
	}
	if err := tierRows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) ListParameterSettings(ctx context.Context) ([]appcosting.ParameterSetting, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT key, label, value::float8, unit, to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.cost_parameters
		ORDER BY key
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]appcosting.ParameterSetting, 0)
	for rows.Next() {
		var row appcosting.ParameterSetting
		if err := rows.Scan(&row.Key, &row.Label, &row.Value, &row.Unit, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) UpdateParameterSetting(ctx context.Context, cmd appcosting.UpdateParameterCommand) (appcosting.ParameterSetting, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return appcosting.ParameterSetting{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return appcosting.ParameterSetting{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var old appcosting.ParameterSetting
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT key, label, value::float8, unit, to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.cost_parameters
		WHERE key=$1
		FOR UPDATE
	`, r.schema), strings.TrimSpace(cmd.Key)).Scan(&old.Key, &old.Label, &old.Value, &old.Unit, &old.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return appcosting.ParameterSetting{}, fmt.Errorf("setting not found")
		}
		return appcosting.ParameterSetting{}, err
	}

	var next appcosting.ParameterSetting
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.cost_parameters
		SET value=$2, updated_at=now()
		WHERE key=$1
		RETURNING key, label, value::float8, unit, to_char(updated_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), strings.TrimSpace(cmd.Key), cmd.Value).Scan(&next.Key, &next.Label, &next.Value, &next.Unit, &next.UpdatedAt); err != nil {
		return appcosting.ParameterSetting{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "cost_parameter", nil, "update", postgresinfra.StrPtr(next.Key), postgresinfra.StrPtr(fmt.Sprintf("%.6f", old.Value)), postgresinfra.StrPtr(fmt.Sprintf("%.6f", next.Value)), postgresinfra.AuditMeta{
		"key":   next.Key,
		"label": next.Label,
		"unit":  next.Unit,
	}); err != nil {
		return appcosting.ParameterSetting{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return appcosting.ParameterSetting{}, err
	}
	return next, nil
}

func (r Repository) ListDripPriceTemplates(ctx context.Context) ([]domain.DripPriceTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, active, bag_grams::float8, box_bag_count, include_packaging
		FROM %s.drip_price_templates
		ORDER BY active DESC, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.DripPriceTemplate, 0)
	templateIndex := map[int64]int{}
	for rows.Next() {
		var row domain.DripPriceTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.Active, &row.BagGrams, &row.BoxBagCount, &row.IncludePackaging); err != nil {
			return nil, err
		}
		templateIndex[row.ID] = len(out)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(out))
	for _, row := range out {
		ids = append(ids, row.ID)
	}
	tierRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, label, min_bags::float8, max_bags::float8, multiplier::float8, position, active
		FROM %s.drip_price_template_tiers
		WHERE template_id = ANY($1)
		ORDER BY template_id, position, min_bags, id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer tierRows.Close()
	for tierRows.Next() {
		var templateID int64
		var tier domain.DripPriceTemplateTier
		if err := tierRows.Scan(&tier.ID, &templateID, &tier.Label, &tier.MinBags, &tier.MaxBags, &tier.Multiplier, &tier.Position, &tier.Active); err != nil {
			return nil, err
		}
		if idx, ok := templateIndex[templateID]; ok {
			out[idx].Tiers = append(out[idx].Tiers, tier)
		}
	}
	return out, tierRows.Err()
}

func (r Repository) SaveDripPriceTemplate(ctx context.Context, cmd appcosting.SaveDripPriceTemplateCommand) (*domain.DripPriceTemplate, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	active := true
	includePackaging := true
	if cmd.ID == 0 {
		active = true
		if cmd.Active != nil {
			active = *cmd.Active
		}
		if cmd.IncludePackaging != nil {
			includePackaging = *cmd.IncludePackaging
		}
	}
	var id int64
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT active, include_packaging
			FROM %s.drip_price_templates
			WHERE id=$1
			FOR UPDATE
		`, r.schema), cmd.ID).Scan(&active, &includePackaging); err != nil {
			if err == pgx.ErrNoRows {
				return nil, fmt.Errorf("template not found")
			}
			return nil, err
		}
		if cmd.Active != nil {
			active = *cmd.Active
		}
		if cmd.IncludePackaging != nil {
			includePackaging = *cmd.IncludePackaging
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.drip_price_templates
			SET name=$2, active=$3, bag_grams=$4, box_bag_count=$5, include_packaging=$6, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, active, cmd.BagGrams, cmd.BoxBagCount, includePackaging).Scan(&id); err != nil {
			if err == pgx.ErrNoRows {
				return nil, fmt.Errorf("template not found")
			}
			return nil, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.drip_price_template_tiers WHERE template_id=$1`, r.schema), id); err != nil {
			return nil, err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.drip_price_templates(name, active, bag_grams, box_bag_count, include_packaging)
			VALUES($1,$2,$3,$4,$5)
			RETURNING id
		`, r.schema), cmd.Name, active, cmd.BagGrams, cmd.BoxBagCount, includePackaging).Scan(&id); err != nil {
			return nil, err
		}
	}
	insertTier := fmt.Sprintf(`
		INSERT INTO %s.drip_price_template_tiers(template_id, label, min_bags, max_bags, multiplier, position, active)
		VALUES($1,$2,$3,$4,$5,$6,$7)
	`, r.schema)
	for i, tier := range cmd.Tiers {
		position := tier.Position
		if position <= 0 {
			position = i + 1
		}
		if _, err := tx.Exec(ctx, insertTier, id, tier.Label, tier.MinBags, tier.MaxBags, tier.Multiplier, position, true); err != nil {
			return nil, err
		}
	}
	action := "create"
	if cmd.ID > 0 {
		action = "update"
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "drip_price_template", &id, action, postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{
		"template_id":       id,
		"bag_grams":         cmd.BagGrams,
		"box_bag_count":     cmd.BoxBagCount,
		"include_packaging": includePackaging,
		"tier_count":        len(cmd.Tiers),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	rows, err := r.ListDripPriceTemplates(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i], nil
		}
	}
	return nil, fmt.Errorf("template not found")
}

func (r Repository) DeactivateDripPriceTemplate(ctx context.Context, cmd appcosting.DeactivateDripPriceTemplateCommand) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.drip_price_templates SET active=false, updated_at=now() WHERE id=$1`, r.schema), cmd.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("template not found")
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "drip_price_template", &cmd.ID, "deactivate", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{
		"template_id": cmd.ID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListBeanListPublications(ctx context.Context, query appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error) {
	whereClause := "WHERE publication_purpose=$1 AND list_type=$2 AND owner_type=$3 AND owner_key=$4"
	args := []any{strings.TrimSpace(query.PublicationPurpose), strings.TrimSpace(query.ListType), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	if query.ProductTypeCategoryID > 0 {
		whereClause = "WHERE publication_purpose=$1 AND product_type_category_id=$2 AND owner_type=$3 AND owner_key=$4"
		args = []any{strings.TrimSpace(query.PublicationPurpose), query.ProductTypeCategoryID, strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(publication_purpose,''),'factory_supply'),
		       list_type,
		       COALESCE(product_type_category_id,0),
		       COALESCE(product_type_name,''),
		       COALESCE(classification_template_id,0),
		       COALESCE(classification_template_name,''),
		       COALESCE(classification_category_id,0),
		       COALESCE(classification_category_name,''),
		       version_no,
		       status,
		       owner_type,
		       owner_key,
		       COALESCE(price_source_publication_id,0),
		       COALESCE(style_source_publication_id,0),
		       source_version_no,
		       config_json,
		       content_json,
		       changelog,
		       to_char(published_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		       to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.bean_list_publications
		%s
		ORDER BY CASE WHEN status='published' THEN 0 ELSE 1 END, created_at DESC, id DESC
	`, r.schema, whereClause), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]appcosting.BeanListPublication, 0)
	for rows.Next() {
		row, err := scanBeanListPublication(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) PublishedBeanList(ctx context.Context, query appcosting.BeanListPublicationQuery) (*appcosting.BeanListPublication, error) {
	whereClause := "publication_purpose=$1 AND list_type=$2 AND owner_type=$3 AND owner_key=$4"
	args := []any{strings.TrimSpace(query.PublicationPurpose), strings.TrimSpace(query.ListType), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	if query.ProductTypeCategoryID > 0 {
		whereClause = "publication_purpose=$1 AND product_type_category_id=$2 AND owner_type=$3 AND owner_key=$4"
		args = []any{strings.TrimSpace(query.PublicationPurpose), query.ProductTypeCategoryID, strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	}
	row, err := scanBeanListPublication(r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(publication_purpose,''),'factory_supply'),
		       list_type,
		       COALESCE(product_type_category_id,0),
		       COALESCE(product_type_name,''),
		       COALESCE(classification_template_id,0),
		       COALESCE(classification_template_name,''),
		       COALESCE(classification_category_id,0),
		       COALESCE(classification_category_name,''),
		       version_no,
		       status,
		       owner_type,
		       owner_key,
		       COALESCE(price_source_publication_id,0),
		       COALESCE(style_source_publication_id,0),
		       source_version_no,
		       config_json,
		       content_json,
		       changelog,
		       to_char(published_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		       to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.bean_list_publications
		WHERE %s AND status='published'
		ORDER BY published_at DESC, id DESC
		LIMIT 1
	`, r.schema, whereClause), args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r Repository) LoadBeanListPublication(ctx context.Context, query appcosting.BeanListPublicationQuery, publicationID int64) (*appcosting.BeanListPublication, error) {
	whereClause := "id=$1 AND publication_purpose=$2 AND list_type=$3 AND owner_type=$4 AND owner_key=$5"
	args := []any{publicationID, strings.TrimSpace(query.PublicationPurpose), strings.TrimSpace(query.ListType), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	if query.ProductTypeCategoryID > 0 {
		whereClause = "id=$1 AND publication_purpose=$2 AND product_type_category_id=$3 AND owner_type=$4 AND owner_key=$5"
		args = []any{publicationID, strings.TrimSpace(query.PublicationPurpose), query.ProductTypeCategoryID, strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	}
	row, err := scanBeanListPublication(r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(publication_purpose,''),'factory_supply'),
		       list_type,
		       COALESCE(product_type_category_id,0),
		       COALESCE(product_type_name,''),
		       COALESCE(classification_template_id,0),
		       COALESCE(classification_template_name,''),
		       COALESCE(classification_category_id,0),
		       COALESCE(classification_category_name,''),
		       version_no,
		       status,
		       owner_type,
		       owner_key,
		       COALESCE(price_source_publication_id,0),
		       COALESCE(style_source_publication_id,0),
		       source_version_no,
		       config_json,
		       content_json,
		       changelog,
		       to_char(published_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		       to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.bean_list_publications
		WHERE %s
	`, r.schema, whereClause), args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appcosting.ErrBeanListPublicationNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r Repository) LoadBeanListPublicationAsset(ctx context.Context, publicationID int64, assetType string) (appcosting.BeanListPublicationAsset, error) {
	assetType = strings.TrimSpace(assetType)
	if assetType == "" {
		assetType = "pdf"
	}
	var asset appcosting.BeanListPublicationAsset
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT publication_id, asset_type, content_type, cache_key, payload
		FROM %s.bean_list_publication_assets
		WHERE publication_id=$1 AND asset_type=$2
	`, r.schema), publicationID, assetType).Scan(&asset.PublicationID, &asset.AssetType, &asset.ContentType, &asset.CacheKey, &asset.Payload)
	if err != nil {
		if err == pgx.ErrNoRows {
			return appcosting.BeanListPublicationAsset{}, appcosting.ErrBeanListPublicationNotFound
		}
		return appcosting.BeanListPublicationAsset{}, err
	}
	return asset, nil
}

func (r Repository) SaveBeanListPublicationAsset(ctx context.Context, asset appcosting.BeanListPublicationAsset, actor string) (appcosting.BeanListPublicationAsset, error) {
	asset.AssetType = strings.TrimSpace(asset.AssetType)
	if asset.AssetType == "" {
		asset.AssetType = "pdf"
	}
	asset.ContentType = strings.TrimSpace(asset.ContentType)
	if asset.ContentType == "" {
		asset.ContentType = "application/octet-stream"
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return appcosting.BeanListPublicationAsset{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var created bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
			WITH inserted AS (
				INSERT INTO %[1]s.bean_list_publication_assets(publication_id, asset_type, content_type, cache_key, payload, created_by)
				VALUES($1,$2,$3,$4,$5,$6)
				ON CONFLICT(publication_id, asset_type) DO UPDATE
				SET content_type=EXCLUDED.content_type,
				    cache_key=EXCLUDED.cache_key,
				    payload=EXCLUDED.payload,
				    created_by=EXCLUDED.created_by,
				    updated_at=now()
				RETURNING publication_id, asset_type, content_type, cache_key, payload, true
			)
		SELECT publication_id, asset_type, content_type, cache_key, payload, true FROM inserted
		UNION ALL
		SELECT publication_id, asset_type, content_type, cache_key, payload, false
		FROM %[1]s.bean_list_publication_assets
		WHERE publication_id=$1 AND asset_type=$2 AND NOT EXISTS (SELECT 1 FROM inserted)
		LIMIT 1
	`, r.schema), asset.PublicationID, asset.AssetType, asset.ContentType, asset.CacheKey, asset.Payload, strings.TrimSpace(actor)).
		Scan(&asset.PublicationID, &asset.AssetType, &asset.ContentType, &asset.CacheKey, &asset.Payload, &created); err != nil {
		return appcosting.BeanListPublicationAsset{}, err
	}
	if created {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, strings.TrimSpace(actor), "bean_list_publication_asset", &asset.PublicationID, "create", postgresinfra.StrPtr("asset_type"), nil, postgresinfra.StrPtr(asset.AssetType), postgresinfra.AuditMeta{
			"publication_id": asset.PublicationID,
			"asset_type":     asset.AssetType,
			"cache_key":      asset.CacheKey,
		}); err != nil {
			return appcosting.BeanListPublicationAsset{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return appcosting.BeanListPublicationAsset{}, err
	}
	return asset, nil
}

func (r Repository) PublishBeanList(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := validateBeanListProductScope(ctx, tx, r.schema, cmd); err != nil {
		return nil, err
	}

	config, err := json.Marshal(cmd.Config)
	if err != nil {
		return nil, err
	}
	content, err := json.Marshal(cmd.Content)
	if err != nil {
		return nil, err
	}
	var published appcosting.BeanListPublication
	var configJSON, contentJSON []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications(publication_purpose, list_type, product_type_category_id, product_type_name, classification_template_id, classification_template_name, classification_category_id, classification_category_name, version_no, status, owner_type, owner_key, price_source_publication_id, style_source_publication_id, source_version_no, config_json, content_json, changelog, actor)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'published',$10,$11,NULLIF($12,0),NULLIF($13,0),$14,$15::jsonb,$16::jsonb,$17,$18)
		RETURNING id, COALESCE(NULLIF(publication_purpose,''),'factory_supply'), list_type, COALESCE(product_type_category_id,0), COALESCE(product_type_name,''), COALESCE(classification_template_id,0), COALESCE(classification_template_name,''), COALESCE(classification_category_id,0), COALESCE(classification_category_name,''), version_no, status, owner_type, owner_key, COALESCE(price_source_publication_id,0), COALESCE(style_source_publication_id,0), source_version_no, config_json, content_json, changelog,
		          to_char(published_at,'YYYY-MM-DD HH24:MI'),
		          COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		          to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), cmd.PublicationPurpose, cmd.ListType, cmd.ProductTypeCategoryID, cmd.ProductTypeName, cmd.ClassificationTemplateID, cmd.ClassificationTemplateName, cmd.ClassificationCategoryID, cmd.ClassificationCategoryName, cmd.Version, cmd.OwnerType, cmd.OwnerKey, cmd.PriceSourcePublicationID, cmd.StyleSourcePublicationID, cmd.SourceVersion, config, content, cmd.Changelog, cmd.Actor).Scan(
		&published.ID,
		&published.PublicationPurpose,
		&published.ListType,
		&published.ProductTypeCategoryID,
		&published.ProductTypeName,
		&published.ClassificationTemplateID,
		&published.ClassificationTemplateName,
		&published.ClassificationCategoryID,
		&published.ClassificationCategoryName,
		&published.Version,
		&published.Status,
		&published.OwnerType,
		&published.OwnerKey,
		&published.PriceSourcePublicationID,
		&published.StyleSourcePublicationID,
		&published.SourceVersion,
		&configJSON,
		&contentJSON,
		&published.Changelog,
		&published.PublishedAt,
		&published.WithdrawnAt,
		&published.CreatedAt,
	); err != nil {
		return nil, err
	}
	published.Config = map[string]any{}
	published.Content = map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &published.Config); err != nil {
			return nil, err
		}
	}
	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &published.Content); err != nil {
			return nil, err
		}
	}
	if published.ID <= 0 {
		return nil, fmt.Errorf("publish failed")
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &published.ID, "publish", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("published"), postgresinfra.AuditMeta{
		"publication_purpose":      cmd.PublicationPurpose,
		"list_type":                cmd.ListType,
		"product_type_category_id": cmd.ProductTypeCategoryID,
		"product_type_name":        cmd.ProductTypeName,
		"version":                  cmd.Version,
		"owner_type":               cmd.OwnerType,
		"owner_key":                cmd.OwnerKey,
		"price_source_publication": cmd.PriceSourcePublicationID,
		"style_source_publication": cmd.StyleSourcePublicationID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &published, nil
}

func (r Repository) SaveBeanListDraft(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := validateBeanListProductScope(ctx, tx, r.schema, cmd); err != nil {
		return nil, err
	}
	config, err := json.Marshal(cmd.Config)
	if err != nil {
		return nil, err
	}
	content, err := json.Marshal(cmd.Content)
	if err != nil {
		return nil, err
	}
	var draft appcosting.BeanListPublication
	var configJSON, contentJSON []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications(publication_purpose, list_type, product_type_category_id, product_type_name, classification_template_id, classification_template_name, classification_category_id, classification_category_name, version_no, status, owner_type, owner_key, price_source_publication_id, style_source_publication_id, source_version_no, config_json, content_json, changelog, actor)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft',$10,$11,NULLIF($12,0),NULLIF($13,0),$14,$15::jsonb,$16::jsonb,$17,$18)
		RETURNING id, COALESCE(NULLIF(publication_purpose,''),'factory_supply'), list_type, COALESCE(product_type_category_id,0), COALESCE(product_type_name,''), COALESCE(classification_template_id,0), COALESCE(classification_template_name,''), COALESCE(classification_category_id,0), COALESCE(classification_category_name,''), version_no, status, owner_type, owner_key, COALESCE(price_source_publication_id,0), COALESCE(style_source_publication_id,0), source_version_no, config_json, content_json, changelog,
		          to_char(published_at,'YYYY-MM-DD HH24:MI'),
		          COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		          to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), cmd.PublicationPurpose, cmd.ListType, cmd.ProductTypeCategoryID, cmd.ProductTypeName, cmd.ClassificationTemplateID, cmd.ClassificationTemplateName, cmd.ClassificationCategoryID, cmd.ClassificationCategoryName, cmd.Version, cmd.OwnerType, cmd.OwnerKey, cmd.PriceSourcePublicationID, cmd.StyleSourcePublicationID, cmd.SourceVersion, config, content, cmd.Changelog, cmd.Actor).Scan(
		&draft.ID,
		&draft.PublicationPurpose,
		&draft.ListType,
		&draft.ProductTypeCategoryID,
		&draft.ProductTypeName,
		&draft.ClassificationTemplateID,
		&draft.ClassificationTemplateName,
		&draft.ClassificationCategoryID,
		&draft.ClassificationCategoryName,
		&draft.Version,
		&draft.Status,
		&draft.OwnerType,
		&draft.OwnerKey,
		&draft.PriceSourcePublicationID,
		&draft.StyleSourcePublicationID,
		&draft.SourceVersion,
		&configJSON,
		&contentJSON,
		&draft.Changelog,
		&draft.PublishedAt,
		&draft.WithdrawnAt,
		&draft.CreatedAt,
	); err != nil {
		return nil, err
	}
	draft.Config = map[string]any{}
	draft.Content = map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &draft.Config); err != nil {
			return nil, err
		}
	}
	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &draft.Content); err != nil {
			return nil, err
		}
	}
	if draft.ID <= 0 {
		return nil, fmt.Errorf("save draft failed")
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &draft.ID, "save_draft", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("draft"), postgresinfra.AuditMeta{
		"publication_purpose":      cmd.PublicationPurpose,
		"list_type":                cmd.ListType,
		"product_type_category_id": cmd.ProductTypeCategoryID,
		"product_type_name":        cmd.ProductTypeName,
		"version":                  cmd.Version,
		"owner_type":               cmd.OwnerType,
		"owner_key":                cmd.OwnerKey,
		"price_source_publication": cmd.PriceSourcePublicationID,
		"style_source_publication": cmd.StyleSourcePublicationID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &draft, nil
}

func (r Repository) WithdrawBeanList(ctx context.Context, cmd appcosting.WithdrawBeanListCommand) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var publicationPurpose, listType, version, ownerType, ownerKey string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.bean_list_publications
		SET status='withdrawn', withdrawn_at=now(), updated_at=now()
		WHERE id=$1 AND publication_purpose=$2 AND owner_type=$3 AND owner_key=$4 AND status='published'
		RETURNING COALESCE(NULLIF(publication_purpose,''),'factory_supply'), list_type, version_no, owner_type, owner_key
	`, r.schema), cmd.ID, cmd.PublicationPurpose, cmd.OwnerType, cmd.OwnerKey).Scan(&publicationPurpose, &listType, &version, &ownerType, &ownerKey); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("published bean list not found")
		}
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &cmd.ID, "withdraw", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("published"), postgresinfra.StrPtr("withdrawn"), postgresinfra.AuditMeta{
		"publication_purpose": publicationPurpose,
		"list_type":           listType,
		"version":             version,
		"owner_type":          ownerType,
		"owner_key":           ownerKey,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) CreateRun(ctx context.Context, actor string, items []domain.ProductResult) (*appcosting.Run, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.cost_calculation_runs(status, actor, product_count)
		VALUES('draft',$1,$2) RETURNING id`, r.schema), actor, len(items)).Scan(&id); err != nil {
		return nil, err
	}
	ins := fmt.Sprintf(`INSERT INTO %s.cost_calculation_items(run_id, product_id, product_name, result_json)
		VALUES($1,$2,$3,$4)`, r.schema)
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, ins, id, item.ProductID, item.Name, b); err != nil {
			return nil, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "costing_run", &id, "create", postgresinfra.StrPtr("product_count"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", len(items))), postgresinfra.AuditMeta{
		"run_id":        id,
		"product_count": len(items),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &appcosting.Run{ID: id, Status: "draft", ProductCount: len(items), Items: items}, nil
}

func (r Repository) PublishRun(ctx context.Context, actor string, runID int64) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	items, err := loadRunItems(ctx, tx, r.schema, runID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("costing run has no items")
	}

	updateProduct := fmt.Sprintf(`UPDATE %s.products
		SET default_price=$2,
		    retail_price_100g=$3,
		    retail_price_200g=$4,
		    retail_price_227g=$5,
		    retail_price_250g=$6
		WHERE id=$1`, r.schema)
	deleteTiers := fmt.Sprintf(`DELETE FROM %s.product_price_tiers WHERE product_id=$1`, r.schema)
	insertTier := fmt.Sprintf(`INSERT INTO %s.product_price_tiers
		(product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active, product_kind, price_basis, sales_unit, unit_bag_count, price_source_json)
		VALUES($1,$2,$3,$4,$5,$3,$4,$6,true,$7,'weight','',0,$8::jsonb)`, r.schema)
	insertDripTier := fmt.Sprintf(`INSERT INTO %s.product_price_tiers
		(product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active, product_kind, price_basis, sales_unit, unit_bag_count, price_source_json)
		VALUES($1,$2,$3,$4,$5,NULL,NULL,NULL,true,$6,'unit',$7,$8,$9::jsonb)`, r.schema)
	publishedProducts := 0
	for _, item := range items {
		if item.ProductID <= 0 {
			continue
		}
		if strings.TrimSpace(item.ProductKind) == "green_bean" {
			continue
		}
		defaultPrice := 0.0
		if len(item.CommercialWholesaleTiers) > 0 {
			defaultPrice = item.CommercialWholesaleTiers[0].PricePerUnit
		} else if len(item.WholesaleKgPrices) > 0 {
			defaultPrice = item.WholesaleKgPrices[0]
		}
		if _, err := tx.Exec(ctx, updateProduct, item.ProductID, defaultPrice, item.Retail100gPrice, item.Retail200gPrice, item.Retail227gPrice, item.Retail250gPrice); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, deleteTiers, item.ProductID); err != nil {
			return err
		}
		if item.ProductKind == "drip_bag" {
			for _, tier := range item.DripWholesaleTiers {
				bagGrams := tier.BagGrams
				if bagGrams <= 0 {
					bagGrams = item.DripBagGrams
				}
				if bagGrams <= 0 {
					bagGrams = 10
				}
				boxBagCount := tier.BoxBagCount
				if boxBagCount <= 0 {
					boxBagCount = item.DripBoxBagCount
				}
				if boxBagCount <= 0 {
					boxBagCount = 10
				}
				source := dripPriceSourceJSON(tier, bagGrams, boxBagCount)
				if _, err := tx.Exec(ctx, insertDripTier, item.ProductID, int64(math.Round(bagGrams)), tier.MinBags, tier.MaxBags, tier.PackedPricePerBag, item.ProductKind, "bag", 1, source); err != nil {
					return err
				}
				minBoxes := dripBoxMinQty(tier.MinBags, boxBagCount)
				maxBoxes := dripBoxMaxQty(tier.MaxBags, boxBagCount)
				boxSource := dripPriceSourceJSON(tier, bagGrams, boxBagCount)
				if _, err := tx.Exec(ctx, insertDripTier, item.ProductID, int64(math.Round(bagGrams))*int64(boxBagCount), minBoxes, maxBoxes, tier.PackedPricePerBag*float64(boxBagCount), item.ProductKind, "box", boxBagCount, boxSource); err != nil {
					return err
				}
			}
		} else {
			for _, tier := range commercialTiersForPublish(item) {
				specG := tier.SpecG
				if specG <= 0 {
					specG = 454
				}
				minQty := tier.MinQty
				if minQty <= 0 {
					minQty = tier.MinLb
				}
				maxQty := tier.MaxQty
				if maxQty == nil {
					maxQty = tier.MaxLb
				}
				pricePerUnit := tier.PricePerUnit
				if pricePerUnit == 0 {
					pricePerUnit = tier.PricePerLb
				}
				pricePerLb := pricePerUnit * 454.0 / float64(specG)
				source := commercialPriceSourceJSON(tier)
				if _, err := tx.Exec(ctx, insertTier, item.ProductID, specG, minQty, maxQty, pricePerUnit, pricePerLb, firstNonEmptyString(item.ProductKind, "roasted_bean"), source); err != nil {
					return err
				}
			}
		}
		publishedProducts++
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.cost_calculation_runs SET status='published', published_at=now() WHERE id=$1`, r.schema), runID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "costing_run", &runID, "publish", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("draft"), postgresinfra.StrPtr("published"), postgresinfra.AuditMeta{
		"run_id":             runID,
		"published_products": publishedProducts,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func dripPriceSourceJSON(tier domain.DripWholesaleTier, bagGrams float64, boxBagCount int) string {
	b, _ := json.Marshal(map[string]any{
		"template_id":          tier.TemplateID,
		"tier_id":              tier.TemplateTierID,
		"bag_grams":            bagGrams,
		"box_bag_count":        boxBagCount,
		"loose_price_per_bag":  tier.LoosePricePerBag,
		"packed_price_per_bag": tier.PackedPricePerBag,
		"multiplier":           tier.Multiplier,
		"tax_rate":             tier.TaxRate,
	})
	return string(b)
}

func commercialPriceSourceJSON(tier domain.CommercialWholesaleTier) string {
	b, _ := json.Marshal(map[string]any{
		"template_id":      tier.TemplateID,
		"template_tier_id": tier.TemplateTierID,
		"display_unit":     tier.DisplayUnit,
		"price_unit":       firstNonEmptyString(tier.PriceUnit, tier.DisplayUnit),
		"price_per_unit":   tier.PricePerUnit,
		"price_per_kg":     tier.PricePerKg,
		"price_per_lb":     tier.PricePerLb,
		"margin_rate":      tier.MarginRate,
	})
	return string(b)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func dripBoxMinQty(minBags int64, boxBagCount int) float64 {
	if boxBagCount <= 0 {
		boxBagCount = 10
	}
	return math.Ceil(float64(minBags) / float64(boxBagCount))
}

func dripBoxMaxQty(maxBags *float64, boxBagCount int) *float64 {
	if maxBags == nil {
		return nil
	}
	if boxBagCount <= 0 {
		boxBagCount = 10
	}
	v := math.Floor(*maxBags / float64(boxBagCount))
	return &v
}

func loadRunItems(ctx context.Context, tx pgx.Tx, schema string, runID int64) ([]domain.ProductResult, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT result_json FROM %s.cost_calculation_items WHERE run_id=$1 ORDER BY id`, schema), runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.ProductResult, 0)
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var item domain.ProductResult
		if err := json.Unmarshal(b, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type beanListPublicationScanner interface {
	Scan(dest ...any) error
}

func scanBeanListPublication(row beanListPublicationScanner) (appcosting.BeanListPublication, error) {
	var out appcosting.BeanListPublication
	var configJSON, contentJSON []byte
	if err := row.Scan(
		&out.ID,
		&out.PublicationPurpose,
		&out.ListType,
		&out.ProductTypeCategoryID,
		&out.ProductTypeName,
		&out.ClassificationTemplateID,
		&out.ClassificationTemplateName,
		&out.ClassificationCategoryID,
		&out.ClassificationCategoryName,
		&out.Version,
		&out.Status,
		&out.OwnerType,
		&out.OwnerKey,
		&out.PriceSourcePublicationID,
		&out.StyleSourcePublicationID,
		&out.SourceVersion,
		&configJSON,
		&contentJSON,
		&out.Changelog,
		&out.PublishedAt,
		&out.WithdrawnAt,
		&out.CreatedAt,
	); err != nil {
		return out, err
	}
	out.Config = map[string]any{}
	out.Content = map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &out.Config); err != nil {
			return out, err
		}
	}
	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &out.Content); err != nil {
			return out, err
		}
	}
	return out, nil
}

func applyParameter(params *domain.Parameters, key string, value float64) {
	switch strings.TrimSpace(key) {
	case "roast_yield_rate":
		params.RoastYieldRate = value
	case "kg_to_lb_factor":
		params.KgToLbFactor = value
	case "small_batch_production_cost_per_kg":
		params.SmallBatchProductionCostPerKg = value
	case "large_batch_production_cost_per_kg":
		params.LargeBatchProductionCostPerKg = value
	case "wholesale_package_cost_per_kg":
		params.WholesalePackageCostPerKg = value
	case "product_loss_per_kg":
		params.ProductLossPerKg = value
	case "retail_bean_margin_rate":
		params.RetailBeanMarginRate = value
	case "retail_tax_rate":
		params.RetailTaxRate = value
	case "retail_logistics_per_kg":
		params.RetailLogisticsPerKg = value
	case "retail_drip_logistics_per_10_bags":
		params.RetailDripLogisticsPer10Bags = value
	case "drip_green_ratio_kg_per_bag":
		params.DripGreenRatioKgPerBag = value
	case "drip_process_cost_per_bag":
		params.DripProcessCostPerBag = value
	case "drip_extra_cost_per_bag":
		params.DripExtraCostPerBag = value
	case "drip_packing_material_per_bag":
		params.DripPackingMaterialPerBag = value
	case "retail_drip_multiplier":
		params.RetailDripMultiplier = value
	case "wholesale_kg_margin_rate_1":
		params.WholesaleKgMarginRates[0] = value
	case "wholesale_kg_margin_rate_2":
		params.WholesaleKgMarginRates[1] = value
	case "wholesale_kg_margin_rate_3":
		params.WholesaleKgMarginRates[2] = value
	case "wholesale_kg_margin_rate_4":
		params.WholesaleKgMarginRates[3] = value
	case "wholesale_kg_margin_rate_5":
		params.WholesaleKgMarginRates[4] = value
	case "wholesale_kg_margin_rate_6":
		params.WholesaleKgMarginRates[5] = value
	case "wholesale_drip_multiplier_1":
		params.WholesaleDripMultipliers[0] = value
	case "wholesale_drip_multiplier_2":
		params.WholesaleDripMultipliers[1] = value
	case "wholesale_drip_multiplier_3":
		params.WholesaleDripMultipliers[2] = value
	case "wholesale_drip_multiplier_4":
		params.WholesaleDripMultipliers[3] = value
	}
}

func floatPtr(v float64) *float64 {
	return &v
}

func validateBeanListProductScope(ctx context.Context, tx pgx.Tx, schema string, cmd appcosting.PublishBeanListCommand) error {
	if strings.TrimSpace(cmd.PublicationPurpose) == appcosting.BeanListPublicationPurposeCustomerResale {
		return nil
	}
	ids := beanListContentProductIDs(cmd.Content)
	if len(ids) == 0 {
		return nil
	}
	ownerType := strings.TrimSpace(cmd.OwnerType)
	if ownerType != "official" && ownerType != "customer" {
		return nil
	}
	customerID := int64(0)
	if ownerType == "customer" {
		id, err := strconv.ParseInt(strings.TrimSpace(cmd.OwnerKey), 10, 64)
		if err != nil || id <= 0 {
			return fmt.Errorf("customer_id required")
		}
		customerID = id
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(customer_id,0)
		FROM %s.products
		WHERE active=true AND id = ANY($1)
	`, schema), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	seen := map[int64]bool{}
	for rows.Next() {
		var productID, productCustomerID int64
		if err := rows.Scan(&productID, &productCustomerID); err != nil {
			return err
		}
		seen[productID] = true
		if ownerType == "official" && productCustomerID > 0 {
			return fmt.Errorf("official bean list cannot include customer SKU")
		}
		if ownerType == "customer" && productCustomerID > 0 && productCustomerID != customerID {
			return fmt.Errorf("customer bean list cannot include another customer's SKU")
		}
		if ownerType == "customer" && productCustomerID <= 0 {
			return fmt.Errorf("customer bean list cannot include public SKU")
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if !seen[id] {
			return fmt.Errorf("bean list product not found")
		}
	}
	return nil
}

func beanListContentProductIDs(content map[string]any) []int64 {
	groups, ok := anySlice(content["groups"])
	if !ok {
		return nil
	}
	seen := map[int64]bool{}
	out := make([]int64, 0)
	for _, group := range groups {
		groupMap, ok := anyMap(group)
		if !ok {
			continue
		}
		items, ok := anySlice(groupMap["items"])
		if !ok {
			continue
		}
		for _, item := range items {
			itemMap, ok := anyMap(item)
			if !ok {
				continue
			}
			id := anyInt64(itemMap["productId"])
			if id <= 0 {
				id = anyInt64(itemMap["product_id"])
			}
			if id <= 0 {
				id = anyInt64(itemMap["productID"])
			}
			if id > 0 && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

func anySlice(value any) ([]any, bool) {
	switch v := value.(type) {
	case []any:
		return v, true
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out, true
	default:
		return nil, false
	}
}

func anyMap(value any) (map[string]any, bool) {
	v, ok := value.(map[string]any)
	return v, ok
}

func anyInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		i, _ := v.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return i
	default:
		return 0
	}
}

func commercialTiersForPublish(item domain.ProductResult) []domain.CommercialWholesaleTier {
	if len(item.CommercialWholesaleTiers) > 0 {
		return item.CommercialWholesaleTiers
	}
	return nil
}
