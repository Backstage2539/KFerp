package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	productiondomain "orderapp/internal/domain/production"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type UnprodNeedRow struct {
	ProductID                int64   `json:"product_id"`
	ParentProductID          int64   `json:"parent_product_id"`
	Product                  string  `json:"product"`
	OrderNos                 string  `json:"order_nos"`
	SpecLabel                string  `json:"spec_label"`
	SalesUnit                string  `json:"sales_unit"`
	SpecG                    int64   `json:"spec_g"`
	NeedUnits                int64   `json:"need_units"`
	NeedG                    int64   `json:"need_g"`
	InvUnits                 int64   `json:"inv_units"`
	InvLooseG                int64   `json:"inv_loose_g"`
	InvG                     int64   `json:"inv_g"`
	GapG                     int64   `json:"gap_g"`
	AvailableG               int64   `json:"-"`
	SalesSpecCount           float64 `json:"sales_spec_count"`
	InventoryQtyPerSalesUnit float64 `json:"inventory_qty_per_sales_unit"`
	InventoryUnit            string  `json:"inventory_unit"`
	NeedInventoryQty         float64 `json:"need_inventory_qty"`
	AvailableInventoryQty    float64 `json:"available_inventory_qty"`
	GapInventoryQty          float64 `json:"gap_inventory_qty"`
	GapSalesSpecCount        float64 `json:"gap_sales_spec_count"`
	SalesSpecSnapshotJSON    string  `json:"sales_spec_snapshot_json"`
	ProductionKind           string  `json:"production_kind,omitempty"`
	ProductTypeCategoryID    int64   `json:"product_type_category_id,omitempty"`
	ProductSubtypeCategoryID int64   `json:"product_subtype_category_id,omitempty"`
	ProductTypeName          string  `json:"product_type_name,omitempty"`
	ProductSubtypeName       string  `json:"product_subtype_name,omitempty"`
	OperationTemplateID      int64   `json:"operation_template_id,omitempty"`
	DemandStatus             string  `json:"demand_status,omitempty"`
	DemandStatusLabel        string  `json:"demand_status_label,omitempty"`
	DemandSelectable         bool    `json:"demand_selectable"`
	BlockingReason           string  `json:"blocking_reason,omitempty"`
	ProductionPlanID         int64   `json:"production_plan_id,omitempty"`
	ProductionPlanNo         string  `json:"production_plan_no,omitempty"`
	WorkOrderID              int64   `json:"work_order_id,omitempty"`
	WorkOrderNo              string  `json:"work_order_no,omitempty"`
}

func productionPlanOpenStatusNames() []string {
	return []string{"待处理", "待生产", "生产中", "生产完成", "已生产完成"}
}

func productionPlanOpenStatusFilter(schema, orderAlias string) string {
	names := productionPlanOpenStatusNames()
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, "'"+strings.ReplaceAll(name, "'", "''")+"'")
	}
	return fmt.Sprintf(`(
		COALESCE(%[2]s.process_status_id,0) = 0
		OR EXISTS (
			SELECT 1 FROM %[1]s.order_process_statuses ops
			WHERE ops.id=%[2]s.process_status_id
			  AND ops.name IN (%[3]s)
		)
	)`, schema, orderAlias, strings.Join(quoted, ","))
}

type productionDemandQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func fetchUnproducedNeeds(ctx context.Context, pool productionDemandQueryer, schema, from, to string, customerID int64) ([]UnprodNeedRow, error) {
	where := fmt.Sprintf(`WHERE o.is_void=false AND p.active=true AND %s
	AND COALESCE(oi.product_id,0) > 0
	AND NOT EXISTS (
		SELECT 1 FROM %s.ship_statuses ss
		WHERE ss.id=o.ship_status_id
		  AND ss.name='已发货'
	)`, productionPlanOpenStatusFilter(schema, "o"), schema)
	demandWhere := []string{"d.status='planned'", "COALESCE(d.product_id,0) > 0", "p.active=true"}
	args := []any{}
	argn := 1
	if customerID > 0 {
		where += fmt.Sprintf(" AND o.customer_id = $%d", argn)
		demandWhere = append(demandWhere, fmt.Sprintf("d.customer_id = $%d", argn))
		args = append(args, customerID)
		argn++
	}
	if s := strings.TrimSpace(from); s != "" {
		where += fmt.Sprintf(" AND o.order_date >= $%d", argn)
		demandWhere = append(demandWhere, fmt.Sprintf("d.created_at::date >= $%d::date", argn))
		args = append(args, s)
		argn++
	}
	if s := strings.TrimSpace(to); s != "" {
		where += fmt.Sprintf(" AND o.order_date <= $%d", argn)
		demandWhere = append(demandWhere, fmt.Sprintf("d.created_at::date <= $%d::date", argn))
		args = append(args, s)
		argn++
	}

	demands, err := fetchSalesOrderProductionDemands(ctx, pool, schema, where, args)
	if err != nil {
		return nil, err
	}
	processing, err := fetchCustomerProcessingProductionDemands(ctx, pool, schema, strings.Join(demandWhere, " AND "), args)
	if err != nil {
		return nil, err
	}
	demands = append(demands, processing...)
	return finalizeUnproducedNeeds(ctx, pool, schema, demands)
}

