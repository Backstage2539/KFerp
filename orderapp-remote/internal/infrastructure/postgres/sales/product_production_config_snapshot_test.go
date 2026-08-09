package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPR584OrderItemSnapshotIncludesOrderedIndustryTemplateIDs(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for sales postgres tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_sales_pr584_snapshot_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE SCHEMA %[1]s;
		CREATE TABLE %[1]s.product_production_configs (
			product_id BIGINT PRIMARY KEY,
			production_bom_id BIGINT NOT NULL DEFAULT 0,
			production_bom_version_id BIGINT NOT NULL DEFAULT 0,
			process_route_id BIGINT NOT NULL DEFAULT 0,
			industry_field_template_id BIGINT NOT NULL DEFAULT 0,
			expected_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.product_production_config_industry_templates (
			product_id BIGINT NOT NULL,
			template_id BIGINT NOT NULL,
			sort_order INT NOT NULL DEFAULT 1,
			PRIMARY KEY(product_id, template_id)
		);
		CREATE TABLE %[1]s.product_production_config_fields (
			id BIGINT PRIMARY KEY,
			product_id BIGINT NOT NULL,
			field_key TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL DEFAULT '',
			field_type TEXT NOT NULL DEFAULT 'text',
			unit TEXT NOT NULL DEFAULT '',
			value_text TEXT NOT NULL DEFAULT '',
			value_number NUMERIC(14,4),
			value_bool BOOLEAN,
			template_field_key TEXT NOT NULL DEFAULT '',
			required BOOLEAN NOT NULL DEFAULT false,
			options_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			show_in_price_list BOOLEAN NOT NULL DEFAULT true,
			sort_order INT NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.product_production_configs(product_id, industry_field_template_id) VALUES
			(91,3002),
			(92,3003);
		INSERT INTO %[1]s.product_production_config_industry_templates(product_id, template_id, sort_order) VALUES
			(91,3002,1),
			(91,3001,2);
	`, schema)); err != nil {
		t.Fatalf("create sales snapshot schema: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for _, tc := range []struct {
		productID int64
		wantIDs   []float64
	}{
		{productID: 91, wantIDs: []float64{3002, 3001}},
		{productID: 92, wantIDs: []float64{3003}},
	} {
		raw, err := loadProductProductionConfigSummaryForOrderItemTx(ctx, tx, schema, tc.productID)
		if err != nil {
			t.Fatalf("load product %d snapshot: %v", tc.productID, err)
		}
		var snapshot struct {
			IndustryFieldTemplateID  float64   `json:"industry_field_template_id"`
			IndustryFieldTemplateIDs []float64 `json:"industry_field_template_ids"`
		}
		if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
			t.Fatalf("decode product %d snapshot %s: %v", tc.productID, raw, err)
		}
		if len(snapshot.IndustryFieldTemplateIDs) != len(tc.wantIDs) {
			t.Fatalf("product %d template ids=%v, want %v", tc.productID, snapshot.IndustryFieldTemplateIDs, tc.wantIDs)
		}
		for i := range tc.wantIDs {
			if snapshot.IndustryFieldTemplateIDs[i] != tc.wantIDs[i] {
				t.Fatalf("product %d template ids=%v, want %v", tc.productID, snapshot.IndustryFieldTemplateIDs, tc.wantIDs)
			}
		}
		if snapshot.IndustryFieldTemplateID != tc.wantIDs[0] {
			t.Fatalf("product %d legacy template id=%v, want first %v", tc.productID, snapshot.IndustryFieldTemplateID, tc.wantIDs[0])
		}
	}
}
