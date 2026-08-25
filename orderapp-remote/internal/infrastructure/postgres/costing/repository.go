package costing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	appcosting "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) LoadParameters(ctx context.Context) (domain.Parameters, error) {
	params := domain.DefaultParameters()
	if r.pool == nil {
		return params, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT key, value::float8 FROM %s.cost_parameters`, r.schema))
	if err != nil {
		return params, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var value float64
		if err := rows.Scan(&key, &value); err != nil {
			return params, err
		}
		applyParameter(&params, key, value)
	}
	return params, rows.Err()
}

func (r Repository) LoadProductInputs(ctx context.Context, params domain.Parameters) ([]domain.ProductInput, error) {
	return r.loadProductInputs(ctx, params, 0)
}

func (r Repository) LoadProductInputsForCustomer(ctx context.Context, params domain.Parameters, customerID int64) ([]domain.ProductInput, error) {
	if customerID < 0 {
		customerID = 0
	}
	return r.loadProductInputs(ctx, params, customerID)
}

func (r Repository) ResolveProductSalesUnitRule(ctx context.Context, productID int64, priceUnit string) (appcosting.ProductSalesUnitRule, error) {
	priceUnit = strings.TrimSpace(priceUnit)
	if productID <= 0 {
		return appcosting.ProductSalesUnitRule{}, appcosting.ErrProductSalesUnitRuleNotFound
	}
	var inventoryUnit string
	var defaultSalesUnit string
	var conversionJSON string
	var autoDerived bool
	var derivedSalesUnit string
	var skuName string
	var skuCode string
	var barcode string
	var specKey string
	var specName string
	var specLabel string
	var netContentQty float64
	var netContentUnit string
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT CASE
		         WHEN COALESCE(p.parent_product_id,0) > 0 THEN COALESCE(
		           NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''),
		           NULLIF(parent_product_unit_template.inventory_unit,''),
		           NULLIF(parent_product_config.inventory_unit,''),
		           NULLIF(parent_product_category.inventory_unit,''),
		           NULLIF(parent_product_parent_category.inventory_unit,''),
		           'kg'
		         )
		         ELSE COALESCE(
		           NULLIF(p.unit_rule_override_json->>'inventory_unit',''),
		           NULLIF(product_unit_template.inventory_unit,''),
		           NULLIF(pct.inventory_unit,''),
		           NULLIF(put.inventory_unit,''),
		           NULLIF(pc.inventory_unit,''),
		           NULLIF(pc_unit.inventory_unit,''),
		           NULLIF(parent_pc.inventory_unit,''),
		           NULLIF(parent_pc_unit.inventory_unit,''),
		           'kg'
		         )
		       END AS inventory_unit,
		       CASE
		         WHEN COALESCE(p.auto_derived_sku,false) THEN COALESCE(NULLIF(p.derived_sales_unit,''), NULLIF(p.sku_name,''), 'kg')
		         ELSE COALESCE(
		           NULLIF(p.unit_rule_override_json->>'default_sales_unit',''),
		           NULLIF(p.unit_rule_override_json->>'order_unit',''),
		           NULLIF(p.unit_rule_override_json->>'quote_unit',''),
		           NULLIF(product_unit_template_default_spec.sales_unit,''),
		           NULLIF(product_unit_template.order_unit,''),
		           NULLIF(product_unit_template.quote_unit,''),
		           NULLIF(pct.order_unit,''),
		           NULLIF(pct.quote_unit,''),
		           NULLIF(put.order_unit,''),
		           NULLIF(put.quote_unit,''),
		           NULLIF(pc.order_unit,''),
		           NULLIF(pc.quote_unit,''),
		           NULLIF(pc_unit.order_unit,''),
		           NULLIF(pc_unit.quote_unit,''),
		           NULLIF(parent_pc.order_unit,''),
		           NULLIF(parent_pc.quote_unit,''),
		           NULLIF(parent_pc_unit.order_unit,''),
		           NULLIF(parent_pc_unit.quote_unit,''),
		           NULLIF(p.unit_rule_override_json->>'inventory_unit',''),
		           NULLIF(product_unit_template.inventory_unit,''),
		           NULLIF(pct.inventory_unit,''),
		           NULLIF(put.inventory_unit,''),
		           NULLIF(pc.inventory_unit,''),
		           NULLIF(pc_unit.inventory_unit,''),
		           NULLIF(parent_pc.inventory_unit,''),
		           NULLIF(parent_pc_unit.inventory_unit,''),
		           'kg'
		         )
		       END AS default_sales_unit,
		       CASE
		         WHEN COALESCE(p.auto_derived_sku,false) AND NULLIF(p.derived_sales_unit,'') IS NOT NULL
		           THEN jsonb_build_object(p.derived_sales_unit, jsonb_build_object(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'), derived_sku_units.derived_sku_unit_factor))::text
		         ELSE COALESCE(
		           NULLIF(p.unit_rule_override_json->>'unit_conversion_json',''),
		           NULLIF(p.unit_rule_override_json->>'conversion_json',''),
		           NULLIF(product_unit_template_default_spec.unit_conversion_json,'{}'),
		           NULLIF(product_unit_template.unit_conversion_json::text,'{}'),
		           NULLIF(pct.unit_conversion_json::text,'{}'),
		           NULLIF(put.unit_conversion_json::text,'{}'),
		           NULLIF(pc.unit_conversion_json::text,'{}'),
		           NULLIF(pc_unit.unit_conversion_json::text,'{}'),
		           NULLIF(parent_pc.unit_conversion_json::text,'{}'),
		           NULLIF(parent_pc_unit.unit_conversion_json::text,'{}'),
		           '{}'
		         )
		       END AS unit_conversion_json,
		       COALESCE(p.auto_derived_sku,false) AS auto_derived_sku,
		       COALESCE(p.derived_sales_unit,'') AS derived_sales_unit,
		       COALESCE(NULLIF(p.sku_name,''), NULLIF(p.name,''), '') AS sku_name,
		       COALESCE(p.sku_code,'') AS sku_code,
		       COALESCE(p.barcode,'') AS barcode,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(NULLIF(p.derived_spec_key,''), '')
		         ELSE COALESCE(NULLIF(product_unit_template_default_spec.spec_key,''), NULLIF(p.derived_spec_key,''), '')
		       END AS spec_key,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(NULLIF(p.derived_spec_name,''), NULLIF(p.spec_label,''), NULLIF(p.sku_name,''), NULLIF(p.derived_sales_unit,''), '')
		         ELSE COALESCE(NULLIF(product_unit_template_default_spec.spec_name,''), NULLIF(p.spec_label,''), NULLIF(p.sku_name,''), '')
		       END AS spec_name,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(NULLIF(p.spec_label,''), NULLIF(p.derived_spec_name,''), NULLIF(p.sku_name,''), '')
		         ELSE COALESCE(NULLIF(product_unit_template_default_spec.spec_label,''), NULLIF(product_unit_template_default_spec.spec_name,''), NULLIF(p.spec_label,''), '')
		       END AS spec_label,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(p.net_content_qty,0)::float8
		         ELSE COALESCE(NULLIF(p.net_content_qty,0), product_unit_template_default_spec.net_content_qty, 0)::float8
		       END AS net_content_qty,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(p.net_content_unit,'')
		         ELSE COALESCE(NULLIF(p.net_content_unit,''), product_unit_template_default_spec.net_content_unit, '')
		       END AS net_content_unit
		FROM %[1]s.products p
		LEFT JOIN %[1]s.product_unit_templates product_unit_template ON product_unit_template.id = p.unit_template_id AND product_unit_template.active = true
		LEFT JOIN LATERAL (
			SELECT NULLIF(spec.row->>'spec_key','') AS spec_key,
			       NULLIF(spec.row->>'spec_name','') AS spec_name,
			       COALESCE(NULLIF(spec.row->>'spec_label',''), NULLIF(spec.row->>'spec_name','')) AS spec_label,
			       NULLIF(spec.row->>'spec_name','') AS sales_unit,
			       COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)::float8 AS net_content_qty,
			       NULLIF(spec.row->>'net_content_unit','') AS net_content_unit,
			       CASE
			         WHEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) > 0
			          AND NULLIF(spec.row->>'spec_name','') IS NOT NULL
			         THEN jsonb_build_object(
			           NULLIF(spec.row->>'spec_name',''),
			           jsonb_build_object(
			             COALESCE(NULLIF(product_unit_template.inventory_unit,''), NULLIF(spec.row->>'net_content_unit',''), 'kg'),
			             CASE
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''), NULLIF(product_unit_template.inventory_unit,''), 'kg')) = lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''), NULLIF(spec.row->>'net_content_unit',''), 'kg'))
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('g','克') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('kg','千克','公斤')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) / 1000.0
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('kg','千克','公斤') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('g','克')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) * 1000.0
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('lb','lbs','磅') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('kg','千克','公斤')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) * 0.45359237
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('kg','千克','公斤') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('lb','lbs','磅')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) / 0.45359237
			               ELSE COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)
			             END
			           )
			         )::text
			         ELSE '{}'
			       END AS unit_conversion_json
			FROM jsonb_array_elements(COALESCE(product_unit_template.sales_specs_json, '[]'::jsonb)) WITH ORDINALITY AS spec(row, ord)
			WHERE COALESCE(spec.row->>'active','true') <> 'false'
			  AND NULLIF(spec.row->>'spec_name','') IS NOT NULL
			ORDER BY CASE WHEN COALESCE(spec.row->>'default','false') = 'true' THEN 0 ELSE 1 END, spec.ord
			LIMIT 1
		) product_unit_template_default_spec ON true
		LEFT JOIN %[1]s.products parent_product ON parent_product.id = p.parent_product_id AND parent_product.active = true
		LEFT JOIN %[1]s.product_unit_templates parent_product_unit_template ON parent_product_unit_template.id = parent_product.unit_template_id AND parent_product_unit_template.active = true
		LEFT JOIN %[1]s.product_config_templates parent_product_config ON parent_product_config.id = parent_product.product_config_template_id AND parent_product_config.active = true
		LEFT JOIN %[1]s.product_categories parent_product_category ON parent_product_category.id = parent_product.product_category_id AND parent_product_category.active = true
		LEFT JOIN %[1]s.product_categories parent_product_parent_category ON parent_product_parent_category.id = parent_product_category.parent_id AND parent_product_parent_category.active = true
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
		LEFT JOIN %[1]s.product_config_templates pct ON pct.id = p.product_config_template_id AND pct.active = true
		LEFT JOIN %[1]s.product_unit_templates put ON put.id = pct.unit_template_id AND put.active = true
		LEFT JOIN %[1]s.product_categories pc ON pc.id = p.product_category_id AND pc.active = true
		LEFT JOIN %[1]s.product_unit_templates pc_unit ON pc_unit.id = pc.unit_template_id AND pc_unit.active = true
		LEFT JOIN %[1]s.product_categories parent_pc ON parent_pc.id = pc.parent_id AND parent_pc.active = true
		LEFT JOIN %[1]s.product_unit_templates parent_pc_unit ON parent_pc_unit.id = parent_pc.unit_template_id AND parent_pc_unit.active = true
		WHERE p.id=$1 AND p.active = true
	`, r.schema), productID).Scan(
		&inventoryUnit,
		&defaultSalesUnit,
		&conversionJSON,
		&autoDerived,
		&derivedSalesUnit,
		&skuName,
		&skuCode,
		&barcode,
		&specKey,
		&specName,
		&specLabel,
		&netContentQty,
		&netContentUnit,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appcosting.ProductSalesUnitRule{}, appcosting.ErrProductSalesUnitRuleNotFound
		}
		return appcosting.ProductSalesUnitRule{}, err
	}
	inventoryUnit = strings.TrimSpace(inventoryUnit)
	if inventoryUnit == "" {
		inventoryUnit = "kg"
	}
	defaultSalesUnit = strings.TrimSpace(defaultSalesUnit)
	derivedSalesUnit = strings.TrimSpace(derivedSalesUnit)
	if autoDerived && derivedSalesUnit != "" {
		defaultSalesUnit = derivedSalesUnit
	}
	if defaultSalesUnit == "" {
		defaultSalesUnit = inventoryUnit
	}
	if priceUnit == "" {
		priceUnit = defaultSalesUnit
	}
	if priceUnit == "" {
		priceUnit = inventoryUnit
	}
	conversion := productSalesUnitConversionMap(conversionJSON, inventoryUnit)
	if _, ok := conversion[priceUnit]; !ok && priceUnit == inventoryUnit {
		conversion[priceUnit] = map[string]float64{inventoryUnit: 1}
	}
	targets := conversion[priceUnit]
	if len(targets) == 0 {
		return appcosting.ProductSalesUnitRule{}, appcosting.ErrProductSalesUnitRuleNotFound
	}
	return appcosting.ProductSalesUnitRule{
		ProductID:        productID,
		SKUName:          strings.TrimSpace(skuName),
		SKUCode:          strings.TrimSpace(skuCode),
		Barcode:          strings.TrimSpace(barcode),
		DefaultSalesUnit: defaultSalesUnit,
		InventoryUnit:    inventoryUnit,
		Conversion:       conversion,
		EffectiveSalesSpec: &domain.EffectiveSalesSpec{
			SKUID:                   productID,
			SpecKey:                 strings.TrimSpace(specKey),
			SpecName:                firstNonEmptyString(strings.TrimSpace(specName), strings.TrimSpace(specLabel), defaultSalesUnit),
			SpecLabel:               firstNonEmptyString(strings.TrimSpace(specLabel), strings.TrimSpace(specName)),
			SalesUnit:               defaultSalesUnit,
			NetContentQty:           netContentQty,
			NetContentUnit:          strings.TrimSpace(netContentUnit),
			InventoryUnit:           inventoryUnit,
			InventoryConversionJSON: conversion,
		},
	}, nil
}

func (r Repository) ResolveProductSpecIdentity(ctx context.Context, productID int64) (appcosting.ProductSpecIdentity, error) {
	if r.pool == nil || productID <= 0 {
		return appcosting.ProductSpecIdentity{}, appcosting.ErrProductSpecIdentityNotFound
	}
	identity := appcosting.ProductSpecIdentity{}
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT p.id,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END AS effective_parent_product_id,
		       COALESCE(NULLIF(CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN parent.name ELSE p.name END,''), p.name, '') AS parent_product_name,
		       COALESCE(p.active,true)
		         AND CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(parent.active,false) ELSE true END AS active,
		       CASE
		         WHEN COALESCE(p.parent_product_id,0)=0 THEN NOT EXISTS (
		           SELECT 1
		           FROM %[1]s.products child
		           WHERE child.parent_product_id=p.id
		             AND child.active=true
		             AND COALESCE(child.derived_spec_status,'active') IN ('', 'active')
		             AND (
		               COALESCE(child.auto_derived_sku,false)=false
		               OR (
		                 COALESCE(child.derived_unit_template_id,0)=COALESCE(p.unit_template_id,0)
		                 AND EXISTS (
		                   SELECT 1
		                   FROM %[1]s.product_unit_templates child_template,
		                        LATERAL jsonb_array_elements(COALESCE(child_template.sales_specs_json,'[]'::jsonb)) child_spec
		                   WHERE child_template.id=p.unit_template_id
		                     AND COALESCE(child_template.active,true)=true
		                     AND COALESCE(child_spec->>'active','true')<>'false'
		                     AND COALESCE(child_spec->>'spec_key','')=COALESCE(child.derived_spec_key,'')
		                 )
		               )
		             )
		         )
		         WHEN COALESCE(p.derived_spec_status,'') NOT IN ('', 'active') THEN false
		         WHEN COALESCE(p.auto_derived_sku,false)=false THEN true
		         ELSE COALESCE(p.derived_spec_status,'')='active'
		          AND COALESCE(p.derived_unit_template_id,0)=COALESCE(parent.unit_template_id,0)
		          AND EXISTS (
		            SELECT 1
		            FROM %[1]s.product_unit_templates template,
		                 LATERAL jsonb_array_elements(COALESCE(template.sales_specs_json,'[]'::jsonb)) spec
		            WHERE template.id=parent.unit_template_id
		              AND COALESCE(template.active,true)=true
		              AND COALESCE(spec->>'active','true')<>'false'
		              AND COALESCE(spec->>'spec_key','')=COALESCE(p.derived_spec_key,'')
		          )
		       END AS spec_valid
		FROM %[1]s.products p
		LEFT JOIN %[1]s.products parent ON parent.id=p.parent_product_id
		WHERE p.id=$1
	`, r.schema), productID).Scan(
		&identity.ProductID,
		&identity.EffectiveParentProductID,
		&identity.ParentProductName,
		&identity.Active,
		&identity.SpecValid,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appcosting.ProductSpecIdentity{}, appcosting.ErrProductSpecIdentityNotFound
		}
		return appcosting.ProductSpecIdentity{}, err
	}
	return identity, nil
}

func (r Repository) ResolveProductBOMSpecIdentity(ctx context.Context, parentProductID, bomSpecID, bomVariantID int64) (appcosting.ProductBOMSpecIdentity, error) {
	if r.pool == nil || parentProductID <= 0 || bomSpecID <= 0 || bomVariantID <= 0 {
		return appcosting.ProductBOMSpecIdentity{}, appcosting.ErrProductBOMSpecIdentityNotFound
	}
	identity := appcosting.ProductBOMSpecIdentity{}
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT parent.id,
		       COALESCE(parent.name,''),
		       binding.bom_id,
		       version.id,
		       spec.id,
		       variant.id,
		       COALESCE(spec.code,''),
		       COALESCE(spec.barcode,''),
		       COALESCE(spec.spec_key,''),
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,''),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit,''),
		       migration.state,
		       COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true),
		       COALESCE(parent.active,true),
		       true,
		       variant.is_default,
		       variant.sort_order
		FROM %[1]s.product_bom_spec_migrations migration
		JOIN %[1]s.products parent
		  ON parent.id=migration.product_id
		JOIN %[1]s.production_bom_output_bindings binding
		  ON binding.output_type='product'
		 AND binding.output_id=parent.id
		 AND binding.is_default=true
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id
		 AND version.bom_id=binding.bom_id
		 AND version.status='published'
		JOIN %[1]s.production_bom_specs spec
		  ON spec.id=$2
		 AND spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.id=$3
		 AND variant.version_id=version.id
		 AND variant.bom_spec_id=spec.id
		WHERE migration.product_id=$1
		  AND (migration.state='cutover' OR COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false)
		LIMIT 1
	`, r.schema), parentProductID, bomSpecID, bomVariantID).Scan(
		&identity.ParentProductID,
		&identity.ParentProductName,
		&identity.BomID,
		&identity.BomVersionID,
		&identity.BomSpecID,
		&identity.BomVariantID,
		&identity.SpecCode,
		&identity.Barcode,
		&identity.SpecKey,
		&identity.SpecName,
		&identity.InventoryUnit,
		&identity.MigrationState,
		&identity.BomSpecAuthoritative,
		&identity.Active,
		&identity.Published,
		&identity.IsDefault,
		&identity.SortOrder,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appcosting.ProductBOMSpecIdentity{}, appcosting.ErrProductBOMSpecIdentityNotFound
		}
		return appcosting.ProductBOMSpecIdentity{}, err
	}
	identity.SpecIdentityMode = "legacy_sku"
	if identity.BomSpecAuthoritative {
		identity.SpecIdentityMode = "bom_spec"
	}
	return identity, nil
}

