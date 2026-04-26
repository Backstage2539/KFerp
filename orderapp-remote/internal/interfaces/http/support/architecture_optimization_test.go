package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBomAPIRoutesUseApplicationService(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/bom/bom_api.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{
		`bomapp.NewService`,
		`postgresbom.NewRepository`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("bom_api.go missing application boundary %q", want)
		}
	}
	if _, err := os.Stat("internal/infrastructure/postgres/bom/repository.go"); err != nil {
		t.Fatalf("missing BOM postgres adapter under infrastructure: %v", err)
	}
	for _, forbidden := range []string{
		"pool.Query(",
		"pool.QueryRow(",
		"pool.Exec(",
		"listBomItems(",
		"fetchOptions(",
		"saveBagSpecMapping(",
		"deleteBagSpecMapping(",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("bom_api.go still owns persistence concern %q", forbidden)
		}
	}
}

func TestReactBomStaticEntrypointIsRemoved(t *testing.T) {
	if _, err := os.Stat("frontend"); err == nil {
		t.Fatal("legacy React BOM frontend directory should be removed after Vue BOM migration")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}

	checks := []struct {
		path      string
		forbidden []string
	}{
		{path: "internal/interfaces/http/support/static_frontend_routes.go", forbidden: []string{"/bom-react", "bomReact"}},
		{path: "Dockerfile", forbidden: []string{"frontend/dist", "React BOM"}},
		{path: "internal/interfaces/http/support/operation_log.go", forbidden: []string{"/bom-react/assets/"}},
	}
	for _, tc := range checks {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", tc.path, err)
		}
		content := string(body)
		for _, forbidden := range tc.forbidden {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s still contains removed React BOM concern %q", tc.path, forbidden)
			}
		}
	}
}

func TestVueShellUsesSharedAPIClient(t *testing.T) {
	client, err := os.ReadFile("frontend-vue-shell/src/api/client.js")
	if err != nil {
		t.Fatal(err)
	}
	clientSrc := string(client)
	for _, want := range []string{"export async function apiGet", "export async function apiSend", "readJson"} {
		if !strings.Contains(clientSrc, want) {
			t.Fatalf("api client missing %q", want)
		}
	}

	for _, path := range []string{
		"frontend-vue-shell/src/api/production.js",
		"frontend-vue-shell/src/views/BomView.vue",
		"frontend-vue-shell/src/views/CustomersView.vue",
		"frontend-vue-shell/src/views/ProductsView.vue",
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		content := string(body)
		if !strings.Contains(content, "from '../api/client'") && !strings.Contains(content, "from './client'") {
			t.Fatalf("%s should import the shared API client", path)
		}
		for _, forbidden := range []string{"async function fetchJSON", "async function readJson"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s still defines duplicate API helper %q", path, forbidden)
			}
		}
	}

	err = filepath.WalkDir("frontend-vue-shell/src", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path == "frontend-vue-shell/src/api/client.js" {
			return nil
		}
		if filepath.Ext(path) != ".vue" && filepath.Ext(path) != ".js" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(body)
		for _, forbidden := range []string{"async function fetchJSON", "async function readJson"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s still defines duplicate API helper %q", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
