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

	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET roast_level=$2, retail_price_100g=$3, retail_price_200g=$4, retail_price_227g=$5, retail_price_250g=$6
		WHERE id=$1`, r.schema), cmd.ProductID, roastLevel, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_bom(product_id,yield_rate,updated_at)
		VALUES($1,$2,now())
		ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()`, r.schema), cmd.ProductID, yieldRate); err != nil {
		return err
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
		"roast_level":       roastLevel,
		"yield_rate":        yieldRate,
		"tier_count":        len(cmd.Tiers),
		"retail_price_100g": cmd.RetailPrice100G,
		"retail_price_200g": cmd.RetailPrice200G,
		"retail_price_227g": cmd.RetailPrice227G,
		"retail_price_250g": cmd.RetailPrice250G,
	})
	return nil
}

func (r Repository) UpdateProductBasics(ctx context.Context, cmd catalogapp.UpdateProductBasicsCommand) error {
	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
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
		SET roast_level=$2, retail_price_100g=$3, retail_price_200g=$4, retail_price_227g=$5, retail_price_250g=$6, margin_rate_override=$7
		WHERE id=$1`, r.schema), cmd.ProductID, roastLevel, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G, cmd.MarginRateOverride); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_bom(product_id,yield_rate,updated_at)
		VALUES($1,$2,now())
		ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()`, r.schema), cmd.ProductID, yieldRate); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product", &cmd.ProductID, "update", postgresinfra.StrPtr("product_basics"), nil, postgresinfra.StrPtr(roastLevel), postgresinfra.AuditMeta{
		"product_id":           cmd.ProductID,
		"roast_level":          roastLevel,
		"yield_rate":           yieldRate,
		"retail_price_100g":    cmd.RetailPrice100G,
		"retail_price_200g":    cmd.RetailPrice200G,
		"retail_price_227g":    cmd.RetailPrice227G,
		"retail_price_250g":    cmd.RetailPrice250G,
		"margin_rate_override": cmd.MarginRateOverride,
	})
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
	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	yieldRate := cmd.YieldRate
	if yieldRate <= 0 {
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
			name, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			customer_id, base_product_id, visibility, custom_type, created_at
		)
		VALUES($1,$2,$3,true,$4,$5,$6,$7,0,0,'public','',now())
		RETURNING id
	`, r.schema), name, roastLevel, cmd.DefaultPrice, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom(product_id,yield_rate,status,updated_at)
		VALUES($1,$2,'active',now())
		ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()
	`, r.schema), productID, yieldRate); err != nil {
		return catalogapp.Product{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &productID, "create", postgresinfra.StrPtr("public_product"), nil, postgresinfra.StrPtr(name), postgresinfra.AuditMeta{
		"product_id":        productID,
		"roast_level":       roastLevel,
		"default_price":     cmd.DefaultPrice,
		"yield_rate":        yieldRate,
		"retail_price_100g": cmd.RetailPrice100G,
		"retail_price_200g": cmd.RetailPrice200G,
		"retail_price_227g": cmd.RetailPrice227G,
		"retail_price_250g": cmd.RetailPrice250G,
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
		Name            string
		DefaultPrice    float64
		RetailPrice100G float64
		RetailPrice200G float64
		RetailPrice227G float64
		RetailPrice250G float64
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT name,
		       default_price,
		       COALESCE(retail_price_100g,0),
		       COALESCE(retail_price_200g,0),
		       COALESCE(retail_price_227g,default_price,0),
		       COALESCE(retail_price_250g,0)
		FROM %s.products
		WHERE id=$1 AND active=true
	`, r.schema), cmd.BaseProductID).Scan(&base.Name, &base.DefaultPrice, &base.RetailPrice100G, &base.RetailPrice200G, &base.RetailPrice227G, &base.RetailPrice250G); err != nil {
		return catalogapp.Product{}, fmt.Errorf("base product not found")
	}

	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	name := strings.TrimSpace(cmd.Name)
	var productID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(
			name, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			product_category_id, product_category_position,
			customer_id, base_product_id, visibility, custom_type, created_at
		)
		VALUES($1,$2,$3,true,$4,$5,$6,$7,NULLIF($8,0),$9,$10,$11,'customer_only',$12,now())
		RETURNING id
	`, r.schema), name, roastLevel, base.DefaultPrice, base.RetailPrice100G, base.RetailPrice200G, base.RetailPrice227G, base.RetailPrice250G, 0, 0, cmd.CustomerID, cmd.BaseProductID, strings.TrimSpace(cmd.CustomType)).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom(product_id,yield_rate,status,updated_at)
		VALUES($1,$2,'active',now())
		ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()
	`, r.schema), productID, yieldRate); err != nil {
		return catalogapp.Product{}, err
	}
	if cmd.CopyBOM {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct,updated_at)
			SELECT $1,material_id,ratio_pct,now()
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
		"customer_id":      cmd.CustomerID,
		"base_product_id":  cmd.BaseProductID,
		"roast_level":      roastLevel,
		"custom_type":      strings.TrimSpace(cmd.CustomType),
		"copy_bom":         cmd.CopyBOM,
		"copy_price_tiers": cmd.CopyPriceTiers,
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
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id, name, COALESCE(roast_level,''), default_price,
		COALESCE(retail_price_100g, 0), COALESCE(retail_price_200g, 0),
		COALESCE(retail_price_227g, default_price, 0), COALESCE(retail_price_250g, 0),
		COALESCE((SELECT yield_rate FROM %[1]s.product_bom WHERE product_id=products.id), 0.8),
		COALESCE(product_category_id,0), COALESCE(product_category_position,0),
		COALESCE(customer_id,0), COALESCE(base_product_id,0),
		COALESCE(NULLIF(visibility,''),'public'), COALESCE(custom_type,''),
		margin_rate_override::float8,
		COALESCE((SELECT COUNT(*) FROM %[1]s.product_bom_items bi WHERE bi.product_id=products.id),0),
		COALESCE((SELECT NULLIF(status,'') FROM %[1]s.product_bom WHERE product_id=products.id), 'missing')
		FROM %[1]s.products WHERE id=$1`, schema), id).Scan(&p.ID, &p.Name, &p.RoastLevel, &p.DefaultPrice, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.YieldRate, &p.ProductCategoryID, &p.ProductCategoryPosition, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.MarginRateOverride, &p.BomItemCount, &p.BomStatus)
	if err != nil {
		return nil, nil
	}
	return &p, nil
}

func catalogProductFromOption(p postgresinfra.ProductOption) catalogapp.Product {
	out := catalogapp.Product{ID: p.ID, Name: p.Name, RoastLevel: p.RoastLevel, DefaultPrice: p.DefaultPrice, RetailPrice100G: p.RetailPrice100G, RetailPrice200G: p.RetailPrice200G, RetailPrice227G: p.RetailPrice227G, RetailPrice250G: p.RetailPrice250G, YieldRate: p.YieldRate, ProductCategoryID: p.ProductCategoryID, ProductCategoryPosition: p.ProductCategoryPosition, CustomerID: p.CustomerID, BaseProductID: p.BaseProductID, Visibility: p.Visibility, CustomType: p.CustomType, MarginRateOverride: p.MarginRateOverride, BomItemCount: p.BomItemCount, BomStatus: p.BomStatus}
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
