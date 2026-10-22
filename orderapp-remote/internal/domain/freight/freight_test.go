package freight

import "testing"

func sfTemplate() Template {
	t := Template{ID: 1, Revision: 1, Active: true, Name: "SF development fixture", CarrierID: 1, ProductID: 1}
	groups := []struct {
		codes []string
		fees  []int64
		extra int64
	}{
		{[]string{"530000"}, []int64{510, 610, 710, 810, 910}, 200},
		{[]string{"500000", "510000", "520000"}, []int64{550, 750, 950, 1150, 1350}, 200},
		{[]string{"340000", "110000", "350000", "440000", "450000", "460000", "130000", "420000", "430000", "320000", "360000", "310000", "120000", "330000"}, []int64{610, 810, 1010, 1210, 1410}, 200},
		{[]string{"620000", "410000", "230000", "220000", "210000", "150000", "640000", "630000", "370000", "140000", "610000"}, []int64{1000, 1600, 2200, 2900, 3600}, 300},
		{[]string{"650000"}, []int64{1300, 2100, 2900, 3900, 4900}, 400},
	}
	for _, g := range groups {
		v := Group{Name: g.codes[0], RegionCodes: g.codes, AdditionalWeightG: 1000, AdditionalFeeCents: g.extra}
		for i, f := range g.fees {
			v.Tiers = append(v.Tiers, Tier{UpperWeightG: int64(i+1) * 1000, FeeCents: f})
		}
		t.Groups = append(t.Groups, v)
	}
	return t
}
func TestSFPictureRatesAndBoundaries(t *testing.T) {
	tm := sfTemplate()
	if err := ValidateTemplate(tm); err != nil {
		t.Fatal(err)
	}
	for _, g := range tm.Groups {
		for _, code := range g.RegionCodes {
			for i, tier := range g.Tiers {
				for _, w := range []int64{int64(i)*1000 + 1, tier.UpperWeightG} {
					q, e := Calculate(tm, Destination{ProvinceCode: code}, w)
					if e != nil || q.BaseCents != tier.FeeCents {
						t.Fatalf("%s weight%d: %+v %v", code, w, q, e)
					}
				}
			}
			for _, w := range []int64{5001, 6000, 6001} {
				q, e := Calculate(tm, Destination{ProvinceCode: code}, w)
				want := g.Tiers[4].FeeCents + ((w-5000+999)/1000)*g.AdditionalFeeCents
				if e != nil || q.BaseCents != want {
					t.Fatalf("continuation %+v %v", q, e)
				}
			}
		}
	}
	for _, c := range []string{"540000", "710000", "810000", "820000"} {
		if _, e := Calculate(tm, Destination{ProvinceCode: c}, 1000); e == nil {
			t.Fatalf("uncovered %s", c)
		}
	}
	tm.PackagingWeightG = 100
	q, e := Calculate(tm, Destination{Province: "云南"}, 2*454)
	if e != nil || q.WeightG != 1008 || q.BaseCents != 610 {
		t.Fatalf("one packaging %+v %v", q, e)
	}
}
func TestRegionsSpecificityAndConflict(t *testing.T) {
	tm := sfTemplate()
	tm.Groups = append(tm.Groups, Group{Name: "大理例外", RegionCodes: []string{"532901"}, Tiers: []Tier{{UpperWeightG: 1000, FeeCents: 999}}, AdditionalWeightG: 1000, AdditionalFeeCents: 200})
	q, e := Calculate(tm, Destination{Province: "云南省", City: "大理白族自治州", District: "大理市"}, 454)
	if e != nil || q.BaseCents != 999 {
		t.Fatalf("specific %+v %v", q, e)
	}
	tm.Groups[1].RegionCodes = append(tm.Groups[1].RegionCodes, "530000")
	if ValidateTemplate(tm) == nil {
		t.Fatal("overlap accepted")
	}
	if _, e = ResolveDestination(Destination{District: "鼓楼区"}); e == nil {
		t.Fatal("ambiguous text guessed")
	}
	for _, query := range []string{"云南", "yunnan", "yn", "530000"} {
		if len(SearchRegions(query)) == 0 {
			t.Fatalf("search %s", query)
		}
	}
}
func TestDeferredFreightAndDelta(t *testing.T) {
	tm := sfTemplate()
	tm.Groups[0].Tiers[0].FeeCents = 1000
	p := Policy{Scene: "stock", TemplateID: 1, Revision: 2, SurchargeCents: 1000}
	q, e := Quote(p, tm, Destination{Province: "云南省"}, 454)
	if e != nil || q.EstimatedTotalCents != 2000 || q.CurrentReceivableCents != 0 || q.Status != "pending_actual" {
		t.Fatalf("quote %+v %v", q, e)
	}
	a, e := Settlement(q, 1200, 0)
	if e != nil || a.DeltaCents != 2200 {
		t.Fatalf("initial %+v %v", a, e)
	}
	a, e = Settlement(q, 1300, 2200)
	if e != nil || a.DeltaCents != 100 {
		t.Fatalf("delta %+v %v", a, e)
	}
	p.FreeShipping = true
	q, e = Quote(p, tm, Destination{}, 0)
	if e != nil || q.Status != "free" || q.EstimateKnown || q.EstimatedTotalCents != 0 {
		t.Fatalf("free unknown %+v %v", q, e)
	}
	a, e = Settlement(q, 1300, 0)
	if e != nil || a.DeltaCents != 0 || a.ActualCents != 1300 {
		t.Fatal(a, e)
	}
	p.FreeShipping = false
	if _, e = Quote(p, tm, Destination{}, 0); e == nil {
		t.Fatal("missing config zeroed")
	}
}

func TestRegionSearchSupportsCommonPolyphonicPlaceNames(t *testing.T) {
	for _, v := range []struct{ query, code string }{{"chongqing", "500000"}, {"cq", "500000"}, {"changsha", "430100"}, {"xiamen", "350200"}, {"liuan", "341500"}} {
		found := false
		for _, r := range SearchRegions(v.query) {
			if r.Code == v.code {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s did not find %s", v.query, v.code)
		}
	}
}

func TestStoredPolicyInheritanceDoesNotAllowUnpricedQuote(t *testing.T) {
	p := Policy{CustomerID: 5, Scene: "stock"}
	if err := ValidatePolicyOverride(p); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePolicy(p); err == nil {
		t.Fatal("unresolved inherited policy cannot quote")
	}
	p.SurchargeCents = 100
	if err := ValidatePolicyOverride(p); err == nil {
		t.Fatal("inheritance cannot silently discard surcharge")
	}
	p.SurchargeCents = 0
	p.TemplateID = -1
	if err := ValidatePolicyOverride(p); err == nil {
		t.Fatal("negative template accepted")
	}
}
