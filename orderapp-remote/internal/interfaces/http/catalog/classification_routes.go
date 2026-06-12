package catalog

import "github.com/labstack/echo/v4"

func registerClassificationRoutes(e *echo.Echo, h productHandler) {
	e.GET("/api/product-classification-templates", h.productClassificationTemplatesAPI)
	e.POST("/api/product-classification-templates", h.saveProductClassificationTemplateAPI)
	e.PUT("/api/product-classification-templates/:id", h.saveProductClassificationTemplateAPI)
	e.DELETE("/api/product-classification-templates/:id", h.deleteProductClassificationTemplateAPI)
	e.POST("/api/product-classification-template-categories", h.saveProductClassificationCategoryAPI)
	e.PUT("/api/product-classification-template-categories/:id", h.saveProductClassificationCategoryAPI)
	e.DELETE("/api/product-classification-template-categories/:id", h.deleteProductClassificationCategoryAPI)
	e.GET("/api/product-classification-template-usages/products", h.productClassificationTemplateUsagesAPI)
	e.POST("/api/product-classification-template-usages/products", h.saveProductClassificationTemplateUsageAPI)
	e.DELETE("/api/product-classification-template-usages/products/:template_id", h.deleteProductClassificationTemplateUsageAPI)
	e.GET("/api/product-classification-template-usages/customer-aliases", h.customerProductAliasClassificationTemplateUsagesAPI)
	e.POST("/api/product-classification-template-usages/customer-aliases", h.saveCustomerProductAliasClassificationTemplateUsageAPI)
	e.DELETE("/api/product-classification-template-usages/customer-aliases/:template_id", h.deleteCustomerProductAliasClassificationTemplateUsageAPI)
	e.POST("/api/product-classification-assignments/products", h.saveProductClassificationAssignmentAPI)
	e.POST("/api/product-classification-assignments/customer-aliases", h.saveCustomerProductAliasClassificationAssignmentAPI)
}
