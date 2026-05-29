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

type skuCopyPlan struct {
	source           catalogapp.Product
	targetID         int64
	targetCategoryID int64
	existingID       int64
}

type skuCopyBOMItem struct {
	materialID         int64
	componentType      string
	componentProductID int64
	componentSpecG     int64
	consumeUnit        string
	qtyPerUnit         float64
	ratioPct           float64
	unitCostSnapshot   float64
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
	yieldRate := 0.0
	if catalogdomain.ProductKindRequiresRoast(productKind) {
		if roastLevel == "" {
			roastLevel = "中烘"
		}
		yieldRate = catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET product_kind=$2, roast_level=$3, default_price=$4,
		    retail_price_100g=$5, retail_price_200g=$6, retail_price_227g=$7, retail_price_250g=$8,
		    green_bean_type=$9, green_bean_bom_product_id=$10
		WHERE id=$1`, r.schema), cmd.ProductID, productKind, roastLevel, cmd.DefaultPrice, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G, greenBeanType, greenBeanBomProductID); err != nil {
		return err
	}
	if catalogdomain.ProductKindSupportsBomParams(productKind) && yieldRate > 0 {
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
	yieldRate := 0.0
	if catalogdomain.ProductKindRequiresRoast(productKind) {
		if roastLevel == "" {
			roastLevel = "中烘"
		}
		yieldRate = catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	}
	if catalogdomain.ProductKindSupportsBomParams(productKind) && cmd.YieldRate > 0 {
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
		    green_bean_type=$13, green_bean_bom_product_id=$14, remark=$15, name=COALESCE(NULLIF($16,''), name),
		    gradient_template_id_override=$17, operation_template_id_override=$18, unit_rule_override_json=$19::jsonb,
		    special_attrs_json=$20::jsonb
		WHERE id=$1`, r.schema), cmd.ProductID, roastLevel, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G, productKind, cmd.DripBagGrams, cmd.MarginRateOverride, cmd.DripBoxBagCount, cmd.AllowFulfillmentOrder, cmd.AllowMallOrder, greenBeanType, greenBeanBomProductID, cmd.Remark, cmd.Name, cmd.GradientTemplateIDOverride, cmd.OperationTemplateIDOverride, cmd.UnitRuleOverrideJSON, cmd.SpecialAttrsJSON); err != nil {
		return err
	}
	if catalogdomain.ProductKindSupportsBomParams(productKind) && yieldRate > 0 {
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
		"product_id":                     cmd.ProductID,
		"product_kind":                   productKind,
		"roast_level":                    roastLevel,
		"yield_rate":                     yieldRate,
		"retail_price_100g":              cmd.RetailPrice100G,
		"retail_price_200g":              cmd.RetailPrice200G,
		"retail_price_227g":              cmd.RetailPrice227G,
		"retail_price_250g":              cmd.RetailPrice250G,
		"margin_rate_override":           cmd.MarginRateOverride,
		"remark":                         cmd.Remark,
		"name":                           cmd.Name,
		"gradient_template_id_override":  cmd.GradientTemplateIDOverride,
		"operation_template_id_override": cmd.OperationTemplateIDOverride,
		"unit_rule_override_json":        cmd.UnitRuleOverrideJSON,
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
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.bom_versions SET status='disabled'
		WHERE product_id = ANY($1) AND status='active'`, r.schema), cmd.ProductIDs); err != nil {
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
	if catalogdomain.ProductKindRequiresRoast(productKind) && roastLevel == "" {
		roastLevel = "中烘"
	}
	if catalogdomain.ProductKindSupportsBomParams(productKind) && yieldRate <= 0 {
		if catalogdomain.ProductKindRequiresRoast(productKind) {
			yieldRate = catalogdomain.ResolveYieldRate(roastLevel, 0.8)
		}
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
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id, special_attrs_json, created_at
		)
		VALUES($1,$2,$3,$4,$5,true,$6,$7,$8,$9,$10,$11,$12,$13,0,0,'public','',$14,$15,$16::jsonb,now())
		RETURNING id
	`, r.schema), name, cmd.Remark, productKind, roastLevel, cmd.DefaultPrice, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G, cmd.DripBagGrams, cmd.DripBoxBagCount, cmd.AllowFulfillmentOrder, cmd.AllowMallOrder, greenBeanType, greenBeanBomProductID, cmd.SpecialAttrsJSON).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}

	if catalogdomain.ProductKindSupportsBomParams(productKind) && yieldRate > 0 {
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

func (r Repository) CreateSKU(ctx context.Context, cmd catalogapp.CreateSKUCommand) (catalogapp.Product, error) {
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

	if cmd.CustomerID > 0 {
		if err := ensureCustomerExistsTx(ctx, tx, r.schema, cmd.CustomerID); err != nil {
			return catalogapp.Product{}, err
		}
	}
	categoryID := int64(0)
	productKind := catalogdomain.ProductKindRoasted
	if cmd.ProductSubtypeCategoryID > 0 {
		category, err := ensureProductCategoryForTargetTx(ctx, tx, r.schema, cmd.Actor, cmd.CustomerID, cmd.ProductSubtypeCategoryID)
		if err != nil {
			return catalogapp.Product{}, err
		}
		categoryID = category.ID
		typeName := category.Name
		if category.ParentID > 0 {
			if parent, err := fetchProductCategoryTx(ctx, tx, r.schema, category.ParentID); err == nil {
				typeName = parent.Name
			}
		}
		productKind = catalogdomain.NormalizeProductKind(typeName)
	}
	position, err := nextProductPositionTx(ctx, tx, r.schema, categoryID, cmd.CustomerID, 0)
	if err != nil {
		return catalogapp.Product{}, err
	}
	visibility := "public"
	if cmd.CustomerID > 0 {
		visibility = "customer_only"
	}
	var productID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(
			name, remark, product_kind, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,
			product_category_id, product_category_position,
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id, special_attrs_json, created_at
		)
		VALUES($1,$2,$3,'',0,$4,0,0,0,0,10,10,true,false,NULLIF($5,0),$6,$7,0,$8,'','',0,$9::jsonb,now())
		RETURNING id
	`, r.schema), strings.TrimSpace(cmd.Name), strings.TrimSpace(cmd.Remark), productKind, cmd.Active, categoryID, position, cmd.CustomerID, visibility, cmd.SpecialAttrsJSON).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}
	if categoryID > 0 {
		if err := normalizeProductPositions(ctx, tx, r.schema, categoryID, cmd.CustomerID); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &productID, "create_sku", postgresinfra.StrPtr("sku"), nil, postgresinfra.StrPtr(strings.TrimSpace(cmd.Name)), postgresinfra.AuditMeta{
		"customer_id":                  cmd.CustomerID,
		"product_type_category_id":     cmd.ProductTypeCategoryID,
		"product_subtype_category_id":  categoryID,
		"legacy_product_kind_snapshot": productKind,
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
		return catalogapp.Product{}, fmt.Errorf("created sku not found")
	}
	return *product, nil
}

func (r Repository) ListSKUCopyOptions(ctx context.Context, query catalogapp.SKUCopyOptionsQuery) (catalogapp.SKUCopyOptions, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.SKUCopyOptions{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.SKUCopyOptions{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT p.id, p.name, COALESCE(p.remark,''), COALESCE(p.customer_id,0), COALESCE(p.product_category_id,0), COALESCE(p.active,true),
		       COALESCE(subtype.id,0), COALESCE(subtype.name,'未分类'),
		       COALESCE(ptype.id,0), COALESCE(ptype.name,'未分类')
		FROM %s.products p
		LEFT JOIN %s.product_categories subtype ON subtype.id=COALESCE(p.product_category_id,0)
		LEFT JOIN %s.product_categories ptype ON ptype.id=COALESCE(subtype.parent_id,0)
		WHERE COALESCE(p.customer_id,0)=$1
		ORDER BY COALESCE(ptype.position,9999), ptype.name, COALESCE(subtype.position,9999), subtype.name, p.name, p.id
	`, r.schema, r.schema, r.schema), query.SourceCustomerID)
	if err != nil {
		return catalogapp.SKUCopyOptions{}, err
	}

	out := catalogapp.SKUCopyOptions{
		Title:            "选择分类和产品",
		TargetCustomerID: query.TargetCustomerID,
		SourceCustomerID: query.SourceCustomerID,
		Groups:           make([]catalogapp.SKUCopyTypeGroup, 0),
	}
	typeIndex := map[int64]int{}
	subtypeIndex := map[string]int{}
	sourceOptions := make([]catalogapp.SKUCopyOption, 0)
	for rows.Next() {
		var option catalogapp.SKUCopyOption
		if err := rows.Scan(&option.ID, &option.Name, &option.Remark, &option.SourceCustomerID, &option.ProductSubtypeCategoryID, &option.Active, &option.ProductSubtypeCategoryID, &option.ProductSubtypeName, &option.ProductTypeCategoryID, &option.ProductTypeName); err != nil {
			rows.Close()
			return catalogapp.SKUCopyOptions{}, err
		}
		option.CopyState = "available"
		if !option.Active {
			option.CopyState = "inactive"
		}
		sourceOptions = append(sourceOptions, option)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return catalogapp.SKUCopyOptions{}, err
	}

	for _, option := range sourceOptions {
		if option.Active {
			targetSubtypeID, err := findEquivalentCategoryForTargetTx(ctx, tx, r.schema, query.TargetCustomerID, option.ProductSubtypeCategoryID)
			if err != nil {
				return catalogapp.SKUCopyOptions{}, err
			}
			if targetSubtypeID >= 0 {
				existingID, err := findTargetSKUByNameTx(ctx, tx, r.schema, query.TargetCustomerID, targetSubtypeID, option.Name)
				if err != nil {
					return catalogapp.SKUCopyOptions{}, err
				}
				if existingID > 0 {
					option.CopyState = "will_overwrite"
				}
			}
		}
		out.TotalCount++
		typeID := option.ProductTypeCategoryID
		if typeID == 0 {
			typeID = -1
		}
		idx, ok := typeIndex[typeID]
		if !ok {
			out.Groups = append(out.Groups, catalogapp.SKUCopyTypeGroup{ID: option.ProductTypeCategoryID, Name: option.ProductTypeName, Children: make([]catalogapp.SKUCopySubtypeGroup, 0)})
			idx = len(out.Groups) - 1
			typeIndex[typeID] = idx
		}
		subtypeKey := fmt.Sprintf("%d:%d", typeID, option.ProductSubtypeCategoryID)
		childIdx, ok := subtypeIndex[subtypeKey]
		if !ok {
			out.Groups[idx].Children = append(out.Groups[idx].Children, catalogapp.SKUCopySubtypeGroup{ID: option.ProductSubtypeCategoryID, Name: option.ProductSubtypeName, Products: make([]catalogapp.SKUCopyOption, 0)})
			childIdx = len(out.Groups[idx].Children) - 1
			subtypeIndex[subtypeKey] = childIdx
		}
		out.Groups[idx].Children[childIdx].Products = append(out.Groups[idx].Children[childIdx].Products, option)
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.SKUCopyOptions{}, err
	}
	return out, nil
}

