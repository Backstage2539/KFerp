package costing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type fakePricingRuleTrialErrorService struct {
	fakeService
}

type capturingPricingRuleTrialService struct {
	fakeService
	last appcosting.PricingRuleTrialCommand
}

type capturingPricingRuleTrialBatchService struct {
	fakeService
	calls int
	last  []appcosting.PricingRuleTrialCommand
}

func containsWarning(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

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

func (fakeService) PricingRuleTrial(context.Context, appcosting.PricingRuleTrialCommand) (*appcosting.PricingRuleTrialResult, error) {
	return &appcosting.PricingRuleTrialResult{
		PricingRuleID:      10,
		PricingRuleName:    "PR452 加价含税",
		FormulaVersion:     "v2",
		ProductID:          549,
		ProductName:        "PR452 试算商品",
		QuoteUnit:          "kg",
		InventoryUnit:      "kg",
		BomVersionID:       3315,
		BomVersionNo:       "BOM-v1",
		BomUsageMode:       "production_bom_output",
		BomStatus:          "active",
		BaseCost:           60,
		BomCostTotal:       50,
		OperationCostTotal: 10,
		BaseCostDetails: []appcosting.PricingRuleTrialBaseCostDetail{
			{Key: "material:1", Type: "material", TypeLabel: "物料", Name: "拼配熟豆原料", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 50, Amount: 50, Unit: "kg", Description: "物料成本 50/kg"},
			{Key: "operation:7:1", Type: "operation", TypeLabel: "工序", Name: "烘焙", ConsumeUnit: "per_kg", UnitCost: 10, Amount: 10, Unit: "kg", Description: "工序成本 10/kg"},
		},
		OtherCostDetails: []appcosting.PricingRuleTrialOtherCostDetail{
			{Name: "包装贴标", Amount: 2.5, Unit: "kg", Source: "pricing_rule", SettingLocation: "价格计算模板编辑区「其他成本」"},
		},
		OtherCostTotal:      2.5,
		CostBaseTotal:       62.5,
		CostAfterYield:      78.13,
		YieldLossAmount:     15.63,
		PriceAfterMarkup:    97.66,
		ProfitMarkupAmount:  19.53,
		PreTaxPrice:         97.66,
		TaxAmount:           5.86,
		TaxInPriceAmount:    5.86,
		FinalBeforeRounding: 103.52,
		RoundingAdjustment:  -0.02,
		FinalUnitPrice:      103.5,
		GrossMarginRate:     0.2,
		MinimumMarginRate:   0.18,
		ProfitExplanation: appcosting.PricingRuleTrialProfitExplanation{
			Method:         "markup",
			MethodLabel:    "加价率",
			Rate:           0.25,
			Source:         "pricing_rule",
			CostAfterYield: 78.13,
			MarkupAmount:   19.53,
			PreTaxPrice:    97.66,
			Formula:        "税前价 = 损耗后成本 * (1 + 加价率 25%)",
		},
		FormulaExpression: "最终售价 = (标准制造成本 60/kg + 其他成本 2.5/kg) / (1 - 损耗率 20%) * (1 + 加价率 25%) * (1 + 税率 6%) = 103.5/kg",
		FormulaExpressionLines: []string{
			"成本基数 = 标准制造成本 60/kg + 其他成本 2.5/kg = 62.5/kg",
			"最终售价 = 103.5/kg",
		},
		Steps: []domain.PriceExplanationStep{
			{Key: "final_unit_price", Label: "试算单价", Value: 103.5, Unit: "kg"},
		},
	}, nil
}

func (fakeService) PricingRuleTrialBatch(ctx context.Context, requests []appcosting.PricingRuleTrialCommand) ([]appcosting.PricingRuleTrialBatchRow, error) {
	rows := make([]appcosting.PricingRuleTrialBatchRow, len(requests))
	for i, request := range requests {
		rows[i].Index = i
		result, err := (fakeService{}).PricingRuleTrial(ctx, request)
		if err != nil {
			rows[i].Error = err.Error()
			continue
		}
		rows[i].Result = result
	}
	return rows, nil
}

func (fakePricingRuleTrialErrorService) PricingRuleTrial(context.Context, appcosting.PricingRuleTrialCommand) (*appcosting.PricingRuleTrialResult, error) {
	return nil, errors.New("product not found")
}

func (s *capturingPricingRuleTrialService) PricingRuleTrial(_ context.Context, cmd appcosting.PricingRuleTrialCommand) (*appcosting.PricingRuleTrialResult, error) {
	s.last = cmd
	return &appcosting.PricingRuleTrialResult{
		PricingRuleID:         cmd.PricingRuleID,
		PricingRuleName:       "PR453 Excel 供应售价",
		FormulaVersion:        "excel-202604-v3",
		ProductID:             cmd.ProductID,
		ProductName:           "测试用",
		QuoteUnit:             cmd.QuoteUnit,
		InventoryUnit:         "kg",
		BomVersionID:          cmd.BomVersionID,
		BomVersionNo:          "V002",
		ProcessRouteID:        cmd.ProcessRouteID,
		ProcessRouteName:      "新版工艺路线",
		OperationTemplateID:   cmd.OperationTemplateID,
		OperationTemplateName: "新版工序",
		BaseCost:              67.5,
		BomCostTotal:          67.5,
		OperationCostTotal:    0,
		BaseCostDetails:       pricingRuleTrialAPIFakeBaseCostDetails(),
		OtherCostDetails: []appcosting.PricingRuleTrialOtherCostDetail{
			{Name: "包装贴标", Amount: 1.25, Unit: "kg", Source: "temporary_override", SettingLocation: "本次试算抽屉「其他成本」"},
		},
		OtherCostTotal:      6.2625,
		CostBaseTotal:       73.7625,
		CostAfterYield:      73.7625,
		PriceAfterMarkup:    113.7495,
		ProfitMarkupAmount:  39.987,
		PostMarkupCostTotal: 2.9596,
		PreTaxPrice:         116.7092,
		TaxInPriceAmount:    0,
		FinalBeforeRounding: 116.7092,
		FinalUnitPrice:      116.7092,
		GrossMarginRate:     0.3684,
		MinimumMarginRate:   0,
		ProfitExplanation: appcosting.PricingRuleTrialProfitExplanation{
			Method:         "supplier_tier_markup",
			MethodLabel:    "档位加价率",
			Rate:           0.3,
			Source:         "temporary_override",
			CostAfterYield: 73.7625,
			MarkupAmount:   39.987,
			PreTaxPrice:    116.7092,
			Formula:        "加价后价格 = 损耗后成本 * (1 + 档位加价率 30%)",
		},
		FormulaExpression: "最终售价 = (标准制造成本 67.5/kg + 生产项目成本 6.2625/kg) * (1 + 档位加价率 54.21%) = 116.7092/kg",
		FormulaExpressionLines: []string{
			"成本基数 = 标准制造成本 67.5/kg + 生产项目成本 6.2625/kg = 73.7625/kg",
			"最终售价 = 116.7092/kg",
		},
		BomVersionOptions: []appcosting.PricingRuleTrialBomVersionOption{
			{BomID: 539, BomCode: "BOM-000539", BomName: "PR439-20260606182321 工厂量单商品 生产 BOM", VersionID: 5391, VersionNo: "V001", Status: "published", IsDefault: false, ProcessRouteID: 7, ProcessRouteName: "旧工艺路线"},
			{BomID: 539, BomCode: "BOM-000539", BomName: "PR439-20260606182321 工厂量单商品 生产 BOM", VersionID: 5392, VersionNo: "V002", Status: "published", IsDefault: true, ProcessRouteID: 19, ProcessRouteName: "新版工艺路线"},
		},
		ProcessRouteOptions: []appcosting.PricingRuleTrialProcessRouteOption{
			{ID: 7, Name: "旧工艺路线", IsDefault: false},
			{ID: 19, Name: "新版工艺路线", IsDefault: true},
		},
		OperationTemplateOptions: []appcosting.PricingRuleTrialOperationTemplateOption{
			{ID: 7, Name: "旧工序", IsDefault: false},
			{ID: 9, Name: "新版工序", IsDefault: true},
		},
		Steps: []domain.PriceExplanationStep{
			{Key: "price_after_markup", Label: "加价后价格", Value: 113.7495, Unit: "kg"},
			{Key: "post_markup_cost_total", Label: "加价附加成本", Value: 2.9596, Unit: "kg"},
			{Key: "final_unit_price", Label: "试算单价", Value: 116.7092, Unit: "kg"},
		},
	}, nil
}

func (s *capturingPricingRuleTrialBatchService) PricingRuleTrialBatch(_ context.Context, requests []appcosting.PricingRuleTrialCommand) ([]appcosting.PricingRuleTrialBatchRow, error) {
	s.calls++
	s.last = append([]appcosting.PricingRuleTrialCommand(nil), requests...)
	return []appcosting.PricingRuleTrialBatchRow{
		{Index: 0, Result: &appcosting.PricingRuleTrialResult{ProductID: requests[0].ProductID, FinalUnitPrice: 88}},
		{Index: 1, Error: "product not found"},
	}, nil
}

func (fakeService) BeanList(context.Context, appcosting.BeanListQuery) (*appcosting.CalculateResponse, error) {
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

func pricingRuleTrialAPIFakeBaseCostDetails() []appcosting.PricingRuleTrialBaseCostDetail {
	return []appcosting.PricingRuleTrialBaseCostDetail{{
		Key:               "material:1",
		Type:              "material",
		TypeLabel:         "物料",
		Name:              "测试原料",
		ConsumeUnit:       "ratio_pct",
		RatioPct:          12,
		RecipeRatioPct:    10,
		EffectiveRatioPct: 12,
		MaterialLossRate:  0.2,
		UnitCost:          30.62,
		CostUnitCost:      67.5,
		CostUnit:          "kg",
		Amount:            30.62,
		Unit:              "lb",
		Description:       "物料：测试原料，配方比例 10%，原料加耗 20%，计价比例 12%，单位成本 67.5/kg，折算金额 30.62/lb",
	}}
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
		ID:                 7,
		PublicationPurpose: appcosting.BeanListPublicationPurposeFactorySupply,
		ListType:           "commercial",
		Version:            "V3.0.5",
		Status:             "published",
		OwnerType:          "official",
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
			"title":      "棵凡咖啡批发产品价格表",
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

func (fakeService) PublishBeanList(_ context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	return &appcosting.BeanListPublication{
		ID:                       8,
		ListType:                 "commercial",
		Version:                  "V3.0.6",
		Status:                   "published",
		PublicationPurpose:       cmd.PublicationPurpose,
		OwnerType:                "actor",
		OwnerKey:                 "employee:7",
		PriceSourcePublicationID: 7,
		StyleSourcePublicationID: 6,
	}, nil
}

func (fakeService) SaveBeanListDraft(_ context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	return &appcosting.BeanListPublication{
		ID:                 9,
		ListType:           "commercial",
		Version:            "V3.0.6",
		Status:             "draft",
		PublicationPurpose: cmd.PublicationPurpose,
		OwnerType:          "actor",
		OwnerKey:           "employee:7",
	}, nil
}

func (fakeService) WithdrawBeanList(context.Context, appcosting.WithdrawBeanListCommand) error {
	return nil
}

func (fakeService) ArchiveBeanListPublications(context.Context, appcosting.ArchiveBeanListPublicationsCommand) error {
	return nil
}

func (fakeService) UnarchiveBeanListPublications(context.Context, appcosting.ArchiveBeanListPublicationsCommand) error {
	return nil
}

func (fakeService) GenerateBeanListPublicationPDF(context.Context, appcosting.BeanListPublicationPDFCommand, func(appcosting.BeanListPublication) ([]byte, error)) (appcosting.BeanListPublicationPDFFile, error) {
	row := fakePublishedBeanListPublication()
	body := []byte("%PDF-1.4")
	return appcosting.BeanListPublicationPDFFile{
		PublicationID: row.ID,
		ListType:      row.ListType,
		Version:       row.Version,
		ContentType:   "application/pdf",
		CacheKey:      "bean-list:7:V3.0.5",
		Filename:      "bean-list-commercial-V3.0.5.pdf",
		Bytes:         len(body),
		Payload:       body,
	}, nil
}

func (fakeService) LoadBeanListPublicationPDF(context.Context, appcosting.BeanListPublicationPDFCommand) (appcosting.BeanListPublicationPDFFile, error) {
	return appcosting.BeanListPublicationPDFFile{
		PublicationID: 7,
		ListType:      "commercial",
		Version:       "V3.0.5",
		ContentType:   "application/pdf",
		CacheKey:      "bean-list:7:V3.0.5",
		Filename:      "bean-list-commercial-V3.0.5.pdf",
		Bytes:         8,
		Payload:       []byte("%PDF-1.4"),
	}, nil
}

type recordingBeanListService struct {
	fakeService
	published      int
	drafted        int
	archived       int
	unarchived     int
	generatedPDFs  int
	lastQuery      appcosting.BeanListPublicationQuery
	lastBeanList   appcosting.BeanListQuery
	lastPublish    appcosting.PublishBeanListCommand
	lastDraft      appcosting.PublishBeanListCommand
	lastArchive    appcosting.ArchiveBeanListPublicationsCommand
	lastUnarchive  appcosting.ArchiveBeanListPublicationsCommand
	lastPDFCommand appcosting.BeanListPublicationPDFCommand
}

func (s *recordingBeanListService) ListBeanListPublications(ctx context.Context, query appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error) {
	s.lastQuery = query
	return s.fakeService.ListBeanListPublications(ctx, query)
}

func (s *recordingBeanListService) PublishedBeanList(ctx context.Context, query appcosting.BeanListPublicationQuery) (*appcosting.BeanListPublication, error) {
	s.lastQuery = query
	return s.fakeService.PublishedBeanList(ctx, query)
}

func (s *recordingBeanListService) BeanList(ctx context.Context, query appcosting.BeanListQuery) (*appcosting.CalculateResponse, error) {
	s.lastBeanList = query
	return s.fakeService.BeanList(ctx, query)
}

func (s *recordingBeanListService) PublishBeanList(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	s.published++
	s.lastPublish = cmd
	return &appcosting.BeanListPublication{
		ID:                         8,
		ListType:                   cmd.ListType,
		ProductTypeCategoryID:      cmd.ProductTypeCategoryID,
		ProductTypeName:            cmd.ProductTypeName,
		ClassificationTemplateID:   cmd.ClassificationTemplateID,
		ClassificationTemplateName: cmd.ClassificationTemplateName,
		ClassificationCategoryID:   cmd.ClassificationCategoryID,
		ClassificationCategoryName: cmd.ClassificationCategoryName,
		Version:                    cmd.Version,
		Status:                     "published",
		PublicationPurpose:         cmd.PublicationPurpose,
		OwnerType:                  cmd.OwnerType,
		OwnerKey:                   cmd.OwnerKey,
	}, nil
}

func (s *recordingBeanListService) SaveBeanListDraft(ctx context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	s.drafted++
	s.lastDraft = cmd
	return &appcosting.BeanListPublication{
		ID:                         9,
		ListType:                   cmd.ListType,
		ProductTypeCategoryID:      cmd.ProductTypeCategoryID,
		ProductTypeName:            cmd.ProductTypeName,
		ClassificationTemplateID:   cmd.ClassificationTemplateID,
		ClassificationTemplateName: cmd.ClassificationTemplateName,
		ClassificationCategoryID:   cmd.ClassificationCategoryID,
		ClassificationCategoryName: cmd.ClassificationCategoryName,
		Version:                    cmd.Version,
		Status:                     "draft",
		PublicationPurpose:         cmd.PublicationPurpose,
		OwnerType:                  cmd.OwnerType,
		OwnerKey:                   cmd.OwnerKey,
	}, nil
}

func (s *recordingBeanListService) ArchiveBeanListPublications(ctx context.Context, cmd appcosting.ArchiveBeanListPublicationsCommand) error {
	s.archived++
	s.lastArchive = cmd
	return nil
}

func (s *recordingBeanListService) UnarchiveBeanListPublications(ctx context.Context, cmd appcosting.ArchiveBeanListPublicationsCommand) error {
	s.unarchived++
	s.lastUnarchive = cmd
	return nil
}

func (s *recordingBeanListService) GenerateBeanListPublicationPDF(ctx context.Context, cmd appcosting.BeanListPublicationPDFCommand, render func(appcosting.BeanListPublication) ([]byte, error)) (appcosting.BeanListPublicationPDFFile, error) {
	s.generatedPDFs++
	s.lastPDFCommand = cmd
	body := []byte("%PDF-1.4")
	return appcosting.BeanListPublicationPDFFile{
		PublicationID: cmd.PublicationID,
		ListType:      cmd.Query.ListType,
		Version:       "V3.0.6",
		ContentType:   "application/pdf",
		CacheKey:      "bean-list:test",
		Filename:      "bean-list-commercial-V3.0.6.pdf",
		Bytes:         len(body),
		Payload:       body,
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

type skuBeanListRepo struct{ fakeRepo }

func (skuBeanListRepo) LoadProductInputs(context.Context, domain.Parameters) ([]domain.ProductInput, error) {
	params := domain.DefaultParameters()
	return []domain.ProductInput{{
		ProductID:          551,
		SKUID:              551,
		ParentProductID:    550,
		DefaultSKUID:       552,
		SKUName:            "初晓 磅",
		SKUCode:            "SKU-000551",
		SpecLabel:          "磅",
		NetContentQty:      1,
		NetContentUnit:     "lb",
		Name:               "初晓 磅",
		ProductName:        "初晓",
		InventoryUnit:      "kg",
		OrderUnit:          "磅",
		QuoteUnit:          "磅",
		UnitConversionJSON: `{"磅":{"kg":0.45359237}}`,
		GreenBeanCostPerKg: 62,
		YieldRate:          params.RoastYieldRate,
	}}, nil
}

type greenBeanWithoutLegacyTemplateRepo struct{ fakeRepo }

func (greenBeanWithoutLegacyTemplateRepo) LoadProductInputs(context.Context, domain.Parameters) ([]domain.ProductInput, error) {
	return []domain.ProductInput{{
		ProductID:   912,
		Name:        "黄波旁水洗生豆",
		ProductKind: "green_bean",
		BomStatus:   "missing_green_bean_template",
	}}, nil
}

func (fakeRepo) LoadProductPricingRule(context.Context, int64) (appcosting.ProductPricingRule, error) {
	return appcosting.ProductPricingRule{}, appcosting.ErrProductPricingRuleNotFound
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

func (fakeRepo) LoadBeanListPublication(context.Context, appcosting.BeanListPublicationQuery, int64) (*appcosting.BeanListPublication, error) {
	return nil, nil
}

func (fakeRepo) LoadBeanListPublicationAsset(context.Context, int64, string) (appcosting.BeanListPublicationAsset, error) {
	return appcosting.BeanListPublicationAsset{}, appcosting.ErrBeanListPublicationNotFound
}

func (fakeRepo) SaveBeanListPublicationAsset(context.Context, appcosting.BeanListPublicationAsset, string) (appcosting.BeanListPublicationAsset, error) {
	return appcosting.BeanListPublicationAsset{}, nil
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

func (fakeRepo) ArchiveBeanListPublications(context.Context, appcosting.ArchiveBeanListPublicationsCommand) error {
	return nil
}

func (fakeRepo) UnarchiveBeanListPublications(context.Context, appcosting.ArchiveBeanListPublicationsCommand) error {
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

func TestBeanListAPIReturnsEmptyItemsWhenCatalogHasNoProducts(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(fakeRepo{})})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/bean-list", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body appcosting.CalculateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 0 {
		t.Fatalf("items = %+v, want empty list", body.Items)
	}
	if strings.Contains(rec.Body.String(), "products required") {
		t.Fatalf("response must not expose products required: %s", rec.Body.String())
	}
}

func TestBeanListAPIGreenBeanWithoutLegacyTemplateDoesNotReportMissingTemplateStatus(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(greenBeanWithoutLegacyTemplateRepo{})})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/bean-list", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body appcosting.CalculateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %+v, want one green bean", body.Items)
	}
	if body.Items[0].BomStatus != "" || strings.Contains(rec.Body.String(), "missing_green_bean_template") {
		t.Fatalf("response must not report the removed legacy-template restriction: %s", rec.Body.String())
	}
}

func TestBeanListAPIReturnsConcreteSKUSalesSpecMetadata(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(skuBeanListRepo{})})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/bean-list", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %#v", body["items"])
	}
	row, _ := items[0].(map[string]any)
	if row["sku_id"] != float64(551) || row["parent_product_id"] != float64(550) {
		t.Fatalf("concrete SKU identity missing: %s", rec.Body.String())
	}
	if row["default_sku_id"] != float64(552) {
		t.Fatalf("parent-authoritative default SKU missing: %s", rec.Body.String())
	}
	spec, _ := row["effective_sales_spec"].(map[string]any)
	if spec["sku_id"] != float64(551) || spec["spec_name"] != "磅" || spec["sales_unit"] != "磅" {
		t.Fatalf("effective_sales_spec = %#v; body=%s", spec, rec.Body.String())
	}
}

