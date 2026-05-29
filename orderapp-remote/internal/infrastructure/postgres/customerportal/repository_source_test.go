package customerportal

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerPortalRepositoryRequiresEnabledProfileForBindingsAndCapabilities(t *testing.T) {
	body, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true",
		"WHERE customer_id=$1 AND enabled=true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("portal repository missing enabled-profile guard %q", want)
		}
	}
}