func (r Repository) CopySKUs(ctx context.Context, cmd catalogapp.CopySKUsCommand) (catalogapp.CopySKUsResult, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.CopySKUsResult{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.CopySKUsResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if cmd.TargetCustomerID > 0 {
		if err := ensureCustomerExistsTx(ctx, tx, r.schema, cmd.TargetCustomerID); err != nil {
			return catalogapp.CopySKUsResult{}, err
		}
	}
	if cmd.SourceCustomerID > 0 {
		if err := ensureCustomerExistsTx(ctx, tx, r.schema, cmd.SourceCustomerID); err != nil {
			return catalogapp.CopySKUsResult{}, err
		}
	}
	result := catalogapp.CopySKUsResult{}
	sourceToTarget := map[int64]int64{}
	plans := make([]skuCopyPlan, 0, len(cmd.SourceSKUIDs))
	for _, sourceID := range cmd.SourceSKUIDs {
		source, err := fetchSKUCopySourceTx(ctx, tx, r.schema, sourceID)
		if err != nil {
			return catalogapp.CopySKUsResult{}, err
		}
		if source.CustomerID != cmd.SourceCustomerID {
			result.SkippedCount++
			continue
		}
		if !source.Active {
			result.SkippedCount++
			continue
		}
		targetCategoryID := int64(0)
		if source.ProductCategoryID > 0 {
			category, err := ensureProductCategoryForTargetTx(ctx, tx, r.schema, cmd.Actor, cmd.TargetCustomerID, source.ProductCategoryID)
			if err != nil {
				return catalogapp.CopySKUsResult{}, err
			}
			targetCategoryID = category.ID
		}
		existingID, err := findTargetSKUByNameTx(ctx, tx, r.schema, cmd.TargetCustomerID, targetCategoryID, source.Name)
		if err != nil {
			return catalogapp.CopySKUsResult{}, err
		}
		targetID := existingID
		position, err := nextProductPositionTx(ctx, tx, r.schema, targetCategoryID, cmd.TargetCustomerID, 0)
		if err != nil {
			return catalogapp.CopySKUsResult{}, err
		}
		visibility := "public"
		if cmd.TargetCustomerID > 0 {
			visibility = "customer_only"
		}
		initialSource := source
		initialSource.GreenBeanBomProductID = 0
		if targetID > 0 {
			if err := updateCopiedSKUProductTx(ctx, tx, r.schema, targetID, initialSource, cmd.TargetCustomerID, targetCategoryID, visibility); err != nil {
				return catalogapp.CopySKUsResult{}, err
			}
			result.OverwrittenCount++
		} else {
			targetID, err = insertCopiedSKUProductTx(ctx, tx, r.schema, initialSource, cmd.TargetCustomerID, targetCategoryID, position, visibility)
			if err != nil {
				return catalogapp.CopySKUsResult{}, err
			}
			result.CreatedCount++
		}
		sourceToTarget[source.ID] = targetID
		plans = append(plans, skuCopyPlan{
			source:           source,
			targetID:         targetID,
			targetCategoryID: targetCategoryID,
			existingID:       existingID,
		})
	}
	for _, plan := range plans {
		greenBeanBOMProductID, err := resolveSKUCopyProductReferenceTx(ctx, tx, r.schema, cmd.Actor, cmd.TargetCustomerID, plan.source.GreenBeanBomProductID, sourceToTarget)
		if err != nil {
			return catalogapp.CopySKUsResult{}, err
		}
		if err := updateCopiedSKUGreenBeanReferenceTx(ctx, tx, r.schema, plan.targetID, greenBeanBOMProductID); err != nil {
			return catalogapp.CopySKUsResult{}, err
		}
		if err := setProductBOMSourceToInheritTx(ctx, tx, r.schema, cmd.Actor, plan.source.ID, plan.targetID); err != nil {
			return catalogapp.CopySKUsResult{}, err
		}
		if err := copyProductPriceTiersTx(ctx, tx, r.schema, plan.source.ID, plan.targetID); err != nil {
			return catalogapp.CopySKUsResult{}, err
		}
		if plan.targetCategoryID > 0 {
			if err := normalizeProductPositions(ctx, tx, r.schema, plan.targetCategoryID, cmd.TargetCustomerID); err != nil {
				return catalogapp.CopySKUsResult{}, err
			}
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &plan.targetID, "copy_sku", postgresinfra.StrPtr("source_sku_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", plan.source.ID)), postgresinfra.AuditMeta{
			"target_customer_id":  cmd.TargetCustomerID,
			"source_customer_id":  cmd.SourceCustomerID,
			"source_sku_id":       plan.source.ID,
			"bom_source_type":     "inherit_current",
			"source_product_id":   plan.source.ID,
			"target_category_id":  plan.targetCategoryID,
			"overwrote_existing":  plan.existingID > 0,
			"preserved_target_id": plan.existingID > 0,
		}); err != nil {
			// Test markers: "bom_source_type":    "inherit_current"; "source_product_id":   plan.source.ID.
			return catalogapp.CopySKUsResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.CopySKUsResult{}, err
	}
	return result, nil
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
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position, COALESCE(gradient_template_id,0),
		       COALESCE(product_config_template_id,0),
		       COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
		       COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'),
		       COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false),
		       COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END)
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
		if err := rows.Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListProductConfigTemplates(ctx context.Context) ([]catalogapp.ProductConfigTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(customer_id,0), COALESCE(source_template_id,0),
		       COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END),
		       name, COALESCE(gradient_template_id,0), COALESCE(operation_template_id,0), COALESCE(unit_template_id,0),
		       COALESCE(price_list_rule_json::text,'{}'), COALESCE(special_attrs_schema_json::text,'[]'),
		       COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'),
		       COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false), active
		FROM %s.product_config_templates
		ORDER BY active DESC, COALESCE(customer_id,0), name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductConfigTemplate, 0)
	for rows.Next() {
		var row catalogapp.ProductConfigTemplate
		if err := rows.Scan(&row.ID, &row.CustomerID, &row.SourceTemplateID, &row.TemplateState, &row.Name, &row.GradientTemplateID, &row.OperationTemplateID, &row.UnitTemplateID, &row.PriceListRuleJSON, &row.SpecialAttrsSchemaJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListProductUnitDefinitions(ctx context.Context) ([]catalogapp.ProductUnitDefinition, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT code, name, unit_type, allow_decimal, active
		FROM %s.product_unit_definitions
		ORDER BY active DESC, unit_type, code
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductUnitDefinition, 0)
	for rows.Next() {
		var row catalogapp.ProductUnitDefinition
		if err := rows.Scan(&row.Code, &row.Name, &row.UnitType, &row.AllowDecimal, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListProductUnitTemplates(ctx context.Context) ([]catalogapp.ProductUnitTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, inventory_unit, quote_unit, order_unit,
		       COALESCE(unit_conversion_json::text,'{}'), integer_unit, active
		FROM %s.product_unit_templates
		ORDER BY active DESC, name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductUnitTemplate, 0)
	for rows.Next() {
		var row catalogapp.ProductUnitTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveProductUnitDefinition(ctx context.Context, cmd catalogapp.SaveProductUnitDefinitionCommand) (catalogapp.ProductUnitDefinition, error) {
	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	var row catalogapp.ProductUnitDefinition
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code,name,unit_type,allow_decimal,active)
		VALUES($1,$2,$3,$4,$5)
		ON CONFLICT (code) DO UPDATE
		SET name=EXCLUDED.name,
		    unit_type=EXCLUDED.unit_type,
		    allow_decimal=EXCLUDED.allow_decimal,
		    active=EXCLUDED.active,
		    updated_at=now()
		RETURNING code, name, unit_type, allow_decimal, active
	`, r.schema), cmd.Code, cmd.Name, cmd.UnitType, cmd.AllowDecimal, active).Scan(&row.Code, &row.Name, &row.UnitType, &row.AllowDecimal, &row.Active); err != nil {
		return catalogapp.ProductUnitDefinition{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_unit_definition", nil, "upsert", postgresinfra.StrPtr("code"), nil, postgresinfra.StrPtr(row.Code), postgresinfra.AuditMeta{"unit_type": row.UnitType, "allow_decimal": row.AllowDecimal, "active": row.Active})
	return row, nil
}

func (r Repository) SaveProductUnitTemplate(ctx context.Context, cmd catalogapp.SaveProductUnitTemplateCommand) (catalogapp.ProductUnitTemplate, error) {
	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	var id int64
	if cmd.ID > 0 {
		if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_unit_templates
			SET name=$2, inventory_unit=$3, quote_unit=$4, order_unit=$5,
			    unit_conversion_json=$6::jsonb, integer_unit=$7, active=$8, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.InventoryUnit, cmd.QuoteUnit, cmd.OrderUnit, cmd.UnitConversionJSON, cmd.IntegerUnit, active).Scan(&id); err != nil {
			return catalogapp.ProductUnitTemplate{}, err
		}
	} else if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_templates(name, inventory_unit, quote_unit, order_unit, unit_conversion_json, integer_unit, active)
		VALUES($1,$2,$3,$4,$5::jsonb,$6,$7)
		RETURNING id
	`, r.schema), cmd.Name, cmd.InventoryUnit, cmd.QuoteUnit, cmd.OrderUnit, cmd.UnitConversionJSON, cmd.IntegerUnit, active).Scan(&id); err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	row, err := fetchProductUnitTemplateTx(ctx, r.pool, r.schema, id)
	if err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_unit_template", &id, "update", postgresinfra.StrPtr("template"), nil, postgresinfra.StrPtr(row.Name), postgresinfra.AuditMeta{"inventory_unit": row.InventoryUnit, "quote_unit": row.QuoteUnit, "order_unit": row.OrderUnit, "integer_unit": row.IntegerUnit})
	return row, nil
}

func (r Repository) SaveProductConfigTemplate(ctx context.Context, cmd catalogapp.SaveProductConfigTemplateCommand) (catalogapp.ProductConfigTemplate, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	customerID := cmd.CustomerID
	if customerID < 0 {
		customerID = 0
	}
	templateState := catalogapp.TemplateStateCustomer
	if customerID == 0 {
		templateState = catalogapp.TemplateStatePublic
	}
	if cmd.UnitTemplateID > 0 {
		unitTemplate, err := fetchProductUnitTemplateTx(ctx, tx, r.schema, cmd.UnitTemplateID)
		if err != nil {
			return catalogapp.ProductConfigTemplate{}, err
		}
		if !unitTemplate.Active {
			return catalogapp.ProductConfigTemplate{}, fmt.Errorf("unit template inactive")
		}
		cmd.InventoryUnit = unitTemplate.InventoryUnit
		cmd.QuoteUnit = unitTemplate.QuoteUnit
		cmd.OrderUnit = unitTemplate.OrderUnit
		cmd.UnitConversionJSON = unitTemplate.UnitConversionJSON
		cmd.IntegerUnit = unitTemplate.IntegerUnit
	}
	var id int64
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_config_templates
			SET name=$2, gradient_template_id=$3, operation_template_id=$4, price_list_rule_json=$5::jsonb,
			    inventory_unit=$6, quote_unit=$7, order_unit=$8, unit_conversion_json=$9::jsonb, integer_unit=$10,
			    unit_template_id=$11, active=$12, special_attrs_schema_json=$13::jsonb, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.GradientTemplateID, cmd.OperationTemplateID, cmd.PriceListRuleJSON, cmd.InventoryUnit, cmd.QuoteUnit, cmd.OrderUnit, cmd.UnitConversionJSON, cmd.IntegerUnit, cmd.UnitTemplateID, active, cmd.SpecialAttrsSchemaJSON).Scan(&id); err != nil {
			return catalogapp.ProductConfigTemplate{}, err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_config_templates(customer_id, source_template_id, template_state, name, gradient_template_id, operation_template_id, unit_template_id, price_list_rule_json, inventory_unit, quote_unit, order_unit, unit_conversion_json, integer_unit, active, special_attrs_schema_json)
			VALUES($1,0,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11::jsonb,$12,$13,$14::jsonb)
			RETURNING id
		`, r.schema), customerID, templateState, cmd.Name, cmd.GradientTemplateID, cmd.OperationTemplateID, cmd.UnitTemplateID, cmd.PriceListRuleJSON, cmd.InventoryUnit, cmd.QuoteUnit, cmd.OrderUnit, cmd.UnitConversionJSON, cmd.IntegerUnit, active, cmd.SpecialAttrsSchemaJSON).Scan(&id); err != nil {
			return catalogapp.ProductConfigTemplate{}, err
		}
	}
	row, err := fetchProductConfigTemplateTx(ctx, tx, r.schema, id)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	if err := materializeProductConfigTemplateToCategoriesTx(ctx, tx, r.schema, row); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_config_template", &id, "update", postgresinfra.StrPtr("template"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{
		"customer_id":             row.CustomerID,
		"gradient_template_id":    row.GradientTemplateID,
		"operation_template_id":   row.OperationTemplateID,
		"unit_template_id":        row.UnitTemplateID,
		"inventory_unit":          row.InventoryUnit,
		"quote_unit":              row.QuoteUnit,
		"order_unit":              row.OrderUnit,
		"integer_unit":            row.IntegerUnit,
		"materializes_categories": true,
	}); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	return row, nil
}

func (r Repository) DeriveProductConfigTemplate(ctx context.Context, cmd catalogapp.DeriveProductConfigTemplateCommand) (catalogapp.ProductConfigTemplate, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureCustomerExistsTx(ctx, tx, r.schema, cmd.CustomerID); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	row, err := deriveProductConfigTemplateTx(ctx, tx, r.schema, cmd)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	return row, nil
}

func (r Repository) ListGradientTemplates(ctx context.Context) ([]catalogapp.GradientTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, COALESCE(customer_id,0), COALESCE(source_template_id,0),
		       COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END),
		       display_unit, COALESCE(unit_template_id,0), active
		FROM %s.pricing_gradient_templates
		ORDER BY active DESC, COALESCE(customer_id,0), name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.GradientTemplate, 0)
	for rows.Next() {
		var row catalogapp.GradientTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.CustomerID, &row.SourceTemplateID, &row.TemplateState, &row.DisplayUnit, &row.UnitTemplateID, &row.Active); err != nil {
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
	var customerID int64
	var sourceTemplateID int64
	var templateState string
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.pricing_gradient_templates
			SET name=$2, display_unit=$3, unit_template_id=$4, active=true, updated_at=now()
			WHERE id=$1
			RETURNING id, COALESCE(customer_id,0), COALESCE(source_template_id,0),
			          COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END)
		`, r.schema), cmd.ID, cmd.Name, cmd.DisplayUnit, cmd.UnitTemplateID).Scan(&id, &customerID, &sourceTemplateID, &templateState); err != nil {
			return catalogapp.GradientTemplate{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.pricing_gradient_template_tiers SET active=false, updated_at=now() WHERE template_id=$1`, r.schema), id); err != nil {
			return catalogapp.GradientTemplate{}, err
		}
	} else {
		customerID = cmd.CustomerID
		if customerID < 0 {
			customerID = 0
		}
		templateState = catalogapp.TemplateStateCustomer
		if customerID == 0 {
			templateState = catalogapp.TemplateStatePublic
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.pricing_gradient_templates(name, display_unit, unit_template_id, customer_id, source_template_id, template_state, active)
			VALUES($1,$2,$3,$4,0,$5,true)
			RETURNING id, COALESCE(customer_id,0), COALESCE(source_template_id,0), template_state
		`, r.schema), cmd.Name, cmd.DisplayUnit, cmd.UnitTemplateID, customerID, templateState).Scan(&id, &customerID, &sourceTemplateID, &templateState); err != nil {
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
		"customer_id":      customerID,
		"display_unit":     cmd.DisplayUnit,
		"unit_template_id": cmd.UnitTemplateID,
		"tier_count":       len(cmd.Tiers),
	}); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	return catalogapp.GradientTemplate{ID: id, Name: cmd.Name, CustomerID: customerID, SourceTemplateID: sourceTemplateID, TemplateState: templateState, DisplayUnit: cmd.DisplayUnit, UnitTemplateID: cmd.UnitTemplateID, Active: true, Tiers: cmd.Tiers}, nil
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
	var oldID int64
	var level int
	var categoryCustomerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(gradient_template_id,0), level, COALESCE(customer_id,0) FROM %s.product_categories WHERE id=$1 AND active=true`, r.schema), cmd.CategoryID).Scan(&oldID, &level, &categoryCustomerID); err != nil {
		return err
	}
	if level != 2 {
		return fmt.Errorf("only secondary category can bind gradient template")
	}
	targetTemplateID := cmd.GradientTemplateID
	sourceTemplateID := int64(0)
	if cmd.GradientTemplateID > 0 {
		template, err := fetchGradientTemplateTx(ctx, tx, r.schema, cmd.GradientTemplateID)
		if err != nil {
			return err
		}
		switch {
		case categoryCustomerID > 0 && template.CustomerID == 0:
			derived, err := deriveGradientTemplateTx(ctx, tx, r.schema, catalogapp.DeriveGradientTemplateCommand{
				Actor:            cmd.Actor,
				CustomerID:       categoryCustomerID,
				SourceTemplateID: cmd.GradientTemplateID,
			})
			if err != nil {
				return err
			}
			targetTemplateID = derived.ID
			sourceTemplateID = template.ID
		case template.CustomerID != categoryCustomerID:
			return fmt.Errorf("gradient template customer mismatch")
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_categories SET gradient_template_id=$2, updated_at=now() WHERE id=$1`, r.schema), cmd.CategoryID, targetTemplateID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_category", &cmd.CategoryID, "update", postgresinfra.StrPtr("gradient_template_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", oldID)), postgresinfra.StrPtr(fmt.Sprintf("%d", targetTemplateID)), postgresinfra.AuditMeta{
		"customer_id":             categoryCustomerID,
		"requested_template_id":   cmd.GradientTemplateID,
		"source_template_id":      sourceTemplateID,
		"resolved_template_id":    targetTemplateID,
		"derived_public_template": sourceTemplateID > 0,
	}); err != nil {
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
	resolvedCmd, err := resolveProductCategoryConfigTemplateTx(ctx, tx, r.schema, cmd, customerID, parentID)
	if err != nil {
		return catalogapp.ProductCategory{}, err
	}
	cmd = resolvedCmd
	var row catalogapp.ProductCategory
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.product_categories
			SET parent_id=NULLIF($2,0), name=$3, level=$4, position=$5, customer_id=$6,
			    gradient_template_id=$7, operation_template_id=$8, price_list_rule_json=$9::jsonb,
			    inventory_unit=$10, quote_unit=$11, order_unit=$12, unit_conversion_json=$13::jsonb, integer_unit=$14,
			    product_config_template_id=$15,
			    updated_at=now()
			WHERE id=$1 AND active=true
			RETURNING id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position, COALESCE(gradient_template_id,0),
			          COALESCE(product_config_template_id,0),
			          COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
			          COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'), COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false),
			          COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END)`, r.schema), cmd.ID, parentID, cmd.Name, level, position, customerID, cmd.GradientTemplateID, cmd.OperationTemplateID, cmd.PriceListRuleJSON, cmd.InventoryUnit, cmd.QuoteUnit, cmd.OrderUnit, cmd.UnitConversionJSON, cmd.IntegerUnit, cmd.ProductConfigTemplateID).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState); err != nil {
			return catalogapp.ProductCategory{}, err
		}
	} else {
		templateState := catalogapp.TemplateStateCustomer
		if customerID == 0 {
			templateState = catalogapp.TemplateStatePublic
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.product_categories(parent_id,customer_id,source_category_id,name,level,position,gradient_template_id,operation_template_id,price_list_rule_json,inventory_unit,quote_unit,order_unit,unit_conversion_json,integer_unit,product_config_template_id,template_state)
			VALUES(NULLIF($1,0),$2,0,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12::jsonb,$13,$14,$15)
			RETURNING id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position, COALESCE(gradient_template_id,0),
			          COALESCE(product_config_template_id,0),
			          COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
			          COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'), COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false), template_state`, r.schema), parentID, customerID, cmd.Name, level, position, cmd.GradientTemplateID, cmd.OperationTemplateID, cmd.PriceListRuleJSON, cmd.InventoryUnit, cmd.QuoteUnit, cmd.OrderUnit, cmd.UnitConversionJSON, cmd.IntegerUnit, cmd.ProductConfigTemplateID, templateState).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState); err != nil {
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
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_category", &row.ID, "update", postgresinfra.StrPtr("category"), nil, postgresinfra.StrPtr(row.Name), postgresinfra.AuditMeta{"parent_id": parentID, "customer_id": customerID, "position": position, "product_config_template_id": cmd.ProductConfigTemplateID, "gradient_template_id": cmd.GradientTemplateID, "operation_template_id": cmd.OperationTemplateID, "inventory_unit": cmd.InventoryUnit, "quote_unit": cmd.QuoteUnit, "order_unit": cmd.OrderUnit, "integer_unit": cmd.IntegerUnit})
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

func (r Repository) AssignProductCategory(ctx context.Context, cmd catalogapp.AssignProductCategoryCommand) (catalogapp.AssignProductCategoryResult, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.AssignProductCategoryResult{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.AssignProductCategoryResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if cmd.CustomerID > 0 {
		if err := ensureCustomerExistsTx(ctx, tx, r.schema, cmd.CustomerID); err != nil {
			return catalogapp.AssignProductCategoryResult{}, err
		}
	}
	result := catalogapp.AssignProductCategoryResult{ProductID: cmd.ProductID, CategoryID: cmd.CategoryID}
	targetProductID := cmd.ProductID
	targetCategoryID := cmd.CategoryID
	productCustomerID, productName, err := fetchProductAssignmentOwnerTx(ctx, tx, r.schema, cmd.ProductID)
	if err != nil {
		return catalogapp.AssignProductCategoryResult{}, err
	}
	if cmd.CustomerID > 0 {
		if targetCategoryID > 0 {
			category, err := fetchProductCategoryTx(ctx, tx, r.schema, targetCategoryID)
			if err != nil {
				return catalogapp.AssignProductCategoryResult{}, err
			}
			switch {
			case category.CustomerID == 0 && cmd.DerivePublicCategory:
				derived, err := deriveProductCategoryTx(ctx, tx, r.schema, catalogapp.DeriveProductCategoryCommand{
					Actor:            cmd.Actor,
					CustomerID:       cmd.CustomerID,
					SourceCategoryID: targetCategoryID,
				})
				if err != nil {
					return catalogapp.AssignProductCategoryResult{}, err
				}
				targetCategoryID = derived.ID
				result.DerivedCategoryID = derived.ID
				result.UsedPublicCategory = true
			case category.CustomerID == cmd.CustomerID:
				// already customer-owned
			case category.CustomerID == 0:
				return catalogapp.AssignProductCategoryResult{}, fmt.Errorf("public category requires derivation")
			default:
				return catalogapp.AssignProductCategoryResult{}, fmt.Errorf("category customer mismatch")
			}
		}
		switch {
		case productCustomerID == 0 && cmd.DerivePublicProduct:
			derived, err := deriveCustomerProductTx(ctx, tx, r.schema, catalogapp.DeriveCustomerProductCommand{
				Actor:          cmd.Actor,
				CustomerID:     cmd.CustomerID,
				BaseProductID:  cmd.ProductID,
				CategoryID:     targetCategoryID,
				Position:       cmd.Position,
				Name:           productName,
				CopyBOM:        true,
				CopyPriceTiers: true,
			})
			if err != nil {
				return catalogapp.AssignProductCategoryResult{}, err
			}
			targetProductID = derived.ID
			result.DerivedProductID = derived.ID
			result.UsedPublicProduct = true
		case productCustomerID == cmd.CustomerID:
			// already customer-owned
		case productCustomerID == 0:
			return catalogapp.AssignProductCategoryResult{}, fmt.Errorf("public product requires derivation")
		default:
			return catalogapp.AssignProductCategoryResult{}, fmt.Errorf("product customer mismatch")
		}
	} else if targetCategoryID > 0 {
		category, err := fetchProductCategoryTx(ctx, tx, r.schema, targetCategoryID)
		if err != nil {
			return catalogapp.AssignProductCategoryResult{}, err
		}
		if category.CustomerID != productCustomerID {
			return catalogapp.AssignProductCategoryResult{}, fmt.Errorf("category customer mismatch")
		}
	}

	var oldCategory int64
	var targetProductCustomerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(product_category_id,0), COALESCE(customer_id,0) FROM %s.products WHERE id=$1 AND active=true`, r.schema), targetProductID).Scan(&oldCategory, &targetProductCustomerID); err != nil {
		return catalogapp.AssignProductCategoryResult{}, err
	}
	position := cmd.Position
	if position <= 0 {
		position = 9999
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET product_category_id=NULLIF($2,0), product_category_position=$3
		WHERE id=$1`, r.schema), targetProductID, targetCategoryID, position); err != nil {
		return catalogapp.AssignProductCategoryResult{}, err
	}
	if err := normalizeProductPositions(ctx, tx, r.schema, oldCategory, targetProductCustomerID); err != nil {
		return catalogapp.AssignProductCategoryResult{}, err
	}
	if oldCategory != targetCategoryID {
		if err := normalizeProductPositions(ctx, tx, r.schema, targetCategoryID, targetProductCustomerID); err != nil {
			return catalogapp.AssignProductCategoryResult{}, err
		}
	}
	result.ProductID = targetProductID
	result.CategoryID = targetCategoryID
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &targetProductID, "move", postgresinfra.StrPtr("product_category"), postgresinfra.StrPtr(fmt.Sprintf("%d", oldCategory)), postgresinfra.StrPtr(fmt.Sprintf("%d:%d", targetCategoryID, position)), postgresinfra.AuditMeta{
		"customer_id":          cmd.CustomerID,
		"source_product_id":    cmd.ProductID,
		"source_category_id":   cmd.CategoryID,
		"derived_product_id":   result.DerivedProductID,
		"derived_category_id":  result.DerivedCategoryID,
		"used_public_product":  result.UsedPublicProduct,
		"used_public_category": result.UsedPublicCategory,
		"target_product_id":    targetProductID,
		"target_category_id":   targetCategoryID,
		"target_product_owner": targetProductCustomerID,
	}); err != nil {
		return catalogapp.AssignProductCategoryResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.AssignProductCategoryResult{}, err
	}
	return result, nil
}

