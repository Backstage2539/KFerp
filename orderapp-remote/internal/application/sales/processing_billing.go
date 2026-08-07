package sales

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	BillingBasisActualInputKG             = "actual_input_kg"
	BillingBasisActualOutputKG            = "actual_output_kg"
	BillingBasisActualMinutes             = "actual_minutes"
	BillingBasisActualUnits               = "actual_units"
	BillingBasisFixedPerWorkOrder         = "fixed_per_work_order"
	BillingBasisFactoryMaterialActualCost = "factory_material_actual_cost"

	ProcessingBillingRunKindStandard   = "standard"
	ProcessingBillingRunKindAdjustment = "adjustment"
	ProcessingBillingRunKindReversal   = "reversal"

	ProcessingBillingStatusConfirmed = "confirmed"
	ProcessingBillingStatusPaid      = "paid"
	ProcessingBillingStatusReversed  = "reversed"
)

var supportedProcessingBillingBases = map[string]bool{
	BillingBasisActualInputKG:             true,
	BillingBasisActualOutputKG:            true,
	BillingBasisActualMinutes:             true,
	BillingBasisActualUnits:               true,
	BillingBasisFixedPerWorkOrder:         true,
	BillingBasisFactoryMaterialActualCost: true,
}

func IsSupportedProcessingBillingBasis(value string) bool {
	return supportedProcessingBillingBases[strings.TrimSpace(value)]
}

func isSupportedProcessingBillingFeeType(value string) bool {
	switch strings.TrimSpace(value) {
	case "roasting", "labor", "material", "packaging", "processing", "storage":
		return true
	default:
		return false
	}
}

type OutsourceTemplateRuleInput struct {
	FeeType   string  `json:"fee_type"`
	Name      string  `json:"name"`
	Basis     string  `json:"basis"`
	UnitPrice float64 `json:"unit_price"`
	SortOrder int     `json:"sort_order"`
}

type OutsourceTemplateRule struct {
	ID        int64   `json:"id"`
	VersionID int64   `json:"version_id"`
	FeeType   string  `json:"fee_type"`
	Name      string  `json:"name"`
	Basis     string  `json:"basis"`
	UnitPrice float64 `json:"unit_price"`
	SortOrder int     `json:"sort_order"`
}

type ProcessingBillingCandidate struct {
	WorkOrderID   int64  `json:"work_order_id"`
	WorkOrderNo   string `json:"work_order_no"`
	CustomerID    int64  `json:"customer_id"`
	CustomerName  string `json:"customer_name"`
	RequestNo     string `json:"request_no"`
	ProductID     int64  `json:"product_id"`
	ProductName   string `json:"product_name"`
	SpecG         int64  `json:"spec_g"`
	Status        string `json:"status"`
	CompletedAt   string `json:"completed_at"`
	AlreadyBilled bool   `json:"already_billed"`
	BillingRunID  int64  `json:"billing_run_id"`
}

type ProcessingBillingCustomerOption struct {
	CustomerID   int64  `json:"customer_id"`
	CustomerName string `json:"customer_name"`
}

type ProcessingBillingOptions struct {
	Templates []OutsourceTemplate               `json:"templates"`
	Customers []ProcessingBillingCustomerOption `json:"customers"`
}

type ProcessingBillingMetrics struct {
	WorkOrderID               int64   `json:"work_order_id"`
	WorkOrderNo               string  `json:"work_order_no"`
	ProductName               string  `json:"product_name"`
	CompletedAt               string  `json:"completed_at"`
	ActualInputKG             float64 `json:"actual_input_kg"`
	ActualOutputKG            float64 `json:"actual_output_kg"`
	ActualMinutes             float64 `json:"actual_minutes"`
	ActualUnits               float64 `json:"actual_units"`
	FactoryMaterialActualCost float64 `json:"factory_material_actual_cost"`
}

type ProcessingBillingLine struct {
	WorkOrderID  int64   `json:"work_order_id"`
	WorkOrderNo  string  `json:"work_order_no"`
	RuleID       int64   `json:"rule_id"`
	FeeType      string  `json:"fee_type"`
	FeeName      string  `json:"fee_name"`
	Basis        string  `json:"basis"`
	BaseQuantity float64 `json:"base_quantity"`
	UnitPrice    float64 `json:"unit_price"`
	Amount       float64 `json:"amount"`
}

