package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const defaultSchema = "p2rms15pepb5ciz"

type refSpec struct {
	table    string
	optional bool
	deferred bool
}

type tableSpec struct {
	name      string
	id        string
	keys      []string
	refs      map[string]refSpec
	preserve  map[string]bool
	customKey func(row map[string]any) string
}

type batchSpec struct {
	name   string
	tables []tableSpec
}

type counts struct {
	Added, Updated, Preserved, Skipped, Conflicts int
}

type migrator struct {
	source, target *pgx.Conn
	schema         string
	dryRun         bool
	idMaps         map[string]map[string]string
	nextIDs        map[string]int64
	report         map[string]counts
	columns        map[string]map[string]columnMeta
}

type columnMeta struct {
	dataType string
	nullable bool
}

type dbQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func qident(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func norm(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	default:
		return strings.TrimSpace(strings.ToLower(fmt.Sprint(v)))
	}
}

func dbValue(v any) any {
	n, ok := v.(json.Number)
	if !ok {
		return v
	}
	if i, err := n.Int64(); err == nil {
		return i
	}
	if f, err := n.Float64(); err == nil {
		return f
	}
	return n.String()
}

func keyFrom(cols []string, row map[string]any) string {
	parts := make([]string, len(cols))
	for i, col := range cols {
		parts[i] = norm(row[col])
	}
	return strings.Join(parts, "\x1f")
}

func productKey(row map[string]any) string {
	if sku := norm(row["sku_code"]); sku != "" {
		return "sku:" + sku
	}
	return "legacy:" + keyFrom([]string{
		"customer_id", "name", "spec_label", "net_content_qty", "net_content_unit", "product_kind",
		"sku_name", "barcode", "derived_spec_key", "is_default_sku", "auto_derived_sku", "roast_level",
	}, row)
}