func (r Repository) DeriveProductCategory(ctx context.Context, cmd catalogapp.DeriveProductCategoryCommand) (catalogapp.ProductCategory, error) {
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
	if err := ensureCustomerExistsTx(ctx, tx, r.schema, cmd.CustomerID); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	row, err := deriveProductCategoryTx(ctx, tx, r.schema, cmd)
	if err != nil {
		return catalogapp.ProductCategory{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	return row, nil
}

func (r Repository) DeriveCustomerProduct(ctx context.Context, cmd catalogapp.DeriveCustomerProductCommand) (catalogapp.Product, error) {
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
	if err := ensureCustomerExistsTx(ctx, tx, r.schema, cmd.CustomerID); err != nil {
		return catalogapp.Product{}, err
	}
	product, err := deriveCustomerProductTx(ctx, tx, r.schema, cmd)
	if err != nil {
		return catalogapp.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.Product{}, err
	}
	return product, nil
}

func (r Repository) DeriveGradientTemplate(ctx context.Context, cmd catalogapp.DeriveGradientTemplateCommand) (catalogapp.GradientTemplate, error) {
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
	if err := ensureCustomerExistsTx(ctx, tx, r.schema, cmd.CustomerID); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	row, err := deriveGradientTemplateTx(ctx, tx, r.schema, cmd)
	if err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	return row, nil
}

func ensureCustomerExistsTx(ctx context.Context, tx pgx.Tx, schema string, customerID int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, schema), customerID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("customer not found")
	}
	return nil
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func fetchProductUnitTemplateTx(ctx context.Context, q queryRower, schema string, id int64) (catalogapp.ProductUnitTemplate, error) {
	var row catalogapp.ProductUnitTemplate
	if err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, name, inventory_unit, quote_unit, order_unit,
		       COALESCE(unit_conversion_json::text,'{}'), integer_unit, active
		FROM %s.product_unit_templates
		WHERE id=$1
	`, schema), id).Scan(&row.ID, &row.Name, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.Active); err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	return row, nil
}

func fetchProductConfigTemplateTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (catalogapp.ProductConfigTemplate, error) {
	var row catalogapp.ProductConfigTemplate
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(customer_id,0), COALESCE(source_template_id,0),
		       COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END),
		       name, COALESCE(gradient_template_id,0), COALESCE(operation_template_id,0),
		       COALESCE(unit_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
		       COALESCE(special_attrs_schema_json::text,'[]'),
		       COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'),
		       COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false), active
		FROM %s.product_config_templates
		WHERE id=$1
	`, schema), id).Scan(&row.ID, &row.CustomerID, &row.SourceTemplateID, &row.TemplateState, &row.Name, &row.GradientTemplateID, &row.OperationTemplateID, &row.UnitTemplateID, &row.PriceListRuleJSON, &row.SpecialAttrsSchemaJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.Active); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	return row, nil
}

func findProductConfigTemplateBySourceTx(ctx context.Context, tx pgx.Tx, schema string, customerID, sourceTemplateID int64) (catalogapp.ProductConfigTemplate, bool, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.product_config_templates
		WHERE active=true AND customer_id=$1 AND source_template_id=$2
		ORDER BY id
		LIMIT 1
	`, schema), customerID, sourceTemplateID).Scan(&id)
	if err == pgx.ErrNoRows {
		return catalogapp.ProductConfigTemplate{}, false, nil
	}
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, false, err
	}
	row, err := fetchProductConfigTemplateTx(ctx, tx, schema, id)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, false, err
	}
	return row, true, nil
}

func deriveProductConfigTemplateTx(ctx context.Context, tx pgx.Tx, schema string, cmd catalogapp.DeriveProductConfigTemplateCommand) (catalogapp.ProductConfigTemplate, error) {
	if existing, ok, err := findProductConfigTemplateBySourceTx(ctx, tx, schema, cmd.CustomerID, cmd.SourceTemplateID); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	} else if ok {
		return existing, nil
	}
	source, err := fetchProductConfigTemplateTx(ctx, tx, schema, cmd.SourceTemplateID)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	if source.CustomerID != 0 {
		return catalogapp.ProductConfigTemplate{}, fmt.Errorf("source product config template must be public")
	}
	if !source.Active {
		return catalogapp.ProductConfigTemplate{}, fmt.Errorf("source product config template inactive")
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		name = source.Name
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_config_templates(customer_id, source_template_id, template_state, name, gradient_template_id, operation_template_id, unit_template_id, price_list_rule_json, inventory_unit, quote_unit, order_unit, unit_conversion_json, integer_unit, active, special_attrs_schema_json)
		VALUES($1,$2,'derived_from_public',$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11::jsonb,$12,true,$13::jsonb)
		RETURNING id
	`, schema), cmd.CustomerID, source.ID, name, source.GradientTemplateID, source.OperationTemplateID, source.UnitTemplateID, source.PriceListRuleJSON, source.InventoryUnit, source.QuoteUnit, source.OrderUnit, source.UnitConversionJSON, source.IntegerUnit, source.SpecialAttrsSchemaJSON).Scan(&id); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, schema, cmd.Actor, "product_config_template", &id, "derive_public_product_config_template", postgresinfra.StrPtr("source_template_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", source.ID)), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "source_template_id": source.ID}); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	return fetchProductConfigTemplateTx(ctx, tx, schema, id)
}

