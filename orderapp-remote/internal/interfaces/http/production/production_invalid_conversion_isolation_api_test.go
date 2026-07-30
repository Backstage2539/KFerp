package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProducePlanSummaryKeepsValidDemandWhenAnotherOrderHasInvalidInventoryConversion(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.order_process_statuses(name,sort,active)
		VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;

		INSERT INTO %[1]s.products(
			id,name,active,parent_product_id,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES
			(644,'如目达摩',true,0,'',0,'','{"inventory_unit":"kg"}'::jsonb),
			(789,'如目达摩 454g',true,644,'454g',454,'g','{}'::jsonb),
			(433,'启用但缺换算的盒装商品',true,0,'',0,'','{"inventory_unit":"盒"}'::jsonb),
			(435,'快照单位优先的缺换算商品',true,0,'',0,'','{}'::jsonb),
			(434,'已停用 Codex 测试商品',false,0,'',0,'','{"inventory_unit":"盒"}'::jsonb);

		INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id)
		VALUES
			(55401,'SO-PR554-VALID','2026-07-26',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1)),
			(55402,'SO-PR554-BLOCKED','2026-07-26',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1)),
			(55404,'SO-PR554-SNAPSHOT-UNIT','2026-07-26',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1)),
			(55403,'CDS-20260526-1186','2026-07-26',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1));

		INSERT INTO %[1]s.order_items(
			order_id,line_no,item_name,qty,unit,sales_unit,spec,product_id,unit_price,line_total,price_source_json
		) VALUES
			(55401,1,'如目达摩',4,'454g','454g','454g',789,0,0,
			 '{"production_quantity_snapshot":{"sku_id":789,"parent_product_id":644,"spec_label":"454g","sales_unit":"454g","inventory_unit":"kg","inventory_qty_per_sales_unit":0.454,"conversion_source":"published"}}'::jsonb),
			(55402,1,'启用但缺换算的盒装商品',1,'件','','100g',433,0,0,'{}'::jsonb),
			(55404,1,'快照单位优先的缺换算商品',1,'袋','袋','袋',435,0,0,
			 '{"inventory_unit":"kg","effective_sales_spec":{"sales_unit":"袋","inventory_unit":"kg"}}'::jsonb),
			(55403,1,'已停用 Codex 测试商品',1,'件','','100g',434,0,0,'{}'::jsonb);
	`, schema))

	app := newProductionFlowTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?from=2026-07-26&to=2026-07-26", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET production demand status=%d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var summary struct {
		Rows []struct {
			ProductID        int64  `json:"product_id"`
			OrderNos         string `json:"order_nos"`
			NeedUnits        int64  `json:"need_units"`
			DemandSelectable bool   `json:"demand_selectable"`
			BlockingReason   string `json:"blocking_reason"`
			SalesUnit        string `json:"sales_unit"`
			InventoryUnit    string `json:"inventory_unit"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode production demand: %v\n%s", err, rec.Body.String())
	}
	if len(summary.Rows) != 3 {
		t.Fatalf("rows=%d, want valid and two blocked demands: %s", len(summary.Rows), rec.Body.String())
	}
	var validSeen, blockedSeen, snapshotUnitSeen bool
	for _, row := range summary.Rows {
		switch row.ProductID {
		case 789:
			validSeen = row.DemandSelectable && row.BlockingReason == ""
		case 433:
			blockedSeen = !row.DemandSelectable &&
				row.NeedUnits == 1 &&
				strings.Contains(row.OrderNos, "SO-PR554-BLOCKED") &&
				strings.Contains(row.BlockingReason, "销售单位“件”无法换算到库存单位“盒”")
		case 435:
			snapshotUnitSeen = !row.DemandSelectable &&
				row.SalesUnit == "袋" &&
				row.InventoryUnit == "kg" &&
				strings.Contains(row.BlockingReason, "销售单位“袋”无法换算到库存单位“kg”") &&
				!strings.Contains(row.BlockingReason, "库存单位“(未设置)”")
		case 434:
			t.Fatalf("inactive product demand must be omitted: %+v", row)
		}
	}
	if !validSeen || !blockedSeen || !snapshotUnitSeen {
		t.Fatalf("validSeen=%v blockedSeen=%v snapshotUnitSeen=%v rows=%+v", validSeen, blockedSeen, snapshotUnitSeen, summary.Rows)
	}

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/production-plans",
		strings.NewReader(`{"from":"2026-07-26","to":"2026-07-26","source_type":"erp_order","selected":["433-0"]}`),
	)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	app.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusBadRequest ||
		!strings.Contains(createRec.Body.String(), "销售单位“件”无法换算到库存单位“盒”") {
		t.Fatalf("blocked demand create status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "1=1", 0)
}
