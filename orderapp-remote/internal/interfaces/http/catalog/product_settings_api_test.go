package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	catalogapp "orderapp/internal/application/catalog"
	"reflect"
	"testing"

	"github.com/labstack/echo/v4"
)

type productSettingsRepo struct {
	products            []catalogapp.Product
	categories          []catalogapp.ProductCategory
	gradientTemplates   []catalogapp.GradientTemplate
	savedCategory       catalogapp.SaveProductCategoryCommand
	movedCategory       catalogapp.MoveProductCategoryCommand
	deletedCategory     catalogapp.DeleteProductCategoryCommand
	assigned            catalogapp.AssignProductCategoryCommand
	savedTemplate       catalogapp.SaveGradientTemplateCommand
	deactivatedTemplate catalogapp.DeactivateGradientTemplateCommand
	boundTemplate       catalogapp.BindCategoryGradientTemplateCommand
	updated             catalogapp.UpdateProductBasicsCommand
	updateErr           error
	deactivated         catalogapp.DeactivateProductsCommand
	createdPublic       catalogapp.CreateProductCommand
	createErr           error
	createdProduct      catalogapp.CreateCustomProductCommand
	categoryCreated     bool
	categoryMoved       bool
	categoryDeleted     bool
	productAssigned     bool
	templateSaved       bool
	templateDeactivated bool
	templateBound       bool
	productUpdated      bool
	productsDeactivated bool
	publicCreated       bool
	productCreated      bool
}

func (r *productSettingsRepo) ListProducts(ctx context.Context) ([]catalogapp.Product, error) {
	return r.products, nil
}

func (r *productSettingsRepo) GetProduct(ctx context.Context, id int64) (*catalogapp.Product, error) {
	for i := range r.products {
		if r.products[i].ID == id {
			return &r.products[i], nil
		}
	}
	return nil, nil
}

func (r *productSettingsRepo) ReplacePriceTiers(ctx context.Context, cmd catalogapp.ReplacePriceTiersCommand) error {
	return nil
}

func (r *productSettingsRepo) UpdateProductBasics(ctx context.Context, cmd catalogapp.UpdateProductBasicsCommand) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updated = cmd
	r.productUpdated = true
	return nil
}

func (r *productSettingsRepo) DeactivateProducts(ctx context.Context, cmd catalogapp.DeactivateProductsCommand) error {
	r.deactivated = cmd
	r.productsDeactivated = true
	return nil
}

func (r *productSettingsRepo) CreateProduct(ctx context.Context, cmd catalogapp.CreateProductCommand) (catalogapp.Product, error) {
	if r.createErr != nil {
		return catalogapp.Product{}, r.createErr
	}
	r.createdPublic = cmd
	r.publicCreated = true
	return catalogapp.Product{
		ID:                    77,
		Name:                  cmd.Name,
		RoastLevel:            cmd.RoastLevel,
		ProductKind:           cmd.ProductKind,
		DripBagGrams:          cmd.DripBagGrams,
		DripBoxBagCount:       cmd.DripBoxBagCount,
		AllowFulfillmentOrder: cmd.AllowFulfillmentOrder,
		AllowMallOrder:        cmd.AllowMallOrder,
		SalesUnits:            cmd.SalesUnits,
		DefaultPrice:          cmd.DefaultPrice,
		YieldRate:             cmd.YieldRate,
		Visibility:            "public",
		BomItemCount:          0,
		CustomerID:            0,
		BaseProductID:         0,
	}, nil
}

func (r *productSettingsRepo) ListProductCategories(ctx context.Context) ([]catalogapp.ProductCategory, error) {
	return r.categories, nil
}

func (r *productSettingsRepo) ListGradientTemplates(ctx context.Context) ([]catalogapp.GradientTemplate, error) {
	return r.gradientTemplates, nil
}

func (r *productSettingsRepo) SaveGradientTemplate(ctx context.Context, cmd catalogapp.SaveGradientTemplateCommand) (catalogapp.GradientTemplate, error) {
	r.savedTemplate = cmd
	r.templateSaved = true
	return catalogapp.GradientTemplate{ID: 77, Name: cmd.Name, DisplayUnit: cmd.DisplayUnit, Active: true, Tiers: cmd.Tiers}, nil
}

