package production

func normalizeBomRatioPct(v float64) float64 {
	if v > 0 && v <= 1 {
		return v * 100
	}
	return v
}
