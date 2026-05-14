package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMiniappEntryModePersistenceEvidenceExists(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository.go")))
	repoTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository_test.go")))
	miniAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"miniappEntryModeForCustomerTx",
		"MiniappEntryMode:  entryMode",
		"COALESCE(NULLIF(miniapp_entry_mode,''),'services')",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("customerportal repository missing miniapp entry mode persistence marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCurrentContextByTokenReturnsCurrentCustomerMiniappEntryMode",
		"TestCreateLoginSessionReturnsCurrentCustomerMiniappEntryMode",
		"MiniappEntryModeMall",
	} {
		if !strings.Contains(repoTest, want) {
			t.Fatalf("customerportal repository test missing miniapp entry mode marker %q", want)
		}
	}
	for _, want := range []string{
		`"miniapp_entry_mode":"mall"`,
		"TestMiniLoginAndMeAPI",
		"TestMiniServicePageAPIRequiresTokenAndReturnsScopedData",
	} {
		if !strings.Contains(miniAPITest, want) {
			t.Fatalf("mini API test missing entry mode marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-183-MINIAPP-ENTRY-MODE-PERSISTENCE",
		"WECHAT_GUI_LOCAL_API_READY",
		"Trust & Run",
		"当前结论：未完成",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing miniapp entry mode marker %q", want)
		}
	}
}

func TestMiniappEntryModePersistenceServicePagesPreserveEntryMode(t *testing.T) {
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service.go")))
	miniAPI := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "api", "customerPortal.ts")))
	miniServicePage := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "pages", "service", "service.vue")))
	miniTest := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "utils", "mall.test.ts")))

	for _, want := range []string{
		"MiniappEntryMode    string                 `json:\"miniapp_entry_mode\"`",
		"page.MiniappEntryMode = NormalizeMiniappEntryMode(current.MiniappEntryMode)",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("customerportal service page missing entry mode marker %q", want)
		}
	}
	for _, want := range []string{
		"miniapp_entry_mode?: MiniappEntryMode | string",
		"miniapp_entry_mode: page.value.miniapp_entry_mode || session.entryMode",
		"preserves mall entry mode when mall customers open service pages",
	} {
		if !strings.Contains(miniAPI+miniServicePage+miniTest, want) {
			t.Fatalf("miniapp source missing service-page entry mode marker %q", want)
		}
	}
}

func TestMiniappEntryModePersistenceRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-183-MINIAPP-ENTRY-MODE-PERSISTENCE",
		"DEV-183-MINIAPP-ENTRY-MODE-PERSISTENCE",
		"UT-183-MINIAPP-ENTRY-MODE-PERSISTENCE",
		"API-183-MINIAPP-ENTRY-MODE-PERSISTENCE",
		"REV-183-MINIAPP-ENTRY-MODE-PERSISTENCE",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestMiniappEntryModePersistenceManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"miniapp_entry_mode",
			"mall",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing miniapp entry mode marker %q", path, want)
			}
		}
	}
}
