package catalog

import (
	"context"
	"fmt"
	"strings"

	catalogapp "orderapp/internal/application/catalog"
	catalogdomain "orderapp/internal/domain/catalog"
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

func (r Repository) ListProducts(ctx context.Context) ([]catalogapp.Product, error) {
	ps, err := postgresinfra.FetchProducts(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	return catalogProductsFromOptions(ps), nil
}

func (r Repository) GetProduct(ctx context.Context, id int64) (*catalogapp.Product, error) {
	p, err := fetchProductByID(ctx, r.pool, r.schema, id)
	if err != nil || p == nil {
		return nil, err
	}
	out := catalogProductFromOption(*p)
	return &out, nil
}

func (r Repository) ReplacePriceTiers(ctx context.Context, cmd catalogapp.ReplacePriceTiersCommand) error {
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

	productKind := catalogdomain.NormalizeProductKind(cmd.ProductKind)
	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	greenBeanType := strings.TrimSpace(cmd.GreenBeanType)
	greenBeanBomProductID := cmd.GreenBeanBomProductID
	if productKind == catalogdomain.ProductKindGreenBean {
		roastLevel = ""
		if greenBeanType == "" {
			greenBeanType = "single_origin"
		}
	} else {
		greenBeanType = ""
		greenBeanBomProductID = 0
	}
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET product_kind=$2, roast_level=$3, default_price=$4,
		    retail_price_100g=$5, retail_price_200g=$6, retail_price_227g=$7, retail_price_250g=$8,
		    green_bean_type=$9, green_bean_bom_product_id=$10
		WHERE id=$1`, r.schema), cmd.ProductID, productKind, roastLevel, cmd.DefaultPrice, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G, greenBeanType, greenBeanBomProductID); err != nil {
		return err
	}
	if productKind == catalogdomain.ProductKindRoasted {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_bom(product_id,yield_rate,updated_at)
			VALUES($1,$2,now())
			ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()`, r.schema), cmd.ProductID, yieldRate); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.product_price_tiers WHERE product_id=$1", r.schema), cmd.ProductID); err != nil {
		return err
	}
	ins := fmt.Sprintf(`INSERT INTO %s.product_price_tiers
		(product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,true)`, r.schema)
	for _, tier := range cmd.Tiers {
		specG := tier.SpecG
		if specG <= 0 {
			specG = 454
		}
		minLb := tier.MinQty * float64(specG) / 454.0
		var maxLb *float64
		if tier.MaxQty != nil {
			v := *tier.MaxQty * float64(specG) / 454.0
			maxLb = &v
		}
		priceLb := tier.UnitPrice * 454.0 / float64(specG)
		if _, err := tx.Exec(ctx, ins, cmd.ProductID, specG, tier.MinQty, tier.MaxQty, tier.UnitPrice, minLb, maxLb, priceLb); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product", &cmd.ProductID, "update", postgresinfra.StrPtr("price_tiers"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", len(cmd.Tiers))), postgresinfra.AuditMeta{
		"product_id":        cmd.ProductID,
		"product_kind":      productKind,
		"roast_level":       roastLevel,
		"yield_rate":        yieldRate,
		"tier_count":        len(cmd.Tiers),
		"retail_price_100g": cmd.RetailPrice100G,
		"retail_price_200g": cmd.RetailPrice200G,
		"retail_price_227g": cmd.RetailPrice227G,
		"retail_price_250g": cmd.RetailPrice250G,
		"green_bean_type":   greenBeanType,
		"green_bean_bom":    greenBeanBomProductID,
	})
	return nil
}

