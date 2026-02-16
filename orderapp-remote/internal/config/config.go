package config

import (
	"context"
)

// Repository persists key/value config in DB.
// Values are strings (often JSON). Keep it simple.
type Repository interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, actor, key, val string) error
	List(ctx context.Context, prefix string) (map[string]string, error)
}

// Keys used by CRM/customer module.
const (
	KeyCustomerAssetMaxBytes = "customer.asset.max_bytes" // default 2097152
)
