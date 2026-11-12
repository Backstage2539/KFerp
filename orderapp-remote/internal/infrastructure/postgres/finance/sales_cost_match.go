package finance

import (
	"math"
	"strings"
)

// convertBatchCost returns quantity in the order unit and cost per order
// unit. Weight is taken only from a frozen authoritative order snapshot.
func convertBatchCost(unit string, weightG, units, grams, batchG, batchUnitCost float64) (float64, float64, bool) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "公斤", "千克":
		if grams <= 0 || batchG <= 0 {
			return 0, 0, false
		}
		return grams / 1000, batchUnitCost, true
	case "g", "克":
		if grams <= 0 || batchG <= 0 {
			return 0, 0, false
		}
		return grams, batchUnitCost / 1000, true
	default:
		if units <= 0 {
			return 0, 0, false
		}
		if batchG > 0 {
			if weightG <= 0 {
				return 0, 0, false
			}
			return units, batchUnitCost * weightG / 1000, true
		}
		return units, batchUnitCost, true
	}
}

// All quantities are expressed in the order item's sales units. A source group
// identifies production which can also appear as a shipped stock batch.
type salesCostItem struct{ quantity float64 }
type salesCostPart struct {
	quantity, unitCost float64
	unknown            bool
	source, batch      string
}
type salesCostMatch struct{ knownCost, knownQty, pendingQty float64 }

func matchSalesCost(item salesCostItem, parts []salesCostPart) salesCostMatch {
	result := salesCostMatch{}
	claimed := 0.0
	actualByBatch := map[string]float64{}
	for _, source := range []string{"actual", "production"} {
		for _, p := range parts {
			if p.source != source || p.quantity <= 0 || claimed >= item.quantity {
				continue
			}
			qty := p.quantity
			if source == "production" && p.batch != "" {
				qty = math.Max(0, qty-actualByBatch[p.batch])
			}
			qty = math.Min(qty, item.quantity-claimed)
			if source == "actual" && p.batch != "" {
				actualByBatch[p.batch] += qty
			}
			claimed += qty
			if !p.unknown {
				result.knownQty += qty
				result.knownCost += qty * p.unitCost
			}
		}
	}
	result.pendingQty = math.Max(0, item.quantity-result.knownQty)
	result.knownCost = math.Round(result.knownCost*100) / 100
	return result
}
