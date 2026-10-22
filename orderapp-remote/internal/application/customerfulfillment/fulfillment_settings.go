package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	freight "orderapp/internal/domain/freight"
	"strings"
)

// Fulfillment settings are customer-owned configuration, independent of portal templates.
type FulfillmentSettings struct {
	DefaultsRevision int64                `json:"defaults_revision"`
	Defaults         *FulfillmentDefaults `json:"defaults,omitempty"`

	CustomerID int64                     `json:"customer_id"`
	Revision   int64                     `json:"revision"`
	Modes      []FulfillmentModeSettings `json:"modes"`
}
type FulfillmentModeSettings struct {
	InheritedFields []string          `json:"inherited_fields"`
	FieldSources    map[string]string `json:"field_sources,omitempty"`

	Mode               string                     `json:"mode"`
	Enabled            bool                       `json:"enabled"`
	WarehouseCode      string                     `json:"warehouse_code"`
	FeeConfigured      bool                       `json:"fee_configured"`
	FixedFeeCents      int64                      `json:"fixed_fee_cents"`
	PerPackageFeeCents int64                      `json:"per_package_fee_cents"`
	ShippingFree       bool                       `json:"shipping_free"`
	ShippingTemplateID int64                      `json:"shipping_template_id"`
	ShippingPolicy     *FulfillmentShippingPolicy `json:"shipping_policy,omitempty"`
}

// FulfillmentShippingPolicy projects the authoritative scenario policy. The
// expected version is supplied only by editors using the atomic settings API.
type FulfillmentShippingPolicy struct {
	TemplateID       int64  `json:"template_id"`
	SurchargeCents   int64  `json:"surcharge_cents"`
	FreeShipping     bool   `json:"free_shipping"`
	Revision         int64  `json:"revision"`
	ExpectedRevision *int64 `json:"expected_revision,omitempty"`
}
type SaveFulfillmentSettingsCommand struct {
	Settings         FulfillmentSettings
	ExpectedRevision int64
	Actor            string
}
type ShippingFeeTemplate = freight.Template
type ShippingFeeRegionRule = freight.LegacyRule
type SaveShippingFeeTemplateCommand struct {
	Template         ShippingFeeTemplate
	ExpectedRevision int64
	Actor            string
}
type FulfillmentFeeItem struct {
	ProductID    int64 `json:"product_id"`
	BomSpecID    int64 `json:"bom_spec_id,omitempty"`
	BomVariantID int64 `json:"bom_variant_id,omitempty"`
	SpecG        int64 `json:"spec_g,omitempty"`
	Qty          int64 `json:"qty"`
}
type FulfillmentFeeQuoteCommand struct {
	FreightProtocol       string               `json:"freight_protocol,omitempty"`
	OrderID               int64                `json:"order_id,omitempty"`
	ExpectedFreightToken  string               `json:"expected_freight_token,omitempty"`
	FreightOverride       *freight.Override    `json:"freight_override,omitempty"`
	CustomerID            int64                `json:"-"`
	FulfillmentMode       string               `json:"fulfillment_mode"`
	Province              string               `json:"province"`
	City                  string               `json:"city"`
	District              string               `json:"district"`
	Items                 []FulfillmentFeeItem `json:"items"`
	ServiceOverrideCents  *int64               `json:"service_override_cents,omitempty"`
	ShippingOverrideCents *int64               `json:"shipping_override_cents,omitempty"`
	AdjustmentReason      string               `json:"adjustment_reason,omitempty"`
	EmployeeAdjustment    bool                 `json:"-"`
}
type FulfillmentFeeQuote struct {
	DefaultsRevision int64 `json:"defaults_revision,omitempty"`

	Freight                    *freight.Snapshot     `json:"freight,omitempty"`
	CustomerID                 int64                 `json:"customer_id"`
	FulfillmentMode            string                `json:"fulfillment_mode"`
	SettingsRevision           int64                 `json:"settings_revision"`
	ShippingTemplateID         int64                 `json:"shipping_template_id"`
	ShippingTemplateRevision   int64                 `json:"shipping_template_revision"`
	Province                   string                `json:"province"`
	City                       string                `json:"city"`
	District                   string                `json:"district"`
	PackageQty                 int64                 `json:"package_qty"`
	NetWeightG                 int64                 `json:"net_weight_g"`
	PackagingWeightG           int64                 `json:"packaging_weight_g"`
	ChargeableWeightG          int64                 `json:"chargeable_weight_g"`
	FixedFeeCents              int64                 `json:"fixed_fee_cents"`
	PerPackageFeeCents         int64                 `json:"per_package_fee_cents"`
	CalculatedServiceFeeCents  int64                 `json:"calculated_service_fee_cents"`
	CalculatedShippingFeeCents int64                 `json:"calculated_shipping_fee_cents"`
	ServiceFeeCents            int64                 `json:"service_fee_cents"`
	ShippingFeeCents           int64                 `json:"shipping_fee_cents"`
	Rule                       ShippingFeeRegionRule `json:"shipping_rule"`
	AdjustmentReason           string                `json:"adjustment_reason,omitempty"`
	FeeQuoteToken              string                `json:"fee_quote_token"`
}
type AdjustFulfillmentOrderFeesCommand struct {
	CustomerID         int64  `json:"-"`
	OrderID            int64  `json:"-"`
	ExpectedQuoteToken string `json:"expected_quote_token"`
	ServiceFeeCents    int64  `json:"service_fee_cents"`
	ShippingFeeCents   int64  `json:"shipping_fee_cents"`
	Reason             string `json:"reason"`
	Actor              string `json:"-"`
}
type FulfillmentSettingsRepository interface {
	LoadFulfillmentSettings(context.Context, int64) (FulfillmentSettings, error)
	SaveFulfillmentSettings(context.Context, SaveFulfillmentSettingsCommand) (FulfillmentSettings, error)
	ListShippingFeeTemplates(context.Context) ([]ShippingFeeTemplate, error)
	SaveShippingFeeTemplate(context.Context, SaveShippingFeeTemplateCommand) (ShippingFeeTemplate, error)
	QuoteFulfillmentFees(context.Context, FulfillmentFeeQuoteCommand) (FulfillmentFeeQuote, error)
	AdjustFulfillmentOrderFees(context.Context, AdjustFulfillmentOrderFeesCommand) (FulfillmentFeeQuote, error)
	GetFulfillmentOrderFees(context.Context, int64, int64) (FulfillmentFeeQuote, error)
}

