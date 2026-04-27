package production

type ProduceBatchSummaryItem struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	SpecG       int64  `json:"spec_g"`
	NeedUnits   int64  `json:"need_units"`
	NeedG       int64  `json:"need_g"`
	DeductedG   int64  `json:"deducted_g"`
	GapG        int64  `json:"gap_g"`
}

type ProduceBatchCreateResult struct {
	BatchID    string                    `json:"batch_id"`
	OrderCount int                       `json:"order_count"`
	Summary    []ProduceBatchSummaryItem `json:"summary"`
}
