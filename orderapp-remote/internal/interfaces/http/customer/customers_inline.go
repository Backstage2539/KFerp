package customer

type CustomerInlineReq struct {
	Name                  string `form:"name"`
	CustomerType          string `form:"customer_type"`
	CompanyName           string `form:"company_name"`
	CompanyAddress        string `form:"company_address"`
	CompanyPhone          string `form:"company_phone"`
	Contact               string `form:"contact"`
	Phone                 string `form:"phone"`
	Address               string `form:"address"`
	DefaultSourceID       string `form:"default_source_id"`
	DefaultOrderTypeID    string `form:"default_order_type_id"`
	ResponsibleEmployeeID string `form:"responsible_employee_id"`
	Active                string `form:"active"`
}
