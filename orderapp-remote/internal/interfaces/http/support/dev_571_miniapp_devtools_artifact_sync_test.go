package support

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev571MiniappArtifactValidationAndDevToolsRefreshContract(t *testing.T) {
	orderappRoot := findAncestorForTest(t, "go.mod")
	workspaceRoot := filepath.Dir(orderappRoot)
	deployScript := filepath.Join(workspaceRoot, "deploy_orderapp.sh")
	if _, err := os.Stat(deployScript); os.IsNotExist(err) {
		t.Skip("complete workspace release contracts run before the Docker safety gate")
	}

	validator := filepath.Join(workspaceRoot, "scripts", "verify_mp_weixin_artifact.mjs")
	manifestVerifier := filepath.Join(workspaceRoot, "scripts", "verify_mp_weixin_manifest.sh")
	swapScript := filepath.Join(workspaceRoot, "scripts", "swap_miniapp_export.sh")
	for _, helper := range []string{validator, manifestVerifier, swapScript} {
		if _, err := os.Stat(helper); err != nil {
			t.Fatalf("required miniapp release helper is missing: %s: %v", helper, err)
		}
	}

	t.Run("declared pages require complete generated files", func(t *testing.T) {
		artifact := filepath.Join(t.TempDir(), "mp-weixin")
		writeDev570MiniappArtifact(t, artifact, []string{
			"pages/index/index",
			"pages/employee-customers/employee-customers",
		})

		manifest := filepath.Join(artifact, "PAGE_FILE_MANIFEST")
		cmd := exec.Command("node", validator, artifact, manifest)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("complete artifact rejected: %v\n%s", err, output)
		}
		cmd = exec.Command("bash", manifestVerifier, artifact)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("downloaded artifact manifest rejected: %v\n%s", err, output)
		}

		missing := filepath.Join(artifact, "pages", "employee-customers", "employee-customers.js")
		if err := os.Remove(missing); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("bash", manifestVerifier, artifact)
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("downloaded artifact with a declared page JavaScript file missing was accepted")
		}
		if !strings.Contains(string(output), "pages/employee-customers/employee-customers.js") {
			t.Fatalf("manifest verifier did not identify the missing page JavaScript file:\n%s", output)
		}
		cmd = exec.Command("node", validator, artifact)
		if output, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(output), "pages/employee-customers/employee-customers.js") {
			t.Fatalf("build validator did not reject the missing page JavaScript file: err=%v\n%s", err, output)
		}
	})

	t.Run("subpackage pages are included", func(t *testing.T) {
		artifact := filepath.Join(t.TempDir(), "mp-weixin")
		writeDev570MiniappArtifact(t, artifact, []string{"pages/index/index"})
		appJSON := `{"pages":["pages/index/index"],"subPackages":[{"root":"package-admin","pages":["customers/index","release..candidate/index"]}]}`
		if err := os.WriteFile(filepath.Join(artifact, "app.json"), []byte(appJSON), 0o644); err != nil {
			t.Fatal(err)
		}
		writeDev570MiniappPageFiles(t, artifact, "package-admin/customers/index")
		writeDev570MiniappPageFiles(t, artifact, "package-admin/release..candidate/index")
		manifest := filepath.Join(artifact, "PAGE_FILE_MANIFEST")
		cmd := exec.Command("node", validator, artifact, manifest)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("complete subpackage artifact rejected: %v\n%s", err, output)
		}
		cmd = exec.Command("bash", manifestVerifier, artifact)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("complete subpackage manifest rejected: %v\n%s", err, output)
		}
	})

	t.Run("failed fixed-directory swap restores the previous package", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "KFerp-miniapp-mp-weixin")
		incoming := filepath.Join(root, "incoming")
		backup := filepath.Join(root, "backup")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(incoming, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "old.txt"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(incoming, "new.txt"), []byte("new"), 0o644); err != nil {
			t.Fatal(err)
		}

		fakeBin := filepath.Join(root, "bin")
		if err := os.MkdirAll(fakeBin, 0o755); err != nil {
			t.Fatal(err)
		}
		counter := filepath.Join(root, "mv-count")
		fakeMV := `#!/bin/sh
count=0
if [ -f "$KFERP_TEST_MV_COUNT" ]; then count=$(cat "$KFERP_TEST_MV_COUNT"); fi
count=$((count + 1))
printf '%s' "$count" > "$KFERP_TEST_MV_COUNT"
if [ "$count" -eq 2 ]; then exit 99; fi
exec /bin/mv "$@"
`
		if err := os.WriteFile(filepath.Join(fakeBin, "mv"), []byte(fakeMV), 0o755); err != nil {
			t.Fatal(err)
		}
		env := append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "KFERP_TEST_MV_COUNT="+counter)
		cmd := exec.Command("bash", swapScript, target, incoming, backup)
		cmd.Env = env
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("fault-injected swap unexpectedly succeeded:\n%s", output)
		}
		for _, restored := range []string{
			filepath.Join(target, "old.txt"),
			filepath.Join(incoming, "new.txt"),
		} {
			if _, err := os.Stat(restored); err != nil {
				t.Fatalf("failed swap did not preserve %s: %v", restored, err)
			}
		}
		if _, err := os.Stat(backup); !os.IsNotExist(err) {
			t.Fatalf("failed swap left the restored package at the backup path: %v", err)
		}

		cmd = exec.Command("bash", swapScript, target, incoming, backup)
		cmd.Env = env
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("healthy swap failed: %v\n%s", err, output)
		}
		for _, expected := range []string{
			filepath.Join(target, "new.txt"),
			filepath.Join(backup, "old.txt"),
		} {
			if _, err := os.Stat(expected); err != nil {
				t.Fatalf("healthy swap missing %s: %v", expected, err)
			}
		}
	})

	t.Run("signal after moving the previous package restores it", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "KFerp-miniapp-mp-weixin")
		incoming := filepath.Join(root, "incoming")
		backup := filepath.Join(root, "backup")
		for _, dir := range []string{target, incoming, filepath.Join(root, "bin")} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(target, "old.txt"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(incoming, "new.txt"), []byte("new"), 0o644); err != nil {
			t.Fatal(err)
		}

		counter := filepath.Join(root, "mv-count")
		fakeMV := `#!/bin/sh
count=0
if [ -f "$KFERP_TEST_MV_COUNT" ]; then count=$(cat "$KFERP_TEST_MV_COUNT"); fi
count=$((count + 1))
printf '%s' "$count" > "$KFERP_TEST_MV_COUNT"
if [ "$count" -eq 1 ]; then
  /bin/mv "$@"
  kill -TERM "$PPID"
  sleep 1
  exit 0
fi
exec /bin/mv "$@"
`
		fakeBin := filepath.Join(root, "bin")
		if err := os.WriteFile(filepath.Join(fakeBin, "mv"), []byte(fakeMV), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", swapScript, target, incoming, backup)
		cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "KFERP_TEST_MV_COUNT="+counter)
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("signal-injected swap unexpectedly succeeded:\n%s", output)
		}
		for _, restored := range []string{
			filepath.Join(target, "old.txt"),
			filepath.Join(incoming, "new.txt"),
		} {
			if _, err := os.Stat(restored); err != nil {
				t.Fatalf("signal recovery did not preserve %s: %v", restored, err)
			}
		}
		if _, err := os.Stat(backup); !os.IsNotExist(err) {
			t.Fatalf("signal recovery left the restored package at the backup path: %v", err)
		}
	})

	deployBytes, err := os.ReadFile(deployScript)
	if err != nil {
		t.Fatal(err)
	}
	remoteBytes, err := os.ReadFile(filepath.Join(workspaceRoot, "scripts", "remote_orderapp_release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(remoteBytes), "verify_mp_weixin_artifact.mjs") ||
		!strings.Contains(string(remoteBytes), "PAGE_FILE_MANIFEST") {
		t.Fatal("remote build does not validate declared pages and emit the download manifest")
	}
	for _, marker := range []string{
		"verify_mp_weixin_manifest.sh",
		"swap_miniapp_export.sh",
		"PAGE_FILE_MANIFEST",
		"close and re-import",
		"clearing compile cache alone",
	} {
		if !strings.Contains(string(deployBytes), marker) {
			t.Fatalf("local deploy is missing the atomic-sync or DevTools refresh marker %q", marker)
		}
	}
	syncStart := strings.Index(string(deployBytes), "sync_miniapp_artifact()")
	if syncStart < 0 {
		t.Fatal("could not locate local miniapp sync function")
	}
	syncEnd := strings.Index(string(deployBytes)[syncStart:], "\n}\n\nif [ \"$PREFLIGHT\"")
	if syncEnd < 0 {
		t.Fatal("could not locate local miniapp sync function end")
	}
	localSync := string(deployBytes)[syncStart : syncStart+syncEnd]
	if strings.Contains(localSync, "node ") {
		t.Fatal("downloaded artifact verification must not introduce a local Node dependency after remote deployment")
	}

	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "customer-portal-miniapp-test.md"),
		filepath.Join("docs", "acceptance", "2026-08-01-miniapp-devtools-preview-sync.md"),
	} {
		contents := string(readOrderAppFileForTest(t, rel))
		for _, marker := range []string{
			"PR-571-MINIAPP-DEVTOOLS-PREVIEW-SYNC",
			"DEV-571-ARTIFACT-CLOSURE",
			"DEV-571-DEVTOOLS-REFRESH",
		} {
			if !strings.Contains(contents, marker) {
				t.Fatalf("%s missing %s", rel, marker)
			}
		}
	}
}

func writeDev570MiniappArtifact(t *testing.T, root string, pages []string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	appJSON := `{"pages":["` + strings.Join(pages, `","`) + `"]}`
	if err := os.WriteFile(filepath.Join(root, "app.json"), []byte(appJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, page := range pages {
		base := filepath.Join(root, filepath.FromSlash(page))
		if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, ext := range []string{".js", ".json", ".wxml", ".wxss"} {
			if err := os.WriteFile(base+ext, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func writeDev570MiniappPageFiles(t *testing.T, root, page string) {
	t.Helper()
	base := filepath.Join(root, filepath.FromSlash(page))
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".js", ".json", ".wxml", ".wxss"} {
		if err := os.WriteFile(base+ext, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
