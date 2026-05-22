package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (r Repository) OrderForm(ctx context.Context, editID int64) (salesapp.OrderFormData, error) {
	data := salesapp.OrderFormData{Today: time.Now().Format("2006-01-02")}
	var err error
	if data.Customers, err = r.fetchOrderCustomers(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.Sources, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.ShipStatuses, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.PayStatuses, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.OrderTypes, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.Products, err = r.fetchOrderProducts(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.Employees, err = r.fetchOrderEmployees(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.LogisticsCompanies, err = r.ListLogisticsCompanies(ctx, false); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.BeanListVersionOptions, err = r.fetchOrderBeanListVersionOptions(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.CustomerPublicUsages, err = r.fetchOrderCustomerPublicUsages(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if editID > 0 {
		editData, err := r.fetchOrderEdit(ctx, editID)
		if err != nil {
			return salesapp.OrderFormData{}, err
		}
		data.EditData = editData
	}
	return data, nil
}

func (r Repository) fetchOrderBeanListVersionOptions(ctx context.Context) ([]salesapp.BeanListVersionOption, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.bean_list_publications", r.schema)).Scan(&exists); err != nil || !exists {
		return nil, err
	}
	q := fmt.Sprintf(`
		WITH active_customers AS (
			SELECT id FROM %[1]s.customers WHERE active=true
		),
		customer_versions AS (
			SELECT c.id AS customer_id,
			       b.list_type,
			       b.id,
			       b.version_no,
			       COALESCE(to_char(b.published_at, 'YYYY-MM-DD HH24:MI'), '') AS published_at,
			       COALESCE(b.changelog, '') AS changelog,
			       true AS is_customer_owned,
			       row_number() OVER (PARTITION BY c.id, b.list_type ORDER BY b.published_at DESC, b.id DESC) = 1 AS is_default
			FROM active_customers c
			JOIN %[1]s.bean_list_publications b
			  ON b.owner_type='customer' AND b.owner_key=c.id::text AND b.status='published'
			WHERE b.list_type IN ('commercial','green','drip')
		),
		official_versions AS (
			SELECT b.list_type,
			       b.id,
			       b.version_no,
			       COALESCE(to_char(b.published_at, 'YYYY-MM-DD HH24:MI'), '') AS published_at,
			       COALESCE(b.changelog, '') AS changelog,
			       row_number() OVER (PARTITION BY b.list_type ORDER BY b.published_at DESC, b.id DESC) = 1 AS is_default
			FROM %[1]s.bean_list_publications b
			WHERE b.owner_type='official' AND b.status='published' AND b.list_type IN ('commercial','green','drip')
		),
		public_fallback AS (
			SELECT c.id AS customer_id,
			       o.list_type,
			       o.id,
			       o.version_no,
			       o.published_at,
			       o.changelog,
			       false AS is_customer_owned,
			       o.is_default
			FROM active_customers c
			CROSS JOIN official_versions o
			WHERE NOT EXISTS (
				SELECT 1 FROM customer_versions cv WHERE cv.customer_id=c.id AND cv.list_type=o.list_type
			)
		)
		SELECT customer_id, list_type, id, version_no, published_at, changelog, is_customer_owned, is_default
		FROM customer_versions
		UNION ALL
		SELECT customer_id, list_type, id, version_no, published_at, changelog, is_customer_owned, is_default
		FROM public_fallback
		ORDER BY customer_id, list_type, is_customer_owned DESC, is_default DESC, published_at DESC, id DESC
	`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.BeanListVersionOption, 0)
	for rows.Next() {
		var row salesapp.BeanListVersionOption
		if err := rows.Scan(&row.CustomerID, &row.ListType, &row.ID, &row.VersionNo, &row.PublishedAt, &row.Changelog, &row.IsCustomerOwned, &row.IsDefault); err != nil {
			return nil, err
		}
		ownerLabel := "公共豆单"
		if row.IsCustomerOwned {
			ownerLabel = "客户豆单"
		}
		row.Label = strings.TrimSpace(fmt.Sprintf("%s %s %s", ownerLabel, row.VersionNo, row.PublishedAt))
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderCustomerPublicUsages(ctx context.Context) ([]salesapp.CustomerPublicUsageOption, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.customer_sku_public_usage", r.schema)).Scan(&exists); err != nil || !exists {
		return nil, err
	}
	q := fmt.Sprintf(`
		SELECT customer_id, use_public_sku
		FROM %s.customer_sku_public_usage
		ORDER BY customer_id
	`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.CustomerPublicUsageOption, 0)
	for rows.Next() {
		var row salesapp.CustomerPublicUsageOption
		if err := rows.Scan(&row.CustomerID, &row.UsePublicSKU); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderCustomers(ctx context.Context) ([]salesapp.CustomerOption, error) {
	q := fmt.Sprintf(`
		SELECT c.id, c.name, COALESCE(c.contact,''), COALESCE(c.phone,''), COALESCE(c.default_source_id,0), COALESCE(c.default_order_type_id,0),
			COALESCE(c.responsible_employee_id,0), COALESCE(e.name,'')
		FROM %s.customers c
		LEFT JOIN %s.company_employees e ON e.id=c.responsible_employee_id
		WHERE c.active=true
		ORDER BY c.name
	`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.CustomerOption, 0)
	for rows.Next() {
		var row salesapp.CustomerOption
		if err := rows.Scan(&row.ID, &row.Name, &row.Contact, &row.Phone, &row.DefaultSourceID, &row.DefaultOrderTypeID, &row.ResponsibleEmployeeID, &row.ResponsibleEmployeeName); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderEmployees(ctx context.Context) ([]salesapp.EmployeeOption, error) {
	q := fmt.Sprintf(`
		SELECT e.id, e.name, COALESCE(e.phone,''), COALESCE(e.department_id,0), COALESCE(d.name,'')
		FROM %s.company_employees e
		LEFT JOIN %s.company_departments d ON d.id=e.department_id
		WHERE e.active=true AND (e.account_type='internal_employee' OR COALESCE(e.account_type,'')='')
		ORDER BY e.id DESC
	`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.EmployeeOption, 0)
	for rows.Next() {
		var row salesapp.EmployeeOption
		if err := rows.Scan(&row.ID, &row.Name, &row.Phone, &row.DepartmentID, &row.Department); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderProducts(ctx context.Context) ([]salesapp.ProductOption, error) {
	sqlstr := fmt.Sprintf(`SELECT id, name, COALESCE(roast_level,''), default_price,
		COALESCE(retail_price_100g, 0),
		COALESCE(retail_price_200g, 0),
		COALESCE(retail_price_227g, default_price, 0),
		COALESCE(retail_price_250g, 0),
		COALESCE(customer_id, 0),
		COALESCE(base_product_id, 0),
		COALESCE(NULLIF(visibility,''), 'public'),
		COALESCE(custom_type, ''),
		COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
		COALESCE(drip_bag_grams, 0)::float8,
		COALESCE(drip_box_bag_count, 0)
		FROM %s.products WHERE active=true ORDER BY name`, r.schema)
	rows, err := r.pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]salesapp.ProductOption, 0)
	for rows.Next() {
		var p salesapp.ProductOption
		if err := rows.Scan(&p.ID, &p.Name, &p.RoastLevel, &p.DefaultPrice, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.ProductKind, &p.DripBagGrams, &p.DripBoxBagCount); err != nil {
			return nil, err
		}
		if p.ProductKind == "drip_bag" {
			p.SalesUnits = []string{"bag", "box"}
		}
		p.RetailSpecs = salesdomain.RetailAvailableSpecs(salesdomain.RetailSpecPrices{
			Price100G: p.RetailPrice100G,
			Price200G: p.RetailPrice200G,
			Price227G: p.RetailPrice227G,
			Price250G: p.RetailPrice250G,
		})
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tierSQL := fmt.Sprintf(`
		SELECT id, product_id,
		       spec_g,
		       min_qty,
		       max_qty,
		       unit_price,
		       product_kind,
		       sales_unit,
		       unit_bag_count,
		       price_source_json::text
		FROM (
			SELECT id,
			       product_id,
			       COALESCE(NULLIF(spec_g,0), 454) AS spec_g,
			       COALESCE(min_qty_units, min_qty_lb) AS min_qty,
			       COALESCE(max_qty_units, max_qty_lb) AS max_qty,
			       COALESCE(price_per_unit, price_per_lb) AS unit_price,
			       COALESCE(NULLIF(product_kind,''), 'roasted_bean') AS product_kind,
			       COALESCE(sales_unit, '') AS sales_unit,
			       COALESCE(unit_bag_count, 0) AS unit_bag_count,
			       COALESCE(price_source_json, '{}'::jsonb) AS price_source_json
			FROM %[1]s.product_price_tiers
			WHERE active=true
		) direct_tiers
		ORDER BY product_id, spec_g, min_qty
	`, r.schema)
	trs, err := r.pool.Query(ctx, tierSQL)
	if err != nil {
		return out, nil
	}
	defer trs.Close()

	tierMap := map[int64][]salesapp.ProductTierOption{}
	for trs.Next() {
		var tid, pid int64
		var specG int64
		var min float64
		var max *float64
		var price float64
		var productKind, salesUnit, priceSourceJSON string
		var unitBagCount int64
		if err := trs.Scan(&tid, &pid, &specG, &min, &max, &price, &productKind, &salesUnit, &unitBagCount, &priceSourceJSON); err != nil {
			return nil, err
		}
		tierMap[pid] = append(tierMap[pid], salesapp.ProductTierOption{ID: tid, SpecG: specG, MinQty: min, MaxQty: max, UnitPrice: price, ProductKind: productKind, SalesUnit: salesUnit, UnitBagCount: unitBagCount, PriceSourceJSON: priceSourceJSON})
	}
	if err := trs.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		out[i].Tiers = tierMap[out[i].ID]
	}
	commercialPublicationTiers, err := r.fetchCommercialOrderPublicationTiers(ctx, out)
	if err != nil {
		return nil, err
	}
	applyCommercialOrderPublicationTiers(out, commercialPublicationTiers)
	greenPublicationTiers, err := r.fetchGreenBeanOrderPublicationTiers(ctx, out)
	if err != nil {
		return nil, err
	}
	applyGreenBeanOrderPublicationTiers(out, greenPublicationTiers)
	return out, nil
}

func (r Repository) fetchCommercialOrderPublicationTiers(ctx context.Context, products []salesapp.ProductOption) (map[int64][]salesapp.ProductTierOption, error) {
	customerOwners := map[string]bool{}
	hasCommercialProduct := false
	for _, product := range products {
		if !orderCommercialProductKind(product.ProductKind) {
			continue
		}
		hasCommercialProduct = true
		if product.CustomerID > 0 {
			customerOwners[strconv.FormatInt(product.CustomerID, 10)] = true
		}
	}
	if !hasCommercialProduct {
		return map[int64][]salesapp.ProductTierOption{}, nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.bean_list_publications", r.schema)).Scan(&exists); err != nil || !exists {
		return map[int64][]salesapp.ProductTierOption{}, err
	}
	ownerKeys := make([]string, 0, len(customerOwners))
	for key := range customerOwners {
		ownerKeys = append(ownerKeys, key)
	}
	q := fmt.Sprintf(`
		WITH customer_publications AS (
			SELECT owner_key,
			       id,
			       COALESCE(version_no, '') AS version_no,
			       COALESCE(content_json, '{}'::jsonb) AS content_json,
			       row_number() OVER (PARTITION BY owner_key ORDER BY published_at DESC, id DESC) AS rn
			FROM %[1]s.bean_list_publications
			WHERE status='published'
			  AND list_type='commercial'
			  AND owner_type='customer'
			  AND owner_key = ANY($1)
		),
		official_publications AS (
			SELECT id,
			       COALESCE(version_no, '') AS version_no,
			       COALESCE(content_json, '{}'::jsonb) AS content_json,
			       row_number() OVER (ORDER BY published_at DESC, id DESC) AS rn
			FROM %[1]s.bean_list_publications
			WHERE status='published'
			  AND list_type='commercial'
			  AND owner_type='official'
		)
		SELECT 'customer' AS owner_type, owner_key, id, version_no, content_json
		FROM customer_publications
		WHERE rn=1
		UNION ALL
		SELECT 'official' AS owner_type, '' AS owner_key, id, version_no, content_json
		FROM official_publications
		WHERE rn=1
	`, r.schema)
	rows, err := r.pool.Query(ctx, q, ownerKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	customerTiers := map[string]map[int64][]salesapp.ProductTierOption{}
	officialTiers := map[int64][]salesapp.ProductTierOption{}
	for rows.Next() {
		var ownerType, ownerKey, versionNo string
		var publicationID int64
		var contentRaw []byte
		if err := rows.Scan(&ownerType, &ownerKey, &publicationID, &versionNo, &contentRaw); err != nil {
			return nil, err
		}
		tiers := commercialOrderTierMapFromPublicationContent(publicationID, versionNo, contentRaw)
		if ownerType == "customer" {
			customerTiers[ownerKey] = tiers
			continue
		}
		officialTiers = tiers
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := map[int64][]salesapp.ProductTierOption{}
	for _, product := range products {
		if !orderCommercialProductKind(product.ProductKind) {
			continue
		}
		if product.CustomerID > 0 {
			ownerKey := strconv.FormatInt(product.CustomerID, 10)
			if tiers := customerTiers[ownerKey][product.ID]; len(tiers) > 0 {
				out[product.ID] = tiers
			}
			continue
		}
		if tiers := officialTiers[product.ID]; len(tiers) > 0 {
			out[product.ID] = tiers
		}
	}
	return out, nil
}

func applyCommercialOrderPublicationTiers(products []salesapp.ProductOption, publicationTiers map[int64][]salesapp.ProductTierOption) {
	for i := range products {
		if !orderCommercialProductKind(products[i].ProductKind) {
			continue
		}
		tiers := publicationTiers[products[i].ID]
		if len(tiers) == 0 {
			continue
		}
		products[i].Tiers = append([]salesapp.ProductTierOption(nil), tiers...)
	}
}

func orderCommercialProductKind(productKind string) bool {
	switch strings.TrimSpace(productKind) {
	case "green_bean", "drip_bag":
		return false
	default:
		return true
	}
}

func (r Repository) fetchGreenBeanOrderPublicationTiers(ctx context.Context, products []salesapp.ProductOption) (map[int64][]salesapp.ProductTierOption, error) {
	customerOwners := map[string]bool{}
	hasGreenBeanProduct := false
	for _, product := range products {
		if strings.TrimSpace(product.ProductKind) != "green_bean" {
			continue
		}
		hasGreenBeanProduct = true
		if product.CustomerID > 0 {
			customerOwners[strconv.FormatInt(product.CustomerID, 10)] = true
		}
	}
	if !hasGreenBeanProduct {
		return map[int64][]salesapp.ProductTierOption{}, nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.bean_list_publications", r.schema)).Scan(&exists); err != nil || !exists {
		return map[int64][]salesapp.ProductTierOption{}, err
	}
	ownerKeys := make([]string, 0, len(customerOwners))
	for key := range customerOwners {
		ownerKeys = append(ownerKeys, key)
	}
	q := fmt.Sprintf(`
		WITH customer_publications AS (
			SELECT owner_key,
			       id,
			       COALESCE(version_no, '') AS version_no,
			       COALESCE(config_json, '{}'::jsonb) AS config_json,
			       COALESCE(content_json, '{}'::jsonb) AS content_json,
			       row_number() OVER (PARTITION BY owner_key ORDER BY published_at DESC, id DESC) AS rn
			FROM %[1]s.bean_list_publications
			WHERE status='published'
			  AND list_type='green'
			  AND owner_type='customer'
			  AND owner_key = ANY($1)
		),
		official_publications AS (
			SELECT id,
			       COALESCE(version_no, '') AS version_no,
			       COALESCE(config_json, '{}'::jsonb) AS config_json,
			       COALESCE(content_json, '{}'::jsonb) AS content_json,
			       row_number() OVER (ORDER BY published_at DESC, id DESC) AS rn
			FROM %[1]s.bean_list_publications
			WHERE status='published'
			  AND list_type='green'
			  AND owner_type='official'
		)
		SELECT 'customer' AS owner_type, owner_key, id, version_no, config_json, content_json
		FROM customer_publications
		WHERE rn=1
		UNION ALL
		SELECT 'official' AS owner_type, '' AS owner_key, id, version_no, config_json, content_json
		FROM official_publications
		WHERE rn=1
	`, r.schema)
	rows, err := r.pool.Query(ctx, q, ownerKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	customerTiers := map[string]map[int64][]salesapp.ProductTierOption{}
	officialTiers := map[int64][]salesapp.ProductTierOption{}
	for rows.Next() {
		var ownerType, ownerKey, versionNo string
		var publicationID int64
		var configRaw, contentRaw []byte
		if err := rows.Scan(&ownerType, &ownerKey, &publicationID, &versionNo, &configRaw, &contentRaw); err != nil {
			return nil, err
		}
		tiers := greenBeanOrderTierMapFromPublicationContent(publicationID, versionNo, contentRaw, configRaw)
		if ownerType == "customer" {
			customerTiers[ownerKey] = tiers
			continue
		}
		officialTiers = tiers
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := map[int64][]salesapp.ProductTierOption{}
	for _, product := range products {
		if strings.TrimSpace(product.ProductKind) != "green_bean" {
			continue
		}
		ownerKey := ""
		if product.CustomerID > 0 {
			ownerKey = strconv.FormatInt(product.CustomerID, 10)
		}
		if tiers := customerTiers[ownerKey][product.ID]; len(tiers) > 0 {
			out[product.ID] = tiers
			continue
		}
		if tiers := officialTiers[product.ID]; len(tiers) > 0 {
			out[product.ID] = tiers
		}
	}
	return out, nil
}

func applyGreenBeanOrderPublicationTiers(products []salesapp.ProductOption, publicationTiers map[int64][]salesapp.ProductTierOption) {
	for i := range products {
		if strings.TrimSpace(products[i].ProductKind) != "green_bean" {
			continue
		}
		products[i].Tiers = append([]salesapp.ProductTierOption(nil), publicationTiers[products[i].ID]...)
		if products[i].Tiers == nil {
			products[i].Tiers = []salesapp.ProductTierOption{}
		}
	}
}

type orderBeanListPublicationContent struct {
	Groups []struct {
		Items []json.RawMessage `json:"items"`
	} `json:"groups"`
}

type orderGreenBeanPublicationTier struct {
	Label          string   `json:"label"`
	SpecG          int64    `json:"spec_g"`
	MinQty         float64  `json:"min_qty"`
	MaxQty         *float64 `json:"max_qty"`
	PricePerUnit   float64  `json:"price_per_unit"`
	PricePerKg     float64  `json:"price_per_kg"`
	PricePerLb     float64  `json:"price_per_lb"`
	TemplateID     int64    `json:"template_id"`
	TemplateTierID int64    `json:"template_tier_id"`
	DisplayUnit    string   `json:"display_unit"`
	PriceUnit      string   `json:"price_unit"`
}

type orderCommercialPublicationTier = orderGreenBeanPublicationTier

func commercialOrderTierMapFromPublicationContent(publicationID int64, versionNo string, raw []byte) map[int64][]salesapp.ProductTierOption {
	out := map[int64][]salesapp.ProductTierOption{}
	if publicationID <= 0 || len(raw) == 0 {
		return out
	}
	var content orderBeanListPublicationContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return out
	}
	for _, group := range content.Groups {
		for _, itemRaw := range group.Items {
			productID := orderBeanListProductID(itemRaw)
			if productID <= 0 {
				continue
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(itemRaw, &fields); err != nil {
				continue
			}
			var tiers []orderCommercialPublicationTier
			if data, ok := fields["commercial_wholesale_tiers"]; !ok || json.Unmarshal(data, &tiers) != nil {
				continue
			}
			for idx, tier := range tiers {
				option := commercialOrderTierOption(publicationID, versionNo, idx, tier)
				if option.UnitPrice <= 0 {
					continue
				}
				out[productID] = append(out[productID], option)
			}
		}
	}
	return out
}

func greenBeanOrderTierMapFromPublicationContent(publicationID int64, versionNo string, raw []byte, configRaw ...[]byte) map[int64][]salesapp.ProductTierOption {
	out := map[int64][]salesapp.ProductTierOption{}
	if publicationID <= 0 || len(raw) == 0 {
		return out
	}
	overrides := map[string]map[string]float64{}
	if len(configRaw) > 0 {
		overrides = greenBeanOrderPriceOverridesFromPublicationConfig(configRaw[0])
	}
	var content orderBeanListPublicationContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return out
	}
	for _, group := range content.Groups {
		for _, itemRaw := range group.Items {
			productID := orderBeanListProductID(itemRaw)
			if productID <= 0 {
				continue
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(itemRaw, &fields); err != nil {
				continue
			}
			var tiers []orderGreenBeanPublicationTier
			if data, ok := fields["green_bean_sale_tiers"]; !ok || json.Unmarshal(data, &tiers) != nil {
				continue
			}
			itemName := rawJSONString(fields["name"])
			productOverrides := overrides[strconv.FormatInt(productID, 10)]
			if len(productOverrides) == 0 && itemName != "" {
				productOverrides = overrides[itemName]
			}
			for idx, tier := range tiers {
				if price, ok := greenBeanOrderManualPriceOverride(tier, productOverrides); ok {
					applyGreenBeanOrderManualPrice(&tier, price)
				}
				option := greenBeanOrderTierOption(publicationID, versionNo, idx, tier)
				if option.UnitPrice <= 0 {
					continue
				}
				out[productID] = append(out[productID], option)
			}
		}
	}
	return out
}

func greenBeanOrderPriceOverridesFromPublicationConfig(raw []byte) map[string]map[string]float64 {
	if len(raw) == 0 {
		return nil
	}
	var config struct {
		Customizers map[string]struct {
			GreenPriceOverrides map[string]float64 `json:"greenPriceOverrides"`
		} `json:"customizers"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil
	}
	out := map[string]map[string]float64{}
	for productKey, customizer := range config.Customizers {
		key := strings.TrimSpace(productKey)
		if key == "" {
			continue
		}
		for tierKey, price := range customizer.GreenPriceOverrides {
			tierKey = strings.TrimSpace(tierKey)
			if tierKey == "" || price <= 0 {
				continue
			}
			if out[key] == nil {
				out[key] = map[string]float64{}
			}
			out[key][tierKey] = price
		}
	}
	return out
}

func greenBeanOrderManualPriceOverride(tier orderGreenBeanPublicationTier, overrides map[string]float64) (float64, bool) {
	if len(overrides) == 0 {
		return 0, false
	}
	for _, key := range []string{
		strconv.FormatInt(tier.TemplateTierID, 10),
		strings.TrimSpace(tier.Label),
	} {
		if key == "" || key == "0" {
			continue
		}
		price, ok := overrides[key]
		if ok && price > 0 {
			return price, true
		}
	}
	return 0, false
}

func applyGreenBeanOrderManualPrice(tier *orderGreenBeanPublicationTier, price float64) {
	unitPrice := roundOrderPrice(price)
	priceUnit := greenBeanOrderPriceUnit(tier.DisplayUnit, tier.PriceUnit, true)
	unitG := greenBeanOrderPriceUnitG(priceUnit, tier.SpecG)
	pricePerKg := unitPrice
	if unitG > 0 && unitG != 1000 {
		pricePerKg = unitPrice * 1000.0 / unitG
	}
	tier.PriceUnit = priceUnit
	tier.PricePerUnit = unitPrice
	tier.PricePerKg = roundOrderPrice(pricePerKg)
	tier.PricePerLb = roundOrderPrice(pricePerKg * 0.454)
}

func orderBeanListProductID(raw json.RawMessage) int64 {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return 0
	}
	for _, key := range []string{"productId", "product_id", "productID"} {
		if id := rawJSONInt64(fields[key]); id > 0 {
			return id
		}
	}
	return 0
}

func rawJSONInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return id
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		id, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		return id
	}
	return 0
}

func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func commercialOrderTierOption(publicationID int64, versionNo string, idx int, tier orderCommercialPublicationTier) salesapp.ProductTierOption {
	specG := tier.SpecG
	if specG <= 0 {
		specG = 454
	}
	displayUnit := normalizeGreenBeanOrderPriceUnit(tier.DisplayUnit)
	if displayUnit == "" {
		displayUnit = "lb"
	}
	priceUnit := normalizeGreenBeanOrderPriceUnit(tier.PriceUnit)
	if priceUnit == "" {
		priceUnit = displayUnit
	}
	priceUnit = greenBeanOrderPriceUnit(displayUnit, priceUnit, false)
	unitPrice := greenBeanOrderTierPrice(orderGreenBeanPublicationTier(tier), specG, displayUnit, priceUnit)
	id := tier.TemplateTierID
	if id <= 0 {
		id = publicationID*100000 + int64(idx+1)
	}
	source := map[string]any{
		"source":           "published_bean_list",
		"list_type":        "commercial",
		"publication_id":   publicationID,
		"version_no":       versionNo,
		"template_id":      tier.TemplateID,
		"template_tier_id": tier.TemplateTierID,
		"display_unit":     displayUnit,
		"price_unit":       priceUnit,
	}
	sourceJSON, _ := json.Marshal(source)
	return salesapp.ProductTierOption{
		ID:              id,
		SpecG:           specG,
		MinQty:          tier.MinQty,
		MaxQty:          tier.MaxQty,
		UnitPrice:       unitPrice,
		DisplayUnit:     priceUnit,
		ProductKind:     "roasted_bean",
		PriceSourceJSON: string(sourceJSON),
	}
}

func greenBeanOrderTierOption(publicationID int64, versionNo string, idx int, tier orderGreenBeanPublicationTier) salesapp.ProductTierOption {
	specG := tier.SpecG
	if specG <= 0 {
		specG = 1000
	}
	displayUnit := strings.TrimSpace(strings.ToLower(tier.DisplayUnit))
	if displayUnit == "" {
		displayUnit = "kg"
	}
	priceUnit := strings.TrimSpace(strings.ToLower(tier.PriceUnit))
	if priceUnit == "" {
		priceUnit = displayUnit
	}
	priceUnit = greenBeanOrderPriceUnit(displayUnit, priceUnit, false)
	unitPrice := greenBeanOrderTierPrice(tier, specG, displayUnit, priceUnit)
	id := tier.TemplateTierID
	if id <= 0 {
		id = publicationID*100000 + int64(idx+1)
	}
	source := map[string]any{
		"source":           "published_bean_list",
		"list_type":        "green",
		"publication_id":   publicationID,
		"version_no":       versionNo,
		"template_id":      tier.TemplateID,
		"template_tier_id": tier.TemplateTierID,
		"display_unit":     displayUnit,
		"price_unit":       priceUnit,
	}
	sourceJSON, _ := json.Marshal(source)
	return salesapp.ProductTierOption{
		ID:              id,
		SpecG:           specG,
		MinQty:          tier.MinQty,
		MaxQty:          tier.MaxQty,
		UnitPrice:       unitPrice,
		DisplayUnit:     priceUnit,
		ProductKind:     "green_bean",
		PriceSourceJSON: string(sourceJSON),
	}
}

func greenBeanOrderTierPrice(tier orderGreenBeanPublicationTier, specG int64, displayUnit string, priceUnit string) float64 {
	switch priceUnit {
	case "kg":
		if tier.PricePerKg > 0 {
			return roundOrderPrice(tier.PricePerKg)
		}
		if displayUnit == "kg" && tier.PricePerUnit > 0 {
			return roundOrderPrice(tier.PricePerUnit)
		}
		if tier.PricePerLb > 0 {
			return roundOrderPrice(tier.PricePerLb / 0.454)
		}
		return roundOrderPrice(greenBeanOrderTierPricePerLb(tier, specG, displayUnit) / 0.454)
	default:
		return greenBeanOrderTierPricePerLb(tier, specG, displayUnit)
	}
}

func greenBeanOrderTierPricePerLb(tier orderGreenBeanPublicationTier, specG int64, displayUnit string) float64 {
	if tier.PricePerLb > 0 {
		return roundOrderPrice(tier.PricePerLb)
	}
	if tier.PricePerKg > 0 {
		return roundOrderPrice(tier.PricePerKg * 454.0 / 1000.0)
	}
	if specG <= 0 {
		specG = 1000
	}
	if tier.PricePerUnit <= 0 {
		return 0
	}
	switch displayUnit {
	case "lb":
		return roundOrderPrice(tier.PricePerUnit)
	case "kg":
		return roundOrderPrice(tier.PricePerUnit * 454.0 / 1000.0)
	default:
		pricePerKg := tier.PricePerUnit * 1000.0 / float64(specG)
		return roundOrderPrice(pricePerKg * 454.0 / 1000.0)
	}
}

func greenBeanOrderPriceUnit(displayUnit string, explicitUnit string, preferDisplay bool) string {
	display := normalizeGreenBeanOrderPriceUnit(displayUnit)
	explicit := normalizeGreenBeanOrderPriceUnit(explicitUnit)
	if preferDisplay {
		if display != "" {
			return display
		}
		if explicit != "" {
			return explicit
		}
		return "lb"
	}
	if explicit != "" {
		return explicit
	}
	if display != "" {
		return display
	}
	return "lb"
}

func normalizeGreenBeanOrderPriceUnit(unit string) string {
	switch strings.TrimSpace(strings.ToLower(unit)) {
	case "kg", "lb", "g100", "g227", "g250":
		return strings.TrimSpace(strings.ToLower(unit))
	default:
		return ""
	}
}

func greenBeanOrderPriceUnitG(unit string, specG int64) float64 {
	switch normalizeGreenBeanOrderPriceUnit(unit) {
	case "kg":
		return 1000
	case "lb":
		return 454
	case "g100":
		return 100
	case "g227":
		return 227
	case "g250":
		return 250
	default:
		if specG > 0 {
			return float64(specG)
		}
		return 454
	}
}

func roundOrderPrice(value float64) float64 {
	return math.Round((value+1e-9)*100) / 100
}

func (r Repository) fetchOrderEdit(ctx context.Context, id int64) (*salesapp.OrderEditData, error) {
	q := fmt.Sprintf(`
		SELECT
			o.id,
			o.order_no,
			to_char(o.order_date,'YYYY-MM-DD') as order_date,
			COALESCE(o.customer_id,0) as customer_id,
			COALESCE(o.source_id,0) as source_id,
			COALESCE(o.order_type_id,0) as order_type_id,
			COALESCE(o.pay_status_id,0) as pay_status_id,
			COALESCE(o.payment_method,'') as payment_method,
			COALESCE(o.ship_status_id,0) as ship_status_id,
			COALESCE(o.ship_method,'') as ship_method,
			%s as ship_tracking_no,
			COALESCE(o.logistics_company_id,0) as logistics_company_id,
			COALESCE(o.logistics_product_id,0) as logistics_product_id,
			COALESCE(o.payment_goods_amount,0) as payment_goods_amount,
			COALESCE(o.payment_shipping_amount,0) as payment_shipping_amount,
			COALESCE(o.payment_voucher_asset_id,0) as payment_voucher_asset_id,
			COALESCE(a.id,0) as payment_voucher_id,
			COALESCE(a.kind,'') as payment_voucher_kind,
			COALESCE(a.filename,'') as payment_voucher_filename,
			COALESCE(a.content_type,'') as payment_voucher_content_type,
			COALESCE(a.bytes,0) as payment_voucher_bytes,
			COALESCE(a.sha256,'') as payment_voucher_sha256,
			COALESCE(a.object_key,'') as payment_voucher_object_key,
			COALESCE(to_char(a.created_at, 'YYYY-MM-DD HH24:MI:SS'),'') as payment_voucher_created_at,
			COALESCE(a.created_by,'') as payment_voucher_created_by,
			COALESCE(o.responsible_party_type,'') as responsible_party_type,
			COALESCE(o.responsible_party_id,0) as responsible_party_id,
			COALESCE(o.responsible_party_name,'') as responsible_party_name,
			COALESCE(NULLIF(o.receiver_name,''), NULLIF(c.contact,''), c.name, '') AS receiver_name,
			COALESCE(NULLIF(o.receiver_phone,''), c.phone, '') AS receiver_phone,
			COALESCE(NULLIF(o.receiver_address,''), c.address, '') AS receiver_address,
			COALESCE(o.receiver_company, '') AS receiver_company,
			COALESCE(o.portal_service_code,'') AS portal_service_code,
			COALESCE(o.source_warehouse,'') AS source_warehouse,
			COALESCE(o.bean_list_publication_id,0) AS bean_list_publication_id,
			COALESCE(o.bean_list_version_no,'') AS bean_list_version_no,
			COALESCE(o.notes,'') as notes,
			COALESCE(o.total_amount,0) as total_amount,
			COALESCE(o.shipping_amount,0) as shipping_amount,
			COALESCE(o.discount_amount,0) as discount_amount,
			COALESCE(o.round_to_int,false) as round_to_int,
			COALESCE(o.rounding_amount,0) as rounding_amount,
			COALESCE(o.grand_total,0) as grand_total,
			COALESCE(o.express_fee,'') as express_fee,
			COALESCE(o.outsource_material_fee,0) as outsource_material_fee,
			COALESCE(o.outsource_roast_fee,0) as outsource_roast_fee,
			COALESCE(o.outsource_packaging_fee,0) as outsource_packaging_fee,
			COALESCE(o.outsource_manual_fee,0) as outsource_manual_fee,
			COALESCE(o.outsource_tax_fee,0) as outsource_tax_fee,
			COALESCE(o.outsource_other_fee,0) as outsource_other_fee,
			COALESCE(o.outsource_total_fee,0) as outsource_total_fee,
			o.is_void,
			CASE WHEN o.voided_at IS NULL THEN NULL ELSE to_char(o.voided_at, 'YYYY-MM-DD HH24:MI:SS') END AS voided_at,
			o.void_reason
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		LEFT JOIN %s.sales_order_assets a ON a.id=o.payment_voucher_asset_id
		WHERE o.id=$1
	`, orderTrackingSummaryExpr(r.schema, "o"), r.schema, r.schema, r.schema)

	var d salesapp.OrderEditData
	var totalAmt, shipAmt, discAmt, roundAmt, grandAmt float64
	var paymentGoodsAmt, paymentShippingAmt float64
	var outsourceMaterial, outsourceRoast, outsourcePackaging, outsourceManual, outsourceTax, outsourceOther, outsourceTotal float64
	var paymentVoucher salesapp.SalesOrderAsset
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&d.ID,
		&d.OrderNo,
		&d.OrderDate,
		&d.CustomerID,
		&d.SourceID,
		&d.OrderTypeID,
		&d.PayStatusID,
		&d.PaymentMethod,
		&d.ShipStatusID,
		&d.ShipMethod,
		&d.ShipTrackingNo,
		&d.LogisticsCompanyID,
		&d.LogisticsProductID,
		&paymentGoodsAmt,
		&paymentShippingAmt,
		&d.PaymentVoucherAssetID,
		&paymentVoucher.ID,
		&paymentVoucher.Kind,
		&paymentVoucher.Filename,
		&paymentVoucher.ContentType,
		&paymentVoucher.Bytes,
		&paymentVoucher.SHA256,
		&paymentVoucher.ObjectKey,
		&paymentVoucher.CreatedAt,
		&paymentVoucher.CreatedBy,
		&d.ResponsibleType,
		&d.ResponsibleID,
		&d.ResponsibleName,
		&d.ReceiverName,
		&d.ReceiverPhone,
		&d.ReceiverAddress,
		&d.ReceiverCompany,
		&d.PortalServiceCode,
		&d.SourceWarehouse,
		&d.BeanListPublicationID,
		&d.BeanListVersionNo,
		&d.Notes,
		&totalAmt,
		&shipAmt,
		&discAmt,
		&d.RoundToInt,
		&roundAmt,
		&grandAmt,
		&d.ExpressFee,
		&outsourceMaterial,
		&outsourceRoast,
		&outsourcePackaging,
		&outsourceManual,
		&outsourceTax,
		&outsourceOther,
		&outsourceTotal,
		&d.IsVoid,
		&d.VoidedAt,
		&d.VoidReason,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	d.TotalAmount = fmt.Sprintf("%.2f", totalAmt)
	d.ShippingAmount = fmt.Sprintf("%.2f", shipAmt)
	d.DiscountAmount = fmt.Sprintf("%.2f", discAmt)
	d.RoundingAmount = fmt.Sprintf("%.2f", roundAmt)
	d.GrandTotal = fmt.Sprintf("%.2f", grandAmt)
	d.PaymentGoodsAmount = fmt.Sprintf("%.2f", paymentGoodsAmt)
	d.PaymentShippingAmount = fmt.Sprintf("%.2f", paymentShippingAmt)
	if paymentVoucher.ID > 0 {
		paymentVoucher.URL = salesOrderAssetURL(paymentVoucher.ObjectKey)
		d.PaymentVoucher = &paymentVoucher
	}
	d.OutsourceMaterialFee = fmt.Sprintf("%.2f", outsourceMaterial)
	d.OutsourceRoastFee = fmt.Sprintf("%.2f", outsourceRoast)
	d.OutsourcePackagingFee = fmt.Sprintf("%.2f", outsourcePackaging)
	d.OutsourceManualFee = fmt.Sprintf("%.2f", outsourceManual)
	d.OutsourceTaxFee = fmt.Sprintf("%.2f", outsourceTax)
	d.OutsourceOtherFee = fmt.Sprintf("%.2f", outsourceOther)
	d.OutsourceTotalFee = fmt.Sprintf("%.2f", outsourceTotal)

	itemsQ := fmt.Sprintf(`
			SELECT oi.id, oi.line_no,
				COALESCE(oi.product_id,0),
				COALESCE(p.name,''),
				COALESCE(oi.item_note,''),
				COALESCE(oi.spec,''),
				COALESCE(oi.qty,0),
				COALESCE(oi.unit,''),
				COALESCE(oi.unit_price,0),
				COALESCE(oi.line_total,0),
				COALESCE(oi.price_tier_id,0),
				COALESCE(oi.bean_list_publication_id,0),
				COALESCE(oi.bean_list_version_no,''),
				COALESCE(oi.discount_type,''),
				COALESCE(oi.discount_value,0),
				COALESCE(oi.discount_amount,0),
				COALESCE(NULLIF(oi.product_kind,''), NULLIF(p.product_kind,''), 'roasted'),
				COALESCE(oi.sales_unit,''),
				COALESCE(oi.unit_bag_count,0),
				COALESCE(oi.unit_bean_g,0)::float8,
				COALESCE(oi.matched_price_qty,0)::float8,
				COALESCE(oi.price_source_json,'{}'::jsonb)::text
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id=$1
		ORDER BY oi.line_no, oi.id
	`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, itemsQ, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	d.Items = make([]salesapp.OrderEditItem, 0)
	for rows.Next() {
		var it salesapp.OrderEditItem
		var qty, unitPrice, lineTotal, discountValue, discountAmount, unitBeanG, matchedPriceQty float64
		if err := rows.Scan(&it.ItemID, &it.LineNo, &it.ProductID, &it.Product, &it.Note, &it.Spec, &qty, &it.Unit, &unitPrice, &lineTotal, &it.PriceTierID, &it.BeanListPublicationID, &it.BeanListVersionNo, &it.DiscountType, &discountValue, &discountAmount, &it.ProductKind, &it.SalesUnit, &it.UnitBagCount, &unitBeanG, &matchedPriceQty, &it.PriceSourceJSON); err != nil {
			return nil, err
		}
		it.Qty = trimFloatZero(qty)
		it.UnitPrice = fmt.Sprintf("%.2f", unitPrice)
		it.LineTotal = fmt.Sprintf("%.2f", lineTotal)
		it.DiscountValue = trimFloatZero(discountValue)
		it.DiscountAmount = fmt.Sprintf("%.2f", discountAmount)
		it.UnitBeanG = trimFloatZero(unitBeanG)
		it.MatchedPriceQty = trimFloatZero(matchedPriceQty)
		it.UnitConversionLabel = orderEditUnitConversionLabel(it.SalesUnit, it.UnitBagCount, unitBeanG)
		d.Items = append(d.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &d, nil
}

func (r Repository) LoadSenderProfile(ctx context.Context) (salesapp.SenderProfile, error) {
	profile := salesapp.SenderProfile{
		Label:     env("SENDER_LABEL", "默认寄件人"),
		Name:      env("SENDER_NAME", ""),
		Phone:     env("SENDER_PHONE", ""),
		Addr:      env("SENDER_ADDR", ""),
		Company:   env("SENDER_COMPANY", ""),
		Goods:     env("SENDER_GOODS", "茶叶"),
		BizType:   env("SF_BIZ_TYPE", ""),
		IsDefault: true,
		Active:    true,
	}
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(sender_label,''), COALESCE(sender_name,''), COALESCE(sender_phone,''), COALESCE(sender_addr,''), COALESCE(sender_company,''), COALESCE(sender_goods,''), COALESCE(sf_biz_type,''), is_default, active
		FROM %s.sender_settings
		WHERE active=true
		ORDER BY is_default DESC, id
		LIMIT 1
	`, r.schema)).Scan(
		&profile.ID, &profile.Label, &profile.Name, &profile.Phone, &profile.Addr, &profile.Company, &profile.Goods, &profile.BizType, &profile.IsDefault, &profile.Active,
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return salesapp.SenderProfile{}, err
	}
	return profile, nil
}

func (r Repository) LoadSenderProfileByID(ctx context.Context, id int64) (salesapp.SenderProfile, error) {
	var profile salesapp.SenderProfile
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(sender_label,''), COALESCE(sender_name,''), COALESCE(sender_phone,''), COALESCE(sender_addr,''), COALESCE(sender_company,''), COALESCE(sender_goods,''), COALESCE(sf_biz_type,''), is_default, active
		FROM %s.sender_settings
		WHERE id=$1 AND active=true
	`, r.schema), id).Scan(
		&profile.ID, &profile.Label, &profile.Name, &profile.Phone, &profile.Addr, &profile.Company, &profile.Goods, &profile.BizType, &profile.IsDefault, &profile.Active,
	)
	if err != nil {
		return salesapp.SenderProfile{}, err
	}
	return profile, nil
}

func (r Repository) ListSenderProfiles(ctx context.Context) ([]salesapp.SenderProfile, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(sender_label,''), COALESCE(sender_name,''), COALESCE(sender_phone,''), COALESCE(sender_addr,''), COALESCE(sender_company,''), COALESCE(sender_goods,''), COALESCE(sf_biz_type,''), is_default, active
		FROM %s.sender_settings
		WHERE active=true
		ORDER BY is_default DESC, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.SenderProfile, 0)
	for rows.Next() {
		var profile salesapp.SenderProfile
		if err := rows.Scan(&profile.ID, &profile.Label, &profile.Name, &profile.Phone, &profile.Addr, &profile.Company, &profile.Goods, &profile.BizType, &profile.IsDefault, &profile.Active); err != nil {
			return nil, err
		}
		out = append(out, profile)
	}
	return out, rows.Err()
}

func (r Repository) SaveSenderProfile(ctx context.Context, profile salesapp.SenderProfile) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if profile.IsDefault {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.sender_settings SET is_default=false WHERE is_default=true`, r.schema)); err != nil {
			return err
		}
	}
	if profile.ID > 0 {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.sender_settings
			SET sender_label=$2,sender_name=$3,sender_phone=$4,sender_addr=$5,sender_company=$6,sender_goods=$7,sf_biz_type=$8,is_default=$9,active=$10,updated_at=now()
			WHERE id=$1
		`, r.schema), profile.ID, profile.Label, profile.Name, profile.Phone, profile.Addr, profile.Company, profile.Goods, profile.BizType, profile.IsDefault, profile.Active)
	} else {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
			VALUES ((SELECT COALESCE(MAX(id),0)+1 FROM %s.sender_settings), $1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, r.schema, r.schema), profile.Label, profile.Name, profile.Phone, profile.Addr, profile.Company, profile.Goods, profile.BizType, profile.IsDefault, profile.Active)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.sender_settings
		SET is_default=true, active=true
		WHERE id=(SELECT id FROM %s.sender_settings WHERE active=true ORDER BY is_default DESC, id LIMIT 1)
		  AND NOT EXISTS (SELECT 1 FROM %s.sender_settings WHERE is_default=true AND active=true)
	`, r.schema, r.schema, r.schema)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func EnsureSenderSettingsTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sender_settings (
		id SMALLINT PRIMARY KEY DEFAULT 1,
		sender_label TEXT NOT NULL DEFAULT '',
		sender_name TEXT NOT NULL DEFAULT '',
		sender_phone TEXT NOT NULL DEFAULT '',
		sender_addr TEXT NOT NULL DEFAULT '',
		sender_company TEXT NOT NULL DEFAULT '',
		sender_goods TEXT NOT NULL DEFAULT '茶叶',
		sf_biz_type TEXT NOT NULL DEFAULT '',
		is_default BOOLEAN NOT NULL DEFAULT false,
		active BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	ALTER TABLE %s.sender_settings ADD COLUMN IF NOT EXISTS sender_label TEXT NOT NULL DEFAULT '';
	ALTER TABLE %s.sender_settings ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;
	ALTER TABLE %s.sender_settings ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
	INSERT INTO %s.sender_settings(id, sender_label, is_default, active) VALUES(1, '默认寄件人', true, true) ON CONFLICT (id) DO NOTHING;
	UPDATE %s.sender_settings
	SET sender_label=COALESCE(NULLIF(sender_label,''), NULLIF(sender_name,''), '默认寄件人');
	UPDATE %s.sender_settings
	SET is_default=true, active=true
	WHERE id=(SELECT id FROM %s.sender_settings ORDER BY is_default DESC, id LIMIT 1)
	  AND NOT EXISTS (SELECT 1 FROM %s.sender_settings WHERE is_default=true AND active=true);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_sender_settings_one_default ON %s.sender_settings ((is_default)) WHERE is_default=true AND active=true;`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func trimFloatZero(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}

func orderEditUnitConversionLabel(salesUnit string, unitBagCount int64, unitBeanG float64) string {
	switch strings.TrimSpace(salesUnit) {
	case "bag":
		if unitBeanG > 0 {
			return trimFloatZero(unitBeanG) + "g/袋"
		}
	case "box":
		if unitBagCount > 0 {
			return strconv.FormatInt(unitBagCount, 10) + "袋/盒"
		}
	}
	return ""
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
