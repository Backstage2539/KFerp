package freight

import "testing"

func TestOrderFreightManualAndFree(t *testing.T) {
	amount := int64(2000)
	free := true
	p := Policy{CustomerID: 1, Scene: "sale"}
	q, e := QuoteOrder(p, Template{}, Destination{}, 0, &Override{ManualAmountCents: &amount}, true)
	if e != nil || q.Protocol != OrderProtocol || q.ChargeMode != "manual_current" || q.CurrentReceivableCents != 2000 || q.EstimateKnown {
		t.Fatalf("manual: %+v %v", q, e)
	}
	for _, actual := range []int64{1200, 1300, 0} {
		d, e := Settlement(q, actual, 0)
		if e != nil || d.DeltaCents != 0 || d.TargetCents != 0 {
			t.Fatalf("duplicate billing: %+v %v", d, e)
		}
	}
	q, e = QuoteOrder(p, Template{}, Destination{}, 0, &Override{ManualAmountCents: &amount, FreeShipping: &free}, true)
	if e != nil || q.ChargeMode != "free" || q.CurrentReceivableCents != 0 {
		t.Fatalf("free: %+v %v", q, e)
	}
	amount = 0
	q, e = QuoteOrder(p, Template{}, Destination{}, 0, &Override{ManualAmountCents: &amount}, true)
	if e != nil || q.ChargeMode != "manual_current" || q.ManualAmountCents == nil {
		t.Fatal(q, e)
	}
	amount = -1
	if _, e = QuoteOrder(p, Template{}, Destination{}, 0, &Override{ManualAmountCents: &amount}, true); e == nil {
		t.Fatal("negative accepted")
	}
	amount = 2000
	if _, e = QuoteOrder(p, Template{}, Destination{}, 0, &Override{ManualAmountCents: &amount}, false); e == nil {
		t.Fatal("customer override accepted")
	}
}
func TestDestinationSearchAndResolve(t *testing.T) {
	for _, query := range []string{"menglianxian", "mlx"} {
		ok := false
		for _, r := range SearchRegions(query) {
			if r.Code == "530827" {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("alias pronunciation %s not searchable", query)
		}
	}
	found := false
	for _, r := range SearchRegions("孟连县") {
		if r.Code == "530827" {
			found = true
		}
	}
	if !found {
		t.Fatal("孟连县 not searchable")
	}
	r := ResolveAddress("云南省普洱市孟连县测试路8号")
	if r.Status != "resolved" || r.Destination.DistrictCode != "530827" || r.Destination.City != "普洱市" {
		t.Fatalf("%+v", r)
	}
	r = ResolveAddress("鼓楼区测试路8号")
	if r.Status != "ambiguous" || len(r.Candidates) < 2 {
		t.Fatalf("ambiguous %+v", r)
	}
	r = ResolveAddress("上海市浦东新区测试路1号")
	if r.Status != "resolved" || r.Destination.Province != "上海市" {
		t.Fatalf("municipality %+v", r)
	}
}
