package costing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appcosting "orderapp/internal/application/costing"

	"github.com/labstack/echo/v4"
)

type initialDefaultRepo struct {
	pr543BeanListPublicationRepo
	bom bool
}

func (r *initialDefaultRepo) SaveBeanListDraft(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	return r.PublishBeanList(ctx, cmd)
}

func (r *initialDefaultRepo) ResolveProductBOMSpecIdentity(_ context.Context, product, spec, variant int64) (appcosting.ProductBOMSpecIdentity, error) {
	if !r.bom || product != 600 || spec != 701 || variant != 703 {
		return appcosting.ProductBOMSpecIdentity{}, appcosting.ErrProductBOMSpecIdentityNotFound
	}
	return appcosting.ProductBOMSpecIdentity{ParentProductID: 600, ParentProductName: "测试商品", BomID: 90, BomVersionID: 902, BomSpecID: 701, BomVariantID: 703, SpecKey: "bag-227", SpecName: "227g袋", InventoryUnit: "袋", MigrationState: "cutover", SpecIdentityMode: "bom_spec", BomSpecAuthoritative: true, Active: true, Published: true}, nil
}

func initialDefaultPayload(bom bool, historicalDefault int64) map[string]any {
	selection := map[string]any{"parent_product_id": 600, "sku_id": 991, "selection_source": "product_default", "default_sku_id_at_selection": historicalDefault}
	row := map[string]any{
		"product_id": 991, "parent_product_id": 600, "sku_id": 991,
		"final_unit_price": 82, "fixed_unit_price": 82, "pricing_mode": "fixed_price", "pricing_mode_source": "product_catalog",
		"price_unit": "227g", "inventory_unit": "g", "inventory_conversion_json": map[string]any{"227g": map[string]any{"g": 227}},
		"group_source": "product_catalog", "group_snapshot": map[string]any{"group_id": 3, "group_item_id": 101},
		"cost_source_snapshot": map[string]any{"source": "fixed_price", "bom_version_id": 901}, "customer_reference_snapshot": map[string]any{}, "manual_adjusted": true,
	}
	if bom {
		for key, value := range map[string]any{"product_id": 600, "bom_id": 90, "bom_version_id": 902, "bom_spec_id": 701, "bom_variant_id": 703} {
			selection[key], row[key] = value, value
		}
		delete(selection, "sku_id")
		delete(row, "sku_id")
		row["price_unit"], row["inventory_unit"] = "袋", "袋"
		delete(row, "inventory_conversion_json")
	}
	return map[string]any{"list_type": "commercial", "version": "V727", "scope": "official", "config": map[string]any{"product_spec_selections": []any{selection}}, "content": map[string]any{"price_rows": []any{row}}}
}

func TestDefaultSpecInitialOnlyPublicationHTTP(t *testing.T) {
	for _, path := range []string{"/api/costing/bean-list/drafts", "/api/costing/bean-list/publications"} {
		for _, bom := range []bool{false, true} {
			for _, historicalDefault := range []int64{0, 990, 991} {
				t.Run(fmt.Sprintf("%s/bom=%t/default=%d", path, bom, historicalDefault), func(t *testing.T) {
					repo := &initialDefaultRepo{bom: bom}
					e := echo.New()
					e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
						return func(c echo.Context) error {
							c.Set("basic_auth_admin", true)
							c.Set("employee_id", int64(7))
							return next(c)
						}
					})
					RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(repo)})
					payload, _ := json.Marshal(initialDefaultPayload(bom, historicalDefault))
					req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(payload)))
					req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
					rec := httptest.NewRecorder()
					e.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
					}
					rows := publicMapsFromAny(repo.published.Content["price_rows"])
					if len(rows) != 1 || mapNumber(rows[0], "final_unit_price", 0) != 82 {
						t.Fatalf("quote changed: %#v", rows)
					}
					if bom && (mapNumber(rows[0], "bom_version_id", 0) != 902 || mapNumber(rows[0], "bom_variant_id", 0) != 703) {
						t.Fatalf("wrong current identity: %#v", rows[0])
					}
				})
			}
		}
	}
}

func TestDefaultSpecInitialOnlyStillRejectsInvalidSelectionsHTTP(t *testing.T) {
	for _, problem := range []string{"foreign_spec", "old_bom_version", "missing_price", "mismatched_price_spec"} {
		for _, path := range []string{"/api/costing/bean-list/drafts", "/api/costing/bean-list/publications"} {
			t.Run(problem+path, func(t *testing.T) {
				repo := &initialDefaultRepo{bom: true}
				e := echo.New()
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error { c.Set("basic_auth_admin", true); return next(c) }
				})
				RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(repo)})
				payload := initialDefaultPayload(true, 0)
				selection := payload["config"].(map[string]any)["product_spec_selections"].([]any)[0].(map[string]any)
				content := payload["content"].(map[string]any)
				switch problem {
				case "foreign_spec":
					selection["bom_spec_id"] = 999
				case "old_bom_version":
					selection["bom_version_id"] = 901
				case "missing_price":
					content["price_rows"] = []any{}
				case "mismatched_price_spec":
					content["price_rows"].([]any)[0].(map[string]any)["bom_spec_id"] = 999
				}
				body, _ := json.Marshal(payload)
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				if repo.published.Content != nil {
					t.Fatal("invalid payload persisted")
				}
			})
		}
	}
}
