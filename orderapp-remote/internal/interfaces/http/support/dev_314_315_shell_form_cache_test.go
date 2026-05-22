package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readDev314Text(t *testing.T, parts ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDev314ShellMenuViewportScrollAndSwipe(t *testing.T) {
	app := readDev314Text(t, "frontend-vue-shell", "src", "App.vue")
	for _, want := range []string{
		`ref="content"`,
		"scrollCurrentViewToTop",
		"content.value.scrollTo",
		"handleTouchStart",
		"handleTouchEnd",
		"touchStartX",
		"mobileSwipeMinDistance",
		"window.addEventListener('touchstart', handleTouchStart",
		"window.addEventListener('touchend', handleTouchEnd",
		"height: 100vh",
		"overflow-y: auto",
		"overscroll-behavior: contain",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("App.vue missing shell menu marker %q", want)
		}
	}

	requirements := readDev314Text(t, "docs", "REQUIREMENTS.md") + "\n" + readDev314Text(t, "docs", "ACCEPTANCE_TESTS.md") + "\n" + readDev314Text(t, "docs", "OP_MANUAL_WORKSPACE_MODE.md")
	for _, want := range []string{
		"PR-314-SHELL-MENU-SCROLL-TOUCH",
		"左侧菜单固定在单屏视口内独立滚动",
		"点击任何功能菜单后功能页面回到顶部",
		"手机端右滑呼出功能菜单，左滑隐藏功能菜单",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("shell menu docs missing %q", want)
		}
	}
}

func TestDev315FormDraftCache(t *testing.T) {
	cache := readDev314Text(t, "frontend-vue-shell", "src", "lib", "form-draft-cache.js")
	for _, want := range []string{
		"const drafts = new Map()",
		"FORM_DRAFT_SCOPES",
		"saveFormDraft",
		"readFormDraft",
		"clearFormDraft",
	} {
		if !strings.Contains(cache, want) {
			t.Fatalf("form draft cache missing %q", want)
		}
	}
	if strings.Contains(cache, "localStorage") || strings.Contains(cache, "sessionStorage") {
		t.Fatalf("form draft cache must stay in memory and clear on browser refresh")
	}

	for _, item := range []struct {
		path    []string
		markers []string
	}{
		{
			path: []string{"frontend-vue-shell", "src", "views", "OrderEntryView.vue"},
			markers: []string{
				"ORDER_ENTRY_DRAFT_SCOPE",
				"saveOrderEntryDraft",
				"restoreOrderEntryDraft",
				"onBeforeUnmount(saveOrderEntryDraft)",
				"clearFormDraft(orderEntryDraftKey())",
			},
		},
		{
			path: []string{"frontend-vue-shell", "src", "views", "BomView.vue"},
			markers: []string{
				"BOM_FORM_DRAFT_SCOPE",
				"saveBomFormDraft",
				"restoreBomFormDraft",
				"onBeforeUnmount(saveBomFormDraft)",
			},
		},
		{
			path: []string{"frontend-vue-shell", "src", "views", "ProductSettingsView.vue"},
			markers: []string{
				"SKU_SETTINGS_FORM_DRAFT_SCOPE",
				"saveProductSettingsDraft",
				"restoreProductSettingsDraft",
				"restoringProductSettingsDraft",
				"onBeforeUnmount(saveProductSettingsDraft)",
			},
		},
	} {
		source := readDev314Text(t, item.path...)
		for _, want := range item.markers {
			if !strings.Contains(source, want) {
				t.Fatalf("%s missing %q", filepath.Join(item.path...), want)
			}
		}
	}

	requirements := readDev314Text(t, "docs", "REQUIREMENTS.md") + "\n" + readDev314Text(t, "docs", "ACCEPTANCE_TESTS.md") + "\n" + readDev314Text(t, "docs", "OP_MANUAL_WORKSPACE_MODE.md") + "\n" + readDev314Text(t, "docs", "OP_MANUAL_ORDER_SALES.md") + "\n" + readDev314Text(t, "docs", "OP_MANUAL_INVENTORY_MATERIALS.md")
	for _, want := range []string{
		"PR-315-FORM-DRAFT-CACHE",
		"录单、BOM配方维护、SKU设置",
		"跳转到其他功能再回来时保留未提交表单",
		"刷新浏览器后草稿清空",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("form draft cache docs missing %q", want)
		}
	}
}