func (r *productSettingsRepo) DeactivateGradientTemplate(ctx context.Context, cmd catalogapp.DeactivateGradientTemplateCommand) error {
	r.deactivatedTemplate = cmd
	r.templateDeactivated = true
	return nil
}

func (r *productSettingsRepo) BindCategoryGradientTemplate(ctx context.Context, cmd catalogapp.BindCategoryGradientTemplateCommand) error {
	r.boundTemplate = cmd
	r.templateBound = true
	return nil
}

func (r *productSettingsRepo) SaveProductCategory(ctx context.Context, cmd catalogapp.SaveProductCategoryCommand) (catalogapp.ProductCategory, error) {
	r.savedCategory = cmd
	r.categoryCreated = true
	return catalogapp.ProductCategory{ID: 99, ParentID: cmd.ParentID, CustomerID: cmd.CustomerID, Name: cmd.Name, Position: cmd.Position}, nil
}

func (r *productSettingsRepo) MoveProductCategory(ctx context.Context, cmd catalogapp.MoveProductCategoryCommand) error {
	r.movedCategory = cmd
	r.categoryMoved = true
	return nil
}

func (r *productSettingsRepo) DeleteProductCategory(ctx context.Context, cmd catalogapp.DeleteProductCategoryCommand) error {
	r.deletedCategory = cmd
	r.categoryDeleted = true
	return nil
}

func (r *productSettingsRepo) AssignProductCategory(ctx context.Context, cmd catalogapp.AssignProductCategoryCommand) error {
	r.assigned = cmd
	r.productAssigned = true
	return nil
}

func (r *productSettingsRepo) CreateCustomProduct(ctx context.Context, cmd catalogapp.CreateCustomProductCommand) (catalogapp.Product, error) {
	r.createdProduct = cmd
	r.productCreated = true
	return catalogapp.Product{
		ID:            88,
		Name:          cmd.Name,
		RoastLevel:    cmd.RoastLevel,
		CustomerID:    cmd.CustomerID,
		BaseProductID: cmd.BaseProductID,
		Visibility:    "customer_only",
		CustomType:    cmd.CustomType,
	}, nil
}

