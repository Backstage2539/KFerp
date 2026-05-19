package costing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authzapp "orderapp/internal/application/authz"
	appcosting "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"

	"github.com/labstack/echo/v4"
)

type fakeService struct{}

func (fakeService) Parameters(context.Context) (domain.Parameters, error) {
	return domain.DefaultParameters(), nil
}

func (fakeService) Calculate(ctx context.Context, req appcosting.CalculateRequest) (*appcosting.CalculateResponse, error) {
	items, err := appcosting.NewService(&fakeRepo{}).Calculate(ctx, req)
	return items, err
}

func (fakeService) ExplainPrice(ctx context.Context, req appcosting.PriceExplanationCommand) (*domain.PriceExplanation, error) {
	return appcosting.NewService(&fakeRepo{}).ExplainPrice(ctx, req)
}

func (fakeService) BeanList(context.Context) (*appcosting.CalculateResponse, error) {
	return &appcosting.CalculateResponse{Parameters: domain.DefaultParameters()}, nil
}

func (fakeService) CreateRun(context.Context, string) (*appcosting.Run, error) {
	return &appcosting.Run{ID: 1, Status: "draft"}, nil
}

func (fakeService) PublishRun(context.Context, string, int64) error {
	return nil
}

func (fakeService) Settings(context.Context) ([]appcosting.ParameterSetting, error) {
	return []appcosting.ParameterSetting{{Key: "roast_yield_rate", Label: "生豆到熟豆转化率", Value: 0.8, Unit: "ratio"}}, nil
}

func (fakeService) UpdateSetting(context.Context, appcosting.UpdateParameterCommand) (appcosting.ParameterSetting, error) {
	return appcosting.ParameterSetting{Key: "roast_yield_rate", Label: "生豆到熟豆转化率", Value: 0.81, Unit: "ratio"}, nil
}

func (fakeService) ListDripPriceTemplates(context.Context) ([]domain.DripPriceTemplate, error) {
	return []domain.DripPriceTemplate{fakeDripPriceTemplate()}, nil
}

func (fakeService) SaveDripPriceTemplate(ctx context.Context, cmd appcosting.SaveDripPriceTemplateCommand) (*domain.DripPriceTemplate, error) {
	return appcosting.NewService(&fakeRepo{}).SaveDripPriceTemplate(ctx, cmd)
}

func (fakeService) DeactivateDripPriceTemplate(context.Context, appcosting.DeactivateDripPriceTemplateCommand) error {
	return nil
}

func (fakeService) ExplainDripPrice(ctx context.Context, req appcosting.DripPriceExplanationCommand) (*domain.DripPriceExplanation, error) {
	return appcosting.NewService(&fakeRepo{}).ExplainDripPrice(ctx, req)
}

func fakeDripPriceTemplate() domain.DripPriceTemplate {
	return domain.DripPriceTemplate{
		ID:               5,
		Name:             "默认挂耳供应价",
		Active:           true,
		BagGrams:         10,
		BoxBagCount:      10,
		IncludePackaging: true,
		Tiers: []domain.DripPriceTemplateTier{
			{ID: 51, Label: "100袋", MinBags: 100, Multiplier: 2.2, Position: 1, Active: true},
			{ID: 52, Label: "1000袋", MinBags: 1000, Multiplier: 1.8, Position: 2, Active: true},
		},
	}
}

func (fakeService) ListBeanListPublications(context.Context, appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error) {
	row := fakePublishedBeanListPublication()
	return []appcosting.BeanListPublication{row}, nil
}

func (fakeService) PublishedBeanList(context.Context, appcosting.BeanListPublicationQuery) (*appcosting.BeanListPublication, error) {
	row := fakePublishedBeanListPublication()
	return &row, nil
}

