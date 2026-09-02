package catalog

import (
	"os"
	"strings"
	"testing"
)

func TestCatalogSchemaDoesNotReseedRetiredProductUnitTemplates(t *testing.T) {
	source, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, "VALUES ('默认kg单位','kg','kg','kg'") {
		t.Fatal("PR-622 cleanup must remain stable across restart; catalog schema cannot recreate a retired product unit template")
	}
}
