package customerfulfillment

import (
	"fmt"
	"strings"
)

// BillingLineDetail explains one existing receivable; it never creates another charge.
type BillingLineDetail struct {
	OrderID           int64              `json:"order_id,omitempty"`
	OrderNo           string             `json:"order_no,omitempty"`
	ChargeType        string             `json:"charge_type"`
	AmountCents       int64              `json:"amount_cents"`
	DiscountCents     int64              `json:"discount_cents,omitempty"`
	InformationSource string             `json:"information_source"`
	Items             []AccountOrderItem `json:"items,omitempty"`
	Shipments         []AccountShipment  `json:"shipments,omitempty"`
}

// BuildBillingLineDetail only adds context to a selected bill line. In
// particular, a freight charge linked to a waybill cannot borrow other
// shipments from the same order.
func BuildBillingLineDetail(line SettlementLineSummary, order AccountOrder, fee AccountFee, shipments []AccountShipment, source string) BillingLineDetail {
	detail := BillingLineDetail{OrderID: order.ID, OrderNo: order.OrderNo, ChargeType: line.ChargeType, AmountCents: line.AmountCents, InformationSource: source}
	if line.ChargeType == "order_goods" || line.ChargeType == "product" {
		detail.DiscountCents = order.DiscountCents
		detail.Items = append(detail.Items, order.Items...)
		return detail
	}
	if line.ChargeType != "shipping" && line.ChargeType != "direct_ship_service" {
		return detail
	}
	for _, shipment := range shipments {
		if shipment.OrderID != order.ID || order.ID == 0 {
			continue
		}
		if line.ChargeType == "shipping" && (fee.TrackingNo == "" || fee.TrackingNo != shipment.TrackingNo) {
			continue
		}
		// Amounts belong to the bill line, not the possibly changed source.
		shipment.ShippingCents = 0
		detail.Shipments = append(detail.Shipments, shipment)
	}
	return detail
}

// BillingDetailText is shared by document renderers and contains no new charges.
func BillingDetailText(detail BillingLineDetail) []string {
	value := func(s string) string {
		if strings.TrimSpace(s) == "" {
			return "未记录"
		}
		return s
	}
	lines := []string{fmt.Sprintf("订单：%s（%s）", value(detail.OrderNo), value(detail.InformationSource))}
	for _, item := range detail.Items {
		lines = append(lines, fmt.Sprintf("商品：%s　规格：%s　数量：%s%s　单价：%s　金额：%s", value(item.Name), value(item.Spec), value(item.Quantity), item.Unit, value(item.UnitPrice), value(item.LineTotal)))
	}
	if (detail.ChargeType == "order_goods" || detail.ChargeType == "product") && len(detail.Items) == 0 {
		lines = append(lines, "商品明细：未记录")
	}
	if len(detail.Items) > 0 {
		lines = append(lines, fmt.Sprintf("订单优惠：¥%.2f", float64(detail.DiscountCents)/100))
	}
	for _, shipment := range detail.Shipments {
		lines = append(lines, fmt.Sprintf("发货时间：%s　运单号：%s　寄件重量：%s kg", value(shipment.ShippedAt), value(shipment.TrackingNo), value(shipment.SettlementWeightKG)))
	}
	if (detail.ChargeType == "shipping" || detail.ChargeType == "direct_ship_service") && len(detail.Shipments) == 0 {
		lines = append(lines, "发货时间：未记录　运单号：未记录　寄件重量：未记录")
	}
	return lines
}
