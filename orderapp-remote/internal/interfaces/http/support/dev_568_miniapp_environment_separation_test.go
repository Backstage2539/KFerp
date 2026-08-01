package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev568MiniappEnvironmentSeparationContract(t *testing.T) {
	for name, rel := range map[string]string{
		"requirement store": filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"requirements":      filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":        filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"manual":            filepath.Join("docs", "OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md"),
		"test guide":        filepath.Join("docs", "customer-portal-miniapp-test.md"),
		"evidence":          filepath.Join("docs", "acceptance", "2026-08-01-miniapp-environment-separation.md"),
	} {
		contents := string(readOrderAppFileForTest(t, rel))
		for _, marker := range []string{
			"PR-568-MINIAPP-ENVIRONMENT-SEPARATION",
			"DEV-568-CLIENT-ENVIRONMENT-GUARD",
			"DEV-568-STORAGE-BOUNDARY",
			"DEV-568-FIXED-ARTIFACTS",
		} {
			if !strings.Contains(contents, marker) {
				t.Fatalf("%s missing %s", name, marker)
			}
		}
	}

	orderappRoot := findAncestorForTest(t, "go.mod")
	workspaceRoot := filepath.Dir(orderappRoot)
	if _, err := os.Stat(filepath.Join(workspaceRoot, "deploy_orderapp.sh")); os.IsNotExist(err) {
		t.Skip("complete workspace release contracts run before the Docker safety gate")
	}

	read := func(rel ...string) string {
		contents, err := os.ReadFile(filepath.Join(append([]string{workspaceRoot}, rel...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(contents)
	}

	deployScript := read("deploy_orderapp.sh")
	remoteRelease := read("scripts", "remote_orderapp_release.sh")
	manifest := read("miniapp", "src", "manifest.json")
	packageJSON := read("miniapp", "package.json")

	for _, marker := range []string{
		"sync_miniapp_artifact()",
		"KFerp-miniapp-mp-weixin-dev",
		"KFerp-miniapp-mp-weixin",
		`grep -Fxq "environment=$TARGET_ENV"`,
		`grep -Fxq "api_base=$API_BASE"`,
	} {
		if !strings.Contains(deployScript, marker) {
			t.Fatalf("deploy script missing %q", marker)
		}
	}

	for _, marker := range []string{
		`VITE_KFERP_ENVIRONMENT="$TARGET_ENV"`,
		"UNEXPECTED_API_BASE",
		"environment=$TARGET_ENV",
		"api_base=$API_BASE",
	} {
		if !strings.Contains(remoteRelease, marker) {
			t.Fatalf("remote release missing %q", marker)
		}
	}

	if !strings.Contains(manifest, `"urlCheck": true`) {
		t.Fatal("miniapp manifest must enable legal-domain checking")
	}
	if !strings.Contains(packageJSON, "check:environment") {
		t.Fatal("miniapp build must validate its explicit environment")
	}
}
