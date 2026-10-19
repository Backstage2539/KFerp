// Package freight contains side-effect-free shipping estimates and settlement deltas.
package freight

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const MaxCents int64 = 999999999999
const Protocol = "deferred_freight_v1"

type Template struct {
	ID               int64        `json:"id"`
	Name             string       `json:"name"`
	Active           bool         `json:"active"`
	Revision         int64        `json:"revision"`
	PackagingWeightG int64        `json:"packaging_weight_g"`
	CarrierID        int64        `json:"carrier_id"`
	ProductID        int64        `json:"logistics_product_id"`
	Groups           []Group      `json:"groups"`
	Rules            []LegacyRule `json:"rules"`
}
type Tier struct {
	UpperWeightG int64 `json:"upper_weight_g"`
	FeeCents     int64 `json:"fee_cents"`
}
type Group struct {
	Name               string   `json:"name"`
	RegionCodes        []string `json:"region_codes"`
	Tiers              []Tier   `json:"tiers"`
	AdditionalWeightG  int64    `json:"additional_weight_g"`
	AdditionalFeeCents int64    `json:"additional_fee_cents"`
	FirstWeightG       int64    `json:"first_weight_g,omitempty"`
	FirstFeeCents      int64    `json:"first_fee_cents,omitempty"`
}
type LegacyRule struct {
	Province           string `json:"province"`
	City               string `json:"city"`
	District           string `json:"district"`
	FirstWeightG       int64  `json:"first_weight_g"`
	FirstFeeCents      int64  `json:"first_fee_cents"`
	AdditionalWeightG  int64  `json:"additional_weight_g"`
	AdditionalFeeCents int64  `json:"additional_fee_cents"`
}
type Destination struct {
	Province     string `json:"province"`
	City         string `json:"city"`
	District     string `json:"district"`
	ProvinceCode string `json:"province_code"`
	CityCode     string `json:"city_code"`
	DistrictCode string `json:"district_code"`
}
type Policy struct {
	CustomerID     int64  `json:"customer_id"`
	Scene          string `json:"scene"`
	Revision       int64  `json:"revision"`
	TemplateID     int64  `json:"template_id"`
	SurchargeCents int64  `json:"surcharge_cents"`
	FreeShipping   bool   `json:"free_shipping"`
}
type Estimate struct {
	BaseCents  int64  `json:"base_cents"`
	WeightG    int64  `json:"chargeable_weight_g"`
	GroupName  string `json:"group_name"`
	RegionCode string `json:"region_code"`
}
type Snapshot struct {
	Protocol               string      `json:"protocol"`
	Policy                 Policy      `json:"policy"`
	TemplateID             int64       `json:"template_id"`
	TemplateRevision       int64       `json:"template_revision"`
	CarrierID              int64       `json:"carrier_id"`
	ProductID              int64       `json:"logistics_product_id"`
	Destination            Destination `json:"destination"`
	NetWeightG             int64       `json:"net_weight_g"`
	PackagingWeightG       int64       `json:"packaging_weight_g"`
	Estimate               Estimate    `json:"estimate"`
	EstimateKnown          bool        `json:"estimate_known"`
	CostPendingReason      string      `json:"cost_pending_reason,omitempty"`
	SurchargeCents         int64       `json:"surcharge_cents"`
	WaiverCents            int64       `json:"waiver_cents"`
	EstimatedTotalCents    int64       `json:"estimated_total_cents"`
	CurrentReceivableCents int64       `json:"current_receivable_cents"`
	Status                 string      `json:"status"`
	Token                  string      `json:"token"`
	AdjustmentReason       string      `json:"adjustment_reason,omitempty"`
}
type SettlementDelta struct {
	ActualCents    int64 `json:"actual_cents"`
	SurchargeCents int64 `json:"surcharge_cents"`
	TargetCents    int64 `json:"target_cents"`
	DeltaCents     int64 `json:"delta_cents"`
}

