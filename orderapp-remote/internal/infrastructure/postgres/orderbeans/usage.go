package orderbeans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	usage, err := ResolveUsageForPublication(ctx, q, schema, customerID, productID, listType, requestedPublicationID)
	if err != nil || usage.PublicationID <= 0 {
		return 0, err
	}
	var raw []byte
	sql := fmt.Sprintf(`SELECT COALESCE(content_json, '{}'::jsonb) FROM %s.bean_list_publications WHERE id=$1`, schema)
	if err := q.QueryRow(ctx, sql, usage.PublicationID).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isMissingBeanListSchema(err) {
			return 0, nil
		}
		return 0, err
	}
	price, ok := publishedUnitPriceFromContentForListType(raw, productID, listType, specG, qty, salesUnit, unitBagCount)
	if !ok {
		return 0, nil
	}
	return price, nil
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
		  AND (
		    ($2 <> '' AND blp.owner_type='customer' AND blp.owner_key=$2)
		    OR blp.owner_type='official'
		  )
		  AND EXISTS (
		    SELECT 1
		    FROM jsonb_array_elements(COALESCE(blp.content_json->'groups', '[]'::jsonb)) AS groups(group_json)
		    CROSS JOIN LATERAL jsonb_array_elements(COALESCE(groups.group_json->'items', '[]'::jsonb)) AS items(item_json)
		    WHERE (
		      (items.item_json->>'productId' ~ '^[0-9]+$' AND (items.item_json->>'productId')::bigint=$3)
		      OR (items.item_json->>'product_id' ~ '^[0-9]+$' AND (items.item_json->>'product_id')::bigint=$3)
		      OR (items.item_json->>'productID' ~ '^[0-9]+$' AND (items.item_json->>'productID')::bigint=$3)
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

type publishedPriceTier struct {
	SpecG             int64    `json:"spec_g"`
	MinQty            float64  `json:"min_qty"`
	MaxQty            *float64 `json:"max_qty"`
	PricePerUnit      float64  `json:"price_per_unit"`
	MinLb             float64  `json:"min_lb"`
	MaxLb             *float64 `json:"max_lb"`
	PricePerLb        float64  `json:"price_per_lb"`
	MinWeightG        float64  `json:"min_weight_g"`
	MaxWeightG        *float64 `json:"max_weight_g"`
	SalesUnit         string   `json:"sales_unit"`
	UnitBagCount      int64    `json:"unit_bag_count"`
	PackedPricePerBag float64  `json:"packed_price_per_bag"`
	PackedPricePerBox float64  `json:"packed_price_per_box"`
}

func publishedUnitPriceFromContent(raw []byte, productID int64, specG int64, qty int64) (float64, bool) {
	return publishedUnitPriceFromContentForListType(raw, productID, ListTypeGreen, specG, qty, "", 0)
}

func publishedUnitPriceFromContentForListType(raw []byte, productID int64, listType string, specG int64, qty int64, salesUnit string, unitBagCount int64) (float64, bool) {
	if productID <= 0 || specG <= 0 || qty <= 0 || len(raw) == 0 {
		return 0, false
	}
	var content publishedBeanListContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return 0, false
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
					return price, price > 0
				}
				continue
			}
			if tier, ok := matchPublishedPriceTier(tiers, specG, qty); ok {
				price := publishedTierDisplayUnitPrice(tier, specG)
				return price, price > 0
			}
		}
	}
	return 0, false
}

func publishedItemMatchesProduct(raw json.RawMessage, productID int64) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	for _, key := range []string{"productId", "product_id", "productID"} {
		var id int64
		if data, ok := fields[key]; ok && json.Unmarshal(data, &id) == nil && id == productID {
			return true
		}
	}
	return false
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

func matchPublishedPriceTier(tiers []publishedPriceTier, specG int64, qty int64) (publishedPriceTier, bool) {
	if len(tiers) == 0 {
		return publishedPriceTier{}, false
	}
	totalG := float64(specG * qty)
	totalLb := totalG / 454.0
	sorted := append([]publishedPriceTier(nil), tiers...)
	sortPublishedTiers(sorted)
	for _, tier := range sorted {
		if tier.MinWeightG > 0 && totalG >= tier.MinWeightG && (tier.MaxWeightG == nil || totalG <= *tier.MaxWeightG) {
			return tier, true
		}
	}
	for _, tier := range sorted {
		tierSpec := tier.SpecG
		if tierSpec <= 0 {
			tierSpec = 1000
		}
		if tierSpec != specG {
			continue
		}
		tierQty := totalG / float64(tierSpec)
		if tierQty >= tier.MinQty && (tier.MaxQty == nil || tierQty <= *tier.MaxQty) {
			return tier, true
		}
	}
	for _, tier := range sorted {
		if tier.MinLb > 0 && totalLb >= tier.MinLb && (tier.MaxLb == nil || totalLb <= *tier.MaxLb) {
			return tier, true
		}
	}
	return sorted[len(sorted)-1], true
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
		if tier, ok := matchPublishedDripTierByQty(boxTiers, float64(qty)); ok {
			return tier, true
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
	return sorted[len(sorted)-1], true
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
	pricePerLb := tier.PricePerLb
	tierSpec := tier.SpecG
	if tierSpec <= 0 {
		tierSpec = 1000
	}
	if pricePerLb <= 0 && tier.PricePerUnit > 0 {
		pricePerLb = tier.PricePerUnit * 454.0 / float64(tierSpec)
	}
	if pricePerLb <= 0 {
		return 0
	}
	unitG := 454.0
	if specG >= 1000 {
		unitG = 1000
	}
	price := pricePerLb * unitG / 454.0
	if unitG == 1000 {
		return math.Round(price)
	}
	return price
}

func isMissingBeanListSchema(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "42P01" || pgErr.Code == "42703"
}
