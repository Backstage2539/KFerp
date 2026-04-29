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
	published int
	drafted   int
	lastDraft appcosting.PublishBeanListCommand
}

func (s *recordingBeanListService) PublishBeanList(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	s.published++
	return s.fakeService.PublishBeanList(ctx, cmd)
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