func (r Repository) ResolveProductDefaultSalesUnit(ctx context.Context, productID int64) (string, error) {
	if r.pool == nil || productID <= 0 {
		return "", appcosting.ErrProductSalesUnitRuleNotFound
	}
	var unit string
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(CASE
		         WHEN COALESCE(p.auto_derived_sku,false) THEN COALESCE(
		           NULLIF(p.derived_sales_unit,''),
		           NULLIF(p.sku_name,'')
		         )
		         ELSE COALESCE(
		           NULLIF(p.unit_rule_override_json->>'default_sales_unit',''),
		           NULLIF(p.unit_rule_override_json->>'order_unit',''),
		           NULLIF(p.unit_rule_override_json->>'quote_unit',''),
		           NULLIF(default_spec.spec_name,''),
		           NULLIF(unit_template.order_unit,''),
		           NULLIF(unit_template.quote_unit,'')
		         )
		       END, '') AS default_sales_unit
		FROM %[1]s.products p
		LEFT JOIN %[1]s.product_unit_templates unit_template
		  ON unit_template.id=p.unit_template_id AND unit_template.active=true
		LEFT JOIN LATERAL (
		  SELECT NULLIF(spec.row->>'spec_name','') AS spec_name
		  FROM jsonb_array_elements(COALESCE(unit_template.sales_specs_json,'[]'::jsonb)) WITH ORDINALITY AS spec(row, ord)
		  WHERE COALESCE(spec.row->>'active','true') <> 'false'
		    AND NULLIF(spec.row->>'spec_name','') IS NOT NULL
		  ORDER BY CASE WHEN COALESCE(spec.row->>'default','false')='true' THEN 0 ELSE 1 END, spec.ord
		  LIMIT 1
		) default_spec ON true
		WHERE p.id=$1 AND p.active=true
	`, r.schema), productID).Scan(&unit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", appcosting.ErrProductSalesUnitRuleNotFound
		}
		return "", err
	}
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return "", appcosting.ErrProductSalesUnitRuleNotFound
	}
	return unit, nil
}

func (r Repository) ResolveCustomerProductSalesUnitRule(ctx context.Context, productID int64, customerProductAliasID int64, priceUnit string) (appcosting.ProductSalesUnitRule, error) {
	priceUnit = strings.TrimSpace(priceUnit)
	if productID <= 0 || customerProductAliasID <= 0 {
		return appcosting.ProductSalesUnitRule{}, appcosting.ErrProductSalesUnitRuleNotFound
	}
	var inventoryUnit string
	var defaultSalesUnit string
	var conversionJSON string
	var autoDerived bool
	var derivedSalesUnit string
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(
		           NULLIF(alias_config.inventory_unit,''),
		           NULLIF(pct.inventory_unit,''),
		           NULLIF(alias_legacy_unit.inventory_unit,''),
		           NULLIF(cpro.unit_rule_json->>'inventory_unit',''),
		           NULLIF(cpti.unit_rule_json->>'inventory_unit',''),
		           CASE
		             WHEN COALESCE(p.parent_product_id,0) > 0 THEN COALESCE(
		               NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''),
		               NULLIF(parent_product_unit_template.inventory_unit,''),
		               NULLIF(parent_product_config.inventory_unit,''),
		               NULLIF(parent_product_category.inventory_unit,''),
		               NULLIF(parent_product_parent_category.inventory_unit,'')
		             )
		             ELSE NULLIF(p.unit_rule_override_json->>'inventory_unit','')
		           END,
		           NULLIF(product_unit_template.inventory_unit,''),
		           NULLIF(put.inventory_unit,''),
		           NULLIF(pc.inventory_unit,''),
		           NULLIF(pc_unit.inventory_unit,''),
		           NULLIF(parent_pc.inventory_unit,''),
		           NULLIF(parent_pc_unit.inventory_unit,''),
		           'kg'
		       ) AS inventory_unit,
		       COALESCE(
		           NULLIF(alias_config.order_unit,''),
		           NULLIF(alias_config.quote_unit,''),
		           NULLIF(pct.order_unit,''),
		           NULLIF(pct.quote_unit,''),
		           NULLIF(alias_legacy_unit.order_unit,''),
		           NULLIF(alias_legacy_unit.quote_unit,''),
		           NULLIF(cpro.unit_rule_json->>'default_sales_unit',''),
		           NULLIF(cpro.unit_rule_json->>'order_unit',''),
		           NULLIF(cpro.unit_rule_json->>'quote_unit',''),
		           NULLIF(cpti.unit_rule_json->>'default_sales_unit',''),
		           NULLIF(cpti.unit_rule_json->>'order_unit',''),
		           NULLIF(cpti.unit_rule_json->>'quote_unit',''),
		           CASE WHEN COALESCE(p.auto_derived_sku,false) THEN NULLIF(p.derived_sales_unit,'') END,
		           NULLIF(p.unit_rule_override_json->>'default_sales_unit',''),
		           NULLIF(p.unit_rule_override_json->>'order_unit',''),
		           NULLIF(p.unit_rule_override_json->>'quote_unit',''),
		           NULLIF(product_unit_template_default_spec.sales_unit,''),
		           NULLIF(product_unit_template.order_unit,''),
		           NULLIF(product_unit_template.quote_unit,''),
		           NULLIF(put.order_unit,''),
		           NULLIF(put.quote_unit,''),
		           NULLIF(pc.order_unit,''),
		           NULLIF(pc.quote_unit,''),
		           NULLIF(pc_unit.order_unit,''),
		           NULLIF(pc_unit.quote_unit,''),
		           NULLIF(parent_pc.order_unit,''),
		           NULLIF(parent_pc.quote_unit,''),
		           NULLIF(parent_pc_unit.order_unit,''),
		           NULLIF(parent_pc_unit.quote_unit,''),
		           NULLIF(alias_config.inventory_unit,''),
		           NULLIF(pct.inventory_unit,''),
		           NULLIF(alias_legacy_unit.inventory_unit,''),
		           NULLIF(cpro.unit_rule_json->>'inventory_unit',''),
		           NULLIF(cpti.unit_rule_json->>'inventory_unit',''),
		           CASE
		             WHEN COALESCE(p.parent_product_id,0) > 0 THEN COALESCE(
		               NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''),
		               NULLIF(parent_product_unit_template.inventory_unit,''),
		               NULLIF(parent_product_config.inventory_unit,''),
		               NULLIF(parent_product_category.inventory_unit,''),
		               NULLIF(parent_product_parent_category.inventory_unit,'')
		             )
		             ELSE NULLIF(p.unit_rule_override_json->>'inventory_unit','')
		           END,
		           NULLIF(product_unit_template.inventory_unit,''),
		           NULLIF(put.inventory_unit,''),
		           NULLIF(pc.inventory_unit,''),
		           NULLIF(pc_unit.inventory_unit,''),
		           NULLIF(parent_pc.inventory_unit,''),
		           NULLIF(parent_pc_unit.inventory_unit,''),
		           'kg'
		       ) AS default_sales_unit,
		       CASE
		         WHEN COALESCE(p.auto_derived_sku,false) AND NULLIF(p.derived_sales_unit,'') IS NOT NULL THEN '{}'
		         ELSE COALESCE(
		           NULLIF(alias_config.unit_conversion_json::text,'{}'),
		           NULLIF(pct.unit_conversion_json::text,'{}'),
		           NULLIF(alias_legacy_unit.unit_conversion_json::text,'{}'),
		           NULLIF(cpro.unit_rule_json->>'unit_conversion_json',''),
		           NULLIF(cpro.unit_rule_json->>'conversion_json',''),
		           NULLIF(cpti.unit_rule_json->>'unit_conversion_json',''),
		           NULLIF(cpti.unit_rule_json->>'conversion_json',''),
		           NULLIF(p.unit_rule_override_json->>'unit_conversion_json',''),
		           NULLIF(p.unit_rule_override_json->>'conversion_json',''),
		           NULLIF(product_unit_template_default_spec.unit_conversion_json,'{}'),
		           NULLIF(product_unit_template.unit_conversion_json::text,'{}'),
		           NULLIF(put.unit_conversion_json::text,'{}'),
		           NULLIF(pc.unit_conversion_json::text,'{}'),
		           NULLIF(pc_unit.unit_conversion_json::text,'{}'),
		           NULLIF(parent_pc.unit_conversion_json::text,'{}'),
		           NULLIF(parent_pc_unit.unit_conversion_json::text,'{}'),
		           '{}'
		         )
		       END AS unit_conversion_json,
		       COALESCE(p.auto_derived_sku,false) AS auto_derived_sku,
		       COALESCE(p.derived_sales_unit,'') AS derived_sales_unit
		FROM %[1]s.products p
		JOIN %[1]s.customer_product_aliases cpa ON cpa.id=$2 AND cpa.product_id=p.id AND cpa.active=true
		LEFT JOIN %[1]s.product_config_templates alias_config ON alias_config.id=cpa.product_config_template_id AND alias_config.active=true
		LEFT JOIN %[1]s.product_config_templates pct ON pct.id = p.product_config_template_id AND pct.active = true
		LEFT JOIN %[1]s.product_unit_templates alias_legacy_unit ON alias_legacy_unit.id=cpa.unit_template_id AND alias_legacy_unit.active=true
		LEFT JOIN %[1]s.product_unit_templates product_unit_template ON product_unit_template.id = p.unit_template_id AND product_unit_template.active = true
		LEFT JOIN LATERAL (
			SELECT NULLIF(spec.row->>'spec_name','') AS spec_name,
			       NULLIF(spec.row->>'spec_name','') AS sales_unit,
			       COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)::float8 AS net_content_qty,
			       NULLIF(spec.row->>'net_content_unit','') AS net_content_unit,
			       CASE
			         WHEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) > 0
			          AND NULLIF(spec.row->>'spec_name','') IS NOT NULL
			         THEN jsonb_build_object(
			           NULLIF(spec.row->>'spec_name',''),
			           jsonb_build_object(
			             COALESCE(NULLIF(product_unit_template.inventory_unit,''), NULLIF(spec.row->>'net_content_unit',''), 'kg'),
			             CASE
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''), NULLIF(product_unit_template.inventory_unit,''), 'kg')) = lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''), NULLIF(spec.row->>'net_content_unit',''), 'kg'))
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('g','克') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('kg','千克','公斤')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) / 1000.0
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('kg','千克','公斤') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('g','克')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) * 1000.0
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('lb','lbs','磅') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('kg','千克','公斤')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) * 0.45359237
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('kg','千克','公斤') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('lb','lbs','磅')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) / 0.45359237
			               ELSE COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)
			             END
			           )
			         )::text
			         ELSE '{}'
			       END AS unit_conversion_json
			FROM jsonb_array_elements(COALESCE(product_unit_template.sales_specs_json, '[]'::jsonb)) WITH ORDINALITY AS spec(row, ord)
			WHERE COALESCE(spec.row->>'active','true') <> 'false'
			  AND NULLIF(spec.row->>'spec_name','') IS NOT NULL
			ORDER BY CASE WHEN COALESCE(spec.row->>'default','false') = 'true' THEN 0 ELSE 1 END, spec.ord
			LIMIT 1
		) product_unit_template_default_spec ON true
		LEFT JOIN %[1]s.products parent_product ON parent_product.id = p.parent_product_id AND parent_product.active = true
		LEFT JOIN %[1]s.product_unit_templates parent_product_unit_template ON parent_product_unit_template.id = parent_product.unit_template_id AND parent_product_unit_template.active = true
		LEFT JOIN %[1]s.product_config_templates parent_product_config ON parent_product_config.id = parent_product.product_config_template_id AND parent_product_config.active = true
		LEFT JOIN %[1]s.product_categories parent_product_category ON parent_product_category.id = parent_product.product_category_id AND parent_product_category.active = true
		LEFT JOIN %[1]s.product_categories parent_product_parent_category ON parent_product_parent_category.id = parent_product_category.parent_id AND parent_product_parent_category.active = true
		LEFT JOIN %[1]s.product_unit_templates put ON put.id = pct.unit_template_id AND put.active = true
		LEFT JOIN %[1]s.product_categories pc ON pc.id = p.product_category_id AND pc.active = true
		LEFT JOIN %[1]s.product_unit_templates pc_unit ON pc_unit.id = pc.unit_template_id AND pc_unit.active = true
		LEFT JOIN %[1]s.product_categories parent_pc ON parent_pc.id = pc.parent_id AND parent_pc.active = true
		LEFT JOIN %[1]s.product_unit_templates parent_pc_unit ON parent_pc_unit.id = parent_pc.unit_template_id AND parent_pc_unit.active = true
		LEFT JOIN %[1]s.customers rule_customer ON rule_customer.id=cpa.customer_id AND rule_customer.active=true
		LEFT JOIN %[1]s.customer_product_rule_template_items cpti
		  ON cpti.active=true
		 AND cpti.template_id=COALESCE(rule_customer.customer_product_rule_template_id,0)
		 AND cpti.product_subtype_category_id=CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.id,0) ELSE 0 END
		LEFT JOIN %[1]s.customer_product_rule_overrides cpro
		  ON cpro.active=true
		 AND cpro.customer_id=cpa.customer_id
		 AND cpro.product_subtype_category_id=CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.id,0) ELSE 0 END
		WHERE p.id=$1 AND p.active = true
	`, r.schema), productID, customerProductAliasID).Scan(&inventoryUnit, &defaultSalesUnit, &conversionJSON, &autoDerived, &derivedSalesUnit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appcosting.ProductSalesUnitRule{}, appcosting.ErrProductSalesUnitRuleNotFound
		}
		return appcosting.ProductSalesUnitRule{}, err
	}
	inventoryUnit = strings.TrimSpace(inventoryUnit)
	if inventoryUnit == "" {
		inventoryUnit = "kg"
	}
	defaultSalesUnit = strings.TrimSpace(defaultSalesUnit)
	derivedSalesUnit = strings.TrimSpace(derivedSalesUnit)
	if autoDerived && derivedSalesUnit != "" {
		defaultSalesUnit = derivedSalesUnit
	}
	if defaultSalesUnit == "" {
		defaultSalesUnit = inventoryUnit
	}
	if priceUnit == "" {
		priceUnit = defaultSalesUnit
	}
	if priceUnit == "" {
		priceUnit = inventoryUnit
	}
	conversion := productSalesUnitConversionMap(conversionJSON, inventoryUnit)
	if autoDerived && derivedSalesUnit != "" {
		if _, ok := conversion[derivedSalesUnit]; !ok {
			conversion[derivedSalesUnit] = map[string]float64{inventoryUnit: 1}
		}
	}
	if _, ok := conversion[priceUnit]; !ok && priceUnit == inventoryUnit {
		conversion[priceUnit] = map[string]float64{inventoryUnit: 1}
	}
	targets := conversion[priceUnit]
	if len(targets) == 0 {
		return appcosting.ProductSalesUnitRule{}, appcosting.ErrProductSalesUnitRuleNotFound
	}
	return appcosting.ProductSalesUnitRule{ProductID: productID, DefaultSalesUnit: defaultSalesUnit, InventoryUnit: inventoryUnit, Conversion: conversion}, nil
}

func (r Repository) ResolvePriceTierTemplateUnitRule(ctx context.Context, templateID int64) (appcosting.PriceTierTemplateUnitRule, error) {
	if r.pool == nil || templateID <= 0 {
		return appcosting.PriceTierTemplateUnitRule{}, appcosting.ErrPriceTierTemplateUnitRuleNotFound
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT template.id, template.name, tier.id, tier.quantity_unit
		FROM %[1]s.price_tier_templates template
		JOIN %[1]s.price_tier_template_tiers tier
		  ON tier.template_id=template.id
		 AND tier.active=true
		WHERE template.id=$1
		  AND template.active=true
		ORDER BY tier.position, tier.min_qty, tier.id
	`, r.schema), templateID)
	if err != nil {
		return appcosting.PriceTierTemplateUnitRule{}, err
	}
	defer rows.Close()

	rule := appcosting.PriceTierTemplateUnitRule{TierUnits: map[int64]string{}}
	for rows.Next() {
		var tierID int64
		var tierUnit string
		if err := rows.Scan(&rule.TemplateID, &rule.TemplateName, &tierID, &tierUnit); err != nil {
			return appcosting.PriceTierTemplateUnitRule{}, err
		}
		rule.TierUnits[tierID] = strings.TrimSpace(tierUnit)
	}
	if err := rows.Err(); err != nil {
		return appcosting.PriceTierTemplateUnitRule{}, err
	}
	if rule.TemplateID <= 0 || len(rule.TierUnits) == 0 {
		return appcosting.PriceTierTemplateUnitRule{}, appcosting.ErrPriceTierTemplateUnitRuleNotFound
	}
	return rule, nil
}

func productSalesUnitConversionMap(raw string, inventoryUnit ...string) map[string]map[string]float64 {
	out := map[string]map[string]float64{}
	raw = strings.TrimSpace(raw)
	targetInventoryUnit := "kg"
	if len(inventoryUnit) > 0 {
		if unit := strings.TrimSpace(inventoryUnit[0]); unit != "" {
			targetInventoryUnit = unit
		}
	}
	if raw == "" || raw == "{}" || raw == "null" {
		productSalesUnitAddStandardWeightConversions(out, targetInventoryUnit)
		return out
	}
	var generic map[string]any
	if err := json.Unmarshal([]byte(raw), &generic); err == nil && generic != nil {
		for fromUnit, rawTargets := range generic {
			fromUnit = strings.TrimSpace(fromUnit)
			if fromUnit == "" {
				continue
			}
			targets, ok := rawTargets.(map[string]any)
			if !ok {
				factor := anyFloat64(rawTargets)
				if factor > 0 && targetInventoryUnit != "" {
					if out[fromUnit] == nil {
						out[fromUnit] = map[string]float64{}
					}
					out[fromUnit][targetInventoryUnit] = factor
				}
				continue
			}
			for toUnit, rawFactor := range targets {
				toUnit = strings.TrimSpace(toUnit)
				factor := anyFloat64(rawFactor)
				if toUnit == "" || factor <= 0 {
					continue
				}
				if out[fromUnit] == nil {
					out[fromUnit] = map[string]float64{}
				}
				out[fromUnit][toUnit] = factor
			}
		}
	}
	productSalesUnitAddStandardWeightConversions(out, targetInventoryUnit)
	return out
}

func productSalesUnitAddStandardWeightConversions(out map[string]map[string]float64, inventoryUnit string) {
	inventoryUnit = strings.TrimSpace(inventoryUnit)
	if inventoryUnit == "" || productSalesUnitWeightKGFactor(inventoryUnit) <= 0 {
		return
	}
	for _, fromUnit := range []string{"g", "kg", "lb", "lbs", "磅", "克", "公斤", "千克"} {
		factor := productSalesUnitStandardWeightFactor(fromUnit, inventoryUnit)
		if factor <= 0 {
			continue
		}
		if out[fromUnit] == nil {
			out[fromUnit] = map[string]float64{}
		}
		if _, exists := out[fromUnit][inventoryUnit]; !exists {
			out[fromUnit][inventoryUnit] = factor
		}
	}
}

func productSalesUnitStandardWeightFactor(fromUnit string, toUnit string) float64 {
	fromKG := productSalesUnitWeightKGFactor(fromUnit)
	toKG := productSalesUnitWeightKGFactor(toUnit)
	if fromKG <= 0 || toKG <= 0 {
		return 0
	}
	return fromKG / toKG
}

func productSalesUnitWeightKGFactor(unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "克":
		return 0.001
	case "kg", "公斤", "千克":
		return 1
	case "lb", "lbs", "磅":
		return 0.45359237
	default:
		return 0
	}
}

func (r Repository) LoadProductPricingRule(ctx context.Context, id int64) (appcosting.ProductPricingRule, error) {
	if id <= 0 {
		return appcosting.ProductPricingRule{}, appcosting.ErrProductPricingRuleNotFound
	}
	var row appcosting.ProductPricingRule
	var calculationJSON []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, name, code, cost_source_mode, margin_rate::float8, tax_rate::float8, rounding_mode, calculation_json, formula_version, active, remark
		FROM %s.product_pricing_rules
		WHERE id=$1
	`, r.schema), id).Scan(&row.ID, &row.Name, &row.Code, &row.CostSourceMode, &row.MarginRate, &row.TaxRate, &row.RoundingMode, &calculationJSON, &row.FormulaVersion, &row.Active, &row.Remark)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appcosting.ProductPricingRule{}, appcosting.ErrProductPricingRuleNotFound
		}
		return appcosting.ProductPricingRule{}, err
	}
	row.CalculationJSON = map[string]any{}
	if len(calculationJSON) > 0 {
		if err := json.Unmarshal(calculationJSON, &row.CalculationJSON); err != nil {
			return appcosting.ProductPricingRule{}, err
		}
	}
	return row, nil
}

func (r Repository) LoadPricingRuleTrialDefaultTaxRate(ctx context.Context) (appcosting.PricingRuleTrialDefaultTaxRate, error) {
	return r.loadPricingRuleTrialDefaultTaxRate(ctx)
}

func (r Repository) loadPricingRuleTrialDefaultTaxRate(ctx context.Context) (appcosting.PricingRuleTrialDefaultTaxRate, error) {
	out := appcosting.PricingRuleTrialDefaultTaxRate{Source: "finance_settings"}
	if r.pool == nil {
		return out, nil
	}
	var taxpayerType string
	var smallRate float64
	var generalRate float64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(taxpayer_type,''),'small_scale'),
		       COALESCE(small_scale_vat_rate,0)::float8,
		       COALESCE(general_output_vat_rate,0)::float8
		FROM %s.finance_settings
		WHERE id=1
	`, r.schema)).Scan(&taxpayerType, &smallRate, &generalRate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, nil
		}
		return out, err
	}
	switch strings.TrimSpace(taxpayerType) {
	case "general":
		out.Rate = generalRate
	default:
		out.Rate = smallRate
	}
	out.Source = "finance_settings"
	return out, nil
}

func (r Repository) LoadPricingRuleTrialProductionOptions(ctx context.Context, input domain.ProductInput) (appcosting.PricingRuleTrialProductionOptions, error) {
	out := appcosting.PricingRuleTrialProductionOptions{}
	if r.pool == nil || input.ProductID <= 0 {
		return out, nil
	}
	bomRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH pricing_rule_trial_selected_products AS (
			SELECT p.id AS product_id, 0 AS source_priority
			FROM %[1]s.products p
			WHERE p.id=$1 AND p.active=true
			UNION ALL
			SELECT p.parent_product_id AS product_id, 1 AS source_priority
			FROM %[1]s.products p
			WHERE p.id=$1 AND p.active=true AND COALESCE(p.parent_product_id,0)>0
		),
		default_bom AS (
			SELECT COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0) AS bom_version_id
			FROM pricing_rule_trial_selected_products selected
			LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=selected.product_id
			LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id=selected.product_id
			WHERE COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0)>0
			ORDER BY selected.source_priority
			LIMIT 1
		),
		pricing_rule_trial_bom_versions AS (
			SELECT pb.id AS bom_id,
			       COALESCE(pb.code,'') AS bom_code,
			       COALESCE(pb.name,'') AS bom_name,
			       v.id AS version_id,
			       COALESCE(v.version_no,'') AS version_no,
			       COALESCE(v.status,'published') AS status,
			       COALESCE(NULLIF(v.process_route_id,0),0) AS process_route_id,
			       COALESCE(NULLIF(pr.name,''),'') AS process_route_name,
			       COALESCE((
			         SELECT COUNT(*)
			         FROM %[1]s.production_bom_version_items component
			         WHERE component.version_id=v.id
			       ),0) AS component_count,
			       COALESCE(latest_nonempty_draft.id,0) AS latest_nonempty_draft_version_id,
			       COALESCE(latest_nonempty_draft.version_no,'') AS latest_nonempty_draft_version_no,
			       COALESCE((SELECT bom_version_id FROM default_bom),0)>0
			         AND v.id=COALESCE((SELECT bom_version_id FROM default_bom),0) AS is_default,
			       selected.source_priority
			FROM pricing_rule_trial_selected_products selected
			JOIN %[1]s.production_boms pb ON pb.output_product_id=selected.product_id
			JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id
			LEFT JOIN %[1]s.process_routes pr ON pr.id=v.process_route_id
			LEFT JOIN LATERAL (
				SELECT draft.id, COALESCE(draft.version_no,'') AS version_no
				FROM %[1]s.production_bom_versions draft
				WHERE draft.bom_id=pb.id
				  AND draft.status='draft'
				  AND EXISTS (
				    SELECT 1
				    FROM %[1]s.production_bom_version_items draft_component
				    WHERE draft_component.version_id=draft.id
				  )
				ORDER BY draft.created_at DESC, draft.id DESC
				LIMIT 1
			) latest_nonempty_draft ON true
			WHERE COALESCE(NULLIF(pb.status,''),'active')='active'
			  AND v.status IN ('published','draft')
			  AND (v.status <> 'draft' OR EXISTS (
			    SELECT 1
			    FROM %[1]s.production_bom_version_items draft_component
			    WHERE draft_component.version_id=v.id
			  ))
		)
		SELECT bom_id, bom_code, bom_name, version_id, version_no, status, process_route_id, process_route_name,
		       component_count, latest_nonempty_draft_version_id, latest_nonempty_draft_version_no, is_default
		FROM pricing_rule_trial_bom_versions
		ORDER BY source_priority, is_default DESC,
		         CASE WHEN status='published' THEN 0 ELSE 1 END,
		         version_id DESC, bom_code, bom_name
	`, r.schema), input.ProductID)
	if err != nil {
		return out, err
	}
	defer bomRows.Close()
	for bomRows.Next() {
		var row appcosting.PricingRuleTrialBomVersionOption
		if err := bomRows.Scan(
			&row.BomID,
			&row.BomCode,
			&row.BomName,
			&row.VersionID,
			&row.VersionNo,
			&row.Status,
			&row.ProcessRouteID,
			&row.ProcessRouteName,
			&row.ComponentCount,
			&row.LatestNonEmptyDraftVersionID,
			&row.LatestNonEmptyDraftVersionNo,
			&row.IsDefault,
		); err != nil {
			return out, err
		}
		out.BomVersions = append(out.BomVersions, row)
	}
	if err := bomRows.Err(); err != nil {
		return out, err
	}
	versionIDs := make([]int64, 0, len(out.BomVersions))
	for _, version := range out.BomVersions {
		if version.VersionID > 0 {
			versionIDs = append(versionIDs, version.VersionID)
		}
	}
	if len(versionIDs) > 0 {
		hasSpecs, err := r.costingRelationExists(ctx, "production_bom_specs")
		if err != nil {
			return out, err
		}
		hasVariants, err := r.costingRelationExists(ctx, "production_bom_version_variants")
		if err != nil {
			return out, err
		}
		if hasSpecs && hasVariants {
			specRows, err := r.pool.Query(ctx, fmt.Sprintf(`
				SELECT v.bom_id,
				       v.id,
				       spec.id,
				       variant.id,
				       COALESCE(spec.spec_key,''),
				       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,''),
			       COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),''),
				       COALESCE(variant.is_default,false),
				       COALESCE(variant.sort_order,0)
				FROM %[1]s.production_bom_version_variants variant
				JOIN %[1]s.production_bom_versions v ON v.id=variant.version_id
				JOIN %[1]s.production_bom_specs spec ON spec.id=variant.bom_spec_id AND spec.bom_id=v.bom_id
				WHERE v.id=ANY($1)
				ORDER BY v.id, variant.is_default DESC, variant.sort_order, variant.id
			`, r.schema), versionIDs)
			if err != nil {
				return out, err
			}
			for specRows.Next() {
				var row appcosting.PricingRuleTrialBomSpecOption
				if err := specRows.Scan(&row.BomID, &row.VersionID, &row.BomSpecID, &row.BomVariantID, &row.SpecKey, &row.SpecName, &row.InventoryUnit, &row.IsDefault, &row.SortOrder); err != nil {
					specRows.Close()
					return out, err
				}
				out.BomSpecs = append(out.BomSpecs, row)
			}
			if err := specRows.Err(); err != nil {
				specRows.Close()
				return out, err
			}
			specRows.Close()
		}
	}

	defaultProcessRouteID := input.ProcessRouteID
	for _, row := range out.BomVersions {
		if row.IsDefault && row.ProcessRouteID > 0 {
			defaultProcessRouteID = row.ProcessRouteID
			break
		}
	}
	routeRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(name,''), '工艺路线 #' || id::text) AS name,
		       id=$1 AS is_default
		FROM %s.process_routes
		WHERE COALESCE(NULLIF(status,''),'active')='active'
		   OR id=$1
		ORDER BY CASE WHEN id=$1 THEN 0 ELSE 1 END, name, id
	`, r.schema), defaultProcessRouteID)
	if err != nil {
		return out, err
	}
	defer routeRows.Close()
	for routeRows.Next() {
		var row appcosting.PricingRuleTrialProcessRouteOption
		if err := routeRows.Scan(&row.ID, &row.Name, &row.IsDefault); err != nil {
			return out, err
		}
		out.ProcessRoutes = append(out.ProcessRoutes, row)
	}
	if err := routeRows.Err(); err != nil {
		return out, err
	}

	opRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(name,''), '工序模板 #' || id::text) AS name,
		       id=$1 AS is_default
		FROM %s.operation_templates
		WHERE active=true
		ORDER BY CASE WHEN id=$1 THEN 0 ELSE 1 END, name, id
	`, r.schema), input.OperationTemplateID)
	if err != nil {
		return out, err
	}
	defer opRows.Close()
	for opRows.Next() {
		var row appcosting.PricingRuleTrialOperationTemplateOption
		if err := opRows.Scan(&row.ID, &row.Name, &row.IsDefault); err != nil {
			return out, err
		}
		out.OperationTemplates = append(out.OperationTemplates, row)
	}
	if err := opRows.Err(); err != nil {
		return out, err
	}
	return out, nil
}

