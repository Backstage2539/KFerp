package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev167ProductBomDeactivationRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-167",
		"DEV-167-01",
		"DEV-167-02",
		"DEV-167-03",
		"UT-167-01",
		"API-167-01",
		"REV-167-01",
		"产品和 BOM 失效",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 167 product/bom deactivation seed missing %q", want)
		}
	}
}

func TestDev167ProductAndBomSoftDeactivationWiring(t *testing.T) {
	cases := []struct {
		rel   string
		wants []string
	}{
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "bom", "schema.go"),
			wants: []string{
				"status TEXT NOT NULL DEFAULT 'active'",
				"ALTER TABLE %s.product_bom ADD COLUMN IF NOT EXISTS status",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"),
			wants: []string{
				"func (r Repository) DeactivateBom",
				"ON CONFLICT (product_id) DO UPDATE SET status='inactive'",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"),
			wants: []string{
				"func (r Repository) DeactivateProducts",
				"SET active=false",
				"product_bom SET status='inactive'",
			},
		},
	}
	for _, tc := range cases {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product/bom deactivation marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev167VueShowsProductMultiDeactivateAndBomInactiveWarnings(t *testing.T) {
	cases := []struct {
		rel   string
		wants []string
	}{
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"),
			wants: []string{
				"selectedProductIds",
				"/api/product-settings/products/deactivate",
				"失效选中产品",
				"失效",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"),
			wants: []string{
				"批量失效",
				"deactivateSelectedProductionBoms",
				"bomStatusLabel",
				"当前 BOM 已失效",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"),
			wants: []string{
				"item.warnings",
				"BOM已失效",
				"warning-icon",
				"warningTooltip",
			},
		},
	}
	for _, tc := range cases {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing deactivation UI marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev167ManualsDocumentProductBomDeactivation(t *testing.T) {
	rels := []string{
		"docs/OP_MANUAL_INVENTORY_MATERIALS.md",
		"docs/OP_MANUAL_COSTING.md",
		"docs/REQUIREMENTS.md",
		"docs/ACCEPTANCE_TESTS.md",
	}
	root := filepath.Join(findAncestorForTest(t, "go.mod"), "..")
	for _, name := range []string{"OP_MANUAL_INVENTORY_MATERIALS.md", "OP_MANUAL_COSTING.md", "REQUIREMENTS.md", "ACCEPTANCE_TESTS.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			rels = append(rels, filepath.Join("..", name))
		}
	}
	for _, rel := range rels {
		doc := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"产品失效", "BOM失效", "BOM已失效"} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing product/bom deactivation manual marker %q", rel, want)
			}
		}
	}
}
