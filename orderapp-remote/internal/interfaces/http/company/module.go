package company

import (
	companyapp "orderapp/internal/application/company"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Company *companyapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerCompanyStaffPages(e)
	registerCompanyStaffAPI(e, deps.Company)
}
