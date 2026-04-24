package main

import (
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
		`return c.Redirect(http.StatusFound, bomReactURL())`,
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
