package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	salesapp "orderapp/internal/application/sales"
)

// The counters commit (and roll back) with the source changes. A customer-specific
// change bumps both the old and new owner when ownership is reassigned.
func EnsureEntryRevisions(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.employee_entry_revisions (
 section text NOT NULL, owner text NOT NULL, revision bigint NOT NULL, PRIMARY KEY(section,owner));
 CREATE OR REPLACE FUNCTION %[1]s.touch_employee_entry_revision() RETURNS trigger LANGUAGE plpgsql AS $$
 DECLARE old_owner text; new_owner text; scope text;
 BEGIN
 IF TG_TABLE_NAME='bean_list_publications' THEN
   IF TG_OP<>'INSERT' AND OLD.status='published' AND OLD.publication_purpose='factory_supply' THEN
     old_owner:=CASE WHEN OLD.owner_type='official' THEN '0' ELSE OLD.owner_key END;
   END IF;
   IF TG_OP<>'DELETE' AND NEW.status='published' AND NEW.publication_purpose='factory_supply' THEN
     new_owner:=CASE WHEN NEW.owner_type='official' THEN '0' ELSE NEW.owner_key END;
   END IF;
 ELSIF TG_TABLE_NAME='production_bom_versions' THEN
   IF TG_OP<>'INSERT' AND OLD.status='published' THEN old_owner:='0'; END IF;
   IF TG_OP<>'DELETE' AND NEW.status='published' THEN new_owner:='0'; END IF;
 ELSIF TG_TABLE_NAME='production_bom_version_variants' THEN
   IF EXISTS(SELECT 1 FROM %[1]s.production_bom_versions WHERE status='published' AND id IN (OLD.version_id,NEW.version_id)) THEN new_owner:='0'; END IF;
 ELSIF TG_TABLE_NAME='production_bom_output_bindings' THEN
   IF TG_OP<>'INSERT' AND OLD.is_default AND OLD.output_type='product' THEN old_owner:='0'; END IF;
   IF TG_OP<>'DELETE' AND NEW.is_default AND NEW.output_type='product' THEN new_owner:='0'; END IF;
 ELSIF TG_NARGS=1 THEN
   new_owner:='0';
 ELSE
   IF TG_OP<>'INSERT' THEN old_owner:=COALESCE(NULLIF(to_jsonb(OLD)->>TG_ARGV[1],''),'0'); END IF;
   IF TG_OP<>'DELETE' THEN new_owner:=COALESCE(NULLIF(to_jsonb(NEW)->>TG_ARGV[1],''),'0'); END IF;
 END IF;
 FOR scope IN SELECT DISTINCT v FROM unnest(ARRAY[old_owner,new_owner]) v WHERE v IS NOT NULL ORDER BY v LOOP
 INSERT INTO %[1]s.employee_entry_revisions AS r VALUES(TG_ARGV[0],scope,1)
 ON CONFLICT(section,owner) DO UPDATE SET revision=r.revision+1;
 END LOOP;
 RETURN NULL;END $$;`, schema))
	if err != nil {
		return err
	}
	for _, entry := range []struct{ table, section, owner string }{
		{"customers", "base", ""}, {"company_employees", "base", ""}, {"sources", "base", ""}, {"order_types", "base", ""}, {"pay_statuses", "base", ""}, {"ship_statuses", "base", ""},
		{"products", "catalog", ""}, {"product_unit_templates", "catalog", ""}, {"product_categories", "catalog", ""}, {"product_config_templates", "catalog", ""},
		{"business_groups", "catalog", ""}, {"business_group_usages", "catalog", ""},
		{"production_bom_output_bindings", "catalog", ""}, {"production_bom_versions", "catalog", ""}, {"production_bom_version_variants", "catalog", ""}, {"production_bom_specs", "catalog", ""},
		{"customer_product_aliases", "catalog", "customer_id"}, {"product_customer_references", "catalog", "customer_id"}, {"customer_sku_public_usage", "catalog", "customer_id"},
		{"bean_list_publications", "catalog", "owner_key"}, {"customer_order_price_table_bindings", "catalog", "customer_id"},
		{"warehouses", "warehouses", "customer_id"}, {"shipping_fee_templates", "freight-templates", ""},
	} {
		args := "'" + entry.section + "'"
		if entry.owner != "" {
			args += ",'" + entry.owner + "'"
		}
		sql := fmt.Sprintf("CREATE OR REPLACE TRIGGER employee_entry_revision AFTER INSERT OR UPDATE OR DELETE ON %s.%s FOR EACH ROW EXECUTE FUNCTION %s.touch_employee_entry_revision(%s)", schema, entry.table, schema, args)
		if _, err = tx.Exec(ctx, sql); err != nil {
			return fmt.Errorf("employee entry revision %s: %w", entry.table, err)
		}
	}
	return tx.Commit(ctx)
}

func (r Repository) EmployeeEntryRevision(ctx context.Context, part string, customerID int64) (string, error) {
	var revision string
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(string_agg(owner||':'||revision,',' ORDER BY owner),'0')
 FROM %[1]s.employee_entry_revisions WHERE section=$1 AND owner IN ('0',$2::bigint::text)
 HAVING $2::bigint=0 OR EXISTS(SELECT 1 FROM %[1]s.customers WHERE id=$2 AND active)`, r.schema), part, customerID).Scan(&revision)
	return revision, err
}

func (r Repository) EmployeeEntryData(ctx context.Context, part string, customerID int64) (salesapp.OrderFormData, error) {
	if part == "base" {
		return r.employeeEntryBase(ctx)
	}
	if part != "catalog" || customerID <= 0 {
		return salesapp.OrderFormData{}, fmt.Errorf("invalid catalog customer")
	}
	r.entryCustomerID = customerID
	var data salesapp.OrderFormData
	var err error
	if data.BeanListVersionOptions, err = r.fetchOrderBeanListVersionOptions(ctx); err != nil {
		return data, err
	}
	if data.CustomerPublicUsages, err = r.fetchOrderCustomerPublicUsages(ctx); err != nil {
		return data, err
	}
	if data.Products, err = r.fetchOrderProducts(ctx); err != nil {
		return data, err
	}
	data.ProductBOMSpecOptions, err = r.fetchOrderBOMSpecOptions(ctx)
	return data, err
}