func (r Repository) LoadPricingRuleTrialBaseCostDetails(ctx context.Context, input domain.ProductInput) ([]appcosting.PricingRuleTrialBaseCostDetail, error) {
	if r.pool == nil || input.ProductID <= 0 {
		return nil, nil
	}
	resolvedBomCosts, err := r.loadResolvedProductionBomCosts(ctx)
	if err != nil {
		return nil, err
	}
	return r.loadPricingRuleTrialBaseCostDetails(ctx, input, resolvedBomCosts)
}

func (r Repository) LoadPricingRuleTrialBaseCostDetailsBatch(ctx context.Context, inputs []domain.ProductInput) ([][]appcosting.PricingRuleTrialBaseCostDetail, []error, error) {
	out := make([][]appcosting.PricingRuleTrialBaseCostDetail, len(inputs))
	errs := make([]error, len(inputs))
	if r.pool == nil || len(inputs) == 0 {
		return out, errs, nil
	}
	resolvedBomCosts, err := r.loadResolvedProductionBomCosts(ctx)
	if err != nil {
		return nil, nil, err
	}
	workerCount := len(inputs)
	if workerCount > 6 {
		workerCount = 6
	}
	jobs := make(chan int)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for index := range jobs {
				input := inputs[index]
				if input.ProductID <= 0 {
					continue
				}
				out[index], errs[index] = r.loadPricingRuleTrialBaseCostDetails(ctx, input, resolvedBomCosts)
			}
		}()
	}
	for index := range inputs {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	return out, errs, nil
}

func (r Repository) loadPricingRuleTrialBaseCostDetails(ctx context.Context, input domain.ProductInput, resolvedBomCosts map[int64]productionBomResolvedCost) ([]appcosting.PricingRuleTrialBaseCostDetail, error) {
	out := make([]appcosting.PricingRuleTrialBaseCostDetail, 0)
	issues := make([]appcosting.PricingRuleTrialCostIssue, 0)
	var err error
	resolvedParentCost, hasResolvedParentCost := productionBomCostForProduct(resolvedBomCosts, input.ProductID, input.ParentProductID, input.BomSpecID)
	if input.BomVersionID > 0 && resolvedParentCost.VersionID != input.BomVersionID {
		hasResolvedParentCost = false
	}
	if hasResolvedParentCost && len(resolvedParentCost.UnresolvedIssues) > 0 {
		issues = append(issues, resolvedParentCost.UnresolvedIssues...)
	}
	finish := func() ([]appcosting.PricingRuleTrialBaseCostDetail, error) {
		return finishPricingRuleTrialBaseCostDetails(out, input, resolvedParentCost, issues)
	}
	bomRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH material_valuation AS (
			SELECT l.material_id,
			       SUM((CASE
			         WHEN lower(btrim(COALESCE(NULLIF(m.unit,''), 'kg'))) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司')
			         THEN l.qty_g::numeric
			         ELSE l.qty_units::numeric
			       END) * COALESCE(b.unit_cost,0))
			       / NULLIF(SUM(CASE
			         WHEN lower(btrim(COALESCE(NULLIF(m.unit,''), 'kg'))) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司')
			         THEN l.qty_g::numeric
			         ELSE l.qty_units::numeric
			       END),0) AS weighted_unit_cost
			FROM %[1]s.material_batch_locations l
			JOIN %[1]s.material_batches b ON b.id = l.material_batch_id
			JOIN %[1]s.materials m ON m.id=l.material_id
			WHERE (l.qty_g > 0 OR l.qty_units > 0)
			  AND b.status='active'
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			GROUP BY l.material_id
		),
		pricing_rule_trial_selected_products AS (
			SELECT p.id AS product_id, 0 AS source_priority
			FROM %[1]s.products p
			WHERE p.id=$2 AND p.active=true
			UNION ALL
			SELECT p.parent_product_id AS product_id, 1 AS source_priority
			FROM %[1]s.products p
			WHERE p.id=$2 AND p.active=true AND COALESCE(p.parent_product_id,0)>0
		),
		default_bom AS (
			SELECT COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0) AS bom_version_id
			FROM pricing_rule_trial_selected_products selected
			LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=selected.product_id
			LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id=selected.product_id
			WHERE COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0)>0
			ORDER BY selected.source_priority
			LIMIT 1
		),
		output_bom_version AS (
			SELECT v.id
			FROM pricing_rule_trial_selected_products selected
			JOIN %[1]s.production_boms pb ON pb.output_product_id=selected.product_id
			JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id
			WHERE $1 <= 0
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			  AND v.status='published'
			ORDER BY selected.source_priority,
			         CASE WHEN COALESCE((SELECT bom_version_id FROM default_bom),0)>0 AND v.id=(SELECT bom_version_id FROM default_bom) THEN 0 ELSE 1 END,
			         v.published_at DESC NULLS LAST, v.id DESC
			LIMIT 1
		),
		bom_items AS (
			SELECT pbi.id, pbi.material_id, pbi.component_type, pbi.component_product_id,
			       COALESCE(pbi.component_bom_spec_id,0) AS component_bom_spec_id, pbi.component_spec_g,
			       pbi.consume_unit, pbi.qty_per_unit::float8, pbi.ratio_pct::float8,
			       CASE WHEN $3 > 0 THEN COALESCE(variant.material_loss_rate,0) ELSE COALESCE(pbi.material_loss_rate,0) END::float8 AS material_loss_rate,
			       pbi.unit_cost_snapshot::float8,
			       COALESCE(v.yield_rate,0)::float8 AS bom_yield_rate,
			       CASE WHEN $3 > 0 THEN 1 ELSE COALESCE(NULLIF(v.output_qty,0),1) END::float8 AS bom_output_qty,
			       CASE WHEN $3 > 0 THEN COALESCE(NULLIF(variant.inventory_unit,''),'') ELSE COALESCE(NULLIF(v.output_unit,''),'unit') END AS bom_output_unit
			FROM %[1]s.production_bom_version_items pbi
			JOIN %[1]s.production_bom_versions v ON v.id=pbi.version_id
			JOIN %[1]s.production_boms pb ON pb.id=v.bom_id
			LEFT JOIN %[1]s.production_bom_version_variants variant
			  ON variant.id=pbi.variant_id AND variant.version_id=v.id
			JOIN pricing_rule_trial_selected_products selected ON selected.product_id=pb.output_product_id
			WHERE COALESCE(NULLIF(pb.status,''),'active')='active'
			  AND (($1 > 0 AND pbi.version_id=$1 AND v.status IN ('published','draft'))
			    OR ($1 <= 0 AND pbi.version_id=(SELECT id FROM output_bom_version) AND v.status='published'))
			  AND ($3 <= 0 OR pbi.variant_id=$3)
		)
		SELECT bi.id,
		       COALESCE(NULLIF(bi.component_type,''),'material') AS component_type,
		       COALESCE(bi.material_id,0) AS material_id,
		       COALESCE(m.is_semi_finished,false) AS is_semi_finished,
		       COALESCE(bi.component_product_id,0) AS component_product_id,
		       COALESCE(bi.component_bom_spec_id,0) AS component_bom_spec_id,
		       COALESCE(bi.component_spec_g,0) AS component_spec_g,
		       COALESCE(NULLIF(m.name,''), NULLIF(cp.name,''), 'BOM项目') AS name,
		       COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') AS consume_unit,
		       COALESCE(bi.qty_per_unit,0)::float8,
		       COALESCE(bi.ratio_pct,0)::float8,
		       COALESCE(bi.material_loss_rate,0)::float8,
		       CASE WHEN COALESCE(m.is_semi_finished,false) THEN 0 ELSE COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0) END::float8 AS unit_cost,
		       COALESCE(NULLIF(m.unit,''),'kg') AS unit_cost_unit,
		       COALESCE(bi.bom_yield_rate,0)::float8 AS bom_yield_rate,
		       COALESCE(NULLIF(bi.bom_output_qty,0),1)::float8 AS bom_output_qty,
		       COALESCE(NULLIF(bi.bom_output_unit,''), CASE WHEN $3 > 0 THEN '' ELSE 'unit' END) AS bom_output_unit,
		       COALESCE(m.purchase_price,0)::float8 AS purchase_price,
		       COALESCE(mv.weighted_unit_cost,0)::float8 AS weighted_batch_unit_cost,
		       COALESCE(bi.unit_cost_snapshot,0)::float8 AS unit_cost_snapshot,
		       CASE
		         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct')='ratio_pct'
		         THEN (CASE WHEN COALESCE(m.is_semi_finished,false) THEN 0 ELSE COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0) END) * COALESCE(bi.ratio_pct,0) / 100.0 / (1 - LEAST(GREATEST(COALESCE(bi.material_loss_rate,0),0),0.9999))
		         ELSE 0
		       END::float8 AS amount_per_kg,
		       CASE
		         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct')='g_per_bag'
		         THEN COALESCE(bi.qty_per_unit,0) / 1000.0 * (CASE WHEN COALESCE(m.is_semi_finished,false) THEN 0 ELSE COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0) END)
		         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct')='g'
		         THEN COALESCE(bi.qty_per_unit,0) / 1000.0 * (CASE WHEN COALESCE(m.is_semi_finished,false) THEN 0 ELSE COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0) END)
		         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct')='kg'
		         THEN COALESCE(bi.qty_per_unit,0) * (CASE WHEN COALESCE(m.is_semi_finished,false) THEN 0 ELSE COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0) END)
		         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') IN ('unit_per_bag','unit_per_box','fixed_qty','unit','length','area')
		         THEN COALESCE(bi.qty_per_unit,0) * (CASE WHEN COALESCE(m.is_semi_finished,false) THEN 0 ELSE COALESCE(NULLIF(m.purchase_price,0), NULLIF(mv.weighted_unit_cost,0), NULLIF(bi.unit_cost_snapshot,0), 0) END)
		         ELSE 0
		       END::float8 AS amount_per_unit
		FROM bom_items bi
		LEFT JOIN %[1]s.materials m ON m.id=bi.material_id
		LEFT JOIN material_valuation mv ON mv.material_id=bi.material_id
		LEFT JOIN %[1]s.products cp ON cp.id=bi.component_product_id
		ORDER BY bi.id
	`, r.schema), input.BomVersionID, input.ProductID, input.BomVariantID)
	if err != nil {
		return nil, err
	}
	defer bomRows.Close()
	for bomRows.Next() {
		var row appcosting.PricingRuleTrialBaseCostDetail
		var id int64
		var componentType string
		var componentMaterialID int64
		var componentIsSemi bool
		var componentProductID int64
		var componentBomSpecID int64
		var componentSpecG int64
		var unitCostUnit string
		var bomYieldRate float64
		var bomOutputQty float64
		var bomOutputUnit string
		var purchasePrice, weightedBatchUnitCost, unitCostSnapshot float64
		if err := bomRows.Scan(&id, &componentType, &componentMaterialID, &componentIsSemi, &componentProductID, &componentBomSpecID, &componentSpecG, &row.Name, &row.ConsumeUnit, &row.Quantity, &row.RatioPct, &row.MaterialLossRate, &row.UnitCost, &unitCostUnit, &bomYieldRate, &bomOutputQty, &bomOutputUnit, &purchasePrice, &weightedBatchUnitCost, &unitCostSnapshot, &row.AmountPerKg, &row.AmountPerUnit); err != nil {
			return nil, err
		}
		row.ComponentID = id
		row.BomID = resolvedParentCost.BomID
		row.BomName = resolvedParentCost.BomName
		row.BomVersionID = resolvedParentCost.VersionID
		row.BomVersionNo = resolvedParentCost.VersionNo
		row.BomSpecID = input.BomSpecID
		row.BomVariantID = input.BomVariantID
		resolvedItemCost, resolvedFromGraph := resolvedParentCost.ItemCosts[id]
		ok := hasResolvedParentCost && resolvedFromGraph && resolvedItemCost.CostUnit != ""
		warning := ""
		if !ok {
			resolvedItemCost, ok, warning = resolveProductionBomTrialItemCost(productionBomCostItem{
				ID:                    id,
				ComponentType:         componentType,
				ComponentMaterialID:   componentMaterialID,
				ComponentIsSemi:       componentIsSemi,
				ComponentProductID:    componentProductID,
				ComponentBomSpecID:    componentBomSpecID,
				ComponentSpecG:        componentSpecG,
				ConsumeUnit:           row.ConsumeUnit,
				QtyPerUnit:            row.Quantity,
				RatioPct:              row.RatioPct,
				MaterialLossRate:      row.MaterialLossRate,
				ComponentName:         row.Name,
				PurchasePrice:         purchasePrice,
				WeightedBatchUnitCost: weightedBatchUnitCost,
				UnitCostSnapshot:      unitCostSnapshot,
			}, row.UnitCost, unitCostUnit, bomYieldRate, bomOutputQty, bomOutputUnit, resolvedBomCosts)
		}
		if ok {
			if massFactor := productionBomCostMassKgFactor(bomOutputUnit); massFactor > 0 {
				row.AmountPerKg = resolvedItemCost.ContributionPerOutputUnit / massFactor
				row.AmountPerUnit = 0
				row.Unit = "kg"
			} else {
				row.AmountPerKg = 0
				row.AmountPerUnit = resolvedItemCost.ContributionPerOutputUnit
				row.Unit = bomOutputUnit
			}
			row.UnitCost = resolvedItemCost.UnitCost
			row.CostUnitCost = resolvedItemCost.UnitCost
			row.CostUnit = resolvedItemCost.CostUnit
		} else {
			if !(hasResolvedParentCost && len(resolvedParentCost.UnresolvedIssues) > 0 && (componentIsSemi || componentType == "product" || componentType == "finished_product")) {
				issues = append(issues, appcosting.PricingRuleTrialCostIssue{
					Code:                  "zero_component_cost",
					Reason:                warning,
					ComponentType:         componentType,
					ComponentID:           id,
					ComponentMaterialID:   componentMaterialID,
					ComponentProductID:    componentProductID,
					ComponentBomSpecID:    componentBomSpecID,
					ComponentName:         strings.TrimSpace(row.Name),
					IsSemiFinished:        componentIsSemi,
					ConsumeUnit:           row.ConsumeUnit,
					CostUnit:              unitCostUnit,
					Quantity:              row.Quantity,
					UnitCost:              row.UnitCost,
					PurchasePrice:         purchasePrice,
					WeightedBatchUnitCost: weightedBatchUnitCost,
					UnitCostSnapshot:      unitCostSnapshot,
					RootProductID:         input.ProductID,
					VersionID:             input.BomVersionID,
				})
			}
			bomID := row.BomID
			if bomID <= 0 {
				bomID = resolvedParentCost.BomID
			}
			row.Warning = warning
			row.Description = fmt.Sprintf("BOM %d / %s", bomID, firstNonEmptyString(row.BomVersionNo, resolvedParentCost.VersionNo, fmt.Sprintf("version-%d", input.BomVersionID)))
			row.Type = "material"
			if componentType == "product" || componentType == "finished_product" {
				row.Type = "component_product"
			}
			row.Key = fmt.Sprintf("%s:%d", row.Type, id)
			row.Unit = firstNonEmptyString(unitCostUnit, input.InventoryUnit, bomOutputUnit)
			out = append(out, row)
			continue
		}
		if strings.TrimSpace(row.ConsumeUnit) == "ratio_pct" && row.MaterialLossRate > 0 && row.MaterialLossRate < 1 {
			row.RecipeRatioPct = row.RatioPct
			row.EffectiveRatioPct = row.RatioPct / (1 - row.MaterialLossRate)
			row.RatioPct = row.EffectiveRatioPct
		} else if strings.TrimSpace(row.ConsumeUnit) == "ratio_pct" {
			row.RecipeRatioPct = row.RatioPct
			row.EffectiveRatioPct = row.RatioPct
		}
		row.Type = "material"
		if componentType == "product" || componentType == "finished_product" {
			row.Type = "component_product"
		}
		row.Key = fmt.Sprintf("%s:%d", row.Type, id)
		out = append(out, row)
	}
	if err := bomRows.Err(); err != nil {
		return nil, err
	}

	operationQuery := fmt.Sprintf(`
		WITH pricing_rule_trial_selected_products AS (
			SELECT p.id AS product_id, 0 AS source_priority
			FROM %[1]s.products p
			WHERE p.id=$2 AND p.active=true
			UNION ALL
			SELECT p.parent_product_id AS product_id, 1 AS source_priority
			FROM %[1]s.products p
			WHERE p.id=$2 AND p.active=true AND COALESCE(p.parent_product_id,0)>0
		),
		default_bom AS (
			SELECT COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0) AS bom_version_id
			FROM pricing_rule_trial_selected_products selected
			LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=selected.product_id
			LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id=selected.product_id
			WHERE COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0)>0
			ORDER BY selected.source_priority
			LIMIT 1
		),
		output_bom_version AS (
			SELECT v.id
			FROM pricing_rule_trial_selected_products selected
			JOIN %[1]s.production_boms pb ON pb.output_product_id=selected.product_id
			JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id
			WHERE $1 <= 0
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			  AND v.status='published'
			ORDER BY selected.source_priority,
			         CASE WHEN COALESCE((SELECT bom_version_id FROM default_bom),0)>0 AND v.id=(SELECT bom_version_id FROM default_bom) THEN 0 ELSE 1 END,
			         v.published_at DESC NULLS LAST, v.id DESC
			LIMIT 1
		),
		selected_bom_version AS (
			SELECT v.id
			FROM pricing_rule_trial_selected_products selected
			JOIN %[1]s.production_boms pb ON pb.output_product_id=selected.product_id
			JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id
			WHERE $1 > 0
			  AND v.id=$1
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			  AND v.status IN ('published','draft')
			UNION ALL
			SELECT id FROM output_bom_version WHERE $1 <= 0
		)
		SELECT oc.id,
		       COALESCE(oc.workstation_capacity_id,0) AS workstation_capacity_id,
		       COALESCE(NULLIF(oc.operation_name,''),'工序') AS operation_name,
		       COALESCE(oc.workstation_name,'') AS workstation_name,
		       COALESCE(oc.capacity_name,'') AS capacity_name,
		       COALESCE(oc.hourly_rate_snapshot,0)::float8,
		       COALESCE(oc.standard_minutes_snapshot,0)::float8,
		       COALESCE(oc.batch_size_qty_snapshot,0)::float8,
		       COALESCE(oc.batch_size_unit_snapshot,'') AS batch_size_unit,
		       COALESCE(NULLIF(oc.cost_method,''),'time') AS cost_method,
		       COALESCE(oc.piece_rate_snapshot,0)::float8 AS piece_rate,
		       COALESCE(NULLIF(oc.rate_unit_snapshot,''),'') AS rate_unit,
		       COALESCE(oc.operation_unit_cost,0)::float8,
		       COALESCE(NULLIF(oc.operation_cost_unit,''),'') AS operation_cost_unit
		FROM %[1]s.production_bom_version_operation_costs oc
		WHERE oc.version_id=(SELECT id FROM selected_bom_version)
		ORDER BY oc.sort_order, oc.id
	`, r.schema)
	operationArgs := []any{input.BomVersionID, input.ProductID}
	if input.BomVariantID > 0 {
		operationQuery = fmt.Sprintf(`
			SELECT operation.id,
			       COALESCE(operation.workstation_capacity_id,0) AS workstation_capacity_id,
			       COALESCE(NULLIF(operation.operation,''),'工序') AS operation_name,
			       COALESCE(operation.workstation,'') AS workstation_name,
			       COALESCE(operation.workstation_capacity_name,'') AS capacity_name,
			       COALESCE(operation.hourly_rate,0)::float8,
			       COALESCE(operation.standard_minutes,0)::float8,
			       COALESCE(operation.batch_size_qty,0)::float8,
			       COALESCE(operation.batch_size_unit,'') AS batch_size_unit,
			       COALESCE(NULLIF(operation.cost_method,''),'time') AS cost_method,
			       COALESCE(operation.piece_rate,0)::float8 AS piece_rate,
			       COALESCE(operation.batch_size_unit,'') AS rate_unit,
			       COALESCE(operation.planned_operation_cost,0)::float8,
			       COALESCE(NULLIF(variant.inventory_unit,''),'') AS operation_cost_unit
			FROM %[1]s.production_bom_version_variants variant
			JOIN %[1]s.process_route_operations operation
			  ON operation.route_id=variant.process_route_id
			WHERE variant.id=$1
			  AND ($2 <= 0 OR variant.version_id=$2)
			ORDER BY operation.seq,operation.id
		`, r.schema)
		operationArgs = []any{input.BomVariantID, input.BomVersionID}
	}
	opRows, err := r.pool.Query(ctx, operationQuery, operationArgs...)
	if err != nil {
		return nil, err
	}
	defer opRows.Close()
	operationSnapshotCount := 0
	for opRows.Next() {
		var row appcosting.PricingRuleTrialBaseCostDetail
		var id int64
		var capacityID int64
		if err := opRows.Scan(&id, &capacityID, &row.Name, &row.WorkstationName, &row.CapacityName, &row.HourlyRate, &row.StandardMinutes, &row.StandardOutputQty, &row.StandardOutputUnit, &row.CostMethod, &row.PieceRate, &row.RateUnit, &row.UnitCost, &row.Unit); err != nil {
			return nil, err
		}
		_ = capacityID
		unit := strings.TrimSpace(row.Unit)
		if unit == "" {
			unit = strings.TrimSpace(input.InventoryUnit)
		}
		if unit == "" {
			unit = "kg"
		}
		row.Key = fmt.Sprintf("operation:bom_snapshot:%d", id)
		row.Type = "operation"
		row.TypeLabel = "标准工序"
		row.ConsumeUnit = "per_inventory_unit"
		row.Quantity = 1
		if strings.EqualFold(strings.TrimSpace(row.CostMethod), "piece") {
			row.ConsumeUnit = "per_sales_unit"
			row.AmountPerUnit = row.PieceRate
			row.UnitCost = row.PieceRate
			row.Unit = firstNonEmptyString(strings.TrimSpace(input.QuoteUnit), strings.TrimSpace(input.OrderUnit))
		}
		row.Unit = unit
		row.CostUnit = unit
		row.CostUnitCost = row.UnitCost
		row.AmountPerUnit = row.UnitCost
		row.CapacitySelectionSource = "bom_operation_snapshot"
		if strings.EqualFold(strings.TrimSpace(row.CostMethod), "piece") {
			row.Unit = firstNonEmptyString(strings.TrimSpace(input.QuoteUnit), strings.TrimSpace(input.OrderUnit), unit)
			row.CostUnit = row.Unit
			row.CostUnitCost = row.PieceRate
			row.Description = fmt.Sprintf("标准工序成本来自 BOM 计件工序成本快照：%s · %s · %.4f元/销售规格件", row.WorkstationName, row.CapacityName, row.PieceRate)
		} else {
			row.Description = fmt.Sprintf("标准工序成本来自 BOM 工序成本快照：%s · %s · %.4f/%s", row.WorkstationName, row.CapacityName, row.UnitCost, unit)
		}
		out = append(out, row)
		operationSnapshotCount++
	}
	if err := opRows.Err(); err != nil {
		return nil, err
	}
	if input.ProcessRouteID > 0 && operationSnapshotCount == 0 {
		unit := strings.TrimSpace(input.InventoryUnit)
		if unit == "" {
			unit = "kg"
		}
		out = append(out, appcosting.PricingRuleTrialBaseCostDetail{
			Key:                     fmt.Sprintf("operation:bom_operation_snapshot_missing:%d", input.ProcessRouteID),
			Type:                    "operation",
			TypeLabel:               "标准工序",
			Name:                    "BOM工序成本快照缺失",
			ConsumeUnit:             "per_inventory_unit",
			Unit:                    unit,
			CapacitySelectionSource: "bom_operation_snapshot_missing",
			Warning:                 "请先发布包含标准成本产能档快照的 BOM",
			Description:             "BOM 已绑定工艺路线，但未找到冻结的工序成本快照",
		})
	}
	if input.ProcessRouteID > 0 || operationSnapshotCount > 0 {
		return finish()
	}
	if input.OperationTemplateID <= 0 {
		return out, nil
	}
	legacyOpRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(operation,''), NULLIF(workstation,''), '工序') AS name,
		       COALESCE(NULLIF(cost_type,''),'fixed') AS cost_type,
		       COALESCE(cost_rate,0)::float8
		FROM %s.operation_template_steps
		WHERE template_id=$1 AND active=true
		ORDER BY position, id
	`, r.schema), input.OperationTemplateID)
	if err != nil {
		return nil, err
	}
	defer legacyOpRows.Close()
	for legacyOpRows.Next() {
		var row appcosting.PricingRuleTrialBaseCostDetail
		var id int64
		if err := legacyOpRows.Scan(&id, &row.Name, &row.ConsumeUnit, &row.UnitCost); err != nil {
			return nil, err
		}
		row.Key = fmt.Sprintf("operation:%d", id)
		row.Type = "operation"
		row.CapacitySelectionSource = "operation_template"
		row.Quantity = 1
		switch row.ConsumeUnit {
		case "per_kg", "per_kg_output", "per_finished_kg":
			row.AmountPerKg = row.UnitCost
		default:
			row.AmountPerUnit = row.UnitCost
		}
		out = append(out, row)
	}
	if err := legacyOpRows.Err(); err != nil {
		return nil, err
	}
	return finish()
}

