package support

type InlineUpdateRequest struct {
	OrderTypeID     string `form:"order_type_id"`
	PayStatusID     string `form:"pay_status_id"`
	PaymentMethod   string `form:"payment_method"`
	ShipStatusID    string `form:"ship_status_id"`
	ProcessStatusID string `form:"process_status_id"`
	Notes           string `form:"notes"`
}
