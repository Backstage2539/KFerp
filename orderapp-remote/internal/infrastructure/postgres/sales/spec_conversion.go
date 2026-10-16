package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	salesapp "orderapp/internal/application/sales"
	conversion "orderapp/internal/domain/specconversion"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	"strconv"
	"strings"
)

func EnsureSpecConversionSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.spec_conversion_groups (
 id BIGSERIAL PRIMARY KEY,name TEXT NOT NULL,base_unit TEXT NOT NULL,revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 active BOOLEAN NOT NULL DEFAULT true,members_json JSONB NOT NULL DEFAULT '[]'::jsonb,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),created_by TEXT NOT NULL DEFAULT '',updated_by TEXT NOT NULL DEFAULT '');
 CREATE UNIQUE INDEX IF NOT EXISTS spec_conversion_groups_name_uq ON %s.spec_conversion_groups(lower(name));`, schema, schema))
	return err
}
func conversionGroupsTx(ctx context.Context, tx pgx.Tx, schema string) ([]conversion.Group, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,name,base_unit,revision,active,members_json FROM %s.spec_conversion_groups ORDER BY active DESC,name,id`, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []conversion.Group{}
	for rows.Next() {
		var g conversion.Group
		var raw []byte
		if err := rows.Scan(&g.ID, &g.Name, &g.BaseUnit, &g.Revision, &g.Active, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &g.Members); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}
func conversionSpecsTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) ([]salesapp.SpecConversionSpec, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT p.id,p.name,s.id,v.id,COALESCE(NULLIF(v.spec_name_snapshot,''),s.name),v.inventory_unit,COALESCE(s.source_spec_template_id,0),COALESCE(s.source_spec_template_key,'')
 FROM %[1]s.production_bom_output_bindings b JOIN %[1]s.products p ON p.id=b.output_id AND p.active
 JOIN %[1]s.product_bom_spec_authorities a ON a.product_id=p.id AND a.configured AND a.state='cutover'
 JOIN %[1]s.production_bom_versions bv ON bv.id=b.bom_version_id AND bv.bom_id=b.bom_id AND bv.status='published'
 JOIN %[1]s.production_bom_version_variants v ON v.version_id=bv.id
 JOIN %[1]s.production_bom_specs s ON s.id=v.bom_spec_id AND s.bom_id=b.bom_id
 WHERE b.output_type='product' AND b.is_default AND ($1::bigint=0 OR p.id=$1) ORDER BY p.name,v.sort_order,s.id`, schema), productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []salesapp.SpecConversionSpec{}
	for rows.Next() {
		var s salesapp.SpecConversionSpec
		if err := rows.Scan(&s.ProductID, &s.ProductName, &s.SpecID, &s.VariantID, &s.Name, &s.InventoryUnit, &s.TemplateID, &s.TemplateKey); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
func (r Repository) SpecConversionSettings(ctx context.Context) (salesapp.SpecConversionCatalog, error) {
	out := salesapp.SpecConversionCatalog{}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if out.Groups, err = conversionGroupsTx(ctx, tx, r.schema); err != nil {
		return out, err
	}
	if out.Specs, err = conversionSpecsTx(ctx, tx, r.schema, 0); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT code,name FROM %s.product_unit_definitions WHERE active ORDER BY code`, r.schema))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Units = []salesapp.SpecConversionUnit{}
	for rows.Next() {
		var u salesapp.SpecConversionUnit
		if err := rows.Scan(&u.Code, &u.Name); err != nil {
			return out, err
		}
		out.Units = append(out.Units, u)
	}
	return out, rows.Err()
}
func (r Repository) SaveSpecConversionGroup(ctx context.Context, c salesapp.SaveSpecConversionGroupCommand) (conversion.Group, error) {
	g := c.Group
	g.Name = strings.TrimSpace(g.Name)
	g.BaseUnit = strings.TrimSpace(g.BaseUnit)
	if err := g.Validate(); err != nil {
		return g, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return g, err
	}
	defer tx.Rollback(ctx)
	if g.Active || g.ID == 0 {
		var exists bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_unit_definitions WHERE code=$1 AND active)`, r.schema), g.BaseUnit).Scan(&exists); err != nil {
			return g, err
		}
		if !exists {
			return g, fmt.Errorf("请选择有效的基准单位")
		}
		specs, err := conversionSpecsTx(ctx, tx, r.schema, 0)
		if err != nil {
			return g, err
		}
		for i, m := range g.Members {
			found := false
			for _, s := range specs {
				if !strings.EqualFold(s.InventoryUnit, m.InventoryUnit) {
					continue
				}
				if (m.SpecID > 0 && m.SpecID == s.SpecID) || (m.TemplateID > 0 && m.TemplateID == s.TemplateID && m.TemplateKey == s.TemplateKey) {
					found = true
					g.Members[i].Label = s.Name
					break
				}
			}
			if !found {
				return g, fmt.Errorf("规格关系已失效，请选择当前默认已发布 BOM 中的规格")
			}
		}
		// A direct association and a template association may resolve to the same spec.
		for _, s := range specs {
			count := 0
			for _, m := range g.Members {
				single := conversion.Group{Members: []conversion.Member{m}}
				if _, e := single.Match(s.Identity()); e == nil {
					count++
				}
			}
			if count > 1 {
				return g, fmt.Errorf("规格「%s」存在冲突的换算关系", s.Name)
			}
		}
	}
	raw, _ := json.Marshal(g.Members)
	old := ""
	action := "create"
	if g.ID > 0 {
		var previous conversion.Group
		var oldRaw []byte
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,name,base_unit,revision,active,members_json FROM %s.spec_conversion_groups WHERE id=$1 FOR UPDATE`, r.schema), g.ID).Scan(&previous.ID, &previous.Name, &previous.BaseUnit, &previous.Revision, &previous.Active, &oldRaw)
		if err != nil {
			return g, fmt.Errorf("换算组不存在")
		}
		if g.Revision != previous.Revision {
			return g, salesapp.NewOrderEditConflictError("换算组已更新，请刷新后重新保存")
		}
		_ = json.Unmarshal(oldRaw, &previous.Members)
		b, _ := json.Marshal(previous)
		old = string(b)
		action = "update"
		if previous.Active && !g.Active {
			action = "disable"
		}
		err = tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.spec_conversion_groups SET name=$2,base_unit=$3,members_json=$4,active=$5,revision=revision+1,updated_at=now(),updated_by=$6 WHERE id=$1 RETURNING revision`, r.schema), g.ID, g.Name, g.BaseUnit, raw, g.Active, c.Actor).Scan(&g.Revision)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.spec_conversion_groups(name,base_unit,members_json,active,created_by,updated_by) VALUES($1,$2,$3,$4,$5,$5) RETURNING id,revision`, r.schema), g.Name, g.BaseUnit, raw, g.Active, c.Actor).Scan(&g.ID, &g.Revision)
	}
	if err != nil {
		return g, fmt.Errorf("无法保存换算组，请检查名称是否重复: %w", err)
	}
	b, _ := json.Marshal(g)
	if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, c.Actor, "spec_conversion_group", &g.ID, action, postgresinfra.StrPtr("configuration"), postgresinfra.StrPtr(old), postgresinfra.StrPtr(string(b)), postgresinfra.AuditMeta{"group_id": g.ID, "revision": g.Revision}); err != nil {
		return g, err
	}
	return g, tx.Commit(ctx)
}