func (s *Service) fulfillmentSettingsRepository() (FulfillmentSettingsRepository, error) {
	r, ok := s.repo.(FulfillmentSettingsRepository)
	if !ok {
		return nil, fmt.Errorf("履约设置服务不可用")
	}
	return r, nil
}
func (s *Service) LoadFulfillmentSettings(ctx context.Context, id int64) (FulfillmentSettings, error) {
	r, e := s.fulfillmentSettingsRepository()
	if e != nil {
		return FulfillmentSettings{}, e
	}
	return r.LoadFulfillmentSettings(ctx, id)
}
func (s *Service) SaveFulfillmentSettings(ctx context.Context, c SaveFulfillmentSettingsCommand) (FulfillmentSettings, error) {
	if e := ValidateFulfillmentSettings(c.Settings); e != nil {
		return FulfillmentSettings{}, e
	}
	r, e := s.fulfillmentSettingsRepository()
	if e != nil {
		return FulfillmentSettings{}, e
	}
	return r.SaveFulfillmentSettings(ctx, c)
}
func (s *Service) ListShippingFeeTemplates(ctx context.Context) ([]ShippingFeeTemplate, error) {
	r, e := s.fulfillmentSettingsRepository()
	if e != nil {
		return nil, e
	}
	return r.ListShippingFeeTemplates(ctx)
}
func (s *Service) SaveShippingFeeTemplate(ctx context.Context, c SaveShippingFeeTemplateCommand) (ShippingFeeTemplate, error) {
	if e := ValidateShippingFeeTemplate(c.Template); e != nil {
		return ShippingFeeTemplate{}, e
	}
	r, e := s.fulfillmentSettingsRepository()
	if e != nil {
		return ShippingFeeTemplate{}, e
	}
	return r.SaveShippingFeeTemplate(ctx, c)
}
func (s *Service) QuoteFulfillmentFees(ctx context.Context, c FulfillmentFeeQuoteCommand) (FulfillmentFeeQuote, error) {
	r, e := s.fulfillmentSettingsRepository()
	if e != nil {
		return FulfillmentFeeQuote{}, e
	}
	return r.QuoteFulfillmentFees(ctx, c)
}
func (s *Service) AdjustFulfillmentOrderFees(ctx context.Context, c AdjustFulfillmentOrderFeesCommand) (FulfillmentFeeQuote, error) {
	r, e := s.fulfillmentSettingsRepository()
	if e != nil {
		return FulfillmentFeeQuote{}, e
	}
	return r.AdjustFulfillmentOrderFees(ctx, c)
}

