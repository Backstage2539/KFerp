package costing

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	appcosting "orderapp/internal/application/costing"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func namedBatchPostgres(t *testing.T) (Repository, context.Context) {
	t.Helper()
	if os.Getenv("KF_RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("disposable PostgreSQL integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("pr635_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	data, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "CREATE TABLE IF NOT EXISTS %[1]s.bean_list_publications (")
	end := start + strings.Index(src[start:], "\n);") + 4
	ddl := strings.ReplaceAll(src[start:end], "%[1]s", schema)
	ddl += `; CREATE TABLE ` + schema + `.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);`
	ddl += `ALTER TABLE ` + schema + `.bean_list_publications ADD CONSTRAINT test_reject CHECK (content_json->>'reject' IS DISTINCT FROM 'yes');`
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	return NewRepository(pool, schema), ctx
}

func postgresBatchCommands() []appcosting.PublishBeanListCommand {
	rows := []appcosting.PublishBeanListCommand{}
	for i := 1; i <= 3; i++ {
		cfg := map[string]any{}
		appcosting.SetBeanListBatchMetadata(cfg, appcosting.PublicationTableMetadata{TableKey: fmt.Sprint(i), TableName: fmt.Sprintf("规格%d", i), IsDefaultTable: i == 1})
		rows = append(rows, appcosting.PublishBeanListCommand{ListType: "commercial", PublicationPurpose: "factory_supply", OwnerType: "official", Version: "V3.0.6", Config: cfg, Content: map[string]any{}, Actor: "pr635-test"})
	}
	return rows
}

func TestNamedPriceTableBatchPostgresAtomicDefaultAndLifecycle(t *testing.T) {
	r, ctx := namedBatchPostgres(t)
	bad := postgresBatchCommands()
	bad[2].Content["reject"] = "yes"
	if _, err := r.SaveBeanListBatch(ctx, bad, true); err == nil {
		t.Fatal("expected failing third insert")
	}
	for _, table := range []string{"bean_list_publications", "audit_logs"} {
		var n int
		if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM "+r.schema+"."+table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("rollback %s=%d err=%v", table, n, err)
		}
	}
	rows, err := r.SaveBeanListBatch(ctx, postgresBatchCommands(), true)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Version != "V3.0.6" || row.ReleaseID != rows[0].ReleaseID || row.ReleaseID == "" {
			t.Fatalf("batch=%+v", rows)
		}
	}
	query := appcosting.BeanListPublicationQuery{PublicationPurpose: "factory_supply", OwnerType: "official", ListType: "commercial"}
	current, err := r.PublishedBeanList(ctx, query)
	if err != nil || current.ID != rows[0].ID {
		t.Fatalf("default=%+v err=%v", current, err)
	}
	if err := r.WithdrawBeanList(ctx, appcosting.WithdrawBeanListCommand{ID: rows[1].ID, PublicationPurpose: "factory_supply", OwnerType: "official", Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM "+r.schema+".bean_list_publications WHERE status='withdrawn'").Scan(&n); err != nil || n != 3 {
		t.Fatalf("withdraw=%d err=%v", n, err)
	}
	archive := appcosting.ArchiveBeanListPublicationsCommand{IDs: []int64{rows[0].ID}, PublicationPurpose: "factory_supply", OwnerType: "official", Actor: "test"}
	if err := r.ArchiveBeanListPublications(ctx, archive); err != nil {
		t.Fatal(err)
	}
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM "+r.schema+".bean_list_publications WHERE status='archived'").Scan(&n); err != nil || n != 3 {
		t.Fatalf("archive=%d err=%v", n, err)
	}
	if err := r.UnarchiveBeanListPublications(ctx, archive); err != nil {
		t.Fatal(err)
	}
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM "+r.schema+".bean_list_publications WHERE status='withdrawn'").Scan(&n); err != nil || n != 3 {
		t.Fatalf("restore=%d err=%v", n, err)
	}
}

func TestNamedPriceTableBatchPostgresConcurrentVersionAllocation(t *testing.T) {
	r, ctx := namedBatchPostgres(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	versions := map[string]bool{}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := r.SaveBeanListBatch(ctx, postgresBatchCommands(), true)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if versions[rows[0].Version] {
				t.Errorf("duplicate version %s", rows[0].Version)
			}
			versions[rows[0].Version] = true
			for _, row := range rows {
				if row.Version != rows[0].Version {
					t.Error("split version")
				}
			}
		}()
	}
	wg.Wait()
	if len(versions) != 4 {
		t.Fatalf("versions=%v", versions)
	}
}
