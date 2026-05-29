package customer

import customerapp "orderapp/internal/application/customer"

type CustomerRow = customerapp.CustomerRow
type CustomerDashboard = customerapp.CustomerDashboard
type CustomerEditData = customerapp.CustomerEditData

type CustomerUpsertRequest struct {
	Name               string `form:"name"`
	RawName            string `form:"raw_name"`
	CustomerType       string `form:"customer_type"`
	CompanyName        string `form:"company_name"`
	CompanyAddress     string `form:"company_address"`
	CompanyPhone       string `form:"company_phone"`
	Contact            string `form:"contact"`
	Phone              string `form:"phone"`
	Address            string `form:"address"`
	DefaultSourceID    string `form:"default_source_id"`
	DefaultOrderTypeID string `form:"default_order_type_id"`
	Active             string `form:"active"`
	PortalEnabled      *bool
}
