package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev490JobCardBatchCardsContracts(t *testing.T) {
	files := map[string]string{
		"reqStore":        filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"producePlanView": filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"),
		"jobCardsView":    filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"),
		"producePlanTest": filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.test.js"),
		"workOrdersTest":  filepath.Join("frontend-vue-shell", "src", "lib", "work-orders.test.js"),
		"requirements":    filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":      filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":          filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		"evidence":        filepath.Join("docs", "acceptance", "2026-06-12-job-card-batch-cards.md"),
	}
	contents := map[string]string{}
	for key, rel := range files {
		contents[key] = string(readOrderAppFileForTest(t, rel))
	}

	for _, marker := range []string{
		"PR-490-JOB-CARD-BATCH-CARDS",
		"DEV-490-SPLIT-BATCH-CARDS",
		"DEV-490-JOB-CARD-GENERIC-ACTUALS",
	} {
		if !strings.Contains(contents["reqStore"], marker) {
			t.Fatalf("req_store missing %s", marker)
		}
	}

	for _, marker := range []string{
		"productionPlanSplitBatchCards",
		"split-batch-cards",
		"split-batch-card",
		"不足标准批量",
	} {
		if !strings.Contains(contents["producePlanView"], marker) {
			t.Fatalf("ProducePlanView missing %s", marker)
		}
		if !strings.Contains(contents["producePlanTest"], marker) {
			t.Fatalf("produce-plan.test missing %s", marker)
		}
	}

	jobCardsTemplate := contents["jobCardsView"]
	if idx := strings.Index(jobCardsTemplate, "<script setup>"); idx >= 0 {
		jobCardsTemplate = jobCardsTemplate[:idx]
	}
	for _, forbidden := range []string{"计划投入", "实际投入", "实际产出", `v-model.number="draftFor(row).planned_input_qty"`, `v-model.number="draftFor(row).actual_input_qty"`, `v-model.number="draftFor(row).actual_output_qty"`, "<input", "保存实际"} {
		if strings.Contains(jobCardsTemplate, forbidden) {
			t.Fatalf("JobCardsView template must not expose %s", forbidden)
		}
	}
	for _, marker := range []string{"工序要求", "实际分钟", "实际工序成本", "实际损耗", "损耗原因", "异常原因", "进入工位", "执行枢纽"} {
		if !strings.Contains(jobCardsTemplate, marker) {
			t.Fatalf("JobCardsView template missing %s", marker)
		}
	}
	for _, forbidden := range []string{"runJobCardAction", "saveActuals"} {
		if strings.Contains(contents["jobCardsView"], forbidden) {
			t.Fatalf("JobCardsView must delegate execution to workstation and omit %s", forbidden)
		}
	}
	if !strings.Contains(contents["workOrdersTest"], "job card main table is a read-only execution record") {
		t.Fatal("work-orders.test missing read-only job card contract")
	}

	for _, key := range []string{"requirements", "acceptance", "manual", "evidence"} {
		if !strings.Contains(contents[key], "PR-490-JOB-CARD-BATCH-CARDS") {
			t.Fatalf("%s missing PR-490 marker", key)
		}
	}
}