func fakePublishedBeanListPublication() appcosting.BeanListPublication {
	return appcosting.BeanListPublication{
		ID:        7,
		ListType:  "commercial",
		Version:   "V3.0.5",
		Status:    "published",
		OwnerType: "official",
		Config: map[string]any{
			"layoutStyle":     "card",
			"cardsPerRow":     float64(2),
			"brandName":       "棵凡咖啡",
			"showVersion":     true,
			"showChangelog":   true,
			"backgroundColor": "#f8f1e5",
			"fontColor":       "#171717",
		},
		Content: map[string]any{
			"title":      "棵凡咖啡批发豆单",
			"subtitle":   "报价不含税、不含运",
			"totalItems": float64(1),
			"groups": []any{map[string]any{
				"category":     "1、工厂量单",
				"showCategory": true,
				"items": []any{map[string]any{
					"code":           "1.1",
					"name":           "曲奇拼配",
					"recommendedUse": "意式",
					"flavor":         "坚果、焦糖、巧克力曲奇",
					"description":    "V1～最新",
					"prices": []any{map[string]any{
						"label": "24-49kg",
						"price": float64(82),
						"unit":  "kg",
					}},
				}},
			}},
		},
		Changelog:   "V3.0.5 初始发布",
		PublishedAt: "2026-04-27 22:08",
	}
}

func (fakeService) PublishBeanList(context.Context, appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	return &appcosting.BeanListPublication{
		ID:                       8,
		ListType:                 "commercial",
		Version:                  "V3.0.6",
		Status:                   "published",
		OwnerType:                "actor",
		OwnerKey:                 "employee:7",
		PriceSourcePublicationID: 7,
		StyleSourcePublicationID: 6,
	}, nil
}

func (fakeService) SaveBeanListDraft(context.Context, appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	return &appcosting.BeanListPublication{
		ID:        9,
		ListType:  "commercial",
		Version:   "V3.0.6",
		Status:    "draft",
		OwnerType: "actor",
		OwnerKey:  "employee:7",
	}, nil
}

func (fakeService) WithdrawBeanList(context.Context, appcosting.WithdrawBeanListCommand) error {
	return nil
}

type recordingBeanListService struct {
	fakeService
	published   int
	drafted     int
	lastQuery   appcosting.BeanListPublicationQuery
	lastPublish appcosting.PublishBeanListCommand
	lastDraft   appcosting.PublishBeanListCommand
}

func (s *recordingBeanListService) ListBeanListPublications(ctx context.Context, query appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error) {
	s.lastQuery = query
	return s.fakeService.ListBeanListPublications(ctx, query)
}

func (s *recordingBeanListService) PublishBeanList(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	s.published++
	s.lastPublish = cmd
	return &appcosting.BeanListPublication{
		ID:        8,
		ListType:  cmd.ListType,
		Version:   cmd.Version,
		Status:    "published",
		OwnerType: cmd.OwnerType,
		OwnerKey:  cmd.OwnerKey,
	}, nil
}

func (s *recordingBeanListService) SaveBeanListDraft(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	s.drafted++
	s.lastDraft = cmd
	return &appcosting.BeanListPublication{
		ID:        9,
		ListType:  cmd.ListType,
		Version:   cmd.Version,
		Status:    "draft",
		OwnerType: cmd.OwnerType,
		OwnerKey:  cmd.OwnerKey,
	}, nil
}

type fakeCostingAuthz struct {
	actor authzapp.Actor
}

func (f *fakeCostingAuthz) ActorByEmployeeID(ctx context.Context, employeeID int64) (authzapp.Actor, error) {
	actor := f.actor
	actor.EmployeeID = employeeID
	return actor, nil
}

func (f *fakeCostingAuthz) ListRoles(context.Context) ([]authzapp.Role, error) {
	return nil, nil
}

func (f *fakeCostingAuthz) ListEmployeeRoles(context.Context) (map[int64][]string, error) {
	return nil, nil
}

func (f *fakeCostingAuthz) AssignEmployeeRoles(context.Context, authzapp.AssignmentCommand) error {
	return nil
}

type fakeRepo struct{}

func (fakeRepo) LoadParameters(context.Context) (domain.Parameters, error) {
	return domain.DefaultParameters(), nil
}

