package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev412ClassificationConfigTemplateInheritanceSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-412-CLASSIFICATION-CONFIG-TEMPLATE-INHERITANCE",
		"DEV-412-CLASSIFICATION-TEMPLATE-CONFIG-REFERENCE",
		"DEV-412-PRICE-LIST-CONFIG-INHERITANCE",
		"商品引用模板>子类引用模板>大类引用模板",
		"分类模板",
		"商品配置模板",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing classification config template inheritance marker %q", want)
		}
	}
}
