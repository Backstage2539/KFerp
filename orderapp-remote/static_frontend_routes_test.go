package main

import (
	"os"
	"strings"
	"testing"
)

func TestStaticFrontendRoutesOnlyServeVueShell(t *testing.T) {
	b, err := os.ReadFile("static_frontend_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	required := []string{
		`e.Static("/vue-shell/assets", "frontend-vue-shell/dist/assets")`,
		`e.GET("/vue-shell", func(c echo.Context) error {`,
		`e.GET("/vue-shell/*", func(c echo.Context) error {`,
		`target := "/vue-shell?view=producePlan"`,
	}
	for _, want := range required {
		if !strings.Contains(src, want) {
			t.Fatalf("static frontend routes missing %q", want)
		}
	}
	for _, bad := range []string{"/bom-react", "bomReact"} {
		if strings.Contains(src, bad) {
			t.Fatalf("static frontend routes still contain %q", bad)
		}
	}
}
