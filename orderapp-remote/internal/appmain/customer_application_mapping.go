package appmain

import customerapp "orderapp/internal/application/customer"

func customerUpsertCommandFromRequest(req CustomerUpsertRequest) customerapp.UpsertCommand {
	return customerapp.UpsertCommand{
		Name:               req.Name,
		RawName:            req.RawName,
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
		Contact:            req.Contact,
		Phone:              req.Phone,
		Address:            req.Address,
		DefaultSourceID:    req.DefaultSourceID,
		DefaultOrderTypeID: req.DefaultOrderTypeID,
		Active:             req.Active,
	}
}
