package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"
	"orderapp/internal/infrastructure/postgres/orderbeans"

	"github.com/jackc/pgx/v5"
)

func (r Repository) LoadServicePage(ctx context.Context, query customerportalapp.ServicePageQuery) (customerportalapp.ServicePage, error) {
	limit := query.Limit
	defaultLimit := 20
	maxLimit := 50
	if query.Key == customerportalapp.ServiceKeySettlement {
		defaultLimit = 200
		maxLimit = 200
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	page := customerportalapp.ServicePage{Key: query.Key}
	var err error
	switch query.Key {
	case customerportalapp.ServiceKeyBeanList:
		page.BeanLists, err = r.listBeanLists(ctx, query.CustomerID, limit)
	case customerportalapp.ServiceKeyOrders:
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, true)
	case customerportalapp.ServiceKeyProductOrder:
		if page.Products, err = r.listProducts(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, true)
	case customerportalapp.ServiceKeyDirectShip:
		if page.Products, err = r.listProducts(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		if page.DirectShipBatches, err = r.listDirectShipBatches(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, true)
	case customerportalapp.ServiceKeyProcessing:
		if page.Products, err = r.listProducts(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		if page.Inventory, err = r.listInventory(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.ProcessingRequests, err = r.listProcessingRequests(ctx, query.CustomerID, limit)
	case customerportalapp.ServiceKeyInventory:
		page.Inventory, err = r.listInventory(ctx, query.CustomerID, limit)
	case customerportalapp.ServiceKeyShipping:
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, true)
	case customerportalapp.ServiceKeySettlement:
		if page.FeeItems, err = r.listFeeItems(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		if page.SettlementBatches, err = r.listSettlementBatches(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, false)
	default:
		err = fmt.Errorf("service key invalid")
	}
	if err != nil {
		return customerportalapp.ServicePage{}, err
	}
	return page, nil
}

func (r Repository) LoadMallPage(ctx context.Context, customerID int64) (customerportalapp.MallPage, error) {
	page := customerportalapp.MallPage{
		ThemeKey:         customerportalapp.PortalThemeCoffeeFactory,
		MiniappEntryMode: customerportalapp.MiniappEntryModeServices,
		Products:         []customerportalapp.MallProduct{},
	}
	_ = r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(p.theme_key,''),'coffee_factory'),
		       COALESCE(NULLIF(p.miniapp_entry_mode,''),'services'),
		       COALESCE(NULLIF(p.display_name,''), c.name, '')
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		WHERE c.id=$1
	`, r.schema, r.schema), customerID).Scan(&page.ThemeKey, &page.MiniappEntryMode, &page.CurrentCustomerName)
	page.ThemeKey = customerportalapp.NormalizePortalThemeKey(page.ThemeKey)
	page.MiniappEntryMode = customerportalapp.NormalizeMiniappEntryMode(page.MiniappEntryMode)
	page.CurrentCustomerID = customerID

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT m.id, m.product_id, COALESCE(p.name,''), COALESCE(NULLIF(p.product_kind,''),'roasted'), COALESCE(NULLIF(m.title,''), p.name, ''), m.subtitle, m.description,
		       m.image_url, m.spec_g, m.unit_price, m.template_key, m.status, m.sort_order,
		       to_char(m.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.mall_products m
		JOIN %s.products p ON p.id=m.product_id
		WHERE p.active=true AND m.status='published'
		  AND %s
		ORDER BY m.sort_order, m.id
	`, r.schema, r.schema, mallProductPublicCatalogSQL("p")))
	if err != nil {
		return customerportalapp.MallPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var row customerportalapp.MallProduct
		if err := rows.Scan(&row.ID, &row.ProductID, &row.ProductName, &row.ProductKind, &row.Title, &row.Subtitle, &row.Description, &row.ImageURL, &row.SpecG, &row.UnitPrice, &row.TemplateKey, &row.Status, &row.SortOrder, &row.UpdatedAt); err != nil {
			return customerportalapp.MallPage{}, err
		}
		row.TemplateKey = customerportalapp.NormalizeMallTemplateKey(row.TemplateKey)
		row.Status = customerportalapp.NormalizeMallProductStatus(row.Status)
		page.Products = append(page.Products, row)
	}
	return page, rows.Err()
}

func (r Repository) LoadBeanListPublication(ctx context.Context, customerID, publicationID int64) (customerportalapp.BeanListSummary, error) {
	var row customerportalapp.BeanListSummary
	var configJSON []byte
	var contentJSON []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM %s.bean_list_publications
		WHERE id=$1
		  AND status='published'
		  AND ((owner_type='customer' AND owner_key=$2) OR owner_type='official')
	`, r.schema), publicationID, fmt.Sprintf("%d", customerID)).
		Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &configJSON, &contentJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.BeanListSummary{}, customerportalapp.ErrBeanListPublicationNotFound
		}
		return customerportalapp.BeanListSummary{}, err
	}
	if err := parseBeanListDisplaySummary(configJSON, contentJSON, &row); err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	return row, nil
}

func (r Repository) listBeanLists(ctx context.Context, customerID int64, limit int) ([]customerportalapp.BeanListSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM %s.bean_list_publications
		WHERE owner_type='customer' AND owner_key=$1 AND status='published'
		ORDER BY published_at DESC, id DESC
		LIMIT $2
	`, r.schema), fmt.Sprintf("%d", customerID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanBeanListSummaries(rows)
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		return out, nil
	}
	return r.listLatestOfficialBeanLists(ctx, limit)
}

