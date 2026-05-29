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
			       CASE
			         WHEN COALESCE(NULLIF(p.product_kind,''),'roasted')='green_bean'
			          AND COALESCE(p.green_bean_bom_product_id,0) > 0
			         THEN p.green_bean_bom_product_id
			         ELSE p.id
			       END AS bom_product_id
			FROM %[1]s.products p
			WHERE p.active = true
		),
		finished_product_cost AS (
			SELECT p.id AS product_id,
			       COALESCE(SUM(COALESCE(mv.weighted_unit_cost, m.purchase_price, 0) * COALESCE(bi.ratio_pct,0) / 100.0),0) AS green_cost_per_kg
			FROM %[1]s.products p
			LEFT JOIN %[1]s.product_bom_items bi ON bi.product_id = p.id
				AND COALESCE(NULLIF(bi.component_type,''),'material') = 'material'
				AND COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') = 'ratio_pct'
			LEFT JOIN %[1]s.materials m ON m.id = bi.material_id
			LEFT JOIN material_valuation mv ON mv.material_id = m.id
			WHERE p.active = true
			GROUP BY p.id
		),
		finished_component_cost AS (
			SELECT bi.product_id,
			       SUM(COALESCE(fpc.green_cost_per_kg,0) * COALESCE(NULLIF(bi.qty_per_unit,0), NULLIF(bi.component_spec_g,0), 1))
			       / NULLIF(SUM(COALESCE(NULLIF(bi.qty_per_unit,0), NULLIF(bi.component_spec_g,0), 1)),0) AS finished_green_cost_per_kg
			FROM %[1]s.product_bom_items bi
			JOIN finished_product_cost fpc ON fpc.product_id = bi.component_product_id
			WHERE COALESCE(NULLIF(bi.component_type,''),'material') = 'finished_product'
			GROUP BY bi.product_id
		)
		SELECT p.id,
		       p.name,
		       COALESCE(base_p.name, p.name),
		       COALESCE(p.roast_level, ''),
		       COALESCE(p.customer_id, 0),
		       COALESCE(p.base_product_id, 0),
		       COALESCE(NULLIF(p.visibility, ''), 'public'),
		       COALESCE(p.custom_type, ''),
		       COALESCE(NULLIF(p.product_kind,''), 'roasted'),
		       COALESCE(p.drip_bag_grams, 10)::float8,
		       COALESCE(p.drip_box_bag_count, 10),
		       COALESCE(p.product_category_id, 0),
		       COALESCE(pc.name, ''),
		       COALESCE(pc.position, 0),
		       COALESCE(p.product_category_position, 0),
		       COALESCE(pc.gradient_template_id, 0),
		       p.margin_rate_override::float8,
		       COALESCE(NULLIF(b.yield_rate,0), $1),
		       CASE
		           WHEN COALESCE(NULLIF(p.product_kind,''), 'roasted') = 'green_bean'
		           THEN COALESCE(SUM(COALESCE(NULLIF(bi.unit_cost_snapshot,0), m.purchase_price, 0) * COALESCE(bi.ratio_pct,0) / 100.0),0)
		           WHEN COALESCE(NULLIF(p.product_kind,''), 'roasted') = 'drip_bag' AND COALESCE(fcc.finished_green_cost_per_kg,0) > 0
		           THEN COALESCE(fcc.finished_green_cost_per_kg,0)
		           ELSE COALESCE(SUM(COALESCE(mv.weighted_unit_cost, m.purchase_price, 0) * COALESCE(bi.ratio_pct,0) / 100.0),0)
		       END,
		       COALESCE(string_agg(DISTINCT NULLIF(bp.flavor, ''), ' / ') FILTER (WHERE NULLIF(bp.flavor, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.origin, ''), ' / ') FILTER (WHERE NULLIF(bp.origin, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.processing_station, ''), ' / ') FILTER (WHERE NULLIF(bp.processing_station, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.variety, ''), ' / ') FILTER (WHERE NULLIF(bp.variety, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.process_method, ''), ' / ') FILTER (WHERE NULLIF(bp.process_method, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.grade, ''), ' / ') FILTER (WHERE NULLIF(bp.grade, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.altitude, ''), ' / ') FILTER (WHERE NULLIF(bp.altitude, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.bean_list_note, ''), ' / ') FILTER (WHERE NULLIF(bp.bean_list_note, '') IS NOT NULL), ''),
		       COALESCE(NULLIF(b.status,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'active' END),
		       COALESCE(qc.factory_flavor_description, ''),
		       COALESCE(qc.moisture, ''),
		       COALESCE(qc.density, ''),
		       COALESCE(qc.inspection_created_at, ''),
		       COALESCE(qc.inspection_reference_no, '')
		FROM product_scope p
		LEFT JOIN %[1]s.product_bom b ON b.product_id = bom_product_id
		LEFT JOIN %[1]s.product_bom_items bi ON bi.product_id = bom_product_id
		LEFT JOIN %[1]s.materials m ON m.id = bi.material_id
		LEFT JOIN material_valuation mv ON mv.material_id = m.id
		LEFT JOIN %[1]s.material_bean_profiles bp ON bp.material_id = m.id
		LEFT JOIN %[1]s.products base_p ON base_p.id = p.base_product_id
		LEFT JOIN %[1]s.product_categories pc ON pc.id = p.product_category_id AND pc.active=true
		LEFT JOIN finished_component_cost fcc ON fcc.product_id = p.id
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
		WHERE p.active = true
		GROUP BY p.id, p.name, base_p.name, p.roast_level, p.customer_id, p.base_product_id, p.visibility, p.custom_type, p.product_kind, p.drip_bag_grams, p.drip_box_bag_count, p.product_category_id, pc.name, pc.position, p.product_category_position, pc.gradient_template_id, p.margin_rate_override, p.bom_product_id, b.yield_rate, b.status, b.product_id, fcc.finished_green_cost_per_kg, qc.factory_flavor_description, qc.moisture, qc.density, qc.inspection_created_at, qc.inspection_reference_no
		ORDER BY p.name
	`, r.schema)
	rows, err := r.pool.Query(ctx, q, params.RoastYieldRate)
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
		if err := rows.Scan(&input.ProductID, &input.Name, &input.BeanListTemplateName, &roastLevel, &input.CustomerID, &input.BaseProductID, &input.Visibility, &input.CustomType, &input.ProductKind, &input.DripBagGrams, &input.DripBoxBagCount, &input.ProductCategoryID, &input.SkuCategoryName, &input.SkuCategoryPosition, &input.SkuProductCategoryPosition, &gradientTemplateID, &input.MarginRateOverride, &fallbackYield, &input.GreenBeanCostPerKg, &input.Flavor, &input.Origin, &input.ProcessingStation, &input.Variety, &input.ProcessMethod, &input.Grade, &input.Altitude, &input.BeanListNote, &input.BomStatus, &input.BeanListQuality.FactoryFlavorDescription, &input.BeanListQuality.Moisture, &input.BeanListQuality.Density, &input.BeanListQuality.InspectionCreatedAt, &input.BeanListQuality.InspectionReferenceNo); err != nil {
			return nil, err
		}
		if gradientTemplateID > 0 {
			templateIDs[gradientTemplateID] = true
			templateIDByProduct[input.ProductID] = gradientTemplateID
		}
		_ = roastLevel
		_ = fallbackYield
		input.YieldRate = params.RoastYieldRate
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
	dripTemplate, err := r.loadDefaultDripPriceTemplate(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if templateID := templateIDByProduct[out[i].ProductID]; templateID > 0 {
			if template := templates[templateID]; template != nil {
				out[i].GradientTemplate = template
			}
		}
		if out[i].ProductKind == "drip_bag" && dripTemplate != nil {
			out[i].DripPriceTemplate = dripTemplate
		}
	}
	return out, nil
}

func (r Repository) loadDefaultDripPriceTemplate(ctx context.Context) (*domain.DripPriceTemplate, error) {
	rows, err := r.ListDripPriceTemplates(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].Active {
			return &rows[i], nil
		}
	}
	return nil, nil
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
	whereClause := "WHERE list_type=$1 AND owner_type=$2 AND owner_key=$3"
	args := []any{strings.TrimSpace(query.ListType), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,
		       list_type,
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
	row, err := scanBeanListPublication(r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,
		       list_type,
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
		WHERE list_type=$1 AND owner_type=$2 AND owner_key=$3 AND status='published'
		ORDER BY published_at DESC, id DESC
		LIMIT 1
	`, r.schema), strings.TrimSpace(query.ListType), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
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
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bean_list_publications
		SET status='withdrawn', withdrawn_at=now(), updated_at=now()
		WHERE list_type=$1 AND owner_type=$2 AND owner_key=$3 AND status='published'
	`, r.schema), cmd.ListType, cmd.OwnerType, cmd.OwnerKey); err != nil {
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
		INSERT INTO %s.bean_list_publications(list_type, version_no, status, owner_type, owner_key, price_source_publication_id, style_source_publication_id, source_version_no, config_json, content_json, changelog, actor)
		VALUES($1,$2,'published',$3,$4,NULLIF($5,0),NULLIF($6,0),$7,$8::jsonb,$9::jsonb,$10,$11)
		RETURNING id, list_type, version_no, status, owner_type, owner_key, COALESCE(price_source_publication_id,0), COALESCE(style_source_publication_id,0), source_version_no, config_json, content_json, changelog,
		          to_char(published_at,'YYYY-MM-DD HH24:MI'),
		          COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		          to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), cmd.ListType, cmd.Version, cmd.OwnerType, cmd.OwnerKey, cmd.PriceSourcePublicationID, cmd.StyleSourcePublicationID, cmd.SourceVersion, config, content, cmd.Changelog, cmd.Actor).Scan(
		&published.ID,
		&published.ListType,
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
		"list_type":                cmd.ListType,
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
		INSERT INTO %s.bean_list_publications(list_type, version_no, status, owner_type, owner_key, price_source_publication_id, style_source_publication_id, source_version_no, config_json, content_json, changelog, actor)
		VALUES($1,$2,'draft',$3,$4,NULLIF($5,0),NULLIF($6,0),$7,$8::jsonb,$9::jsonb,$10,$11)
		RETURNING id, list_type, version_no, status, owner_type, owner_key, COALESCE(price_source_publication_id,0), COALESCE(style_source_publication_id,0), source_version_no, config_json, content_json, changelog,
		          to_char(published_at,'YYYY-MM-DD HH24:MI'),
		          COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		          to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), cmd.ListType, cmd.Version, cmd.OwnerType, cmd.OwnerKey, cmd.PriceSourcePublicationID, cmd.StyleSourcePublicationID, cmd.SourceVersion, config, content, cmd.Changelog, cmd.Actor).Scan(
		&draft.ID,
		&draft.ListType,
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
		"list_type":                cmd.ListType,
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

	var listType, version, ownerType, ownerKey string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.bean_list_publications
		SET status='withdrawn', withdrawn_at=now(), updated_at=now()
		WHERE id=$1 AND owner_type=$2 AND owner_key=$3 AND status='published'
		RETURNING list_type, version_no, owner_type, owner_key
	`, r.schema), cmd.ID, cmd.OwnerType, cmd.OwnerKey).Scan(&listType, &version, &ownerType, &ownerKey); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("published bean list not found")
		}
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &cmd.ID, "withdraw", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("published"), postgresinfra.StrPtr("withdrawn"), postgresinfra.AuditMeta{
		"list_type":  listType,
		"version":    version,
		"owner_type": ownerType,
		"owner_key":  ownerKey,
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
		VALUES($1,$2,$3,$4,$5,$3,$4,$6,true,'roasted_bean','weight','',0,'{}'::jsonb)`, r.schema)
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
				if _, err := tx.Exec(ctx, insertTier, item.ProductID, specG, minQty, maxQty, pricePerUnit, pricePerLb); err != nil {
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
		&out.ListType,
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
	ranges := []struct {
		label string
		min   float64
		max   *float64
	}{
		{"2包-13包", 2, floatPtr(13)},
		{"14包-23包", 14, floatPtr(23)},
		{"24包-47包", 24, floatPtr(47)},
		{"48包+", 48, nil},
	}
	out := make([]domain.CommercialWholesaleTier, 0, len(ranges))
	for i, r := range ranges {
		if i >= len(item.WholesaleKgPrices) || i >= len(item.WholesaleLbPrices) {
			break
		}
		out = append(out, domain.CommercialWholesaleTier{
			Label:        r.label,
			Scheme:       domain.WholesaleTierScheme454GFour,
			SpecG:        454,
			MinQty:       r.min,
			MaxQty:       r.max,
			PricePerUnit: item.WholesaleLbPrices[i],
			MinLb:        r.min,
			MaxLb:        r.max,
			PricePerKg:   item.WholesaleKgPrices[i],
			PricePerLb:   item.WholesaleLbPrices[i],
		})
	}
	return out
}
