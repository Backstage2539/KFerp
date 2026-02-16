package customer

import (
	"context"
)

type SearchQuery struct {
	Q      string
	Limit  int
	Offset int
	// IncludeInactive: if true, return inactive/archived records too.
	IncludeInactive bool
}

type SearchResult struct {
	Rows    []Customer
	HasNext bool
}

type Repository interface {
	Search(ctx context.Context, q SearchQuery) (SearchResult, error)
	Get(ctx context.Context, id ID) (*Customer, error)
	Upsert(ctx context.Context, actor string, c Customer) (ID, error)
	Archive(ctx context.Context, actor string, id ID) error
	Restore(ctx context.Context, actor string, id ID) error
}
