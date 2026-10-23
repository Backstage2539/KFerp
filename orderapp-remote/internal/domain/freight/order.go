package freight

import (
	"encoding/json"
	"fmt"
	"strings"
)

const OrderProtocol = "order_freight_v2"

// Explicit customer charges are independent of our estimated courier cost.
func QuoteOrder(p Policy, t Template, d Destination, grams int64, o *Override, employee bool) (Snapshot, error) {
	if !employee || p.Scene != "sale" {
		return Snapshot{}, fmt.Errorf("仅员工普通录单可调整本单运费")
	}
	if o != nil {
		if o.ManualAmountCents != nil && !validMoney(*o.ManualAmountCents) {
			return Snapshot{}, fmt.Errorf("运费需为非负金额，最多两位小数")
		}
		if o.SurchargeCents != nil {
			return Snapshot{}, fmt.Errorf("本单不能修改模板附加运费")
		}
		if o.ChargeMode != "" && o.ChargeMode != "auto_current" && o.ChargeMode != "manual_current" {
			return Snapshot{}, fmt.Errorf("本单运费计费方式无效")
		}
		if o.ParcelWeightG != nil && (*o.ParcelWeightG <= 0 || *o.ParcelWeightG > 1000000000) {
			return Snapshot{}, fmt.Errorf("本单包裹总重量需为1至1000000000克")
		}
		if o.FreeShipping != nil {
			p.FreeShipping = *o.FreeShipping
		}
	}
	if normalized, e := ResolveDestination(d); e == nil {
		d = normalized
	}
	manual := o != nil && o.ManualAmountCents != nil
	if manual && o.ParcelWeightG != nil {
		return Snapshot{}, fmt.Errorf("手填运费时不能同时填写本单包裹重量")
	}
	if o != nil && o.TemplateID != nil {
		if *o.TemplateID <= 0 || *o.TemplateID != t.ID || !t.Active {
			return Snapshot{}, fmt.Errorf("所选运费模板已停用或不可用")
		}
		p.TemplateID = t.ID
	}
	probe := p
	if manual {
		probe.FreeShipping = true
	}
	if o != nil && o.ParcelWeightG != nil {
		if *o.ParcelWeightG < t.PackagingWeightG {
			return Snapshot{}, fmt.Errorf("本单包裹总重量不能小于模板包装重量")
		}
		grams = *o.ParcelWeightG - t.PackagingWeightG
	}
	q, e := Quote(probe, t, d, grams)
	if e != nil {
		return q, e
	}
	q.Policy = p
	q.Protocol = OrderProtocol
	q.ChargeMode = "deferred"
	if o != nil && o.ParcelWeightG != nil {
		q.ParcelWeightG = *o.ParcelWeightG
		q.WeightSource = "order_override"
	} else if o != nil && (o.ChargeMode == "auto_current" || o.ChargeMode == "manual_current") {
		q.WeightSource = "item_logistics_weights"
	}
	if manual {
		value := *o.ManualAmountCents
		q.ManualAmountCents = &value
		q.ChargeMode = "manual_current"
		q.Status = "manual_current"
		q.CurrentReceivableCents = value
		q.EstimatedTotalCents = value
		q.WaiverCents = 0
	} else if o != nil && (o.ChargeMode == "auto_current" || o.ChargeMode == "manual_current") {
		q.ChargeMode = "auto_current"
		if q.EstimateKnown {
			q.Status = "auto_current"
			q.CurrentReceivableCents = q.EstimatedTotalCents
		}
	}
	if p.FreeShipping {
		q.ChargeMode = "free"
		q.Status = "free"
		q.CurrentReceivableCents = 0
		q.EstimatedTotalCents = 0
	}
	q.Token = Token(q)
	return q, nil
}

type AddressResolution struct {
	Status      string        `json:"status"`
	Destination Destination   `json:"destination"`
	Candidates  []Destination `json:"candidates"`
}

func countyAlias(name string) string {
	if !strings.HasSuffix(name, "自治县") && !strings.HasSuffix(name, "自治旗") {
		return name
	}
	// Common short names retain the place name and administrative suffix.
	ethnic := []string{"拉祜族", "哈尼族", "布朗族", "傈僳族", "达斡尔族", "鄂温克族", "鄂伦春族", "塔吉克族", "保安族", "撒拉族", "土家族", "仡佬族", "毛南族", "瑶族", "苗族", "侗族", "壮族", "彝族", "傣族", "佤族", "回族", "满族", "蒙古族", "藏族", "朝鲜族", "白族", "纳西族", "普米族", "裕固族", "土族", "东乡族", "锡伯族", "水族", "布依族", "黎族", "羌族", "景颇族", "独龙族", "怒族", "仫佬族", "畲族", "各族"}
	end := len(name)
	for _, v := range ethnic {
		if i := strings.Index(name, v); i > 0 && i < end {
			end = i
		}
	}
	if end == len(name) {
		return name
	}
	suffix := "县"
	if strings.HasSuffix(name, "旗") {
		suffix = "旗"
	}
	return name[:end] + suffix
}
func destinationForRegion(r Region) Destination {
	d := Destination{}
	switch r.Level {
	case 3:
		d.District = r.Name
		d.DistrictCode = r.Code
		r = regionIndex[r.ParentCode]
		fallthrough
	case 2:
		d.City = r.Name
		d.CityCode = r.Code
		r = regionIndex[r.ParentCode]
		fallthrough
	case 1:
		d.Province = r.Name
		d.ProvinceCode = r.Code
	}
	return d
}

// Match only names from our local directory, never a fuzzy search result for billing.
func ResolveAddress(address string) AddressResolution {
	out := AddressResolution{Status: "unrecognized", Candidates: []Destination{}}
	address = strings.Join(strings.Fields(address), "")
	if address == "" {
		return out
	}
	matched := map[int][]Region{}
	for _, r := range regions {
		if strings.Contains(address, r.Name) || (len([]rune(r.Alias)) >= 2 && strings.Contains(address, r.Alias)) {
			matched[r.Level] = append(matched[r.Level], r)
		}
	}
	for level := 3; level >= 1; level-- {
		candidates := []Region{}
		for _, r := range matched[level] {
			valid := true
			for parent := 1; parent < level; parent++ {
				if len(matched[parent]) > 0 {
					found := false
					for _, p := range matched[parent] {
						prefix := p.Code[:2]
						if parent == 2 {
							prefix = p.Code[:4]
						}
						if strings.HasPrefix(r.Code, prefix) {
							found = true
						}
					}
					if !found {
						valid = false
					}
				}
			}
			if valid {
				candidates = append(candidates, r)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		for _, r := range candidates {
			out.Candidates = append(out.Candidates, destinationForRegion(r))
		}
		if len(candidates) == 1 {
			out.Status = "resolved"
			out.Destination = out.Candidates[0]
		} else {
			out.Status = "ambiguous"
		}
		return out
	}
	return out
}

// Ignore transport versions when an editor submits the same frozen business quote.
func EquivalentOrderQuote(a, b Snapshot) bool {
	clean := func(q Snapshot) Snapshot {
		q.Token = ""
		q.Protocol = ""
		q.PreviousToken = ""
		q.OrderID = 0
		q.AdjustmentReason = ""
		if q.ChargeMode == "" {
			q.ChargeMode = "deferred"
			if q.Policy.FreeShipping {
				q.ChargeMode = "free"
			}
		}
		if d, e := ResolveDestination(q.Destination); e == nil {
			q.Destination = d
		}
		return q
	}
	x, _ := json.Marshal(clean(a))
	y, _ := json.Marshal(clean(b))
	return string(x) == string(y)
}