func TestCostingCalculateAPICustomerSkuWithoutCategoryDoesNotReturnExcelCategory(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{{
		ProductID:            417,
		Name:                 "曲奇拼配2.0",
		BeanListTemplateName: "红岩2.0",
		CustomerID:           152,
		BaseProductID:        199,
		Visibility:           "customer_only",
		CustomType:           "public_sku_alias",
		GreenBeanCostPerKg:   63.9,
		YieldRate:            0.8,
		ProductCategoryID:    0,
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
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v", got.Items)
	}
	category := got.Items[0].CommercialBeanList.Category
	item := got.Items[0].CommercialBeanList
	if category != "未分类" || item.Code != "999.2" || strings.Contains(category, "精品意式拼配") {
		t.Fatalf("customer SKU category = %q, item = %+v", category, item)
	}
}

func TestCostingCalculateAPICustomerSkuUsesSkuCategory(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{{
		ProductID:                 417,
		Name:                      "曲奇拼配2.0",
		BeanListTemplateName:      "红岩2.0",
		CustomerID:                152,
		BaseProductID:             199,
		Visibility:                "customer_only",
		CustomType:                "public_sku_alias",
		GreenBeanCostPerKg:        63.9,
		YieldRate:                 0.8,
		ProductCategoryID:         143,
		ProductCategoryPosition:   2,
		CategorySecondaryName:     "商用拼配",
		CategorySecondaryPosition: 1,
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
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v", got.Items)
	}
	item := got.Items[0].CommercialBeanList
	if item.Category != "1、商用拼配" || item.Code != "1.2" || strings.Contains(item.Category, "精品意式拼配") {
		t.Fatalf("customer SKU category = %+v", item)
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
	if got.TemplateName != "工厂量单模板" || got.TierLabel != "大客户量单" || got.SavedFinalPrice != 67 || got.PreviewFinalPrice != 74 {
		t.Fatalf("explanation = %+v", got)
	}
	if got.HasStep("expected_loss_rate") || strings.Contains(rec.Body.String(), "预期损耗率") {
		t.Fatalf("current price explanation must not expose retired overall loss: %s", rec.Body.String())
	}
	for _, want := range []string{`"key":"large_batch_production_cost_per_kg"`, `"key":"wholesale_package_cost_per_kg"`, `"key":"product_loss_per_kg"`, `"key":"retail_tax_rate"`, `"key":"template_margin_rate"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestPricingRuleTrialAPI(t *testing.T) {
	e := echo.New()
	svc := &capturingPricingRuleTrialService{}
	RegisterRoutes(e, Dependencies{Costing: svc})

	body, err := json.Marshal(appcosting.PricingRuleTrialCommand{
		PricingRuleID:       10,
		ProductID:           549,
		BomVersionID:        5392,
		ProcessRouteID:      19,
		OperationTemplateID: 9,
		QuoteUnit:           "kg",
		Overrides: appcosting.PricingRuleTrialOverrides{
			ExpectedLossRate: floatPtr(0.12),
			MarginRate:       floatPtr(0.3),
			TaxRate:          floatPtr(0.06),
			OtherCosts: map[string]float64{
				"包装贴标": 1.25,
			},
			PostMarkupCosts: map[string]float64{
				"利润税额": 1.1996,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trial", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got appcosting.PricingRuleTrialResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got.PricingRuleID != 10 || got.ProductID != 549 || got.FinalUnitPrice != 116.7092 {
		t.Fatalf("trial response = %+v", got)
	}
	if svc.last.BomVersionID != 5392 || svc.last.ProcessRouteID != 19 || svc.last.OperationTemplateID != 9 {
		t.Fatalf("selected production sources not bound: %+v", svc.last)
	}
	if got.BomVersionID != 5392 || got.ProcessRouteID != 19 || got.ProcessRouteName != "新版工艺路线" || got.OperationTemplateID != 9 || len(got.BomVersionOptions) != 2 || len(got.ProcessRouteOptions) != 2 || len(got.OperationTemplateOptions) != 2 {
		t.Fatalf("trial response missing selected production source options: %+v", got)
	}
	if svc.last.Overrides.PostMarkupCosts["利润税额"] != 1.1996 {
		t.Fatalf("post markup costs not bound: %+v", svc.last.Overrides.PostMarkupCosts)
	}
	if got.PriceAfterMarkup <= 0 || got.PostMarkupCostTotal <= 0 {
		t.Fatalf("trial response missing supplier formula fields: %+v", got)
	}
	if got.BomCostTotal != 67.5 || len(got.BaseCostDetails) != 1 || got.CostBaseTotal != 73.7625 || got.ProfitMarkupAmount != 39.987 || got.TaxInPriceAmount != 0 {
		t.Fatalf("trial response missing waterfall/detail fields: %+v", got)
	}
	if got.FormulaExpression == "" || len(got.FormulaExpressionLines) == 0 {
		t.Fatalf("trial response missing formula expression: %+v", got)
	}
	if len(got.OtherCostDetails) == 0 || got.ProfitExplanation.Method == "" {
		t.Fatalf("trial response missing explanation fields: %+v", got)
	}
	if got.ProfitExplanation.MethodLabel != "档位加价率" || strings.Contains(rec.Body.String(), "档位利润率/加价率") {
		t.Fatalf("trial response must expose markup-only labels: %+v", got.ProfitExplanation)
	}
	if !strings.Contains(rec.Body.String(), `"key":"post_markup_cost_total"`) || !strings.Contains(rec.Body.String(), `"key":"final_unit_price"`) {
		t.Fatalf("response missing formula steps: %s", rec.Body.String())
	}
	for _, want := range []string{`"formula_expression"`, `"formula_expression_lines"`, `"base_cost_details"`, `"recipe_ratio_pct":10`, `"effective_ratio_pct":12`, `"material_loss_rate":0.2`, `"cost_unit_cost":67.5`, `"cost_unit":"kg"`, `"other_cost_details"`, `"profit_explanation"`, `"yield_loss_amount"`, `"profit_markup_amount"`, `"tax_in_price_amount"`, `"process_route_id":19`, `"process_route_options"`, `最终售价 = 116.7092/kg`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing formula expression marker %s: %s", want, rec.Body.String())
		}
	}
}

func TestPricingRuleTrialBatchAPIUsesBatchServiceAndReturnsPartialErrorsInOrder(t *testing.T) {
	e := echo.New()
	svc := &capturingPricingRuleTrialBatchService{}
	RegisterRoutes(e, Dependencies{Costing: svc})

	body := bytes.NewBufferString(`{"requests":[{"pricing_rule_id":7,"product_id":101,"quote_unit":"kg"},{"pricing_rule_id":7,"product_id":999,"quote_unit":"kg"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trials", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.calls != 1 || len(svc.last) != 2 || svc.last[0].ProductID != 101 || svc.last[1].ProductID != 999 {
		t.Fatalf("batch service calls=%d requests=%+v", svc.calls, svc.last)
	}
	var got struct {
		Rows []appcosting.PricingRuleTrialBatchRow `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(got.Rows) != 2 || got.Rows[0].Index != 0 || got.Rows[0].Result == nil || got.Rows[0].Result.FinalUnitPrice != 88 || got.Rows[1].Index != 1 || got.Rows[1].Error != "product not found" {
		t.Fatalf("batch response = %+v", got.Rows)
	}
}

func TestPricingRuleTrialBatchAPIRejectsMoreThanOneHundredRequests(t *testing.T) {
	e := echo.New()
	svc := &capturingPricingRuleTrialBatchService{}
	RegisterRoutes(e, Dependencies{Costing: svc})
	requests := make([]appcosting.PricingRuleTrialCommand, 101)
	for i := range requests {
		requests[i] = appcosting.PricingRuleTrialCommand{PricingRuleID: 7, ProductID: int64(i + 1)}
	}
	body, err := json.Marshal(map[string]any{"requests": requests})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trials", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || svc.calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, svc.calls, rec.Body.String())
	}
}

func TestPricingRuleTrialAPIRejectsUnresolvableQuoteUnit(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(pricingRuleTrialUnitValidationRepo{})})

	body := bytes.NewBufferString(`{"pricing_rule_id":507,"product_id":552,"quote_unit":"盒"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trial", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "销售单位") || !strings.Contains(rec.Body.String(), "单位换算") {
		t.Fatalf("response missing unit conversion error: %s", rec.Body.String())
	}
}

func TestPricingRuleTrialAPIRejectsEmptyPublishedBomWithNonEmptyDraft(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(emptyPublishedBomTrialRepo{})})

	body := bytes.NewBufferString(`{"pricing_rule_id":18,"product_id":911,"quote_unit":"kg"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trial", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"V001", "没有组件", "V002", "草稿未发布", "先发布"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response = %s, want %q", rec.Body.String(), want)
		}
	}
}

func TestPricingRuleTrialAPIRejectsProductWithoutPublishedProductionBom(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(missingPublishedBomTrialRepo{})})

	body := bytes.NewBufferString(`{"pricing_rule_id":18,"product_id":715,"quote_unit":"kg"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trial", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	const want = "该商品未配置可用于试算的已发布生产 BOM，无法计算标准制造成本；请到 生产管理 → 生产 BOM 新增或发布 BOM 后再试算"
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("response = %s, want %q", rec.Body.String(), want)
	}
}

func TestPricingRuleTrialBatchAPIReturnsMissingProductionBomRowError(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(missingPublishedBomTrialRepo{})})

	body := bytes.NewBufferString(`{"requests":[{"pricing_rule_id":18,"product_id":715,"quote_unit":"kg"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trials", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Rows []appcosting.PricingRuleTrialBatchRow `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	const want = "该商品未配置可用于试算的已发布生产 BOM，无法计算标准制造成本；请到 生产管理 → 生产 BOM 新增或发布 BOM 后再试算"
	if len(got.Rows) != 1 || got.Rows[0].Index != 0 || got.Rows[0].Result != nil || got.Rows[0].Error != want {
		t.Fatalf("batch rows = %+v, want exact missing production BOM row error", got.Rows)
	}
}

func TestPricingRuleTrialAPIReturnsBadRequestOnServiceError(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakePricingRuleTrialErrorService{}})

	body := bytes.NewBufferString(`{"pricing_rule_id":10,"product_id":999}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trial", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "product not found") {
		t.Fatalf("response missing service error: %s", rec.Body.String())
	}
}

func TestCostingCalculateAPIRequiresGradientTemplateForCommercialTiers(t *testing.T) {
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
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v", got.Items)
	}
	if len(got.Items[0].CommercialWholesaleTiers) != 0 {
		t.Fatalf("commercial tiers = %+v, want none without gradient template", got.Items[0].CommercialWholesaleTiers)
	}
	if !containsWarning(got.Items[0].Warnings, domain.MissingPricingMethodWarning) {
		t.Fatalf("warnings = %+v, want missing pricing method warning", got.Items[0].Warnings)
	}
}

func TestCostingCalculateAPIRoundsRetailBeanListPricesWithoutCommercialTemplate(t *testing.T) {
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
	if len(item.CommercialWholesaleTiers) != 0 {
		t.Fatalf("commercial tiers = %+v, want none without gradient template", item.CommercialWholesaleTiers)
	}
	if len(item.RetailBeanTiers) != 2 || item.RetailBeanTiers[0].Label != "100g" || item.RetailBeanTiers[0].PricePerUnit != 92 {
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

func TestCostingCalculateAPIReturnsCustomerSkuCategoryBeanListMetadata(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	input := domain.ProductInput{
		ProductID:                 902,
		Name:                      "芬纳定制-红酒日晒-中深烘",
		ProductKind:               "roasted",
		CustomerID:                74,
		CustomType:                "custom_roast",
		ProductCategoryID:         502,
		ProductCategoryPosition:   2,
		CategoryPrimaryName:       "咖啡豆",
		CategoryPrimaryPosition:   1,
		CategorySecondaryName:     "定制咖啡熟豆",
		CategorySecondaryPosition: 1,
		GreenBeanCostPerKg:        67,
		YieldRate:                 0.815,
		Flavor:                    "红酒、莓果",
		BeanListNote:              "客户自有定制熟豆",
	}

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{input}})
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
				Code        string `json:"code"`
				Category    string `json:"category"`
				DisplayName string `json:"display_name"`
				Flavor      string `json:"flavor"`
				Description string `json:"description"`
			} `json:"commercial_bean_list"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v", got.Items)
	}
	item := got.Items[0].CommercialBeanList
	if item.Code != "1.2" || item.Category != "1、定制咖啡熟豆" || item.DisplayName != "芬纳定制-红酒日晒-中深烘" {
		t.Fatalf("customer commercial bean list = %+v", item)
	}
	if item.Flavor != "红酒、莓果" || item.Description != "客户自有定制熟豆" {
		t.Fatalf("customer commercial bean list missing fallbacks: %+v", item)
	}
}

func TestCostingCalculateAPICustomerAliasWithoutCategoryUsesUnclassifiedGroup(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	input := domain.ProductInput{
		ProductID:            417,
		Name:                 "曲奇拼配2.0",
		BeanListTemplateName: "红岩2.0",
		ProductKind:          "roasted",
		CustomerID:           152,
		CustomType:           "public_sku_alias",
		BaseProductID:        199,
		GreenBeanCostPerKg:   63.9,
		YieldRate:            0.8,
	}

	body, err := json.Marshal(appcosting.CalculateRequest{Products: []domain.ProductInput{input}})
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
				Code        string `json:"code"`
				Category    string `json:"category"`
				DisplayName string `json:"display_name"`
			} `json:"commercial_bean_list"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v", got.Items)
	}
	item := got.Items[0].CommercialBeanList
	if item.Code != "999.2" || item.Category != "未分类" || item.DisplayName != "曲奇拼配2.0" {
		t.Fatalf("customer alias without SKU category commercial bean list = %+v", item)
	}
	if strings.Contains(item.Category, "精品意式拼配") {
		t.Fatalf("customer alias without SKU category must not inherit Excel category: %+v", item)
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
		"POST /api/costing/bean-list/publications/:id/pdf",
		"GET /api/costing/bean-list/publications/:id/pdf",
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
	for _, gone := range []string{
		"GET /api/drip-price-templates",
		"POST /api/drip-price-templates",
		"PUT /api/drip-price-templates/:id",
		"POST /api/drip-price-templates/:id/deactivate",
		"POST /api/costing/drip-price-explanation",
	} {
		if seen[gone] {
			t.Fatalf("legacy drip template route must not be registered: %s", gone)
		}
	}
}

func TestBeanListPublicationPDFAPIGeneratesSavedPDFThenDownloadsIt(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	req := httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications/7/pdf?list_type=commercial&scope=official", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("generate status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var generated appcosting.BeanListPublicationPDFFile
	if err := json.Unmarshal(rec.Body.Bytes(), &generated); err != nil {
		t.Fatal(err)
	}
	if generated.PublicationID != 7 || generated.ContentType != "application/pdf" || generated.Bytes <= 0 {
		t.Fatalf("generated = %+v", generated)
	}
	if generated.DownloadURL != "/api/costing/bean-list/publications/7/pdf?list_type=commercial&scope=official" {
		t.Fatalf("download url = %q", generated.DownloadURL)
	}
	if generated.Payload != nil {
		t.Fatalf("payload must not be serialized in generate response")
	}

	req = httptest.NewRequest(http.MethodGet, generated.DownloadURL, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get(echo.HeaderContentType); got != "application/pdf" {
		t.Fatalf("content type = %q", got)
	}
	if cd := rec.Header().Get(echo.HeaderContentDisposition); !strings.Contains(cd, "bean-list-commercial-V3.0.5.pdf") {
		t.Fatalf("content disposition = %q", cd)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("download body is not a pdf: %q", rec.Body.String())
	}
}

func TestDripTemplateAndExplanationAPIsAreRemoved(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: fakeService{}})

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/drip-price-templates"},
		{method: http.MethodPost, path: "/api/drip-price-templates", body: `{}`},
		{method: http.MethodPut, path: "/api/drip-price-templates/5", body: `{}`},
		{method: http.MethodPost, path: "/api/drip-price-templates/5/deactivate"},
		{method: http.MethodPost, path: "/api/costing/drip-price-explanation", body: `{}`},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, body = %s", tc.method, tc.path, rec.Code, rec.Body.String())
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
		"<title>棵凡咖啡批发产品价格表 V3.0.5</title>",
		"棵凡咖啡批发产品价格表",
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

func TestPublicBeanListPageKeepsProductNameSeparateFromSelectedSalesSpec(t *testing.T) {
	page, err := renderPublicBeanListPage(appcosting.BeanListPublication{
		ListType: "commercial",
		Version:  "V4.4.0",
		Config:   map[string]any{"layoutStyle": "card", "cardsPerRow": 2},
		Content: map[string]any{
			"groups": []any{map[string]any{
				"category": "1、咖啡熟豆",
				"items": []any{
					map[string]any{
						"name":           "白月光瑰夏",
						"attributeLines": []any{"规格：227g", "烘焙度：浅烘"},
						"prices":         []any{map[string]any{"label": "2-13件", "price": 82.0, "unit": "227g"}},
					},
					map[string]any{
						"name":           "白月光瑰夏",
						"attributeLines": []any{"规格：454g", "烘焙度：浅烘"},
						"prices":         []any{map[string]any{"label": "2-13件", "price": 148.0, "unit": "454g"}},
					},
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(page, "白月光瑰夏"); got != 2 {
		t.Fatalf("public page product name count=%d, want 2; body=%s", got, page)
	}
	for _, want := range []string{"规格：227g", "规格：454g", "2-13件"} {
		if !strings.Contains(page, want) {
			t.Fatalf("public page missing %q; body=%s", want, page)
		}
	}
	for _, forbidden := range []string{"白月光瑰夏227g", "白月光瑰夏454g", "白月光瑰夏 · 227g", "白月光瑰夏 · 454g", "2-13个227g", "2-13个454g"} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("public page concatenated product name and spec as %q; body=%s", forbidden, page)
		}
	}
}

func TestPublicBeanListPagePassesProductTypeCategory(t *testing.T) {
	svc := &recordingBeanListService{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Costing: svc})

	req := httptest.NewRequest(http.MethodGet, "/public/bean-list/commercial?product_type_category_id=12", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.lastQuery.ProductTypeCategoryID != 12 {
		t.Fatalf("public product type query = %+v, want product_type_category_id 12", svc.lastQuery)
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
		"棵凡咖啡生豆产品价格表",
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

type pricingRuleTrialUnitValidationRepo struct {
	fakeRepo
}

type emptyPublishedBomTrialRepo struct {
	fakeRepo
}

type missingPublishedBomTrialRepo struct {
	fakeRepo
}

type priceTierTemplateUnitMismatchRepo struct {
	fakeRepo
}

func (priceTierTemplateUnitMismatchRepo) ResolveProductSalesUnitRule(_ context.Context, productID int64, priceUnit string) (appcosting.ProductSalesUnitRule, error) {
	if productID != 550 || (priceUnit != "" && priceUnit != "磅") {
		return appcosting.ProductSalesUnitRule{}, appcosting.ErrProductSalesUnitRuleNotFound
	}
	return appcosting.ProductSalesUnitRule{
		ProductID:        550,
		DefaultSalesUnit: "磅",
		InventoryUnit:    "kg",
		Conversion: map[string]map[string]float64{
			"磅": {"kg": 0.45359237},
		},
	}, nil
}

func (priceTierTemplateUnitMismatchRepo) ResolveProductDefaultSalesUnit(_ context.Context, productID int64) (string, error) {
	if productID != 550 {
		return "", appcosting.ErrProductSalesUnitRuleNotFound
	}
	return "磅", nil
}

func (priceTierTemplateUnitMismatchRepo) ResolveProductSpecIdentity(_ context.Context, productID int64) (appcosting.ProductSpecIdentity, error) {
	if productID != 550 {
		return appcosting.ProductSpecIdentity{}, appcosting.ErrProductSpecIdentityNotFound
	}
	return appcosting.ProductSpecIdentity{ProductID: 550, EffectiveParentProductID: 550, Active: true, SpecValid: true}, nil
}

func (priceTierTemplateUnitMismatchRepo) ResolvePriceTierTemplateUnitRule(_ context.Context, templateID int64) (appcosting.PriceTierTemplateUnitRule, error) {
	if templateID != 8 {
		return appcosting.PriceTierTemplateUnitRule{}, appcosting.ErrPriceTierTemplateUnitRuleNotFound
	}
	return appcosting.PriceTierTemplateUnitRule{
		TemplateID:   8,
		TemplateName: "咖啡熟豆",
		TierUnits:    map[int64]string{81: "lb", 82: "kg"},
	}, nil
}

func (pricingRuleTrialUnitValidationRepo) LoadProductInputs(context.Context, domain.Parameters) ([]domain.ProductInput, error) {
	return []domain.ProductInput{{
		ProductID:          552,
		Name:               "PR507 无盒换算商品",
		InventoryUnit:      "kg",
		QuoteUnit:          "kg",
		UnitConversionJSON: "{}",
		GreenBeanCostPerKg: 20,
		OperationCostPerKg: 5,
		YieldRate:          1,
	}}, nil
}

func (emptyPublishedBomTrialRepo) LoadProductInputs(context.Context, domain.Parameters) ([]domain.ProductInput, error) {
	return []domain.ProductInput{{
		ProductID:     911,
		ProductCode:   "SKU-000911",
		Name:          "萨其姆-生豆",
		ProductKind:   "roasted",
		InventoryUnit: "kg",
		QuoteUnit:     "kg",
		YieldRate:     0.8,
		BomVersionID:  91101,
		BomVersionNo:  "V001",
		BomUsageMode:  "production_bom_output",
		BomStatus:     "active",
	}}, nil
}

func (emptyPublishedBomTrialRepo) LoadProductPricingRule(context.Context, int64) (appcosting.ProductPricingRule, error) {
	return appcosting.ProductPricingRule{
		ID:             18,
		Name:           "生豆计算模板-麻袋",
		CostSourceMode: "bom_current_cost",
		MarginRate:     0.03,
		TaxRate:        0.30,
		RoundingMode:   "jiao",
		Active:         true,
		CalculationJSON: map[string]any{
			"yield_loss_mode": "bom_or_product",
			"profit_method":   "markup",
			"tax_mode":        "tax_included",
		},
	}, nil
}

func (emptyPublishedBomTrialRepo) LoadPricingRuleTrialProductionOptions(context.Context, domain.ProductInput) (appcosting.PricingRuleTrialProductionOptions, error) {
	return appcosting.PricingRuleTrialProductionOptions{BomVersions: []appcosting.PricingRuleTrialBomVersionOption{{
		BomID:                        9110,
		BomCode:                      "BOM-000911",
		BomName:                      "萨其姆-生豆 生产 BOM",
		VersionID:                    91101,
		VersionNo:                    "V001",
		Status:                       "published",
		IsDefault:                    true,
		ComponentCount:               0,
		LatestNonEmptyDraftVersionID: 91102,
		LatestNonEmptyDraftVersionNo: "V002",
	}}}, nil
}

func (emptyPublishedBomTrialRepo) LoadPricingRuleTrialBaseCostDetails(context.Context, domain.ProductInput) ([]appcosting.PricingRuleTrialBaseCostDetail, error) {
	return nil, nil
}

func (missingPublishedBomTrialRepo) LoadProductInputs(context.Context, domain.Parameters) ([]domain.ProductInput, error) {
	return []domain.ProductInput{{
		ProductID:     715,
		ProductCode:   "SKU-000715",
		Name:          "萨琪姆 生豆 Kg",
		ProductKind:   "green_bean",
		InventoryUnit: "kg",
		QuoteUnit:     "kg",
		YieldRate:     1,
		BomStatus:     "missing",
	}}, nil
}

func (missingPublishedBomTrialRepo) LoadProductPricingRule(context.Context, int64) (appcosting.ProductPricingRule, error) {
	return appcosting.ProductPricingRule{
		ID:             18,
		Name:           "生豆计算模板-麻袋",
		CostSourceMode: "bom_current_cost",
		MarginRate:     0.03,
		TaxRate:        0.30,
		RoundingMode:   "jiao",
		Active:         true,
		CalculationJSON: map[string]any{
			"yield_loss_mode": "bom_or_product",
			"profit_method":   "markup",
			"tax_mode":        "tax_included",
		},
	}, nil
}

func (missingPublishedBomTrialRepo) LoadPricingRuleTrialProductionOptions(context.Context, domain.ProductInput) (appcosting.PricingRuleTrialProductionOptions, error) {
	return appcosting.PricingRuleTrialProductionOptions{}, nil
}

func (pricingRuleTrialUnitValidationRepo) LoadProductPricingRule(context.Context, int64) (appcosting.ProductPricingRule, error) {
	return appcosting.ProductPricingRule{
		ID:             507,
		Name:           "PR507 单位校验",
		MarginRate:     0.2,
		TaxRate:        0,
		RoundingMode:   "none",
		FormulaVersion: "v1",
		Active:         true,
		CalculationJSON: map[string]any{
			"yield_loss_mode": "none",
			"profit_method":   "markup",
			"tax_mode":        "none",
		},
	}, nil
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
	if listed.Rows[0].PublicationPurpose != "factory_supply" {
		t.Fatalf("publication purpose=%q, want factory_supply", listed.Rows[0].PublicationPurpose)
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
	if published.PublicationPurpose != "factory_supply" {
		t.Fatalf("published purpose=%q, want factory_supply", published.PublicationPurpose)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications/8/withdraw", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("withdraw status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestBeanListPublicationAPIPreservesSeparatedProductNameAndSalesSpecSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "publish", path: "/api/costing/bean-list/publications"},
		{name: "draft", path: "/api/costing/bean-list/drafts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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

			body := bytes.NewBufferString(`{
				"list_type":"commercial",
				"version":"V4.4.0",
				"scope":"mine",
				"config":{"product_spec_selections":[{"parent_product_id":600,"sku_id":991,"selection_source":"explicit","default_sku_id_at_selection":990}]},
				"content":{
					"groups":[{"category":"1、咖啡豆","items":[{
						"name":"白月光瑰夏",
						"display_name_snapshot":"白月光瑰夏",
						"product_name_snapshot":"白月光瑰夏",
						"sku_id":991,
						"parent_product_id":600,
						"sku_name":"227g袋装",
						"spec_label":"227g",
						"effective_sales_spec":{"sku_id":991,"spec_name":"227g","sales_unit":"袋"},
						"productAttributes":[{"key":"sales_spec","label":"规格","value":"227g"}],
						"attributeLines":["规格：227g"]
					}]}],
					"price_rows":[{"product_id":991,"sku_id":991,"parent_product_id":600,"product_name":"白月光瑰夏","sku_name":"227g袋装","spec_label":"227g","quantity_basis":"sales_spec_count"}]
				}
			}`)
			req := httptest.NewRequest(http.MethodPost, tc.path, body)
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}

			cmd := svc.lastPublish
			if tc.name == "draft" {
				cmd = svc.lastDraft
			}
			groups := publicMapsFromAny(cmd.Content["groups"])
			if len(groups) != 1 {
				t.Fatalf("groups=%v", cmd.Content["groups"])
			}
			items := publicMapsFromAny(groups[0]["items"])
			if len(items) != 1 {
				t.Fatalf("items=%v", groups[0]["items"])
			}
			item := items[0]
			if got := mapString(item, "name", ""); got != "白月光瑰夏" {
				t.Fatalf("item name=%q", got)
			}
			if got := mapString(item, "product_name_snapshot", ""); got != "白月光瑰夏" {
				t.Fatalf("product snapshot name=%q", got)
			}
			if got := publicStringList(item["attributeLines"]); len(got) != 1 || got[0] != "规格：227g" {
				t.Fatalf("attribute lines=%v", got)
			}
			priceRows := publicMapsFromAny(cmd.Content["price_rows"])
			if len(priceRows) != 1 || mapString(priceRows[0], "product_name", "") != "白月光瑰夏" || int64(mapNumber(priceRows[0], "sku_id", 0)) != 991 {
				t.Fatalf("price rows=%v", cmd.Content["price_rows"])
			}
		})
	}
}

type pr543BeanListPublicationRepo struct {
	fakeRepo
	published appcosting.PublishBeanListCommand
}

func (r *pr543BeanListPublicationRepo) ResolveProductSpecIdentity(_ context.Context, productID int64) (appcosting.ProductSpecIdentity, error) {
	switch productID {
	case 600:
		return appcosting.ProductSpecIdentity{ProductID: 600, EffectiveParentProductID: 600, ParentProductName: "白月光瑰夏", Active: true, SpecValid: true}, nil
	case 991:
		return appcosting.ProductSpecIdentity{ProductID: 991, EffectiveParentProductID: 600, ParentProductName: "白月光瑰夏", Active: true, SpecValid: true}, nil
	default:
		return appcosting.ProductSpecIdentity{}, appcosting.ErrProductSpecIdentityNotFound
	}
}

func (r *pr543BeanListPublicationRepo) ResolveProductSalesUnitRule(_ context.Context, productID int64, _ string) (appcosting.ProductSalesUnitRule, error) {
	if productID != 991 {
		return appcosting.ProductSalesUnitRule{}, appcosting.ErrProductSalesUnitRuleNotFound
	}
	return appcosting.ProductSalesUnitRule{
		ProductID:        991,
		SKUName:          "227g袋装",
		DefaultSalesUnit: "227g",
		InventoryUnit:    "g",
		Conversion:       map[string]map[string]float64{"227g": {"g": 227}},
		EffectiveSalesSpec: &domain.EffectiveSalesSpec{
			SKUID:                   991,
			SpecName:                "227g",
			SpecLabel:               "227g",
			SalesUnit:               "227g",
			InventoryUnit:           "g",
			InventoryConversionJSON: map[string]map[string]float64{"227g": {"g": 227}},
		},
	}, nil
}

func (r *pr543BeanListPublicationRepo) PublishBeanList(_ context.Context, cmd appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error) {
	r.published = cmd
	return &appcosting.BeanListPublication{
		ListType:           cmd.ListType,
		Version:            cmd.Version,
		Status:             "published",
		PublicationPurpose: cmd.PublicationPurpose,
		OwnerType:          cmd.OwnerType,
		OwnerKey:           cmd.OwnerKey,
		Config:             cmd.Config,
		Content:            cmd.Content,
	}, nil
}

func TestBeanListPublicationHTTPPersistsPieceTierAndSeparatedParentProductName(t *testing.T) {
	repo := &pr543BeanListPublicationRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(repo)})

	body := bytes.NewBufferString(`{
		"list_type":"commercial",
		"version":"V5.3.1",
		"scope":"mine",
		"config":{"product_spec_selections":[{
			"parent_product_id":600,
			"sku_id":991,
			"selection_source":"explicit",
			"default_sku_id_at_selection":991
		}]},
		"content":{
			"groups":[{"category":"1、咖啡豆","items":[{
				"product_id":991,
				"sku_id":991,
				"parent_product_id":600,
				"name":"白月光瑰夏227g",
				"display_name_snapshot":"白月光瑰夏227g",
				"product_name_snapshot":"白月光瑰夏227g",
				"sku_name":"227g袋装",
				"spec_label":"227g",
				"effective_sales_spec":{"sku_id":991,"spec_name":"227g","sales_unit":"227g"},
				"productAttributes":[{"key":"sales_spec","label":"规格","value":"227g"}],
				"attributeLines":["规格：227g"],
				"prices":[{"label":"2-13个227g","price":82,"unit":"227g"}]
			}]}],
			"price_rows":[{
				"product_id":991,
				"sku_id":991,
				"parent_product_id":600,
				"product_name":"白月光瑰夏227g",
				"sku_name":"227g袋装",
				"spec_label":"227g",
				"tier_label":"2-13个227g",
				"min_qty":2,
				"max_qty":13,
				"pricing_mode":"tier_template",
				"pricing_mode_source":"sku",
				"tier_template_id":8,
				"tier_template_source":"sku",
				"template_tier_id":81,
				"pricing_rule_id":40,
				"pricing_rule_source":"tier_template",
				"pricing_rule_version":"熟豆-v1",
				"tier_pricing_rule_id":40,
				"tier_pricing_rule_version":"熟豆-v1",
				"final_unit_price":82,
				"original_final_unit_price":82,
				"price_unit":"227g",
				"inventory_unit":"g",
				"inventory_conversion_json":{"227g":{"g":227}},
				"group_snapshot":{"group_id":3,"group_name":"商品价格表分组","group_item_id":101,"group_item_name":"咖啡熟豆"},
				"group_source":"product_catalog",
				"cost_source_snapshot":{"source":"fixed_price"},
				"customer_reference_snapshot":{},
				"manual_adjusted":false,
				"quantity_basis":"sales_spec_count"
			}]
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	groups := publicMapsFromAny(repo.published.Content["groups"])
	if len(groups) != 1 {
		t.Fatalf("persisted groups=%v", repo.published.Content["groups"])
	}
	items := publicMapsFromAny(groups[0]["items"])
	if len(items) != 1 {
		t.Fatalf("persisted items=%v", groups[0]["items"])
	}
	if got := mapString(items[0], "name", ""); got != "白月光瑰夏" {
		t.Fatalf("persisted product name=%q, want parent product name", got)
	}
	if got := mapString(items[0], "display_name_snapshot", ""); got != "白月光瑰夏" {
		t.Fatalf("persisted display name=%q, want parent product name", got)
	}
	rows := publicMapsFromAny(repo.published.Content["price_rows"])
	if len(rows) != 1 {
		t.Fatalf("persisted price rows=%v", repo.published.Content["price_rows"])
	}
	if got := mapString(rows[0], "tier_label", ""); got != "2-13件" {
		t.Fatalf("persisted tier label=%q, want 2-13件", got)
	}
	persistedPrices := publicMapsFromAny(items[0]["prices"])
	if len(persistedPrices) != 1 || mapString(persistedPrices[0], "label", "") != "2-13件" {
		t.Fatalf("persisted preview prices=%v, want generic piece label", items[0]["prices"])
	}
}

func TestBeanListPublicationAndDraftAPIsUseConcreteSalesSpecCountInsteadOfTemplateUnitLabels(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "publish", path: "/api/costing/bean-list/publications"},
		{name: "draft", path: "/api/costing/bean-list/drafts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error {
					c.Set("basic_auth_admin", true)
					c.Set("employee_id", int64(7))
					return next(c)
				}
			})
			RegisterRoutes(e, Dependencies{Costing: appcosting.NewService(priceTierTemplateUnitMismatchRepo{})})

			body := bytes.NewBufferString(`{
				"list_type":"commercial",
				"version":"V4.3.0",
				"scope":"mine",
				"config":{"product_spec_selections":[{"parent_product_id":550,"sku_id":550,"selection_source":"product_default","default_sku_id_at_selection":550}]},
				"content":{"price_rows":[{
					"product_id":550,
					"sku_id":550,
					"parent_product_id":550,
					"product_name":"初晓",
					"tier_label":"1kg+",
					"min_qty":1,
					"final_unit_price":68,
					"original_final_unit_price":68,
					"price_unit":"磅",
					"inventory_unit":"kg",
					"inventory_conversion_json":{"磅":{"kg":0.45359237}},
					"group_snapshot":{"group_id":3,"group_name":"商品价格表分组","group_item_id":101,"group_item_name":"咖啡豆"},
					"group_source":"product_catalog",
					"pricing_mode":"tier_template",
					"pricing_mode_source":"product",
					"tier_template_id":8,
					"tier_template_name":"伪造可用模板",
					"tier_template_source":"product",
					"template_tier_id":81,
					"tier_quantity_unit":"磅",
					"pricing_rule_id":40,
					"pricing_rule_source":"tier_template",
					"pricing_rule_version":"咖啡熟豆模板-v1",
					"tier_pricing_rule_id":40,
					"tier_pricing_rule_version":"咖啡熟豆模板-v1",
					"cost_source_snapshot":{"bom_version_no":"BOM-CHUXIAO/V001"},
					"customer_reference_snapshot":{"customer_id":0},
					"manual_adjusted":true,
					"quantity_basis":"sales_spec_count"
				}]}
			}`)
			req := httptest.NewRequest(http.MethodPost, tc.path, body)
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s, want 200", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "阶梯模板不可用") || strings.Contains(rec.Body.String(), "不匹配") {
				t.Fatalf("historical template quantity-unit wording must not block a concrete sales-spec-count request: %s", rec.Body.String())
			}
		})
	}
}

