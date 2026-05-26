package support

import (
	"os"
	"strings"
	"testing"
)

func TestProductSettingsBasicProductRowsExposeBomEntry(t *testing.T) {
	b, err := os.ReadFile("frontend-vue-shell/src/views/ProductSettingsView.vue")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	rowStart := strings.Index(src, `<template v-for="row in skuRenderRows"`)
	if rowStart < 0 {
		t.Fatalf("product SKU list table rows not found")
	}
	rowEnd := strings.Index(src[rowStart:], `<tr v-if="!skuRenderRows.length"`)
	if rowEnd < 0 {
		t.Fatalf("product SKU list empty row not found")
	}
	rowBlock := src[rowStart : rowStart+rowEnd]
	if !strings.Contains(src, "<th>BOM</th>") {
		t.Fatalf("product basics table must include BOM column")
	}
	for _, want := range []string{
		`@click="openProductBom(row)"`,
		"维护 BOM",
	} {
		if !strings.Contains(rowBlock, want) {
			t.Fatalf("product basics row must expose BOM maintenance entry, missing %q", want)
		}
	}

	reqs := string(readDev153File(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-153",
		"DEV-153-01",
		"UT-153-01",
		"API-153-01",
		"REV-153-01",
		"普通产品和新建产品也能在产品设置直接维护 BOM",
	} {
		if !strings.Contains(reqs, want) {
			t.Fatalf("requirement seed missing %q", want)
		}
	}
}

func readDev153File(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
