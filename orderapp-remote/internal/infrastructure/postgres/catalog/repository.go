package catalog

import (
	"context"
	"encoding/json"
	"errors"
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

func jsonTextOrDefaultArray(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "[]"
	}
	return raw
}

func jsonMapOrEmpty(raw []byte) map[string]any {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func productSalesSpecsFromJSON(raw string) []catalogapp.ProductSalesSpec {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []catalogapp.ProductSalesSpec
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func productSalesSpecsJSON(specs []catalogapp.ProductSalesSpec) string {
	if len(specs) == 0 {
		return "[]"
	}
	encoded, err := json.Marshal(specs)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

type productUnitAuditSnapshot struct {
	InventoryUnit        string
	IntegerInventoryUnit bool
	DefaultSalesUnit     string
	UnitConversionJSON   string
	SalesUnitRulesJSON   string
}

func productUnitAuditValues(raw string) productUnitAuditSnapshot {
	snapshot := productUnitAuditSnapshot{
		InventoryUnit:      "kg",
		UnitConversionJSON: "{}",
		SalesUnitRulesJSON: "{}",
	}
	rule := map[string]any{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &rule)
	}
	if value, ok := rule["inventory_unit"].(string); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			snapshot.InventoryUnit = trimmed
		}
	}
	if value, ok := rule["integer_inventory_unit"]; ok {
		snapshot.IntegerInventoryUnit = boolFromRuleValue(value)
	} else if value, ok := rule["integer_unit"]; ok {
		snapshot.IntegerInventoryUnit = boolFromRuleValue(value)
	}
	for _, key := range []string{"default_sales_unit", "quote_unit", "order_unit"} {
		if value, ok := rule[key].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				snapshot.DefaultSalesUnit = trimmed
				break
			}
		}
	}
	if strings.TrimSpace(snapshot.DefaultSalesUnit) == "" {
		snapshot.DefaultSalesUnit = snapshot.InventoryUnit
	}
	if value, ok := rule["unit_conversion_json"]; ok {
		snapshot.UnitConversionJSON = jsonAuditTextOrDefault(value, "{}")
	} else if value, ok := rule["conversion_json"]; ok {
		snapshot.UnitConversionJSON = jsonAuditTextOrDefault(value, "{}")
	}
	if value, ok := rule["sales_unit_rules"]; ok {
		snapshot.SalesUnitRulesJSON = jsonAuditTextOrDefault(value, "{}")
	}
	return snapshot
}

func inventoryUnitAuditValues(raw string) (string, bool) {
	snapshot := productUnitAuditValues(raw)
	return snapshot.InventoryUnit, snapshot.IntegerInventoryUnit
}

func jsonAuditTextOrDefault(value any, fallback string) string {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return fallback
		}
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil && decoded != nil {
			if encoded, err := json.Marshal(decoded); err == nil {
				return string(encoded)
			}
		}
		return trimmed
	default:
		if value == nil {
			return fallback
		}
		if encoded, err := json.Marshal(value); err == nil && string(encoded) != "null" {
			return string(encoded)
		}
	}
	return fallback
}

func boolFromRuleValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y":
			return true
		}
	case float64:
		return v != 0
	}
	return false
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) ListProducts(ctx context.Context) ([]catalogapp.Product, error) {
	ps, err := postgresinfra.FetchProducts(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	out := catalogProductsFromOptions(ps)
	if err := r.attachProductGroupSummaries(ctx, out); err != nil {
		return nil, err
	}
	if err := r.attachProductPriceSummaries(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) GetProduct(ctx context.Context, id int64) (*catalogapp.Product, error) {
	p, err := fetchProductByID(ctx, r.pool, r.schema, id)
	if err != nil || p == nil {
		return nil, err
	}
	out := catalogProductFromOption(*p)
	rows := []catalogapp.Product{out}
	if err := r.attachProductGroupSummaries(ctx, rows); err != nil {
		return nil, err
	}
	out = rows[0]
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
	var oldUnitRuleOverrideJSON string
	var oldUnitTemplateID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(unit_rule_override_json::text,'{}'), COALESCE(unit_template_id,0)
		FROM %s.products
		WHERE id=$1
		FOR UPDATE
	`, r.schema), cmd.ProductID).Scan(&oldUnitRuleOverrideJSON, &oldUnitTemplateID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET roast_level=$2, retail_price_100g=$3, retail_price_200g=$4, retail_price_227g=$5, retail_price_250g=$6,
		    product_kind=$7, drip_bag_grams=$8, drip_box_bag_count=$9, allow_fulfillment_order=$10, allow_mall_order=$11,
		    green_bean_type=$12, green_bean_bom_product_id=$13, remark=$14, name=COALESCE(NULLIF($15,''), name),
		    special_attrs_json=$16::jsonb, unit_rule_override_json=$17::jsonb, unit_template_id=$18
		WHERE id=$1`, r.schema), cmd.ProductID, roastLevel, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G, productKind, cmd.DripBagGrams, cmd.DripBoxBagCount, cmd.AllowFulfillmentOrder, cmd.AllowMallOrder, greenBeanType, greenBeanBomProductID, cmd.Remark, cmd.Name, cmd.SpecialAttrsJSON, cmd.UnitRuleOverrideJSON, cmd.UnitTemplateID); err != nil {
		return err
	}
	if catalogdomain.ProductKindSupportsBomParams(productKind) && yieldRate > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_bom(product_id,yield_rate,updated_at)
			VALUES($1,$2,now())
			ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()`, r.schema), cmd.ProductID, yieldRate); err != nil {
			return err
		}
	}
	if err := syncDerivedSKUsForParentTx(ctx, tx, r.schema, cmd.Actor, cmd.ProductID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	oldUnitAudit := productUnitAuditValues(oldUnitRuleOverrideJSON)
	newUnitAudit := productUnitAuditValues(cmd.UnitRuleOverrideJSON)
	meta := postgresinfra.AuditMeta{
		"product_id":        cmd.ProductID,
		"product_kind":      productKind,
		"roast_level":       roastLevel,
		"yield_rate":        yieldRate,
		"retail_price_100g": cmd.RetailPrice100G,
		"retail_price_200g": cmd.RetailPrice200G,
		"retail_price_227g": cmd.RetailPrice227G,
		"retail_price_250g": cmd.RetailPrice250G,
		"remark":            cmd.Remark,
		"name":              cmd.Name,
	}
	meta["old_inventory_unit"] = oldUnitAudit.InventoryUnit
	meta["new_inventory_unit"] = newUnitAudit.InventoryUnit
	meta["old_integer_inventory_unit"] = oldUnitAudit.IntegerInventoryUnit
	meta["new_integer_inventory_unit"] = newUnitAudit.IntegerInventoryUnit
	meta["old_default_sales_unit"] = oldUnitAudit.DefaultSalesUnit
	meta["new_default_sales_unit"] = newUnitAudit.DefaultSalesUnit
	meta["old_unit_conversion_json"] = oldUnitAudit.UnitConversionJSON
	meta["new_unit_conversion_json"] = newUnitAudit.UnitConversionJSON
	meta["old_sales_unit_rules"] = oldUnitAudit.SalesUnitRulesJSON
	meta["new_sales_unit_rules"] = newUnitAudit.SalesUnitRulesJSON
	meta["old_unit_template_id"] = oldUnitTemplateID
	meta["new_unit_template_id"] = cmd.UnitTemplateID
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
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id,
			special_attrs_json, unit_rule_override_json, unit_template_id, created_at
		)
		VALUES($1,$2,$3,$4,$5,true,$6,$7,$8,$9,$10,$11,$12,$13,0,0,'public','',$14,$15,$16::jsonb,$17::jsonb,$18,now())
		RETURNING id
	`, r.schema), name, cmd.Remark, productKind, roastLevel, cmd.DefaultPrice, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G, cmd.DripBagGrams, cmd.DripBoxBagCount, cmd.AllowFulfillmentOrder, cmd.AllowMallOrder, greenBeanType, greenBeanBomProductID, cmd.SpecialAttrsJSON, cmd.UnitRuleOverrideJSON, cmd.UnitTemplateID).Scan(&productID); err != nil {
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
	unitAudit := productUnitAuditValues(cmd.UnitRuleOverrideJSON)
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
		"inventory_unit":          unitAudit.InventoryUnit,
		"integer_inventory_unit":  unitAudit.IntegerInventoryUnit,
		"default_sales_unit":      unitAudit.DefaultSalesUnit,
		"unit_conversion_json":    unitAudit.UnitConversionJSON,
		"sales_unit_rules":        unitAudit.SalesUnitRulesJSON,
		"unit_template_id":        cmd.UnitTemplateID,
	}); err != nil {
		return catalogapp.Product{}, err
	}
	if err := syncDerivedSKUsForParentTx(ctx, tx, r.schema, cmd.Actor, productID); err != nil {
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

func (r Repository) CopyProduct(ctx context.Context, cmd catalogapp.CopyProductCommand) (catalogapp.Product, error) {
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

	source, err := fetchProductForCopyTx(ctx, tx, r.schema, cmd.SourceProductID)
	if err != nil {
		return catalogapp.Product{}, err
	}
	if source == nil {
		return catalogapp.Product{}, fmt.Errorf("source product not found")
	}
	copyName, err := nextProductArchiveCopyNameTx(ctx, tx, r.schema, source.Name)
	if err != nil {
		return catalogapp.Product{}, err
	}
	var productID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(
			name, remark, product_kind, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id,
			special_attrs_json, unit_rule_override_json, unit_template_id, created_at
		)
		SELECT
			$2, remark, product_kind, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id,
			special_attrs_json, unit_rule_override_json, unit_template_id, now()
		FROM %s.products
		WHERE id=$1
		RETURNING id
	`, r.schema, r.schema), cmd.SourceProductID, copyName).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_production_config_fields(
			product_id, field_key, label, field_type, unit, value_text, value_number, value_bool,
			template_field_key, required, options_json, show_in_price_list, sort_order, created_at, updated_at
		)
		SELECT $2, field_key, label, field_type, unit, value_text, value_number, value_bool,
			template_field_key, required, options_json, show_in_price_list, sort_order, now(), now()
		FROM %s.product_production_config_fields
		WHERE product_id=$1
	`, r.schema, r.schema), cmd.SourceProductID, productID); err != nil {
		return catalogapp.Product{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &productID, "copy_product_archive", postgresinfra.StrPtr("source_product_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.SourceProductID)), postgresinfra.StrPtr(fmt.Sprintf("%d", productID)), postgresinfra.AuditMeta{
		"source_product_id":   cmd.SourceProductID,
		"source_product_name": source.Name,
		"target_product_id":   productID,
		"target_product_name": copyName,
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
		return catalogapp.Product{}, fmt.Errorf("copied product not found")
	}
	return *product, nil
}

func fetchProductForCopyTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (*catalogapp.Product, error) {
	var product catalogapp.Product
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, name
		FROM %s.products
		WHERE id=$1
	`, schema), productID).Scan(&product.ID, &product.Name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &product, nil
}

func nextProductArchiveCopyNameTx(ctx context.Context, tx pgx.Tx, schema string, sourceName string) (string, error) {
	baseName := strings.TrimSpace(sourceName)
	if baseName == "" {
		baseName = "未命名商品"
	}
	candidate := fmt.Sprintf("%s 复制", baseName)
	for i := 0; i < 200; i++ {
		if i > 0 {
			candidate = fmt.Sprintf("%s 复制 %d", baseName, i+1)
		}
		var exists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT EXISTS(SELECT 1 FROM %s.products WHERE lower(name)=lower($1))
		`, schema), candidate).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("product copy name exhausted")
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
	if cmd.ParentProductID > 0 {
		var parentCustomerID, parentCategoryID, parentUnitTemplateID int64
		var parentProductKind string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(customer_id,0), COALESCE(product_category_id,0), COALESCE(unit_template_id,0), COALESCE(NULLIF(product_kind,''),'roasted_bean')
			FROM %s.products
			WHERE id=$1
		`, r.schema), cmd.ParentProductID).Scan(&parentCustomerID, &parentCategoryID, &parentUnitTemplateID, &parentProductKind); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return catalogapp.Product{}, fmt.Errorf("parent product not found")
			}
			return catalogapp.Product{}, err
		}
		if cmd.CustomerID == 0 {
			cmd.CustomerID = parentCustomerID
		}
		if cmd.ProductSubtypeCategoryID == 0 {
			categoryID = parentCategoryID
		}
		if cmd.UnitTemplateID == 0 {
			cmd.UnitTemplateID = parentUnitTemplateID
		}
		productKind = catalogdomain.NormalizeProductKind(parentProductKind)
	}
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
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id,
			special_attrs_json, unit_rule_override_json, unit_template_id,
			parent_product_id, sku_name, sku_code, barcode, spec_label, net_content_qty, net_content_unit, is_default_sku,
			product_category_id, created_at
		)
		VALUES($1,$2,$3,'',0,$4,0,0,0,0,10,10,true,false,$5,0,$6,'','',0,$7::jsonb,$8::jsonb,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,now())
		RETURNING id
	`, r.schema), strings.TrimSpace(cmd.Name), strings.TrimSpace(cmd.Remark), productKind, cmd.Active, cmd.CustomerID, visibility, cmd.SpecialAttrsJSON, cmd.UnitRuleOverrideJSON, cmd.UnitTemplateID, cmd.ParentProductID, strings.TrimSpace(cmd.SKUName), strings.TrimSpace(cmd.SKUCode), strings.TrimSpace(cmd.Barcode), strings.TrimSpace(cmd.SpecLabel), cmd.NetContentQty, strings.TrimSpace(cmd.NetContentUnit), cmd.IsDefaultSKU, categoryID).Scan(&productID); err != nil {
		return catalogapp.Product{}, err
	}
	unitAudit := productUnitAuditValues(cmd.UnitRuleOverrideJSON)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &productID, "create_sku", postgresinfra.StrPtr("sku"), nil, postgresinfra.StrPtr(strings.TrimSpace(cmd.Name)), postgresinfra.AuditMeta{
		"customer_id":                  cmd.CustomerID,
		"product_type_category_id":     cmd.ProductTypeCategoryID,
		"product_subtype_category_id":  categoryID,
		"legacy_product_kind_snapshot": productKind,
		"inventory_unit":               unitAudit.InventoryUnit,
		"integer_inventory_unit":       unitAudit.IntegerInventoryUnit,
		"default_sales_unit":           unitAudit.DefaultSalesUnit,
		"unit_conversion_json":         unitAudit.UnitConversionJSON,
		"sales_unit_rules":             unitAudit.SalesUnitRulesJSON,
		"unit_template_id":             cmd.UnitTemplateID,
		"parent_product_id":            cmd.ParentProductID,
		"sku_name":                     strings.TrimSpace(cmd.SKUName),
		"sku_code":                     strings.TrimSpace(cmd.SKUCode),
		"barcode":                      strings.TrimSpace(cmd.Barcode),
		"spec_label":                   strings.TrimSpace(cmd.SpecLabel),
		"net_content_qty":              cmd.NetContentQty,
		"net_content_unit":             strings.TrimSpace(cmd.NetContentUnit),
	}); err != nil {
		return catalogapp.Product{}, err
	}
	if cmd.ParentProductID == 0 {
		if err := syncDerivedSKUsForParentTx(ctx, tx, r.schema, cmd.Actor, productID); err != nil {
			return catalogapp.Product{}, err
		}
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

type derivedSKUParent struct {
	ID             int64
	Name           string
	Remark         string
	ProductKind    string
	CustomerID     int64
	CategoryID     int64
	UnitTemplateID int64
	Visibility     string
}

func syncDerivedSKUsForTemplateTx(ctx context.Context, tx pgx.Tx, schema string, actor string, templateID int64) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.products
		WHERE COALESCE(unit_template_id,0)=$1 AND COALESCE(parent_product_id,0)=0
		ORDER BY id
	`, schema), templateID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var parentID int64
		if err := rows.Scan(&parentID); err != nil {
			return err
		}
		if err := syncDerivedSKUsForParentTx(ctx, tx, schema, actor, parentID); err != nil {
			return err
		}
	}
	return rows.Err()
}

func syncDerivedSKUsForParentTx(ctx context.Context, tx pgx.Tx, schema string, actor string, parentID int64) error {
	if parentID <= 0 {
		return nil
	}
	var parent derivedSKUParent
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(name,''), COALESCE(remark,''), COALESCE(NULLIF(product_kind,''),'roasted_bean'),
		       COALESCE(customer_id,0), COALESCE(product_category_id,0), COALESCE(unit_template_id,0),
		       COALESCE(NULLIF(visibility,''),'public')
		FROM %s.products
		WHERE id=$1 AND COALESCE(parent_product_id,0)=0
	`, schema), parentID).Scan(&parent.ID, &parent.Name, &parent.Remark, &parent.ProductKind, &parent.CustomerID, &parent.CategoryID, &parent.UnitTemplateID, &parent.Visibility); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if parent.UnitTemplateID <= 0 {
		_, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.products
			SET derived_spec_status='template_removed'
			WHERE parent_product_id=$1 AND auto_derived_sku=true AND derived_spec_status<>'template_removed'
		`, schema), parent.ID)
		return err
	}
	var salesSpecsJSON string
	var templateActive bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(sales_specs_json::text,'[]'), COALESCE(active,true)
		FROM %s.product_unit_templates
		WHERE id=$1 AND deleted_at IS NULL
	`, schema), parent.UnitTemplateID).Scan(&salesSpecsJSON, &templateActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, updateErr := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.products
				SET derived_spec_status='template_removed'
				WHERE parent_product_id=$1 AND auto_derived_sku=true AND derived_unit_template_id=$2
			`, schema), parent.ID, parent.UnitTemplateID)
			return updateErr
		}
		return err
	}
	specs := productSalesSpecsFromJSON(salesSpecsJSON)
	allKeys := map[string]bool{}
	activeKeys := map[string]bool{}
	for _, spec := range specs {
		specKey := strings.TrimSpace(spec.SpecKey)
		if specKey == "" {
			continue
		}
		allKeys[specKey] = true
		if !templateActive || !spec.Active {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.products
				SET derived_spec_status='template_disabled',
				    derived_spec_name=$4,
				    derived_sales_unit=$5,
				    spec_label=$4,
				    net_content_qty=$6,
				    net_content_unit=$7
				WHERE parent_product_id=$1 AND auto_derived_sku=true AND derived_unit_template_id=$2 AND derived_spec_key=$3
			`, schema), parent.ID, parent.UnitTemplateID, specKey, strings.TrimSpace(spec.SpecName), strings.TrimSpace(spec.SalesUnit), spec.NetContentQty, strings.TrimSpace(spec.NetContentUnit)); err != nil {
				return err
			}
			continue
		}
		activeKeys[specKey] = true
		if err := upsertDerivedSKUForSpecTx(ctx, tx, schema, actor, parent, spec); err != nil {
			return err
		}
	}
	if len(allKeys) == 0 {
		_, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.products
			SET derived_spec_status='template_removed'
			WHERE parent_product_id=$1 AND auto_derived_sku=true AND derived_unit_template_id=$2
		`, schema), parent.ID, parent.UnitTemplateID)
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(derived_spec_key,'')
		FROM %s.products
		WHERE parent_product_id=$1 AND auto_derived_sku=true AND derived_unit_template_id=$2
	`, schema), parent.ID, parent.UnitTemplateID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var childID int64
		var specKey string
		if err := rows.Scan(&childID, &specKey); err != nil {
			return err
		}
		if activeKeys[specKey] {
			continue
		}
		status := "template_removed"
		if allKeys[specKey] {
			status = "template_disabled"
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET derived_spec_status=$2 WHERE id=$1`, schema), childID, status); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.products
		SET derived_spec_status='template_removed'
		WHERE parent_product_id=$1 AND auto_derived_sku=true AND derived_unit_template_id<>$2 AND derived_spec_status<>'template_removed'
	`, schema), parent.ID, parent.UnitTemplateID)
	return err
}

