package costing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appcosting "orderapp/internal/application/costing"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

const customerOrderPriceTableTypeKeySQL = `CASE
	WHEN b.classification_template_id>0 THEN 'classification:'||b.classification_template_id::text
	WHEN b.product_type_category_id>0 THEN 'classification:'||b.product_type_category_id::text
	ELSE 'legacy:'||trim(b.list_type)
END`

func (r Repository) rejectBoundCustomerOrderPriceTablePublications(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, ids []int64) error {
	var usages string
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(string_agg(DISTINCT CASE usage_code WHEN 'direct_ship' THEN '一件代发' ELSE '商品下单' END,'、'),'')
		FROM %s.customer_order_price_table_bindings
		WHERE publication_id=ANY($1)
	`, r.schema), ids).Scan(&usages)
	if err != nil {
		return err
	}
	if usages != "" {
		return fmt.Errorf("该价格表正在用于%s，请先取消或更换指定", usages)
	}
	return nil
}

func (r Repository) CustomerOrderPriceTableConfig(ctx context.Context, query appcosting.CustomerOrderPriceTableQuery) (appcosting.CustomerOrderPriceTableConfig, error) {
	if query.CustomerID <= 0 {
		return appcosting.CustomerOrderPriceTableConfig{}, fmt.Errorf("customer required")
	}
	out := appcosting.CustomerOrderPriceTableConfig{
		Candidates: []appcosting.CustomerOrderPriceTableCandidate{},
		Bindings:   []appcosting.CustomerOrderPriceTableBinding{},
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT b.id,%s,b.list_type,b.product_type_category_id,b.product_type_name,
		       b.classification_template_id,
		       COALESCE(NULLIF(b.publication_table_name,''),NULLIF(b.config_json->'publication_batch'->>'table_name',''),b.version_no),
		       b.version_no
		FROM %s.bean_list_publications b
		WHERE b.owner_type='customer' AND b.owner_key=($1::bigint)::text
		  AND b.publication_purpose='factory_supply'
		  AND b.status='published' AND b.deleted_at IS NULL
		ORDER BY b.product_type_name,b.published_at DESC NULLS LAST,b.id DESC
	`, customerOrderPriceTableTypeKeySQL, r.schema), query.CustomerID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var row appcosting.CustomerOrderPriceTableCandidate
		if err := rows.Scan(&row.PublicationID, &row.ProductTypeKey, &row.ListType, &row.ProductTypeCategoryID, &row.ProductTypeName, &row.ClassificationTemplateID, &row.TableName, &row.Version); err != nil {
			rows.Close()
			return out, err
		}
		out.Candidates = append(out.Candidates, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows, err = r.pool.Query(ctx, fmt.Sprintf(`
		SELECT x.customer_id,x.usage_code,x.product_type_key,x.publication_id,
		       b.list_type,b.product_type_category_id,b.product_type_name,b.classification_template_id,
		       COALESCE(NULLIF(b.publication_table_name,''),NULLIF(b.config_json->'publication_batch'->>'table_name',''),b.version_no),
		       b.version_no,x.revision,to_char(x.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM %s.customer_order_price_table_bindings x
		JOIN %s.bean_list_publications b ON b.id=x.publication_id
		WHERE x.customer_id=$1
		ORDER BY x.product_type_key,x.usage_code
	`, r.schema, r.schema), query.CustomerID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var row appcosting.CustomerOrderPriceTableBinding
		if err := rows.Scan(&row.CustomerID, &row.UsageCode, &row.ProductTypeKey, &row.PublicationID, &row.ListType, &row.ProductTypeCategoryID, &row.ProductTypeName, &row.ClassificationTemplateID, &row.TableName, &row.Version, &row.Revision, &row.UpdatedAt); err != nil {
			return out, err
		}
		out.Bindings = append(out.Bindings, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveCustomerOrderPriceTableBinding(ctx context.Context, cmd appcosting.SaveCustomerOrderPriceTableBindingCommand) (appcosting.CustomerOrderPriceTableConfig, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return appcosting.CustomerOrderPriceTableConfig{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("customer-order-price-table:%d:%s", cmd.CustomerID, cmd.UsageCode)); err != nil {
		return appcosting.CustomerOrderPriceTableConfig{}, err
	}
	productTypeKey := strings.TrimSpace(cmd.ProductTypeKey)
	if cmd.PublicationID > 0 {
		var resolved string
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT %s
			FROM %s.bean_list_publications b
			WHERE b.id=$1 AND b.owner_type='customer' AND b.owner_key=($2::bigint)::text
			  AND b.publication_purpose='factory_supply'
			  AND b.status='published' AND b.deleted_at IS NULL
			FOR SHARE
		`, customerOrderPriceTableTypeKeySQL, r.schema), cmd.PublicationID, cmd.CustomerID).Scan(&resolved)
		if errors.Is(err, pgx.ErrNoRows) {
			return appcosting.CustomerOrderPriceTableConfig{}, fmt.Errorf("价格表版本不存在、未发布或不属于当前客户")
		}
		if err != nil {
			return appcosting.CustomerOrderPriceTableConfig{}, err
		}
		if productTypeKey != "" && productTypeKey != resolved {
			return appcosting.CustomerOrderPriceTableConfig{}, fmt.Errorf("价格表版本不属于当前商品类型")
		}
		productTypeKey = resolved
	}
	var oldPublicationID, oldRevision int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT publication_id,revision
		FROM %s.customer_order_price_table_bindings
		WHERE customer_id=$1 AND usage_code=$2 AND product_type_key=$3
		FOR UPDATE
	`, r.schema), cmd.CustomerID, cmd.UsageCode, productTypeKey).Scan(&oldPublicationID, &oldRevision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return appcosting.CustomerOrderPriceTableConfig{}, err
	}
	exists := err == nil
	if (exists && (cmd.ExpectedRevision <= 0 || oldRevision != cmd.ExpectedRevision)) || (!exists && cmd.ExpectedRevision > 0) {
		return appcosting.CustomerOrderPriceTableConfig{}, fmt.Errorf("价格表指定已变化，请刷新后重试")
	}
	if exists && oldPublicationID == cmd.PublicationID {
		if err := tx.Commit(ctx); err != nil {
			return appcosting.CustomerOrderPriceTableConfig{}, err
		}
		return r.CustomerOrderPriceTableConfig(ctx, appcosting.CustomerOrderPriceTableQuery{CustomerID: cmd.CustomerID})
	}
	if cmd.PublicationID <= 0 {
		if exists {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.customer_order_price_table_bindings WHERE customer_id=$1 AND usage_code=$2 AND product_type_key=$3`, r.schema), cmd.CustomerID, cmd.UsageCode, productTypeKey); err != nil {
				return appcosting.CustomerOrderPriceTableConfig{}, err
			}
		}
	} else {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_order_price_table_bindings(customer_id,usage_code,product_type_key,publication_id,revision,updated_by)
			VALUES($1,$2,$3,$4,1,$5)
			ON CONFLICT(customer_id,usage_code,product_type_key) DO UPDATE
			SET publication_id=EXCLUDED.publication_id,revision=%s.customer_order_price_table_bindings.revision+1,
			    updated_by=EXCLUDED.updated_by,updated_at=now()
		`, r.schema, r.schema), cmd.CustomerID, cmd.UsageCode, productTypeKey, cmd.PublicationID, cmd.Actor); err != nil {
			return appcosting.CustomerOrderPriceTableConfig{}, err
		}
	}
	oldValue := ""
	if oldPublicationID > 0 {
		oldValue = fmt.Sprintf("%d", oldPublicationID)
	}
	newValue := ""
	if cmd.PublicationID > 0 {
		newValue = fmt.Sprintf("%d", cmd.PublicationID)
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_order_price_table_binding", &cmd.CustomerID, "save_customer_order_price_table_binding", postgresinfra.StrPtr(cmd.UsageCode+":"+productTypeKey), postgresinfra.StrPtr(oldValue), postgresinfra.StrPtr(newValue), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "usage_code": cmd.UsageCode, "product_type_key": productTypeKey, "publication_id": cmd.PublicationID}); err != nil {
		return appcosting.CustomerOrderPriceTableConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return appcosting.CustomerOrderPriceTableConfig{}, err
	}
	return r.CustomerOrderPriceTableConfig(ctx, appcosting.CustomerOrderPriceTableQuery{CustomerID: cmd.CustomerID})
}
