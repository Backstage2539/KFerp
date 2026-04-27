package materials

type MaterialRow struct {
	ID            int64   `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"` // bean|pack|other
	Unit          string  `json:"unit"` // g|unit
	PurchasePrice float64 `json:"purchase_price"`
	SalePrice     float64 `json:"sale_price"`
	OnhandG       int64   `json:"onhand_g"`
	OnhandUnits   int64   `json:"onhand_units"`
	MinLevelG     int64   `json:"min_level_g"`
	MinLevelUnits int64   `json:"min_level_units"`
	UpdatedAt     string  `json:"updated_at"`
}

type MaterialInput struct {
	Code          string  `json:"code" form:"code"`
	Name          string  `json:"name" form:"name"`
	Kind          string  `json:"kind" form:"kind"`
	Unit          string  `json:"unit" form:"unit"`
	PurchasePrice float64 `json:"purchase_price" form:"purchase_price"`
	SalePrice     float64 `json:"sale_price" form:"sale_price"`
	OnhandG       int64   `json:"onhand_g" form:"onhand_g"`
	OnhandUnits   int64   `json:"onhand_units" form:"onhand_units"`
	MinLevelG     int64   `json:"min_level_g" form:"min_level_g"`
	MinLevelUnits int64   `json:"min_level_units" form:"min_level_units"`
}