func specs() []batchSpec {
	r := func(table string, optional, deferred bool) refSpec {
		return refSpec{table: table, optional: optional, deferred: deferred}
	}
	return []batchSpec{
		{name: "base", tables: []tableSpec{
			{name: "sources", id: "id", keys: []string{"name"}},
			{name: "order_types", id: "id", keys: []string{"name"}},
			{name: "pay_statuses", id: "id", keys: []string{"name"}},
			{name: "ship_statuses", id: "id", keys: []string{"name"}},
			{name: "logistics_companies", id: "id", keys: []string{"name"}},
			{name: "logistics_products", id: "id", keys: []string{"company_id", "name"}, refs: map[string]refSpec{"company_id": r("logistics_companies", false, false)}},
			{name: "app_config", keys: []string{"key"}},
			{name: "cost_parameters", keys: []string{"key"}},
			{name: "company_profile", id: "id", keys: []string{"id"}},
			{name: "sender_settings", id: "id", keys: []string{"sender_label", "sender_phone"}},
			{name: "sales_order_settings", id: "id", keys: []string{"id"}, preserve: map[string]bool{"seal_asset_id": true}},
		}},
		{name: "customers", tables: []tableSpec{
			{name: "customers", id: "id", keys: []string{"name"}, refs: map[string]refSpec{
				"default_source_id":                 r("sources", true, false),
				"default_order_type_id":             r("order_types", true, false),
				"responsible_employee_id":           r("company_employees", true, false),
				"customer_product_rule_template_id": r("customer_product_rule_templates", true, true),
			}},
		}},
		{name: "products", tables: []tableSpec{
			{name: "product_unit_definitions", keys: []string{"code"}},
			{name: "pricing_gradient_templates", id: "id", keys: []string{"customer_id", "name"}, refs: map[string]refSpec{"customer_id": r("customers", true, false), "source_template_id": r("pricing_gradient_templates", true, true)}},
			{name: "pricing_gradient_template_tiers", id: "id", keys: []string{"template_id", "label", "min_weight_g", "max_weight_g", "margin_rate"}, refs: map[string]refSpec{"template_id": r("pricing_gradient_templates", false, false)}},
			{name: "operation_templates", id: "id", keys: []string{"customer_id", "name"}, refs: map[string]refSpec{"customer_id": r("customers", true, false), "source_template_id": r("operation_templates", true, true)}},
			{name: "operation_template_steps", id: "id", keys: []string{"template_id", "seq"}, refs: map[string]refSpec{"template_id": r("operation_templates", false, false)}},
			{name: "product_unit_templates", id: "id", keys: []string{"name", "inventory_unit", "quote_unit", "order_unit", "sales_specs_json"}},
			{name: "product_classification_templates", id: "id", keys: []string{"customer_id", "name"}, refs: map[string]refSpec{"customer_id": r("customers", true, false), "source_template_id": r("product_classification_templates", true, true)}},
			{name: "product_classification_template_categories", id: "id", keys: []string{"template_id", "name"}, refs: map[string]refSpec{"template_id": r("product_classification_templates", false, false), "parent_id": r("product_classification_template_categories", true, true)}},
			{name: "product_config_templates", id: "id", keys: []string{"customer_id", "name"}, refs: map[string]refSpec{
				"customer_id": r("customers", true, false), "source_template_id": r("product_config_templates", true, true),
				"gradient_template_id": r("pricing_gradient_templates", true, false), "operation_template_id": r("operation_templates", true, false),
				"unit_template_id": r("product_unit_templates", true, false),
			}},
			{name: "customer_product_rule_templates", id: "id", keys: []string{"customer_id", "name"}, refs: map[string]refSpec{"customer_id": r("customers", false, false)}},
			{name: "customer_product_rule_template_items", id: "id", keys: []string{"template_id", "sort_order"}, refs: map[string]refSpec{"template_id": r("customer_product_rule_templates", false, false)}},
			{name: "product_categories", id: "id", keys: []string{"customer_id", "parent_id", "name", "level"}, refs: map[string]refSpec{
				"customer_id": r("customers", true, false), "parent_id": r("product_categories", true, false),
				"source_category_id": r("product_categories", true, true), "gradient_template_id": r("pricing_gradient_templates", true, false),
				"operation_template_id": r("operation_templates", true, false), "product_config_template_id": r("product_config_templates", true, false),
				"unit_template_id": r("product_unit_templates", true, false),
			}},
			{name: "products", id: "id", customKey: productKey, refs: map[string]refSpec{
				"customer_id": r("customers", true, false), "product_category_id": r("product_categories", true, false),
				"base_product_id": r("products", true, true), "green_bean_bom_product_id": r("products", true, true),
				"parent_product_id": r("products", true, true), "default_sku_id": r("products", true, true),
				"gradient_template_id_override":  r("pricing_gradient_templates", true, false),
				"operation_template_id_override": r("operation_templates", true, false),
				"product_config_template_id":     r("product_config_templates", true, false),
				"classification_template_id":     r("product_classification_templates", true, false),
				"unit_template_id":               r("product_unit_templates", true, false), "derived_unit_template_id": r("product_unit_templates", true, false),
			}},
			{name: "product_classification_template_usages", keys: []string{"classification_template_id"}, refs: map[string]refSpec{"classification_template_id": r("product_classification_templates", false, false)}},
			{name: "product_classification_assignments", keys: []string{"product_id", "template_id", "category_id"}, refs: map[string]refSpec{
				"product_id": r("products", false, false), "template_id": r("product_classification_templates", false, false),
				"category_id": r("product_classification_template_categories", false, false),
			}},
			{name: "product_price_groups", id: "id", keys: []string{"name"}},
			{name: "product_pricing_rules", id: "id", keys: []string{"code", "name"}},
			{name: "customer_product_aliases", id: "id", keys: []string{"customer_id", "product_id", "display_name", "customer_item_code", "brand_name"}, refs: map[string]refSpec{
				"customer_id": r("customers", false, false), "product_id": r("products", false, false),
				"display_category_id": r("product_categories", true, false), "classification_template_id": r("product_classification_templates", true, false),
				"gradient_template_id": r("pricing_gradient_templates", true, false), "unit_template_id": r("product_unit_templates", true, false),
				"product_config_template_id": r("product_config_templates", true, false),
			}},
			{name: "product_price_records", id: "id", keys: []string{"product_id", "customer_product_alias_id", "price_group_id", "price_unit", "status"}, refs: map[string]refSpec{
				"product_id": r("products", true, false), "customer_product_alias_id": r("customer_product_aliases", true, false),
				"price_group_id": r("product_price_groups", true, false),
			}},
			{name: "product_price_tiers", id: "id", keys: []string{"product_id", "spec_g", "min_qty_units", "max_qty_units", "price_per_unit", "sales_unit", "price_basis", "product_kind"}, refs: map[string]refSpec{"product_id": r("products", false, false)}},
		}},
		{name: "materials", tables: []tableSpec{
			{name: "industry_field_templates", id: "id", keys: []string{"industry_key", "name"}},
			{name: "industry_field_definitions", id: "id", keys: []string{"template_id", "field_key"}, refs: map[string]refSpec{"template_id": r("industry_field_templates", false, false)}},
			{name: "material_classification_groups", id: "id", keys: []string{"name"}},
			{name: "material_classification_group_categories", id: "id", keys: []string{"group_id", "name"}, refs: map[string]refSpec{"group_id": r("material_classification_groups", false, false)}},
			{name: "materials", id: "id", keys: []string{"code"}, refs: map[string]refSpec{"industry_field_template_id": r("industry_field_templates", true, false)}, preserve: map[string]bool{
				"onhand_g": true, "onhand_units": true, "batch_no": true,
			}},
			{name: "material_bean_profiles", keys: []string{"material_id"}, refs: map[string]refSpec{"material_id": r("materials", false, false)}},
			{name: "material_pack_profiles", keys: []string{"material_id"}, refs: map[string]refSpec{"material_id": r("materials", false, false)}},
			{name: "material_classification_assignments", keys: []string{"material_id", "group_id"}, refs: map[string]refSpec{
				"material_id": r("materials", false, false), "group_id": r("material_classification_groups", false, false),
				"category_id": r("material_classification_group_categories", false, false),
			}},
			{name: "material_industry_field_values", keys: []string{"material_id", "field_key"}, refs: map[string]refSpec{"material_id": r("materials", false, false)}},
			{name: "warehouses", id: "code", keys: []string{"code"}},
		}},
		{name: "production", tables: []tableSpec{
			{name: "manufacturing_operations", id: "id", keys: []string{"code"}},
			{name: "manufacturing_workstations", id: "id", keys: []string{"code"}},
			{name: "manufacturing_workstation_capacities", id: "id", keys: []string{"workstation_id", "code"}, refs: map[string]refSpec{"workstation_id": r("manufacturing_workstations", false, false)}},
			{name: "manufacturing_workstation_operations", keys: []string{"workstation_id", "operation_id"}, refs: map[string]refSpec{
				"workstation_id": r("manufacturing_workstations", false, false), "operation_id": r("manufacturing_operations", false, false),
			}},
			{name: "manufacturing_workstation_capacity_operations", keys: []string{"capacity_id", "operation_id"}, refs: map[string]refSpec{
				"capacity_id": r("manufacturing_workstation_capacities", false, false), "operation_id": r("manufacturing_operations", false, false),
			}},
			{name: "process_routes", id: "id", keys: []string{"name"}},
			{name: "process_route_operations", id: "id", keys: []string{"route_id", "seq"}, refs: map[string]refSpec{
				"route_id": r("process_routes", false, false), "operation_id": r("manufacturing_operations", true, false),
				"workstation_id": r("manufacturing_workstations", true, false), "workstation_capacity_id": r("manufacturing_workstation_capacities", true, false),
				"standard_cost_capacity_id": r("manufacturing_workstation_capacities", true, false),
			}},
			{name: "production_bom_groups", id: "id", keys: []string{"name"}},
			{name: "production_bom_group_categories", id: "id", keys: []string{"group_id", "name"}, refs: map[string]refSpec{"group_id": r("production_bom_groups", false, false)}},
			{name: "production_boms", id: "id", keys: []string{"code"}, refs: map[string]refSpec{
				"group_id": r("production_bom_groups", true, false), "group_category_id": r("production_bom_group_categories", true, false),
				"source_product_id": r("products", true, false), "output_product_id": r("products", true, false),
				"source_bom_id": r("production_boms", true, true), "source_bom_version_id": r("production_bom_versions", true, true),
			}},
			{name: "production_bom_versions", id: "id", keys: []string{"bom_id", "version_no"}, refs: map[string]refSpec{
				"bom_id": r("production_boms", false, false), "process_route_id": r("process_routes", true, false),
			}},
			{name: "production_bom_version_items", id: "id", keys: []string{"version_id", "component_type", "material_id", "component_product_id"}, refs: map[string]refSpec{
				"version_id": r("production_bom_versions", false, false), "material_id": r("materials", true, false),
				"component_product_id": r("products", true, false),
			}},
			{name: "production_bom_version_operation_costs", id: "id", keys: []string{"version_id", "sort_order", "operation_id"}, refs: map[string]refSpec{
				"version_id": r("production_bom_versions", false, false), "operation_id": r("manufacturing_operations", true, false),
				"workstation_id": r("manufacturing_workstations", true, false), "workstation_capacity_id": r("manufacturing_workstation_capacities", true, false),
			}},
			{name: "process_templates", id: "id", keys: []string{"name", "product_id"}, refs: map[string]refSpec{
				"product_id": r("products", true, false), "bom_version_id": r("production_bom_versions", true, false),
				"industry_template_id": r("industry_field_templates", true, false),
			}},
			{name: "process_template_operations", id: "id", keys: []string{"template_id", "seq"}, refs: map[string]refSpec{
				"template_id": r("process_templates", false, false), "operation_id": r("manufacturing_operations", true, false),
				"workstation_id": r("manufacturing_workstations", true, false), "workstation_capacity_id": r("manufacturing_workstation_capacities", true, false),
			}},
			{name: "product_production_bom_bindings", keys: []string{"product_id"}, refs: map[string]refSpec{
				"product_id": r("products", false, false), "bom_id": r("production_boms", false, false),
				"bom_version_id": r("production_bom_versions", false, false),
			}},
			{name: "product_production_configs", keys: []string{"product_id"}, refs: map[string]refSpec{
				"product_id": r("products", false, false), "production_bom_id": r("production_boms", true, false),
				"production_bom_version_id": r("production_bom_versions", true, false), "process_route_id": r("process_routes", true, false),
				"industry_field_template_id": r("industry_field_templates", true, false),
			}},
			{name: "product_production_config_fields", id: "id", keys: []string{"product_id", "field_key"}, refs: map[string]refSpec{"product_id": r("products", false, false)}},
		}},
	}
}

