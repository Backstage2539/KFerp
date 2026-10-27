package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	app "orderapp/internal/application/customerfulfillment"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

type billingRows interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (r *Repository) ListUnbilledCharges(ctx context.Context, query app.BillingQuery) (app.BillingPage, error) {
	rows, err := r.loadUnbilledCharges(ctx, r.pool, query)
	if err != nil {
		return app.BillingPage{}, err
	}
	return pageBillingCharges(rows, query.Page, query.Limit), nil
}

func (r *Repository) loadUnbilledCharges(ctx context.Context, q billingRows, query app.BillingQuery) ([]app.BillingCharge, error) {
	from, to := query.DateFrom, query.DateTo
	if from == "" {
		from = "1900-01-01"
	}
	if to == "" {
		to = "2999-12-31"
	}
	orderLines, err := customerSettlementOrderChargesTx(ctx, q, r.schema, query.CustomerID, from, to, false, 0)
	if err != nil {
		return nil, err
	}
	billedOrders, err := r.issuedOrderGoodsSources(ctx, q, query.CustomerID)
	if err != nil {
		return nil, err
	}
	out := make([]app.BillingCharge, 0, len(orderLines))
	for _, line := range orderLines {
		if line.ChargeType != "order_goods" || line.DueCents <= 0 || billedOrders[line.SourceID] || query.FeeType != "" && query.FeeType != line.ChargeType {
			continue
		}
		out = append(out, app.BillingCharge{ID: billingSourceID(line.ChargeType, line.SourceType, line.SourceID), BillingSource: app.BillingSource{ChargeType: line.ChargeType, SourceType: line.SourceType, SourceID: line.SourceID}, SourceNo: line.SourceNo, Description: line.Description, OccurredAt: line.OccurredAt, ReceivableCents: line.ReceivableCents, ReceivedCents: line.ReceivedCents, DueCents: line.DueCents, ConfirmationStatus: line.ConfirmationStatus, SettlementStatus: "unsettled"})
	}
	fees, err := q.Query(ctx, fmt.Sprintf(`
		SELECT f.id,coalesce(f.source_type,''),coalesce(f.source_id,0),f.fee_type,coalesce(f.note,''),
		       round(f.amount*100)::bigint,to_char(f.occurred_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD'),
		       COALESCE(o.order_no,''),CASE WHEN f.fee_type='shipping' AND f.source_type<>'freight_actual' THEN 'pending_billing' ELSE 'confirmed' END,
		       CASE WHEN b.status='draft' THEN 'draft' ELSE 'unsettled' END
		FROM %s.customer_fee_items f
		LEFT JOIN %s.orders o ON f.source_type='order' AND o.id=f.source_id AND o.customer_id=f.customer_id
		LEFT JOIN %s.customer_settlement_batches b ON b.id=f.settlement_batch_id AND b.customer_id=f.customer_id
		WHERE f.customer_id=$1 AND f.fee_type<>'order_goods'
		  AND (f.status='unsettled' OR (f.status='settled' AND b.status='draft' AND COALESCE(b.processing_billing_run_id,0)=0))
		  AND (f.occurred_at AT TIME ZONE 'Asia/Shanghai')::date BETWEEN $2::date AND $3::date
		  AND ($4='' OR f.fee_type=$4)
		  AND NOT (f.fee_type='shipping' AND f.source_type<>'freight_actual')
		ORDER BY f.occurred_at DESC,f.id DESC
	`, r.schema, r.schema, r.schema), query.CustomerID, from, to, query.FeeType)
	if err != nil {
		return nil, err
	}
	for fees.Next() {
		var row app.BillingCharge
		var feeID int64
		if err := fees.Scan(&feeID, &row.SourceType, &row.SourceID, &row.ChargeType, &row.Description, &row.DueCents, &row.OccurredAt, &row.SourceNo, &row.ConfirmationStatus, &row.SettlementStatus); err != nil {
			fees.Close()
			return nil, err
		}
		if row.DueCents <= 0 {
			continue
		}
		row.ID = billingSourceID(row.ChargeType, "customer_fee_item", feeID)
		row.SourceType, row.SourceID = "customer_fee_item", feeID
		row.ReceivableCents, row.DueCents = row.DueCents, row.DueCents
		out = append(out, row)
	}
	err = fees.Err()
	fees.Close()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].OccurredAt == out[j].OccurredAt {
			return out[i].ID > out[j].ID
		}
		return out[i].OccurredAt > out[j].OccurredAt
	})
	return out, nil
}

