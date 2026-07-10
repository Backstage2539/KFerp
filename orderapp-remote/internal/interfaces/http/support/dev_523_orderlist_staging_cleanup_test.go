package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev523OrderlistStagingCleanupContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-523-ORDERLIST-STAGING-CLEANUP",
			"DEV-523-ORDERLIST-ETL",
			"DEV-523-STAGING-DATABASE",
			"DEV-523-REVIEW-WORKBOOK",
			"API-523-ORDERLIST-STAGING-CLEANUP",
		},
		filepath.Join("internal", "migration", "orderliststaging", "source_keys.go"): {
			"AssignSourceKeys",
			"duplicate_suffix_collision",
		},
		filepath.Join("internal", "migration", "orderliststaging", "schema.go"): {
			"CREATE SCHEMA IF NOT EXISTS raw",
			"CREATE SCHEMA IF NOT EXISTS curated",
			"raw.order_revisions",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-523-ORDERLIST-STAGING-CLEANUP",
			"工作表名:A列有效序号",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-523-ORDERLIST-STAGING-CLEANUP",
			"2,109 条原始业务行",
		},
		filepath.Join("docs", "OP_MANUAL_ORDERLIST_STAGING.md"): {
			"source-key-mapping.json",
			"正式 `nocodb`",
		},
		filepath.Join("docs", "acceptance", "2026-07-10-orderlist-staging-cleanup.md"): {
			"PR-523",
			"零写入",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-523 marker %q", rel, want)
			}
		}
	}
}
