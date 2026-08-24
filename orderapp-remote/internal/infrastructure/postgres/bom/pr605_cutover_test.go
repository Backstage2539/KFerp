package bom

import (
	"math"
	"testing"
)

func TestPR605ProductionManifestLocksCountsAndSpecialSources(t *testing.T) {
	manifest, err := LoadPR605ProductionManifest()
	if err != nil {
		t.Fatal(err)
	}
	published, drafts, newMaterials := 0, 0, 0
	byName := map[string]PR605CutoverManifestEntry{}
	for _, entry := range manifest.Entries {
		byName[entry.SourceName] = entry
		if entry.Publish {
			published++
		} else {
			drafts++
		}
		if entry.ExistingMaterialID == 0 {
			newMaterials++
		}
	}
	if len(manifest.Entries) != 31 || published != 26 || drafts != 5 || newMaterials != 30 {
		t.Fatalf("manifest counts=%d/%d/%d/%d want 31/26/5/30", len(manifest.Entries), published, drafts, newMaterials)
	}
	sources, targets := pr605ComponentMaterialReplacementIDs(manifest)
	if len(sources) != 2 || sources[0] != 42 || targets[0] != 7 || sources[1] != 67 || targets[1] != 27 {
		t.Fatalf("component replacements=%v -> %v", sources, targets)
	}
	if byName["初晓"].ExistingMaterialID != 106 || byName["初晓"].RecipeOverride != "initial-screenshot" {
		t.Fatalf("初晓 manifest=%+v", byName["初晓"])
	}
	if byName["曜石2.0"].SourceVersionID != 254 || byName["曜石2.0"].SourceVersionNo != "V006" {
		t.Fatalf("曜石2.0 manifest=%+v", byName["曜石2.0"])
	}
	for _, name := range []string{"墨昙", "果皮茶", "晨曦·娜依", "巴布亚新几内亚", "萨琪姆水洗"} {
		if byName[name].Publish {
			t.Fatalf("%s must remain draft-only", name)
		}
	}
	if _, exists := byName["曜石"]; exists {
		t.Fatal("inactive old 曜石 must be excluded")
	}
}

func TestPR605PoundFixedRecipeScalesToOneKilogram(t *testing.T) {
	scale := pr605FixedScale(1, "磅")
	want := []float64{249.122356, 249.122356, 749.571691}
	for index, grams := range []float64{113, 113, 340} {
		got := grams * scale
		if math.Abs(got-want[index]) > 0.000001 {
			t.Fatalf("item %d scaled grams=%.9f want %.9f", index, got, want[index])
		}
	}
}

func TestPR605InitialRecipeMatchesApprovedScreenshot(t *testing.T) {
	got := pr605InitialRecipe()
	wantIDs := []int64{56, 85, 10, 47}
	wantRatios := []float64{50, 15, 20, 15}
	if len(got) != 4 {
		t.Fatalf("recipe rows=%d want 4", len(got))
	}
	for index, item := range got {
		if item.id != wantIDs[index] || item.ratio != wantRatios[index] {
			t.Fatalf("recipe[%d]=%+v want material=%d ratio=%v", index, item, wantIDs[index], wantRatios[index])
		}
	}
	loss := 0.195
	wantAdjusted := []float64{62.11, 18.63, 24.84, 18.63}
	for index, item := range got {
		adjusted := math.Round(item.ratio/(1-loss)*100) / 100
		if adjusted != wantAdjusted[index] {
			t.Fatalf("adjusted[%d]=%.2f want %.2f", index, adjusted, wantAdjusted[index])
		}
	}
}