func materializeProductConfigTemplateToCategoriesTx(ctx context.Context, tx pgx.Tx, schema string, template catalogapp.ProductConfigTemplate) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_categories
		SET gradient_template_id=$2,
		    operation_template_id=$3,
		    price_list_rule_json=$4::jsonb,
		    inventory_unit=$5,
		    quote_unit=$6,
		    order_unit=$7,
		    unit_conversion_json=$8::jsonb,
		    integer_unit=$9,
		    updated_at=now()
		WHERE active=true AND product_config_template_id=$1
	`, schema), template.ID, template.GradientTemplateID, template.OperationTemplateID, template.PriceListRuleJSON, template.InventoryUnit, template.QuoteUnit, template.OrderUnit, template.UnitConversionJSON, template.IntegerUnit)
	return err
}

func resolveProductCategoryConfigTemplateTx(ctx context.Context, tx pgx.Tx, schema string, cmd catalogapp.SaveProductCategoryCommand, customerID int64, parentID int64) (catalogapp.SaveProductCategoryCommand, error) {
	if parentID <= 0 {
		cmd.ProductConfigTemplateID = 0
		return cmd, nil
	}
	if cmd.ProductConfigTemplateID <= 0 {
		return cmd, nil
	}
	template, err := fetchProductConfigTemplateTx(ctx, tx, schema, cmd.ProductConfigTemplateID)
	if err != nil {
		return catalogapp.SaveProductCategoryCommand{}, err
	}
	if !template.Active {
		return catalogapp.SaveProductCategoryCommand{}, fmt.Errorf("product config template inactive")
	}
	target := template
	switch {
	case customerID > 0 && template.CustomerID == 0:
		target, err = deriveProductConfigTemplateTx(ctx, tx, schema, catalogapp.DeriveProductConfigTemplateCommand{
			Actor:            cmd.Actor,
			CustomerID:       customerID,
			SourceTemplateID: template.ID,
		})
		if err != nil {
			return catalogapp.SaveProductCategoryCommand{}, err
		}
	case template.CustomerID != customerID:
		return catalogapp.SaveProductCategoryCommand{}, fmt.Errorf("product config template customer mismatch")
	}
	cmd.ProductConfigTemplateID = target.ID
	cmd.GradientTemplateID = target.GradientTemplateID
	cmd.OperationTemplateID = target.OperationTemplateID
	cmd.PriceListRuleJSON = target.PriceListRuleJSON
	cmd.InventoryUnit = target.InventoryUnit
	cmd.QuoteUnit = target.QuoteUnit
	cmd.OrderUnit = target.OrderUnit
	cmd.UnitConversionJSON = target.UnitConversionJSON
	cmd.IntegerUnit = target.IntegerUnit
	return cmd, nil
}

func fetchProductAssignmentOwnerTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (int64, string, error) {
	var customerID int64
	var name string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0), name FROM %s.products WHERE id=$1 AND active=true`, schema), productID).Scan(&customerID, &name)
	return customerID, name, err
}

func fetchProductCategoryTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (catalogapp.ProductCategory, error) {
	var row catalogapp.ProductCategory
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position,
		       COALESCE(gradient_template_id,0),
		       COALESCE(product_config_template_id,0),
		       COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
		       COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'),
		       COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false),
		       COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END)
		FROM %s.product_categories
		WHERE id=$1 AND active=true
	`, schema), id).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	return row, nil
}

func findProductCategoryBySourceTx(ctx context.Context, tx pgx.Tx, schema string, customerID, sourceCategoryID int64) (catalogapp.ProductCategory, bool, error) {
	var row catalogapp.ProductCategory
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position,
		       COALESCE(gradient_template_id,0),
		       COALESCE(product_config_template_id,0),
		       COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
		       COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'),
		       COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false),
		       COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END)
		FROM %s.product_categories
		WHERE active=true AND customer_id=$1 AND source_category_id=$2
		ORDER BY id
		LIMIT 1
	`, schema), customerID, sourceCategoryID).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState)
	if err == pgx.ErrNoRows {
		return catalogapp.ProductCategory{}, false, nil
	}
	if err != nil {
		return catalogapp.ProductCategory{}, false, err
	}
	return row, true, nil
}

func findProductCategoryByNameTx(ctx context.Context, tx pgx.Tx, schema string, customerID, parentID int64, name string) (catalogapp.ProductCategory, bool, error) {
	var row catalogapp.ProductCategory
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position,
		       COALESCE(gradient_template_id,0),
		       COALESCE(product_config_template_id,0),
		       COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
		       COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'),
		       COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false),
		       COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END)
		FROM %s.product_categories
		WHERE active=true AND customer_id=$1 AND COALESCE(parent_id,0)=$2 AND lower(name)=lower($3)
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, schema), customerID, parentID, strings.TrimSpace(name)).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState)
	if err == pgx.ErrNoRows {
		return catalogapp.ProductCategory{}, false, nil
	}
	if err != nil {
		return catalogapp.ProductCategory{}, false, err
	}
	return row, true, nil
}

func deriveProductCategoryTx(ctx context.Context, tx pgx.Tx, schema string, cmd catalogapp.DeriveProductCategoryCommand) (catalogapp.ProductCategory, error) {
	if existing, ok, err := findProductCategoryBySourceTx(ctx, tx, schema, cmd.CustomerID, cmd.SourceCategoryID); err != nil {
		return catalogapp.ProductCategory{}, err
	} else if ok {
		return existing, nil
	}
	source, err := fetchProductCategoryTx(ctx, tx, schema, cmd.SourceCategoryID)
	if err != nil {
		return catalogapp.ProductCategory{}, err
	}
	if source.CustomerID != 0 {
		return catalogapp.ProductCategory{}, fmt.Errorf("source category must be public")
	}
	parentID := int64(0)
	if source.ParentID > 0 {
		parent, err := deriveProductCategoryTx(ctx, tx, schema, catalogapp.DeriveProductCategoryCommand{
			Actor:            cmd.Actor,
			CustomerID:       cmd.CustomerID,
			SourceCategoryID: source.ParentID,
		})
		if err != nil {
			return catalogapp.ProductCategory{}, err
		}
		parentID = parent.ID
	}
	position := source.Position
	if position <= 0 {
		position = 9999
	}
	productConfigTemplateID := source.ProductConfigTemplateID
	gradientTemplateID := source.GradientTemplateID
	operationTemplateID := source.OperationTemplateID
	priceListRuleJSON := source.PriceListRuleJSON
	inventoryUnit := source.InventoryUnit
	quoteUnit := source.QuoteUnit
	orderUnit := source.OrderUnit
	unitConversionJSON := source.UnitConversionJSON
	integerUnit := source.IntegerUnit
	if source.ProductConfigTemplateID > 0 {
		configTemplate, err := deriveProductConfigTemplateTx(ctx, tx, schema, catalogapp.DeriveProductConfigTemplateCommand{
			Actor:            cmd.Actor,
			CustomerID:       cmd.CustomerID,
			SourceTemplateID: source.ProductConfigTemplateID,
		})
		if err != nil {
			return catalogapp.ProductCategory{}, err
		}
		productConfigTemplateID = configTemplate.ID
		gradientTemplateID = configTemplate.GradientTemplateID
		operationTemplateID = configTemplate.OperationTemplateID
		priceListRuleJSON = configTemplate.PriceListRuleJSON
		inventoryUnit = configTemplate.InventoryUnit
		quoteUnit = configTemplate.QuoteUnit
		orderUnit = configTemplate.OrderUnit
		unitConversionJSON = configTemplate.UnitConversionJSON
		integerUnit = configTemplate.IntegerUnit
	}
	if row, ok, err := findProductCategoryByNameTx(ctx, tx, schema, cmd.CustomerID, parentID, source.Name); err != nil {
		return catalogapp.ProductCategory{}, err
	} else if ok {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_categories
			SET source_category_id=$2, template_state='derived_from_public', gradient_template_id=$3,
			    operation_template_id=$4, price_list_rule_json=$5::jsonb,
			    inventory_unit=$6, quote_unit=$7, order_unit=$8, unit_conversion_json=$9::jsonb, integer_unit=$10,
			    product_config_template_id=$11,
			    updated_at=now()
			WHERE id=$1
			RETURNING id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position, COALESCE(gradient_template_id,0),
			          COALESCE(product_config_template_id,0),
			          COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
			          COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'), COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false), template_state
		`, schema), row.ID, source.ID, gradientTemplateID, operationTemplateID, priceListRuleJSON, inventoryUnit, quoteUnit, orderUnit, unitConversionJSON, integerUnit, productConfigTemplateID).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState); err != nil {
			return catalogapp.ProductCategory{}, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, cmd.Actor, "product_category", &row.ID, "derive_public_category", postgresinfra.StrPtr("source_category_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", source.ID)), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "source_category_id": source.ID, "parent_id": parentID}); err != nil {
			return catalogapp.ProductCategory{}, err
		}
		return row, nil
	}
	var row catalogapp.ProductCategory
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_categories(parent_id, customer_id, source_category_id, name, level, position, gradient_template_id, operation_template_id, price_list_rule_json, inventory_unit, quote_unit, order_unit, unit_conversion_json, integer_unit, product_config_template_id, template_state, active)
		VALUES(NULLIF($1,0), $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13::jsonb, $14, $15, 'derived_from_public', true)
		RETURNING id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position, COALESCE(gradient_template_id,0),
		          COALESCE(product_config_template_id,0),
		          COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
		          COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'), COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false), template_state
	`, schema), parentID, cmd.CustomerID, source.ID, source.Name, source.Level, position, gradientTemplateID, operationTemplateID, priceListRuleJSON, inventoryUnit, quoteUnit, orderUnit, unitConversionJSON, integerUnit, productConfigTemplateID).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	if err := normalizeCategoryPositions(ctx, tx, schema, parentID, cmd.CustomerID); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, schema, cmd.Actor, "product_category", &row.ID, "derive_public_category", postgresinfra.StrPtr("source_category_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", source.ID)), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "source_category_id": source.ID, "parent_id": parentID}); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	return row, nil
}

func ensureProductCategoryForTargetTx(ctx context.Context, tx pgx.Tx, schema string, actor string, targetCustomerID int64, sourceCategoryID int64) (catalogapp.ProductCategory, error) {
	source, err := fetchProductCategoryTx(ctx, tx, schema, sourceCategoryID)
	if err != nil {
		return catalogapp.ProductCategory{}, err
	}
	if source.CustomerID == targetCustomerID {
		return source, nil
	}
	if targetCustomerID > 0 && source.CustomerID == 0 {
		return deriveProductCategoryTx(ctx, tx, schema, catalogapp.DeriveProductCategoryCommand{
			Actor:            actor,
			CustomerID:       targetCustomerID,
			SourceCategoryID: source.ID,
		})
	}

	parentID := int64(0)
	if source.ParentID > 0 {
		parent, err := ensureProductCategoryForTargetTx(ctx, tx, schema, actor, targetCustomerID, source.ParentID)
		if err != nil {
			return catalogapp.ProductCategory{}, err
		}
		parentID = parent.ID
	}
	position := source.Position
	if position <= 0 {
		position = 9999
	}
	sourceCategoryIDForTarget := int64(0)
	templateState := catalogapp.TemplateStateCustomer
	if targetCustomerID == 0 {
		templateState = catalogapp.TemplateStatePublic
	} else if source.SourceCategoryID > 0 {
		sourceCategoryIDForTarget = source.SourceCategoryID
		templateState = catalogapp.TemplateStateDerived
	}
	if sourceCategoryIDForTarget > 0 {
		if existing, ok, err := findProductCategoryBySourceTx(ctx, tx, schema, targetCustomerID, sourceCategoryIDForTarget); err != nil {
			return catalogapp.ProductCategory{}, err
		} else if ok {
			return updateCopiedProductCategoryTx(ctx, tx, schema, actor, existing.ID, source, targetCustomerID, parentID, sourceCategoryIDForTarget, templateState)
		}
	}
	if existing, ok, err := findProductCategoryByNameTx(ctx, tx, schema, targetCustomerID, parentID, source.Name); err != nil {
		return catalogapp.ProductCategory{}, err
	} else if ok {
		return updateCopiedProductCategoryTx(ctx, tx, schema, actor, existing.ID, source, targetCustomerID, parentID, sourceCategoryIDForTarget, templateState)
	}

	source, err = resolveCopiedCategoryConfigForTargetTx(ctx, tx, schema, actor, targetCustomerID, source)
	if err != nil {
		return catalogapp.ProductCategory{}, err
	}
	var row catalogapp.ProductCategory
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_categories(parent_id, customer_id, source_category_id, name, level, position,
			gradient_template_id, operation_template_id, price_list_rule_json,
			inventory_unit, quote_unit, order_unit, unit_conversion_json, integer_unit,
			product_config_template_id, template_state, active, updated_at)
		VALUES(NULLIF($1,0),$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12,$13::jsonb,$14,$15,$16,true,now())
		RETURNING id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position,
		          COALESCE(gradient_template_id,0), COALESCE(product_config_template_id,0),
		          COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
		          COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'),
		          COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false), template_state
	`, schema), parentID, targetCustomerID, sourceCategoryIDForTarget, source.Name, source.Level, position, source.GradientTemplateID, source.OperationTemplateID, source.PriceListRuleJSON, source.InventoryUnit, source.QuoteUnit, source.OrderUnit, source.UnitConversionJSON, source.IntegerUnit, source.ProductConfigTemplateID, templateState).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	if err := normalizeCategoryPositions(ctx, tx, schema, parentID, targetCustomerID); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	return row, nil
}

func updateCopiedProductCategoryTx(ctx context.Context, tx pgx.Tx, schema string, actor string, categoryID int64, source catalogapp.ProductCategory, targetCustomerID int64, parentID int64, sourceCategoryIDForTarget int64, templateState string) (catalogapp.ProductCategory, error) {
	var err error
	source, err = resolveCopiedCategoryConfigForTargetTx(ctx, tx, schema, actor, targetCustomerID, source)
	if err != nil {
		return catalogapp.ProductCategory{}, err
	}
	var row catalogapp.ProductCategory
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.product_categories
		SET parent_id=NULLIF($2,0), customer_id=$3, source_category_id=$4, name=$5, level=$6,
		    gradient_template_id=$7, operation_template_id=$8, price_list_rule_json=$9::jsonb,
		    inventory_unit=$10, quote_unit=$11, order_unit=$12, unit_conversion_json=$13::jsonb, integer_unit=$14,
		    product_config_template_id=$15, template_state=$16, active=true, updated_at=now()
		WHERE id=$1
		RETURNING id, COALESCE(parent_id,0), COALESCE(customer_id,0), COALESCE(source_category_id,0), name, level, position,
		          COALESCE(gradient_template_id,0), COALESCE(product_config_template_id,0),
		          COALESCE(operation_template_id,0), COALESCE(price_list_rule_json::text,'{}'),
		          COALESCE(inventory_unit,'kg'), COALESCE(quote_unit,'kg'), COALESCE(order_unit,'kg'),
		          COALESCE(unit_conversion_json::text,'{}'), COALESCE(integer_unit,false), template_state
	`, schema), categoryID, parentID, targetCustomerID, sourceCategoryIDForTarget, source.Name, source.Level, source.GradientTemplateID, source.OperationTemplateID, source.PriceListRuleJSON, source.InventoryUnit, source.QuoteUnit, source.OrderUnit, source.UnitConversionJSON, source.IntegerUnit, source.ProductConfigTemplateID, templateState).Scan(&row.ID, &row.ParentID, &row.CustomerID, &row.SourceCategoryID, &row.Name, &row.Level, &row.Position, &row.GradientTemplateID, &row.ProductConfigTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.TemplateState); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	if err := normalizeCategoryPositions(ctx, tx, schema, parentID, targetCustomerID); err != nil {
		return catalogapp.ProductCategory{}, err
	}
	return row, nil
}