func (fakeRepo) LoadProductInputs(context.Context, domain.Parameters) ([]domain.ProductInput, error) {
	return nil, nil
}

func (fakeRepo) CreateRun(context.Context, string, []domain.ProductResult) (*appcosting.Run, error) {
	return nil, nil
}

func (fakeRepo) PublishRun(context.Context, string, int64) error {
	return nil
}

func (fakeRepo) ListParameterSettings(context.Context) ([]appcosting.ParameterSetting, error) {
	return nil, nil
}

func (fakeRepo) UpdateParameterSetting(context.Context, appcosting.UpdateParameterCommand) (appcosting.ParameterSetting, error) {
	return appcosting.ParameterSetting{}, nil
}

func (fakeRepo) ListDripPriceTemplates(context.Context) ([]domain.DripPriceTemplate, error) {
	return []domain.DripPriceTemplate{fakeDripPriceTemplate()}, nil
}

func (fakeRepo) SaveDripPriceTemplate(context.Context, appcosting.SaveDripPriceTemplateCommand) (*domain.DripPriceTemplate, error) {
	template := fakeDripPriceTemplate()
	return &template, nil
}

func (fakeRepo) DeactivateDripPriceTemplate(context.Context, appcosting.DeactivateDripPriceTemplateCommand) error {
	return nil
}

func (fakeRepo) ListBeanListPublications(context.Context, appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error) {
	return nil, nil
}

func (fakeRepo) PublishedBeanList(context.Context, appcosting.BeanListPublicationQuery) (*appcosting.BeanListPublication, error) {
	return nil, nil
}

func (fakeRepo) PublishBeanList(context.Context, appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	return nil, nil
}

func (fakeRepo) SaveBeanListDraft(context.Context, appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	return nil, nil
}

func (fakeRepo) WithdrawBeanList(context.Context, appcosting.WithdrawBeanListCommand) error {
	return nil
}

