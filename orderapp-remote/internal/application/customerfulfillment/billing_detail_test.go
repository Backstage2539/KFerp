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

func TestOtherFeeAppearsOnlyInBilledOrderDetail(t *testing.T) {
	order := AccountOrder{ID: 9, OtherFeeCents: 2350, OtherFeeNote: "定制标签"}
	for _, tt := range []struct {
		charge  string
		amount  int64
		wantFee int64
	}{{"order_goods", 12350, 2350}, {"shipping", 2000, 0}} {
		detail := BuildBillingLineDetail(SettlementLineSummary{ChargeType: tt.charge, AmountCents: tt.amount}, order, AccountFee{}, nil, "出账快照")
		if detail.AmountCents != tt.amount || detail.OtherFeeCents != tt.wantFee {
			t.Fatalf("%s detail: %+v", tt.charge, detail)
		}
		shown := strings.Contains(strings.Join(BillingDetailText(detail), "\n"), "其他费用：¥23.50（定制标签）")
		if shown != (tt.wantFee != 0) {
			t.Fatalf("%s detail text: %v", tt.charge, BillingDetailText(detail))
		}
	}
}

func TestOrderFeeGroupIncludesOtherFeeInOrderAmountOnce(t *testing.T) {
	order := AccountOrder{ID: 1, TotalCents: 15200, ShippingCents: 1200, OtherFeeCents: 4000, OtherFeeNote: "贴标", PaidCents: 5000, InPeriod: true}
	groups, summary := BuildOrderFeeGroups([]AccountOrder{order}, nil)
	if summary.PayableCents != 15200 || summary.DueCents != 10200 || len(groups[0].FeeLines) != 2 || !strings.Contains(groups[0].FeeLines[0].FeeName, "其他费用：¥40.00（贴标）") {
		t.Fatalf("customer order charges: %+v, %+v", groups, summary)
	}
}

func TestBillingGroupShowsOnlySelectedPackages(t *testing.T) {
	order := AccountOrder{ID: 12}
	fee := AccountFee{TrackingNo: "SF-A\nSF-C"}
	shipments := []AccountShipment{{OrderID: 12, TrackingNo: "SF-A"}, {OrderID: 12, TrackingNo: "SF-B"}, {OrderID: 12, TrackingNo: "SF-C"}}
	detail := BuildBillingLineDetail(SettlementLineSummary{ChargeType: "shipping"}, order, fee, shipments, "出账快照")
	if len(detail.Shipments) != 2 || detail.Shipments[0].TrackingNo != "SF-A" || detail.Shipments[1].TrackingNo != "SF-C" {
		t.Fatalf("group packages %+v", detail.Shipments)
	}
}