type PreviewProcessingBillingCommand struct {
	CustomerID        int64   `json:"customer_id"`
	TemplateID        int64   `json:"template_id"`
	TemplateVersionID int64   `json:"template_version_id,omitempty"`
	WorkOrderIDs      []int64 `json:"work_order_ids"`
}

type ProcessingBillingPreview struct {
	CustomerID        int64                      `json:"customer_id"`
	CustomerName      string                     `json:"customer_name"`
	TemplateID        int64                      `json:"template_id"`
	TemplateName      string                     `json:"template_name"`
	TemplateVersionID int64                      `json:"template_version_id"`
	TemplateVersionNo int                        `json:"template_version_no"`
	WorkOrders        []ProcessingBillingMetrics `json:"work_orders"`
	Lines             []ProcessingBillingLine    `json:"lines"`
	TotalAmount       float64                    `json:"total_amount"`
	Currency          string                     `json:"currency"`
}

type ConfirmProcessingBillingCommand struct {
	CustomerID        int64   `json:"customer_id"`
	TemplateVersionID int64   `json:"template_version_id"`
	WorkOrderIDs      []int64 `json:"work_order_ids"`
	Actor             string  `json:"-"`
}

type ProcessingBillingConfirmation struct {
	BillingRunID      int64   `json:"billing_run_id"`
	SettlementBatchID int64   `json:"settlement_batch_id"`
	SettlementNo      string  `json:"settlement_no"`
	TotalAmount       float64 `json:"total_amount"`
	Currency          string  `json:"currency"`
	Reused            bool    `json:"reused"`
}

type ProcessingBillingRunsQuery struct {
	CustomerID int64 `json:"customer_id"`
	Limit      int   `json:"limit"`
}

type ProcessingBillingRun struct {
	ID                 int64   `json:"id"`
	CustomerID         int64   `json:"customer_id"`
	CustomerName       string  `json:"customer_name"`
	TemplateID         int64   `json:"template_id"`
	TemplateVersionID  int64   `json:"template_version_id"`
	SettlementBatchID  int64   `json:"settlement_batch_id"`
	SettlementNo       string  `json:"settlement_no"`
	RunKind            string  `json:"run_kind"`
	SourceBillingRunID int64   `json:"source_billing_run_id"`
	Status             string  `json:"status"`
	TotalAmount        float64 `json:"total_amount"`
	Currency           string  `json:"currency"`
	WorkOrderCount     int     `json:"work_order_count"`
	ConfirmedAt        string  `json:"confirmed_at"`
	PaidAt             string  `json:"paid_at"`
	ReversedAt         string  `json:"reversed_at"`
	Reason             string  `json:"reason"`
}

type PayProcessingBillingCommand struct {
	BillingRunID int64  `json:"-"`
	Note         string `json:"note"`
	Actor        string `json:"-"`
}

type ReverseProcessingBillingCommand struct {
	BillingRunID int64  `json:"-"`
	Reason       string `json:"reason"`
	Actor        string `json:"-"`
}

type ProcessingBillingAdjustmentLineInput struct {
	WorkOrderID int64   `json:"work_order_id"`
	FeeType     string  `json:"fee_type"`
	FeeName     string  `json:"fee_name"`
	Amount      float64 `json:"amount"`
}

type AdjustProcessingBillingCommand struct {
	BillingRunID int64                                  `json:"-"`
	Reason       string                                 `json:"reason"`
	Lines        []ProcessingBillingAdjustmentLineInput `json:"lines"`
	Actor        string                                 `json:"-"`
	RequestKey   string                                 `json:"-"`
}

type ProcessingBillingLifecycleResult struct {
	BillingRunID       int64   `json:"billing_run_id"`
	SourceBillingRunID int64   `json:"source_billing_run_id"`
	SettlementBatchID  int64   `json:"settlement_batch_id"`
	SettlementNo       string  `json:"settlement_no"`
	Status             string  `json:"status"`
	TotalAmount        float64 `json:"total_amount"`
	Currency           string  `json:"currency"`
	Reused             bool    `json:"reused"`
}

func (s *Service) ListProcessingBillingCandidates(ctx context.Context, customerID int64) ([]ProcessingBillingCandidate, error) {
	if customerID <= 0 {
		return nil, fmt.Errorf("customer_id required")
	}
	return s.repo.ListProcessingBillingCandidates(ctx, customerID)
}