func TestCostingCalculateAPI(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{{
		ProductID:          1,
		Name:               "金色山脉",
		GreenBeanCostPerKg: 62,
		YieldRate:          0.8,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/calculate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got appcosting.CalculateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "金色山脉" {
		t.Fatalf("items = %+v", got.Items)
	}
}

func TestCostingPriceExplanationAPIIncludesFastCostParameters(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	body, err := json.Marshal(appcosting.PriceExplanationCommand{
		Product: domain.ProductInput{
			ProductID:          501,
			Name:               "模板拼配",
			GreenBeanCostPerKg: 51.75,
			YieldRate:          0.8,
			GradientTemplate: &domain.GradientTemplate{
				ID:          9,
				Name:        "工厂量单模板",
				DisplayUnit: domain.GradientDisplayUnitKg,
				Tiers: []domain.GradientTemplateTier{{
					ID: 91, Label: "大客户量单", MinWeightG: 24000, MaxWeightG: floatPtr(49000), MarginRate: 0.175, Position: 1,
				}},
			},
		},
		TierLabel: "大客户量单",
		Overrides: domain.PriceExplanationOverrides{
			MarginRate: floatPtr(0.30),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/price-explanation", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got domain.PriceExplanation
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got.TemplateName != "工厂量单模板" || got.TierLabel != "大客户量单" || got.SavedFinalPrice != 82 || got.PreviewFinalPrice != 91 {
		t.Fatalf("explanation = %+v", got)
	}
	for _, want := range []string{`"key":"large_batch_production_cost_per_kg"`, `"key":"wholesale_package_cost_per_kg"`, `"key":"product_loss_per_kg"`, `"key":"retail_tax_rate"`, `"key":"template_margin_rate"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestCostingCalculateAPIReturnsExcelTierSchemeMetadata(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{{
		ProductID:           1,
		Name:                "白月光-瑰夏",
		GreenBeanCostPerKg:  362.5,
		YieldRate:           0.8,
		WholesaleTierScheme: domain.WholesaleTierScheme227GTwo,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/calculate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got appcosting.CalculateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(got.Items) != 1 || len(got.Items[0].CommercialWholesaleTiers) != 2 {
		t.Fatalf("items = %+v", got.Items)
	}
	tier := got.Items[0].CommercialWholesaleTiers[0]
	if tier.SpecG != 227 || tier.Label != "2包-7包" || tier.PricePerUnit <= 0 {
		t.Fatalf("tier = %+v", tier)
	}
}

func TestCostingCalculateAPIRoundsExcelBeanListPrices(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{{
		ProductID:          1,
		Name:               "白月光-瑰夏",
		GreenBeanCostPerKg: 360,
		YieldRate:          0.8,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/calculate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got appcosting.CalculateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	item := got.Items[0]
	if item.CommercialWholesaleTiers[0].PricePerUnit != 190 || item.CommercialWholesaleTiers[1].PricePerUnit != 165 {
		t.Fatalf("commercial tiers = %+v", item.CommercialWholesaleTiers)
	}
	if len(item.RetailBeanTiers) != 2 || item.RetailBeanTiers[0].Label != "100g" || item.RetailBeanTiers[0].PricePerUnit != 115 {
		t.Fatalf("retail tiers = %+v", item.RetailBeanTiers)
	}
}

func TestCostingCalculateAPIReturnsExcelBeanListDisplayMetadata(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{{
		ProductID:          1,
		Name:               "Uraga乌拉嘎",
		GreenBeanCostPerKg: 108,
		YieldRate:          0.8,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/calculate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Items []struct {
			CommercialBeanList struct {
				Code           string `json:"code"`
				Category       string `json:"category"`
				RecommendedUse string `json:"recommended_use"`
				Flavor         string `json:"flavor"`
				Description    string `json:"description"`
			} `json:"commercial_bean_list"`
			RetailBeanList struct {
				Code     string `json:"code"`
				Category string `json:"category"`
			} `json:"retail_bean_list"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	item := got.Items[0]
	if item.CommercialBeanList.Code != "5.2" || item.CommercialBeanList.Category != "5、原产地精选豆：" {
		t.Fatalf("commercial bean list = %+v", item.CommercialBeanList)
	}
	if item.CommercialBeanList.RecommendedUse != "手冲/SOE/冷萃" {
		t.Fatalf("commercial recommended use = %q", item.CommercialBeanList.RecommendedUse)
	}
	if item.CommercialBeanList.Flavor == "" || item.CommercialBeanList.Description == "" {
		t.Fatalf("commercial bean list missing flavor/description: %+v", item.CommercialBeanList)
	}
	if item.RetailBeanList.Code != "3.2" || item.RetailBeanList.Category != "3、原产地精选豆：" {
		t.Fatalf("retail bean list = %+v", item.RetailBeanList)
	}
}

func TestRoutesAreRegistered(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})
	seen := map[string]bool{}
	for _, route := range e.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /api/costing/parameters",
		"GET /api/costing/settings",
		"POST /api/costing/settings/:key",
		"POST /api/costing/calculate",
		"GET /api/drip-price-templates",
		"POST /api/drip-price-templates",
		"PUT /api/drip-price-templates/:id",
		"POST /api/drip-price-templates/:id/deactivate",
		"POST /api/costing/drip-price-explanation",
		"GET /api/costing/bean-list",
		"GET /api/costing/bean-list/publications",
		"POST /api/costing/bean-list/publications",
		"POST /api/costing/bean-list/publications/:id/withdraw",
		"POST /api/costing/bean-list/drafts",
		"GET /public/bean-list/:list_type",
		"POST /api/costing/runs",
		"POST /api/costing/runs/:id/publish",
	} {
		if !seen[want] {
			t.Fatalf("missing route %s; got %+v", want, seen)
		}
	}
}

func TestDripPriceTemplateAPIValidatesBagAndBoxConfig(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "bag grams", body: `{"name":"坏模板","bag_grams":0,"box_bag_count":10,"tiers":[{"label":"100袋","min_bags":100,"multiplier":2.2}]}`, want: "bag_grams"},
		{name: "box count", body: `{"name":"坏模板","bag_grams":10,"box_bag_count":0,"tiers":[{"label":"100袋","min_bags":100,"multiplier":2.2}]}`, want: "box_bag_count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/drip-price-templates", bytes.NewBufferString(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("body %s missing %s", rec.Body.String(), tc.want)
			}
		})
	}
}