func (r Repository) listLatestOfficialBeanLists(ctx context.Context, limit int) ([]customerportalapp.BeanListSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM (
			SELECT DISTINCT ON (list_type) id, list_type, version_no, status, published_at, changelog, config_json, content_json
			FROM %s.bean_list_publications
			WHERE owner_type='official' AND status='published'
			ORDER BY list_type, published_at DESC, id DESC
		) latest
		ORDER BY published_at DESC, id DESC
		LIMIT $1
	`, r.schema), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBeanListSummaries(rows)
}

type beanListRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanBeanListSummaries(rows beanListRows) ([]customerportalapp.BeanListSummary, error) {
	out := make([]customerportalapp.BeanListSummary, 0)
	for rows.Next() {
		var row customerportalapp.BeanListSummary
		var configJSON []byte
		var contentJSON []byte
		if err := rows.Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &configJSON, &contentJSON); err != nil {
			return nil, err
		}
		if err := parseBeanListDisplaySummary(configJSON, contentJSON, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func populateBeanListMetadata(row *customerportalapp.BeanListSummary) {
	if row == nil || row.ID <= 0 {
		return
	}
	row.CacheKey = beanListCacheKey(*row)
}

func beanListCacheKey(row customerportalapp.BeanListSummary) string {
	version := strings.TrimSpace(row.VersionNo)
	if version == "" {
		version = "published"
	}
	return fmt.Sprintf("bean-list:%d:%s", row.ID, version)
}

func parseBeanListDisplaySummary(configJSON, contentJSON []byte, row *customerportalapp.BeanListSummary) error {
	if row == nil {
		return nil
	}
	cfg := map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return err
		}
	}
	var content map[string]any
	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &content); err != nil {
			return err
		}
	}
	if content == nil {
		content = map[string]any{}
	}

	row.BrandName = beanListMapString(cfg, "brandName", "棵凡咖啡")
	row.BrandIntro = beanListMapString(cfg, "brandIntro", "")
	row.Title = beanListMapString(content, "title", buildBeanListDisplayTitle(row.ListType, row.BrandName))
	row.Subtitle = beanListMapString(content, "subtitle", buildBeanListDisplaySubtitle(row.ListType))
	row.ListTypeLabel = beanListTypeLabel(row.ListType)
	if changelog := beanListMapString(cfg, "changelog", ""); changelog != "" {
		row.Changelog = changelog
	}
	row.LayoutStyle = beanListLayoutStyle(beanListMapString(cfg, "layoutStyle", "card"))
	row.CardsPerRow = clampBeanListInt(beanListMapNumber(cfg, "cardsPerRow", 2), 2, 1, 4)
	row.ShowVersion = beanListMapBool(cfg, "showVersion", true)
	row.ShowChangelog = beanListMapBool(cfg, "showChangelog", true)
	row.ShowCategoryNumbers = beanListMapBool(cfg, "showCategoryNumbers", true)
	row.BackgroundColor = beanListHexColor(beanListMapString(cfg, "backgroundColor", "#f8f1e5"), "#f8f1e5")
	row.FontColor = beanListHexColor(beanListMapString(cfg, "fontColor", "#171717"), "#171717")
	row.BackgroundImage = safeBeanListImageURL(beanListMapString(cfg, "backgroundImage", ""))
	row.LogoImage = safeBeanListImageURL(beanListMapString(cfg, "logoImage", ""))

	groups := make([]customerportalapp.BeanListGroupSummary, 0)
	for _, groupMap := range beanListMapsFromAny(content["groups"]) {
		group := customerportalapp.BeanListGroupSummary{
			Category:     beanListMapString(groupMap, "category", ""),
			ShowCategory: beanListMapBool(groupMap, "showCategory", true),
			Items:        make([]customerportalapp.BeanListProductSummary, 0),
		}
		for _, itemMap := range beanListMapsFromAny(groupMap["items"]) {
			highlightTerms := beanListStringList(itemMap["highlightTerms"])
			item := customerportalapp.BeanListProductSummary{
				Code:           beanListMapString(itemMap, "code", ""),
				Name:           beanListMapString(itemMap, "name", ""),
				Badge:          beanListMapString(itemMap, "badge", ""),
				BadgeLabel:     beanListMapString(itemMap, "badgeLabel", ""),
				RecommendedUse: beanListMapString(itemMap, "recommendedUse", ""),
				Flavor:         beanListMapString(itemMap, "flavor", ""),
				Description:    beanListMapString(itemMap, "description", ""),
				HighlightTerms: highlightTerms,
				Prices:         make([]customerportalapp.BeanListPriceSummary, 0),
			}
			if strings.TrimSpace(item.Name) == "" {
				continue
			}
			for _, priceMap := range beanListMapsFromAny(itemMap["prices"]) {
				price := customerportalapp.BeanListPriceSummary{
					Label: beanListMapString(priceMap, "label", ""),
					Value: beanListMapString(priceMap, "value", ""),
					Red:   beanListMapBool(priceMap, "red", false),
				}
				if price.Value == "" {
					price.Value = formatBeanListPrice(beanListMapNumber(priceMap, "price", 0), beanListMapString(priceMap, "unit", ""))
				}
				if !price.Red {
					price.Red = beanListContainsHighlight(price.Label, highlightTerms) || beanListContainsHighlight(price.Value, highlightTerms)
				}
				if strings.TrimSpace(price.Label) == "" && strings.TrimSpace(price.Value) == "" {
					continue
				}
				item.Prices = append(item.Prices, price)
			}
			group.Items = append(group.Items, item)
		}
		if len(group.Items) > 0 {
			groups = append(groups, group)
		}
	}
	row.Groups = groups
	populateBeanListMetadata(row)
	return nil
}

func beanListMapsFromAny(value any) []map[string]any {
	switch items := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]any:
		return items
	default:
		return nil
	}
}

func beanListMapString(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	if v, ok := m[key]; ok {
		switch value := v.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case fmt.Stringer:
			if strings.TrimSpace(value.String()) != "" {
				return strings.TrimSpace(value.String())
			}
		default:
			if s := strings.TrimSpace(fmt.Sprint(value)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return fallback
}

func beanListMapBool(m map[string]any, key string, fallback bool) bool {
	if m == nil {
		return fallback
	}
	if v, ok := m[key]; ok {
		switch value := v.(type) {
		case bool:
			return value
		case string:
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "true", "1", "yes":
				return true
			case "false", "0", "no":
				return false
			}
		}
	}
	return fallback
}

func beanListMapNumber(m map[string]any, key string, fallback float64) float64 {
	if m == nil {
		return fallback
	}
	if v, ok := m[key]; ok {
		switch value := v.(type) {
		case float64:
			return value
		case int:
			return float64(value)
		case int64:
			return float64(value)
		case json.Number:
			if n, err := value.Float64(); err == nil {
				return n
			}
		case string:
			if n, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				return n
			}
		}
	}
	return fallback
}

func beanListStringList(value any) []string {
	switch rows := value.(type) {
	case []string:
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if s := strings.TrimSpace(row); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if s := strings.TrimSpace(fmt.Sprint(row)); s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case string:
		parts := strings.FieldsFunc(rows, func(r rune) bool { return r == ',' || r == '，' || r == '\n' })
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if s := strings.TrimSpace(part); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func buildBeanListDisplayTitle(listType, brandName string) string {
	brand := strings.TrimSpace(brandName)
	if brand == "" {
		brand = "棵凡咖啡"
	}
	switch strings.TrimSpace(listType) {
	case "retail":
		return brand + "零售豆单"
	case "green", "green_bean":
		return brand + "生豆豆单"
	}
	return brand + "批发豆单"
}

func buildBeanListDisplaySubtitle(listType string) string {
	switch strings.TrimSpace(listType) {
	case "retail":
		return "报价含税运"
	case "green", "green_bean":
		return "生豆销售报价"
	}
	return "报价不含税、不含运"
}

func beanListTypeLabel(listType string) string {
	switch strings.TrimSpace(listType) {
	case "retail":
		return "零售"
	case "green", "green_bean":
		return "生豆"
	}
	return "商用"
}

func beanListLayoutStyle(value string) string {
	if strings.TrimSpace(value) == "table" {
		return "table"
	}
	return "card"
}

func clampBeanListInt(value float64, fallback, min, max int) int {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	n := int(value)
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func beanListHexColor(value, fallback string) string {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return fallback
	}
	for _, r := range value[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fallback
		}
	}
	return value
}

func safeBeanListImageURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:image/") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(value, "/") {
		return value
	}
	return ""
}

func beanListContainsHighlight(text string, terms []string) bool {
	if text == "" || len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term != "" && strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func formatBeanListPrice(price float64, unit string) string {
	if price <= 0 {
		return ""
	}
	value := strconv.FormatFloat(math.Round(price), 'f', 0, 64)
	out := value
	if unit = strings.TrimSpace(unit); unit != "" {
		out += "/" + unit
	}
	return out
}

func (r Repository) listProducts(ctx context.Context, customerID int64, limit int) ([]customerportalapp.ProductSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, COALESCE(NULLIF(product_kind,''),'roasted'), roast_level,
		       to_char(COALESCE(default_price,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_100g,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_200g,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_227g,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_250g,0), 'FM999999990.00')
		FROM %s.products
		WHERE active=true
		  AND %s
		ORDER BY name, id
		LIMIT $1
	`, r.schema, portalProductVisibleToCustomerSQL(r.schema+".products", "$2")), limit, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.ProductSummary, 0)
	for rows.Next() {
		var row customerportalapp.ProductSummary
		if err := rows.Scan(&row.ID, &row.Name, &row.ProductKind, &row.RoastLevel, &row.DefaultPrice, &row.RetailPrice100, &row.RetailPrice200, &row.RetailPrice227, &row.RetailPrice250); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func portalProductVisibleToCustomerSQL(productTable, customerPlaceholder string) string {
	return portalProductVisibleToCustomerAliasSQL(productTable, "", customerPlaceholder)
}

func portalProductVisibleToCustomerAliasSQL(productTable, productAlias, customerPlaceholder string) string {
	productTable = strings.TrimSpace(productTable)
	if productTable == "" {
		productTable = "products"
	}
	productAlias = strings.TrimSpace(productAlias)
	if productAlias != "" {
		productAlias += "."
	}
	return fmt.Sprintf(`(
		(
			CASE
				WHEN COALESCE(%[1]scustomer_id,0)>0 THEN COALESCE(NULLIF(%[1]svisibility,''),'customer_only')
				ELSE COALESCE(NULLIF(%[1]svisibility,''),'public')
			END <> 'customer_only'
			OR COALESCE(%[1]scustomer_id,0)=%[2]s
		)
		AND NOT (
			COALESCE(%[1]scustomer_id,0)=0
			AND EXISTS (
				SELECT 1 FROM %[3]s alias_products
				WHERE alias_products.active=true
				  AND COALESCE(alias_products.customer_id,0)=%[2]s
				  AND COALESCE(alias_products.base_product_id,0)=%[1]sid
				  AND COALESCE(NULLIF(alias_products.visibility,''),'customer_only')='customer_only'
			)
		)
	)`, productAlias, customerPlaceholder, productTable)
}

func mallProductPublicCatalogSQL(productAlias string) string {
	productAlias = strings.TrimSpace(productAlias)
	if productAlias != "" {
		productAlias += "."
	}
	return fmt.Sprintf(`(COALESCE(%scustomer_id,0)=0 AND COALESCE(NULLIF(%svisibility,''),'public')='public')`, productAlias, productAlias)
}

func (r Repository) listCustomerOrders(ctx context.Context, query customerportalapp.ServicePageQuery, limit int, includeItems bool) ([]customerportalapp.CustomerOrderSummary, error) {
	where := []string{"o.customer_id=$1", "o.is_void=false"}
	args := []any{query.CustomerID}
	if keyword := strings.TrimSpace(query.Query); keyword != "" {
		args = append(args, "%"+strings.ToLower(keyword)+"%")
		placeholder := fmt.Sprintf("$%d", len(args))
		where = append(where, fmt.Sprintf(`(
			LOWER(COALESCE(o.order_no,'')) LIKE %[1]s
			OR LOWER(COALESCE(c.contact,'')) LIKE %[1]s
			OR LOWER(COALESCE(c.name,'')) LIKE %[1]s
			OR LOWER(COALESCE(c.phone,'')) LIKE %[1]s
			OR LOWER(COALESCE(c.address,'')) LIKE %[1]s
			OR LOWER(COALESCE(c.company_address,'')) LIKE %[1]s
			OR EXISTS (SELECT 1 FROM %s.order_items oi2
				WHERE oi2.order_id=o.id
				  AND (LOWER(COALESCE(oi2.item_name,'')) LIKE %[1]s OR LOWER(COALESCE(oi2.spec,'')) LIKE %[1]s))
		)`, placeholder, r.schema))
	}
	if query.DateFrom != "" {
		args = append(args, query.DateFrom)
		where = append(where, fmt.Sprintf("o.order_date >= $%d::date", len(args)))
	}
	if query.DateTo != "" {
		args = append(args, query.DateTo)
		where = append(where, fmt.Sprintf("o.order_date <= $%d::date", len(args)))
	}
	if status := strings.TrimSpace(query.ProcessStatus); status != "" {
		args = append(args, strings.ToLower(status))
		where = append(where, fmt.Sprintf("LOWER(COALESCE(ops.name,'')) = $%d", len(args)))
	}
	if status := strings.TrimSpace(query.PayStatus); status != "" {
		args = append(args, strings.ToLower(status))
		where = append(where, fmt.Sprintf("LOWER(COALESCE(ps.name,'')) = $%d", len(args)))
	}
	if status := strings.TrimSpace(query.ShipStatus); status != "" {
		args = append(args, strings.ToLower(status))
		where = append(where, fmt.Sprintf("LOWER(COALESCE(ss.name,'')) = $%d", len(args)))
	}
	args = append(args, limit)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT o.id,
		       COALESCE(o.order_no,''),
		       COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''),
		       COALESCE(NULLIF(c.contact,''), c.name, ''),
		       COALESCE(c.phone,''),
		       COALESCE(NULLIF(c.address,''), c.company_address, ''),
		       COALESCE(ops.name,''),
		       COALESCE(ps.name,''),
		       COALESCE(o.payment_method,''),
		       COALESCE(ss.name,''),
		       COALESCE(o.ship_tracking_no,''),
		       to_char(COALESCE(o.grand_total,0), 'FM999999990.00'),
		       to_char(COALESCE(o.shipping_amount,0), 'FM999999990.00')
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		LEFT JOIN %s.order_process_statuses ops ON ops.id=o.process_status_id
		LEFT JOIN %s.pay_statuses ps ON ps.id=o.pay_status_id
		LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
		WHERE %s
		ORDER BY o.order_date DESC, o.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, r.schema, strings.Join(where, " AND "), len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.CustomerOrderSummary, 0)
	orderIDs := make([]int64, 0)
	for rows.Next() {
		var row customerportalapp.CustomerOrderSummary
		if err := rows.Scan(&row.ID, &row.OrderNo, &row.OrderDate, &row.ReceiverName, &row.ReceiverPhone, &row.ReceiverAddress, &row.ProcessStatus, &row.PayStatus, &row.PaymentMethod, &row.ShipStatus, &row.ShipTrackingNo, &row.GrandTotal, &row.ShippingAmount); err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, row.ID)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := map[int64][]customerportalapp.CustomerOrderItemSummary{}
	if includeItems {
		var err error
		items, err = r.listCustomerOrderItems(ctx, orderIDs)
		if err != nil {
			return nil, err
		}
	}
	for i := range out {
		if includeItems {
			out[i].Items = items[out[i].ID]
		}
		out[i].SalesOrderURL = fmt.Sprintf("/api/mini/orders/%d/sales-order-latest.pdf", out[i].ID)
		out[i].DeliveryNoteURL = fmt.Sprintf("/api/mini/orders/%d/delivery-note-latest.pdf", out[i].ID)
	}
	return out, nil
}

