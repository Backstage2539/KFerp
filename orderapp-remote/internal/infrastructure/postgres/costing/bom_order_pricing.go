package costing

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

type costingReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r Repository) readDB(ctx context.Context) costingReader {
	if tx, ok := postgresinfra.TransactionFromContext(ctx); ok {
		return tx
	}
	return r.pool
}

// A published row owns a server-loaded executable rule. Client rule snapshots
// are discarded, including snapshots carried over from copied publications.
func (r Repository) freezeBOMPricingRules(ctx context.Context, tx pgx.Tx, content map[string]any) error {
	raw, err := json.Marshal(content["price_rows"])
	if err != nil {
		return err
	}
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) != nil {
		return nil
	}
	for _, row := range rows {
		delete(row, "pricing_rule_snapshot")
		id := int64(0)
		for _, key := range []string{"tier_pricing_rule_id", "pricing_rule_id"} {
			if n, ok := row[key].(float64); ok && n > 0 {
				id = int64(n)
				break
			}
		}
		if id <= 0 {
			continue
		}
		rule, e := r.LoadProductPricingRule(postgresinfra.WithTransaction(ctx, tx), id)
		if e != nil {
			return fmt.Errorf("价格档位的计算模板不可用，请刷新价格表: %w", e)
		}
		if !rule.Active {
			return fmt.Errorf("价格档位的计算模板已停用")
		}
		row["pricing_rule_snapshot"] = rule
	}
	if len(rows) > 0 {
		content["price_rows"] = rows
	}
	return nil
}
