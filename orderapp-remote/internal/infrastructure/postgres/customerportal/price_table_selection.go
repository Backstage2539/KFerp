package customerportal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	salesrepo "orderapp/internal/infrastructure/postgres/sales"
)

func (r Repository) loadPortalPriceTableCatalog(ctx context.Context, query app.ServicePageQuery, page *app.ServicePage) error {
	options, err := salesrepo.NewRepository(r.pool, r.schema).OrderPriceTableOptions(ctx)
	if err != nil {
		return err
	}
	page.PriceTableOptions = salesapp.CurrentOrderPriceTableOptions(options, query.CustomerID)
	selected, err := salesapp.ResolveOrderPriceTableSelection(options, query.CustomerID, query.SelectedPriceTableIDs, true)
	if err != nil {
		return err
	}
	ids := []int64{}
	for _, row := range selected {
		ids = append(ids, row.ID)
	}
	page.SelectedPriceTableIDs = ids
	snapshots := map[int64][]byte{}
	if len(ids) > 0 {
		rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id,content_json FROM %s.bean_list_publications WHERE id=ANY($1)`, r.schema), ids)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			var raw []byte
			if err := rows.Scan(&id, &raw); err != nil {
				rows.Close()
				return err
			}
			snapshots[id] = raw
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	page.Products = filterPortalSelectedPriceTableProducts(page.Products, selected, snapshots)
	return nil
}

func filterPortalSelectedPriceTableProducts(products []app.ProductSummary, tables []salesapp.BeanListVersionOption, snapshots map[int64][]byte) []app.ProductSummary {
	result := []app.ProductSummary{}
	for _, product := range products {
		for _, table := range tables {
			price, ok := orderbeans.PublishedCatalogPricing(snapshots[table.ID], product.ID, product.BomSpecID, product.BomVariantID, table.ListType)
			if !ok || price.UnitPrice <= 0 {
				continue
			}
			row := product
			row.BeanListPublicationID = table.ID
			row.PriceTableName = table.TableName
			row.DefaultPrice = fmt.Sprintf("%.2f", price.UnitPrice)
			result = append(result, row)
			break
		}
	}
	return result
}

func (r Repository) freezePortalPriceTableTx(ctx context.Context, tx pgx.Tx, cmd app.CreateFulfillmentOrderCommand, usage orderbeans.Usage, listType, source string) (string, error) {
	if usage.PublicationID <= 0 {
		return source, nil
	}
	var metadata salesapp.BeanListVersionOption
	var raw []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(config_json->'publication_batch','{}'::jsonb) FROM %s.bean_list_publications WHERE id=$1`, r.schema), usage.PublicationID).Scan(&raw); err != nil {
		return "", err
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return "", err
	}
	if cmd.BeanListPublicationID > 0 || metadata.ReleaseID != "" {
		if cmd.BeanListPublicationID == 0 && !metadata.IsDefaultTable {
			return "", fmt.Errorf("请先选择价格表，再选择该表中的商品规格")
		}
		current, err := salesrepo.IsCurrentOrderPriceTableTx(ctx, tx, r.schema, cmd.CustomerID, usage.PublicationID, listType)
		if err != nil {
			return "", err
		}
		if !current {
			return "", fmt.Errorf("价格表已更新或不属于当前客户，请重新选择")
		}
	}
	if metadata.TableName != "" {
		snapshot := map[string]any{}
		_ = json.Unmarshal([]byte(source), &snapshot)
		if snapshot == nil {
			snapshot = map[string]any{}
		}
		snapshot["price_table_name"] = metadata.TableName
		snapshot["price_table_key"] = metadata.TableKey
		snapshot["publication_release_id"] = metadata.ReleaseID
		b, err := json.Marshal(snapshot)
		return string(b), err
	}
	return source, nil
}
