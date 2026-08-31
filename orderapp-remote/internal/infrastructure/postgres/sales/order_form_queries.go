package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const orderProductCatalogPublicationTypeIDBase int64 = 8_000_000_000_000_000

func (r Repository) OrderForm(ctx context.Context, editID int64) (salesapp.OrderFormData, error) {
	data := salesapp.OrderFormData{Today: time.Now().Format("2006-01-02")}
	var err error
	if data.Customers, err = r.fetchOrderCustomers(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.Sources, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.ShipStatuses, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.PayStatuses, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.OrderTypes, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.Products, err = r.fetchOrderProducts(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.ProductBOMSpecOptions, err = r.fetchOrderBOMSpecOptions(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.Employees, err = r.fetchOrderEmployees(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.LogisticsCompanies, err = r.ListLogisticsCompanies(ctx, false); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.BeanListVersionOptions, err = r.fetchOrderBeanListVersionOptions(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.CustomerPublicUsages, err = r.fetchOrderCustomerPublicUsages(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.CustomerProductUsages, err = r.fetchOrderCustomerProductUsages(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if editID > 0 {
		editData, err := r.fetchOrderEdit(ctx, editID)
		if err != nil {
			return salesapp.OrderFormData{}, err
		}
		data.EditData = editData
	}
	return data, nil
}

func (r Repository) fetchOrderBOMSpecOptions(ctx context.Context) ([]salesapp.ProductBOMSpecOption, error) {
	for _, relation := range []string{
		"product_bom_spec_migrations",
		"legacy_child_sku_bom_spec_mappings",
		"production_bom_output_bindings",
		"production_bom_versions",
		"production_bom_specs",
		"production_bom_version_variants",
	} {
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.%s", r.schema, relation)).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return []salesapp.ProductBOMSpecOption{}, nil
		}
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT binding.output_id,
		       COALESCE(mapping.legacy_child_product_id,0),
		       binding.bom_id,
		       version.id,
		       COALESCE(version.version_no,''),
		       spec.id,
		       variant.id,
		       COALESCE(spec.code,''),
		       COALESCE(spec.barcode,''),
		       spec.spec_key,
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit),
		       migration.state,
		       variant.is_default,
		       variant.sort_order,
		       CASE WHEN COALESCE(NULLIF(to_jsonb(migration)->>'spec_identity_mode',''),CASE WHEN migration.state='cutover' OR COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false THEN 'bom_spec' ELSE 'legacy_sku' END)='bom_spec' THEN binding.output_id ELSE COALESCE(mapping.legacy_child_product_id,0) END,
		       CASE WHEN COALESCE(NULLIF(to_jsonb(migration)->>'spec_identity_mode',''),CASE WHEN migration.state='cutover' OR COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false THEN 'bom_spec' ELSE 'legacy_sku' END)='bom_spec' THEN spec.id ELSE 0 END,
		       CASE WHEN COALESCE(NULLIF(to_jsonb(migration)->>'spec_identity_mode',''),CASE WHEN migration.state='cutover' OR COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false THEN 'bom_spec' ELSE 'legacy_sku' END)='bom_spec' THEN variant.id ELSE 0 END,
		       COALESCE(legacy.customer_id,0),
		       COALESCE(NULLIF(legacy.product_kind,''),NULLIF(parent.product_kind,''),'roasted_bean')
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.product_bom_spec_migrations migration
		  ON migration.product_id=binding.output_id
		JOIN %[1]s.products parent
		  ON parent.id=binding.output_id AND parent.active=true
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id
		 AND version.bom_id=binding.bom_id
		 AND version.status='published'
		JOIN %[1]s.production_bom_specs spec
		  ON spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.version_id=version.id
		 AND variant.bom_spec_id=spec.id
		LEFT JOIN LATERAL (
			SELECT candidate.legacy_child_product_id
			FROM %[1]s.legacy_child_sku_bom_spec_mappings candidate
			WHERE candidate.parent_product_id=binding.output_id
			  AND candidate.bom_spec_id=spec.id
			ORDER BY candidate.legacy_child_product_id
			LIMIT 1
		) mapping ON true
		LEFT JOIN %[1]s.products legacy
		  ON legacy.id=mapping.legacy_child_product_id
		WHERE COALESCE(NULLIF(to_jsonb(migration)->>'spec_identity_mode',''),CASE WHEN migration.state='cutover' OR COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false THEN 'bom_spec' ELSE 'legacy_sku' END)='bom_spec'
		  AND binding.output_type='product'
		  AND binding.is_default=true
		ORDER BY parent.name,variant.sort_order,spec.spec_key,spec.id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := make([]salesapp.ProductBOMSpecOption, 0)
	legacyProducts := make([]salesapp.ProductOption, 0)
	for rows.Next() {
		var option salesapp.ProductBOMSpecOption
		var customerID int64
		var productKind string
		if err := rows.Scan(
			&option.ParentProductID,
			&option.LegacyChildProductID,
			&option.BomID,
			&option.BomVersionID,
			&option.BomVersionNo,
			&option.BomSpecID,
			&option.BomVariantID,
			&option.SpecCode,
			&option.Barcode,
			&option.SpecKey,
			&option.SpecName,
			&option.InventoryUnit,
			&option.MigrationState,
			&option.IsDefault,
			&option.SortOrder,
			&option.WriteProductID,
			&option.WriteBomSpecID,
			&option.WriteBomVariantID,
			&customerID,
			&productKind,
		); err != nil {
			return nil, err
		}
		option.Published = true
		options = append(options, option)
		publicationProductID := orderBOMSpecPublicationLookupProductID(option)
		legacyProducts = append(legacyProducts, salesapp.ProductOption{
			ID:              publicationProductID,
			SKUID:           publicationProductID,
			ParentProductID: option.ParentProductID,
			CustomerID:      customerID,
			ProductKind:     productKind,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	commercial, err := r.fetchCommercialOrderPublicationTiers(ctx, legacyProducts)
	if err != nil {
		return nil, err
	}
	applyCommercialOrderPublicationTiers(legacyProducts, commercial)
	drip, err := r.fetchDripOrderPublicationTiers(ctx, legacyProducts)
	if err != nil {
		return nil, err
	}
	applyHistoricalDripOrderPublicationTiers(legacyProducts, drip)
	retail, err := r.fetchRetailOrderPublicationTiers(ctx, legacyProducts)
	if err != nil {
		return nil, err
	}
	applyRetailOrderPublicationTiers(legacyProducts, retail)
	green, err := r.fetchGreenBeanOrderPublicationTiers(ctx, legacyProducts)
	if err != nil {
		return nil, err
	}
	applyGreenBeanOrderPublicationTiers(legacyProducts, green)
	for idx := range options {
		if idx >= len(legacyProducts) {
			break
		}
		options[idx].Tiers = orderBOMSpecPublicationTiers(options[idx], legacyProducts[idx].Tiers)
		for tierIdx := range options[idx].Tiers {
			tier := &options[idx].Tiers[tierIdx]
			tier.ParentProductID = options[idx].ParentProductID
			tier.BomSpecID = options[idx].BomSpecID
			tier.BomVariantID = options[idx].BomVariantID
			if tier.EffectiveSalesSpec == nil {
				tier.EffectiveSalesSpec = map[string]any{}
			}
			tier.EffectiveSalesSpec["parent_product_id"] = options[idx].ParentProductID
			tier.EffectiveSalesSpec["bom_spec_id"] = options[idx].BomSpecID
			tier.EffectiveSalesSpec["bom_variant_id"] = options[idx].BomVariantID
			if options[idx].MigrationState == "cutover" {
				tier.PriceSourceJSON = withOrderBOMSpecPriceSourceJSON(tier.PriceSourceJSON, orderBOMSpecIdentity{
					ProductID:              options[idx].ParentProductID,
					BomSpecID:              options[idx].BomSpecID,
					BomVariantID:           options[idx].BomVariantID,
					BomSpecKey:             options[idx].SpecKey,
					BomSpecName:            options[idx].SpecName,
					InventoryUnit:          options[idx].InventoryUnit,
					LegacyPricingProductID: options[idx].LegacyChildProductID,
				})
			}
		}
	}
	return options, nil
}

func orderBOMSpecPublicationLookupProductID(option salesapp.ProductBOMSpecOption) int64 {
	if strings.TrimSpace(option.MigrationState) == "cutover" && option.ParentProductID > 0 {
		return option.ParentProductID
	}
	return option.LegacyChildProductID
}

func orderBOMSpecPublicationTiers(option salesapp.ProductBOMSpecOption, tiers []salesapp.ProductTierOption) []salesapp.ProductTierOption {
	if strings.TrimSpace(option.MigrationState) != "cutover" {
		return append([]salesapp.ProductTierOption(nil), tiers...)
	}
	out := make([]salesapp.ProductTierOption, 0, len(tiers))
	for _, tier := range tiers {
		tierSpecID := orderFamilyTierMapInt64(tier.EffectiveSalesSpec, "bom_spec_id")
		if tierSpecID <= 0 || tierSpecID != option.BomSpecID {
			continue
		}
		tierVariantID := orderFamilyTierMapInt64(tier.EffectiveSalesSpec, "bom_variant_id")
		if option.BomVariantID > 0 && tierVariantID != option.BomVariantID {
			continue
		}
		out = append(out, tier)
	}
	return out
}

func orderEditItemsQuery(schema string) string {
	return fmt.Sprintf(`
			SELECT oi.id, oi.line_no,
				COALESCE(oi.product_id,0),
				COALESCE(oi.bom_spec_id,0),
				COALESCE(oi.bom_variant_id,0),
				CASE WHEN COALESCE(oi.bom_spec_id,0)>0 THEN COALESCE(oi.price_source_json->>'bom_spec_key','') ELSE '' END,
				CASE WHEN COALESCE(oi.bom_spec_id,0)>0 THEN COALESCE(NULLIF(oi.price_source_json->>'bom_spec_name',''),oi.spec,'') ELSE '' END,
				COALESCE(NULLIF(oi.customer_product_display_name_snapshot,''), NULLIF(oi.item_name,''), p.name, ''),
				COALESCE(oi.customer_product_alias_id,0),
				COALESCE(oi.customer_product_display_name_snapshot,''),
				COALESCE(oi.customer_item_code_snapshot,''),
				COALESCE(oi.brand_name_snapshot,''),
				COALESCE(oi.product_code_snapshot,''),
				COALESCE(oi.product_name_snapshot,''),
				COALESCE(oi.item_note,''),
				COALESCE(oi.spec,''),
				COALESCE(oi.qty,0),
				COALESCE(oi.unit,''),
				COALESCE(oi.unit_price,0),
				COALESCE(oi.line_total,0),
				COALESCE(oi.price_tier_id,0),
				COALESCE(oi.price_overridden,false),
				COALESCE(oi.bean_list_publication_id,0),
				COALESCE(oi.bean_list_version_no,''),
				COALESCE(oi.discount_type,''),
				COALESCE(oi.discount_value,0),
				COALESCE(oi.discount_amount,0),
				COALESCE(NULLIF(oi.product_kind,''), NULLIF(p.product_kind,''), 'roasted'),
				COALESCE(oi.sales_unit,''),
				COALESCE(oi.unit_bag_count,0),
				COALESCE(oi.unit_bean_g,0)::float8,
				COALESCE(oi.matched_price_qty,0)::float8,
				COALESCE(oi.price_source_json,'{}'::jsonb)::text
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id=$1
		ORDER BY oi.line_no, oi.id
	`, schema, schema)
}

func (r Repository) fetchOrderBeanListVersionOptions(ctx context.Context) ([]salesapp.BeanListVersionOption, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.bean_list_publications", r.schema)).Scan(&exists); err != nil || !exists {
		return nil, err
	}
	q := fmt.Sprintf(`
		WITH active_customers AS (
			SELECT id FROM %[1]s.customers WHERE active=true
		),
		customer_publications AS (
			SELECT c.id AS customer_id,
			       b.list_type,
			       COALESCE(b.product_type_category_id,0) AS product_type_category_id,
			       COALESCE(b.product_type_name,'') AS product_type_name,
			       COALESCE(NULLIF(b.classification_template_id,0), NULLIF(b.product_type_category_id,0), 0) AS classification_template_id,
			       COALESCE(NULLIF(BTRIM(b.classification_template_name),''), NULLIF(BTRIM(b.product_type_name),''), '') AS classification_template_name,
			       b.id,
			       b.version_no,
			       COALESCE(to_char(b.published_at, 'YYYY-MM-DD HH24:MI'), '') AS published_at,
			       COALESCE(b.changelog, '') AS changelog,
			       b.published_at AS published_at_sort,
			       CASE
			         WHEN COALESCE(NULLIF(b.classification_template_id,0), NULLIF(b.product_type_category_id,0), 0) > 0
			           THEN 'classification:' || COALESCE(NULLIF(b.classification_template_id,0), NULLIF(b.product_type_category_id,0), 0)::text
			         ELSE 'legacy:' || b.list_type
			       END AS price_list_group_key
			FROM active_customers c
			JOIN %[1]s.bean_list_publications b
			  ON b.owner_type='customer' AND b.owner_key=c.id::text AND b.status='published'
			 AND b.publication_purpose='factory_supply'
			WHERE b.list_type IN ('commercial','retail','green','drip')
			   OR COALESCE(b.classification_template_id,0)>0
			   OR COALESCE(b.product_type_category_id,0)>0
		),
		customer_ranked AS (
			SELECT b.*,
			       row_number() OVER (PARTITION BY b.customer_id, b.list_type, b.price_list_group_key ORDER BY b.published_at_sort DESC NULLS LAST, b.id DESC) AS group_rank,
			       bool_or(b.classification_template_id > 0) OVER (PARTITION BY b.customer_id, b.list_type) AS has_classified_publication
			FROM customer_publications b
		),
		customer_versions AS (
			SELECT b.customer_id,
			       b.list_type,
			       b.product_type_category_id,
			       b.product_type_name,
			       b.classification_template_id,
			       b.classification_template_name,
			       b.id,
			       b.version_no,
			       b.published_at,
			       b.changelog,
			       true AS is_customer_owned,
			       (b.group_rank=1 AND (b.classification_template_id > 0 OR NOT b.has_classified_publication)) AS is_default,
			       b.price_list_group_key,
			       b.has_classified_publication
			FROM customer_ranked b
		),
		official_publications AS (
			SELECT b.list_type,
			       COALESCE(b.product_type_category_id,0) AS product_type_category_id,
			       COALESCE(b.product_type_name,'') AS product_type_name,
			       COALESCE(NULLIF(b.classification_template_id,0), NULLIF(b.product_type_category_id,0), 0) AS classification_template_id,
			       COALESCE(NULLIF(BTRIM(b.classification_template_name),''), NULLIF(BTRIM(b.product_type_name),''), '') AS classification_template_name,
			       b.id,
			       b.version_no,
			       COALESCE(to_char(b.published_at, 'YYYY-MM-DD HH24:MI'), '') AS published_at,
			       COALESCE(b.changelog, '') AS changelog,
			       b.published_at AS published_at_sort,
			       CASE
			         WHEN COALESCE(NULLIF(b.classification_template_id,0), NULLIF(b.product_type_category_id,0), 0) > 0
			           THEN 'classification:' || COALESCE(NULLIF(b.classification_template_id,0), NULLIF(b.product_type_category_id,0), 0)::text
			         ELSE 'legacy:' || b.list_type
			       END AS price_list_group_key
			FROM %[1]s.bean_list_publications b
			WHERE b.owner_type='official' AND b.status='published'
			  AND b.publication_purpose='factory_supply'
			  AND (b.list_type IN ('commercial','retail','green','drip')
			       OR COALESCE(b.classification_template_id,0)>0
			       OR COALESCE(b.product_type_category_id,0)>0)
		),
		official_ranked AS (
			SELECT b.*,
			       row_number() OVER (PARTITION BY b.list_type, b.price_list_group_key ORDER BY b.published_at_sort DESC NULLS LAST, b.id DESC) AS group_rank,
			       bool_or(b.classification_template_id > 0) OVER (PARTITION BY b.list_type) AS has_classified_publication
			FROM official_publications b
		),
		official_versions AS (
			SELECT b.list_type,
			       b.product_type_category_id,
			       b.product_type_name,
			       b.classification_template_id,
			       b.classification_template_name,
			       b.id,
			       b.version_no,
			       b.published_at,
			       b.changelog,
			       (b.group_rank=1 AND (b.classification_template_id > 0 OR NOT b.has_classified_publication)) AS is_default,
			       b.price_list_group_key
			FROM official_ranked b
		),
		global_public_versions AS (
			SELECT 0::bigint AS customer_id,
			       o.list_type,
			       o.product_type_category_id,
			       o.product_type_name,
			       o.classification_template_id,
			       o.classification_template_name,
			       o.id,
			       o.version_no,
			       o.published_at,
			       o.changelog,
			       false AS is_customer_owned,
			       o.is_default,
			       o.price_list_group_key
			FROM official_versions o
		),
		public_fallback AS (
			SELECT c.id AS customer_id,
			       o.list_type,
			       o.product_type_category_id,
			       o.product_type_name,
			       o.classification_template_id,
			       o.classification_template_name,
			       o.id,
			       o.version_no,
			       o.published_at,
			       o.changelog,
			       false AS is_customer_owned,
			       o.is_default,
			       o.price_list_group_key
			FROM active_customers c
			CROSS JOIN official_versions o
			WHERE NOT EXISTS (
				SELECT 1 FROM customer_versions cv
				WHERE cv.customer_id=c.id
				  AND cv.list_type=o.list_type
				  AND (cv.price_list_group_key=o.price_list_group_key
				       OR (cv.classification_template_id=0 AND NOT cv.has_classified_publication))
			)
		)
		SELECT customer_id, list_type, product_type_category_id, product_type_name, classification_template_id, classification_template_name, id, version_no, published_at, changelog, is_customer_owned, is_default
		FROM customer_versions
		UNION ALL
		SELECT customer_id, list_type, product_type_category_id, product_type_name, classification_template_id, classification_template_name, id, version_no, published_at, changelog, is_customer_owned, is_default
		FROM global_public_versions
		UNION ALL
		SELECT customer_id, list_type, product_type_category_id, product_type_name, classification_template_id, classification_template_name, id, version_no, published_at, changelog, is_customer_owned, is_default
		FROM public_fallback
		ORDER BY customer_id, list_type, classification_template_id, is_customer_owned DESC, is_default DESC, published_at DESC, id DESC
	`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.BeanListVersionOption, 0)
	for rows.Next() {
		var row salesapp.BeanListVersionOption
		if err := rows.Scan(&row.CustomerID, &row.ListType, &row.ProductTypeCategoryID, &row.ProductTypeName, &row.ClassificationTemplateID, &row.ClassificationTemplateName, &row.ID, &row.VersionNo, &row.PublishedAt, &row.Changelog, &row.IsCustomerOwned, &row.IsDefault); err != nil {
			return nil, err
		}
		ownerLabel := "公共豆单"
		if row.IsCustomerOwned {
			ownerLabel = "客户豆单"
		}
		row.Label = strings.TrimSpace(fmt.Sprintf("%s %s %s", ownerLabel, row.VersionNo, row.PublishedAt))
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	currentTypeIDs, err := r.fetchCurrentProductCatalogPublicationTypeIDs(ctx)
	if err != nil {
		return nil, err
	}
	return filterOrderBeanListVersionOptionsToCurrentProductCatalogTypes(out, currentTypeIDs), nil
}

func (r Repository) fetchCurrentProductCatalogPublicationTypeIDs(ctx context.Context) ([]int64, error) {
	for _, relation := range []string{"business_group_usages", "business_groups"} {
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.%s", r.schema, relation)).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, nil
		}
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT $1::bigint + u.group_id
		FROM %[1]s.business_group_usages u
		JOIN %[1]s.business_groups bg ON bg.id=u.group_id AND bg.active=true
		WHERE u.active=true
		  AND lower(u.usage_key)='product_catalog'
		  AND left(lower(btrim(bg.code)),8)<>'default_'
		  AND btrim(bg.name) NOT IN ('商品默认分组','生产 BOM 默认分组','仓库库存默认分组')
		ORDER BY u.sort_order,u.id
	`, r.schema), orderProductCatalogPublicationTypeIDBase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func filterOrderBeanListVersionOptionsToCurrentProductCatalogTypes(options []salesapp.BeanListVersionOption, currentTypeIDs []int64) []salesapp.BeanListVersionOption {
	if len(currentTypeIDs) == 0 {
		return options
	}
	allowed := make(map[int64]bool, len(currentTypeIDs))
	for _, id := range currentTypeIDs {
		if id > 0 {
			allowed[id] = true
		}
	}
	if len(allowed) == 0 {
		return options
	}
	out := make([]salesapp.BeanListVersionOption, 0, len(options))
	for _, option := range options {
		if allowed[option.ClassificationTemplateID] {
			out = append(out, option)
		}
	}
	return out
}

func (r Repository) fetchOrderCustomerPublicUsages(ctx context.Context) ([]salesapp.CustomerPublicUsageOption, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.customer_sku_public_usage", r.schema)).Scan(&exists); err != nil || !exists {
		return nil, err
	}
	q := fmt.Sprintf(`
		SELECT customer_id, use_public_sku
		FROM %s.customer_sku_public_usage
		ORDER BY customer_id
	`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.CustomerPublicUsageOption, 0)
	for rows.Next() {
		var row salesapp.CustomerPublicUsageOption
		if err := rows.Scan(&row.CustomerID, &row.UsePublicSKU); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderCustomers(ctx context.Context) ([]salesapp.CustomerOption, error) {
	q := fmt.Sprintf(`
		SELECT c.id, c.name, COALESCE(c.customer_type,''), COALESCE(c.contact,''), COALESCE(c.phone,''),
			COALESCE(c.address,''), COALESCE(c.company_name,''), COALESCE(c.company_address,''), COALESCE(c.company_phone,''),
			COALESCE(c.default_source_id,0), COALESCE(c.default_order_type_id,0),
			COALESCE(c.responsible_employee_id,0), COALESCE(e.name,'')
		FROM %s.customers c
		LEFT JOIN %s.company_employees e ON e.id=c.responsible_employee_id
		WHERE c.active=true
		ORDER BY c.name
	`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.CustomerOption, 0)
	for rows.Next() {
		var row salesapp.CustomerOption
		if err := rows.Scan(&row.ID, &row.Name, &row.CustomerType, &row.Contact, &row.Phone,
			&row.Address, &row.CompanyName, &row.CompanyAddress, &row.CompanyPhone,
			&row.DefaultSourceID, &row.DefaultOrderTypeID, &row.ResponsibleEmployeeID, &row.ResponsibleEmployeeName); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderEmployees(ctx context.Context) ([]salesapp.EmployeeOption, error) {
	q := fmt.Sprintf(`
		SELECT e.id, e.name, COALESCE(e.phone,''), COALESCE(e.department_id,0), COALESCE(d.name,'')
		FROM %s.company_employees e
		LEFT JOIN %s.company_departments d ON d.id=e.department_id
		WHERE e.active=true AND (e.account_type='internal_employee' OR COALESCE(e.account_type,'')='')
		ORDER BY e.id DESC
	`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.EmployeeOption, 0)
	for rows.Next() {
		var row salesapp.EmployeeOption
		if err := rows.Scan(&row.ID, &row.Name, &row.Phone, &row.DepartmentID, &row.Department); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderCustomerProductUsages(ctx context.Context) ([]salesapp.CustomerProductUsageOption, error) {
	q := fmt.Sprintf(`
		WITH usage_rows AS (
			SELECT
				o.customer_id,
				oi.product_id,
				COUNT(DISTINCT o.id) AS order_count,
				COUNT(*) AS item_count,
				MAX(o.order_date) AS last_order_date
			FROM %[1]s.orders o
			JOIN %[1]s.order_items oi ON oi.order_id=o.id
			WHERE COALESCE(o.is_void,false)=false
			  AND COALESCE(o.customer_id,0)>0
			  AND COALESCE(oi.product_id,0)>0
			GROUP BY o.customer_id, oi.product_id
		),
		ranked_usage AS (
			SELECT
				u.*,
				ROW_NUMBER() OVER (
					PARTITION BY u.customer_id
					ORDER BY u.order_count DESC, u.item_count DESC, u.last_order_date DESC, u.product_id
				) AS usage_rank
			FROM usage_rows u
		),
		latest_rows AS (
			SELECT DISTINCT ON (o.customer_id, oi.product_id)
				o.customer_id,
				oi.product_id,
				o.id AS last_order_id,
				o.order_no AS last_order_no,
				oi.item_name AS last_order_item,
				oi.spec AS last_order_spec,
				oi.qty::text AS last_order_units
			FROM %[1]s.orders o
			JOIN %[1]s.order_items oi ON oi.order_id=o.id
			WHERE COALESCE(o.is_void,false)=false
			  AND COALESCE(o.customer_id,0)>0
			  AND COALESCE(oi.product_id,0)>0
			ORDER BY o.customer_id, oi.product_id, o.order_date DESC, o.id DESC, oi.line_no
		)
		SELECT
			u.customer_id,
			u.product_id,
			u.order_count,
			u.item_count,
			COALESCE(to_char(u.last_order_date, 'YYYY-MM-DD'), '') AS last_order_date,
			COALESCE(l.last_order_id, 0) AS last_order_id,
			COALESCE(l.last_order_no, '') AS last_order_no,
			COALESCE(l.last_order_item, '') AS last_order_item,
			COALESCE(l.last_order_spec, '') AS last_order_spec,
			COALESCE(l.last_order_units, '') AS last_order_units
		FROM ranked_usage u
		LEFT JOIN latest_rows l ON l.customer_id=u.customer_id AND l.product_id=u.product_id
		WHERE u.usage_rank <= 50
		ORDER BY u.customer_id, u.order_count DESC, u.item_count DESC, u.last_order_date DESC, u.product_id
	`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.CustomerProductUsageOption, 0)
	for rows.Next() {
		var row salesapp.CustomerProductUsageOption
		if err := rows.Scan(&row.CustomerID, &row.ProductID, &row.OrderCount, &row.ItemCount, &row.LastOrderDate, &row.LastOrderID, &row.LastOrderNo, &row.LastOrderItem, &row.LastOrderSpec, &row.LastOrderUnits); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderProducts(ctx context.Context) ([]salesapp.ProductOption, error) {
	sqlstr := fmt.Sprintf(`SELECT p.id, p.name, COALESCE(p.roast_level,''), 0::numeric AS default_price,
		0::numeric AS retail_price_100g,
		0::numeric AS retail_price_200g,
		0::numeric AS retail_price_227g,
		0::numeric AS retail_price_250g,
		COALESCE(p.customer_id, 0),
		COALESCE(p.base_product_id, 0),
		COALESCE(NULLIF(p.visibility,''), 'public'),
		COALESCE(p.custom_type, ''),
		COALESCE(NULLIF(p.product_kind,''), 'roasted_bean'),
		COALESCE(p.drip_bag_grams, 0)::float8,
		COALESCE(p.drip_box_bag_count, 0),
		COALESCE(type_cat.id, 0) AS product_type_category_id,
		COALESCE(subtype_cat.id, 0) AS product_subtype_category_id,
		COALESCE(type_cat.name, '') AS product_type_name,
		COALESCE(subtype_cat.name, '') AS product_subtype_name,
		CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg') ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'inventory_unit',''), NULLIF(product_direct_unit_template.inventory_unit,''), NULLIF(subtype_cat.inventory_unit,''), NULLIF(type_cat.inventory_unit,''), 'kg') END AS inventory_unit,
		CASE WHEN COALESCE(p.auto_derived_sku,false) THEN COALESCE(NULLIF(p.derived_sales_unit,''), NULLIF(p.sku_name,''), NULLIF(parent_units.parent_inventory_unit,''), 'kg') ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'default_sales_unit',''), NULLIF(p.unit_rule_override_json->>'quote_unit',''), NULLIF(product_direct_unit_template.quote_unit,''), NULLIF(product_direct_unit_template.order_unit,''), NULLIF(product_direct_unit_template.inventory_unit,''), NULLIF(subtype_cat.quote_unit,''), NULLIF(type_cat.quote_unit,''), 'kg') END AS quote_unit,
		CASE WHEN COALESCE(p.auto_derived_sku,false) THEN COALESCE(NULLIF(p.derived_sales_unit,''), NULLIF(p.sku_name,''), NULLIF(parent_units.parent_inventory_unit,''), 'kg') ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'default_sales_unit',''), NULLIF(p.unit_rule_override_json->>'order_unit',''), NULLIF(product_direct_unit_template.order_unit,''), NULLIF(product_direct_unit_template.quote_unit,''), NULLIF(product_direct_unit_template.inventory_unit,''), NULLIF(subtype_cat.order_unit,''), NULLIF(type_cat.order_unit,''), 'kg') END AS order_unit,
		CASE WHEN COALESCE(p.auto_derived_sku,false) AND NULLIF(p.derived_sales_unit,'') IS NOT NULL THEN jsonb_build_object(p.derived_sales_unit, jsonb_build_object(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'), derived_sku_units.derived_sku_unit_factor))::text ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'unit_conversion_json',''), NULLIF(p.unit_rule_override_json->>'conversion_json',''), NULLIF(product_direct_unit_template.unit_conversion_json::text,'{}'), NULLIF(subtype_cat.unit_conversion_json::text,'{}'), NULLIF(type_cat.unit_conversion_json::text,'{}'), '{}') END AS unit_conversion_json,
		COALESCE(product_direct_unit_template.integer_unit, subtype_cat.integer_unit, type_cat.integer_unit, false) AS integer_unit,
		p.id AS sku_id,
		CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END AS effective_parent_product_id,
		COALESCE(NULLIF(parent_product.name,''), p.name, '') AS parent_product_name,
		COALESCE(NULLIF(p.sku_name,''), NULLIF(p.spec_label,''), CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.name ELSE '默认规格' END) AS sku_name,
		COALESCE(NULLIF(p.sku_code,''), 'SKU-' || p.id::text) AS sku_code,
		COALESCE(p.spec_label,'') AS spec_label,
		COALESCE(p.net_content_qty,0)::float8 AS net_content_qty,
		COALESCE(p.net_content_unit,'') AS net_content_unit,
		(p.id=CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(NULLIF(parent_product.default_sku_id,0),p.id) ELSE COALESCE(NULLIF(p.default_sku_id,0),p.id) END) AS is_default_sku,
		CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(NULLIF(parent_product.default_sku_id,0),p.id) ELSE COALESCE(NULLIF(p.default_sku_id,0),p.id) END AS default_sku_id
		FROM %[1]s.products p
		LEFT JOIN %[1]s.product_unit_templates product_direct_unit_template ON product_direct_unit_template.id=COALESCE(p.unit_template_id,0) AND product_direct_unit_template.active=true AND product_direct_unit_template.deleted_at IS NULL
		LEFT JOIN %[1]s.product_categories subtype_cat ON subtype_cat.id=p.product_category_id AND subtype_cat.active=true
		LEFT JOIN %[1]s.product_categories type_cat ON type_cat.id=subtype_cat.parent_id AND type_cat.active=true
		LEFT JOIN %[1]s.products parent_product ON parent_product.id=p.parent_product_id AND parent_product.active=true
		LEFT JOIN %[1]s.product_unit_templates parent_product_unit_template ON parent_product_unit_template.id=parent_product.unit_template_id AND parent_product_unit_template.active=true AND parent_product_unit_template.deleted_at IS NULL
		LEFT JOIN %[1]s.product_config_templates parent_product_config ON parent_product_config.id=parent_product.product_config_template_id AND parent_product_config.active=true
		LEFT JOIN %[1]s.product_categories parent_product_category ON parent_product_category.id=parent_product.product_category_id AND parent_product_category.active=true
		LEFT JOIN %[1]s.product_categories parent_product_parent_category ON parent_product_parent_category.id=parent_product_category.parent_id AND parent_product_parent_category.active=true
		LEFT JOIN LATERAL (
			SELECT COALESCE(
			           NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''),
			           NULLIF(parent_product_unit_template.inventory_unit,''),
			           NULLIF(parent_product_config.inventory_unit,''),
			           NULLIF(parent_product_category.inventory_unit,''),
			           NULLIF(parent_product_parent_category.inventory_unit,''),
			           'kg'
			       ) AS parent_inventory_unit
		) parent_units ON true
		LEFT JOIN LATERAL (
			SELECT CASE
			         WHEN COALESCE(p.net_content_qty,0) <= 0 THEN 1::float8
			         WHEN lower(COALESCE(NULLIF(p.net_content_unit,''), COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))) = lower(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))
			         THEN COALESCE(p.net_content_qty,0)::float8
			         WHEN lower(COALESCE(p.net_content_unit,''))='g' AND lower(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))='kg'
			         THEN COALESCE(p.net_content_qty,0)::float8 / 1000.0
			         WHEN lower(COALESCE(p.net_content_unit,''))='kg' AND lower(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))='g'
			         THEN COALESCE(p.net_content_qty,0)::float8 * 1000.0
			         WHEN COALESCE(p.net_content_unit,'')='g' AND COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg')='磅'
			         THEN COALESCE(p.net_content_qty,0)::float8 / 454.0
			         WHEN COALESCE(p.net_content_unit,'')='磅' AND COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg')='g'
			         THEN COALESCE(p.net_content_qty,0)::float8 * 454.0
			         WHEN lower(COALESCE(p.net_content_unit,''))='kg' AND COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg')='磅'
			         THEN COALESCE(p.net_content_qty,0)::float8 / 0.454
			         WHEN COALESCE(p.net_content_unit,'')='磅' AND lower(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))='kg'
			         THEN COALESCE(p.net_content_qty,0)::float8 * 0.454
			         ELSE 1::float8
			       END AS derived_sku_unit_factor
		) derived_sku_units ON true
		WHERE p.active=true
		  AND (COALESCE(p.parent_product_id,0)=0 OR parent_product.id IS NOT NULL)
		  AND (NOT COALESCE(p.auto_derived_sku,false) OR COALESCE(NULLIF(p.derived_spec_status,''),'active')<>'template_removed')
		ORDER BY p.name`, r.schema)
	rows, err := r.pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]salesapp.ProductOption, 0)
	for rows.Next() {
		var p salesapp.ProductOption
		if err := rows.Scan(&p.ID, &p.Name, &p.RoastLevel, &p.DefaultPrice, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.ProductKind, &p.DripBagGrams, &p.DripBoxBagCount, &p.ProductTypeCategoryID, &p.ProductSubtypeCategoryID, &p.ProductTypeName, &p.ProductSubtypeName, &p.InventoryUnit, &p.QuoteUnit, &p.OrderUnit, &p.UnitConversionJSON, &p.IntegerUnit, &p.SKUID, &p.ParentProductID, &p.ParentProductName, &p.SKUName, &p.SKUCode, &p.SpecLabel, &p.NetContentQty, &p.NetContentUnit, &p.IsDefaultSKU, &p.DefaultSKUID); err != nil {
			return nil, err
		}
		p.ProductCode = firstOrderSalesSpecText(p.SKUCode, fmt.Sprintf("SKU-%d", p.ID))
		p.ProductRecordName = firstOrderSalesSpecText(p.ParentProductName, p.Name)
		if p.ProductKind == "drip_bag" {
			p.SalesUnits = []string{"bag", "box"}
		}
		p.RetailSpecs = salesdomain.RetailAvailableSpecs(salesdomain.RetailSpecPrices{
			Price100G: p.RetailPrice100G,
			Price200G: p.RetailPrice200G,
			Price227G: p.RetailPrice227G,
			Price250G: p.RetailPrice250G,
		})
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if aliasProducts, err := r.fetchOrderCustomerAliasProducts(ctx); err != nil {
		return nil, err
	} else {
		out = append(out, aliasProducts...)
	}
	identities, err := postgresinfra.FetchProductSpecIdentities(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	for index := range out {
		parentID := out[index].ParentProductID
		if parentID <= 0 {
			parentID = out[index].ID
		}
		identity := identities[parentID]
		out[index].SpecIdentityMode = identity.SpecIdentityMode
		out[index].BomSpecAuthoritative = identity.BomSpecAuthoritative
	}

	commercialPublicationTiers, err := r.fetchCommercialOrderPublicationTiers(ctx, out)
	if err != nil {
		return nil, err
	}
	applyCommercialOrderPublicationTiers(out, commercialPublicationTiers)
	dripPublicationTiers, err := r.fetchDripOrderPublicationTiers(ctx, out)
	if err != nil {
		return nil, err
	}
	applyHistoricalDripOrderPublicationTiers(out, dripPublicationTiers)
	retailPublicationTiers, err := r.fetchRetailOrderPublicationTiers(ctx, out)
	if err != nil {
		return nil, err
	}
	applyRetailOrderPublicationTiers(out, retailPublicationTiers)
	greenPublicationTiers, err := r.fetchGreenBeanOrderPublicationTiers(ctx, out)
	if err != nil {
		return nil, err
	}
	applyGreenBeanOrderPublicationTiers(out, greenPublicationTiers)
	return out, nil
}

func (r Repository) fetchOrderCustomerAliasProducts(ctx context.Context) ([]salesapp.ProductOption, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.customer_product_aliases", r.schema)).Scan(&exists); err != nil || !exists {
		return nil, nil
	}
	sqlstr := fmt.Sprintf(`SELECT p.id,
		COALESCE(NULLIF(a.display_name,''), p.name, ''),
		COALESCE(p.roast_level,''),
		0::numeric AS default_price,
		0::numeric AS retail_price_100g,
		0::numeric AS retail_price_200g,
		0::numeric AS retail_price_227g,
		0::numeric AS retail_price_250g,
		a.customer_id,
		COALESCE(p.base_product_id, 0),
		'customer_alias',
		COALESCE(p.custom_type, ''),
		COALESCE(NULLIF(p.product_kind,''), 'roasted_bean'),
		COALESCE(p.drip_bag_grams, 0)::float8,
		COALESCE(p.drip_box_bag_count, 0),
		COALESCE(type_cat.id, 0) AS product_type_category_id,
		COALESCE(subtype_cat.id, 0) AS product_subtype_category_id,
		COALESCE(type_cat.name, '') AS product_type_name,
		COALESCE(subtype_cat.name, '') AS product_subtype_name,
		CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg') ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'inventory_unit',''), NULLIF(product_direct_unit_template.inventory_unit,''), NULLIF(subtype_cat.inventory_unit,''), NULLIF(type_cat.inventory_unit,''), 'kg') END AS inventory_unit,
		CASE WHEN COALESCE(p.auto_derived_sku,false) THEN COALESCE(NULLIF(p.derived_sales_unit,''), NULLIF(p.sku_name,''), NULLIF(parent_units.parent_inventory_unit,''), 'kg') ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'default_sales_unit',''), NULLIF(p.unit_rule_override_json->>'quote_unit',''), NULLIF(product_direct_unit_template.quote_unit,''), NULLIF(product_direct_unit_template.order_unit,''), NULLIF(product_direct_unit_template.inventory_unit,''), NULLIF(subtype_cat.quote_unit,''), NULLIF(type_cat.quote_unit,''), 'kg') END AS quote_unit,
		CASE WHEN COALESCE(p.auto_derived_sku,false) THEN COALESCE(NULLIF(p.derived_sales_unit,''), NULLIF(p.sku_name,''), NULLIF(parent_units.parent_inventory_unit,''), 'kg') ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'default_sales_unit',''), NULLIF(p.unit_rule_override_json->>'order_unit',''), NULLIF(product_direct_unit_template.order_unit,''), NULLIF(product_direct_unit_template.quote_unit,''), NULLIF(product_direct_unit_template.inventory_unit,''), NULLIF(subtype_cat.order_unit,''), NULLIF(type_cat.order_unit,''), 'kg') END AS order_unit,
		CASE WHEN COALESCE(p.auto_derived_sku,false) AND NULLIF(p.derived_sales_unit,'') IS NOT NULL THEN jsonb_build_object(p.derived_sales_unit, jsonb_build_object(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'), derived_sku_units.derived_sku_unit_factor))::text ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'unit_conversion_json',''), NULLIF(p.unit_rule_override_json->>'conversion_json',''), NULLIF(product_direct_unit_template.unit_conversion_json::text,'{}'), NULLIF(subtype_cat.unit_conversion_json::text,'{}'), NULLIF(type_cat.unit_conversion_json::text,'{}'), '{}') END AS unit_conversion_json,
		COALESCE(product_direct_unit_template.integer_unit, subtype_cat.integer_unit, type_cat.integer_unit, false) AS integer_unit,
		a.id,
		COALESCE(NULLIF(a.display_name,''), p.name, ''),
		COALESCE(a.customer_item_code,''),
		COALESCE(a.brand_name,''),
		COALESCE(NULLIF(parent_product.name,''),p.name,''),
		COALESCE(a.display_category_id,0),
		p.id AS sku_id,
		CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END AS effective_parent_product_id,
		COALESCE(NULLIF(parent_product.name,''), p.name, '') AS parent_product_name,
		COALESCE(NULLIF(p.sku_name,''), NULLIF(p.spec_label,''), CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.name ELSE '默认规格' END) AS sku_name,
		COALESCE(NULLIF(p.sku_code,''), 'SKU-' || p.id::text) AS sku_code,
		COALESCE(p.spec_label,'') AS spec_label,
		COALESCE(p.net_content_qty,0)::float8 AS net_content_qty,
		COALESCE(p.net_content_unit,'') AS net_content_unit,
		(p.id=CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(NULLIF(parent_product.default_sku_id,0),p.id) ELSE COALESCE(NULLIF(p.default_sku_id,0),p.id) END) AS is_default_sku,
		CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(NULLIF(parent_product.default_sku_id,0),p.id) ELSE COALESCE(NULLIF(p.default_sku_id,0),p.id) END AS default_sku_id
		FROM %[1]s.customer_product_aliases a
		JOIN %[1]s.products alias_product ON alias_product.id=a.product_id AND alias_product.active=true
		JOIN %[1]s.products p ON p.active=true AND (
			(COALESCE(alias_product.parent_product_id,0)>0 AND (p.id=alias_product.parent_product_id OR p.parent_product_id=alias_product.parent_product_id))
			OR
			(COALESCE(alias_product.parent_product_id,0)=0 AND (p.id=alias_product.id OR p.parent_product_id=alias_product.id))
		)
		LEFT JOIN %[1]s.product_unit_templates product_direct_unit_template ON product_direct_unit_template.id=COALESCE(p.unit_template_id,0) AND product_direct_unit_template.active=true AND product_direct_unit_template.deleted_at IS NULL
		LEFT JOIN %[1]s.product_categories subtype_cat ON subtype_cat.id=p.product_category_id AND subtype_cat.active=true
		LEFT JOIN %[1]s.product_categories type_cat ON type_cat.id=subtype_cat.parent_id AND type_cat.active=true
		LEFT JOIN %[1]s.products parent_product ON parent_product.id=p.parent_product_id AND parent_product.active=true
		LEFT JOIN %[1]s.product_unit_templates parent_product_unit_template ON parent_product_unit_template.id=parent_product.unit_template_id AND parent_product_unit_template.active=true AND parent_product_unit_template.deleted_at IS NULL
		LEFT JOIN %[1]s.product_config_templates parent_product_config ON parent_product_config.id=parent_product.product_config_template_id AND parent_product_config.active=true
		LEFT JOIN %[1]s.product_categories parent_product_category ON parent_product_category.id=parent_product.product_category_id AND parent_product_category.active=true
		LEFT JOIN %[1]s.product_categories parent_product_parent_category ON parent_product_parent_category.id=parent_product_category.parent_id AND parent_product_parent_category.active=true
		LEFT JOIN LATERAL (
			SELECT COALESCE(
			           NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''),
			           NULLIF(parent_product_unit_template.inventory_unit,''),
			           NULLIF(parent_product_config.inventory_unit,''),
			           NULLIF(parent_product_category.inventory_unit,''),
			           NULLIF(parent_product_parent_category.inventory_unit,''),
			           'kg'
			       ) AS parent_inventory_unit
		) parent_units ON true
		LEFT JOIN LATERAL (
			SELECT CASE
			         WHEN COALESCE(p.net_content_qty,0) <= 0 THEN 1::float8
			         WHEN lower(COALESCE(NULLIF(p.net_content_unit,''), COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))) = lower(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))
			         THEN COALESCE(p.net_content_qty,0)::float8
			         WHEN lower(COALESCE(p.net_content_unit,''))='g' AND lower(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))='kg'
			         THEN COALESCE(p.net_content_qty,0)::float8 / 1000.0
			         WHEN lower(COALESCE(p.net_content_unit,''))='kg' AND lower(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))='g'
			         THEN COALESCE(p.net_content_qty,0)::float8 * 1000.0
			         WHEN COALESCE(p.net_content_unit,'')='g' AND COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg')='磅'
			         THEN COALESCE(p.net_content_qty,0)::float8 / 454.0
			         WHEN COALESCE(p.net_content_unit,'')='磅' AND COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg')='g'
			         THEN COALESCE(p.net_content_qty,0)::float8 * 454.0
			         WHEN lower(COALESCE(p.net_content_unit,''))='kg' AND COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg')='磅'
			         THEN COALESCE(p.net_content_qty,0)::float8 / 0.454
			         WHEN COALESCE(p.net_content_unit,'')='磅' AND lower(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'))='kg'
			         THEN COALESCE(p.net_content_qty,0)::float8 * 0.454
			         ELSE 1::float8
			       END AS derived_sku_unit_factor
		) derived_sku_units ON true
		WHERE a.active=true
		  AND (COALESCE(p.parent_product_id,0)=0 OR parent_product.id IS NOT NULL)
		  AND (NOT COALESCE(p.auto_derived_sku,false) OR COALESCE(NULLIF(p.derived_spec_status,''),'active')<>'template_removed')
		ORDER BY a.customer_id, a.sort_order, a.id`, r.schema)
	rows, err := r.pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.ProductOption, 0)
	for rows.Next() {
		var p salesapp.ProductOption
		if err := rows.Scan(&p.ID, &p.Name, &p.RoastLevel, &p.DefaultPrice, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.ProductKind, &p.DripBagGrams, &p.DripBoxBagCount, &p.ProductTypeCategoryID, &p.ProductSubtypeCategoryID, &p.ProductTypeName, &p.ProductSubtypeName, &p.InventoryUnit, &p.QuoteUnit, &p.OrderUnit, &p.UnitConversionJSON, &p.IntegerUnit, &p.CustomerProductAliasID, &p.CustomerProductDisplayName, &p.CustomerItemCode, &p.BrandName, &p.ProductRecordName, &p.CustomerAliasDisplayCategoryID, &p.SKUID, &p.ParentProductID, &p.ParentProductName, &p.SKUName, &p.SKUCode, &p.SpecLabel, &p.NetContentQty, &p.NetContentUnit, &p.IsDefaultSKU, &p.DefaultSKUID); err != nil {
			return nil, err
		}
		p.ProductCode = firstOrderSalesSpecText(p.SKUCode, fmt.Sprintf("SKU-%d", p.ID))
		if p.ProductKind == "drip_bag" {
			p.SalesUnits = []string{"bag", "box"}
		}
		p.RetailSpecs = salesdomain.RetailAvailableSpecs(salesdomain.RetailSpecPrices{
			Price100G: p.RetailPrice100G,
			Price200G: p.RetailPrice200G,
			Price227G: p.RetailPrice227G,
			Price250G: p.RetailPrice250G,
		})
		out = append(out, p)
	}
	return out, rows.Err()
}

type orderPublicationProductKey struct {
	CustomerID int64
	ProductID  int64
}

func orderPublicationProductKeyForProduct(product salesapp.ProductOption) orderPublicationProductKey {
	return orderPublicationProductKey{
		CustomerID: product.CustomerID,
		ProductID:  product.ID,
	}
}

func (r Repository) fetchCommercialOrderPublicationTiers(ctx context.Context, products []salesapp.ProductOption) (map[orderPublicationProductKey][]salesapp.ProductTierOption, error) {
	return r.fetchStandardOrderPublicationTiers(ctx, products, "commercial")
}

func (r Repository) fetchRetailOrderPublicationTiers(ctx context.Context, products []salesapp.ProductOption) (map[orderPublicationProductKey][]salesapp.ProductTierOption, error) {
	return r.fetchStandardOrderPublicationTiers(ctx, products, "retail")
}

func (r Repository) fetchDripOrderPublicationTiers(ctx context.Context, products []salesapp.ProductOption) (map[orderPublicationProductKey][]salesapp.ProductTierOption, error) {
	return r.fetchStandardOrderPublicationTiers(ctx, products, "drip")
}

func (r Repository) fetchStandardOrderPublicationTiers(ctx context.Context, products []salesapp.ProductOption, listType string) (map[orderPublicationProductKey][]salesapp.ProductTierOption, error) {
	listType = standardOrderPublicationListType(listType)
	customerOwners := map[string]bool{}
	hasCommercialProduct := false
	for _, product := range products {
		if !orderCommercialProductKind(product.ProductKind) {
			continue
		}
		hasCommercialProduct = true
		if product.CustomerID > 0 {
			customerOwners[strconv.FormatInt(product.CustomerID, 10)] = true
		}
	}
	if !hasCommercialProduct {
		return map[orderPublicationProductKey][]salesapp.ProductTierOption{}, nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.bean_list_publications", r.schema)).Scan(&exists); err != nil || !exists {
		return map[orderPublicationProductKey][]salesapp.ProductTierOption{}, err
	}
	ownerKeys := make([]string, 0, len(customerOwners))
	for key := range customerOwners {
		ownerKeys = append(ownerKeys, key)
	}
	q := fmt.Sprintf(`
		WITH customer_publications AS (
			SELECT 'customer'::text AS owner_type,
			       owner_key,
			       id,
			       COALESCE(version_no, '') AS version_no,
			       COALESCE(content_json, '{}'::jsonb) AS content_json,
			       published_at
			FROM %[1]s.bean_list_publications
			WHERE status='published'
			  AND list_type=$2
			  AND publication_purpose='factory_supply'
			  AND owner_type='customer'
			  AND owner_key = ANY($1)
		),
		official_publications AS (
			SELECT 'official'::text AS owner_type,
			       ''::text AS owner_key,
			       id,
			       COALESCE(version_no, '') AS version_no,
			       COALESCE(content_json, '{}'::jsonb) AS content_json,
			       published_at
			FROM %[1]s.bean_list_publications
			WHERE status='published'
			  AND list_type=$2
			  AND publication_purpose='factory_supply'
			  AND owner_type='official'
		),
		applicable_publications AS (
			SELECT owner_type, owner_key, id, version_no, content_json, published_at
			FROM customer_publications
			UNION ALL
			SELECT owner_type, owner_key, id, version_no, content_json, published_at
			FROM official_publications
		)
		SELECT owner_type, owner_key, id, version_no, content_json
		FROM applicable_publications
		ORDER BY owner_type, owner_key, published_at DESC, id DESC
	`, r.schema)
	rows, err := r.pool.Query(ctx, q, ownerKeys, listType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	customerTiers := map[string]map[int64][]salesapp.ProductTierOption{}
	officialTiers := map[int64][]salesapp.ProductTierOption{}
	customerLegacyTierCoverage := map[string]map[int64]bool{}
	officialLegacyTierCoverage := map[int64]bool{}
	for rows.Next() {
		var ownerType, ownerKey, versionNo string
		var publicationID int64
		var contentRaw []byte
		if err := rows.Scan(&ownerType, &ownerKey, &publicationID, &versionNo, &contentRaw); err != nil {
			return nil, err
		}
		tiers := commercialOrderTierMapFromPublicationContent(publicationID, versionNo, contentRaw, listType)
		if ownerType == "customer" {
			if customerLegacyTierCoverage[ownerKey] == nil {
				customerLegacyTierCoverage[ownerKey] = map[int64]bool{}
			}
			customerTiers[ownerKey] = mergeLatestCommercialOrderPublicationTierMaps(customerTiers[ownerKey], tiers, customerLegacyTierCoverage[ownerKey])
			continue
		}
		officialTiers = mergeLatestCommercialOrderPublicationTierMaps(officialTiers, tiers, officialLegacyTierCoverage)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := map[orderPublicationProductKey][]salesapp.ProductTierOption{}
	for _, product := range products {
		if !orderCommercialProductKind(product.ProductKind) {
			continue
		}
		key := orderPublicationProductKeyForProduct(product)
		if product.CustomerID > 0 {
			ownerKey := strconv.FormatInt(product.CustomerID, 10)
			if tiers := customerTiers[ownerKey][product.ID]; len(tiers) > 0 {
				out[key] = tiers
			}
			continue
		}
		if tiers := officialTiers[product.ID]; len(tiers) > 0 {
			out[key] = tiers
		}
	}
	return out, nil
}

func standardOrderPublicationListType(listType string) string {
	switch strings.TrimSpace(listType) {
	case "retail":
		return "retail"
	case "green":
		return "green"
	case "drip":
		return "drip"
	default:
		return "commercial"
	}
}

func applyCommercialOrderPublicationTiers(products []salesapp.ProductOption, publicationTiers map[orderPublicationProductKey][]salesapp.ProductTierOption) {
	for i := range products {
		if !orderCommercialProductKind(products[i].ProductKind) {
			continue
		}
		tiers := publicationTiers[orderPublicationProductKeyForProduct(products[i])]
		if len(tiers) == 0 {
			continue
		}
		products[i].Tiers = append([]salesapp.ProductTierOption(nil), tiers...)
	}
}

func applyRetailOrderPublicationTiers(products []salesapp.ProductOption, publicationTiers map[orderPublicationProductKey][]salesapp.ProductTierOption) {
	for i := range products {
		if !orderCommercialProductKind(products[i].ProductKind) {
			continue
		}
		tiers := publicationTiers[orderPublicationProductKeyForProduct(products[i])]
		if len(tiers) == 0 {
			continue
		}
		products[i].Tiers = append(products[i].Tiers, tiers...)
	}
}

func applyHistoricalDripOrderPublicationTiers(products []salesapp.ProductOption, publicationTiers map[orderPublicationProductKey][]salesapp.ProductTierOption) {
	for i := range products {
		if strings.TrimSpace(products[i].ProductKind) != "drip_bag" || len(products[i].Tiers) > 0 {
			continue
		}
		tiers := publicationTiers[orderPublicationProductKeyForProduct(products[i])]
		if len(tiers) == 0 {
			continue
		}
		products[i].Tiers = append([]salesapp.ProductTierOption(nil), tiers...)
	}
}

func orderCommercialProductKind(productKind string) bool {
	switch strings.TrimSpace(productKind) {
	case "green_bean":
		return false
	default:
		return true
	}
}

func (r Repository) fetchGreenBeanOrderPublicationTiers(ctx context.Context, products []salesapp.ProductOption) (map[orderPublicationProductKey][]salesapp.ProductTierOption, error) {
	customerOwners := map[string]bool{}
	for _, product := range products {
		if product.CustomerID > 0 {
			customerOwners[strconv.FormatInt(product.CustomerID, 10)] = true
		}
	}
	if len(products) == 0 {
		return map[orderPublicationProductKey][]salesapp.ProductTierOption{}, nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.bean_list_publications", r.schema)).Scan(&exists); err != nil || !exists {
		return map[orderPublicationProductKey][]salesapp.ProductTierOption{}, err
	}
	ownerKeys := make([]string, 0, len(customerOwners))
	for key := range customerOwners {
		ownerKeys = append(ownerKeys, key)
	}
	q := fmt.Sprintf(`
		WITH customer_publications AS (
			SELECT owner_key,
			       id,
			       COALESCE(version_no, '') AS version_no,
			       COALESCE(config_json, '{}'::jsonb) AS config_json,
			       COALESCE(content_json, '{}'::jsonb) AS content_json,
			       row_number() OVER (PARTITION BY owner_key ORDER BY published_at DESC, id DESC) AS rn
			FROM %[1]s.bean_list_publications
			WHERE status='published'
			  AND list_type='green'
			  AND publication_purpose='factory_supply'
			  AND owner_type='customer'
			  AND owner_key = ANY($1)
		),
		official_publications AS (
			SELECT id,
			       COALESCE(version_no, '') AS version_no,
			       COALESCE(config_json, '{}'::jsonb) AS config_json,
			       COALESCE(content_json, '{}'::jsonb) AS content_json,
			       row_number() OVER (ORDER BY published_at DESC, id DESC) AS rn
			FROM %[1]s.bean_list_publications
			WHERE status='published'
			  AND list_type='green'
			  AND publication_purpose='factory_supply'
			  AND owner_type='official'
		)
		SELECT 'customer' AS owner_type, owner_key, id, version_no, config_json, content_json
		FROM customer_publications
		UNION ALL
		SELECT 'official' AS owner_type, '' AS owner_key, id, version_no, config_json, content_json
		FROM official_publications
	`, r.schema)
	rows, err := r.pool.Query(ctx, q, ownerKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	customerTiers := map[string]map[int64][]salesapp.ProductTierOption{}
	officialTiers := map[int64][]salesapp.ProductTierOption{}
	for rows.Next() {
		var ownerType, ownerKey, versionNo string
		var publicationID int64
		var configRaw, contentRaw []byte
		if err := rows.Scan(&ownerType, &ownerKey, &publicationID, &versionNo, &configRaw, &contentRaw); err != nil {
			return nil, err
		}
		tiers := greenBeanOrderTierMapFromPublicationContent(publicationID, versionNo, contentRaw, configRaw)
		if ownerType == "customer" {
			customerTiers[ownerKey] = mergeOrderPublicationTierMaps(customerTiers[ownerKey], tiers)
			continue
		}
		officialTiers = mergeOrderPublicationTierMaps(officialTiers, tiers)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := map[orderPublicationProductKey][]salesapp.ProductTierOption{}
	for _, product := range products {
		key := orderPublicationProductKeyForProduct(product)
		ownerKey := ""
		if product.CustomerID > 0 {
			ownerKey = strconv.FormatInt(product.CustomerID, 10)
		}
		if tiers := customerTiers[ownerKey][product.ID]; len(tiers) > 0 {
			out[key] = tiers
			continue
		}
		if tiers := officialTiers[product.ID]; len(tiers) > 0 {
			out[key] = tiers
		}
	}
	return out, nil
}

func mergeOrderPublicationTierMaps(dst, src map[int64][]salesapp.ProductTierOption) map[int64][]salesapp.ProductTierOption {
	if dst == nil {
		dst = map[int64][]salesapp.ProductTierOption{}
	}
	for productID, tiers := range src {
		if len(tiers) == 0 {
			continue
		}
		dst[productID] = append(dst[productID], tiers...)
	}
	return dst
}

func mergeLatestCommercialOrderPublicationTierMaps(dst, src map[int64][]salesapp.ProductTierOption, legacyCoverageStates ...map[int64]bool) map[int64][]salesapp.ProductTierOption {
	if dst == nil {
		dst = map[int64][]salesapp.ProductTierOption{}
	}
	legacyCovered := map[int64]bool{}
	if len(legacyCoverageStates) > 0 && legacyCoverageStates[0] != nil {
		legacyCovered = legacyCoverageStates[0]
	} else {
		// Direct callers historically passed only the merged tier map. Rebuild the
		// visible legacy coverage so that the helper keeps that call shape working.
		// A persistent coverage map is still required by the publication reader to
		// remember an empty legacy snapshot alongside concrete SKU tiers.
		for productID, tiers := range dst {
			if len(tiers) == 0 {
				legacyCovered[productID] = true
				continue
			}
			for _, tier := range tiers {
				if !orderPublicationTierHasConcreteSKU(tier) {
					legacyCovered[productID] = true
					break
				}
			}
		}
	}
	for productID, tiers := range src {
		concrete := make([]salesapp.ProductTierOption, 0, len(tiers))
		legacy := make([]salesapp.ProductTierOption, 0, len(tiers))
		for _, tier := range tiers {
			if orderPublicationTierHasConcreteSKU(tier) {
				concrete = append(concrete, tier)
			} else {
				legacy = append(legacy, tier)
			}
		}
		if len(concrete) > 0 {
			dst[productID] = appendUniqueOrderPublicationTiers(dst[productID], concrete)
		}
		if legacyCovered[productID] {
			continue
		}
		if len(legacy) > 0 {
			dst[productID] = append(dst[productID], legacy...)
			legacyCovered[productID] = true
			continue
		}
		if len(tiers) == 0 {
			if _, covered := dst[productID]; !covered {
				dst[productID] = []salesapp.ProductTierOption{}
			}
			legacyCovered[productID] = true
		}
	}
	return dst
}

func orderPublicationTierHasConcreteSKU(tier salesapp.ProductTierOption) bool {
	if strings.TrimSpace(tier.QuantityBasis) != "sales_spec_count" || tier.PublicationID <= 0 || len(tier.EffectiveSalesSpec) == 0 {
		return false
	}
	return orderFamilyTierMapInt64(tier.EffectiveSalesSpec, "sku_id") > 0
}

func appendUniqueOrderPublicationTiers(dst, src []salesapp.ProductTierOption) []salesapp.ProductTierOption {
	seen := make(map[string]bool, len(dst)+len(src))
	key := func(tier salesapp.ProductTierOption) string {
		return fmt.Sprintf("%d:%d", tier.PublicationID, tier.ID)
	}
	for _, tier := range dst {
		seen[key(tier)] = true
	}
	for _, tier := range src {
		tierKey := key(tier)
		if seen[tierKey] {
			continue
		}
		seen[tierKey] = true
		dst = append(dst, tier)
	}
	return dst
}

func orderFamilyTierMapInt64(values map[string]any, key string) int64 {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch value := value.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return parsed
	default:
		return 0
	}
}

func applyGreenBeanOrderPublicationTiers(products []salesapp.ProductOption, publicationTiers map[orderPublicationProductKey][]salesapp.ProductTierOption) {
	greenFamilies := map[orderPublicationProductKey]bool{}
	for i := range products {
		parentID := products[i].ParentProductID
		if parentID <= 0 {
			parentID = products[i].ID
		}
		familyKey := orderPublicationProductKey{CustomerID: products[i].CustomerID, ProductID: parentID}
		if strings.TrimSpace(products[i].ProductKind) == "green_bean" {
			greenFamilies[familyKey] = true
		}
		tiers := publicationTiers[orderPublicationProductKeyForProduct(products[i])]
		if len(tiers) > 0 {
			greenFamilies[familyKey] = true
		}
	}
	for i := range products {
		parentID := products[i].ParentProductID
		if parentID <= 0 {
			parentID = products[i].ID
		}
		if !greenFamilies[orderPublicationProductKey{CustomerID: products[i].CustomerID, ProductID: parentID}] {
			continue
		}
		products[i].ProductKind = "green_bean"
		products[i].Tiers = append([]salesapp.ProductTierOption(nil), publicationTiers[orderPublicationProductKeyForProduct(products[i])]...)
		if products[i].Tiers == nil {
			products[i].Tiers = []salesapp.ProductTierOption{}
		}
	}
}

type orderBeanListPublicationContent struct {
	Groups []struct {
		Items []json.RawMessage `json:"items"`
	} `json:"groups"`
	PriceRows []json.RawMessage `json:"price_rows"`
}

type orderGreenBeanPublicationTier struct {
	Label                   string          `json:"label"`
	SourcePriceRecordID     int64           `json:"source_price_record_id"`
	FinalUnitPrice          float64         `json:"final_unit_price"`
	SpecG                   int64           `json:"spec_g"`
	MinQty                  float64         `json:"min_qty"`
	MaxQty                  *float64        `json:"max_qty"`
	MinWeightG              float64         `json:"min_weight_g"`
	MaxWeightG              *float64        `json:"max_weight_g"`
	MinLb                   float64         `json:"min_lb"`
	MaxLb                   *float64        `json:"max_lb"`
	PricePerUnit            float64         `json:"price_per_unit"`
	PricePerKg              float64         `json:"price_per_kg"`
	PricePerLb              float64         `json:"price_per_lb"`
	TemplateID              int64           `json:"template_id"`
	TemplateTierID          int64           `json:"template_tier_id"`
	DisplayUnit             string          `json:"display_unit"`
	PriceUnit               string          `json:"price_unit"`
	SalesUnit               string          `json:"sales_unit"`
	UnitBagCount            int64           `json:"unit_bag_count"`
	InventoryUnit           string          `json:"inventory_unit"`
	InventoryConversionJSON json.RawMessage `json:"inventory_conversion_json"`
	ProductKind             string          `json:"product_kind"`
	QuantityBasis           string          `json:"quantity_basis"`
	TierQuantityUnit        string          `json:"tier_quantity_unit"`
	EffectiveSalesSpec      json.RawMessage `json:"effective_sales_spec"`
}

type orderCommercialPublicationTier = orderGreenBeanPublicationTier

type orderEffectiveSalesSpecSnapshot struct {
	SKUID          int64   `json:"sku_id"`
	SpecName       string  `json:"spec_name"`
	SpecLabel      string  `json:"spec_label"`
	SalesUnit      string  `json:"sales_unit"`
	NetContentQty  float64 `json:"net_content_qty"`
	NetContentUnit string  `json:"net_content_unit"`
}

func commercialOrderTierMapFromPublicationContent(publicationID int64, versionNo string, raw []byte, listTypes ...string) map[int64][]salesapp.ProductTierOption {
	out := map[int64][]salesapp.ProductTierOption{}
	if publicationID <= 0 || len(raw) == 0 {
		return out
	}
	var content orderBeanListPublicationContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return out
	}
	listType := "commercial"
	if len(listTypes) > 0 {
		listType = standardOrderPublicationListType(listTypes[0])
	}
	legacyTierKey := "commercial_wholesale_tiers"
	switch listType {
	case "retail":
		legacyTierKey = "retail_bean_tiers"
	case "drip":
		legacyTierKey = "drip_wholesale_tiers"
	}
	flatParentIDs := map[int64]bool{}
	flatTiers := map[int64][]salesapp.ProductTierOption{}
	for idx, rowRaw := range content.PriceRows {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rowRaw, &fields); err != nil {
			continue
		}
		productID := orderBeanListFlatRowProductID(fields)
		if productID <= 0 {
			continue
		}
		if parentID := rawJSONInt64(fields["parent_product_id"]); parentID > 0 && parentID != productID {
			flatParentIDs[parentID] = true
		} else if parentID := rawJSONInt64(fields["product_id"]); parentID > 0 && parentID != productID {
			flatParentIDs[parentID] = true
		}
		if _, covered := flatTiers[productID]; !covered {
			flatTiers[productID] = []salesapp.ProductTierOption{}
		}
		var tier orderCommercialPublicationTier
		if err := json.Unmarshal(rowRaw, &tier); err != nil {
			continue
		}
		if strings.TrimSpace(tier.ProductKind) == "" {
			switch strings.TrimSpace(tier.SalesUnit) {
			case "bag", "box":
				tier.ProductKind = "drip_bag"
			}
		}
		normalizeCommercialOrderFlatTierBounds(&tier)
		option := commercialOrderTierOption(publicationID, versionNo, idx, tier, tier.ProductKind, listType)
		if option.UnitPrice <= 0 {
			continue
		}
		flatTiers[productID] = append(flatTiers[productID], option)
	}
	for _, group := range content.Groups {
		for _, itemRaw := range group.Items {
			productID := orderBeanListProductID(itemRaw)
			if productID <= 0 {
				continue
			}
			if flatParentIDs[productID] {
				continue
			}
			if _, covered := out[productID]; !covered {
				out[productID] = []salesapp.ProductTierOption{}
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(itemRaw, &fields); err != nil {
				continue
			}
			var tiers []orderCommercialPublicationTier
			if data, ok := fields[legacyTierKey]; !ok || json.Unmarshal(data, &tiers) != nil {
				continue
			}
			productKind := rawJSONString(fields["product_kind"])
			for idx, tier := range tiers {
				option := commercialOrderTierOption(publicationID, versionNo, idx, tier, productKind, listType)
				if option.UnitPrice <= 0 {
					continue
				}
				out[productID] = append(out[productID], option)
			}
		}
	}
	for productID, tiers := range flatTiers {
		if len(tiers) > 0 {
			out[productID] = tiers
			continue
		}
		if _, covered := out[productID]; !covered {
			out[productID] = []salesapp.ProductTierOption{}
		}
	}
	return out
}

func orderBeanListFlatRowProductID(fields map[string]json.RawMessage) int64 {
	for _, key := range []string{"sku_id", "skuId", "skuID"} {
		if id := rawJSONInt64(fields[key]); id > 0 {
			return id
		}
	}
	for _, key := range []string{"product_id", "productId", "productID"} {
		if id := rawJSONInt64(fields[key]); id > 0 {
			return id
		}
	}
	return 0
}

func normalizeCommercialOrderFlatTierBounds(tier *orderCommercialPublicationTier) {
	if tier == nil {
		return
	}
	if strings.TrimSpace(tier.QuantityBasis) == "sales_spec_count" {
		return
	}
	if tier.SpecG <= 0 {
		tier.SpecG = int64(greenBeanOrderPriceUnitG(tier.PriceUnit, 0))
	}
	if tier.SpecG <= 0 {
		return
	}
	if tier.MinWeightG > 0 || tier.MaxWeightG != nil {
		tier.MinQty = tier.MinWeightG / float64(tier.SpecG)
		if tier.MaxWeightG != nil {
			maxQty := *tier.MaxWeightG / float64(tier.SpecG)
			tier.MaxQty = &maxQty
		} else {
			tier.MaxQty = nil
		}
		return
	}
	if tier.MinLb > 0 || tier.MaxLb != nil {
		tier.MinQty = tier.MinLb * 454.0 / float64(tier.SpecG)
		if tier.MaxLb != nil {
			maxQty := *tier.MaxLb * 454.0 / float64(tier.SpecG)
			tier.MaxQty = &maxQty
		} else {
			tier.MaxQty = nil
		}
	}
}

func greenBeanOrderTierMapFromPublicationContent(publicationID int64, versionNo string, raw []byte, configRaw ...[]byte) map[int64][]salesapp.ProductTierOption {
	out := map[int64][]salesapp.ProductTierOption{}
	if publicationID <= 0 || len(raw) == 0 {
		return out
	}
	overrides := map[string]map[string]float64{}
	if len(configRaw) > 0 {
		overrides = greenBeanOrderPriceOverridesFromPublicationConfig(configRaw[0])
	}
	var content orderBeanListPublicationContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return out
	}
	flatParentIDs := map[int64]bool{}
	flatTiers := map[int64][]salesapp.ProductTierOption{}
	for idx, rowRaw := range content.PriceRows {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rowRaw, &fields); err != nil {
			continue
		}
		productID := orderBeanListFlatRowProductID(fields)
		if productID <= 0 {
			continue
		}
		if parentID := rawJSONInt64(fields["parent_product_id"]); parentID > 0 && parentID != productID {
			flatParentIDs[parentID] = true
		}
		if _, covered := flatTiers[productID]; !covered {
			flatTiers[productID] = []salesapp.ProductTierOption{}
		}
		var tier orderCommercialPublicationTier
		if err := json.Unmarshal(rowRaw, &tier); err != nil {
			continue
		}
		normalizeCommercialOrderFlatTierBounds(&tier)
		option := commercialOrderTierOption(publicationID, versionNo, idx, tier, "green_bean", "green")
		if option.UnitPrice > 0 {
			flatTiers[productID] = append(flatTiers[productID], option)
		}
	}
	for _, group := range content.Groups {
		for _, itemRaw := range group.Items {
			productID := orderBeanListProductID(itemRaw)
			if productID <= 0 {
				continue
			}
			if flatParentIDs[productID] {
				continue
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(itemRaw, &fields); err != nil {
				continue
			}
			var tiers []orderGreenBeanPublicationTier
			if data, ok := fields["green_bean_sale_tiers"]; !ok || json.Unmarshal(data, &tiers) != nil {
				continue
			}
			itemName := rawJSONString(fields["name"])
			productOverrides := overrides[strconv.FormatInt(productID, 10)]
			if len(productOverrides) == 0 && itemName != "" {
				productOverrides = overrides[itemName]
			}
			for idx, tier := range tiers {
				if price, ok := greenBeanOrderManualPriceOverride(tier, productOverrides); ok {
					applyGreenBeanOrderManualPrice(&tier, price)
				}
				option := greenBeanOrderTierOption(publicationID, versionNo, idx, tier)
				if option.UnitPrice <= 0 {
					continue
				}
				out[productID] = append(out[productID], option)
			}
		}
	}
	for productID, tiers := range flatTiers {
		if len(tiers) > 0 {
			out[productID] = tiers
		}
	}
	return out
}

func greenBeanOrderPriceOverridesFromPublicationConfig(raw []byte) map[string]map[string]float64 {
	if len(raw) == 0 {
		return nil
	}
	var config struct {
		Customizers map[string]struct {
			GreenPriceOverrides map[string]float64 `json:"greenPriceOverrides"`
		} `json:"customizers"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil
	}
	out := map[string]map[string]float64{}
	for productKey, customizer := range config.Customizers {
		key := strings.TrimSpace(productKey)
		if key == "" {
			continue
		}
		for tierKey, price := range customizer.GreenPriceOverrides {
			tierKey = strings.TrimSpace(tierKey)
			if tierKey == "" || price <= 0 {
				continue
			}
			if out[key] == nil {
				out[key] = map[string]float64{}
			}
			out[key][tierKey] = price
		}
	}
	return out
}