func TestProductSettingsAPISupportsCategoryTreeAndDragAssignments(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID: 7, Name: "曲奇拼配", ProductCategoryID: 2, ProductCategoryPosition: 1, YieldRate: 0.82, BomItemCount: 2,
		}},
		categories: []catalogapp.ProductCategory{
			{ID: 1, Name: "咖啡豆", Level: 1, Position: 1},
			{ID: 2, ParentID: 1, Name: "意式拼配", Level: 2, Position: 1, GradientTemplateID: 9},
			{ID: 10, Name: "客户A分类", Level: 1, Position: 1, CustomerID: 3},
			{ID: 11, ParentID: 10, Name: "客户A二级", Level: 2, Position: 1, CustomerID: 3},
		},
		gradientTemplates: []catalogapp.GradientTemplate{{
			ID: 9, Name: "工厂量单模板", DisplayUnit: "kg", Active: true,
			Tiers: []catalogapp.GradientTemplateTier{{ID: 91, Label: "24-49kg", MinWeightG: 24000, MaxWeightG: f64(49000), MarginRate: 0.175, Position: 1}},
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"categories"`, `"children"`, `"products"`, `"gradient_templates"`, `"gradient_template_id":9`, `"name":"工厂量单模板"`, `"display_unit":"kg"`, `"number":1`, `"name":"咖啡豆"`, `"name":"意式拼配"`, `"name":"客户A分类"`, `"customer_id":3`, `"name":"曲奇拼配"`, `"yield_rate":0.82`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing %s: %s", want, rec.Body.String())
		}
	}
	for _, want := range []string{`"customer_id":0`, `"base_product_id":0`, `"visibility":"public"`, `"custom_type":""`, `"bom_item_count":2`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("product settings response missing ownership field %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/categories", bytes.NewBufferString(`{"name":"单品豆","parent_id":1,"position":2,"customer_id":3}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST category status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.categoryCreated || repo.savedCategory.Name != "单品豆" || repo.savedCategory.ParentID != 1 || repo.savedCategory.Position != 2 || repo.savedCategory.CustomerID != 3 {
		t.Fatalf("category command = %+v created=%v", repo.savedCategory, repo.categoryCreated)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/categories/2/move", bytes.NewBufferString(`{"parent_id":1,"position":1}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST move category status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.categoryMoved || repo.movedCategory.ID != 2 || repo.movedCategory.ParentID != 1 || repo.movedCategory.Position != 1 {
		t.Fatalf("move category command = %+v moved=%v", repo.movedCategory, repo.categoryMoved)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/product-settings/categories/2", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE category status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.categoryDeleted || repo.deletedCategory.ID != 2 {
		t.Fatalf("delete category command = %+v deleted=%v", repo.deletedCategory, repo.categoryDeleted)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/products/7/category", bytes.NewBufferString(`{"category_id":2,"position":3}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST assign product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productAssigned || repo.assigned.ProductID != 7 || repo.assigned.CategoryID != 2 || repo.assigned.Position != 3 {
		t.Fatalf("assign product command = %+v assigned=%v", repo.assigned, repo.productAssigned)
	}
}

func TestProductSettingsAPIManagesGradientTemplatesAndCategoryBinding(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"name":"工厂量单模板","display_unit":"g227","tiers":[{"label":"2-7份","min_display_qty":2,"max_display_qty":7,"margin_rate":0.175,"position":1},{"label":"8份+","min_display_qty":8,"margin_rate":0.12,"position":2}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/pricing-gradient-templates", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.templateSaved || repo.savedTemplate.Name != "工厂量单模板" || repo.savedTemplate.DisplayUnit != "g227" || len(repo.savedTemplate.Tiers) != 2 {
		t.Fatalf("template command = %+v saved=%v", repo.savedTemplate, repo.templateSaved)
	}
	if repo.savedTemplate.Tiers[0].MinWeightG != 454 || repo.savedTemplate.Tiers[0].MaxWeightG == nil || *repo.savedTemplate.Tiers[0].MaxWeightG != 1589 || repo.savedTemplate.Tiers[0].MarginRate != 0.175 {
		t.Fatalf("template tiers = %+v", repo.savedTemplate.Tiers)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/product-settings/categories/2/gradient-template", bytes.NewBufferString(`{"gradient_template_id":77}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST category template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.templateBound || repo.boundTemplate.CategoryID != 2 || repo.boundTemplate.GradientTemplateID != 77 {
		t.Fatalf("bind command = %+v bound=%v", repo.boundTemplate, repo.templateBound)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/pricing-gradient-templates/77/deactivate", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST deactivate template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.templateDeactivated || repo.deactivatedTemplate.ID != 77 {
		t.Fatalf("deactivate command = %+v deactivated=%v", repo.deactivatedTemplate, repo.templateDeactivated)
	}
}

func TestProductSettingsAPICreatesCustomerCustomProduct(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"customer_id":3,"base_product_id":7,"name":"测试客户-橘皮乌龙-中深烘","roast_level":"中深烘","custom_type":"custom_roast","copy_bom":true,"copy_price_tiers":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/custom-products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST custom product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productCreated || repo.createdProduct.CustomerID != 3 || repo.createdProduct.BaseProductID != 7 || !repo.createdProduct.CopyBOM || !repo.createdProduct.CopyPriceTiers {
		t.Fatalf("custom product command = %+v created=%v", repo.createdProduct, repo.productCreated)
	}
	for _, want := range []string{`"product"`, `"customer_id":3`, `"base_product_id":7`, `"visibility":"customer_only"`, `"custom_type":"custom_roast"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("custom product response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPICreatesPublicProduct(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"name":"新公共拼配","roast_level":"中深烘","default_price":88,"yield_rate":0.805}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST public product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicCreated || repo.createdPublic.Name != "新公共拼配" || repo.createdPublic.RoastLevel != "中深烘" || repo.createdPublic.DefaultPrice != 88 || repo.createdPublic.RetailPrice227G != 88 || repo.createdPublic.YieldRate != 0.805 {
		t.Fatalf("public product command = %+v created=%v", repo.createdPublic, repo.publicCreated)
	}
	for _, want := range []string{`"product"`, `"name":"新公共拼配"`, `"customer_id":0`, `"visibility":"public"`, `"bom_item_count":0`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("public product response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestProductSettingsAPICreatesDripBagProduct(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"name":"耶加雪菲挂耳","product_kind":"drip_bag","drip_bag_grams":10,"drip_box_bag_count":10,"allow_fulfillment_order":true,"allow_mall_order":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST drip product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicCreated || repo.createdPublic.ProductKind != "drip_bag" || repo.createdPublic.DripBagGrams != 10 || repo.createdPublic.DripBoxBagCount != 10 || !repo.createdPublic.AllowFulfillmentOrder || !repo.createdPublic.AllowMallOrder {
		t.Fatalf("drip product command = %+v created=%v", repo.createdPublic, repo.publicCreated)
	}
	var payload struct {
		Product struct {
			ProductKind     string   `json:"product_kind"`
			DripBagGrams    float64  `json:"drip_bag_grams"`
			DripBoxBagCount int      `json:"drip_box_bag_count"`
			SalesUnits      []string `json:"sales_units"`
		} `json:"product"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode product response: %v body=%s", err, rec.Body.String())
	}
	if payload.Product.ProductKind != "drip_bag" || payload.Product.DripBagGrams != 10 || payload.Product.DripBoxBagCount != 10 {
		t.Fatalf("drip product response = %+v body=%s", payload.Product, rec.Body.String())
	}
	if !reflect.DeepEqual(payload.Product.SalesUnits, []string{"bag", "box"}) {
		t.Fatalf("sales_units = %#v, want bag/box body=%s", payload.Product.SalesUnits, rec.Body.String())
	}
}

func TestProductSettingsAPIDefaultsOmittedDripBagConfig(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	body := `{"name":"耶加雪菲挂耳","product_kind":"drip_bag"}`
	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST drip product with omitted config status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.publicCreated || repo.createdPublic.ProductKind != "drip_bag" || repo.createdPublic.DripBagGrams != 10 || repo.createdPublic.DripBoxBagCount != 10 {
		t.Fatalf("omitted drip config command = %+v created=%v", repo.createdPublic, repo.publicCreated)
	}
	var payload struct {
		Product struct {
			DripBagGrams    float64  `json:"drip_bag_grams"`
			DripBoxBagCount int      `json:"drip_box_bag_count"`
			SalesUnits      []string `json:"sales_units"`
		} `json:"product"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode product response: %v body=%s", err, rec.Body.String())
	}
	if payload.Product.DripBagGrams != 10 || payload.Product.DripBoxBagCount != 10 || !reflect.DeepEqual(payload.Product.SalesUnits, []string{"bag", "box"}) {
		t.Fatalf("omitted drip config response = %+v body=%s", payload.Product, rec.Body.String())
	}
}

func TestProductSettingsAPIReturnsInternalErrorForProductCreatePersistenceFailure(t *testing.T) {
	repo := &productSettingsRepo{createErr: errors.New("database unavailable")}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(`{"name":"新公共拼配","roast_level":"中深烘"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST product persistence failure status=%d, want 500 body=%s", rec.Code, rec.Body.String())
	}
	if repo.publicCreated {
		t.Fatalf("failed persistence should not mark product created, command=%+v", repo.createdPublic)
	}
}

func TestProductSettingsAPIReturnsBadRequestForCreateValidationFailures(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing name",
			body: `{"roast_level":"中深烘"}`,
			want: "name required",
		},
		{
			name: "negative price",
			body: `{"name":"新公共拼配","roast_level":"中深烘","default_price":-1}`,
			want: "price must not be negative",
		},
		{
			name: "invalid roast level",
			body: `{"name":"X","roast_level":"not-a-level"}`,
			want: "invalid roast_level",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &productSettingsRepo{}
			e := echo.New()
			registerProductRoutes(e, catalogapp.NewService(repo))

			req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("POST product validation failure status=%d, want 400 body=%s", rec.Code, rec.Body.String())
			}
			if repo.publicCreated {
				t.Fatalf("invalid create should not reach repo, command=%+v", repo.createdPublic)
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(tc.want)) {
				t.Fatalf("validation response should mention %s: %s", tc.want, rec.Body.String())
			}
		})
	}
}

