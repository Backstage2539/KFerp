package catalog

import "testing"

func TestSearchPinyin(t *testing.T) {
	if got := SearchPinyin(" 咖啡豆 "); got != "kafeidou" {
		t.Fatalf("expected kafeidou, got %q", got)
	}
}

func TestSearchInitials(t *testing.T) {
	if got := SearchInitials("咖啡豆"); got != "kfd" {
		t.Fatalf("expected kfd, got %q", got)
	}
}

func TestSearchPinyinEmpty(t *testing.T) {
	if got := SearchPinyin("   "); got != "" {
		t.Fatalf("expected empty token, got %q", got)
	}
}

func TestNormalizeProductKindTrimsWhitespace(t *testing.T) {
	if got := NormalizeProductKind(" drip_bag "); got != ProductKindDripBag {
		t.Fatalf("NormalizeProductKind() = %q, want %q", got, ProductKindDripBag)
	}
}
