package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev277SalesAndDeliveryUsePDFStampPreview(t *testing.T) {
	checks := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "sales order",
			path: filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue"),
			want: []string{
				"PDFStampPreview",
				"`/api/orders/${orderID.value}/sales-order-preview.pdf`",
				"PREVIEW 预览版",
				"@placement-commit",
				"pdfPlacementToSalesSealMM",
				"salesSealMMToPDFPlacement",
			},
		},
		{
			name: "delivery note",
			path: filepath.Join("frontend-vue-shell", "src", "views", "DeliveryNoteView.vue"),
			want: []string{
				"PDFStampPreview",
				"`/api/orders/${orderID.value}/delivery-note-preview.pdf`",
				"PREVIEW 预览版",
				"@placement-commit",
				"pdfPlacementToSalesSealMM",
				"salesSealMMToPDFPlacement",
			},
		},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			src := string(readOrderAppFileForTest(t, tc.path))
			for _, want := range tc.want {
				if !strings.Contains(src, want) {
					t.Fatalf("%s missing PDF preview marker %q", tc.path, want)
				}
			}
		})
	}
}