func greenBeanOrderManualPriceOverride(tier orderGreenBeanPublicationTier, overrides map[string]float64) (float64, bool) {
	if len(overrides) == 0 {
		return 0, false
	}
	for _, key := range []string{
		strconv.FormatInt(tier.TemplateTierID, 10),
		strings.TrimSpace(tier.Label),
	} {
		if key == "" || key == "0" {
			continue
		}
		price, ok := overrides[key]
		if ok && price > 0 {
			return price, true
		}
	}
	return 0, false
}

func applyGreenBeanOrderManualPrice(tier *orderGreenBeanPublicationTier, price float64) {
	unitPrice := roundOrderPrice(price)
	priceUnit := greenBeanOrderPriceUnit(tier.DisplayUnit, tier.PriceUnit, true)
	unitG := greenBeanOrderPriceUnitG(priceUnit, tier.SpecG)
	pricePerKg := unitPrice
	if unitG > 0 && unitG != 1000 {
		pricePerKg = unitPrice * 1000.0 / unitG
	}
	tier.PriceUnit = priceUnit
	tier.PricePerUnit = unitPrice
	tier.PricePerKg = roundOrderPrice(pricePerKg)
	tier.PricePerLb = roundOrderPrice(pricePerKg * 0.454)
}

func orderBeanListProductID(raw json.RawMessage) int64 {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return 0
	}
	for _, key := range []string{"sku_id", "skuId", "skuID", "productId", "product_id", "productID"} {
		if id := rawJSONInt64(fields[key]); id > 0 {
			return id
		}
	}
	return 0
}

func rawJSONInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return id
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		id, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		return id
	}
	return 0
}

func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func commercialOrderTierOption(publicationID int64, versionNo string, idx int, tier orderCommercialPublicationTier, productKind string, listTypes ...string) salesapp.ProductTierOption {
	listType := "commercial"
	if len(listTypes) > 0 {
		listType = standardOrderPublicationListType(listTypes[0])
	}
	effectiveSalesSpec := map[string]any{}
	effectiveSpec := orderEffectiveSalesSpecSnapshot{}
	if len(tier.EffectiveSalesSpec) > 0 && string(tier.EffectiveSalesSpec) != "null" {
		_ = json.Unmarshal(tier.EffectiveSalesSpec, &effectiveSalesSpec)
		_ = json.Unmarshal(tier.EffectiveSalesSpec, &effectiveSpec)
	}
	countBySalesSpec := strings.TrimSpace(tier.QuantityBasis) == "sales_spec_count"
	specG := tier.SpecG
	if countBySalesSpec {
		if frozenSpecG := effectiveSalesSpecWeightG(effectiveSpec); frozenSpecG > 0 {
			specG = frozenSpecG
		}
	} else if specG <= 0 {
		specG = 454
	}
	displayUnit := ""
	if countBySalesSpec {
		displayUnit = firstOrderSalesSpecText(effectiveSpec.SalesUnit, effectiveSpec.SpecName, effectiveSpec.SpecLabel, tier.TierQuantityUnit, tier.SalesUnit)
	} else {
		displayUnit = normalizeGreenBeanOrderPriceUnit(tier.DisplayUnit)
		if displayUnit == "" {
			displayUnit = "lb"
		}
	}
	priceUnit := ""
	if countBySalesSpec {
		priceUnit = displayUnit
	} else {
		priceUnit = normalizeGreenBeanOrderPriceUnit(tier.PriceUnit)
		if priceUnit == "" {
			priceUnit = displayUnit
		}
		priceUnit = greenBeanOrderPriceUnit(displayUnit, priceUnit, false)
	}
	unitPrice := greenBeanOrderTierPrice(orderGreenBeanPublicationTier(tier), specG, displayUnit, priceUnit)
	if tier.FinalUnitPrice > 0 {
		unitPrice = roundOrderPrice(tier.FinalUnitPrice)
	}
	id := tier.TemplateTierID
	if id <= 0 {
		id = publicationID*100000 + int64(idx+1)
	}
	source := map[string]any{
		"source":                   "published_bean_list",
		"published_price_snapshot": true,
		"list_type":                listType,
		"publication_id":           publicationID,
		"version_no":               versionNo,
		"template_id":              tier.TemplateID,
		"template_tier_id":         tier.TemplateTierID,
		"display_unit":             displayUnit,
		"price_unit":               priceUnit,
	}
	if quantityBasis := strings.TrimSpace(tier.QuantityBasis); quantityBasis != "" {
		source["quantity_basis"] = quantityBasis
	}
	if quantityUnit := strings.TrimSpace(tier.TierQuantityUnit); quantityUnit != "" {
		source["tier_quantity_unit"] = quantityUnit
	}
	if len(effectiveSalesSpec) > 0 {
		source["effective_sales_spec"] = effectiveSalesSpec
	}
	addOrderPublicationSnapshotSource(source, tier.SourcePriceRecordID, tier.InventoryUnit, tier.InventoryConversionJSON)
	sourceJSON, _ := json.Marshal(source)
	return salesapp.ProductTierOption{
		ID:                   id,
		SpecG:                specG,
		MinQty:               tier.MinQty,
		MaxQty:               tier.MaxQty,
		UnitPrice:            unitPrice,
		DisplayUnit:          priceUnit,
		ProductKind:          orderCommercialPublicationProductKind(productKind),
		SalesUnit:            firstOrderSalesSpecText(tier.SalesUnit, effectiveSpec.SalesUnit),
		UnitBagCount:         tier.UnitBagCount,
		PriceSourceJSON:      string(sourceJSON),
		QuantityBasis:        strings.TrimSpace(tier.QuantityBasis),
		TierQuantityUnit:     strings.TrimSpace(tier.TierQuantityUnit),
		EffectiveSalesSpec:   effectiveSalesSpec,
		PublicationID:        publicationID,
		PublicationVersionNo: versionNo,
		ListType:             listType,
	}
}

