package sales

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	progresspg "orderapp/internal/infrastructure/postgres/orderprogress"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
)

func (r Repository) ListEmployeeOrderCards(ctx context.Context, q salesapp.EmployeeOrderCardsQuery) (salesapp.EmployeeOrderCardsPage, error) {
	if q.Limit < 1 || q.Limit > 50 || (q.Scope != "" && q.Scope != "mine") {
		return salesapp.EmployeeOrderCardsPage{}, fmt.Errorf("invalid employee order page")
	}
	where, args, next := orderListWhere(r.schema, salesapp.OrderListQuery{Scope: q.Scope, EmployeeID: q.EmployeeID, Q: q.Q, Void: "normal"})
	if after := q.After; after != nil {
		if after.Date == "" {
			where = append(where, fmt.Sprintf("(o.document_date IS NULL AND o.id < $%d)", next))
			args = append(args, after.ID)
		} else {
			where = append(where, fmt.Sprintf("(o.document_date IS NULL OR (o.document_date, o.id) < ($%d::date, $%d::bigint))", next, next+1))
			args = append(args, after.Date, after.ID)
		}
	}
	args = append(args, q.Limit+1)
	// Resolve proposed orders before checking ownership, searching, and sorting.
	// Only rows with a pending version need JSON expansion. Materializing the
	// page bounds the money and item calculations to the returned cards.
	query := fmt.Sprintf(`
		WITH versions AS (
			SELECT b AS content FROM %[1]s.orders b WHERE NULLIF(b.confirmation_pending, 'null'::jsonb) IS NULL
			UNION ALL
			SELECT proposed AS content FROM %[1]s.orders b
			CROSS JOIN LATERAL jsonb_populate_record(b,b.confirmation_pending->'order') proposed
			WHERE NULLIF(b.confirmation_pending, 'null'::jsonb) IS NOT NULL
		), page AS MATERIALIZED (
			SELECT v.content, COALESCE(c.name,'') customer
			FROM versions v CROSS JOIN LATERAL (SELECT (v.content).*) o
			LEFT JOIN %[1]s.customers c ON c.id=o.customer_id
			WHERE %[2]s ORDER BY o.document_date DESC NULLS LAST, o.id DESC LIMIT $%[3]d
		), card_items AS (
			SELECT i.order_id,i.qty,i.product_id,i.product_kind FROM page p
			JOIN %[1]s.order_items i ON i.order_id=(p.content).id
			WHERE NULLIF((p.content).confirmation_pending, 'null'::jsonb) IS NULL
			UNION ALL
			SELECT i.order_id,i.qty,i.product_id,i.product_kind FROM page p
			CROSS JOIN LATERAL jsonb_to_recordset(NULLIF((p.content).confirmation_pending->'items','null'::jsonb))
			AS i(order_id bigint, qty numeric, product_id bigint, product_kind text)
		)
		SELECT o.id, COALESCE(to_char(o.document_date,'YYYY-MM-DD'),''),
			COALESCE(o.order_no,''), COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''),p.customer,
			COALESCE(to_char(%[4]s,'FM999999999.00'),''),
			COALESCE(o.pay_status_id,0),
			CASE WHEN to_jsonb(o)->>'settlement_reason'='free_sample' THEN '无需收款' ELSE COALESCE(ps.name,'') END,
			COALESCE(o.ship_status_id,0),COALESCE(ss.name,''),%[5]s,
			COALESCE(to_jsonb(o)->>'delivery_mode','ship'),COALESCE(to_jsonb(o)->>'delivery_status',''),
			COALESCE(to_jsonb(o)->>'settlement_reason',''), COALESCE((to_jsonb(o)->>'prepayment_amount')::numeric,0)::text,
			COALESCE(to_jsonb(o)->>'confirmation_status','accepted'), COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false),
			(%[6]s)::float8
		FROM page p CROSS JOIN LATERAL (SELECT (p.content).*) o
		LEFT JOIN %[1]s.pay_statuses ps ON ps.id=o.pay_status_id
		LEFT JOIN %[1]s.ship_statuses ss ON ss.id=o.ship_status_id
		LEFT JOIN %[1]s.order_process_statuses ops ON ops.id=o.process_status_id
		ORDER BY o.document_date DESC NULLS LAST, o.id DESC`,
		r.schema, strings.Join(where, " AND "), len(args), orderReceivableSQL("o"), orderProcessStatusExprForItems(r.schema, "card_items"), resolutionpg.SettledAmountSQL("o", "ps.name"))
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return salesapp.EmployeeOrderCardsPage{}, err
	}
	defer rows.Close()
	page := salesapp.EmployeeOrderCardsPage{Rows: make([]salesapp.EmployeeOrderCard, 0, q.Limit)}
	ids := make([]int64, 0, q.Limit)
	var tail salesapp.EmployeeOrderCursor
	for rows.Next() {
		if len(page.Rows) == q.Limit {
			page.Next = &tail
			break
		}
		var card salesapp.EmployeeOrderCard
		var settled float64
		if err := rows.Scan(&card.ID, &tail.Date, &card.OrderNo, &card.OrderDate, &card.Customer, &card.GrandTotal, &card.PayStatusID, &card.PayStatus, &card.ShipStatusID, &card.ShipStatus, &card.ProcessStatus, &card.DeliveryMode, &card.DeliveryStatus, &card.SettlementReason, &card.PrepaymentAmount, &card.ConfirmationStatus, &card.ConfirmationRequired, &settled); err != nil {
			return salesapp.EmployeeOrderCardsPage{}, err
		}
		total, _ := strconv.ParseFloat(card.GrandTotal, 64)
		card.PaidAmount, card.UnpaidAmount = salesdomain.OrderPaymentAmounts("", settled, total)
		tail.ID = card.ID
		page.Rows = append(page.Rows, card)
		ids = append(ids, card.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return salesapp.EmployeeOrderCardsPage{}, err
	}
	progress, err := progresspg.Load(ctx, r.pool, r.schema, ids)
	if err != nil {
		return salesapp.EmployeeOrderCardsPage{}, err
	}
	for i := range page.Rows {
		page.Rows[i].FulfillmentProgress = progress[page.Rows[i].ID]
	}
	return page, nil
}
