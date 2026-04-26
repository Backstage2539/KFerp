package bom

import bomdomain "orderapp/internal/domain/bom"

func normalizeBomRatioPct(v float64) float64 {
	return bomdomain.NormalizeRatioPct(v)
}

func NormalizeBomRatioPct(v float64) float64 {
	return bomdomain.NormalizeRatioPct(v)
}
