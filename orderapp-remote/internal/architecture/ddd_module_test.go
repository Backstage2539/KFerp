package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppmainIsCompositionRootOnly(t *testing.T) {
	root := moduleRoot(t)
	allowed := map[string]bool{
		"app.go":           true,
		"app_bootstrap.go": true,
		"app_routes.go":    true,
		"schema_setup.go":  true,
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal", "appmain"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if !allowed[name] {
			t.Fatalf("internal/appmain/%s is business or adapter code; move it into a DDD module", name)
		}
	}
}

func TestDDDModuleDirectoriesExist(t *testing.T) {
	root := moduleRoot(t)
	for _, dir := range []string{
		"internal/interfaces/http/company",
		"internal/interfaces/http/customer",
		"internal/interfaces/http/production",
		"internal/interfaces/http/sales",
		"internal/interfaces/http/support",
		"internal/application/company",
		"internal/application/customer",
		"internal/application/production",
		"internal/application/sales",
		"internal/domain/production",
		"internal/domain/sales",
	} {
		if info, err := os.Stat(filepath.Join(root, dir)); err != nil || !info.IsDir() {
			t.Fatalf("missing DDD module directory %s", dir)
		}
	}
}

func TestDomainLayerHasNoTransportOrPersistenceImports(t *testing.T) {
	assertNoImports(t, "internal/domain", []string{
		"github.com/labstack/echo",
		"github.com/jackc/pgx",
		"net/http",
		"database/sql",
	})
}

func TestApplicationLayerHasNoHTTPFrameworkImports(t *testing.T) {
	assertNoImports(t, "internal/application", []string{
		"github.com/labstack/echo",
		"net/http",
	})
}

func assertNoImports(t *testing.T, rel string, forbidden []string) {
	t.Helper()
	root := moduleRoot(t)
	fset := token.NewFileSet()
	err := filepath.WalkDir(filepath.Join(root, rel), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			value := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if value == bad || strings.HasPrefix(value, bad+"/") {
					t.Fatalf("%s imports forbidden dependency %s", path, value)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDDDDevelopmentRequirementSeedsExist(t *testing.T) {
	root := moduleRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, code := range []string{
		"DEV-DDD-001",
		"DEV-DDD-002",
		"DEV-DDD-003",
		"DEV-DDD-004",
		"UT-DDD-001",
		"API-DDD-001",
		"REV-DDD-001",
	} {
		if !strings.Contains(src, code) {
			t.Fatalf("requirement seed missing %s", code)
		}
	}
}

var _ ast.File
