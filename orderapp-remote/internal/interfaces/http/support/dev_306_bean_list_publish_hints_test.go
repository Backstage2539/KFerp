package support

import (
	"strings"
	"testing"
)

func TestDev306BeanListPublishHintsSeed(t *testing.T) {
	store := string(readOrderAppFileForTest(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-306-BEAN-LIST-PUBLISH-HINTS",
		"DEV-306-BEAN-LIST-PUBLISH-HINTS",
		"UT-306-BEAN-LIST-PUBLISH-HINTS",
		"API-306-BEAN-LIST-PUBLISH-HINTS",
		"REV-306-BEAN-LIST-PUBLISH-HINTS",
		"docs/acceptance/2026-05-21-bean-list-publish-hints.md",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-306 seed marker %q", want)
		}
	}
}

func TestDev306BeanListPublishHintsDocs(t *testing.T) {
	markers := map[string][]string{
		"docs/REQUIREMENTS.md": {
			"PR-306-BEAN-LIST-PUBLISH-HINTS",
			"`V1` 变 `V1.01`",
			"生成并发布新版豆单后，录单才会使用新价格",
		},
		"docs/ACCEPTANCE_TESTS.md": {
			"梯度按 KG，单价按元/磅",
			"当前客户已有 `V1` 发布版本",
		},
		"docs/OP_MANUAL_COSTING.md": {
			"页面会提示“梯度按 KG，单价按元/磅”",
			"`V3.0.5` 默认 `V3.0.6`",
		},
		"docs/OP_MANUAL_GREEN_BEAN_SALES.md": {
			"按模板 KG 档位理解数量区间",
			"生成并发布新版豆单后，录单和客户下单才会使用新价格",
		},
		"docs/acceptance/2026-05-21-bean-list-publish-hints.md": {
			"客户豆单生成时默认版本号",
			"`V1.01`",
		},
	}
	for rel, wants := range markers {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing publish hints marker %q", rel, want)
			}
		}
	}
}
