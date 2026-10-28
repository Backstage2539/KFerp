package customerfulfillment

import (
	"context"
	"fmt"
	freightdomain "orderapp/internal/domain/freight"
	salesdomain "orderapp/internal/domain/sales"
	"sort"
	"strings"
	"time"
)

type AccountQuery struct {
	CurrentVersion bool
	UnpaidOnly     bool
	View           string
	ChargeType     string
	CustomerID     int64
	OrderID        int64
	Query          string
	DateFrom       string
	DateTo         string
	Period         string
	Anchor         string
	PayStatus      string
	ShipStatus     string
	IncludeVoid    bool
	Page           int
	Limit          int
}
type AccountOrder struct {
	Freight              *freightdomain.OrderView `json:"freight,omitempty"`
	ConfirmationStatus   string                   `json:"confirmation_status"`
	ConfirmationRequired bool                     `json:"confirmation_required"`
	AcceptedRevision     int64                    `json:"accepted_revision"`
	ProcessStatus        string                   `json:"process_status"`
	ID                   int64                    `json:"id"`
	OrderNo              string                   `json:"order_no"`
	OrderDate            string                   `json:"order_date"`
	ReceiverName         string                   `json:"receiver_name"`
	ReceiverPhone        string                   `json:"receiver_phone"`
	ReceiverAddress      string                   `json:"receiver_address"`
	ShipStatus           string                   `json:"ship_status"`
	TrackingNo           string                   `json:"ship_tracking_no"`
	PayStatus            string                   `json:"pay_status"`
	PaymentStatus        string                   `json:"payment_status"`
	Service              string                   `json:"portal_service_code"`
	IsVoid               bool                     `json:"is_void"`
	GoodsCents           int64                    `json:"goods_cents"`
	ShippingCents        int64                    `json:"shipping_cents"`
	DiscountCents        int64                    `json:"discount_cents"`
	TotalCents           int64                    `json:"total_cents"`
	PrepaymentCents      int64                    `json:"prepayment_cents"`
	PaidCents            int64                    `json:"paid_cents"`
	DueCents             int64                    `json:"due_cents"`
	Items                []AccountOrderItem       `json:"items,omitempty"`
	InPeriod             bool                     `json:"in_period,omitempty"`
	HasOrderShippingFee  bool                     `json:"-"`
}
type AccountOrderItem struct {
	Name                 string  `json:"item_name"`
	Spec                 string  `json:"spec"`
	Quantity             string  `json:"qty"`
	Unit                 string  `json:"unit"`
	UnitPrice            string  `json:"unit_price"`
	LineTotal            string  `json:"line_total"`
	MaterialUnitWeightG  *string `json:"material_unit_weight_g"`
	MaterialItemWeightG  *string `json:"material_item_weight_g"`
	LogisticsUnitWeightG *string `json:"logistics_unit_weight_g"`
	LogisticsItemWeightG *string `json:"logistics_item_weight_g"`
	WeightSnapshotJSON   string  `json:"weight_snapshot_json"`
}

func (o *AccountOrder) SetPayment() {
	o.PaidCents = o.PrepaymentCents
	if salesdomain.IsFullyPaid(o.PayStatus) {
		o.PaidCents = o.TotalCents
	}
	o.DueCents = max(int64(0), o.TotalCents-o.PaidCents)
	o.PaymentStatus = "unpaid"
	if o.DueCents == 0 {
		o.PaymentStatus = "paid"
	} else if o.PaidCents > 0 {
		o.PaymentStatus = "partial"
	}
}