func (r Repository) UpdateProductBasics(ctx context.Context, cmd catalogapp.UpdateProductBasicsCommand) error {
	productKind := catalogdomain.NormalizeProductKind(cmd.ProductKind)
	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	greenBeanType := strings.TrimSpace(cmd.GreenBeanType)
	greenBeanBomProductID := cmd.GreenBeanBomProductID
	if productKind == catalogdomain.ProductKindGreenBean {
		roastLevel = ""
		if greenBeanType == "" {
			greenBeanType = "single_origin"
		}
	} else {
		greenBeanType = ""
		greenBeanBomProductID = 0
	}
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	if cmd.YieldRate > 0 {
		yieldRate = cmd.YieldRate
	}
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
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET roast_level=$2, retail_price_100g=$3, retail_price_200g=$4, retail_price_227g=$5, retail_price_250g=$6,
		    product_kind=$7, drip_bag_grams=$8, margin_rate_override=$9, drip_box_bag_count=$10, allow_fulfillment_order=$11, allow_mall_order=$12,
		    green_bean_type=$13, green_bean_bom_product_id=$14, remark=$15
		WHERE id=$1`, r.schema), cmd.ProductID, roastLevel, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G, productKind, cmd.DripBagGrams, cmd.MarginRateOverride, cmd.DripBoxBagCount, cmd.AllowFulfillmentOrder, cmd.AllowMallOrder, greenBeanType, greenBeanBomProductID, cmd.Remark); err != nil {
		return err
	}
	if productKind == catalogdomain.ProductKindRoasted {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_bom(product_id,yield_rate,updated_at)
			VALUES($1,$2,now())
			ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()`, r.schema), cmd.ProductID, yieldRate); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	meta := postgresinfra.AuditMeta{
		"product_id":           cmd.ProductID,
		"product_kind":         productKind,
		"roast_level":          roastLevel,
		"yield_rate":           yieldRate,
		"retail_price_100g":    cmd.RetailPrice100G,
		"retail_price_200g":    cmd.RetailPrice200G,
		"retail_price_227g":    cmd.RetailPrice227G,
		"retail_price_250g":    cmd.RetailPrice250G,
		"margin_rate_override": cmd.MarginRateOverride,
		"remark":               cmd.Remark,
	}
	meta["product_kind"] = cmd.ProductKind
	meta["drip_bag_grams"] = cmd.DripBagGrams
	meta["drip_box_bag_count"] = cmd.DripBoxBagCount
	meta["allow_fulfillment_order"] = cmd.AllowFulfillmentOrder
	meta["allow_mall_order"] = cmd.AllowMallOrder
	meta["sales_units"] = cmd.SalesUnits
	meta["green_bean_type"] = greenBeanType
	meta["green_bean_bom"] = greenBeanBomProductID
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product", &cmd.ProductID, "update", postgresinfra.StrPtr("product_basics"), nil, postgresinfra.StrPtr(roastLevel), meta)
	return nil
}

func (r Repository) DeactivateProducts(ctx context.Context, cmd catalogapp.DeactivateProductsCommand) error {
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

	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET active=false
		WHERE id = ANY($1) AND active=true`, r.schema), cmd.ProductIDs); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_bom SET status='inactive', updated_at=now()
		WHERE product_id = ANY($1)`, r.schema), cmd.ProductIDs); err != nil {
		return err
	}
	for _, productID := range cmd.ProductIDs {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &productID, "deactivate", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"product_id": productID, "bom_status": "inactive"}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r Repository) CreateProduct(ctx context.Context, cmd catalogapp.CreateProductCommand) (catalogapp.Product, error) {
	productKind := catalogdomain.NormalizeProductKind(cmd.ProductKind)
	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	greenBeanType := strings.TrimSpace(cmd.GreenBeanType)
	greenBeanBomProductID := cmd.GreenBeanBomProductID
	if productKind == catalogdomain.ProductKindGreenBean {
		roastLevel = ""
		if greenBeanType == "" {
			greenBeanType = "single_origin"
		}
	} else {
		greenBeanType = ""
		greenBeanBomProductID = 0
	}
	yieldRate := cmd.YieldRate
	if productKind == catalogdomain.ProductKindRoasted && yieldRate <= 0 {
		yieldRate = catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	}
	name := strings.TrimSpace(cmd.Name)

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.Product{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.Product{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(
			name, remark, product_kind, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id, created_at
		)
		VALUES($1,$2,$3,$4,$5,true,$6,$7,$8,$9,$10,$11,$12,$13,0,0,'public','',$14,$15,now())
		RETURNING id
	`, r.schema), name, cmd.Remark, productKind, roastLevel, cmd.DefaultPrice, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G, cmd.DripBagGrams, cmd.DripBoxBagCount, cmd.AllowFulfillmentOrder, cmd.AllowMallOrder, greenBeanType, greenBeanBomProductID).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}

	if productKind == catalogdomain.ProductKindRoasted {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_bom(product_id,yield_rate,status,updated_at)
			VALUES($1,$2,'active',now())
			ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()
		`, r.schema), productID, yieldRate); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if len(cmd.Tiers) > 0 {
		if err := replaceProductPriceTiersTx(ctx, tx, r.schema, productID, cmd.Tiers); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &productID, "create", postgresinfra.StrPtr("public_product"), nil, postgresinfra.StrPtr(name), postgresinfra.AuditMeta{
		"product_id":              productID,
		"roast_level":             roastLevel,
		"default_price":           cmd.DefaultPrice,
		"yield_rate":              yieldRate,
		"retail_price_100g":       cmd.RetailPrice100G,
		"retail_price_200g":       cmd.RetailPrice200G,
		"retail_price_227g":       cmd.RetailPrice227G,
		"retail_price_250g":       cmd.RetailPrice250G,
		"remark":                  cmd.Remark,
		"product_kind":            productKind,
		"drip_bag_grams":          cmd.DripBagGrams,
		"drip_box_bag_count":      cmd.DripBoxBagCount,
		"allow_fulfillment_order": cmd.AllowFulfillmentOrder,
		"allow_mall_order":        cmd.AllowMallOrder,
		"sales_units":             cmd.SalesUnits,
		"green_bean_type":         greenBeanType,
		"green_bean_bom":          greenBeanBomProductID,
	}); err != nil {
		return catalogapp.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.Product{}, err
	}
	product, err := r.GetProduct(ctx, productID)
	if err != nil {
		return catalogapp.Product{}, err
	}
	if product == nil {
		return catalogapp.Product{}, fmt.Errorf("created product not found")
	}
	return *product, nil
}

