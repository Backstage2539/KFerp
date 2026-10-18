package sales

// IsFreeSampleSettlement never treats discounts or rounding away a charge as a free sample.
// The sample identity is established by the repository, never by a request flag.
func IsFreeSampleSettlement(sample bool, goods, shipping, other, total float64) bool {
	return sample && goods == 0 && shipping == 0 && other == 0 && total == 0
}
