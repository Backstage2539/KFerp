package main

import (
	"context"
	"errors"
	"fmt"
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
	ID        int64
	BatchID   string
	Product   string
	ProductID int64
	SpecG     int64
	NeedG     int64
	OrderNos  string
	StartedAt string
}

type ProduceRunPageData struct {
	Rows  []ProduceRunRow
	Ok    bool
	Error string
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
	_, err := pool.Exec(ctx, q)
	return err
}

func registerProductionFlowPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.POST("/produce/start", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		sel := strings.TrimSpace(c.FormValue("selected"))
		from := strings.TrimSpace(c.FormValue("from"))
		to := strings.TrimSpace(c.FormValue("to"))
		var cid int64
		if v := strings.TrimSpace(c.FormValue("customer_id")); v != "" {
			cid, _ = strconv.ParseInt(v, 10, 64)
		}
		if sel == "" {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape("请先生成计划并选择项目"))
		}
		selected := map[string]bool{}
		for _, k := range strings.Split(sel, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				selected[k] = true
			}
		}
		rows, err := fetchUnproducedNeeds(c.Request().Context(), pool, schema, from, to, cid)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		plan := make([]UnprodNeedRow, 0)
		for _, r := range rows {
			if r.GapG <= 0 {
				continue
			}
			if selected[fmt.Sprintf("%d-%d", r.ProductID, r.SpecG)] {
				plan = append(plan, r)
			}
		}
		if len(plan) == 0 {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape("没有可开始生产的数据"))
		}
		operator := actorOf(c)
		batchID, _, _, err := allocateUnproducedRows(c.Request().Context(), pool, schema, plan, operator)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		if err := saveRunningItems(c.Request().Context(), pool, schema, batchID, plan, operator); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/unproduced?err="+url.QueryEscape(err.Error()))
		}
		_ = setOrdersProcessStatusByNeeds(c.Request().Context(), pool, schema, plan, "生产中")
		return c.Redirect(http.StatusSeeOther, "/produce/running?ok=1")
	})

	e.GET("/produce/running", func(c echo.Context) error {
		data := ProduceRunPageData{Ok: strings.TrimSpace(c.QueryParam("ok")) == "1"}
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
		if err := finishRunningItem(c.Request().Context(), pool, schema, id, actorOf(c)); err != nil {
			return c.Redirect(http.StatusSeeOther, "/produce/running?err="+url.QueryEscape(err.Error()))
		}
		return c.Redirect(http.StatusSeeOther, "/produce/running?ok=1")
	})
}

func saveRunningItems(ctx context.Context, pool *pgxpool.Pool, schema, batchID string, rows []UnprodNeedRow, operator string) error {
	for _, r := range rows {
		needG := r.GapG
		if needG <= 0 {
			continue
		}
		_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.produce_running_items(batch_id,product_id,product_name,spec_g,need_g,order_nos,status,started_by,started_at) VALUES($1,$2,$3,$4,$5,$6,'running',$7,now())`, schema), batchID, r.ProductID, r.Product, r.SpecG, needG, r.OrderNos, operator)
		if err != nil {
			return err
		}
	}
	return nil
}

func listRunningItems(ctx context.Context, pool *pgxpool.Pool, schema string) ([]ProduceRunRow, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT id,batch_id,product_name,product_id,spec_g,need_g,order_nos,to_char(started_at,'YYYY-MM-DD HH24:MI') FROM %s.produce_running_items WHERE status='running' ORDER BY started_at DESC,id DESC`, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProduceRunRow, 0)
	for rows.Next() {
		var r ProduceRunRow
		if err := rows.Scan(&r.ID, &r.BatchID, &r.Product, &r.ProductID, &r.SpecG, &r.NeedG, &r.OrderNos, &r.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func finishRunningItem(ctx context.Context, pool *pgxpool.Pool, schema string, id int64, operator string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var r ProduceRunRow
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,batch_id,product_name,product_id,spec_g,need_g,order_nos,to_char(started_at,'YYYY-MM-DD HH24:MI') FROM %s.produce_running_items WHERE id=$1 AND status='running' FOR UPDATE`, schema), id).Scan(&r.ID, &r.BatchID, &r.Product, &r.ProductID, &r.SpecG, &r.NeedG, &r.OrderNos, &r.StartedAt); err != nil {
		return err
	}

	var units, loose int64
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND spec_g=$2 FOR UPDATE`, schema), r.ProductID, r.SpecG).Scan(&units, &loose)
	cur := InvQty{Units: units, LooseG: loose}
	add := InvQty{Units: r.NeedG / r.SpecG, LooseG: r.NeedG % r.SpecG}
	nowQty := InvQty{Units: cur.Units + add.Units, LooseG: cur.LooseG + add.LooseG}
	norm, err := invNormalize(r.SpecG, nowQty)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g,updated_at) VALUES($1,$2,$3,$4,now()) ON CONFLICT (product_id,spec_g) DO UPDATE SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()`, schema), r.ProductID, r.SpecG, norm.Units, norm.LooseG); err != nil {
		return err
	}
	if err := deductMaterialsForRunningItemTx(ctx, tx, schema, r, operator); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_items SET status='done',finished_by=$2,finished_at=$3 WHERE id=$1`, schema), id, operator, time.Now()); err != nil {
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
	auditInsert(ctx, pool, schema, operator, "produce_running", &id, "finish", strPtrStr("material_consumption"), nil, strPtrStr("deducted"), AuditMeta{"running_item_id": id, "product_id": r.ProductID, "spec_g": r.SpecG, "need_g": r.NeedG})
	return nil
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