func finishPricingRuleTrialBaseCostDetails(out []appcosting.PricingRuleTrialBaseCostDetail, input domain.ProductInput, resolved productionBomResolvedCost, issues []appcosting.PricingRuleTrialCostIssue) ([]appcosting.PricingRuleTrialBaseCostDetail, error) {
	if len(issues) == 0 {
		return out, nil
	}
	partialCost := resolved.PartialTotalCostPerOutputUnit
	if !finiteNonNegative(partialCost) || partialCost <= 0 {
		for _, detail := range out {
			if detail.Type == "operation" {
				partialCost += detail.AmountPerKg
				if detail.AmountPerKg == 0 {
					partialCost += detail.AmountPerUnit
				}
				continue
			}
			partialCost += detail.AmountPerKg
			if detail.AmountPerKg == 0 {
				partialCost += detail.AmountPerUnit
			}
		}
	}
	if !finiteNonNegative(partialCost) {
		partialCost = 0
	}
	unique := make([]appcosting.PricingRuleTrialCostIssue, 0, len(issues))
	seen := map[string]struct{}{}
	for _, issue := range issues {
		if issue.ComponentID == 0 && issue.Reason == "" {
			continue
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%s", issue.Code, issue.ComponentID, issue.ComponentMaterialID, issue.ComponentProductID, issue.Reason)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if issue.RootProductID == 0 {
			issue.RootProductID = input.ProductID
		}
		if issue.VersionID == 0 {
			issue.VersionID = resolved.VersionID
		}
		if issue.BomID == 0 {
			issue.BomID = resolved.BomID
		}
		if issue.BomName == "" {
			issue.BomName = resolved.BomName
		}
		if issue.VersionNo == "" {
			issue.VersionNo = resolved.VersionNo
		}
		if issue.BomSpecID == 0 {
			issue.BomSpecID = input.BomSpecID
		}
		if issue.BomVariantID == 0 {
			issue.BomVariantID = input.BomVariantID
		}
		unique = append(unique, issue)
	}
	return out, &appcosting.PricingRuleTrialCostIncompleteError{
		ProductID:       input.ProductID,
		BomID:           resolved.BomID,
		BomName:         resolved.BomName,
		BomVersionID:    resolved.VersionID,
		BomVersionNo:    resolved.VersionNo,
		BomSpecID:       input.BomSpecID,
		BomVariantID:    input.BomVariantID,
		PartialCost:     partialCost,
		Issues:          unique,
		BaseCostDetails: out,
	}
}

