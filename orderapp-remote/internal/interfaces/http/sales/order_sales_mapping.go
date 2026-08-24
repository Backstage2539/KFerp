package sales

import (
	"fmt"
	"math"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"
)

func saveOrderCommandFromCreateRequest(req CreateOrderRequest, editID int64, actor string) (salesapp.SaveOrderCommand, error) {
	orderDate := strings.TrimSpace(req.OrderDate)
	if orderDate == "" {
		orderDate = time.Now().Format("2006-01-02")
	}
	od, err := time.Parse("2006-01-02", orderDate)
	if err != nil {
		return salesapp.SaveOrderCommand{}, fmt.Errorf("invalid order_date")
	}
	documentDate := strings.TrimSpace(req.DocumentDate)
	if documentDate == "" {
		documentDate = orderDate
	}
	dd, err := time.Parse("2006-01-02", documentDate)
	if err != nil {
		return salesapp.SaveOrderCommand{}, fmt.Errorf("invalid document_date")
	}
	shippingAmount, err := parseCreateOrderAmount(req.ShippingAmount, "shipping_amount")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	discountAmount, err := parseCreateOrderAmount(req.DiscountAmount, "discount_amount")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	paymentGoodsAmount, err := parseCreateOrderAmount(req.PaymentGoodsAmount, "payment_goods_amount")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	paymentShippingAmount, err := parseCreateOrderAmount(req.PaymentShippingAmount, "payment_shipping_amount")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	materialFee, err := parseCreateOrderAmount(req.OutsourceMaterialFee, "outsource_material_fee")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	roastFee, err := parseCreateOrderAmount(req.OutsourceRoastFee, "outsource_roast_fee")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	packagingFee, err := parseCreateOrderAmount(req.OutsourcePackagingFee, "outsource_packaging_fee")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	manualFee, err := parseCreateOrderAmount(req.OutsourceManualFee, "outsource_manual_fee")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	taxFee, err := parseCreateOrderAmount(req.OutsourceTaxFee, "outsource_tax_fee")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	otherFee, err := parseCreateOrderAmount(req.OutsourceOtherFee, "outsource_other_fee")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	if err := validateCreateOrderManualPrices(req); err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	return salesapp.SaveOrderCommand{
		Actor:                           actor,
		EditID:                          editID,
		DocumentDate:                    dd,
		OrderDate:                       od,
		CustomerID:                      req.CustomerID,
		SourceID:                        req.SourceID,
		OrderTypeID:                     req.OrderTypeID,
		PayStatusID:                     req.PayStatusID,
		PaymentMethod:                   strings.TrimSpace(req.PaymentMethod),
		ShipStatusID:                    req.ShipStatusID,
		ShipMethod:                      req.ShipMethod,
		ShipTrackingNo:                  req.ShipTrackingNo,
		LogisticsCompanyID:              req.LogisticsCompanyID,
		LogisticsProductID:              req.LogisticsProductID,
		PaymentGoodsAmount:              paymentGoodsAmount,
		PaymentShippingAmount:           paymentShippingAmount,
		PaymentVoucherAssetID:           req.PaymentVoucherAssetID,
		ResponsibleType:                 strings.TrimSpace(req.ResponsibleType),
		ResponsibleID:                   req.ResponsibleID,
		Notes:                           req.Notes,
		ShippingAmount:                  shippingAmount,
		DiscountAmount:                  discountAmount,
		RoundToInt:                      strings.TrimSpace(req.RoundToInt) != "",
		ExpressFee:                      req.ExpressFee,
		OutsourceMaterialFee:            materialFee,
		OutsourceRoastFee:               roastFee,
		OutsourcePackagingFee:           packagingFee,
		OutsourceManualFee:              manualFee,
		OutsourceTaxFee:                 taxFee,
		OutsourceOtherFee:               otherFee,
		StockBatchDecision:              strings.TrimSpace(req.StockBatchDecision),
		BeanListPublicationID:           req.BeanListPublicationID,
		CommercialBeanListPublicationID: req.CommercialBeanListPublicationID,
		GreenBeanListPublicationID:      req.GreenBeanListPublicationID,
		DripBeanListPublicationID:       req.DripBeanListPublicationID,
		ReceiverName:                    strings.TrimSpace(req.ReceiverName),
		ReceiverPhone:                   strings.TrimSpace(req.ReceiverPhone),
		ReceiverAddress:                 strings.TrimSpace(req.ReceiverAddress),
		ReceiverCompany:                 strings.TrimSpace(req.ReceiverCompany),
		PortalServiceCode:               strings.TrimSpace(req.PortalServiceCode),
		OrdersScope:                     strings.TrimSpace(req.OrdersScope),
		Items:                           orderItemCommandsFromCreateRequest(req),
	}, nil
}