func firstOrderSalesSpecText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func effectiveSalesSpecWeightG(spec orderEffectiveSalesSpecSnapshot) int64 {
	if spec.NetContentQty <= 0 {
		return 0
	}
	factor := float64(0)
	switch strings.ToLower(strings.TrimSpace(spec.NetContentUnit)) {
	case "g", "克":
		factor = 1
	case "kg", "千克", "公斤":
		factor = 1000
	case "lb", "lbs", "磅":
		factor = 453.59237
	}
	if factor <= 0 {
		return 0
	}
	return int64(math.Round(spec.NetContentQty * factor))
}

func orderCommercialPublicationProductKind(productKind string) string {
	productKind = strings.TrimSpace(productKind)
	if productKind == "" {
		return "roasted_bean"
	}
	return productKind
}

func greenBeanOrderTierOption(publicationID int64, versionNo string, idx int, tier orderGreenBeanPublicationTier) salesapp.ProductTierOption {
	specG := tier.SpecG
	if specG <= 0 {
		specG = 1000
	}
	displayUnit := strings.TrimSpace(strings.ToLower(tier.DisplayUnit))
	if displayUnit == "" {
		displayUnit = "kg"
	}
	priceUnit := strings.TrimSpace(strings.ToLower(tier.PriceUnit))
	if priceUnit == "" {
		priceUnit = displayUnit
	}
	priceUnit = greenBeanOrderPriceUnit(displayUnit, priceUnit, false)
	unitPrice := greenBeanOrderTierPrice(tier, specG, displayUnit, priceUnit)
	if tier.FinalUnitPrice > 0 {
		unitPrice = roundOrderPrice(tier.FinalUnitPrice)
	}
	id := tier.TemplateTierID
	if id <= 0 {
		id = publicationID*100000 + int64(idx+1)
	}
	source := map[string]any{
		"source":                   "published_bean_list",
		"published_price_snapshot": true,
		"list_type":                "green",
		"publication_id":           publicationID,
		"version_no":               versionNo,
		"template_id":              tier.TemplateID,
		"template_tier_id":         tier.TemplateTierID,
		"display_unit":             displayUnit,
		"price_unit":               priceUnit,
	}
	addOrderPublicationSnapshotSource(source, tier.SourcePriceRecordID, tier.InventoryUnit, tier.InventoryConversionJSON)
	sourceJSON, _ := json.Marshal(source)
	return salesapp.ProductTierOption{
		ID:                   id,
		SpecG:                specG,
		MinQty:               tier.MinQty,
		MaxQty:               tier.MaxQty,
		UnitPrice:            unitPrice,
		DisplayUnit:          priceUnit,
		ProductKind:          "green_bean",
		PriceSourceJSON:      string(sourceJSON),
		QuantityBasis:        strings.TrimSpace(tier.QuantityBasis),
		TierQuantityUnit:     strings.TrimSpace(tier.TierQuantityUnit),
		PublicationID:        publicationID,
		PublicationVersionNo: versionNo,
		ListType:             "green",
	}
}

