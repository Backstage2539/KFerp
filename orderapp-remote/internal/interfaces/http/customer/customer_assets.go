package customer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerAsset struct {
	ID          int64
	CustomerID  int64
	Kind        string
	ObjectKey   string
	ContentType string
	Bytes       int64
	Sha256      string
	CreatedAt   string
}

// fetchCustomerAssets returns all assets for a customer.
func fetchCustomerAssets(ctx context.Context, pool *pgxpool.Pool, schema string, customerID int64) ([]CustomerAsset, error) {
	q := fmt.Sprintf(`
		SELECT id, customer_id, kind, object_key, content_type, bytes, sha256,
			to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_assets
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
	`, schema)
	rows, err := pool.Query(ctx, q, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CustomerAsset, 0)
	for rows.Next() {
		var a CustomerAsset
		if err := rows.Scan(&a.ID, &a.CustomerID, &a.Kind, &a.ObjectKey, &a.ContentType, &a.Bytes, &a.Sha256, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