func (r *Repository) issuedOrderGoodsSources(ctx context.Context, q billingRows, customerID int64) (map[int64]bool, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT l.source_id
		FROM %s.customer_settlement_lines l
		JOIN %s.customer_settlement_batches b ON b.id=l.settlement_batch_id AND b.customer_id=l.customer_id
		WHERE l.customer_id=$1 AND l.charge_type='order_goods' AND l.source_type='order'
		  AND l.reversed_at IS NULL AND b.status IN('confirmed','paid','settled')
		  AND COALESCE(b.processing_billing_run_id,0)=0
	`, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func billingSourceID(chargeType, sourceType string, sourceID int64) string {
	return fmt.Sprintf("%s:%s:%d", chargeType, sourceType, sourceID)
}

func pageBillingCharges(rows []app.BillingCharge, page, limit int) app.BillingPage {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	start := (page - 1) * limit
	end := start + limit
	if start > len(rows) {
		start = len(rows)
	}
	if end > len(rows) {
		end = len(rows)
	}
	return app.BillingPage{Rows: rows[start:end], Total: len(rows), Page: page, Limit: limit}
}

func (r *Repository) BillingSummary(ctx context.Context, customerID int64) (app.BillingTotals, error) {
	charges, err := r.loadUnbilledCharges(ctx, r.pool, app.BillingQuery{CustomerID: customerID, Page: 1, Limit: 200})
	if err != nil {
		return app.BillingTotals{}, err
	}
	totals := app.BillingTotals{}
	for _, row := range charges {
		totals.UnbilledCents += row.DueCents
	}
	var ids []int64
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_settlement_batches WHERE customer_id=$1 AND status IN('confirmed','paid','settled') AND COALESCE(processing_billing_run_id,0)=0`, r.schema), customerID)
	if err != nil {
		return totals, err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return totals, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return totals, err
	}
	for _, id := range ids {
		bill, err := r.GetIssuedBill(ctx, customerID, id)
		if err != nil {
			return totals, err
		}
		totals.IssuedCents += bill.DueAmountCents
	}
	totals.TotalUnpaidCents = totals.UnbilledCents + totals.IssuedCents
	return totals, nil
}

