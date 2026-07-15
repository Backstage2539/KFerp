package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev537KMMCommercialOrderContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table  string
		code   string
		status string
	}{
		{table: "req_product", code: "PR-537-KMM-PRICING-ORDER-COMPAT", status: "review"},
		{table: "req_dev", code: "DEV-537-DRIP-COMMERCIAL-ORDER", status: "done"},
		{table: "req_dev", code: "DEV-537-LEGACY-DRIP-FALLBACK", status: "done"},
		{table: "req_dev", code: "DEV-537-KMM-DATA-ACCEPTANCE", status: "done"},
		{table: "req_review", code: "REV-537-KMM-PRICING-ORDER-COMPAT", status: "todo"},
	} {
		requireDev537SeedRow(t, reqStore, row.table, row.code, row.status)
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-537-KMM-PRICING-ORDER-COMPAT",
			"commercial` 发布版本及其 `price_rows`",
			"只读回退历史 `list_type=drip` 快照",
			"不能用旧价掩盖新发布缺陷",
			"只允许导入开发环境",
			"不得猜测物料、配方、售价或用 0 元兜底发布",
			"COGS 仅在对应生产工单发生实际物料耗用、完工和成本归集后形成",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K79. KMM 阶梯售价与挂耳通用价格表录单兼容",
			"挂耳的袋、盒派生子 SKU",
			"commercial 发布存在但缺价格行或阶梯无效时必须报错",
			"生产环境未写入、未部署、未切换入口",
			"系统没有猜测物料、BOM 或售价",
			"订单合计与财务收入一致",
			"没有生产工单实际耗用时不得伪造 COGS",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"挂耳袋、盒派生子 SKU 的新价格统一发布到 `commercial price_rows`",
			"只读回退历史 `list_type=drip` 快照",
			"不能用历史旧价掩盖新表缺陷",
			"KMM 开发环境导入与验收",
			"不要补猜配方、复制相似商品价格或用 0 元兜底",
			"只有实际生产工单发生物料耗用、完工并归集成本后才形成 COGS",
		},
		filepath.Join("docs", "acceptance", "2026-07-15-kmm-pricing-order-compat.md"): {
			"PR-537 KMM 阶梯售价与挂耳通用价格表录单兼容验收",
			"生产环境未部署、未写入",
			"共享 `ListTypeForProductKind` 继续保留历史 `drip` 映射",
			"前者存在但缺价则报错",
			"未匹配、名称歧义、空白或零价格、配方不完整时不猜测",
			"未发生生产耗用时记录“COGS 尚未形成”",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-537 marker %q", rel, want)
			}
		}
	}
}

func requireDev537SeedRow(t *testing.T, src, table, code, status string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s", table, code, status)
	}
}
