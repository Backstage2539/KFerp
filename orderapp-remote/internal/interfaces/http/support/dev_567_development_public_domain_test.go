package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev567DevelopmentPublicDomainContract(t *testing.T) {
	orderappRoot := findAncestorForTest(t, "go.mod")
	for name, rel := range map[string]string{
		"requirement store": filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"requirements":      filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":        filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"evidence":          filepath.Join("docs", "acceptance", "2026-08-01-development-public-domain.md"),
	} {
		contents := string(readOrderAppFileForTest(t, rel))
		for _, marker := range []string{
			"PR-567-DEVELOPMENT-PUBLIC-DOMAIN",
			"DEV-567-DEVELOPMENT-URL",
			"DEV-567-PUBLIC-INGRESS",
			"DEV-567-DEPLOYMENT-GUARD",
		} {
			if !strings.Contains(contents, marker) {
				t.Fatalf("%s missing %s", name, marker)
			}
		}
	}

	workspaceRoot := filepath.Dir(orderappRoot)
	if _, err := os.Stat(filepath.Join(workspaceRoot, "deploy_orderapp.sh")); os.IsNotExist(err) {
		t.Skip("root deployment files are checked by the complete workspace test gate")
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
	publicCaddy := read("scripts", "Caddyfile.public")
	ingressScript := read("scripts", "configure_public_ingress.sh")

	for name, contents := range map[string]string{
		"deploy script":  deployScript,
		"remote release": remoteRelease,
		"public Caddy":   publicCaddy,
		"ingress script": ingressScript,
	} {
		if strings.Contains(contents, "dev.erp.qacoohee.com") {
			t.Fatalf("%s still contains the retired development domain", name)
		}
	}

	for _, marker := range []string{
		`API_BASE="https://dev.qacoohee.com/app"`,
		`PUBLIC_URL="${DEVELOPMENT_PUBLIC_URL:-https://dev.qacoohee.com/app/}"`,
		`--resolve "dev.qacoohee.com:443:`,
	} {
		if !strings.Contains(deployScript, marker) {
			t.Fatalf("deploy script missing %q", marker)
		}
	}
	if strings.Contains(deployScript, "curl_args+=(\n      -k") {
		t.Fatal("development external smoke must validate the public certificate")
	}

	for _, marker := range []string{
		"development:https://dev.qacoohee.com/app",
		"production:https://erp.qacoohee.com/app",
		"--resolve dev.qacoohee.com:443:127.0.0.1",
		`"$SOURCE_ROOT/scripts/configure_public_ingress.sh"`,
	} {
		if !strings.Contains(remoteRelease, marker) {
			t.Fatalf("remote release missing %q", marker)
		}
	}
	if strings.Contains(remoteRelease, "curl -ksS") {
		t.Fatal("remote public readiness must validate the certificate")
	}

	for _, marker := range []string{
		"erp.qacoohee.com:443",
		"reverse_proxy erp_prod_orderapp:8080",
		"dev.qacoohee.com:443",
		"reverse_proxy erp_orderapp:8080",
	} {
		if !strings.Contains(publicCaddy, marker) {
			t.Fatalf("public Caddy config missing %q", marker)
		}
	}
	if strings.Contains(publicCaddy, "tls internal") {
		t.Fatal("the DNS-published development domain must use a public certificate")
	}

	for _, marker := range []string{
		"caddy validate",
		"caddy reload",
		"backup.domain-",
		"erp_prod_caddy",
	} {
		if !strings.Contains(ingressScript, marker) {
			t.Fatalf("ingress installer missing %q", marker)
		}
	}
}