func validateCreateOrderManualPrices(req CreateOrderRequest) error {
	for i := 0; i < maxLen(req.TierID, req.UnitPrice); i++ {
		if strings.TrimSpace(getStr(req.TierID, i)) != "manual" {
			continue
		}
		price, err := strconv.ParseFloat(strings.TrimSpace(getStr(req.UnitPrice, i)), 64)
		if err != nil || price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return fmt.Errorf("手动单价必须大于0")
		}
	}
	return nil
}

func parseCreateOrderAmount(raw, field string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return v, nil
}

func orderItemCommandsFromCreateRequest(req CreateOrderRequest) []salesapp.OrderItemCommand {
	items := make([]salesapp.OrderItemCommand, 0)
	for i := 0; i < maxLen(req.ItemName, req.ItemNote, req.ProductID, req.ParentProductID, req.ItemParentProductID, req.BomSpecID, req.BomVariantID, req.CustomerProductAliasID, req.CustomerProductDisplayNameSnapshot, req.CustomerItemCodeSnapshot, req.BrandNameSnapshot, req.ProductCodeSnapshot, req.ProductNameSnapshot, req.ItemBeanListPublicationID, req.ItemBeanListVersionNo, req.PriceSourceJSON, req.TierID, req.UnitPrice, req.Qty, req.Unit, req.Spec, req.ProductKind, req.SalesUnit, req.UnitBagCount, req.UnitBeanG, req.DiscountType, req.DiscountValue); i++ {
		pidStr := strings.TrimSpace(getStr(req.ProductID, i))
		name := strings.TrimSpace(getStr(req.ItemName, i))
		if pidStr == "" && name == "" {
			continue
		}
		it := salesapp.OrderItemCommand{
			Name: name,
			// Note: strings.TrimSpace(getStr(req.ItemNote, i))
			Note:                               strings.TrimSpace(getStr(req.ItemNote, i)),
			CustomerProductDisplayNameSnapshot: strings.TrimSpace(getStr(req.CustomerProductDisplayNameSnapshot, i)),
			CustomerItemCodeSnapshot:           strings.TrimSpace(getStr(req.CustomerItemCodeSnapshot, i)),
			BrandNameSnapshot:                  strings.TrimSpace(getStr(req.BrandNameSnapshot, i)),
			ProductCodeSnapshot:                strings.TrimSpace(getStr(req.ProductCodeSnapshot, i)),
			ProductNameSnapshot:                strings.TrimSpace(getStr(req.ProductNameSnapshot, i)),
			BeanListVersionNo:                  strings.TrimSpace(getStr(req.ItemBeanListVersionNo, i)),
			PriceSourceJSON:                    strings.TrimSpace(getStr(req.PriceSourceJSON, i)),
			ProductKind:                        strings.TrimSpace(getStr(req.ProductKind, i)),
			SalesUnit:                          strings.TrimSpace(getStr(req.SalesUnit, i)),
		}
		if pidStr != "" {
			if pid, err := strconv.ParseInt(pidStr, 10, 64); err == nil && pid > 0 {
				it.ProductID = &pid
			}
		}
		parentIDStr := strings.TrimSpace(getStr(req.ItemParentProductID, i))
		if parentIDStr == "" {
			parentIDStr = strings.TrimSpace(getStr(req.ParentProductID, i))
		}
		if parentIDStr != "" {
			if parentID, err := strconv.ParseInt(parentIDStr, 10, 64); err == nil && parentID > 0 {
				it.ParentProductID = parentID
			}
		}
		if value := strings.TrimSpace(getStr(req.BomSpecID, i)); value != "" {
			if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
				it.BomSpecID = id
			}
		}
		if value := strings.TrimSpace(getStr(req.BomVariantID, i)); value != "" {
			if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
				it.BomVariantID = id
			}
		}
		if aliasStr := strings.TrimSpace(getStr(req.CustomerProductAliasID, i)); aliasStr != "" {
			if aliasID, err := strconv.ParseInt(aliasStr, 10, 64); err == nil && aliasID > 0 {
				it.CustomerProductAliasID = aliasID
			}
		}
		if publicationStr := strings.TrimSpace(getStr(req.ItemBeanListPublicationID, i)); publicationStr != "" {
			if publicationID, err := strconv.ParseInt(publicationStr, 10, 64); err == nil && publicationID > 0 {
				it.BeanListPublicationID = publicationID
			}
		}
		if tidStr := strings.TrimSpace(getStr(req.TierID, i)); tidStr != "" && tidStr != "auto" {
			if tidStr == "manual" {
				if v := strings.TrimSpace(getStr(req.UnitPrice, i)); v != "" {
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						it.ManualPrice = &f
					}
				}
			} else if tid, err := strconv.ParseInt(tidStr, 10, 64); err == nil && tid > 0 {
				it.TierID = &tid
			}
		}
		it.DiscountType = strings.TrimSpace(strings.ToLower(getStr(req.DiscountType, i)))
		if v := strings.TrimSpace(getStr(req.DiscountValue, i)); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				it.DiscountValue = f
			}
		}
		if q := strings.TrimSpace(getStr(req.Qty, i)); q != "" {
			if n, err := strconv.ParseInt(q, 10, 64); err == nil && n > 0 {
				it.Units = n
			}
		}
		if sg := strings.TrimSpace(getStr(req.Spec, i)); sg != "" {
			sg = strings.TrimSuffix(strings.ToLower(sg), "g")
			if n, err := strconv.ParseInt(sg, 10, 64); err == nil && n > 0 {
				it.SpecG = n
			}
		}
		if v := strings.TrimSpace(getStr(req.UnitBagCount, i)); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				it.UnitBagCount = n
			}
		}
		if v := strings.TrimSpace(getStr(req.UnitBeanG, i)); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				it.UnitBeanG = f
				if it.SpecG <= 0 {
					it.SpecG = int64(math.Round(f))
				}
			}
		}
		it.Unit = strings.TrimSpace(getStr(req.Unit, i))
		items = append(items, it)
	}
	return items
}