func (r Repository) CustomerOwnsOrder(ctx context.Context, customerID, orderID int64) (bool, error) {
	if customerID <= 0 || orderID <= 0 {
		return false, nil
	}
	var ok bool
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s.orders
			WHERE id=$1 AND customer_id=$2 AND is_void=false
		)
	`, r.schema), orderID, customerID).Scan(&ok)
	return ok, err
}

func (r Repository) listCustomerOrderItems(ctx context.Context, orderIDs []int64) (map[int64][]customerportalapp.CustomerOrderItemSummary, error) {
	out := make(map[int64][]customerportalapp.CustomerOrderItemSummary, len(orderIDs))
	if len(orderIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(orderIDs))
	args := make([]any, 0, len(orderIDs))
	for i, id := range orderIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT oi.order_id,
		       oi.id,
		       COALESCE(oi.item_name,''),
		       COALESCE(NULLIF(oi.product_kind,''), NULLIF(p.product_kind,''), 'roasted'),
		       COALESCE(oi.spec,''),
		       to_char(COALESCE(oi.qty,0), 'FM999999990.##'),
		       COALESCE(oi.unit,''),
		       to_char(COALESCE(oi.unit_price,0), 'FM999999990.00'),
		       to_char(COALESCE(oi.line_total,0), 'FM999999990.00'),
		       COALESCE(oi.bean_list_publication_id,0),
		       COALESCE(oi.bean_list_version_no,'')
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY oi.order_id, oi.line_no, oi.id
	`, r.schema, r.schema), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var orderID int64
		var row customerportalapp.CustomerOrderItemSummary
		if err := rows.Scan(&orderID, &row.ID, &row.ItemName, &row.ProductKind, &row.Spec, &row.Qty, &row.Unit, &row.UnitPrice, &row.LineTotal, &row.BeanListPublicationID, &row.BeanListVersionNo); err != nil {
			return nil, err
		}
		out[orderID] = append(out[orderID], row)
	}
	return out, rows.Err()
}