func upsertDerivedSKUForSpecTx(ctx context.Context, tx pgx.Tx, schema string, actor string, parent derivedSKUParent, spec catalogapp.ProductSalesSpec) error {
	specKey := strings.TrimSpace(spec.SpecKey)
	if specKey == "" {
		return nil
	}
	specName := strings.TrimSpace(spec.SpecName)
	salesUnit := strings.TrimSpace(spec.SalesUnit)
	netContentUnit := strings.TrimSpace(spec.NetContentUnit)
	childName := strings.TrimSpace(parent.Name + " " + specName)
	if childName == "" {
		childName = specName
	}
	var childID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.products
		WHERE parent_product_id=$1 AND auto_derived_sku=true AND derived_unit_template_id=$2 AND derived_spec_key=$3
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, schema), parent.ID, parent.UnitTemplateID, specKey).Scan(&childID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if childID > 0 {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.products
			SET name=$2, sku_name=$3, spec_label=$3, net_content_qty=$4, net_content_unit=$5,
			    unit_template_id=$6, derived_spec_name=$3, derived_sales_unit=$7, derived_spec_status='active',
			    active=true
			WHERE id=$1
		`, schema), childID, childName, specName, spec.NetContentQty, netContentUnit, parent.UnitTemplateID, salesUnit)
		return err
	}
	visibility := parent.Visibility
	if parent.CustomerID > 0 {
		visibility = "customer_only"
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(
			name, remark, product_kind, roast_level, default_price, active,
			retail_price_100g, retail_price_200g, retail_price_227g, retail_price_250g,
			drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,
			customer_id, base_product_id, visibility, custom_type, green_bean_type, green_bean_bom_product_id,
			special_attrs_json, unit_rule_override_json, unit_template_id,
			parent_product_id, sku_name, sku_code, barcode, spec_label, net_content_qty, net_content_unit, is_default_sku,
			product_category_id, auto_derived_sku, derived_unit_template_id, derived_spec_key, derived_spec_name, derived_sales_unit, derived_spec_status, created_at
		)
		VALUES($1,$2,$3,'',0,true,0,0,0,0,10,10,true,false,$4,0,$5,'','',0,'{}'::jsonb,'{}'::jsonb,$6,$7,$8,'','',$8,$9,$10,$11,$12,true,$6,$13,$8,$14,'active',now())
		RETURNING id
	`, schema), childName, parent.Remark, parent.ProductKind, parent.CustomerID, visibility, parent.UnitTemplateID, parent.ID, specName, spec.NetContentQty, netContentUnit, spec.Default, parent.CategoryID, specKey, salesUnit).Scan(&childID); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "product", &childID, "derive_sku_from_sales_spec", postgresinfra.StrPtr("sales_spec"), nil, postgresinfra.StrPtr(childName), postgresinfra.AuditMeta{
		"parent_product_id": parent.ID,
		"unit_template_id":  parent.UnitTemplateID,
		"spec_key":          specKey,
		"spec_name":         specName,
		"sales_unit":        salesUnit,
		"net_content_qty":   spec.NetContentQty,
		"net_content_unit":  netContentUnit,
	})
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

func (r Repository) ListProductProductionConfigs(ctx context.Context) ([]catalogapp.ProductProductionConfig, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT product_id, production_bom_id, production_bom_version_id, process_route_id, COALESCE(industry_field_template_id,0),
			       COALESCE(expected_loss_rate,0)::float8, COALESCE(note,'')
		FROM %s.product_production_configs
		ORDER BY product_id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductProductionConfig, 0)
	for rows.Next() {
		var row catalogapp.ProductProductionConfig
		if err := rows.Scan(&row.ProductID, &row.ProductionBomID, &row.ProductionBomVersionID, &row.ProcessRouteID, &row.IndustryFieldTemplateID, &row.ExpectedLossRate, &row.Note); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachProductProductionConfigFields(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) GetProductProductionConfig(ctx context.Context, productID int64) (catalogapp.ProductProductionConfig, error) {
	rows, err := r.ListProductProductionConfigs(ctx)
	if err != nil {
		return catalogapp.ProductProductionConfig{}, err
	}
	for _, row := range rows {
		if row.ProductID == productID {
			return row, nil
		}
	}
	return catalogapp.ProductProductionConfig{ProductID: productID, Fields: []catalogapp.ProductProductionConfigField{}}, nil
}

func (r Repository) attachProductProductionConfigFields(ctx context.Context, configs []catalogapp.ProductProductionConfig) error {
	if len(configs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(configs))
	index := map[int64]int{}
	for i, row := range configs {
		ids = append(ids, row.ProductID)
		index[row.ProductID] = i
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT id, product_id, field_key, label, field_type, unit, value_text,
			       value_number::float8, value_bool, COALESCE(template_field_key,''), COALESCE(required,false),
			       COALESCE(options_json::text,'[]'), show_in_price_list, sort_order
		FROM %s.product_production_config_fields
		WHERE product_id = ANY($1)
		ORDER BY product_id, sort_order, id
	`, r.schema), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var field catalogapp.ProductProductionConfigField
		if err := rows.Scan(&field.ID, &field.ProductID, &field.FieldKey, &field.Label, &field.FieldType, &field.Unit, &field.ValueText, &field.ValueNumber, &field.ValueBool, &field.TemplateFieldKey, &field.Required, &field.OptionsJSON, &field.ShowInPriceList, &field.SortOrder); err != nil {
			return err
		}
		if i, ok := index[field.ProductID]; ok {
			configs[i].Fields = append(configs[i].Fields, field)
		}
	}
	return rows.Err()
}

func (r Repository) SaveProductProductionConfig(ctx context.Context, cmd catalogapp.SaveProductProductionConfigCommand) (catalogapp.ProductProductionConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return catalogapp.ProductProductionConfig{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	fields, err := normalizeProductProductionConfigFieldsAgainstTemplateTx(ctx, tx, r.schema, cmd.IndustryFieldTemplateID, cmd.Fields)
	if err != nil {
		return catalogapp.ProductProductionConfig{}, err
	}
	cmd.Fields = fields
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_production_configs(
				product_id, production_bom_id, production_bom_version_id, process_route_id, industry_field_template_id,
				expected_loss_rate, note, created_by, updated_by, created_at, updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8,now(),now())
			ON CONFLICT (product_id) DO UPDATE SET
				production_bom_id=excluded.production_bom_id,
				production_bom_version_id=excluded.production_bom_version_id,
				process_route_id=excluded.process_route_id,
				industry_field_template_id=excluded.industry_field_template_id,
				expected_loss_rate=excluded.expected_loss_rate,
				note=excluded.note,
				updated_by=excluded.updated_by,
				updated_at=now()
		`, r.schema), cmd.ProductID, cmd.ProductionBomID, cmd.ProductionBomVersionID, cmd.ProcessRouteID, cmd.IndustryFieldTemplateID, cmd.ExpectedLossRate, strings.TrimSpace(cmd.Note), strings.TrimSpace(cmd.Actor)); err != nil {
		return catalogapp.ProductProductionConfig{}, err
	}
	if cmd.ProductionBomID > 0 && cmd.ProductionBomVersionID > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_at,bound_by)
			VALUES($1,$2,$3,now(),$4)
			ON CONFLICT (product_id) DO UPDATE SET
				bom_id=excluded.bom_id,
				bom_version_id=excluded.bom_version_id,
				bound_at=now(),
				bound_by=excluded.bound_by
		`, r.schema), cmd.ProductID, cmd.ProductionBomID, cmd.ProductionBomVersionID, strings.TrimSpace(cmd.Actor)); err != nil {
			return catalogapp.ProductProductionConfig{}, err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_production_config_fields WHERE product_id=$1`, r.schema), cmd.ProductID); err != nil {
		return catalogapp.ProductProductionConfig{}, err
	}
	for _, field := range cmd.Fields {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.product_production_config_fields(
					product_id, field_key, label, field_type, unit, value_text, value_number, value_bool,
					template_field_key, required, options_json, show_in_price_list, sort_order, created_at, updated_at
				) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13,now(),now())
			`, r.schema), cmd.ProductID, field.FieldKey, field.Label, field.FieldType, field.Unit, field.ValueText, field.ValueNumber, field.ValueBool, field.TemplateFieldKey, field.Required, jsonTextOrDefaultArray(field.OptionsJSON), field.ShowInPriceList, field.SortOrder); err != nil {
			return catalogapp.ProductProductionConfig{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_production_config", &cmd.ProductID, "save_product_production_config", postgresinfra.StrPtr("product_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.ProductID)), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "production_bom_id": cmd.ProductionBomID, "production_bom_version_id": cmd.ProductionBomVersionID, "process_route_id": cmd.ProcessRouteID, "industry_field_template_id": cmd.IndustryFieldTemplateID, "expected_loss_rate": cmd.ExpectedLossRate, "field_count": len(cmd.Fields)}); err != nil {
		return catalogapp.ProductProductionConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductProductionConfig{}, err
	}
	return r.GetProductProductionConfig(ctx, cmd.ProductID)
}

func normalizeProductProductionConfigFieldsAgainstTemplateTx(ctx context.Context, tx pgx.Tx, schema string, templateID int64, fields []catalogapp.ProductProductionConfigField) ([]catalogapp.ProductProductionConfigField, error) {
	if templateID <= 0 {
		return fields, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT field_key, label, field_type, unit, required, COALESCE(options_json,'[]'::jsonb)::text, sort_order
		FROM %s.industry_field_definitions
		WHERE template_id=$1
		ORDER BY sort_order, id
	`, schema), templateID)
	if err != nil {
		return nil, err
	}
	type def struct {
		key      string
		label    string
		kind     string
		unit     string
		required bool
		options  string
		sort     int
	}
	defs := map[string]def{}
	for rows.Next() {
		var d def
		if err := rows.Scan(&d.key, &d.label, &d.kind, &d.unit, &d.required, &d.options, &d.sort); err != nil {
			rows.Close()
			return nil, err
		}
		key := strings.TrimSpace(d.key)
		if key != "" {
			defs[key] = d
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(defs) == 0 && len(fields) > 0 {
		return nil, fmt.Errorf("industry field template has no fields")
	}
	out := make([]catalogapp.ProductProductionConfigField, 0, len(fields))
	for _, field := range fields {
		key := strings.TrimSpace(field.TemplateFieldKey)
		if key == "" {
			key = strings.TrimSpace(field.FieldKey)
		}
		d, ok := defs[key]
		if !ok {
			return nil, fmt.Errorf("field %s is not defined by industry field template", key)
		}
		field.FieldKey = d.key
		field.TemplateFieldKey = d.key
		field.Label = d.label
		field.FieldType = d.kind
		field.Unit = d.unit
		field.Required = d.required
		field.OptionsJSON = d.options
		field.SortOrder = d.sort
		out = append(out, field)
	}
	return out, nil
}

func (r Repository) ListProductClassificationTemplates(ctx context.Context) ([]catalogapp.ProductClassificationTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(customer_id,0), COALESCE(source_template_id,0),
		       COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END),
			       name, COALESCE(remark,''), COALESCE(product_config_template_id,0), COALESCE(gradient_template_id,0), COALESCE(unit_template_id,0), active, COALESCE(sort_order,100)
		FROM %s.product_classification_templates
		WHERE active=true AND deleted_at IS NULL
		ORDER BY COALESCE(customer_id,0), COALESCE(sort_order,100), name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductClassificationTemplate, 0)
	index := map[int64]int{}
	for rows.Next() {
		var row catalogapp.ProductClassificationTemplate
		if err := rows.Scan(&row.ID, &row.CustomerID, &row.SourceTemplateID, &row.TemplateState, &row.Name, &row.Remark, &row.ProductConfigTemplateID, &row.GradientTemplateID, &row.UnitTemplateID, &row.Active, &row.SortOrder); err != nil {
			return nil, err
		}
		row.Categories = []catalogapp.ProductClassificationCategory{}
		row.ProductAssignments = []catalogapp.ProductClassificationAssignment{}
		row.CustomerAliasAssignments = []catalogapp.CustomerProductAliasClassificationAssignment{}
		index[row.ID] = len(out)
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
	categoryRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, COALESCE(parent_id,0), name, COALESCE(level,1), COALESCE(sort_order,100), COALESCE(product_config_template_id,0), COALESCE(gradient_template_id,0), COALESCE(unit_template_id,0), active
		FROM %s.product_classification_template_categories
		WHERE active=true AND template_id=ANY($1)
		ORDER BY template_id, COALESCE(sort_order,100), id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	for categoryRows.Next() {
		var row catalogapp.ProductClassificationCategory
		if err := categoryRows.Scan(&row.ID, &row.TemplateID, &row.ParentID, &row.Name, &row.Level, &row.SortOrder, &row.ProductConfigTemplateID, &row.GradientTemplateID, &row.UnitTemplateID, &row.Active); err != nil {
			categoryRows.Close()
			return nil, err
		}
		if i, ok := index[row.TemplateID]; ok {
			out[i].Categories = append(out[i].Categories, row)
		}
	}
	if err := categoryRows.Err(); err != nil {
		categoryRows.Close()
		return nil, err
	}
	categoryRows.Close()

	assignmentRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT product_id, template_id, COALESCE(category_id,0), COALESCE(sort_order,100)
		FROM %s.product_classification_assignments
		WHERE template_id=ANY($1)
		ORDER BY template_id, COALESCE(category_id,0), COALESCE(sort_order,100), product_id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	for assignmentRows.Next() {
		var row catalogapp.ProductClassificationAssignment
		if err := assignmentRows.Scan(&row.ProductID, &row.TemplateID, &row.CategoryID, &row.SortOrder); err != nil {
			assignmentRows.Close()
			return nil, err
		}
		if i, ok := index[row.TemplateID]; ok {
			out[i].ProductAssignments = append(out[i].ProductAssignments, row)
		}
	}
	if err := assignmentRows.Err(); err != nil {
		assignmentRows.Close()
		return nil, err
	}
	assignmentRows.Close()

	aliasRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT alias_id, template_id, COALESCE(category_id,0), COALESCE(sort_order,100)
		FROM %s.customer_product_alias_classification_assignments
		WHERE template_id=ANY($1)
		ORDER BY template_id, COALESCE(category_id,0), COALESCE(sort_order,100), alias_id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	for aliasRows.Next() {
		var row catalogapp.CustomerProductAliasClassificationAssignment
		if err := aliasRows.Scan(&row.AliasID, &row.TemplateID, &row.CategoryID, &row.SortOrder); err != nil {
			aliasRows.Close()
			return nil, err
		}
		if i, ok := index[row.TemplateID]; ok {
			out[i].CustomerAliasAssignments = append(out[i].CustomerAliasAssignments, row)
		}
	}
	if err := aliasRows.Err(); err != nil {
		aliasRows.Close()
		return nil, err
	}
	aliasRows.Close()
	return out, nil
}

func (r Repository) SaveProductClassificationTemplate(ctx context.Context, cmd catalogapp.SaveProductClassificationTemplateCommand) (catalogapp.ProductClassificationTemplate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return catalogapp.ProductClassificationTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id := cmd.ID
	action := "create_product_classification_template"
	if id > 0 {
		action = "update_product_classification_template"
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_classification_templates
				SET customer_id=$2, source_template_id=$3, name=$4, remark=$5, product_config_template_id=$6, gradient_template_id=$7, unit_template_id=$8, active=$9, sort_order=$10, updated_at=now(), updated_by=$11
				WHERE id=$1
				RETURNING id
			`, r.schema), id, cmd.CustomerID, cmd.SourceTemplateID, cmd.Name, cmd.Remark, cmd.ProductConfigTemplateID, cmd.GradientTemplateID, cmd.UnitTemplateID, cmd.Active, cmd.SortOrder, cmd.Actor).Scan(&id); err != nil {
			return catalogapp.ProductClassificationTemplate{}, err
		}
	} else {
		templateState := "public_template"
		if cmd.CustomerID > 0 {
			templateState = "customer_owned"
		}
		if cmd.SourceTemplateID > 0 {
			templateState = "derived_from_public"
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
				INSERT INTO %s.product_classification_templates(customer_id, source_template_id, template_state, name, remark, product_config_template_id, gradient_template_id, unit_template_id, active, sort_order, created_by, updated_by)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)
				RETURNING id
			`, r.schema), cmd.CustomerID, cmd.SourceTemplateID, templateState, cmd.Name, cmd.Remark, cmd.ProductConfigTemplateID, cmd.GradientTemplateID, cmd.UnitTemplateID, cmd.Active, cmd.SortOrder, cmd.Actor).Scan(&id); err != nil {
			return catalogapp.ProductClassificationTemplate{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_template", &id, action, postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"template_id": id, "customer_id": cmd.CustomerID, "source_template_id": cmd.SourceTemplateID, "product_config_template_id": cmd.ProductConfigTemplateID, "gradient_template_id": cmd.GradientTemplateID, "unit_template_id": cmd.UnitTemplateID, "sort_order": cmd.SortOrder, "active": cmd.Active, "remark": cmd.Remark}); err != nil {
		return catalogapp.ProductClassificationTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductClassificationTemplate{}, err
	}
	rows, err := r.ListProductClassificationTemplates(ctx)
	if err != nil {
		return catalogapp.ProductClassificationTemplate{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return catalogapp.ProductClassificationTemplate{}, fmt.Errorf("classification template not found")
}

func (r Repository) DeleteProductClassificationTemplate(ctx context.Context, cmd catalogapp.DeleteProductClassificationTemplateCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_classification_templates
		SET active=false, deleted_at=now(), updated_at=now(), updated_by=$2
		WHERE id=$1 AND deleted_at IS NULL
	`, r.schema), cmd.ID, cmd.Actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("classification template not found")
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_template", &cmd.ID, "delete_product_classification_template", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"template_id": cmd.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListProductClassificationTemplateUsages(ctx context.Context) ([]catalogapp.ProductClassificationTemplateUsage, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT classification_template_id, active, COALESCE(sort_order,100)
		FROM %s.product_classification_template_usages
		WHERE active=true
		ORDER BY COALESCE(sort_order,100), classification_template_id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []catalogapp.ProductClassificationTemplateUsage{}
	for rows.Next() {
		var row catalogapp.ProductClassificationTemplateUsage
		if err := rows.Scan(&row.ClassificationTemplateID, &row.Active, &row.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveProductClassificationTemplateUsage(ctx context.Context, cmd catalogapp.SaveProductClassificationTemplateUsageCommand) (catalogapp.ProductClassificationTemplateUsage, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return catalogapp.ProductClassificationTemplateUsage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_classification_templates WHERE id=$1 AND active=true)`, r.schema), cmd.ClassificationTemplateID).Scan(&exists); err != nil {
		return catalogapp.ProductClassificationTemplateUsage{}, err
	}
	if !exists {
		return catalogapp.ProductClassificationTemplateUsage{}, fmt.Errorf("classification template not found")
	}
	var row catalogapp.ProductClassificationTemplateUsage
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_classification_template_usages(classification_template_id, active, sort_order, created_by, updated_by)
		VALUES($1,true,$2,$3,$3)
		ON CONFLICT(classification_template_id) DO UPDATE SET
			active=true,
			sort_order=excluded.sort_order,
			updated_by=excluded.updated_by,
			updated_at=now()
		RETURNING classification_template_id, active, sort_order
	`, r.schema), cmd.ClassificationTemplateID, cmd.SortOrder, cmd.Actor).Scan(&row.ClassificationTemplateID, &row.Active, &row.SortOrder); err != nil {
		return catalogapp.ProductClassificationTemplateUsage{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_template_usage", &cmd.ClassificationTemplateID, "save_product_classification_template_usage", postgresinfra.StrPtr("classification_template_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.ClassificationTemplateID)), postgresinfra.AuditMeta{"classification_template_id": cmd.ClassificationTemplateID, "sort_order": cmd.SortOrder}); err != nil {
		return catalogapp.ProductClassificationTemplateUsage{}, err
	}
	return row, tx.Commit(ctx)
}

func (r Repository) DeleteProductClassificationTemplateUsage(ctx context.Context, cmd catalogapp.DeleteProductClassificationTemplateUsageCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_classification_template_usages SET active=false, updated_at=now(), updated_by=$2 WHERE classification_template_id=$1`, r.schema), cmd.ClassificationTemplateID, cmd.Actor); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_template_usage", &cmd.ClassificationTemplateID, "delete_product_classification_template_usage", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"classification_template_id": cmd.ClassificationTemplateID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListCustomerProductAliasClassificationTemplateUsages(ctx context.Context, customerID int64) ([]catalogapp.CustomerProductAliasClassificationTemplateUsage, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT customer_id, classification_template_id, active, COALESCE(sort_order,100)
		FROM %s.customer_product_alias_classification_template_usages
		WHERE active=true AND ($1::bigint=0 OR customer_id=$1)
		ORDER BY customer_id, COALESCE(sort_order,100), classification_template_id
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []catalogapp.CustomerProductAliasClassificationTemplateUsage{}
	for rows.Next() {
		var row catalogapp.CustomerProductAliasClassificationTemplateUsage
		if err := rows.Scan(&row.CustomerID, &row.ClassificationTemplateID, &row.Active, &row.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd catalogapp.SaveCustomerProductAliasClassificationTemplateUsageCommand) (catalogapp.CustomerProductAliasClassificationTemplateUsage, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return catalogapp.CustomerProductAliasClassificationTemplateUsage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, r.schema), cmd.CustomerID).Scan(&exists); err != nil {
		return catalogapp.CustomerProductAliasClassificationTemplateUsage{}, err
	}
	if !exists {
		return catalogapp.CustomerProductAliasClassificationTemplateUsage{}, fmt.Errorf("customer not found")
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_classification_templates WHERE id=$1 AND active=true)`, r.schema), cmd.ClassificationTemplateID).Scan(&exists); err != nil {
		return catalogapp.CustomerProductAliasClassificationTemplateUsage{}, err
	}
	if !exists {
		return catalogapp.CustomerProductAliasClassificationTemplateUsage{}, fmt.Errorf("classification template not found")
	}
	var row catalogapp.CustomerProductAliasClassificationTemplateUsage
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_product_alias_classification_template_usages(customer_id, classification_template_id, active, sort_order, created_by, updated_by)
		VALUES($1,$2,true,$3,$4,$4)
		ON CONFLICT(customer_id, classification_template_id) DO UPDATE SET
			active=true,
			sort_order=excluded.sort_order,
			updated_by=excluded.updated_by,
			updated_at=now()
		RETURNING customer_id, classification_template_id, active, sort_order
	`, r.schema), cmd.CustomerID, cmd.ClassificationTemplateID, cmd.SortOrder, cmd.Actor).Scan(&row.CustomerID, &row.ClassificationTemplateID, &row.Active, &row.SortOrder); err != nil {
		return catalogapp.CustomerProductAliasClassificationTemplateUsage{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_alias_classification_template_usage", &cmd.CustomerID, "save_customer_alias_classification_template_usage", postgresinfra.StrPtr("classification_template_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.ClassificationTemplateID)), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "classification_template_id": cmd.ClassificationTemplateID, "sort_order": cmd.SortOrder}); err != nil {
		return catalogapp.CustomerProductAliasClassificationTemplateUsage{}, err
	}
	return row, tx.Commit(ctx)
}

