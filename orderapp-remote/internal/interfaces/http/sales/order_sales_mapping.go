package sales

import (
	"fmt"
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
	shippingAmount, err := parseCreateOrderAmount(req.ShippingAmount, "shipping_amount")
	if err != nil {
		return salesapp.SaveOrderCommand{}, err
	}
	discountAmount, err := parseCreateOrderAmount(req.DiscountAmount, "discount_amount")
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
	return salesapp.SaveOrderCommand{
		Actor:                 actor,
		EditID:                editID,
		OrderDate:             od,
		CustomerID:            req.CustomerID,
		SourceID:              req.SourceID,
		OrderTypeID:           req.OrderTypeID,
		PayStatusID:           req.PayStatusID,
		PaymentMethod:         strings.TrimSpace(req.PaymentMethod),
		ShipStatusID:          req.ShipStatusID,
		ShipMethod:            req.ShipMethod,
		ShipTrackingNo:        req.ShipTrackingNo,
		ResponsibleType:       strings.TrimSpace(req.ResponsibleType),
		ResponsibleID:         req.ResponsibleID,
		Notes:                 req.Notes,
		ShippingAmount:        shippingAmount,
		DiscountAmount:        discountAmount,
		RoundToInt:            strings.TrimSpace(req.RoundToInt) != "",
		ExpressFee:            req.ExpressFee,
		OutsourceMaterialFee:  materialFee,
		OutsourceRoastFee:     roastFee,
		OutsourcePackagingFee: packagingFee,
		OutsourceManualFee:    manualFee,
		OutsourceTaxFee:       taxFee,
		OutsourceOtherFee:     otherFee,
		StockBatchDecision:    strings.TrimSpace(req.StockBatchDecision),
		Items:                 orderItemCommandsFromCreateRequest(req),
	}, nil
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
	for i := 0; i < maxLen(req.ItemName, req.ItemNote, req.ProductID, req.TierID, req.UnitPrice, req.Qty, req.Unit, req.Spec, req.DiscountType, req.DiscountValue); i++ {
		pidStr := strings.TrimSpace(getStr(req.ProductID, i))
		name := strings.TrimSpace(getStr(req.ItemName, i))
		if pidStr == "" && name == "" {
			continue
		}
		it := salesapp.OrderItemCommand{Name: name, Note: strings.TrimSpace(getStr(req.ItemNote, i))}
		if pidStr != "" {
			if pid, err := strconv.ParseInt(pidStr, 10, 64); err == nil && pid > 0 {
				it.ProductID = &pid
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
		it.Unit = strings.TrimSpace(getStr(req.Unit, i))
		items = append(items, it)
	}
	return items
}

func updateHeaderCommandFromRequest(req UpdateOrderRequest, actor string) salesapp.UpdateHeaderCommand {
	return salesapp.UpdateHeaderCommand{
		Actor:                 actor,
		OrderDate:             req.OrderDate,
		CustomerID:            req.CustomerID,
		SourceID:              req.SourceID,
		OrderTypeID:           req.OrderTypeID,
		PayStatusID:           req.PayStatusID,
		PaymentMethod:         strings.TrimSpace(req.PaymentMethod),
		ShipStatusID:          req.ShipStatusID,
		ShipMethod:            req.ShipMethod,
		ShipTrackingNo:        req.ShipTrackingNo,
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
