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
