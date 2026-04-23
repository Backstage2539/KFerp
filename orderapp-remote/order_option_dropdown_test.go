package main

import (
	"os"
	"strings"
	"testing"
)

func TestOrderOptionDropdownsShowAllAndStripQuotes(t *testing.T) {
	b, err := os.ReadFile("templates/order.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)

	required := []string{
		`onclick="comboShowAny(this,'customers')"`,
		`onclick="comboShowAny(this,'sources')"`,
		`onclick="comboShowAny(this,'orderTypes')"`,
		`onclick="comboShowAny(this,'payStatuses')"`,
		`onclick="comboShowAny(this,'shipStatuses')"`,
		"comboRenderAny(dropdown, '', key);",
		"const name = cleanPicked(o.name);",
		"return it ? cleanPicked(it.name) : '';",
		"name:{{jsstr .Name}}",
	}
	for _, want := range required {
		if !strings.Contains(html, want) {
			t.Fatalf("order dropdown template missing %q", want)
		}
	}
	if strings.Contains(html, `name:{{printf "%q"`) {
		t.Fatalf("option dropdowns must not double-quote template strings")
	}
}