func resolveCopiedCategoryConfigForTargetTx(ctx context.Context, tx pgx.Tx, schema string, actor string, targetCustomerID int64, source catalogapp.ProductCategory) (catalogapp.ProductCategory, error) {
	if source.ProductConfigTemplateID <= 0 {
		return source, nil
	}
	template, err := ensureProductConfigTemplateForTargetTx(ctx, tx, schema, actor, targetCustomerID, source.ProductConfigTemplateID)
	if err != nil {
		return catalogapp.ProductCategory{}, err
	}
	source.ProductConfigTemplateID = template.ID
	source.GradientTemplateID = template.GradientTemplateID
	source.OperationTemplateID = template.OperationTemplateID
	source.PriceListRuleJSON = template.PriceListRuleJSON
	source.InventoryUnit = template.InventoryUnit
	source.QuoteUnit = template.QuoteUnit
	source.OrderUnit = template.OrderUnit
	source.UnitConversionJSON = template.UnitConversionJSON
	source.IntegerUnit = template.IntegerUnit
	return source, nil
}

func findEquivalentCategoryForTargetTx(ctx context.Context, tx pgx.Tx, schema string, targetCustomerID int64, sourceCategoryID int64) (int64, error) {
	if sourceCategoryID <= 0 {
		return 0, nil
	}
	source, err := fetchProductCategoryTx(ctx, tx, schema, sourceCategoryID)
	if err != nil {
		return -1, err
	}
	if source.CustomerID == targetCustomerID {
		return source.ID, nil
	}
	sourceCategoryIDForTarget := int64(0)
	if targetCustomerID > 0 {
		if source.CustomerID == 0 {
			sourceCategoryIDForTarget = source.ID
		} else if source.SourceCategoryID > 0 {
			sourceCategoryIDForTarget = source.SourceCategoryID
		}
		if sourceCategoryIDForTarget > 0 {
			if existing, ok, err := findProductCategoryBySourceTx(ctx, tx, schema, targetCustomerID, sourceCategoryIDForTarget); err != nil {
				return -1, err
			} else if ok {
				return existing.ID, nil
			}
		}
	} else if source.SourceCategoryID > 0 {
		if publicSource, err := fetchProductCategoryTx(ctx, tx, schema, source.SourceCategoryID); err == nil && publicSource.CustomerID == 0 {
			return publicSource.ID, nil
		}
	}
	parentID := int64(0)
	if source.ParentID > 0 {
		resolvedParentID, err := findEquivalentCategoryForTargetTx(ctx, tx, schema, targetCustomerID, source.ParentID)
		if err != nil {
			return -1, err
		}
		if resolvedParentID < 0 {
			return -1, nil
		}
		parentID = resolvedParentID
	}
	if existing, ok, err := findProductCategoryByNameTx(ctx, tx, schema, targetCustomerID, parentID, source.Name); err != nil {
		return -1, err
	} else if ok {
		return existing.ID, nil
	}
	return -1, nil
}

func findProductConfigTemplateByNameTx(ctx context.Context, tx pgx.Tx, schema string, customerID int64, name string) (catalogapp.ProductConfigTemplate, bool, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.product_config_templates
		WHERE active=true AND customer_id=$1 AND lower(name)=lower($2)
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, schema), customerID, strings.TrimSpace(name)).Scan(&id)
	if err == pgx.ErrNoRows {
		return catalogapp.ProductConfigTemplate{}, false, nil
	}
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, false, err
	}
	row, err := fetchProductConfigTemplateTx(ctx, tx, schema, id)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, false, err
	}
	return row, true, nil
}

func ensureProductConfigTemplateForTargetTx(ctx context.Context, tx pgx.Tx, schema string, actor string, targetCustomerID int64, sourceTemplateID int64) (catalogapp.ProductConfigTemplate, error) {
	source, err := fetchProductConfigTemplateTx(ctx, tx, schema, sourceTemplateID)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	if source.CustomerID == targetCustomerID {
		return source, nil
	}
	if targetCustomerID > 0 && source.CustomerID == 0 {
		return deriveProductConfigTemplateTx(ctx, tx, schema, catalogapp.DeriveProductConfigTemplateCommand{
			Actor:            actor,
			CustomerID:       targetCustomerID,
			SourceTemplateID: source.ID,
		})
	}
	gradientTemplateID, err := ensureGradientTemplateForTargetTx(ctx, tx, schema, actor, targetCustomerID, source.GradientTemplateID)
	if err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	sourceTemplateIDForTarget := int64(0)
	templateState := catalogapp.TemplateStateCustomer
	if targetCustomerID == 0 {
		templateState = catalogapp.TemplateStatePublic
	} else if source.SourceTemplateID > 0 {
		sourceTemplateIDForTarget = source.SourceTemplateID
		templateState = catalogapp.TemplateStateDerived
	}
	if sourceTemplateIDForTarget > 0 {
		if existing, ok, err := findProductConfigTemplateBySourceTx(ctx, tx, schema, targetCustomerID, sourceTemplateIDForTarget); err != nil {
			return catalogapp.ProductConfigTemplate{}, err
		} else if ok {
			return updateCopiedProductConfigTemplateTx(ctx, tx, schema, existing.ID, source, targetCustomerID, sourceTemplateIDForTarget, templateState, gradientTemplateID)
		}
	}
	if existing, ok, err := findProductConfigTemplateByNameTx(ctx, tx, schema, targetCustomerID, source.Name); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	} else if ok {
		return updateCopiedProductConfigTemplateTx(ctx, tx, schema, existing.ID, source, targetCustomerID, sourceTemplateIDForTarget, templateState, gradientTemplateID)
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_config_templates(customer_id, source_template_id, template_state, name,
			gradient_template_id, operation_template_id, unit_template_id, price_list_rule_json,
			inventory_unit, quote_unit, order_unit, unit_conversion_json, integer_unit, active, special_attrs_schema_json)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12::jsonb,$13,true,$14::jsonb)
		RETURNING id
	`, schema), targetCustomerID, sourceTemplateIDForTarget, templateState, source.Name, gradientTemplateID, source.OperationTemplateID, source.UnitTemplateID, source.PriceListRuleJSON, source.InventoryUnit, source.QuoteUnit, source.OrderUnit, source.UnitConversionJSON, source.IntegerUnit, source.SpecialAttrsSchemaJSON).Scan(&id); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	return fetchProductConfigTemplateTx(ctx, tx, schema, id)
}

func updateCopiedProductConfigTemplateTx(ctx context.Context, tx pgx.Tx, schema string, id int64, source catalogapp.ProductConfigTemplate, targetCustomerID int64, sourceTemplateIDForTarget int64, templateState string, gradientTemplateID int64) (catalogapp.ProductConfigTemplate, error) {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_config_templates
		SET customer_id=$2, source_template_id=$3, template_state=$4, name=$5,
		    gradient_template_id=$6, operation_template_id=$7, unit_template_id=$8, price_list_rule_json=$9::jsonb,
		    inventory_unit=$10, quote_unit=$11, order_unit=$12, unit_conversion_json=$13::jsonb, integer_unit=$14,
		    active=true, special_attrs_schema_json=$15::jsonb, updated_at=now()
		WHERE id=$1
	`, schema), id, targetCustomerID, sourceTemplateIDForTarget, templateState, source.Name, gradientTemplateID, source.OperationTemplateID, source.UnitTemplateID, source.PriceListRuleJSON, source.InventoryUnit, source.QuoteUnit, source.OrderUnit, source.UnitConversionJSON, source.IntegerUnit, source.SpecialAttrsSchemaJSON); err != nil {
		return catalogapp.ProductConfigTemplate{}, err
	}
	return fetchProductConfigTemplateTx(ctx, tx, schema, id)
}

func findGradientTemplateByNameTx(ctx context.Context, tx pgx.Tx, schema string, customerID int64, name string) (catalogapp.GradientTemplate, bool, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.pricing_gradient_templates
		WHERE active=true AND customer_id=$1 AND lower(name)=lower($2)
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, schema), customerID, strings.TrimSpace(name)).Scan(&id)
	if err == pgx.ErrNoRows {
		return catalogapp.GradientTemplate{}, false, nil
	}
	if err != nil {
		return catalogapp.GradientTemplate{}, false, err
	}
	row, err := fetchGradientTemplateTx(ctx, tx, schema, id)
	if err != nil {
		return catalogapp.GradientTemplate{}, false, err
	}
	return row, true, nil
}

func ensureGradientTemplateForTargetTx(ctx context.Context, tx pgx.Tx, schema string, actor string, targetCustomerID int64, sourceTemplateID int64) (int64, error) {
	if sourceTemplateID <= 0 {
		return 0, nil
	}
	source, err := fetchGradientTemplateTx(ctx, tx, schema, sourceTemplateID)
	if err != nil {
		return 0, err
	}
	if source.CustomerID == targetCustomerID {
		return source.ID, nil
	}
	if targetCustomerID > 0 && source.CustomerID == 0 {
		derived, err := deriveGradientTemplateTx(ctx, tx, schema, catalogapp.DeriveGradientTemplateCommand{
			Actor:            actor,
			CustomerID:       targetCustomerID,
			SourceTemplateID: source.ID,
		})
		if err != nil {
			return 0, err
		}
		return derived.ID, nil
	}
	sourceTemplateIDForTarget := int64(0)
	templateState := catalogapp.TemplateStateCustomer
	if targetCustomerID == 0 {
		templateState = catalogapp.TemplateStatePublic
	} else if source.SourceTemplateID > 0 {
		sourceTemplateIDForTarget = source.SourceTemplateID
		templateState = catalogapp.TemplateStateDerived
	}
	if sourceTemplateIDForTarget > 0 {
		if existing, ok, err := findGradientTemplateBySourceTx(ctx, tx, schema, targetCustomerID, sourceTemplateIDForTarget); err != nil {
			return 0, err
		} else if ok {
			if err := overwriteGradientTemplateTx(ctx, tx, schema, existing.ID, source, targetCustomerID, sourceTemplateIDForTarget, templateState); err != nil {
				return 0, err
			}
			return existing.ID, nil
		}
	}
	if existing, ok, err := findGradientTemplateByNameTx(ctx, tx, schema, targetCustomerID, source.Name); err != nil {
		return 0, err
	} else if ok {
		if err := overwriteGradientTemplateTx(ctx, tx, schema, existing.ID, source, targetCustomerID, sourceTemplateIDForTarget, templateState); err != nil {
			return 0, err
		}
		return existing.ID, nil
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.pricing_gradient_templates(name, display_unit, unit_template_id, customer_id, source_template_id, template_state, active)
		VALUES($1,$2,$3,$4,$5,$6,true)
		RETURNING id
	`, schema), source.Name, source.DisplayUnit, source.UnitTemplateID, targetCustomerID, sourceTemplateIDForTarget, templateState).Scan(&id); err != nil {
		return 0, err
	}
	if err := copyGradientTemplateTiersTx(ctx, tx, schema, source.ID, id); err != nil {
		return 0, err
	}
	return id, nil
}

func overwriteGradientTemplateTx(ctx context.Context, tx pgx.Tx, schema string, targetID int64, source catalogapp.GradientTemplate, targetCustomerID int64, sourceTemplateIDForTarget int64, templateState string) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.pricing_gradient_templates
		SET name=$2, display_unit=$3, unit_template_id=$4, customer_id=$5, source_template_id=$6,
		    template_state=$7, active=true, updated_at=now()
		WHERE id=$1
	`, schema), targetID, source.Name, source.DisplayUnit, source.UnitTemplateID, targetCustomerID, sourceTemplateIDForTarget, templateState); err != nil {
		return err
	}
	return copyGradientTemplateTiersTx(ctx, tx, schema, source.ID, targetID)
}

func copyGradientTemplateTiersTx(ctx context.Context, tx pgx.Tx, schema string, sourceID int64, targetID int64) error {
	if sourceID == targetID {
		return nil
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.pricing_gradient_template_tiers SET active=false, updated_at=now() WHERE template_id=$1`, schema), targetID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.pricing_gradient_template_tiers(template_id,label,min_weight_g,max_weight_g,margin_rate,position,active)
		SELECT $1,label,min_weight_g,max_weight_g,margin_rate,position,true
		FROM %s.pricing_gradient_template_tiers
		WHERE template_id=$2 AND active=true
		ORDER BY position, id
	`, schema, schema), targetID, sourceID)
	return err
}

