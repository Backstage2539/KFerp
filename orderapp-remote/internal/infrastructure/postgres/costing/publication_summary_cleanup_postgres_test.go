package costing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	appcosting "orderapp/internal/application/costing"
)

func publicationCleanupPostgres(t *testing.T) (Repository, context.Context) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("pr663_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	ddl := fmt.Sprintf(`
		CREATE TABLE %[1]s.bean_list_publications (
			id BIGSERIAL PRIMARY KEY, publication_purpose TEXT NOT NULL DEFAULT 'factory_supply', list_type TEXT NOT NULL,
			product_type_category_id BIGINT NOT NULL DEFAULT 0, product_type_name TEXT NOT NULL DEFAULT '',
			classification_template_id BIGINT NOT NULL DEFAULT 0, classification_template_name TEXT NOT NULL DEFAULT '',
			classification_category_id BIGINT NOT NULL DEFAULT 0, classification_category_name TEXT NOT NULL DEFAULT '',
			version_no TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'published', owner_type TEXT NOT NULL DEFAULT 'official', owner_key TEXT NOT NULL DEFAULT '',
			price_source_publication_id BIGINT NULL, style_source_publication_id BIGINT NULL, source_version_no TEXT NOT NULL DEFAULT '',
			publication_release_id TEXT NOT NULL DEFAULT '', publication_table_key TEXT NOT NULL DEFAULT '', publication_table_name TEXT NOT NULL DEFAULT '',
			publication_is_default_table BOOLEAN NOT NULL DEFAULT true, publication_has_content BOOLEAN NOT NULL DEFAULT false,
			publication_summary_ready BOOLEAN NOT NULL DEFAULT false,
			config_json JSONB NOT NULL DEFAULT '{}'::jsonb, content_json JSONB NOT NULL DEFAULT '{}'::jsonb, changelog TEXT NOT NULL DEFAULT '', actor TEXT NOT NULL DEFAULT '',
			published_at TIMESTAMPTZ NOT NULL DEFAULT now(), withdrawn_at TIMESTAMPTZ NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			deleted_at TIMESTAMPTZ NULL, deleted_by TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.bean_list_publication_assets(id BIGSERIAL PRIMARY KEY, publication_id BIGINT NOT NULL REFERENCES %[1]s.bean_list_publications(id) ON DELETE CASCADE, asset_type TEXT, content_type TEXT, cache_key TEXT, payload BYTEA, created_by TEXT, created_at TIMESTAMPTZ DEFAULT now(), updated_at TIMESTAMPTZ DEFAULT now(), UNIQUE(publication_id,asset_type));
		CREATE TABLE %[1]s.customer_bean_list_acknowledgements(id BIGSERIAL PRIMARY KEY, customer_id BIGINT NOT NULL, publication_id BIGINT NOT NULL REFERENCES %[1]s.bean_list_publications(id) ON DELETE CASCADE, acknowledged_by TEXT, acknowledged_at TIMESTAMPTZ DEFAULT now());
		CREATE TABLE %[1]s.bean_list_publication_delete_previews(token_hash TEXT PRIMARY KEY, actor TEXT NOT NULL, publication_ids BIGINT[] NOT NULL, row_versions JSONB NOT NULL, expires_at TIMESTAMPTZ NOT NULL, completed_at TIMESTAMPTZ NULL, result_json JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY, actor TEXT, entity_type TEXT, entity_id BIGINT, action TEXT, field TEXT, old_value TEXT, new_value TEXT, meta JSONB, ts TIMESTAMPTZ DEFAULT now());
	`, schema)
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	return NewRepository(pool, schema), ctx
}

func TestPublicationSummaryPaginatesWholeReleaseWithoutSnapshotPayload(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	for release := 1; release <= 12; release++ {
		for table := 1; table <= 2; table++ {
			_, err := r.pool.Exec(ctx, `INSERT INTO `+r.schema+`.bean_list_publications
				(list_type,classification_template_id,classification_template_name,version_no,status,owner_type,publication_release_id,publication_table_key,publication_table_name,publication_is_default_table,publication_has_content,config_json,content_json,created_at)
					VALUES('commercial',8000000000000056,'工厂量单',$1,'archived','official',$2,$3,$4,$5,true,'{"large":"metadata"}','{"price_rows":[{"secret":999}]}',now()-($6::int||' minutes')::interval)`,
				fmt.Sprintf("V3.0.%d", release), fmt.Sprintf("release-%02d", release), fmt.Sprintf("table-%d", table), fmt.Sprintf("价格表%d", table), table == 1, release)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	page, err := r.ListBeanListPublicationSummaries(ctx, appcosting.BeanListPublicationSummaryQuery{
		BeanListPublicationQuery: appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 8000000000000056, OwnerType: "official"},
		Status:                   "archived", Page: 2, PageSize: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 12 || len(page.Rows) != 10 {
		t.Fatalf("total=%d rows=%d", page.Total, len(page.Rows))
	}
	for _, row := range page.Rows {
		if row.ReleaseID == "" || row.TableName == "" || !row.HasContent {
			t.Fatalf("bad summary: %+v", row)
		}
	}
}

func TestPublicationSummaryMatchesFullRowsSearchesWholeReleaseAndIsolatesCustomers(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	for index := 1; index <= 2; index++ {
		if _, err := r.pool.Exec(ctx, `INSERT INTO `+r.schema+`.bean_list_publications
			(list_type,publication_purpose,classification_template_id,classification_template_name,version_no,status,owner_type,owner_key,publication_release_id,publication_table_key,publication_table_name,publication_is_default_table,publication_has_content,publication_summary_ready,config_json,content_json)
			VALUES('commercial','factory_supply',56,'工厂量单','V3.2.1','published','customer','450','release-customer','table-'||$1::int,$2::text,$3::boolean,true,true,'{"large":"configuration"}',jsonb_build_object('price_rows',jsonb_build_array(jsonb_build_object('sku_id',$1::int,'price',99))))`, index, []string{"日常表", "曲奇专用表"}[index-1], index == 1); err != nil {
			t.Fatal(err)
		}
	}
	insertPublicationForCleanupTest(t, r, ctx, "published", "customer", "451", 56, "V9.9.9")
	query := appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 56, OwnerType: "customer", OwnerKey: "450"}
	page, err := r.ListBeanListPublicationSummaries(ctx, appcosting.BeanListPublicationSummaryQuery{BeanListPublicationQuery: query, Status: "active", Search: "曲奇", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	full, err := r.ListBeanListPublications(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Rows) != 2 || len(full) != 2 {
		t.Fatalf("summary total=%d rows=%d full=%d", page.Total, len(page.Rows), len(full))
	}
	fullByID := map[int64]appcosting.BeanListPublication{}
	for _, row := range full {
		fullByID[row.ID] = row
	}
	for _, summary := range page.Rows {
		row, ok := fullByID[summary.ID]
		if !ok || row.Version != summary.Version || row.OwnerKey != summary.OwnerKey || row.Status != summary.Status {
			t.Fatalf("summary mismatch: %+v", summary)
		}
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "configuration") || strings.Contains(string(raw), "price_rows") || strings.Contains(string(raw), `"config"`) || strings.Contains(string(raw), `"content"`) {
		t.Fatalf("summary leaked snapshot payload: %s", raw)
	}
}

func TestPublicationSummaryAndPriceSourcesPreferExactClassificationBeforeNewerLegacyRows(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	var officialExact, customerExact int64
	for _, row := range []struct {
		classificationID int64
		ownerType        string
		ownerKey         string
		status           string
		version          string
		age              string
		marker           string
		target           *int64
	}{
		{56, "official", "", "published", "V3.0.1", "2 days", "official-exact", &officialExact},
		{0, "official", "", "published", "V3.0.2", "1 day", "official-legacy", nil},
		{56, "customer", "450", "draft", "V3.0.3", "2 hours", "customer-exact", &customerExact},
		{0, "customer", "450", "draft", "V3.0.4", "1 hour", "customer-legacy", nil},
	} {
		var id int64
		err := r.pool.QueryRow(ctx, `INSERT INTO `+r.schema+`.bean_list_publications
			(list_type,publication_purpose,classification_template_id,version_no,status,owner_type,owner_key,publication_release_id,publication_table_key,publication_table_name,publication_has_content,publication_summary_ready,config_json,content_json,published_at,created_at)
			VALUES('commercial','factory_supply',$1,$2,$3,$4,$5,$2,$2,$2,true,true,'{}',jsonb_build_object('price_rows',jsonb_build_array(jsonb_build_object('marker',$7::text))),now()-$6::interval,now()-$6::interval) RETURNING id`,
			row.classificationID, row.version, row.status, row.ownerType, row.ownerKey, row.age, row.marker).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		if row.target != nil {
			*row.target = id
		}
	}
	query := appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 56, OwnerType: "official"}
	page, err := r.ListBeanListPublicationSummaries(ctx, appcosting.BeanListPublicationSummaryQuery{BeanListPublicationQuery: query, Status: "active", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Current == nil || page.Current.ID != officialExact {
		t.Fatalf("current=%+v want exact id=%d", page.Current, officialExact)
	}
	query.OwnerType, query.OwnerKey = "customer", "450"
	sources, err := r.ListBeanListPriceSources(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0].ID != customerExact || sources[1].ID != officialExact {
		t.Fatalf("sources=%+v want customer=%d official=%d", sources, customerExact, officialExact)
	}
}

func TestPublicationDeleteRemovesOnlySelectedArchivedTableAndIsIdempotent(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	var first, second int64
	for index, target := range []*int64{&first, &second} {
		err := r.pool.QueryRow(ctx, `INSERT INTO `+r.schema+`.bean_list_publications
			(list_type,classification_template_id,version_no,status,owner_type,publication_release_id,publication_table_key,publication_table_name,publication_is_default_table,publication_has_content,config_json,content_json)
			VALUES('commercial',8000000000000056,'V3.0.1','archived','official','same-release',$1,$2,$3,true,'{"publication_batch":{"release_id":"same-release"}}','{"price_rows":[1]}') RETURNING id`,
			fmt.Sprintf("table-%d", index+1), fmt.Sprintf("表%d", index+1), index == 0).Scan(target)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.pool.Exec(ctx, `INSERT INTO `+r.schema+`.bean_list_publication_assets(publication_id,asset_type,content_type,cache_key,payload,created_by) VALUES($1,'pdf','application/pdf','x','pdf','tester')`, first); err != nil {
		t.Fatal(err)
	}
	if _, err := r.pool.Exec(ctx, `INSERT INTO `+r.schema+`.customer_bean_list_acknowledgements(customer_id,publication_id,acknowledged_by) VALUES(450,$1,'customer')`, first); err != nil {
		t.Fatal(err)
	}
	referencing := insertPublicationForCleanupTest(t, r, ctx, "published", "customer", "450", 8000000000000056, "V3.0.2")
	if _, err := r.pool.Exec(ctx, `UPDATE `+r.schema+`.bean_list_publications SET price_source_publication_id=$1,content_json='{"price_rows":[{"price":88}]}' WHERE id=$2`, first, referencing); err != nil {
		t.Fatal(err)
	}
	preview, err := r.PreviewDeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsPreviewCommand{
		IDs: []int64{first}, Query: appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 8000000000000056, OwnerType: "official"}, Actor: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.DeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsCommand{ConfirmationToken: preview.ConfirmationToken, Actor: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedCount != 1 || len(result.IDs) != 1 || result.IDs[0] != first {
		t.Fatalf("result=%+v", result)
	}
	if again, err := r.DeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsCommand{ConfirmationToken: preview.ConfirmationToken, Actor: "admin"}); err != nil || again.DeletedCount != 1 {
		t.Fatalf("retry=%+v err=%v", again, err)
	}
	var firstStatus, secondStatus string
	var firstConfig, firstContent []byte
	if err := r.pool.QueryRow(ctx, `SELECT status,config_json,content_json FROM `+r.schema+`.bean_list_publications WHERE id=$1`, first).Scan(&firstStatus, &firstConfig, &firstContent); err != nil {
		t.Fatal(err)
	}
	if err := r.pool.QueryRow(ctx, `SELECT status FROM `+r.schema+`.bean_list_publications WHERE id=$1`, second).Scan(&secondStatus); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "deleted" || string(firstConfig) != "{}" || string(firstContent) != "{}" || secondStatus != "archived" {
		t.Fatalf("statuses=%s/%s payload=%s/%s", firstStatus, secondStatus, firstConfig, firstContent)
	}
	var assets, audits, acknowledgements int
	_ = r.pool.QueryRow(ctx, `SELECT count(*) FROM `+r.schema+`.bean_list_publication_assets WHERE publication_id=$1`, first).Scan(&assets)
	_ = r.pool.QueryRow(ctx, `SELECT count(*) FROM `+r.schema+`.audit_logs WHERE entity_id=$1 AND action='delete_archived'`, first).Scan(&audits)
	_ = r.pool.QueryRow(ctx, `SELECT count(*) FROM `+r.schema+`.customer_bean_list_acknowledgements WHERE publication_id=$1`, first).Scan(&acknowledgements)
	var referenceSource int64
	var referenceContent []byte
	if err := r.pool.QueryRow(ctx, `SELECT price_source_publication_id,content_json FROM `+r.schema+`.bean_list_publications WHERE id=$1`, referencing).Scan(&referenceSource, &referenceContent); err != nil {
		t.Fatal(err)
	}
	if assets != 0 || audits != 1 || acknowledgements != 1 || referenceSource != first || !strings.Contains(string(referenceContent), `"price": 88`) {
		t.Fatalf("assets=%d audits=%d acknowledgements=%d reference=%d content=%s", assets, audits, acknowledgements, referenceSource, referenceContent)
	}
}

func insertPublicationForCleanupTest(t *testing.T, r Repository, ctx context.Context, status, ownerType, ownerKey string, classificationID int64, version string) int64 {
	t.Helper()
	var id int64
	err := r.pool.QueryRow(ctx, `INSERT INTO `+r.schema+`.bean_list_publications
		(list_type,publication_purpose,classification_template_id,version_no,status,owner_type,owner_key,publication_release_id,publication_table_key,publication_table_name,publication_has_content,config_json,content_json)
		VALUES('commercial','factory_supply',$1,$2,$3,$4,$5,$2,$2,$2,true,'{"publication_batch":{"release_id":"test"}}','{"price_rows":[1]}') RETURNING id`,
		classificationID, version, status, ownerType, ownerKey).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPublicationDeleteClearAllIsBoundToCurrentOwnerAndClassification(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	first := insertPublicationForCleanupTest(t, r, ctx, "archived", "customer", "450", 56, "V3.0.1")
	second := insertPublicationForCleanupTest(t, r, ctx, "archived", "customer", "450", 56, "V3.0.2")
	otherOwner := insertPublicationForCleanupTest(t, r, ctx, "archived", "customer", "451", 56, "V3.0.3")
	otherType := insertPublicationForCleanupTest(t, r, ctx, "archived", "customer", "450", 57, "V3.0.4")
	active := insertPublicationForCleanupTest(t, r, ctx, "published", "customer", "450", 56, "V3.0.5")

	preview, err := r.PreviewDeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsPreviewCommand{
		ClearAll: true,
		Query:    appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 56, OwnerType: "customer", OwnerKey: "450"},
		Actor:    "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Count != 2 || preview.Rows[0].ID == otherOwner || preview.Rows[0].ID == otherType {
		t.Fatalf("preview=%+v", preview)
	}
	if _, err := r.DeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsCommand{ConfirmationToken: preview.ConfirmationToken, Actor: "admin"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id     int64
		status string
	}{{first, "deleted"}, {second, "deleted"}, {otherOwner, "archived"}, {otherType, "archived"}, {active, "published"}} {
		var status string
		if err := r.pool.QueryRow(ctx, `SELECT status FROM `+r.schema+`.bean_list_publications WHERE id=$1`, tc.id).Scan(&status); err != nil || status != tc.status {
			t.Fatalf("id=%d status=%q want=%q err=%v", tc.id, status, tc.status, err)
		}
	}
}

func TestPublicationDeleteRejectsNonArchivedAndStalePreview(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	active := insertPublicationForCleanupTest(t, r, ctx, "published", "official", "", 56, "V3.0.1")
	query := appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 56, OwnerType: "official"}
	if _, err := r.PreviewDeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsPreviewCommand{IDs: []int64{active}, Query: query, Actor: "admin"}); err == nil {
		t.Fatal("non-archived publication was accepted")
	}
	archived := insertPublicationForCleanupTest(t, r, ctx, "archived", "official", "", 56, "V3.0.2")
	preview, err := r.PreviewDeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsPreviewCommand{IDs: []int64{archived}, Query: query, Actor: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.pool.Exec(ctx, `UPDATE `+r.schema+`.bean_list_publications SET updated_at=updated_at+interval '1 second' WHERE id=$1`, archived); err != nil {
		t.Fatal(err)
	}
	if _, err := r.DeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsCommand{ConfirmationToken: preview.ConfirmationToken, Actor: "admin"}); err == nil {
		t.Fatal("stale preview was accepted")
	}
	var status string
	if err := r.pool.QueryRow(ctx, `SELECT status FROM `+r.schema+`.bean_list_publications WHERE id=$1`, archived).Scan(&status); err != nil || status != "archived" {
		t.Fatalf("status=%q err=%v", status, err)
	}
}

func TestPublicationDeleteRollsBackSnapshotAndPDFWhenAuditFails(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	id := insertPublicationForCleanupTest(t, r, ctx, "archived", "official", "", 56, "V3.0.1")
	if _, err := r.pool.Exec(ctx, `INSERT INTO `+r.schema+`.bean_list_publication_assets(publication_id,asset_type,content_type,cache_key,payload,created_by) VALUES($1,'pdf','application/pdf','x','pdf','tester')`, id); err != nil {
		t.Fatal(err)
	}
	preview, err := r.PreviewDeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsPreviewCommand{
		IDs: []int64{id}, Query: appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 56, OwnerType: "official"}, Actor: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.pool.Exec(ctx, `ALTER TABLE `+r.schema+`.audit_logs ADD CONSTRAINT reject_delete_audit CHECK(action <> 'delete_archived')`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.DeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsCommand{ConfirmationToken: preview.ConfirmationToken, Actor: "admin"}); err == nil {
		t.Fatal("delete succeeded despite failed audit")
	}
	var status string
	var assets int
	if err := r.pool.QueryRow(ctx, `SELECT status FROM `+r.schema+`.bean_list_publications WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM `+r.schema+`.bean_list_publication_assets WHERE publication_id=$1`, id).Scan(&assets); err != nil {
		t.Fatal(err)
	}
	if status != "archived" || assets != 1 {
		t.Fatalf("status=%q assets=%d", status, assets)
	}
}

func TestPublicationSummarySuggestedVersionIncludesDeletedHistory(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	insertPublicationForCleanupTest(t, r, ctx, "published", "official", "", 56, "V3.0.2")
	deleted := insertPublicationForCleanupTest(t, r, ctx, "deleted", "official", "", 56, "V3.0.9")
	if _, err := r.pool.Exec(ctx, `UPDATE `+r.schema+`.bean_list_publications SET deleted_at=now(),config_json='{}',content_json='{}' WHERE id=$1`, deleted); err != nil {
		t.Fatal(err)
	}
	page, err := r.ListBeanListPublicationSummaries(ctx, appcosting.BeanListPublicationSummaryQuery{
		BeanListPublicationQuery: appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 56, OwnerType: "official"},
		Status:                   "active", Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.SuggestedVersion != "V3.0.10" {
		t.Fatalf("suggested=%q", page.SuggestedVersion)
	}
}

func TestPublicationSummaryMetadataBackfillRunsInBoundedBatches(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	_, err := r.pool.Exec(ctx, `INSERT INTO `+r.schema+`.bean_list_publications
		(list_type,classification_template_id,version_no,status,owner_type,config_json,content_json)
		SELECT 'commercial',56,'V3.0.'||n,'archived','official',
		       jsonb_build_object('publication_batch',jsonb_build_object('release_id','release-'||n,'table_key','table-'||n,'table_name','表'||n,'is_default_table',n%2=0)),
		       jsonb_build_object('groups',jsonb_build_array(jsonb_build_object('items',jsonb_build_array(jsonb_build_object('id',n)))))
		FROM generate_series(1,1201) AS n`)
	if err != nil {
		t.Fatal(err)
	}
	if err := backfillBeanListPublicationSummaryMetadata(ctx, r.pool, r.schema); err != nil {
		t.Fatal(err)
	}
	var pending, withContent, named int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE publication_summary_ready=false),count(*) FILTER(WHERE publication_has_content),count(*) FILTER(WHERE publication_table_name<>'') FROM `+r.schema+`.bean_list_publications`).Scan(&pending, &withContent, &named); err != nil {
		t.Fatal(err)
	}
	if pending != 0 || withContent != 1201 || named != 1201 {
		t.Fatalf("pending=%d withContent=%d named=%d", pending, withContent, named)
	}
}

