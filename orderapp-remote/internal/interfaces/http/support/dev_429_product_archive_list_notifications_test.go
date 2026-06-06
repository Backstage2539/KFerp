package support

import (
	"os"
	"strings"
	"testing"
)

func TestProductArchiveListNoticeCleanupRequirementSeedsExist(t *testing.T) {
	body, err := os.ReadFile(supportFilePath(t, "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	src := string(body)
	for _, want := range []string{
		"PR-429-PRODUCT-ARCHIVE-LIST-NOTICE-CLEANUP",
		"DEV-429-PRODUCT-ARCHIVE-LIST-NO-HARDCODED-FIELDS",
		"DEV-429-NOTIFICATION-DISMISS-PERSISTENCE",
		"UT-429-PRODUCT-ARCHIVE-LIST-NOTICE-CLEANUP",
		"API-429-PRODUCT-ARCHIVE-LIST-NOTICE-CLEANUP",
		"REV-429-PRODUCT-ARCHIVE-LIST-NOTICE-CLEANUP",
		"商品档案列表删除 BOM 使用列和挂耳包装字段",
		"关闭新订单通知后等待轮询不再弹回",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product archive list notice cleanup requirement seed missing %q", want)
		}
	}
}

func TestProductArchiveListNoticeCleanupWiringAndManuals(t *testing.T) {
	checks := []struct {
		path    string
		want    []string
		notWant []string
	}{
		{
			path: "orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue",
			want: []string{
				`class="text-button sku-name-button"`,
				`openProductProductionConfig(row)`,
				`被哪些 BOM 使用`,
			},
			notWant: []string{
				`<th>BOM 使用</th>`,
				`查看使用关系`,
				`bom-source-cell`,
				`v-model.number="row.drip_bag_grams"`,
				`v-model.number="row.drip_box_bag_count"`,
				`挂耳每袋克重必须大于 0`,
				`挂耳每盒袋数必须大于 0`,
			},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/lib/product-settings.js",
			want: []string{
				"buildProductBasicsPayload",
				"buildCustomProductCreatePayload",
			},
			notWant: []string{
				"payload.drip_bag_grams",
				"payload.drip_box_bag_count",
				"form.drip_bag_grams || 10",
				"row.drip_box_bag_count || 10",
			},
		},
		{
			path: "orderapp-remote/frontend-vue-shell/src/App.vue",
			want: []string{
				"dismissedNotificationIDs",
				"rememberDismissedNotification(item)",
				"filterDismissedNotifications(notifications.value, dismissedNotificationIDs.value)",
				"markNotificationRead(item.id)",
			},
		},
		{
			path: "orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md",
			want: []string{
				"PR-429-PRODUCT-ARCHIVE-LIST-NOTICE-CLEANUP",
				"不单独展示 `BOM 使用` 列",
				"挂耳每袋克重或每盒袋数",
			},
		},
		{
			path: "orderapp-remote/docs/OP_MANUAL_NOTIFICATIONS.md",
			want: []string{
				"PR-429-PRODUCT-ARCHIVE-LIST-NOTICE-CLEANUP",
				"当前浏览器记录已关闭通知 ID",
			},
		},
	}

	for _, check := range checks {
		body, err := os.ReadFile(repoFilePath(t, check.path))
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		src := string(body)
		for _, want := range check.want {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", check.path, want)
			}
		}
		for _, notWant := range check.notWant {
			if strings.Contains(src, notWant) {
				t.Fatalf("%s should not contain %q", check.path, notWant)
			}
		}
	}
}
