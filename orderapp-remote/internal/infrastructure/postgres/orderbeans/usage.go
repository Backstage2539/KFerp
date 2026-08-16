package orderbeans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	ListTypeCommercial = "commercial"
	ListTypeRetail     = "retail"
	ListTypeGreen      = "green"
	ListTypeDrip       = "drip"
)

type Usage struct {
	PublicationID int64
	VersionNo     string
}

type PublishedPricing struct {
	UnitPrice               float64
	PriceUnit               string
	UnitG                   float64
	SourcePriceRecordID     int64
	InventoryUnit           string
	InventoryConversionJSON string
	TierLabel               string
	FinalUnitPrice          float64
	PricingRuleVersion      string
	ManualAdjusted          bool
	CostSourceSnapshotJSON  string
	CustomerSnapshotJSON    string
	QuantityBasis           string
	TierQuantityUnit        string
	EffectiveSalesSpecJSON  string
}

// PublishedProductSpec is the immutable SKU identity and sales specification
// frozen into a published price list. ConcretePublication is false for legacy
// publications that predate per-SKU sales-spec snapshots.
type PublishedProductSpec struct {
	ConcretePublication bool
	ProductFound        bool
	// CurrentCatalogAuthority marks a spec resolved from the active concrete
	// SKU at order-write time. It is used when no concrete published sales-spec
	// snapshot is authoritative, including legacy publications and non-price-list
	// order sources.
	CurrentCatalogAuthority bool
	SKUID                   int64
	ParentProductID         int64
	SpecKey                 string
	SpecName                string
	SpecLabel               string
	SalesUnit               string
	NetContentQty           float64
	NetContentUnit          string
	ProductKind             string
	UnitBagCount            int64
	UnitBeanG               float64
	QuantityBasis           string
	InventoryUnit           string
	InventoryConversionJSON string
	EffectiveSalesSpecJSON  string
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func ListTypeForRetail(retail bool) string {
	if retail {
		return ListTypeRetail
	}
	return ListTypeCommercial
}

func ListTypeForProductKind(productKind string, retail bool) string {
	switch strings.TrimSpace(productKind) {
	case "green_bean":
		return ListTypeGreen
	case "drip_bag":
		return ListTypeDrip
	}
	return ListTypeForRetail(retail)
}

func ResolvePublishedUnitPrice(ctx context.Context, q rowQuerier, schema string, customerID int64, productID int64, listType string, specG int64, qty int64) (float64, error) {
	return ResolvePublishedUnitPriceForPublication(ctx, q, schema, customerID, productID, listType, 0, specG, qty)
}

func ResolvePublishedUnitPriceForPublication(ctx context.Context, q rowQuerier, schema string, customerID int64, productID int64, listType string, requestedPublicationID int64, specG int64, qty int64) (float64, error) {
	return ResolvePublishedUnitPriceForPublicationWithUnit(ctx, q, schema, customerID, productID, listType, requestedPublicationID, specG, qty, "", 0)
}

func ResolvePublishedUnitPriceForPublicationWithUnit(ctx context.Context, q rowQuerier, schema string, customerID int64, productID int64, listType string, requestedPublicationID int64, specG int64, qty int64, salesUnit string, unitBagCount int64) (float64, error) {
	pricing, err := ResolvePublishedPricingForPublicationWithUnit(ctx, q, schema, customerID, productID, listType, requestedPublicationID, specG, qty, salesUnit, unitBagCount)
	return pricing.UnitPrice, err
}

func ResolvePublishedPricingForPublicationWithUnit(ctx context.Context, q rowQuerier, schema string, customerID int64, productID int64, listType string, requestedPublicationID int64, specG int64, qty int64, salesUnit string, unitBagCount int64) (PublishedPricing, error) {
	usage, err := ResolveUsageForPublication(ctx, q, schema, customerID, productID, listType, requestedPublicationID)
	if err != nil || usage.PublicationID <= 0 {
		return PublishedPricing{}, err
	}
	var raw []byte
	sql := fmt.Sprintf(`SELECT COALESCE(content_json, '{}'::jsonb) FROM %s.bean_list_publications WHERE id=$1`, schema)
	if err := q.QueryRow(ctx, sql, usage.PublicationID).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isMissingBeanListSchema(err) {
			return PublishedPricing{}, nil
		}
		return PublishedPricing{}, err
	}
	price, ok := publishedPricingFromContentForListType(raw, productID, listType, specG, qty, salesUnit, unitBagCount)
	if !ok {
		return PublishedPricing{}, nil
	}
	return price, nil
}

