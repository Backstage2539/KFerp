package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type ProduceRunRow struct {
	ID            int64
	BatchID       string
	Product       string
	ProductID     int64
	SpecG         int64
	NeedG         int64
	InputG        int64
	BomYieldRate  float64
	PlanUnits     int64
	PlanLooseG    int64
	OrderNos      string
	StartedBy     string
	StartedAt     string
	StartedAtTime time.Time
}

type ProduceRunPageData struct {
	Rows  []ProduceRunRow
	Ok    bool
	Error string
}

type ProduceStartAPIRequest struct {
	From       string           `json:"from"`
	To         string           `json:"to"`
	CustomerID int64            `json:"customer_id"`
	Selected   []string         `json:"selected"`
	InputByKey map[string]int64 `json:"input_by_key"`
}

type ProduceStartAPIResponse struct {
	OK      bool   `json:"ok"`
	BatchID string `json:"batch_id"`
}

func ensureProductionRunTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.produce_running_items (
		id BIGSERIAL PRIMARY KEY,
		batch_id TEXT NOT NULL,
		product_id BIGINT NOT NULL,
		product_name TEXT NOT NULL DEFAULT '',
		spec_g BIGINT NOT NULL,
		need_g BIGINT NOT NULL,
		order_nos TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'running',
		started_by TEXT NOT NULL DEFAULT '',
		started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		finished_by TEXT,
		finished_at TIMESTAMPTZ
	);
	CREATE INDEX IF NOT EXISTS produce_running_items_status_idx ON %s.produce_running_items(status, started_at DESC);`, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS input_g BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS bom_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS planned_units BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS planned_loose_g BIGINT NOT NULL DEFAULT 0`, schema))
	return nil
}

func normalizeYieldRate(rate float64) float64 {
	if rate <= 0 || rate > 1 {
		return 0.8
	}
	return rate
}

func defaultProductionInputG(needG int64, yieldRate float64) int64 {
	if needG <= 0 {
		return 0
	}
	return int64(math.Ceil(float64(needG) / normalizeYieldRate(yieldRate)))
}

func finishedTotalG(specG, units, looseG int64) int64 {
	if specG <= 0 || units < 0 || looseG < 0 {
		return 0
	}
	return units*specG + looseG
}

func actualYieldRate(specG, units, looseG, inputG int64) (float64, error) {
	if inputG <= 0 {
		return 0, fmt.Errorf("input_g must be greater than 0")
	}
	total := finishedTotalG(specG, units, looseG)
	rate := float64(total) / float64(inputG)
	return math.Round(rate*10000) / 10000, nil
}

func plannedFinishedInventoryByInput(specG, inputG int64, yieldRate float64) InvQty {
	if specG <= 0 || inputG <= 0 {
		return InvQty{}
	}
	totalG := int64(math.Floor(float64(inputG) * normalizeYieldRate(yieldRate)))
	return plannedFinishedInventoryAddition(specG, totalG)
}

func runningInventoryPlan(specG, needG, inputG int64, yieldRate float64) InvQty {
	plan := plannedFinishedInventoryByInput(specG, inputG, yieldRate)
	if plan.Units > 0 || plan.LooseG > 0 {
		return plan
	}
	return plannedFinishedInventoryAddition(specG, needG)
}

func registerProductionFlowPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.POST("/produce/start", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		selected, sel := parseSelectedCSV(strings.TrimSpace(c.FormValue("selected")))
		from := strings.TrimSpace(c.FormValue("from"))
		to := strings.TrimSpace(c.FormValue("to"))
		var cid int64
		if v := strings.TrimSpace(c.FormValue("customer_id")); v != "" {
			cid, _ = strconv.ParseInt(v, 10, 64)
		}
		if sel == "" {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape("请先生成计划并选择项目"))
		}
		inputByKey := map[string]int64{}
		rows, err := fetchUnproducedNeeds(c.Request().Context(), pool, schema, from, to, cid)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		for _, r := range rows {
			if !selected[producePlanKey(r.ProductID, r.SpecG)] || r.GapG <= 0 {
				continue
			}
			formKey := fmt.Sprintf("%d_%d", r.ProductID, r.SpecG)
			inputG, _, err := parseOptionalInt64(c.FormValue("input_g_" + formKey))
			if err != nil {
				return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape("投料数格式不正确"))
			}
			if inputG <= 0 {
				return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape("投料数必须大于0"))
			}
			inputByKey[producePlanKey(r.ProductID, r.SpecG)] = inputG
		}
		operator := actorOf(c)
		if _, err := startProductionWithInputs(c.Request().Context(), pool, schema, from, to, cid, selected, inputByKey, operator); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/produce/running?ok=1")
	})

	e.POST("/api/produce/start", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req ProduceStartAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if len(req.Selected) == 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请先生成计划并选择项目"})
		}
		selected := map[string]bool{}
		for _, key := range req.Selected {
			key = strings.TrimSpace(key)
			if key != "" {
				selected[key] = true
			}
		}
		batchID, err := startProductionWithInputs(c.Request().Context(), pool, schema, req.From, req.To, req.CustomerID, selected, req.InputByKey, actorOf(c))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, ProduceStartAPIResponse{OK: true, BatchID: batchID})
	})

	e.GET("/produce/running", func(c echo.Context) error {
		data := ProduceRunPageData{
			Ok:    strings.TrimSpace(c.QueryParam("ok")) == "1",
			Error: strings.TrimSpace(c.QueryParam("err")),
		}
		rows, err := listRunningItems(c.Request().Context(), pool, schema)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Rows = rows
		}
		return c.Render(http.StatusOK, "produce_running.html", data)
	})

	e.POST("/produce/running/finish", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape(err.Error()))
		}
		id, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("id")), 10, 64)
		if id <= 0 {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape("invalid id"))
		}
		finishedUnits, hasFinishedUnits, err := parseOptionalInt64(c.FormValue("finished_units"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape("完成件数格式不正确"))
		}
		finishedLooseG, hasFinishedLooseG, err := parseOptionalInt64(c.FormValue("finished_loose_g"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape("散装余量格式不正确"))
		}
		if err := finishRunningItem(c.Request().Context(), pool, schema, id, finishedUnits, finishedLooseG, hasFinishedUnits || hasFinishedLooseG, actorOf(c)); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/produce/running?ok=1")
	})

	e.POST("/produce/running/cancel", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape(err.Error()))
		}
		id, _ := strconv.ParseInt(strings.TrimSpace(c.FormValue("id")), 10, 64)
		if id <= 0 {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape("invalid id"))
		}
		if err := cancelRunningItem(c.Request().Context(), pool, schema, id, actorOf(c)); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/produce/running?ok=1")
	})
}

func parseSelectedCSV(sel string) (map[string]bool, string) {
	selected := map[string]bool{}
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return selected, ""
	}
	for _, key := range strings.Split(sel, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			selected[key] = true
		}
	}
	return selected, sel
}

func startProductionWithInputs(ctx context.Context, pool *pgxpool.Pool, schema, from, to string, customerID int64, selected map[string]bool, inputByKey map[string]int64, operator string) (string, error) {
	rows, err := fetchUnproducedNeeds(ctx, pool, schema, from, to, customerID)
	if err != nil {
		return "", err
	}
	plan := make([]UnprodNeedRow, 0)
	for _, r := range rows {
		if r.GapG <= 0 {
			continue
		}
		if selected[producePlanKey(r.ProductID, r.SpecG)] {
			plan = append(plan, r)
		}
	}
	if len(plan) == 0 {
		return "", fmt.Errorf("没有可开始生产的数据")
	}
	yieldMap, err := loadProductYieldRateMap(ctx, pool, schema)
	if err != nil {
		return "", err
	}
	for _, r := range plan {
		key := producePlanKey(r.ProductID, r.SpecG)
		if inputByKey[key] <= 0 {
			return "", fmt.Errorf("投料数必须大于0")
		}
	}
	batchID, _, _, err := allocateUnproducedRows(ctx, pool, schema, plan, operator)
	if err != nil {
		return "", err
	}
	if err := saveRunningItems(ctx, pool, schema, batchID, plan, inputByKey, yieldMap, operator); err != nil {
		return "", err
	}
	_ = setOrdersProcessStatusByNeeds(ctx, pool, schema, plan, "生产中")
	return batchID, nil
}

