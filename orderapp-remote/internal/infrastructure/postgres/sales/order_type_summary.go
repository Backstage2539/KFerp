package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	salesapp "orderapp/internal/application/sales"
)

// One batch for the visible page, including every item (not just its first kind).
// No active filter: historical orders may reference archived products/categories.
func populateOrderTypeSummaries(ctx context.Context, pool *pgxpool.Pool, schema string, orders []salesapp.OrderRow) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]int64, len(orders))
	index := make(map[int64]int, len(orders))
	for i := range orders {
		ids[i] = orders[i].ID
		index[orders[i].ID] = i
		orders[i].ProductTypeLabels = []string{}
	}
	var samplesExist bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", schema+".sample_work_orders").Scan(&samplesExist); err != nil {
		return err
	}
	sampleExpr := "false"
	if samplesExist {
		sampleExpr = fmt.Sprintf("EXISTS(SELECT 1 FROM %s.sample_work_orders w WHERE w.order_id=o.id)", schema)
	}
	query := fmt.Sprintf(`SELECT o.id, ARRAY(
   SELECT label FROM (
     SELECT COALESCE(NULLIF(btrim(root.name),''),NULLIF(btrim(cat.name),''),
       CASE btrim(COALESCE(NULLIF(i.product_kind,''),p.product_kind,''))
        WHEN 'roasted' THEN '熟豆' WHEN 'roasted_bean' THEN '熟豆'
        WHEN 'green_bean' THEN '生豆' WHEN 'drip_bag' THEN '挂耳'
        WHEN 'instant' THEN '速溶咖啡' WHEN 'instant_coffee' THEN '速溶咖啡'
        ELSE '未分类' END) AS label,
       min(COALESCE(i.line_no,0)) AS position
     FROM %[1]s.order_items i
     LEFT JOIN %[1]s.products p ON p.id=i.product_id
     LEFT JOIN %[1]s.products parent ON parent.id=COALESCE((to_jsonb(p)->>'parent_product_id')::bigint,0)
     LEFT JOIN %[1]s.product_categories cat ON cat.id=COALESCE(NULLIF((to_jsonb(p)->>'product_category_id')::bigint,0),(to_jsonb(parent)->>'product_category_id')::bigint)
     LEFT JOIN %[1]s.product_categories root ON root.id=cat.parent_id
     WHERE i.order_id=o.id
     GROUP BY label
   ) types ORDER BY position,label
 ), %[2]s FROM %[1]s.orders o WHERE o.id=ANY($1)`, schema, sampleExpr)
	rows, err := pool.Query(ctx, query, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var labels []string
		var sample bool
		if err := rows.Scan(&id, &labels, &sample); err != nil {
			return err
		}
		i := index[id]
		orders[i].ProductTypeLabels = labels
		orders[i].IsSampleOrder = sample
	}
	return rows.Err()
}