func (r *Repository) ListIssuedBills(ctx context.Context, query app.BillingQuery) (app.BillingBillsPage, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_settlement_batches WHERE customer_id=$1 AND status IN('confirmed','paid','settled') AND COALESCE(processing_billing_run_id,0)=0 ORDER BY COALESCE(confirmed_at,created_at) DESC,id DESC`, r.schema), query.CustomerID)
	if err != nil {
		return app.BillingBillsPage{}, err
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return app.BillingBillsPage{}, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return app.BillingBillsPage{}, err
	}
	out := make([]app.BillingBill, 0, len(ids))
	for _, id := range ids {
		bill, err := r.GetIssuedBill(ctx, query.CustomerID, id)
		if err != nil {
			return app.BillingBillsPage{}, err
		}
		if query.Status == "unpaid" && bill.DueAmountCents == 0 || query.Status == "paid" && bill.DueAmountCents > 0 {
			continue
		}
		out = append(out, bill)
	}
	page, limit := query.Page, query.Limit
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	start := (page - 1) * limit
	end := start + limit
	if start > len(out) {
		start = len(out)
	}
	if end > len(out) {
		end = len(out)
	}
	return app.BillingBillsPage{Rows: out[start:end], Total: len(out), Page: page, Limit: limit}, nil
}

func (r *Repository) GetIssuedBill(ctx context.Context, customerID, batchID int64) (app.BillingBill, error) {
	var out app.BillingBill
	var total int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT b.settlement_no,COALESCE(to_char(b.period_from,'YYYY-MM-DD'),''),COALESCE(to_char(b.period_to,'YYYY-MM-DD'),''),b.status,round(b.total_amount*100)::bigint,COALESCE(to_char(b.confirmed_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),''),COALESCE(c.name,'') FROM %s.customer_settlement_batches b LEFT JOIN %s.customers c ON c.id=b.customer_id WHERE b.id=$1 AND b.customer_id=$2 AND b.status IN('confirmed','paid','settled') AND COALESCE(b.processing_billing_run_id,0)=0`, r.schema, r.schema), batchID, customerID).Scan(&out.SettlementNo, &out.PeriodFrom, &out.PeriodTo, &out.Status, &total, &out.IssuedAt, &out.CustomerName)
	if err != nil {
		return out, err
	}
	lines, err := listSettlementLines(ctx, r.pool, r.schema, customerID, batchID)
	if err != nil {
		return out, err
	}
	out.CustomerID = customerID
	out.BatchID = batchID
	out.TotalAmountCents = total
	out.Lines = lines
	out.FeeItems = len(lines)
	out.DueAmountCents = total
	for i := range out.Lines {
		line := &out.Lines[i]
		out.ReceivableCents += line.ReceivableCents
		out.ChargeTypes = appendChargeType(out.ChargeTypes, line.ChargeType)
		line.IssuedReceivedCents = line.ReceivedCents
		currentReceived := int64(0)
		if line.ChargeType == "order_goods" && line.SourceType == "order" {
			current, err := customerSettlementOrderChargesTx(ctx, r.pool, r.schema, customerID, "1900-01-01", "2999-12-31", true, 0)
			if err != nil {
				return out, err
			}
			for _, now := range current {
				if now.ChargeType == line.ChargeType && now.SourceID == line.SourceID {
					currentReceived = max(int64(0), now.ReceivedCents-line.ReceivedCents)
					break
				}
			}
		} else if out.Status == "paid" {
			currentReceived = line.AmountCents
		}
		if currentReceived > line.AmountCents {
			currentReceived = line.AmountCents
		}
		line.ReceivedCents = currentReceived
		line.DueCents = max(int64(0), line.AmountCents-currentReceived)
		out.ReceivedCents += currentReceived
		out.DueAmountCents -= currentReceived
	}
	if out.DueAmountCents < 0 {
		out.DueAmountCents = 0
	}
	return out, nil
}

func appendChargeType(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}

func (r *Repository) PreviewBilling(ctx context.Context, cmd app.BillingPreviewCommand) (app.BillingPreview, error) {
	charges, err := r.loadUnbilledCharges(ctx, r.pool, app.BillingQuery{CustomerID: cmd.CustomerID, Page: 1, Limit: 200})
	if err != nil {
		return app.BillingPreview{}, err
	}
	byKey := make(map[string]app.BillingCharge, len(charges))
	for _, v := range charges {
		byKey[v.ID] = v
	}
	preview := app.BillingPreview{CustomerID: cmd.CustomerID, Lines: make([]app.SettlementLineSummary, 0, len(cmd.Sources))}
	for _, source := range cmd.Sources {
		id := billingSourceID(source.ChargeType, source.SourceType, source.SourceID)
		row, ok := byKey[id]
		if !ok {
			return app.BillingPreview{}, fmt.Errorf("billing source changed or unavailable: %s", id)
		}
		preview.Lines = append(preview.Lines, app.SettlementLineSummary{ChargeType: row.ChargeType, SourceType: row.SourceType, SourceID: row.SourceID, SourceNo: row.SourceNo, Description: row.Description, OccurredAt: row.OccurredAt, ReceivableCents: row.ReceivableCents, AmountCents: row.DueCents, ReceivedCents: row.ReceivedCents, IssuedReceivedCents: row.ReceivedCents, DueCents: row.DueCents, ConfirmationStatus: row.ConfirmationStatus, SettlementStatus: row.SettlementStatus})
		preview.TotalAmountCents += row.DueCents
		if preview.PeriodFrom == "" || row.OccurredAt < preview.PeriodFrom {
			preview.PeriodFrom = row.OccurredAt
		}
		if preview.PeriodTo == "" || row.OccurredAt > preview.PeriodTo {
			preview.PeriodTo = row.OccurredAt
		}
	}
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, r.schema), cmd.CustomerID).Scan(&preview.CustomerName); err != nil {
		return app.BillingPreview{}, err
	}
	preview.FeeItems = len(preview.Lines)
	preview.PreviewVersion = billingPreviewVersion(preview)
	return preview, nil
}