func (r Repository) listDirectShipBatches(ctx context.Context, customerID int64, limit int) ([]customerportalapp.DirectShipBatch, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, batch_no, source_name, status, total_rows, valid_rows, invalid_rows, note, to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.direct_ship_import_batches
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.DirectShipBatch, 0)
	for rows.Next() {
		var row customerportalapp.DirectShipBatch
		if err := rows.Scan(&row.ID, &row.BatchNo, &row.SourceName, &row.Status, &row.TotalRows, &row.ValidRows, &row.InvalidRows, &row.Note, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listInventory(ctx context.Context, customerID int64, limit int) ([]customerportalapp.InventoryItem, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, item_type, item_id, item_name, spec_g, warehouse, qty_g, qty_units, status, note, to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_inventory_items
		WHERE customer_id=$1
		ORDER BY item_type, item_name, warehouse, id
		LIMIT $2
	`, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.InventoryItem, 0)
	for rows.Next() {
		var row customerportalapp.InventoryItem
		if err := rows.Scan(&row.ID, &row.ItemType, &row.ItemID, &row.ItemName, &row.SpecG, &row.Warehouse, &row.QtyG, &row.QtyUnits, &row.Status, &row.Note, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listProcessingRequests(ctx context.Context, customerID int64, limit int) ([]customerportalapp.ProcessingRequest, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT r.id, r.request_no, r.input_material_id, COALESCE(m.name,''), r.input_qty_g,
		       r.target_product_id, COALESCE(p.name,''), r.target_spec_g, r.target_qty,
		       r.status, r.note, to_char(r.created_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(r.accepted_at,'YYYY-MM-DD HH24:MI'), ''), r.linked_work_order_id
		FROM %s.processing_job_requests r
		LEFT JOIN %s.materials m ON m.id=r.input_material_id
		LEFT JOIN %s.products p ON p.id=r.target_product_id
		WHERE r.customer_id=$1
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2
	`, r.schema, r.schema, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.ProcessingRequest, 0)
	for rows.Next() {
		var row customerportalapp.ProcessingRequest
		if err := rows.Scan(&row.ID, &row.RequestNo, &row.InputMaterialID, &row.InputMaterialName, &row.InputQtyG, &row.TargetProductID, &row.TargetProductName, &row.TargetSpecG, &row.TargetQty, &row.Status, &row.Note, &row.CreatedAt, &row.AcceptedAt, &row.LinkedWorkOrderID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listFeeItems(ctx context.Context, customerID int64, limit int) ([]customerportalapp.FeeItem, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, source_type, source_id, fee_type, to_char(amount, 'FM999999990.00'), currency,
		       to_char(occurred_at,'YYYY-MM-DD HH24:MI'), settlement_batch_id, status, note
		FROM %s.customer_fee_items
		WHERE customer_id=$1
		ORDER BY occurred_at DESC, id DESC
		LIMIT $2
	`, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.FeeItem, 0)
	for rows.Next() {
		var row customerportalapp.FeeItem
		if err := rows.Scan(&row.ID, &row.SourceType, &row.SourceID, &row.FeeType, &row.Amount, &row.Currency, &row.OccurredAt, &row.SettlementBatchID, &row.Status, &row.Note); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listSettlementBatches(ctx context.Context, customerID int64, limit int) ([]customerportalapp.SettlementBatch, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, settlement_no, COALESCE(to_char(period_from,'YYYY-MM-DD'), ''), COALESCE(to_char(period_to,'YYYY-MM-DD'), ''),
		       status, to_char(total_amount, 'FM999999990.00'), COALESCE(to_char(confirmed_at,'YYYY-MM-DD HH24:MI'), ''),
		       COALESCE(to_char(paid_at,'YYYY-MM-DD HH24:MI'), ''), to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_settlement_batches
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.SettlementBatch, 0)
	for rows.Next() {
		var row customerportalapp.SettlementBatch
		if err := rows.Scan(&row.ID, &row.SettlementNo, &row.PeriodFrom, &row.PeriodTo, &row.Status, &row.TotalAmount, &row.ConfirmedAt, &row.PaidAt, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) CreateDirectShipBatch(ctx context.Context, cmd customerportalapp.CreateDirectShipBatchCommand) (customerportalapp.DirectShipBatch, error) {
	sourceName := strings.TrimSpace(cmd.SourceName)
	if sourceName == "" {
		return customerportalapp.DirectShipBatch{}, fmt.Errorf("source_name required")
	}
	if cmd.TotalRows <= 0 {
		return customerportalapp.DirectShipBatch{}, fmt.Errorf("total_rows invalid")
	}
	note := strings.TrimSpace(cmd.Note)
	var id int64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.direct_ship_import_batches(customer_id, source_name, status, total_rows, valid_rows, invalid_rows, note, created_by_mini_user_id)
		VALUES($1,$2,'submitted',$3,$3,0,$4,$5)
		RETURNING id
	`, r.schema), cmd.CustomerID, sourceName, cmd.TotalRows, note, cmd.CreatedByMiniUserID).Scan(&id); err != nil {
		return customerportalapp.DirectShipBatch{}, err
	}
	var row customerportalapp.DirectShipBatch
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.direct_ship_import_batches
		SET batch_no='DS-' || to_char(created_at,'YYYYMMDD') || '-' || lpad(id::text,4,'0')
		WHERE id=$1
		RETURNING id, batch_no, source_name, status, total_rows, valid_rows, invalid_rows, note, to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), id).Scan(&row.ID, &row.BatchNo, &row.SourceName, &row.Status, &row.TotalRows, &row.ValidRows, &row.InvalidRows, &row.Note, &row.CreatedAt); err != nil {
		return customerportalapp.DirectShipBatch{}, err
	}
	return row, nil
}