func addOrderPublicationSnapshotSource(source map[string]any, sourcePriceRecordID int64, inventoryUnit string, conversionJSON json.RawMessage) {
	if sourcePriceRecordID > 0 {
		source["source_price_record_id"] = sourcePriceRecordID
	}
	if strings.TrimSpace(inventoryUnit) != "" {
		source["inventory_unit"] = strings.TrimSpace(inventoryUnit)
	}
	if len(conversionJSON) == 0 || string(conversionJSON) == "null" {
		return
	}
	var conversion map[string]any
	if err := json.Unmarshal(conversionJSON, &conversion); err == nil && len(conversion) > 0 {
		source["inventory_conversion_json"] = conversion
	}
}

func greenBeanOrderTierPrice(tier orderGreenBeanPublicationTier, specG int64, displayUnit string, priceUnit string) float64 {
	if priceUnit != "kg" && priceUnit != "lb" {
		unitG := greenBeanOrderPriceUnitG(priceUnit, specG)
		displayUnitG := greenBeanOrderPriceUnitG(displayUnit, specG)
		if tier.PricePerUnit > 0 && displayUnit == priceUnit {
			return roundOrderPrice(tier.PricePerUnit)
		}
		if tier.PricePerKg > 0 {
			return roundOrderPrice(tier.PricePerKg * unitG / 1000.0)
		}
		if tier.PricePerLb > 0 {
			return roundOrderPrice(tier.PricePerLb * unitG / 454.0)
		}
		if tier.PricePerUnit > 0 && displayUnitG > 0 {
			return roundOrderPrice(tier.PricePerUnit * unitG / displayUnitG)
		}
		return 0
	}
	switch priceUnit {
	case "kg":
		if tier.PricePerKg > 0 {
			return roundOrderPrice(tier.PricePerKg)
		}
		if displayUnit == "kg" && tier.PricePerUnit > 0 {
			return roundOrderPrice(tier.PricePerUnit)
		}
		if tier.PricePerLb > 0 {
			return roundOrderPrice(tier.PricePerLb / 0.454)
		}
		return roundOrderPrice(greenBeanOrderTierPricePerLb(tier, specG, displayUnit) / 0.454)
	default:
		return greenBeanOrderTierPricePerLb(tier, specG, displayUnit)
	}
}

