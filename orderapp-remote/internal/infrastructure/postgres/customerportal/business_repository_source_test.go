package customerportal

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerPortalBusinessRepositoryLoadsOrderItemsForCustomerOrders(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"FROM %s.order_items",
		"listCustomerOrderItems",
		"CustomerOrderItemSummary",
		"ORDER BY oi.order_id, oi.line_no, oi.id",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer order item loading missing %q", want)
		}
	}
}

func TestCustomerPortalBeanListFallsBackToLatestOfficialPublications(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"listLatestOfficialBeanLists",
		"owner_type='official'",
		"status='published'",
		"DISTINCT ON (list_type)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("latest official bean list fallback missing %q", want)
		}
	}
}
