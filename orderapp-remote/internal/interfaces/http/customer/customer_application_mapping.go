package customer

import customerapp "orderapp/internal/application/customer"

func customerUpsertCommandFromRequest(req CustomerUpsertRequest) customerapp.UpsertCommand {
	return customerapp.UpsertCommand{
		Name:               req.Name,
		RawName:            req.RawName,
		CustomerType:       req.CustomerType,
		CompanyName:        req.CompanyName,
		CompanyAddress:     req.CompanyAddress,
		CompanyPhone:       req.CompanyPhone,
		Contact:            req.Contact,
		Phone:              req.Phone,
		Address:            req.Address,
		DefaultSourceID:    req.DefaultSourceID,
		DefaultOrderTypeID: req.DefaultOrderTypeID,
		Active:             req.Active,
	}
}

func customerInlineCommandFromRequest(req CustomerInlineReq) customerapp.InlineUpdateCommand {
	return customerapp.InlineUpdateCommand{
		Name:               req.Name,
		CustomerType:       req.CustomerType,
		CompanyName:        req.CompanyName,
		CompanyAddress:     req.CompanyAddress,
		CompanyPhone:       req.CompanyPhone,
		Contact:            req.Contact,
		Phone:              req.Phone,
		Address:            req.Address,
		DefaultSourceID:    req.DefaultSourceID,
		DefaultOrderTypeID: req.DefaultOrderTypeID,
		Active:             req.Active,
	}
}
