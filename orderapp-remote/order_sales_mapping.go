package main

import salesapp "orderapp/internal/application/sales"

func saveOrderCommandFromCreateRequest(req CreateOrderRequest, editID int64, actor string) salesapp.SaveOrderCommand {
	return salesapp.SaveOrderCommand{
		Actor:                 actor,
		EditID:                editID,
		OrderDate:             req.OrderDate,
		CustomerID:            req.CustomerID,
		SourceID:              req.SourceID,
		OrderTypeID:           req.OrderTypeID,
		PayStatusID:           req.PayStatusID,
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
		ProductID:             req.ProductID,
		TierID:                req.TierID,
		UnitPrice:             req.UnitPrice,
		ItemName:              req.ItemName,
		Qty:                   req.Qty,
		Unit:                  req.Unit,
		Spec:                  req.Spec,
	}
}

func updateHeaderCommandFromRequest(req UpdateOrderRequest, actor string) salesapp.UpdateHeaderCommand {
	return salesapp.UpdateHeaderCommand{
		Actor:                 actor,
		OrderDate:             req.OrderDate,
		CustomerID:            req.CustomerID,
		SourceID:              req.SourceID,
		OrderTypeID:           req.OrderTypeID,
		PayStatusID:           req.PayStatusID,
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

func inlineUpdateCommandFromRequest(req InlineUpdateRequest) salesapp.InlineUpdateCommand {
	return salesapp.InlineUpdateCommand{
		OrderTypeID:     req.OrderTypeID,
		PayStatusID:     req.PayStatusID,
		ShipStatusID:    req.ShipStatusID,
		ProcessStatusID: req.ProcessStatusID,
		Notes:           req.Notes,
	}
}