func (r Repository) CreateProcessingRequest(ctx context.Context, cmd customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequest, error) {
	note := strings.TrimSpace(cmd.Note)
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.ensureProcessingInputInventoryTx(ctx, tx, cmd.CustomerID, cmd.InputMaterialID, cmd.InputQtyG); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	if err := r.ensureProcessingTargetProductTx(ctx, tx, cmd.CustomerID, cmd.TargetProductID); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}

	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.processing_job_requests(customer_id, input_material_id, input_qty_g, target_product_id, target_spec_g, target_qty, status, note, created_by_mini_user_id)
		VALUES($1,$2,$3,$4,$5,$6,'submitted',$7,$8)
		RETURNING id
	`, r.schema), cmd.CustomerID, cmd.InputMaterialID, cmd.InputQtyG, cmd.TargetProductID, cmd.TargetSpecG, cmd.TargetQty, note, cmd.CreatedByMiniUserID).Scan(&id); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	var row customerportalapp.ProcessingRequest
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.processing_job_requests
		SET request_no='PJ-' || to_char(created_at,'YYYYMMDD') || '-' || lpad(id::text,4,'0')
		WHERE id=$1
		RETURNING id, request_no, input_material_id, input_qty_g, target_product_id, target_spec_g, target_qty, status, note, to_char(created_at,'YYYY-MM-DD HH24:MI'), COALESCE(to_char(accepted_at,'YYYY-MM-DD HH24:MI'), ''), linked_work_order_id
	`, r.schema), id).Scan(&row.ID, &row.RequestNo, &row.InputMaterialID, &row.InputQtyG, &row.TargetProductID, &row.TargetSpecG, &row.TargetQty, &row.Status, &row.Note, &row.CreatedAt, &row.AcceptedAt, &row.LinkedWorkOrderID); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	warehouseCode, err := r.processingWarehouseForCustomerTx(ctx, tx, cmd.CustomerID)
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	if err := r.ensureProcessingWarehouseTx(ctx, tx, warehouseCode, ""); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	ct, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_processing_production_demands(
			request_id,request_no,customer_id,product_id,product_name,spec_g,target_qty,need_g,target_warehouse,status,created_at,updated_at
		)
		SELECT $1,$2,$3,$4,COALESCE(p.name,''),$5,$6,$7,$8,'planned',now(),now()
		FROM %s.products p
		WHERE p.id=$4
		  AND p.active=true
		  AND %s
		ON CONFLICT(request_id) DO UPDATE SET
			request_no=excluded.request_no,
			product_id=excluded.product_id,
			product_name=excluded.product_name,
			spec_g=excluded.spec_g,
			target_qty=excluded.target_qty,
			need_g=excluded.need_g,
			target_warehouse=excluded.target_warehouse,
			updated_at=now()
	`, r.schema, r.schema, portalProductVisibleToCustomerAliasSQL(r.schema+".products", "p", "$3")), row.ID, row.RequestNo, cmd.CustomerID, cmd.TargetProductID, cmd.TargetSpecG, int64(cmd.TargetQty), int64(cmd.TargetQty)*cmd.TargetSpecG, warehouseCode)
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	if ct.RowsAffected() == 0 {
		return customerportalapp.ProcessingRequest{}, fmt.Errorf("target product unavailable")
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	return row, nil
}

func (r Repository) ensureProcessingInputInventoryTx(ctx context.Context, tx pgx.Tx, customerID, inputMaterialID, inputQtyG int64) error {
	var availableG int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(qty_g),0)
		FROM %s.customer_inventory_items
		WHERE customer_id=$1
		  AND item_id=$2
		  AND item_type IN ('raw_bean','material','green_bean')
		  AND COALESCE(NULLIF(status,''),'available')='available'
	`, r.schema), customerID, inputMaterialID).Scan(&availableG); err != nil {
		return err
	}
	if availableG < inputQtyG {
		return fmt.Errorf("input material unavailable")
	}
	return nil
}