type productionDemand struct {
	UnprodNeedRow
	forceSalesSpecCount float64
	orderNos            map[string]bool
}

type productionQuantitySnapshot struct {
	SKUID                    int64   `json:"sku_id"`
	ParentProductID          int64   `json:"parent_product_id"`
	SpecLabel                string  `json:"spec_label"`
	SalesUnit                string  `json:"sales_unit"`
	InventoryUnit            string  `json:"inventory_unit"`
	InventoryQtyPerSalesUnit float64 `json:"inventory_qty_per_sales_unit"`
	ConversionSource         string  `json:"conversion_source"`
	CustomerID               int64   `json:"customer_id,omitempty"`
	TargetWarehouse          string  `json:"target_warehouse,omitempty"`
	ProcessingRequestItemID  int64   `json:"processing_request_item_id,omitempty"`
}

func fetchSalesOrderProductionDemands(ctx context.Context, pool productionDemandQueryer, schema, where string, args []any) ([]productionDemand, error) {
	// The former aggregate CTE exposed n.effective_operation_template_id; this
	// line query keeps the same resolved alias and scans it into each demand.
	q := fmt.Sprintf(`
		SELECT
			oi.product_id,
			CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END AS parent_product_id,
			COALESCE(p.name,'') AS product,
			COALESCE(NULLIF(oi.product_kind,''), NULLIF(p.product_kind,''), 'roasted_bean') AS production_kind,
			COALESCE(type_pc.id,0),
			COALESCE(subtype_pc.id,0),
			COALESCE(type_pc.name,''),
			COALESCE(subtype_pc.name,''),
			COALESCE(
				NULLIF(cpro.operation_template_id,0),
				NULLIF(cpti.operation_template_id,0),
				NULLIF(p.operation_template_id_override,0),
				NULLIF(subtype_pc.operation_template_id,0),
				type_pc.operation_template_id,
				0
			) AS effective_operation_template_id,
			COALESCE(o.order_no,''),
			COALESCE(oi.qty,0)::float8,
			(COALESCE(osd.decision,'')='produce'),
			COALESCE(oi.price_source_json,'{}'::jsonb)::text,
			COALESCE(NULLIF(oi.sales_unit,''),NULLIF(oi.unit,''),''),
			COALESCE(NULLIF(p.spec_label,''),NULLIF(p.sku_name,''),NULLIF(p.derived_spec_name,''),''),
			COALESCE(p.net_content_qty,0)::float8,
			COALESCE(p.net_content_unit,''),
			COALESCE(CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN
				COALESCE(
					NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''),
					NULLIF(parent_product_unit_template.inventory_unit,''),
					NULLIF(parent_product_config.inventory_unit,''),
					NULLIF(parent_product_category.inventory_unit,''),
					NULLIF(parent_product_parent_category.inventory_unit,'')
				)
			ELSE
				COALESCE(
					NULLIF(p.unit_rule_override_json->>'inventory_unit',''),
					NULLIF(product_unit_template.inventory_unit,''),
					NULLIF(product_config.inventory_unit,''),
					NULLIF(subtype_pc.inventory_unit,''),
					NULLIF(type_pc.inventory_unit,'')
				)
			END,'') AS inventory_unit
		FROM %s.order_items oi
		JOIN %s.orders o ON o.id=oi.order_id
		JOIN %s.products p ON p.id=oi.product_id
		LEFT JOIN %s.products parent_product ON parent_product.id=p.parent_product_id
		LEFT JOIN %s.product_unit_templates parent_product_unit_template ON parent_product_unit_template.id=parent_product.unit_template_id
		LEFT JOIN %s.product_config_templates parent_product_config ON parent_product_config.id=parent_product.product_config_template_id
		LEFT JOIN %s.product_categories parent_product_category ON parent_product_category.id=parent_product.product_category_id
		LEFT JOIN %s.product_categories parent_product_parent_category ON parent_product_parent_category.id=parent_product_category.parent_id
		LEFT JOIN %s.product_unit_templates product_unit_template ON product_unit_template.id=p.unit_template_id
		LEFT JOIN %s.product_config_templates product_config ON product_config.id=p.product_config_template_id
		LEFT JOIN %s.product_categories subtype_pc ON subtype_pc.id=COALESCE(p.product_category_id,0)
		LEFT JOIN %s.product_categories type_pc ON type_pc.id=COALESCE(subtype_pc.parent_id,0)
		LEFT JOIN %s.customers rule_customer ON rule_customer.id=o.customer_id AND rule_customer.active=true
		LEFT JOIN %s.customer_product_rule_template_items cpti
		  ON cpti.active=true
		 AND cpti.template_id=COALESCE(rule_customer.customer_product_rule_template_id,0)
		 AND cpti.product_subtype_category_id=COALESCE(subtype_pc.id,0)
		LEFT JOIN %s.customer_product_rule_overrides cpro
		  ON cpro.active=true
		 AND cpro.customer_id=o.customer_id
		 AND cpro.product_subtype_category_id=COALESCE(subtype_pc.id,0)
		LEFT JOIN %s.order_stock_decisions osd ON osd.order_id=o.id
		%s
		ORDER BY oi.product_id,o.order_no,oi.id
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, where)
	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bySnapshot := map[string]*productionDemand{}
	order := make([]string, 0)
	for rows.Next() {
		var (
			productID, parentProductID                                           int64
			product, productionKind, typeName, subtypeName, orderNo              string
			typeID, subtypeID, operationTemplateID                               int64
			qty                                                                  float64
			forceProduce                                                         bool
			priceSourceJSON, salesUnit, specLabel, netContentUnit, inventoryUnit string
			netContentQty                                                        float64
		)
		if err := rows.Scan(
			&productID, &parentProductID, &product, &productionKind,
			&typeID, &subtypeID, &typeName, &subtypeName, &operationTemplateID,
			&orderNo, &qty, &forceProduce, &priceSourceJSON, &salesUnit, &specLabel,
			&netContentQty, &netContentUnit, &inventoryUnit,
		); err != nil {
			return nil, err
		}
		snapshot, snapshotJSON, err := resolveProductionQuantitySnapshot(
			productID, parentProductID, priceSourceJSON, salesUnit, specLabel,
			netContentQty, netContentUnit, inventoryUnit,
		)
		if err != nil {
			blockingSalesUnit, blockingInventoryUnit := productionQuantitySnapshotUnits(
				priceSourceJSON,
				salesUnit,
				specLabel,
				inventoryUnit,
			)
			blockingReason := productionQuantitySnapshotBlockingReason(blockingSalesUnit, blockingInventoryUnit, err)
			groupKey := strings.Join([]string{
				"blocked",
				strconv.FormatInt(productID, 10),
				strconv.FormatInt(parentProductID, 10),
				strings.TrimSpace(orderNo),
				strings.TrimSpace(specLabel),
				strings.TrimSpace(salesUnit),
				blockingReason,
			}, "\x1f")
			demand := bySnapshot[groupKey]
			if demand == nil {
				demand = &productionDemand{
					UnprodNeedRow: UnprodNeedRow{
						ProductID: productID, ParentProductID: parentProductID, Product: product,
						ProductionKind: productionKind, ProductTypeCategoryID: typeID,
						ProductSubtypeCategoryID: subtypeID, ProductTypeName: typeName,
						ProductSubtypeName: subtypeName, OperationTemplateID: operationTemplateID,
						SpecLabel: firstNonEmpty(specLabel, blockingSalesUnit), SalesUnit: blockingSalesUnit,
						InventoryUnit:  blockingInventoryUnit,
						BlockingReason: blockingReason,
					},
					orderNos: map[string]bool{},
				}
				bySnapshot[groupKey] = demand
				order = append(order, groupKey)
			}
			demand.SalesSpecCount += qty
			if forceProduce {
				demand.forceSalesSpecCount += qty
			}
			if strings.TrimSpace(orderNo) != "" {
				demand.orderNos[strings.TrimSpace(orderNo)] = true
			}
			continue
		}
		groupKey := productionQuantitySnapshotGroupKey(snapshot)
		demand := bySnapshot[groupKey]
		if demand == nil {
			demand = &productionDemand{
				UnprodNeedRow: UnprodNeedRow{
					ProductID: productID, ParentProductID: snapshot.ParentProductID, Product: product,
					ProductionKind: productionKind, ProductTypeCategoryID: typeID,
					ProductSubtypeCategoryID: subtypeID, ProductTypeName: typeName,
					ProductSubtypeName: subtypeName, OperationTemplateID: operationTemplateID,
					SpecLabel: snapshot.SpecLabel, SalesUnit: snapshot.SalesUnit,
					InventoryQtyPerSalesUnit: snapshot.InventoryQtyPerSalesUnit,
					InventoryUnit:            snapshot.InventoryUnit, SalesSpecSnapshotJSON: snapshotJSON,
				},
				orderNos: map[string]bool{},
			}
			demand.SpecG = productiondomain.InventoryQuantityToLegacyGrams(snapshot.InventoryQtyPerSalesUnit, snapshot.InventoryUnit)
			bySnapshot[groupKey] = demand
			order = append(order, groupKey)
		}
		demand.SalesSpecCount += qty
		if forceProduce {
			demand.forceSalesSpecCount += qty
		}
		if strings.TrimSpace(orderNo) != "" {
			demand.orderNos[strings.TrimSpace(orderNo)] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return productionDemandMapRows(bySnapshot, order), nil
}

func fetchCustomerProcessingProductionDemands(ctx context.Context, pool productionDemandQueryer, schema, where string, args []any) ([]productionDemand, error) {
	q := fmt.Sprintf(`
		SELECT
			d.product_id,
			CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END,
			COALESCE(NULLIF(d.product_name,''),p.name,''),
			COALESCE(NULLIF(p.product_kind,''),'roasted_bean'),
			COALESCE(type_pc.id,0),
			COALESCE(subtype_pc.id,0),
			COALESCE(type_pc.name,''),
			COALESCE(subtype_pc.name,''),
			COALESCE(
				NULLIF(cpro.operation_template_id,0),
				NULLIF(cpti.operation_template_id,0),
				NULLIF(p.operation_template_id_override,0),
				NULLIF(subtype_pc.operation_template_id,0),
				type_pc.operation_template_id,
				0
			),
			COALESCE(d.request_no,''),
			COALESCE(d.spec_g,0),
			COALESCE(d.target_qty,0)::float8,
			COALESCE(d.customer_id,0),
			COALESCE(d.target_warehouse,''),
			COALESCE(NULLIF(to_jsonb(d)->>'request_item_id','')::bigint,0)
		FROM %s.customer_processing_production_demands d
		JOIN %s.products p ON p.id=d.product_id
		LEFT JOIN %s.product_categories subtype_pc ON subtype_pc.id=COALESCE(p.product_category_id,0)
		LEFT JOIN %s.product_categories type_pc ON type_pc.id=COALESCE(subtype_pc.parent_id,0)
		LEFT JOIN %s.customers rule_customer ON rule_customer.id=d.customer_id AND rule_customer.active=true
		LEFT JOIN %s.customer_product_rule_template_items cpti
		  ON cpti.active=true
		 AND cpti.template_id=COALESCE(rule_customer.customer_product_rule_template_id,0)
		 AND cpti.product_subtype_category_id=COALESCE(subtype_pc.id,0)
		LEFT JOIN %s.customer_product_rule_overrides cpro
		  ON cpro.active=true
		 AND cpro.customer_id=d.customer_id
		 AND cpro.product_subtype_category_id=COALESCE(subtype_pc.id,0)
		WHERE %s
		ORDER BY d.product_id,d.request_no,d.id
	`, schema, schema, schema, schema, schema, schema, schema, where)
	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bySnapshot := map[string]*productionDemand{}
	order := make([]string, 0)
	for rows.Next() {
		var (
			productID, parentProductID, typeID, subtypeID, operationTemplateID, specG int64
			customerID, requestItemID                                                 int64
			product, productionKind, typeName, subtypeName, requestNo                 string
			targetWarehouse                                                           string
			qty                                                                       float64
		)
		if err := rows.Scan(
			&productID, &parentProductID, &product, &productionKind, &typeID, &subtypeID,
			&typeName, &subtypeName, &operationTemplateID, &requestNo, &specG, &qty,
			&customerID, &targetWarehouse, &requestItemID,
		); err != nil {
			return nil, err
		}
		if specG <= 0 || qty <= 0 {
			continue
		}
		snapshot := productionQuantitySnapshot{
			SKUID: productID, ParentProductID: parentProductID,
			SpecLabel: fmt.Sprintf("%dg", specG), SalesUnit: fmt.Sprintf("%dg", specG),
			InventoryUnit: "kg", InventoryQtyPerSalesUnit: float64(specG) / 1000,
			ConversionSource: "customer_processing_spec_g",
			CustomerID:       customerID, TargetWarehouse: strings.TrimSpace(targetWarehouse),
			ProcessingRequestItemID: requestItemID,
		}
		groupKey := productionQuantitySnapshotGroupKey(snapshot)
		demand := bySnapshot[groupKey]
		if demand == nil {
			raw, _ := json.Marshal(snapshot)
			demand = &productionDemand{
				UnprodNeedRow: UnprodNeedRow{
					ProductID: productID, ParentProductID: parentProductID, Product: product,
					ProductionKind: productionKind, ProductTypeCategoryID: typeID,
					ProductSubtypeCategoryID: subtypeID, ProductTypeName: typeName,
					ProductSubtypeName: subtypeName, OperationTemplateID: operationTemplateID,
					SpecLabel: snapshot.SpecLabel, SalesUnit: snapshot.SalesUnit, SpecG: specG,
					InventoryQtyPerSalesUnit: snapshot.InventoryQtyPerSalesUnit,
					InventoryUnit:            snapshot.InventoryUnit, SalesSpecSnapshotJSON: string(raw),
				},
				orderNos: map[string]bool{},
			}
			bySnapshot[groupKey] = demand
			order = append(order, groupKey)
		}
		demand.SalesSpecCount += qty
		demand.forceSalesSpecCount += qty
		if strings.TrimSpace(requestNo) != "" {
			demand.orderNos[strings.TrimSpace(requestNo)] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return productionDemandMapRows(bySnapshot, order), nil
}

func productionDemandMapRows(bySnapshot map[string]*productionDemand, order []string) []productionDemand {
	out := make([]productionDemand, 0, len(order))
	for _, groupKey := range order {
		demand := bySnapshot[groupKey]
		refs := make([]string, 0, len(demand.orderNos))
		for ref := range demand.orderNos {
			refs = append(refs, ref)
		}
		sort.Strings(refs)
		demand.OrderNos = strings.Join(refs, ",")
		out = append(out, *demand)
	}
	return out
}

func productionQuantitySnapshotGroupKey(snapshot productionQuantitySnapshot) string {
	return strings.Join([]string{
		strconv.FormatInt(snapshot.SKUID, 10),
		strconv.FormatInt(snapshot.ParentProductID, 10),
		strings.TrimSpace(snapshot.SpecLabel),
		strings.TrimSpace(snapshot.SalesUnit),
		normalizeProductionQuantityUnit(snapshot.InventoryUnit),
		strconv.FormatFloat(snapshot.InventoryQtyPerSalesUnit, 'g', -1, 64),
		strings.TrimSpace(snapshot.ConversionSource),
		strconv.FormatInt(snapshot.CustomerID, 10),
		strings.TrimSpace(snapshot.TargetWarehouse),
		strconv.FormatInt(snapshot.ProcessingRequestItemID, 10),
	}, "\x1f")
}

func resolveProductionQuantitySnapshot(
	productID, parentProductID int64,
	priceSourceJSON, orderSalesUnit, catalogSpecLabel string,
	catalogNetContentQty float64, catalogNetContentUnit, catalogInventoryUnit string,
) (productionQuantitySnapshot, string, error) {
	var source map[string]any
	if strings.TrimSpace(priceSourceJSON) != "" {
		if err := json.Unmarshal([]byte(priceSourceJSON), &source); err != nil {
			return productionQuantitySnapshot{}, "", fmt.Errorf("price source snapshot invalid")
		}
	}
	if raw, exists := source["production_quantity_snapshot"]; exists {
		m, ok := raw.(map[string]any)
		if !ok {
			return productionQuantitySnapshot{}, "", fmt.Errorf("production quantity snapshot invalid")
		}
		snapshot := productionQuantitySnapshot{
			SKUID: jsonInt64(m["sku_id"]), ParentProductID: jsonInt64(m["parent_product_id"]),
			SpecLabel: jsonString(m["spec_label"]), SalesUnit: jsonString(m["sales_unit"]),
			InventoryUnit:            jsonString(m["inventory_unit"]),
			InventoryQtyPerSalesUnit: jsonFloat64(m["inventory_qty_per_sales_unit"]),
			ConversionSource:         jsonString(m["conversion_source"]),
		}
		if snapshot.SKUID != productID || snapshot.ParentProductID <= 0 ||
			snapshot.InventoryQtyPerSalesUnit <= 0 || strings.TrimSpace(snapshot.InventoryUnit) == "" {
			return productionQuantitySnapshot{}, "", fmt.Errorf("production quantity snapshot does not match concrete SKU, has no frozen parent product, or has no valid inventory conversion")
		}
		snapshot.InventoryUnit = normalizeProductionQuantityUnit(snapshot.InventoryUnit)
		snapshot.SpecLabel = firstNonEmpty(snapshot.SpecLabel, catalogSpecLabel)
		snapshot.SalesUnit = firstNonEmpty(snapshot.SalesUnit, orderSalesUnit, snapshot.SpecLabel)
		rawJSON, _ := json.Marshal(snapshot)
		return snapshot, string(rawJSON), nil
	}

	effective, _ := source["effective_sales_spec"].(map[string]any)
	inventoryUnit := firstNonEmpty(jsonString(effective["inventory_unit"]), jsonString(source["inventory_unit"]), catalogInventoryUnit)
	salesUnit := firstNonEmpty(jsonString(effective["sales_unit"]), orderSalesUnit, catalogSpecLabel)
	specLabel := firstNonEmpty(jsonString(effective["spec_label"]), jsonString(effective["spec_name"]), catalogSpecLabel)
	factor := productionInventoryConversionFactor(source, effective, salesUnit, inventoryUnit)
	conversionSource := "legacy_price_snapshot"
	if factor <= 0 {
		netQty := jsonFloat64(effective["net_content_qty"])
		netUnit := jsonString(effective["net_content_unit"])
		if netQty > 0 {
			factor = convertProductionQuantity(netQty, netUnit, inventoryUnit)
		}
	}
	if factor <= 0 {
		factor = convertProductionQuantity(catalogNetContentQty, catalogNetContentUnit, inventoryUnit)
		conversionSource = "catalog_sku_net_content"
	}
	if factor <= 0 || strings.TrimSpace(inventoryUnit) == "" {
		return productionQuantitySnapshot{}, "", fmt.Errorf("concrete SKU has no valid inventory unit conversion")
	}
	snapshot := productionQuantitySnapshot{
		SKUID: productID, ParentProductID: parentProductID, SpecLabel: specLabel,
		SalesUnit: salesUnit, InventoryUnit: normalizeProductionQuantityUnit(inventoryUnit),
		InventoryQtyPerSalesUnit: factor, ConversionSource: conversionSource,
	}
	rawJSON, _ := json.Marshal(snapshot)
	return snapshot, string(rawJSON), nil
}

func productionQuantitySnapshotUnits(priceSourceJSON, orderSalesUnit, catalogSpecLabel, catalogInventoryUnit string) (string, string) {
	var source map[string]any
	if strings.TrimSpace(priceSourceJSON) != "" {
		_ = json.Unmarshal([]byte(priceSourceJSON), &source)
	}
	frozen, _ := source["production_quantity_snapshot"].(map[string]any)
	effective, _ := source["effective_sales_spec"].(map[string]any)
	salesUnit := firstNonEmpty(
		jsonString(frozen["sales_unit"]),
		jsonString(effective["sales_unit"]),
		orderSalesUnit,
		catalogSpecLabel,
	)
	inventoryUnit := firstNonEmpty(
		jsonString(frozen["inventory_unit"]),
		jsonString(effective["inventory_unit"]),
		jsonString(source["inventory_unit"]),
		catalogInventoryUnit,
	)
	return salesUnit, normalizeProductionQuantityUnit(inventoryUnit)
}

func productionQuantitySnapshotBlockingReason(salesUnit, inventoryUnit string, err error) string {
	message := ""
	if err != nil {
		message = err.Error()
	}
	switch {
	case strings.Contains(message, "no valid inventory unit conversion"):
		return fmt.Sprintf(
			"销售单位“%s”无法换算到库存单位“%s”，请在商品档案配置该销售规格的库存换算",
			firstNonEmpty(strings.TrimSpace(salesUnit), "(未设置)"),
			firstNonEmpty(strings.TrimSpace(inventoryUnit), "(未设置)"),
		)
	case strings.Contains(message, "price source snapshot invalid"):
		return "订单价格来源快照格式错误，请检查订单商品规格后重新保存"
	case strings.Contains(message, "production quantity snapshot invalid"):
		return "订单生产数量快照格式错误，请检查订单商品规格后重新保存"
	case strings.Contains(message, "production quantity snapshot does not match"):
		return "订单冻结的生产数量换算与具体 SKU 不一致或不完整，请检查订单商品规格"
	default:
		return "订单生产数量换算不可用，请检查订单商品规格与库存单位换算"
	}
}

func productionInventoryConversionFactor(source, effective map[string]any, salesUnit, inventoryUnit string) float64 {
	for _, owner := range []map[string]any{effective, source} {
		raw, _ := owner["inventory_conversion_json"].(map[string]any)
		if len(raw) == 0 {
			continue
		}
		if nested := productionConversionMap(raw, salesUnit); nested != nil {
			if factor := productionConversionFactor(nested, inventoryUnit); factor > 0 {
				return factor
			}
		}
		if factor := productionConversionFactor(raw, inventoryUnit); factor > 0 {
			return factor
		}
	}
	return 0
}

func productionConversionMap(values map[string]any, unit string) map[string]any {
	for key, value := range values {
		if !sameProductionUnit(key, unit) {
			continue
		}
		nested, _ := value.(map[string]any)
		return nested
	}
	return nil
}

func productionConversionFactor(values map[string]any, unit string) float64 {
	for key, value := range values {
		if sameProductionUnit(key, unit) {
			return jsonFloat64(value)
		}
	}
	return 0
}

func convertProductionQuantity(qty float64, fromUnit, toUnit string) float64 {
	if qty <= 0 {
		return 0
	}
	from := normalizeProductionQuantityUnit(fromUnit)
	to := normalizeProductionQuantityUnit(toUnit)
	if from == "" || to == "" {
		return 0
	}
	if from == to {
		return qty
	}
	toGrams := map[string]float64{"g": 1, "kg": 1000, "lb": 453.59237, "oz": 28.349523125}
	fromFactor, fromOK := toGrams[from]
	toFactor, toOK := toGrams[to]
	if !fromOK || !toOK {
		return 0
	}
	return qty * fromFactor / toFactor
}

func normalizeProductionQuantityUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "gram", "grams", "克":
		return "g"
	case "kg", "kilogram", "kilograms", "千克", "公斤":
		return "kg"
	case "lb", "lbs", "pound", "pounds", "磅":
		return "lb"
	case "oz", "ounce", "ounces", "盎司":
		return "oz"
	default:
		return strings.ToLower(strings.TrimSpace(unit))
	}
}

func sameProductionUnit(left, right string) bool {
	return normalizeProductionQuantityUnit(left) == normalizeProductionQuantityUnit(right)
}

func sameProductionQuantity(left, right float64) bool {
	return math.Abs(left-right) <= 0.000000001
}

func jsonString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func jsonFloat64(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case json.Number:
		out, _ := typed.Float64()
		return out
	case string:
		var out float64
		_, _ = fmt.Sscan(strings.TrimSpace(typed), &out)
		return out
	default:
		return 0
	}
}

func jsonInt64(value any) int64 {
	return int64(math.Round(jsonFloat64(value)))
}

type finishedInventoryAvailability struct {
	units    int64
	looseG   int64
	reserved int64
}

func finalizeUnproducedNeeds(ctx context.Context, pool productionDemandQueryer, schema string, demands []productionDemand) ([]UnprodNeedRow, error) {
	if len(demands) == 0 {
		return []UnprodNeedRow{}, nil
	}
	productIDs := make([]int64, 0, len(demands))
	productSeen := map[int64]bool{}
	for _, demand := range demands {
		if !productSeen[demand.ProductID] {
			productSeen[demand.ProductID] = true
			productIDs = append(productIDs, demand.ProductID)
		}
	}
	availability := map[string]*finishedInventoryAvailability{}
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT product_id,spec_g,COALESCE(onhand_units,0),COALESCE(onhand_loose_g,0)
		FROM %s.finished_inventory
		WHERE product_id=ANY($1) AND warehouse='finished_goods'
	`, schema), productIDs)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var productID, specG, units, looseG int64
		if err := rows.Scan(&productID, &specG, &units, &looseG); err != nil {
			rows.Close()
			return nil, err
		}
		availability[fmt.Sprintf("%d-%d", productID, specG)] = &finishedInventoryAvailability{units: units, looseG: looseG}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = pool.Query(ctx, fmt.Sprintf(`
		SELECT a.product_id,a.spec_g,COALESCE(SUM(a.allocated_g),0)::bigint
		FROM %s.order_stock_batch_allocations a
		WHERE a.product_id=ANY($1)
		  AND NOT EXISTS (
			SELECT 1 FROM %s.order_stock_deductions d
			WHERE d.order_id=a.order_id
			  AND d.product_id=a.product_id
			  AND d.spec_g=a.spec_g
			  AND d.batch_code=a.batch_code
		  )
		GROUP BY a.product_id,a.spec_g
	`, schema, schema), productIDs)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var productID, specG, reserved int64
		if err := rows.Scan(&productID, &specG, &reserved); err != nil {
			rows.Close()
			return nil, err
		}
		key := fmt.Sprintf("%d-%d", productID, specG)
		current := availability[key]
		if current == nil {
			current = &finishedInventoryAvailability{}
			availability[key] = current
		}
		current.reserved = reserved
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	orderedDemands := append([]productionDemand(nil), demands...)
	sort.SliceStable(orderedDemands, func(i, j int) bool {
		left, right := orderedDemands[i], orderedDemands[j]
		if left.ProductID != right.ProductID {
			return left.ProductID < right.ProductID
		}
		if left.SpecG != right.SpecG {
			return left.SpecG < right.SpecG
		}
		if left.OrderNos != right.OrderNos {
			return left.OrderNos < right.OrderNos
		}
		return productionQuantitySnapshotGroupKey(productionQuantitySnapshotFromDemand(left)) <
			productionQuantitySnapshotGroupKey(productionQuantitySnapshotFromDemand(right))
	})
	remainingAvailableG := map[string]int64{}
	remainingAvailableUnits := map[string]int64{}

	out := make([]UnprodNeedRow, 0, len(orderedDemands))
	for _, demand := range orderedDemands {
		row := demand.UnprodNeedRow
		if strings.TrimSpace(row.BlockingReason) != "" {
			row.NeedUnits = int64(math.Ceil(demand.SalesSpecCount))
			row.DemandSelectable = false
			out = append(out, row)
			continue
		}
		availabilityKey := fmt.Sprintf("%d-%d", row.ProductID, row.SpecG)
		current := availability[availabilityKey]
		if current == nil {
			current = &finishedInventoryAvailability{}
		}
		row.NeedUnits = int64(math.Ceil(demand.SalesSpecCount))
		row.NeedInventoryQty = productiondomain.SalesSpecCountToInventoryQuantity(demand.SalesSpecCount, row.InventoryQtyPerSalesUnit)
		row.NeedG = productiondomain.InventoryQuantityToLegacyGrams(row.NeedInventoryQty, row.InventoryUnit)
		row.InvUnits = current.units
		row.InvLooseG = current.looseG
		row.InvG = current.units*row.SpecG + current.looseG
		if _, exists := remainingAvailableG[availabilityKey]; !exists {
			remainingAvailableG[availabilityKey] = maxInt64(0, row.InvG-current.reserved)
			remainingAvailableUnits[availabilityKey] = maxInt64(0, current.units)
		}
		row.AvailableG = remainingAvailableG[availabilityKey]
		availableSalesSpecCount := float64(remainingAvailableUnits[availabilityKey])
		if row.SpecG > 0 {
			availableSalesSpecCount = float64(row.AvailableG) / float64(row.SpecG)
		}
		row.AvailableInventoryQty = productiondomain.SalesSpecCountToInventoryQuantity(availableSalesSpecCount, row.InventoryQtyPerSalesUnit)
		normalDemandQty := productiondomain.SalesSpecCountToInventoryQuantity(
			math.Max(0, demand.SalesSpecCount-demand.forceSalesSpecCount),
			row.InventoryQtyPerSalesUnit,
		)
		forcedDemandQty := productiondomain.SalesSpecCountToInventoryQuantity(demand.forceSalesSpecCount, row.InventoryQtyPerSalesUnit)
		row.GapInventoryQty = forcedDemandQty + math.Max(0, normalDemandQty-row.AvailableInventoryQty)
		normalDemandG := productiondomain.InventoryQuantityToLegacyGrams(normalDemandQty, row.InventoryUnit)
		remainingAvailableG[availabilityKey] = maxInt64(0, row.AvailableG-minInt64(row.AvailableG, normalDemandG))
		if row.SpecG <= 0 {
			normalDemandSalesSpecCount := math.Max(0, demand.SalesSpecCount-demand.forceSalesSpecCount)
			remainingAvailableUnits[availabilityKey] = maxInt64(
				0,
				remainingAvailableUnits[availabilityKey]-minInt64(
					remainingAvailableUnits[availabilityKey],
					int64(math.Ceil(normalDemandSalesSpecCount)),
				),
			)
		}
		if row.InventoryQtyPerSalesUnit > 0 {
			row.GapSalesSpecCount = row.GapInventoryQty / row.InventoryQtyPerSalesUnit
		}
		row.GapG = productiondomain.InventoryQuantityToLegacyGrams(row.GapInventoryQty, row.InventoryUnit)
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].GapInventoryQty == out[j].GapInventoryQty {
			if out[i].Product == out[j].Product {
				return out[i].SpecG < out[j].SpecG
			}
			return out[i].Product < out[j].Product
		}
		return out[i].GapInventoryQty > out[j].GapInventoryQty
	})
	return out, nil
}

func productionQuantitySnapshotFromDemand(demand productionDemand) productionQuantitySnapshot {
	return productionQuantitySnapshot{
		SKUID: demand.ProductID, ParentProductID: demand.ParentProductID,
		SpecLabel: demand.SpecLabel, SalesUnit: demand.SalesUnit,
		InventoryUnit: demand.InventoryUnit, InventoryQtyPerSalesUnit: demand.InventoryQtyPerSalesUnit,
	}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
