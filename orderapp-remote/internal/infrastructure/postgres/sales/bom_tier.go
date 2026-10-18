package sales

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	costingapp "orderapp/internal/application/costing"
	salesapp "orderapp/internal/application/sales"
	conversion "orderapp/internal/domain/specconversion"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	costingpg "orderapp/internal/infrastructure/postgres/costing"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	"strconv"
	"strings"
	"time"
)

type bomTierRow struct {
	ProductID      int64                          `json:"product_id"`
	ParentID       int64                          `json:"parent_product_id"`
	SpecID         int64                          `json:"bom_spec_id"`
	VariantID      int64                          `json:"bom_variant_id"`
	PricingMode    string                         `json:"pricing_mode"`
	CostSource     map[string]any                 `json:"cost_source_snapshot"`
	RuleID         int64                          `json:"pricing_rule_id"`
	TierRuleID     int64                          `json:"tier_pricing_rule_id"`
	Rule           *costingapp.ProductPricingRule `json:"pricing_rule_snapshot"`
	Price          json.Number                    `json:"final_unit_price"`
	OriginalPrice  json.Number                    `json:"original_final_unit_price"`
	ManualAdjusted bool                           `json:"manual_adjusted"`
	TierLabel      string                         `json:"tier_label"`
	Unit           string                         `json:"inventory_unit"`
	PriceUnit      string                         `json:"price_unit"`
	Basis          string                         `json:"quantity_basis"`
}

func bomTierSource(raw []byte, listType string, c salesapp.SpecConversionQuoteRequest) (bomTierRow, error) {
	var content struct {
		Rows []bomTierRow `json:"price_rows"`
	}
	if err := json.Unmarshal(raw, &content); err != nil {
		return bomTierRow{}, fmt.Errorf("来源价格快照无效")
	}
	for i, row := range content.Rows {
		pid := row.ProductID
		if row.ParentID > 0 {
			pid = row.ParentID
		}
		if pid != c.ProductID || row.SpecID != c.SourceSpecID || row.VariantID != c.SourceVariantID || c.PriceRowKey != orderbeans.PublishedPriceRowKey(c.PublicationID, listType, pid, row.SpecID, row.VariantID, "flat", i) {
			continue
		}
		if row.Basis != "sales_spec_count" || row.Unit == "" || !strings.EqualFold(row.Unit, row.PriceUnit) {
			return row, fmt.Errorf("来源档位缺少明确的规格计价单位")
		}
		id := row.RuleID
		if row.TierRuleID > 0 {
			id = row.TierRuleID
		}
		if row.Rule != nil && (row.Rule.ID != id || id <= 0) {
			return row, fmt.Errorf("来源档位的模板快照不匹配")
		}
		return row, nil
	}
	return bomTierRow{}, fmt.Errorf("所选档位不属于当前商品、来源规格或价格表")
}

