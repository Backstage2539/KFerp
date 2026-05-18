package orderbeans

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	ListTypeCommercial = "commercial"
	ListTypeRetail     = "retail"
)

type Usage struct {
	PublicationID int64
	VersionNo     string
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func ListTypeForRetail(retail bool) string {
	if retail {
		return ListTypeRetail
	}
	return ListTypeCommercial
}

func ResolveUsage(ctx context.Context, q rowQuerier, schema string, customerID int64, productID int64, listType string) (Usage, error) {
	if q == nil || strings.TrimSpace(schema) == "" || productID <= 0 {
		return Usage{}, nil
	}
	listType = strings.TrimSpace(listType)
	if listType == "" {
		listType = ListTypeCommercial
	}
	customerKey := ""
	if customerID > 0 {
		customerKey = fmt.Sprintf("%d", customerID)
	}

	var usage Usage
	sql := fmt.Sprintf(`
		SELECT id, COALESCE(version_no,'')
		FROM %s.bean_list_publications blp
		WHERE blp.status='published'
		  AND blp.list_type=$1
		  AND (
		    ($2 <> '' AND blp.owner_type='customer' AND blp.owner_key=$2)
		    OR blp.owner_type='official'
		  )
		  AND EXISTS (
		    SELECT 1
		    FROM jsonb_array_elements(COALESCE(blp.content_json->'groups', '[]'::jsonb)) AS groups(group_json)
		    CROSS JOIN LATERAL jsonb_array_elements(COALESCE(groups.group_json->'items', '[]'::jsonb)) AS items(item_json)
		    WHERE (
		      (items.item_json->>'productId' ~ '^[0-9]+$' AND (items.item_json->>'productId')::bigint=$3)
		      OR (items.item_json->>'product_id' ~ '^[0-9]+$' AND (items.item_json->>'product_id')::bigint=$3)
		      OR (items.item_json->>'productID' ~ '^[0-9]+$' AND (items.item_json->>'productID')::bigint=$3)
		    )
		  )
		ORDER BY CASE WHEN $2 <> '' AND blp.owner_type='customer' AND blp.owner_key=$2 THEN 0 ELSE 1 END,
		         blp.published_at DESC,
		         blp.id DESC
		LIMIT 1
	`, schema)
	if err := q.QueryRow(ctx, sql, listType, customerKey, productID).Scan(&usage.PublicationID, &usage.VersionNo); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isMissingBeanListSchema(err) {
			return Usage{}, nil
		}
		return Usage{}, err
	}
	return usage, nil
}

func isMissingBeanListSchema(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "42P01" || pgErr.Code == "42703"
}