func (r Repository) loadProductInputs(ctx context.Context, params domain.Parameters, customerID int64) ([]domain.ProductInput, error) {
	q := fmt.Sprintf(`
		WITH material_valuation AS (
			SELECT l.material_id,
			       SUM((CASE
			         WHEN lower(btrim(COALESCE(NULLIF(m.unit,''), 'kg'))) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司')
			         THEN l.qty_g::numeric
			         ELSE l.qty_units::numeric
			       END) * COALESCE(b.unit_cost,0))
			       / NULLIF(SUM(CASE
			         WHEN lower(btrim(COALESCE(NULLIF(m.unit,''), 'kg'))) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司')
			         THEN l.qty_g::numeric
			         ELSE l.qty_units::numeric
			       END),0) AS weighted_unit_cost
			FROM %[1]s.material_batch_locations l
			JOIN %[1]s.material_batches b ON b.id = l.material_batch_id
			JOIN %[1]s.materials m ON m.id=l.material_id
			WHERE (l.qty_g > 0 OR l.qty_units > 0)
			  AND b.status='active'
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			GROUP BY l.material_id
		),
		product_scope AS (
			SELECT p.*,
			       COALESCE(cpa.id,0) AS customer_product_alias_id,
			       COALESCE(NULLIF(cpa.brand_name,''), NULLIF(cpa.display_name,''), p.name) AS customer_product_display_name,
			       COALESCE(cpa.customer_item_code,'') AS customer_item_code,
			       COALESCE(cpa.brand_name,'') AS brand_name,
			       COALESCE(cpa.display_category_id,0) AS display_category_id,
			       COALESCE(alias_pc.name,'') AS display_category_name,
			       COALESCE(cpa.product_config_template_id,0) AS customer_product_alias_product_config_template_id,
			       COALESCE(cpa.gradient_template_id,0) AS customer_product_alias_gradient_template_id,
			       COALESCE(cpa.unit_template_id,0) AS customer_product_alias_unit_template_id,
			       COALESCE(CASE WHEN $1 > 0 THEN alias_class.template_id ELSE product_class.template_id END,0) AS current_classification_template_id,
			       COALESCE(CASE WHEN $1 > 0 THEN alias_ct.name ELSE product_ct.name END,'') AS current_classification_template_name,
			       COALESCE(CASE WHEN $1 > 0 THEN alias_class.category_id ELSE product_class.category_id END,0) AS current_classification_category_id,
			       COALESCE(CASE
			         WHEN $1 > 0 AND COALESCE(alias_class.template_id,0) > 0 AND COALESCE(alias_class.category_id,0)=0 THEN '未分类'
			         WHEN $1 > 0 THEN alias_cc.name
			         WHEN COALESCE(product_class.template_id,0) > 0 AND COALESCE(product_class.category_id,0)=0 THEN '未分类'
			         ELSE product_cc.name
			       END,'') AS current_classification_category_name,
			       COALESCE(CASE WHEN $1 > 0 THEN alias_cc.product_config_template_id ELSE product_cc.product_config_template_id END,0) AS current_classification_category_product_config_template_id,
			       COALESCE(CASE WHEN $1 > 0 THEN alias_ct.product_config_template_id ELSE product_ct.product_config_template_id END,0) AS current_classification_template_product_config_template_id,
			       CASE WHEN output_bom.bom_id IS NOT NULL THEN 'production_bom_output' ELSE COALESCE(NULLIF(bs.source_type,''), '') END AS bom_usage_mode,
			       COALESCE(NULLIF(output_bom.bom_id,0),0) AS production_bom_id,
			       COALESCE(NULLIF(output_bom.bom_version_id,0),0) AS production_bom_version_id,
			       0::float8 AS expected_loss_rate,
			       1::float8 AS production_config_yield_rate,
			       COALESCE(NULLIF(ppc.process_route_id,0),0) AS process_route_id,
			       CASE
			         WHEN COALESCE(NULLIF(p.product_kind,''),'roasted')='green_bean'
			          AND COALESCE(p.green_bean_bom_product_id,0) > 0
			         THEN p.green_bean_bom_product_id
			         WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version')
			          AND COALESCE(bs.source_product_id,0) > 0
			         THEN bs.source_product_id
			         ELSE p.id
			       END AS bom_product_id
			FROM %[1]s.products p
			LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id = p.id
			LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id = p.id
			LEFT JOIN %[1]s.product_bom_sources bs ON bs.product_id = p.id
			LEFT JOIN LATERAL (
				SELECT pb.id AS bom_id, latest.id AS bom_version_id
				FROM %[1]s.production_boms pb
				JOIN LATERAL (
					SELECT v.id, v.published_at
					FROM %[1]s.production_bom_versions v
					WHERE v.bom_id=pb.id AND v.status='published'
					ORDER BY v.published_at DESC NULLS LAST, v.id DESC
					LIMIT 1
				) latest ON true
				WHERE pb.output_product_id=p.id
				  AND COALESCE(NULLIF(pb.status,''),'active')='active'
				ORDER BY CASE WHEN COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0)>0
				                    AND latest.id=COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0)
				              THEN 0 ELSE 1 END,
				         latest.published_at DESC NULLS LAST, latest.id DESC, pb.id DESC
				LIMIT 1
			) output_bom ON true
			LEFT JOIN %[1]s.customer_product_aliases cpa
			  ON $1 > 0
			 AND cpa.product_id = p.id
			 AND cpa.customer_id = $1
			 AND cpa.active=true
			 AND cpa.include_in_price_list=true
			LEFT JOIN %[1]s.product_categories alias_pc ON alias_pc.id=cpa.display_category_id AND alias_pc.active=true
			LEFT JOIN %[1]s.product_classification_assignments product_class
			  ON $1 <= 0
			 AND product_class.product_id = p.id
			LEFT JOIN %[1]s.product_classification_templates product_ct
			  ON product_ct.id = product_class.template_id
			 AND product_ct.active=true
			LEFT JOIN %[1]s.product_classification_template_categories product_cc
			  ON product_cc.id = product_class.category_id
			 AND product_cc.template_id = product_class.template_id
			 AND product_cc.active=true
			LEFT JOIN %[1]s.customer_product_alias_classification_assignments alias_class
			  ON $1 > 0
			 AND alias_class.alias_id = cpa.id
			LEFT JOIN %[1]s.product_classification_templates alias_ct
			  ON alias_ct.id = alias_class.template_id
			 AND alias_ct.active=true
			LEFT JOIN %[1]s.product_classification_template_categories alias_cc
			  ON alias_cc.id = alias_class.category_id
			 AND alias_cc.template_id = alias_class.template_id
			 AND alias_cc.active=true
			WHERE p.active = true
			  AND (COALESCE(p.parent_product_id,0)=0 OR EXISTS (
				SELECT 1 FROM %[1]s.products active_parent
				WHERE active_parent.id=p.parent_product_id AND active_parent.active=true
			  ))
			  AND (NOT COALESCE(p.auto_derived_sku,false) OR COALESCE(NULLIF(p.derived_spec_status,''),'active')<>'template_removed')
			  AND (($1 <= 0 AND COALESCE(p.customer_id,0)=0) OR ($1 > 0 AND cpa.id IS NOT NULL))
		),
		all_effective_bom_items AS (
			SELECT p.id AS product_id,
			       pbi.material_id, pbi.component_type, pbi.component_product_id, pbi.component_spec_g,
			       pbi.consume_unit, pbi.qty_per_unit, pbi.ratio_pct, COALESCE(pbi.material_loss_rate,0)::float8 AS material_loss_rate, pbi.unit_cost_snapshot
			FROM %[1]s.products p
			LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=p.id
			LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id=p.id
			JOIN LATERAL (
				SELECT latest.id AS bom_version_id
				FROM %[1]s.production_boms pb
				JOIN LATERAL (
					SELECT v.id, v.published_at, v.created_at
					FROM %[1]s.production_bom_versions v
					WHERE v.bom_id=pb.id AND v.status='published'
					ORDER BY v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC
					LIMIT 1
				) latest ON true
				WHERE pb.output_product_id=p.id
				  AND COALESCE(NULLIF(pb.status,''),'active')='active'
				ORDER BY CASE WHEN COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0)>0
				                    AND latest.id=COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0)
				              THEN 0 ELSE 1 END,
				         latest.published_at DESC NULLS LAST, latest.created_at DESC, latest.id DESC, pb.id DESC
				LIMIT 1
			) output_bom ON true
			JOIN %[1]s.production_bom_version_items pbi ON pbi.version_id=output_bom.bom_version_id
			WHERE p.active=true
			UNION ALL
			SELECT p.id AS product_id,
			       bi.material_id, bi.component_type, bi.component_product_id, bi.component_spec_g,
			       bi.consume_unit, bi.qty_per_unit, bi.ratio_pct, 0::float8 AS material_loss_rate, bi.unit_cost_snapshot
			FROM %[1]s.products p
			LEFT JOIN %[1]s.product_bom_sources bs ON bs.product_id=p.id
			JOIN %[1]s.product_bom_items bi ON bi.product_id=CASE
				WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
				ELSE p.id
			END
			WHERE p.active=true
			  AND NOT EXISTS (
			    SELECT 1
			    FROM %[1]s.production_boms pb
			    JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id
			    WHERE pb.output_product_id=p.id
			      AND COALESCE(NULLIF(pb.status,''),'active')='active'
			      AND v.status='published'
			  )
		),
		effective_bom_items AS (
			SELECT p.id AS product_id,
			       pbi.material_id, pbi.component_type, pbi.component_product_id, pbi.component_spec_g,
			       pbi.consume_unit, pbi.qty_per_unit, pbi.ratio_pct, COALESCE(pbi.material_loss_rate,0)::float8 AS material_loss_rate, pbi.unit_cost_snapshot
			FROM product_scope p
			JOIN %[1]s.production_bom_version_items pbi ON pbi.version_id=p.production_bom_version_id
			UNION ALL
			SELECT p.id AS product_id,
			       bi.material_id, bi.component_type, bi.component_product_id, bi.component_spec_g,
			       bi.consume_unit, bi.qty_per_unit, bi.ratio_pct, 0::float8 AS material_loss_rate, bi.unit_cost_snapshot
			FROM product_scope p
			JOIN %[1]s.product_bom_items bi ON p.production_bom_version_id=0 AND bi.product_id=p.bom_product_id
		),
		finished_product_cost AS (
			SELECT p.id AS product_id,
			       COALESCE(SUM(COALESCE(mv.weighted_unit_cost, m.purchase_price, 0) * COALESCE(bi.ratio_pct,0) / 100.0 / (1 - LEAST(GREATEST(COALESCE(bi.material_loss_rate,0),0),0.9999))),0) AS green_cost_per_kg
			FROM %[1]s.products p
			LEFT JOIN all_effective_bom_items bi ON bi.product_id = p.id
				AND COALESCE(NULLIF(bi.component_type,''),'material') = 'material'
				AND COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') = 'ratio_pct'
			LEFT JOIN %[1]s.materials m ON m.id = bi.material_id
			LEFT JOIN material_valuation mv ON mv.material_id = m.id
			WHERE p.active = true
			GROUP BY p.id
		),
		bom_unit_cost AS (
			SELECT p.id AS product_id,
			       COALESCE(SUM(CASE
			         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') = 'g_per_bag'
			         THEN COALESCE(bi.qty_per_unit,0) / 1000.0 * COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0)
			         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') IN ('unit_per_bag','unit_per_box')
			         THEN COALESCE(bi.qty_per_unit,0) * COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0)
			         WHEN COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct') NOT IN ('ratio_pct','g_per_bag')
			          AND lower(btrim(COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct'))) = lower(btrim(COALESCE(NULLIF(m.unit,''), '')))
			         THEN COALESCE(bi.qty_per_unit,0) * COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(bi.unit_cost_snapshot,0), 0)
			         ELSE 0
			       END),0) AS bom_cost_per_unit
			FROM product_scope p
			LEFT JOIN effective_bom_items bi ON bi.product_id = p.id
				AND COALESCE(NULLIF(bi.component_type,''),'material') = 'material'
			LEFT JOIN %[1]s.materials m ON m.id = bi.material_id
			LEFT JOIN material_valuation mv ON mv.material_id = m.id
			GROUP BY p.id
		),
		finished_component_cost AS (
			SELECT bi.product_id,
			       SUM(COALESCE(fpc.green_cost_per_kg,0) * COALESCE(NULLIF(bi.qty_per_unit,0), NULLIF(bi.component_spec_g,0), 1))
			       / NULLIF(SUM(COALESCE(NULLIF(bi.qty_per_unit,0), NULLIF(bi.component_spec_g,0), 1)),0) AS finished_green_cost_per_kg
			FROM all_effective_bom_items bi
			JOIN finished_product_cost fpc ON fpc.product_id = bi.component_product_id
			WHERE COALESCE(NULLIF(bi.component_type,''),'material') = 'finished_product'
			GROUP BY bi.product_id
		),
		operation_unit_cost AS (
			SELECT template_id,
			       COALESCE(SUM(CASE
			         WHEN COALESCE(NULLIF(cost_type,''),'fixed') IN ('fixed','per_unit','per_quote_unit')
			         THEN COALESCE(cost_rate,0)
			         ELSE 0
			       END),0) AS operation_cost_per_unit,
			       COALESCE(SUM(CASE
			         WHEN COALESCE(NULLIF(cost_type,''),'fixed') IN ('per_kg','per_kg_output','per_finished_kg')
			         THEN COALESCE(cost_rate,0)
			         ELSE 0
			       END),0) AS operation_cost_per_kg
			FROM %[1]s.operation_template_steps
			WHERE active=true
			GROUP BY template_id
		),
		production_config_attrs AS (
			SELECT ppcf.product_id,
			       COALESCE(jsonb_object_agg(ppcf.field_key,
			         COALESCE(to_jsonb(ppcf.value_text), to_jsonb(ppcf.value_number), to_jsonb(ppcf.value_bool), 'null'::jsonb)
			       ) FILTER (WHERE ppcf.show_in_price_list=true AND NULLIF(ppcf.field_key,'') IS NOT NULL), '{}'::jsonb) AS production_config_attrs_json,
			       COALESCE(jsonb_agg(jsonb_build_object(
			         'key', ppcf.field_key,
			         'label', ppcf.label,
			         'field_type', ppcf.field_type,
			         'unit', ppcf.unit,
			         'show_in_price_list', ppcf.show_in_price_list,
			         'sort_order', ppcf.sort_order
			       ) ORDER BY ppcf.sort_order, ppcf.id) FILTER (WHERE ppcf.show_in_price_list=true AND NULLIF(ppcf.field_key,'') IS NOT NULL), '[]'::jsonb) AS production_config_attrs_schema_json
			FROM %[1]s.product_production_config_fields ppcf
			GROUP BY ppcf.product_id
		),
		alias_config_attrs AS (
			SELECT cpa.id AS alias_id,
			       COALESCE(jsonb_object_agg(ppcf.field_key,
			         to_jsonb(COALESCE(NULLIF(cpaf.value_text,''), ppcf.value_text, ''))
			       ) FILTER (WHERE ppcf.show_in_price_list=true AND NULLIF(ppcf.field_key,'') IS NOT NULL), '{}'::jsonb) AS alias_attrs_json
			FROM %[1]s.customer_product_aliases cpa
			JOIN %[1]s.product_production_config_fields ppcf ON ppcf.product_id=cpa.product_id
			LEFT JOIN %[1]s.customer_product_alias_industry_field_values cpaf
			  ON cpaf.alias_id=cpa.id AND lower(cpaf.field_key)=lower(ppcf.field_key)
			WHERE $1 > 0
			  AND cpa.customer_id=$1
			  AND cpa.active=true
			  AND cpa.include_in_price_list=true
			GROUP BY cpa.id
		)
		SELECT p.id,
		       p.id AS sku_id,
		       COALESCE(p.parent_product_id,0) AS parent_product_id,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(p.parent_product_id,0) ELSE p.id END AS effective_parent_product_id,
		       CASE
		         WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(NULLIF(p.sku_name,''), p.name)
		         ELSE COALESCE(NULLIF(NULLIF(p.sku_name,''),'默认规格'), NULLIF(product_unit_template_default_spec.spec_name,''), '默认规格')
		       END AS sku_name,
		       COALESCE(p.sku_code,'') AS sku_code,
		       COALESCE(p.barcode,'') AS barcode,
		       CASE
		         WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(p.derived_spec_key,'')
		         ELSE COALESCE(product_unit_template_default_spec.spec_key,'')
		       END AS derived_spec_key,
		       COALESCE(p.spec_label,'') AS spec_label,
		       COALESCE(NULLIF(p.net_content_qty,0), NULLIF(product_unit_template_default_spec.net_content_qty,0), 0)::float8 AS net_content_qty,
		       COALESCE(NULLIF(p.net_content_unit,''), NULLIF(product_unit_template_default_spec.net_content_unit,''), '') AS net_content_unit,
		       COALESCE(p.id=COALESCE(NULLIF(parent_product.default_sku_id,0), parent_product.id), false) AS is_default_sku,
		       COALESCE(NULLIF(parent_product.default_sku_id,0), parent_product.id, 0) AS default_sku_id,
		       CASE WHEN $1 > 0 THEN COALESCE(NULLIF(p.customer_product_display_name,''), p.name) ELSE p.name END,
		       'SKU-' || p.id::text,
		       p.name,
		       COALESCE(p.customer_product_alias_id,0),
		       COALESCE(p.customer_product_display_name,''),
		       COALESCE(p.customer_item_code,''),
		       COALESCE(p.brand_name,''),
		       COALESCE(p.display_category_id,0),
		       COALESCE(p.display_category_name,''),
		       COALESCE(p.current_classification_template_id,0),
		       COALESCE(p.current_classification_template_name,''),
		       COALESCE(p.current_classification_category_id,0),
		       COALESCE(p.current_classification_category_name,''),
		       COALESCE(base_p.name, p.name),
		       COALESCE(p.roast_level, ''),
		       COALESCE(CASE WHEN $1 > 0 THEN NULLIF(alias_attrs.alias_attrs_json::text,'{}') ELSE NULL END, NULLIF(pca.production_config_attrs_json::text,'{}'), NULLIF(current_bv.special_attrs_json::text,'{}'), NULLIF(p.special_attrs_json::text,'{}'), '{}'),
		       CASE WHEN $1 > 0 THEN $1 ELSE COALESCE(p.customer_id, 0) END,
		       COALESCE(p.base_product_id, 0),
		       COALESCE(NULLIF(p.visibility, ''), 'public'),
		       COALESCE(p.custom_type, ''),
		       COALESCE(NULLIF(p.product_kind,''), 'roasted'),
		       COALESCE(p.drip_bag_grams, 10)::float8,
		       COALESCE(p.drip_box_bag_count, 10),
		       COALESCE(p.product_category_id, 0),
		       COALESCE(p.product_category_position, 0),
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(parent_pc.id,0) ELSE COALESCE(pc.id,0) END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.id,0) ELSE 0 END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(parent_pc.name,'') ELSE COALESCE(pc.name,'') END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(parent_pc.position,0) ELSE COALESCE(pc.position,0) END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.name,'') ELSE '' END,
		       CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.position,0) ELSE 0 END,
		       COALESCE(
			           NULLIF(alias_config.gradient_template_id,0),
			           NULLIF(p_config.gradient_template_id,0),
			           NULLIF(classification_category_config.gradient_template_id,0),
			           NULLIF(classification_template_config.gradient_template_id,0),
			           NULLIF(p.customer_product_alias_gradient_template_id,0),
			           NULLIF(cpro.gradient_template_id,0),
			           NULLIF(cpti.gradient_template_id,0),
			           NULLIF(p.gradient_template_id_override,0),
		           0
		       ) AS effective_gradient_template_id,
		       COALESCE(
			           NULLIF(alias_config.operation_template_id,0),
			           NULLIF(p_config.operation_template_id,0),
			           NULLIF(classification_category_config.operation_template_id,0),
			           NULLIF(classification_template_config.operation_template_id,0),
			           NULLIF(cpro.operation_template_id,0),
			           NULLIF(cpti.operation_template_id,0),
			           NULLIF(p.operation_template_id_override,0),
			           NULLIF(pc.operation_template_id,0),
		           NULLIF(parent_pc.operation_template_id,0),
		           0
		       ) AS effective_operation_template_id,
		       COALESCE(NULLIF(p.process_route_id,0),0) AS effective_process_route_id,
		       COALESCE(NULLIF(product_process_route.name,''), '') AS effective_process_route_name,
		       COALESCE(
			           NULLIF(alias_config.price_list_rule_json::text,'{}'),
			           NULLIF(p_config.price_list_rule_json::text,'{}'),
			           NULLIF(classification_category_config.price_list_rule_json::text,'{}'),
			           NULLIF(classification_template_config.price_list_rule_json::text,'{}'),
			           NULLIF(cpro.price_list_rule_json::text,'{}'),
			           NULLIF(cpti.price_list_rule_json::text,'{}'),
			           NULLIF(p.unit_rule_override_json->>'price_list_rule_json',''),
			           NULLIF(pc.price_list_rule_json::text,'{}'),
		           NULLIF(parent_pc.price_list_rule_json::text,'{}'),
		           '{}'
		       ) AS effective_price_list_rule_json,
		       COALESCE(
			           NULLIF(pca.production_config_attrs_schema_json::text,'[]'),
			           NULLIF(current_bv.special_attrs_schema_json::text,'[]'),
			           NULLIF(alias_config.special_attrs_schema_json::text,'[]'),
			           NULLIF(p_config.special_attrs_schema_json::text,'[]'),
			           NULLIF(classification_category_config.special_attrs_schema_json::text,'[]'),
			           NULLIF(classification_template_config.special_attrs_schema_json::text,'[]'),
			           NULLIF(pc_config.special_attrs_schema_json::text,'[]'),
		           NULLIF(parent_pc_config.special_attrs_schema_json::text,'[]'),
		           '[]'
		       ) AS effective_special_attrs_schema_json,
		       CASE
		         WHEN COALESCE(p.parent_product_id,0) > 0 THEN COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg')
		         ELSE COALESCE(
			           NULLIF(alias_config.inventory_unit,''),
			           NULLIF(alias_legacy_unit.inventory_unit,''),
			           NULLIF(cpro.unit_rule_json->>'inventory_unit',''),
			           NULLIF(cpti.unit_rule_json->>'inventory_unit',''),
			           NULLIF(p.unit_rule_override_json->>'inventory_unit',''),
			           NULLIF(product_unit_template.inventory_unit,''),
			           NULLIF(p_config.inventory_unit,''),
			           NULLIF(classification_category_config.inventory_unit,''),
			           NULLIF(classification_template_config.inventory_unit,''),
			           NULLIF(pc.inventory_unit,''),
		           NULLIF(parent_pc.inventory_unit,''),
		           'kg'
		         )
		       END,
		       CASE
		         WHEN COALESCE(p.auto_derived_sku,false) THEN COALESCE(NULLIF(p.derived_sales_unit,''), NULLIF(p.sku_name,''), NULLIF(parent_units.parent_inventory_unit,''), 'kg')
		         ELSE COALESCE(
			           NULLIF(alias_config.quote_unit,''),
			           NULLIF(alias_legacy_unit.quote_unit,''),
			           NULLIF(cpro.unit_rule_json->>'quote_unit',''),
			           NULLIF(cpti.unit_rule_json->>'quote_unit',''),
			           NULLIF(p.unit_rule_override_json->>'quote_unit',''),
			           NULLIF(product_unit_template_default_spec.sales_unit,''),
			           NULLIF(product_unit_template.quote_unit,''),
			           NULLIF(product_unit_template.order_unit,''),
			           NULLIF(p_config.quote_unit,''),
			           NULLIF(classification_category_config.quote_unit,''),
			           NULLIF(classification_template_config.quote_unit,''),
			           NULLIF(pc.quote_unit,''),
		           NULLIF(parent_pc.quote_unit,''),
		           'kg'
		         )
		       END,
		       CASE
		         WHEN COALESCE(p.auto_derived_sku,false) THEN COALESCE(NULLIF(p.derived_sales_unit,''), NULLIF(p.sku_name,''), NULLIF(parent_units.parent_inventory_unit,''), 'kg')
		         ELSE COALESCE(
			           NULLIF(alias_config.order_unit,''),
			           NULLIF(alias_legacy_unit.order_unit,''),
			           NULLIF(cpro.unit_rule_json->>'order_unit',''),
			           NULLIF(cpti.unit_rule_json->>'order_unit',''),
			           NULLIF(p.unit_rule_override_json->>'order_unit',''),
			           NULLIF(product_unit_template_default_spec.sales_unit,''),
			           NULLIF(product_unit_template.order_unit,''),
			           NULLIF(product_unit_template.quote_unit,''),
			           NULLIF(p_config.order_unit,''),
			           NULLIF(classification_category_config.order_unit,''),
			           NULLIF(classification_template_config.order_unit,''),
			           NULLIF(pc.order_unit,''),
		           NULLIF(parent_pc.order_unit,''),
		           'kg'
		         )
		       END,
		       CASE
		         WHEN COALESCE(p.auto_derived_sku,false) AND NULLIF(p.derived_sales_unit,'') IS NOT NULL
		           THEN jsonb_build_object(p.derived_sales_unit, jsonb_build_object(COALESCE(NULLIF(parent_units.parent_inventory_unit,''), 'kg'), derived_sku_units.derived_sku_unit_factor))::text
		         ELSE COALESCE(
		           NULLIF(alias_config.unit_conversion_json::text,'{}'),
		           NULLIF(alias_legacy_unit.unit_conversion_json::text,'{}'),
		           NULLIF(cpro.unit_rule_json->>'unit_conversion_json',''),
		           NULLIF(cpro.unit_rule_json->>'conversion_json',''),
		           NULLIF(cpti.unit_rule_json->>'unit_conversion_json',''),
			           NULLIF(cpti.unit_rule_json->>'conversion_json',''),
			           NULLIF(p.unit_rule_override_json->>'unit_conversion_json',''),
			           NULLIF(p.unit_rule_override_json->>'conversion_json',''),
			           NULLIF(product_unit_template_default_spec.unit_conversion_json,'{}'),
			           NULLIF(product_unit_template.unit_conversion_json::text,'{}'),
		           NULLIF(p_config.unit_conversion_json::text,'{}'),
		           NULLIF(classification_category_config.unit_conversion_json::text,'{}'),
		           NULLIF(classification_template_config.unit_conversion_json::text,'{}'),
			           NULLIF(pc.unit_conversion_json::text,'{}'),
		           NULLIF(parent_pc.unit_conversion_json::text,'{}'),
		           '{}'
		         )
		       END,
		       COALESCE(
			           alias_config.integer_unit,
			           alias_legacy_unit.integer_unit,
			           CASE WHEN lower(cpro.unit_rule_json->>'integer_unit') IN ('true','1','yes') THEN true WHEN lower(cpro.unit_rule_json->>'integer_unit') IN ('false','0','no') THEN false ELSE NULL END,
			           CASE WHEN lower(cpti.unit_rule_json->>'integer_unit') IN ('true','1','yes') THEN true WHEN lower(cpti.unit_rule_json->>'integer_unit') IN ('false','0','no') THEN false ELSE NULL END,
			           CASE WHEN lower(p.unit_rule_override_json->>'integer_unit') IN ('true','1','yes') THEN true WHEN lower(p.unit_rule_override_json->>'integer_unit') IN ('false','0','no') THEN false ELSE NULL END,
			           product_unit_template.integer_unit,
			           p_config.integer_unit,
			           classification_category_config.integer_unit,
			           classification_template_config.integer_unit,
			           pc.integer_unit,
		           parent_pc.integer_unit,
		           false
		       ),
		       COALESCE(pps.product_price_snapshots_json, '[]'),
		       p.margin_rate_override::float8,
		       1::float8,
		       CASE
		           WHEN COALESCE(NULLIF(p.product_kind,''), 'roasted') = 'green_bean'
		           THEN COALESCE(SUM(COALESCE(NULLIF(bi.unit_cost_snapshot,0), m.purchase_price, 0) * COALESCE(bi.ratio_pct,0) / 100.0 / (1 - LEAST(GREATEST(COALESCE(bi.material_loss_rate,0),0),0.9999))),0)
		           WHEN COALESCE(NULLIF(p.product_kind,''), 'roasted') = 'drip_bag' AND COALESCE(fcc.finished_green_cost_per_kg,0) > 0
		           THEN COALESCE(fcc.finished_green_cost_per_kg,0)
		           ELSE COALESCE(SUM(COALESCE(mv.weighted_unit_cost, m.purchase_price, 0) * COALESCE(bi.ratio_pct,0) / 100.0 / (1 - LEAST(GREATEST(COALESCE(bi.material_loss_rate,0),0),0.9999))),0)
		       END,
		       COALESCE(MAX(buc.bom_cost_per_unit),0)::float8,
		       COALESCE(MAX(ouc.operation_cost_per_unit),0)::float8,
		       COALESCE(MAX(ouc.operation_cost_per_kg),0)::float8,
		       COALESCE(string_agg(DISTINCT NULLIF(bp.flavor, ''), ' / ') FILTER (WHERE NULLIF(bp.flavor, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.origin, ''), ' / ') FILTER (WHERE NULLIF(bp.origin, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.processing_station, ''), ' / ') FILTER (WHERE NULLIF(bp.processing_station, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.variety, ''), ' / ') FILTER (WHERE NULLIF(bp.variety, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.process_method, ''), ' / ') FILTER (WHERE NULLIF(bp.process_method, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.grade, ''), ' / ') FILTER (WHERE NULLIF(bp.grade, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.altitude, ''), ' / ') FILTER (WHERE NULLIF(bp.altitude, '') IS NOT NULL), ''),
		       COALESCE(string_agg(DISTINCT NULLIF(bp.bean_list_note, ''), ' / ') FILTER (WHERE NULLIF(bp.bean_list_note, '') IS NOT NULL), ''),
		       COALESCE(NULLIF(current_bom.status,''), NULLIF(b.status,''), CASE WHEN b.product_id IS NULL AND current_bom.id IS NULL THEN 'missing' ELSE 'active' END),
		       COALESCE(current_bv.id, active_bv.id, 0),
		       COALESCE(current_bv.version_no, active_bv.version_no, ''),
		       COALESCE(NULLIF(p.bom_usage_mode,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'owned' END),
		       COALESCE(qc.factory_flavor_description, ''),
		       COALESCE(qc.moisture, ''),
		       COALESCE(qc.density, ''),
		       COALESCE(qc.inspection_created_at, ''),
		       COALESCE(qc.inspection_reference_no, '')
		FROM product_scope p
		LEFT JOIN %[1]s.product_bom b ON b.product_id = bom_product_id
		LEFT JOIN %[1]s.bom_versions active_bv ON active_bv.product_id=p.bom_product_id AND active_bv.status='active'
		LEFT JOIN %[1]s.production_boms current_bom ON current_bom.id=p.production_bom_id
		LEFT JOIN %[1]s.production_bom_versions current_bv ON current_bv.id=p.production_bom_version_id
		LEFT JOIN effective_bom_items bi ON bi.product_id = p.id
		LEFT JOIN %[1]s.materials m ON m.id = bi.material_id
		LEFT JOIN material_valuation mv ON mv.material_id = m.id
		LEFT JOIN %[1]s.material_bean_profiles bp ON bp.material_id = m.id
		LEFT JOIN %[1]s.products base_p ON base_p.id = p.base_product_id
			LEFT JOIN %[1]s.product_categories pc ON pc.id = p.product_category_id AND pc.active=true
			LEFT JOIN %[1]s.product_categories parent_pc ON parent_pc.id = pc.parent_id AND parent_pc.active=true
		LEFT JOIN %[1]s.product_config_templates p_config ON p_config.id = p.product_config_template_id AND p_config.active=true
		LEFT JOIN %[1]s.product_config_templates alias_config ON alias_config.id = p.customer_product_alias_product_config_template_id AND alias_config.active=true
		LEFT JOIN %[1]s.product_config_templates classification_category_config ON classification_category_config.id = p.current_classification_category_product_config_template_id AND classification_category_config.active=true
		LEFT JOIN %[1]s.product_config_templates classification_template_config ON classification_template_config.id = p.current_classification_template_product_config_template_id AND classification_template_config.active=true
		LEFT JOIN %[1]s.product_config_templates pc_config ON pc_config.id = pc.product_config_template_id AND pc_config.active=true
		LEFT JOIN %[1]s.product_config_templates parent_pc_config ON parent_pc_config.id = parent_pc.product_config_template_id AND parent_pc_config.active=true
		LEFT JOIN %[1]s.product_unit_templates alias_legacy_unit ON alias_legacy_unit.id=p.customer_product_alias_unit_template_id AND alias_legacy_unit.active=true
		LEFT JOIN %[1]s.product_unit_templates product_unit_template ON product_unit_template.id=p.unit_template_id AND product_unit_template.active=true
		LEFT JOIN LATERAL (
			SELECT NULLIF(spec.row->>'spec_key','') AS spec_key,
			       NULLIF(spec.row->>'spec_name','') AS spec_name,
			       NULLIF(spec.row->>'spec_name','') AS sales_unit,
			       COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)::float8 AS net_content_qty,
			       NULLIF(spec.row->>'net_content_unit','') AS net_content_unit,
			       CASE
			         WHEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) > 0
			          AND NULLIF(spec.row->>'spec_name','') IS NOT NULL
			         THEN jsonb_build_object(
			           NULLIF(spec.row->>'spec_name',''),
			           jsonb_build_object(
			             COALESCE(NULLIF(product_unit_template.inventory_unit,''), NULLIF(spec.row->>'net_content_unit',''), 'kg'),
			             CASE
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''), NULLIF(product_unit_template.inventory_unit,''), 'kg')) = lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''), NULLIF(spec.row->>'net_content_unit',''), 'kg'))
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('g','克') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('kg','千克','公斤')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) / 1000.0
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('kg','千克','公斤') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('g','克')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) * 1000.0
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('lb','lbs','磅') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('kg','千克','公斤')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) * 0.45359237
			               WHEN lower(COALESCE(NULLIF(spec.row->>'net_content_unit',''),'')) IN ('kg','千克','公斤') AND lower(COALESCE(NULLIF(product_unit_template.inventory_unit,''),'')) IN ('lb','lbs','磅')
			                 THEN COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0) / 0.45359237
			               ELSE COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)
			             END
			           )
			         )::text
			         ELSE '{}'
			       END AS unit_conversion_json
			FROM jsonb_array_elements(COALESCE(product_unit_template.sales_specs_json, '[]'::jsonb)) WITH ORDINALITY AS spec(row, ord)
			WHERE COALESCE(spec.row->>'active','true') <> 'false'
			  AND NULLIF(spec.row->>'spec_name','') IS NOT NULL
			ORDER BY CASE WHEN COALESCE(spec.row->>'default','false') = 'true' THEN 0 ELSE 1 END, spec.ord
			LIMIT 1
		) product_unit_template_default_spec ON true
		LEFT JOIN %[1]s.products parent_product ON parent_product.id=CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END AND parent_product.active=true
		LEFT JOIN %[1]s.product_unit_templates parent_product_unit_template ON parent_product_unit_template.id=parent_product.unit_template_id AND parent_product_unit_template.active=true
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
		LEFT JOIN production_config_attrs pca ON pca.product_id=p.id
		LEFT JOIN alias_config_attrs alias_attrs ON alias_attrs.alias_id=p.customer_product_alias_id
		LEFT JOIN %[1]s.customers rule_customer ON rule_customer.id = $1 AND rule_customer.active=true
		LEFT JOIN %[1]s.customer_product_rule_template_items cpti
		  ON cpti.active=true
		 AND cpti.template_id=COALESCE(rule_customer.customer_product_rule_template_id,0)
		 AND cpti.product_subtype_category_id=CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.id,0) ELSE 0 END
		LEFT JOIN %[1]s.customer_product_rule_overrides cpro
		  ON cpro.active=true
		 AND cpro.customer_id=$1
		 AND cpro.product_subtype_category_id=CASE WHEN COALESCE(pc.level,0)=2 THEN COALESCE(pc.id,0) ELSE 0 END
		LEFT JOIN operation_unit_cost ouc
		  ON ouc.template_id=COALESCE(
			           NULLIF(alias_config.operation_template_id,0),
			           NULLIF(p_config.operation_template_id,0),
			           NULLIF(classification_category_config.operation_template_id,0),
			           NULLIF(classification_template_config.operation_template_id,0),
			           NULLIF(cpro.operation_template_id,0),
			           NULLIF(cpti.operation_template_id,0),
			           NULLIF(p.operation_template_id_override,0),
			           NULLIF(pc.operation_template_id,0),
		           NULLIF(parent_pc.operation_template_id,0),
		           0
		       )
		LEFT JOIN finished_component_cost fcc ON fcc.product_id = p.id
		LEFT JOIN bom_unit_cost buc ON buc.product_id = p.id
		LEFT JOIN %[1]s.process_routes product_process_route ON product_process_route.id=p.process_route_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(NULLIF(qi.metrics_json->>'factory_flavor_description',''), NULLIF(qi.metrics_json->>'factory_flavor',''), NULLIF(qi.metrics_json->>'工厂风味描述',''), '') AS factory_flavor_description,
			       COALESCE(NULLIF(qi.metrics_json->>'moisture',''), NULLIF(qi.metrics_json->>'水分',''), '') AS moisture,
			       COALESCE(NULLIF(qi.metrics_json->>'density',''), NULLIF(qi.metrics_json->>'密度',''), '') AS density,
			       to_char(qi.created_at,'YYYY-MM-DD HH24:MI') AS inspection_created_at,
			       qi.reference_no AS inspection_reference_no
			FROM %[1]s.quality_inspections qi
			LEFT JOIN %[1]s.work_orders qi_work_order
			  ON (qi.reference_type='work_order' OR qi.scope='work_order')
			 AND qi_work_order.work_order_no=qi.reference_no
			LEFT JOIN %[1]s.stock_batches qi_work_batch
			  ON (qi.reference_type='work_order' OR qi.scope='work_order')
			 AND qi_work_batch.item_type='finished_product'
			 AND qi_work_batch.source_doc_id=qi_work_order.running_item_id
			LEFT JOIN %[1]s.stock_batches qi_finished_batch
			  ON (qi.reference_type='finished_batch' OR qi.scope='finished_batch')
			 AND qi_finished_batch.item_type='finished_product'
			 AND qi_finished_batch.batch_code=qi.reference_no
			WHERE qi.result='pass'
			  AND (
			    ((qi.reference_type='work_order' OR qi.scope='work_order') AND (qi_work_order.product_id=p.bom_product_id OR qi_work_batch.item_id=p.bom_product_id))
			    OR ((qi.reference_type='finished_batch' OR qi.scope='finished_batch') AND qi_finished_batch.item_id=p.bom_product_id)
			  )
			ORDER BY qi.created_at DESC, qi.id DESC
			LIMIT 1
		) qc ON true
		LEFT JOIN LATERAL (
			WITH price_records AS (
				SELECT ppr.id,
				       ppr.final_unit_price,
				       ppr.price_unit,
				       ppr.currency,
				       COALESCE(ppr.price_group_id,0) AS price_group_id,
				       COALESCE(ppr.price_group_name,'') AS price_group_name,
				       ppr.inventory_unit,
				       COALESCE(ppr.inventory_conversion_json, '{}'::jsonb) AS inventory_conversion_json,
				       COALESCE(ppr.product_id,0) AS product_id,
				       COALESCE(ppr.customer_product_alias_id,0) AS customer_product_alias_id
				FROM %[1]s.product_price_records ppr
				WHERE ppr.active=true
				  AND ppr.status='published'
				  AND (
				    (COALESCE(p.customer_product_alias_id,0)>0 AND COALESCE(ppr.customer_product_alias_id,0)=COALESCE(p.customer_product_alias_id,0))
				    OR (COALESCE(ppr.customer_product_alias_id,0)=0 AND COALESCE(ppr.product_id,0)=p.id)
				  )
			),
			tier_rows AS (
				SELECT pr.id AS source_price_record_id,
				       COALESCE(NULLIF(ptst.label,''), NULLIF(pts.name,''), pr.price_group_name, '') AS tier_label,
				       COALESCE(ptst.min_qty,0)::float8 AS min_qty,
				       ptst.max_qty::float8 AS max_qty,
				       ptst.final_unit_price,
				       ptst.price_unit,
				       ptst.currency,
				       COALESCE(NULLIF(pts.price_group_id,0), pr.price_group_id,0) AS price_group_id,
				       pr.price_group_name,
				       pr.inventory_unit,
				       pr.inventory_conversion_json,
				       pr.product_id,
				       pr.customer_product_alias_id,
				       COALESCE(NULLIF(ptst.position,0),100) AS position
				FROM %[1]s.product_tier_price_schemes pts
				JOIN %[1]s.product_tier_price_scheme_tiers ptst
				  ON ptst.scheme_id=pts.id
				 AND ptst.active=true
				JOIN price_records pr ON pr.id=ptst.source_price_record_id
				WHERE pts.active=true
				  AND (
				    (COALESCE(p.customer_product_alias_id,0)>0 AND COALESCE(pts.customer_product_alias_id,0)=COALESCE(p.customer_product_alias_id,0))
				    OR (COALESCE(pts.customer_product_alias_id,0)=0 AND COALESCE(pts.product_id,0)=p.id)
				  )
			),
			source_rows AS (
				SELECT source_price_record_id,
				       tier_label,
				       min_qty,
				       max_qty,
				       final_unit_price,
				       price_unit,
				       currency,
				       price_group_id,
				       price_group_name,
				       inventory_unit,
				       inventory_conversion_json,
				       product_id,
				       customer_product_alias_id,
				       position
				FROM tier_rows
				UNION ALL
				SELECT pr.id AS source_price_record_id,
				       pr.price_group_name AS tier_label,
				       0::float8 AS min_qty,
				       NULL::float8 AS max_qty,
				       pr.final_unit_price,
				       pr.price_unit,
				       pr.currency,
				       pr.price_group_id,
				       pr.price_group_name,
				       pr.inventory_unit,
				       pr.inventory_conversion_json,
				       pr.product_id,
				       pr.customer_product_alias_id,
				       1000 + (row_number() OVER (ORDER BY
					       CASE
						       WHEN COALESCE(p.customer_product_alias_id,0)>0 AND pr.customer_product_alias_id=COALESCE(p.customer_product_alias_id,0) THEN 0
						       WHEN pr.product_id=p.id THEN 1
						       ELSE 2
					       END,
					       pr.price_group_id,
					       pr.id
				       ))::int AS position
				FROM price_records pr
				WHERE NOT EXISTS (SELECT 1 FROM tier_rows)
			)
			SELECT COALESCE(jsonb_agg(jsonb_build_object(
				'source_price_record_id', source_price_record_id,
				'tier_label', tier_label,
				'min_qty', min_qty,
				'max_qty', max_qty,
				'final_unit_price', final_unit_price,
				'price_unit', price_unit,
				'currency', currency,
				'price_group_id', price_group_id,
				'price_group_name', price_group_name,
				'inventory_unit', inventory_unit,
				'inventory_conversion_json', inventory_conversion_json,
				'product_id', product_id,
				'customer_product_alias_id', customer_product_alias_id,
				'position', position
			) ORDER BY position, min_qty, source_price_record_id), '[]'::jsonb)::text AS product_price_snapshots_json
			FROM source_rows
		) pps ON true
		WHERE p.active = true
			GROUP BY p.id, p.parent_product_id, p.sku_name, p.sku_code, p.barcode, p.derived_spec_key, p.spec_label, p.net_content_qty, p.net_content_unit, p.is_default_sku, parent_product.id, parent_product.default_sku_id, p.name, p.customer_product_alias_id, p.customer_product_display_name, p.customer_item_code, p.brand_name, p.display_category_id, p.display_category_name, p.customer_product_alias_product_config_template_id, p.customer_product_alias_gradient_template_id, p.customer_product_alias_unit_template_id, p.current_classification_template_id, p.current_classification_template_name, p.current_classification_category_id, p.current_classification_category_name, p.current_classification_category_product_config_template_id, p.current_classification_template_product_config_template_id, p.bom_usage_mode, p.production_bom_id, p.production_bom_version_id, p.production_config_yield_rate, p.process_route_id, product_process_route.name, base_p.name, p.roast_level, p.special_attrs_json, p.customer_id, p.base_product_id, p.visibility, p.custom_type, p.product_kind, p.drip_bag_grams, p.drip_box_bag_count, p.product_category_id, p.product_category_position, p.product_config_template_id, p.gradient_template_id_override, p.operation_template_id_override, p.unit_rule_override_json, p.auto_derived_sku, p.derived_sales_unit, parent_units.parent_inventory_unit, derived_sku_units.derived_sku_unit_factor, alias_config.gradient_template_id, alias_config.operation_template_id, alias_config.price_list_rule_json, alias_config.inventory_unit, alias_config.quote_unit, alias_config.order_unit, alias_config.unit_conversion_json, alias_config.integer_unit, alias_config.special_attrs_schema_json, p_config.gradient_template_id, p_config.operation_template_id, p_config.price_list_rule_json, p_config.inventory_unit, p_config.quote_unit, p_config.order_unit, p_config.unit_conversion_json, p_config.integer_unit, p_config.special_attrs_schema_json, classification_category_config.gradient_template_id, classification_category_config.operation_template_id, classification_category_config.price_list_rule_json, classification_category_config.inventory_unit, classification_category_config.quote_unit, classification_category_config.order_unit, classification_category_config.unit_conversion_json, classification_category_config.integer_unit, classification_category_config.special_attrs_schema_json, classification_template_config.gradient_template_id, classification_template_config.operation_template_id, classification_template_config.price_list_rule_json, classification_template_config.inventory_unit, classification_template_config.quote_unit, classification_template_config.order_unit, classification_template_config.unit_conversion_json, classification_template_config.integer_unit, classification_template_config.special_attrs_schema_json, pc.id, pc.level, pc.name, pc.position, pc.gradient_template_id, pc.operation_template_id, pc.price_list_rule_json, pc.inventory_unit, pc.quote_unit, pc.order_unit, pc.unit_conversion_json, pc.integer_unit, pc_config.special_attrs_schema_json, parent_pc.id, parent_pc.name, parent_pc.position, parent_pc.gradient_template_id, parent_pc.operation_template_id, parent_pc.price_list_rule_json, parent_pc.inventory_unit, parent_pc.quote_unit, parent_pc.order_unit, parent_pc.unit_conversion_json, parent_pc.integer_unit, parent_pc_config.special_attrs_schema_json, alias_legacy_unit.inventory_unit, alias_legacy_unit.quote_unit, alias_legacy_unit.order_unit, alias_legacy_unit.unit_conversion_json, alias_legacy_unit.integer_unit, product_unit_template.inventory_unit, product_unit_template.quote_unit, product_unit_template.order_unit, product_unit_template.unit_conversion_json, product_unit_template.integer_unit, product_unit_template_default_spec.spec_key, product_unit_template_default_spec.spec_name, product_unit_template_default_spec.sales_unit, product_unit_template_default_spec.net_content_qty, product_unit_template_default_spec.net_content_unit, product_unit_template_default_spec.unit_conversion_json, pca.production_config_attrs_json, pca.production_config_attrs_schema_json, alias_attrs.alias_attrs_json, cpti.gradient_template_id, cpti.operation_template_id, cpti.price_list_rule_json, cpti.unit_rule_json, cpro.gradient_template_id, cpro.operation_template_id, cpro.customer_id, cpro.product_subtype_category_id, cpro.price_list_rule_json, cpro.unit_rule_json, pps.product_price_snapshots_json, p.margin_rate_override, p.bom_product_id, b.yield_rate, b.status, b.product_id, active_bv.id, active_bv.version_no, current_bom.id, current_bom.status, current_bv.id, current_bv.version_no, current_bv.yield_rate, current_bv.special_attrs_json, current_bv.special_attrs_schema_json, fcc.finished_green_cost_per_kg, qc.factory_flavor_description, qc.moisture, qc.density, qc.inspection_created_at, qc.inspection_reference_no
		ORDER BY p.name
	`, r.schema)
	rows, err := r.pool.Query(ctx, q, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.ProductInput, 0)
	templateIDs := map[int64]bool{}
	templateIDByProduct := map[int64]int64{}
	for rows.Next() {
		var input domain.ProductInput
		var roastLevel string
		var fallbackYield float64
		var gradientTemplateID int64
		var productPriceSnapshotsJSON string
		if err := rows.Scan(
			&input.ProductID,
			&input.SKUID,
			&input.ParentProductID,
			&input.EffectiveParentProductID,
			&input.SKUName,
			&input.SKUCode,
			&input.Barcode,
			&input.SpecKey,
			&input.SpecLabel,
			&input.NetContentQty,
			&input.NetContentUnit,
			&input.IsDefaultSKU,
			&input.DefaultSKUID,
			&input.Name,
			&input.ProductCode,
			&input.ProductName,
			&input.CustomerProductAliasID,
			&input.CustomerProductDisplayName,
			&input.CustomerItemCode,
			&input.BrandName,
			&input.DisplayCategoryID,
			&input.DisplayCategoryName,
			&input.ClassificationTemplateID,
			&input.ClassificationTemplateName,
			&input.ClassificationCategoryID,
			&input.ClassificationCategoryName,
			&input.BeanListTemplateName,
			&roastLevel,
			&input.SpecialAttrsJSON,
			&input.CustomerID,
			&input.BaseProductID,
			&input.Visibility,
			&input.CustomType,
			&input.ProductKind,
			&input.DripBagGrams,
			&input.DripBoxBagCount,
			&input.ProductCategoryID,
			&input.ProductCategoryPosition,
			&input.ProductTypeCategoryID,
			&input.ProductSubtypeCategoryID,
			&input.CategoryPrimaryName,
			&input.CategoryPrimaryPosition,
			&input.CategorySecondaryName,
			&input.CategorySecondaryPosition,
			&gradientTemplateID,
			&input.OperationTemplateID,
			&input.ProcessRouteID,
			&input.ProcessRouteName,
			&input.PriceListRuleJSON,
			&input.SpecialAttrsSchemaJSON,
			&input.InventoryUnit,
			&input.QuoteUnit,
			&input.OrderUnit,
			&input.UnitConversionJSON,
			&input.IntegerUnit,
			&productPriceSnapshotsJSON,
			&input.MarginRateOverride,
			&fallbackYield,
			&input.GreenBeanCostPerKg,
			&input.BomCostPerUnit,
			&input.OperationCostPerUnit,
			&input.OperationCostPerKg,
			&input.Flavor,
			&input.Origin,
			&input.ProcessingStation,
			&input.Variety,
			&input.ProcessMethod,
			&input.Grade,
			&input.Altitude,
			&input.BeanListNote,
			&input.BomStatus,
			&input.BomVersionID,
			&input.BomVersionNo,
			&input.BomUsageMode,
			&input.BeanListQuality.FactoryFlavorDescription,
			&input.BeanListQuality.Moisture,
			&input.BeanListQuality.Density,
			&input.BeanListQuality.InspectionCreatedAt,
			&input.BeanListQuality.InspectionReferenceNo,
		); err != nil {
			return nil, err
		}
		input.ProductTypeName = input.CategoryPrimaryName
		input.ProductSubtypeName = input.CategorySecondaryName
		if gradientTemplateID > 0 {
			templateIDs[gradientTemplateID] = true
			templateIDByProduct[input.ProductID] = gradientTemplateID
		}
		_ = roastLevel
		input.ProductPriceSnapshots = productPriceSnapshotsFromJSON(productPriceSnapshotsJSON)
		input.YieldRate = 1
		input.ExpectedLossRate = 0
		if strings.TrimSpace(input.BomStatus) == "inactive" {
			input.Warnings = append(input.Warnings, "BOM已失效：请重新启用 BOM 后再发布价格策略")
		}
		input = domain.ApplyExcelCommercialPricingProfile(params, input)
		out = append(out, input)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	productIDs := make([]int64, 0, len(out))
	seenProductIDs := map[int64]bool{}
	for _, input := range out {
		parentID := input.EffectiveParentProductID
		if parentID <= 0 {
			parentID = input.ParentProductID
		}
		if parentID <= 0 {
			parentID = input.ProductID
		}
		if parentID > 0 && !seenProductIDs[parentID] {
			seenProductIDs[parentID] = true
			productIDs = append(productIDs, parentID)
		}
	}
	authorities, err := r.loadProductBOMSpecAuthorities(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	for i := range out {
		parentID := out[i].EffectiveParentProductID
		if parentID <= 0 {
			parentID = out[i].ParentProductID
		}
		if parentID <= 0 {
			parentID = out[i].ProductID
		}
		if authority, ok := authorities[parentID]; ok {
			out[i].MigrationState = authority.MigrationState
			out[i].BomSpecAuthoritative = authority.BomSpecAuthoritative
			if authority.BomSpecAuthoritative {
				out[i].SpecIdentityMode = "bom_spec"
			} else {
				out[i].SpecIdentityMode = "legacy_sku"
			}
		}
	}
	resolvedBomCosts, err := r.loadResolvedProductionBomCosts(ctx)
	if err != nil {
		return nil, err
	}
	templates, err := r.loadGradientTemplatesByID(ctx, templateIDs)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if templateID := templateIDByProduct[out[i].ProductID]; templateID > 0 {
			if template := templates[templateID]; template != nil {
				out[i].GradientTemplate = template
			}
		}
	}
	cutoverSpecs, err := r.loadCutoverProductBOMSpecs(ctx)
	if err != nil {
		return nil, err
	}
	out = applyCutoverProductBOMSpecs(out, cutoverSpecs)
	// Once a product is BOM-authoritative, the parent plus its BOM variants are
	// the only active catalog candidates.  Retired child rows remain readable
	// through immutable snapshots/migration mappings, but must not re-enter a
	// price-list or trial selection.
	out = filterBOMAuthoritativeProductInputs(out)
	return applyResolvedProductionBomCosts(out, resolvedBomCosts), nil
}

func filterBOMAuthoritativeProductInputs(inputs []domain.ProductInput) []domain.ProductInput {
	if len(inputs) == 0 {
		return inputs
	}
	out := make([]domain.ProductInput, 0, len(inputs))
	for _, input := range inputs {
		if input.BomSpecAuthoritative && input.SKUID > 0 && input.BomSpecID <= 0 {
			// This is the legacy parent/SKU projection; the BOM-spec projection
			// added by applyCutoverProductBOMSpecs is authoritative instead.
			continue
		}
		if input.BomSpecAuthoritative && input.ParentProductID > 0 && input.BomSpecID <= 0 {
			continue
		}
		out = append(out, input)
	}
	return out
}

type productBOMSpecAuthority struct {
	MigrationState       string
	BomSpecAuthoritative bool
}

func (r Repository) loadProductBOMSpecAuthorities(ctx context.Context, productIDs []int64) (map[int64]productBOMSpecAuthority, error) {
	result := map[int64]productBOMSpecAuthority{}
	if r.pool == nil || len(productIDs) == 0 {
		return result, nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.product_bom_spec_migrations", r.schema)).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT product_id,
		       COALESCE(state,''),
		       COALESCE((to_jsonb(product_bom_spec_migrations)->>'legacy_catalog_product')::boolean,true)=false
		         OR COALESCE(state,'')='cutover'
		FROM %s.product_bom_spec_migrations
		WHERE product_id=ANY($1::bigint[])
	`, r.schema), productIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var productID int64
		var authority productBOMSpecAuthority
		if err := rows.Scan(&productID, &authority.MigrationState, &authority.BomSpecAuthoritative); err != nil {
			return nil, err
		}
		result[productID] = authority
	}
	return result, rows.Err()
}

type cutoverProductBOMSpec struct {
	ParentProductID      int64
	BomID                int64
	BomVersionID         int64
	BomVersionNo         string
	BomSpecID            int64
	BomVariantID         int64
	SpecCode             string
	Barcode              string
	SpecKey              string
	SpecName             string
	InventoryUnit        string
	MigrationState       string
	BomSpecAuthoritative bool
	ProcessRouteID       int64
	IsDefault            bool
	SortOrder            int
}

func (r Repository) loadCutoverProductBOMSpecs(ctx context.Context) ([]cutoverProductBOMSpec, error) {
	for _, relation := range []string{
		"product_bom_spec_migrations",
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
			return []cutoverProductBOMSpec{}, nil
		}
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT migration.product_id,
		       binding.bom_id,
		       version.id,
		       COALESCE(version.version_no,''),
		       spec.id,
		       variant.id,
		       COALESCE(spec.code,''),
		       COALESCE(spec.barcode,''),
		       COALESCE(spec.spec_key,''),
		       COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,''),
		       COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit,''),
		       migration.state,
		       COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false OR migration.state='cutover',
		       COALESCE(variant.process_route_id,0),
		       variant.is_default,
		       variant.sort_order
		FROM %[1]s.product_bom_spec_migrations migration
		JOIN %[1]s.products parent_product
		  ON parent_product.id=migration.product_id
		 AND parent_product.active=true
		JOIN %[1]s.production_bom_output_bindings binding
		  ON binding.output_type='product'
		 AND binding.output_id=migration.product_id
		 AND binding.is_default=true
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id
		 AND version.bom_id=binding.bom_id
		 AND version.status='published'
		JOIN %[1]s.production_bom_specs spec
		  ON spec.bom_id=binding.bom_id
		JOIN %[1]s.production_bom_version_variants variant
		  ON variant.version_id=version.id
		 AND variant.bom_spec_id=spec.id
		WHERE migration.state='cutover'
		   OR COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)=false
		ORDER BY migration.product_id,variant.sort_order,spec.spec_key,spec.id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cutoverProductBOMSpec, 0)
	for rows.Next() {
		var row cutoverProductBOMSpec
		if err := rows.Scan(
			&row.ParentProductID,
			&row.BomID,
			&row.BomVersionID,
			&row.BomVersionNo,
			&row.BomSpecID,
			&row.BomVariantID,
			&row.SpecCode,
			&row.Barcode,
			&row.SpecKey,
			&row.SpecName,
			&row.InventoryUnit,
			&row.MigrationState,
			&row.BomSpecAuthoritative,
			&row.ProcessRouteID,
			&row.IsDefault,
			&row.SortOrder,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func applyCutoverProductBOMSpecs(inputs []domain.ProductInput, specs []cutoverProductBOMSpec) []domain.ProductInput {
	if len(specs) == 0 {
		return inputs
	}
	specsByParent := map[int64][]cutoverProductBOMSpec{}
	defaultByParent := map[int64]int64{}
	for _, spec := range specs {
		if spec.ParentProductID <= 0 || spec.BomSpecID <= 0 || spec.BomVariantID <= 0 || strings.TrimSpace(spec.InventoryUnit) == "" {
			continue
		}
		specsByParent[spec.ParentProductID] = append(specsByParent[spec.ParentProductID], spec)
		if spec.IsDefault {
			defaultByParent[spec.ParentProductID] = spec.BomSpecID
		}
	}
	if len(specsByParent) == 0 {
		return inputs
	}
	basesByParent := map[int64][]domain.ProductInput{}
	out := make([]domain.ProductInput, 0, len(inputs)+len(specs))
	for _, input := range inputs {
		parentID := input.EffectiveParentProductID
		if parentID <= 0 {
			parentID = input.ParentProductID
		}
		if parentID <= 0 {
			parentID = input.ProductID
		}
		if _, cutover := specsByParent[parentID]; !cutover {
			out = append(out, input)
			continue
		}
		if input.ProductID == parentID && input.ParentProductID <= 0 {
			basesByParent[parentID] = append([]domain.ProductInput{input}, basesByParent[parentID]...)
		} else {
			basesByParent[parentID] = append(basesByParent[parentID], input)
		}
	}
	parentIDs := make([]int64, 0, len(specsByParent))
	for parentID := range specsByParent {
		parentIDs = append(parentIDs, parentID)
	}
	sort.Slice(parentIDs, func(i, j int) bool { return parentIDs[i] < parentIDs[j] })
	for _, parentID := range parentIDs {
		bases := basesByParent[parentID]
		if len(bases) == 0 {
			continue
		}
		// Preserve each customer/alias scope that survived the base query, but
		// never preserve one row per retired child SKU.
		baseByScope := map[string]domain.ProductInput{}
		for _, base := range bases {
			key := fmt.Sprintf("%d:%d", base.CustomerID, base.CustomerProductAliasID)
			if _, exists := baseByScope[key]; !exists || (base.ProductID == parentID && base.ParentProductID <= 0) {
				baseByScope[key] = base
			}
		}
		scopeKeys := make([]string, 0, len(baseByScope))
		for key := range baseByScope {
			scopeKeys = append(scopeKeys, key)
		}
		sort.Strings(scopeKeys)
		for _, scopeKey := range scopeKeys {
			base := baseByScope[scopeKey]
			for _, spec := range specsByParent[parentID] {
				row := base
				migrationState := strings.TrimSpace(spec.MigrationState)
				if migrationState == "" {
					migrationState = "cutover"
				}
				row.ProductID = parentID
				row.SKUID = 0
				row.ParentProductID = parentID
				row.EffectiveParentProductID = parentID
				row.BomSpecID = spec.BomSpecID
				row.BomVariantID = spec.BomVariantID
				row.BomID = spec.BomID
				row.DefaultBOMSpecID = defaultByParent[parentID]
				row.MigrationState = migrationState
				row.SpecIdentityMode = "bom_spec"
				row.BomSpecAuthoritative = spec.BomSpecAuthoritative || migrationState == "cutover"
				row.SpecCode = strings.TrimSpace(spec.SpecCode)
				row.SpecBarcode = strings.TrimSpace(spec.Barcode)
				row.SKUCode = row.SpecCode
				row.Barcode = row.SpecBarcode
				row.SpecSortOrder = spec.SortOrder
				row.SpecPublished = true
				row.SKUName = strings.TrimSpace(spec.SpecName)
				row.SpecKey = strings.TrimSpace(spec.SpecKey)
				row.SpecLabel = strings.TrimSpace(spec.SpecName)
				row.IsDefaultSKU = spec.IsDefault
				row.DefaultSKUID = 0
				row.InventoryUnit = strings.TrimSpace(spec.InventoryUnit)
				row.QuoteUnit = row.InventoryUnit
				row.OrderUnit = row.InventoryUnit
				conversion, _ := json.Marshal(map[string]map[string]float64{row.InventoryUnit: {row.InventoryUnit: 1}})
				row.UnitConversionJSON = string(conversion)
				row.BomVersionID = spec.BomVersionID
				row.BomVersionNo = spec.BomVersionNo
				row.BomUsageMode = "production_bom_output"
				row.ProcessRouteID = spec.ProcessRouteID
				out = append(out, row)
			}
		}
	}
	return out
}

func applyResolvedProductionBomCosts(inputs []domain.ProductInput, costs map[int64]productionBomResolvedCost) []domain.ProductInput {
	for i := range inputs {
		input := &inputs[i]
		resolvedCost, ok := productionBomCostForProduct(costs, input.ProductID, input.ParentProductID, input.BomSpecID)
		if input.BomVersionID > 0 && ok && resolvedCost.VersionID != input.BomVersionID {
			ok = false
		}
		if input.BomSpecID > 0 {
			// A cutover row is priced only from its own immutable BOM specification.
			// Never carry the parent query's aggregate/legacy unit cost into every
			// specification row.
			input.BomCostPerUnit = 0
			input.OperationCostPerUnit = 0
			input.OperationCostPerKg = 0
			if !ok || !resolvedCost.Resolved {
				input.Warnings = append(input.Warnings, "BOM规格成本无法完整解析：请检查该规格的物料价格、明确组件规格、工艺路线和循环引用")
				continue
			}
			input.BomCostPerUnit = resolvedCost.InputCostPerOutputUnit
			input.OperationCostPerUnit = resolvedCost.OperationCostPerOutputUnit
			continue
		}
		if !ok || (!resolvedCost.HasProductComponent && !resolvedCost.HasManufacturedMaterialComponent) || productionBomCostMassKgFactor(resolvedCost.OutputUnit) > 0 {
			continue
		}
		if !resolvedCost.Resolved {
			input.BomCostPerUnit = 0
			input.OperationCostPerUnit = 0
			input.OperationCostPerKg = 0
			input.Warnings = append(input.Warnings, "递归组件成本无法完整解析：请检查组件商品或物料的默认已发布生产 BOM、物料价格、BOM 原料损耗和循环引用")
			continue
		}
		input.BomCostPerUnit = resolvedCost.InputCostPerOutputUnit
		input.OperationCostPerUnit = resolvedCost.OperationCostPerOutputUnit
		input.OperationCostPerKg = 0
	}
	return inputs
}

func productPriceSnapshotsFromJSON(raw string) []domain.ProductPriceSnapshot {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var rows []domain.ProductPriceSnapshot
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil
	}
	out := make([]domain.ProductPriceSnapshot, 0, len(rows))
	for _, row := range rows {
		row.Label = strings.TrimSpace(row.Label)
		row.PriceUnit = strings.TrimSpace(row.PriceUnit)
		row.Currency = strings.TrimSpace(row.Currency)
		row.PriceGroupName = strings.TrimSpace(row.PriceGroupName)
		row.InventoryUnit = strings.TrimSpace(row.InventoryUnit)
		if row.SourcePriceRecordID <= 0 || row.FinalUnitPrice <= 0 || row.PriceUnit == "" {
			continue
		}
		if row.Currency == "" {
			row.Currency = "CNY"
		}
		if len(row.InventoryConversionJSON) == 0 || string(row.InventoryConversionJSON) == "null" {
			row.InventoryConversionJSON = json.RawMessage(`{}`)
		}
		out = append(out, row)
	}
	return out
}

func (r Repository) loadGradientTemplatesByID(ctx context.Context, ids map[int64]bool) (map[int64]*domain.GradientTemplate, error) {
	out := map[int64]*domain.GradientTemplate{}
	if len(ids) == 0 {
		return out, nil
	}
	idList := make([]int64, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, display_unit, active
		FROM %s.pricing_gradient_templates
		WHERE id = ANY($1)
	`, r.schema), idList)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		template := &domain.GradientTemplate{}
		if err := rows.Scan(&template.ID, &template.Name, &template.DisplayUnit, &template.Active); err != nil {
			return nil, err
		}
		out[template.ID] = template
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	tierRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, label, min_weight_g::float8, max_weight_g::float8, margin_rate::float8, position
		FROM %s.pricing_gradient_template_tiers
		WHERE active=true AND template_id = ANY($1)
		ORDER BY template_id, position, min_weight_g, id
	`, r.schema), idList)
	if err != nil {
		return nil, err
	}
	defer tierRows.Close()
	for tierRows.Next() {
		var templateID int64
		var tier domain.GradientTemplateTier
		if err := tierRows.Scan(&tier.ID, &templateID, &tier.Label, &tier.MinWeightG, &tier.MaxWeightG, &tier.MarginRate, &tier.Position); err != nil {
			return nil, err
		}
		if template := out[templateID]; template != nil {
			template.Tiers = append(template.Tiers, tier)
		}
	}
	if err := tierRows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) ListParameterSettings(ctx context.Context) ([]appcosting.ParameterSetting, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT key, label, value::float8, unit, to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.cost_parameters
		ORDER BY key
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]appcosting.ParameterSetting, 0)
	for rows.Next() {
		var row appcosting.ParameterSetting
		if err := rows.Scan(&row.Key, &row.Label, &row.Value, &row.Unit, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) UpdateParameterSetting(ctx context.Context, cmd appcosting.UpdateParameterCommand) (appcosting.ParameterSetting, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return appcosting.ParameterSetting{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return appcosting.ParameterSetting{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var old appcosting.ParameterSetting
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT key, label, value::float8, unit, to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.cost_parameters
		WHERE key=$1
		FOR UPDATE
	`, r.schema), strings.TrimSpace(cmd.Key)).Scan(&old.Key, &old.Label, &old.Value, &old.Unit, &old.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return appcosting.ParameterSetting{}, fmt.Errorf("setting not found")
		}
		return appcosting.ParameterSetting{}, err
	}

	var next appcosting.ParameterSetting
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.cost_parameters
		SET value=$2, updated_at=now()
		WHERE key=$1
		RETURNING key, label, value::float8, unit, to_char(updated_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), strings.TrimSpace(cmd.Key), cmd.Value).Scan(&next.Key, &next.Label, &next.Value, &next.Unit, &next.UpdatedAt); err != nil {
		return appcosting.ParameterSetting{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "cost_parameter", nil, "update", postgresinfra.StrPtr(next.Key), postgresinfra.StrPtr(fmt.Sprintf("%.6f", old.Value)), postgresinfra.StrPtr(fmt.Sprintf("%.6f", next.Value)), postgresinfra.AuditMeta{
		"key":   next.Key,
		"label": next.Label,
		"unit":  next.Unit,
	}); err != nil {
		return appcosting.ParameterSetting{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return appcosting.ParameterSetting{}, err
	}
	return next, nil
}

func (r Repository) ListDripPriceTemplates(ctx context.Context) ([]domain.DripPriceTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, active, bag_grams::float8, box_bag_count, include_packaging
		FROM %s.drip_price_templates
		ORDER BY active DESC, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.DripPriceTemplate, 0)
	templateIndex := map[int64]int{}
	for rows.Next() {
		var row domain.DripPriceTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.Active, &row.BagGrams, &row.BoxBagCount, &row.IncludePackaging); err != nil {
			return nil, err
		}
		templateIndex[row.ID] = len(out)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(out))
	for _, row := range out {
		ids = append(ids, row.ID)
	}
	tierRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, label, min_bags::float8, max_bags::float8, multiplier::float8, position, active
		FROM %s.drip_price_template_tiers
		WHERE template_id = ANY($1)
		ORDER BY template_id, position, min_bags, id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer tierRows.Close()
	for tierRows.Next() {
		var templateID int64
		var tier domain.DripPriceTemplateTier
		if err := tierRows.Scan(&tier.ID, &templateID, &tier.Label, &tier.MinBags, &tier.MaxBags, &tier.Multiplier, &tier.Position, &tier.Active); err != nil {
			return nil, err
		}
		if idx, ok := templateIndex[templateID]; ok {
			out[idx].Tiers = append(out[idx].Tiers, tier)
		}
	}
	return out, tierRows.Err()
}

