package sales

import (
	"context"
	"fmt"
	"sort"
	"strings"

	salesapp "orderapp/internal/application/sales"

	"github.com/jackc/pgx/v5"
)

func validateCustomerOrderPriceBindingsTx(ctx context.Context, tx pgx.Tx, schema string, cmd salesapp.SaveOrderCommand) error {
	usageCode := strings.TrimSpace(cmd.PortalServiceCode)
	if usageCode != "direct_ship" && usageCode != "product_order" {
		return fmt.Errorf("无效录单能力")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.customer_order_price_table_bindings IN SHARE MODE", schema)); err != nil {
		return err
	}
	var enabled bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT enabled FROM %s.customer_service_capabilities WHERE customer_id=$1 AND capability_code=$2 FOR SHARE`, schema), cmd.CustomerID, usageCode).Scan(&enabled); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("录单能力未开通")
		}
		return err
	}
	if !enabled {
		return fmt.Errorf("录单能力未开通")
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT x.publication_id
		FROM %s.customer_order_price_table_bindings x
		JOIN %s.bean_list_publications b ON b.id=x.publication_id
		WHERE x.customer_id=$1 AND x.usage_code=$2
		  AND b.owner_type='customer' AND b.owner_key=($1::bigint)::text
		  AND b.publication_purpose='factory_supply' AND b.status='published' AND b.deleted_at IS NULL
		ORDER BY x.publication_id
	`, schema, schema), cmd.CustomerID, usageCode)
	if err != nil {
		return err
	}
	bound := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		bound = append(bound, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	selected := uniquePositiveOrderPublicationIDs(cmd.SelectedPriceTableIDs)
	if len(bound) == 0 || len(selected) != len(bound) {
		return salesapp.NewOrderEditConflictError("价格表指定已变化，请重新核价确认")
	}
	for index := range bound {
		if bound[index] != selected[index] {
			return salesapp.NewOrderEditConflictError("价格表指定已变化，请重新核价确认")
		}
	}
	return nil
}

func uniquePositiveOrderPublicationIDs(values []int64) []int64 {
	seen := map[int64]bool{}
	out := []int64{}
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
