package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProduceBatchOrderItem struct {
	OrderItemID    int64
	OrderID        int64
	ProductID      int64
	ProductName    string
	SpecG          int64
	NeedUnits      int64
	TotalUnits     int64
	AllocatedUnits int64
}

type ProduceBatchSummaryItem struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	SpecG       int64  `json:"spec_g"`
	NeedUnits   int64  `json:"need_units"`
	NeedG       int64  `json:"need_g"`
}

type ProduceBatchCreateResult struct {
	BatchID    string                    `json:"batch_id"`
	OrderCount int                       `json:"order_count"`
	Summary    []ProduceBatchSummaryItem `json:"summary"`
}

func ensureProduceBatchTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.produce_batches (
	batch_id TEXT PRIMARY KEY,
	status TEXT NOT NULL DEFAULT 'planned',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %s.produce_batch_items (
	id BIGSERIAL PRIMARY KEY,
	batch_id TEXT NOT NULL,
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	need_units BIGINT NOT NULL,
	need_g BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(batch_id, product_id, spec_g)
);
CREATE TABLE IF NOT EXISTS %s.produce_batch_order_items (
	id BIGSERIAL PRIMARY KEY,
	batch_id TEXT NOT NULL,
	order_id BIGINT NOT NULL,
	order_item_id BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %s.produce_batch_order_items DROP CONSTRAINT IF EXISTS produce_batch_order_items_order_item_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS produce_batch_order_items_batch_item_uq ON %s.produce_batch_order_items(batch_id, order_item_id);
-- DEV-044 v2: allow partial allocation across multiple batches
CREATE TABLE IF NOT EXISTS %s.produce_batch_allocations (
	id BIGSERIAL PRIMARY KEY,
	batch_id TEXT NOT NULL,
	order_id BIGINT NOT NULL,
	order_item_id BIGINT NOT NULL,
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	allocated_units BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(batch_id, order_item_id)
);
CREATE INDEX IF NOT EXISTS produce_batch_items_batch_idx ON %s.produce_batch_items(batch_id);
CREATE INDEX IF NOT EXISTS produce_batch_order_items_batch_idx ON %s.produce_batch_order_items(batch_id);
CREATE INDEX IF NOT EXISTS produce_batch_allocations_batch_idx ON %s.produce_batch_allocations(batch_id);
CREATE INDEX IF NOT EXISTS produce_batch_allocations_order_item_idx ON %s.produce_batch_allocations(order_item_id);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func newProduceBatchID() string {
	b := make([]byte, 1)
	_, _ = rand.Read(b)
	return fmt.Sprintf("P%s-%s", time.Now().Format("20060102-150405"), hex.EncodeToString(b))
}

func calcRemainingUnits(totalUnits, allocatedUnits int64) (int64, error) {
	if totalUnits < 0 || allocatedUnits < 0 {
		return 0, fmt.Errorf("units must be non-negative")
	}
	if allocatedUnits > totalUnits {
		return 0, fmt.Errorf("allocated units exceed total units")
	}
	return totalUnits - allocatedUnits, nil
}

func validateAllocateUnits(totalUnits, allocatedUnits, requestUnits int64) error {
	if requestUnits <= 0 {
		return fmt.Errorf("request units must be > 0")
	}
	remain, err := calcRemainingUnits(totalUnits, allocatedUnits)
	if err != nil {
		return err
	}
	if requestUnits > remain {
		return fmt.Errorf("request units exceed remaining units")
	}
	return nil
}

func aggregateBatchSummary(items []ProduceBatchOrderItem) []ProduceBatchSummaryItem {
	type key struct{ p, s int64 }
	m := map[key]ProduceBatchSummaryItem{}
	for _, it := range items {
		if it.ProductID <= 0 || it.SpecG <= 0 || it.NeedUnits <= 0 {
			continue
		}
		k := key{p: it.ProductID, s: it.SpecG}
		x := m[k]
		x.ProductID = it.ProductID
		x.ProductName = it.ProductName
		x.SpecG = it.SpecG
		x.NeedUnits += it.NeedUnits
		x.NeedG = x.NeedUnits * it.SpecG
		m[k] = x
	}
	out := make([]ProduceBatchSummaryItem, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProductName != out[j].ProductName {
			return out[i].ProductName < out[j].ProductName
		}
		if out[i].ProductID != out[j].ProductID {
			return out[i].ProductID < out[j].ProductID
		}
		return out[i].SpecG < out[j].SpecG
	})
	return out
}

func createProduceBatchFromOrders(ctx context.Context, pool *pgxpool.Pool, schema string, orderIDs []int64, operator string, requestUnitsByOrderItem map[int64]int64) (*ProduceBatchCreateResult, error) {
	if len(orderIDs) == 0 {
		return nil, fmt.Errorf("order_ids required")
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		operator = "order"
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qItems := fmt.Sprintf(`
SELECT oi.id, oi.order_id, oi.product_id, COALESCE(p.name,''),
       COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')::bigint AS spec_g,
       COALESCE(oi.qty,0)::bigint AS total_units,
       COALESCE((SELECT SUM(a.allocated_units) FROM %s.produce_batch_allocations a WHERE a.order_item_id=oi.id),0)::bigint AS allocated_units
FROM %s.order_items oi
JOIN %s.orders o ON o.id=oi.order_id
LEFT JOIN %s.products p ON p.id=oi.product_id
WHERE oi.order_id = ANY($1)
  AND o.is_void=false
  AND COALESCE(o.process_status_id,0) IN (1,2)
FOR UPDATE OF oi
`, schema, schema, schema, schema)
	rows, err := tx.Query(ctx, qItems, orderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ProduceBatchOrderItem, 0)
	seenOrders := map[int64]bool{}
	for rows.Next() {
		var it ProduceBatchOrderItem
		if err := rows.Scan(&it.OrderItemID, &it.OrderID, &it.ProductID, &it.ProductName, &it.SpecG, &it.TotalUnits, &it.AllocatedUnits); err != nil {
			return nil, err
		}
		seenOrders[it.OrderID] = true
		if it.ProductID <= 0 || it.SpecG <= 0 || it.TotalUnits <= 0 {
			continue
		}
		remain, err := calcRemainingUnits(it.TotalUnits, it.AllocatedUnits)
		if err != nil || remain <= 0 {
			continue
		}
		if len(requestUnitsByOrderItem) > 0 {
			req, ok := requestUnitsByOrderItem[it.OrderItemID]
			if !ok {
				continue
			}
			if err := validateAllocateUnits(it.TotalUnits, it.AllocatedUnits, req); err != nil {
				return nil, fmt.Errorf("order_item %d: %v", it.OrderItemID, err)
			}
			it.NeedUnits = req
		} else {
			it.NeedUnits = remain
		}
		if it.NeedUnits > 0 {
			items = append(items, it)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no eligible unproduced order items")
	}

	batchID := newProduceBatchID()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.produce_batches(batch_id,status,operator) VALUES($1,'planned',$2)`, schema), batchID, operator); err != nil {
		return nil, err
	}

	summary := aggregateBatchSummary(items)
	for _, s := range summary {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.produce_batch_items(batch_id,product_id,spec_g,need_units,need_g) VALUES($1,$2,$3,$4,$5)`, schema),
			batchID, s.ProductID, s.SpecG, s.NeedUnits, s.NeedG); err != nil {
			return nil, err
		}
	}

	for _, it := range items {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.produce_batch_order_items(batch_id,order_id,order_item_id) VALUES($1,$2,$3)`, schema),
			batchID, it.OrderID, it.OrderItemID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.produce_batch_allocations(batch_id,order_id,order_item_id,product_id,spec_g,allocated_units) VALUES($1,$2,$3,$4,$5,$6)`, schema),
			batchID, it.OrderID, it.OrderItemID, it.ProductID, it.SpecG, it.NeedUnits); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &ProduceBatchCreateResult{BatchID: batchID, OrderCount: len(seenOrders), Summary: summary}, nil
}
