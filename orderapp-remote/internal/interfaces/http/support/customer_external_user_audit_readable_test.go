package support

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecorateCustomerExternalUserAuditRows(t *testing.T) {
	tests := []struct {
		name        string
		action      string
		field       *string
		oldValue    *string
		newValue    *string
		meta        string
		wantFeature string
		wantField   string
		wantOld     string
		wantNew     string
		wantSummary []string
	}{
		{name: "create", action: "create", wantFeature: "创建外部用户", wantSummary: []string{"新增了客户外部用户 外部用户甲"}},
		{name: "reuse existing", action: "create", field: strPtr("name"), oldValue: strPtr("外部用户旧名"), newValue: strPtr("外部用户甲"), meta: `{"customer_id":9,"employee_id":27,"external_user_name":"外部用户甲","previous_external_user_name":"外部用户旧名","binding_status":"active","reused_existing":true,"name_changed":true,"password_reset":true,"login_reenabled":true}`, wantFeature: "复用外部用户", wantField: "名称", wantOld: "外部用户旧名", wantNew: "外部用户甲", wantSummary: []string{"复用了客户外部用户 外部用户甲", "姓名由外部用户旧名改为外部用户甲", "重置了密码", "重新启用了登录"}},
		{name: "create and replace", action: "create", meta: `{"customer_id":9,"employee_id":27,"external_user_name":"外部用户甲","binding_status":"active","reused_existing":false,"replaced_external_users":[{"employee_id":17,"external_user_name":"外部用户旧账号"}]}`, wantFeature: "创建并替换外部用户", wantSummary: []string{"新增了客户外部用户 外部用户甲", "自动停用了原外部用户 外部用户旧账号（员工ID 17）"}},
		{name: "reset password and re-enable", action: "reset_password", field: strPtr("login_enabled"), oldValue: strPtr("false"), newValue: strPtr("true"), wantFeature: "重置外部用户密码", wantField: "登录启用状态", wantOld: "关闭", wantNew: "开启", wantSummary: []string{"并将登录启用状态由关闭改为开启"}},
		{name: "disable login", action: "set_login_enabled", field: strPtr("login_enabled"), oldValue: strPtr("true"), newValue: strPtr("false"), wantFeature: "修改外部用户登录状态", wantField: "登录启用状态", wantOld: "开启", wantNew: "关闭", wantSummary: []string{"开启 -> 关闭"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := tc.meta
			if meta == "" {
				meta = `{"customer_id":9,"employee_id":27,"external_user_name":"外部用户甲","binding_status":"active"}`
			}
			row := AuditLogRow{
				Actor:      "管理员甲",
				EntityType: "customer_external_user",
				EntityID:   int64Ptr(27),
				Action:     tc.action,
				Field:      tc.field,
				OldValue:   tc.oldValue,
				NewValue:   tc.newValue,
				Meta:       strPtr(meta),
			}

			decorateAuditLogRow(&row, nil, nil)

			if row.Menu != "客户管理 / 客户门户配置" || row.Feature != tc.wantFeature {
				t.Fatalf("menu/feature = %q/%q", row.Menu, row.Feature)
			}
			if row.EntityType != "客户外部用户" || row.EntityLabel == nil || *row.EntityLabel != "客户外部用户 外部用户甲" {
				t.Fatalf("entity/label = %q/%v", row.EntityType, row.EntityLabel)
			}
			if tc.wantField != "" && (row.Field == nil || *row.Field != tc.wantField) {
				t.Fatalf("field = %v, want %q", row.Field, tc.wantField)
			}
			if tc.wantOld != "" && (row.OldValue == nil || *row.OldValue != tc.wantOld) {
				t.Fatalf("old = %v, want %q", row.OldValue, tc.wantOld)
			}
			if tc.wantNew != "" && (row.NewValue == nil || *row.NewValue != tc.wantNew) {
				t.Fatalf("new = %v, want %q", row.NewValue, tc.wantNew)
			}
			for _, want := range tc.wantSummary {
				if !strings.Contains(row.Summary, want) {
					t.Fatalf("summary = %q, want %q", row.Summary, want)
				}
			}
		})
	}
}

func TestAuditSearchTermsIncludeCustomerExternalUserChineseAliases(t *testing.T) {
	for query, wantTerm := range map[string]string{
		"客户外部用户":   "customer_external_user",
		"外部用户":     "customer_external_user",
		"客户门户外部用户": "customer_external_user",
		"重置外部用户密码": "reset_password",
		"登录启用状态":   "login_enabled",
	} {
		terms := auditSearchTerms(query)
		if !containsAuditSearchTerm(terms, wantTerm) {
			t.Fatalf("auditSearchTerms(%q) = %#v, want %s", query, terms, wantTerm)
		}
	}
}

func TestAuditViewIncludesCustomerExternalUserTypeFilter(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "AuditView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `<option value="customer_external_user">客户外部用户</option>`) {
		t.Fatal("AuditView must expose the customer external user type filter")
	}
}

func TestFetchAuditPageFindsCustomerExternalUserByTypeAndChineseAlias(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	ctx := context.Background()
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.materials(id BIGINT PRIMARY KEY,code TEXT NOT NULL DEFAULT '',name TEXT NOT NULL DEFAULT '')`, schema))
	mustExecERPLoginGateSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.audit_logs(actor,entity_type,entity_id,action,meta)
		VALUES('管理员甲','customer_external_user',27,'create','{"customer_id":9,"employee_id":27,"external_user_name":"王甲"}'::jsonb)
	`, schema))

	byType, err := fetchAuditPage(ctx, pool, schema, "", "", "", "customer_external_user", 20, 0)
	if err != nil {
		t.Fatalf("fetchAuditPage by type: %v", err)
	}
	if byType.Total != 1 || len(byType.Rows) != 1 {
		t.Fatalf("type filter result = %+v", byType)
	}
	byAlias, err := fetchAuditPage(ctx, pool, schema, "", "", "客户外部用户", "", 20, 0)
	if err != nil {
		t.Fatalf("fetchAuditPage by Chinese alias: %v", err)
	}
	if byAlias.Total != 1 || len(byAlias.Rows) != 1 || byAlias.Rows[0].EntityType != "客户外部用户" {
		t.Fatalf("Chinese alias result = %+v", byAlias)
	}
}
