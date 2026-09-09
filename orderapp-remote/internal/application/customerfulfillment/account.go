package customerfulfillment

import (
	"context"
	"fmt"
	salesdomain "orderapp/internal/domain/sales"
	"time"
)

type AccountQuery struct {
	CustomerID  int64
	OrderID     int64
	Query       string
	DateFrom    string
	DateTo      string
	Period      string
	Anchor      string
	PayStatus   string
	ShipStatus  string
	IncludeVoid bool
	Page        int
	Limit       int
}
type AccountOrder struct {
	ID              int64              `json:"id"`
	OrderNo         string             `json:"order_no"`
	OrderDate       string             `json:"order_date"`
	ReceiverName    string             `json:"receiver_name"`
	ReceiverPhone   string             `json:"receiver_phone"`
	ReceiverAddress string             `json:"receiver_address"`
	ShipStatus      string             `json:"ship_status"`
	TrackingNo      string             `json:"ship_tracking_no"`
	PayStatus       string             `json:"pay_status"`
	PaymentStatus   string             `json:"payment_status"`
	Service         string             `json:"portal_service_code"`
	IsVoid          bool               `json:"is_void"`
	GoodsCents      int64              `json:"goods_cents"`
	ShippingCents   int64              `json:"shipping_cents"`
	DiscountCents   int64              `json:"discount_cents"`
	TotalCents      int64              `json:"total_cents"`
	PrepaymentCents int64              `json:"prepayment_cents"`
	PaidCents       int64              `json:"paid_cents"`
	DueCents        int64              `json:"due_cents"`
	Items           []AccountOrderItem `json:"items,omitempty"`
}
type AccountOrderItem struct {
	Name      string `json:"item_name"`
	Spec      string `json:"spec"`
	Quantity  string `json:"qty"`
	Unit      string `json:"unit"`
	UnitPrice string `json:"unit_price"`
	LineTotal string `json:"line_total"`
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
	Count         int   `json:"count"`
	GoodsCents    int64 `json:"goods_cents"`
	ShippingCents int64 `json:"shipping_cents"`
	DiscountCents int64 `json:"discount_cents"`
	TotalCents    int64 `json:"total_cents"`
	PaidCents     int64 `json:"paid_cents"`
	DueCents      int64 `json:"due_cents"`
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
}
type AccountSettlement struct {
	ID           int64        `json:"id"`
	SettlementNo string       `json:"settlement_no"`
	PeriodFrom   string       `json:"period_from"`
	PeriodTo     string       `json:"period_to"`
	Status       string       `json:"status"`
	TotalCents   int64        `json:"total_cents"`
	ConfirmedAt  string       `json:"confirmed_at"`
	PaidAt       string       `json:"paid_at"`
	Fees         []AccountFee `json:"fees,omitempty"`
}
type AccountData struct {
	CustomerName string              `json:"customer_name"`
	DateFrom     string              `json:"date_from"`
	DateTo       string              `json:"date_to"`
	AsOf         string              `json:"as_of"`
	Rows         []AccountOrder      `json:"rows"`
	Summary      AccountSummary      `json:"summary"`
	Fees         []AccountFee        `json:"fees"`
	Settlements  []AccountSettlement `json:"settlements"`
	Total        int                 `json:"total"`
	Page         int                 `json:"page"`
	Limit        int                 `json:"limit"`
	TotalPages   int                 `json:"total_pages"`
}

func NormalizeAccountQuery(q AccountQuery) (AccountQuery, error) {
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
func (d *AccountData) Paginate(page, limit int) {
	d.Total = len(d.Rows)
	d.Limit = limit
	d.TotalPages = max(1, (d.Total+limit-1)/limit)
	page = max(1, min(page, d.TotalPages))
	d.Page = page
	start := min((page-1)*limit, d.Total)
	end := min(start+limit, d.Total)
	d.Rows = d.Rows[start:end]
}

func AccountFeeLabel(value string) string {
	if label, ok := map[string]string{"processing": "加工费", "packaging": "包装费", "direct_ship_service": "代发服务费", "storage": "仓储费", "shipping": "运费", "adjustment": "调整"}[value]; ok {
		return label
	}
	return value
}
