package production

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"

	"github.com/labstack/echo/v4"
)

type productionReplanAPIRepo struct {
	workOrderAPIRepo
	preview productionapp.ProductionReplanPreviewCommand
	commit  productionapp.ProductionReplanCommand
}

func (r *productionReplanAPIRepo) PreviewProductionReplan(_ context.Context, cmd productionapp.ProductionReplanPreviewCommand) (productionapp.ProductionReplanPreview, error) {
	r.preview = cmd
	return productionapp.ProductionReplanPreview{ProductionPlanID: cmd.ProductionPlanID, Revision: cmd.Revision, CanReplan: true, TotalQuantityG: 40000}, nil
}

func (r *productionReplanAPIRepo) ReplanProduction(_ context.Context, cmd productionapp.ProductionReplanCommand) (productionapp.ProductionReplanResult, error) {
	r.commit = cmd
	return productionapp.ProductionReplanResult{PreviousPlanID: cmd.ProductionPlanID, PreviousPlanNo: "PP-OLD", NewPlan: productionapp.ProductionPlanDetail{ID: 99, PlanNo: "PP-NEW", Status: "draft"}}, nil
}

func TestProductionReplanPreviewAndCommitAPI(t *testing.T) {
	e := echo.New()
	repo := &productionReplanAPIRepo{}
	registerProductionPlanAPI(e, productionapp.NewService(repo))
	body := `{"revision":3,"production_plan_item_ids":[7],"from":"2026-09-01","to":"2026-09-30","selected":["new-demand"],"request_id":"rp-api-1"}`
	for _, path := range []string{"/api/production-plans/41/replan/preview", "/api/production-plans/41/replan"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if repo.preview.ProductionPlanID != 41 || repo.preview.Revision != 3 || !repo.preview.Selected["new-demand"] {
		t.Fatalf("preview command = %+v", repo.preview)
	}
	if repo.commit.RequestID != "rp-api-1" || repo.commit.Operator == "" || repo.commit.ProductionPlanItemIDs[0] != 7 {
		t.Fatalf("commit command = %+v", repo.commit)
	}
}
