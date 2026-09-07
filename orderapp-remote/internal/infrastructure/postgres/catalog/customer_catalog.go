package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/catalog"
	infra "orderapp/internal/infrastructure/postgres"
	"sort"
	"strings"
)

func ensureCustomerCatalogSchema(ctx context.Context, p *pgxpool.Pool, s string) error {
	_, e := p.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %[1]s.customer_product_catalog_nodes(
 id bigserial PRIMARY KEY,customer_id bigint NOT NULL,source_group_id bigint NOT NULL,source_item_id bigint NOT NULL DEFAULT 0,
 parent_source_item_id bigint NOT NULL DEFAULT 0,name text NOT NULL,code text NOT NULL DEFAULT '',sort_order int NOT NULL DEFAULT 100,
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),created_by text NOT NULL DEFAULT '',updated_by text NOT NULL DEFAULT '',
 UNIQUE(customer_id,source_group_id,source_item_id));
 ALTER TABLE %[1]s.product_customer_references ADD COLUMN IF NOT EXISTS customer_catalog_node_id bigint;
 ALTER TABLE %[1]s.product_customer_references ADD COLUMN IF NOT EXISTS catalog_sort_order int NOT NULL DEFAULT 100;
 CREATE INDEX IF NOT EXISTS customer_product_catalog_reference_idx ON %[1]s.product_customer_references(customer_id,customer_catalog_node_id);`, s))
	return e
}
func (r Repository) CopyCustomerCatalog(ctx context.Context, c app.CopyCustomerCatalogCommand) (app.CopyCustomerCatalogResult, error) {
	out := app.CopyCustomerCatalogResult{CustomerID: c.CustomerID, ProductIDs: []int64{}}
	if c.CustomerID <= 0 || (c.Mode != "all" && c.Mode != "selected") || (c.Mode == "selected" && len(c.ProductIDs) == 0) {
		return out, app.ValidationError{Message: "请选择客户和商品"}
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	var active bool
	e = tx.QueryRow(ctx, "SELECT active FROM "+r.schema+".customers WHERE id=$1 FOR UPDATE", c.CustomerID).Scan(&active)
	if e == pgx.ErrNoRows || e == nil && !active {
		return out, app.ValidationError{Message: "客户不存在或已停用"}
	}
	if e != nil {
		return out, e
	}
	ids := append([]int64{}, c.ProductIDs...)
	if c.Mode == "all" {
		ids = nil
		rows, e := tx.Query(ctx, "SELECT id FROM "+r.schema+".products WHERE active=true AND COALESCE(customer_id,0)=0 ORDER BY id")
		if e != nil {
			return out, e
		}
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return out, e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, e
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		var name string
		var owner int64
		e = tx.QueryRow(ctx, "SELECT name,COALESCE(customer_id,0),active FROM "+r.schema+".products WHERE id=$1 FOR SHARE", id).Scan(&name, &owner, &active)
		if e == pgx.ErrNoRows || e == nil && (!active || (owner > 0 && owner != c.CustomerID)) {
			return out, app.ValidationError{Message: fmt.Sprintf("商品 #%d 不存在、已停用或属于其他客户", id)}
		}
		if e != nil {
			return out, e
		}
		var refID int64
		var refActive bool
		e = tx.QueryRow(ctx, "SELECT id,active FROM "+r.schema+".product_customer_references WHERE product_id=$1 AND customer_id=$2 ORDER BY active DESC,id DESC LIMIT 1 FOR UPDATE", id, c.CustomerID).Scan(&refID, &refActive)
		if e == pgx.ErrNoRows {
			e = tx.QueryRow(ctx, "INSERT INTO "+r.schema+".product_customer_references(product_id,customer_id,customer_display_name,created_by,updated_by) VALUES($1,$2,$3,$4,$4) RETURNING id", id, c.CustomerID, name, c.Actor).Scan(&refID)
			out.Created++
		} else if e == nil && !refActive {
			_, e = tx.Exec(ctx, "UPDATE "+r.schema+".product_customer_references SET active=true,updated_at=now(),updated_by=$2 WHERE id=$1", refID, c.Actor)
			out.Restored++
		} else if e == nil {
			out.Unchanged++
		}
		if e != nil {
			return out, e
		}
		if e = r.attachCustomerCatalogTx(ctx, tx, refID, c.CustomerID, id, c.Actor); e != nil {
			return out, e
		}
		out.ProductIDs = append(out.ProductIDs, id)
	}
	if e = infra.AuditInsertTx(ctx, tx, r.schema, c.Actor, "customer_product_catalog", &c.CustomerID, "copy_products_to_customer", nil, nil, nil, infra.AuditMeta{"mode": c.Mode, "product_ids": out.ProductIDs, "created": out.Created, "restored": out.Restored, "unchanged": out.Unchanged}); e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	return out, nil
}
func (r Repository) attachCustomerCatalogTx(ctx context.Context, tx pgx.Tx, refID, cid, pid int64, actor string) error {
	var existing int64
	if e := tx.QueryRow(ctx, "SELECT COALESCE(customer_catalog_node_id,0) FROM "+r.schema+".product_customer_references WHERE id=$1", refID).Scan(&existing); e != nil {
		return e
	}
	if existing > 0 {
		return nil
	}
	var gid, iid int64
	position := 100
	e := tx.QueryRow(ctx, "SELECT group_id,group_item_id,sort_order FROM "+r.schema+".business_group_assignments WHERE usage_key='product_catalog' AND object_key='product' AND object_id=$1 AND object_ref='' ORDER BY id DESC LIMIT 1", pid).Scan(&gid, &iid, &position)
	if e != nil && e != pgx.ErrNoRows {
		return e
	}
	nid, e := r.ensureCustomerNodeTx(ctx, tx, cid, gid, iid, actor, map[int64]bool{})
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE "+r.schema+".product_customer_references SET customer_catalog_node_id=$2,catalog_sort_order=$3 WHERE id=$1 AND customer_catalog_node_id IS NULL", refID, nid, position)
	return e
}
func (r Repository) ensureCustomerNodeTx(ctx context.Context, tx pgx.Tx, cid, gid, iid int64, actor string, path map[int64]bool) (int64, error) {
	var id int64
	e := tx.QueryRow(ctx, "SELECT id FROM "+r.schema+".customer_product_catalog_nodes WHERE customer_id=$1 AND source_group_id=$2 AND source_item_id=$3", cid, gid, iid).Scan(&id)
	if e == nil {
		return id, nil
	}
	if e != pgx.ErrNoRows {
		return 0, e
	}
	name, code, position, parent := "未分类", "", 100, int64(0)
	if gid > 0 && iid == 0 {
		e = tx.QueryRow(ctx, "SELECT name,code,sort_order FROM "+r.schema+".business_groups WHERE id=$1", gid).Scan(&name, &code, &position)
	} else if iid > 0 {
		if path[iid] {
			return 0, app.ValidationError{Message: "来源分类存在循环"}
		}
		path[iid] = true
		e = tx.QueryRow(ctx, "SELECT name,code,sort_order,parent_id FROM "+r.schema+".business_group_items WHERE id=$1 AND group_id=$2", iid, gid).Scan(&name, &code, &position, &parent)
		if e == nil {
			_, e = r.ensureCustomerNodeTx(ctx, tx, cid, gid, parent, actor, path)
		}
	}
	if e != nil && !(gid == 0 && iid == 0 && e == pgx.ErrNoRows) {
		return 0, fmt.Errorf("来源分类 %d/%d: %w", gid, iid, e)
	}
	e = tx.QueryRow(ctx, "INSERT INTO "+r.schema+".customer_product_catalog_nodes(customer_id,source_group_id,source_item_id,parent_source_item_id,name,code,sort_order,created_by,updated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8) RETURNING id", cid, gid, iid, parent, name, code, position, actor).Scan(&id)
	return id, e
}
func (r Repository) CustomerCatalog(ctx context.Context, cid int64) (app.CustomerCatalog, error) {
	out := app.CustomerCatalog{CustomerID: cid, Nodes: []app.CustomerCatalogNode{}, Assignments: []app.BusinessGroupAssignment{}}
	rows, e := r.pool.Query(ctx, "SELECT id,customer_id,source_group_id,source_item_id,parent_source_item_id,name,code,sort_order FROM "+r.schema+".customer_product_catalog_nodes WHERE customer_id=$1 ORDER BY sort_order,id", cid)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var n app.CustomerCatalogNode
		if e = rows.Scan(&n.ID, &n.CustomerID, &n.SourceGroupID, &n.SourceItemID, &n.ParentSourceItemID, &n.Name, &n.Code, &n.SortOrder); e != nil {
			rows.Close()
			return out, e
		}
		out.Nodes = append(out.Nodes, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	rows, e = r.pool.Query(ctx, fmt.Sprintf(`SELECT r.id,n.source_group_id,n.source_item_id,r.product_id,r.catalog_sort_order FROM %[1]s.product_customer_references r JOIN %[1]s.customer_product_catalog_nodes n ON n.id=r.customer_catalog_node_id AND n.customer_id=r.customer_id WHERE r.customer_id=$1 AND r.active=true`, r.schema), cid)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		a := app.BusinessGroupAssignment{UsageKey: "product_catalog", ObjectKey: "product"}
		if e = rows.Scan(&a.ID, &a.GroupID, &a.GroupItemID, &a.ObjectID, &a.SortOrder); e != nil {
			return out, e
		}
		out.Assignments = append(out.Assignments, a)
	}
	return out, rows.Err()
}
func (r Repository) RenameCustomerCatalogNode(ctx context.Context, c app.RenameCustomerCatalogNodeCommand) error {
	if strings.TrimSpace(c.Name) == "" {
		return app.ValidationError{Message: "请填写客户分类名称"}
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var old string
	e = tx.QueryRow(ctx, "SELECT name FROM "+r.schema+".customer_product_catalog_nodes WHERE id=$1 AND customer_id=$2 FOR UPDATE", c.ID, c.CustomerID).Scan(&old)
	if e == pgx.ErrNoRows {
		return app.ValidationError{Message: "客户分类不存在或不属于当前客户"}
	}
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE "+r.schema+".customer_product_catalog_nodes SET name=$3,updated_at=now(),updated_by=$4 WHERE id=$1 AND customer_id=$2", c.ID, c.CustomerID, strings.TrimSpace(c.Name), c.Actor)
	if e != nil {
		return e
	}
	if e = infra.AuditInsertTx(ctx, tx, r.schema, c.Actor, "customer_product_catalog", &c.ID, "rename_customer_category", infra.StrPtr("name"), infra.StrPtr(old), infra.StrPtr(c.Name), infra.AuditMeta{"customer_id": c.CustomerID}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r Repository) MigrateCustomerCatalog(ctx context.Context, preview bool, actor string) (app.CustomerCatalogMigrationResult, error) {
	out := app.CustomerCatalogMigrationResult{ReferenceIDs: []int64{}}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	// Serialize migration with customer copy transactions in customer ID order.
	rows, e := tx.Query(ctx, "SELECT id FROM "+r.schema+".customers ORDER BY id FOR UPDATE")
	if e != nil {
		return out, e
	}
	rows.Close()
	rows, e = tx.Query(ctx, "SELECT id,customer_id,product_id FROM "+r.schema+".product_customer_references WHERE active=true AND customer_catalog_node_id IS NULL ORDER BY customer_id,id FOR UPDATE")
	if e != nil {
		return out, e
	}
	type ref struct{ id, cid, pid int64 }
	refs := []ref{}
	for rows.Next() {
		var v ref
		if e = rows.Scan(&v.id, &v.cid, &v.pid); e != nil {
			rows.Close()
			return out, e
		}
		refs = append(refs, v)
		out.ReferenceIDs = append(out.ReferenceIDs, v.id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	out.Missing = len(refs)
	if preview {
		return out, nil
	}
	var before, after, createdNodes json.RawMessage
	var maxNodeID int64
	if e = tx.QueryRow(ctx, "SELECT COALESCE(max(id),0) FROM "+r.schema+".customer_product_catalog_nodes").Scan(&maxNodeID); e != nil {
		return out, e
	}
	snapshotSQL := "SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'customer_catalog_node_id',customer_catalog_node_id,'catalog_sort_order',catalog_sort_order) ORDER BY id),'[]') FROM " + r.schema + ".product_customer_references WHERE id=ANY($1::bigint[])"
	if e = tx.QueryRow(ctx, snapshotSQL, out.ReferenceIDs).Scan(&before); e != nil {
		return out, e
	}
	for _, v := range refs {
		if e = r.attachCustomerCatalogTx(ctx, tx, v.id, v.cid, v.pid, actor); e != nil {
			return out, e
		}
		out.Applied++
	}
	if out.Applied > 0 {
		if e = tx.QueryRow(ctx, snapshotSQL, out.ReferenceIDs).Scan(&after); e != nil {
			return out, e
		}
		if e = tx.QueryRow(ctx, "SELECT COALESCE(jsonb_agg(to_jsonb(n) ORDER BY id),'[]') FROM "+r.schema+".customer_product_catalog_nodes n WHERE id>$1", maxNodeID).Scan(&createdNodes); e != nil {
			return out, e
		}
		if e = infra.AuditInsertTx(ctx, tx, r.schema, actor, "customer_product_catalog", nil, "migrate_customer_catalog", nil, nil, nil, infra.AuditMeta{"reference_ids": out.ReferenceIDs, "previous_node_id": nil, "applied": out.Applied, "before_references": before, "after_references": after, "created_nodes": createdNodes}); e != nil {
			return out, e
		}
	}
	return out, tx.Commit(ctx)
}
