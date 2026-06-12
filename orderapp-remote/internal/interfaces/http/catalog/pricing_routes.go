package catalog

import "github.com/labstack/echo/v4"

func registerPricingRoutes(e *echo.Echo, h productHandler) {
	e.GET("/api/product-pricing-rules", h.productPricingRulesAPI)
	e.POST("/api/product-pricing-rules", h.saveProductPricingRuleAPI)
	e.PUT("/api/product-pricing-rules/:id", h.saveProductPricingRuleAPI)
	e.GET("/api/price-tier-templates", h.priceTierTemplatesAPI)
	e.POST("/api/price-tier-templates", h.savePriceTierTemplateAPI)
	e.PUT("/api/price-tier-templates/:id", h.savePriceTierTemplateAPI)
	e.DELETE("/api/price-tier-templates/:id", h.deletePriceTierTemplateAPI)
	e.GET("/api/pricing-gradient-templates", h.gradientTemplatesAPI)
	e.POST("/api/pricing-gradient-templates", h.saveGradientTemplateAPI)
	e.PUT("/api/pricing-gradient-templates/:id", h.saveGradientTemplateAPI)
	e.POST("/api/pricing-gradient-templates/:id/deactivate", h.deactivateGradientTemplateAPI)
	e.GET("/api/product-price-groups", h.productPriceGroupsAPI)
	e.POST("/api/product-price-groups", h.saveProductPriceGroupAPI)
	e.PUT("/api/product-price-groups/:id", h.saveProductPriceGroupAPI)
	e.GET("/api/product-price-records", h.productPriceRecordsAPI)
	e.POST("/api/product-price-records", h.saveProductPriceRecordAPI)
	e.PUT("/api/product-price-records/:id", h.saveProductPriceRecordAPI)
	e.GET("/api/product-tier-price-schemes", h.productTierPriceSchemesAPI)
	e.POST("/api/product-tier-price-schemes", h.saveProductTierPriceSchemeAPI)
	e.PUT("/api/product-tier-price-schemes/:id", h.saveProductTierPriceSchemeAPI)
}