func readRows(ctx context.Context, q dbQuerier, schema, table string) ([]map[string]any, error) {
	rows, err := q.Query(ctx, fmt.Sprintf("SELECT to_jsonb(t) FROM %s.%s t", qident(schema), qident(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item map[string]any
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.UseNumber()
		if err := dec.Decode(&item); err != nil {
			return nil, err
		}
		for k, v := range item {
			item[k] = dbValue(v)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func tableExists(ctx context.Context, q dbQuerier, schema, table string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=$1 AND table_name=$2)`, schema, table).Scan(&ok)
	return ok, err
}

func (m *migrator) mapRef(table string, value any, optional bool) (any, error) {
	if value == nil || norm(value) == "" || norm(value) == "0" {
		return value, nil
	}
	mapped, ok := m.idMaps[table][norm(value)]
	if ok {
		n, err := strconv.ParseInt(mapped, 10, 64)
		if err != nil {
			return mapped, nil
		}
		return n, nil
	}
	if optional {
		return nil, nil
	}
	return nil, fmt.Errorf("missing %s id mapping for %v", table, value)
}

func (m *migrator) sanitizeRow(ctx context.Context, tx pgx.Tx, table string, row map[string]any) error {
	meta, ok := m.columns[table]
	if !ok {
		meta = map[string]columnMeta{}
		rows, err := tx.Query(ctx, `
			SELECT column_name, data_type, is_nullable='YES'
			FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2`, m.schema, table)
		if err != nil {
			return err
		}
		for rows.Next() {
			var name string
			var c columnMeta
			if err := rows.Scan(&name, &c.dataType, &c.nullable); err != nil {
				rows.Close()
				return err
			}
			meta[name] = c
		}
		rows.Close()
		m.columns[table] = meta
	}
	for col, value := range row {
		c, exists := meta[col]
		if !exists {
			delete(row, col)
			continue
		}
		if value != nil || c.nullable {
			continue
		}
		switch {
		case strings.Contains(c.dataType, "character") || c.dataType == "text":
			row[col] = ""
		case c.dataType == "boolean":
			row[col] = false
		case strings.Contains(c.dataType, "int") || strings.Contains(c.dataType, "numeric") || strings.Contains(c.dataType, "double"):
			row[col] = int64(0)
		}
	}
	return nil
}

func specKey(s tableSpec, row map[string]any) string {
	if s.customKey != nil {
		return s.customKey(row)
	}
	return keyFrom(s.keys, row)
}

func sortedColumns(row map[string]any, exclude map[string]bool) []string {
	cols := make([]string, 0, len(row))
	for col := range row {
		if !exclude[col] {
			cols = append(cols, col)
		}
	}
	sort.Strings(cols)
	return cols
}

func (m *migrator) nextID(ctx context.Context, tx pgx.Tx, s tableSpec) (any, error) {
	if s.id == "" {
		return nil, nil
	}
	if s.id == "code" {
		return nil, errors.New("text key must be supplied by source")
	}
	n, ok := m.nextIDs[s.name]
	if !ok {
		var max int64
		err := tx.QueryRow(ctx, fmt.Sprintf("SELECT COALESCE(MAX(%s),0) FROM %s.%s", qident(s.id), qident(m.schema), qident(s.name))).Scan(&max)
		if err != nil {
			return nil, err
		}
		n = max
	}
	n++
	m.nextIDs[s.name] = n
	return n, nil
}

func updateRow(ctx context.Context, tx pgx.Tx, schema string, s tableSpec, id any, row map[string]any) error {
	exclude := map[string]bool{s.id: true, "created_at": true}
	for k := range s.preserve {
		exclude[k] = true
	}
	cols := sortedColumns(row, exclude)
	if len(cols) == 0 {
		return nil
	}
	args := make([]any, 0, len(cols)+1)
	sets := make([]string, 0, len(cols))
	for i, col := range cols {
		sets = append(sets, fmt.Sprintf("%s=$%d", qident(col), i+1))
		args = append(args, row[col])
	}
	if s.id != "" {
		args = append(args, id)
		_, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.%s SET %s WHERE %s=$%d", qident(schema), qident(s.name), strings.Join(sets, ","), qident(s.id), len(args)), args...)
		return err
	}
	where := make([]string, 0, len(s.keys))
	for _, col := range s.keys {
		args = append(args, row[col])
		where = append(where, fmt.Sprintf("%s IS NOT DISTINCT FROM $%d", qident(col), len(args)))
	}
	_, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.%s SET %s WHERE %s", qident(schema), qident(s.name), strings.Join(sets, ","), strings.Join(where, " AND ")), args...)
	return err
}

func insertRow(ctx context.Context, tx pgx.Tx, schema string, s tableSpec, row map[string]any, id any) (any, error) {
	if s.id != "" && id != nil {
		row[s.id] = id
	}
	cols := sortedColumns(row, nil)
	args := make([]any, len(cols))
	holders := make([]string, len(cols))
	for i, col := range cols {
		args[i] = row[col]
		holders[i] = fmt.Sprintf("$%d", i+1)
	}
	sql := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)", qident(schema), qident(s.name), quoteJoin(cols), strings.Join(holders, ","))
	if s.id != "" {
		sql += " RETURNING " + qident(s.id)
		var got any
		if err := tx.QueryRow(ctx, sql, args...).Scan(&got); err != nil {
			return nil, err
		}
		return got, nil
	}
	_, err := tx.Exec(ctx, sql, args...)
	return nil, err
}

func quoteJoin(cols []string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = qident(c)
	}
	return strings.Join(out, ",")
}

func (m *migrator) mergeTable(ctx context.Context, tx pgx.Tx, s tableSpec) error {
	srcExists, err := tableExists(ctx, m.source, m.schema, s.name)
	if err != nil || !srcExists {
		if err == nil {
			m.report[s.name] = counts{Skipped: 1}
			return nil
		}
		return err
	}
	dstExists, err := tableExists(ctx, tx, m.schema, s.name)
	if err != nil || !dstExists {
		return fmt.Errorf("target table %s unavailable: %w", s.name, err)
	}
	srcRows, err := readRows(ctx, m.source, m.schema, s.name)
	if err != nil {
		return err
	}
	if s.name == "product_categories" {
		sort.SliceStable(srcRows, func(i, j int) bool {
			li, _ := strconv.ParseInt(norm(srcRows[i]["level"]), 10, 64)
			lj, _ := strconv.ParseInt(norm(srcRows[j]["level"]), 10, 64)
			if li != lj {
				return li < lj
			}
			return norm(srcRows[i]["id"]) < norm(srcRows[j]["id"])
		})
	}
	dstRows, err := readRows(ctx, tx, m.schema, s.name)
	if err != nil {
		return err
	}
	dstByKey := map[string]map[string]any{}
	originalKeys := map[string]bool{}
	for _, row := range dstRows {
		k := specKey(s, row)
		if _, exists := dstByKey[k]; exists {
			return fmt.Errorf("%s target has duplicate business key", s.name)
		}
		dstByKey[k] = row
		originalKeys[k] = true
	}
	if m.idMaps[s.name] == nil {
		m.idMaps[s.name] = map[string]string{}
	}
	stat := counts{Preserved: len(dstRows)}
	seenSource := map[string]any{}
	type deferred struct {
		sourceID, targetID string
		refs               map[string]any
	}
	var deferredRows []deferred
	for _, original := range srcRows {
		row := make(map[string]any, len(original))
		for k, v := range original {
			row[k] = v
		}
		if s.name == "sender_settings" && row["is_default"] == true {
			if _, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.%s SET is_default=false WHERE is_default=true", qident(m.schema), qident(s.name))); err != nil {
				return err
			}
		}
		deferredValues := map[string]any{}
		for col, ref := range s.refs {
			if ref.deferred {
				deferredValues[col] = row[col]
				if row[col] != nil && norm(row[col]) != "0" {
					row[col] = int64(0)
				}
				continue
			}
			mapped, err := m.mapRef(ref.table, row[col], ref.optional)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", s.name, col, err)
			}
			row[col] = mapped
		}
		if err := m.sanitizeRow(ctx, tx, s.name, row); err != nil {
			return err
		}
		k := specKey(s, row)
		sourceID := norm(original[s.id])
		if priorTargetID, duplicate := seenSource[k]; duplicate {
			if s.id != "" {
				m.idMaps[s.name][sourceID] = norm(priorTargetID)
			}
			stat.Conflicts++
			stat.Skipped++
			continue
		}
		targetRow, matched := dstByKey[k]
		var targetID any
		if matched {
			targetID = targetRow[s.id]
			if err := updateRow(ctx, tx, m.schema, s, targetID, row); err != nil {
				return fmt.Errorf("update %s: %w", s.name, err)
			}
			stat.Updated++
			if originalKeys[k] {
				stat.Preserved--
			}
		} else {
			if s.id == "code" {
				targetID = row[s.id]
			} else {
				targetID, err = m.nextID(ctx, tx, s)
				if err != nil {
					return err
				}
			}
			targetID, err = insertRow(ctx, tx, m.schema, s, row, targetID)
			if err != nil {
				return fmt.Errorf("insert %s: %w", s.name, err)
			}
			stat.Added++
			dstByKey[k] = row
		}
		seenSource[k] = targetID
		if s.id != "" {
			m.idMaps[s.name][sourceID] = norm(targetID)
			if len(deferredValues) > 0 {
				deferredRows = append(deferredRows, deferred{sourceID: sourceID, targetID: norm(targetID), refs: deferredValues})
			}
		}
	}
	for _, d := range deferredRows {
		for col, old := range d.refs {
			if old == nil || norm(old) == "0" {
				continue
			}
			ref := s.refs[col]
			mapped, err := m.mapRef(ref.table, old, ref.optional)
			if err != nil {
				return err
			}
			if mapped == nil {
				continue
			}
			targetID, parseErr := strconv.ParseInt(d.targetID, 10, 64)
			if parseErr != nil {
				return parseErr
			}
			_, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s.%s SET %s=$1 WHERE %s=$2", qident(m.schema), qident(s.name), qident(col), qident(s.id)), mapped, targetID)
			if err != nil {
				return err
			}
		}
	}
	m.report[s.name] = stat
	return nil
}

func (m *migrator) buildEmployeeMap(ctx context.Context, tx pgx.Tx) error {
	m.idMaps["company_employees"] = map[string]string{}
	src, err := readRows(ctx, m.source, m.schema, "company_employees")
	if err != nil {
		return err
	}
	dst, err := readRows(ctx, tx, m.schema, "company_employees")
	if err != nil {
		return err
	}
	byPhone := map[string]string{}
	for _, r := range dst {
		if p := norm(r["phone"]); p != "" {
			byPhone[p] = norm(r["id"])
		}
	}
	for _, r := range src {
		if id, ok := byPhone[norm(r["phone"])]; ok {
			m.idMaps["company_employees"][norm(r["id"])] = id
		}
	}
	return nil
}

func (m *migrator) dedupeCustomers(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, name
		FROM %s.customers
		WHERE name IN (SELECT name FROM %s.customers GROUP BY name HAVING count(*) > 1)
		ORDER BY name, active DESC, id DESC`, qident(m.schema), qident(m.schema)))
	if err != nil {
		return err
	}
	type customer struct {
		id   int64
		name string
	}
	var duplicates []customer
	for rows.Next() {
		var c customer
		if err := rows.Scan(&c.id, &c.name); err != nil {
			rows.Close()
			return err
		}
		duplicates = append(duplicates, c)
	}
	rows.Close()
	columns, err := tx.Query(ctx, `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema=$1
		  AND ((column_name='customer_id' AND table_name<>'customers')
		       OR (table_name='mini_sessions' AND column_name='current_customer_id'))
		ORDER BY table_name, column_name`, m.schema)
	if err != nil {
		return err
	}
	type refColumn struct{ table, column string }
	var refs []refColumn
	for columns.Next() {
		var r refColumn
		if err := columns.Scan(&r.table, &r.column); err != nil {
			columns.Close()
			return err
		}
		refs = append(refs, r)
	}
	columns.Close()
	canonical := map[string]int64{}
	for _, c := range duplicates {
		if _, ok := canonical[c.name]; !ok {
			canonical[c.name] = c.id
			continue
		}
		keep := canonical[c.name]
		for _, ref := range refs {
			if ref.table == "customer_portal_profiles" && ref.column == "customer_id" {
				var keepExists bool
				if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s.%s WHERE %s=$1)",
					qident(m.schema), qident(ref.table), qident(ref.column)), keep).Scan(&keepExists); err != nil {
					return err
				}
				if keepExists {
					if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.%s WHERE %s=$1",
						qident(m.schema), qident(ref.table), qident(ref.column)), c.id); err != nil {
						return err
					}
					continue
				}
			}
			_, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.%s SET %s=$1 WHERE %s=$2",
				qident(m.schema), qident(ref.table), qident(ref.column), qident(ref.column)), keep, c.id)
			if err != nil {
				return fmt.Errorf("dedupe customer reference %s.%s: %w", ref.table, ref.column, err)
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.customers WHERE id=$1", qident(m.schema)), c.id); err != nil {
			return err
		}
	}
	return nil
}