func (r Repository) SaveDripPriceTemplate(ctx context.Context, cmd appcosting.SaveDripPriceTemplateCommand) (*domain.DripPriceTemplate, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	active := true
	includePackaging := true
	if cmd.ID == 0 {
		active = true
		if cmd.Active != nil {
			active = *cmd.Active
		}
		if cmd.IncludePackaging != nil {
			includePackaging = *cmd.IncludePackaging
		}
	}
	var id int64
	if cmd.ID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT active, include_packaging
			FROM %s.drip_price_templates
			WHERE id=$1
			FOR UPDATE
		`, r.schema), cmd.ID).Scan(&active, &includePackaging); err != nil {
			if err == pgx.ErrNoRows {
				return nil, fmt.Errorf("template not found")
			}
			return nil, err
		}
		if cmd.Active != nil {
			active = *cmd.Active
		}
		if cmd.IncludePackaging != nil {
			includePackaging = *cmd.IncludePackaging
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.drip_price_templates
			SET name=$2, active=$3, bag_grams=$4, box_bag_count=$5, include_packaging=$6, updated_at=now()
			WHERE id=$1
			RETURNING id
		`, r.schema), cmd.ID, cmd.Name, active, cmd.BagGrams, cmd.BoxBagCount, includePackaging).Scan(&id); err != nil {
			if err == pgx.ErrNoRows {
				return nil, fmt.Errorf("template not found")
			}
			return nil, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.drip_price_template_tiers WHERE template_id=$1`, r.schema), id); err != nil {
			return nil, err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.drip_price_templates(name, active, bag_grams, box_bag_count, include_packaging)
			VALUES($1,$2,$3,$4,$5)
			RETURNING id
		`, r.schema), cmd.Name, active, cmd.BagGrams, cmd.BoxBagCount, includePackaging).Scan(&id); err != nil {
			return nil, err
		}
	}
	insertTier := fmt.Sprintf(`
		INSERT INTO %s.drip_price_template_tiers(template_id, label, min_bags, max_bags, multiplier, position, active)
		VALUES($1,$2,$3,$4,$5,$6,$7)
	`, r.schema)
	for i, tier := range cmd.Tiers {
		position := tier.Position
		if position <= 0 {
			position = i + 1
		}
		if _, err := tx.Exec(ctx, insertTier, id, tier.Label, tier.MinBags, tier.MaxBags, tier.Multiplier, position, true); err != nil {
			return nil, err
		}
	}
	action := "create"
	if cmd.ID > 0 {
		action = "update"
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "drip_price_template", &id, action, postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(cmd.Name), postgresinfra.AuditMeta{
		"template_id":       id,
		"bag_grams":         cmd.BagGrams,
		"box_bag_count":     cmd.BoxBagCount,
		"include_packaging": includePackaging,
		"tier_count":        len(cmd.Tiers),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	rows, err := r.ListDripPriceTemplates(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i], nil
		}
	}
	return nil, fmt.Errorf("template not found")
}