func validMoney(v int64) bool  { return v >= 0 && v <= MaxCents }
func ValidScene(s string) bool { return s == "sale" || s == "stock" || s == "on_demand" }
func ValidatePolicy(p Policy) error {
	if !ValidScene(p.Scene) || !validMoney(p.SurchargeCents) || p.CustomerID < 0 || (!p.FreeShipping && p.TemplateID <= 0) {
		return fmt.Errorf("请配置有效场景、运费模板及附加运费")
	}
	return nil
}
func ValidateTemplate(t Template) error {
	if strings.TrimSpace(t.Name) == "" || t.PackagingWeightG < 0 || t.PackagingWeightG > 1000000 || len(t.Groups)+len(t.Rules) == 0 {
		return fmt.Errorf("请填写运费模板名称、包装附加重和地区规则")
	}
	seen := map[string]bool{}
	if len(t.Groups) > 0 && (t.CarrierID <= 0 || t.ProductID <= 0 || len(t.Rules) > 0) {
		return fmt.Errorf("请选择物流公司和物流产品，新分组不能混用旧规则")
	}
	for _, g := range t.Groups {
		if strings.TrimSpace(g.Name) == "" || len(g.RegionCodes) == 0 || g.AdditionalWeightG <= 0 || !validMoney(g.AdditionalFeeCents) {
			return fmt.Errorf("地区分组、续重及金额无效")
		}
		if len(g.Tiers) == 0 {
			if g.FirstWeightG <= 0 || !validMoney(g.FirstFeeCents) {
				return fmt.Errorf("首重规则无效")
			}
		}
		last := int64(0)
		for _, tier := range g.Tiers {
			if tier.UpperWeightG <= last || tier.UpperWeightG > 1000000000 || !validMoney(tier.FeeCents) {
				return fmt.Errorf("重量档位必须递增，金额需有效")
			}
			last = tier.UpperWeightG
		}
		for _, c := range g.RegionCodes {
			if _, ok := regionIndex[c]; !ok {
				return fmt.Errorf("地区代码无效：%s", c)
			}
			if seen[c] {
				return fmt.Errorf("同层地区重复：%s", regionIndex[c].Name)
			}
			seen[c] = true
		}
	}
	for _, r := range t.Rules {
		if r.Province == "" || r.FirstWeightG <= 0 || r.AdditionalWeightG <= 0 || !validMoney(r.FirstFeeCents) || !validMoney(r.AdditionalFeeCents) || (r.District != "" && r.City == "") {
			return fmt.Errorf("旧首重续重地区规则无效")
		}
		k := r.Province + "|" + r.City + "|" + r.District
		if seen[k] {
			return fmt.Errorf("地区规则重复")
		}
		seen[k] = true
	}
	return nil
}
func Calculate(t Template, d Destination, netG int64) (Estimate, error) {
	var out Estimate
	if !t.Active || netG <= 0 || netG > 1000000000 {
		return out, fmt.Errorf("模板停用或商品未配置有效净重")
	}
	if err := ValidateTemplate(t); err != nil {
		return out, err
	}
	out.WeightG = netG + t.PackagingWeightG
	var group Group
	found := false
	if len(t.Groups) > 0 {
		dst, err := ResolveDestination(d)
		if err != nil {
			return out, err
		}
		best := 0
		for _, g := range t.Groups {
			for _, code := range g.RegionCodes {
				r := regionIndex[code]
				match := code == dst.ProvinceCode || code == dst.CityCode || code == dst.DistrictCode
				if match && r.Level > best {
					best = r.Level
					group = g
					out.RegionCode = code
					found = true
				}
			}
		}
	} else {
		// Legacy text rules keep their original matching semantics and prices.
		best := -1
		for _, r := range t.Rules {
			if r.Province != "*" && r.Province != strings.TrimSpace(d.Province) {
				continue
			}
			if r.City != "" && r.City != "*" && r.City != d.City {
				continue
			}
			if r.District != "" && r.District != "*" && r.District != d.District {
				continue
			}
			score := 0
			if r.Province != "*" {
				score++
			}
			if r.City != "" && r.City != "*" {
				score += 2
			}
			if r.District != "" && r.District != "*" {
				score += 4
			}
			if score > best {
				best = score
				group = Group{Name: r.Province + r.City + r.District, FirstWeightG: r.FirstWeightG, FirstFeeCents: r.FirstFeeCents, AdditionalWeightG: r.AdditionalWeightG, AdditionalFeeCents: r.AdditionalFeeCents}
				found = true
			}
		}
	}
	if !found {
		return out, fmt.Errorf("运费模板未覆盖当前收件地区，请配置或选择地区")
	}
	out.GroupName = group.Name
	lastG, fee := group.FirstWeightG, group.FirstFeeCents
	if len(group.Tiers) > 0 {
		for _, v := range group.Tiers {
			lastG, fee = v.UpperWeightG, v.FeeCents
			if out.WeightG <= lastG {
				break
			}
		}
	}
	steps := int64(0)
	if out.WeightG > lastG {
		steps = (out.WeightG-lastG-1)/group.AdditionalWeightG + 1
	}
	if steps > 0 && group.AdditionalFeeCents > (MaxCents-fee)/steps {
		return out, fmt.Errorf("运费超出可处理范围")
	}
	out.BaseCents = fee + steps*group.AdditionalFeeCents
	return out, nil
}
func Quote(p Policy, t Template, d Destination, netG int64) (Snapshot, error) {
	var q Snapshot
	if err := ValidatePolicy(p); err != nil {
		return q, err
	}
	q = Snapshot{Protocol: Protocol, Policy: p, TemplateID: t.ID, TemplateRevision: t.Revision, CarrierID: t.CarrierID, ProductID: t.ProductID, Destination: d, NetWeightG: netG, PackagingWeightG: t.PackagingWeightG, SurchargeCents: p.SurchargeCents, Status: "pending_actual"}
	e, err := Calculate(t, d, netG)
	if p.TemplateID != t.ID {
		err = fmt.Errorf("客户运费模板未配置")
	}
	if err != nil && !p.FreeShipping {
		return Snapshot{}, err
	}
	q.Estimate = e
	q.EstimateKnown = err == nil
	if err != nil {
		q.CostPendingReason = err.Error()
	}
	if p.FreeShipping {
		q.Status = "free"
		if q.EstimateKnown {
			q.WaiverCents = e.BaseCents + p.SurchargeCents
		}
	} else {
		if e.BaseCents > MaxCents-p.SurchargeCents {
			return Snapshot{}, fmt.Errorf("运费超出可处理范围")
		}
		q.EstimatedTotalCents = e.BaseCents + p.SurchargeCents
	}
	q.Token = Token(q)
	return q, nil
}
func Token(q Snapshot) string {
	q.Token = ""
	b, _ := json.Marshal(q)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func Settlement(q Snapshot, actual, already int64) (SettlementDelta, error) {
	var a SettlementDelta
	if !validMoney(actual) || already < 0 || already > MaxCents || q.Protocol != Protocol {
		return a, fmt.Errorf("实际运费或订单计费规则无效")
	}
	a.ActualCents = actual
	if !q.Policy.FreeShipping {
		a.SurchargeCents = q.SurchargeCents
		if actual > MaxCents-a.SurchargeCents {
			return a, fmt.Errorf("实际运费超出范围")
		}
		a.TargetCents = actual + a.SurchargeCents
	}
	a.DeltaCents = a.TargetCents - already
	return a, nil
}

// OrderView keeps the immutable quote alongside the current actual-billing state.
type OrderView struct {
	ServiceFeeCents int64 `json:"service_fee_cents"`
	ServiceDueCents int64 `json:"service_due_cents"`
	Snapshot
	BillingStatus   string `json:"billing_status"`
	ActualCostCents *int64 `json:"actual_cost_cents"`
	BilledCents     int64  `json:"billed_cents"`
}

type Override struct {
	TemplateID     *int64 `json:"template_id,omitempty"`
	SurchargeCents *int64 `json:"surcharge_cents,omitempty"`
	FreeShipping   *bool  `json:"free_shipping,omitempty"`
	Reason         string `json:"reason"`
}

func ApplyOverride(p Policy, o *Override, employee bool) (Policy, error) {
	if o == nil {
		return p, nil
	}
	if !employee || strings.TrimSpace(o.Reason) == "" {
		return p, fmt.Errorf("员工按单调整运费必须填写原因")
	}
	if o.TemplateID != nil {
		p.TemplateID = *o.TemplateID
	}
	if o.SurchargeCents != nil {
		p.SurchargeCents = *o.SurchargeCents
	}
	if o.FreeShipping != nil {
		p.FreeShipping = *o.FreeShipping
	}
	return p, ValidatePolicy(p)
}
