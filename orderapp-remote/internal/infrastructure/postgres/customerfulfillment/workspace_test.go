package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestWorkspacePricePreviewScopeLazyAndLiveCapabilities(t *testing.T) {
	pool, schema := newCustomerFulfillmentTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	var id int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("INSERT INTO %s.customers(name,active,customer_type) VALUES('测试履约客户',true,'wholesale') RETURNING id", schema)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.customer_capability_templates(template_key,label,erp_permissions,erp_view_keys,capabilities_json,active)VALUES('efs_test','测试','["customer_processing.read","customer_processing.submit"]','["customerProcessingPortal"]','[{"code":"bean_list","enabled":true},{"code":"direct_ship","enabled":true}]',true);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf("INSERT INTO %s.customer_portal_profiles(customer_id,enabled,capability_template_key)VALUES($1,true,'efs_test')", schema), id)
	if err != nil {
		t.Fatal(err)
	}
	var publication int64
	err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,publication_purpose,content_json)VALUES('commercial','V1','published','customer',$1,'factory_supply','{"price_rows":[{"product_name":"测试商品","spec_label":"227g","min_qty":1,"max_qty":null,"final_unit_price":30,"price_unit":"袋","cost_source_snapshot":{"private":"must not expose"}}]}')RETURNING id`, schema), fmt.Sprint(id)).Scan(&publication)
	if err != nil {
		t.Fatal(err)
	}
	list, err := repo.CustomerWorkspace(ctx, id, "bean_list", 0)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(list)
	if strings.Contains(string(body), "测试商品") || strings.Contains(string(body), "cost_source_snapshot") {
		t.Fatal("list eagerly loaded preview content")
	}
	preview, err := repo.CustomerWorkspace(ctx, id, "bean_list", publication)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = json.Marshal(preview)
	if !strings.Contains(string(body), "测试商品") || strings.Contains(string(body), "cost_source_snapshot") || strings.Contains(string(body), "must not expose") {
		t.Fatal("preview did not project public price fields")
	}
	if _, err = repo.CustomerWorkspace(ctx, id, "bean_list", publication+999); err == nil {
		t.Fatal("cross customer / missing publication accepted")
	}
	if _, err = repo.CustomerWorkspace(ctx, id, "processing", 0); err == nil {
		t.Fatal("disabled capability accepted")
	}
	_, err = pool.Exec(ctx, fmt.Sprintf("UPDATE %s.customer_capability_templates SET capabilities_json='[{\"code\":\"direct_ship\",\"enabled\":true}]' WHERE template_key='efs_test'", schema))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CustomerWorkspace(ctx, id, "bean_list", 0); err == nil {
		t.Fatal("capability removal did not apply live")
	}
}

func TestWorkspacePricePreviewSupportsLegacyGroupedPublications(t *testing.T) {
	rows, err := customerPricePreviewRows([]byte(`{"groups":[{"items":[{"name":"旧版商品","commercial_wholesale_tiers":[{"spec_g":454,"min_qty":2,"max_qty":13,"price_per_unit":65,"display_unit":"lb"}]}]}]}`))
	if err != nil || len(rows) != 1 || rows[0]["product_name"] != "旧版商品" || rows[0]["unit_price"] != float64(65) {
		t.Fatal("legacy price preview missing", rows, err)
	}
}