func greenBeanOrderTierPricePerLb(tier orderGreenBeanPublicationTier, specG int64, displayUnit string) float64 {
	if tier.PricePerLb > 0 {
		return roundOrderPrice(tier.PricePerLb)
	}
	if tier.PricePerKg > 0 {
		return roundOrderPrice(tier.PricePerKg * 454.0 / 1000.0)
	}
	if specG <= 0 {
		specG = 1000
	}
	if tier.PricePerUnit <= 0 {
		return 0
	}
	switch displayUnit {
	case "lb":
		return roundOrderPrice(tier.PricePerUnit)
	case "kg":
		return roundOrderPrice(tier.PricePerUnit * 454.0 / 1000.0)
	default:
		pricePerKg := tier.PricePerUnit * 1000.0 / float64(specG)
		return roundOrderPrice(pricePerKg * 454.0 / 1000.0)
	}
}

func greenBeanOrderPriceUnit(displayUnit string, explicitUnit string, preferDisplay bool) string {
	display := normalizeGreenBeanOrderPriceUnit(displayUnit)
	explicit := normalizeGreenBeanOrderPriceUnit(explicitUnit)
	if preferDisplay {
		if display != "" {
			return display
		}
		if explicit != "" {
			return explicit
		}
		return "lb"
	}
	if explicit != "" {
		return explicit
	}
	if display != "" {
		return display
	}
	return "lb"
}

