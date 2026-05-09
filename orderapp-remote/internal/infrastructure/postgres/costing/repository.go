package costing

import (
	"context"
	"encoding/json"
	"fmt"
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
		SELECT p.id,
		       p.name,
		       COALESCE(base_p.name, p.name),
		       COALESCE(p.roast_level, ''),
		       COALESCE(p.customer_id, 0),
		       COALESCE(p.base_product_id, 0),
		       COALESCE(NULLIF(p.visibility, ''), 'public'),
		       COALESCE(p.custom_type, ''),
		       COALESCE(NULLIF(b.yield_rate,0), $1),
		       COALESCE(SUM(COALESCE(m.purchase_price,0) * COALESCE(bi.ratio_pct,0) / 100.0),0),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.flavor, ''), ' / ') FILTER (WHERE NULLIF(bp.flavor, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.origin, ''), ' / ') FILTER (WHERE NULLIF(bp.origin, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.processing_station, ''), ' / ') FILTER (WHERE NULLIF(bp.processing_station, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.variety, ''), ' / ') FILTER (WHERE NULLIF(bp.variety, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.process_method, ''), ' / ') FILTER (WHERE NULLIF(bp.process_method, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.grade, ''), ' / ') FILTER (WHERE NULLIF(bp.grade, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.altitude, ''), ' / ') FILTER (WHERE NULLIF(bp.altitude, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.bean_list_note, ''), ' / ') FILTER (WHERE NULLIF(bp.bean_list_note, '') IS NOT NULL), ''),
		       COALESCE(NULLIF(b.status,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'active' END)
		FROM %s.products p
		LEFT JOIN %s.product_bom b ON b.product_id = p.id
		LEFT JOIN %s.product_bom_items bi ON bi.product_id = p.id
		LEFT JOIN %s.materials m ON m.id = bi.material_id
		LEFT JOIN %s.material_bean_profiles bp ON bp.material_id = m.id
		LEFT JOIN %s.products base_p ON base_p.id = p.base_product_id
		WHERE p.active = true
		GROUP BY p.id, p.name, base_p.name, p.roast_level, p.customer_id, p.base_product_id, p.visibility, p.custom_type, b.yield_rate, b.status, b.product_id
		ORDER BY p.name
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q, params.RoastYieldRate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.ProductInput, 0)
	for rows.Next() {
		var input domain.ProductInput
		var roastLevel string
		var fallbackYield float64
		if err := rows.Scan(&input.ProductID, &input.Name, &input.BeanListTemplateName, &roastLevel, &input.CustomerID, &input.BaseProductID, &input.Visibility, &input.CustomType, &fallbackYield, &input.GreenBeanCostPerKg, &input.Flavor, &input.Origin, &input.ProcessingStation, &input.Variety, &input.ProcessMethod, &input.Grade, &input.Altitude, &input.BeanListNote, &input.BomStatus); err != nil {
			return nil, err
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
	return out, rows.Err()
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

func (r Repository) ListBeanListPublications(ctx context.Context, query appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error) {
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
		WHERE list_type=$1 AND owner_type=$2 AND owner_key=$3
		ORDER BY CASE WHEN status='published' THEN 0 ELSE 1 END, created_at DESC, id DESC
	`, r.schema), strings.TrimSpace(query.ListType), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey))
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
		(product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active)
		VALUES($1,$2,$3,$4,$5,$3,$4,$6,true)`, r.schema)
	publishedProducts := 0
	for _, item := range items {
		if item.ProductID <= 0 {
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