type AccountSummary struct {
	Count                  int   `json:"count"`
	GoodsCents             int64 `json:"goods_cents"`
	ShippingCents          int64 `json:"shipping_cents"`
	DiscountCents          int64 `json:"discount_cents"`
	TotalCents             int64 `json:"total_cents"`
	PaidCents              int64 `json:"paid_cents"`
	DueCents               int64 `json:"due_cents"`
	ProcessingCents        int64 `json:"processing_cents"`
	DirectShipServiceCents int64 `json:"direct_ship_service_cents"`
	FeeShippingCents       int64 `json:"fee_shipping_cents"`
	AdjustmentCents        int64 `json:"adjustment_cents"`
	RefundCents            int64 `json:"refund_cents"`
	AdditionalFeeCents     int64 `json:"additional_fee_cents"`
	PayableCents           int64 `json:"payable_cents"`
}
type AccountFee struct {
	ID               int64  `json:"id"`
	OrderID          int64  `json:"order_id"`
	OrderNo          string `json:"order_no"`
	FeeType          string `json:"fee_type"`
	AmountCents      int64  `json:"amount_cents"`
	Currency         string `json:"currency"`
	OccurredAt       string `json:"occurred_at"`
	SettlementID     int64  `json:"settlement_id"`
	SettlementNo     string `json:"settlement_no"`
	SettlementStatus string `json:"settlement_status"`
	PaymentStatus    string `json:"payment_status"`
	SourceType       string `json:"source_type"`
	SourceID         int64  `json:"source_id"`
	FeeName          string `json:"fee_name"`
	IncludedInOrder  bool   `json:"included_in_order"`
	ShipmentID       int64  `json:"shipment_id,omitempty"`
	TrackingNo       string `json:"tracking_no,omitempty"`
}

type AccountShipment struct {
	OrderID            int64  `json:"order_id"`
	OrderNo            string `json:"order_no"`
	TrackingNo         string `json:"tracking_no"`
	ShippedAt          string `json:"shipped_at"`
	PickupAt           string `json:"pickup_at"`
	SettlementWeightKG string `json:"settlement_weight_kg"`
	ShippingCents      int64  `json:"shipping_cents"`
	Source             string `json:"source"`
}

type AccountFeeLine struct {
	AccountFee
	Category        string `json:"category"`
	ReceivableCents int64  `json:"receivable_cents,omitempty"`
	ReceivedCents   int64  `json:"received_cents,omitempty"`
}

// AccountUnpaidSource identifies one receivable using the same source identity
// as the ERP billing ledger. Amounts are current, not a new charge.
type AccountUnpaidSource struct {
	ChargeType, SourceType, SourceNo, Description, OccurredAt string
	SourceID, ReceivableCents, ReceivedCents, DueCents        int64
}

type AccountOrderFeeGroup struct {
	Order        AccountOrder      `json:"order"`
	Unlinked     bool              `json:"unlinked,omitempty"`
	PayableCents int64             `json:"payable_cents"`
	PaidCents    int64             `json:"paid_cents"`
	DueCents     int64             `json:"due_cents"`
	RefundCents  int64             `json:"refund_cents"`
	FeeLines     []AccountFeeLine  `json:"fee_lines"`
	Shipments    []AccountShipment `json:"shipments"`
}

type AccountFeeSummary struct {
	GoodsCents             int64 `json:"goods_cents"`
	ProcessingCents        int64 `json:"processing_cents"`
	DirectShipServiceCents int64 `json:"direct_ship_service_cents"`
	ShippingCents          int64 `json:"shipping_cents"`
	OtherCents             int64 `json:"other_cents"`
	PayableCents           int64 `json:"payable_cents"`
	PaidCents              int64 `json:"paid_cents"`
	DueCents               int64 `json:"due_cents"`
	RefundCents            int64 `json:"refund_cents"`
}

