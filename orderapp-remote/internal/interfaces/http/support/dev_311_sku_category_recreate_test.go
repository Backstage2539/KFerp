package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev311SkuCategoryRecreateAfterDelete(t *testing.T) {
	markers := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-311-SKU-CATEGORY-RECREATE-AFTER-DELETE",
			"DEV-311-SKU-CATEGORY-ACTIVE-ONLY-UNIQUE-INDEX",
			"UT-311-SKU-CATEGORY-RECREATE-AFTER-DELETE",
			"API-311-SKU-CATEGORY-RECREATE-AFTER-DELETE",
			"REV-311-SKU-CATEGORY-RECREATE-AFTER-DELETE",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"DROP INDEX IF EXISTS %[1]s.product_categories_customer_parent_name_uniq",
			"CREATE UNIQUE INDEX product_categories_customer_parent_name_uniq",
			"WHERE active=true",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"function isDuplicatedPublicCategory",
			"!(category.products || []).length && !(category.children || []).length",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.test.js"): {
			"customer category tree keeps empty owned categories that share a public category name",
			"customer_id: 74",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-311-SKU-CATEGORY-RECREATE-AFTER-DELETE",
			"删除客户自己的一级或二级商品分类后",
			"不得触发 `product_categories_customer_parent_name_uniq`",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-311-SKU-CATEGORY-RECREATE-AFTER-DELETE",
			"芬纳等客户 SKU 归属下新增一级分类",
			"不拦截软删除历史分类",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"删除客户自己的商品分类后",
			"重新新增同名分类",
			"product_categories_customer_parent_name_uniq",
		},
		filepath.Join("docs", "acceptance", "2026-05-22-sku-category-recreate-after-delete.md"): {
			"SKU 分类删除后同名重建",
			"唯一约束只约束 `active=true`",
			"页面显示 0 个分类",
			"再次新增同名",
		},
	}
	for rel, wants := range markers {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing SKU category recreate marker %q", rel, want)
			}
		}
	}
}
