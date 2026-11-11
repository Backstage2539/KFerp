package appmain

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	bomapp "orderapp/internal/application/bom"
	costingapp "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"
	bompg "orderapp/internal/infrastructure/postgres/bom"
	costingpg "orderapp/internal/infrastructure/postgres/costing"
	costinghttp "orderapp/internal/interfaces/http/costing"
)

func TestPricingRuleTrialSkipsUnresolvedCatalogWarningPostgresHTTP(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL is required for isolated pricing trial tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_pricing_trial_warning_%d_%d", os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	// Reuse only the schema-aware query helpers; no unrelated sample data is needed.
	f := &sampleFixture{t: t, ctx: ctx, pool: pool, schema: schema}
	f.exec("CREATE SCHEMA " + schema)
	if err := ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	f.exec("INSERT INTO %s.product_unit_definitions(code,name,unit_type,active) VALUES('袋','袋','package',true) ON CONFLICT(code) DO UPDATE SET active=true,deleted_at=NULL")
	productID := f.id("INSERT INTO %s.products(name,active,product_kind,unit_rule_override_json) VALUES('试算警告回归商品',true,'roasted_bean','{\"inventory_unit\":\"袋\"}') RETURNING id")
	materialID := f.id("INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price) VALUES('TRIAL-WARNING-MAT','熟豆','bean','kg','kg',80) RETURNING id")
	ruleID := f.id("INSERT INTO %s.product_pricing_rules(name,code,cost_source_mode,margin_rate,tax_rate,rounding_mode,calculation_json,formula_version,active) VALUES('试算回归规则','TRIAL-WARNING','bom_current_cost',.2,0,'cent','{\"tax_mode\":\"none\"}','v1',true) RETURNING id")
	bomsvc := bomapp.NewService(bompg.NewRepository(pool, schema))
	weight, largeWeight := float64(100), float64(500)
	bom, err := bomsvc.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "试算警告回归BOM", OutputType: "product", OutputProductID: productID, OutputQty: 1, Actor: "test",
		Variants: []bomapp.ProductionBomDraftVariant{{
			SpecKey: "100g", Name: "100g", InventoryUnit: "袋", IsDefault: true, LogisticsWeightG: &weight,
			Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: materialID, ConsumeUnit: "kg", QtyPerUnit: .1}},
		}, {
			SpecKey: "500g", Name: "500g", InventoryUnit: "袋", LogisticsWeightG: &largeWeight,
			Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: materialID, ConsumeUnit: "kg", QtyPerUnit: .5}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := bomsvc.PublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: bom.LatestVersionID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := bomsvc.BindProductProductionBom(ctx, bomapp.BindProductProductionBomCommand{ProductID: productID, BomID: bom.ID, BomVersionID: bom.LatestVersionID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	defaultSpecID := f.id("SELECT bom_spec_id FROM %s.production_bom_version_variants WHERE version_id=$1 AND spec_name_snapshot='100g'", bom.LatestVersionID)
	specID := f.id("SELECT bom_spec_id FROM %s.production_bom_version_variants WHERE version_id=$1 AND spec_name_snapshot='500g'", bom.LatestVersionID)
	repo := costingpg.NewRepository(pool, schema)
	e := echo.New()
	costinghttp.RegisterRoutes(e, costinghttp.Dependencies{Costing: costingapp.NewService(repo)})
	request := map[string]any{"pricing_rule_id": ruleID, "product_id": productID, "bom_spec_id": specID, "quote_unit": "袋"}
	const unresolvedWarning = "BOM规格成本无法完整解析"

	t.Run("scoped loader defers cost diagnostics", func(t *testing.T) {
		inputs, err := repo.LoadPricingRuleTrialProductInputs(ctx, domain.DefaultParameters(), 0, []int64{productID})
		if err != nil {
			t.Fatal(err)
		}
		if len(inputs) != 2 {
			t.Fatalf("trial input lost selected BOM specification: %+v", inputs)
		}
		for _, input := range inputs {
			if input.BomSpecID != defaultSpecID && input.BomSpecID != specID {
				t.Fatalf("unexpected trial specification: %+v", input)
			}
			if strings.Contains(strings.Join(input.Warnings, " "), unresolvedWarning) {
				t.Errorf("skipped cost resolution produced a false warning: %v", input.Warnings)
			}
		}
	})
	t.Run("complete trial has no unresolved warning", func(t *testing.T) {
		result := pr600APIJSON(t, e, http.MethodPost, "/api/costing/pricing-rule-trial", request, http.StatusOK)
		if result["cost_status"] != "complete" || result["base_cost"] != float64(40) || result["bom_spec_id"] != float64(specID) {
			t.Fatalf("expected complete, independently calculated 500g cost of 40: %v", result)
		}
		if strings.Contains(fmt.Sprint(result["warnings"]), unresolvedWarning) {
			t.Fatalf("complete trial returned a false unresolved warning: %v", result["warnings"])
		}
	})
	t.Run("batch trial keeps each specification cost", func(t *testing.T) {
		defaultRequest := map[string]any{"pricing_rule_id": ruleID, "product_id": productID, "bom_spec_id": defaultSpecID, "quote_unit": "袋"}
		response := pr600APIJSON(t, e, http.MethodPost, "/api/costing/pricing-rule-trials", map[string]any{"requests": []any{defaultRequest, request}}, http.StatusOK)
		rows := response["rows"].([]any)
		if len(rows) != 2 {
			t.Fatalf("expected both specification results: %v", response)
		}
		for i, want := range []float64{8, 40} {
			result := rows[i].(map[string]any)["result"].(map[string]any)
			if result["cost_status"] != "complete" || result["base_cost"] != want || strings.Contains(fmt.Sprint(result["warnings"]), unresolvedWarning) {
				t.Errorf("specification %d expected cost %v without false warning: %v", i, want, result)
			}
		}
	})

	// A real unit mismatch must still be diagnosed by the full catalog resolver
	// and by the trial's independent detailed-cost calculation.
	f.exec("UPDATE %[1]s.production_bom_version_items SET consume_unit='个' WHERE version_id=$1 AND variant_id=(SELECT id FROM %[1]s.production_bom_version_variants WHERE version_id=$1 AND bom_spec_id=$2)", bom.LatestVersionID, specID)
	t.Run("full loader retains real unresolved warning", func(t *testing.T) {
		inputs, err := repo.LoadProductInputs(ctx, domain.DefaultParameters())
		if err != nil {
			t.Fatal(err)
		}
		if len(inputs) != 2 {
			t.Fatalf("real unresolved BOM cost was hidden: %+v", inputs)
		}
		for _, input := range inputs {
			if got := strings.Contains(strings.Join(input.Warnings, " "), unresolvedWarning); got != (input.BomSpecID == specID) {
				t.Errorf("unresolved warning must affect only the broken specification: %+v", input)
			}
		}
	})
	t.Run("incomplete trial rejects price", func(t *testing.T) {
		result := pr600APIJSON(t, e, http.MethodPost, "/api/costing/pricing-rule-trial", request, http.StatusOK)
		if result["cost_status"] != "incomplete" || !strings.Contains(fmt.Sprint(result["unresolved_components"]), "invalid_component_quantity_or_unit") || result["final_unit_price"] != nil {
			t.Fatalf("incomplete trial must explain the unresolved cost without a price: %v", result)
		}
	})
}
