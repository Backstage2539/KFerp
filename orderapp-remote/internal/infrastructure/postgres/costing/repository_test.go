package costing

import (
	"os"
	"strings"
	"testing"
)

func TestLoadProductInputsReadsBeanMetadataFromProfileTable(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "material_bean_profiles") {
		t.Fatalf("costing repository must join material_bean_profiles for bean-list metadata")
	}
	for _, forbidden := range []string{"m.flavor", "m.origin", "m.processing_station", "m.variety", "m.process_method", "m.grade", "m.altitude", "m.bean_list_note"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("costing repository still reads %s from materials", forbidden)
		}
	}
}

func TestLoadProductInputsDoesNotUsePublishedDefaultPriceAsBeanCost(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if strings.Contains(src, "NULLIF(p.default_price") {
		t.Fatalf("costing repository must not reuse published product default_price as green bean cost")
	}
}

func TestPublishBeanListUsesQueryRowBeforeAuditToAvoidBusyConnection(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) PublishBeanList")
	end := strings.Index(src, "func (r Repository) WithdrawBeanList")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("PublishBeanList function not found")
	}
	body := src[start:end]
	if !strings.Contains(body, "tx.QueryRow(ctx, fmt.Sprintf(`") {
		t.Fatalf("PublishBeanList must use QueryRow for INSERT ... RETURNING before audit writes")
	}
	if strings.Contains(body, "tx.Query(ctx, fmt.Sprintf(`\n\t\tINSERT INTO %s.bean_list_publications") {
		t.Fatalf("PublishBeanList must not leave pgx Rows open before AuditInsertTx; it causes conn busy")
	}
}

func TestPublishedBeanListReadsOnlyCurrentPublishedSnapshot(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) PublishedBeanList")
	end := strings.Index(src, "func (r Repository) PublishBeanList")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("PublishedBeanList function not found")
	}
	body := src[start:end]
	for _, want := range []string{
		"WHERE list_type=$1 AND owner_type=$2 AND owner_key=$3 AND status='published'",
		"ORDER BY published_at DESC, id DESC",
		"LIMIT 1",
		"pgx.ErrNoRows",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PublishedBeanList must read current published snapshot; missing %q", want)
		}
	}
}

func TestBeanListPublicationSchemaSupportsOwnedLockedSnapshots(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"owner_type TEXT NOT NULL DEFAULT 'official'",
		"owner_key TEXT NOT NULL DEFAULT ''",
		"price_source_publication_id BIGINT",
		"style_source_publication_id BIGINT",
		"source_version_no TEXT NOT NULL DEFAULT ''",
		"bean_list_publications_one_published_owner_idx",
		"ON %[1]s.bean_list_publications(list_type, owner_type, owner_key)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("bean list publication schema must support owned locked snapshots; missing %q", want)
		}
	}
}

func TestPublishBeanListWithdrawsOnlySameOwnerSnapshot(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) PublishBeanList")
	end := strings.Index(src, "func (r Repository) WithdrawBeanList")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("PublishBeanList function not found")
	}
	body := src[start:end]
	for _, want := range []string{
		"WHERE list_type=$1 AND owner_type=$2 AND owner_key=$3 AND status='published'",
		"price_source_publication_id",
		"style_source_publication_id",
		"source_version_no",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("PublishBeanList must lock customer snapshots independently; missing %q", want)
		}
	}
}
