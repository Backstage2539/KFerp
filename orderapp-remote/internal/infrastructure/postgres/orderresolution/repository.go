package orderresolution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
	"strings"
)

var ErrStale = errors.New("订单或关联单据已变化，请刷新并重新确认处理预览")

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository { return Repository{pool, schema} }

type Step struct {
	domain.Task
	ID        int64           `json:"id"`
	Status    string          `json:"status"`
	Result    json.RawMessage `json:"result"`
	LastError string          `json:"last_error"`
	Failures  []StepFailure   `json:"failures"`
}
type Result struct {
	Applied        bool           `json:"applied"`
	CanWithdraw    bool           `json:"can_withdraw"`
	ID             int64          `json:"id"`
	AmendmentID    int64          `json:"amendment_id"`
	RevisedOrderID int64          `json:"revised_order_id"`
	Status         string         `json:"status"`
	Command        domain.Command `json:"command"`
	Plan           domain.Plan    `json:"plan"`
	Steps          []Step         `json:"steps"`
}
type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func hash(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
func (r Repository) Context(ctx context.Context, id int64) (domain.Context, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.Context{}, err
	}
	defer tx.Rollback(ctx)
	out, err := r.load(ctx, tx, id)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (r Repository) load(ctx context.Context, q querier, id int64) (domain.Context, error) {
	c := domain.Context{Lines: []domain.Line{}, Dependencies: []domain.Dependency{}, Unavailable: unavailableCapabilities()}
	feePredicate, err := r.feeOrderPredicate(ctx, q)
	if err != nil {
		return c, err
	}
	var raw []byte
	err = q.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object(
 'order',to_jsonb(o),'items',COALESCE((SELECT jsonb_agg(to_jsonb(i) ORDER BY i.id) FROM %[1]s.order_items i WHERE i.order_id=o.id),'[]'),
 'entitlements',COALESCE((SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM %[1]s.customer_stock_entitlements e WHERE e.source_order_id=o.id),'[]'),
 'reservations',COALESCE((SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM %[1]s.customer_stock_supply_reservations s JOIN %[1]s.customer_stock_entitlements e ON e.id=s.entitlement_id WHERE e.source_order_id=o.id),'[]'),
 'direct_requests',COALESCE((SELECT jsonb_agg(to_jsonb(d)ORDER BY d.id)FROM %[1]s.customer_direct_ship_requests d WHERE d.id IN(SELECT request_id FROM %[1]s.customer_direct_ship_request_orders WHERE order_id=o.id UNION SELECT s.request_id FROM %[1]s.customer_stock_supply_reservations s JOIN %[1]s.customer_stock_entitlements e ON e.id=s.entitlement_id WHERE e.source_order_id=o.id)),'[]'),
 'packages',COALESCE((SELECT jsonb_agg(to_jsonb(p)ORDER BY p.id)FROM %[1]s.customer_direct_ship_request_orders p WHERE p.request_id IN(SELECT request_id FROM %[1]s.customer_direct_ship_request_orders WHERE order_id=o.id UNION SELECT s.request_id FROM %[1]s.customer_stock_supply_reservations s JOIN %[1]s.customer_stock_entitlements e ON e.id=s.entitlement_id WHERE e.source_order_id=o.id)),'[]'),
 'direct_allocations',COALESCE((SELECT jsonb_agg(to_jsonb(a)ORDER BY a.id)FROM %[1]s.customer_direct_ship_request_allocations a WHERE a.order_id=o.id),'[]'),
 'production',COALESCE((SELECT jsonb_agg(to_jsonb(w) ORDER BY w.id) FROM %[1]s.work_orders w WHERE o.order_no=ANY(string_to_array(replace(w.order_nos,' ',''),','))),'[]'),
 'plan_items',COALESCE((SELECT jsonb_agg(to_jsonb(i)ORDER BY i.id)FROM %[1]s.production_plan_items i WHERE i.production_plan_id IN(SELECT x.production_plan_id FROM %[1]s.production_plan_items x WHERE o.order_no=ANY(string_to_array(replace(x.order_nos,' ',''),',')))),'[]'),
 'plans',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM %[1]s.production_plans p WHERE EXISTS(SELECT 1 FROM %[1]s.production_plan_items i WHERE i.production_plan_id=p.id AND o.order_no=ANY(string_to_array(replace(i.order_nos,' ',''),',')))),'[]'),
 'invoice',(SELECT to_jsonb(i) FROM %[1]s.order_invoices i WHERE i.order_id=o.id),
 'billing',COALESCE((SELECT jsonb_agg(to_jsonb(l)||jsonb_build_object('bill_status',b.status) ORDER BY l.id) FROM %[1]s.customer_settlement_lines l JOIN %[1]s.customer_settlement_batches b ON b.id=l.settlement_batch_id AND b.customer_id=l.customer_id WHERE ((l.source_type='order' AND l.source_id=o.id) OR (l.source_type='customer_fee_item' AND l.source_id IN(SELECT f.id FROM %[1]s.customer_fee_items f WHERE %[2]s))) AND l.reversed_at IS NULL AND b.status IN('confirmed','paid','settled') AND COALESCE(b.processing_billing_run_id,0)=0),'[]'),
 'fees',COALESCE((SELECT jsonb_agg(to_jsonb(f)||jsonb_build_object('paid_cents',CASE WHEN f.resolution_cash_tracked THEN f.resolution_cash_received_cents WHEN b.status='paid' THEN round((f.amount-f.resolution_credit_cents/100.0)*100)::bigint ELSE 0 END,'bill_status',COALESCE(b.status,'')) ORDER BY f.id) FROM %[1]s.customer_fee_items f LEFT JOIN %[1]s.customer_settlement_batches b ON b.id=f.settlement_batch_id AND b.customer_id=f.customer_id WHERE %[2]s),'[]'),
 'stock',COALESCE((SELECT jsonb_agg(to_jsonb(d) ORDER BY d.id) FROM %[1]s.order_stock_deductions d WHERE d.order_id=o.id),'[]'),
 'allocated',COALESCE((SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=o.id),'[]'),
 'fee_events',COALESCE((SELECT jsonb_agg(to_jsonb(f) ORDER BY f.id) FROM %[1]s.order_resolution_fee_events f WHERE f.order_id=o.id),'[]'),
 'financial',COALESCE((SELECT jsonb_agg(to_jsonb(f) ORDER BY f.id) FROM %[1]s.order_resolution_financial_entries f WHERE f.order_id=o.id),'[]'),
 'policy',(SELECT to_jsonb(p) FROM %[1]s.order_resolution_policy p WHERE p.id=1),
 'pay_status',COALESCE((SELECT name FROM %[1]s.pay_statuses WHERE id=o.pay_status_id),''),
 'ship_status',COALESCE((SELECT name FROM %[1]s.ship_statuses WHERE id=o.ship_status_id),'')
 ) FROM %[1]s.orders o WHERE o.id=$1`, r.schema, feePredicate), id).Scan(&raw)
	if err != nil {
		return c, err
	}
	c.Version = hash(raw)
	var snap struct {
		Policy struct {
			Value domain.Policy `json:"policy_json"`
		} `json:"policy"`
		Order          map[string]json.RawMessage   `json:"order"`
		Items          []map[string]json.RawMessage `json:"items"`
		Entitlements   []map[string]json.RawMessage `json:"entitlements"`
		DirectRequests []map[string]json.RawMessage `json:"direct_requests"`
		Packages       []map[string]json.RawMessage `json:"packages"`
		Reservations   []map[string]json.RawMessage `json:"reservations"`
		Production     []map[string]json.RawMessage `json:"production"`
		PlanItems      []map[string]json.RawMessage `json:"plan_items"`
		Plans          []map[string]json.RawMessage `json:"plans"`
		Invoice        json.RawMessage              `json:"invoice"`
		Billing        []json.RawMessage            `json:"billing"`
		Fees           []map[string]json.RawMessage `json:"fees"`
		Stock          []map[string]json.RawMessage `json:"stock"`
		Allocated      []map[string]json.RawMessage `json:"allocated"`
		FeeEvents      []map[string]json.RawMessage `json:"fee_events"`
		Financial      []map[string]json.RawMessage `json:"financial"`
		PayStatus      string                       `json:"pay_status"`
		ShipStatus     string                       `json:"ship_status"`
	}
	if err = json.Unmarshal(raw, &snap); err != nil {
		return c, err
	}
	c.Policy = snap.Policy.Value
	o := snap.Order
	c.OrderID = id
	c.OrderNo = str(o, "order_no")
	c.CustomerID = int64(num(o, "customer_id"))
	c.Status = str(o, "resolution_status")
	c.Paused = boolean(o, "fulfillment_paused")
	c.Void = boolean(o, "is_void") || c.Status == "cancelled"
	c.DeliveryMode = str(o, "delivery_mode")
	c.FulfillmentMode = str(o, "fulfillment_mode")
	if c.FulfillmentMode != "" {
		c.Unavailable["action:reopen"] = "代发预订解除后须重新选择货源创建发货申请，不能直接恢复原包裹"
	}
	c.OrderDate = str(o, "order_date")
	c.TotalCents = cents(num(o, "grand_total"))
	if c.TotalCents == 0 && num(o, "discount_amount") == 0 && num(o, "shipping_amount") == 0 && num(o, "rounding_amount") == 0 {
		c.TotalCents = cents(num(o, "total_amount"))
	}
	c.CreditCents = int64(num(o, "resolution_credit_cents"))
	c.OffsetCents = int64(num(o, "resolution_offset_cents"))
	c.PaidCents = cents(num(o, "prepayment_amount"))
	if v := cents(num(o, "payment_goods_amount") + num(o, "payment_shipping_amount")); v > c.PaidCents {
		c.PaidCents = v
	}
	if boolean(o, "resolution_payment_tracked") {
		c.PaidCents += int64(num(o, "resolution_legacy_cash_cents"))
	} else if snap.PayStatus == "已付款" || snap.PayStatus == "已收款" || snap.PayStatus == "已支付" {
		if c.TotalCents > c.PaidCents {
			c.PaidCents = c.TotalCents
		}
	}
	c.ExternalVersion = hash(snap.Invoice)
	c.Invoiced = len(snap.Invoice) > 0 && string(snap.Invoice) != "null"
	c.Billed = len(snap.Billing) > 0
	for _, f := range snap.Financial {
		if num(f, "source_fee_id") == 0 && one(str(f, "kind"), "refund", "customer_credit", "offset") {
			c.CreditUsedCents += int64(num(f, "amount_cents"))
		}
	}
	delivered, fulfillmentIssue := shipmentQuantities(snap.Items, snap.Stock, snap.Allocated)
	if c.DeliveryMode != "customer_stock" {
		c.FulfillmentIssue = fulfillmentIssue
	}
	for _, i := range snap.Items {
		l := domain.Line{ID: int64(num(i, "id")), Name: str(i, "item_name"), Unit: str(i, "unit"), Quantity: num(i, "qty"), Closed: num(i, "resolution_closed_qty"), AmountCents: cents(num(i, "line_total"))}
		for _, e := range snap.Entitlements {
			if int64(num(e, "source_order_item_id")) == l.ID {
				l.Delivered = num(e, "received_qty")
				if l.Delivered > 0 {
					status := str(e, "status")
					if l.Delivered == num(e, "returned_qty") {
						status = "released"
					}
					c.Dependencies = append(c.Dependencies, domain.Dependency{Kind: "customer_stock", ID: int64(num(e, "id")), Label: fmt.Sprintf("客户库存权益 #%d · %s · 原入库 %g · 货权已退回 %g %s", int64(num(e, "id")), l.Name, l.Delivered, num(e, "returned_qty"), l.Unit), Status: status, Quantity: l.Delivered - num(e, "returned_qty")})
				}
			}
		}
		if c.DeliveryMode != "customer_stock" {
			l.Delivered = delivered[l.ID]
			// Historical whole-order shipments can predate batch deductions.
			if len(snap.Stock) == 0 && one(snap.ShipStatus, "已发货", "已出库", "已签收", "已收货", "已完成") {
				l.Delivered = l.Quantity
			}
		}
		err = q.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(e.credit_cents) FILTER(WHERE e.closed_delta<>0),0),COALESCE(sum(e.return_delta),0)::float8+(SELECT COALESCE(sum(x.quantity),0)::float8 FROM %[1]s.order_resolution_stock_returns x JOIN %[1]s.order_resolution_requests r ON r.id=x.request_id WHERE x.order_item_id=$1 AND x.status='posted' AND NOT r.applied),COALESCE(sum(e.credit_cents),0),COALESCE(sum(e.return_delta) FILTER(WHERE r.command_json->>'return_settlement'='credit_returned_goods'),0)::float8,COALESCE(sum(e.credit_cents) FILTER(WHERE r.command_json->>'action'='return'),0) FROM %[1]s.order_resolution_line_events e JOIN %[1]s.order_resolution_requests r ON r.id=e.request_id WHERE e.order_item_id=$1`, r.schema), l.ID).Scan(&l.ClosedCents, &l.Returned, &l.CreditedCents, &l.ReturnCreditQuantity, &l.ReturnCreditCents)
		if err != nil {
			return c, err
		}
		c.Lines = append(c.Lines, l)
	}
	// Allocate the frozen net goods amount cumulatively so order-level discounts
	// and rounding never create an extra cent. Freight remains a separate fee.
	var gross int64
	for _, l := range c.Lines {
		gross += l.AmountCents
	}
	budget := min(gross, max(int64(0), c.TotalCents-cents(num(o, "shipping_amount"))))
	if gross > 0 && budget != gross {
		var cumulative, allocated int64
		for n := range c.Lines {
			cumulative += c.Lines[n].AmountCents
			next := int64(math.Round(float64(budget) * float64(cumulative) / float64(gross)))
			c.Lines[n].AmountCents = next - allocated
			allocated = next
		}
	}
	for _, fee := range []struct {
		id     int64
		label  string
		amount int64
	}{{-1, "本单运费", cents(num(o, "shipping_amount"))}, {-2, "本单其他费用", max(int64(0), c.TotalCents-budget-cents(num(o, "shipping_amount")))}} {
		credit := int64(0)
		for _, e := range snap.FeeEvents {
			if int64(num(e, "source_id")) == fee.id {
				credit += int64(num(e, "credit_cents"))
			}
		}
		if fee.amount > credit {
			c.Dependencies = append(c.Dependencies, domain.Dependency{Kind: "fee", ID: fee.id, Label: fee.label, Status: "active", AmountCents: fee.amount - credit})
		}
	}
	for _, p := range snap.Plans {
		pid := int64(num(p, "id"))
		shared := false
		for _, item := range snap.PlanItems {
			if int64(num(item, "production_plan_id")) != pid {
				continue
			}
			var demandSources []map[string]json.RawMessage
			_ = json.Unmarshal(item["demand_sources_json"], &demandSources)
			for _, source := range demandSources {
				if int64(num(source, "order_id")) != c.OrderID || str(source, "order_no") != c.OrderNo {
					shared = true
				}
			}
			for _, no := range strings.Split(str(item, "order_nos"), ",") {
				if other := strings.TrimSpace(no); other != "" && other != c.OrderNo {
					shared = true
				}
			}
		}
		for _, w := range snap.Production {
			if int64(num(w, "production_plan_id")) == pid && len(strings.Split(str(w, "order_nos"), ",")) > 1 {
				shared = true
			}
		}
		var ownershipIssue string
		if err = q.QueryRow(ctx, fmt.Sprintf(`SELECT %s.order_resolution_company_production_issue($1)`, r.schema), pid).Scan(&ownershipIssue); err != nil {
			return c, err
		}
		c.Version = hash([]byte(c.Version + fmt.Sprintf("|%d|%s", pid, ownershipIssue)))
		c.Dependencies = append(c.Dependencies, domain.Dependency{Kind: "production", ID: pid, Label: str(p, "plan_no"), Status: str(p, "status"), Shared: shared, OwnershipIssue: ownershipIssue})
	}
	seen := map[int64]bool{}
	for _, pkg := range snap.Packages {
		if int64(num(pkg, "order_id")) != id {
			continue
		}
		rid := int64(num(pkg, "request_id"))
		seen[rid] = true
		c.Dependencies = append(c.Dependencies, directShipDependency(rid, str(pkg, "status"), "package", snap.DirectRequests, snap.Packages))
	}
	for _, reservation := range snap.Reservations {
		rid := int64(num(reservation, "request_id"))
		if seen[rid] {
			continue
		}
		seen[rid] = true
		state := str(reservation, "status")
		if num(reservation, "reserved_qty") <= num(reservation, "released_qty") {
			state = "released"
		}
		d := directShipDependency(rid, state, "supply", snap.DirectRequests, snap.Packages)
		d.Quantity = max(float64(0), num(reservation, "reserved_qty")-num(reservation, "released_qty")-num(reservation, "converted_qty"))
		c.Dependencies = append(c.Dependencies, d)
	}
	for n := range c.Dependencies {
		d := &c.Dependencies[n]
		if d.Kind != "direct_ship" || d.Relation != "supply" || d.Status == "cancelled" {
			continue
		}
		d.Quantity = 0
		unreleased := float64(0)
		for _, reservation := range snap.Reservations {
			if int64(num(reservation, "request_id")) != d.ID {
				continue
			}
			unreleased += max(float64(0), num(reservation, "reserved_qty")-num(reservation, "released_qty"))
			d.Quantity += max(float64(0), num(reservation, "reserved_qty")-num(reservation, "released_qty")-num(reservation, "converted_qty"))
		}
		switch {
		case d.Quantity > 0:
			d.Status = "reserved"
		case unreleased > 0:
			d.Status = "converted"
		default:
			d.Status = "released"
		}
	}
	reservationDeps, reservationIssue := reservationDependencies(snap.Items, snap.Allocated, snap.Stock)
	c.Dependencies = append(c.Dependencies, reservationDeps...)
	if reservationIssue != "" {
		c.FulfillmentIssue = reservationIssue
	}
	for _, f := range snap.Fees {
		cashIssue := ""
		if (str(f, "bill_status") == "settled" || (str(f, "bill_status") == "" && str(f, "status") == "settled")) && int64(num(f, "paid_cents")) == 0 {
			cashIssue = "费用仅有历史结算标记，尚无可核对的收款金额，请先由财务核对收款来源"
		}
		if one(str(f, "status"), "reversed", "cancelled") {
			continue
		}
		c.Dependencies = append(c.Dependencies, domain.Dependency{Kind: "fee", ID: int64(num(f, "id")), Label: feeLabel(str(f, "fee_type")), CashIssue: cashIssue, Status: str(f, "status"), AmountCents: cents(num(f, "amount")) - int64(num(f, "resolution_credit_cents")), PaidCents: int64(num(f, "paid_cents")), SettledOutCents: int64(num(f, "resolution_settled_out_cents"))})
	}
	if err = r.replacementSources(ctx, q, &c); err != nil {
		return c, err
	}
	if err = r.productionStopSources(ctx, q, &c); err != nil {
		return c, err
	}
	if err = r.companyStockSources(ctx, q, &c, snap.Items); err != nil {
		return c, err
	}
	if err = r.returnSources(ctx, q, &c); err != nil {
		return c, err
	}
	return c, nil
}
func str(m map[string]json.RawMessage, k string) string {
	var s string
	_ = json.Unmarshal(m[k], &s)
	return s
}
func num(m map[string]json.RawMessage, k string) float64 {
	var n float64
	_ = json.Unmarshal(m[k], &n)
	return n
}
func boolean(m map[string]json.RawMessage, k string) bool {
	var b bool
	_ = json.Unmarshal(m[k], &b)
	return b
}
func one(s string, v ...string) bool {
	for _, a := range v {
		if a == s {
			return true
		}
	}
	return false
}
func cents(v float64) int64 {
	if v >= 0 {
		return int64(v*100 + 0.5)
	}
	return int64(v*100 - 0.5)
}