func saveRunningItems(ctx context.Context, pool *pgxpool.Pool, schema, batchID string, rows []UnprodNeedRow, inputByKey map[string]int64, yieldByProductID map[int64]float64, operator string) error {
	for _, r := range rows {
		needG := r.GapG
		if needG <= 0 {
			continue
		}
		key := producePlanKey(r.ProductID, r.SpecG)
		inputG := inputByKey[key]
		if inputG <= 0 {
			inputG = defaultProductionInputG(needG, yieldByProductID[r.ProductID])
		}
		yieldRate := normalizeYieldRate(yieldByProductID[r.ProductID])
		plan := runningInventoryPlan(r.SpecG, needG, inputG, yieldRate)
		_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.produce_running_items(batch_id,product_id,product_name,spec_g,need_g,order_nos,status,started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g) VALUES($1,$2,$3,$4,$5,$6,'running',$7,now(),$8,$9,$10,$11)`, schema), batchID, r.ProductID, r.Product, r.SpecG, needG, r.OrderNos, operator, inputG, yieldRate, plan.Units, plan.LooseG)
		if err != nil {
			return err
		}
	}
	return nil
}

func listRunningItems(ctx context.Context, pool *pgxpool.Pool, schema string) ([]ProduceRunRow, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT id,batch_id,product_name,product_id,spec_g,need_g,COALESCE(input_g,0),COALESCE(bom_yield_rate,0.8),COALESCE(planned_units,0),COALESCE(planned_loose_g,0),order_nos,COALESCE(started_by,''),started_at,to_char(started_at,'YYYY-MM-DD HH24:MI') FROM %s.produce_running_items WHERE status='running' ORDER BY started_at DESC,id DESC`, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProduceRunRow, 0)
	for rows.Next() {
		var r ProduceRunRow
		if err := rows.Scan(&r.ID, &r.BatchID, &r.Product, &r.ProductID, &r.SpecG, &r.NeedG, &r.InputG, &r.BomYieldRate, &r.PlanUnits, &r.PlanLooseG, &r.OrderNos, &r.StartedBy, &r.StartedAtTime, &r.StartedAt); err != nil {
			return nil, err
		}
		r.BomYieldRate = normalizeYieldRate(r.BomYieldRate)
		if r.InputG <= 0 {
			r.InputG = defaultProductionInputG(r.NeedG, r.BomYieldRate)
		}
		plan := runningInventoryPlan(r.SpecG, r.NeedG, r.InputG, r.BomYieldRate)
		r.PlanUnits = plan.Units
		r.PlanLooseG = plan.LooseG
		out = append(out, r)
	}
	return out, rows.Err()
}

