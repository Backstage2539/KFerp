package bom

import "testing"

func TestPR607ProductionManifestLocksApprovedProductPackagingScope(t *testing.T) {
	manifest, err := LoadPR607ProductionManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ManifestID != "pr607-roasted-bean-product-packaging-2026-08-25" {
		t.Fatalf("manifest id=%q", manifest.ManifestID)
	}
	if manifest.SemiFinishedGroupID != 165 || manifest.ProductGroupID != 191 || manifest.ProductSingleItemID != 1098 || manifest.ProductBlendItemID != 1097 {
		t.Fatalf("group mapping=%+v", manifest)
	}
	if manifest.SpecTemplateID != 1 || manifest.SpecTemplateVersionID != 1 || manifest.SpecTemplateVersionNo != "V001" || manifest.SpecTemplateFingerprint != "ad34976e9aa043a3f4da8fdfc0ae153a" {
		t.Fatalf("template lock=%+v", manifest)
	}
	published, drafts, reactivated, renamed := 0, 0, 0, 0
	byName := map[string]PR607ProductPackagingEntry{}
	for _, entry := range manifest.Entries {
		byName[entry.Name] = entry
		if entry.Publish {
			published++
		} else {
			drafts++
		}
		if !entry.ExpectedProductActive {
			reactivated++
		}
		if entry.ExpectedProductName != entry.Name {
			renamed++
		}
	}
	if len(manifest.Entries) != 31 || published != 26 || drafts != 5 || reactivated != 11 || renamed != 3 {
		t.Fatalf("counts=%d/%d/%d/%d/%d want 31/26/5/11/3", len(manifest.Entries), published, drafts, reactivated, renamed)
	}
	for _, name := range []string{"墨昙", "果皮茶", "晨曦·娜依", "巴布亚新几内亚", "萨琪姆水洗"} {
		if byName[name].Publish {
			t.Fatalf("%s must remain draft", name)
		}
	}
	if got := byName["云上莓梦"]; got.TargetProductID != 68 || got.ExpectedProductName != "云山莓梦" {
		t.Fatalf("云上莓梦=%+v", got)
	}
	if got := byName["曜石2.0"]; got.TargetProductID != 58 || got.ExpectedProductName != "曜石" {
		t.Fatalf("曜石2.0=%+v", got)
	}
	if got := byName["白巧坚果"]; got.TargetProductID != 345 || got.ExpectedProductName != "熟豆-白巧坚果拼配" || got.ExpectedProductActive {
		t.Fatalf("白巧坚果=%+v", got)
	}
	if got := byName["榛巧拼配"]; got.SourceProductID != 456 || got.TargetProductID != 62 || !got.ExpectedProductActive || got.ExpectedDefaultBomID != 138 || got.ExpectedDefaultVersionID != 223 || got.ExpectedDefaultBomStatus != "inactive" {
		t.Fatalf("榛巧拼配=%+v", got)
	}
	for _, entry := range manifest.Entries {
		want := manifest.ProductSingleItemID
		if entry.SourceGroupItemID == 890 || entry.SourceGroupItemID == 891 {
			want = manifest.ProductBlendItemID
		}
		if entry.TargetGroupItemID != want {
			t.Fatalf("%s category=%d want %d", entry.Name, entry.TargetGroupItemID, want)
		}
	}
}

func TestPR607ProductionManifestLocksSevenPackagingVariants(t *testing.T) {
	manifest, err := LoadPR607ProductionManifest()
	if err != nil {
		t.Fatal(err)
	}
	want := []PR607PackagingSpec{
		{SpecKey: "spec-1", MainQtyKG: .021, PackagingMaterialID: 101},
		{SpecKey: "spec-2", MainQtyKG: .039, PackagingMaterialID: 101},
		{SpecKey: "spec-3", MainQtyKG: .083, PackagingMaterialID: 102},
		{SpecKey: "spec-4", MainQtyKG: .116, PackagingMaterialID: 102},
		{SpecKey: "spec-5", MainQtyKG: .230, PackagingMaterialID: 103},
		{SpecKey: "spec-6", MainQtyKG: .454, PackagingMaterialID: 100},
		{SpecKey: "spec-7", MainQtyKG: 2.5, PackagingMaterialID: 105},
	}
	if len(manifest.PackagingSpecs) != len(want) {
		t.Fatalf("specs=%d", len(manifest.PackagingSpecs))
	}
	for i, got := range manifest.PackagingSpecs {
		if got.SpecKey != want[i].SpecKey || got.MainQtyKG != want[i].MainQtyKG || got.PackagingMaterialID != want[i].PackagingMaterialID {
			t.Fatalf("spec[%d]=%+v want %+v", i, got, want[i])
		}
	}
}
