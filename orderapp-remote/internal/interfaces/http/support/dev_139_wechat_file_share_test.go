package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWechatFileShareRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-139",
		"DEV-139-01",
		"DEV-139-02",
		"UT-139-01",
		"API-139-01",
		"REV-139-01",
		"直接分享文件",
		"不发送链接",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("wechat file share requirement seed missing %q", want)
		}
	}
}

func TestWechatShareHelperSharesFilesNotLinks(t *testing.T) {
	helper := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "external-share.js")))
	for _, want := range []string{
		"resource.file_url",
		"new FileCtor",
		"files: [file]",
		"nav.canShare",
		"file-shared",
		"unsupported",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("wechat share helper missing file-share marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"url:",
		"clipboard.writeText",
		"resource.share_url || resource.file_url",
	} {
		if strings.Contains(helper, forbidden) {
			t.Fatalf("wechat share helper must not share/copy a link marker %q", forbidden)
		}
	}
}

func TestSalesAndDeliveryViewsTellUserToShareFiles(t *testing.T) {
	salesView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	deliveryView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "DeliveryNoteView.vue")))
	for _, want := range []string{
		"已打开系统分享面板，请选择微信发送文件",
		"当前浏览器不支持直接分享文件，请下载最新版后手动发送到微信",
	} {
		if !strings.Contains(salesView+"\n"+deliveryView, want) {
			t.Fatalf("sales/delivery share UI missing file-share copy %q", want)
		}
	}
	for _, forbidden := range []string{
		"微信分享链接已复制",
		"复制链接后发给客户",
		"分享链接已生成",
	} {
		if strings.Contains(salesView+"\n"+deliveryView, forbidden) {
			t.Fatalf("sales/delivery share UI still contains link-share copy %q", forbidden)
		}
	}
}
