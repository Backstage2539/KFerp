package costing

import (
	"encoding/json"
	"fmt"

	"orderapp/internal/application/catalog"
)

func (cmd *PricingRuleTrialCommand) UnmarshalJSON(data []byte) error {
	type alias PricingRuleTrialCommand
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*cmd = PricingRuleTrialCommand(decoded)
	_, cmd.pricingRuleTrialOverridesProvided = fields["overrides"]
	return nil
}

func (cmd PricingRuleTrialCommand) ValidateDraftUsage(batch bool) error {
	if cmd.PricingRuleDraft == nil {
		return nil
	}
	if batch {
		return fmt.Errorf("pricing_rule_draft is only supported by single read-only trials")
	}
	o := cmd.Overrides
	if cmd.pricingRuleTrialOverridesProvided || o.ExpectedLossRate != nil || o.BaseCost != nil || o.MarginRate != nil || o.TaxRate != nil || o.OtherCosts != nil || o.PostMarkupCosts != nil {
		return fmt.Errorf("pricing_rule_draft and overrides cannot be used together")
	}
	return nil
}

func pricingRuleTrialDraftRule(stored, draft ProductPricingRule) (ProductPricingRule, error) {
	if err := catalog.ValidateProductPricingRuleReplacement(stored.CalculationJSON); err != nil {
		return ProductPricingRule{}, err
	}
	if draft.ID != 0 && draft.ID != stored.ID {
		return ProductPricingRule{}, fmt.Errorf("pricing_rule_draft id must match pricing_rule_id")
	}
	// Identity and lifecycle always come from the stored template. The draft
	// replaces the entire calculation configuration; it never mutates that row.
	normalized, err := catalog.NormalizeProductPricingRule(catalog.ProductPricingRule{
		ID: stored.ID, Name: draft.Name, Code: draft.Code, Active: stored.Active,
		CostSourceMode: draft.CostSourceMode, MarginRate: draft.MarginRate,
		TaxRate: draft.TaxRate, RoundingMode: draft.RoundingMode,
		FormulaVersion: draft.FormulaVersion, CalculationJSON: draft.CalculationJSON,
		Remark: draft.Remark,
	})
	if err != nil {
		return ProductPricingRule{}, err
	}
	return ProductPricingRule{
		ID: normalized.ID, Name: normalized.Name, Code: normalized.Code, Active: normalized.Active,
		CostSourceMode: normalized.CostSourceMode, MarginRate: normalized.MarginRate,
		TaxRate: normalized.TaxRate, RoundingMode: normalized.RoundingMode,
		FormulaVersion: normalized.FormulaVersion, CalculationJSON: normalized.CalculationJSON,
		Remark: normalized.Remark,
	}, nil
}