func (r Repository) DeactivateDripPriceTemplate(ctx context.Context, cmd appcosting.DeactivateDripPriceTemplateCommand) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.drip_price_templates SET active=false, updated_at=now() WHERE id=$1`, r.schema), cmd.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("template not found")
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "drip_price_template", &cmd.ID, "deactivate", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{
		"template_id": cmd.ID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListBeanListPublications(ctx context.Context, query appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error) {
	whereClause := "WHERE publication_purpose=$1 AND list_type=$2 AND owner_type=$3 AND owner_key=$4"
	args := []any{strings.TrimSpace(query.PublicationPurpose), strings.TrimSpace(query.ListType), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	orderClause := "ORDER BY CASE WHEN status='published' THEN 0 ELSE 1 END, created_at DESC, id DESC"
	if query.ClassificationTemplateID > 0 {
		whereClause = "WHERE publication_purpose=$1 AND owner_type=$3 AND owner_key=$4 AND (COALESCE(classification_template_id,0)=$2 OR (COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=$2) OR (COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=0 AND list_type=$5))"
		args = []any{strings.TrimSpace(query.PublicationPurpose), query.ClassificationTemplateID, strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey), strings.TrimSpace(query.ListType)}
		orderClause = "ORDER BY CASE WHEN status='published' THEN 0 ELSE 1 END, CASE WHEN COALESCE(classification_template_id,0)=$2 THEN 0 WHEN COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=$2 THEN 1 ELSE 2 END, created_at DESC, id DESC"
	} else if query.ProductTypeCategoryID > 0 {
		whereClause = "WHERE publication_purpose=$1 AND owner_type=$3 AND owner_key=$4 AND (COALESCE(product_type_category_id,0)=$2 OR (COALESCE(product_type_category_id,0)=0 AND list_type=$5))"
		args = []any{strings.TrimSpace(query.PublicationPurpose), query.ProductTypeCategoryID, strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey), strings.TrimSpace(query.ListType)}
		orderClause = "ORDER BY CASE WHEN status='published' THEN 0 ELSE 1 END, CASE WHEN COALESCE(product_type_category_id,0)=$2 THEN 0 ELSE 1 END, created_at DESC, id DESC"
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(publication_purpose,''),'factory_supply'),
		       list_type,
		       COALESCE(product_type_category_id,0),
		       COALESCE(product_type_name,''),
		       COALESCE(classification_template_id,0),
		       COALESCE(classification_template_name,''),
		       COALESCE(classification_category_id,0),
		       COALESCE(classification_category_name,''),
		       version_no,
		       status,
		       owner_type,
		       owner_key,
		       COALESCE(price_source_publication_id,0),
		       COALESCE(style_source_publication_id,0),
		       source_version_no,
		       config_json,
		       content_json,
		       changelog,
		       to_char(published_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		       to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.bean_list_publications
		%s
		%s
	`, r.schema, whereClause, orderClause), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]appcosting.BeanListPublication, 0)
	for rows.Next() {
		row, err := scanBeanListPublication(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) PublishedBeanList(ctx context.Context, query appcosting.BeanListPublicationQuery) (*appcosting.BeanListPublication, error) {
	whereClause := "publication_purpose=$1 AND list_type=$2 AND owner_type=$3 AND owner_key=$4"
	args := []any{strings.TrimSpace(query.PublicationPurpose), strings.TrimSpace(query.ListType), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	orderClause := "published_at DESC, id DESC"
	if query.ClassificationTemplateID > 0 {
		whereClause = "publication_purpose=$1 AND owner_type=$3 AND owner_key=$4 AND (COALESCE(classification_template_id,0)=$2 OR (COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=$2) OR (COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=0 AND list_type=$5))"
		args = []any{strings.TrimSpace(query.PublicationPurpose), query.ClassificationTemplateID, strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey), strings.TrimSpace(query.ListType)}
		orderClause = "CASE WHEN COALESCE(classification_template_id,0)=$2 THEN 0 WHEN COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=$2 THEN 1 ELSE 2 END, published_at DESC, id DESC"
	} else if query.ProductTypeCategoryID > 0 {
		whereClause = "publication_purpose=$1 AND owner_type=$3 AND owner_key=$4 AND (COALESCE(product_type_category_id,0)=$2 OR (COALESCE(product_type_category_id,0)=0 AND list_type=$5))"
		args = []any{strings.TrimSpace(query.PublicationPurpose), query.ProductTypeCategoryID, strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey), strings.TrimSpace(query.ListType)}
		orderClause = "CASE WHEN COALESCE(product_type_category_id,0)=$2 THEN 0 ELSE 1 END, published_at DESC, id DESC"
	}
	row, err := scanBeanListPublication(r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(publication_purpose,''),'factory_supply'),
		       list_type,
		       COALESCE(product_type_category_id,0),
		       COALESCE(product_type_name,''),
		       COALESCE(classification_template_id,0),
		       COALESCE(classification_template_name,''),
		       COALESCE(classification_category_id,0),
		       COALESCE(classification_category_name,''),
		       version_no,
		       status,
		       owner_type,
		       owner_key,
		       COALESCE(price_source_publication_id,0),
		       COALESCE(style_source_publication_id,0),
		       source_version_no,
		       config_json,
		       content_json,
		       changelog,
		       to_char(published_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		       to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.bean_list_publications
		WHERE %s AND status='published'
		ORDER BY %s
		LIMIT 1
	`, r.schema, whereClause, orderClause), args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r Repository) LoadBeanListPublication(ctx context.Context, query appcosting.BeanListPublicationQuery, publicationID int64) (*appcosting.BeanListPublication, error) {
	whereClause := "id=$1 AND publication_purpose=$2 AND list_type=$3 AND owner_type=$4 AND owner_key=$5"
	args := []any{publicationID, strings.TrimSpace(query.PublicationPurpose), strings.TrimSpace(query.ListType), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey)}
	if query.ClassificationTemplateID > 0 {
		whereClause = "id=$1 AND publication_purpose=$2 AND owner_type=$4 AND owner_key=$5 AND (COALESCE(classification_template_id,0)=$3 OR (COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=$3) OR (COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=0 AND list_type=$6))"
		args = []any{publicationID, strings.TrimSpace(query.PublicationPurpose), query.ClassificationTemplateID, strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey), strings.TrimSpace(query.ListType)}
	} else if query.ProductTypeCategoryID > 0 {
		whereClause = "id=$1 AND publication_purpose=$2 AND owner_type=$4 AND owner_key=$5 AND (COALESCE(product_type_category_id,0)=$3 OR (COALESCE(product_type_category_id,0)=0 AND list_type=$6))"
		args = []any{publicationID, strings.TrimSpace(query.PublicationPurpose), query.ProductTypeCategoryID, strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey), strings.TrimSpace(query.ListType)}
	}
	row, err := scanBeanListPublication(r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(publication_purpose,''),'factory_supply'),
		       list_type,
		       COALESCE(product_type_category_id,0),
		       COALESCE(product_type_name,''),
		       COALESCE(classification_template_id,0),
		       COALESCE(classification_template_name,''),
		       COALESCE(classification_category_id,0),
		       COALESCE(classification_category_name,''),
		       version_no,
		       status,
		       owner_type,
		       owner_key,
		       COALESCE(price_source_publication_id,0),
		       COALESCE(style_source_publication_id,0),
		       source_version_no,
		       config_json,
		       content_json,
		       changelog,
		       to_char(published_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		       to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.bean_list_publications
		WHERE %s
	`, r.schema, whereClause), args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appcosting.ErrBeanListPublicationNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r Repository) LoadBeanListPublicationAsset(ctx context.Context, publicationID int64, assetType string) (appcosting.BeanListPublicationAsset, error) {
	assetType = strings.TrimSpace(assetType)
	if assetType == "" {
		assetType = "pdf"
	}
	var asset appcosting.BeanListPublicationAsset
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT publication_id, asset_type, content_type, cache_key, payload
		FROM %s.bean_list_publication_assets
		WHERE publication_id=$1 AND asset_type=$2
	`, r.schema), publicationID, assetType).Scan(&asset.PublicationID, &asset.AssetType, &asset.ContentType, &asset.CacheKey, &asset.Payload)
	if err != nil {
		if err == pgx.ErrNoRows {
			return appcosting.BeanListPublicationAsset{}, appcosting.ErrBeanListPublicationNotFound
		}
		return appcosting.BeanListPublicationAsset{}, err
	}
	return asset, nil
}

func (r Repository) SaveBeanListPublicationAsset(ctx context.Context, asset appcosting.BeanListPublicationAsset, actor string) (appcosting.BeanListPublicationAsset, error) {
	asset.AssetType = strings.TrimSpace(asset.AssetType)
	if asset.AssetType == "" {
		asset.AssetType = "pdf"
	}
	asset.ContentType = strings.TrimSpace(asset.ContentType)
	if asset.ContentType == "" {
		asset.ContentType = "application/octet-stream"
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return appcosting.BeanListPublicationAsset{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var created bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
			WITH inserted AS (
				INSERT INTO %[1]s.bean_list_publication_assets(publication_id, asset_type, content_type, cache_key, payload, created_by)
				VALUES($1,$2,$3,$4,$5,$6)
				ON CONFLICT(publication_id, asset_type) DO UPDATE
				SET content_type=EXCLUDED.content_type,
				    cache_key=EXCLUDED.cache_key,
				    payload=EXCLUDED.payload,
				    created_by=EXCLUDED.created_by,
				    updated_at=now()
				RETURNING publication_id, asset_type, content_type, cache_key, payload, true
			)
		SELECT publication_id, asset_type, content_type, cache_key, payload, true FROM inserted
		UNION ALL
		SELECT publication_id, asset_type, content_type, cache_key, payload, false
		FROM %[1]s.bean_list_publication_assets
		WHERE publication_id=$1 AND asset_type=$2 AND NOT EXISTS (SELECT 1 FROM inserted)
		LIMIT 1
	`, r.schema), asset.PublicationID, asset.AssetType, asset.ContentType, asset.CacheKey, asset.Payload, strings.TrimSpace(actor)).
		Scan(&asset.PublicationID, &asset.AssetType, &asset.ContentType, &asset.CacheKey, &asset.Payload, &created); err != nil {
		return appcosting.BeanListPublicationAsset{}, err
	}
	if created {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, strings.TrimSpace(actor), "bean_list_publication_asset", &asset.PublicationID, "create", postgresinfra.StrPtr("asset_type"), nil, postgresinfra.StrPtr(asset.AssetType), postgresinfra.AuditMeta{
			"publication_id": asset.PublicationID,
			"asset_type":     asset.AssetType,
			"cache_key":      asset.CacheKey,
		}); err != nil {
			return appcosting.BeanListPublicationAsset{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return appcosting.BeanListPublicationAsset{}, err
	}
	return asset, nil
}

