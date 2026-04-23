package main

import (
	"os"
	"strings"
	"testing"
)

func TestOrderProductSelectionUsesMouseDown(t *testing.T) {
	b, err := os.ReadFile("templates/order.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)

	required := []string{
		"function selectDropdownOption(opt){",
		"document.addEventListener('mousedown', (ev) => {",
		"ev.preventDefault();",
		"selectDropdownOption(opt);",
		"setProductForRow(tr, opt.dataset.id",
	}
	for _, want := range required {
		if !strings.Contains(html, want) {
			t.Fatalf("order product selection handler missing %q", want)
		}
	}
}
