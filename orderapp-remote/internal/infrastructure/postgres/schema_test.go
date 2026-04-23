package postgres

import (
	"context"
	"errors"
	"testing"
)

func TestEnsureSchemaRunsStepsInOrder(t *testing.T) {
	var got []string
	err := EnsureSchema(context.Background(), []SchemaStep{
		{Name: "a", Run: func(context.Context) error { got = append(got, "a"); return nil }},
		{Name: "b", Run: func(context.Context) error { got = append(got, "b"); return nil }},
	})
	if err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("steps = %+v", got)
	}
}

func TestEnsureSchemaWrapsStepName(t *testing.T) {
	want := errors.New("boom")
	err := EnsureSchema(context.Background(), []SchemaStep{
		{Name: "ddl", Run: func(context.Context) error { return want }},
	})
	if !errors.Is(err, want) {
		t.Fatalf("EnsureSchema() error = %v, want %v", err, want)
	}
	if err == nil || err.Error() != "ddl: boom" {
		t.Fatalf("EnsureSchema() error text = %v", err)
	}
}
