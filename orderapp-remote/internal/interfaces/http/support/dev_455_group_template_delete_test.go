package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev455GroupTemplateDeleteContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-455-GROUP-TEMPLATE-DELETE",
			"DEV-455-GROUP-TEMPLATE-DELETE-UI",
			"DEV-455-GROUP-TEMPLATE-DELETE-API",
			"DEV-455-GROUP-TEMPLATE-DELETE-DOCS",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "UISettingsView.vue"): {
			"删除模板",
			"deleteGroupTemplate",
			"`/api/business-groups/${id}`",
			"method: 'DELETE'",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "business_group_routes.go"): {
			`e.DELETE("/api/business-groups/:id", h.deleteBusinessGroupAPI)`,
			"deleteBusinessGroupAPI",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"type DeleteBusinessGroupCommand struct",
			"func (s *Service) DeleteBusinessGroup",
			"DeleteBusinessGroup(ctx context.Context, cmd DeleteBusinessGroupCommand) error",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"func (r Repository) DeleteBusinessGroup",
			"DELETE FROM %s.business_group_assignments WHERE group_id=$1",
			"DELETE FROM %s.business_group_usages WHERE group_id=$1",
			"DELETE FROM %s.business_group_items WHERE group_id=$1",
			"DELETE FROM %s.business_groups WHERE id=$1",
			`"delete_business_group"`,
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-455-GROUP-TEMPLATE-DELETE",
			"删除模板",
			"不提供启用/停用",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-455-GROUP-TEMPLATE-DELETE",
			"删除模板",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-455-GROUP-TEMPLATE-DELETE",
			"删除模板",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-group-template-delete.md"): {
			"PR-455",
			"分组模板删除",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-455 marker %q", rel, want)
			}
		}
	}
}

func TestDev455GroupTemplateUIHasNoTemplateActivation(t *testing.T) {
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "UISettingsView.vue")))
	panelStart := strings.Index(settings, `data-section-mode="groupTemplates"`)
	if panelStart < 0 {
		t.Fatalf("group template panel markers missing")
	}
	nextPanel := strings.Index(settings[panelStart:], "\n    <section class=\"panel\">")
	if nextPanel <= 0 {
		t.Fatalf("group template panel end marker missing")
	}
	panel := settings[panelStart : panelStart+nextPanel]
	templateFormEnd := strings.Index(panel, `<div v-if="selectedGroupTemplate" class="category-editor">`)
	if templateFormEnd <= 0 {
		t.Fatalf("group template form end marker missing")
	}
	templateForm := panel[:templateFormEnd]
	for _, forbidden := range []string{
		"groupTemplateForm.active",
		"template.active === false ? '停用' : '启用'",
		"停用模板",
		">启用</span>",
	} {
		if strings.Contains(templateForm, forbidden) {
			t.Fatalf("group template panel must use delete instead of template activation, found %q", forbidden)
		}
	}
}