func (r Repository) PublishBeanList(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := validateBeanListProductScope(ctx, tx, r.schema, cmd); err != nil {
		return nil, err
	}

	config, err := json.Marshal(cmd.Config)
	if err != nil {
		return nil, err
	}
	content, err := json.Marshal(cmd.Content)
	if err != nil {
		return nil, err
	}
	var published appcosting.BeanListPublication
	var configJSON, contentJSON []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications(publication_purpose, list_type, product_type_category_id, product_type_name, classification_template_id, classification_template_name, classification_category_id, classification_category_name, version_no, status, owner_type, owner_key, price_source_publication_id, style_source_publication_id, source_version_no, config_json, content_json, changelog, actor)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'published',$10,$11,NULLIF($12,0),NULLIF($13,0),$14,$15::jsonb,$16::jsonb,$17,$18)
		RETURNING id, COALESCE(NULLIF(publication_purpose,''),'factory_supply'), list_type, COALESCE(product_type_category_id,0), COALESCE(product_type_name,''), COALESCE(classification_template_id,0), COALESCE(classification_template_name,''), COALESCE(classification_category_id,0), COALESCE(classification_category_name,''), version_no, status, owner_type, owner_key, COALESCE(price_source_publication_id,0), COALESCE(style_source_publication_id,0), source_version_no, config_json, content_json, changelog,
		          to_char(published_at,'YYYY-MM-DD HH24:MI'),
		          COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		          to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), cmd.PublicationPurpose, cmd.ListType, cmd.ProductTypeCategoryID, cmd.ProductTypeName, cmd.ClassificationTemplateID, cmd.ClassificationTemplateName, cmd.ClassificationCategoryID, cmd.ClassificationCategoryName, cmd.Version, cmd.OwnerType, cmd.OwnerKey, cmd.PriceSourcePublicationID, cmd.StyleSourcePublicationID, cmd.SourceVersion, config, content, cmd.Changelog, cmd.Actor).Scan(
		&published.ID,
		&published.PublicationPurpose,
		&published.ListType,
		&published.ProductTypeCategoryID,
		&published.ProductTypeName,
		&published.ClassificationTemplateID,
		&published.ClassificationTemplateName,
		&published.ClassificationCategoryID,
		&published.ClassificationCategoryName,
		&published.Version,
		&published.Status,
		&published.OwnerType,
		&published.OwnerKey,
		&published.PriceSourcePublicationID,
		&published.StyleSourcePublicationID,
		&published.SourceVersion,
		&configJSON,
		&contentJSON,
		&published.Changelog,
		&published.PublishedAt,
		&published.WithdrawnAt,
		&published.CreatedAt,
	); err != nil {
		return nil, err
	}
	published.Config = map[string]any{}
	published.Content = map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &published.Config); err != nil {
			return nil, err
		}
	}
	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &published.Content); err != nil {
			return nil, err
		}
	}
	if published.ID <= 0 {
		return nil, fmt.Errorf("publish failed")
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &published.ID, "publish", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("published"), postgresinfra.AuditMeta{
		"publication_purpose":          cmd.PublicationPurpose,
		"list_type":                    cmd.ListType,
		"product_type_category_id":     cmd.ProductTypeCategoryID,
		"product_type_name":            cmd.ProductTypeName,
		"classification_template_id":   cmd.ClassificationTemplateID,
		"classification_template_name": cmd.ClassificationTemplateName,
		"version":                      cmd.Version,
		"owner_type":                   cmd.OwnerType,
		"owner_key":                    cmd.OwnerKey,
		"price_source_publication":     cmd.PriceSourcePublicationID,
		"style_source_publication":     cmd.StyleSourcePublicationID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &published, nil
}

func (r Repository) SaveBeanListDraft(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := validateBeanListProductScope(ctx, tx, r.schema, cmd); err != nil {
		return nil, err
	}
	config, err := json.Marshal(cmd.Config)
	if err != nil {
		return nil, err
	}
	content, err := json.Marshal(cmd.Content)
	if err != nil {
		return nil, err
	}
	var draft appcosting.BeanListPublication
	var configJSON, contentJSON []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications(publication_purpose, list_type, product_type_category_id, product_type_name, classification_template_id, classification_template_name, classification_category_id, classification_category_name, version_no, status, owner_type, owner_key, price_source_publication_id, style_source_publication_id, source_version_no, config_json, content_json, changelog, actor)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft',$10,$11,NULLIF($12,0),NULLIF($13,0),$14,$15::jsonb,$16::jsonb,$17,$18)
		RETURNING id, COALESCE(NULLIF(publication_purpose,''),'factory_supply'), list_type, COALESCE(product_type_category_id,0), COALESCE(product_type_name,''), COALESCE(classification_template_id,0), COALESCE(classification_template_name,''), COALESCE(classification_category_id,0), COALESCE(classification_category_name,''), version_no, status, owner_type, owner_key, COALESCE(price_source_publication_id,0), COALESCE(style_source_publication_id,0), source_version_no, config_json, content_json, changelog,
		          to_char(published_at,'YYYY-MM-DD HH24:MI'),
		          COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''),
		          to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), cmd.PublicationPurpose, cmd.ListType, cmd.ProductTypeCategoryID, cmd.ProductTypeName, cmd.ClassificationTemplateID, cmd.ClassificationTemplateName, cmd.ClassificationCategoryID, cmd.ClassificationCategoryName, cmd.Version, cmd.OwnerType, cmd.OwnerKey, cmd.PriceSourcePublicationID, cmd.StyleSourcePublicationID, cmd.SourceVersion, config, content, cmd.Changelog, cmd.Actor).Scan(
		&draft.ID,
		&draft.PublicationPurpose,
		&draft.ListType,
		&draft.ProductTypeCategoryID,
		&draft.ProductTypeName,
		&draft.ClassificationTemplateID,
		&draft.ClassificationTemplateName,
		&draft.ClassificationCategoryID,
		&draft.ClassificationCategoryName,
		&draft.Version,
		&draft.Status,
		&draft.OwnerType,
		&draft.OwnerKey,
		&draft.PriceSourcePublicationID,
		&draft.StyleSourcePublicationID,
		&draft.SourceVersion,
		&configJSON,
		&contentJSON,
		&draft.Changelog,
		&draft.PublishedAt,
		&draft.WithdrawnAt,
		&draft.CreatedAt,
	); err != nil {
		return nil, err
	}
	draft.Config = map[string]any{}
	draft.Content = map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &draft.Config); err != nil {
			return nil, err
		}
	}
	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &draft.Content); err != nil {
			return nil, err
		}
	}
	if draft.ID <= 0 {
		return nil, fmt.Errorf("save draft failed")
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &draft.ID, "save_draft", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("draft"), postgresinfra.AuditMeta{
		"publication_purpose":          cmd.PublicationPurpose,
		"list_type":                    cmd.ListType,
		"product_type_category_id":     cmd.ProductTypeCategoryID,
		"product_type_name":            cmd.ProductTypeName,
		"classification_template_id":   cmd.ClassificationTemplateID,
		"classification_template_name": cmd.ClassificationTemplateName,
		"version":                      cmd.Version,
		"owner_type":                   cmd.OwnerType,
		"owner_key":                    cmd.OwnerKey,
		"price_source_publication":     cmd.PriceSourcePublicationID,
		"style_source_publication":     cmd.StyleSourcePublicationID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &draft, nil
}

func (r Repository) WithdrawBeanList(ctx context.Context, cmd appcosting.WithdrawBeanListCommand) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var publicationPurpose, listType, version, ownerType, ownerKey string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.bean_list_publications
		SET status='withdrawn', withdrawn_at=now(), updated_at=now()
		WHERE id=$1 AND publication_purpose=$2 AND owner_type=$3 AND owner_key=$4 AND status='published'
		RETURNING COALESCE(NULLIF(publication_purpose,''),'factory_supply'), list_type, version_no, owner_type, owner_key
	`, r.schema), cmd.ID, cmd.PublicationPurpose, cmd.OwnerType, cmd.OwnerKey).Scan(&publicationPurpose, &listType, &version, &ownerType, &ownerKey); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("published bean list not found")
		}
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &cmd.ID, "withdraw", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("published"), postgresinfra.StrPtr("withdrawn"), postgresinfra.AuditMeta{
		"publication_purpose": publicationPurpose,
		"list_type":           listType,
		"version":             version,
		"owner_type":          ownerType,
		"owner_key":           ownerKey,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ArchiveBeanListPublications(ctx context.Context, cmd appcosting.ArchiveBeanListPublicationsCommand) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		WITH selected AS (
			SELECT id, COALESCE(NULLIF(publication_purpose,''),'factory_supply') AS publication_purpose,
			       list_type, version_no, owner_type, owner_key, status AS old_status
			FROM %s.bean_list_publications
			WHERE id=ANY($1) AND publication_purpose=$2 AND owner_type=$3 AND owner_key=$4 AND status<>'archived'
		)
		UPDATE %s.bean_list_publications b
		SET status='archived',
		    config_json=jsonb_set(COALESCE(b.config_json,'{}'::jsonb), '{archived_from_status}', to_jsonb(s.old_status), true),
		    updated_at=now()
		FROM selected s
		WHERE b.id=s.id
		RETURNING b.id, s.publication_purpose, s.list_type, s.version_no, s.owner_type, s.owner_key, s.old_status
	`, r.schema, r.schema), cmd.IDs, cmd.PublicationPurpose, cmd.OwnerType, cmd.OwnerKey)
	if err != nil {
		return err
	}
	defer rows.Close()
	type archivedRow struct {
		id                 int64
		publicationPurpose string
		listType           string
		version            string
		ownerType          string
		ownerKey           string
		oldStatus          string
	}
	archived := make([]archivedRow, 0)
	for rows.Next() {
		var row archivedRow
		if err := rows.Scan(&row.id, &row.publicationPurpose, &row.listType, &row.version, &row.ownerType, &row.ownerKey, &row.oldStatus); err != nil {
			return err
		}
		archived = append(archived, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(archived) == 0 {
		return fmt.Errorf("bean list publication not found")
	}
	for _, row := range archived {
		id := row.id
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &id, "archive", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(row.oldStatus), postgresinfra.StrPtr("archived"), postgresinfra.AuditMeta{
			"publication_purpose": row.publicationPurpose,
			"list_type":           row.listType,
			"version":             row.version,
			"owner_type":          row.ownerType,
			"owner_key":           row.ownerKey,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r Repository) UnarchiveBeanListPublications(ctx context.Context, cmd appcosting.ArchiveBeanListPublicationsCommand) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		WITH selected AS (
			SELECT id, COALESCE(NULLIF(publication_purpose,''),'factory_supply') AS publication_purpose,
			       list_type, version_no, owner_type, owner_key, status AS old_status,
			       COALESCE(NULLIF(config_json->>'archived_from_status',''), 'published') AS restored_status
			FROM %s.bean_list_publications
			WHERE id=ANY($1) AND publication_purpose=$2 AND owner_type=$3 AND owner_key=$4 AND status='archived'
		)
		UPDATE %s.bean_list_publications b
		SET status=s.restored_status,
		    config_json = b.config_json - 'archived_from_status',
		    updated_at=now()
		FROM selected s
		WHERE b.id=s.id
		RETURNING b.id, s.publication_purpose, s.list_type, s.version_no, s.owner_type, s.owner_key, s.old_status, s.restored_status
	`, r.schema, r.schema), cmd.IDs, cmd.PublicationPurpose, cmd.OwnerType, cmd.OwnerKey)
	if err != nil {
		return err
	}
	defer rows.Close()
	type unarchivedRow struct {
		id                 int64
		publicationPurpose string
		listType           string
		version            string
		ownerType          string
		ownerKey           string
		oldStatus          string
		newStatus          string
	}
	unarchived := make([]unarchivedRow, 0)
	for rows.Next() {
		var row unarchivedRow
		if err := rows.Scan(&row.id, &row.publicationPurpose, &row.listType, &row.version, &row.ownerType, &row.ownerKey, &row.oldStatus, &row.newStatus); err != nil {
			return err
		}
		unarchived = append(unarchived, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(unarchived) == 0 {
		return fmt.Errorf("archived bean list publication not found")
	}
	for _, row := range unarchived {
		id := row.id
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &id, "unarchive", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(row.oldStatus), postgresinfra.StrPtr(row.newStatus), postgresinfra.AuditMeta{
			"publication_purpose": row.publicationPurpose,
			"list_type":           row.listType,
			"version":             row.version,
			"owner_type":          row.ownerType,
			"owner_key":           row.ownerKey,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r Repository) CreateRun(ctx context.Context, actor string, items []domain.ProductResult) (*appcosting.Run, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.cost_calculation_runs(status, actor, product_count)
		VALUES('draft',$1,$2) RETURNING id`, r.schema), actor, len(items)).Scan(&id); err != nil {
		return nil, err
	}
	ins := fmt.Sprintf(`INSERT INTO %s.cost_calculation_items(run_id, product_id, product_name, result_json)
		VALUES($1,$2,$3,$4)`, r.schema)
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, ins, id, item.ProductID, item.Name, b); err != nil {
			return nil, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "costing_run", &id, "create", postgresinfra.StrPtr("product_count"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", len(items))), postgresinfra.AuditMeta{
		"run_id":        id,
		"product_count": len(items),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &appcosting.Run{ID: id, Status: "draft", ProductCount: len(items), Items: items}, nil
}

func (r Repository) PublishRun(ctx context.Context, actor string, runID int64) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	items, err := loadRunItems(ctx, tx, r.schema, runID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("costing run has no items")
	}

	updateProduct := fmt.Sprintf(`UPDATE %s.products
		SET default_price=$2,
		    retail_price_100g=$3,
		    retail_price_200g=$4,
		    retail_price_227g=$5,
		    retail_price_250g=$6
		WHERE id=$1`, r.schema)
	deleteTiers := fmt.Sprintf(`DELETE FROM %s.product_price_tiers WHERE product_id=$1`, r.schema)
	deleteLegacyTiers := fmt.Sprintf(`DELETE FROM %s.product_price_tiers WHERE product_id=$1 AND COALESCE(bom_spec_id,0)=0`, r.schema)
	deleteSpecTiers := fmt.Sprintf(`DELETE FROM %s.product_price_tiers WHERE product_id=$1 AND bom_spec_id=$2`, r.schema)
	insertTier := fmt.Sprintf(`INSERT INTO %s.product_price_tiers
		(product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active, product_kind, price_basis, sales_unit, unit_bag_count, price_source_json)
		VALUES($1,$2,$3,$4,$5,$3,$4,$6,true,$7,'weight','',0,$8::jsonb)`, r.schema)
	insertSpecTier := fmt.Sprintf(`INSERT INTO %s.product_price_tiers
		(product_id,bom_spec_id,bom_variant_id,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active,product_kind,price_basis,sales_unit,unit_bag_count,price_source_json)
		VALUES($1,$2,$3,$4,$5,$6,$7,$5,$6,$8,true,$9,'weight','',0,$10::jsonb)`, r.schema)
	insertDripTier := fmt.Sprintf(`INSERT INTO %s.product_price_tiers
		(product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active, product_kind, price_basis, sales_unit, unit_bag_count, price_source_json)
		VALUES($1,$2,$3,$4,$5,NULL,NULL,NULL,true,$6,'unit',$7,$8,$9::jsonb)`, r.schema)
	insertSpecDripTier := fmt.Sprintf(`INSERT INTO %s.product_price_tiers
		(product_id,bom_spec_id,bom_variant_id,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active,product_kind,price_basis,sales_unit,unit_bag_count,price_source_json)
		VALUES($1,$2,$3,$4,$5,$6,$7,NULL,NULL,NULL,true,$8,'unit',$9,$10,$11::jsonb)`, r.schema)
	tierSpecIdentityEnabled, err := productPriceTierSpecIdentityEnabled(ctx, tx, r.schema)
	if err != nil {
		return err
	}
	publishedProducts := 0
	for _, item := range items {
		if item.ProductID <= 0 {
			continue
		}
		if strings.TrimSpace(item.ProductKind) == "green_bean" {
			continue
		}
		defaultPrice := 0.0
		if len(item.CommercialWholesaleTiers) > 0 {
			defaultPrice = item.CommercialWholesaleTiers[0].PricePerUnit
		} else if len(item.WholesaleKgPrices) > 0 {
			defaultPrice = item.WholesaleKgPrices[0]
		}
		isSpecPrice := item.BomSpecID > 0
		if isSpecPrice && !tierSpecIdentityEnabled {
			return fmt.Errorf("product_price_tiers BOM specification identity schema is unavailable")
		}
		if !isSpecPrice || item.IsDefaultSKU {
			if _, err := tx.Exec(ctx, updateProduct, item.ProductID, defaultPrice, item.Retail100gPrice, item.Retail200gPrice, item.Retail227gPrice, item.Retail250gPrice); err != nil {
				return err
			}
		}
		if isSpecPrice {
			if _, err := tx.Exec(ctx, deleteSpecTiers, item.ProductID, item.BomSpecID); err != nil {
				return err
			}
		} else if tierSpecIdentityEnabled {
			if _, err := tx.Exec(ctx, deleteLegacyTiers, item.ProductID); err != nil {
				return err
			}
		} else if _, err := tx.Exec(ctx, deleteTiers, item.ProductID); err != nil {
			return err
		}
		if item.ProductKind == "drip_bag" {
			for _, tier := range item.DripWholesaleTiers {
				bagGrams := tier.BagGrams
				if bagGrams <= 0 {
					bagGrams = item.DripBagGrams
				}
				if bagGrams <= 0 {
					bagGrams = 10
				}
				boxBagCount := tier.BoxBagCount
				if boxBagCount <= 0 {
					boxBagCount = item.DripBoxBagCount
				}
				if boxBagCount <= 0 {
					boxBagCount = 10
				}
				source := dripPriceSourceJSON(tier, bagGrams, boxBagCount)
				if isSpecPrice {
					source = priceSourceWithBOMSpec(source, item.BomSpecID, item.BomVariantID)
					if _, err := tx.Exec(ctx, insertSpecDripTier, item.ProductID, item.BomSpecID, item.BomVariantID, int64(math.Round(bagGrams)), tier.MinBags, tier.MaxBags, tier.PackedPricePerBag, item.ProductKind, "bag", 1, source); err != nil {
						return err
					}
				} else if _, err := tx.Exec(ctx, insertDripTier, item.ProductID, int64(math.Round(bagGrams)), tier.MinBags, tier.MaxBags, tier.PackedPricePerBag, item.ProductKind, "bag", 1, source); err != nil {
					return err
				}
				minBoxes := dripBoxMinQty(tier.MinBags, boxBagCount)
				maxBoxes := dripBoxMaxQty(tier.MaxBags, boxBagCount)
				boxSource := dripPriceSourceJSON(tier, bagGrams, boxBagCount)
				if isSpecPrice {
					boxSource = priceSourceWithBOMSpec(boxSource, item.BomSpecID, item.BomVariantID)
					if _, err := tx.Exec(ctx, insertSpecDripTier, item.ProductID, item.BomSpecID, item.BomVariantID, int64(math.Round(bagGrams))*int64(boxBagCount), minBoxes, maxBoxes, tier.PackedPricePerBag*float64(boxBagCount), item.ProductKind, "box", boxBagCount, boxSource); err != nil {
						return err
					}
				} else if _, err := tx.Exec(ctx, insertDripTier, item.ProductID, int64(math.Round(bagGrams))*int64(boxBagCount), minBoxes, maxBoxes, tier.PackedPricePerBag*float64(boxBagCount), item.ProductKind, "box", boxBagCount, boxSource); err != nil {
					return err
				}
			}
		} else {
			for _, tier := range commercialTiersForPublish(item) {
				specG := tier.SpecG
				if specG <= 0 {
					specG = 454
				}
				minQty := tier.MinQty
				if minQty <= 0 {
					minQty = tier.MinLb
				}
				maxQty := tier.MaxQty
				if maxQty == nil {
					maxQty = tier.MaxLb
				}
				pricePerUnit := tier.PricePerUnit
				if pricePerUnit == 0 {
					pricePerUnit = tier.PricePerLb
				}
				pricePerLb := pricePerUnit * 454.0 / float64(specG)
				source := commercialPriceSourceJSON(tier)
				if isSpecPrice {
					source = priceSourceWithBOMSpec(source, item.BomSpecID, item.BomVariantID)
					if _, err := tx.Exec(ctx, insertSpecTier, item.ProductID, item.BomSpecID, item.BomVariantID, specG, minQty, maxQty, pricePerUnit, pricePerLb, firstNonEmptyString(item.ProductKind, "roasted_bean"), source); err != nil {
						return err
					}
				} else if _, err := tx.Exec(ctx, insertTier, item.ProductID, specG, minQty, maxQty, pricePerUnit, pricePerLb, firstNonEmptyString(item.ProductKind, "roasted_bean"), source); err != nil {
					return err
				}
			}
		}
		publishedProducts++
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.cost_calculation_runs SET status='published', published_at=now() WHERE id=$1`, r.schema), runID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "costing_run", &runID, "publish", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("draft"), postgresinfra.StrPtr("published"), postgresinfra.AuditMeta{
		"run_id":             runID,
		"published_products": publishedProducts,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func dripPriceSourceJSON(tier domain.DripWholesaleTier, bagGrams float64, boxBagCount int) string {
	b, _ := json.Marshal(map[string]any{
		"template_id":          tier.TemplateID,
		"tier_id":              tier.TemplateTierID,
		"bag_grams":            bagGrams,
		"box_bag_count":        boxBagCount,
		"loose_price_per_bag":  tier.LoosePricePerBag,
		"packed_price_per_bag": tier.PackedPricePerBag,
		"multiplier":           tier.Multiplier,
		"tax_rate":             tier.TaxRate,
	})
	return string(b)
}

func commercialPriceSourceJSON(tier domain.CommercialWholesaleTier) string {
	b, _ := json.Marshal(map[string]any{
		"template_id":      tier.TemplateID,
		"template_tier_id": tier.TemplateTierID,
		"display_unit":     tier.DisplayUnit,
		"price_unit":       firstNonEmptyString(tier.PriceUnit, tier.DisplayUnit),
		"price_per_unit":   tier.PricePerUnit,
		"price_per_kg":     tier.PricePerKg,
		"price_per_lb":     tier.PricePerLb,
		"margin_rate":      tier.MarginRate,
	})
	return string(b)
}

func priceSourceWithBOMSpec(source string, bomSpecID, bomVariantID int64) string {
	if bomSpecID <= 0 {
		return source
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(source), &payload); err != nil {
		payload = map[string]any{}
	}
	payload["bom_spec_id"] = bomSpecID
	if bomVariantID > 0 {
		payload["bom_variant_id"] = bomVariantID
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return source
	}
	return string(b)
}

func productPriceTierSpecIdentityEnabled(ctx context.Context, tx pgx.Tx, schema string) (bool, error) {
	var count int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM information_schema.columns
		WHERE table_schema=$1 AND table_name='product_price_tiers'
		  AND column_name IN ('bom_spec_id','bom_variant_id')
	`, schema).Scan(&count)
	return count == 2, err
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func dripBoxMinQty(minBags int64, boxBagCount int) float64 {
	if boxBagCount <= 0 {
		boxBagCount = 10
	}
	return math.Ceil(float64(minBags) / float64(boxBagCount))
}

func dripBoxMaxQty(maxBags *float64, boxBagCount int) *float64 {
	if maxBags == nil {
		return nil
	}
	if boxBagCount <= 0 {
		boxBagCount = 10
	}
	v := math.Floor(*maxBags / float64(boxBagCount))
	return &v
}

func loadRunItems(ctx context.Context, tx pgx.Tx, schema string, runID int64) ([]domain.ProductResult, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT result_json FROM %s.cost_calculation_items WHERE run_id=$1 ORDER BY id`, schema), runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.ProductResult, 0)
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var item domain.ProductResult
		if err := json.Unmarshal(b, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type beanListPublicationScanner interface {
	Scan(dest ...any) error
}

func scanBeanListPublication(row beanListPublicationScanner) (appcosting.BeanListPublication, error) {
	var out appcosting.BeanListPublication
	var configJSON, contentJSON []byte
	if err := row.Scan(
		&out.ID,
		&out.PublicationPurpose,
		&out.ListType,
		&out.ProductTypeCategoryID,
		&out.ProductTypeName,
		&out.ClassificationTemplateID,
		&out.ClassificationTemplateName,
		&out.ClassificationCategoryID,
		&out.ClassificationCategoryName,
		&out.Version,
		&out.Status,
		&out.OwnerType,
		&out.OwnerKey,
		&out.PriceSourcePublicationID,
		&out.StyleSourcePublicationID,
		&out.SourceVersion,
		&configJSON,
		&contentJSON,
		&out.Changelog,
		&out.PublishedAt,
		&out.WithdrawnAt,
		&out.CreatedAt,
	); err != nil {
		return out, err
	}
	out.Config = map[string]any{}
	out.Content = map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &out.Config); err != nil {
			return out, err
		}
	}
	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &out.Content); err != nil {
			return out, err
		}
	}
	return out, nil
}

func applyParameter(params *domain.Parameters, key string, value float64) {
	switch strings.TrimSpace(key) {
	case "roast_yield_rate":
		params.RoastYieldRate = value
	case "kg_to_lb_factor":
		params.KgToLbFactor = value
	case "small_batch_production_cost_per_kg":
		params.SmallBatchProductionCostPerKg = value
	case "large_batch_production_cost_per_kg":
		params.LargeBatchProductionCostPerKg = value
	case "wholesale_package_cost_per_kg":
		params.WholesalePackageCostPerKg = value
	case "product_loss_per_kg":
		params.ProductLossPerKg = value
	case "retail_bean_margin_rate":
		params.RetailBeanMarginRate = value
	case "retail_tax_rate":
		params.RetailTaxRate = value
	case "retail_logistics_per_kg":
		params.RetailLogisticsPerKg = value
	case "retail_drip_logistics_per_10_bags":
		params.RetailDripLogisticsPer10Bags = value
	case "drip_green_ratio_kg_per_bag":
		params.DripGreenRatioKgPerBag = value
	case "drip_process_cost_per_bag":
		params.DripProcessCostPerBag = value
	case "drip_extra_cost_per_bag":
		params.DripExtraCostPerBag = value
	case "drip_packing_material_per_bag":
		params.DripPackingMaterialPerBag = value
	case "retail_drip_multiplier":
		params.RetailDripMultiplier = value
	case "wholesale_kg_margin_rate_1":
		params.WholesaleKgMarginRates[0] = value
	case "wholesale_kg_margin_rate_2":
		params.WholesaleKgMarginRates[1] = value
	case "wholesale_kg_margin_rate_3":
		params.WholesaleKgMarginRates[2] = value
	case "wholesale_kg_margin_rate_4":
		params.WholesaleKgMarginRates[3] = value
	case "wholesale_kg_margin_rate_5":
		params.WholesaleKgMarginRates[4] = value
	case "wholesale_kg_margin_rate_6":
		params.WholesaleKgMarginRates[5] = value
	case "wholesale_drip_multiplier_1":
		params.WholesaleDripMultipliers[0] = value
	case "wholesale_drip_multiplier_2":
		params.WholesaleDripMultipliers[1] = value
	case "wholesale_drip_multiplier_3":
		params.WholesaleDripMultipliers[2] = value
	case "wholesale_drip_multiplier_4":
		params.WholesaleDripMultipliers[3] = value
	}
}

func floatPtr(v float64) *float64 {
	return &v
}

func validateBeanListProductScope(ctx context.Context, tx pgx.Tx, schema string, cmd appcosting.PublishBeanListCommand) error {
	if strings.TrimSpace(cmd.PublicationPurpose) == appcosting.BeanListPublicationPurposeCustomerResale {
		return nil
	}
	ids := beanListContentProductIDs(cmd.Content)
	if len(ids) == 0 {
		return nil
	}
	ownerType := strings.TrimSpace(cmd.OwnerType)
	if ownerType != "official" && ownerType != "customer" {
		return nil
	}
	customerID := int64(0)
	if ownerType == "customer" {
		id, err := strconv.ParseInt(strings.TrimSpace(cmd.OwnerKey), 10, 64)
		if err != nil || id <= 0 {
			return fmt.Errorf("customer_id required")
		}
		customerID = id
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(customer_id,0)
		FROM %s.products
		WHERE active=true AND id = ANY($1)
	`, schema), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	allowPublicProductArchiveRows := ownerType == "customer" && beanListContentHasPR440FlatRowsForProducts(cmd.Content, ids)
	seen := map[int64]bool{}
	for rows.Next() {
		var productID, productCustomerID int64
		if err := rows.Scan(&productID, &productCustomerID); err != nil {
			return err
		}
		seen[productID] = true
		if ownerType == "official" && productCustomerID > 0 {
			return fmt.Errorf("official bean list cannot include customer SKU")
		}
		if ownerType == "customer" && productCustomerID > 0 && productCustomerID != customerID {
			return fmt.Errorf("customer bean list cannot include another customer's SKU")
		}
		if ownerType == "customer" && productCustomerID <= 0 && !allowPublicProductArchiveRows {
			return fmt.Errorf("customer bean list cannot include public SKU")
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if !seen[id] {
			return fmt.Errorf("bean list product not found")
		}
	}
	return nil
}

func beanListContentProductIDs(content map[string]any) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0)
	appendID := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if groups, ok := anySlice(content["groups"]); ok {
		for _, group := range groups {
			groupMap, ok := anyMap(group)
			if !ok {
				continue
			}
			items, ok := anySlice(groupMap["items"])
			if !ok {
				continue
			}
			for _, item := range items {
				itemMap, ok := anyMap(item)
				if !ok {
					continue
				}
				id := anyInt64(itemMap["productId"])
				if id <= 0 {
					id = anyInt64(itemMap["product_id"])
				}
				if id <= 0 {
					id = anyInt64(itemMap["productID"])
				}
				appendID(id)
			}
		}
	}
	if rows, ok := anySlice(content["price_rows"]); ok {
		for _, raw := range rows {
			row, ok := anyMap(raw)
			if !ok {
				continue
			}
			productID := anyInt64(row["product_id"])
			if productID <= 0 {
				productID = anyInt64(row["productId"])
			}
			appendID(productID)
			appendID(anyInt64(row["sku_id"]))
		}
	}
	return out
}

func beanListContentHasPR440FlatRowsForProducts(content map[string]any, productIDs []int64) bool {
	if len(productIDs) == 0 {
		return false
	}
	rows, ok := anySlice(content["price_rows"])
	if !ok || len(rows) == 0 {
		return false
	}
	needed := make(map[int64]bool, len(productIDs))
	for _, id := range productIDs {
		if id > 0 {
			needed[id] = false
		}
	}
	if len(needed) == 0 {
		return false
	}
	for _, raw := range rows {
		row, ok := anyMap(raw)
		if !ok {
			continue
		}
		id := anyInt64(row["product_id"])
		if id <= 0 {
			id = anyInt64(row["productId"])
		}
		if id <= 0 {
			id = anyInt64(row["productID"])
		}
		if _, exists := needed[id]; exists {
			needed[id] = true
		}
		skuID := anyInt64(row["sku_id"])
		if _, exists := needed[skuID]; exists {
			needed[skuID] = true
		}
	}
	for _, found := range needed {
		if !found {
			return false
		}
	}
	return true
}

func anySlice(value any) ([]any, bool) {
	switch v := value.(type) {
	case []any:
		return v, true
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out, true
	default:
		return nil, false
	}
}

func anyMap(value any) (map[string]any, bool) {
	v, ok := value.(map[string]any)
	return v, ok
}

func anyInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		i, _ := v.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return i
	default:
		return 0
	}
}

func anyFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		return 0
	}
}

func commercialTiersForPublish(item domain.ProductResult) []domain.CommercialWholesaleTier {
	if len(item.CommercialWholesaleTiers) > 0 {
		return item.CommercialWholesaleTiers
	}
	return nil
}
