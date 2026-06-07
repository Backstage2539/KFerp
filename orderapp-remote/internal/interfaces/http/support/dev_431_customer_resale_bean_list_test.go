package support

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerResaleBeanListRequirementSeedsExist(t *testing.T) {
	body, err := os.ReadFile(supportFilePath(t, "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	src := string(body)
	for _, want := range []string{
		"PR-431-CUSTOMER-RESALE-BEAN-LIST",
		"DEV-431-BEAN-LIST-PURPOSE-SNAPSHOT",
		"DEV-431-GRADIENT-TEMPLATE-AUTHORIZATION",
		"DEV-431-MINI-RESALE-BEAN-LIST-API",
		"DEV-431-RESALE-PRICE-CALCULATION",
		"DEV-431-MINIAPP-LIGHT-EDITOR",
		"DEV-431-COSTING-PURPOSE-FILTER",
		"REV-431-CUSTOMER-RESALE-BEAN-LIST",
		"生成 PDF 和长图分享",
		"不影响工厂履约计价",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer resale bean list requirement seed missing %q", want)
		}
	}
}

func TestCustomerResaleBeanListWiringAndManuals(t *testing.T) {
	checks := []struct {
		path string
		want []string
	}{
		{
			path: "orderapp-remote/docs/REQUIREMENTS.md",
			want: []string{
				"PR-431-CUSTOMER-RESALE-BEAN-LIST",
				"publication_purpose=factory_supply",
				"publication_purpose=customer_resale",
				"允许客户转售豆单使用",
			},
		},
		{
			path: "orderapp-remote/docs/ACCEPTANCE_TESTS.md",
			want: []string{
				"客户转售价格按授权模板档位匹配来源豆单价格",
				"PDF 和服务端长图 PNG",
				"当前客户绑定和 `bean_list` 能力",
			},
		},
		{
			path: "orderapp-remote/docs/OP_MANUAL_COSTING.md",
			want: []string{
				"客户转售豆单的 `publication_purpose=customer_resale`",
				"“用途”筛选",
				"允许客户转售豆单使用",
			},
		},
		{
			path: "orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md",
			want: []string{
				"客户自有销售豆单",
				"发布销售豆单",
				"`customer_resale` 发布快照",
			},
		},
		{
			path: "orderapp-remote/docs/customer-portal-miniapp-test.md",
			want: []string{
				"我的商品联调",
				"GET /app/api/mini/resale-bean-lists",
				"GET /app/api/mini/resale-bean-lists/:id.png",
			},
		},
		{
			path: "miniapp/src/pages/service/service.vue",
			want: []string{
				"已发布商品价格表",
				"发布商品价格表",
				"openResaleOutput(item, 'png')",
			},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/views/CostingView.vue",
			want: []string{
				"publicationPurposeFilter",
				"客户转售价格表",
				"factory_supply",
			},
		},
		{
			path: "orderapp-remote/internal/interfaces/http/customerportal/mini_api.go",
			want: []string{
				"/api/mini/resale-bean-lists",
				`/api/mini/resale-bean-lists/:file`,
				"RenderPNG",
			},
		},
	}

	for _, check := range checks {
		body, err := os.ReadFile(repoFilePath(t, check.path))
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		src := string(body)
		for _, want := range check.want {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", check.path, want)
			}
		}
	}
}