// Explicit source rows are already priced per actual BOM-spec inventory unit.
// Do not call legacy weight/label inference on this path.
func conversionSourceTier(raw []byte, listType string, c salesapp.SpecConversionQuoteRequest, unit string) (string, string, error) {
	var content struct {
		Rows []struct {
			ProductID     int64       `json:"product_id"`
			ParentID      int64       `json:"parent_product_id"`
			SpecID        int64       `json:"bom_spec_id"`
			VariantID     int64       `json:"bom_variant_id"`
			Basis         string      `json:"quantity_basis"`
			Price         json.Number `json:"final_unit_price"`
			PriceUnit     string      `json:"price_unit"`
			InventoryUnit string      `json:"inventory_unit"`
			TierLabel     string      `json:"tier_label"`
			Label         string      `json:"label"`
		} `json:"price_rows"`
	}
	if json.Unmarshal(raw, &content) != nil {
		return "", "", fmt.Errorf("来源价格快照无效")
	}
	for i, row := range content.Rows {
		productID := row.ProductID
		if row.ParentID > 0 {
			productID = row.ParentID
		}
		if productID != c.ProductID || row.SpecID != c.SourceSpecID || row.VariantID != c.SourceVariantID {
			continue
		}
		if c.PriceRowKey != orderbeans.PublishedPriceRowKey(c.PublicationID, listType, productID, row.SpecID, row.VariantID, "flat", i) {
			continue
		}
		if row.Basis != "sales_spec_count" || !strings.EqualFold(row.InventoryUnit, unit) || !strings.EqualFold(row.PriceUnit, unit) {
			return "", "", fmt.Errorf("来源档位必须明确按规格库存单位计价，请补齐价格单位关系")
		}
		if _, err := conversion.Convert(string(row.Price), "1", "1"); err != nil {
			return "", "", err
		}
		return string(row.Price), firstNonEmpty(row.TierLabel, row.Label), nil
	}
	return "", "", fmt.Errorf("所选价格档已失效或不属于当前来源商品规格")
}
func conversionPublicationTx(ctx context.Context, tx pgx.Tx, schema string, customerID, productID, publicationID int64) (raw []byte, listType, version string, err error) {
	if customerID <= 0 || productID <= 0 || publicationID <= 0 {
		return nil, "", "", fmt.Errorf("请选择客户、商品和已发布价格表")
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT content_json,list_type,version_no FROM %s.bean_list_publications WHERE id=$1 AND status='published' AND publication_purpose='factory_supply' AND (owner_type='official' OR (owner_type='customer' AND owner_key=$2)) FOR SHARE`, schema), publicationID, strconv.FormatInt(customerID, 10)).Scan(&raw, &listType, &version)
	if err != nil {
		return nil, "", "", fmt.Errorf("来源价格表已失效或不属于当前客户")
	}
	var allowed bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.products WHERE id=$1 AND active AND (COALESCE(customer_id,0)=0 OR customer_id=$2))`, schema), productID, customerID).Scan(&allowed)
	if err != nil {
		return
	}
	if !allowed {
		err = fmt.Errorf("商品不属于当前客户可用范围")
		return
	}
	var content struct {
		Rows []struct {
			ProductID int64 `json:"product_id"`
			ParentID  int64 `json:"parent_product_id"`
		} `json:"price_rows"`
	}
	_ = json.Unmarshal(raw, &content)
	for _, row := range content.Rows {
		pid := row.ProductID
		if row.ParentID > 0 {
			pid = row.ParentID
		}
		if pid == productID {
			return
		}
	}
	err = fmt.Errorf("当前价格表未包含该商品")
	return
}