func (r Repository) ensureProcessingTargetProductTx(ctx context.Context, tx pgx.Tx, customerID, targetProductID int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s.products
			WHERE id=$1 AND active=true
			  AND %s
		)
	`, r.schema, portalProductVisibleToCustomerSQL(r.schema+".products", "$2")), targetProductID, customerID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("target product unavailable")
	}
	return nil
}

type mallOrderLine struct {
	MallProductID int64
	ProductID     int64
	ProductKind   string
	Title         string
	SpecG         int64
	UnitPrice     float64
}

func (r Repository) CreateMallOrder(ctx context.Context, cmd customerportalapp.CreateMallOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.orders IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}

	ids := make([]int64, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		ids = append(ids, item.MallProductID)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT m.id, m.product_id, COALESCE(NULLIF(p.product_kind,''),'roasted'), COALESCE(NULLIF(m.title,''), p.name, ''), m.spec_g, m.unit_price
		FROM %s.mall_products m
		JOIN %s.products p ON p.id=m.product_id
		WHERE m.id = ANY($1) AND m.status='published' AND p.active=true
		  AND %s
	`, r.schema, r.schema, mallProductPublicCatalogSQL("p")), ids)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	linesByMallProduct := map[int64]mallOrderLine{}
	for rows.Next() {
		var line mallOrderLine
		if err := rows.Scan(&line.MallProductID, &line.ProductID, &line.ProductKind, &line.Title, &line.SpecG, &line.UnitPrice); err != nil {
			rows.Close()
			return customerportalapp.FulfillmentOrder{}, err
		}
		linesByMallProduct[line.MallProductID] = line
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return customerportalapp.FulfillmentOrder{}, err
	}
	rows.Close()

	totalAmount := 0.0
	for _, item := range cmd.Items {
		line, ok := linesByMallProduct[item.MallProductID]
		if !ok {
			return customerportalapp.FulfillmentOrder{}, fmt.Errorf("mall product unavailable")
		}
		totalAmount += line.UnitPrice * float64(item.Qty)
	}
	shippingAmount := cmd.ShippingAmount
	if shippingAmount < 0 {
		shippingAmount = 0
	}
	grandTotal := totalAmount + shippingAmount
	senderID, err := r.defaultSenderForCustomerTx(ctx, tx, cmd.CustomerID)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	orderDate := time.Now()
	orderNo, err := nextCustomerPortalOrderNo(ctx, tx, r.schema, orderDate)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	payStatusID := customerPortalStatusID(ctx, tx, r.schema, "pay_statuses", "未付款", "未收款")
	shipStatusID := customerPortalStatusID(ctx, tx, r.schema, "ship_statuses", "未发货")
	processStatusID := customerPortalStatusID(ctx, tx, r.schema, "order_process_statuses", "待处理", "待生产")

	var orderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.orders(
			order_date,customer_id,pay_status_id,ship_status_id,process_status_id,
			receiver_name,receiver_phone,receiver_address,receiver_company,
			portal_service_code,source_warehouse,sender_id,notes,
			total_amount,shipping_amount,grand_total,order_no
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
		)
		RETURNING id
	`, r.schema),
		orderDate,
		cmd.CustomerID,
		portalNullInt(payStatusID),
		portalNullInt(shipStatusID),
		portalNullInt(processStatusID),
		strings.TrimSpace(cmd.RecipientName),
		strings.TrimSpace(cmd.RecipientPhone),
		strings.TrimSpace(cmd.RecipientAddress),
		strings.TrimSpace(cmd.RecipientCompany),
		customerportalapp.PortalServiceMall,
		"finished_goods",
		senderID,
		strings.TrimSpace(cmd.Note),
		totalAmount,
		shippingAmount,
		grandTotal,
		orderNo,
	).Scan(&orderID); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}

	for i, item := range cmd.Items {
		line := linesByMallProduct[item.MallProductID]
		lineTotal := line.UnitPrice * float64(item.Qty)
		usage, err := orderbeans.ResolveUsage(ctx, tx, r.schema, cmd.CustomerID, line.ProductID, orderbeans.ListTypeRetail)
		if err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_items(order_id,line_no,product_id,product_kind,bean_list_publication_id,bean_list_version_no,item_name,qty,unit,spec,unit_price,line_total)
			VALUES($1,$2,$3,$4,NULLIF($5,0),$6,$7,$8,'件',$9,$10,$11)
		`, r.schema), orderID, i+1, line.ProductID, line.ProductKind, usage.PublicationID, usage.VersionNo, line.Title, item.Qty, fmt.Sprintf("%dg", line.SpecG), line.UnitPrice, lineTotal); err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
	}
	_ = cmd.CreatedByMiniUserID
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	return customerportalapp.FulfillmentOrder{
		OrderID:           orderID,
		OrderNo:           orderNo,
		PortalServiceCode: customerportalapp.PortalServiceMall,
		SourceWarehouse:   "finished_goods",
	}, nil
}

