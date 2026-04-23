package main

import (
	"os"
	"strings"
	"testing"
)

func TestOrderProductDropdownLayering(t *testing.T) {
	b, err := os.ReadFile("templates/order.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)

	required := []string{
		".items tbody tr:focus-within{z-index:1000;}",
		".items td.p:focus-within{z-index:1001;}",
		".items .combo:focus-within{z-index:1002;}",
		".items .dropdown{z-index:9999 !important;}",
	}
	for _, want := range required {
		if !strings.Contains(html, want) {
			t.Fatalf("order page dropdown layering missing %q", want)
		}
	}
}