func (r Repository) DeleteCustomerProductAliasClassificationTemplateUsage(ctx context.Context, cmd catalogapp.DeleteCustomerProductAliasClassificationTemplateUsageCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_product_alias_classification_template_usages SET active=false, updated_at=now(), updated_by=$3 WHERE customer_id=$1 AND classification_template_id=$2`, r.schema), cmd.CustomerID, cmd.ClassificationTemplateID, cmd.Actor); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_alias_classification_template_usage", &cmd.CustomerID, "delete_customer_alias_classification_template_usage", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "classification_template_id": cmd.ClassificationTemplateID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) SaveProductClassificationCategory(ctx context.Context, cmd catalogapp.SaveProductClassificationCategoryCommand) (catalogapp.ProductClassificationCategory, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return catalogapp.ProductClassificationCategory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id := cmd.ID
	action := "create_product_classification_category"
	if id > 0 {
		action = "update_product_classification_category"
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_classification_template_categories
			SET template_id=$2, parent_id=$3, name=$4, level=$5, sort_order=$6, product_config_template_id=$7, gradient_template_id=$8, unit_template_id=$9, active=true, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), id, cmd.TemplateID, cmd.ParentID, cmd.Name, cmd.Level, cmd.SortOrder, cmd.ProductConfigTemplateID, cmd.GradientTemplateID, cmd.UnitTemplateID).Scan(&id); err != nil {
			return catalogapp.ProductClassificationCategory{}, err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_classification_template_categories(template_id, parent_id, name, level, sort_order, product_config_template_id, gradient_template_id, unit_template_id, active)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,true)
			RETURNING id
		`, r.schema), cmd.TemplateID, cmd.ParentID, cmd.Name, cmd.Level, cmd.SortOrder, cmd.ProductConfigTemplateID, cmd.GradientTemplateID, cmd.UnitTemplateID).Scan(&id); err != nil {
			return catalogapp.ProductClassificationCategory{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_category", &id, action, postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"category_id": id, "template_id": cmd.TemplateID, "parent_id": cmd.ParentID, "product_config_template_id": cmd.ProductConfigTemplateID, "gradient_template_id": cmd.GradientTemplateID, "unit_template_id": cmd.UnitTemplateID, "sort_order": cmd.SortOrder}); err != nil {
		return catalogapp.ProductClassificationCategory{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductClassificationCategory{}, err
	}
	return catalogapp.ProductClassificationCategory{ID: id, TemplateID: cmd.TemplateID, ParentID: cmd.ParentID, Name: cmd.Name, Level: cmd.Level, SortOrder: cmd.SortOrder, ProductConfigTemplateID: cmd.ProductConfigTemplateID, GradientTemplateID: cmd.GradientTemplateID, UnitTemplateID: cmd.UnitTemplateID, Active: true}, nil
}

func (r Repository) DeleteProductClassificationCategory(ctx context.Context, cmd catalogapp.DeleteProductClassificationCategoryCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_classification_template_categories
		SET active=false, updated_at=now()
		WHERE id=$1 AND template_id=$2 AND active=true
	`, r.schema), cmd.ID, cmd.TemplateID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("classification category not found")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_classification_assignments SET category_id=0, updated_at=now(), updated_by=$3 WHERE template_id=$1 AND category_id=$2`, r.schema), cmd.TemplateID, cmd.ID, cmd.Actor); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_product_alias_classification_assignments SET category_id=0, updated_at=now(), updated_by=$3 WHERE template_id=$1 AND category_id=$2`, r.schema), cmd.TemplateID, cmd.ID, cmd.Actor); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_category", &cmd.ID, "delete_product_classification_category", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"category_id": cmd.ID, "template_id": cmd.TemplateID, "objects_returned_to_uncategorized": true}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) SaveProductClassificationAssignment(ctx context.Context, cmd catalogapp.SaveProductClassificationAssignmentCommand) (catalogapp.ProductClassificationAssignment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return catalogapp.ProductClassificationAssignment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var usageExists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_classification_template_usages WHERE classification_template_id=$1 AND active=true)`, r.schema), cmd.TemplateID).Scan(&usageExists); err != nil {
		return catalogapp.ProductClassificationAssignment{}, err
	}
	if !usageExists {
		return catalogapp.ProductClassificationAssignment{}, fmt.Errorf("classification template usage not enabled")
	}
	if cmd.CategoryID > 0 {
		var categoryExists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_classification_template_categories WHERE id=$1 AND template_id=$2 AND active=true)`, r.schema), cmd.CategoryID, cmd.TemplateID).Scan(&categoryExists); err != nil {
			return catalogapp.ProductClassificationAssignment{}, err
		}
		if !categoryExists {
			return catalogapp.ProductClassificationAssignment{}, fmt.Errorf("classification category not found")
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_classification_assignments WHERE product_id=$1 AND template_id<>$2`, r.schema), cmd.ProductID, cmd.TemplateID); err != nil {
		return catalogapp.ProductClassificationAssignment{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_classification_assignments(product_id, template_id, category_id, sort_order, updated_by, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,now(),now())
		ON CONFLICT(product_id, template_id) DO UPDATE SET
			category_id=excluded.category_id,
			sort_order=excluded.sort_order,
			updated_by=excluded.updated_by,
			updated_at=now()
	`, r.schema), cmd.ProductID, cmd.TemplateID, cmd.CategoryID, cmd.SortOrder, cmd.Actor); err != nil {
		return catalogapp.ProductClassificationAssignment{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_assignment", &cmd.ProductID, "save_product_classification_assignment", postgresinfra.StrPtr("category_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.CategoryID)), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "template_id": cmd.TemplateID, "category_id": cmd.CategoryID, "sort_order": cmd.SortOrder}); err != nil {
		return catalogapp.ProductClassificationAssignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductClassificationAssignment{}, err
	}
	return catalogapp.ProductClassificationAssignment{ProductID: cmd.ProductID, TemplateID: cmd.TemplateID, CategoryID: cmd.CategoryID, SortOrder: cmd.SortOrder}, nil
}

func (r Repository) SaveCustomerProductAliasClassificationAssignment(ctx context.Context, cmd catalogapp.SaveCustomerProductAliasClassificationAssignmentCommand) (catalogapp.CustomerProductAliasClassificationAssignment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return catalogapp.CustomerProductAliasClassificationAssignment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var customerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id FROM %s.customer_product_aliases WHERE id=$1 AND active=true`, r.schema), cmd.AliasID).Scan(&customerID); err != nil {
		if err == pgx.ErrNoRows {
			return catalogapp.CustomerProductAliasClassificationAssignment{}, fmt.Errorf("customer product alias not found")
		}
		return catalogapp.CustomerProductAliasClassificationAssignment{}, err
	}
	var usageExists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customer_product_alias_classification_template_usages WHERE customer_id=$1 AND classification_template_id=$2 AND active=true)`, r.schema), customerID, cmd.TemplateID).Scan(&usageExists); err != nil {
		return catalogapp.CustomerProductAliasClassificationAssignment{}, err
	}
	if !usageExists {
		return catalogapp.CustomerProductAliasClassificationAssignment{}, fmt.Errorf("classification template usage not enabled")
	}
	if cmd.CategoryID > 0 {
		var categoryExists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_classification_template_categories WHERE id=$1 AND template_id=$2 AND active=true)`, r.schema), cmd.CategoryID, cmd.TemplateID).Scan(&categoryExists); err != nil {
			return catalogapp.CustomerProductAliasClassificationAssignment{}, err
		}
		if !categoryExists {
			return catalogapp.CustomerProductAliasClassificationAssignment{}, fmt.Errorf("classification category not found")
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.customer_product_alias_classification_assignments WHERE alias_id=$1 AND template_id<>$2`, r.schema), cmd.AliasID, cmd.TemplateID); err != nil {
		return catalogapp.CustomerProductAliasClassificationAssignment{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_product_alias_classification_assignments(alias_id, template_id, category_id, sort_order, updated_by, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,now(),now())
		ON CONFLICT(alias_id, template_id) DO UPDATE SET
			category_id=excluded.category_id,
			sort_order=excluded.sort_order,
			updated_by=excluded.updated_by,
			updated_at=now()
	`, r.schema), cmd.AliasID, cmd.TemplateID, cmd.CategoryID, cmd.SortOrder, cmd.Actor); err != nil {
		return catalogapp.CustomerProductAliasClassificationAssignment{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_alias_classification_assignment", &cmd.AliasID, "save_customer_product_alias_classification_assignment", postgresinfra.StrPtr("category_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.CategoryID)), postgresinfra.AuditMeta{"alias_id": cmd.AliasID, "template_id": cmd.TemplateID, "category_id": cmd.CategoryID, "sort_order": cmd.SortOrder}); err != nil {
		return catalogapp.CustomerProductAliasClassificationAssignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.CustomerProductAliasClassificationAssignment{}, err
	}
	return catalogapp.CustomerProductAliasClassificationAssignment{AliasID: cmd.AliasID, TemplateID: cmd.TemplateID, CategoryID: cmd.CategoryID, SortOrder: cmd.SortOrder}, nil
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
		WHERE deleted_at IS NULL
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
		WHERE deleted_at IS NULL
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
		       COALESCE(unit_conversion_json::text,'{}'), COALESCE(sales_specs_json::text,'[]'), integer_unit, active
		FROM %s.product_unit_templates
		WHERE deleted_at IS NULL
		ORDER BY active DESC, name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductUnitTemplate, 0)
	for rows.Next() {
		var row catalogapp.ProductUnitTemplate
		var salesSpecsJSON string
		if err := rows.Scan(&row.ID, &row.Name, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &salesSpecsJSON, &row.IntegerUnit, &row.Active); err != nil {
			return nil, err
		}
		row.SalesSpecs = productSalesSpecsFromJSON(salesSpecsJSON)
		row.SalesUnit = firstNonEmptyString(row.OrderUnit, row.QuoteUnit, row.InventoryUnit)
		row.DefaultSalesUnit = row.SalesUnit
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListProductPriceGroups(ctx context.Context) ([]catalogapp.ProductPriceGroup, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, COALESCE(sort_order,100), active
		FROM %s.product_price_groups
		WHERE deleted_at IS NULL
		ORDER BY active DESC, COALESCE(sort_order,100), name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductPriceGroup, 0)
	for rows.Next() {
		var row catalogapp.ProductPriceGroup
		if err := rows.Scan(&row.ID, &row.Name, &row.SortOrder, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (r Repository) SaveProductPriceGroup(ctx context.Context, cmd catalogapp.SaveProductPriceGroupCommand) (catalogapp.ProductPriceGroup, error) {
	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	var id int64
	if cmd.ID > 0 {
		if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_price_groups
			SET name=$2, sort_order=$3, active=$4, deleted_at=NULL, updated_at=now(), updated_by=$5
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.SortOrder, active, cmd.Actor).Scan(&id); err != nil {
			return catalogapp.ProductPriceGroup{}, err
		}
	} else if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_price_groups(name, sort_order, active, created_by, updated_by)
		VALUES($1,$2,$3,$4,$4)
		RETURNING id
	`, r.schema), cmd.Name, cmd.SortOrder, active, cmd.Actor).Scan(&id); err != nil {
		return catalogapp.ProductPriceGroup{}, err
	}
	row, err := r.findProductPriceGroup(ctx, id)
	if err != nil {
		return catalogapp.ProductPriceGroup{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_price_group", &id, "save_product_price_group", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(row.Name), postgresinfra.AuditMeta{"sort_order": row.SortOrder, "active": row.Active})
	return row, nil
}

func (r Repository) findProductPriceGroup(ctx context.Context, id int64) (catalogapp.ProductPriceGroup, error) {
	var row catalogapp.ProductPriceGroup
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, name, COALESCE(sort_order,100), active
		FROM %s.product_price_groups
		WHERE id=$1 AND deleted_at IS NULL
	`, r.schema), id).Scan(&row.ID, &row.Name, &row.SortOrder, &row.Active)
	return row, err
}

func (r Repository) ListBusinessGroups(ctx context.Context) ([]catalogapp.BusinessGroup, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, code, remark, active, sort_order
		FROM %s.business_groups
		ORDER BY active DESC, sort_order, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.BusinessGroup, 0)
	index := map[int64]int{}
	for rows.Next() {
		var row catalogapp.BusinessGroup
		if err := rows.Scan(&row.ID, &row.Name, &row.Code, &row.Remark, &row.Active, &row.SortOrder); err != nil {
			return nil, err
		}
		row.Usages = []catalogapp.BusinessGroupUsage{}
		row.Items = []catalogapp.BusinessGroupItem{}
		index[row.ID] = len(out)
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
	usageRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, group_id, usage_key, usage_label, active
		FROM %s.business_group_usages
		WHERE group_id=ANY($1) AND active=true
		ORDER BY group_id, id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer usageRows.Close()
	for usageRows.Next() {
		var row catalogapp.BusinessGroupUsage
		if err := usageRows.Scan(&row.ID, &row.GroupID, &row.UsageKey, &row.UsageLabel, &row.Active); err != nil {
			return nil, err
		}
		if i, ok := index[row.GroupID]; ok {
			out[i].Usages = append(out[i].Usages, row)
		}
	}
	if err := usageRows.Err(); err != nil {
		return nil, err
	}
	itemRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, group_id, parent_id, name, code, remark, active, sort_order
		FROM %s.business_group_items
		WHERE group_id=ANY($1) AND active=true
		ORDER BY group_id, parent_id, sort_order, id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	itemsByGroup := map[int64][]catalogapp.BusinessGroupItem{}
	for itemRows.Next() {
		var row catalogapp.BusinessGroupItem
		if err := itemRows.Scan(&row.ID, &row.GroupID, &row.ParentID, &row.Name, &row.Code, &row.Remark, &row.Active, &row.SortOrder); err != nil {
			return nil, err
		}
		itemsByGroup[row.GroupID] = append(itemsByGroup[row.GroupID], row)
	}
	if err := itemRows.Err(); err != nil {
		return nil, err
	}
	for groupID, items := range itemsByGroup {
		if i, ok := index[groupID]; ok {
			out[i].Items = businessGroupItemTree(items)
		}
	}
	return out, nil
}