// GetProcessingBillingOptions returns only the small, finance-safe directory
// needed to create customer processing bills. Template maintenance continues to
// use the settings.write endpoint.
func (s *Service) GetProcessingBillingOptions(ctx context.Context) (ProcessingBillingOptions, error) {
	templates, err := s.repo.ListOutsourceTemplates(ctx)
	if err != nil {
		return ProcessingBillingOptions{}, err
	}
	customers, err := s.repo.ListProcessingBillingCustomerOptions(ctx)
	if err != nil {
		return ProcessingBillingOptions{}, err
	}
	billable := make([]OutsourceTemplate, 0, len(templates))
	for _, template := range templates {
		if template.CurrentVersionID <= 0 || template.CurrentVersionStatus != "published" || len(template.Rules) == 0 {
			continue
		}
		billable = append(billable, template)
	}
	return ProcessingBillingOptions{Templates: billable, Customers: customers}, nil
}

func (s *Service) PreviewProcessingBilling(ctx context.Context, cmd PreviewProcessingBillingCommand) (ProcessingBillingPreview, error) {
	if cmd.CustomerID <= 0 {
		return ProcessingBillingPreview{}, fmt.Errorf("customer_id required")
	}
	if cmd.TemplateID <= 0 && cmd.TemplateVersionID <= 0 {
		return ProcessingBillingPreview{}, fmt.Errorf("template_id required")
	}
	cmd.WorkOrderIDs = normalizedPositiveIDs(cmd.WorkOrderIDs)
	if len(cmd.WorkOrderIDs) == 0 {
		return ProcessingBillingPreview{}, fmt.Errorf("work_order_ids required")
	}
	return s.repo.PreviewProcessingBilling(ctx, cmd)
}

