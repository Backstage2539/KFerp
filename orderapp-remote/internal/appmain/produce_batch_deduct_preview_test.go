package appmain

import (
	"encoding/json"
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"
)

const deductPreviewContractRoute = "/api/produce/batch/:batch_id/deduct-preview"

func TestDeductPreviewAPIContractIncludesLowStockWarning(t *testing.T) {
	payload := productionPreviewFromApp(productionapp.DeductPreview{
		BatchID: "PB-1",
		Summary: []productionapp.DeductPreviewItem{{
			ProductID:       7,
			ProductName:     "橘皮乌龙",
			SpecG:           227,
			NeedUnits:       2,
			NeedG:           454,
			InvUnits:        2,
			InvLooseG:       0,
			InvTotalG:       454,
			DeductedG:       454,
			GapG:            0,
			WarningLowStock: true,
		}},
	})
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, want := range []string{"/api/produce/batch/", "deduct-preview", "warning_low_stock"} {
		if !strings.Contains(deductPreviewContractRoute+src, want) {
			t.Fatalf("deduct-preview contract missing %q in route=%s body=%s", want, deductPreviewContractRoute, src)
		}
	}
}