func replaceProductPriceTiersTx(ctx context.Context, tx pgx.Tx, schema string, productID int64, tiers []catalogapp.PriceTier) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.product_price_tiers WHERE product_id=$1", schema), productID); err != nil {
		return err
	}
	ins := fmt.Sprintf(`INSERT INTO %s.product_price_tiers
		(product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,true)`, schema)
	for _, tier := range tiers {
		specG := tier.SpecG
		if specG <= 0 {
			specG = 454
		}
		minLb := tier.MinQty * float64(specG) / 454.0
		var maxLb *float64
		if tier.MaxQty != nil {
			v := *tier.MaxQty * float64(specG) / 454.0
			maxLb = &v
		}
		priceLb := 0.0
		if tier.UnitPrice > 0 {
			priceLb = tier.UnitPrice * 454.0 / float64(specG)
		}
		if _, err := tx.Exec(ctx, ins, productID, specG, tier.MinQty, tier.MaxQty, tier.UnitPrice, minLb, maxLb, priceLb); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) ListProductCategories(ctx context.Context) ([]catalogapp.ProductCategory, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id, COALESCE(parent_id,0), COALESCE(customer_id,0), name, level, position, COALESCE(gradient_template_id,0)
		FROM %s.product_categories
		WHERE active=true
		ORDER BY COALESCE(customer_id,0), COALESCE(parent_id,0), position, id`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductCategory, 0)
	for rows.Next() {
		var row catalogapp.ProductCategory
		if err := rows.Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListGradientTemplates(ctx context.Context) ([]catalogapp.GradientTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, display_unit, active
		FROM %s.pricing_gradient_templates
		ORDER BY active DESC, name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.GradientTemplate, 0)
	for rows.Next() {
		var row catalogapp.GradientTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.DisplayUnit, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	tierRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, label, min_weight_g::float8, max_weight_g::float8, margin_rate::float8, position
		FROM %s.pricing_gradient_template_tiers
		WHERE active=true
		ORDER BY template_id, position, min_weight_g, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer tierRows.Close()
	tiersByTemplate := map[int64][]catalogapp.GradientTemplateTier{}
	for tierRows.Next() {
		var tier catalogapp.GradientTemplateTier
		if err := tierRows.Scan(&tier.ID, &tier.TemplateID, &tier.Label, &tier.MinWeightG, &tier.MaxWeightG, &tier.MarginRate, &tier.Position); err != nil {
			return nil, err
		}
		tiersByTemplate[tier.TemplateID] = append(tiersByTemplate[tier.TemplateID], tier)
	}
	if err := tierRows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Tiers = tiersByTemplate[out[i].ID]
		if out[i].Tiers == nil {
			out[i].Tiers = make([]catalogapp.GradientTemplateTier, 0)
		}
	}
	return out, nil
}

func (r Repository) SaveGradientTemplate(ctx context.Context, cmd catalogapp.SaveGradientTemplateCommand) (catalogapp.GradientTemplate, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.pricing_gradient_templates
			SET name=$2, display_unit=$3, active=true, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.DisplayUnit).Scan(&id); err != nil {
			return catalogapp.GradientTemplate{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.pricing_gradient_template_tiers SET active=false, updated_at=now() WHERE template_id=$1`, r.schema), id); err != nil {
			return catalogapp.GradientTemplate{}, err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.pricing_gradient_templates(name, display_unit, active)
			VALUES($1,$2,true)
			RETURNING id
		`, r.schema), cmd.Name, cmd.DisplayUnit).Scan(&id); err != nil {
			return catalogapp.GradientTemplate{}, err
		}
	}
	insertTier := fmt.Sprintf(`
		INSERT INTO %s.pricing_gradient_template_tiers(template_id,label,min_weight_g,max_weight_g,margin_rate,position,active)
		VALUES($1,$2,$3,$4,$5,$6,true)
		RETURNING id
	`, r.schema)
	for i := range cmd.Tiers {
		cmd.Tiers[i].TemplateID = id
		if err := tx.QueryRow(ctx, insertTier, id, cmd.Tiers[i].Label, cmd.Tiers[i].MinWeightG, cmd.Tiers[i].MaxWeightG, cmd.Tiers[i].MarginRate, cmd.Tiers[i].Position).Scan(&cmd.Tiers[i].ID); err != nil {
			return catalogapp.GradientTemplate{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "pricing_gradient_template", &id, "update", postgresinfra.StrPtr("template"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{
		"display_unit": cmd.DisplayUnit,
		"tier_count":   len(cmd.Tiers),
	}); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	return catalogapp.GradientTemplate{ID: id, Name: cmd.Name, DisplayUnit: cmd.DisplayUnit, Active: true, Tiers: cmd.Tiers}, nil
}

func (r Repository) DeactivateGradientTemplate(ctx context.Context, cmd catalogapp.DeactivateGradientTemplateCommand) error {
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.pricing_gradient_templates SET active=false, updated_at=now() WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "pricing_gradient_template", &cmd.ID, "deactivate", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), nil)
	return nil
}

func (r Repository) BindCategoryGradientTemplate(ctx context.Context, cmd catalogapp.BindCategoryGradientTemplateCommand) error {
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
	if cmd.GradientTemplateID > 0 {
		var exists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.pricing_gradient_templates WHERE id=$1 AND active=true)`, r.schema), cmd.GradientTemplateID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("gradient template not found")
		}
	}
	var oldID int64
	var level int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(gradient_template_id,0), level FROM %s.product_categories WHERE id=$1 AND active=true`, r.schema), cmd.CategoryID).Scan(&oldID, &level); err != nil {
		return err
	}
	if level != 2 {
		return fmt.Errorf("only secondary category can bind gradient template")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_categories SET gradient_template_id=$2, updated_at=now() WHERE id=$1`, r.schema), cmd.CategoryID, cmd.GradientTemplateID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_category", &cmd.CategoryID, "update", postgresinfra.StrPtr("gradient_template_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", oldID)), postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.GradientTemplateID)), nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) SaveProductCategory(ctx context.Context, cmd catalogapp.SaveProductCategoryCommand) (catalogapp.ProductCategory, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.ProductCategory{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.ProductCategory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	level := 1
	parentID := cmd.ParentID
	if parentID > 0 {
		level = 2
	}
	customerID := cmd.CustomerID
	if customerID < 0 {
		customerID = 0
	}
	if parentID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0)
			FROM %s.product_categories
			WHERE id=$1 AND active=true`, r.schema), parentID).Scan(&customerID); err != nil {
			return catalogapp.ProductCategory{}, err
		}
	}
	position := cmd.Position
	if position <= 0 {
		position = 9999
	}
	var row catalogapp.ProductCategory
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.product_categories
			SET parent_id=NULLIF($2,0), name=$3, level=$4, position=$5, customer_id=$6, updated_at=now()
			WHERE id=$1 AND active=true
			RETURNING id, COALESCE(parent_id,0), COALESCE(customer_id,0), name, level, position`, r.schema), cmd.ID, parentID, cmd.Name, level, position, customerID).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.Name, &row.Level, &row.Position); err != nil {
			return catalogapp.ProductCategory{}, err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.product_categories(parent_id,customer_id,name,level,position)
			VALUES(NULLIF($1,0),$2,$3,$4,$5)
			RETURNING id, COALESCE(parent_id,0), COALESCE(customer_id,0), name, level, position`, r.schema), parentID, customerID, cmd.Name, level, position).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.Name, &row.Level, &row.Position); err != nil {
			return catalogapp.ProductCategory{}, err
		}
	}
	if err := normalizeCategoryPositions(ctx, tx, r.schema, parentID, customerID); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	if err := placeCategoryPosition(ctx, tx, r.schema, parentID, customerID, row.ID, position); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_category", &row.ID, "update", postgresinfra.StrPtr("category"), nil, postgresinfra.StrPtr(row.Name), postgresinfra.AuditMeta{"parent_id": parentID, "customer_id": customerID, "position": position})
	return row, nil
}

func (r Repository) MoveProductCategory(ctx context.Context, cmd catalogapp.MoveProductCategoryCommand) error {
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
	var oldParent int64
	var customerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(parent_id,0), COALESCE(customer_id,0) FROM %s.product_categories WHERE id=$1`, r.schema), cmd.ID).Scan(&oldParent, &customerID); err != nil {
		return err
	}
	oldCustomerID := customerID
	if cmd.ParentID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0)
			FROM %s.product_categories
			WHERE id=$1 AND active=true`, r.schema), cmd.ParentID).Scan(&customerID); err != nil {
			return err
		}
	}
	level := 1
	if cmd.ParentID > 0 {
		level = 2
	}
	position := cmd.Position
	if position <= 0 {
		position = 9999
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_categories
		SET parent_id=NULLIF($2,0), level=$3, position=$4, customer_id=$5, updated_at=now()
		WHERE id=$1 AND active=true`, r.schema), cmd.ID, cmd.ParentID, level, position, customerID); err != nil {
		return err
	}
	if oldParent != cmd.ParentID || oldCustomerID != customerID {
		if err := normalizeCategoryPositions(ctx, tx, r.schema, oldParent, oldCustomerID); err != nil {
			return err
		}
	}
	if err := placeCategoryPosition(ctx, tx, r.schema, cmd.ParentID, customerID, cmd.ID, position); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_category", &cmd.ID, "move", postgresinfra.StrPtr("parent_position"), postgresinfra.StrPtr(fmt.Sprintf("%d", oldParent)), postgresinfra.StrPtr(fmt.Sprintf("%d:%d", cmd.ParentID, position)), postgresinfra.AuditMeta{"customer_id": customerID})
	return nil
}

