package main

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestBomReactIndexDisablesCache(t *testing.T) {
	b, err := os.ReadFile("static_frontend_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	required := []string{
		`e.GET("/bom-react", func(c echo.Context) error {`,
		`e.GET("/bom-react/*", func(c echo.Context) error {`,
		`if c.QueryParam("rev") != currentBomReactRev() {`,
		`return c.Redirect(http.StatusFound, bomReactRedirectURL(c.QueryParams()))`,
		`Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")`,
		`Header().Set("Pragma", "no-cache")`,
		`Header().Set("Expires", "0")`,
	}
	for _, want := range required {
		if !strings.Contains(src, want) {
			t.Fatalf("static frontend routes missing %q", want)
		}
	}
}

func TestBomReactRedirectURLPreservesEmbedMode(t *testing.T) {
	params := url.Values{}
	params.Set("embed", "1")
	got := bomReactRedirectURL(params)
	if !strings.HasPrefix(got, "/bom-react?") {
		t.Fatalf("bomReactRedirectURL() = %q", got)
	}
	if !strings.Contains(got, "embed=1") {
		t.Fatalf("bomReactRedirectURL() = %q, missing embed=1", got)
	}
	if !strings.Contains(got, "rev=") {
		t.Fatalf("bomReactRedirectURL() = %q, missing rev", got)
	}
}