type AccountStatementDispute struct {
	ID                int64  `json:"id"`
	SettlementID      int64  `json:"settlement_id"`
	FeeItemID         int64  `json:"fee_item_id,omitempty"`
	StatementRevision string `json:"statement_revision"`
	Reason            string `json:"reason"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
	Reply             string `json:"reply,omitempty"`
	RepliedBy         string `json:"replied_by,omitempty"`
	RepliedAt         string `json:"replied_at,omitempty"`
}

type AccountSettlement struct {
	ID                   int64                     `json:"id"`
	SettlementNo         string                    `json:"settlement_no"`
	PeriodFrom           string                    `json:"period_from"`
	PeriodTo             string                    `json:"period_to"`
	Status               string                    `json:"status"`
	TotalCents           int64                     `json:"total_cents"`
	ConfirmedAt          string                    `json:"confirmed_at"`
	PaidAt               string                    `json:"paid_at"`
	Fees                 []AccountFee              `json:"fees,omitempty"`
	Lines                []SettlementLineSummary   `json:"lines,omitempty"`
	StatementRevision    string                    `json:"statement_revision"`
	ReconciliationStatus string                    `json:"reconciliation_status"`
	ReconciledAt         string                    `json:"reconciled_at,omitempty"`
	Disputes             []AccountStatementDispute `json:"disputes,omitempty"`
}
type AccountData struct {
	CustomerName   string                 `json:"customer_name"`
	DateFrom       string                 `json:"date_from"`
	DateTo         string                 `json:"date_to"`
	AsOf           string                 `json:"as_of"`
	Rows           []AccountOrder         `json:"rows"`
	Summary        AccountSummary         `json:"summary"`
	Fees           []AccountFee           `json:"fees"`
	Settlements    []AccountSettlement    `json:"settlements"`
	Total          int                    `json:"total"`
	Page           int                    `json:"page"`
	Limit          int                    `json:"limit"`
	TotalPages     int                    `json:"total_pages"`
	View           string                 `json:"view,omitempty"`
	ChargeType     string                 `json:"charge_type,omitempty"`
	UnpaidOnly     bool                   `json:"unpaid_only,omitempty"`
	FeeSummary     AccountFeeSummary      `json:"fee_summary,omitempty"`
	OrderFeeGroups []AccountOrderFeeGroup `json:"order_fee_groups,omitempty"`
}

func NormalizeAccountQuery(q AccountQuery) (AccountQuery, error) {
	if q.UnpaidOnly && (q.View != "order_fees" || q.DateFrom != "" || q.DateTo != "" || q.Period != "" || q.Anchor != "" || q.PayStatus != "" || q.ShipStatus != "" || q.Query != "" || q.OrderID != 0) {
		return q, fmt.Errorf("所有未付不支持同时指定账期或订单筛选")
	}
	if q.View != "" && q.View != "order_fees" {
		return q, fmt.Errorf("费用视图无效")
	}
	if q.ChargeType != "" {
		switch q.ChargeType {
		case "payable", "product", "processing", "direct_ship_service", "shipping":
		default:
			return q, fmt.Errorf("费用类型无效")
		}
	}
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	if q.Period != "" {
		anchor := time.Now().In(loc)
		if q.Anchor != "" {
			v, err := time.ParseInLocation("2006-01-02", q.Anchor, loc)
			if err != nil {
				return q, fmt.Errorf("账期日期无效")
			}
			anchor = v
		}
		var start, end time.Time
		switch q.Period {
		case "week":
			start = anchor.AddDate(0, 0, -(int(anchor.Weekday())+6)%7)
			end = start.AddDate(0, 0, 6)
		case "month":
			start = time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, loc)
			end = start.AddDate(0, 1, -1)
		default:
			return q, fmt.Errorf("账期类型无效")
		}
		q.DateFrom = start.Format("2006-01-02")
		q.DateTo = end.Format("2006-01-02")
	}
	for _, v := range []string{q.DateFrom, q.DateTo} {
		if v != "" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				return q, fmt.Errorf("日期无效")
			}
		}
	}
	if q.DateFrom != "" && q.DateTo != "" && q.DateFrom > q.DateTo {
		return q, fmt.Errorf("开始日期不能晚于结束日期")
	}
	if q.PayStatus != "" && q.PayStatus != "unpaid" && q.PayStatus != "partial" && q.PayStatus != "paid" {
		return q, fmt.Errorf("付款状态无效")
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 20
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	return q, nil
}
func (s *Service) CustomerAccount(ctx context.Context, q AccountQuery) (AccountData, error) {
	var err error
	q, err = NormalizeAccountQuery(q)
	if err != nil {
		return AccountData{}, err
	}
	if q.CustomerID <= 0 {
		return AccountData{}, fmt.Errorf("客户未绑定")
	}
	repo, ok := s.repo.(interface {
		CustomerAccount(context.Context, AccountQuery) (AccountData, error)
	})
	if !ok {
		return AccountData{}, fmt.Errorf("客户账目服务不可用")
	}
	return repo.CustomerAccount(ctx, q)
}

type ConfirmCustomerStatementCommand struct {
	CustomerID        int64
	SettlementID      int64
	StatementRevision string
	MiniUserID        int64
	Actor             string
}

type CreateCustomerStatementDisputeCommand struct {
	CustomerID        int64
	SettlementID      int64
	FeeItemID         int64
	StatementRevision string
	Reason            string
	MiniUserID        int64
	Actor             string
}

type ReplyCustomerStatementDisputeCommand struct {
	DisputeID int64
	Reply     string
	Status    string
	Actor     string
}

type customerStatementRepository interface {
	ConfirmCustomerStatement(context.Context, ConfirmCustomerStatementCommand) (AccountSettlement, error)
	CreateCustomerStatementDispute(context.Context, CreateCustomerStatementDisputeCommand) (AccountStatementDispute, error)
	ReplyCustomerStatementDispute(context.Context, ReplyCustomerStatementDisputeCommand) (AccountStatementDispute, error)
}

func (s *Service) ConfirmCustomerStatement(ctx context.Context, cmd ConfirmCustomerStatementCommand) (AccountSettlement, error) {
	cmd.StatementRevision = strings.TrimSpace(cmd.StatementRevision)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.CustomerID <= 0 || cmd.SettlementID <= 0 || cmd.StatementRevision == "" {
		return AccountSettlement{}, fmt.Errorf("账单和版本信息不能为空")
	}
	repo, ok := s.repo.(customerStatementRepository)
	if !ok {
		return AccountSettlement{}, fmt.Errorf("对账服务不可用")
	}
	return repo.ConfirmCustomerStatement(ctx, cmd)
}

func (s *Service) CreateCustomerStatementDispute(ctx context.Context, cmd CreateCustomerStatementDisputeCommand) (AccountStatementDispute, error) {
	cmd.StatementRevision = strings.TrimSpace(cmd.StatementRevision)
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.CustomerID <= 0 || cmd.SettlementID <= 0 || cmd.StatementRevision == "" || cmd.Reason == "" {
		return AccountStatementDispute{}, fmt.Errorf("账单版本和异议原因不能为空")
	}
	if len([]rune(cmd.Reason)) > 1000 {
		return AccountStatementDispute{}, fmt.Errorf("异议原因过长")
	}
	repo, ok := s.repo.(customerStatementRepository)
	if !ok {
		return AccountStatementDispute{}, fmt.Errorf("对账服务不可用")
	}
	return repo.CreateCustomerStatementDispute(ctx, cmd)
}

func (s *Service) ReplyCustomerStatementDispute(ctx context.Context, cmd ReplyCustomerStatementDisputeCommand) (AccountStatementDispute, error) {
	cmd.Reply = strings.TrimSpace(cmd.Reply)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	cmd.Status = strings.TrimSpace(cmd.Status)
	if cmd.DisputeID <= 0 || cmd.Reply == "" || cmd.Actor == "" {
		return AccountStatementDispute{}, fmt.Errorf("异议、回复和操作人不能为空")
	}
	if cmd.Status == "" {
		cmd.Status = "replied"
	}
	if cmd.Status != "replied" && cmd.Status != "resolved" && cmd.Status != "closed" {
		return AccountStatementDispute{}, fmt.Errorf("异议状态无效")
	}
	repo, ok := s.repo.(customerStatementRepository)
	if !ok {
		return AccountStatementDispute{}, fmt.Errorf("对账服务不可用")
	}
	return repo.ReplyCustomerStatementDispute(ctx, cmd)
}
func (d *AccountData) Paginate(page, limit int) {
	if d.View == "order_fees" {
		filtered := FilterOrderFeeGroups(d.OrderFeeGroups, d.ChargeType)
		d.OrderFeeGroups, d.Total, d.TotalPages = PaginateOrderFeeGroups(filtered, page, limit)
		d.Limit = limit
		d.Page = max(1, min(page, d.TotalPages))
		d.Rows = make([]AccountOrder, 0, len(d.OrderFeeGroups))
		for _, group := range d.OrderFeeGroups {
			if !group.Unlinked {
				d.Rows = append(d.Rows, group.Order)
			}
		}
		return
	}
	d.Total = len(d.Rows)
	d.Limit = limit
	d.TotalPages = max(1, (d.Total+limit-1)/limit)
	page = max(1, min(page, d.TotalPages))
	d.Page = page
	start := min((page-1)*limit, d.Total)
	end := min(start+limit, d.Total)
	d.Rows = d.Rows[start:end]
}

func BuildOrderFeeGroups(orders []AccountOrder, fees []AccountFee) ([]AccountOrderFeeGroup, AccountFeeSummary) {
	groups := make([]AccountOrderFeeGroup, 0, len(orders)+1)
	byOrder := make(map[int64]int, len(orders))
	for _, order := range orders {
		if order.ID <= 0 {
			continue
		}
		byOrder[order.ID] = len(groups)
		groups = append(groups, AccountOrderFeeGroup{Order: order, FeeLines: []AccountFeeLine{}, Shipments: []AccountShipment{}})
	}
	add := func(group *AccountOrderFeeGroup, fee AccountFee) {
		category := accountFeeCategory(fee.FeeType)
		group.FeeLines = append(group.FeeLines, AccountFeeLine{AccountFee: fee, Category: category})
		if fee.AmountCents < 0 {
			group.RefundCents += -fee.AmountCents
		}
		group.PayableCents += fee.AmountCents
	}
	for i := range groups {
		g := &groups[i]
		if !g.Order.InPeriod {
			continue
		}
		goods := g.Order.TotalCents - g.Order.ShippingCents
		if goods != 0 {
			add(g, AccountFee{OrderID: g.Order.ID, OrderNo: g.Order.OrderNo, FeeType: "product", FeeName: "商品货款", AmountCents: goods, OccurredAt: g.Order.OrderDate, SourceType: "order", SourceID: g.Order.ID, PaymentStatus: g.Order.PaymentStatus, IncludedInOrder: true})
		}
		if g.Order.ShippingCents != 0 && !g.Order.HasOrderShippingFee {
			add(g, AccountFee{OrderID: g.Order.ID, OrderNo: g.Order.OrderNo, FeeType: "shipping", FeeName: "订单运费", AmountCents: g.Order.ShippingCents, OccurredAt: g.Order.OrderDate, SourceType: "order", SourceID: g.Order.ID, PaymentStatus: g.Order.PaymentStatus, IncludedInOrder: true})
		}
	}
	for _, fee := range fees {
		if fee.IncludedInOrder && (fee.FeeType == "product" || fee.FeeType == "order_goods") {
			continue
		}
		index, ok := byOrder[fee.OrderID]
		if !ok {
			if fee.OrderID > 0 && fee.OrderNo != "" {
				order := AccountOrder{ID: fee.OrderID, OrderNo: fee.OrderNo}
				byOrder[fee.OrderID] = len(groups)
				groups = append(groups, AccountOrderFeeGroup{Order: order, FeeLines: []AccountFeeLine{}, Shipments: []AccountShipment{}})
				index, ok = byOrder[fee.OrderID]
			} else {
				index = len(groups)
				byOrder[0] = index
				groups = append(groups, AccountOrderFeeGroup{Order: AccountOrder{OrderNo: "未关联订单费用"}, Unlinked: true, FeeLines: []AccountFeeLine{}, Shipments: []AccountShipment{}})
				ok = true
			}
		}
		if !ok {
			continue
		}
		add(&groups[index], fee)
	}
	var summary AccountFeeSummary
	for i := range groups {
		g := &groups[i]
		basePayable := int64(0)
		paid := int64(0)
		if g.Order.InPeriod {
			for _, line := range g.FeeLines {
				if line.Category == "product" || line.IncludedInOrder {
					basePayable += line.AmountCents
				}
			}
			paid = min(g.Order.PaidCents, max(int64(0), basePayable))
		}
		for _, line := range g.FeeLines {
			if !line.IncludedInOrder && line.PaymentStatus == "paid" && line.AmountCents > 0 {
				paid += line.AmountCents
			}
		}
		g.PaidCents = paid
		g.DueCents = max(int64(0), g.PayableCents-g.PaidCents)
		for _, line := range g.FeeLines {
			amount := line.AmountCents
			switch line.Category {
			case "product":
				summary.GoodsCents += amount
			case "processing":
				summary.ProcessingCents += amount
			case "direct_ship_service":
				summary.DirectShipServiceCents += amount
			case "shipping":
				summary.ShippingCents += amount
			default:
				summary.OtherCents += amount
			}
		}
		summary.PayableCents += g.PayableCents
		summary.PaidCents += g.PaidCents
		summary.DueCents += g.DueCents
		summary.RefundCents += g.RefundCents
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Unlinked != groups[j].Unlinked {
			return !groups[i].Unlinked
		}
		if groups[i].Order.OrderDate != groups[j].Order.OrderDate {
			return groups[i].Order.OrderDate > groups[j].Order.OrderDate
		}
		return groups[i].Order.ID > groups[j].Order.ID
	})
	return groups, summary
}

func accountFeeCategory(feeType string) string {
	switch feeType {
	case "product", "order_goods":
		return "product"
	case "processing", "roasting", "labor", "material", "packaging":
		return "processing"
	case "direct_ship_service":
		return "direct_ship_service"
	case "shipping":
		return "shipping"
	default:
		return "other"
	}
}

func ApplyUnpaidSources(groups []AccountOrderFeeGroup, sources []AccountUnpaidSource) ([]AccountOrderFeeGroup, AccountFeeSummary) {
	key := func(kind, source string, id int64) string { return fmt.Sprintf("%s:%s:%d", kind, source, id) }
	byKey := make(map[string]AccountUnpaidSource, len(sources))
	keys := make([]string, 0, len(sources))
	for _, source := range sources {
		if source.DueCents <= 0 {
			continue
		}
		k := key(source.ChargeType, source.SourceType, source.SourceID)
		if old, exists := byKey[k]; !exists {
			keys = append(keys, k)
		} else if old.DueCents >= source.DueCents {
			continue
		}
		byKey[k] = source
	}
	out := make([]AccountOrderFeeGroup, 0, len(groups))
	var summary AccountFeeSummary
	add := func(group *AccountOrderFeeGroup, line AccountFeeLine, source AccountUnpaidSource) {
		line.AmountCents = source.DueCents
		line.ReceivableCents, line.ReceivedCents = source.ReceivableCents, source.ReceivedCents
		if source.ReceivedCents > 0 {
			line.PaymentStatus = "partial"
		} else {
			line.PaymentStatus = "unpaid"
		}
		group.FeeLines = append(group.FeeLines, line)
		group.PayableCents += source.DueCents
		group.DueCents += source.DueCents
		switch line.Category {
		case "product":
			summary.GoodsCents += source.DueCents
		case "processing":
			summary.ProcessingCents += source.DueCents
		case "direct_ship_service":
			summary.DirectShipServiceCents += source.DueCents
		case "shipping":
			summary.ShippingCents += source.DueCents
		default:
			summary.OtherCents += source.DueCents
		}
		summary.PayableCents += source.DueCents
		summary.DueCents += source.DueCents
	}
	for _, original := range groups {
		group := original
		group.FeeLines = nil
		group.PayableCents = 0
		group.PaidCents = 0
		group.DueCents = 0
		group.RefundCents = 0
		for _, line := range original.FeeLines {
			k := key(line.FeeType, "customer_fee_item", line.ID)
			if line.Category == "product" && line.SourceType == "order" {
				k = key("order_goods", "order", original.Order.ID)
			}
			source, found := byKey[k]
			if !found {
				continue
			}
			add(&group, line, source)
			delete(byKey, k)
		}
		if len(group.FeeLines) > 0 {
			out = append(out, group)
		}
	}
	// Legacy orders can have their receivable in total_amount while the old
	// account snapshot has no merchandise line. Keep their order association.
	for _, k := range keys {
		source, found := byKey[k]
		if !found || source.ChargeType != "order_goods" || source.SourceType != "order" {
			continue
		}
		var original *AccountOrderFeeGroup
		for i := range groups {
			if groups[i].Order.ID == source.SourceID {
				original = &groups[i]
				break
			}
		}
		if original == nil {
			continue
		}
		index := -1
		for i := range out {
			if out[i].Order.ID == source.SourceID {
				index = i
				break
			}
		}
		if index < 0 {
			group := *original
			group.FeeLines = nil
			group.PayableCents = 0
			group.PaidCents = 0
			group.DueCents = 0
			group.RefundCents = 0
			out = append(out, group)
			index = len(out) - 1
		}
		line := AccountFeeLine{AccountFee: AccountFee{OrderID: source.SourceID, OrderNo: original.Order.OrderNo, FeeType: "product", FeeName: "商品货款", SourceType: "order", SourceID: source.SourceID, OccurredAt: source.OccurredAt, IncludedInOrder: true}, Category: "product"}
		add(&out[index], line, source)
		delete(byKey, k)
	}
	var orphan *AccountOrderFeeGroup
	for i := range out {
		if out[i].Unlinked {
			orphan = &out[i]
			break
		}
	}
	if orphan == nil {
		out = append(out, AccountOrderFeeGroup{Order: AccountOrder{OrderNo: "未关联订单费用"}, Unlinked: true})
		orphan = &out[len(out)-1]
	}
	for _, k := range keys {
		source, found := byKey[k]
		if !found {
			continue
		}
		name := source.Description
		if name == "" {
			name = AccountFeeLabel(source.ChargeType)
		}
		line := AccountFeeLine{AccountFee: AccountFee{ID: source.SourceID, SourceType: source.SourceType, SourceID: source.SourceID, FeeType: source.ChargeType, FeeName: name, OccurredAt: source.OccurredAt, OrderNo: source.SourceNo}, Category: accountFeeCategory(source.ChargeType)}
		add(orphan, line, source)
	}
	if len(orphan.FeeLines) == 0 {
		out = out[:len(out)-1]
	}
	return out, summary
}

func FilterOrderFeeGroups(groups []AccountOrderFeeGroup, chargeType string) []AccountOrderFeeGroup {
	if chargeType == "" || chargeType == "payable" {
		return groups
	}
	out := make([]AccountOrderFeeGroup, 0, len(groups))
	for _, group := range groups {
		lines := make([]AccountFeeLine, 0, len(group.FeeLines))
		for _, line := range group.FeeLines {
			if line.Category == chargeType {
				lines = append(lines, line)
			}
		}
		if len(lines) == 0 {
			continue
		}
		group.FeeLines = lines
		out = append(out, group)
	}
	return out
}

func PaginateOrderFeeGroups(groups []AccountOrderFeeGroup, page, limit int) ([]AccountOrderFeeGroup, int, int) {
	if limit < 1 {
		limit = 20
	}
	total := len(groups)
	totalPages := max(1, (total+limit-1)/limit)
	page = max(1, min(page, totalPages))
	start := min((page-1)*limit, total)
	end := min(start+limit, total)
	return groups[start:end], total, totalPages
}

func AccountFeeLabel(value string) string {
	if label, ok := map[string]string{
		"order_goods": "订单款项", "product": "商品货款", "processing": "加工费", "roasting": "烘焙加工费", "labor": "人工加工费",
		"material": "加工物料费", "packaging": "包装费", "direct_ship_service": "代发服务费",
		"storage": "仓储费", "shipping": "运费", "adjustment": "调整",
	}[value]; ok {
		return label
	}
	return value
}