func TestProductSettingsAPIRejectsExplicitZeroDripBagCreate(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "zero grams",
			body: `{"name":"耶加雪菲挂耳","product_kind":"drip_bag","drip_bag_grams":0,"drip_box_bag_count":10}`,
			want: "drip_bag_grams",
		},
		{
			name: "zero box count",
			body: `{"name":"耶加雪菲挂耳","product_kind":"drip_bag","drip_bag_grams":10,"drip_box_bag_count":0}`,
			want: "drip_box_bag_count",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &productSettingsRepo{}
			e := echo.New()
			registerProductRoutes(e, catalogapp.NewService(repo))

			req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products", bytes.NewBufferString(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("POST explicit zero drip product status=%d, want 400 body=%s", rec.Code, rec.Body.String())
			}
			if repo.publicCreated {
				t.Fatalf("invalid drip create should not reach repo, command=%+v", repo.createdPublic)
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(tc.want)) {
				t.Fatalf("invalid drip create response should mention %s: %s", tc.want, rec.Body.String())
			}
		})
	}
}

func TestProductSettingsAPIUpdatesProductYieldRate(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{ID: 7, Name: "曲奇拼配", RoastLevel: "中烘", YieldRate: 0.82}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","yield_rate":0.835}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productUpdated || repo.updated.ProductID != 7 || repo.updated.YieldRate != 0.835 {
		t.Fatalf("update command = %+v updated=%v", repo.updated, repo.productUpdated)
	}
}

