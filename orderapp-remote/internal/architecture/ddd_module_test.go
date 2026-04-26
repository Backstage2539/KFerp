package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
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
		"internal/interfaces/http/bom",
		"internal/interfaces/http/catalog",
		"internal/interfaces/http/company",
		"internal/interfaces/http/customer",
		"internal/interfaces/http/inventory",
		"internal/interfaces/http/materials",
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

func TestProductionModuleDoesNotOwnInventoryMaterialsOrBOMRoutes(t *testing.T) {
	root := moduleRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "internal", "interfaces", "http", "production", "module.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, forbidden := range []string{
		"registerFinishedInventoryPages",
		"registerMaterialsPages",
		"registerMaterialsAPI",
		"registerBomPages",
		"registerBomAPI",
		"registerAllocationLogPages",
		"ensureFinishedInventoryTable",
		"ensureMaterialTables",
		"ensureBomTables",
		"ensureBagSpecMappingTable",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("production module still owns split responsibility %s", forbidden)
		}
	}
}

func TestRemainingSettingsAndAllocationPagesAreVueOnly(t *testing.T) {
	root := moduleRoot(t)
	for _, file := range []string{
		"internal/interfaces/http/production/allocation_log_page.go",
		"internal/interfaces/http/sales/outsource_templates.go",
	} {
		body, err := os.ReadFile(filepath.Join(root, file))
		if err == nil && strings.Contains(string(body), "c.Render") {
			t.Fatalf("%s still renders a server template", file)
		}
	}
	for _, tmpl := range []string{
		"templates/allocation_logs.html",
		"templates/outsource_settings.html",
	} {
		if _, err := os.Stat(filepath.Join(root, tmpl)); err == nil {
			t.Fatalf("%s should be migrated out of server templates", tmpl)
		}
	}
	app, err := os.ReadFile(filepath.Join(root, "frontend-vue-shell", "src", "App.vue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"allocationLogs", "outsourceSettings"} {
		if !strings.Contains(string(app), required) {
			t.Fatalf("Vue shell missing internal view %s", required)
		}
	}
}

func TestLegacyOrderTemplatesAreRemoved(t *testing.T) {
	root := moduleRoot(t)
	for _, tmpl := range []string{
		"templates/order_edit.html",
		"templates/order_detail.html",
	} {
		if _, err := os.Stat(filepath.Join(root, tmpl)); err == nil {
			t.Fatalf("%s is legacy server-rendered order UI; order pages should stay Vue/Vite only", tmpl)
		}
	}
}

func TestBOMPostgresAdapterLivesInInfrastructure(t *testing.T) {
	root := moduleRoot(t)
	if _, err := os.Stat(filepath.Join(root, "internal", "interfaces", "http", "bom", "bom_application_repository.go")); err == nil {
		t.Fatal("BOM postgres repository adapter still lives in HTTP interface package")
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "infrastructure", "postgres", "bom", "repository.go")); err != nil {
		t.Fatalf("missing BOM postgres adapter under infrastructure/postgres/bom: %v", err)
	}
}

func TestProductionRunningUseCaseLivesInApplication(t *testing.T) {
	root := moduleRoot(t)
	httpPath := filepath.Join(root, "internal", "interfaces", "http", "production", "production_running_repository.go")
	if body, err := os.ReadFile(httpPath); err == nil {
		for _, forbidden := range []string{
			"func startProductionWithInputs",
			"func saveRunningItems",
			"func finishRunningItem",
			"func cancelRunningItem",
		} {
			if strings.Contains(string(body), forbidden) {
				t.Fatalf("%s still owns production running use case %s", httpPath, forbidden)
			}
		}
	}
	assertHTTPPackageDoesNotImplementMethods(t, "internal/interfaces/http/production", map[string]bool{
		"Start":  true,
		"Finish": true,
		"Cancel": true,
	})
	appPath := filepath.Join(root, "internal", "application", "production", "running_service.go")
	if _, err := os.Stat(appPath); err != nil {
		t.Fatalf("missing production running application service: %v", err)
	}
	appBody, err := os.ReadFile(appPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"AllocateStartBatch",
		"SaveRunningItems",
		"SetOrdersProcessStatus",
	} {
		if strings.Contains(string(appBody), forbidden) {
			t.Fatalf("production Start orchestration still calls non-atomic repository step %s", forbidden)
		}
	}
	if !strings.Contains(string(appBody), "repo.Start(") {
		t.Fatal("production Start should delegate atomic persistence to repo.Start")
	}
}

func TestPostgresAdaptersLiveOutsideHTTPInterface(t *testing.T) {
	root := moduleRoot(t)
	for _, rel := range []string{
		"internal/interfaces/http/catalog/catalog_application_repository.go",
		"internal/interfaces/http/company/company_application_repository.go",
		"internal/interfaces/http/customer/customer_application_repository.go",
		"internal/interfaces/http/materials/materials_application_repository.go",
		"internal/interfaces/http/production/production_application_repository.go",
		"internal/interfaces/http/production/production_running_repository.go",
		"internal/interfaces/http/sales/sales_order_repository.go",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Fatalf("postgres adapter still lives in HTTP interface layer: %s", rel)
		}
	}
	for _, rel := range []string{
		"internal/infrastructure/postgres/catalog/repository.go",
		"internal/infrastructure/postgres/company/repository.go",
		"internal/infrastructure/postgres/customer/repository.go",
		"internal/infrastructure/postgres/materials/repository.go",
		"internal/infrastructure/postgres/production/repository.go",
		"internal/infrastructure/postgres/sales/repository.go",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("missing postgres adapter under infrastructure: %s", rel)
		}
	}
}

func TestInfrastructureDoesNotImportHTTPInterface(t *testing.T) {
	root := filepath.Join(moduleRoot(t), "internal", "infrastructure")
	forbidden := "orderapp/internal/interfaces/http/"
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(importPath, forbidden) {
				t.Fatalf("%s imports HTTP interface package %s; infrastructure adapters must not depend on interface layer", path, importPath)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPModulesDoNotImportSiblingHTTPModules(t *testing.T) {
	root := filepath.Join(moduleRoot(t), "internal", "interfaces", "http")
	forbidden := "orderapp/internal/interfaces/http/"
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(importPath, forbidden) && importPath != forbidden+"support" {
				t.Fatalf("%s imports sibling HTTP module %s; move shared behavior behind domain/application/infrastructure boundary", path, importPath)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func assertHTTPPackageDoesNotImplementMethods(t *testing.T, rel string, names map[string]bool) {
	t.Helper()
	root := filepath.Join(moduleRoot(t), rel)
	fset := token.NewFileSet()
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !names[fn.Name.Name] {
				continue
			}
			t.Fatalf("%s implements use-case method %s in HTTP layer", path, fn.Name.Name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
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
