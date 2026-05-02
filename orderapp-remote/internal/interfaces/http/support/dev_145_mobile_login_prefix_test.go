package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMobileLoginPrefixRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-145",
		"DEV-145-01",
		"DEV-145-02",
		"DEV-145-03",
		"UT-145-01",
		"API-145-01",
		"REV-145-01",
		"手机从 /app 登录",
		"只需一次外层 order 认证和一次系统账号登录",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("mobile login prefix requirement seed missing %q", want)
		}
	}
}

func TestMobileLoginPrefixSourceGuards(t *testing.T) {
	login := string(readOrderAppFileForTest(t, "templates/login.html"))
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	client := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "api", "client.js")))
	redirects := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "vue_shell_redirect.go")))

	for _, want := range []string{
		"function appPath(path)",
		"appPath('/api/auth/login')",
		"appPath('/vue-shell?fresh_login=1')",
		"appURL('/login')",
		"export function appURL(url)",
		"appURL(`${url.pathname}${url.search}${url.hash}`)",
		"func PrefixRelativeLocation",
		"strings.Repeat(\"../\", climb)",
	} {
		if !strings.Contains(login+"\n"+app+"\n"+client+"\n"+redirects, want) {
			t.Fatalf("mobile login prefix source guard missing %q", want)
		}
	}
}