func updateHeaderCommandFromRequest(req UpdateOrderRequest, actor string) salesapp.UpdateHeaderCommand {
	return salesapp.UpdateHeaderCommand{
		Actor:                 actor,
		DocumentDate:          req.DocumentDate,
		OrderDate:             req.OrderDate,
		CustomerID:            req.CustomerID,
		SourceID:              req.SourceID,
		OrderTypeID:           req.OrderTypeID,
		PayStatusID:           req.PayStatusID,
		PaymentMethod:         strings.TrimSpace(req.PaymentMethod),
		ShipStatusID:          req.ShipStatusID,
		ShipMethod:            req.ShipMethod,
		ShipTrackingNo:        req.ShipTrackingNo,
		LogisticsCompanyID:    req.LogisticsCompanyID,
		LogisticsProductID:    req.LogisticsProductID,
		PaymentGoodsAmount:    req.PaymentGoodsAmount,
		PaymentShippingAmount: req.PaymentShippingAmount,
		PaymentVoucherAssetID: req.PaymentVoucherAssetID,
		Notes:                 req.Notes,
		ShippingAmount:        req.ShippingAmount,
		DiscountAmount:        req.DiscountAmount,
		RoundToInt:            req.RoundToInt,
		ExpressFee:            req.ExpressFee,
		OutsourceMaterialFee:  req.OutsourceMaterialFee,
		OutsourceRoastFee:     req.OutsourceRoastFee,
		OutsourcePackagingFee: req.OutsourcePackagingFee,
		OutsourceManualFee:    req.OutsourceManualFee,
		OutsourceTaxFee:       req.OutsourceTaxFee,
		OutsourceOtherFee:     req.OutsourceOtherFee,
		ItemID:                req.ItemID,
		Qty:                   req.Qty,
		UnitPrice:             req.UnitPrice,
	}
}

func inlineUpdateCommandFromRequest(req support.InlineUpdateRequest) salesapp.InlineUpdateCommand {
	return salesapp.InlineUpdateCommand{
		OrderTypeID:     req.OrderTypeID,
		PayStatusID:     req.PayStatusID,
		PaymentMethod:   strings.TrimSpace(req.PaymentMethod),
		ShipStatusID:    req.ShipStatusID,
		ProcessStatusID: req.ProcessStatusID,
		Notes:           req.Notes,
	}
}