func (r Repository) CreateFulfillmentOrder(ctx context.Context, cmd customerportalapp.CreateFulfillmentOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.orders IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}

	serviceCode := strings.TrimSpace(cmd.PortalServiceCode)
	sourceWarehouse := "finished_goods"
	if serviceCode == customerportalapp.PortalServiceProcessingShipment {
		sourceWarehouse, err = r.processingWarehouseForCustomerTx(ctx, tx, cmd.CustomerID)
		if err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
		if err := r.ensureProcessingWarehouseTx(ctx, tx, sourceWarehouse, ""); err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
	}
	senderID, err := r.defaultSenderForCustomerTx(ctx, tx, cmd.CustomerID)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}

	var productName string
	var productKind string
	var defaultPrice float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(name,''), COALESCE(NULLIF(product_kind,''),'roasted'), COALESCE(default_price,0)
		FROM %s.products
		WHERE id=$1 AND active=true
		  AND %s
	`, r.schema, portalProductVisibleToCustomerSQL(r.schema+".products", "$2")), cmd.ProductID, cmd.CustomerID).Scan(&productName, &productKind, &defaultPrice); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.FulfillmentOrder{}, fmt.Errorf("product unavailable")
		}
		return customerportalapp.FulfillmentOrder{}, err
	}
	productName = firstNonEmpty(strings.TrimSpace(cmd.ProductName), productName)
	unitPrice := r.portalFulfillmentUnitPriceTx(ctx, tx, cmd.CustomerID, cmd.ProductID, cmd.SpecG, cmd.Qty, defaultPrice)
	totalAmount := portalLineTotalFromDisplayUnit(unitPrice, cmd.SpecG, cmd.Qty)
	shippingAmount := cmd.ShippingAmount
	if shippingAmount < 0 {
		shippingAmount = 0
	}
	grandTotal := totalAmount + shippingAmount

	orderDate := time.Now()
	orderNo, err := nextCustomerPortalOrderNo(ctx, tx, r.schema, orderDate)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	payStatusID := customerPortalStatusID(ctx, tx, r.schema, "pay_statuses", "未付款", "未收款")
	shipStatusID := customerPortalStatusID(ctx, tx, r.schema, "ship_statuses", "未发货")
	processStatusNames := []string{"待处理", "待生产"}
	if serviceCode == customerportalapp.PortalServiceProcessingShipment {
		processStatusNames = []string{"无需生产", "库存待发货", "生产完成"}
	}
	processStatusID := customerPortalStatusID(ctx, tx, r.schema, "order_process_statuses", processStatusNames...)

	var orderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.orders(
			order_date,customer_id,pay_status_id,ship_status_id,process_status_id,
			receiver_name,receiver_phone,receiver_address,receiver_company,
			portal_service_code,source_warehouse,sender_id,notes,
			total_amount,shipping_amount,grand_total,order_no
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
		)
		RETURNING id
	`, r.schema),
		orderDate,
		cmd.CustomerID,
		portalNullInt(payStatusID),
		portalNullInt(shipStatusID),
		portalNullInt(processStatusID),
		strings.TrimSpace(cmd.RecipientName),
		strings.TrimSpace(cmd.RecipientPhone),
		strings.TrimSpace(cmd.RecipientAddress),
		strings.TrimSpace(cmd.RecipientCompany),
		serviceCode,
		sourceWarehouse,
		senderID,
		strings.TrimSpace(cmd.Note),
		totalAmount,
		shippingAmount,
		grandTotal,
		orderNo,
	).Scan(&orderID); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	usage, err := orderbeans.ResolveUsage(ctx, tx, r.schema, cmd.CustomerID, cmd.ProductID, orderbeans.ListTypeCommercial)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_items(order_id,line_no,product_id,product_kind,bean_list_publication_id,bean_list_version_no,item_name,qty,unit,spec,unit_price,line_total)
		VALUES($1,1,$2,$3,NULLIF($4,0),$5,$6,$7,'件',$8,$9,$10)
	`, r.schema), orderID, cmd.ProductID, productKind, usage.PublicationID, usage.VersionNo, productName, cmd.Qty, fmt.Sprintf("%dg", cmd.SpecG), unitPrice, totalAmount); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	return customerportalapp.FulfillmentOrder{
		OrderID:           orderID,
		OrderNo:           orderNo,
		PortalServiceCode: serviceCode,
		SourceWarehouse:   sourceWarehouse,
	}, nil
}

type portalDirectShipCapabilityConfig struct {
	SmallBatchPriceRule customerportalapp.SmallBatchPriceRule `json:"small_batch_price_rule"`
}

func (r Repository) portalFulfillmentUnitPriceTx(ctx context.Context, tx pgx.Tx, customerID, productID, specG int64, qty int64, defaultPrice float64) float64 {
	if productID <= 0 || specG <= 0 || qty <= 0 {
		return defaultPrice
	}
	rule := r.portalDirectShipSmallBatchPriceRuleTx(ctx, tx, customerID)
	tierQty := portalTierQuantityForSpec(specG, qty)
	qtyLb := float64(specG*qty) / 454.0
	tierQtyLb := qtyLb
	if adjustedQty, ok := portalSmallBatchTierQuantity(specG, qtyLb, rule); ok {
		tierQty = portalTierQuantityForSpec(specG, adjustedQty)
		tierQtyLb = float64(specG*adjustedQty) / 454.0
	}
	var packagePrice, pricePerLb float64
	q := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(price_per_unit,0), NULLIF(price_per_lb,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0),
			COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		  AND COALESCE(NULLIF(spec_g,0),454)=$2
		  AND COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) <= $3
		  AND (COALESCE(NULLIF(max_qty_units,0), max_qty_lb) IS NULL OR COALESCE(NULLIF(max_qty_units,0), max_qty_lb) >= $3)
		ORDER BY COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) DESC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID, specG, tierQty).Scan(&packagePrice, &pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	q = fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(price_per_unit,0), NULLIF(price_per_lb,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0),
			COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		  AND COALESCE(NULLIF(spec_g,0),454)=$2
		ORDER BY COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) ASC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID, specG).Scan(&packagePrice, &pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	q = fmt.Sprintf(`
		SELECT COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		  AND COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) <= $2
		  AND (
		    COALESCE(NULLIF(max_qty_lb,0), NULLIF(max_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0) IS NULL
		    OR COALESCE(NULLIF(max_qty_lb,0), NULLIF(max_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0) >= $2
		  )
		ORDER BY COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) DESC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID, tierQtyLb).Scan(&pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	q = fmt.Sprintf(`
		SELECT COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		  AND COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) <= $2
		ORDER BY COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) DESC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID, tierQtyLb).Scan(&pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	q = fmt.Sprintf(`
		SELECT COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		ORDER BY COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) ASC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID).Scan(&pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	return defaultPrice
}

