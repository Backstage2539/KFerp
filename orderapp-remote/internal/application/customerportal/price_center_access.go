package customerportal

func CanUsePriceCenter(c CurrentContext) bool {
	if c.AccountType != "employee" || c.EmployeeID <= 0 {
		return false
	}
	for _, role := range c.Roles {
		if role == "sales" || role == "admin" {
			return true
		}
	}
	return false
}