func TestProductSettingsAPIRejectsExplicitZeroDripBagUpdate(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "zero grams",
			body: `{"roast_level":"中烘","product_kind":"drip_bag","drip_bag_grams":0,"drip_box_bag_count":10}`,
			want: "drip_bag_grams",
		},
		{
			name: "zero box count",
			body: `{"roast_level":"中烘","product_kind":"drip_bag","drip_bag_grams":10,"drip_box_bag_count":0}`,
			want: "drip_box_bag_count",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &productSettingsRepo{
				products: []catalogapp.Product{{
					ID:                    7,
					Name:                  "耶加雪菲挂耳",
					RoastLevel:            "中烘",
					ProductKind:           "drip_bag",
					DripBagGrams:          10,
					DripBoxBagCount:       10,
					AllowFulfillmentOrder: true,
					AllowMallOrder:        true,
					YieldRate:             0.82,
				}},
			}
			e := echo.New()
			registerProductRoutes(e, catalogapp.NewService(repo))

			req := httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("PUT explicit zero drip product status=%d, want 400 body=%s", rec.Code, rec.Body.String())
			}
			if repo.productUpdated {
				t.Fatalf("invalid drip update should not reach repo, command=%+v", repo.updated)
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(tc.want)) {
				t.Fatalf("invalid drip update response should mention %s: %s", tc.want, rec.Body.String())
			}
		})
	}
}

func TestProductSettingsAPIRejectsInvalidDripBagUpdate(t *testing.T) {
	repo := &productSettingsRepo{
		products: []catalogapp.Product{{
			ID:                    7,
			Name:                  "耶加雪菲挂耳",
			RoastLevel:            "中烘",
			ProductKind:           "drip_bag",
			DripBagGrams:          10,
			DripBoxBagCount:       10,
			AllowFulfillmentOrder: true,
			AllowMallOrder:        true,
			YieldRate:             0.82,
		}},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","product_kind":"drip_bag","drip_bag_grams":-1,"drip_box_bag_count":10}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT invalid drip product status=%d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	if repo.productUpdated {
		t.Fatalf("invalid drip update should not reach repo, command=%+v", repo.updated)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("drip_bag_grams")) {
		t.Fatalf("invalid drip response should mention drip_bag_grams: %s", rec.Body.String())
	}
}

func TestProductSettingsAPIReturnsInternalErrorForProductUpdatePersistenceFailure(t *testing.T) {
	repo := &productSettingsRepo{
		products:  []catalogapp.Product{{ID: 7, Name: "曲奇拼配", RoastLevel: "中烘", YieldRate: 0.82}},
		updateErr: errors.New("database unavailable"),
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","yield_rate":0.835}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("PUT product persistence failure status=%d, want 500 body=%s", rec.Code, rec.Body.String())
	}
	if repo.productUpdated {
		t.Fatalf("failed persistence should not mark product updated, command=%+v", repo.updated)
	}
}