func (r Repository) SaveBusinessGroup(ctx context.Context, cmd catalogapp.BusinessGroup) (catalogapp.BusinessGroup, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.BusinessGroup{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.BusinessGroup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id int64
	if cmd.ID > 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.business_groups
			SET name=$2, code=$3, remark=$4, active=$5, sort_order=$6, updated_at=now(), updated_by=$7
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.Code, cmd.Remark, cmd.Active, cmd.SortOrder, cmd.Actor).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.business_groups(name, code, remark, active, sort_order, created_by, updated_by)
			VALUES($1,$2,$3,$4,$5,$6,$6)
			RETURNING id
		`, r.schema), cmd.Name, cmd.Code, cmd.Remark, cmd.Active, cmd.SortOrder, cmd.Actor).Scan(&id)
	}
	if err != nil {
		return catalogapp.BusinessGroup{}, err
	}
	if len(cmd.Usages) > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.business_group_usages SET active=false, updated_at=now() WHERE group_id=$1`, r.schema), id); err != nil {
			return catalogapp.BusinessGroup{}, err
		}
		for _, usage := range cmd.Usages {
			usage.UsageKey = strings.TrimSpace(usage.UsageKey)
			usage.UsageLabel = strings.TrimSpace(usage.UsageLabel)
			if usage.UsageKey == "" {
				continue
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.business_group_usages(group_id, usage_key, usage_label, active, created_by, updated_by)
				VALUES($1,$2,$3,true,$4,$4)
			`, r.schema), id, usage.UsageKey, usage.UsageLabel, cmd.Actor); err != nil {
				return catalogapp.BusinessGroup{}, err
			}
		}
	}
	if len(cmd.Items) > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.business_group_items SET active=false, updated_at=now() WHERE group_id=$1`, r.schema), id); err != nil {
			return catalogapp.BusinessGroup{}, err
		}
		flat := make([]catalogapp.BusinessGroupItem, 0)
		flattenBusinessGroupItems(id, 0, cmd.Items, &flat)
		for _, item := range flat {
			if item.Name == "" {
				continue
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.business_group_items(group_id, parent_id, name, code, remark, active, sort_order)
				VALUES($1,$2,$3,$4,$5,true,$6)
			`, r.schema), id, item.ParentID, item.Name, item.Code, item.Remark, item.SortOrder); err != nil {
				return catalogapp.BusinessGroup{}, err
			}
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "business_group", &id, "save_business_group", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"active": cmd.Active, "sort_order": cmd.SortOrder}); err != nil {
		return catalogapp.BusinessGroup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.BusinessGroup{}, err
	}
	rows, err := r.ListBusinessGroups(ctx)
	if err != nil {
		return catalogapp.BusinessGroup{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return catalogapp.BusinessGroup{}, fmt.Errorf("business group not found")
}

func (r Repository) DeleteBusinessGroup(ctx context.Context, cmd catalogapp.DeleteBusinessGroupCommand) error {
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

	var name string
	var code string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT name, code
		FROM %s.business_groups
		WHERE id=$1
	`, r.schema), cmd.ID).Scan(&name, &code); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if protectedBusinessGroupTemplate(name, code) {
		return catalogapp.ValidationError{Message: "system business group cannot be deleted"}
	}

	assignmentsDeleted, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_group_assignments WHERE group_id=$1`, r.schema), cmd.ID)
	if err != nil {
		return err
	}
	usagesDeleted, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_group_usages WHERE group_id=$1`, r.schema), cmd.ID)
	if err != nil {
		return err
	}
	itemsDeleted, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_group_items WHERE group_id=$1`, r.schema), cmd.ID)
	if err != nil {
		return err
	}
	groupDeleted, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_groups WHERE id=$1`, r.schema), cmd.ID)
	if err != nil {
		return err
	}
	if groupDeleted.RowsAffected() == 0 {
		return nil
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "business_group", &cmd.ID, "delete_business_group", postgresinfra.StrPtr("name"), postgresinfra.StrPtr(name), nil, postgresinfra.AuditMeta{
		"code":                code,
		"assignments_deleted": assignmentsDeleted.RowsAffected(),
		"usages_deleted":      usagesDeleted.RowsAffected(),
		"items_deleted":       itemsDeleted.RowsAffected(),
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func protectedBusinessGroupTemplate(name string, code string) bool {
	normalizedCode := strings.ToLower(strings.TrimSpace(code))
	normalizedName := strings.TrimSpace(name)
	if strings.HasPrefix(normalizedCode, "default_") {
		return true
	}
	switch normalizedName {
	case "商品默认分组", "生产 BOM 默认分组", "仓库库存默认分组":
		return true
	default:
		return false
	}
}

func (r Repository) SaveBusinessGroupItem(ctx context.Context, cmd catalogapp.BusinessGroupItem) (catalogapp.BusinessGroupItem, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	name := strings.TrimSpace(cmd.Name)
	code := strings.TrimSpace(cmd.Code)
	remark := strings.TrimSpace(cmd.Remark)
	var id int64
	var oldName string
	if cmd.ID > 0 {
		var existing catalogapp.BusinessGroupItem
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id, group_id, parent_id, name, code, remark, active, sort_order
			FROM %s.business_group_items
			WHERE id=$1
		`, r.schema), cmd.ID).Scan(&existing.ID, &existing.GroupID, &existing.ParentID, &existing.Name, &existing.Code, &existing.Remark, &existing.Active, &existing.SortOrder); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return catalogapp.BusinessGroupItem{}, fmt.Errorf("business group item not found")
			}
			return catalogapp.BusinessGroupItem{}, err
		}
		if cmd.GroupID > 0 && cmd.GroupID != existing.GroupID {
			return catalogapp.BusinessGroupItem{}, fmt.Errorf("business group item group mismatch")
		}
		if err := validateBusinessGroupItemParentTx(ctx, tx, r.schema, existing.GroupID, cmd.ParentID, cmd.ID); err != nil {
			return catalogapp.BusinessGroupItem{}, err
		}
		sortOrder := cmd.SortOrder
		if sortOrder <= 0 {
			sortOrder = existing.SortOrder
		}
		oldName = existing.Name
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.business_group_items
			SET parent_id=$2, name=$3, code=$4, remark=$5, active=$6, sort_order=$7, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.ParentID, name, code, remark, existing.Active, sortOrder).Scan(&id); err != nil {
			return catalogapp.BusinessGroupItem{}, err
		}
	} else {
		if err := validateBusinessGroupItemParentTx(ctx, tx, r.schema, cmd.GroupID, cmd.ParentID, 0); err != nil {
			return catalogapp.BusinessGroupItem{}, err
		}
		sortOrder := cmd.SortOrder
		if sortOrder <= 0 {
			sortOrder, err = nextBusinessGroupItemSortOrderTx(ctx, tx, r.schema, cmd.GroupID, cmd.ParentID)
			if err != nil {
				return catalogapp.BusinessGroupItem{}, err
			}
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.business_group_items(group_id, parent_id, name, code, remark, active, sort_order)
			VALUES($1,$2,$3,$4,$5,true,$6)
			RETURNING id
		`, r.schema), cmd.GroupID, cmd.ParentID, name, code, remark, sortOrder).Scan(&id); err != nil {
			return catalogapp.BusinessGroupItem{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "business_group_item", &id, "save_business_group_item", postgresinfra.StrPtr("name"), postgresinfra.StrPtr(oldName), postgresinfra.StrPtr(name), postgresinfra.AuditMeta{"group_id": cmd.GroupID, "parent_id": cmd.ParentID, "sort_order": cmd.SortOrder}); err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	return r.businessGroupItemByID(ctx, id)
}

func (r Repository) DeleteBusinessGroupItem(ctx context.Context, cmd catalogapp.DeleteBusinessGroupItemCommand) error {
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
	var ids []int64
	var groupID int64
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		WITH RECURSIVE targets AS (
			SELECT id, group_id
			FROM %s.business_group_items
			WHERE id=$1
			UNION ALL
			SELECT child.id, child.group_id
			FROM %s.business_group_items child
			JOIN targets parent ON parent.id=child.parent_id AND parent.group_id=child.group_id
		)
		SELECT id, group_id FROM targets
	`, r.schema, r.schema), cmd.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var rowGroupID int64
		if err := rows.Scan(&id, &rowGroupID); err != nil {
			rows.Close()
			return err
		}
		if groupID == 0 {
			groupID = rowGroupID
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(ids) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.business_group_items SET active=false, updated_at=now() WHERE id=ANY($1)`, r.schema), ids); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.business_group_assignments
		SET group_item_id=0, updated_at=now(), updated_by=$2
		WHERE group_id=$1 AND group_item_id=ANY($3)
	`, r.schema), groupID, cmd.Actor, ids); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "business_group_item", &cmd.ID, "delete_business_group_item", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"group_id": groupID, "item_ids": ids}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) MoveBusinessGroupItem(ctx context.Context, cmd catalogapp.MoveBusinessGroupItemCommand) (catalogapp.BusinessGroupItem, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var groupID int64
	var oldParentID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT group_id, parent_id
		FROM %s.business_group_items
		WHERE id=$1 AND active=true
	`, r.schema), cmd.ID).Scan(&groupID, &oldParentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalogapp.BusinessGroupItem{}, fmt.Errorf("business group item not found")
		}
		return catalogapp.BusinessGroupItem{}, err
	}
	if err := validateBusinessGroupItemParentTx(ctx, tx, r.schema, groupID, cmd.ParentID, cmd.ID); err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	if err := reorderBusinessGroupItemSiblingsTx(ctx, tx, r.schema, groupID, cmd.ParentID, cmd.ID, cmd.Position); err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "business_group_item", &cmd.ID, "move_business_group_item", postgresinfra.StrPtr("parent_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", oldParentID)), postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.ParentID)), postgresinfra.AuditMeta{"group_id": groupID, "position": cmd.Position}); err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	return r.businessGroupItemByID(ctx, cmd.ID)
}

func (r Repository) EnsureBusinessGroupUsage(ctx context.Context, groupID int64, usageKey string, actor string) error {
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
	var groupOK bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_groups WHERE id=$1 AND active=true)`, r.schema), groupID).Scan(&groupOK); err != nil {
		return err
	}
	if !groupOK {
		return fmt.Errorf("business group usage mismatch")
	}
	if err := ensureBusinessGroupUsageForAssignmentTx(ctx, tx, r.schema, groupID, usageKey, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListBusinessGroupAssignments(ctx context.Context, query catalogapp.BusinessGroupAssignmentQuery) ([]catalogapp.BusinessGroupAssignment, error) {
	where := []string{"1=1"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if strings.TrimSpace(query.UsageKey) != "" {
		add("lower(bga.usage_key)=lower($%d)", strings.TrimSpace(query.UsageKey))
	}
	if strings.TrimSpace(query.ObjectKey) != "" {
		add("lower(bga.object_key)=lower($%d)", strings.TrimSpace(query.ObjectKey))
	}
	if query.ObjectID > 0 {
		add("bga.object_id=$%d", query.ObjectID)
	}
	if strings.TrimSpace(query.ObjectRef) != "" {
		add("lower(bga.object_ref)=lower($%d)", strings.TrimSpace(query.ObjectRef))
	}
	if query.GroupID > 0 {
		add("bga.group_id=$%d", query.GroupID)
	}
	if query.GroupItemID > 0 {
		add("bga.group_item_id=$%d", query.GroupItemID)
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT bga.id, bga.group_id, COALESCE(bg.name,''), bga.group_item_id, COALESCE(item.name,''),
		       COALESCE(parent.id,0), COALESCE(parent.name,''),
		       bga.usage_key, bga.object_key, bga.object_id, COALESCE(bga.object_ref,''), bga.sort_order
		FROM %s.business_group_assignments bga
		JOIN %s.business_groups bg ON bg.id=bga.group_id
		LEFT JOIN %s.business_group_items item ON item.id=bga.group_item_id
		LEFT JOIN %s.business_group_items parent ON parent.id=item.parent_id
		WHERE %s
		ORDER BY bg.sort_order, bg.id, parent.sort_order, parent.id, item.sort_order, item.id, bga.sort_order, bga.id
	`, r.schema, r.schema, r.schema, r.schema, strings.Join(where, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.BusinessGroupAssignment, 0)
	for rows.Next() {
		var row catalogapp.BusinessGroupAssignment
		if err := rows.Scan(&row.ID, &row.GroupID, &row.GroupName, &row.GroupItemID, &row.GroupItemName, &row.ParentGroupItemID, &row.ParentGroupItemName, &row.UsageKey, &row.ObjectKey, &row.ObjectID, &row.ObjectRef, &row.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveBusinessGroupAssignment(ctx context.Context, cmd catalogapp.BusinessGroupAssignment) (catalogapp.BusinessGroupAssignment, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	usageKey := strings.TrimSpace(cmd.UsageKey)
	objectKey := strings.TrimSpace(cmd.ObjectKey)
	objectRef := strings.TrimSpace(cmd.ObjectRef)
	if cmd.ObjectID > 0 {
		objectRef = ""
	}
	var groupOK bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_groups WHERE id=$1 AND active=true)`, r.schema), cmd.GroupID).Scan(&groupOK); err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	if !groupOK {
		return catalogapp.BusinessGroupAssignment{}, fmt.Errorf("business group usage mismatch")
	}
	var itemOK bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_group_items WHERE id=$1 AND group_id=$2 AND active=true)`, r.schema), cmd.GroupItemID, cmd.GroupID).Scan(&itemOK); err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	if !itemOK {
		return catalogapp.BusinessGroupAssignment{}, fmt.Errorf("business group item mismatch")
	}
	if err := ensureBusinessGroupUsageForAssignmentTx(ctx, tx, r.schema, cmd.GroupID, usageKey, cmd.Actor); err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	if cmd.ID > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_group_assignments WHERE id=$1`, r.schema), cmd.ID); err != nil {
			return catalogapp.BusinessGroupAssignment{}, err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.business_group_assignments
		WHERE lower(usage_key)=lower($1) AND lower(object_key)=lower($2) AND object_id=$3 AND lower(object_ref)=lower($4)
	`, r.schema), usageKey, objectKey, cmd.ObjectID, objectRef); err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.business_group_assignments(group_id, group_item_id, usage_key, object_key, object_id, object_ref, sort_order, created_by, updated_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8)
		RETURNING id
	`, r.schema), cmd.GroupID, cmd.GroupItemID, usageKey, objectKey, cmd.ObjectID, objectRef, cmd.SortOrder, cmd.Actor).Scan(&id); err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "business_group_assignment", &id, "save_business_group_assignment", postgresinfra.StrPtr("group_item_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.GroupItemID)), postgresinfra.AuditMeta{"group_id": cmd.GroupID, "group_item_id": cmd.GroupItemID, "usage_key": usageKey, "object_key": objectKey, "object_id": cmd.ObjectID, "object_ref": objectRef}); err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	rows, err := r.ListBusinessGroupAssignments(ctx, catalogapp.BusinessGroupAssignmentQuery{UsageKey: usageKey, ObjectKey: objectKey, ObjectID: cmd.ObjectID, ObjectRef: objectRef})
	if err != nil {
		return catalogapp.BusinessGroupAssignment{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return catalogapp.BusinessGroupAssignment{}, fmt.Errorf("business group assignment not found")
}

func (r Repository) DeleteBusinessGroupAssignment(ctx context.Context, cmd catalogapp.DeleteBusinessGroupAssignmentCommand) error {
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
	var row catalogapp.BusinessGroupAssignment
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id, group_id, group_item_id, usage_key, object_key, object_id, COALESCE(object_ref,''), sort_order FROM %s.business_group_assignments WHERE id=$1`, r.schema), cmd.ID).Scan(&row.ID, &row.GroupID, &row.GroupItemID, &row.UsageKey, &row.ObjectKey, &row.ObjectID, &row.ObjectRef, &row.SortOrder); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_group_assignments WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "business_group_assignment", &cmd.ID, "delete_business_group_assignment", postgresinfra.StrPtr("group_item_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", row.GroupItemID)), nil, postgresinfra.AuditMeta{"group_id": row.GroupID, "group_item_id": row.GroupItemID, "usage_key": row.UsageKey, "object_key": row.ObjectKey, "object_id": row.ObjectID, "object_ref": row.ObjectRef}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func businessGroupItemTree(items []catalogapp.BusinessGroupItem) []catalogapp.BusinessGroupItem {
	nodes := map[int64]catalogapp.BusinessGroupItem{}
	childrenByParent := map[int64][]int64{}
	rootIDs := make([]int64, 0)
	for _, item := range items {
		item.Children = []catalogapp.BusinessGroupItem{}
		nodes[item.ID] = item
	}
	for _, item := range items {
		if item.ParentID > 0 {
			if _, ok := nodes[item.ParentID]; ok && item.ParentID != item.ID {
				childrenByParent[item.ParentID] = append(childrenByParent[item.ParentID], item.ID)
				continue
			}
		}
		rootIDs = append(rootIDs, item.ID)
	}
	var build func(id int64, seen map[int64]bool) catalogapp.BusinessGroupItem
	build = func(id int64, seen map[int64]bool) catalogapp.BusinessGroupItem {
		row := nodes[id]
		if seen[id] {
			row.Children = []catalogapp.BusinessGroupItem{}
			return row
		}
		nextSeen := make(map[int64]bool, len(seen)+1)
		for key, value := range seen {
			nextSeen[key] = value
		}
		nextSeen[id] = true
		row.Children = make([]catalogapp.BusinessGroupItem, 0, len(childrenByParent[id]))
		for _, childID := range childrenByParent[id] {
			row.Children = append(row.Children, build(childID, nextSeen))
		}
		return row
	}
	roots := make([]catalogapp.BusinessGroupItem, 0, len(rootIDs))
	for _, id := range rootIDs {
		roots = append(roots, build(id, map[int64]bool{}))
	}
	return roots
}

func flattenBusinessGroupItems(groupID, parentID int64, items []catalogapp.BusinessGroupItem, out *[]catalogapp.BusinessGroupItem) {
	for i, item := range items {
		item.GroupID = groupID
		item.ParentID = parentID
		item.Name = strings.TrimSpace(item.Name)
		item.Code = strings.TrimSpace(item.Code)
		item.Remark = strings.TrimSpace(item.Remark)
		if item.SortOrder <= 0 {
			item.SortOrder = (i + 1) * 10
		}
		*out = append(*out, item)
		// Newly saved nested items are reinserted, so child parent IDs cannot be
		// preserved until the UI grows item-level edit endpoints.
		if len(item.Children) > 0 && item.ID > 0 {
			flattenBusinessGroupItems(groupID, item.ID, item.Children, out)
		}
	}
}

func (r Repository) businessGroupItemByID(ctx context.Context, id int64) (catalogapp.BusinessGroupItem, error) {
	var item catalogapp.BusinessGroupItem
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, group_id, parent_id, name, code, remark, active, sort_order
		FROM %s.business_group_items
		WHERE id=$1
	`, r.schema), id).Scan(&item.ID, &item.GroupID, &item.ParentID, &item.Name, &item.Code, &item.Remark, &item.Active, &item.SortOrder); err != nil {
		return catalogapp.BusinessGroupItem{}, err
	}
	return item, nil
}

func validateBusinessGroupItemParentTx(ctx context.Context, tx pgx.Tx, schema string, groupID int64, parentID int64, itemID int64) error {
	var groupOK bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_groups WHERE id=$1 AND active=true)`, schema), groupID).Scan(&groupOK); err != nil {
		return err
	}
	if !groupOK {
		return fmt.Errorf("business group not found")
	}
	if parentID == 0 {
		return nil
	}
	if parentID == itemID {
		return fmt.Errorf("business group item cannot be its own parent")
	}
	var parentOK bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_group_items WHERE id=$1 AND group_id=$2 AND active=true)`, schema), parentID, groupID).Scan(&parentOK); err != nil {
		return err
	}
	if !parentOK {
		return fmt.Errorf("business group parent item not found")
	}
	if itemID > 0 {
		var descendant bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			WITH RECURSIVE descendants AS (
				SELECT id
				FROM %s.business_group_items
				WHERE parent_id=$1 AND group_id=$2 AND active=true
				UNION ALL
				SELECT child.id
				FROM %s.business_group_items child
				JOIN descendants parent ON parent.id=child.parent_id
				WHERE child.group_id=$2 AND child.active=true
			)
			SELECT EXISTS(SELECT 1 FROM descendants WHERE id=$3)
		`, schema, schema), itemID, groupID, parentID).Scan(&descendant); err != nil {
			return err
		}
		if descendant {
			return fmt.Errorf("business group item cannot move under its descendant")
		}
	}
	return nil
}