// The source identity belongs to the published price row. Its BOM version may
// predate the current default version; only the fulfillment target must be current.
func conversionPublishedSourcesTx(ctx context.Context, tx pgx.Tx, schema string, raw []byte, listType string, productID, publicationID int64) ([]salesapp.SpecConversionSourceTier, error) {
	var content struct {
		Rows []struct {
			ProductID int64       `json:"product_id"`
			ParentID  int64       `json:"parent_product_id"`
			SpecID    int64       `json:"bom_spec_id"`
			VariantID int64       `json:"bom_variant_id"`
			Price     json.Number `json:"final_unit_price"`
			Label     string      `json:"tier_label"`
			MinQty    json.Number `json:"min_qty"`
		} `json:"price_rows"`
	}
	if err := json.Unmarshal(raw, &content); err != nil {
		return nil, err
	}
	result := []salesapp.SpecConversionSourceTier{}
	for index, row := range content.Rows {
		pid := row.ProductID
		if row.ParentID > 0 {
			pid = row.ParentID
		}
		if pid != productID || row.SpecID <= 0 || row.VariantID <= 0 {
			continue
		}
		var source salesapp.SpecConversionSourceTier
		source.ProductID = productID
		source.SpecID = row.SpecID
		source.VariantID = row.VariantID
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(v.spec_name_snapshot,''),s.name),v.inventory_unit,COALESCE(s.source_spec_template_id,0),COALESCE(s.source_spec_template_key,'') FROM %[1]s.production_bom_specs s JOIN %[1]s.production_bom_version_variants v ON v.bom_spec_id=s.id JOIN %[1]s.production_bom_versions bv ON bv.id=v.version_id AND bv.bom_id=s.bom_id WHERE s.id=$1 AND v.id=$2`, schema), row.SpecID, row.VariantID).Scan(&source.Name, &source.InventoryUnit, &source.TemplateID, &source.TemplateKey)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		source.PriceRowKey = orderbeans.PublishedPriceRowKey(publicationID, listType, productID, row.SpecID, row.VariantID, "flat", index)
		source.UnitPrice = string(row.Price)
		source.TierLabel = row.Label
		if source.TierLabel == "" {
			source.TierLabel = string(row.MinQty) + "+"
		}
		result = append(result, source)
	}
	return result, nil
}
func (r Repository) OrderSpecConversions(ctx context.Context, customerID, productID, publicationID int64) (salesapp.SpecConversionCatalog, error) {
	out := salesapp.SpecConversionCatalog{Groups: []conversion.Group{}, Specs: []salesapp.SpecConversionSpec{}}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	raw, listType, _, err := conversionPublicationTx(ctx, tx, r.schema, customerID, productID, publicationID)
	if err != nil {
		return out, err
	}
	if out.Specs, err = conversionSpecsTx(ctx, tx, r.schema, productID); err != nil {
		return out, err
	}
	out.SourceTiers, err = conversionPublishedSourcesTx(ctx, tx, r.schema, raw, listType, productID, publicationID)
	if err != nil {
		return out, err
	}
	groups, err := conversionGroupsTx(ctx, tx, r.schema)
	if err != nil {
		return out, err
	}
	for _, g := range groups {
		if !g.Active {
			continue
		}
		for _, target := range out.Specs {
			if _, matchErr := g.Match(target.Identity()); matchErr == nil {
				out.Groups = append(out.Groups, g)
				break
			}
		}
	}
	return out, nil
}
func (r Repository) QuoteSpecConversion(ctx context.Context, c salesapp.SpecConversionQuoteRequest) (salesapp.SpecConversionQuote, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.SpecConversionQuote{}, err
	}
	defer tx.Rollback(ctx)
	return quoteSpecConversionTx(ctx, tx, r.schema, c)
}
func quoteSpecConversionTx(ctx context.Context, tx pgx.Tx, schema string, c salesapp.SpecConversionQuoteRequest) (salesapp.SpecConversionQuote, error) {
	q := salesapp.SpecConversionQuote{SpecConversionQuoteRequest: c}
	if c.Quantity <= 0 || c.Quantity > 1000000000 || c.GroupID <= 0 || c.Revision <= 0 || c.SourceSpecID <= 0 || c.SourceVariantID <= 0 || c.TargetSpecID <= 0 || c.TargetVariantID <= 0 {
		return q, fmt.Errorf("请完整选择来源规格、档位、换算组、目标规格和有效数量")
	}
	raw, listType, version, err := conversionPublicationTx(ctx, tx, schema, c.CustomerID, c.ProductID, c.PublicationID)
	if err != nil {
		return q, err
	}
	specs, err := conversionSpecsTx(ctx, tx, schema, c.ProductID)
	if err != nil {
		return q, err
	}
	var source, target *salesapp.SpecConversionSpec
	for i := range specs {
		s := &specs[i]
		if s.SpecID == c.TargetSpecID && s.VariantID == c.TargetVariantID {
			target = s
		}
	}
	sources, err := conversionPublishedSourcesTx(ctx, tx, schema, raw, listType, c.ProductID, c.PublicationID)
	if err != nil {
		return q, err
	}
	for i := range sources {
		if sources[i].SpecID == c.SourceSpecID && sources[i].VariantID == c.SourceVariantID && sources[i].PriceRowKey == c.PriceRowKey {
			source = &sources[i].SpecConversionSpec
			break
		}
	}
	if source == nil || target == nil {
		return q, fmt.Errorf("来源规格须属于当前价格表，目标规格须属于同一商品的当前默认已发布 BOM")
	}
	var g conversion.Group
	var members []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,name,base_unit,revision,active,members_json FROM %s.spec_conversion_groups WHERE id=$1 FOR SHARE`, schema), c.GroupID).Scan(&g.ID, &g.Name, &g.BaseUnit, &g.Revision, &g.Active, &members)
	if err != nil || !g.Active {
		return q, fmt.Errorf("换算组已停用或不存在")
	}
	if g.Revision != c.Revision {
		return q, salesapp.NewOrderEditConflictError("换算组已更新，请重新报价后提交")
	}
	if err = json.Unmarshal(members, &g.Members); err != nil {
		return q, err
	}
	sm, err := g.Match(source.Identity())
	if err != nil {
		return q, err
	}
	tm, err := g.Match(target.Identity())
	if err != nil {
		return q, err
	}
	price, label, err := conversionSourceTier(raw, listType, c, source.InventoryUnit)
	if err != nil {
		return q, err
	}
	converted, err := conversion.Convert(price, sm.BaseQuantity, tm.BaseQuantity)
	if err != nil {
		return q, err
	}
	q.GroupName = g.Name
	q.BaseUnit = g.BaseUnit
	q.SourceQuantity = sm.BaseQuantity
	q.TargetQuantity = tm.BaseQuantity
	q.SourceUnitPrice = price
	q.UnitPrice = converted
	q.VersionNo = version
	q.TierLabel = label
	q.SourceSpecName = source.Name
	q.TargetSpecName = target.Name
	q.SourceUnit = source.InventoryUnit
	q.TargetUnit = target.InventoryUnit
	q.Rounding = "half_up_2"
	q.Reprice = false
	amount, _ := strconv.ParseFloat(converted, 64)
	q.LineTotal, err = conversion.Convert(converted, "1", strconv.FormatInt(c.Quantity, 10))
	if err != nil {
		return q, err
	}
	snapshot := map[string]any{"source": "converted_tier", "price_selection_mode": "converted_tier", "selected_price_row_key": c.PriceRowKey, "publication_id": c.PublicationID, "version_no": version, "list_type": listType, "quantity_basis": "sales_spec_count", "inventory_unit": target.InventoryUnit, "price_unit": target.InventoryUnit, "display_unit": target.InventoryUnit, "final_unit_price": amount, "published_price_snapshot": true, "spec_conversion": q}
	data, _ := json.Marshal(snapshot)
	q.PriceSourceJSON = withOrderBOMSpecPriceSourceJSON(string(data), orderBOMSpecIdentity{ProductID: c.ProductID, BomSpecID: target.SpecID, BomVariantID: target.VariantID, BomSpecName: target.Name, InventoryUnit: target.InventoryUnit})
	return q, nil
}

