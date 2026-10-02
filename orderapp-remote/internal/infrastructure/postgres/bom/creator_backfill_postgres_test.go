package bom

import (
	"fmt"
	"testing"
)

func TestStartupBackfillDoesNotDuplicateExplicitProductBOM(t *testing.T) {
	ctx, pool, schema := newPR600SpecTemplatePublishTestDB(t)
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name) VALUES (1,'创建器商品'),(2,'只有旧式配方的商品');
		INSERT INTO %[1]s.product_bom(product_id) VALUES (1),(2);
		INSERT INTO %[1]s.production_boms(code,name,output_type,output_product_id,legacy_product_id)
		VALUES ('PC-CREATOR-1','创建器商品 BOM','product',1,0);
	`, schema))
	if err != nil {
		t.Fatal(err)
	}
	for restart := 0; restart < 2; restart++ {
		if err := EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
		var explicitCount, legacyCount int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT count(*) FILTER (WHERE output_product_id=1),
			       count(*) FILTER (WHERE output_product_id=2 AND legacy_product_id=2)
			FROM %s.production_boms
		`, schema)).Scan(&explicitCount, &legacyCount); err != nil {
			t.Fatal(err)
		}
		if explicitCount != 1 || legacyCount != 1 {
			t.Fatalf("restart %d: explicit BOM must remain unique; legacy-only product must migrate once: explicit=%d legacy=%d", restart, explicitCount, legacyCount)
		}
	}
}