// This quote only reads published rules and current published fulfillment specs.
// It never trusts rule parameters, prices or historical snapshots from clients.
func (r Repository) quoteBOMTierTx(ctx context.Context, tx pgx.Tx, c salesapp.SpecConversionQuoteRequest) (salesapp.SpecConversionQuote, error) {
	q := salesapp.SpecConversionQuote{SpecConversionQuoteRequest: c}
	if c.Quantity <= 0 || c.Quantity > 1000000000 || c.GroupID != 0 || c.PricingMethod != "bom_rule" {
		return q, fmt.Errorf("请选择档位、目标规格和有效数量")
	}
	raw, listType, version, err := conversionPublicationTx(ctx, tx, r.schema, c.CustomerID, c.ProductID, c.PublicationID)
	if err != nil {
		return q, err
	}
	row, err := bomTierSource(raw, listType, c)
	if err != nil {
		return q, err
	}
	specs, err := conversionSpecsTx(ctx, tx, r.schema, c.ProductID)
	if err != nil {
		return q, err
	}
	var target *salesapp.SpecConversionSpec
	for i := range specs {
		if specs[i].SpecID == c.TargetSpecID && specs[i].VariantID == c.TargetVariantID {
			target = &specs[i]
			break
		}
	}
	if target == nil {
		return q, fmt.Errorf("目标规格不属于该商品当前默认已发布 BOM")
	}
	q.TargetSpecName = target.Name
	q.TargetUnit = target.InventoryUnit
	q.SourceUnit = row.Unit
	q.SourceUnitPrice = string(row.Price)
	q.TierLabel = row.TierLabel
	q.VersionNo = version
	q.Rounding = "half_up_2"
	q.Reprice = false
	sources, err := conversionPublishedSourcesTx(ctx, tx, r.schema, raw, listType, c.ProductID, c.PublicationID)
	if err != nil {
		return q, err
	}
	for _, source := range sources {
		if source.PriceRowKey == c.PriceRowKey {
			q.SourceSpecName = source.Name
		}
	}
	if c.TargetSpecID == c.SourceSpecID && c.TargetVariantID == c.SourceVariantID {
		q.UnitPrice, err = conversion.Convert(string(row.Price), "1", "1")
		q.PricingRuleName = "已发布价格"
		q.PricingRuleBasis = "published_price"
		q.CurrentRuleRevision = ""
	} else {
		if row.ManualAdjusted {
			return q, fmt.Errorf("来源档位经过手工改价，请使用换算组按成交价换算，或选择未手工调整的模板档位")
		}
		q.PricingRuleBasis = "published_snapshot"
		if row.Rule == nil {
			row.Rule, q.PricingRuleBasis, q.PricingRuleUpdatedAt, err = r.legacyBOMRuleTx(ctx, tx, row, c)
			if err != nil {
				return q, err
			}
		} else {
			q.CurrentRuleRevision = ""
		}
		if !row.Rule.Active {
			return q, fmt.Errorf("来源档位未保存有效的计算模板")
		}

		trialCtx := postgresinfra.WithTransaction(ctx, tx)
		result, e := costingapp.NewService(costingpg.NewRepository(r.pool, r.schema)).PricingRuleTrial(trialCtx, costingapp.PricingRuleTrialCommand{PricingRuleID: row.Rule.ID, PublishedRuleSnapshot: row.Rule, PublishedOnly: true, CustomerID: c.CustomerID, ProductID: c.ProductID, BomSpecID: c.TargetSpecID, BomVariantID: c.TargetVariantID, QuoteUnit: target.InventoryUnit})
		if e != nil {
			return q, e
		}
		if result.CostStatus != "complete" || result.FinalUnitPrice <= 0 || result.BomSpecID != c.TargetSpecID || result.BomVariantID != c.TargetVariantID || result.BomStatus != "active" {
			return q, fmt.Errorf("目标规格成本不完整或 BOM 已变化，请补齐成本后重新报价")
		}
		q.UnitPrice, err = conversion.Convert(strconv.FormatFloat(result.FinalUnitPrice, 'f', -1, 64), "1", "1")
		q.PricingRuleName = row.Rule.Name
		// Candidate lists are UI data, not part of the frozen price calculation.
		result.BomVersionOptions = nil
		result.BomSpecOptions = nil
		result.ProcessRouteOptions = nil
		result.OperationTemplateOptions = nil
		q.Calculation, _ = json.Marshal(result)
	}
	if err != nil {
		return q, err
	}
	revisionData, _ := json.Marshal(struct {
		Row                 bomTierRow
		Target              salesapp.SpecConversionSpec
		Calculation         json.RawMessage
		Price               string
		Basis               string
		UpdatedAt           string
		CurrentRuleRevision string
	}{row, *target, q.Calculation, q.UnitPrice, q.PricingRuleBasis, q.PricingRuleUpdatedAt, q.CurrentRuleRevision})
	revision := fmt.Sprintf("%x", sha256.Sum256(revisionData))
	if c.QuoteRevision != "" && c.QuoteRevision != revision {
		return q, salesapp.NewOrderEditConflictError("BOM、成本或价格依据已变化，请刷新报价后提交")
	}
	q.QuoteRevision = revision
	q.LineTotal, err = conversion.Convert(q.UnitPrice, "1", strconv.FormatInt(c.Quantity, 10))
	if err != nil {
		return q, err
	}
	price, _ := strconv.ParseFloat(q.UnitPrice, 64)
	snapshot := map[string]any{"source": "bom_tier", "price_selection_mode": "bom_tier", "selected_price_row_key": c.PriceRowKey, "publication_id": c.PublicationID, "version_no": version, "list_type": listType, "quantity_basis": "sales_spec_count", "inventory_unit": target.InventoryUnit, "price_unit": target.InventoryUnit, "display_unit": target.InventoryUnit, "final_unit_price": price, "spec_conversion": q, "pricing_rule_snapshot": row.Rule}
	data, _ := json.Marshal(snapshot)
	q.PriceSourceJSON = withOrderBOMSpecPriceSourceJSON(string(data), orderBOMSpecIdentity{ProductID: c.ProductID, BomSpecID: target.SpecID, BomVariantID: target.VariantID, BomSpecName: target.Name, InventoryUnit: target.InventoryUnit})
	return q, nil
}