func nextBusinessGroupItemSortOrderTx(ctx context.Context, tx pgx.Tx, schema string, groupID int64, parentID int64) (int, error) {
	var maxSort int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(sort_order),0)
		FROM %s.business_group_items
		WHERE group_id=$1 AND parent_id=$2 AND active=true
	`, schema), groupID, parentID).Scan(&maxSort); err != nil {
		return 0, err
	}
	return maxSort + 10, nil
}

func reorderBusinessGroupItemSiblingsTx(ctx context.Context, tx pgx.Tx, schema string, groupID int64, parentID int64, itemID int64, position int) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.business_group_items
		WHERE group_id=$1 AND parent_id=$2 AND active=true AND id<>$3
		ORDER BY sort_order, id
	`, schema), groupID, parentID, itemID)
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
	if position < 1 {
		position = 1
	}
	if position > len(ids)+1 {
		position = len(ids) + 1
	}
	ordered := make([]int64, 0, len(ids)+1)
	ordered = append(ordered, ids[:position-1]...)
	ordered = append(ordered, itemID)
	ordered = append(ordered, ids[position-1:]...)
	for index, id := range ordered {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.business_group_items
			SET parent_id=$2, sort_order=$3, updated_at=now()
			WHERE id=$1
		`, schema), id, parentID, (index+1)*10); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) ListProductCustomerReferences(ctx context.Context, productID int64) ([]catalogapp.ProductCustomerReference, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, product_id, customer_id, customer_item_code, customer_display_name, active, remark
		FROM %s.product_customer_references
		WHERE ($1::bigint=0 OR product_id=$1)
		ORDER BY active DESC, product_id, customer_id, id
	`, r.schema), productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductCustomerReference, 0)
	for rows.Next() {
		var row catalogapp.ProductCustomerReference
		if err := rows.Scan(&row.ID, &row.ProductID, &row.CustomerID, &row.CustomerItemCode, &row.CustomerDisplayName, &row.Active, &row.Remark); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveProductCustomerReference(ctx context.Context, cmd catalogapp.ProductCustomerReference) (catalogapp.ProductCustomerReference, error) {
	var id int64
	var err error
	if cmd.ID > 0 {
		err = r.pool.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_customer_references
			SET product_id=$2, customer_id=$3, customer_item_code=$4, customer_display_name=$5, active=$6, remark=$7, updated_at=now(), updated_by=$8
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.ProductID, cmd.CustomerID, cmd.CustomerItemCode, cmd.CustomerDisplayName, cmd.Active, cmd.Remark, cmd.Actor).Scan(&id)
	} else {
		err = r.pool.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_customer_references(product_id, customer_id, customer_item_code, customer_display_name, active, remark, created_by, updated_by)
			VALUES($1,$2,$3,$4,$5,$6,$7,$7)
			RETURNING id
		`, r.schema), cmd.ProductID, cmd.CustomerID, cmd.CustomerItemCode, cmd.CustomerDisplayName, cmd.Active, cmd.Remark, cmd.Actor).Scan(&id)
	}
	if err != nil {
		return catalogapp.ProductCustomerReference{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_customer_reference", &id, "save_product_customer_reference", postgresinfra.StrPtr("customer_display_name"), nil, postgresinfra.StrPtr(cmd.CustomerDisplayName), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "customer_id": cmd.CustomerID, "customer_item_code": cmd.CustomerItemCode, "active": cmd.Active})
	rows, err := r.ListProductCustomerReferences(ctx, cmd.ProductID)
	if err != nil {
		return catalogapp.ProductCustomerReference{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return catalogapp.ProductCustomerReference{}, fmt.Errorf("product customer reference not found")
}

func (r Repository) ListProductPricingRules(ctx context.Context) ([]catalogapp.ProductPricingRule, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, code, cost_source_mode, margin_rate::float8, tax_rate::float8, rounding_mode, calculation_json, formula_version, active, remark
		FROM %s.product_pricing_rules
		ORDER BY active DESC, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductPricingRule, 0)
	for rows.Next() {
		var row catalogapp.ProductPricingRule
		var calculationJSON []byte
		if err := rows.Scan(&row.ID, &row.Name, &row.Code, &row.CostSourceMode, &row.MarginRate, &row.TaxRate, &row.RoundingMode, &calculationJSON, &row.FormulaVersion, &row.Active, &row.Remark); err != nil {
			return nil, err
		}
		row.CalculationJSON = jsonMapOrEmpty(calculationJSON)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveProductPricingRule(ctx context.Context, cmd catalogapp.ProductPricingRule) (catalogapp.ProductPricingRule, error) {
	var id int64
	var err error
	calculationJSON, marshalErr := json.Marshal(cmd.CalculationJSON)
	if marshalErr != nil {
		return catalogapp.ProductPricingRule{}, marshalErr
	}
	if cmd.ID > 0 {
		err = r.pool.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_pricing_rules
			SET name=$2, code=$3, cost_source_mode=$4, margin_rate=$5, tax_rate=$6, rounding_mode=$7, calculation_json=$8, formula_version=$9, active=$10, remark=$11, updated_at=now(), updated_by=$12
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.Code, cmd.CostSourceMode, cmd.MarginRate, cmd.TaxRate, cmd.RoundingMode, calculationJSON, cmd.FormulaVersion, cmd.Active, cmd.Remark, cmd.Actor).Scan(&id)
	} else {
		err = r.pool.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_pricing_rules(name, code, cost_source_mode, margin_rate, tax_rate, rounding_mode, calculation_json, formula_version, active, remark, created_by, updated_by)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)
			RETURNING id
		`, r.schema), cmd.Name, cmd.Code, cmd.CostSourceMode, cmd.MarginRate, cmd.TaxRate, cmd.RoundingMode, calculationJSON, cmd.FormulaVersion, cmd.Active, cmd.Remark, cmd.Actor).Scan(&id)
	}
	if err != nil {
		return catalogapp.ProductPricingRule{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_pricing_rule", &id, "save_product_pricing_rule", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"margin_rate": cmd.MarginRate, "tax_rate": cmd.TaxRate, "rounding_mode": cmd.RoundingMode, "formula_version": cmd.FormulaVersion, "active": cmd.Active})
	rows, err := r.ListProductPricingRules(ctx)
	if err != nil {
		return catalogapp.ProductPricingRule{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return catalogapp.ProductPricingRule{}, fmt.Errorf("product pricing rule not found")
}

func (r Repository) ListPriceTierTemplates(ctx context.Context) ([]catalogapp.PriceTierTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, active, remark
		FROM %s.price_tier_templates
		ORDER BY active DESC, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.PriceTierTemplate, 0)
	index := map[int64]int{}
	for rows.Next() {
		var row catalogapp.PriceTierTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.Active, &row.Remark); err != nil {
			return nil, err
		}
		row.Tiers = []catalogapp.PriceTierTemplateTier{}
		index[row.ID] = len(out)
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
		SELECT id, template_id, label, min_qty::float8, max_qty::float8, quantity_unit, pricing_rule_id, position, active, remark
		FROM %s.price_tier_template_tiers
		WHERE active=true AND template_id=ANY($1)
		ORDER BY template_id, position, min_qty, id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer tierRows.Close()
	for tierRows.Next() {
		var row catalogapp.PriceTierTemplateTier
		if err := tierRows.Scan(&row.ID, &row.TemplateID, &row.Label, &row.MinQty, &row.MaxQty, &row.QuantityUnit, &row.PricingRuleID, &row.Position, &row.Active, &row.Remark); err != nil {
			return nil, err
		}
		if i, ok := index[row.TemplateID]; ok {
			out[i].Tiers = append(out[i].Tiers, row)
		}
	}
	return out, tierRows.Err()
}

func (r Repository) SavePriceTierTemplate(ctx context.Context, cmd catalogapp.PriceTierTemplate) (catalogapp.PriceTierTemplate, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.PriceTierTemplate{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.PriceTierTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id int64
	if cmd.ID > 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.price_tier_templates
			SET name=$2, active=$3, remark=$4, updated_at=now(), updated_by=$5
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.Active, cmd.Remark, cmd.Actor).Scan(&id)
		if err == nil {
			_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.price_tier_template_tiers SET active=false, updated_at=now() WHERE template_id=$1`, r.schema), id)
		}
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.price_tier_templates(name, active, remark, created_by, updated_by)
			VALUES($1,$2,$3,$4,$4)
			RETURNING id
		`, r.schema), cmd.Name, cmd.Active, cmd.Remark, cmd.Actor).Scan(&id)
	}
	if err != nil {
		return catalogapp.PriceTierTemplate{}, err
	}
	for i := range cmd.Tiers {
		cmd.Tiers[i].TemplateID = id
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.price_tier_template_tiers(template_id, label, min_qty, max_qty, quantity_unit, pricing_rule_id, position, active, remark)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, r.schema), id, cmd.Tiers[i].Label, cmd.Tiers[i].MinQty, cmd.Tiers[i].MaxQty, cmd.Tiers[i].QuantityUnit, cmd.Tiers[i].PricingRuleID, cmd.Tiers[i].Position, cmd.Tiers[i].Active, cmd.Tiers[i].Remark); err != nil {
			return catalogapp.PriceTierTemplate{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "price_tier_template", &id, "save_price_tier_template", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{"tier_count": len(cmd.Tiers), "active": cmd.Active}); err != nil {
		return catalogapp.PriceTierTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.PriceTierTemplate{}, err
	}
	rows, err := r.ListPriceTierTemplates(ctx)
	if err != nil {
		return catalogapp.PriceTierTemplate{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return catalogapp.PriceTierTemplate{}, fmt.Errorf("price tier template not found")
}

func (r Repository) DeletePriceTierTemplate(ctx context.Context, id int64, actor string) error {
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
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.price_tier_templates
		SET active=false, updated_at=now(), updated_by=$2
		WHERE id=$1
	`, r.schema), id, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("price tier template not found")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.price_tier_template_tiers
		SET active=false, updated_at=now()
		WHERE template_id=$1
	`, r.schema), id); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "price_tier_template", &id, "delete_price_tier_template", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListProductPriceRecords(ctx context.Context, query catalogapp.ProductPriceRecordQuery) ([]catalogapp.ProductPriceRecord, error) {
	activeMode := strings.ToLower(strings.TrimSpace(query.ActiveMode))
	status := strings.TrimSpace(query.Status)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(product_id,0), COALESCE(customer_product_alias_id,0),
		       final_unit_price::float8, price_unit, currency,
		       COALESCE(price_group_id,0), price_group_name, inventory_unit,
		       COALESCE(inventory_conversion_json::text,'{}'), status, remark, active
		FROM %s.product_price_records
		WHERE ($1::bigint=0 OR product_id=$1)
		  AND ($2::bigint=0 OR customer_product_alias_id=$2)
		  AND ($3::bigint=0 OR price_group_id=$3)
		  AND ($4::text='' OR status=$4)
		  AND (CASE WHEN $5::text='all' THEN true WHEN $5::text='inactive' THEN active=false ELSE active=true END)
		ORDER BY active DESC, price_group_id, product_id, customer_product_alias_id, id
	`, r.schema), query.ProductID, query.CustomerProductAliasID, query.PriceGroupID, status, activeMode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductPriceRecord, 0)
	for rows.Next() {
		var row catalogapp.ProductPriceRecord
		if err := rows.Scan(&row.ID, &row.ProductID, &row.CustomerProductAliasID, &row.FinalUnitPrice, &row.PriceUnit, &row.Currency, &row.PriceGroupID, &row.PriceGroupName, &row.InventoryUnit, &row.InventoryConversionJSON, &row.Status, &row.Remark, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) GetProductPriceRecord(ctx context.Context, id int64) (catalogapp.ProductPriceRecord, error) {
	return fetchProductPriceRecordByID(ctx, r.pool, r.schema, id)
}

func (r Repository) SaveProductPriceRecord(ctx context.Context, cmd catalogapp.SaveProductPriceRecordCommand) (catalogapp.ProductPriceRecord, error) {
	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	var id int64
	if cmd.ID > 0 {
		if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_price_records
			SET product_id=$2,
			    customer_product_alias_id=$3,
			    final_unit_price=$4,
			    price_unit=$5,
			    currency=$6,
			    price_group_id=$7,
			    price_group_name=$8,
			    inventory_unit=$9,
			    inventory_conversion_json=$10::jsonb,
			    status=$11,
			    active=$12,
			    remark=$13,
			    updated_at=now(),
			    updated_by=$14
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.ProductID, cmd.CustomerProductAliasID, cmd.FinalUnitPrice, cmd.PriceUnit, cmd.Currency, cmd.PriceGroupID, cmd.PriceGroupName, cmd.InventoryUnit, cmd.InventoryConversionJSON, cmd.Status, active, cmd.Remark, cmd.Actor).Scan(&id); err != nil {
			return catalogapp.ProductPriceRecord{}, err
		}
	} else if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_price_records(
			product_id, customer_product_alias_id, final_unit_price, price_unit, currency,
			price_group_id, price_group_name, inventory_unit, inventory_conversion_json,
			status, active, remark, created_by, updated_by
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12,$13,$13)
		RETURNING id
	`, r.schema), cmd.ProductID, cmd.CustomerProductAliasID, cmd.FinalUnitPrice, cmd.PriceUnit, cmd.Currency, cmd.PriceGroupID, cmd.PriceGroupName, cmd.InventoryUnit, cmd.InventoryConversionJSON, cmd.Status, active, cmd.Remark, cmd.Actor).Scan(&id); err != nil {
		return catalogapp.ProductPriceRecord{}, err
	}
	row, err := fetchProductPriceRecordByID(ctx, r.pool, r.schema, id)
	if err != nil {
		return catalogapp.ProductPriceRecord{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_price_record", &id, "save_product_price_record", postgresinfra.StrPtr("final_unit_price"), nil, postgresinfra.StrPtr(fmt.Sprintf("%.4f", row.FinalUnitPrice)), postgresinfra.AuditMeta{
		"product_id":                row.ProductID,
		"customer_product_alias_id": row.CustomerProductAliasID,
		"price_unit":                row.PriceUnit,
		"currency":                  row.Currency,
		"price_group_id":            row.PriceGroupID,
		"price_group_name":          row.PriceGroupName,
		"inventory_unit":            row.InventoryUnit,
		"inventory_conversion_json": row.InventoryConversionJSON,
		"status":                    row.Status,
		"active":                    row.Active,
	})
	return row, nil
}

func fetchProductPriceRecordByID(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, schema string, id int64) (catalogapp.ProductPriceRecord, error) {
	var row catalogapp.ProductPriceRecord
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(product_id,0), COALESCE(customer_product_alias_id,0),
		       final_unit_price::float8, price_unit, currency,
		       COALESCE(price_group_id,0), price_group_name, inventory_unit,
		       COALESCE(inventory_conversion_json::text,'{}'), status, remark, active
		FROM %s.product_price_records
		WHERE id=$1
	`, schema), id).Scan(&row.ID, &row.ProductID, &row.CustomerProductAliasID, &row.FinalUnitPrice, &row.PriceUnit, &row.Currency, &row.PriceGroupID, &row.PriceGroupName, &row.InventoryUnit, &row.InventoryConversionJSON, &row.Status, &row.Remark, &row.Active)
	return row, err
}

func (r Repository) ListProductTierPriceSchemes(ctx context.Context, query catalogapp.ProductTierPriceSchemeQuery) ([]catalogapp.ProductTierPriceScheme, error) {
	activeMode := strings.ToLower(strings.TrimSpace(query.ActiveMode))
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, COALESCE(product_id,0), COALESCE(customer_product_alias_id,0),
		       COALESCE(price_group_id,0), active, remark
		FROM %s.product_tier_price_schemes
		WHERE ($1::bigint=0 OR product_id=$1)
		  AND ($2::bigint=0 OR customer_product_alias_id=$2)
		  AND ($3::bigint=0 OR price_group_id=$3)
		  AND (CASE WHEN $4::text='all' THEN true WHEN $4::text='inactive' THEN active=false ELSE active=true END)
		ORDER BY active DESC, price_group_id, product_id, customer_product_alias_id, name, id
	`, r.schema), query.ProductID, query.CustomerProductAliasID, query.PriceGroupID, activeMode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.ProductTierPriceScheme, 0)
	index := map[int64]int{}
	for rows.Next() {
		var row catalogapp.ProductTierPriceScheme
		if err := rows.Scan(&row.ID, &row.Name, &row.ProductID, &row.CustomerProductAliasID, &row.PriceGroupID, &row.Active, &row.Remark); err != nil {
			return nil, err
		}
		row.Tiers = []catalogapp.ProductTierPriceSchemeTier{}
		index[row.ID] = len(out)
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
		SELECT id, scheme_id, label, min_qty::float8, max_qty::float8,
		       source_price_record_id, final_unit_price, price_unit, currency, position
		FROM %s.product_tier_price_scheme_tiers
		WHERE active=true AND scheme_id=ANY($1)
		ORDER BY scheme_id, position, min_qty, id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer tierRows.Close()
	for tierRows.Next() {
		var tier catalogapp.ProductTierPriceSchemeTier
		if err := tierRows.Scan(&tier.ID, &tier.SchemeID, &tier.Label, &tier.MinQty, &tier.MaxQty, &tier.SourcePriceRecordID, &tier.FinalUnitPrice, &tier.PriceUnit, &tier.Currency, &tier.Position); err != nil {
			return nil, err
		}
		if i, ok := index[tier.SchemeID]; ok {
			out[i].Tiers = append(out[i].Tiers, tier)
		}
	}
	return out, tierRows.Err()
}

