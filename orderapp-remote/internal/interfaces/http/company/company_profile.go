package company

import (
	"net/http"

	companyapp "orderapp/internal/application/company"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type companyProfileReq struct {
	Name    string `json:"company_name"`
	Address string `json:"company_address"`
	Phone   string `json:"company_phone"`
}

func registerCompanyProfileAPI(e *echo.Echo, companySvc *companyapp.Service) {
	e.GET("/api/company/profile", func(c echo.Context) error {
		profile, err := companySvc.LoadCompanyProfile(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, companyProfileFromApp(profile))
	})
	e.POST("/api/company/profile", func(c echo.Context) error {
		var req companyProfileReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		profile, err := companySvc.SaveCompanyProfile(c.Request().Context(), companyapp.CompanyProfileCommand{
			Actor:   support.ActorOf(c),
			Name:    req.Name,
			Address: req.Address,
			Phone:   req.Phone,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, companyProfileFromApp(profile))
	})
}

func companyProfileFromApp(profile companyapp.CompanyProfile) map[string]string {
	return map[string]string{
		"company_name":    profile.Name,
		"company_address": profile.Address,
		"company_phone":   profile.Phone,
	}
}