func TestDripPriceTemplateDeactivateAPIKeepsPublishedTiersByOnlyDeactivatingTemplate(t *testing.T) {
	svc := &recordingDripTemplateService{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: svc})

	req := httptest.NewRequest(http.MethodPost, "/api/drip-price-templates/5/deactivate", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.deactivatedID != 5 {
		t.Fatalf("deactivated id = %d", svc.deactivatedID)
	}
	if svc.publishRunCalls != 0 {
		t.Fatalf("deactivate must not republish or delete product price tiers, publish calls = %d", svc.publishRunCalls)
	}
}

type recordingDripTemplateService struct {
	fakeService
	deactivatedID   int64
	publishRunCalls int
}

func (s *recordingDripTemplateService) DeactivateDripPriceTemplate(ctx context.Context, cmd appcosting.DeactivateDripPriceTemplateCommand) error {
	s.deactivatedID = cmd.ID
	return nil
}

func (s *recordingDripTemplateService) PublishRun(ctx context.Context, actor string, runID int64) error {
	s.publishRunCalls++
	return nil
}

func TestCostingDripPriceExplanationAPIIncludesCostFormulaAndBoxConversion(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	template := fakeDripPriceTemplate()
	body, err := json.Marshal(appcosting.DripPriceExplanationCommand{
		Product: domain.ProductInput{
			ProductID:          701,
			Name:               "耶加雪菲挂耳",
			ProductKind:        "drip_bag",
			DripBagGrams:       10,
			DripBoxBagCount:    10,
			GreenBeanCostPerKg: 60,
			YieldRate:          0.8,
			DripPriceTemplate:  &template,
		},
		TierLabel: "100袋",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/drip-price-explanation", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	bodyText := rec.Body.String()
	for _, want := range []string{
		`"key":"roasted_bean_cost_per_kg"`,
		`"key":"bag_grams"`,
		`"key":"drip_process_cost_per_bag"`,
		`"key":"drip_extra_cost_per_bag"`,
		`"key":"template_multiplier"`,
		`"key":"retail_tax_rate"`,
		`"key":"packed_price_per_bag"`,
		`"key":"box_conversion"`,
		`"box_bag_count":10`,
	} {
		if !strings.Contains(bodyText, want) {
			t.Fatalf("drip explanation missing %s: %s", want, bodyText)
		}
	}
}

func TestPublicBeanListPageRendersPublishedSnapshot(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	req := httptest.NewRequest(http.MethodGet, "/public/bean-list/commercial", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<title>棵凡咖啡批发豆单 V3.0.5</title>",
		"棵凡咖啡批发豆单",
		"报价不含税、不含运",
		"V3.0.5",
		"1、工厂量单",
		"1.1",
		"曲奇拼配",
		"出品建议",
		"坚果、焦糖、巧克力曲奇",
		"24-49kg",
		"82/kg",
		"V3.0.5 初始发布",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("public bean list page missing %q in body: %s", want, body)
		}
	}
	for _, forbidden := range []string{"发布豆单", "撤回发布", "/api/costing/bean-list/publications"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public bean list page leaked admin content %q in body: %s", forbidden, body)
		}
	}
}

