package customerportal

import (
	"context"
	"net/http"

	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type miniProcessingCatalogFilter interface {
	FilterProcessingCatalogProductIDs(context.Context, string, []int64) ([]int64, error)
}

type miniProcessingBOMSpecCatalogFilter interface {
	ListProcessingCatalogTargets(context.Context, string, []int64) ([]customerportalapp.ProcessingCatalogTarget, error)
}

func registerMiniProcessingCatalogAPI(e *echo.Echo, portal Service, sales EmployeeSales) {
	e.GET("/api/mini/processing/catalog", func(c echo.Context) error {
		if portal == nil || sales == nil {
			return miniInternalError(c)
		}
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		current, err := portal.Me(c.Request().Context(), token)
		if err != nil {
			return miniBusinessError(c, err)
		}
		if current.CurrentCustomerID <= 0 {
			return miniBusinessError(c, customerportalapp.ErrCustomerBindingNotFound)
		}
		if !current.HasCapability(customerportalapp.CapabilityProcessing) {
			return miniBusinessError(c, customerportalapp.ErrCapabilityNotEnabled)
		}
		form, err := sales.OrderForm(c.Request().Context(), 0)
		if err != nil {
			return miniInternalError(c)
		}
		visible := salesapp.FilterOrderProductsForCustomer(form.Products, current.CurrentCustomerID, nil, form.CustomerPublicUsages)
		candidateIDs := make([]int64, 0, len(visible))
		seen := make(map[int64]bool, len(visible)*2)
		for _, product := range visible {
			productID := product.SKUID
			if productID <= 0 {
				productID = product.ID
			}
			if productID > 0 && !seen[productID] {
				seen[productID] = true
				candidateIDs = append(candidateIDs, productID)
			}
		}

		if specFilter, ok := portal.(miniProcessingBOMSpecCatalogFilter); ok {
			for _, product := range visible {
				parentID := product.ParentProductID
				if parentID <= 0 {
					parentID = product.ID
				}
				if parentID > 0 && !seen[parentID] {
					seen[parentID] = true
					candidateIDs = append(candidateIDs, parentID)
				}
			}
			targets, err := specFilter.ListProcessingCatalogTargets(c.Request().Context(), token, candidateIDs)
			if err != nil {
				return miniBusinessError(c, err)
			}
			return miniProcessingCatalogResponse(c, current.CurrentCustomerID, visible, form.ProductBOMSpecOptions, targets)
		}

		filter, ok := portal.(miniProcessingCatalogFilter)
		if !ok {
			return miniInternalError(c)
		}
		allowedIDs, err := filter.FilterProcessingCatalogProductIDs(c.Request().Context(), token, candidateIDs)
		if err != nil {
			return miniBusinessError(c, err)
		}
		allowed := make(map[int64]bool, len(allowedIDs))
		for _, productID := range allowedIDs {
			allowed[productID] = true
		}
		products := make([]salesapp.ProductOption, 0, len(visible))
		for _, product := range visible {
			productID := product.SKUID
			if productID <= 0 {
				productID = product.ID
			}
			if !allowed[productID] {
				continue
			}
			// A production request has no sales price contract. Keep the shared
			// selector metadata while removing every pricing source.
			product.DefaultPrice = 0
			product.RetailPrice100G = 0
			product.RetailPrice200G = 0
			product.RetailPrice227G = 0
			product.RetailPrice250G = 0
			product.Tiers = nil
			products = append(products, product)
		}
		families := salesapp.BuildOrderProductFamilies(products)
		for _, family := range families {
			rawSpecs, _ := family["specs"].([]map[string]any)
			for _, spec := range rawSpecs {
				delete(spec, "tiers")
				delete(spec, "publication_ids")
				delete(spec, "default_publication_id")
			}
		}
		return c.JSON(http.StatusOK, map[string]any{
			"current_customer_id": current.CurrentCustomerID,
			"product_families":    families,
		})
	})
}

func miniProcessingCatalogResponse(
	c echo.Context,
	currentCustomerID int64,
	visible []salesapp.ProductOption,
	options []salesapp.ProductBOMSpecOption,
	targets []customerportalapp.ProcessingCatalogTarget,
) error {
	allowedLegacy := map[int64]bool{}
	canonicalBySpec := map[int64]customerportalapp.ProcessingCatalogTarget{}
	cutoverParents := map[int64]bool{}
	for _, target := range targets {
		if target.BomSpecID > 0 {
			canonicalBySpec[target.BomSpecID] = target
			cutoverParents[target.ProductID] = true
			continue
		}
		allowedLegacy[target.ProductID] = true
	}
	legacyProducts := make([]salesapp.ProductOption, 0, len(visible))
	for _, product := range visible {
		productID := product.SKUID
		if productID <= 0 {
			productID = product.ID
		}
		parentID := product.ParentProductID
		if parentID <= 0 {
			parentID = product.ID
		}
		if cutoverParents[parentID] || !allowedLegacy[productID] {
			continue
		}
		clearProcessingCatalogPrices(&product)
		legacyProducts = append(legacyProducts, product)
	}
	families := salesapp.BuildOrderProductFamilies(legacyProducts)

	canonicalOptions := make([]salesapp.ProductBOMSpecOption, 0, len(canonicalBySpec))
	for _, option := range options {
		target, ok := canonicalBySpec[option.BomSpecID]
		if !ok || option.ParentProductID != target.ProductID || option.BomVariantID != target.BomVariantID {
			continue
		}
		option.Tiers = nil
		canonicalOptions = append(canonicalOptions, option)
	}
	canonicalProducts, optionBySpec := miniEmployeeBOMSpecFilterProducts(visible, canonicalOptions)
	for index := range canonicalProducts {
		clearProcessingCatalogPrices(&canonicalProducts[index])
	}
	canonicalFamilies, visibleCanonicalOptions := miniEmployeeBOMSpecFamilies(canonicalProducts, optionBySpec)
	families = append(families, canonicalFamilies...)

	for _, family := range families {
		rawSpecs, _ := family["specs"].([]map[string]any)
		for _, spec := range rawSpecs {
			delete(spec, "tiers")
			delete(spec, "publication_ids")
			delete(spec, "default_publication_id")
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"current_customer_id":      currentCustomerID,
		"product_families":         families,
		"product_bom_spec_options": visibleCanonicalOptions,
	})
}

func clearProcessingCatalogPrices(product *salesapp.ProductOption) {
	if product == nil {
		return
	}
	product.DefaultPrice = 0
	product.RetailPrice100G = 0
	product.RetailPrice200G = 0
	product.RetailPrice227G = 0
	product.RetailPrice250G = 0
	product.Tiers = nil
}
