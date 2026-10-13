package pageentry

import "testing"

func TestDocumentValidation(t *testing.T) {
	cases := []struct {
		name    string
		d       Document
		publish bool
		ok      bool
	}{
		{"new draft", Document{Name: "菜单", Kind: "price", Visibility: "authenticated"}, false, true},
		{"price needs version", Document{Name: "菜单", Kind: "price", Visibility: "authenticated"}, true, false},
		{"article", Document{Name: "介绍", Kind: "article", Visibility: "public", Blocks: []Block{{Kind: "heading", Text: "咖啡"}, {Kind: "text", Text: "欢迎"}}}, true, true},
		{"script block", Document{Name: "介绍", Kind: "article", Visibility: "public", Blocks: []Block{{Kind: "html", Text: "<script>"}}}, true, false},
		{"function allowlist", Document{Name: "订单", Kind: "function", Visibility: "authenticated", Target: "orders"}, true, true},
		{"unsafe route", Document{Name: "订单", Kind: "function", Visibility: "public", Target: "https://example.com"}, true, false},
		{"public orders", Document{Name: "订单", Kind: "function", Visibility: "public", Target: "orders"}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.d, tc.publish); (err == nil) != tc.ok {
				t.Fatalf("validation=%v", err)
			}
		})
	}
}

func TestImmutablePathsAndAssetMembership(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	if MiniPath(key) != "pages/page-entry/page-entry?entry="+key {
		t.Fatal(MiniPath(key))
	}
	if !ValidKey(key) || ValidKey("../admin") {
		t.Fatal("key validation")
	}
	d := Document{Blocks: []Block{{Kind: "image", AssetID: key}}}
	if !d.HasAsset(key) || d.HasAsset("draft-secret") {
		t.Fatal("asset membership")
	}
	if FunctionPath("orders") != "pages/service/service?key=orders&source=official" || FunctionPath("employeeOrders") != "" {
		t.Fatal("function routes")
	}
}