func (r Repository) DeleteProductCategory(ctx context.Context, cmd catalogapp.DeleteProductCategoryCommand) error {
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

	var parentID int64
	var customerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(parent_id,0), COALESCE(customer_id,0)
		FROM %s.product_categories
		WHERE id=$1 AND active=true`, r.schema), cmd.ID).Scan(&parentID, &customerID); err != nil {
		return err
	}

	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id
		FROM %s.product_categories
		WHERE active=true AND (id=$1 OR COALESCE(parent_id,0)=$1)
		ORDER BY id`, r.schema), cmd.ID)
	if err != nil {
		return err
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(ids) == 0 {
		return pgx.ErrNoRows
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET product_category_id=NULL, product_category_position=0
		WHERE product_category_id = ANY($1)`, r.schema), ids); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_categories
		SET active=false, updated_at=now()
		WHERE id = ANY($1)`, r.schema), ids); err != nil {
		return err
	}
	if parentID == 0 {
		if err := normalizeCategoryPositions(ctx, tx, r.schema, 0, customerID); err != nil {
			return err
		}
	} else if err := normalizeCategoryPositions(ctx, tx, r.schema, parentID, customerID); err != nil {
		return err
	}
	if err := normalizeProductPositions(ctx, tx, r.schema, 0, customerID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_category", &cmd.ID, "delete", postgresinfra.StrPtr("category"), postgresinfra.StrPtr(fmt.Sprintf("%d", parentID)), nil, postgresinfra.AuditMeta{"customer_id": customerID, "deleted_category_ids": ids})
	return nil
}

