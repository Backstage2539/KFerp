package finance

import (
	"context"
	"encoding/json"
	domain "orderapp/internal/domain/finance"
	"strings"
	"testing"
)

func TestUnknownCostReportMasksDerivedValuesAndPreventsClosing(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusDraft
	repo.totals = domain.MonthlySourceTotals{Month: "2026-11", RevenueTaxInclusive: 1000, MainBusinessCost: 200, CostUnavailable: true}
	service := NewService(repo)
	report, err := service.DraftReport(context.Background(), ReportFilter{Month: "2026-11"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.CostUnavailable {
		t.Fatal("missing cost unknown flag")
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"main_business_cost":null`, `"gross_profit":null`, `"adjusted_net_profit":null`, `"cit_payable":null`, `"total_tax":null`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("missing %s in %s", key, raw)
		}
	}
	if _, err = service.CloseMonth(context.Background(), CloseMonthCommand{Month: "2026-11", Actor: "test"}); err == nil || !strings.Contains(err.Error(), "成本待核算") {
		t.Fatalf("close error=%v", err)
	}
	if _, err = service.AccountantHandoff(context.Background(), ReportFilter{Month: "2026-11"}); err == nil || !strings.Contains(err.Error(), "成本待核算") {
		t.Fatalf("handoff error=%v", err)
	}
}

func TestUnknownCostDrilldownMasksAmounts(t *testing.T) {
	repo := newFakeRepo()
	repo.reportStatus = domain.MonthStatusDraft
	repo.totals = domain.MonthlySourceTotals{Month: "2026-11", CostUnavailable: true}
	repo.sourceDetails = []SourceDetail{{Section: "main_cost", SourceType: "production_cost", Name: "批次", Amount: 100}}
	drill, err := NewService(repo).ReportDrilldown(context.Background(), ReportFilter{Month: "2026-11"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(drill)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"total":null`, `"amount":null`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("missing %s in %s", key, raw)
		}
	}
}
