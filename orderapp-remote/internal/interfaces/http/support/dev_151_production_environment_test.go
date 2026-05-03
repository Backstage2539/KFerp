package support

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionEnvironmentRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-151",
		"DEV-151-01",
		"DEV-151-02",
		"UT-151-01",
		"API-151-01",
		"REV-151-01",
		"新增正式环境",
		"未来线上发布默认走正式环境",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("production environment requirement seed missing %q", want)
		}
	}
}

func TestProductionEnvironmentTemplatesExist(t *testing.T) {
	repoRoot := repoRootForProductionDeployTest(t)
	for _, rel := range []string{
		filepath.Join("deploy", "production", "docker-compose.yml"),
		filepath.Join("deploy", "production", "Caddyfile"),
		filepath.Join("deploy", "production", ".env.example"),
	} {
		if _, err := os.Stat(filepath.Join(repoRoot, rel)); err != nil {
			t.Fatalf("production environment template missing %s: %v", rel, err)
		}
	}
}

func TestDeployScriptDefaultsToProductionAndKeepsDevelopmentPlan(t *testing.T) {
	repoRoot := repoRootForProductionDeployTest(t)
	script := filepath.Join(repoRoot, "deploy_orderapp.sh")

	prodPlan := deployPlanOutput(t, repoRoot, script, "--print-plan")
	for _, want := range []string{
		"deploy_env=production",
		"required_branch=main",
		"remote_ref=origin/main",
		"stack_dir=/opt/stacks/erp-production",
		"app_dir=/opt/stacks/erp-production/orderapp",
	} {
		if !strings.Contains(prodPlan, want) {
			t.Fatalf("production deploy plan missing %q\nplan:\n%s", want, prodPlan)
		}
	}

	devPlan := deployPlanOutput(t, repoRoot, script, "--print-plan", "development")
	for _, want := range []string{
		"deploy_env=development",
		"required_branch=develop",
		"remote_ref=origin/develop",
		"stack_dir=/opt/stacks/erp",
		"app_dir=/opt/stacks/erp/orderapp",
	} {
		if !strings.Contains(devPlan, want) {
			t.Fatalf("development deploy plan missing %q\nplan:\n%s", want, devPlan)
		}
	}
}

func TestDeploymentDocsDescribeProductionDefaultAndDevelopmentPreservation(t *testing.T) {
	repoRoot := repoRootForProductionDeployTest(t)
	combined := string(readOrderAppFileForTest(t, filepath.Join("..", "DEPLOYMENT.md"))) +
		"\n" + string(readOrderAppFileForTest(t, filepath.Join("..", "README.md"))) +
		"\n" + string(readOrderAppFileForTest(t, filepath.Join("..", "AGENTS.md")))

	for _, want := range []string{
		"`./deploy_orderapp.sh` 默认发布正式环境",
		"`./deploy_orderapp.sh development` 保留开发环境发布",
		"`/opt/stacks/erp-production`",
		"`/opt/stacks/erp`",
		"production",
		"development",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("deployment docs missing %q under %s", want, repoRoot)
		}
	}
}

func deployPlanOutput(t *testing.T, dir, script string, args ...string) string {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", script, strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func repoRootForProductionDeployTest(t *testing.T) string {
	t.Helper()
	root := filepath.Dir(findAncestorForTest(t, "go.mod"))
	if _, err := os.Stat(filepath.Join(root, "deploy_orderapp.sh")); err != nil {
		t.Skip("repository root deployment files are outside this Go test context")
	}
	return root
}
