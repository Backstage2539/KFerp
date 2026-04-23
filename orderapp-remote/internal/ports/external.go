package ports

import "context"

type CRMCustomer struct {
	ID      string
	Name    string
	Phone   string
	Address string
}

type CRMClient interface {
	UpsertCustomer(ctx context.Context, customer CRMCustomer) (CRMCustomer, error)
}
