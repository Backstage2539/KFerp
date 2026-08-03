package sales

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func currentOrderEditRevisionTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64) (string, error) {
	var revision string
	query := fmt.Sprintf(`
		SELECT md5(to_jsonb(o)::text || '|' || COALESCE((
			SELECT jsonb_agg(to_jsonb(revision_item) ORDER BY revision_item.id)::text
			FROM %[1]s.order_items revision_item
			WHERE revision_item.order_id=o.id
		), '[]'))
		FROM %[1]s.orders o
		WHERE o.id=$1
	`, schema)
	if err := tx.QueryRow(ctx, query, orderID).Scan(&revision); err != nil {
		return "", err
	}
	return revision, nil
}