func portalTierQuantityForSpec(specG int64, units int64) float64 {
	if specG >= 1000 {
		return float64(specG*units) / 1000.0
	}
	return float64(units)
}

func portalDisplayUnitG(specG int64) float64 {
	if specG >= 1000 {
		return 1000
	}
	return 454
}

func portalDisplayUnitPriceFromLb(pricePerLb float64, specG int64) float64 {
	if pricePerLb <= 0 || specG <= 0 {
		return 0
	}
	unitG := portalDisplayUnitG(specG)
	displayUnitPrice := pricePerLb * unitG / 454.0
	if unitG == 1000 {
		displayUnitPrice = math.Round(displayUnitPrice)
	}
	return displayUnitPrice
}

func portalLineTotalFromDisplayUnit(unitPrice float64, specG int64, units int64) float64 {
	if unitPrice <= 0 || specG <= 0 || units <= 0 {
		return 0
	}
	return unitPrice * float64(specG*units) / portalDisplayUnitG(specG)
}

func (r Repository) portalDirectShipSmallBatchPriceRuleTx(ctx context.Context, tx pgx.Tx, customerID int64) customerportalapp.SmallBatchPriceRule {
	if customerID <= 0 {
		return customerportalapp.SmallBatchPriceRule{}
	}
	var raw []byte
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT config_json
		FROM %s.customer_service_capabilities
		WHERE customer_id=$1 AND capability_code=$2 AND enabled=true
	`, r.schema), customerID, customerportalapp.CapabilityDirectShip).Scan(&raw)
	if err != nil {
		return customerportalapp.SmallBatchPriceRule{}
	}
	var config portalDirectShipCapabilityConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return customerportalapp.SmallBatchPriceRule{}
	}
	return portalNormalizeSmallBatchPriceRule(config.SmallBatchPriceRule)
}

func portalNormalizeSmallBatchPriceRule(rule customerportalapp.SmallBatchPriceRule) customerportalapp.SmallBatchPriceRule {
	if !rule.Enabled {
		return customerportalapp.SmallBatchPriceRule{}
	}
	if rule.ThresholdLB <= 0 {
		rule.ThresholdLB = 14
	}
	if rule.TierMinLB <= 0 {
		rule.TierMinLB = 15
	}
	if rule.TierMaxLB <= 0 {
		rule.TierMaxLB = 28
	}
	return rule
}

func portalSmallBatchTierQuantity(specG int64, qtyLb float64, rule customerportalapp.SmallBatchPriceRule) (int64, bool) {
	rule = portalNormalizeSmallBatchPriceRule(rule)
	if !rule.Enabled || specG <= 0 || qtyLb <= 0 || qtyLb >= rule.ThresholdLB {
		return 0, false
	}
	targetUnits := int64(math.Ceil(rule.TierMinLB * 454.0 / float64(specG)))
	if targetUnits < 1 {
		targetUnits = 1
	}
	return targetUnits, true
}

func (r Repository) processingWarehouseForCustomerTx(ctx context.Context, tx pgx.Tx, customerID int64) (string, error) {
	code := ""
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(processing_warehouse_code,'')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, r.schema), customerID).Scan(&code)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		code = defaultProcessingWarehouseCode(customerID)
	}
	return code, nil
}

func (r Repository) defaultSenderForCustomerTx(ctx context.Context, tx pgx.Tx, customerID int64) (int64, error) {
	var senderID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(default_sender_id,0)
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, r.schema), customerID).Scan(&senderID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	return senderID, nil
}

func nextCustomerPortalOrderNo(ctx context.Context, tx pgx.Tx, schema string, od time.Time) (string, error) {
	ymd := od.Format("20060102")
	prefix := "SO-" + ymd + "-"
	var maxNo int
	q := fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(right(order_no,4) AS INT)), 0)
		FROM %s.orders
		WHERE order_no LIKE $1
		  AND right(order_no,4) ~ '^[0-9]{4}$'
	`, schema)
	if err := tx.QueryRow(ctx, q, prefix+"%").Scan(&maxNo); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, maxNo+1), nil
}

func customerPortalStatusID(ctx context.Context, tx pgx.Tx, schema, table string, names ...string) int64 {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var id int64
		q := fmt.Sprintf("SELECT id FROM %s.%s WHERE name=$1 ORDER BY id LIMIT 1", schema, table)
		if err := tx.QueryRow(ctx, q, name).Scan(&id); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func portalNullInt(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}