func findTargetSKUByNameTx(ctx context.Context, tx pgx.Tx, schema string, targetCustomerID int64, targetCategoryID int64, name string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.products
		WHERE COALESCE(customer_id,0)=$1
		  AND COALESCE(product_category_id,0)=$2
		  AND lower(name)=lower($3)
		ORDER BY active DESC, id
		LIMIT 1
		FOR UPDATE
	`, schema), targetCustomerID, targetCategoryID, strings.TrimSpace(name)).Scan(&id)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

func fetchSKUCopySourceTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (catalogapp.Product, error) {
	var p catalogapp.Product
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id, name, COALESCE(remark,''), COALESCE(roast_level,''), COALESCE(special_attrs_json::text,'{}'), default_price,
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
		COALESCE(gradient_template_id_override,0),
		COALESCE(operation_template_id_override,0),
		COALESCE(unit_rule_override_json::text,'{}'),
		COALESCE(active,true),
		COALESCE((SELECT COUNT(*) FROM %[1]s.product_bom_items bi WHERE bi.product_id=products.id),0),
		COALESCE((SELECT NULLIF(status,'') FROM %[1]s.product_bom WHERE product_id=products.id), 'missing')
		FROM %[1]s.products WHERE id=$1
		FOR UPDATE`, schema), productID).Scan(&p.ID, &p.Name, &p.Remark, &p.RoastLevel, &p.SpecialAttrsJSON, &p.DefaultPrice, &p.ProductKind, &p.GreenBeanType, &p.GreenBeanBomProductID, &p.DripBagGrams, &p.DripBoxBagCount, &p.AllowFulfillmentOrder, &p.AllowMallOrder, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.YieldRate, &p.ProductCategoryID, &p.ProductCategoryPosition, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.MarginRateOverride, &p.GradientTemplateIDOverride, &p.OperationTemplateIDOverride, &p.UnitRuleOverrideJSON, &p.Active, &p.BomItemCount, &p.BomStatus)
	if err != nil {
		return catalogapp.Product{}, err
	}
	p.ProductKind = catalogdomain.NormalizeProductKind(p.ProductKind)
	if p.ProductKind == catalogdomain.ProductKindDripBag {
		p.SalesUnits = []string{"bag", "box"}
	}
	if !catalogdomain.ProductKindSupportsBomParams(p.ProductKind) {
		p.RoastLevel = ""
		p.YieldRate = 0
	}
	return p, nil
}

func updateCopiedSKUProductTx(ctx context.Context, tx pgx.Tx, schema string, targetID int64, source catalogapp.Product, targetCustomerID int64, targetCategoryID int64, visibility string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.products
		SET name=$2, remark=$3, product_kind=$4, roast_level=$5, default_price=$6, active=true,
		    retail_price_100g=$7, retail_price_200g=$8, retail_price_227g=$9, retail_price_250g=$10,
		    drip_bag_grams=$11, drip_box_bag_count=$12, allow_fulfillment_order=$13, allow_mall_order=$14,
		    product_category_id=NULLIF($15,0), customer_id=$16, base_product_id=0, visibility=$17, custom_type='',
		    green_bean_type=$18, green_bean_bom_product_id=$19, special_attrs_json=$20::jsonb,
		    margin_rate_override=$21, gradient_template_id_override=$22, operation_template_id_override=$23,
		    unit_rule_override_json=$24::jsonb
		WHERE id=$1
	`, schema), targetID, source.Name, source.Remark, source.ProductKind, source.RoastLevel, source.DefaultPrice, source.RetailPrice100G, source.RetailPrice200G, source.RetailPrice227G, source.RetailPrice250G, source.DripBagGrams, source.DripBoxBagCount, source.AllowFulfillmentOrder, source.AllowMallOrder, targetCategoryID, targetCustomerID, visibility, source.GreenBeanType, source.GreenBeanBomProductID, source.SpecialAttrsJSON, source.MarginRateOverride, source.GradientTemplateIDOverride, source.OperationTemplateIDOverride, source.UnitRuleOverrideJSON)
	return err
}

func insertCopiedSKUProductTx(ctx context.Context, tx pgx.Tx, schema string, source catalogapp.Product, targetCustomerID int64, targetCategoryID int64, position int, visibility string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(
			name, remark, product_kind, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,
			product_category_id, product_category_position,
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id,
			special_attrs_json, margin_rate_override, gradient_template_id_override, operation_template_id_override,
			unit_rule_override_json, created_at
		)
		VALUES($1,$2,$3,$4,$5,true,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,0),$15,$16,0,$17,'',$18,$19,$20::jsonb,$21,$22,$23,$24::jsonb,now())
		RETURNING id
	`, schema), source.Name, source.Remark, source.ProductKind, source.RoastLevel, source.DefaultPrice, source.RetailPrice100G, source.RetailPrice200G, source.RetailPrice227G, source.RetailPrice250G, source.DripBagGrams, source.DripBoxBagCount, source.AllowFulfillmentOrder, source.AllowMallOrder, targetCategoryID, position, targetCustomerID, visibility, source.GreenBeanType, source.GreenBeanBomProductID, source.SpecialAttrsJSON, source.MarginRateOverride, source.GradientTemplateIDOverride, source.OperationTemplateIDOverride, source.UnitRuleOverrideJSON).Scan(&id)
	return id, err
}

func updateCopiedSKUGreenBeanReferenceTx(ctx context.Context, tx pgx.Tx, schema string, targetProductID int64, greenBeanBOMProductID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET green_bean_bom_product_id=$2 WHERE id=$1`, schema), targetProductID, greenBeanBOMProductID)
	return err
}

func resolveSKUCopyProductReferenceTx(ctx context.Context, tx pgx.Tx, schema string, actor string, targetCustomerID int64, sourceProductID int64, sourceToTarget map[int64]int64) (int64, error) {
	if sourceProductID <= 0 {
		return 0, nil
	}
	if targetID := sourceToTarget[sourceProductID]; targetID > 0 {
		return targetID, nil
	}
	var refID int64
	var refName string
	var refCustomerID int64
	var refCategoryID int64
	var refActive bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, name, COALESCE(customer_id,0), COALESCE(product_category_id,0), COALESCE(active,true)
		FROM %s.products
		WHERE id=$1
	`, schema), sourceProductID).Scan(&refID, &refName, &refCustomerID, &refCategoryID, &refActive)
	if err != nil {
		return 0, err
	}
	if refCustomerID == targetCustomerID {
		return refID, nil
	}
	targetCategoryID := int64(0)
	if refCategoryID > 0 {
		resolvedCategoryID, err := findEquivalentCategoryForTargetTx(ctx, tx, schema, targetCustomerID, refCategoryID)
		if err != nil {
			return 0, err
		}
		if resolvedCategoryID <= 0 {
			return 0, fmt.Errorf("copied SKU product reference %d (%s) belongs to customer %d and has no target category equivalent for customer %d", refID, refName, refCustomerID, targetCustomerID)
		}
		targetCategoryID = resolvedCategoryID
	}
	if targetID, err := findTargetSKUByNameTx(ctx, tx, schema, targetCustomerID, targetCategoryID, refName); err != nil {
		return 0, err
	} else if targetID > 0 {
		return targetID, nil
	}
	if !refActive {
		return 0, fmt.Errorf("copied SKU product reference %d (%s) belongs to customer %d and is inactive with no target equivalent for customer %d", refID, refName, refCustomerID, targetCustomerID)
	}
	if actor == "" {
		actor = "system"
	}
	return 0, fmt.Errorf("copied SKU product reference %d (%s) belongs to customer %d and has no target equivalent for customer %d", refID, refName, refCustomerID, targetCustomerID)
}

func copyProductBOMTx(ctx context.Context, tx pgx.Tx, schema string, actor string, targetCustomerID int64, sourceProductID int64, targetProductID int64, sourceToTarget map[int64]int64) error {
	if sourceProductID == targetProductID {
		return nil
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_bom_items WHERE product_id=$1`, schema), targetProductID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_bom WHERE product_id=$1`, schema), targetProductID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom(product_id,yield_rate,status,updated_at)
		SELECT $1,yield_rate,status,now()
		FROM %s.product_bom
		WHERE product_id=$2
	`, schema, schema), targetProductID, sourceProductID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT material_id,
		       COALESCE(NULLIF(component_type,''),'material'),
		       COALESCE(component_product_id,0),
		       COALESCE(component_spec_g,0),
		       COALESCE(NULLIF(consume_unit,''),'ratio_pct'),
		       COALESCE(qty_per_unit,0)::float8,
		       COALESCE(ratio_pct,0)::float8,
		       COALESCE(unit_cost_snapshot,0)::float8
		FROM %s.product_bom_items
		WHERE product_id=$1
		ORDER BY id
	`, schema), sourceProductID)
	if err != nil {
		return err
	}
	bomItems := make([]skuCopyBOMItem, 0)
	for rows.Next() {
		var item skuCopyBOMItem
		if err := rows.Scan(&item.materialID, &item.componentType, &item.componentProductID, &item.componentSpecG, &item.consumeUnit, &item.qtyPerUnit, &item.ratioPct, &item.unitCostSnapshot); err != nil {
			rows.Close()
			return err
		}
		bomItems = append(bomItems, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s.product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,now())
	`, schema)
	for _, item := range bomItems {
		componentType := strings.TrimSpace(item.componentType)
		if componentType == "" {
			componentType = "material"
		}
		componentProductID := item.componentProductID
		materialID := item.materialID
		if componentType == "finished_product" && componentProductID > 0 {
			var err error
			componentProductID, err = resolveSKUCopyProductReferenceTx(ctx, tx, schema, actor, targetCustomerID, componentProductID, sourceToTarget)
			if err != nil {
				return err
			}
			materialID = 0
		} else if componentType != "finished_product" {
			componentProductID = 0
		}
		if _, err := tx.Exec(ctx, insertSQL, targetProductID, materialID, componentType, componentProductID, item.componentSpecG, item.consumeUnit, item.qtyPerUnit, item.ratioPct, item.unitCostSnapshot); err != nil {
			return err
		}
	}
	return nil
}

func setProductBOMSourceToInheritTx(ctx context.Context, tx pgx.Tx, schema string, actor string, sourceProductID int64, targetProductID int64) error {
	if sourceProductID == targetProductID {
		return nil
	}
	var sourceName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1 AND active=true`, schema), sourceProductID).Scan(&sourceName); err != nil {
		return err
	}
	var sourceBomVersionID int64
	var sourceBomVersionNo string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(version_no,'')
		FROM %s.bom_versions
		WHERE product_id=$1 AND status='active'
		ORDER BY activated_at DESC NULLS LAST, id DESC
		LIMIT 1
	`, schema), sourceProductID).Scan(&sourceBomVersionID, &sourceBomVersionNo)
	if err == pgx.ErrNoRows {
		sourceBomVersionNo = "当前BOM"
	} else if err != nil {
		return err
	}
	if strings.TrimSpace(sourceBomVersionNo) == "" {
		sourceBomVersionNo = "当前BOM"
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_bom_items WHERE product_id=$1`, schema), targetProductID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_bom WHERE product_id=$1`, schema), targetProductID); err != nil {
		return err
	}
	sourceProductCode := fmt.Sprintf("SKU-%d", sourceProductID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_sources(
			product_id, source_type, source_product_id, source_product_code_snapshot, source_product_name_snapshot,
			source_bom_product_id, source_bom_version_id, source_bom_version_no_snapshot,
			derived_from_product_id, derived_from_bom_version_id, derived_at, derived_by, updated_at
		)
		VALUES($1,'inherit_current',$2,$3,$4,$2,$5,$6,0,0,NULL,'',now())
		ON CONFLICT (product_id) DO UPDATE SET
			source_type='inherit_current',
			source_product_id=excluded.source_product_id,
			source_product_code_snapshot=excluded.source_product_code_snapshot,
			source_product_name_snapshot=excluded.source_product_name_snapshot,
			source_bom_product_id=excluded.source_bom_product_id,
			source_bom_version_id=excluded.source_bom_version_id,
			source_bom_version_no_snapshot=excluded.source_bom_version_no_snapshot,
			derived_from_product_id=0,
			derived_from_bom_version_id=0,
			derived_at=NULL,
			derived_by='',
			updated_at=now()
	`, schema), targetProductID, sourceProductID, sourceProductCode, sourceName, sourceBomVersionID, sourceBomVersionNo); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "product_bom_source", &targetProductID, "inherit_current", postgresinfra.StrPtr("source_product_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", sourceProductID)), postgresinfra.AuditMeta{
		"target_product_id":     targetProductID,
		"source_product_id":     sourceProductID,
		"source_product_code":   sourceProductCode,
		"source_product_name":   sourceName,
		"source_bom_version_id": sourceBomVersionID,
		"source_bom_version_no": sourceBomVersionNo,
	})
}

func copyProductPriceTiersTx(ctx context.Context, tx pgx.Tx, schema string, sourceProductID int64, targetProductID int64) error {
	if sourceProductID == targetProductID {
		return nil
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_price_tiers WHERE product_id=$1`, schema), targetProductID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_price_tiers(product_id,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active,product_kind,price_basis,sales_unit,unit_bag_count,price_source_json)
		SELECT $1,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active,product_kind,price_basis,sales_unit,unit_bag_count,price_source_json
		FROM %s.product_price_tiers
		WHERE product_id=$2 AND active=true
		ORDER BY id
	`, schema, schema), targetProductID, sourceProductID)
	return err
}

type productDeriveBase struct {
	Name                  string
	Remark                string
	ProductKind           string
	GreenBeanType         string
	GreenBeanBomProductID int64
	RoastLevel            string
	SpecialAttrsJSON      string
	DripBagGrams          float64
	DripBoxBagCount       int
	AllowFulfillmentOrder bool
	AllowMallOrder        bool
	DefaultPrice          float64
	RetailPrice100G       float64
	RetailPrice200G       float64
	RetailPrice227G       float64
	RetailPrice250G       float64
	YieldRate             float64
	CustomerID            int64
}

func fetchProductDeriveBaseTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (productDeriveBase, error) {
	var base productDeriveBase
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT name, COALESCE(remark,''), COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
		       COALESCE(green_bean_type,''), COALESCE(green_bean_bom_product_id,0),
		       COALESCE(roast_level,''), COALESCE(special_attrs_json::text,'{}'), COALESCE(drip_bag_grams,10)::float8, COALESCE(drip_box_bag_count,10),
		       COALESCE(allow_fulfillment_order,true), COALESCE(allow_mall_order,false),
		       COALESCE(default_price,0), COALESCE(retail_price_100g,0), COALESCE(retail_price_200g,0),
		       COALESCE(retail_price_227g,default_price,0), COALESCE(retail_price_250g,0),
		       COALESCE((SELECT yield_rate FROM %[1]s.product_bom WHERE product_id=products.id),0.8),
		       COALESCE(customer_id,0)
		FROM %[1]s.products
		WHERE id=$1 AND active=true
	`, schema), productID).Scan(&base.Name, &base.Remark, &base.ProductKind, &base.GreenBeanType, &base.GreenBeanBomProductID, &base.RoastLevel, &base.SpecialAttrsJSON, &base.DripBagGrams, &base.DripBoxBagCount, &base.AllowFulfillmentOrder, &base.AllowMallOrder, &base.DefaultPrice, &base.RetailPrice100G, &base.RetailPrice200G, &base.RetailPrice227G, &base.RetailPrice250G, &base.YieldRate, &base.CustomerID)
	base.ProductKind = catalogdomain.NormalizeProductKind(base.ProductKind)
	return base, err
}

func fetchCatalogProductByIDTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (catalogapp.Product, error) {
	var p catalogapp.Product
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id, name, COALESCE(remark,''), COALESCE(roast_level,''), COALESCE(special_attrs_json::text,'{}'), default_price,
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
		COALESCE(gradient_template_id_override,0),
		COALESCE(operation_template_id_override,0),
		COALESCE(unit_rule_override_json::text,'{}'),
		COALESCE((SELECT COUNT(*) FROM %[1]s.product_bom_items bi WHERE bi.product_id=products.id),0),
		COALESCE((SELECT NULLIF(status,'') FROM %[1]s.product_bom WHERE product_id=products.id), 'missing')
		FROM %[1]s.products WHERE id=$1`, schema), productID).Scan(&p.ID, &p.Name, &p.Remark, &p.RoastLevel, &p.SpecialAttrsJSON, &p.DefaultPrice, &p.ProductKind, &p.GreenBeanType, &p.GreenBeanBomProductID, &p.DripBagGrams, &p.DripBoxBagCount, &p.AllowFulfillmentOrder, &p.AllowMallOrder, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.YieldRate, &p.ProductCategoryID, &p.ProductCategoryPosition, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.MarginRateOverride, &p.GradientTemplateIDOverride, &p.OperationTemplateIDOverride, &p.UnitRuleOverrideJSON, &p.BomItemCount, &p.BomStatus)
	if err != nil {
		return catalogapp.Product{}, err
	}
	p.ProductKind = catalogdomain.NormalizeProductKind(p.ProductKind)
	if p.ProductKind == catalogdomain.ProductKindDripBag {
		p.SalesUnits = []string{"bag", "box"}
	}
	if !catalogdomain.ProductKindSupportsBomParams(p.ProductKind) {
		p.RoastLevel = ""
		p.YieldRate = 0
	}
	return p, nil
}

func nextProductPositionTx(ctx context.Context, tx pgx.Tx, schema string, categoryID, customerID int64, requested int) (int, error) {
	if requested > 0 {
		return requested, nil
	}
	if categoryID <= 0 {
		return 0, nil
	}
	var maxPosition int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(MAX(product_category_position),0) FROM %s.products WHERE active=true AND COALESCE(product_category_id,0)=$1 AND COALESCE(customer_id,0)=$2`, schema), categoryID, customerID).Scan(&maxPosition); err != nil {
		return 0, err
	}
	return maxPosition + 1, nil
}

func deriveCustomerProductTx(ctx context.Context, tx pgx.Tx, schema string, cmd catalogapp.DeriveCustomerProductCommand) (catalogapp.Product, error) {
	base, err := fetchProductDeriveBaseTx(ctx, tx, schema, cmd.BaseProductID)
	if err != nil {
		return catalogapp.Product{}, err
	}
	if base.CustomerID != 0 {
		return catalogapp.Product{}, fmt.Errorf("base product must be public")
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		name = base.Name
	}
	position, err := nextProductPositionTx(ctx, tx, schema, cmd.CategoryID, cmd.CustomerID, cmd.Position)
	if err != nil {
		return catalogapp.Product{}, err
	}
	var existingID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.products
		WHERE active=true AND customer_id=$1 AND base_product_id=$2 AND COALESCE(custom_type,'')='public_sku_alias'
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, schema), cmd.CustomerID, cmd.BaseProductID).Scan(&existingID)
	if err != nil && err != pgx.ErrNoRows {
		return catalogapp.Product{}, err
	}
	if existingID > 0 {
		return fetchCatalogProductByIDTx(ctx, tx, schema, existingID)
	}
	var productID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(
			name, remark, product_kind, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,
			product_category_id, product_category_position,
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id, special_attrs_json, created_at
		)
		VALUES($1,$2,$3,$4,$5,true,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,0),$15,$16,$17,'customer_only','public_sku_alias',$18,$19,$20::jsonb,now())
		RETURNING id
	`, schema), name, base.Remark, base.ProductKind, base.RoastLevel, base.DefaultPrice, base.RetailPrice100G, base.RetailPrice200G, base.RetailPrice227G, base.RetailPrice250G, base.DripBagGrams, base.DripBoxBagCount, base.AllowFulfillmentOrder, base.AllowMallOrder, cmd.CategoryID, position, cmd.CustomerID, cmd.BaseProductID, base.GreenBeanType, base.GreenBeanBomProductID, base.SpecialAttrsJSON).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}
	if base.ProductKind == catalogdomain.ProductKindRoasted {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_bom(product_id,yield_rate,status,updated_at)
			VALUES($1,$2,'active',now())
			ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()
		`, schema), productID, base.YieldRate); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if cmd.CopyBOM && base.ProductKind == catalogdomain.ProductKindRoasted {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,updated_at)
			SELECT $1,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,now()
			FROM %s.product_bom_items
			WHERE product_id=$2
			ORDER BY id
		`, schema, schema), productID, cmd.BaseProductID); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if cmd.CopyPriceTiers {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_price_tiers(product_id,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active,product_kind,price_basis,sales_unit,unit_bag_count,price_source_json)
			SELECT $1,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active,product_kind,price_basis,sales_unit,unit_bag_count,price_source_json
			FROM %s.product_price_tiers
			WHERE product_id=$2 AND active=true
			ORDER BY id
		`, schema, schema), productID, cmd.BaseProductID); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if err := normalizeProductPositions(ctx, tx, schema, cmd.CategoryID, cmd.CustomerID); err != nil {
		return catalogapp.Product{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, schema, cmd.Actor, "product", &productID, "derive_public_sku", postgresinfra.StrPtr("base_product_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.BaseProductID)), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "base_product_id": cmd.BaseProductID, "category_id": cmd.CategoryID, "copy_bom": cmd.CopyBOM, "copy_price_tiers": cmd.CopyPriceTiers}); err != nil {
		return catalogapp.Product{}, err
	}
	return fetchCatalogProductByIDTx(ctx, tx, schema, productID)
}

func deriveGradientTemplateTx(ctx context.Context, tx pgx.Tx, schema string, cmd catalogapp.DeriveGradientTemplateCommand) (catalogapp.GradientTemplate, error) {
	if existing, ok, err := findGradientTemplateBySourceTx(ctx, tx, schema, cmd.CustomerID, cmd.SourceTemplateID); err != nil {
		return catalogapp.GradientTemplate{}, err
	} else if ok {
		return existing, nil
	}
	source, err := fetchGradientTemplateTx(ctx, tx, schema, cmd.SourceTemplateID)
	if err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	if source.CustomerID != 0 {
		return catalogapp.GradientTemplate{}, fmt.Errorf("source template must be public")
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		name = source.Name
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.pricing_gradient_templates(name, display_unit, unit_template_id, customer_id, source_template_id, template_state, active)
		VALUES($1,$2,$3,$4,$5,'derived_from_public',true)
		RETURNING id
	`, schema), name, source.DisplayUnit, source.UnitTemplateID, cmd.CustomerID, source.ID).Scan(&id); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.pricing_gradient_template_tiers(template_id,label,min_weight_g,max_weight_g,margin_rate,position,active)
		SELECT $1,label,min_weight_g,max_weight_g,margin_rate,position,true
		FROM %s.pricing_gradient_template_tiers
		WHERE template_id=$2 AND active=true
		ORDER BY position, id
	`, schema, schema), id, source.ID); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, schema, cmd.Actor, "pricing_gradient_template", &id, "derive_public_gradient_template", postgresinfra.StrPtr("source_template_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", source.ID)), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "source_template_id": source.ID}); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	return fetchGradientTemplateTx(ctx, tx, schema, id)
}

func fetchGradientTemplateTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (catalogapp.GradientTemplate, error) {
	var row catalogapp.GradientTemplate
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, name, COALESCE(customer_id,0), COALESCE(source_template_id,0),
		       COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END),
		       display_unit, COALESCE(unit_template_id,0), active
		FROM %s.pricing_gradient_templates
		WHERE id=$1 AND active=true
	`, schema), id).Scan(&row.ID, &row.Name, &row.CustomerID, &row.SourceTemplateID, &row.TemplateState, &row.DisplayUnit, &row.UnitTemplateID, &row.Active); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, label, min_weight_g::float8, max_weight_g::float8, margin_rate::float8, position
		FROM %s.pricing_gradient_template_tiers
		WHERE active=true AND template_id=$1
		ORDER BY position, min_weight_g, id
	`, schema), id)
	if err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	defer rows.Close()
	row.Tiers = make([]catalogapp.GradientTemplateTier, 0)
	for rows.Next() {
		var tier catalogapp.GradientTemplateTier
		if err := rows.Scan(&tier.ID, &tier.TemplateID, &tier.Label, &tier.MinWeightG, &tier.MaxWeightG, &tier.MarginRate, &tier.Position); err != nil {
			return catalogapp.GradientTemplate{}, err
		}
		row.Tiers = append(row.Tiers, tier)
	}
	return row, rows.Err()
}

func findGradientTemplateBySourceTx(ctx context.Context, tx pgx.Tx, schema string, customerID, sourceTemplateID int64) (catalogapp.GradientTemplate, bool, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.pricing_gradient_templates
		WHERE active=true AND customer_id=$1 AND source_template_id=$2
		ORDER BY id
		LIMIT 1
	`, schema), customerID, sourceTemplateID).Scan(&id)
	if err == pgx.ErrNoRows {
		return catalogapp.GradientTemplate{}, false, nil
	}
	if err != nil {
		return catalogapp.GradientTemplate{}, false, err
	}
	row, err := fetchGradientTemplateTx(ctx, tx, schema, id)
	if err != nil {
		return catalogapp.GradientTemplate{}, false, err
	}
	return row, true, nil
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
	base.DripBagGrams = 10
	base.DripBoxBagCount = 10
	base.AllowFulfillmentOrder = true
	base.AllowMallOrder = false
	requestedKind := strings.TrimSpace(cmd.ProductKind)
	productKind := catalogdomain.NormalizeProductKind(cmd.ProductKind)
	customType := strings.TrimSpace(cmd.CustomType)
	baseProductID := cmd.BaseProductID
	shouldLoadBase := false
	if cmd.BaseProductID > 0 && customType != "custom_roast" {
		shouldLoadBase = !(requestedKind != "" && productKind == catalogdomain.ProductKindGreenBean)
	}
	if shouldLoadBase {
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
		if requestedKind == "" {
			productKind = catalogdomain.NormalizeProductKind(base.ProductKind)
		}
	} else if productKind != catalogdomain.ProductKindGreenBean && customType != "custom_roast" {
		return catalogapp.Product{}, fmt.Errorf("base product not found")
	}
	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	greenBeanType := strings.TrimSpace(base.GreenBeanType)
	greenBeanBomProductID := base.GreenBeanBomProductID
	if productKind == catalogdomain.ProductKindGreenBean {
		baseProductID = 0
		roastLevel = ""
		if strings.TrimSpace(cmd.GreenBeanType) != "" {
			greenBeanType = strings.TrimSpace(cmd.GreenBeanType)
		}
		if cmd.GreenBeanBomProductID > 0 {
			greenBeanBomProductID = cmd.GreenBeanBomProductID
		}
		if greenBeanType == "" {
			greenBeanType = "single_origin"
		}
	} else {
		if customType == "custom_roast" {
			baseProductID = 0
		}
		greenBeanType = ""
		greenBeanBomProductID = 0
	}
	if catalogdomain.ProductKindRequiresRoast(productKind) && roastLevel == "" {
		roastLevel = "中烘"
	}
	copyBOM := cmd.CopyBOM && productKind == catalogdomain.ProductKindRoasted && baseProductID > 0
	copyPriceTiers := cmd.CopyPriceTiers && baseProductID > 0
	dripBagGrams := base.DripBagGrams
	if cmd.DripBagGrams > 0 {
		dripBagGrams = cmd.DripBagGrams
	}
	dripBoxBagCount := base.DripBoxBagCount
	if cmd.DripBoxBagCount > 0 {
		dripBoxBagCount = cmd.DripBoxBagCount
	}
	yieldRate := cmd.YieldRate
	if yieldRate <= 0 && catalogdomain.ProductKindRequiresRoast(productKind) {
		yieldRate = catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	}
	name := strings.TrimSpace(cmd.Name)
	remark := strings.TrimSpace(cmd.Remark)
	var productID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(
			name, remark, product_kind, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,
			product_category_id, product_category_position,
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id, special_attrs_json, created_at
		)
		VALUES($1,$2,$3,$4,$5,true,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,0),$15,$16,$17,'customer_only',$18,$19,$20,$21::jsonb,now())
		RETURNING id
	`, r.schema), name, remark, productKind, roastLevel, base.DefaultPrice, base.RetailPrice100G, base.RetailPrice200G, base.RetailPrice227G, base.RetailPrice250G, dripBagGrams, dripBoxBagCount, base.AllowFulfillmentOrder, base.AllowMallOrder, 0, 0, cmd.CustomerID, baseProductID, customType, greenBeanType, greenBeanBomProductID, cmd.SpecialAttrsJSON).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}

	if catalogdomain.ProductKindSupportsBomParams(productKind) && yieldRate > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_bom(product_id,yield_rate,status,updated_at)
			VALUES($1,$2,'active',now())
			ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()
		`, r.schema), productID, yieldRate); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if copyBOM {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,updated_at)
			SELECT $1,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,now()
			FROM %s.product_bom_items
			WHERE product_id=$2
			ORDER BY id
		`, r.schema, r.schema), productID, baseProductID); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if copyPriceTiers {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_price_tiers(product_id,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active,product_kind,price_basis,sales_unit,unit_bag_count,price_source_json)
			SELECT $1,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active,product_kind,price_basis,sales_unit,unit_bag_count,price_source_json
			FROM %s.product_price_tiers
			WHERE product_id=$2 AND active=true
			ORDER BY id
		`, r.schema, r.schema), productID, baseProductID); err != nil {
			return catalogapp.Product{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &productID, "create", postgresinfra.StrPtr("customer_custom_product"), nil, postgresinfra.StrPtr(name), postgresinfra.AuditMeta{
		"customer_id":               cmd.CustomerID,
		"base_product_id":           baseProductID,
		"roast_level":               roastLevel,
		"remark":                    cmd.Remark,
		"custom_type":               strings.TrimSpace(cmd.CustomType),
		"copy_bom":                  copyBOM,
		"copy_price_tiers":          copyPriceTiers,
		"product_kind":              productKind,
		"green_bean_type":           greenBeanType,
		"green_bean_bom_product_id": greenBeanBomProductID,
		"drip_bag_grams":            dripBagGrams,
		"drip_box_bag_count":        dripBoxBagCount,
		"allow_fulfillment_order":   base.AllowFulfillmentOrder,
		"allow_mall_order":          base.AllowMallOrder,
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
		SELECT customer_id, use_public_sku, use_public_categories, COALESCE(use_public_gradient_templates,false)
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
		if err := rows.Scan(&row.CustomerID, &row.UsePublicSKU, &row.UsePublicCategories, &row.UsePublicGradientTemplates); err != nil {
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
		INSERT INTO %s.customer_sku_public_usage(customer_id, use_public_sku, use_public_categories, use_public_gradient_templates, created_at, updated_at)
		VALUES($1, $2, $3, $4, now(), now())
		ON CONFLICT (customer_id) DO UPDATE
		SET use_public_sku=excluded.use_public_sku,
		    use_public_categories=excluded.use_public_categories,
		    use_public_gradient_templates=excluded.use_public_gradient_templates,
		    updated_at=now()
		RETURNING customer_id, use_public_sku, use_public_categories, use_public_gradient_templates
	`, r.schema), cmd.CustomerID, cmd.UsePublicSKU, cmd.UsePublicCategories, cmd.UsePublicGradientTemplates).Scan(&usage.CustomerID, &usage.UsePublicSKU, &usage.UsePublicCategories, &usage.UsePublicGradientTemplates); err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_catalog", &cmd.CustomerID, "update_public_usage", postgresinfra.StrPtr("public_catalog_reference"), nil, postgresinfra.StrPtr(fmt.Sprintf("sku:%t categories:%t gradients:%t", usage.UsePublicSKU, usage.UsePublicCategories, usage.UsePublicGradientTemplates)), postgresinfra.AuditMeta{
		"customer_id":                   cmd.CustomerID,
		"use_public_sku":                usage.UsePublicSKU,
		"use_public_categories":         usage.UsePublicCategories,
		"use_public_gradient_templates": usage.UsePublicGradientTemplates,
	}); err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.CustomerPublicUsage{}, err
	}
	return usage, nil
}

func (r Repository) ListCustomerProductRuleTemplates(ctx context.Context) ([]catalogapp.CustomerProductRuleTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(customer_id,0), name, active
		FROM %s.customer_product_rule_templates
		WHERE active=true
		ORDER BY COALESCE(customer_id,0), name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.CustomerProductRuleTemplate, 0)
	indexByID := map[int64]int{}
	for rows.Next() {
		var row catalogapp.CustomerProductRuleTemplate
		if err := rows.Scan(&row.ID, &row.CustomerID, &row.Name, &row.Active); err != nil {
			return nil, err
		}
		indexByID[row.ID] = len(out)
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
	itemRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, product_subtype_category_id, gradient_template_id, operation_template_id,
		       COALESCE(price_list_rule_json::text,'{}'), COALESCE(unit_rule_json::text,'{}')
		FROM %s.customer_product_rule_template_items
		WHERE active=true AND template_id = ANY($1)
		ORDER BY template_id, product_subtype_category_id, id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	for itemRows.Next() {
		var item catalogapp.CustomerProductRuleTemplateItem
		if err := itemRows.Scan(&item.ID, &item.TemplateID, &item.ProductSubtypeCategoryID, &item.GradientTemplateID, &item.OperationTemplateID, &item.PriceListRuleJSON, &item.UnitRuleJSON); err != nil {
			return nil, err
		}
		if idx, ok := indexByID[item.TemplateID]; ok {
			out[idx].Items = append(out[idx].Items, item)
		}
	}
	return out, itemRows.Err()
}

func (r Repository) ListCustomerProductRuleOverrides(ctx context.Context) ([]catalogapp.CustomerProductRuleOverride, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, customer_id, product_subtype_category_id, gradient_template_id, operation_template_id,
		       COALESCE(price_list_rule_json::text,'{}'), COALESCE(unit_rule_json::text,'{}'), active
		FROM %s.customer_product_rule_overrides
		WHERE active=true
		ORDER BY customer_id, product_subtype_category_id, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.CustomerProductRuleOverride, 0)
	for rows.Next() {
		var row catalogapp.CustomerProductRuleOverride
		if err := rows.Scan(&row.ID, &row.CustomerID, &row.ProductSubtypeCategoryID, &row.GradientTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.UnitRuleJSON, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListCustomerProductRuleBindings(ctx context.Context) ([]catalogapp.CustomerProductRuleBinding, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(customer_product_rule_template_id,0)
		FROM %s.customers
		WHERE active=true AND COALESCE(customer_product_rule_template_id,0) > 0
		ORDER BY id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.CustomerProductRuleBinding, 0)
	for rows.Next() {
		var row catalogapp.CustomerProductRuleBinding
		if err := rows.Scan(&row.CustomerID, &row.TemplateID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveCustomerProductRuleTemplate(ctx context.Context, cmd catalogapp.SaveCustomerProductRuleTemplateCommand) (catalogapp.CustomerProductRuleTemplate, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.CustomerProductRuleTemplate{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.CustomerProductRuleTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	var id int64
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.customer_product_rule_templates
			SET customer_id=$2, name=$3, active=$4, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.CustomerID, cmd.Name, active).Scan(&id); err != nil {
			if err == pgx.ErrNoRows {
				return catalogapp.CustomerProductRuleTemplate{}, fmt.Errorf("customer product rule template not found")
			}
			return catalogapp.CustomerProductRuleTemplate{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_product_rule_template_items SET active=false, updated_at=now() WHERE template_id=$1`, r.schema), id); err != nil {
			return catalogapp.CustomerProductRuleTemplate{}, err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_product_rule_templates(customer_id, name, active, created_at, updated_at)
			VALUES($1,$2,$3,now(),now())
			RETURNING id
		`, r.schema), cmd.CustomerID, cmd.Name, active).Scan(&id); err != nil {
			return catalogapp.CustomerProductRuleTemplate{}, err
		}
	}
	for _, item := range cmd.Items {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_product_rule_template_items(template_id, product_subtype_category_id, gradient_template_id, operation_template_id, price_list_rule_json, unit_rule_json, active, created_at, updated_at)
			VALUES($1,$2,$3,$4,$5::jsonb,$6::jsonb,true,now(),now())
		`, r.schema), id, item.ProductSubtypeCategoryID, item.GradientTemplateID, item.OperationTemplateID, item.PriceListRuleJSON, item.UnitRuleJSON); err != nil {
			return catalogapp.CustomerProductRuleTemplate{}, err
		}
	}
	action := "create"
	if cmd.ID > 0 {
		action = "update"
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_rule_template", &id, action, postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{
		"customer_id": cmd.CustomerID,
		"item_count":  len(cmd.Items),
		"active":      active,
	}); err != nil {
		return catalogapp.CustomerProductRuleTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.CustomerProductRuleTemplate{}, err
	}
	return r.findCustomerProductRuleTemplate(ctx, id)
}

