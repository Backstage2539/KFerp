package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type sampleOrderQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// sampleOrderLinked uses persisted workflow ownership, never a client-supplied flag.
// Older test fixtures and installations without the optional workflow remain valid.
func sampleOrderLinked(ctx context.Context, q sampleOrderQuerier, schema string, id int64) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	var exists bool
	if err := q.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", schema+".sample_work_orders").Scan(&exists); err != nil || !exists {
		return false, err
	}
	var linked bool
	err := q.QueryRow(ctx, fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s.sample_work_orders WHERE order_id=$1)", schema), id).Scan(&linked)
	return linked, err
}

// Only an existing manually-priced line may keep its sample price outside the
// ordinary catalogue. New lines and changed identities still use normal guards.
func sampleOrderItemMatches(ctx context.Context, q sampleOrderQuerier, schema string, orderID, itemID, productID, specID, variantID, aliasID, referenceID, publicationID int64) (bool, error) {
	if itemID <= 0 {
		return false, nil
	}
	var matches bool
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.order_items
		WHERE order_id=$1 AND id=$2 AND product_id=$3
		AND COALESCE(bom_spec_id,0)=$4 AND COALESCE(bom_variant_id,0)=$5
		AND COALESCE(customer_product_alias_id,0)=$6
		AND COALESCE(customer_product_reference_id,0)=$7
		AND COALESCE(bean_list_publication_id,0)=$8 AND price_overridden)`, schema),
		orderID, itemID, productID, specID, variantID, aliasID, referenceID, publicationID).Scan(&matches)
	return matches, err
}
