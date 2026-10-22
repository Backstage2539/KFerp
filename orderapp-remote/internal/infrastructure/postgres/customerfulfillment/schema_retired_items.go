package customerfulfillment

import "fmt"

// Retired fulfillment lines retain their immutable FK history. Uniqueness is
// required only among active lines; a later published variant must get new ids.
// Reject unknown index definitions rather than silently replacing local policy.
func fulfillmentActiveItemIdentityIndexSQL(schema, name, columns, identityPredicate string) string {
	return fmt.Sprintf(`DO $active_identity$
 DECLARE predicate text;
 BEGIN
 SELECT COALESCE(regexp_replace(pg_get_expr(i.indpred,i.indrelid),'[()[:space:]]','','g'),'[unconditional]') INTO predicate FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='%[1]s' AND c.relname='%[2]s';
 IF predicate='%[4]s' THEN DROP INDEX %[1]s.%[2]s;
 ELSIF predicate IS NOT NULL AND predicate<>'%[4]sANDqty>0' THEN RAISE EXCEPTION 'unexpected fulfillment item identity index predicate: %%',predicate;
 END IF;
 CREATE UNIQUE INDEX IF NOT EXISTS %[2]s ON %[1]s.customer_direct_ship_request_items(%[3]s) WHERE %[4]s AND qty>0;
 END $active_identity$`, schema, name, columns, identityPredicate)
}
