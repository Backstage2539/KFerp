package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev369UnitTemplateSaveCreateUpdateRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-369-UNIT-TEMPLATE-SAVE-CREATE-UPDATE",
		"DEV-369-UNIT-TEMPLATE-SAVE-CREATE-UPDATE",
		"UT-369-UNIT-TEMPLATE-SAVE-CREATE-UPDATE",
		"API-369-UNIT-TEMPLATE-SAVE-CREATE-UPDATE",
		"REV-369-UNIT-TEMPLATE-SAVE-CREATE-UPDATE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("unit template save create/update seed missing %q", want)
		}
	}
}

func TestDev369UnitTemplateSaveCreateUpdateUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "UISettingsView.vue")))
	unitPaneStart := strings.Index(src, `class="panel unit-template-panel unit-template-pane"`)
	productConfigPaneStart := strings.Index(src, `class="panel product-config-panel product-config-template-pane"`)
	if unitPaneStart < 0 || productConfigPaneStart < 0 || productConfigPaneStart <= unitPaneStart {
		t.Fatal("ProductSettingsView.vue should keep unit template pane before product config pane")
	}
	unitPane := src[unitPaneStart:productConfigPaneStart]
	for _, blocked := range []string{
		">新建模板<",
	} {
		if strings.Contains(unitPane, blocked) {
			t.Fatalf("unit template pane should not keep new-template marker %q", blocked)
		}
	}
	for _, want := range []string{
		"function resetProductUnitTemplateForm()",
		"await apiSend(url, { method, body: payload })",
		"await loadAll()",
		"resetProductUnitTemplateForm()",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing save create/update marker %q", want)
		}
	}
	if !strings.Contains(src, "await loadAll()\n    resetProductUnitTemplateForm()") {
		t.Fatal("saving a unit template should refresh the list before returning to create mode")
	}
	if strings.Contains(settings, ">新建单位<") {
		t.Fatal("global unit dictionary should not keep a separate new-unit button")
	}
}

func TestDev369UnitTemplateSaveCreateUpdateDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-369-UNIT-TEMPLATE-SAVE-CREATE-UPDATE",
			"单位模板页不再显示“新建模板”按钮",
			"点击保存完成新增或更新",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-369-UNIT-TEMPLATE-SAVE-CREATE-UPDATE",
			"页面不显示“新建模板”按钮",
			"再次填写下一套模板并保存就是新增",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-369-UNIT-TEMPLATE-SAVE-CREATE-UPDATE",
			"销售规格模板页直接在空白表单填写后保存",
			"保存后刷新列表并回到空白编辑状态",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-unit-template-edit-retains-saved.md"): {
			"PR-369",
			"点击保存完成新增或更新",
			"再次填写下一套模板并保存就是新增",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing unit template edit-retain doc marker %q", rel, want)
			}
		}
	}
}
