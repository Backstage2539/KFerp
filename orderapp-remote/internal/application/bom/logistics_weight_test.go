package bom

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestPR706LogisticsWeightDraftsValidateAndRoundTrip(t *testing.T) {
	for _, rawWeight := range []string{"0", "-1", "1e-8", "1000000001", "null", "227.125"} {
		t.Run(rawWeight, func(t *testing.T) {
			payload := `{"spec_key":"bag","name":"袋装","inventory_unit":"袋","is_default":true,"logistics_weight_g":` + rawWeight + `}`
			var template ProductionBomSpecTemplateVariant
			if err := json.Unmarshal([]byte(payload), &template); err != nil {
				t.Fatal(err)
			}
			var draft ProductionBomDraftVariant
			if err := json.Unmarshal([]byte(payload), &draft); err != nil {
				t.Fatal(err)
			}
			invalid := rawWeight == "0" || rawWeight == "-1" || rawWeight == "1e-8" || rawWeight == "1000000001"
			if err := normalizeProductionBomSpecTemplateVariants([]ProductionBomSpecTemplateVariant{template}); (err != nil) != invalid || err != nil && !strings.Contains(err.Error(), "logistics_weight_g") {
				t.Errorf("template weight %s error=%v", rawWeight, err)
			}
			_, err := NewService(&fakeRepo{}).UpdateProductionBomVersionDraft(context.Background(), UpdateProductionBomVersionDraftCommand{VersionID: 1, Variants: []ProductionBomDraftVariant{draft}})
			if (err != nil) != invalid || err != nil && !strings.Contains(err.Error(), "logistics_weight_g") {
				t.Errorf("draft weight %s error=%v", rawWeight, err)
			}
			for _, dto := range []any{template, draft, ProductionBomVersionVariant{}, ProductionBomPublishedSpec{}} {
				encoded, err := json.Marshal(dto)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(encoded), `"logistics_weight_g":`) {
					t.Errorf("DTO %T drops nullable logistics_weight_g: %s", dto, encoded)
				}
			}
		})
	}
}

func TestPR706LogisticsWeightRejectsNonFiniteAndRequiresPublishedValue(t *testing.T) {
	for _, value := range []float64{0, -1, 0.00000001, 1000000001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := ValidateLogisticsWeightG(&value, false); err == nil {
			t.Errorf("accepted %v", value)
		}
	}
	if err := ValidateLogisticsWeightG(nil, true); err == nil {
		t.Fatal("published missing weight accepted")
	}
	for _, value := range []float64{0.000001, 0.125, 1000000000} {
		if err := ValidateLogisticsWeightG(&value, true); err != nil {
			t.Fatal(err)
		}
	}
}
