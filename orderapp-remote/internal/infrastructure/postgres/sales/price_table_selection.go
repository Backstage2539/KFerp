package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/infrastructure/postgres/orderbeans"
)

func (r Repository) loadNamedPriceTableOptions(ctx context.Context, options []salesapp.BeanListVersionOption) error {
	ids := []int64{}
	seen := map[int64]bool{}
	for _, row := range options {
		if !seen[row.ID] {
			ids = append(ids, row.ID)
			seen[row.ID] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id,COALESCE(config_json->'publication_batch','{}'::jsonb) FROM %s.bean_list_publications WHERE id=ANY($1)`, r.schema), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	meta := map[int64]salesapp.BeanListVersionOption{}
	for rows.Next() {
		var id int64
		var raw []byte
		var item salesapp.BeanListVersionOption
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		meta[id] = item
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range options {
		m := meta[options[i].ID]
		options[i].ReleaseID = m.ReleaseID
		options[i].TableKey = m.TableKey
		options[i].TableName = m.TableName
		options[i].IsDefaultTable = m.IsDefaultTable
		if m.TableName != "" {
			options[i].Label = m.TableName + " · " + options[i].VersionNo
		}
	}
	return nil
}

func validateSelectedPriceTablesTx(ctx context.Context, tx pgx.Tx, schema string, cmd salesapp.SaveOrderCommand, itemIDs []int64) (map[int64]salesapp.BeanListVersionOption, error) {
	ids := append([]int64(nil), cmd.SelectedPriceTableIDs...)
	for _, id := range itemIDs {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,list_type,COALESCE(NULLIF(classification_template_id,0),product_type_category_id,0),owner_type,owner_key,status,publication_purpose,COALESCE(config_json->'publication_batch','{}'::jsonb)
	FROM %s.bean_list_publications WHERE id=ANY($1) FOR SHARE`, schema), ids)
	if err != nil {
		return nil, err
	}
	meta := map[int64]salesapp.BeanListVersionOption{}
	for rows.Next() {
		var row salesapp.BeanListVersionOption
		var owner, key, status, purpose string
		var raw []byte
		if err := rows.Scan(&row.ID, &row.ListType, &row.ClassificationTemplateID, &owner, &key, &status, &purpose, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			rows.Close()
			return nil, err
		}
		if (len(cmd.SelectedPriceTableIDs) > 0 || row.ReleaseID != "") && (status != "published" || purpose != "factory_supply" || (owner != "official" && (owner != "customer" || key != fmt.Sprint(cmd.CustomerID)))) {
			rows.Close()
			return nil, fmt.Errorf("所选价格表已失效或不属于当前客户")
		}
		meta[row.ID] = row
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	hasNamed := false
	for _, row := range meta {
		hasNamed = hasNamed || row.ReleaseID != ""
	}
	if !hasNamed && len(cmd.SelectedPriceTableIDs) == 0 {
		return meta, nil
	}
	selected := map[string]int64{}
	for _, id := range cmd.SelectedPriceTableIDs {
		row, ok := meta[id]
		if !ok {
			return nil, fmt.Errorf("所选价格表不存在")
		}
		key := salesapp.OrderPriceTableTypeKey(row)
		if prior := selected[key]; prior > 0 && prior != id {
			return nil, fmt.Errorf("同一商品类型只能选择一张价格表")
		}
		selected[key] = id
	}
	for _, id := range itemIDs {
		if id <= 0 {
			if len(cmd.SelectedPriceTableIDs) > 0 {
				return nil, fmt.Errorf("商品未绑定所选价格表，请重新选择规格")
			}
			continue
		}
		row, ok := meta[id]
		if !ok {
			return nil, fmt.Errorf("所选价格表不存在")
		}
		key := salesapp.OrderPriceTableTypeKey(row)
		if prior := selected[key]; prior > 0 && prior != id {
			return nil, fmt.Errorf("商品价格来源与所选价格表不一致")
		}
		if len(cmd.SelectedPriceTableIDs) > 0 && selected[key] == 0 {
			return nil, fmt.Errorf("请先选择该商品类型的价格表")
		}
		selected[key] = id
	}
	return meta, nil
}

func withNamedPriceTableSnapshot(raw string, table salesapp.BeanListVersionOption) string {
	if table.TableName == "" {
		return raw
	}
	source := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &source)
	if source == nil {
		source = map[string]any{}
	}
	source["price_table_name"] = table.TableName
	source["price_table_key"] = table.TableKey
	source["publication_release_id"] = table.ReleaseID
	body, err := json.Marshal(source)
	if err != nil {
		return raw
	}
	return string(body)
}

// OrderPriceTableOptions shares the customer ownership and public fallback rules
// with customer self-service catalogs.
func (r Repository) OrderPriceTableOptions(ctx context.Context) ([]salesapp.BeanListVersionOption, error) {
	return r.fetchOrderBeanListVersionOptions(ctx)
}

func IsCurrentOrderPriceTableTx(ctx context.Context, tx pgx.Tx, schema string, customerID, publicationID int64, listType string) (bool, error) {
	return isCurrentDefaultOrderPublicationTx(ctx, tx, schema, customerID, publicationID, listType)
}

func validateNamedOrderItemSnapshot(raw []byte, table salesapp.BeanListVersionOption, productID, bomSpecID, bomVariantID, specG, qty int64, salesUnit string, unitBagCount int64) error {
	price, ok := orderbeans.PublishedSnapshotPricing(raw, productID, bomSpecID, bomVariantID, specG, qty, table.ListType, salesUnit, unitBagCount)
	if !ok || price.UnitPrice <= 0 {
		return fmt.Errorf("价格表「%s」未发布该商品规格或数量档位，请重新选择", table.TableName)
	}
	return nil
}
