package catalog

import catalogapp "orderapp/internal/application/catalog"

func productOptionFromCatalog(p catalogapp.Product) ProductOption {
	skuID := p.SKUID
	if skuID <= 0 {
		skuID = p.ID
	}
	effectiveParentProductID := p.EffectiveParentProductID
	if effectiveParentProductID <= 0 {
		if p.ParentProductID > 0 {
			effectiveParentProductID = p.ParentProductID
		} else {
			effectiveParentProductID = p.ID
		}
	}
	skuName := p.SKUName
	if skuName == "" {
		if p.ParentProductID > 0 {
			skuName = p.Name
		} else {
			skuName = "默认规格"
		}
	}
	out := ProductOption{
		ID:                          p.ID,
		SKUID:                       skuID,
		ParentProductID:             p.ParentProductID,
		EffectiveParentProductID:    effectiveParentProductID,
		SKUName:                     skuName,
		SKUCode:                     p.SKUCode,
		Barcode:                     p.Barcode,
		SpecLabel:                   p.SpecLabel,
		NetContentQty:               p.NetContentQty,
		NetContentUnit:              p.NetContentUnit,
		IsDefaultSKU:                p.IsDefaultSKU || p.ParentProductID == 0,
		AutoDerivedSKU:              p.AutoDerivedSKU,
		DerivedUnitTemplateID:       p.DerivedUnitTemplateID,
		DerivedSpecKey:              p.DerivedSpecKey,
		DerivedSpecName:             p.DerivedSpecName,
		DerivedSalesUnit:            p.DerivedSalesUnit,
		DerivedSpecStatus:           p.DerivedSpecStatus,
		Name:                        p.Name,
		Remark:                      p.Remark,
		ProductKind:                 p.ProductKind,
		GreenBeanType:               p.GreenBeanType,
		GreenBeanBomProductID:       p.GreenBeanBomProductID,
		RoastLevel:                  p.RoastLevel,
		SpecialAttrsJSON:            p.SpecialAttrsJSON,
		DripBagGrams:                p.DripBagGrams,
		DripBoxBagCount:             p.DripBoxBagCount,
		AllowFulfillmentOrder:       p.AllowFulfillmentOrder,
		AllowMallOrder:              p.AllowMallOrder,
		SalesUnits:                  p.SalesUnits,
		DefaultPrice:                p.DefaultPrice,
		RetailPrice100G:             p.RetailPrice100G,
		RetailPrice200G:             p.RetailPrice200G,
		RetailPrice227G:             p.RetailPrice227G,
		RetailPrice250G:             p.RetailPrice250G,
		YieldRate:                   p.YieldRate,
		ProductCategoryID:           p.ProductCategoryID,
		ProductCategoryPosition:     p.ProductCategoryPosition,
		ClassificationTemplateID:    p.ClassificationTemplateID,
		CustomerID:                  p.CustomerID,
		BaseProductID:               p.BaseProductID,
		Visibility:                  p.Visibility,
		CustomType:                  p.CustomType,
		MarginRateOverride:          p.MarginRateOverride,
		GradientTemplateIDOverride:  p.GradientTemplateIDOverride,
		OperationTemplateIDOverride: p.OperationTemplateIDOverride,
		UnitRuleOverrideJSON:        p.UnitRuleOverrideJSON,
		InventoryUnit:               p.InventoryUnit,
		IntegerInventoryUnit:        p.IntegerInventoryUnit,
		DefaultSalesUnit:            p.DefaultSalesUnit,
		UnitConversionJSON:          p.UnitConversionJSON,
		SalesUnitRulesJSON:          p.SalesUnitRulesJSON,
		UnitTemplateID:              p.UnitTemplateID,
		UnitTemplateName:            p.UnitTemplateName,
		UnitRuleSource:              p.UnitRuleSource,
		ProductConfigTemplateID:     p.ProductConfigTemplateID,
		BomItemCount:                p.BomItemCount,
		BomStatus:                   p.BomStatus,
		BomSourceType:               p.BomSourceType,
		EffectiveProductID:          p.EffectiveProductID,
		EffectiveBomVersionID:       p.EffectiveBomVersionID,
		SourceProductID:             p.SourceProductID,
		SourceProductCode:           p.SourceProductCode,
		SourceProductName:           p.SourceProductName,
		SourceBomVersionID:          p.SourceBomVersionID,
		SourceBomVersionNo:          p.SourceBomVersionNo,
		DerivedFromLabel:            p.DerivedFromLabel,
		CanEditBOM:                  p.CanEditBOM,
		ProductionBomID:             p.ProductionBomID,
		ProductionBomCode:           p.ProductionBomCode,
		ProductionBomName:           p.ProductionBomName,
		ProductionBomVersionID:      p.ProductionBomVersionID,
		ProductionBomVersionNo:      p.ProductionBomVersionNo,
		LatestBomVersionID:          p.LatestBomVersionID,
		LatestBomVersionNo:          p.LatestBomVersionNo,
		IsLatestBomVersion:          p.IsLatestBomVersion,
		ProductionBomGroupID:        p.ProductionBomGroupID,
		ProductionBomGroupName:      p.ProductionBomGroupName,
		OrderUsageCount:             p.OrderUsageCount,
	}
	out.Tiers = make([]ProductTierOption, 0, len(p.Tiers))
	for _, t := range p.Tiers {
		out.Tiers = append(out.Tiers, ProductTierOption{
			ID:        t.ID,
			SpecG:     t.SpecG,
			MinQty:    t.MinQty,
			MaxQty:    t.MaxQty,
			UnitPrice: t.UnitPrice,
		})
	}
	return out
}

func productOptionsFromCatalog(products []catalogapp.Product) []ProductOption {
	out := make([]ProductOption, 0, len(products))
	for _, p := range products {
		out = append(out, productOptionFromCatalog(p))
	}
	return out
}