func TestBeanListPublicationArchiveAPI(t *testing.T) {
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

	body := bytes.NewBufferString(`{"ids":[7,8]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications/archive?list_type=commercial&scope=customer&customer_id=42&publication_purpose=factory_supply", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.archived != 1 || len(svc.lastArchive.IDs) != 2 || svc.lastArchive.IDs[0] != 7 || svc.lastArchive.IDs[1] != 8 {
		t.Fatalf("archive command = %+v, count=%d", svc.lastArchive, svc.archived)
	}
	if svc.lastArchive.OwnerType != "customer" || svc.lastArchive.OwnerKey != "42" || svc.lastArchive.PublicationPurpose != "factory_supply" || svc.lastArchive.Actor == "" {
		t.Fatalf("archive owner/scope = %+v", svc.lastArchive)
	}

	body = bytes.NewBufferString(`{"ids":[7]}`)
	req = httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications/unarchive?list_type=commercial&scope=customer&customer_id=42&publication_purpose=factory_supply", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unarchive status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.unarchived != 1 || len(svc.lastUnarchive.IDs) != 1 || svc.lastUnarchive.IDs[0] != 7 {
		t.Fatalf("unarchive command = %+v, count=%d", svc.lastUnarchive, svc.unarchived)
	}
	if svc.lastUnarchive.OwnerType != "customer" || svc.lastUnarchive.OwnerKey != "42" || svc.lastUnarchive.PublicationPurpose != "factory_supply" {
		t.Fatalf("unarchive owner/scope = %+v", svc.lastUnarchive)
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

func TestBeanListPublicationAPISupportsPurposeFilter(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/costing/bean-list/publications?list_type=green&scope=customer&customer_id=42&publication_purpose=customer_resale", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.lastQuery.PublicationPurpose != "customer_resale" || svc.lastQuery.OwnerType != "customer" || svc.lastQuery.OwnerKey != "42" {
		t.Fatalf("query = %+v, want customer_resale customer 42", svc.lastQuery)
	}

	body := bytes.NewBufferString(`{"list_type":"green","version":"V2","scope":"customer","customer_id":42,"publication_purpose":"customer_resale","price_source_publication_id":11,"source_version":"G-1","config":{"brandName":"客户品牌"},"content":{"totalItems":1},"changelog":"客户转售豆单"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.lastPublish.PublicationPurpose != "customer_resale" || svc.lastPublish.PriceSourcePublicationID != 11 || svc.lastPublish.SourceVersion != "G-1" {
		t.Fatalf("publish = %+v, want customer_resale source trace", svc.lastPublish)
	}
}

func TestBeanListPublicationAPIPassesProductTypeCategory(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/costing/bean-list/publications?product_type_category_id=12&scope=official", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.lastQuery.ProductTypeCategoryID != 12 {
		t.Fatalf("product type query = %+v, want product_type_category_id 12", svc.lastQuery)
	}

	body := bytes.NewBufferString(`{"list_type":"green","product_type_category_id":12,"product_type_name":"生豆","classification_template_id":12,"classification_template_name":"报价分类","classification_category_id":34,"classification_category_name":"现货生豆","version":"V4.0.1","scope":"official","config":{"layoutStyle":"card"},"content":{"totalItems":1},"changelog":"生豆商品价格表"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/costing/bean-list/publications", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if svc.lastPublish.ProductTypeCategoryID != 12 || svc.lastPublish.ProductTypeName != "生豆" {
		t.Fatalf("product type publish = %+v", svc.lastPublish)
	}
	if svc.lastPublish.ClassificationTemplateID != 12 || svc.lastPublish.ClassificationTemplateName != "报价分类" || svc.lastPublish.ClassificationCategoryID != 34 || svc.lastPublish.ClassificationCategoryName != "现货生豆" {
		t.Fatalf("classification publish = %+v", svc.lastPublish)
	}
}

func TestBeanListAPIPassesCustomerIDForCustomerProductRules(t *testing.T) {
	svc := &recordingBeanListService{}
	e := echo.New()
	registerCostingAPI(e, svc, &fakeCostingAuthz{})

	req := httptest.NewRequest(http.MethodGet, "/api/costing/bean-list?customer_id=42", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/costing/bean-list status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastBeanList.CustomerID != 42 {
		t.Fatalf("bean list query = %+v, want customer_id 42", svc.lastBeanList)
	}
}

func TestBeanListPublicationPublishAndDraftGenerateStoredPreviewPDF(t *testing.T) {
	for _, tc := range []struct {
		name          string
		path          string
		wantID        int64
		wantOwnerType string
		wantOwnerKey  string
	}{
		{name: "publish", path: "/api/costing/bean-list/publications", wantID: 8, wantOwnerType: "customer", wantOwnerKey: "42"},
		{name: "draft", path: "/api/costing/bean-list/drafts", wantID: 9, wantOwnerType: "customer", wantOwnerKey: "42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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

			body := bytes.NewBufferString(`{"list_type":"commercial","version":"V3.0.8","scope":"customer","customer_id":42,"config":{"layoutStyle":"card","backgroundColor":"#f8f1e5"},"content":{"title":"棵凡咖啡批发产品价格表","groups":[{"category":"1、工厂量单","showCategory":true,"items":[{"code":"1.1","name":"曲奇拼配","prices":[{"label":"25-49kg","price":21,"unit":"kg"}]}]}]},"changelog":"客户 A 豆单"}`)
			req := httptest.NewRequest(http.MethodPost, tc.path, body)
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if svc.generatedPDFs != 1 {
				t.Fatalf("generated PDFs = %d", svc.generatedPDFs)
			}
			if svc.lastPDFCommand.PublicationID != tc.wantID ||
				svc.lastPDFCommand.Query.ListType != "commercial" ||
				svc.lastPDFCommand.Query.OwnerType != tc.wantOwnerType ||
				svc.lastPDFCommand.Query.OwnerKey != tc.wantOwnerKey {
				t.Fatalf("pdf command = %+v", svc.lastPDFCommand)
			}
		})
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