func TestUnarchiveSelectsNewDefaultAfterDeletedDefaultTable(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	var originalDefault, remaining int64
	for index, target := range []*int64{&originalDefault, &remaining} {
		if err := r.pool.QueryRow(ctx, `INSERT INTO `+r.schema+`.bean_list_publications
			(list_type,publication_purpose,classification_template_id,version_no,status,owner_type,publication_release_id,publication_table_key,publication_table_name,publication_is_default_table,publication_has_content,publication_summary_ready,config_json,content_json)
			VALUES('commercial','factory_supply',56,'V3.0.1','archived','official','release-one',$1::text,$2::text,$3::boolean,true,true,jsonb_build_object('publication_batch',jsonb_build_object('release_id','release-one','table_key',$1::text,'table_name',$2::text,'is_default_table',$3::boolean)),'{"price_rows":[1]}') RETURNING id`,
			fmt.Sprintf("table-%d", index+1), fmt.Sprintf("表%d", index+1), index == 0).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := r.PreviewDeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsPreviewCommand{
		IDs: []int64{originalDefault}, Query: appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 56, OwnerType: "official"}, Actor: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.DeleteBeanListPublications(ctx, appcosting.DeleteBeanListPublicationsCommand{ConfirmationToken: preview.ConfirmationToken, Actor: "admin"}); err != nil {
		t.Fatal(err)
	}
	if err := r.UnarchiveBeanListPublications(ctx, appcosting.ArchiveBeanListPublicationsCommand{IDs: []int64{remaining}, PublicationPurpose: "factory_supply", OwnerType: "official", Actor: "admin"}); err != nil {
		t.Fatal(err)
	}
	var status string
	var isDefault, configDefault bool
	if err := r.pool.QueryRow(ctx, `SELECT status,publication_is_default_table,(config_json->'publication_batch'->>'is_default_table')::boolean FROM `+r.schema+`.bean_list_publications WHERE id=$1`, remaining).Scan(&status, &isDefault, &configDefault); err != nil {
		t.Fatal(err)
	}
	if status != "published" || !isDefault || !configDefault {
		t.Fatalf("status=%q isDefault=%v configDefault=%v", status, isDefault, configDefault)
	}
}

func TestPublicationSummaryPerformanceAtCurrentAndTenfoldHistory(t *testing.T) {
	r, ctx := publicationCleanupPostgres(t)
	insertHistory := func(from, to int) {
		t.Helper()
		_, err := r.pool.Exec(ctx, `INSERT INTO `+r.schema+`.bean_list_publications
			(list_type,publication_purpose,classification_template_id,classification_template_name,version_no,status,owner_type,publication_release_id,publication_table_key,publication_table_name,publication_is_default_table,publication_has_content,publication_summary_ready,config_json,content_json,created_at)
			SELECT 'commercial','factory_supply',56,'工厂量单','V3.0.'||n,'archived','official','release-'||n,'table-'||n,'价格表 '||n,true,true,true,
			       jsonb_build_object('large',repeat('c',100000)),jsonb_build_object('groups',jsonb_build_array(jsonb_build_object('items',jsonb_build_array(jsonb_build_object('name','商品'||n,'price',99))))),
			       now()-(n||' minutes')::interval
			FROM generate_series($1::int,$2::int) AS n`, from, to)
		if err != nil {
			t.Fatal(err)
		}
	}
	query := appcosting.BeanListPublicationSummaryQuery{
		BeanListPublicationQuery: appcosting.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: "factory_supply", ClassificationTemplateID: 56, OwnerType: "official"},
		Status:                   "archived", Page: 1, PageSize: 10,
	}
	measure := func() (time.Duration, int) {
		t.Helper()
		if _, err := r.ListBeanListPublicationSummaries(ctx, query); err != nil {
			t.Fatal(err)
		}
		durations := make([]time.Duration, 0, 20)
		payloadSize := 0
		for i := 0; i < 20; i++ {
			started := time.Now()
			page, err := r.ListBeanListPublicationSummaries(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			durations = append(durations, time.Since(started))
			raw, _ := json.Marshal(page)
			payloadSize = len(raw)
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		return durations[18], payloadSize
	}
	insertHistory(1, 53)
	currentP95, payloadSize := measure()
	if payloadSize > 50*1024 || currentP95 > 300*time.Millisecond {
		t.Fatalf("current summary p95=%s payload=%d bytes", currentP95, payloadSize)
	}
	insertHistory(54, 530)
	tenfoldP95, tenfoldPayloadSize := measure()
	if tenfoldPayloadSize > 50*1024 || tenfoldP95 > time.Second {
		t.Fatalf("tenfold summary p95=%s payload=%d bytes", tenfoldP95, tenfoldPayloadSize)
	}
	t.Logf("summary performance current_p95=%s tenfold_p95=%s growth=%.2fx payload=%d/%d bytes", currentP95, tenfoldP95, float64(tenfoldP95)/float64(currentP95), payloadSize, tenfoldPayloadSize)
}
