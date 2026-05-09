package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev168OrderItemNotesRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-168",
		"DEV-168-01",
		"DEV-168-02",
		"DEV-168-03",
		"UT-168-01",
		"API-168-01",
		"REV-168-01",
		"条目备注",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 168 order item notes seed missing %q", want)
		}
	}
}

func TestDev168OrderItemNotesFlowThroughAPISnapshotsAndViews(t *testing.T) {
	cases := []struct {
		rel   string
		wants []string
	}{
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "core", "schema.go"),
			wants: []string{
				"item_note TEXT NOT NULL DEFAULT ''",
				"ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS item_note",
			},
		},
		{
			rel: filepath.Join("internal", "interfaces", "http", "sales", "order_api.go"),
			wants: []string{
				"ItemNote  []string `json:\"item_note\"`",
				"ItemNote:              r.ItemNote",
				"Note        string `json:\"note\"`",
			},
		},
		{
			rel: filepath.Join("internal", "interfaces", "http", "sales", "order_sales_mapping.go"),
			wants: []string{
				"req.ItemNote",
				"Note: strings.TrimSpace(getStr(req.ItemNote, i))",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go"),
			wants: []string{
				"item_note",
				"note:        strings.TrimSpace(src.Note)",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "sales_order_repository.go"),
			wants: []string{
				"COALESCE(oi.item_note,'')",
				"&item.Note",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "delivery_note_repository.go"),
			wants: []string{
				"COALESCE(oi.item_note,'')",
				"&item.Note",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"),
			wants: []string{
				"条目备注",
				"v-model.trim=\"row.item_note\"",
				"item_note: item.note || ''",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue"),
			wants: []string{
				"<th>备注</th>",
				"item.note || '-'",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "DeliveryNoteView.vue"),
			wants: []string{
				"<th>备注</th>",
				"item.note || '-'",
			},
		},
	}
	for _, tc := range cases {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing order item notes marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev168OrderItemNotesRenderInGeneratedDocuments(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go"),
		filepath.Join("internal", "infrastructure", "pdf", "sales_order_png.go"),
		filepath.Join("internal", "infrastructure", "pdf", "delivery_note_pdf.go"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"备注", "item.Note"} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing document item note marker %q", rel, want)
			}
		}
	}
}

func TestDev168ManualDocumentsOrderItemNotes(t *testing.T) {
	rels := []string{
		"docs/OP_MANUAL_ORDER_SALES.md",
		"docs/REQUIREMENTS.md",
		"docs/ACCEPTANCE_TESTS.md",
	}
	root := filepath.Join(findAncestorForTest(t, "go.mod"), "..")
	for _, name := range []string{"OP_MANUAL_ORDER_SALES.md", "REQUIREMENTS.md", "ACCEPTANCE_TESTS.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			rels = append(rels, filepath.Join("..", name))
		}
	}
	for _, rel := range rels {
		doc := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"条目备注", "销售单", "出库单"} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing order item notes manual marker %q", rel, want)
			}
		}
	}
}