func (r Repository) Submit(ctx context.Context, id int64, cmd domain.Command, actor string) (Result, error) {
	var out Result
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(cmd.IdempotencyKey) == "" || len(cmd.IdempotencyKey) > 128 || cmd.Version == "" {
		return out, fmt.Errorf("缺少操作人、预览版本或提交标识")
	}
	body, err := json.Marshal(cmd)
	if err != nil {
		return out, err
	}
	digest := hash(body)
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = r.lockCompanyTargets(ctx, tx, id, cmd); err != nil {
		return out, err
	}
	if err = r.lock(ctx, tx, id); err != nil {
		return out, err
	}
	var priorID int64
	var priorHash string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,request_hash FROM %s.order_resolution_requests WHERE order_id=$1 AND idempotency_key=$2`, r.schema), id, cmd.IdempotencyKey).Scan(&priorID, &priorHash)
	if err == nil {
		if priorHash != digest {
			return out, fmt.Errorf("重复提交标识对应的内容不同")
		}
		out, err = r.result(ctx, tx, priorID)
		return out, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	current, err := r.load(ctx, tx, id)
	if err != nil {
		return out, err
	}
	if current.Version != cmd.Version {
		return out, ErrStale
	}
	plan, err := domain.Preview(current, cmd)
	if err != nil {
		return out, err
	}
	if current.FulfillmentMode != "" && one(cmd.Action, "close", "cancel", "amend") {
		linked := false
		for _, d := range current.Dependencies {
			if d.Kind == "direct_ship" && d.Relation == "package" {
				linked = true
			}
		}
		if !linked {
			return out, fmt.Errorf("代发订单缺少可核对的发货申请，请先核对关联关系")
		}
	}
	if cmd.PostingMode == "open_period" {
		if err = r.checkPeriod(ctx, tx, cmd.PostingDate); err != nil {
			return out, err
		}
	}
	var pending bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.order_resolution_requests WHERE order_id=$1 AND status='pending')`, r.schema), id).Scan(&pending); err != nil {
		return out, err
	}
	if pending {
		return out, fmt.Errorf("已有订单处理单待完成，请先处理或撤回")
	}
	// Freeze only the legacy status-derived cash gap. Subsequent fully-paid
	// statuses refer to the adjusted receivable, never the original gross amount.
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET resolution_payment_tracked=true,resolution_legacy_cash_cents=GREATEST(0,$2-round(GREATEST(COALESCE(prepayment_amount,0),COALESCE(payment_goods_amount,0)+COALESCE(payment_shipping_amount,0))*100)::bigint) WHERE id=$1 AND NOT resolution_payment_tracked`, r.schema), id, current.PaidCents); err != nil {
		return out, err
	}
	contextRaw, _ := json.Marshal(current)
	planRaw, _ := json.Marshal(plan)
	state := "completed"
	if len(plan.Tasks) > 0 {
		state = "pending"
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_requests(order_id,idempotency_key,request_hash,command_json,plan_json,status,actor,context_json)VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, r.schema), id, cmd.IdempotencyKey, digest, body, planRaw, state, actor, contextRaw).Scan(&out.ID); err != nil {
		return out, err
	}
	for _, t := range plan.Tasks {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_steps(request_id,kind,source_id,choice,label,permission)VALUES($1,$2,$3,$4,$5,$6)`, r.schema), out.ID, t.Kind, t.SourceID, t.Choice, t.Label, t.Permission); err != nil {
			return out, err
		}
	}
	if state == "completed" || (cmd.Timing == "stop_first" && onlyFunds(plan.Tasks)) {
		if err = r.apply(ctx, tx, id, out.ID, current, cmd, plan, actor); err != nil {
			return out, err
		}
	} else if cmd.Scope != "fees_only" {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET fulfillment_paused=true WHERE id=$1`, r.schema), id); err != nil {
			return out, err
		}
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &id, "resolution_submit", nil, nil, nil, infra.AuditMeta{"resolution_id": out.ID, "command": cmd, "plan": plan, "status": state}); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_resolution_drafts WHERE order_id=$1 AND actor=$2`, r.schema), id, actor); err != nil {
		return out, err
	}
	out, err = r.result(ctx, tx, out.ID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (r Repository) checkPeriod(ctx context.Context, q querier, date string) error {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, r.schema+".finance_monthly_reports").Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	var closed bool
	if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.finance_monthly_reports WHERE month=left($1,7) AND status IN('closed','adjusted'))`, r.schema), date).Scan(&closed); err != nil {
		return err
	}
	if closed {
		return fmt.Errorf("所选账期已结账，请选择开放账期或申请历史更正")
	}
	return nil
}
func (r Repository) apply(ctx context.Context, tx pgx.Tx, id, requestID int64, current domain.Context, cmd domain.Command, p domain.Plan, actor string) error {
	if err := r.lockCompanyStockIdentities(ctx, tx, cmd); err != nil {
		return err
	}
	if err := r.revalidatePlan(ctx, tx, id, requestID, current, cmd, p); err != nil {
		return err
	}
	if one(cmd.Action, "hold", "resume") {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET fulfillment_paused=$2 WHERE id=$1`, r.schema), id, cmd.Action == "hold"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_requests SET applied=true WHERE id=$1`, r.schema), requestID)
		return err
	}
	if err := r.adjustIssuedBilling(ctx, tx, id, requestID, cmd, p, actor); err != nil {
		return err
	}
	if err := r.releaseReservations(ctx, tx, id, requestID, current, cmd, actor); err != nil {
		return err
	}
	if err := r.applyCompanyAllocations(ctx, tx, id, requestID, cmd, actor); err != nil {
		return err
	}
	if err := r.applyProductionStops(ctx, tx, id, requestID, cmd, actor); err != nil {
		return err
	}
	if cmd.Action == "amend" {
		_, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_resolution_amendments(request_id,source_order_id,customer_id,snapshot_json,actor)
        SELECT $1,o.id,o.customer_id,jsonb_build_object('order',to_jsonb(o),'items',(SELECT jsonb_agg(to_jsonb(i)ORDER BY i.id)FROM %[1]s.order_items i WHERE i.order_id=o.id)),$3 FROM %[1]s.orders o WHERE o.id=$2`, r.schema), requestID, id, actor)
		if err != nil {
			return err
		}
	}
	feePredicate, err := r.feeOrderPredicate(ctx, tx)
	if err != nil {
		return err
	}
	for _, fee := range p.Fees {
		if fee.SourceID == -1 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET resolution_shipping_credit_cents=resolution_shipping_credit_cents+$2 WHERE id=$1`, r.schema), id, fee.CreditCents); err != nil {
				return err
			}
		}
		if fee.SourceID > 0 && fee.CreditCents > 0 {
			paid := int64(0)
			for _, dep := range current.Dependencies {
				if dep.Kind == "fee" && dep.ID == fee.SourceID {
					paid = dep.PaidCents
				}
			}
			tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.customer_fee_items f SET resolution_credit_cents=resolution_credit_cents+$2,resolution_cash_tracked=resolution_cash_tracked OR $5>0,resolution_cash_received_cents=CASE WHEN resolution_cash_tracked THEN resolution_cash_received_cents ELSE $5 END WHERE id=$1 AND customer_id=$3 AND EXISTS(SELECT 1 FROM %[1]s.orders o WHERE o.id=$4 AND %[2]s) AND round(amount*100)::bigint-resolution_credit_cents >= $2 AND NOT EXISTS(SELECT 1 FROM %[1]s.customer_settlement_batches b WHERE b.id=f.settlement_batch_id AND b.status='settled' AND NOT f.resolution_cash_tracked)`, r.schema, feePredicate), fee.SourceID, fee.CreditCents, current.CustomerID, id, paid)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("费用金额或收款状态已变化，请重新核对")
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_fee_events(request_id,order_id,source_id,credit_cents)VALUES($1,$2,$3,$4)`, r.schema), requestID, id, fee.SourceID, fee.CreditCents); err != nil {
			return err
		}
	}
	for _, l := range p.Lines {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_line_events(request_id,order_item_id,closed_delta,return_delta,credit_cents)VALUES($1,$2,$3,$4,$5)`, r.schema), requestID, l.ItemID, l.ClosedDelta, l.ReturnDelta, l.CreditCents); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET resolution_closed_qty=resolution_closed_qty+$2 WHERE id=$1 AND order_id=$3`, r.schema), l.ItemID, l.ClosedDelta, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_entitlements SET closed_qty=closed_qty+$2::bigint,updated_at=now() WHERE source_order_item_id=$1`, r.schema), l.ItemID, l.ClosedDelta); err != nil {
			return err
		}
	}
	if p.CreditCents != 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_financial_entries(request_id,order_id,customer_id,posting_date,kind,amount_cents,actor)VALUES($1,$2,$3,$4::date,'receivable_credit',$5,$6)`, r.schema), requestID, id, current.CustomerID, cmd.PostingDate, p.CreditCents, actor); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET resolution_status=$2,fulfillment_paused=CASE WHEN $4 THEN fulfillment_paused ELSE false END,resolution_credit_cents=resolution_credit_cents+$3,is_void=CASE WHEN $2='cancelled' THEN true ELSE is_void END WHERE id=$1`, r.schema), id, p.Status, p.CreditCents, cmd.Scope == "fees_only")
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_requests SET applied=true WHERE id=$1`, r.schema), requestID)
	if err != nil {
		return err
	}
	return r.finalizeHistoricalPeriod(ctx, tx, requestID, cmd, actor)
}
func (r Repository) result(ctx context.Context, q querier, id int64) (Result, error) {
	out := Result{ID: id, Steps: []Step{}}
	var cmd, plan []byte
	if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT status,command_json,plan_json,applied,status='pending' AND NOT applied AND NOT EXISTS(SELECT 1 FROM %[1]s.order_resolution_step_actions a JOIN %[1]s.order_resolution_steps s ON s.id=a.step_id WHERE s.request_id=$1) FROM %[1]s.order_resolution_requests WHERE id=$1`, r.schema), id).Scan(&out.Status, &cmd, &plan, &out.Applied, &out.CanWithdraw); err != nil {
		return out, err
	}
	if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT id FROM %[1]s.order_resolution_amendments WHERE request_id=$1),0),COALESCE((SELECT new_order_id FROM %[1]s.order_resolution_amendments WHERE request_id=$1),0)`, r.schema), id).Scan(&out.AmendmentID, &out.RevisedOrderID); err != nil {
		return out, err
	}
	if err := json.Unmarshal(cmd, &out.Command); err != nil {
		return out, err
	}
	if err := json.Unmarshal(plan, &out.Plan); err != nil {
		return out, err
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT s.id,kind,source_id,choice,label,permission,status,result_json,last_error,COALESCE((SELECT jsonb_agg(jsonb_build_object('message',f.message,'actor',f.actor,'created_at',f.created_at)ORDER BY f.id)FROM %s.order_resolution_step_failures f WHERE f.step_id=s.id),'[]') FROM %s.order_resolution_steps s WHERE request_id=$1 ORDER BY s.id`, r.schema, r.schema), id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var s Step
		var failures []byte
		if err = rows.Scan(&s.ID, &s.Kind, &s.SourceID, &s.Choice, &s.Label, &s.Permission, &s.Status, &s.Result, &s.LastError, &failures); err != nil {
			return out, err
		}
		if err = json.Unmarshal(failures, &s.Failures); err != nil {
			return out, err
		}
		out.Steps = append(out.Steps, s)
	}
	return out, rows.Err()
}
func (r Repository) History(ctx context.Context, id int64) ([]Result, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.order_resolution_requests WHERE order_id=$1 ORDER BY id DESC LIMIT 100`, r.schema), id)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	for rows.Next() {
		var n int64
		if err = rows.Scan(&n); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, n)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []Result{}
	for _, n := range ids {
		v, err := r.result(ctx, r.pool, n)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func (r Repository) SaveDraft(ctx context.Context, id int64, cmd domain.Command, actor string) error {
	body, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_drafts(order_id,actor,command_json)VALUES($1,$2,$3)ON CONFLICT(order_id,actor)DO UPDATE SET command_json=excluded.command_json,updated_at=now()`, r.schema), id, actor, body); err != nil {
		return err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &id, "resolution_draft", nil, nil, nil, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r Repository) Draft(ctx context.Context, id int64, actor string) (domain.Command, error) {
	var cmd domain.Command
	var raw []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT command_json FROM %s.order_resolution_drafts WHERE order_id=$1 AND actor=$2`, r.schema), id, actor).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return cmd, nil
	}
	if err != nil {
		return cmd, err
	}
	return cmd, json.Unmarshal(raw, &cmd)
}

func onlyFunds(tasks []domain.Task) bool {
	for _, t := range tasks {
		if t.Kind != "funds" {
			return false
		}
	}
	return true
}

func directShipDependency(id int64, status, relation string, requests, packages []map[string]json.RawMessage) domain.Dependency {
	d := domain.Dependency{Kind: "direct_ship", ID: id, Label: fmt.Sprintf("代发申请 #%d", id), Status: status, Relation: relation}
	for _, r := range requests {
		if int64(num(r, "id")) == id {
			d.Label = str(r, "request_no")
			if str(r, "status") == "cancelled" {
				d.Status = "cancelled"
			}
		}
	}
	if relation == "package" {
		d.Label += " · 本订单包裹"
	}
	for _, p := range packages {
		if int64(num(p, "request_id")) == id {
			d.Orders = append(d.Orders, domain.LinkedOrder{ID: int64(num(p, "order_id")), Number: str(p, "order_no"), Status: str(p, "status")})
		}
	}
	return d
}
