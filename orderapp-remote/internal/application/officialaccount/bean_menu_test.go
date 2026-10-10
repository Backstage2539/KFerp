package officialaccount

import "testing"

func TestMiniNavigationMenu(t *testing.T) {
	for _, path := range []string{"pages/bean-list-center/bean-list-center", "pages/page-entry/page-entry?entry=0123456789abcdef0123456789abcdef", "pages/service/service?key=orders&source=official", "pages/service/service?key=productOrder"} {
		err := ValidateMenu(Menu{Buttons: []Button{{Name: "入口", Type: "miniprogram", AppID: "mini", PagePath: path, URL: "https://example.test"}}}, "mini")
		if err != nil {
			t.Fatal(path, err)
		}
	}
}