func normalizeGreenBeanOrderPriceUnit(unit string) string {
	value := strings.TrimSpace(unit)
	lower := strings.ToLower(value)
	switch lower {
	case "kg", "lb", "g100", "g227", "g250":
		return lower
	default:
		return value
	}
}

func greenBeanOrderPriceUnitG(unit string, specG int64) float64 {
	switch normalizeGreenBeanOrderPriceUnit(unit) {
	case "kg":
		return 1000
	case "lb":
		return 454
	case "g100":
		return 100
	case "g227":
		return 227
	case "g250":
		return 250
	default:
		if specG > 0 {
			return float64(specG)
		}
		return 454
	}
}

func roundOrderPrice(value float64) float64 {
	return math.Round((value+1e-9)*100) / 100
}

func (r Repository) fetchOrderEdit(ctx context.Context, id int64) (*salesapp.OrderEditData, error) {
	q := fmt.Sprintf(`
		SELECT
			o.id,
			o.order_no,
			COALESCE(to_char(o.document_date,'YYYY-MM-DD'), to_char(o.order_date,'YYYY-MM-DD'), '') as document_date,
			COALESCE(to_char(o.order_date,'YYYY-MM-DD'), '') as order_date,
			COALESCE(o.customer_id,0) as customer_id,
			COALESCE(o.source_id,0) as source_id,
			COALESCE(o.order_type_id,0) as order_type_id,
			COALESCE(o.pay_status_id,0) as pay_status_id,
			COALESCE(o.payment_method,'') as payment_method,
			COALESCE(o.ship_status_id,0) as ship_status_id,
			COALESCE(o.ship_method,'') as ship_method,
			%s as ship_tracking_no,
			COALESCE(o.logistics_company_id,0) as logistics_company_id,
			COALESCE(o.logistics_product_id,0) as logistics_product_id,
			COALESCE(o.payment_goods_amount,0) as payment_goods_amount,
			COALESCE(o.payment_shipping_amount,0) as payment_shipping_amount,
			COALESCE(o.payment_voucher_asset_id,0) as payment_voucher_asset_id,
			COALESCE(a.id,0) as payment_voucher_id,
			COALESCE(a.kind,'') as payment_voucher_kind,
			COALESCE(a.filename,'') as payment_voucher_filename,
			COALESCE(a.content_type,'') as payment_voucher_content_type,
			COALESCE(a.bytes,0) as payment_voucher_bytes,
			COALESCE(a.sha256,'') as payment_voucher_sha256,
			COALESCE(a.object_key,'') as payment_voucher_object_key,
			COALESCE(to_char(a.created_at, 'YYYY-MM-DD HH24:MI:SS'),'') as payment_voucher_created_at,
			COALESCE(a.created_by,'') as payment_voucher_created_by,
			COALESCE(o.responsible_party_type,'') as responsible_party_type,
			COALESCE(o.responsible_party_id,0) as responsible_party_id,
			COALESCE(o.responsible_party_name,'') as responsible_party_name,
			COALESCE(NULLIF(o.receiver_name,''), NULLIF(c.contact,''), c.name, '') AS receiver_name,
			COALESCE(NULLIF(o.receiver_phone,''), c.phone, '') AS receiver_phone,
			COALESCE(NULLIF(o.receiver_address,''), c.address, '') AS receiver_address,
			COALESCE(o.receiver_company, '') AS receiver_company,
			COALESCE(o.portal_service_code,'') AS portal_service_code,
			COALESCE(o.source_warehouse,'') AS source_warehouse,
			COALESCE(o.bean_list_publication_id,0) AS bean_list_publication_id,
			COALESCE(o.bean_list_version_no,'') AS bean_list_version_no,
			COALESCE(o.notes,'') as notes,
			COALESCE(o.total_amount,0) as total_amount,
			COALESCE(o.shipping_amount,0) as shipping_amount,
			COALESCE(o.discount_amount,0) as discount_amount,
			COALESCE(o.round_to_int,false) as round_to_int,
			COALESCE(o.rounding_amount,0) as rounding_amount,
			COALESCE(o.grand_total,0) as grand_total,
			COALESCE(o.express_fee,'') as express_fee,
			COALESCE(o.outsource_material_fee,0) as outsource_material_fee,
			COALESCE(o.outsource_roast_fee,0) as outsource_roast_fee,
			COALESCE(o.outsource_packaging_fee,0) as outsource_packaging_fee,
			COALESCE(o.outsource_manual_fee,0) as outsource_manual_fee,
			COALESCE(o.outsource_tax_fee,0) as outsource_tax_fee,
			COALESCE(o.outsource_other_fee,0) as outsource_other_fee,
			COALESCE(o.outsource_total_fee,0) as outsource_total_fee,
			md5(to_jsonb(o)::text || '|' || COALESCE((
				SELECT jsonb_agg(to_jsonb(revision_item) ORDER BY revision_item.id)::text
				FROM %s.order_items revision_item
				WHERE revision_item.order_id=o.id
			), '[]')) AS edit_revision,
			o.is_void,
			CASE WHEN o.voided_at IS NULL THEN NULL ELSE to_char(o.voided_at, 'YYYY-MM-DD HH24:MI:SS') END AS voided_at,
			o.void_reason
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		LEFT JOIN %s.sales_order_assets a ON a.id=o.payment_voucher_asset_id
		WHERE o.id=$1
	`, orderTrackingSummaryExpr(r.schema, "o"), r.schema, r.schema, r.schema, r.schema)

	var d salesapp.OrderEditData
	var totalAmt, shipAmt, discAmt, roundAmt, grandAmt float64
	var paymentGoodsAmt, paymentShippingAmt float64
	var outsourceMaterial, outsourceRoast, outsourcePackaging, outsourceManual, outsourceTax, outsourceOther, outsourceTotal float64
	var paymentVoucher salesapp.SalesOrderAsset
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&d.ID,
		&d.OrderNo,
		&d.DocumentDate,
		&d.OrderDate,
		&d.CustomerID,
		&d.SourceID,
		&d.OrderTypeID,
		&d.PayStatusID,
		&d.PaymentMethod,
		&d.ShipStatusID,
		&d.ShipMethod,
		&d.ShipTrackingNo,
		&d.LogisticsCompanyID,
		&d.LogisticsProductID,
		&paymentGoodsAmt,
		&paymentShippingAmt,
		&d.PaymentVoucherAssetID,
		&paymentVoucher.ID,
		&paymentVoucher.Kind,
		&paymentVoucher.Filename,
		&paymentVoucher.ContentType,
		&paymentVoucher.Bytes,
		&paymentVoucher.SHA256,
		&paymentVoucher.ObjectKey,
		&paymentVoucher.CreatedAt,
		&paymentVoucher.CreatedBy,
		&d.ResponsibleType,
		&d.ResponsibleID,
		&d.ResponsibleName,
		&d.ReceiverName,
		&d.ReceiverPhone,
		&d.ReceiverAddress,
		&d.ReceiverCompany,
		&d.PortalServiceCode,
		&d.SourceWarehouse,
		&d.BeanListPublicationID,
		&d.BeanListVersionNo,
		&d.Notes,
		&totalAmt,
		&shipAmt,
		&discAmt,
		&d.RoundToInt,
		&roundAmt,
		&grandAmt,
		&d.ExpressFee,
		&outsourceMaterial,
		&outsourceRoast,
		&outsourcePackaging,
		&outsourceManual,
		&outsourceTax,
		&outsourceOther,
		&outsourceTotal,
		&d.EditRevision,
		&d.IsVoid,
		&d.VoidedAt,
		&d.VoidReason,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	d.TotalAmount = fmt.Sprintf("%.2f", totalAmt)
	d.ShippingAmount = fmt.Sprintf("%.2f", shipAmt)
	d.DiscountAmount = fmt.Sprintf("%.2f", discAmt)
	d.RoundingAmount = fmt.Sprintf("%.2f", roundAmt)
	d.GrandTotal = fmt.Sprintf("%.2f", grandAmt)
	d.PaymentGoodsAmount = fmt.Sprintf("%.2f", paymentGoodsAmt)
	d.PaymentShippingAmount = fmt.Sprintf("%.2f", paymentShippingAmt)
	if paymentVoucher.ID > 0 {
		paymentVoucher.URL = salesOrderAssetURL(paymentVoucher.ObjectKey)
		d.PaymentVoucher = &paymentVoucher
	}
	d.OutsourceMaterialFee = fmt.Sprintf("%.2f", outsourceMaterial)
	d.OutsourceRoastFee = fmt.Sprintf("%.2f", outsourceRoast)
	d.OutsourcePackagingFee = fmt.Sprintf("%.2f", outsourcePackaging)
	d.OutsourceManualFee = fmt.Sprintf("%.2f", outsourceManual)
	d.OutsourceTaxFee = fmt.Sprintf("%.2f", outsourceTax)
	d.OutsourceOtherFee = fmt.Sprintf("%.2f", outsourceOther)
	d.OutsourceTotalFee = fmt.Sprintf("%.2f", outsourceTotal)

	itemsQ := orderEditItemsQuery(r.schema)
	rows, err := r.pool.Query(ctx, itemsQ, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	d.Items = make([]salesapp.OrderEditItem, 0)
	for rows.Next() {
		var it salesapp.OrderEditItem
		var qty, unitPrice, lineTotal, discountValue, discountAmount, unitBeanG, matchedPriceQty float64
		if err := rows.Scan(&it.ItemID, &it.LineNo, &it.ProductID, &it.BomSpecID, &it.BomVariantID, &it.BomSpecKey, &it.BomSpecName, &it.Product, &it.CustomerProductAliasID, &it.CustomerProductDisplayNameSnapshot, &it.CustomerItemCodeSnapshot, &it.BrandNameSnapshot, &it.ProductCodeSnapshot, &it.ProductNameSnapshot, &it.Note, &it.Spec, &qty, &it.Unit, &unitPrice, &lineTotal, &it.PriceTierID, &it.PriceOverride, &it.BeanListPublicationID, &it.BeanListVersionNo, &it.DiscountType, &discountValue, &discountAmount, &it.ProductKind, &it.SalesUnit, &it.UnitBagCount, &unitBeanG, &matchedPriceQty, &it.PriceSourceJSON); err != nil {
			return nil, err
		}
		it.Qty = trimFloatZero(qty)
		it.UnitPrice = fmt.Sprintf("%.2f", unitPrice)
		it.LineTotal = fmt.Sprintf("%.2f", lineTotal)
		it.DiscountValue = trimFloatZero(discountValue)
		it.DiscountAmount = fmt.Sprintf("%.2f", discountAmount)
		it.UnitBeanG = trimFloatZero(unitBeanG)
		it.MatchedPriceQty = trimFloatZero(matchedPriceQty)
		it.UnitConversionLabel = orderEditUnitConversionLabel(it.SalesUnit, it.UnitBagCount, unitBeanG)
		d.Items = append(d.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &d, nil
}

func (r Repository) LoadSenderProfile(ctx context.Context) (salesapp.SenderProfile, error) {
	profile := salesapp.SenderProfile{
		Label:     env("SENDER_LABEL", "默认寄件人"),
		Name:      env("SENDER_NAME", ""),
		Phone:     env("SENDER_PHONE", ""),
		Addr:      env("SENDER_ADDR", ""),
		Company:   env("SENDER_COMPANY", ""),
		Goods:     env("SENDER_GOODS", "茶叶"),
		BizType:   env("SF_BIZ_TYPE", ""),
		IsDefault: true,
		Active:    true,
	}
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(sender_label,''), COALESCE(sender_name,''), COALESCE(sender_phone,''), COALESCE(sender_addr,''), COALESCE(sender_company,''), COALESCE(sender_goods,''), COALESCE(sf_biz_type,''), is_default, active
		FROM %s.sender_settings
		WHERE active=true
		ORDER BY is_default DESC, id
		LIMIT 1
	`, r.schema)).Scan(
		&profile.ID, &profile.Label, &profile.Name, &profile.Phone, &profile.Addr, &profile.Company, &profile.Goods, &profile.BizType, &profile.IsDefault, &profile.Active,
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return salesapp.SenderProfile{}, err
	}
	return profile, nil
}

func (r Repository) LoadSenderProfileByID(ctx context.Context, id int64) (salesapp.SenderProfile, error) {
	var profile salesapp.SenderProfile
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(sender_label,''), COALESCE(sender_name,''), COALESCE(sender_phone,''), COALESCE(sender_addr,''), COALESCE(sender_company,''), COALESCE(sender_goods,''), COALESCE(sf_biz_type,''), is_default, active
		FROM %s.sender_settings
		WHERE id=$1 AND active=true
	`, r.schema), id).Scan(
		&profile.ID, &profile.Label, &profile.Name, &profile.Phone, &profile.Addr, &profile.Company, &profile.Goods, &profile.BizType, &profile.IsDefault, &profile.Active,
	)
	if err != nil {
		return salesapp.SenderProfile{}, err
	}
	return profile, nil
}

func (r Repository) ListSenderProfiles(ctx context.Context) ([]salesapp.SenderProfile, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(sender_label,''), COALESCE(sender_name,''), COALESCE(sender_phone,''), COALESCE(sender_addr,''), COALESCE(sender_company,''), COALESCE(sender_goods,''), COALESCE(sf_biz_type,''), is_default, active
		FROM %s.sender_settings
		WHERE active=true
		ORDER BY is_default DESC, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.SenderProfile, 0)
	for rows.Next() {
		var profile salesapp.SenderProfile
		if err := rows.Scan(&profile.ID, &profile.Label, &profile.Name, &profile.Phone, &profile.Addr, &profile.Company, &profile.Goods, &profile.BizType, &profile.IsDefault, &profile.Active); err != nil {
			return nil, err
		}
		out = append(out, profile)
	}
	return out, rows.Err()
}