func billingPreviewVersion(p app.BillingPreview) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%s|%d\n", p.CustomerID, p.PeriodFrom, p.PeriodTo, p.TotalAmountCents)
	for _, l := range p.Lines {
		fmt.Fprintf(h, "%s|%s|%d|%s|%s|%s|%d|%d|%d\n", l.ChargeType, l.SourceType, l.SourceID, l.OccurredAt, l.SourceNo, l.Description, l.ReceivableCents, l.ReceivedCents, l.DueCents)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (r *Repository) IssueBilling(ctx context.Context, cmd app.IssueBillingCommand) (app.BillingBill, error) {
	if err := r.requireCustomerCapability(ctx, cmd.CustomerID, "settlement"); err != nil {
		return app.BillingBill{}, err
	}
	if existing, err := r.findIdempotentBill(ctx, cmd.CustomerID, cmd.IdempotencyKey); err == nil {
		return existing, nil
	} else if !errorsIsNoRows(err) {
		return app.BillingBill{}, err
	}
	preview, err := r.PreviewBilling(ctx, cmd.BillingPreviewCommand)
	if err != nil {
		return app.BillingBill{}, err
	}
	if preview.PreviewVersion != cmd.PreviewVersion {
		return app.BillingBill{}, fmt.Errorf("billing preview changed; review again")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.BillingBill{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext('customer_settlement'))`, fmt.Sprint(cmd.CustomerID)); err != nil {
		return app.BillingBill{}, err
	}
	if existing, e := r.findIdempotentBillTx(ctx, tx, cmd.CustomerID, cmd.IdempotencyKey); e == nil {
		return existing, tx.Commit(ctx)
	} else if !errorsIsNoRows(e) {
		return app.BillingBill{}, e
	}
	current, err := r.previewBillingTx(ctx, tx, cmd.BillingPreviewCommand)
	if err != nil {
		return app.BillingBill{}, err
	}
	if current.PreviewVersion != cmd.PreviewVersion {
		return app.BillingBill{}, fmt.Errorf("billing preview changed; review again")
	}
	if err := r.lockBillingSourcesTx(ctx, tx, cmd.CustomerID, cmd.Sources); err != nil {
		return app.BillingBill{}, err
	}
	current, err = r.previewBillingTx(ctx, tx, cmd.BillingPreviewCommand)
	if err != nil {
		return app.BillingBill{}, err
	}
	if current.PreviewVersion != cmd.PreviewVersion {
		return app.BillingBill{}, fmt.Errorf("billing preview changed; review again")
	}
	keyHash := sha256.Sum256([]byte(cmd.IdempotencyKey))
	billNo := fmt.Sprintf("CB-%d-%s", cmd.CustomerID, hex.EncodeToString(keyHash[:6]))
	var batchID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_batches(customer_id,settlement_no,period_from,period_to,status,total_amount,confirmed_at,created_by) VALUES($1,$2,$3,$4,'confirmed',$5,now(),$6) RETURNING id`, r.schema), cmd.CustomerID, billNo, current.PeriodFrom, current.PeriodTo, float64(current.TotalAmountCents)/100, cmd.Actor).Scan(&batchID); err != nil {
		return app.BillingBill{}, err
	}
	for _, line := range current.Lines {
		if line.SourceType == "customer_fee_item" {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_lines SET reversed_at=now(),settlement_status='draft_archived' WHERE customer_id=$1 AND charge_type=$2 AND source_type='customer_fee_item' AND source_id=$3 AND reversed_at IS NULL`, r.schema), cmd.CustomerID, line.ChargeType, line.SourceID); err != nil {
				return app.BillingBill{}, err
			}
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fee_items SET settlement_batch_id=0,status='unsettled' WHERE id=$1 AND customer_id=$2 AND settlement_batch_id IN(SELECT id FROM %s.customer_settlement_batches WHERE status='draft' AND COALESCE(processing_billing_run_id,0)=0)`, r.schema, r.schema), line.SourceID, cmd.CustomerID); err != nil {
				return app.BillingBill{}, err
			}
		} else {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_lines SET reversed_at=now(),settlement_status='draft_archived' WHERE customer_id=$1 AND charge_type=$2 AND source_type=$3 AND source_id=$4 AND reversed_at IS NULL AND settlement_batch_id IN(SELECT id FROM %s.customer_settlement_batches WHERE status='draft' AND COALESCE(processing_billing_run_id,0)=0)`, r.schema, r.schema), cmd.CustomerID, line.ChargeType, line.SourceType, line.SourceID); err != nil {
				return app.BillingBill{}, err
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_lines(customer_id,settlement_batch_id,charge_type,source_type,source_id,source_no,description,occurred_at,receivable_cents,amount_cents,received_cents,due_cents,confirmation_status,settlement_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8::date,$9,$10,$11,$12,$13,'confirmed')`, r.schema), cmd.CustomerID, batchID, line.ChargeType, line.SourceType, line.SourceID, line.SourceNo, line.Description, line.OccurredAt, line.ReceivableCents, line.AmountCents, line.ReceivedCents, line.DueCents, line.ConfirmationStatus); err != nil {
			return app.BillingBill{}, err
		}
		if line.SourceType == "customer_fee_item" {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fee_items SET settlement_batch_id=$2,status='settled' WHERE id=$1 AND customer_id=$3`, r.schema), line.SourceID, batchID, cmd.CustomerID); err != nil {
				return app.BillingBill{}, err
			}
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_batches b SET status='reversed' WHERE b.customer_id=$1 AND b.status='draft' AND COALESCE(b.processing_billing_run_id,0)=0 AND NOT EXISTS(SELECT 1 FROM %s.customer_settlement_lines l WHERE l.settlement_batch_id=b.id AND l.reversed_at IS NULL)`, r.schema, r.schema), cmd.CustomerID); err != nil {
		return app.BillingBill{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_issue_keys(customer_id,idempotency_key,batch_id) VALUES($1,$2,$3)`, r.schema), cmd.CustomerID, cmd.IdempotencyKey, batchID); err != nil {
		return app.BillingBill{}, err
	}
	if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_fulfillment_settlement", &batchID, "issue", nil, nil, postgresinfra.StrPtr(billNo), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "settlement_no": billNo, "line_count": len(current.Lines), "total_amount_cents": current.TotalAmountCents, "preview_version": cmd.PreviewVersion, "idempotency_key": cmd.IdempotencyKey}); err != nil {
		return app.BillingBill{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return app.BillingBill{}, err
	}
	return r.GetIssuedBill(ctx, cmd.CustomerID, batchID)
}

func (r *Repository) lockBillingSourcesTx(ctx context.Context, tx pgx.Tx, customerID int64, sources []app.BillingSource) error {
	for _, source := range sources {
		if source.SourceType == "order" {
			var id int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=$1 AND customer_id=$2 AND COALESCE(is_void,false)=false FOR UPDATE`, r.schema), source.SourceID, customerID).Scan(&id); err != nil {
				return fmt.Errorf("billing source changed; review again")
			}
			continue
		}
		if source.SourceType != "customer_fee_item" {
			return fmt.Errorf("billing source invalid")
		}
		var amount int64
		var status, batchStatus string
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT round(f.amount*100)::bigint,f.status,COALESCE(b.status,'') FROM %s.customer_fee_items f LEFT JOIN %s.customer_settlement_batches b ON b.id=f.settlement_batch_id AND b.customer_id=f.customer_id WHERE f.id=$1 AND f.customer_id=$2 FOR UPDATE OF f`, r.schema, r.schema), source.SourceID, customerID).Scan(&amount, &status, &batchStatus)
		if err != nil || !(status == "unsettled" || status == "settled" && batchStatus == "draft") {
			return fmt.Errorf("billing source changed; review again")
		}
		if amount <= 0 {
			return fmt.Errorf("billing source changed; review again")
		}
	}
	return nil
}

func (r *Repository) previewBillingTx(ctx context.Context, tx pgx.Tx, cmd app.BillingPreviewCommand) (app.BillingPreview, error) {
	// The same source query is rerun after taking the customer lock so changed amounts or source ownership block stale confirmation.
	query := app.BillingQuery{CustomerID: cmd.CustomerID, Page: 1, Limit: 200}
	charges, err := r.loadUnbilledCharges(ctx, tx, query)
	if err != nil {
		return app.BillingPreview{}, err
	}
	byKey := map[string]app.BillingCharge{}
	for _, c := range charges {
		byKey[c.ID] = c
	}
	p := app.BillingPreview{CustomerID: cmd.CustomerID, Lines: make([]app.SettlementLineSummary, 0, len(cmd.Sources))}
	for _, s := range cmd.Sources {
		row, ok := byKey[billingSourceID(s.ChargeType, s.SourceType, s.SourceID)]
		if !ok {
			return p, fmt.Errorf("billing source changed or unavailable")
		}
		p.Lines = append(p.Lines, app.SettlementLineSummary{ChargeType: row.ChargeType, SourceType: row.SourceType, SourceID: row.SourceID, SourceNo: row.SourceNo, Description: row.Description, OccurredAt: row.OccurredAt, ReceivableCents: row.ReceivableCents, AmountCents: row.DueCents, ReceivedCents: row.ReceivedCents, IssuedReceivedCents: row.ReceivedCents, DueCents: row.DueCents, ConfirmationStatus: row.ConfirmationStatus, SettlementStatus: row.SettlementStatus})
		p.TotalAmountCents += row.DueCents
		if p.PeriodFrom == "" || row.OccurredAt < p.PeriodFrom {
			p.PeriodFrom = row.OccurredAt
		}
		if p.PeriodTo == "" || row.OccurredAt > p.PeriodTo {
			p.PeriodTo = row.OccurredAt
		}
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, r.schema), cmd.CustomerID).Scan(&p.CustomerName); err != nil {
		return p, err
	}
	p.FeeItems = len(p.Lines)
	p.PreviewVersion = billingPreviewVersion(p)
	return p, nil
}

func (r *Repository) findIdempotentBill(ctx context.Context, customerID int64, key string) (app.BillingBill, error) {
	var id int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT batch_id FROM %s.customer_settlement_issue_keys WHERE customer_id=$1 AND idempotency_key=$2`, r.schema), customerID, key).Scan(&id)
	if err != nil {
		return app.BillingBill{}, err
	}
	return r.GetIssuedBill(ctx, customerID, id)
}
func (r *Repository) findIdempotentBillTx(ctx context.Context, tx pgx.Tx, customerID int64, key string) (app.BillingBill, error) {
	var id int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT batch_id FROM %s.customer_settlement_issue_keys WHERE customer_id=$1 AND idempotency_key=$2`, r.schema), customerID, key).Scan(&id)
	if err != nil {
		return app.BillingBill{}, err
	}
	return r.GetIssuedBill(ctx, customerID, id)
}
func errorsIsNoRows(err error) bool { return err == pgx.ErrNoRows }

func (r *Repository) GetBillingPDFSettings(ctx context.Context, customerID, settlementID int64) (app.BillingPDFSettings, error) {
	var out app.BillingPDFSettings
	var raw []byte
	var hasSavedSettings bool
	out.CustomerID, out.SettlementID = customerID, settlementID
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(p.seal_asset_id,0),COALESCE(p.placements,'[]'::jsonb),p.customer_id IS NOT NULL
		FROM %s.customer_settlement_batches b
		LEFT JOIN %s.customer_settlement_bill_pdf_settings p ON p.customer_id=b.customer_id AND p.settlement_id=b.id
		WHERE b.customer_id=$1 AND b.id=$2 AND b.status IN('confirmed','paid','settled') AND COALESCE(b.processing_billing_run_id,0)=0
	`, r.schema, r.schema), customerID, settlementID).Scan(&out.SealAssetID, &raw, &hasSavedSettings)
	if err != nil {
		return app.BillingPDFSettings{}, err
	}
	if !hasSavedSettings && relationExists(ctx, r.pool, r.schema+".sales_order_settings") {
		if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(seal_asset_id,0) FROM %s.sales_order_settings WHERE id=1`, r.schema)).Scan(&out.SealAssetID); err != nil {
			return app.BillingPDFSettings{}, err
		}
	}
	if err = json.Unmarshal(raw, &out.Placements); err != nil {
		return app.BillingPDFSettings{}, err
	}
	if out.Placements == nil {
		out.Placements = []app.BillingPDFPlacement{}
	}
	return out, nil
}

func (r *Repository) SaveBillingPDFSettings(ctx context.Context, settings app.BillingPDFSettings, actor string) (app.BillingPDFSettings, error) {
	if settings.SealAssetID > 0 {
		var valid bool
		if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.sales_order_assets WHERE id=$1 AND kind='seal')`, r.schema), settings.SealAssetID).Scan(&valid); err != nil {
			return app.BillingPDFSettings{}, err
		}
		if !valid {
			return app.BillingPDFSettings{}, fmt.Errorf("seal asset unavailable")
		}
	}
	raw, err := json.Marshal(settings.Placements)
	if err != nil {
		return app.BillingPDFSettings{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.BillingPDFSettings{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customer_settlement_batches WHERE id=$1 AND customer_id=$2 AND status IN('confirmed','paid','settled') AND COALESCE(processing_billing_run_id,0)=0) FOR UPDATE`, r.schema), settings.SettlementID, settings.CustomerID).Scan(&exists); err != nil {
		return app.BillingPDFSettings{}, err
	}
	if !exists {
		return app.BillingPDFSettings{}, fmt.Errorf("bill unavailable")
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_bill_pdf_settings(customer_id,settlement_id,seal_asset_id,placements,updated_by,updated_at) VALUES($1,$2,$3,$4::jsonb,$5,now()) ON CONFLICT(customer_id,settlement_id) DO UPDATE SET seal_asset_id=excluded.seal_asset_id,placements=excluded.placements,updated_by=excluded.updated_by,updated_at=now()`, r.schema), settings.CustomerID, settings.SettlementID, settings.SealAssetID, raw, actor); err != nil {
		return app.BillingPDFSettings{}, err
	}
	var entityID = int64(settings.SettlementID)
	if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "customer_fulfillment_settlement", &entityID, "pdf_layout", nil, nil, postgresinfra.StrPtr(string(raw)), postgresinfra.AuditMeta{"customer_id": settings.CustomerID, "settlement_id": settings.SettlementID, "seal_asset_id": settings.SealAssetID}); err != nil {
		return app.BillingPDFSettings{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return app.BillingPDFSettings{}, err
	}
	return settings, nil
}