func finishRunningItem(ctx context.Context, pool *pgxpool.Pool, schema string, id, finishedUnits, finishedLooseG int64, hasFinishedInput bool, operator string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var r ProduceRunRow
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,batch_id,product_name,product_id,spec_g,need_g,COALESCE(input_g,0),COALESCE(bom_yield_rate,0.8),COALESCE(planned_units,0),COALESCE(planned_loose_g,0),order_nos,COALESCE(started_by,''),started_at,to_char(started_at,'YYYY-MM-DD HH24:MI') FROM %s.produce_running_items WHERE id=$1 AND status='running' FOR UPDATE`, schema), id).Scan(&r.ID, &r.BatchID, &r.Product, &r.ProductID, &r.SpecG, &r.NeedG, &r.InputG, &r.BomYieldRate, &r.PlanUnits, &r.PlanLooseG, &r.OrderNos, &r.StartedBy, &r.StartedAtTime, &r.StartedAt); err != nil {
		return err
	}
	r.BomYieldRate = normalizeYieldRate(r.BomYieldRate)
	if r.InputG <= 0 {
		r.InputG = defaultProductionInputG(r.NeedG, r.BomYieldRate)
	}

	var unitsBefore, looseBefore int64
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND spec_g=$2 FOR UPDATE`, schema), r.ProductID, r.SpecG).Scan(&unitsBefore, &looseBefore)
	cur := InvQty{Units: unitsBefore, LooseG: looseBefore}
	add := runningInventoryPlan(r.SpecG, r.NeedG, r.InputG, r.BomYieldRate)
	if hasFinishedInput {
		add, err = normalizeFinishedInventoryAddition(r.SpecG, finishedUnits, finishedLooseG)
		if err != nil {
			return err
		}
	}
	if add.Units <= 0 && add.LooseG <= 0 {
		return fmt.Errorf("请填写完成件数或散装余量")
	}
	actualYield, err := actualYieldRate(r.SpecG, add.Units, add.LooseG, r.InputG)
	if err != nil {
		return err
	}
	finishedTotal := finishedTotalG(r.SpecG, add.Units, add.LooseG)
	nowQty := InvQty{Units: cur.Units + add.Units, LooseG: cur.LooseG + add.LooseG}
	norm, err := invNormalize(r.SpecG, nowQty)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g,updated_at) VALUES($1,$2,$3,$4,now()) ON CONFLICT (product_id,spec_g) DO UPDATE SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()`, schema), r.ProductID, r.SpecG, norm.Units, norm.LooseG); err != nil {
		return err
	}
	if err := deductMaterialsForRunningItemTx(ctx, tx, schema, r, add, operator); err != nil {
		return err
	}
	materialSummary, err := listMaterialConsumptionSummaryTx(ctx, tx, schema, r.ID)
	if err != nil {
		return err
	}
	materialSummaryJSON, err := marshalMaterialConsumptionSummary(materialSummary)
	if err != nil {
		return err
	}
	finishedAt := time.Now()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_logs(
			running_item_id,batch_id,product_id,product_name,spec_g,order_nos,
			planned_need_g,input_g,bom_yield_rate,
			finished_units,finished_loose_g,finished_total_g,actual_yield_rate,
			started_by,started_at,finished_by,finished_at,
			inventory_units_before,inventory_loose_g_before,
			inventory_units_after,inventory_loose_g_after,
			material_summary,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,now())
	`, schema),
		r.ID, r.BatchID, r.ProductID, r.Product, r.SpecG, r.OrderNos,
		r.NeedG, r.InputG, r.BomYieldRate,
		add.Units, add.LooseG, finishedTotal, actualYield,
		r.StartedBy, r.StartedAtTime, operator, finishedAt,
		unitsBefore, looseBefore, norm.Units, norm.LooseG,
		materialSummaryJSON,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_items SET status='done',finished_by=$2,finished_at=$3 WHERE id=$1`, schema), id, operator, finishedAt); err != nil {
		return err
	}
	for _, no := range splitOrderNos(r.OrderNos) {
		if err := completeOrderIfAllRunningDone(ctx, tx, schema, no); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	auditInsert(ctx, pool, schema, operator, "produce_running", &id, "finish", strPtrStr("material_consumption"), nil, strPtrStr("deducted"), AuditMeta{"running_item_id": id, "product_id": r.ProductID, "spec_g": r.SpecG, "need_g": r.NeedG, "input_g": r.InputG, "bom_yield_rate": r.BomYieldRate, "finished_units": add.Units, "finished_loose_g": add.LooseG, "finished_total_g": finishedTotal, "actual_yield_rate": actualYield})
	return nil
}

func cancelRunningItem(ctx context.Context, pool *pgxpool.Pool, schema string, id int64, operator string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var r ProduceRunRow
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,batch_id,product_name,product_id,spec_g,need_g,COALESCE(input_g,0),COALESCE(bom_yield_rate,0.8),COALESCE(planned_units,0),COALESCE(planned_loose_g,0),order_nos,COALESCE(started_by,''),started_at,to_char(started_at,'YYYY-MM-DD HH24:MI') FROM %s.produce_running_items WHERE id=$1 AND status='running' FOR UPDATE`, schema), id).Scan(&r.ID, &r.BatchID, &r.Product, &r.ProductID, &r.SpecG, &r.NeedG, &r.InputG, &r.BomYieldRate, &r.PlanUnits, &r.PlanLooseG, &r.OrderNos, &r.StartedBy, &r.StartedAtTime, &r.StartedAt); err != nil {
		return err
	}
	restoredG, err := restoreRunningAllocationTx(ctx, tx, schema, r)
	if err != nil {
		return err
	}
	cancelledAt := time.Now()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_items SET status='cancelled',finished_by=$2,finished_at=$3 WHERE id=$1`, schema), id, operator, cancelledAt); err != nil {
		return err
	}
	for _, no := range splitOrderNos(r.OrderNos) {
		if err := resetOrderIfNoRunningItemsTx(ctx, tx, schema, no); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	auditInsert(ctx, pool, schema, operator, "produce_running", &id, "cancel", strPtrStr("finished_allocation"), strPtrStr(fmt.Sprintf("%d", restoredG)), strPtrStr("restored"), AuditMeta{"running_item_id": id, "batch_id": r.BatchID, "product_id": r.ProductID, "spec_g": r.SpecG, "restored_g": restoredG})
	return nil
}

func parseOptionalInt64(v string) (int64, bool, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}

func plannedFinishedInventoryAddition(specG, needG int64) InvQty {
	if specG <= 0 || needG <= 0 {
		return InvQty{}
	}
	return InvQty{Units: needG / specG, LooseG: needG % specG}
}

func normalizeFinishedInventoryAddition(specG, units, looseG int64) (InvQty, error) {
	if units < 0 || looseG < 0 {
		return InvQty{}, fmt.Errorf("完成件数和散装余量不能为负数")
	}
	return invNormalize(specG, InvQty{Units: units, LooseG: looseG})
}

func restoreAllocatedInventory(specG int64, current InvQty, deductedG int64) (InvQty, error) {
	if deductedG < 0 {
		return InvQty{}, fmt.Errorf("restored grams cannot be negative")
	}
	return invNormalize(specG, InvQty{Units: current.Units, LooseG: current.LooseG + deductedG})
}

func restoreRunningAllocationTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow) (int64, error) {
	var deductedG int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT deducted_g FROM %s.finished_allocation_logs WHERE batch_id=$1 AND product_id=$2 AND spec_g=$3 ORDER BY id DESC LIMIT 1`, schema), r.BatchID, r.ProductID, r.SpecG).Scan(&deductedG)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	if deductedG <= 0 {
		return 0, nil
	}
	var units, loose int64
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND spec_g=$2 FOR UPDATE`, schema), r.ProductID, r.SpecG).Scan(&units, &loose)
	norm, err := restoreAllocatedInventory(r.SpecG, InvQty{Units: units, LooseG: loose}, deductedG)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g,updated_at) VALUES($1,$2,$3,$4,now()) ON CONFLICT (product_id,spec_g) DO UPDATE SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()`, schema), r.ProductID, r.SpecG, norm.Units, norm.LooseG); err != nil {
		return 0, err
	}
	return deductedG, nil
}

