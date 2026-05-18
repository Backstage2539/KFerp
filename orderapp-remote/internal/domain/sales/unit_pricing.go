package sales

import (
	"fmt"
	"orderapp/internal/domain/catalog"
	"strings"
)

type UnitPriceTier struct {
	ProductKind  string
	SalesUnit    string
	MinQty       float64
	PricePerUnit float64
	UnitBagCount float64
}

type UnitLineInput struct {
	ProductKind  string
	SalesUnit    string
	Quantity     float64
	UnitBagCount float64
	Tiers        []UnitPriceTier
}

type UnitLineResult struct {
	UnitPrice         float64
	LineTotal         float64
	MatchedQtyForTier float64
	Tier              UnitPriceTier
}

func CalculateUnitLineTotal(in UnitLineInput) (UnitLineResult, error) {
	if in.Quantity <= 0 {
		return UnitLineResult{}, fmt.Errorf("quantity must be > 0")
	}
	productKind := catalog.NormalizeProductKind(in.ProductKind)
	salesUnit := normalizeSalesUnit(in.SalesUnit)
	matchedQty := in.Quantity
	if productKind == catalog.ProductKindDripBag && salesUnit == "box" {
		matchedQty = in.Quantity * normalizeUnitBagCount(in.UnitBagCount)
	}

	var matched UnitPriceTier
	found := false
	for _, tier := range in.Tiers {
		if catalog.NormalizeProductKind(tier.ProductKind) != productKind {
			continue
		}
		if normalizeSalesUnit(tier.SalesUnit) != salesUnit {
			continue
		}
		if matchedQty < tier.MinQty {
			continue
		}
		if !found || tier.MinQty > matched.MinQty {
			matched = tier
			found = true
		}
	}
	if !found {
		return UnitLineResult{}, fmt.Errorf("no unit price tier matched")
	}
	return UnitLineResult{
		UnitPrice:         matched.PricePerUnit,
		LineTotal:         matched.PricePerUnit * in.Quantity,
		MatchedQtyForTier: matchedQty,
		Tier:              matched,
	}, nil
}

func CalculateLegacyWeightLineTotal(unitPrice float64, specG int64, units float64) float64 {
	if unitPrice <= 0 || specG <= 0 || units <= 0 {
		return 0
	}
	return unitPrice * (float64(specG) * units / legacyWeightDisplayUnitG(specG))
}

func normalizeSalesUnit(unit string) string {
	return strings.TrimSpace(unit)
}

func normalizeUnitBagCount(count float64) float64 {
	if count > 0 {
		return count
	}
	return 1
}

func legacyWeightDisplayUnitG(specG int64) float64 {
	if specG >= 1000 {
		return 1000
	}
	return 454
}
