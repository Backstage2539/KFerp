package appmain

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func reqPrefixByTable(table string) (string, error) {
	switch table {
	case "req_product":
		return "PR-", nil
	case "req_dev":
		return "DEV-", nil
	case "req_unit":
		return "UT-", nil
	case "req_api":
		return "API-", nil
	case "req_review":
		return "REV-", nil
	default:
		return "", fmt.Errorf("unknown table: %s", table)
	}
}

func nextReqCodeForTable(ctx context.Context, pool *pgxpool.Pool, schema, table string) (string, error) {
	prefix, err := reqPrefixByTable(table)
	if err != nil {
		return "", err
	}
	// Extract numeric suffix and find max.
	// Example: PR-001 -> 1
	q := fmt.Sprintf(`SELECT COALESCE(MAX((regexp_replace(code, '^%s', ''))::int), 0)
		FROM %s.%s
		WHERE code ~ '^%s[0-9]+$'`, prefix, schema, table, prefix)
	var max int
	if err := pool.QueryRow(ctx, q).Scan(&max); err != nil {
		return "", err
	}
	next := max + 1
	// Pad to 3 digits for consistency with existing PR-001, DEV-001...
	return fmt.Sprintf("%s%03d", prefix, next), nil
}

func nextReqCodeByType(ctx context.Context, pool *pgxpool.Pool, schema, typ string) (string, error) {
	typ = strings.ToLower(strings.TrimSpace(typ))
	switch typ {
	case "product":
		return nextReqCodeForTable(ctx, pool, schema, "req_product")
	case "dev":
		return nextReqCodeForTable(ctx, pool, schema, "req_dev")
	case "unit":
		return nextReqCodeForTable(ctx, pool, schema, "req_unit")
	case "api":
		return nextReqCodeForTable(ctx, pool, schema, "req_api")
	case "review":
		return nextReqCodeForTable(ctx, pool, schema, "req_review")
	default:
		return "", fmt.Errorf("invalid type")
	}
}
