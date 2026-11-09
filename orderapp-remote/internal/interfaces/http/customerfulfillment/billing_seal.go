package customerfulfillment

import (
	"context"
	"orderapp/internal/infrastructure/billingseal"
)

// Read the configured seal only on the server.
func (a api) readBillingSeal(ctx context.Context, id int64) ([]byte, error) {
	return billingseal.Read(ctx, a.sales, a.assetDir, id)
}