func (r Repository) SaveSenderProfile(ctx context.Context, profile salesapp.SenderProfile) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if profile.IsDefault {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.sender_settings SET is_default=false WHERE is_default=true`, r.schema)); err != nil {
			return err
		}
	}
	if profile.ID > 0 {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.sender_settings
			SET sender_label=$2,sender_name=$3,sender_phone=$4,sender_addr=$5,sender_company=$6,sender_goods=$7,sf_biz_type=$8,is_default=$9,active=$10,updated_at=now()
			WHERE id=$1
		`, r.schema), profile.ID, profile.Label, profile.Name, profile.Phone, profile.Addr, profile.Company, profile.Goods, profile.BizType, profile.IsDefault, profile.Active)
	} else {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
			VALUES ((SELECT COALESCE(MAX(id),0)+1 FROM %s.sender_settings), $1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, r.schema, r.schema), profile.Label, profile.Name, profile.Phone, profile.Addr, profile.Company, profile.Goods, profile.BizType, profile.IsDefault, profile.Active)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.sender_settings
		SET is_default=true, active=true
		WHERE id=(SELECT id FROM %s.sender_settings WHERE active=true ORDER BY is_default DESC, id LIMIT 1)
		  AND NOT EXISTS (SELECT 1 FROM %s.sender_settings WHERE is_default=true AND active=true)
	`, r.schema, r.schema, r.schema)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func EnsureSenderSettingsTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sender_settings (
		id SMALLINT PRIMARY KEY DEFAULT 1,
		sender_label TEXT NOT NULL DEFAULT '',
		sender_name TEXT NOT NULL DEFAULT '',
		sender_phone TEXT NOT NULL DEFAULT '',
		sender_addr TEXT NOT NULL DEFAULT '',
		sender_company TEXT NOT NULL DEFAULT '',
		sender_goods TEXT NOT NULL DEFAULT '茶叶',
		sf_biz_type TEXT NOT NULL DEFAULT '',
		is_default BOOLEAN NOT NULL DEFAULT false,
		active BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	ALTER TABLE %s.sender_settings ADD COLUMN IF NOT EXISTS sender_label TEXT NOT NULL DEFAULT '';
	ALTER TABLE %s.sender_settings ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;
	ALTER TABLE %s.sender_settings ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
	INSERT INTO %s.sender_settings(id, sender_label, is_default, active) VALUES(1, '默认寄件人', true, true) ON CONFLICT (id) DO NOTHING;
	UPDATE %s.sender_settings
	SET sender_label=COALESCE(NULLIF(sender_label,''), NULLIF(sender_name,''), '默认寄件人');
	UPDATE %s.sender_settings
	SET is_default=true, active=true
	WHERE id=(SELECT id FROM %s.sender_settings ORDER BY is_default DESC, id LIMIT 1)
	  AND NOT EXISTS (SELECT 1 FROM %s.sender_settings WHERE is_default=true AND active=true);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_sender_settings_one_default ON %s.sender_settings ((is_default)) WHERE is_default=true AND active=true;`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func trimFloatZero(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}

func orderEditUnitConversionLabel(salesUnit string, unitBagCount int64, unitBeanG float64) string {
	switch strings.TrimSpace(salesUnit) {
	case "bag":
		if unitBeanG > 0 {
			return trimFloatZero(unitBeanG) + "g/袋"
		}
	case "box":
		if unitBagCount > 0 {
			return strconv.FormatInt(unitBagCount, 10) + "袋/盒"
		}
	}
	return ""
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
