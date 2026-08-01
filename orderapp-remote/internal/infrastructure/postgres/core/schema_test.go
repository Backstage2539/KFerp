package core

import (
	"os"
	"strings"
	"testing"
)

func TestEnsureSchemaSynchronizesProductIDSequence(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"syncSerialIDSequence(ctx, pool, schema, \"products\")",
		"pg_get_serial_sequence",
		"SELECT MAX(id)",
		"PERFORM setval",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("core schema must repair product IDs after explicit-ID imports; missing %q", want)
		}
	}
}
