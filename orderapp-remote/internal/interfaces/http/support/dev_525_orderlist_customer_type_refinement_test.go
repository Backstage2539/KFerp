package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev525OrderlistCustomerTypeRefinementContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-525-ORDERLIST-CUSTOMER-TYPE-REFINEMENT",
			"DEV-525-REMARK-CUSTOMER-IDENTITY",
			"DEV-525-CUSTOMER-TYPE-INFERENCE",
			"DEV-525-CUSTOMER-REVIEW-WORKBOOK",
		},
		filepath.Join("internal", "migration", "orderliststaging", "customer_import.go"): {
			"customer_remark_name_unresolved",
			"InferredCustomerType",
			"DeliveryAddressCount",
		},
		filepath.Join("scripts", "orderlist-staging", "build-review.mjs"): {
			"推断客户类型",
			"客户类型判定依据",
			"收件地址样本",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-525-ORDERLIST-CUSTOMER-TYPE-REFINEMENT",
			"备注为空",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-525-ORDERLIST-CUSTOMER-TYPE-REFINEMENT",
			"渠道客户",
		},
		filepath.Join("docs", "OP_MANUAL_ORDERLIST_STAGING.md"): {
			"备注客户",
			"规范收件地址",
		},
		filepath.Join("docs", "acceptance", "2026-07-11-orderlist-customer-type-refinement.md"): {
			"PR-525",
			"正式客户表零写入",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-525 marker %q", rel, want)
			}
		}
	}
}
