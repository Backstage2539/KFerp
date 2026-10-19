package freight

import "testing"

func TestOrdinaryOrderMissingLogisticsWeightHasExplicitUnknownEstimate(t *testing.T) {
	tm := Template{ID: 1, Active: true, Name: "测试", Rules: []LegacyRule{{Province: "云南省", FirstWeightG: 1000, FirstFeeCents: 800, AdditionalWeightG: 1000, AdditionalFeeCents: 200}}}
	p := Policy{CustomerID: 1, Scene: "sale", TemplateID: 1, SurchargeCents: 50}
	q, e := Quote(p, tm, Destination{Province: "云南省"}, 0)
	if e != nil || q.EstimateKnown || q.CostPendingReason == "" || q.CurrentReceivableCents != 0 || q.EstimatedTotalCents != 0 {
		t.Fatalf("unknown quote=%+v err=%v", q, e)
	}
	for _, destination := range []Destination{{}, {Province: "未覆盖地区"}} {
		if _, err := Quote(p, tm, destination, 0); err == nil {
			t.Fatal("missing logistics weight bypassed invalid destination")
		}
	}
	invalid := tm
	invalid.Rules = nil
	if _, err := Quote(p, invalid, Destination{Province: "云南省"}, 0); err == nil {
		t.Fatal("missing logistics weight bypassed invalid template")
	}
	p.Scene = "on_demand"
	if _, e = Quote(p, tm, Destination{Province: "云南省"}, 0); e == nil {
		t.Fatal("on-demand missing weight quoted")
	}
	p.Scene = "sale"
	p.TemplateID = 2
	if _, e = Quote(p, tm, Destination{Province: "云南省"}, 0); e == nil {
		t.Fatal("missing template bypassed")
	}
}

func TestFreightTokenIncludesManualWeightAndBOMVersion(t *testing.T) {
	g := 250.0
	q := Snapshot{ItemWeights: []WeightBasis{{ProductID: 1, BomSpecID: 10, BomVersionID: 20, Qty: 4, LogisticsUnitWeightG: &g}}}
	a := Token(q)
	q.ItemWeights[0].BomVersionID++
	if Token(q) == a {
		t.Fatal("BOM version change not detected")
	}
	a = Token(q)
	g = 250.001
	if Token(q) == a {
		t.Fatal("manual weight change not detected")
	}
}