func (r Repository) AssignProductCategory(ctx context.Context, cmd catalogapp.AssignProductCategoryCommand) error {
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
	var oldCategory int64
	var productCustomerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(product_category_id,0), COALESCE(customer_id,0) FROM %s.products WHERE id=$1`, r.schema), cmd.ProductID).Scan(&oldCategory, &productCustomerID); err != nil {
		return err
	}
	if cmd.CategoryID > 0 {
		var categoryCustomerID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0)
			FROM %s.product_categories
			WHERE id=$1 AND active=true`, r.schema), cmd.CategoryID).Scan(&categoryCustomerID); err != nil {
			return err
		}
		if categoryCustomerID != productCustomerID {
			return fmt.Errorf("category customer mismatch")
		}
	}
	position := cmd.Position
	if position <= 0 {
		position = 9999
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET product_category_id=NULLIF($2,0), product_category_position=$3
		WHERE id=$1`, r.schema), cmd.ProductID, cmd.CategoryID, position); err != nil {
		return err
	}
	if err := normalizeProductPositions(ctx, tx, r.schema, oldCategory, productCustomerID); err != nil {
		return err
	}
	if oldCategory != cmd.CategoryID {
		if err := normalizeProductPositions(ctx, tx, r.schema, cmd.CategoryID, productCustomerID); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product", &cmd.ProductID, "move", postgresinfra.StrPtr("product_category"), postgresinfra.StrPtr(fmt.Sprintf("%d", oldCategory)), postgresinfra.StrPtr(fmt.Sprintf("%d:%d", cmd.CategoryID, position)), nil)
	return nil
}

func (r Repository) CreateCustomProduct(ctx context.Context, cmd catalogapp.CreateCustomProductCommand) (catalogapp.Product, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.Product{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.Product{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerExists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, r.schema), cmd.CustomerID).Scan(&customerExists); err != nil {
		return catalogapp.Product{}, err
	}
	if !customerExists {
		return catalogapp.Product{}, fmt.Errorf("customer not found")
	}

	var base struct {
		Name                  string
		DefaultPrice          float64
		RetailPrice100G       float64
		RetailPrice200G       float64
		RetailPrice227G       float64
		RetailPrice250G       float64
		ProductKind           string
		GreenBeanType         string
		GreenBeanBomProductID int64
		DripBagGrams          float64
		DripBoxBagCount       int
		AllowFulfillmentOrder bool
		AllowMallOrder        bool
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT name,
		       default_price,
		       COALESCE(retail_price_100g,0),
		       COALESCE(retail_price_200g,0),
		       COALESCE(retail_price_227g,default_price,0),
		       COALESCE(retail_price_250g,0),
		       COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
		       COALESCE(green_bean_type,''),
		       COALESCE(green_bean_bom_product_id,0),
		       COALESCE(drip_bag_grams,10),
		       COALESCE(drip_box_bag_count,10),
		       COALESCE(allow_fulfillment_order,true),
		       COALESCE(allow_mall_order,false)
		FROM %s.products
		WHERE id=$1 AND active=true
	`, r.schema), cmd.BaseProductID).Scan(&base.Name, &base.DefaultPrice, &base.RetailPrice100G, &base.RetailPrice200G, &base.RetailPrice227G, &base.RetailPrice250G, &base.ProductKind, &base.GreenBeanType, &base.GreenBeanBomProductID, &base.DripBagGrams, &base.DripBoxBagCount, &base.AllowFulfillmentOrder, &base.AllowMallOrder); err != nil {
		return catalogapp.Product{}, fmt.Errorf("base product not found")
	}

	productKind := catalogdomain.NormalizeProductKind(base.ProductKind)
	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	greenBeanType := strings.TrimSpace(base.GreenBeanType)
	greenBeanBomProductID := base.GreenBeanBomProductID
	if productKind == catalogdomain.ProductKindGreenBean {
		roastLevel = ""
		if greenBeanType == "" {
			greenBeanType = "single_origin"
		}
	} else {
		greenBeanType = ""
		greenBeanBomProductID = 0
	}
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	name := strings.TrimSpace(cmd.Name)
	remark := strings.TrimSpace(cmd.Remark)
	var productID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(
			name, remark, product_kind, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,
			product_category_id, product_category_position,
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id, created_at
		)
		VALUES($1,$2,$3,$4,$5,true,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,0),$15,$16,$17,'customer_only',$18,$19,$20,now())
		RETURNING id
	`, r.schema), name, remark, productKind, roastLevel, base.DefaultPrice, base.RetailPrice100G, base.RetailPrice200G, base.RetailPrice227G, base.RetailPrice250G, base.DripBagGrams, base.DripBoxBagCount, base.AllowFulfillmentOrder, base.AllowMallOrder, 0, 0, cmd.CustomerID, cmd.BaseProductID, strings.TrimSpace(cmd.CustomType), greenBeanType, greenBeanBomProductID).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}

	if productKind == catalogdomain.ProductKindRoasted {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_bom(product_id,yield_rate,status,updated_at)
			VALUES($1,$2,'active',now())
			ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()
		`, r.schema), productID, yieldRate); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if cmd.CopyBOM && productKind == catalogdomain.ProductKindRoasted {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,updated_at)
			SELECT $1,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,now()
			FROM %s.product_bom_items
			WHERE product_id=$2
			ORDER BY id
		`, r.schema, r.schema), productID, cmd.BaseProductID); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if cmd.CopyPriceTiers {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_price_tiers(product_id,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active)
			SELECT $1,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active
			FROM %s.product_price_tiers
			WHERE product_id=$2 AND active=true
			ORDER BY id
		`, r.schema, r.schema), productID, cmd.BaseProductID); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &productID, "create", postgresinfra.StrPtr("customer_custom_product"), nil, postgresinfra.StrPtr(name), postgresinfra.AuditMeta{
		"customer_id":             cmd.CustomerID,
		"base_product_id":         cmd.BaseProductID,
		"roast_level":             roastLevel,
		"remark":                  cmd.Remark,
		"custom_type":             strings.TrimSpace(cmd.CustomType),
		"copy_bom":                cmd.CopyBOM,
		"copy_price_tiers":        cmd.CopyPriceTiers,
		"product_kind":            base.ProductKind,
		"drip_bag_grams":          base.DripBagGrams,
		"drip_box_bag_count":      base.DripBoxBagCount,
		"allow_fulfillment_order": base.AllowFulfillmentOrder,
		"allow_mall_order":        base.AllowMallOrder,
	}); err != nil {
		return catalogapp.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.Product{}, err
	}
	product, err := r.GetProduct(ctx, productID)
	if err != nil {
		return catalogapp.Product{}, err
	}
	if product == nil {
		return catalogapp.Product{}, fmt.Errorf("created product not found")
	}
	return *product, nil
}