func (m *migrator) dedupeProducts(ctx context.Context, tx pgx.Tx) error {
	items, err := readRows(ctx, tx, m.schema, "products")
	if err != nil {
		return err
	}
	sort.SliceStable(items, func(i, j int) bool {
		ai, _ := items[i]["active"].(bool)
		aj, _ := items[j]["active"].(bool)
		if ai != aj {
			return ai
		}
		return norm(items[i]["id"]) > norm(items[j]["id"])
	})
	columns, err := tx.Query(ctx, `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema=$1
		  AND (column_name='product_id' OR column_name IN
		       ('base_product_id','parent_product_id','default_sku_id','green_bean_bom_product_id',
		        'component_product_id','output_product_id','source_product_id'))
		ORDER BY table_name, column_name`, m.schema)
	if err != nil {
		return err
	}
	type refColumn struct{ table, column string }
	var refs []refColumn
	for columns.Next() {
		var r refColumn
		if err := columns.Scan(&r.table, &r.column); err != nil {
			columns.Close()
			return err
		}
		if r.table != "products" || r.column != "product_id" {
			refs = append(refs, r)
		}
	}
	columns.Close()
	kept := map[string]int64{}
	for _, item := range items {
		key := productKey(item)
		id, err := strconv.ParseInt(norm(item["id"]), 10, 64)
		if err != nil {
			return err
		}
		keep, exists := kept[key]
		if !exists {
			kept[key] = id
			continue
		}
		for _, ref := range refs {
			var uniqueColumn bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM information_schema.table_constraints tc
					JOIN information_schema.key_column_usage kcu
					  ON kcu.constraint_schema=tc.constraint_schema
					 AND kcu.constraint_name=tc.constraint_name
					WHERE tc.constraint_schema=$1 AND tc.table_name=$2
					  AND tc.constraint_type IN ('PRIMARY KEY','UNIQUE')
					GROUP BY tc.constraint_name
					HAVING count(*)=1 AND min(kcu.column_name)=$3
				)`, m.schema, ref.table, ref.column).Scan(&uniqueColumn); err != nil {
				return err
			}
			if uniqueColumn {
				var keepExists bool
				if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s.%s WHERE %s=$1)",
					qident(m.schema), qident(ref.table), qident(ref.column)), keep).Scan(&keepExists); err != nil {
					return err
				}
				if keepExists {
					if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.%s WHERE %s=$1",
						qident(m.schema), qident(ref.table), qident(ref.column)), id); err != nil {
						return err
					}
					continue
				}
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.%s SET %s=$1 WHERE %s=$2",
				qident(m.schema), qident(ref.table), qident(ref.column), qident(ref.column)), keep, id); err != nil {
				return fmt.Errorf("dedupe product reference %s.%s: %w", ref.table, ref.column, err)
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.products WHERE id=$1", qident(m.schema)), id); err != nil {
			return err
		}
	}
	return nil
}

func (m *migrator) validate(ctx context.Context, tx pgx.Tx) error {
	checks := []string{
		`SELECT count(*) FROM products p LEFT JOIN customers c ON c.id=p.customer_id WHERE p.customer_id<>0 AND c.id IS NULL`,
		`SELECT count(*) FROM product_price_tiers t LEFT JOIN products p ON p.id=t.product_id WHERE p.id IS NULL`,
		`SELECT count(*) FROM production_bom_versions v LEFT JOIN production_boms b ON b.id=v.bom_id WHERE b.id IS NULL`,
		`SELECT count(*) FROM production_bom_version_items i LEFT JOIN production_bom_versions v ON v.id=i.version_id WHERE v.id IS NULL`,
		`SELECT count(*) FROM product_production_configs c LEFT JOIN products p ON p.id=c.product_id WHERE p.id IS NULL`,
	}
	for _, body := range checks {
		var n int
		if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+qident(m.schema)+",public"); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, body).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			return fmt.Errorf("validation failed: %s returned %d", body, n)
		}
	}
	return nil
}

func (m *migrator) processBatch(ctx context.Context, tx pgx.Tx, batch batchSpec) error {
	if err := m.buildEmployeeMap(ctx, tx); err != nil {
		return err
	}
	if batch.name == "customers" {
		if err := m.dedupeCustomers(ctx, tx); err != nil {
			return err
		}
	}
	if batch.name == "products" {
		if err := m.dedupeProducts(ctx, tx); err != nil {
			return err
		}
	}
	for _, table := range batch.tables {
		if err := m.mergeTable(ctx, tx, table); err != nil {
			return err
		}
	}
	if err := m.validate(ctx, tx); err != nil {
		return err
	}
	if batch.name == "customers" {
		var duplicateGroups int
		if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM (SELECT name FROM %s.customers GROUP BY name HAVING count(*)>1) q", qident(m.schema))).Scan(&duplicateGroups); err != nil {
			return err
		}
		if duplicateGroups != 0 {
			return fmt.Errorf("customer duplicate groups remain: %d", duplicateGroups)
		}
	}
	return nil
}

func run() error {
	var apply bool
	var schema string
	var reportPath string
	flag.BoolVar(&apply, "apply", false, "commit changes; default is rollback-only dry run")
	flag.StringVar(&schema, "schema", defaultSchema, "business schema")
	flag.StringVar(&reportPath, "report", "", "write a count-only JSON report")
	flag.Parse()
	srcDSN, dstDSN := os.Getenv("SOURCE_DATABASE_URL"), os.Getenv("TARGET_DATABASE_URL")
	if srcDSN == "" || dstDSN == "" {
		return errors.New("SOURCE_DATABASE_URL and TARGET_DATABASE_URL are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	src, err := pgx.Connect(ctx, srcDSN)
	if err != nil {
		return err
	}
	defer src.Close(ctx)
	dst, err := pgx.Connect(ctx, dstDSN)
	if err != nil {
		return err
	}
	defer dst.Close(ctx)
	m := &migrator{source: src, target: dst, schema: schema, dryRun: !apply, idMaps: map[string]map[string]string{}, nextIDs: map[string]int64{}, report: map[string]counts{}, columns: map[string]map[string]columnMeta{}}
	if _, err := dst.Exec(ctx, "SELECT pg_advisory_lock(726831904)"); err != nil {
		return err
	}
	defer dst.Exec(context.Background(), "SELECT pg_advisory_unlock(726831904)")
	if !apply {
		tx, err := dst.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			return err
		}
		for _, batch := range specs() {
			if err := m.processBatch(ctx, tx, batch); err != nil {
				tx.Rollback(ctx)
				return fmt.Errorf("batch %s: %w", batch.name, err)
			}
		}
		if err := tx.Rollback(ctx); err != nil {
			return err
		}
	} else {
		for _, batch := range specs() {
			tx, err := dst.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
			if err != nil {
				return err
			}
			if err := m.processBatch(ctx, tx, batch); err != nil {
				tx.Rollback(ctx)
				return fmt.Errorf("batch %s: %w", batch.name, err)
			}
			if err := tx.Commit(ctx); err != nil {
				return err
			}
		}
	}
	payload := map[string]any{"mode": map[bool]string{true: "apply", false: "dry-run"}[apply], "schema": schema, "tables": m.report}
	data, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(data))
	if reportPath != "" {
		if err := os.WriteFile(reportPath, append(data, '\n'), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migration failed:", err)
		os.Exit(1)
	}
}