func (r Repository) SaveProductTierPriceScheme(ctx context.Context, cmd catalogapp.SaveProductTierPriceSchemeCommand) (catalogapp.ProductTierPriceScheme, error) {
	active := true
	if cmd.Active != nil {
		active = *cmd.Active
	}
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.ProductTierPriceScheme{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.ProductTierPriceScheme{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_tier_price_schemes
			SET name=$2,
			    product_id=$3,
			    customer_product_alias_id=$4,
			    price_group_id=$5,
			    active=$6,
			    remark=$7,
			    updated_at=now(),
			    updated_by=$8
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.ProductID, cmd.CustomerProductAliasID, cmd.PriceGroupID, active, cmd.Remark, cmd.Actor).Scan(&id); err != nil {
			return catalogapp.ProductTierPriceScheme{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_tier_price_scheme_tiers SET active=false, updated_at=now() WHERE scheme_id=$1`, r.schema), id); err != nil {
			return catalogapp.ProductTierPriceScheme{}, err
		}
	} else if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_tier_price_schemes(name, product_id, customer_product_alias_id, price_group_id, active, remark, created_by, updated_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$7)
		RETURNING id
	`, r.schema), cmd.Name, cmd.ProductID, cmd.CustomerProductAliasID, cmd.PriceGroupID, active, cmd.Remark, cmd.Actor).Scan(&id); err != nil {
		return catalogapp.ProductTierPriceScheme{}, err
	}
	insertTier := fmt.Sprintf(`
		INSERT INTO %s.product_tier_price_scheme_tiers(
			scheme_id, label, min_qty, max_qty, source_price_record_id, final_unit_price, price_unit, currency, position, active
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,true)
		RETURNING id
	`, r.schema)
	for i := range cmd.Tiers {
		cmd.Tiers[i].SchemeID = id
		if err := tx.QueryRow(ctx, insertTier, id, cmd.Tiers[i].Label, cmd.Tiers[i].MinQty, cmd.Tiers[i].MaxQty, cmd.Tiers[i].SourcePriceRecordID, cmd.Tiers[i].FinalUnitPrice, cmd.Tiers[i].PriceUnit, cmd.Tiers[i].Currency, cmd.Tiers[i].Position).Scan(&cmd.Tiers[i].ID); err != nil {
			return catalogapp.ProductTierPriceScheme{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_tier_price_scheme", &id, "save_product_tier_price_scheme", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{
		"product_id":                cmd.ProductID,
		"customer_product_alias_id": cmd.CustomerProductAliasID,
		"price_group_id":            cmd.PriceGroupID,
		"tier_count":                len(cmd.Tiers),
		"source_price_record_count": len(cmd.Tiers),
		"active":                    active,
	}); err != nil {
		return catalogapp.ProductTierPriceScheme{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductTierPriceScheme{}, err
	}
	rows, err := r.ListProductTierPriceSchemes(ctx, catalogapp.ProductTierPriceSchemeQuery{ActiveMode: "all"})
	if err != nil {
		return catalogapp.ProductTierPriceScheme{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return catalogapp.ProductTierPriceScheme{}, fmt.Errorf("product tier price scheme not found")
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
		    deleted_at=NULL,
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
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id int64
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.product_unit_templates
			SET name=$2, inventory_unit=$3, quote_unit=$4, order_unit=$5,
			    unit_conversion_json=$6::jsonb, sales_specs_json=$7::jsonb, integer_unit=$8, active=$9, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, cmd.InventoryUnit, cmd.QuoteUnit, cmd.OrderUnit, cmd.UnitConversionJSON, productSalesSpecsJSON(cmd.SalesSpecs), cmd.IntegerUnit, active).Scan(&id); err != nil {
			return catalogapp.ProductUnitTemplate{}, err
		}
	} else if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_templates(name, inventory_unit, quote_unit, order_unit, unit_conversion_json, sales_specs_json, integer_unit, active)
		VALUES($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7,$8)
		RETURNING id
	`, r.schema), cmd.Name, cmd.InventoryUnit, cmd.QuoteUnit, cmd.OrderUnit, cmd.UnitConversionJSON, productSalesSpecsJSON(cmd.SalesSpecs), cmd.IntegerUnit, active).Scan(&id); err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	if err := syncDerivedSKUsForTemplateTx(ctx, tx, r.schema, cmd.Actor, id); err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	row, err := fetchProductUnitTemplateTx(ctx, tx, r.schema, id)
	if err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	row.SalesUnit = firstNonEmptyString(row.OrderUnit, row.QuoteUnit, row.InventoryUnit)
	row.DefaultSalesUnit = row.SalesUnit
	row.SalesUnits = cmd.SalesUnits
	row.SalesSpecs = cmd.SalesSpecs
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_unit_template", &id, "update", postgresinfra.StrPtr("template"), nil, postgresinfra.StrPtr(row.Name), postgresinfra.AuditMeta{"inventory_unit": row.InventoryUnit, "default_sales_unit": row.DefaultSalesUnit, "sales_units": row.SalesUnits, "sales_specs": row.SalesSpecs, "quote_unit": row.QuoteUnit, "order_unit": row.OrderUnit, "integer_unit": row.IntegerUnit}); err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	return row, nil
}

func (r Repository) DeleteProductUnitDefinition(ctx context.Context, cmd catalogapp.DeleteProductUnitDefinitionCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var code, name, unitType string
	var allowDecimal, active bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.product_unit_definitions
		SET active=false, deleted_at=now(), updated_at=now()
		WHERE code=$1 AND deleted_at IS NULL
		RETURNING code, name, unit_type, allow_decimal, active
	`, r.schema), cmd.Code).Scan(&code, &name, &unitType, &allowDecimal, &active); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_unit_definition", nil, "delete_product_unit_definition", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"code": code, "name": name, "unit_type": unitType, "allow_decimal": allowDecimal, "active": active}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) DeleteProductUnitTemplate(ctx context.Context, cmd catalogapp.DeleteProductUnitTemplateCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var row catalogapp.ProductUnitTemplate
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.product_unit_templates
		SET active=false, deleted_at=now(), updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL
		RETURNING id, name, inventory_unit, quote_unit, order_unit, COALESCE(unit_conversion_json::text,'{}'), integer_unit, active
	`, r.schema), cmd.ID).Scan(&row.ID, &row.Name, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &row.IntegerUnit, &row.Active); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_unit_template", &cmd.ID, "delete_product_unit_template", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"template_id": row.ID, "name": row.Name, "inventory_unit": row.InventoryUnit, "quote_unit": row.QuoteUnit, "order_unit": row.OrderUnit, "integer_unit": row.IntegerUnit}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.products
		SET derived_spec_status='template_removed'
		WHERE auto_derived_sku=true AND derived_unit_template_id=$1 AND derived_spec_status<>'template_removed'
	`, r.schema), cmd.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) DeleteProductConfigTemplate(ctx context.Context, cmd catalogapp.DeleteProductConfigTemplateCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var name string
	var customerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.product_config_templates
		SET active=false, deleted_at=now(), updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL
		RETURNING name, COALESCE(customer_id,0)
	`, r.schema), cmd.ID).Scan(&name, &customerID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_config_template", &cmd.ID, "delete_product_config_template", postgresinfra.StrPtr("deleted_at"), nil, postgresinfra.StrPtr("now"), postgresinfra.AuditMeta{"template_id": cmd.ID, "name": name, "customer_id": customerID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
	deactivating := cmd.Active != nil && !*cmd.Active
	if cmd.UnitTemplateID > 0 {
		unitTemplate, err := fetchProductUnitTemplateTx(ctx, tx, r.schema, cmd.UnitTemplateID)
		if err != nil {
			return catalogapp.ProductConfigTemplate{}, err
		}
		if !unitTemplate.Active && !deactivating {
			return catalogapp.ProductConfigTemplate{}, fmt.Errorf("unit template inactive")
		}
		if !unitTemplate.Active && deactivating {
			// unit_template_inactive_skipped_for_deactivate: 状态变更保留历史单位快照，不因已删除/停用单位模板阻断停用。
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

func ensureBusinessGroupUsageForAssignmentTx(ctx context.Context, tx pgx.Tx, schema string, groupID int64, usageKey string, actor string) error {
	usageKey = strings.TrimSpace(usageKey)
	if groupID <= 0 || usageKey == "" {
		return fmt.Errorf("business group usage mismatch")
	}
	if tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.business_group_usages
		SET active=true, updated_at=now(), updated_by=$3
		WHERE group_id=$1 AND lower(usage_key)=lower($2) AND active=false
	`, schema), groupID, usageKey, actor); err != nil {
		return err
	} else if tag.RowsAffected() > 0 {
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "business_group_usage", &groupID, "ensure_business_group_usage", postgresinfra.StrPtr("usage_key"), nil, postgresinfra.StrPtr(usageKey), postgresinfra.AuditMeta{"group_id": groupID, "usage_key": usageKey}); err != nil {
			return err
		}
	}
	var activeOK bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s.business_group_usages
			WHERE group_id=$1 AND lower(usage_key)=lower($2) AND active=true
		)
	`, schema), groupID, usageKey).Scan(&activeOK); err != nil {
		return err
	}
	if activeOK {
		return nil
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.business_group_usages(group_id, usage_key, usage_label, active, created_by, updated_by)
		VALUES($1,$2,$2,true,$3,$3)
	`, schema), groupID, usageKey, actor); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "business_group_usage", &groupID, "ensure_business_group_usage", postgresinfra.StrPtr("usage_key"), nil, postgresinfra.StrPtr(usageKey), postgresinfra.AuditMeta{"group_id": groupID, "usage_key": usageKey})
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
		       display_unit, COALESCE(unit_template_id,0), COALESCE(allow_customer_resale,false), active
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
		if err := rows.Scan(&row.ID, &row.Name, &row.CustomerID, &row.SourceTemplateID, &row.TemplateState, &row.DisplayUnit, &row.UnitTemplateID, &row.AllowCustomerResale, &row.Active); err != nil {
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
			SET name=$2, display_unit=$3, unit_template_id=$4, allow_customer_resale=$5, active=true, updated_at=now()
			WHERE id=$1
			RETURNING id, COALESCE(customer_id,0), COALESCE(source_template_id,0),
			          COALESCE(NULLIF(template_state,''), CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE 'customer_owned' END)
		`, r.schema), cmd.ID, cmd.Name, cmd.DisplayUnit, cmd.UnitTemplateID, cmd.AllowCustomerResale).Scan(&id, &customerID, &sourceTemplateID, &templateState); err != nil {
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
			INSERT INTO %s.pricing_gradient_templates(name, display_unit, unit_template_id, customer_id, source_template_id, template_state, allow_customer_resale, active)
			VALUES($1,$2,$3,$4,0,$5,$6,true)
			RETURNING id, COALESCE(customer_id,0), COALESCE(source_template_id,0), template_state
		`, r.schema), cmd.Name, cmd.DisplayUnit, cmd.UnitTemplateID, customerID, templateState, cmd.AllowCustomerResale).Scan(&id, &customerID, &sourceTemplateID, &templateState); err != nil {
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
		"customer_id":           customerID,
		"display_unit":          cmd.DisplayUnit,
		"unit_template_id":      cmd.UnitTemplateID,
		"allow_customer_resale": cmd.AllowCustomerResale,
		"tier_count":            len(cmd.Tiers),
	}); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.GradientTemplate{}, err
	}
	return catalogapp.GradientTemplate{ID: id, Name: cmd.Name, CustomerID: customerID, SourceTemplateID: sourceTemplateID, TemplateState: templateState, DisplayUnit: cmd.DisplayUnit, UnitTemplateID: cmd.UnitTemplateID, AllowCustomerResale: cmd.AllowCustomerResale, Active: true, Tiers: cmd.Tiers}, nil
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
	var salesSpecsJSON string
	if err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, name, inventory_unit, quote_unit, order_unit,
		       COALESCE(unit_conversion_json::text,'{}'), COALESCE(sales_specs_json::text,'[]'), integer_unit, active
		FROM %s.product_unit_templates
		WHERE id=$1
	`, schema), id).Scan(&row.ID, &row.Name, &row.InventoryUnit, &row.QuoteUnit, &row.OrderUnit, &row.UnitConversionJSON, &salesSpecsJSON, &row.IntegerUnit, &row.Active); err != nil {
		return catalogapp.ProductUnitTemplate{}, err
	}
	row.SalesSpecs = productSalesSpecsFromJSON(salesSpecsJSON)
	row.SalesUnit = row.QuoteUnit
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
		INSERT INTO %s.pricing_gradient_templates(name, display_unit, unit_template_id, customer_id, source_template_id, template_state, allow_customer_resale, active)
		VALUES($1,$2,$3,$4,$5,$6,$7,true)
		RETURNING id
	`, schema), source.Name, source.DisplayUnit, source.UnitTemplateID, targetCustomerID, sourceTemplateIDForTarget, templateState, source.AllowCustomerResale).Scan(&id); err != nil {
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
		    template_state=$7, allow_customer_resale=$8, active=true, updated_at=now()
		WHERE id=$1
	`, schema), targetID, source.Name, source.DisplayUnit, source.UnitTemplateID, targetCustomerID, sourceTemplateIDForTarget, templateState, source.AllowCustomerResale); err != nil {
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
		INSERT INTO %s.pricing_gradient_templates(name, display_unit, unit_template_id, customer_id, source_template_id, template_state, allow_customer_resale, active)
		VALUES($1,$2,$3,$4,$5,'derived_from_public',$6,true)
		RETURNING id
	`, schema), name, source.DisplayUnit, source.UnitTemplateID, cmd.CustomerID, source.ID, source.AllowCustomerResale).Scan(&id); err != nil {
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
		       display_unit, COALESCE(unit_template_id,0), COALESCE(allow_customer_resale,false), active
		FROM %s.pricing_gradient_templates
		WHERE id=$1 AND active=true
	`, schema), id).Scan(&row.ID, &row.Name, &row.CustomerID, &row.SourceTemplateID, &row.TemplateState, &row.DisplayUnit, &row.UnitTemplateID, &row.AllowCustomerResale, &row.Active); err != nil {
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

func (r Repository) EnsureFactoryCustomer(ctx context.Context, actor string) (int64, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customers WHERE active=true AND name='工厂自营' ORDER BY id LIMIT 1`, r.schema)).Scan(&id)
	if err == nil {
		return id, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return 0, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, raw_name, customer_type, active, created_at, updated_at)
		VALUES('工厂自营', '工厂自营', 'wholesale', true, now(), now())
		RETURNING id
	`, r.schema)).Scan(&id); err != nil {
		return 0, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "customer", &id, "ensure_factory_customer", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr("工厂自营"), postgresinfra.AuditMeta{"customer_id": id, "built_in": true}); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (r Repository) ListCustomerProductAliases(ctx context.Context, query catalogapp.CustomerProductAliasQuery) ([]catalogapp.CustomerProductAlias, error) {
	activeMode := strings.ToLower(strings.TrimSpace(query.ActiveMode))
	if activeMode == "" {
		if query.ActiveOnly {
			activeMode = "active"
		} else {
			activeMode = "all"
		}
	}
	if activeMode != "active" && activeMode != "inactive" && activeMode != "all" {
		activeMode = "active"
	}
	searchQuery := strings.ToLower(strings.TrimSpace(query.SearchQuery))
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT a.id,
		       a.customer_id,
		       COALESCE(c.name,''),
		       a.product_id,
		       'SKU-' || LPAD(a.product_id::text, 6, '0'),
		       COALESCE(p.name,''),
		       COALESCE(p.active,false),
		       a.display_name,
		       COALESCE(a.customer_item_code,''),
		       COALESCE(a.brand_name,''),
		       COALESCE(a.display_category_id,0),
		       COALESCE(cat.name,''),
		       COALESCE(a.classification_template_id,0),
		       COALESCE(a.product_config_template_id,0),
		       COALESCE(a.gradient_template_id,0),
		       COALESCE(a.unit_template_id,0),
		       COALESCE(a.sort_order,0),
		       COALESCE(a.include_in_price_list,true),
		       COALESCE(a.active,true),
		       COALESCE(a.remark,''),
		       COALESCE(a.created_by,''),
		       COALESCE(a.updated_by,'')
		FROM %s.customer_product_aliases a
		LEFT JOIN %s.customers c ON c.id=a.customer_id
		LEFT JOIN %s.products p ON p.id=a.product_id
		LEFT JOIN %s.product_categories cat ON cat.id=a.display_category_id
		WHERE ($1::bigint=0 OR a.customer_id=$1)
		  AND ($2::text='all' OR ($2::text='active' AND a.active=true) OR ($2::text='inactive' AND a.active=false))
		  AND ($3::text='' OR lower(a.display_name || ' ' || COALESCE(a.customer_item_code,'') || ' ' || COALESCE(p.name,'') || ' SKU-' || LPAD(a.product_id::text, 6, '0')) LIKE '%%' || $3::text || '%%')
		ORDER BY a.customer_id, a.sort_order, a.id
	`, r.schema, r.schema, r.schema, r.schema), query.CustomerID, activeMode, searchQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]catalogapp.CustomerProductAlias, 0)
	for rows.Next() {
		var row catalogapp.CustomerProductAlias
		if err := rows.Scan(
			&row.ID,
			&row.CustomerID,
			&row.CustomerName,
			&row.ProductID,
			&row.ProductCode,
			&row.ProductName,
			&row.ProductActive,
			&row.DisplayName,
			&row.CustomerItemCode,
			&row.BrandName,
			&row.DisplayCategoryID,
			&row.DisplayCategoryName,
			&row.ClassificationTemplateID,
			&row.ProductConfigTemplateID,
			&row.GradientTemplateID,
			&row.UnitTemplateID,
			&row.SortOrder,
			&row.IncludeInPriceList,
			&row.Active,
			&row.Remark,
			&row.CreatedBy,
			&row.UpdatedBy,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachCustomerProductAliasIndustryFields(ctx, out); err != nil {
		return nil, err
	}
	if err := r.attachCustomerProductAliasPriceSummaries(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

type publishedPriceSummaryKey struct {
	ProductID  int64
	CustomerID int64
}

func (r Repository) attachProductGroupSummaries(ctx context.Context, products []catalogapp.Product) error {
	if len(products) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(products))
	index := map[int64]int{}
	for i, product := range products {
		if product.ID <= 0 {
			continue
		}
		ids = append(ids, product.ID)
		index[product.ID] = i
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT product_ids.id,
		       COALESCE(bga.group_id,0),
		       COALESCE(bg.name,''),
		       COALESCE(bga.group_item_id,0),
		       COALESCE(item.name,''),
		       COALESCE(parent.id,0),
		       COALESCE(parent.name,'')
		FROM unnest($1::bigint[]) AS product_ids(id)
		LEFT JOIN %[1]s.business_group_assignments bga ON bga.object_id=product_ids.id
		  AND lower(bga.usage_key)='product_catalog'
		  AND lower(bga.object_key)='product'
		LEFT JOIN %[1]s.business_groups bg ON bg.id=bga.group_id
		LEFT JOIN %[1]s.business_group_items item ON item.id=bga.group_item_id
		LEFT JOIN %[1]s.business_group_items parent ON parent.id=item.parent_id
	`, r.schema), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var productID int64
		var groupID, groupItemID, parentGroupItemID int64
		var groupName, groupItemName, parentGroupItemName string
		if err := rows.Scan(&productID, &groupID, &groupName, &groupItemID, &groupItemName, &parentGroupItemID, &parentGroupItemName); err != nil {
			return err
		}
		if groupID <= 0 {
			continue
		}
		if i, ok := index[productID]; ok {
			products[i].GroupID = groupID
			products[i].GroupName = groupName
			products[i].GroupItemID = groupItemID
			products[i].GroupItemName = groupItemName
			products[i].ParentGroupItemID = parentGroupItemID
			products[i].ParentGroupItemName = parentGroupItemName
			products[i].GroupSource = "product_catalog"
		}
	}
	return rows.Err()
}

func (r Repository) attachProductPriceSummaries(ctx context.Context, products []catalogapp.Product) error {
	if len(products) == 0 {
		return nil
	}
	keys := make([]publishedPriceSummaryKey, 0, len(products))
	for _, product := range products {
		if product.ID <= 0 {
			continue
		}
		keys = append(keys, publishedPriceSummaryKey{ProductID: product.ID})
	}
	summaries, err := r.loadPublishedPriceSummaries(ctx, keys, false)
	if err != nil {
		return err
	}
	for i := range products {
		if summary, ok := summaries[publishedPriceSummaryKey{ProductID: products[i].ID}]; ok {
			products[i].PriceSummary = summary
		}
	}
	return nil
}

func (r Repository) attachCustomerProductAliasPriceSummaries(ctx context.Context, aliases []catalogapp.CustomerProductAlias) error {
	if len(aliases) == 0 {
		return nil
	}
	keys := make([]publishedPriceSummaryKey, 0, len(aliases))
	for _, alias := range aliases {
		if alias.ProductID <= 0 || alias.CustomerID <= 0 {
			continue
		}
		keys = append(keys, publishedPriceSummaryKey{ProductID: alias.ProductID, CustomerID: alias.CustomerID})
	}
	summaries, err := r.loadPublishedPriceSummaries(ctx, keys, true)
	if err != nil {
		return err
	}
	for i := range aliases {
		key := publishedPriceSummaryKey{ProductID: aliases[i].ProductID, CustomerID: aliases[i].CustomerID}
		if summary, ok := summaries[key]; ok {
			aliases[i].PriceSummary = summary
		}
	}
	return nil
}

func (r Repository) loadPublishedPriceSummaries(ctx context.Context, keys []publishedPriceSummaryKey, customerScoped bool) (map[publishedPriceSummaryKey]catalogapp.PriceSummary, error) {
	out := map[publishedPriceSummaryKey]catalogapp.PriceSummary{}
	if len(keys) == 0 {
		return out, nil
	}
	if !catalogRelationExists(ctx, r.pool, fmt.Sprintf("%s.bean_list_publications", r.schema)) {
		return out, nil
	}
	productIDs := make([]int64, 0, len(keys))
	customerIDs := make([]int64, 0, len(keys))
	seen := map[publishedPriceSummaryKey]struct{}{}
	for _, key := range keys {
		if key.ProductID <= 0 {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		productIDs = append(productIDs, key.ProductID)
		customerIDs = append(customerIDs, key.CustomerID)
	}
	if len(productIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH wanted AS (
			SELECT DISTINCT product_id, customer_id
			FROM unnest($1::bigint[], $2::bigint[]) AS w(product_id, customer_id)
			WHERE product_id > 0
		), candidates AS (
			SELECT w.product_id,
			       w.customer_id,
			       blp.id AS publication_id,
			       COALESCE(blp.version_no, '') AS price_table_version,
			       COALESCE(blp.published_at::text, blp.updated_at::text, '') AS updated_at,
			       COALESCE(tier.tier_json->>'label', '') AS tier_label,
			       COALESCE(NULLIF(tier.tier_json->>'price_unit', ''), NULLIF(tier.tier_json->>'display_unit', ''), '') AS price_unit,
			       COALESCE(NULLIF(tier.tier_json->>'source_price_record_id', '')::bigint, 0) AS source_price_record_id,
			       COALESCE(
			         NULLIF(tier.tier_json->>'final_unit_price', '')::float8,
			         NULLIF(tier.tier_json->>'price_per_unit', '')::float8,
			         NULLIF(tier.tier_json->>'price_per_kg', '')::float8,
			         NULLIF(tier.tier_json->>'price_per_lb', '')::float8,
			         0
			       ) AS final_price,
			       CASE WHEN $3::boolean AND blp.owner_type='customer' AND blp.owner_key=w.customer_id::text THEN 0 ELSE 1 END AS owner_priority,
			       COALESCE(NULLIF(tier.tier_json->>'min_qty', '')::float8, 0) AS min_qty
			FROM wanted w
			JOIN %[1]s.bean_list_publications blp ON blp.status='published' AND blp.list_type='commercial'
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(blp.content_json->'groups', '[]'::jsonb)) AS grp(group_json)
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(grp.group_json->'items', '[]'::jsonb)) AS item(item_json)
			CROSS JOIN LATERAL jsonb_array_elements(COALESCE(item.item_json->'commercial_wholesale_tiers', '[]'::jsonb)) AS tier(tier_json)
			WHERE COALESCE(
			        NULLIF(item.item_json->>'productId', '')::bigint,
			        NULLIF(item.item_json->>'product_id', '')::bigint,
			        NULLIF(item.item_json->>'productID', '')::bigint,
			        0
			      ) = w.product_id
			  AND (
			    (NOT $3::boolean AND blp.owner_type='official')
			    OR (
			      $3::boolean
			      AND (
			        (blp.owner_type='customer' AND blp.owner_key=w.customer_id::text)
			        OR blp.owner_type='official'
			      )
			    )
			  )
		)
		SELECT DISTINCT ON (product_id, customer_id)
		       product_id, customer_id, publication_id, price_table_version, updated_at, tier_label,
		       price_unit, source_price_record_id, final_price
		FROM candidates
		WHERE final_price > 0
		ORDER BY product_id, customer_id, owner_priority, updated_at DESC, publication_id DESC, min_qty ASC
	`, r.schema), productIDs, customerIDs, customerScoped)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key publishedPriceSummaryKey
		var summary catalogapp.PriceSummary
		if err := rows.Scan(
			&key.ProductID,
			&key.CustomerID,
			&summary.PublicationID,
			&summary.PriceTableVersion,
			&summary.UpdatedAt,
			&summary.TierLabel,
			&summary.PriceUnit,
			&summary.SourcePriceRecordID,
			&summary.FinalPrice,
		); err != nil {
			return nil, err
		}
		out[key] = summary
	}
	return out, rows.Err()
}

