package customerfulfillment

import (
	"context"
	"fmt"
)

// Defaults contain business rules only. Activation, warehouse and customer
// authorization always remain customer-owned.
type FulfillmentDefaults struct {
	InheritingCustomers int                       `json:"inheriting_customers"`
	Revision            int64                     `json:"revision"`
	Modes               []FulfillmentModeSettings `json:"modes"`
}
type SaveFulfillmentDefaultsCommand struct {
	Defaults         FulfillmentDefaults
	ExpectedRevision int64
	Actor            string
}
type fulfillmentDefaultsRepository interface {
	LoadFulfillmentDefaults(context.Context) (FulfillmentDefaults, error)
	SaveFulfillmentDefaults(context.Context, SaveFulfillmentDefaultsCommand) (FulfillmentDefaults, error)
}

func (s *Service) LoadFulfillmentDefaults(ctx context.Context) (FulfillmentDefaults, error) {
	r, ok := s.repo.(fulfillmentDefaultsRepository)
	if !ok {
		return FulfillmentDefaults{}, fmt.Errorf("履约默认配置服务不可用")
	}
	return r.LoadFulfillmentDefaults(ctx)
}
func (s *Service) SaveFulfillmentDefaults(ctx context.Context, c SaveFulfillmentDefaultsCommand) (FulfillmentDefaults, error) {
	r, ok := s.repo.(fulfillmentDefaultsRepository)
	if !ok {
		return FulfillmentDefaults{}, fmt.Errorf("履约默认配置服务不可用")
	}
	return r.SaveFulfillmentDefaults(ctx, c)
}
func FulfillmentDefaultFields() []string {
	return []string{"fixed_fee_cents", "per_package_fee_cents", "shipping_template_id", "shipping_surcharge_cents", "shipping_free"}
}
func ValidateFulfillmentInheritedFields(fields []string) error {
	allowed := map[string]bool{}
	for _, f := range FulfillmentDefaultFields() {
		allowed[f] = true
	}
	for _, f := range fields {
		if !allowed[f] {
			return fmt.Errorf("不支持或重复的默认配置字段：%s", f)
		}
		delete(allowed, f)
	}
	return nil
}
func FulfillmentModeDefault(d FulfillmentDefaults, mode string) FulfillmentModeSettings {
	for _, m := range d.Modes {
		if m.Mode == mode {
			return m
		}
	}
	return FulfillmentModeSettings{Mode: mode}
}
func ResolveFulfillmentModeDefaults(m, d FulfillmentModeSettings) FulfillmentModeSettings {
	if m.ShippingPolicy != nil {
		p := *m.ShippingPolicy
		m.ShippingPolicy = &p
	} else {
		m.ShippingPolicy = &FulfillmentShippingPolicy{TemplateID: m.ShippingTemplateID, FreeShipping: m.ShippingFree}
	}
	m.FieldSources = map[string]string{}
	for _, f := range FulfillmentDefaultFields() {
		m.FieldSources[f] = "customer"
	}
	for _, f := range m.InheritedFields {
		m.FieldSources[f] = "default"
		switch f {
		case "fixed_fee_cents":
			m.FixedFeeCents = d.FixedFeeCents
			if !d.FeeConfigured {
				m.FeeConfigured = false
			}
		case "per_package_fee_cents":
			m.PerPackageFeeCents = d.PerPackageFeeCents
			if !d.FeeConfigured {
				m.FeeConfigured = false
			}
		case "shipping_template_id":
			m.ShippingPolicy.TemplateID = d.ShippingTemplateID
		case "shipping_free":
			m.ShippingPolicy.FreeShipping = d.ShippingFree
		case "shipping_surcharge_cents":
			m.ShippingPolicy.SurchargeCents = 0
			if d.ShippingPolicy != nil {
				m.ShippingPolicy.SurchargeCents = d.ShippingPolicy.SurchargeCents
			}
		}
	}
	if containsDefaultField(m.InheritedFields, "fixed_fee_cents") && containsDefaultField(m.InheritedFields, "per_package_fee_cents") {
		m.FeeConfigured = d.FeeConfigured
	}
	m.ShippingTemplateID, m.ShippingFree = m.ShippingPolicy.TemplateID, m.ShippingPolicy.FreeShipping
	return m
}
func containsDefaultField(fields []string, field string) bool {
	for _, f := range fields {
		if f == field {
			return true
		}
	}
	return false
}
