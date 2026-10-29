package customerfulfillment

import (
	"strings"
	"testing"
)

func TestBillingDetailUsesOnlyMatchedShipmentAndPreservesAmount(t *testing.T) {
	line := SettlementLineSummary{ChargeType: "shipping", SourceType: "customer_fee_item", SourceID: 21, AmountCents: 1110}
	fee := AccountFee{ID: 21, OrderID: 7, OrderNo: "SO-7", TrackingNo: "SF-A"}
	order := AccountOrder{ID: 7, OrderNo: "SO-7"}
	shipments := []AccountShipment{{OrderID: 7, TrackingNo: "SF-A", ShippedAt: "2026-10-26 10:00", SettlementWeightKG: "1.2"}, {OrderID: 7, TrackingNo: "SF-B"}}
	detail := BuildBillingLineDetail(line, order, fee, shipments, "原单当前信息")
	if detail.OrderNo != "SO-7" || detail.AmountCents != 1110 || len(detail.Shipments) != 1 || detail.Shipments[0].TrackingNo != "SF-A" || detail.InformationSource != "原单当前信息" {
		t.Fatalf("incorrect bill detail: %+v", detail)
	}
}

func TestBillingDetailDoesNotAddGoodsToServiceFee(t *testing.T) {
	line := SettlementLineSummary{ChargeType: "direct_ship_service", AmountCents: 300}
	order := AccountOrder{ID: 7, OrderNo: "SO-7", Items: []AccountOrderItem{{Name: "咖啡豆"}}}
	detail := BuildBillingLineDetail(line, order, AccountFee{}, nil, "出账快照")
	if len(detail.Items) != 0 || detail.AmountCents != 300 {
		t.Fatalf("service line leaked goods: %+v", detail)
	}
}

func TestBillingDetailTextShowsMissingFieldsWithoutInventingThem(t *testing.T) {
	d := BillingLineDetail{OrderNo: "SO-7", InformationSource: "原单当前信息", Shipments: []AccountShipment{{TrackingNo: "SF-1"}}}
	got := BillingDetailText(d)
	joined := strings.Join(got, " ")
	for _, want := range []string{"SO-7", "原单当前信息", "SF-1", "未记录"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("detail %q missing %q", joined, want)
		}
	}
}