// ResolvePublishedPricingForPublicationWithBOMSpec resolves a cutover product
// price by its immutable BOM specification identity. Quantity is interpreted
// directly in the specification inventory unit; no legacy spec_g conversion is
// applied.
func ResolvePublishedPricingForPublicationWithBOMSpec(ctx context.Context, q rowQuerier, schema string, customerID, productID int64, listType string, requestedPublicationID, bomSpecID, bomVariantID, qty int64) (PublishedPricing, error) {
	if bomSpecID <= 0 || bomVariantID <= 0 || qty <= 0 {
		return PublishedPricing{}, nil
	}
	usage, err := ResolveUsageForPublication(ctx, q, schema, customerID, productID, listType, requestedPublicationID)
	if err != nil || usage.PublicationID <= 0 {
		return PublishedPricing{}, err
	}
	var raw []byte
	if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(content_json, '{}'::jsonb) FROM %s.bean_list_publications WHERE id=$1`, schema), usage.PublicationID).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isMissingBeanListSchema(err) {
			return PublishedPricing{}, nil
		}
		return PublishedPricing{}, err
	}
	pricing, ok := publishedPricingFromContentForBOMSpec(raw, productID, bomSpecID, bomVariantID, qty, listType)
	if !ok {
		return PublishedPricing{}, nil
	}
	return pricing, nil
}

// ResolvePublishedProductSpecForPublication resolves the effective price-list
// publication and inspects its frozen concrete-SKU identity. It intentionally
// keeps legacy publications compatible by returning ConcretePublication=false.
func ResolvePublishedProductSpecForPublication(ctx context.Context, q rowQuerier, schema string, customerID int64, productID int64, listType string, requestedPublicationID int64) (Usage, PublishedProductSpec, error) {
	usage, err := ResolveUsageForPublication(ctx, q, schema, customerID, productID, listType, requestedPublicationID)
	if err != nil || usage.PublicationID <= 0 {
		return usage, PublishedProductSpec{}, err
	}
	var raw []byte
	sql := fmt.Sprintf(`SELECT COALESCE(content_json, '{}'::jsonb) FROM %s.bean_list_publications WHERE id=$1`, schema)
	if err := q.QueryRow(ctx, sql, usage.PublicationID).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isMissingBeanListSchema(err) {
			return Usage{}, PublishedProductSpec{}, nil
		}
		return Usage{}, PublishedProductSpec{}, err
	}
	spec, err := inspectPublishedProductSpecContent(raw, productID)
	return usage, spec, err
}

func ResolveUsage(ctx context.Context, q rowQuerier, schema string, customerID int64, productID int64, listType string) (Usage, error) {
	return ResolveUsageForPublication(ctx, q, schema, customerID, productID, listType, 0)
}

func ResolveUsageForPublication(ctx context.Context, q rowQuerier, schema string, customerID int64, productID int64, listType string, requestedPublicationID int64) (Usage, error) {
	if q == nil || strings.TrimSpace(schema) == "" || productID <= 0 {
		return Usage{}, nil
	}
	listType = strings.TrimSpace(listType)
	if listType == "" {
		listType = ListTypeCommercial
	}
	customerKey := ""
	if customerID > 0 {
		customerKey = fmt.Sprintf("%d", customerID)
	}

	var usage Usage
	if requestedPublicationID > 0 {
		sql := fmt.Sprintf(`
			SELECT id, COALESCE(version_no,'')
			FROM %s.bean_list_publications blp
			WHERE blp.id=$1
			  AND blp.status='published'
			  AND blp.list_type=$2
			  AND blp.publication_purpose='factory_supply'
			  AND ((blp.owner_type='customer' AND blp.owner_key=$3) OR blp.owner_type='official')
		`, schema)
		if err := q.QueryRow(ctx, sql, requestedPublicationID, listType, customerKey).Scan(&usage.PublicationID, &usage.VersionNo); err != nil {
			if errors.Is(err, pgx.ErrNoRows) || isMissingBeanListSchema(err) {
				return Usage{}, nil
			}
			return Usage{}, err
		}
		return usage, nil
	}

	sql := fmt.Sprintf(`
		SELECT id, COALESCE(version_no,'')
		FROM %s.bean_list_publications blp
		WHERE blp.status='published'
		  AND blp.list_type=$1
		  AND blp.publication_purpose='factory_supply'
		  AND (
		    ($2 <> '' AND blp.owner_type='customer' AND blp.owner_key=$2)
		    OR blp.owner_type='official'
		  )
		  AND (
		    EXISTS (
		      SELECT 1
		      FROM jsonb_array_elements(COALESCE(blp.content_json->'groups', '[]'::jsonb)) AS groups(group_json)
		      CROSS JOIN LATERAL jsonb_array_elements(COALESCE(groups.group_json->'items', '[]'::jsonb)) AS items(item_json)
		      WHERE CASE
		        WHEN items.item_json->>'sku_id' ~ '^[0-9]+$' THEN (items.item_json->>'sku_id')::bigint
		        WHEN items.item_json->>'productId' ~ '^[0-9]+$' THEN (items.item_json->>'productId')::bigint
		        WHEN items.item_json->>'product_id' ~ '^[0-9]+$' THEN (items.item_json->>'product_id')::bigint
		        WHEN items.item_json->>'productID' ~ '^[0-9]+$' THEN (items.item_json->>'productID')::bigint
		        ELSE 0
		      END = $3
		    )
		    OR EXISTS (
		      SELECT 1
		      FROM jsonb_array_elements(COALESCE(blp.content_json->'price_rows', '[]'::jsonb)) AS price_rows(row_json)
		      WHERE CASE
		        WHEN row_json->>'sku_id' ~ '^[0-9]+$' THEN (row_json->>'sku_id')::bigint
		        WHEN row_json->>'product_id' ~ '^[0-9]+$' THEN (row_json->>'product_id')::bigint
		        WHEN row_json->>'productId' ~ '^[0-9]+$' THEN (row_json->>'productId')::bigint
		        WHEN row_json->>'productID' ~ '^[0-9]+$' THEN (row_json->>'productID')::bigint
		        ELSE 0
		      END = $3
		    )
		  )
		ORDER BY CASE WHEN $2 <> '' AND blp.owner_type='customer' AND blp.owner_key=$2 THEN 0 ELSE 1 END,
		         blp.published_at DESC,
		         blp.id DESC
		LIMIT 1
	`, schema)
	if err := q.QueryRow(ctx, sql, listType, customerKey, productID).Scan(&usage.PublicationID, &usage.VersionNo); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isMissingBeanListSchema(err) {
			return Usage{}, nil
		}
		return Usage{}, err
	}
	return usage, nil
}

type publishedBeanListContent struct {
	Groups []struct {
		Items []json.RawMessage `json:"items"`
	} `json:"groups"`
}

type publishedEffectiveSalesSpec struct {
	SKUID                   int64           `json:"sku_id"`
	ParentProductID         int64           `json:"parent_product_id"`
	SpecKey                 string          `json:"spec_key"`
	SpecName                string          `json:"spec_name"`
	SpecLabel               string          `json:"spec_label"`
	SalesUnit               string          `json:"sales_unit"`
	NetContentQty           float64         `json:"net_content_qty"`
	NetContentUnit          string          `json:"net_content_unit"`
	ProductKind             string          `json:"product_kind"`
	UnitBagCount            int64           `json:"unit_bag_count"`
	UnitBeanG               float64         `json:"unit_bean_g"`
	InventoryUnit           string          `json:"inventory_unit"`
	InventoryConversionJSON json.RawMessage `json:"inventory_conversion_json"`
}

func inspectPublishedProductSpecContent(raw []byte, productID int64) (PublishedProductSpec, error) {
	result := PublishedProductSpec{}
	if productID <= 0 || len(raw) == 0 {
		return result, nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return result, fmt.Errorf("解析价格表发布快照失败: %w", err)
	}
	var rows []json.RawMessage
	_ = json.Unmarshal(root["price_rows"], &rows)
	type inspectedRow struct {
		fields        map[string]json.RawMessage
		skuID         int64
		parentID      int64
		quantityBasis string
		frozen        publishedEffectiveSalesSpec
		frozenRaw     json.RawMessage
		concrete      bool
	}
	inspected := make([]inspectedRow, 0, len(rows))
	for _, rowRaw := range rows {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rowRaw, &fields); err != nil {
			continue
		}
		row := inspectedRow{
			fields:        fields,
			skuID:         publishedJSONInt64Field(fields, "sku_id", "skuId", "skuID"),
			parentID:      publishedJSONInt64Field(fields, "parent_product_id", "parentProductId", "parentProductID"),
			quantityBasis: publishedJSONStringField(fields, "quantity_basis"),
			frozenRaw:     fields["effective_sales_spec"],
		}
		if len(row.frozenRaw) > 0 && string(row.frozenRaw) != "null" {
			_ = json.Unmarshal(row.frozenRaw, &row.frozen)
			row.frozen, row.frozenRaw = enrichPublishedEffectiveSalesSpec(
				row.frozenRaw,
				row.parentID,
				publishedJSONStringField(fields, "inventory_unit"),
				fields["inventory_conversion_json"],
			)
		}
		// A concrete publication is identified by the full PR-541 marker. Rows
		// that only carried quantity_basis before the frozen SKU snapshot existed
		// remain on the historical compatibility path.
		row.concrete = strings.TrimSpace(row.quantityBasis) == "sales_spec_count" && row.skuID > 0 && row.parentID > 0 && row.frozen.SKUID > 0
		inspected = append(inspected, row)
	}
	hasConcreteRows := false
	for _, row := range inspected {
		if row.concrete {
			hasConcreteRows = true
		}
		if !row.concrete || row.skuID != productID {
			continue
		}
		if err := validatePublishedEffectiveSalesSpecInventoryAuthority(
			row.frozen,
			publishedJSONStringField(row.fields, "inventory_unit"),
			row.fields["inventory_conversion_json"],
		); err != nil {
			return PublishedProductSpec{}, err
		}
		if row.frozen.SKUID != row.skuID {
			return PublishedProductSpec{}, fmt.Errorf("价格表 SKU 快照身份不一致: 价格行 SKU=%d，有效销售规格 SKU=%d", row.skuID, row.frozen.SKUID)
		}
		if row.frozen.ParentProductID > 0 && row.frozen.ParentProductID != row.parentID {
			return PublishedProductSpec{}, fmt.Errorf("价格表父商品快照身份不一致: 价格行父商品=%d，有效销售规格父商品=%d", row.parentID, row.frozen.ParentProductID)
		}
		candidate := PublishedProductSpec{
			ConcretePublication:     true,
			ProductFound:            true,
			SKUID:                   row.skuID,
			ParentProductID:         row.parentID,
			SpecKey:                 strings.TrimSpace(row.frozen.SpecKey),
			SpecName:                strings.TrimSpace(row.frozen.SpecName),
			SpecLabel:               strings.TrimSpace(row.frozen.SpecLabel),
			SalesUnit:               strings.TrimSpace(row.frozen.SalesUnit),
			NetContentQty:           row.frozen.NetContentQty,
			NetContentUnit:          strings.TrimSpace(row.frozen.NetContentUnit),
			ProductKind:             firstPublishedString(strings.TrimSpace(row.frozen.ProductKind), publishedJSONStringField(row.fields, "product_kind")),
			UnitBagCount:            row.frozen.UnitBagCount,
			UnitBeanG:               row.frozen.UnitBeanG,
			QuantityBasis:           "sales_spec_count",
			InventoryUnit:           strings.TrimSpace(row.frozen.InventoryUnit),
			InventoryConversionJSON: compactPublishedJSON(row.frozen.InventoryConversionJSON),
			EffectiveSalesSpecJSON:  strings.TrimSpace(string(row.frozenRaw)),
		}
		if candidate.UnitBagCount <= 0 {
			candidate.UnitBagCount = publishedJSONInt64Field(row.fields, "unit_bag_count")
		}
		if candidate.UnitBeanG <= 0 {
			candidate.UnitBeanG = publishedJSONFloat64Field(row.fields, "unit_bean_g")
		}
		if result.ProductFound && !samePublishedProductSpec(result, candidate) {
			return PublishedProductSpec{}, fmt.Errorf("价格表 SKU %d 的有效销售规格快照不一致", row.skuID)
		}
		result = candidate
	}
	if result.ProductFound {
		return result, nil
	}

	legacyProductFound := false
	incompleteConcreteTarget := false
	for _, row := range inspected {
		matchesProduct := row.skuID == productID || (row.skuID <= 0 && publishedFlatPriceRowProductID(row.fields) == productID)
		if !matchesProduct {
			continue
		}
		if hasConcreteRows && strings.TrimSpace(row.quantityBasis) == "sales_spec_count" {
			// A count-based row inside a concrete publication must carry the full
			// frozen SKU marker. Do not silently downgrade a malformed concrete row
			// to the legacy path.
			incompleteConcreteTarget = true
			continue
		}
		legacyProductFound = true
	}
	// Legacy groups and incomplete flat rows are intentionally matched by the
	// historical product identity rules.
	var content publishedBeanListContent
	if err := json.Unmarshal(raw, &content); err == nil {
		for _, group := range content.Groups {
			for _, itemRaw := range group.Items {
				if publishedItemMatchesProduct(itemRaw, productID) {
					legacyProductFound = true
				}
			}
		}
	}
	if incompleteConcreteTarget {
		return PublishedProductSpec{ConcretePublication: true}, nil
	}
	if legacyProductFound {
		return PublishedProductSpec{ProductFound: true}, nil
	}
	if hasConcreteRows {
		return PublishedProductSpec{ConcretePublication: true}, nil
	}
	return result, nil
}

// The nested effective_sales_spec is the immutable per-SKU authority. The
// top-level price row intentionally freezes only the selected price unit, while
// the nested snapshot may carry the product's complete conversion graph. Treat
// a semantically equal top-level subset as compatible, but keep rejecting any
// duplicated edge whose factor really differs from the authority.
func validatePublishedEffectiveSalesSpecInventoryAuthority(frozen publishedEffectiveSalesSpec, inventoryUnit string, inventoryConversionJSON json.RawMessage) error {
	nestedUnit := strings.TrimSpace(frozen.InventoryUnit)
	topLevelUnit := strings.TrimSpace(inventoryUnit)
	if nestedUnit != "" && topLevelUnit != "" && !publishedUnitsEquivalent(nestedUnit, topLevelUnit) {
		return fmt.Errorf("价格表有效销售规格库存单位与价格行库存单位冲突: %s / %s", nestedUnit, topLevelUnit)
	}
	if publishedJSONHasContent(frozen.InventoryConversionJSON) &&
		publishedJSONHasContent(inventoryConversionJSON) {
		salesUnit := strings.TrimSpace(frozen.SalesUnit)
		inventoryUnit := nestedUnit
		if inventoryUnit == "" {
			inventoryUnit = topLevelUnit
		}
		if salesUnit == "" || inventoryUnit == "" {
			if compactPublishedJSON(frozen.InventoryConversionJSON) != compactPublishedJSON(inventoryConversionJSON) {
				return fmt.Errorf("价格表有效销售规格库存换算与价格行库存换算冲突")
			}
			return nil
		}
		authorityFactor, authorityOK := publishedInventoryAuthorityFactor(frozen.InventoryConversionJSON, salesUnit, inventoryUnit)
		rowFactor, rowOK := publishedInventoryAuthorityFactor(inventoryConversionJSON, salesUnit, inventoryUnit)
		if !authorityOK || !rowOK || authorityFactor != rowFactor {
			return fmt.Errorf("价格表有效销售规格库存换算与价格行库存换算冲突")
		}
	}
	return nil
}

func publishedInventoryAuthorityFactor(raw json.RawMessage, salesUnit string, inventoryUnit string) (float64, bool) {
	var graph map[string]any
	if json.Unmarshal(raw, &graph) != nil {
		return 0, false
	}
	factor := float64(0)
	found := false
	for sourceUnit, rawTargets := range graph {
		if !publishedUnitsEquivalent(sourceUnit, salesUnit) {
			continue
		}
		factors, valid := publishedInventoryTargetFactors(rawTargets, inventoryUnit)
		if !valid || len(factors) == 0 {
			return 0, false
		}
		for _, candidate := range factors {
			candidate = normalizePublishedInventoryFactor(candidate)
			if candidate <= 0 || (found && candidate != factor) {
				return 0, false
			}
			factor = candidate
			found = true
		}
	}
	return factor, found
}

func publishedInventoryTargetFactors(rawTargets any, inventoryUnit string) ([]float64, bool) {
	if direct, ok := rawTargets.(float64); ok {
		return []float64{direct}, direct > 0
	}
	targets, ok := rawTargets.(map[string]any)
	if !ok {
		return nil, false
	}
	factors := make([]float64, 0, 1)
	for targetUnit, rawFactor := range targets {
		if !publishedUnitsEquivalent(targetUnit, inventoryUnit) {
			continue
		}
		factor, ok := rawFactor.(float64)
		if !ok || factor <= 0 {
			return nil, false
		}
		factors = append(factors, factor)
	}
	return factors, len(factors) > 0
}

func samePublishedProductSpec(left, right PublishedProductSpec) bool {
	return left.SKUID == right.SKUID &&
		left.ParentProductID == right.ParentProductID &&
		left.SpecKey == right.SpecKey &&
		left.SpecName == right.SpecName &&
		left.SpecLabel == right.SpecLabel &&
		left.SalesUnit == right.SalesUnit &&
		left.NetContentQty == right.NetContentQty &&
		left.NetContentUnit == right.NetContentUnit &&
		left.ProductKind == right.ProductKind &&
		left.UnitBagCount == right.UnitBagCount &&
		left.UnitBeanG == right.UnitBeanG &&
		left.InventoryUnit == right.InventoryUnit &&
		left.InventoryConversionJSON == right.InventoryConversionJSON
}

func enrichPublishedEffectiveSalesSpec(raw json.RawMessage, parentProductID int64, inventoryUnit string, inventoryConversionJSON json.RawMessage) (publishedEffectiveSalesSpec, json.RawMessage) {
	var frozen publishedEffectiveSalesSpec
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return frozen, raw
	}
	if err := json.Unmarshal(raw, &frozen); err != nil {
		return frozen, raw
	}
	if frozen.ParentProductID <= 0 {
		frozen.ParentProductID = parentProductID
	}
	if strings.TrimSpace(frozen.InventoryUnit) == "" {
		frozen.InventoryUnit = strings.TrimSpace(inventoryUnit)
	}
	if !publishedJSONHasContent(frozen.InventoryConversionJSON) && publishedJSONHasContent(inventoryConversionJSON) {
		frozen.InventoryConversionJSON = append(json.RawMessage(nil), inventoryConversionJSON...)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return frozen, raw
	}
	if frozen.ParentProductID > 0 {
		fields["parent_product_id"] = frozen.ParentProductID
	}
	if strings.TrimSpace(frozen.InventoryUnit) != "" {
		fields["inventory_unit"] = strings.TrimSpace(frozen.InventoryUnit)
	}
	if publishedJSONHasContent(frozen.InventoryConversionJSON) {
		var conversion any
		if json.Unmarshal(frozen.InventoryConversionJSON, &conversion) == nil {
			fields["inventory_conversion_json"] = conversion
		}
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return frozen, raw
	}
	return frozen, encoded
}

func publishedJSONHasContent(raw json.RawMessage) bool {
	text := strings.TrimSpace(string(raw))
	return text != "" && text != "null" && text != "{}"
}

func compactPublishedJSON(raw json.RawMessage) string {
	if !publishedJSONHasContent(raw) {
		return ""
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return strings.TrimSpace(string(raw))
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(string(raw))
	}
	return string(encoded)
}

func firstPublishedString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func publishedJSONStringField(fields map[string]json.RawMessage, key string) string {
	var value string
	if data, ok := fields[key]; ok {
		_ = json.Unmarshal(data, &value)
	}
	return strings.TrimSpace(value)
}

func publishedJSONFloat64Field(fields map[string]json.RawMessage, key string) float64 {
	data, ok := fields[key]
	if !ok {
		return 0
	}
	var value float64
	if json.Unmarshal(data, &value) == nil {
		return value
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		value, _ = strconv.ParseFloat(strings.TrimSpace(text), 64)
	}
	return value
}

type publishedPriceTier struct {
	SKUID                   int64           `json:"sku_id"`
	ParentProductID         int64           `json:"parent_product_id"`
	BomSpecID               int64           `json:"bom_spec_id"`
	BomVariantID            int64           `json:"bom_variant_id"`
	Label                   string          `json:"label"`
	TierLabel               string          `json:"tier_label"`
	SourcePriceRecordID     int64           `json:"source_price_record_id"`
	FinalUnitPrice          float64         `json:"final_unit_price"`
	SpecG                   int64           `json:"spec_g"`
	MinQty                  float64         `json:"min_qty"`
	MaxQty                  *float64        `json:"max_qty"`
	PricePerUnit            float64         `json:"price_per_unit"`
	PricePerKg              float64         `json:"price_per_kg"`
	MinLb                   float64         `json:"min_lb"`
	MaxLb                   *float64        `json:"max_lb"`
	PricePerLb              float64         `json:"price_per_lb"`
	MinWeightG              float64         `json:"min_weight_g"`
	MaxWeightG              *float64        `json:"max_weight_g"`
	SalesUnit               string          `json:"sales_unit"`
	UnitBagCount            int64           `json:"unit_bag_count"`
	PackedPricePerBag       float64         `json:"packed_price_per_bag"`
	PackedPricePerBox       float64         `json:"packed_price_per_box"`
	DisplayUnit             string          `json:"display_unit"`
	PriceUnit               string          `json:"price_unit"`
	InventoryUnit           string          `json:"inventory_unit"`
	InventoryConversionJSON json.RawMessage `json:"inventory_conversion_json"`
	PricingRuleVersion      string          `json:"pricing_rule_version"`
	ManualAdjusted          bool            `json:"manual_adjusted"`
	CostSourceSnapshot      json.RawMessage `json:"cost_source_snapshot"`
	CustomerSnapshot        json.RawMessage `json:"customer_reference_snapshot"`
	QuantityBasis           string          `json:"quantity_basis"`
	TierQuantityUnit        string          `json:"tier_quantity_unit"`
	EffectiveSalesSpec      json.RawMessage `json:"effective_sales_spec"`
}

func publishedPricingFromContentForBOMSpec(raw []byte, productID, bomSpecID, bomVariantID, qty int64, listType string) (PublishedPricing, bool) {
	if productID <= 0 || bomSpecID <= 0 || bomVariantID <= 0 || qty <= 0 || len(raw) == 0 {
		return PublishedPricing{}, false
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return PublishedPricing{}, false
	}
	var flatRows []json.RawMessage
	_ = json.Unmarshal(root["price_rows"], &flatRows)
	flat := make([]publishedPriceTier, 0)
	for _, rowRaw := range flatRows {
		var fields map[string]json.RawMessage
		if json.Unmarshal(rowRaw, &fields) != nil || publishedFlatPriceRowProductID(fields) != productID {
			continue
		}
		if publishedJSONInt64Field(fields, "bom_spec_id") != bomSpecID {
			continue
		}
		var tier publishedPriceTier
		if json.Unmarshal(rowRaw, &tier) == nil {
			flat = append(flat, tier)
		}
	}
	if tier, ok := matchPublishedPriceTier(flat, 0, qty); ok {
		pricing := publishedTierPricing(tier, 0)
		return pricing, pricing.UnitPrice > 0
	}

	var content publishedBeanListContent
	if json.Unmarshal(raw, &content) != nil {
		return PublishedPricing{}, false
	}
	for _, group := range content.Groups {
		for _, itemRaw := range group.Items {
			var fields map[string]json.RawMessage
			if json.Unmarshal(itemRaw, &fields) != nil || publishedBeanListItemProductID(fields) != productID {
				continue
			}
			itemSpecID := publishedJSONInt64Field(fields, "bom_spec_id")
			itemVariantID := publishedJSONInt64Field(fields, "bom_variant_id")
			if itemSpecID != bomSpecID {
				continue
			}
			tiers := publishedItemTiers(itemRaw, listType)
			matching := make([]publishedPriceTier, 0, len(tiers))
			for _, tier := range tiers {
				if tier.BomSpecID == 0 {
					tier.BomSpecID = itemSpecID
				}
				if tier.BomVariantID == 0 {
					tier.BomVariantID = itemVariantID
				}
				if tier.BomSpecID == bomSpecID {
					matching = append(matching, tier)
				}
			}
			if tier, ok := matchPublishedPriceTier(matching, 0, qty); ok {
				pricing := publishedTierPricing(tier, 0)
				return pricing, pricing.UnitPrice > 0
			}
		}
	}
	return PublishedPricing{}, false
}

func publishedUnitPriceFromContent(raw []byte, productID int64, specG int64, qty int64) (float64, bool) {
	return publishedUnitPriceFromContentForListType(raw, productID, ListTypeGreen, specG, qty, "", 0)
}

func publishedUnitPriceFromContentForListType(raw []byte, productID int64, listType string, specG int64, qty int64, salesUnit string, unitBagCount int64) (float64, bool) {
	pricing, ok := publishedPricingFromContentForListType(raw, productID, listType, specG, qty, salesUnit, unitBagCount)
	return pricing.UnitPrice, ok
}

func publishedPricingFromContentForListType(raw []byte, productID int64, listType string, specG int64, qty int64, salesUnit string, unitBagCount int64) (PublishedPricing, bool) {
	if productID <= 0 || qty <= 0 || len(raw) == 0 {
		return PublishedPricing{}, false
	}
	if tiers := publishedFlatPriceRows(raw, productID); len(tiers) > 0 {
		if tier, ok := matchPublishedPriceTier(tiers, specG, qty); ok {
			price := publishedTierPricing(tier, specG)
			return price, price.UnitPrice > 0
		}
	}
	var content publishedBeanListContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return PublishedPricing{}, false
	}
	for _, group := range content.Groups {
		for _, itemRaw := range group.Items {
			if !publishedItemMatchesProduct(itemRaw, productID) {
				continue
			}
			tiers := publishedItemTiers(itemRaw, listType)
			if strings.TrimSpace(listType) == ListTypeDrip {
				if tier, ok := matchPublishedDripPriceTier(tiers, salesUnit, qty, unitBagCount); ok {
					price := publishedDripUnitPrice(tier, salesUnit, unitBagCount)
					if tier.FinalUnitPrice > 0 {
						price = tier.FinalUnitPrice
					}
					return publishedPricingWithSnapshot(PublishedPricing{UnitPrice: price, PriceUnit: normalizePublishedDripSalesUnit(salesUnit), UnitG: 1}, tier), price > 0
				}
				continue
			}
			if tier, ok := matchPublishedPriceTier(tiers, specG, qty); ok {
				price := publishedTierPricing(tier, specG)
				return price, price.UnitPrice > 0
			}
		}
	}
	return PublishedPricing{}, false
}

func publishedItemMatchesProduct(raw json.RawMessage, productID int64) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	return publishedBeanListItemProductID(fields) == productID
}

func publishedBeanListItemProductID(fields map[string]json.RawMessage) int64 {
	if skuID := publishedJSONInt64Field(fields, "sku_id", "skuId", "skuID"); skuID > 0 {
		return skuID
	}
	return publishedJSONInt64Field(fields, "productId", "product_id", "productID")
}

func publishedItemTiers(raw json.RawMessage, listType string) []publishedPriceTier {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil
	}
	var tiers []publishedPriceTier
	key := "commercial_wholesale_tiers"
	switch strings.TrimSpace(listType) {
	case ListTypeGreen:
		key = "green_bean_sale_tiers"
	case ListTypeRetail:
		key = "retail_bean_tiers"
	case ListTypeDrip:
		key = "drip_wholesale_tiers"
	}
	if data, ok := fields[key]; ok {
		_ = json.Unmarshal(data, &tiers)
	}
	return tiers
}

func publishedFlatPriceRows(raw []byte, productID int64) []publishedPriceTier {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil
	}
	var rows []json.RawMessage
	if data, ok := fields["price_rows"]; !ok || json.Unmarshal(data, &rows) != nil {
		return nil
	}
	out := make([]publishedPriceTier, 0, len(rows))
	for _, rowRaw := range rows {
		var rowFields map[string]json.RawMessage
		if err := json.Unmarshal(rowRaw, &rowFields); err != nil {
			continue
		}
		if publishedFlatPriceRowProductID(rowFields) != productID {
			continue
		}
		var tier publishedPriceTier
		if err := json.Unmarshal(rowRaw, &tier); err != nil {
			continue
		}
		if tier.SpecG <= 0 {
			tier.SpecG = int64(publishedPriceUnitG(tier.PriceUnit, 0))
		}
		out = append(out, tier)
	}
	return out
}

func publishedFlatPriceRowProductID(fields map[string]json.RawMessage) int64 {
	if skuID := publishedJSONInt64Field(fields, "sku_id", "skuId", "skuID"); skuID > 0 {
		return skuID
	}
	return publishedJSONInt64Field(fields, "product_id", "productId", "productID")
}

func publishedJSONInt64Field(fields map[string]json.RawMessage, keys ...string) int64 {
	for _, key := range keys {
		data, ok := fields[key]
		if !ok {
			continue
		}
		var id int64
		if json.Unmarshal(data, &id) == nil {
			return id
		}
		var text string
		if json.Unmarshal(data, &text) == nil {
			if id, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64); err == nil {
				return id
			}
		}
	}
	return 0
}

func matchPublishedPriceTier(tiers []publishedPriceTier, specG int64, qty int64) (publishedPriceTier, bool) {
	if len(tiers) == 0 {
		return publishedPriceTier{}, false
	}
	countTiers := make([]publishedPriceTier, 0, len(tiers))
	for _, tier := range tiers {
		if strings.TrimSpace(tier.QuantityBasis) == "sales_spec_count" {
			countTiers = append(countTiers, tier)
		}
	}
	if len(countTiers) > 0 {
		sortPublishedTiers(countTiers)
		for _, tier := range countTiers {
			if float64(qty) >= tier.MinQty && (tier.MaxQty == nil || float64(qty) <= *tier.MaxQty) {
				return tier, true
			}
		}
		return publishedPriceTier{}, false
	}
	totalG := float64(specG * qty)
	totalLb := totalG / 454.0
	sorted := append([]publishedPriceTier(nil), tiers...)
	sortPublishedTiers(sorted)
	for _, tier := range sorted {
		if tier.MinWeightG > 0 || tier.MaxWeightG != nil {
			if totalG >= tier.MinWeightG && (tier.MaxWeightG == nil || totalG <= *tier.MaxWeightG) {
				return tier, true
			}
		}
	}
	exactSpecTiers := make([]publishedPriceTier, 0, len(sorted))
	for _, tier := range sorted {
		if tier.MinWeightG > 0 || tier.MaxWeightG != nil || tier.MinLb > 0 || tier.MaxLb != nil || tier.SpecG != specG {
			continue
		}
		exactSpecTiers = append(exactSpecTiers, tier)
		if float64(qty) >= tier.MinQty && (tier.MaxQty == nil || float64(qty) <= *tier.MaxQty) {
			return tier, true
		}
	}
	for _, tier := range sorted {
		if tier.MinLb > 0 || tier.MaxLb != nil {
			if totalLb >= tier.MinLb && (tier.MaxLb == nil || totalLb <= *tier.MaxLb) {
				return tier, true
			}
		}
	}
	if len(exactSpecTiers) > 0 {
		return publishedPriceTier{}, false
	}
	for _, tier := range sorted {
		if tier.MinWeightG > 0 || tier.MaxWeightG != nil || tier.MinLb > 0 || tier.MaxLb != nil {
			continue
		}
		tierSpecG := tier.SpecG
		if tierSpecG <= 0 {
			tierSpecG = 1000
		}
		tierQty := totalG / float64(tierSpecG)
		if tierQty >= tier.MinQty && (tier.MaxQty == nil || tierQty <= *tier.MaxQty) {
			return tier, true
		}
	}
	return publishedPriceTier{}, false
}

func matchPublishedDripPriceTier(tiers []publishedPriceTier, salesUnit string, qty int64, unitBagCount int64) (publishedPriceTier, bool) {
	if len(tiers) == 0 || qty <= 0 {
		return publishedPriceTier{}, false
	}
	salesUnit = normalizePublishedDripSalesUnit(salesUnit)
	if unitBagCount <= 0 {
		unitBagCount = 1
	}
	if salesUnit == "box" {
		boxTiers := filterPublishedDripTiers(tiers, "box")
		if len(boxTiers) > 0 {
			return matchPublishedDripTierByQty(boxTiers, float64(qty))
		}
		bagTiers := filterPublishedDripTiers(tiers, "bag")
		return matchPublishedDripTierByQty(bagTiers, float64(qty*unitBagCount))
	}
	bagTiers := filterPublishedDripTiers(tiers, "bag")
	return matchPublishedDripTierByQty(bagTiers, float64(qty))
}

func filterPublishedDripTiers(tiers []publishedPriceTier, salesUnit string) []publishedPriceTier {
	out := make([]publishedPriceTier, 0, len(tiers))
	for _, tier := range tiers {
		if normalizePublishedDripSalesUnit(tier.SalesUnit) == salesUnit {
			out = append(out, tier)
		}
	}
	return out
}

func matchPublishedDripTierByQty(tiers []publishedPriceTier, qty float64) (publishedPriceTier, bool) {
	if len(tiers) == 0 {
		return publishedPriceTier{}, false
	}
	sorted := append([]publishedPriceTier(nil), tiers...)
	sortPublishedTiers(sorted)
	for _, tier := range sorted {
		if qty >= tier.MinQty && (tier.MaxQty == nil || qty <= *tier.MaxQty) {
			return tier, true
		}
	}
	return publishedPriceTier{}, false
}

func publishedDripUnitPrice(tier publishedPriceTier, salesUnit string, unitBagCount int64) float64 {
	salesUnit = normalizePublishedDripSalesUnit(salesUnit)
	if unitBagCount <= 0 {
		unitBagCount = 1
	}
	if salesUnit == "box" {
		if normalizePublishedDripSalesUnit(tier.SalesUnit) == "box" {
			if tier.PricePerUnit > 0 {
				return tier.PricePerUnit
			}
			return tier.PackedPricePerBox
		}
		if tier.PackedPricePerBox > 0 {
			return tier.PackedPricePerBox
		}
		bagPrice := publishedDripBagPrice(tier)
		if bagPrice > 0 {
			return bagPrice * float64(unitBagCount)
		}
		return 0
	}
	return publishedDripBagPrice(tier)
}

func publishedDripBagPrice(tier publishedPriceTier) float64 {
	if tier.PricePerUnit > 0 {
		return tier.PricePerUnit
	}
	if tier.PackedPricePerBag > 0 {
		return tier.PackedPricePerBag
	}
	return tier.PricePerLb
}

func normalizePublishedDripSalesUnit(unit string) string {
	if strings.TrimSpace(unit) == "box" {
		return "box"
	}
	return "bag"
}

func sortPublishedTiers(tiers []publishedPriceTier) {
	for i := 1; i < len(tiers); i++ {
		for j := i; j > 0 && publishedTierMinWeight(tiers[j]) > publishedTierMinWeight(tiers[j-1]); j-- {
			tiers[j], tiers[j-1] = tiers[j-1], tiers[j]
		}
	}
}

func publishedTierMinWeight(tier publishedPriceTier) float64 {
	if tier.MinWeightG > 0 {
		return tier.MinWeightG
	}
	if tier.MinLb > 0 {
		return tier.MinLb * 454.0
	}
	specG := tier.SpecG
	if specG <= 0 {
		specG = 1000
	}
	return tier.MinQty * float64(specG)
}

func publishedTierDisplayUnitPrice(tier publishedPriceTier, specG int64) float64 {
	return publishedTierPricing(tier, specG).UnitPrice
}

func publishedTierPricing(tier publishedPriceTier, specG int64) PublishedPricing {
	priceUnit := publishedTierPriceUnit(tier, specG)
	unitG := publishedPriceUnitG(priceUnit, specG)
	if tier.FinalUnitPrice > 0 {
		return publishedPricingWithSnapshot(PublishedPricing{UnitPrice: roundPublishedPrice(tier.FinalUnitPrice), PriceUnit: priceUnit, UnitG: unitG}, tier)
	}
	displayUnit := normalizePublishedPriceUnit(tier.DisplayUnit)
	displayG := publishedPriceUnitG(displayUnit, tier.SpecG)
	if displayG <= 0 {
		displayG = publishedPriceUnitG("", tier.SpecG)
	}
	if tier.PricePerKg > 0 {
		return publishedPricingWithSnapshot(PublishedPricing{UnitPrice: roundPublishedPrice(tier.PricePerKg * unitG / 1000.0), PriceUnit: priceUnit, UnitG: unitG}, tier)
	}
	if tier.PricePerLb > 0 && normalizePublishedPriceUnit(tier.PriceUnit) == "" && normalizePublishedPriceUnit(tier.DisplayUnit) == "" {
		price := tier.PricePerLb * unitG / 454.0
		if unitG == 1000 {
			price = math.Round(price)
		}
		return publishedPricingWithSnapshot(PublishedPricing{UnitPrice: roundPublishedPrice(price), PriceUnit: priceUnit, UnitG: unitG}, tier)
	}
	if tier.PricePerUnit > 0 {
		return publishedPricingWithSnapshot(PublishedPricing{UnitPrice: roundPublishedPrice(tier.PricePerUnit * unitG / displayG), PriceUnit: priceUnit, UnitG: unitG}, tier)
	}
	if tier.PricePerLb > 0 {
		return publishedPricingWithSnapshot(PublishedPricing{UnitPrice: roundPublishedPrice(tier.PricePerLb * unitG / 454.0), PriceUnit: priceUnit, UnitG: unitG}, tier)
	}
	return publishedPricingWithSnapshot(PublishedPricing{PriceUnit: priceUnit, UnitG: unitG}, tier)
}

func publishedPricingWithSnapshot(pricing PublishedPricing, tier publishedPriceTier) PublishedPricing {
	pricing.SourcePriceRecordID = tier.SourcePriceRecordID
	pricing.InventoryUnit = strings.TrimSpace(tier.InventoryUnit)
	pricing.TierLabel = strings.TrimSpace(tier.TierLabel)
	if pricing.TierLabel == "" {
		pricing.TierLabel = strings.TrimSpace(tier.Label)
	}
	if tier.FinalUnitPrice > 0 {
		pricing.FinalUnitPrice = roundPublishedPrice(tier.FinalUnitPrice)
	} else if pricing.UnitPrice > 0 {
		pricing.FinalUnitPrice = roundPublishedPrice(pricing.UnitPrice)
	}
	pricing.PricingRuleVersion = strings.TrimSpace(tier.PricingRuleVersion)
	pricing.ManualAdjusted = tier.ManualAdjusted
	if len(tier.InventoryConversionJSON) > 0 && string(tier.InventoryConversionJSON) != "null" {
		pricing.InventoryConversionJSON = strings.TrimSpace(string(tier.InventoryConversionJSON))
	}
	if pricing.InventoryConversionJSON == "" {
		pricing.InventoryConversionJSON = "{}"
	}
	if len(tier.CostSourceSnapshot) > 0 && string(tier.CostSourceSnapshot) != "null" {
		pricing.CostSourceSnapshotJSON = strings.TrimSpace(string(tier.CostSourceSnapshot))
	}
	if len(tier.CustomerSnapshot) > 0 && string(tier.CustomerSnapshot) != "null" {
		pricing.CustomerSnapshotJSON = strings.TrimSpace(string(tier.CustomerSnapshot))
	}
	pricing.QuantityBasis = strings.TrimSpace(tier.QuantityBasis)
	pricing.TierQuantityUnit = strings.TrimSpace(tier.TierQuantityUnit)
	if len(tier.EffectiveSalesSpec) > 0 && string(tier.EffectiveSalesSpec) != "null" {
		_, enriched := enrichPublishedEffectiveSalesSpec(
			tier.EffectiveSalesSpec,
			tier.ParentProductID,
			tier.InventoryUnit,
			tier.InventoryConversionJSON,
		)
		pricing.EffectiveSalesSpecJSON = strings.TrimSpace(string(enriched))
	}
	return pricing
}

func publishedTierPriceUnit(tier publishedPriceTier, specG int64) string {
	if unit := strings.TrimSpace(tier.PriceUnit); unit != "" {
		if normalized := normalizePublishedPriceUnit(unit); normalized != "" {
			return normalized
		}
		return unit
	}
	if unit := strings.TrimSpace(tier.DisplayUnit); unit != "" {
		if normalized := normalizePublishedPriceUnit(unit); normalized != "" {
			return normalized
		}
		return unit
	}
	if specG >= 1000 {
		return "kg"
	}
	return "lb"
}

func normalizePublishedPriceUnit(unit string) string {
	switch strings.TrimSpace(strings.ToLower(unit)) {
	case "kg", "lb", "g100", "g227", "g250":
		return strings.TrimSpace(strings.ToLower(unit))
	default:
		return ""
	}
}

func publishedPriceUnitG(unit string, specG int64) float64 {
	switch normalizePublishedPriceUnit(unit) {
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

func roundPublishedPrice(value float64) float64 {
	return math.Round((value+1e-9)*100) / 100
}

func isMissingBeanListSchema(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "42P01" || pgErr.Code == "42703"
}