func (r Repository) attachCustomerProductAliasIndustryFields(ctx context.Context, aliases []catalogapp.CustomerProductAlias) error {
	if len(aliases) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(aliases))
	index := map[int64]int{}
	for i, row := range aliases {
		if row.ID <= 0 {
			continue
		}
		ids = append(ids, row.ID)
		index[row.ID] = i
	}
	if len(ids) == 0 {
		return nil
	}
	fieldsByAlias, err := r.loadCustomerProductAliasIndustryFields(ctx, ids)
	if err != nil {
		return err
	}
	for aliasID, fields := range fieldsByAlias {
		if i, ok := index[aliasID]; ok {
			aliases[i].IndustryFields = fields
		}
	}
	return nil
}

func (r Repository) loadCustomerProductAliasIndustryFields(ctx context.Context, aliasIDs []int64) (map[int64][]catalogapp.ProductProductionConfigField, error) {
	out := map[int64][]catalogapp.ProductProductionConfigField{}
	if len(aliasIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT a.id,
		       f.id,
		       f.product_id,
		       f.field_key,
		       f.label,
		       f.field_type,
		       f.unit,
		       COALESCE(NULLIF(v.value_text,''), f.value_text, ''),
		       f.value_number::float8,
		       f.value_bool,
		       COALESCE(f.template_field_key,''),
		       COALESCE(f.required,false),
		       COALESCE(f.options_json::text,'[]'),
		       COALESCE(f.show_in_price_list,true),
		       COALESCE(f.sort_order,0)
		FROM %s.customer_product_aliases a
		JOIN %s.product_production_config_fields f ON f.product_id=a.product_id
		LEFT JOIN %s.customer_product_alias_industry_field_values v
		  ON v.alias_id=a.id AND lower(v.field_key)=lower(f.field_key)
		WHERE a.id=ANY($1)
		ORDER BY a.id, f.sort_order, f.id
	`, r.schema, r.schema, r.schema), aliasIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var aliasID int64
		var field catalogapp.ProductProductionConfigField
		if err := rows.Scan(&aliasID, &field.ID, &field.ProductID, &field.FieldKey, &field.Label, &field.FieldType, &field.Unit, &field.ValueText, &field.ValueNumber, &field.ValueBool, &field.TemplateFieldKey, &field.Required, &field.OptionsJSON, &field.ShowInPriceList, &field.SortOrder); err != nil {
			return nil, err
		}
		out[aliasID] = append(out[aliasID], field)
	}
	return out, rows.Err()
}

func (r Repository) SaveCustomerProductAlias(ctx context.Context, cmd catalogapp.CustomerProductAliasCommand) (catalogapp.CustomerProductAlias, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.CustomerProductAlias{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.CustomerProductAlias{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerExists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, r.schema), cmd.CustomerID).Scan(&customerExists); err != nil {
		return catalogapp.CustomerProductAlias{}, err
	}
	if !customerExists {
		return catalogapp.CustomerProductAlias{}, fmt.Errorf("customer not found")
	}
	var productExists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.products WHERE id=$1 AND active=true)`, r.schema), cmd.ProductID).Scan(&productExists); err != nil {
		return catalogapp.CustomerProductAlias{}, err
	}
	if !productExists {
		return catalogapp.CustomerProductAlias{}, fmt.Errorf("product not found")
	}

	action := "create_customer_product_alias"
	id := cmd.ID
	if cmd.ID > 0 {
		action = "update_customer_product_alias"
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.customer_product_aliases
			SET customer_id=$2,
			    product_id=$3,
			    display_name=$4,
			    brand_name=$5,
			    display_category_id=$6,
			    classification_template_id=$7,
			    product_config_template_id=$8,
			    gradient_template_id=$9,
			    unit_template_id=$10,
			    sort_order=$11,
			    include_in_price_list=$12,
			    active=$13,
			    remark=$14,
			    updated_at=now(),
			    updated_by=$15
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.CustomerID, cmd.ProductID, cmd.DisplayName, cmd.BrandName, cmd.DisplayCategoryID, cmd.ClassificationTemplateID, cmd.ProductConfigTemplateID, cmd.GradientTemplateID, cmd.UnitTemplateID, cmd.SortOrder, cmd.IncludeInPriceList, cmd.Active, cmd.Remark, cmd.Actor).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_product_aliases(
				customer_id, product_id, display_name, customer_item_code, brand_name,
				display_category_id, classification_template_id, product_config_template_id, gradient_template_id, unit_template_id, sort_order, include_in_price_list, active, remark,
				created_at, updated_at, created_by, updated_by
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,now(),now(),$15,$15)
			RETURNING id
		`, r.schema), cmd.CustomerID, cmd.ProductID, cmd.DisplayName, "", cmd.BrandName, cmd.DisplayCategoryID, cmd.ClassificationTemplateID, cmd.ProductConfigTemplateID, cmd.GradientTemplateID, cmd.UnitTemplateID, cmd.SortOrder, cmd.IncludeInPriceList, cmd.Active, cmd.Remark, cmd.Actor).Scan(&id)
		if err == nil {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_product_aliases SET customer_item_code=$2 WHERE id=$1`, r.schema), id, generatedCustomerProductAliasCode(id)); err != nil {
				return catalogapp.CustomerProductAlias{}, err
			}
		}
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return catalogapp.CustomerProductAlias{}, fmt.Errorf("customer product alias not found")
		}
		return catalogapp.CustomerProductAlias{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_alias", &id, action, postgresinfra.StrPtr("display_name"), nil, postgresinfra.StrPtr(cmd.DisplayName), postgresinfra.AuditMeta{
		"alias_id":                   id,
		"customer_id":                cmd.CustomerID,
		"product_id":                 cmd.ProductID,
		"display_name":               cmd.DisplayName,
		"customer_item_code":         generatedCustomerProductAliasCode(id),
		"brand_name":                 cmd.BrandName,
		"display_category_id":        cmd.DisplayCategoryID,
		"classification_template_id": cmd.ClassificationTemplateID,
		"product_config_template_id": cmd.ProductConfigTemplateID,
		"gradient_template_id":       cmd.GradientTemplateID,
		"unit_template_id":           cmd.UnitTemplateID,
		"sort_order":                 cmd.SortOrder,
		"include_in_price_list":      cmd.IncludeInPriceList,
		"active":                     cmd.Active,
	}); err != nil {
		return catalogapp.CustomerProductAlias{}, err
	}
	row, err := fetchCustomerProductAliasTx(ctx, tx, r.schema, id)
	if err != nil {
		return catalogapp.CustomerProductAlias{}, err
	}
	return row, tx.Commit(ctx)
}

func generatedCustomerProductAliasCode(id int64) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("CPA-%06d", id)
}

func (r Repository) BatchCreateCustomerProductAliases(ctx context.Context, cmd catalogapp.BatchCustomerProductAliasesCommand) (catalogapp.BatchCustomerProductAliasesResult, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.BatchCustomerProductAliasesResult{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.BatchCustomerProductAliasesResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerExists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, r.schema), cmd.CustomerID).Scan(&customerExists); err != nil {
		return catalogapp.BatchCustomerProductAliasesResult{}, err
	}
	if !customerExists {
		return catalogapp.BatchCustomerProductAliasesResult{}, fmt.Errorf("customer not found")
	}

	existing := map[int64]bool{}
	existingRows, err := tx.Query(ctx, fmt.Sprintf(`SELECT product_id FROM %s.customer_product_aliases WHERE customer_id=$1 AND active=true AND product_id=ANY($2)`, r.schema), cmd.CustomerID, cmd.ProductIDs)
	if err != nil {
		return catalogapp.BatchCustomerProductAliasesResult{}, err
	}
	for existingRows.Next() {
		var productID int64
		if err := existingRows.Scan(&productID); err != nil {
			existingRows.Close()
			return catalogapp.BatchCustomerProductAliasesResult{}, err
		}
		existing[productID] = true
	}
	if err := existingRows.Err(); err != nil {
		existingRows.Close()
		return catalogapp.BatchCustomerProductAliasesResult{}, err
	}
	existingRows.Close()

	productRows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, name, 'SKU-' || LPAD(id::text, 6, '0')
		FROM %s.products
		WHERE active=true AND id=ANY($1)
	`, r.schema), cmd.ProductIDs)
	if err != nil {
		return catalogapp.BatchCustomerProductAliasesResult{}, err
	}
	products := map[int64]struct {
		Name string
		Code string
	}{}
	for productRows.Next() {
		var id int64
		var name, code string
		if err := productRows.Scan(&id, &name, &code); err != nil {
			productRows.Close()
			return catalogapp.BatchCustomerProductAliasesResult{}, err
		}
		products[id] = struct {
			Name string
			Code string
		}{Name: name, Code: code}
	}
	if err := productRows.Err(); err != nil {
		productRows.Close()
		return catalogapp.BatchCustomerProductAliasesResult{}, err
	}
	productRows.Close()

	result := catalogapp.BatchCustomerProductAliasesResult{
		Created: make([]catalogapp.CustomerProductAlias, 0),
		Skipped: make([]catalogapp.CustomerProductAliasBatchSkipped, 0),
	}
	for _, productID := range cmd.ProductIDs {
		product, ok := products[productID]
		reason := ""
		if !ok {
			reason = "product_not_found"
		} else if existing[productID] {
			reason = "exists"
		}
		if reason != "" {
			result.Skipped = append(result.Skipped, catalogapp.CustomerProductAliasBatchSkipped{ProductID: productID, Reason: reason})
			if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_alias", nil, "skip_customer_product_alias_batch", postgresinfra.StrPtr("product_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", productID)), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "product_id": productID, "reason": reason}); err != nil {
				return catalogapp.BatchCustomerProductAliasesResult{}, err
			}
			continue
		}

		var id int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_product_aliases(
				customer_id, product_id, display_name, customer_item_code, brand_name,
				display_category_id, classification_template_id, gradient_template_id, unit_template_id, sort_order, include_in_price_list, active, remark,
				created_at, updated_at, created_by, updated_by
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,0,0,0,$8,true,'',now(),now(),$9,$9)
			RETURNING id
		`, r.schema), cmd.CustomerID, productID, product.Name, "", cmd.BrandName, cmd.DisplayCategoryID, int64(0), cmd.IncludeInPriceList, cmd.Actor).Scan(&id); err != nil {
			return catalogapp.BatchCustomerProductAliasesResult{}, err
		}
		code := generatedCustomerProductAliasCode(id)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_product_aliases SET customer_item_code=$2 WHERE id=$1`, r.schema), id, code); err != nil {
			return catalogapp.BatchCustomerProductAliasesResult{}, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_alias", &id, "create_customer_product_alias_batch", postgresinfra.StrPtr("display_name"), nil, postgresinfra.StrPtr(product.Name), postgresinfra.AuditMeta{"alias_id": id, "customer_id": cmd.CustomerID, "product_id": productID, "customer_item_code": code, "brand_name": cmd.BrandName, "display_category_id": cmd.DisplayCategoryID, "classification_template_id": 0, "include_in_price_list": cmd.IncludeInPriceList}); err != nil {
			return catalogapp.BatchCustomerProductAliasesResult{}, err
		}
		row, err := fetchCustomerProductAliasTx(ctx, tx, r.schema, id)
		if err != nil {
			return catalogapp.BatchCustomerProductAliasesResult{}, err
		}
		result.Created = append(result.Created, row)
		existing[productID] = true
	}
	result.CreatedCount = len(result.Created)
	result.SkippedCount = len(result.Skipped)
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.BatchCustomerProductAliasesResult{}, err
	}
	return result, nil
}

func (r Repository) DisableCustomerProductAlias(ctx context.Context, cmd catalogapp.DisableCustomerProductAliasCommand) error {
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

	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_product_aliases
		SET active=false, updated_at=now(), updated_by=$2
		WHERE id=$1 AND active=true
	`, r.schema), cmd.ID, cmd.Actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("customer product alias not found")
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_alias", &cmd.ID, "disable_customer_product_alias", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"alias_id": cmd.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) BatchDisableCustomerProductAliases(ctx context.Context, cmd catalogapp.BatchDisableCustomerProductAliasesCommand) (catalogapp.BatchDisableCustomerProductAliasesResult, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return catalogapp.BatchDisableCustomerProductAliasesResult{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return catalogapp.BatchDisableCustomerProductAliasesResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result := catalogapp.BatchDisableCustomerProductAliasesResult{
		Disabled: []int64{},
		Skipped:  []int64{},
	}
	for _, id := range cmd.IDs {
		if id <= 0 {
			continue
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_product_aliases
			SET active=false, updated_at=now(), updated_by=$2
			WHERE id=$1 AND active=true
		`, r.schema), id, cmd.Actor)
		if err != nil {
			return catalogapp.BatchDisableCustomerProductAliasesResult{}, err
		}
		if tag.RowsAffected() == 0 {
			result.Skipped = append(result.Skipped, id)
			continue
		}
		result.Disabled = append(result.Disabled, id)
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_alias", &id, "disable_customer_product_alias", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"alias_id": id, "batch": true}); err != nil {
			return catalogapp.BatchDisableCustomerProductAliasesResult{}, err
		}
	}
	result.DisabledCount = len(result.Disabled)
	result.SkippedCount = len(result.Skipped)
	if err := tx.Commit(ctx); err != nil {
		return catalogapp.BatchDisableCustomerProductAliasesResult{}, err
	}
	return result, nil
}

func (r Repository) ListCustomerProductAliasIndustryFields(ctx context.Context, query catalogapp.CustomerProductAliasIndustryFieldQuery) ([]catalogapp.ProductProductionConfigField, error) {
	fieldsByAlias, err := r.loadCustomerProductAliasIndustryFields(ctx, []int64{query.AliasID})
	if err != nil {
		return nil, err
	}
	return fieldsByAlias[query.AliasID], nil
}

func (r Repository) SaveCustomerProductAliasIndustryFields(ctx context.Context, cmd catalogapp.SaveCustomerProductAliasIndustryFieldsCommand) ([]catalogapp.ProductProductionConfigField, error) {
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

	var productID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id FROM %s.customer_product_aliases WHERE id=$1 AND active=true`, r.schema), cmd.AliasID).Scan(&productID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, catalogapp.ValidationError{Message: "customer product alias not found"}
		}
		return nil, err
	}
	allowedRows, err := tx.Query(ctx, fmt.Sprintf(`SELECT field_key FROM %s.product_production_config_fields WHERE product_id=$1`, r.schema), productID)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for allowedRows.Next() {
		var key string
		if err := allowedRows.Scan(&key); err != nil {
			allowedRows.Close()
			return nil, err
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "" {
			allowed[key] = true
		}
	}
	if err := allowedRows.Err(); err != nil {
		allowedRows.Close()
		return nil, err
	}
	allowedRows.Close()
	for _, field := range cmd.Fields {
		key := strings.ToLower(strings.TrimSpace(field.FieldKey))
		if key == "" {
			continue
		}
		if !allowed[key] {
			return nil, catalogapp.ValidationError{Message: fmt.Sprintf("field %s is not defined by product industry field template", field.FieldKey)}
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.customer_product_alias_industry_field_values WHERE alias_id=$1`, r.schema), cmd.AliasID); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, field := range cmd.Fields {
		key := strings.TrimSpace(field.FieldKey)
		lowerKey := strings.ToLower(key)
		if key == "" || seen[lowerKey] {
			continue
		}
		seen[lowerKey] = true
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_product_alias_industry_field_values(alias_id, field_key, value_text, created_at, updated_at, updated_by)
			VALUES($1,$2,$3,now(),now(),$4)
		`, r.schema), cmd.AliasID, key, strings.TrimSpace(field.ValueText), cmd.Actor); err != nil {
			return nil, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_alias_industry_fields", &cmd.AliasID, "save_customer_product_alias_industry_fields", postgresinfra.StrPtr("field_count"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", len(seen))), postgresinfra.AuditMeta{"alias_id": cmd.AliasID, "field_count": len(seen)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.ListCustomerProductAliasIndustryFields(ctx, catalogapp.CustomerProductAliasIndustryFieldQuery{AliasID: cmd.AliasID})
}

