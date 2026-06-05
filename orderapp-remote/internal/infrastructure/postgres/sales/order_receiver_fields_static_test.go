package sales

import (
	"os"
	"strings"
	"testing"
)

func TestSaveOrderReceiverFieldsUseNonNullText(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	source := string(data)
	start := strings.Index(source, "INSERT INTO %s.orders(")
	if start < 0 {
		t.Fatal("cannot locate order insert SQL")
	}
	end := strings.Index(source[start:], ").Scan(&orderID)")
	if end < 0 {
		t.Fatal("cannot locate order insert arguments")
	}
	section := source[start : start+end]
	for _, field := range []string{"ReceiverName", "ReceiverPhone", "ReceiverAddress", "ReceiverCompany"} {
		if strings.Contains(section, "nullText(cmd."+field+")") {
			t.Fatalf("%s must not be inserted with nullText; receiver columns are NOT NULL DEFAULT ''", field)
		}
		if !strings.Contains(section, "notNullText(cmd."+field+")") {
			t.Fatalf("%s insert argument must use notNullText", field)
		}
	}
}