func (s *Service) ConfirmProcessingBilling(ctx context.Context, cmd ConfirmProcessingBillingCommand) (ProcessingBillingConfirmation, error) {
	if cmd.CustomerID <= 0 {
		return ProcessingBillingConfirmation{}, fmt.Errorf("customer_id required")
	}
	if cmd.TemplateVersionID <= 0 {
		return ProcessingBillingConfirmation{}, fmt.Errorf("template_version_id required")
	}
	cmd.WorkOrderIDs = normalizedPositiveIDs(cmd.WorkOrderIDs)
	if len(cmd.WorkOrderIDs) == 0 {
		return ProcessingBillingConfirmation{}, fmt.Errorf("work_order_ids required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Actor == "" {
		cmd.Actor = "unknown"
	}
	return s.repo.ConfirmProcessingBilling(ctx, cmd)
}

func (s *Service) ListProcessingBillingRuns(ctx context.Context, query ProcessingBillingRunsQuery) ([]ProcessingBillingRun, error) {
	if query.CustomerID <= 0 {
		return nil, fmt.Errorf("customer_id required")
	}
	if query.Limit <= 0 {
		query.Limit = 100
	}
	if query.Limit > 200 {
		query.Limit = 200
	}
	return s.repo.ListProcessingBillingRuns(ctx, query)
}

func (s *Service) PayProcessingBilling(ctx context.Context, cmd PayProcessingBillingCommand) (ProcessingBillingLifecycleResult, error) {
	if cmd.BillingRunID <= 0 {
		return ProcessingBillingLifecycleResult{}, fmt.Errorf("billing_run_id required")
	}
	cmd.Actor = normalizedProcessingBillingActor(cmd.Actor)
	cmd.Note = strings.TrimSpace(cmd.Note)
	return s.repo.PayProcessingBilling(ctx, cmd)
}

func (s *Service) ReverseProcessingBilling(ctx context.Context, cmd ReverseProcessingBillingCommand) (ProcessingBillingLifecycleResult, error) {
	if cmd.BillingRunID <= 0 {
		return ProcessingBillingLifecycleResult{}, fmt.Errorf("billing_run_id required")
	}
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	if cmd.Reason == "" {
		return ProcessingBillingLifecycleResult{}, fmt.Errorf("reason required")
	}
	cmd.Actor = normalizedProcessingBillingActor(cmd.Actor)
	return s.repo.ReverseProcessingBilling(ctx, cmd)
}

func (s *Service) AdjustProcessingBilling(ctx context.Context, cmd AdjustProcessingBillingCommand) (ProcessingBillingLifecycleResult, error) {
	if cmd.BillingRunID <= 0 {
		return ProcessingBillingLifecycleResult{}, fmt.Errorf("billing_run_id required")
	}
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	if cmd.Reason == "" {
		return ProcessingBillingLifecycleResult{}, fmt.Errorf("reason required")
	}
	if len(cmd.Lines) == 0 {
		return ProcessingBillingLifecycleResult{}, fmt.Errorf("adjustment lines required")
	}
	total := 0.0
	for i := range cmd.Lines {
		line := &cmd.Lines[i]
		line.FeeType = strings.TrimSpace(line.FeeType)
		line.FeeName = strings.TrimSpace(line.FeeName)
		line.Amount = roundProcessingMoney(line.Amount)
		if line.WorkOrderID < 0 {
			return ProcessingBillingLifecycleResult{}, fmt.Errorf("work_order_id invalid")
		}
		if !isSupportedProcessingBillingFeeType(line.FeeType) {
			return ProcessingBillingLifecycleResult{}, fmt.Errorf("unsupported processing fee type: %s", line.FeeType)
		}
		if line.FeeName == "" {
			return ProcessingBillingLifecycleResult{}, fmt.Errorf("fee_name required")
		}
		if line.Amount == 0 {
			return ProcessingBillingLifecycleResult{}, fmt.Errorf("adjustment amount must not be zero")
		}
		total = roundProcessingMoney(total + line.Amount)
	}
	if total == 0 {
		return ProcessingBillingLifecycleResult{}, fmt.Errorf("adjustment total must not be zero")
	}
	cmd.Actor = normalizedProcessingBillingActor(cmd.Actor)
	payload, err := json.Marshal(struct {
		BillingRunID int64                                  `json:"billing_run_id"`
		Reason       string                                 `json:"reason"`
		Lines        []ProcessingBillingAdjustmentLineInput `json:"lines"`
	}{BillingRunID: cmd.BillingRunID, Reason: cmd.Reason, Lines: cmd.Lines})
	if err != nil {
		return ProcessingBillingLifecycleResult{}, err
	}
	cmd.RequestKey = fmt.Sprintf("%x", sha256.Sum256(payload))
	return s.repo.AdjustProcessingBilling(ctx, cmd)
}

func normalizedProcessingBillingActor(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func normalizedPositiveIDs(values []int64) []int64 {
	seen := make(map[int64]bool, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func CalculateProcessingBillingLines(metrics ProcessingBillingMetrics, rules []OutsourceTemplateRule) ([]ProcessingBillingLine, float64, error) {
	lines := make([]ProcessingBillingLine, 0, len(rules))
	total := 0.0
	for _, rule := range rules {
		var basis float64
		switch rule.Basis {
		case BillingBasisActualInputKG:
			basis = metrics.ActualInputKG
		case BillingBasisActualOutputKG:
			basis = metrics.ActualOutputKG
		case BillingBasisActualMinutes:
			basis = metrics.ActualMinutes
		case BillingBasisActualUnits:
			basis = metrics.ActualUnits
		case BillingBasisFixedPerWorkOrder:
			basis = 1
		case BillingBasisFactoryMaterialActualCost:
			basis = metrics.FactoryMaterialActualCost
		default:
			return nil, 0, fmt.Errorf("unsupported processing billing basis: %s", rule.Basis)
		}
		basis = roundProcessingBillingQuantity(basis)
		amount := roundProcessingMoney(basis * rule.UnitPrice)
		lines = append(lines, ProcessingBillingLine{
			WorkOrderID:  metrics.WorkOrderID,
			WorkOrderNo:  metrics.WorkOrderNo,
			RuleID:       rule.ID,
			FeeType:      rule.FeeType,
			FeeName:      rule.Name,
			Basis:        rule.Basis,
			BaseQuantity: basis,
			UnitPrice:    rule.UnitPrice,
			Amount:       amount,
		})
		total = roundProcessingMoney(total + amount)
	}
	return lines, total, nil
}

func roundProcessingMoney(value float64) float64 {
	return math.Round((value+1e-9)*100) / 100
}

func roundProcessingBillingQuantity(value float64) float64 {
	return math.Round((value+1e-12)*10000) / 10000
}