func (r Repository) ListCustomerPublicUsages(ctx context.Context) ([]catalogapp.CustomerPublicUsage, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT customer_id, use_public_sku, use_public_categories
		FROM %s.customer_sku_public_usage
		ORDER BY customer_id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.CustomerPublicUsage, 0)
	for rows.Next() {
		var row catalogapp.CustomerPublicUsage
		if err := rows.Scan(&row.CustomerID, &row.UsePublicSKU, &row.UsePublicCategories); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveCustomerPublicUsage(ctx context.Context, cmd catalogapp.CustomerPublicUsageCommand) (catalogapp.CustomerPublicUsage, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerExists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, r.schema), cmd.CustomerID).Scan(&customerExists); err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	if !customerExists {
		return catalogapp.CustomerPublicUsage{}, fmt.Errorf("customer not found")
	}
	if err := cleanupLegacyPublicCopiesTx(ctx, tx, r.schema, cmd.CustomerID); err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	var usage catalogapp.CustomerPublicUsage
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_sku_public_usage(customer_id, use_public_sku, use_public_categories, created_at, updated_at)
		VALUES($1, $2, $3, now(), now())
		ON CONFLICT (customer_id) DO UPDATE
		SET use_public_sku=excluded.use_public_sku,
		    use_public_categories=excluded.use_public_categories,
		    updated_at=now()
		RETURNING customer_id, use_public_sku, use_public_categories
	`, r.schema), cmd.CustomerID, cmd.UsePublicSKU, cmd.UsePublicCategories).Scan(&usage.CustomerID, &usage.UsePublicSKU, &usage.UsePublicCategories); err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_catalog", &cmd.CustomerID, "update_public_usage", postgresinfra.StrPtr("public_catalog_reference"), nil, postgresinfra.StrPtr(fmt.Sprintf("sku:%t categories:%t", usage.UsePublicSKU, usage.UsePublicCategories)), postgresinfra.AuditMeta{
		"customer_id":           cmd.CustomerID,
		"use_public_sku":        usage.UsePublicSKU,
		"use_public_categories": usage.UsePublicCategories,
	}); err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	return usage, nil
}

func cleanupLegacyPublicCopiesTx(ctx context.Context, tx pgx.Tx, schema string, customerID int64) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		WITH legacy_products AS (
			SELECT p.id
			FROM %[1]s.products p
			JOIN %[1]s.products base ON base.id=p.base_product_id
			WHERE p.active=true
			  AND p.customer_id=$1
			  AND p.base_product_id > 0
			  AND COALESCE(NULLIF(p.custom_type,''),'')='public_sku_alias'
			  AND COALESCE(base.customer_id,0)=0
			  AND lower(p.name)=lower(base.name)
		)
		UPDATE %[1]s.products p
		SET active=false, product_category_id=NULL, product_category_position=0
		FROM legacy_products lp
		WHERE p.id=lp.id
	`, schema), customerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.product_categories c
		SET active=false, updated_at=now()
		WHERE c.active=true
		  AND c.customer_id=$1
		  AND NOT EXISTS (
			  SELECT 1 FROM %[1]s.products p
			  WHERE p.active=true AND p.product_category_id=c.id
		  )
		  AND EXISTS (
			  SELECT 1
			  FROM %[1]s.product_categories pub
			  LEFT JOIN %[1]s.product_categories pub_parent ON pub_parent.id=pub.parent_id
			  LEFT JOIN %[1]s.product_categories c_parent ON c_parent.id=c.parent_id
			  WHERE pub.active=true
			    AND COALESCE(pub.customer_id,0)=0
			    AND pub.level=c.level
			    AND lower(pub.name)=lower(c.name)
			    AND COALESCE(lower(pub_parent.name),'')=COALESCE(lower(c_parent.name),'')
		  )
	`, schema), customerID); err != nil {
		return err
	}
	return nil
}

func fetchProductByID(ctx context.Context, pool *pgxpool.Pool, schema string, id int64) (*postgresinfra.ProductOption, error) {
	ps, err := postgresinfra.FetchProducts(ctx, pool, schema)
	if err != nil {
		return nil, err
	}
	for i := range ps {
		if ps[i].ID == id {
			return &ps[i], nil
		}
	}
	var p postgresinfra.ProductOption
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id, name, COALESCE(remark,''), COALESCE(roast_level,''), default_price,
		COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
		COALESCE(green_bean_type, ''),
		COALESCE(green_bean_bom_product_id, 0),
		COALESCE(drip_bag_grams, 10)::float8,
		COALESCE(drip_box_bag_count, 10),
		COALESCE(allow_fulfillment_order, true),
		COALESCE(allow_mall_order, false),
		COALESCE(retail_price_100g, 0), COALESCE(retail_price_200g, 0),
		COALESCE(retail_price_227g, default_price, 0), COALESCE(retail_price_250g, 0),
		COALESCE((SELECT yield_rate FROM %[1]s.product_bom WHERE product_id=products.id), 0.8),
		COALESCE(product_category_id,0), COALESCE(product_category_position,0),
		COALESCE(customer_id,0), COALESCE(base_product_id,0),
		COALESCE(NULLIF(visibility,''),'public'), COALESCE(custom_type,''),
		margin_rate_override::float8,
		COALESCE((SELECT COUNT(*) FROM %[1]s.product_bom_items bi WHERE bi.product_id=products.id),0),
		COALESCE((SELECT NULLIF(status,'') FROM %[1]s.product_bom WHERE product_id=products.id), 'missing')
		FROM %[1]s.products WHERE id=$1`, schema), id).Scan(&p.ID, &p.Name, &p.Remark, &p.RoastLevel, &p.DefaultPrice, &p.ProductKind, &p.GreenBeanType, &p.GreenBeanBomProductID, &p.DripBagGrams, &p.DripBoxBagCount, &p.AllowFulfillmentOrder, &p.AllowMallOrder, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.YieldRate, &p.ProductCategoryID, &p.ProductCategoryPosition, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.MarginRateOverride, &p.BomItemCount, &p.BomStatus)
	if err != nil {
		return nil, nil
	}
	if p.ProductKind == catalogdomain.ProductKindDripBag {
		p.SalesUnits = []string{"bag", "box"}
	}
	return &p, nil
}