func TestPublicGreenBeanListPageRendersQualitySnapshot(t *testing.T) {
	page, err := renderPublicBeanListPage(appcosting.BeanListPublication{
		ID:        12,
		ListType:  "green",
		Version:   "V3.0.6",
		Status:    "published",
		OwnerType: "official",
		Config: map[string]any{
			"brandName":   "棵凡咖啡",
			"layoutStyle": "table",
		},
		Content: map[string]any{
			"groups": []any{map[string]any{
				"category":     "G、生豆销售",
				"showCategory": true,
				"items": []any{map[string]any{
					"code":        "G.1",
					"name":        "埃塞瑰夏生豆",
					"flavor":      "茉莉、柑橘",
					"description": "来自绑定熟豆 BOM 的阶梯模板报价",
					"beanListQuality": map[string]any{
						"factoryFlavorDescription": "茉莉花、柑橘",
						"moisture":                 "10.8%",
						"density":                  "780g/L",
						"inspectionCreatedAt":      "2026-05-18 09:30",
					},
					"prices": []any{map[string]any{
						"label": "1kg+",
						"price": float64(128),
						"unit":  "kg",
					}},
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"棵凡咖啡生豆豆单",
		"生豆销售报价",
		"生豆",
		"工厂风味",
		"茉莉花、柑橘",
		"水分",
		"10.8%",
		"密度",
		"780g/L",
		"质检时间",
		"2026-05-18 09:30",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("green public bean list page missing %q in body: %s", want, page)
		}
	}
}

func TestCostingSettingsAPI(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	body := bytes.NewBufferString(`{"value":0.81}`)
	req = httptest.NewRequest(http.MethodPost, "/api/costing/settings/roast_yield_rate", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got appcosting.ParameterSetting
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Key != "roast_yield_rate" || got.Value != 0.81 {
		t.Fatalf("setting = %+v", got)
	}
}

type costingSettingsRepo struct {
	fakeRepo
}

func (costingSettingsRepo) ListParameterSettings(context.Context) ([]appcosting.ParameterSetting, error) {
	return []appcosting.ParameterSetting{
		{Key: "roast_yield_rate", Label: "生豆到熟豆转化率", Value: 0.8, Unit: "ratio"},
		{Key: "kg_to_lb_factor", Label: "kg 到 lb 换算", Value: 0.454, Unit: "lb/kg"},
		{Key: "retail_bean_margin_rate", Label: "零售熟豆利润系数", Value: 0.6, Unit: "ratio"},
		{Key: "retail_tax_rate", Label: "零售税率", Value: 0.03, Unit: "ratio"},
		{Key: "retail_drip_multiplier", Label: "零售挂耳利润系数", Value: 2.5, Unit: "ratio"},
		{Key: "wholesale_kg_margin_rate_1", Label: "商用熟豆 2包-13包 利润系数", Value: 0.42, Unit: "ratio"},
		{Key: "wholesale_drip_multiplier_1", Label: "商用挂耳 100包 利润系数", Value: 2.2, Unit: "ratio"},
	}, nil
}

func (costingSettingsRepo) UpdateParameterSetting(_ context.Context, cmd appcosting.UpdateParameterCommand) (appcosting.ParameterSetting, error) {
	return appcosting.ParameterSetting{Key: cmd.Key, Label: "kg 到 lb 换算", Value: cmd.Value, Unit: "lb/kg"}, nil
}

func TestCostingSettingsAPIFiltersDeprecatedQuickSettings(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(costingSettingsRepo{})})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/settings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Rows []appcosting.ParameterSetting `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	for _, removed := range []string{"roast_yield_rate", "retail_bean_margin_rate", "retail_drip_multiplier", "wholesale_kg_margin_rate_1", "wholesale_drip_multiplier_1"} {
		if strings.Contains(body, removed) {
			t.Fatalf("settings response exposed deprecated quick setting %q: %s", removed, body)
		}
	}
	if len(listed.Rows) != 2 {
		t.Fatalf("rows = %+v", listed.Rows)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/costing/settings/roast_yield_rate", bytes.NewBufferString(`{"value":0.81}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("deprecated update status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/costing/settings/kg_to_lb_factor", bytes.NewBufferString(`{"value":0.454}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("editable update status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestBeanListPublicationAPI(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/bean-list/publications?list_type=commercial", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Rows []appcosting.BeanListPublication `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Rows) != 1 || listed.Rows[0].Version != "V3.0.5" || listed.Rows[0].Config["layoutStyle"] != "card" {
		t.Fatalf("publications = %+v", listed.Rows)
	}

	body := bytes.NewBufferString(`{"list_type":"commercial","version":"V3.0.6","scope":"mine","price_source_publication_id":7,"style_source_publication_id":6,"config":{"layoutStyle":"table"},"content":{"totalItems":25},"changelog":"补充标签和筛选"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var published appcosting.BeanListPublication
	if err := json.Unmarshal(rec.Body.Bytes(), &published); err != nil {
		t.Fatal(err)
	}
	if published.ID != 8 || published.Version != "V3.0.6" || published.Status != "published" {
		t.Fatalf("published = %+v", published)
	}
	if published.OwnerType != "actor" || published.OwnerKey == "" || published.PriceSourcePublicationID != 7 || published.StyleSourcePublicationID != 6 {
		t.Fatalf("published owner/source = %+v", published)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications/8/withdraw", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("withdraw status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestBeanListPublicationAPISupportsCustomerScope(t *testing.T) {
	svc := &recordingBeanListService{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{Costing: svc})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/bean-list/publications?list_type=commercial&scope=customer&customer_id=42", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.lastQuery.OwnerType != "customer" || svc.lastQuery.OwnerKey != "42" {
		t.Fatalf("customer query owner = %+v", svc.lastQuery)
	}

	body := bytes.NewBufferString(`{"list_type":"commercial","version":"V3.0.8","scope":"customer","customer_id":42,"config":{"layoutStyle":"card"},"content":{"totalItems":1},"changelog":"客户 A 豆单"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.lastPublish.OwnerType != "customer" || svc.lastPublish.OwnerKey != "42" {
		t.Fatalf("customer publish owner = %+v", svc.lastPublish)
	}
}

func TestBeanListPublicationAPIRejectsUnknownScope(t *testing.T) {
	svc := &recordingBeanListService{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{Costing: svc})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/bean-list/publications?list_type=commercial&scope=fulfillment_customers", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.lastQuery.ListType != "" {
		t.Fatalf("unknown scope should not reach service, query = %+v", svc.lastQuery)
	}
}

func TestBeanListPublicationPublishRequiresAdmin(t *testing.T) {
	svc := &recordingBeanListService{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{
		Costing: svc,
		Authz: &fakeCostingAuthz{actor: authzapp.Actor{
			Name:        "客户",
			Permissions: []string{"costing.read", "costing.write"},
		}},
	})

	body := bytes.NewBufferString(`{"list_type":"commercial","version":"V3.0.7","scope":"mine","config":{},"content":{"totalItems":1},"changelog":"客户修改"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.published != 0 {
		t.Fatalf("non-admin publish should not call service, calls=%d", svc.published)
	}
}

func TestBeanListDraftAPISavesCustomerOwnedDraft(t *testing.T) {
	svc := &recordingBeanListService{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{
		Costing: svc,
		Authz: &fakeCostingAuthz{actor: authzapp.Actor{
			Name:        "客户",
			Permissions: []string{"costing.read"},
		}},
	})

	body := bytes.NewBufferString(`{"list_type":"commercial","version":"V3.0.7","scope":"official","config":{"layoutStyle":"card"},"content":{"totalItems":1},"changelog":"客户修改"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/drafts", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.drafted != 1 {
		t.Fatalf("draft calls = %d", svc.drafted)
	}
	if svc.lastDraft.OwnerType != "actor" || svc.lastDraft.OwnerKey != "employee:7" {
		t.Fatalf("customer draft owner = %+v", svc.lastDraft)
	}
	var row appcosting.BeanListPublication
	if err := json.Unmarshal(rec.Body.Bytes(), &row); err != nil {
		t.Fatal(err)
	}
	if row.Status != "draft" || row.OwnerType != "actor" || row.OwnerKey != "employee:7" {
		t.Fatalf("draft row = %+v", row)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