func splitOrderNos(v string) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, x := range strings.Split(strings.TrimSpace(v), ",") {
		no := strings.TrimSpace(x)
		if no == "" || seen[no] {
			continue
		}
		seen[no] = true
		out = append(out, no)
	}
	return out
}

func completeOrderIfAllRunningDone(ctx context.Context, tx pgx.Tx, schema, orderNo string) error {
	if strings.TrimSpace(orderNo) == "" {
		return nil
	}
	var hasRunning bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.produce_running_items WHERE status='running' AND $1 = ANY(string_to_array(replace(order_nos,' ',''),',')))`, schema), orderNo).Scan(&hasRunning); err != nil {
		return err
	}
	if hasRunning {
		return nil
	}
	statusID, err := lookupProcessStatusIDTx(ctx, tx, schema, "生产完成", "已生产完成")
	if err != nil {
		return err
	}
	if statusID <= 0 {
		return nil
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=$2 WHERE order_no=$1`, schema), orderNo, statusID)
	return err
}

func resetOrderIfNoRunningItemsTx(ctx context.Context, tx pgx.Tx, schema, orderNo string) error {
	if strings.TrimSpace(orderNo) == "" {
		return nil
	}
	var hasRunning bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.produce_running_items WHERE status='running' AND $1 = ANY(string_to_array(replace(order_nos,' ',''),',')))`, schema), orderNo).Scan(&hasRunning); err != nil {
		return err
	}
	if hasRunning {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=NULL WHERE order_no=$1`, schema), orderNo)
	return err
}

func lookupProcessStatusIDTx(ctx context.Context, tx pgx.Tx, schema string, names ...string) (int64, error) {
	for _, name := range names {
		var id int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.order_process_statuses WHERE name=$1 ORDER BY id LIMIT 1`, schema), name).Scan(&id)
		if err == nil && id > 0 {
			return id, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
	}
	return 0, nil
}

func setOrdersProcessStatusByNeeds(ctx context.Context, pool *pgxpool.Pool, schema string, rows []UnprodNeedRow, statusName string) error {
	statusID := int64(0)
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(id,0) FROM %s.order_process_statuses WHERE name=$1 ORDER BY id LIMIT 1`, schema), statusName).Scan(&statusID)
	if statusID <= 0 {
		return nil
	}
	nos := map[string]bool{}
	for _, r := range rows {
		for _, x := range strings.Split(strings.TrimSpace(r.OrderNos), ",") {
			x = strings.TrimSpace(x)
			if x != "" {
				nos[x] = true
			}
		}
	}
	for no := range nos {
		_, _ = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=$2 WHERE order_no=$1`, schema), no, statusID)
	}
	return nil
}