func catalogProductFromOption(p postgresinfra.ProductOption) catalogapp.Product {
	out := catalogapp.Product{ID: p.ID, Name: p.Name, Remark: p.Remark, RoastLevel: p.RoastLevel, ProductKind: p.ProductKind, GreenBeanType: p.GreenBeanType, GreenBeanBomProductID: p.GreenBeanBomProductID, DripBagGrams: p.DripBagGrams, DripBoxBagCount: p.DripBoxBagCount, AllowFulfillmentOrder: p.AllowFulfillmentOrder, AllowMallOrder: p.AllowMallOrder, SalesUnits: p.SalesUnits, DefaultPrice: p.DefaultPrice, RetailPrice100G: p.RetailPrice100G, RetailPrice200G: p.RetailPrice200G, RetailPrice227G: p.RetailPrice227G, RetailPrice250G: p.RetailPrice250G, YieldRate: p.YieldRate, ProductCategoryID: p.ProductCategoryID, ProductCategoryPosition: p.ProductCategoryPosition, CustomerID: p.CustomerID, BaseProductID: p.BaseProductID, Visibility: p.Visibility, CustomType: p.CustomType, MarginRateOverride: p.MarginRateOverride, BomItemCount: p.BomItemCount, BomStatus: p.BomStatus}
	out.Tiers = make([]catalogapp.PriceTier, 0, len(p.Tiers))
	for _, t := range p.Tiers {
		out.Tiers = append(out.Tiers, catalogapp.PriceTier{ID: t.ID, SpecG: t.SpecG, MinQty: t.MinQty, MaxQty: t.MaxQty, UnitPrice: t.UnitPrice})
	}
	return out
}