func (r Repository) findCustomerProductRuleTemplate(ctx context.Context, id int64) (catalogapp.CustomerProductRuleTemplate, error) {
	rows, err := r.ListCustomerProductRuleTemplates(ctx)
	if err != nil {
		return catalogapp.CustomerProductRuleTemplate{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return catalogapp.CustomerProductRuleTemplate{}, fmt.Errorf("customer product rule template not found")
}

func (r Repository) SaveCustomerProductRuleOverride(ctx context.Context, cmd catalogapp.SaveCustomerProductRuleOverrideCommand) (catalogapp.CustomerProductRuleOverride, error) {
	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	row := catalogapp.CustomerProductRuleOverride{}
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_product_rule_overrides(customer_id, product_subtype_category_id, gradient_template_id, operation_template_id, price_list_rule_json, unit_rule_json, active, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7,now(),now())
		ON CONFLICT (customer_id, product_subtype_category_id) WHERE active=true DO UPDATE
		SET gradient_template_id=excluded.gradient_template_id,
		    operation_template_id=excluded.operation_template_id,
		    price_list_rule_json=excluded.price_list_rule_json,
		    unit_rule_json=excluded.unit_rule_json,
		    active=excluded.active,
		    updated_at=now()
		RETURNING id, customer_id, product_subtype_category_id, gradient_template_id, operation_template_id,
		          COALESCE(price_list_rule_json::text,'{}'), COALESCE(unit_rule_json::text,'{}'), active
	`, r.schema), cmd.CustomerID, cmd.ProductSubtypeCategoryID, cmd.GradientTemplateID, cmd.OperationTemplateID, cmd.PriceListRuleJSON, cmd.UnitRuleJSON, active).
		Scan(&row.ID, &row.CustomerID, &row.ProductSubtypeCategoryID, &row.GradientTemplateID, &row.OperationTemplateID, &row.PriceListRuleJSON, &row.UnitRuleJSON, &row.Active)
	if err != nil {
		return catalogapp.CustomerProductRuleOverride{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "customer_product_rule_override", &row.ID, "upsert", postgresinfra.StrPtr("product_subtype_category_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", row.ProductSubtypeCategoryID)), postgresinfra.AuditMeta{
		"customer_id":           row.CustomerID,
		"gradient_template_id":  row.GradientTemplateID,
		"operation_template_id": row.OperationTemplateID,
		"price_list_rule_json":  row.PriceListRuleJSON,
		"unit_rule_json":        row.UnitRuleJSON,
	})
	return row, nil
}

func (r Repository) BindCustomerProductRuleTemplate(ctx context.Context, cmd catalogapp.CustomerProductRuleTemplateBindingCommand) (catalogapp.CustomerProductRuleBinding, error) {
	tag, err := r.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customers
		SET customer_product_rule_template_id=$2
		WHERE id=$1 AND active=true
	`, r.schema), cmd.CustomerID, cmd.TemplateID)
	if err != nil {
		return catalogapp.CustomerProductRuleBinding{}, err
	}
	if tag.RowsAffected() == 0 {
		return catalogapp.CustomerProductRuleBinding{}, fmt.Errorf("customer not found")
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "customer_product_rule_template_binding", &cmd.CustomerID, "update", postgresinfra.StrPtr("customer_product_rule_template_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.TemplateID)), postgresinfra.AuditMeta{
		"customer_id": cmd.CustomerID,
		"template_id": cmd.TemplateID,
	})
	return catalogapp.CustomerProductRuleBinding{CustomerID: cmd.CustomerID, TemplateID: cmd.TemplateID}, nil
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
		  AND NOT EXISTS (
			  SELECT 1 FROM %[1]s.product_categories child
			  WHERE child.active=true AND child.parent_id=c.id
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
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id, name, COALESCE(remark,''), COALESCE(roast_level,''), COALESCE(special_attrs_json::text,'{}'), default_price,
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
		COALESCE(gradient_template_id_override,0),
		COALESCE(operation_template_id_override,0),
		COALESCE(unit_rule_override_json::text,'{}'),
		COALESCE((SELECT COUNT(*) FROM %[1]s.product_bom_items bi WHERE bi.product_id=products.id),0),
		COALESCE((SELECT NULLIF(status,'') FROM %[1]s.product_bom WHERE product_id=products.id), 'missing')
		FROM %[1]s.products WHERE id=$1`, schema), id).Scan(&p.ID, &p.Name, &p.Remark, &p.RoastLevel, &p.SpecialAttrsJSON, &p.DefaultPrice, &p.ProductKind, &p.GreenBeanType, &p.GreenBeanBomProductID, &p.DripBagGrams, &p.DripBoxBagCount, &p.AllowFulfillmentOrder, &p.AllowMallOrder, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.YieldRate, &p.ProductCategoryID, &p.ProductCategoryPosition, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.MarginRateOverride, &p.GradientTemplateIDOverride, &p.OperationTemplateIDOverride, &p.UnitRuleOverrideJSON, &p.BomItemCount, &p.BomStatus)
	if err != nil {
		return nil, nil
	}
	p.ProductKind = catalogdomain.NormalizeProductKind(p.ProductKind)
	if p.ProductKind == catalogdomain.ProductKindDripBag {
		p.SalesUnits = []string{"bag", "box"}
	}
	if !catalogdomain.ProductKindSupportsBomParams(p.ProductKind) {
		p.RoastLevel = ""
		p.YieldRate = 0
	}
	return &p, nil
}

func catalogProductFromOption(p postgresinfra.ProductOption) catalogapp.Product {
	out := catalogapp.Product{ID: p.ID, Name: p.Name, Remark: p.Remark, RoastLevel: p.RoastLevel, SpecialAttrsJSON: p.SpecialAttrsJSON, ProductKind: p.ProductKind, GreenBeanType: p.GreenBeanType, GreenBeanBomProductID: p.GreenBeanBomProductID, DripBagGrams: p.DripBagGrams, DripBoxBagCount: p.DripBoxBagCount, AllowFulfillmentOrder: p.AllowFulfillmentOrder, AllowMallOrder: p.AllowMallOrder, SalesUnits: p.SalesUnits, DefaultPrice: p.DefaultPrice, RetailPrice100G: p.RetailPrice100G, RetailPrice200G: p.RetailPrice200G, RetailPrice227G: p.RetailPrice227G, RetailPrice250G: p.RetailPrice250G, YieldRate: p.YieldRate, ProductCategoryID: p.ProductCategoryID, ProductCategoryPosition: p.ProductCategoryPosition, CustomerID: p.CustomerID, BaseProductID: p.BaseProductID, Visibility: p.Visibility, CustomType: p.CustomType, MarginRateOverride: p.MarginRateOverride, GradientTemplateIDOverride: p.GradientTemplateIDOverride, OperationTemplateIDOverride: p.OperationTemplateIDOverride, UnitRuleOverrideJSON: p.UnitRuleOverrideJSON, Active: true, BomItemCount: p.BomItemCount, BomStatus: p.BomStatus, BomSourceType: p.BomSourceType, EffectiveProductID: p.EffectiveProductID, EffectiveBomVersionID: p.EffectiveBomVersionID, SourceProductID: p.SourceProductID, SourceProductCode: p.SourceProductCode, SourceProductName: p.SourceProductName, SourceBomVersionID: p.SourceBomVersionID, SourceBomVersionNo: p.SourceBomVersionNo, DerivedFromLabel: p.DerivedFromLabel, CanEditBOM: p.CanEditBOM, OrderUsageCount: p.OrderUsageCount}
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