// Only the original database row can authorize frozen pricing. Client supplied
// snapshots and item IDs from other orders cannot create or change a quote.
func frozenSpecConversionTx(ctx context.Context, tx pgx.Tx, schema string, orderID, itemID int64, c salesapp.SpecConversionQuoteRequest) (salesapp.SpecConversionQuote, bool, error) {
	var q salesapp.SpecConversionQuote
	if orderID <= 0 || itemID <= 0 || c.Reprice {
		return q, false, nil
	}
	var productID, specID, variantID, publicationID int64
	var raw, price string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,COALESCE(bom_spec_id,0),COALESCE(bom_variant_id,0),COALESCE(bean_list_publication_id,0),unit_price::text,price_source_json::text FROM %s.order_items WHERE order_id=$1 AND id=$2 FOR SHARE`, schema), orderID, itemID).Scan(&productID, &specID, &variantID, &publicationID, &price, &raw)
	if err != nil {
		return q, false, fmt.Errorf("原订单明细不存在，请重新打开订单")
	}
	var snapshot struct {
		Mode  string                        `json:"price_selection_mode"`
		Quote *salesapp.SpecConversionQuote `json:"spec_conversion"`
	}
	if json.Unmarshal([]byte(raw), &snapshot) != nil || snapshot.Mode != "converted_tier" || snapshot.Quote == nil {
		return q, false, nil
	}
	q = *snapshot.Quote
	if productID != c.ProductID || specID != c.TargetSpecID || variantID != c.TargetVariantID || publicationID != c.PublicationID || !sameSpecConversionSelection(q.SpecConversionQuoteRequest, c) {
		return q, false, nil
	}
	q.Quantity = c.Quantity
	q.UnitPrice = price
	q.PriceSourceJSON = raw
	q.LineTotal, err = conversion.Convert(price, "1", strconv.FormatInt(c.Quantity, 10))
	if err != nil {
		return q, false, err
	}
	return q, true, nil
}
func sameSpecConversionSelection(old, next salesapp.SpecConversionQuoteRequest) bool {
	return old.CustomerID == next.CustomerID && old.ProductID == next.ProductID && old.PublicationID == next.PublicationID && old.PriceRowKey == next.PriceRowKey && old.TargetSpecID == next.TargetSpecID && old.TargetVariantID == next.TargetVariantID && old.GroupID == next.GroupID && old.Revision == next.Revision && old.SourceSpecID == next.SourceSpecID && old.SourceVariantID == next.SourceVariantID
}