func (s *Service) GetFulfillmentOrderFees(ctx context.Context, customerID, orderID int64) (FulfillmentFeeQuote, error) {
	r, e := s.fulfillmentSettingsRepository()
	if e != nil {
		return FulfillmentFeeQuote{}, e
	}
	return r.GetFulfillmentOrderFees(ctx, customerID, orderID)
}

const maxFulfillmentFeeCents int64 = 999999999999

func ValidateFulfillmentSettings(s FulfillmentSettings) error {
	if s.CustomerID <= 0 {
		return fmt.Errorf("请选择客户")
	}
	seen := map[string]bool{}
	for _, m := range s.Modes {
		if (m.Mode != "stock" && m.Mode != "on_demand") || seen[m.Mode] {
			return fmt.Errorf("履约场景无效或重复")
		}
		seen[m.Mode] = true
		if err := ValidateFulfillmentInheritedFields(m.InheritedFields); err != nil {
			return err
		}
		if !m.Enabled {
			continue
		}
		if m.FixedFeeCents < 0 || m.PerPackageFeeCents < 0 || m.FixedFeeCents > maxFulfillmentFeeCents || m.PerPackageFeeCents > maxFulfillmentFeeCents {
			return fmt.Errorf("代发服务费金额无效")
		}
		if m.Enabled {
			if !m.FeeConfigured {
				return fmt.Errorf("请明确配置代发服务费，免费请填写 0")
			}
			templateID := m.ShippingTemplateID
			if policy := m.ShippingPolicy; policy != nil {
				if policy.ExpectedRevision == nil || *policy.ExpectedRevision < 0 {
					return fmt.Errorf("请提供当前场景运费政策版本")
				}
				if policy.TemplateID < 0 {
					return fmt.Errorf("运费模板无效")
				}
				if err := freight.ValidatePolicyOverride(freight.Policy{CustomerID: s.CustomerID, Scene: m.Mode, TemplateID: policy.TemplateID, SurchargeCents: policy.SurchargeCents, FreeShipping: policy.FreeShipping}); err != nil {
					return err
				}
				templateID = policy.TemplateID
			}
			if templateID < 0 {
				return fmt.Errorf("请选择运费模板")
			}
			if m.Mode == "stock" && strings.TrimSpace(m.WarehouseCode) == "" {
				return fmt.Errorf("库存代发必须绑定客户成品仓")
			}
		}
	}
	return nil
}
func ValidateShippingFeeTemplate(t ShippingFeeTemplate) error { return freight.ValidateTemplate(t) }
func CalculateFulfillmentFees(s FulfillmentSettings, t ShippingFeeTemplate, c FulfillmentFeeQuoteCommand, qty, netG int64) (FulfillmentFeeQuote, error) {
	var q FulfillmentFeeQuote
	var mode *FulfillmentModeSettings
	for i := range s.Modes {
		if s.Modes[i].Mode == c.FulfillmentMode {
			mode = &s.Modes[i]
			break
		}
	}
	if c.CustomerID != s.CustomerID || mode == nil || !mode.Enabled {
		return q, fmt.Errorf("该客户未启用所选代发场景")
	}
	if !mode.FeeConfigured {
		return q, fmt.Errorf("该场景尚未配置代发服务费")
	}
	if !t.Active || t.ID != mode.ShippingTemplateID {
		return q, fmt.Errorf("该场景运费模板未配置或已停用")
	}
	if qty <= 0 || qty > 1000000 || netG <= 0 || netG > 1000000000 {
		return q, fmt.Errorf("商品包装数量或净重未配置，无法计算运费")
	}
	if (c.ShippingOverrideCents != nil || c.ServiceOverrideCents != nil) && (!c.EmployeeAdjustment || strings.TrimSpace(c.AdjustmentReason) == "") {
		return q, fmt.Errorf("费用调整仅限员工操作且必须填写原因")
	}
	best := -1
	var rule ShippingFeeRegionRule
	for _, r := range t.Rules {
		if r.Province != "*" && strings.TrimSpace(r.Province) != strings.TrimSpace(c.Province) {
			continue
		}
		if r.City != "" && r.City != "*" && r.City != c.City {
			continue
		}
		if r.District != "" && r.District != "*" && r.District != c.District {
			continue
		}
		score := 0
		if r.Province != "*" {
			score += 1
		}
		if r.City != "" && r.City != "*" {
			score += 2
		}
		if r.District != "" && r.District != "*" {
			score += 4
		}
		if score > best {
			best = score
			rule = r
		}
	}
	if best < 0 {
		return q, fmt.Errorf("运费模板未覆盖当前收件地区")
	}
	if rule.FirstWeightG <= 0 || rule.AdditionalWeightG <= 0 {
		return q, fmt.Errorf("运费模板重量规则无效")
	}
	weight := netG + t.PackagingWeightG
	steps := int64(0)
	if weight > rule.FirstWeightG {
		steps = (weight-rule.FirstWeightG-1)/rule.AdditionalWeightG + 1
	}
	if mode.PerPackageFeeCents > maxFulfillmentFeeCents/qty || steps > 0 && rule.AdditionalFeeCents > maxFulfillmentFeeCents/steps {
		return q, fmt.Errorf("费用超出可处理范围")
	}
	service := mode.FixedFeeCents + mode.PerPackageFeeCents*qty
	shipping := rule.FirstFeeCents + steps*rule.AdditionalFeeCents
	q = FulfillmentFeeQuote{CustomerID: s.CustomerID, FulfillmentMode: c.FulfillmentMode, SettingsRevision: s.Revision, ShippingTemplateID: t.ID, ShippingTemplateRevision: t.Revision, Province: c.Province, City: c.City, District: c.District, PackageQty: qty, NetWeightG: netG, PackagingWeightG: t.PackagingWeightG, ChargeableWeightG: weight, FixedFeeCents: mode.FixedFeeCents, PerPackageFeeCents: mode.PerPackageFeeCents, CalculatedServiceFeeCents: service, CalculatedShippingFeeCents: shipping, ServiceFeeCents: service, ShippingFeeCents: shipping, Rule: rule, AdjustmentReason: strings.TrimSpace(c.AdjustmentReason)}
	if c.ServiceOverrideCents != nil {
		q.ServiceFeeCents = *c.ServiceOverrideCents
	}
	if c.ShippingOverrideCents != nil {
		q.ShippingFeeCents = *c.ShippingOverrideCents
	}
	if q.ServiceFeeCents < 0 || q.ShippingFeeCents < 0 || q.ServiceFeeCents > maxFulfillmentFeeCents || q.ShippingFeeCents > maxFulfillmentFeeCents {
		return FulfillmentFeeQuote{}, fmt.Errorf("费用金额无效")
	}
	q.FeeQuoteToken = FulfillmentFeeQuoteToken(q)
	return q, nil
}
func FulfillmentFeeQuoteToken(q FulfillmentFeeQuote) string {
	q.FeeQuoteToken = ""
	raw, _ := json.Marshal(q)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
