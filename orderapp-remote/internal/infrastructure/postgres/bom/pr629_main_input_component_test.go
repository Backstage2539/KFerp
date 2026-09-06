package bom

import (
	"os"
	"strings"
	"testing"
)

func TestProductionBomMainInputComponentUsesTypedProvenanceAndPublishedProductSpec(t *testing.T) {
	for name, path := range map[string]string{
		"schema":     "schema.go",
		"repository": "repository.go",
		"spec_group": "spec_group_repository.go",
	} {
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		src := string(bytes)
		for _, marker := range []string{"main_input_component_type", "main_input_product_id", "main_input_bom_spec_id"} {
			if !strings.Contains(src, marker) {
				t.Fatalf("%s missing typed main-input provenance marker %q", name, marker)
			}
		}
	}
	bytes, err := os.ReadFile("spec_group_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	specGroup := string(bytes)
	if !strings.Contains(specGroup, "status='published'") || !strings.Contains(specGroup, "component BOM specification") {
		t.Fatal("product main-input component must validate a published BOM specification")
	}
	repositoryBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := string(repositoryBytes)
	start := strings.Index(repository, "func (r Repository) UpdateProductionBomDraftWorkspace")
	end := strings.Index(repository, "func (r Repository) CreateProductionBomReplacementDraft")
	if start < 0 || end <= start || !strings.Contains(repository[start:end], "copySpecTemplateToProductionBomWithComponentTx") {
		t.Fatal("draft workspace saves must preserve typed main-input components")
	}
}
