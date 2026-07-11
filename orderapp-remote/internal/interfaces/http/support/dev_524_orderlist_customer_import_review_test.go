package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev524OrderlistCustomerImportReviewContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-524-ORDERLIST-CUSTOMER-IMPORT-REVIEW",
			"DEV-524-CUSTOMER-IDENTITY",
			"DEV-524-LATEST-PHONE",
			"DEV-524-ERP-FIELD-CONTRACT",
			"DEV-524-REVIEW-WORKBOOK",
		},
		filepath.Join("internal", "migration", "orderliststaging", "customer_import.go"): {
			"BuildCustomerImportRows",
			"customer_cross_phone_name_unsafe",
			"LatestPhoneObservedDate",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-524-ORDERLIST-CUSTOMER-IMPORT-REVIEW",
			"客户导入审核",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-524-ORDERLIST-CUSTOMER-IMPORT-REVIEW",
			"最近订单记录中的唯一有效号码",
		},
		filepath.Join("docs", "OP_MANUAL_ORDERLIST_STAGING.md"): {
			"客户导入审核",
			"历史号码",
		},
		filepath.Join("docs", "acceptance", "2026-07-11-orderlist-customer-import-review.md"): {
			"PR-524",
			"正式客户表零写入",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-524 marker %q", rel, want)
			}
		}
	}
}
