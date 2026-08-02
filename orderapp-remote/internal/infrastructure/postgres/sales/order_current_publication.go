package sales

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// isCurrentDefaultOrderPublicationTx mirrors the order-form default-publication
// rules for one customer and one selected publication. SaveOrder calls it while
// holding a SHARE table lock, so a publish cannot change the default between
// validation and commit.
func isCurrentDefaultOrderPublicationTx(ctx context.Context, tx pgx.Tx, schema string, customerID, publicationID int64, listType string) (bool, error) {
	if customerID <= 0 || publicationID <= 0 || strings.TrimSpace(listType) == "" {
		return false, nil
	}
	query := fmt.Sprintf(`
		WITH selected AS (
			SELECT b.id, b.owner_type, b.owner_key, b.list_type,
			       COALESCE(NULLIF(b.classification_template_id,0), NULLIF(b.product_type_category_id,0), 0) AS group_id,
			       b.published_at
			FROM %[1]s.bean_list_publications b
			WHERE b.id=$2
			  AND b.list_type=$3
			  AND b.status='published'
			  AND b.publication_purpose='factory_supply'
		),
		eligible AS (
			SELECT s.*
			FROM selected s
			WHERE (
				s.owner_type='customer'
				AND s.owner_key=($1::bigint)::text
				AND NOT EXISTS (
					SELECT 1 FROM %[1]s.bean_list_publications newer
					WHERE newer.owner_type='customer' AND newer.owner_key=($1::bigint)::text
					  AND newer.list_type=s.list_type AND newer.status='published'
					  AND newer.publication_purpose='factory_supply'
					  AND COALESCE(NULLIF(newer.classification_template_id,0), NULLIF(newer.product_type_category_id,0), 0)=s.group_id
					  AND (
						newer.published_at > s.published_at
						OR (newer.published_at IS NOT NULL AND s.published_at IS NULL)
						OR (newer.published_at IS NOT DISTINCT FROM s.published_at AND newer.id > s.id)
					  )
				)
				AND (
					s.group_id>0 OR NOT EXISTS (
						SELECT 1 FROM %[1]s.bean_list_publications classified
						WHERE classified.owner_type='customer' AND classified.owner_key=($1::bigint)::text
						  AND classified.list_type=s.list_type AND classified.status='published'
						  AND classified.publication_purpose='factory_supply'
						  AND COALESCE(NULLIF(classified.classification_template_id,0), NULLIF(classified.product_type_category_id,0), 0)>0
					)
				)
			) OR (
				s.owner_type='official'
				AND NOT EXISTS (
					SELECT 1 FROM %[1]s.bean_list_publications newer
					WHERE newer.owner_type='official' AND newer.list_type=s.list_type
					  AND newer.status='published' AND newer.publication_purpose='factory_supply'
					  AND COALESCE(NULLIF(newer.classification_template_id,0), NULLIF(newer.product_type_category_id,0), 0)=s.group_id
					  AND (
						newer.published_at > s.published_at
						OR (newer.published_at IS NOT NULL AND s.published_at IS NULL)
						OR (newer.published_at IS NOT DISTINCT FROM s.published_at AND newer.id > s.id)
					  )
				)
				AND (
					s.group_id>0 OR NOT EXISTS (
						SELECT 1 FROM %[1]s.bean_list_publications classified
						WHERE classified.owner_type='official' AND classified.list_type=s.list_type
						  AND classified.status='published' AND classified.publication_purpose='factory_supply'
						  AND COALESCE(NULLIF(classified.classification_template_id,0), NULLIF(classified.product_type_category_id,0), 0)>0
					)
				)
				AND NOT EXISTS (
					SELECT 1 FROM %[1]s.bean_list_publications customer_version
					WHERE customer_version.owner_type='customer' AND customer_version.owner_key=($1::bigint)::text
					  AND customer_version.list_type=s.list_type
					  AND customer_version.status='published' AND customer_version.publication_purpose='factory_supply'
					  AND (
						COALESCE(NULLIF(customer_version.classification_template_id,0), NULLIF(customer_version.product_type_category_id,0), 0)=s.group_id
						OR (
							COALESCE(NULLIF(customer_version.classification_template_id,0), NULLIF(customer_version.product_type_category_id,0), 0)=0
							AND NOT EXISTS (
								SELECT 1 FROM %[1]s.bean_list_publications customer_classified
								WHERE customer_classified.owner_type='customer' AND customer_classified.owner_key=($1::bigint)::text
								  AND customer_classified.list_type=s.list_type
								  AND customer_classified.status='published' AND customer_classified.publication_purpose='factory_supply'
								  AND COALESCE(NULLIF(customer_classified.classification_template_id,0), NULLIF(customer_classified.product_type_category_id,0), 0)>0
							)
						)
					  )
				)
			)
		)
		SELECT EXISTS(SELECT 1 FROM eligible)
	`, schema)
	var current bool
	if err := tx.QueryRow(ctx, query, customerID, publicationID, strings.TrimSpace(listType)).Scan(&current); err != nil {
		return false, err
	}
	return current, nil
}