func TestProductSettingsAPISavesAndReturnsProductMarginOverride(t *testing.T) {
	margin := 0.235
	product := catalogapp.Product{ID: 7, Name: "曲奇拼配", RoastLevel: "中烘", YieldRate: 0.82}
	setFloat64PtrField(t, &product, "MarginRateOverride", margin)
	repo := &productSettingsRepo{
		products: []catalogapp.Product{product},
		categories: []catalogapp.ProductCategory{
			{ID: 1, Name: "咖啡豆", Level: 1, Position: 1},
			{ID: 2, ParentID: 1, Name: "意式拼配", Level: 2, Position: 1, GradientTemplateID: 9},
		},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"margin_rate_override":0.235`)) {
		t.Fatalf("product settings response missing product margin override: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","yield_rate":0.82,"margin_rate_override":0.31}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := float64PtrField(t, repo.updated, "MarginRateOverride")
	if got == nil || *got != 0.31 {
		t.Fatalf("margin override command = %+v, got %v", repo.updated, got)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/products/7", bytes.NewBufferString(`{"roast_level":"中烘","yield_rate":0.82,"margin_rate_override":null}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product clear status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := float64PtrField(t, repo.updated, "MarginRateOverride"); got != nil {
		t.Fatalf("margin override should clear to nil, got %v in %+v", *got, repo.updated)
	}
}

func TestProductSettingsAPIDeactivatesMultipleProducts(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/product-settings/products/deactivate", bytes.NewBufferString(`{"product_ids":[7,8]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST deactivate products status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.productsDeactivated || len(repo.deactivated.ProductIDs) != 2 || repo.deactivated.ProductIDs[0] != 7 || repo.deactivated.ProductIDs[1] != 8 {
		t.Fatalf("deactivate command = %+v deactivated=%v", repo.deactivated, repo.productsDeactivated)
	}
}

func TestProductSettingsAPIReturnsEmptyArraysForEmptyCategories(t *testing.T) {
	repo := &productSettingsRepo{
		categories: []catalogapp.ProductCategory{
			{ID: 1, Name: "咖啡豆", Level: 1, Position: 1},
			{ID: 2, ParentID: 1, Name: "意式拼配", Level: 2, Position: 1},
			{ID: 3, Name: "挂耳", Level: 1, Position: 2},
		},
	}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/product-settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/product-settings status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Categories []struct {
			Name     string `json:"name"`
			Children []struct {
				Name     string `json:"name"`
				Products []any  `json:"products"`
			} `json:"children"`
			Products []any `json:"products"`
		} `json:"categories"`
		Products []any `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode product settings: %v body=%s", err, rec.Body.String())
	}
	if payload.Categories == nil || payload.Products == nil {
		t.Fatalf("top-level arrays must not be nil: %+v", payload)
	}
	if len(payload.Categories) != 2 {
		t.Fatalf("categories = %+v", payload.Categories)
	}
	if payload.Categories[0].Children == nil || payload.Categories[0].Children[0].Products == nil {
		t.Fatalf("empty category children/products must encode as [] not null: %s", rec.Body.String())
	}
	if payload.Categories[1].Children == nil || payload.Categories[1].Products == nil {
		t.Fatalf("empty root category arrays must encode as [] not null: %s", rec.Body.String())
	}
}

func TestLegacyProductAndCostingRoutesRedirectToProductSettings(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(&productSettingsRepo{}))

	cases := []struct {
		path string
		want string
	}{
		{path: "/products", want: "/vue-shell?view=productSettings"},
		{path: "/products/7", want: "/vue-shell?view=productSettings&edit_id=7"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		loc, err := url.Parse(rec.Header().Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		base, err := url.Parse("https://example.test" + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusFound || base.ResolveReference(loc).RequestURI() != tc.want {
			t.Fatalf("GET %s status=%d location=%q want %s", tc.path, rec.Code, rec.Header().Get("Location"), tc.want)
		}
	}
}

func setFloat64PtrField(t *testing.T, target any, fieldName string, value float64) {
	t.Helper()
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("missing %s field", fieldName)
	}
	if field.Kind() != reflect.Ptr || field.Type().Elem().Kind() != reflect.Float64 {
		t.Fatalf("%s field type = %s, want *float64", fieldName, field.Type())
	}
	field.Set(reflect.ValueOf(&value))
}

func float64PtrField(t *testing.T, target any, fieldName string) *float64 {
	t.Helper()
	field := reflect.ValueOf(target).FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("missing %s field", fieldName)
	}
	if field.Kind() != reflect.Ptr || field.Type().Elem().Kind() != reflect.Float64 {
		t.Fatalf("%s field type = %s, want *float64", fieldName, field.Type())
	}
	if field.IsNil() {
		return nil
	}
	v := field.Elem().Float()
	return &v
}

func f64(v float64) *float64 {
	return &v
}