// Recover only a positively identified legacy rule. Locks keep its parameters
// stable between the timestamp check, trial calculation and order persistence.
func (r Repository) legacyBOMRuleTx(ctx context.Context, tx pgx.Tx, row bomTierRow, c salesapp.SpecConversionQuoteRequest) (*costingapp.ProductPricingRule, string, string, error) {
	id := row.RuleID
	if row.TierRuleID > 0 {
		id = row.TierRuleID
	}
	if id <= 0 || row.PricingMode == "fixed" || row.PricingMode == "fixed_price" {
		return nil, "", "", fmt.Errorf("来源档位没有明确关联的计算模板；固定价请使用换算组，或选择有模板的档位")
	}
	var created, updated, published time.Time
	err := tx.QueryRow(ctx, fmt.Sprintf("SELECT r.created_at,r.updated_at,p.published_at FROM %[1]s.product_pricing_rules r JOIN %[1]s.bean_list_publications p ON p.id=$2 WHERE r.id=$1 FOR SHARE OF r", r.schema), id, c.PublicationID).Scan(&created, &updated, &published)
	if err != nil {
		return nil, "", "", fmt.Errorf("来源档位关联的计算模板不可用，请选择其他档位")
	}
	rule, err := costingpg.NewRepository(r.pool, r.schema).LoadProductPricingRule(postgresinfra.WithTransaction(ctx, tx), id)
	if err != nil || !rule.Active {
		return nil, "", "", fmt.Errorf("来源档位关联的计算模板已停用或不可用，请选择其他档位")
	}
	stamp := updated.UTC().Format(time.RFC3339Nano)
	data, _ := json.Marshal(struct {
		Rule                                                                   costingapp.ProductPricingRule
		Updated                                                                string
		Customer, Product, Publication, Source, Variant, Target, TargetVariant int64
		Key                                                                    string
	}{rule, stamp, c.CustomerID, c.ProductID, c.PublicationID, c.SourceSpecID, c.SourceVariantID, c.TargetSpecID, c.TargetVariantID, c.PriceRowKey})
	revision := fmt.Sprintf("%x", sha256.Sum256(data))
	if c.CurrentRuleRevision != "" {
		if c.CurrentRuleRevision != revision {
			return nil, "", "", &salesapp.PricingRuleConfirmationError{RuleName: rule.Name, Revision: revision}
		}
		return &rule, "confirmed_current_template", stamp, nil
	}
	unchanged := !created.IsZero() && !updated.IsZero() && !published.IsZero() && !updated.Before(created) && !updated.After(published) && !created.After(published)
	// Existing partial published metadata must not contradict the live rule.
	checks := map[string]any{"pricing_rule_id": rule.ID, "pricing_rule_name": rule.Name, "pricing_rule_code": rule.Code, "cost_source_mode": rule.CostSourceMode, "pricing_rule_formula_version": rule.FormulaVersion, "pricing_rule_calculation": rule.CalculationJSON}
	for key, want := range checks {
		if got, exists := row.CostSource[key]; exists {
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(want)
			if string(a) != string(b) {
				unchanged = false
			}
		}
	}
	if !unchanged {
		return nil, "", "", &salesapp.PricingRuleConfirmationError{RuleName: rule.Name, Revision: revision}
	}
	return &rule, "verified_legacy_template", stamp, nil
}
