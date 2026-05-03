package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5"
)

func (r Repository) LoadServicePage(ctx context.Context, query customerportalapp.ServicePageQuery) (customerportalapp.ServicePage, error) {
	limit := query.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	page := customerportalapp.ServicePage{Key: query.Key}
	var err error
	switch query.Key {
	case customerportalapp.ServiceKeyBeanList:
		page.BeanLists, err = r.listBeanLists(ctx, query.CustomerID, limit)
	case customerportalapp.ServiceKeyOrders:
		page.Orders, err = r.listCustomerOrders(ctx, query, limit)
	case customerportalapp.ServiceKeyProductOrder:
		if page.Products, err = r.listProducts(ctx, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.Orders, err = r.listCustomerOrders(ctx, query, limit)
	case customerportalapp.ServiceKeyDirectShip:
		if page.DirectShipBatches, err = r.listDirectShipBatches(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.Orders, err = r.listCustomerOrders(ctx, query, limit)
	case customerportalapp.ServiceKeyProcessing:
		if page.Inventory, err = r.listInventory(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.ProcessingRequests, err = r.listProcessingRequests(ctx, query.CustomerID, limit)
	case customerportalapp.ServiceKeyInventory:
		page.Inventory, err = r.listInventory(ctx, query.CustomerID, limit)
	case customerportalapp.ServiceKeyShipping:
		page.Orders, err = r.listCustomerOrders(ctx, query, limit)
	case customerportalapp.ServiceKeySettlement:
		if page.FeeItems, err = r.listFeeItems(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.SettlementBatches, err = r.listSettlementBatches(ctx, query.CustomerID, limit)
	default:
		err = fmt.Errorf("service key invalid")
	}
	if err != nil {
		return customerportalapp.ServicePage{}, err
	}
	return page, nil
}

func (r Repository) LoadBeanListPublication(ctx context.Context, customerID, publicationID int64) (customerportalapp.BeanListSummary, error) {
	var row customerportalapp.BeanListSummary
	var contentJSON []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, content_json
		FROM %s.bean_list_publications
		WHERE id=$1
		  AND status='published'
		  AND ((owner_type='customer' AND owner_key=$2) OR owner_type='official')
	`, r.schema), publicationID, fmt.Sprintf("%d", customerID)).
		Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &contentJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.BeanListSummary{}, customerportalapp.ErrBeanListPublicationNotFound
		}
		return customerportalapp.BeanListSummary{}, err
	}
	groups, err := parseBeanListContentSummary(contentJSON)
	if err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	row.Groups = groups
	populateBeanListPDFMetadata(&row)
	return row, nil
}

func (r Repository) listBeanLists(ctx context.Context, customerID int64, limit int) ([]customerportalapp.BeanListSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, content_json
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
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, content_json
		FROM (
			SELECT DISTINCT ON (list_type) id, list_type, version_no, status, published_at, changelog, content_json
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
		var contentJSON []byte
		if err := rows.Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &contentJSON); err != nil {
			return nil, err
		}
		groups, err := parseBeanListContentSummary(contentJSON)
		if err != nil {
			return nil, err
		}
		row.Groups = groups
		populateBeanListPDFMetadata(&row)
		out = append(out, row)
	}
	return out, rows.Err()
}

func populateBeanListPDFMetadata(row *customerportalapp.BeanListSummary) {
	if row == nil || row.ID <= 0 {
		return
	}
	row.PDFURL = beanListPDFPath(row.ID)
	row.CacheKey = beanListCacheKey(*row)
}

func beanListPDFPath(publicationID int64) string {
	return fmt.Sprintf("/api/mini/bean-lists/%d.pdf", publicationID)
}

func beanListCacheKey(row customerportalapp.BeanListSummary) string {
	version := strings.TrimSpace(row.VersionNo)
	if version == "" {
		version = "published"
	}
	return fmt.Sprintf("bean-list:%d:%s", row.ID, version)
}

func parseBeanListContentSummary(contentJSON []byte) ([]customerportalapp.BeanListGroupSummary, error) {
	if len(contentJSON) == 0 {
		return nil, nil
	}
	var content map[string]any
	if err := json.Unmarshal(contentJSON, &content); err != nil {
		return nil, err
	}
	groups := make([]customerportalapp.BeanListGroupSummary, 0)
	for _, groupMap := range beanListMapsFromAny(content["groups"]) {
		group := customerportalapp.BeanListGroupSummary{
			Category: beanListMapString(groupMap, "category", ""),
			Items:    make([]customerportalapp.BeanListProductSummary, 0),
		}
		for _, itemMap := range beanListMapsFromAny(groupMap["items"]) {
			item := customerportalapp.BeanListProductSummary{
				Code:           beanListMapString(itemMap, "code", ""),
				Name:           beanListMapString(itemMap, "name", ""),
				BadgeLabel:     beanListMapString(itemMap, "badgeLabel", ""),
				RecommendedUse: beanListMapString(itemMap, "recommendedUse", ""),
				Flavor:         beanListMapString(itemMap, "flavor", ""),
				Description:    beanListMapString(itemMap, "description", ""),
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
	return groups, nil
}

func beanListMapsFromAny(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func beanListMapString(m map[string]any, key, fallback string) string {
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
		}
	}
	return fallback
}

func beanListMapBool(m map[string]any, key string, fallback bool) bool {
	if v, ok := m[key]; ok {
		if value, ok := v.(bool); ok {
			return value
		}
	}
	return fallback
}

func beanListMapNumber(m map[string]any, key string, fallback float64) float64 {
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

func formatBeanListPrice(price float64, unit string) string {
	if price <= 0 {
		return ""
	}
	value := strconv.FormatFloat(price, 'f', 2, 64)
	value = strings.TrimSuffix(strings.TrimRight(value, "0"), ".")
	out := "¥" + value
	if unit = strings.TrimSpace(unit); unit != "" {
		out += "/" + unit
	}
	return out
}

func (r Repository) listProducts(ctx context.Context, limit int) ([]customerportalapp.ProductSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, roast_level,
		       to_char(COALESCE(default_price,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_100g,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_200g,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_227g,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_250g,0), 'FM999999990.00')
		FROM %s.products
		WHERE active=true
		ORDER BY name, id
		LIMIT $1
	`, r.schema), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.ProductSummary, 0)
	for rows.Next() {
		var row customerportalapp.ProductSummary
		if err := rows.Scan(&row.ID, &row.Name, &row.RoastLevel, &row.DefaultPrice, &row.RetailPrice100, &row.RetailPrice200, &row.RetailPrice227, &row.RetailPrice250); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listCustomerOrders(ctx context.Context, query customerportalapp.ServicePageQuery, limit int) ([]customerportalapp.CustomerOrderSummary, error) {
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
		if err := rows.Scan(&row.ID, &row.OrderNo, &row.OrderDate, &row.ReceiverName, &row.ReceiverPhone, &row.ReceiverAddress, &row.ProcessStatus, &row.PayStatus, &row.ShipStatus, &row.ShipTrackingNo, &row.GrandTotal, &row.ShippingAmount); err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, row.ID)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items, err := r.listCustomerOrderItems(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Items = items[out[i].ID]
	}
	return out, nil
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
		       COALESCE(oi.spec,''),
		       to_char(COALESCE(oi.qty,0), 'FM999999990.##'),
		       COALESCE(oi.unit,''),
		       to_char(COALESCE(oi.unit_price,0), 'FM999999990.00'),
		       to_char(COALESCE(oi.line_total,0), 'FM999999990.00')
		FROM %s.order_items oi
		WHERE oi.order_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY oi.order_id, oi.line_no, oi.id
	`, r.schema), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var orderID int64
		var row customerportalapp.CustomerOrderItemSummary
		if err := rows.Scan(&orderID, &row.ID, &row.ItemName, &row.Spec, &row.Qty, &row.Unit, &row.UnitPrice, &row.LineTotal); err != nil {
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
	var id int64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.processing_job_requests(customer_id, input_material_id, input_qty_g, target_product_id, target_spec_g, target_qty, status, note, created_by_mini_user_id)
		VALUES($1,$2,$3,$4,$5,$6,'submitted',$7,$8)
		RETURNING id
	`, r.schema), cmd.CustomerID, cmd.InputMaterialID, cmd.InputQtyG, cmd.TargetProductID, cmd.TargetSpecG, cmd.TargetQty, note, cmd.CreatedByMiniUserID).Scan(&id); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	var row customerportalapp.ProcessingRequest
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.processing_job_requests
		SET request_no='PJ-' || to_char(created_at,'YYYYMMDD') || '-' || lpad(id::text,4,'0')
		WHERE id=$1
		RETURNING id, request_no, input_material_id, input_qty_g, target_product_id, target_spec_g, target_qty, status, note, to_char(created_at,'YYYY-MM-DD HH24:MI'), COALESCE(to_char(accepted_at,'YYYY-MM-DD HH24:MI'), ''), linked_work_order_id
	`, r.schema), id).Scan(&row.ID, &row.RequestNo, &row.InputMaterialID, &row.InputQtyG, &row.TargetProductID, &row.TargetSpecG, &row.TargetQty, &row.Status, &row.Note, &row.CreatedAt, &row.AcceptedAt, &row.LinkedWorkOrderID); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	return row, nil
}