func catalogProductsFromOptions(products []postgresinfra.ProductOption) []catalogapp.Product {
	out := make([]catalogapp.Product, 0, len(products))
	for _, p := range products {
		out = append(out, catalogProductFromOption(p))
	}
	return out
}

func normalizeCategoryPositions(ctx context.Context, tx pgx.Tx, schema string, parentID, customerID int64) error {
	return writeCategoryPositions(ctx, tx, schema, parentID, customerID, 0, 0)
}

func placeCategoryPosition(ctx context.Context, tx pgx.Tx, schema string, parentID, customerID, movedID int64, position int) error {
	return writeCategoryPositions(ctx, tx, schema, parentID, customerID, movedID, position)
}

func writeCategoryPositions(ctx context.Context, tx pgx.Tx, schema string, parentID, customerID, movedID int64, position int) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.product_categories
		WHERE active=true AND COALESCE(parent_id,0)=$1 AND ($1<>0 OR COALESCE(customer_id,0)=$2)
		ORDER BY position, id`, schema), parentID, customerID)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if movedID > 0 && id == movedID {
			continue
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if movedID > 0 {
		ids = insertIDAtPosition(ids, movedID, position)
	}
	for i, id := range ids {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_categories SET position=$2, updated_at=now() WHERE id=$1`, schema), id, i+1); err != nil {
			return err
		}
	}
	return nil
}

func insertIDAtPosition(ids []int64, movedID int64, position int) []int64 {
	out := append([]int64(nil), ids...)
	if position <= 0 || position > len(out)+1 {
		position = len(out) + 1
	}
	insertAt := position - 1
	out = append(out, 0)
	copy(out[insertAt+1:], out[insertAt:])
	out[insertAt] = movedID
	return out
}

func normalizeProductPositions(ctx context.Context, tx pgx.Tx, schema string, categoryID, customerID int64) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.products
		WHERE active=true AND COALESCE(product_category_id,0)=$1 AND ($1<>0 OR COALESCE(customer_id,0)=$2)
		ORDER BY product_category_position, name, id`, schema), categoryID, customerID)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET product_category_position=$2 WHERE id=$1`, schema), id, i+1); err != nil {
			return err
		}
	}
	return nil
}
