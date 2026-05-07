package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalWechatConnectRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-WECHAT-CONNECT",
		"DEV-CUSTOMER-PORTAL-WECHAT-CONNECT-01",
		"DEV-CUSTOMER-PORTAL-WECHAT-CONNECT-02",
		"UT-CUSTOMER-PORTAL-WECHAT-CONNECT-01",
		"API-CUSTOMER-PORTAL-WECHAT-CONNECT-01",
		"REV-CUSTOMER-PORTAL-WECHAT-CONNECT-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal wechat connect seed missing %q", want)
		}
	}
}