func (r Repository) ListCustomerProductAliasMigrationCandidates(ctx context.Context, query catalogapp.CustomerProductAliasMigrationCandidateQuery) ([]catalogapp.CustomerProductAliasMigrationCandidate, error) {
	if query.CustomerID <= 0 {
		return nil, nil
	}
	hasSourceTable := catalogRelationExists(ctx, r.pool, fmt.Sprintf("%s.product_bom_sources", r.schema))
	hasBomTable := catalogRelationExists(ctx, r.pool, fmt.Sprintf("%s.product_bom", r.schema))
	hasFinishedInventory := catalogRelationExists(ctx, r.pool, fmt.Sprintf("%s.finished_inventory", r.schema))
	hasWorkOrders := catalogRelationExists(ctx, r.pool, fmt.Sprintf("%s.work_orders", r.schema))
	hasRunningItems := catalogRelationExists(ctx, r.pool, fmt.Sprintf("%s.produce_running_items", r.schema))
	hasProductionLogs := catalogRelationExists(ctx, r.pool, fmt.Sprintf("%s.production_logs", r.schema))

	sourceJoin := ""
	sourceProductIDExpr := "COALESCE(NULLIF(p.base_product_id,0),0)"
	sourceTypeExpr := "''"
	if hasSourceTable {
		sourceJoin = fmt.Sprintf("LEFT JOIN %s.product_bom_sources s ON s.product_id=p.id", r.schema)
		sourceProductIDExpr = "COALESCE(NULLIF(s.source_product_id,0), NULLIF(p.base_product_id,0), 0)"
		sourceTypeExpr = "COALESCE(NULLIF(s.source_type,''), '')"
	}
	ownBomExpr := "false"
	if hasBomTable {
		ownBomExpr = fmt.Sprintf("EXISTS (SELECT 1 FROM %s.product_bom b WHERE b.product_id=p.id AND COALESCE(NULLIF(b.status,''),'active')='active')", r.schema)
	}
	hasOwnBomExpr := ownBomExpr
	if hasSourceTable {
		hasOwnBomExpr = fmt.Sprintf("(%[1]s IN ('owned','derived_owned') OR (%[1]s='' AND %[2]s))", sourceTypeExpr, ownBomExpr)
	}
	inventoryExpr := "false"
	if hasFinishedInventory {
		inventoryExpr = fmt.Sprintf("EXISTS (SELECT 1 FROM %s.finished_inventory fi WHERE fi.product_id=p.id AND (COALESCE(fi.onhand_units,0)>0 OR COALESCE(fi.onhand_loose_g,0)>0))", r.schema)
	}
	productionChecks := make([]string, 0, 3)
	if hasWorkOrders {
		productionChecks = append(productionChecks, fmt.Sprintf("EXISTS (SELECT 1 FROM %s.work_orders wo WHERE wo.product_id=p.id)", r.schema))
	}
	if hasRunningItems {
		productionChecks = append(productionChecks, fmt.Sprintf("EXISTS (SELECT 1 FROM %s.produce_running_items ri WHERE ri.product_id=p.id)", r.schema))
	}
	if hasProductionLogs {
		productionChecks = append(productionChecks, fmt.Sprintf("EXISTS (SELECT 1 FROM %s.production_logs pl WHERE pl.product_id=p.id)", r.schema))
	}
	productionExpr := "false"
	if len(productionChecks) > 0 {
		productionExpr = "(" + strings.Join(productionChecks, " OR ") + ")"
	}

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT p.id,
		       'SKU-' || LPAD(p.id::text, 6, '0'),
		       COALESCE(p.name,''),
		       COALESCE(base.id,0),
		       CASE WHEN base.id IS NULL THEN '' ELSE 'SKU-' || LPAD(base.id::text, 6, '0') END,
		       COALESCE(base.name,''),
		       %[1]s,
		       %[2]s,
		       %[3]s,
		       %[4]s
		FROM %[5]s.products p
		%[6]s
		LEFT JOIN %[5]s.products base ON base.id=%[7]s
		WHERE COALESCE(p.customer_id,0)=$1
		  AND COALESCE(p.active,true)=true
		  AND %[7]s > 0
		ORDER BY p.id
	`, sourceTypeExpr, hasOwnBomExpr, productionExpr, inventoryExpr, r.schema, sourceJoin, sourceProductIDExpr), query.CustomerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]catalogapp.CustomerProductAliasMigrationCandidate, 0)
	for rows.Next() {
		var row catalogapp.CustomerProductAliasMigrationCandidate
		row.CustomerID = query.CustomerID
		if err := rows.Scan(&row.ProductID, &row.ProductCode, &row.ProductName, &row.BaseProductID, &row.BaseProductCode, &row.BaseProductName, &row.BomSourceType, &row.HasOwnBom, &row.HasProductionRecord, &row.HasInventoryRecord); err != nil {
			return nil, err
		}
		row.SuggestedAction = "keep_product_record"
		row.SuggestedReason = "存在独立生产、库存或生产 BOM 证据，保留为商品档案更稳妥"
		if canRecommendCustomerAliasMigration(row) {
			row.SuggestedAction = "convert_to_customer_product_alias"
			row.SuggestedReason = "仅名称、编号、展示分类或价格差异，生产定义跟随来源商品档案"
			row.CanAutoRecommend = true
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func canRecommendCustomerAliasMigration(row catalogapp.CustomerProductAliasMigrationCandidate) bool {
	if row.BaseProductID <= 0 || row.HasOwnBom || row.HasProductionRecord || row.HasInventoryRecord {
		return false
	}
	switch strings.TrimSpace(row.BomSourceType) {
	case "", "missing", "inherit_current", "inherit_version":
		return true
	default:
		return false
	}
}

type catalogQueryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func catalogRelationExists(ctx context.Context, q catalogQueryRower, relation string) bool {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, relation).Scan(&exists); err != nil {
		return false
	}
	return exists
}

func ensureCustomerClassificationTemplateTx(ctx context.Context, tx pgx.Tx, schema string, actor string, targetCustomerID int64, sourceTemplateID int64) (int64, error) {
	if sourceTemplateID <= 0 {
		return 0, nil
	}
	var sourceCustomerID int64
	var sourceName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(customer_id,0), name
		FROM %s.product_classification_templates
		WHERE id=$1 AND active=true
	`, schema), sourceTemplateID).Scan(&sourceCustomerID, &sourceName); err != nil {
		return 0, err
	}
	if sourceCustomerID == targetCustomerID {
		return sourceTemplateID, nil
	}
	var existingID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id
		FROM %s.product_classification_templates
		WHERE active=true AND customer_id=$1 AND (source_template_id=$2 OR lower(name)=lower($3))
		ORDER BY CASE WHEN source_template_id=$2 THEN 0 ELSE 1 END, id
		LIMIT 1
	`, schema), targetCustomerID, sourceTemplateID, sourceName).Scan(&existingID)
	if err == nil && existingID > 0 {
		return existingID, nil
	}
	if err != nil && err != pgx.ErrNoRows {
		return 0, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_classification_templates(customer_id, source_template_id, template_state, name, product_config_template_id, gradient_template_id, unit_template_id, active, sort_order, created_by, updated_by)
		SELECT $1, $2, 'derived_from_public', name, COALESCE(product_config_template_id,0), COALESCE(gradient_template_id,0), COALESCE(unit_template_id,0), true, sort_order, $3, $3
		FROM %s.product_classification_templates
		WHERE id=$2
		RETURNING id
	`, schema, schema), targetCustomerID, sourceTemplateID, actor).Scan(&existingID); err != nil {
		return 0, err
	}
	type categoryCopy struct {
		sourceID                int64
		parentID                int64
		newID                   int64
		name                    string
		level                   int
		sort                    int
		productConfigTemplateID int64
		gradientTemplateID      int64
		unitTemplateID          int64
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(parent_id,0), name, COALESCE(level,1), COALESCE(sort_order,100), COALESCE(product_config_template_id,0), COALESCE(gradient_template_id,0), COALESCE(unit_template_id,0)
		FROM %s.product_classification_template_categories
		WHERE template_id=$1 AND active=true
		ORDER BY COALESCE(parent_id,0), COALESCE(sort_order,100), id
	`, schema), sourceTemplateID)
	if err != nil {
		return 0, err
	}
	copies := make([]categoryCopy, 0)
	for rows.Next() {
		var row categoryCopy
		if err := rows.Scan(&row.sourceID, &row.parentID, &row.name, &row.level, &row.sort, &row.productConfigTemplateID, &row.gradientTemplateID, &row.unitTemplateID); err != nil {
			rows.Close()
			return 0, err
		}
		copies = append(copies, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	idMap := map[int64]int64{}
	for i := range copies {
		parentID := idMap[copies[i].parentID]
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_classification_template_categories(template_id, parent_id, name, level, sort_order, product_config_template_id, gradient_template_id, unit_template_id, active)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,true)
			RETURNING id
		`, schema), existingID, parentID, copies[i].name, copies[i].level, copies[i].sort, copies[i].productConfigTemplateID, copies[i].gradientTemplateID, copies[i].unitTemplateID).Scan(&copies[i].newID); err != nil {
			return 0, err
		}
		idMap[copies[i].sourceID] = copies[i].newID
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "product_classification_template", &existingID, "copy_customer_classification_template", postgresinfra.StrPtr("source_template_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", sourceTemplateID)), postgresinfra.AuditMeta{"template_id": existingID, "customer_id": targetCustomerID, "source_template_id": sourceTemplateID}); err != nil {
		return 0, err
	}
	return existingID, nil
}

func copyProductClassificationAssignmentToAliasTx(ctx context.Context, tx pgx.Tx, schema string, actor string, productID int64, aliasID int64, sourceTemplateID int64, targetTemplateID int64) error {
	if productID <= 0 || aliasID <= 0 || targetTemplateID <= 0 {
		return nil
	}
	categoryID := int64(0)
	if sourceTemplateID > 0 {
		var sourceCategoryID int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(category_id,0)
			FROM %s.product_classification_assignments
			WHERE product_id=$1 AND template_id=$2
		`, schema), productID, sourceTemplateID).Scan(&sourceCategoryID)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		if sourceCategoryID > 0 {
			err = tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT target.id
				FROM %s.product_classification_template_categories source
				JOIN %s.product_classification_template_categories target
				  ON target.template_id=$3 AND target.active=true AND lower(target.name)=lower(source.name)
				WHERE source.id=$1 AND source.template_id=$2 AND source.active=true
				ORDER BY target.sort_order, target.id
				LIMIT 1
			`, schema, schema), sourceCategoryID, sourceTemplateID, targetTemplateID).Scan(&categoryID)
			if err != nil && err != pgx.ErrNoRows {
				return err
			}
		}
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_product_alias_classification_assignments(alias_id, template_id, category_id, sort_order, updated_by, created_at, updated_at)
		VALUES($1,$2,$3,100,$4,now(),now())
		ON CONFLICT(alias_id, template_id) DO UPDATE SET
			category_id=excluded.category_id,
			updated_by=excluded.updated_by,
			updated_at=now()
	`, schema), aliasID, targetTemplateID, categoryID, actor)
	return err
}

func fetchCustomerProductAliasTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (catalogapp.CustomerProductAlias, error) {
	var row catalogapp.CustomerProductAlias
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT a.id,
		       a.customer_id,
		       COALESCE(c.name,''),
		       a.product_id,
		       'SKU-' || LPAD(a.product_id::text, 6, '0'),
		       COALESCE(p.name,''),
		       COALESCE(p.active,false),
		       a.display_name,
		       COALESCE(a.customer_item_code,''),
		       COALESCE(a.brand_name,''),
		       COALESCE(a.display_category_id,0),
		       COALESCE(cat.name,''),
		       COALESCE(a.classification_template_id,0),
		       COALESCE(a.product_config_template_id,0),
		       COALESCE(a.gradient_template_id,0),
		       COALESCE(a.unit_template_id,0),
		       COALESCE(a.sort_order,0),
		       COALESCE(a.include_in_price_list,true),
		       COALESCE(a.active,true),
		       COALESCE(a.remark,''),
		       COALESCE(a.created_by,''),
		       COALESCE(a.updated_by,'')
		FROM %s.customer_product_aliases a
		LEFT JOIN %s.customers c ON c.id=a.customer_id
		LEFT JOIN %s.products p ON p.id=a.product_id
		LEFT JOIN %s.product_categories cat ON cat.id=a.display_category_id
		WHERE a.id=$1
	`, schema, schema, schema, schema), id).Scan(
		&row.ID,
		&row.CustomerID,
		&row.CustomerName,
		&row.ProductID,
		&row.ProductCode,
		&row.ProductName,
		&row.ProductActive,
		&row.DisplayName,
		&row.CustomerItemCode,
		&row.BrandName,
		&row.DisplayCategoryID,
		&row.DisplayCategoryName,
		&row.ClassificationTemplateID,
		&row.ProductConfigTemplateID,
		&row.GradientTemplateID,
		&row.UnitTemplateID,
		&row.SortOrder,
		&row.IncludeInPriceList,
		&row.Active,
		&row.Remark,
		&row.CreatedBy,
		&row.UpdatedBy,
	)
	return row, err
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
		COALESCE(classification_template_id,0),
		COALESCE(customer_id,0), COALESCE(base_product_id,0),
		COALESCE(NULLIF(visibility,''),'public'), COALESCE(custom_type,''),
		margin_rate_override::float8,
		COALESCE(gradient_template_id_override,0),
		COALESCE(operation_template_id_override,0),
		COALESCE(unit_rule_override_json::text,'{}'),
		COALESCE(NULLIF(unit_rule_override_json->>'inventory_unit',''), 'kg'),
		COALESCE(
			CASE WHEN lower(unit_rule_override_json->>'integer_inventory_unit') IN ('true','1','yes') THEN true WHEN lower(unit_rule_override_json->>'integer_inventory_unit') IN ('false','0','no') THEN false ELSE NULL END,
			CASE WHEN lower(unit_rule_override_json->>'integer_unit') IN ('true','1','yes') THEN true WHEN lower(unit_rule_override_json->>'integer_unit') IN ('false','0','no') THEN false ELSE NULL END,
			false
		),
		COALESCE(product_config_template_id,0),
		COALESCE((SELECT COUNT(*) FROM %[1]s.product_bom_items bi WHERE bi.product_id=products.id),0),
		COALESCE((SELECT NULLIF(status,'') FROM %[1]s.product_bom WHERE product_id=products.id), 'missing')
		FROM %[1]s.products WHERE id=$1`, schema), id).Scan(&p.ID, &p.Name, &p.Remark, &p.RoastLevel, &p.SpecialAttrsJSON, &p.DefaultPrice, &p.ProductKind, &p.GreenBeanType, &p.GreenBeanBomProductID, &p.DripBagGrams, &p.DripBoxBagCount, &p.AllowFulfillmentOrder, &p.AllowMallOrder, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.YieldRate, &p.ProductCategoryID, &p.ProductCategoryPosition, &p.ClassificationTemplateID, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.MarginRateOverride, &p.GradientTemplateIDOverride, &p.OperationTemplateIDOverride, &p.UnitRuleOverrideJSON, &p.InventoryUnit, &p.IntegerInventoryUnit, &p.ProductConfigTemplateID, &p.BomItemCount, &p.BomStatus)
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
	out := catalogapp.Product{ID: p.ID, SKUID: p.SKUID, ParentProductID: p.ParentProductID, EffectiveParentProductID: p.EffectiveParentProductID, SKUName: p.SKUName, SKUCode: p.SKUCode, Barcode: p.Barcode, SpecLabel: p.SpecLabel, NetContentQty: p.NetContentQty, NetContentUnit: p.NetContentUnit, IsDefaultSKU: p.IsDefaultSKU, AutoDerivedSKU: p.AutoDerivedSKU, DerivedUnitTemplateID: p.DerivedUnitTemplateID, DerivedSpecKey: p.DerivedSpecKey, DerivedSpecName: p.DerivedSpecName, DerivedSalesUnit: p.DerivedSalesUnit, DerivedSpecStatus: p.DerivedSpecStatus, Name: p.Name, Remark: p.Remark, RoastLevel: p.RoastLevel, SpecialAttrsJSON: p.SpecialAttrsJSON, ProductKind: p.ProductKind, GreenBeanType: p.GreenBeanType, GreenBeanBomProductID: p.GreenBeanBomProductID, DripBagGrams: p.DripBagGrams, DripBoxBagCount: p.DripBoxBagCount, AllowFulfillmentOrder: p.AllowFulfillmentOrder, AllowMallOrder: p.AllowMallOrder, SalesUnits: p.SalesUnits, DefaultPrice: p.DefaultPrice, RetailPrice100G: p.RetailPrice100G, RetailPrice200G: p.RetailPrice200G, RetailPrice227G: p.RetailPrice227G, RetailPrice250G: p.RetailPrice250G, YieldRate: p.YieldRate, ExpectedLossRate: p.ExpectedLossRate, ProcessRouteID: p.ProcessRouteID, ProductionConfigNote: p.ProductionConfigNote, ProductCategoryID: p.ProductCategoryID, ProductCategoryPosition: p.ProductCategoryPosition, ClassificationTemplateID: p.ClassificationTemplateID, CustomerID: p.CustomerID, BaseProductID: p.BaseProductID, Visibility: p.Visibility, CustomType: p.CustomType, MarginRateOverride: p.MarginRateOverride, GradientTemplateIDOverride: p.GradientTemplateIDOverride, OperationTemplateIDOverride: p.OperationTemplateIDOverride, UnitRuleOverrideJSON: p.UnitRuleOverrideJSON, InventoryUnit: p.InventoryUnit, IntegerInventoryUnit: p.IntegerInventoryUnit, DefaultSalesUnit: p.DefaultSalesUnit, UnitConversionJSON: p.UnitConversionJSON, SalesUnitRulesJSON: p.SalesUnitRulesJSON, UnitTemplateID: p.UnitTemplateID, UnitTemplateName: p.UnitTemplateName, UnitRuleSource: p.UnitRuleSource, ProductConfigTemplateID: p.ProductConfigTemplateID, Active: p.Active, BomItemCount: p.BomItemCount, BomStatus: p.BomStatus, BomSourceType: p.BomSourceType, EffectiveProductID: p.EffectiveProductID, EffectiveBomVersionID: p.EffectiveBomVersionID, SourceProductID: p.SourceProductID, SourceProductCode: p.SourceProductCode, SourceProductName: p.SourceProductName, SourceBomVersionID: p.SourceBomVersionID, SourceBomVersionNo: p.SourceBomVersionNo, DerivedFromLabel: p.DerivedFromLabel, CanEditBOM: p.CanEditBOM, ProductionBomID: p.ProductionBomID, ProductionBomCode: p.ProductionBomCode, ProductionBomName: p.ProductionBomName, ProductionBomVersionID: p.ProductionBomVersionID, ProductionBomVersionNo: p.ProductionBomVersionNo, LatestBomVersionID: p.LatestBomVersionID, LatestBomVersionNo: p.LatestBomVersionNo, IsLatestBomVersion: p.IsLatestBomVersion, ProductionBomGroupID: p.ProductionBomGroupID, ProductionBomGroupName: p.ProductionBomGroupName, OrderUsageCount: p.OrderUsageCount}
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
