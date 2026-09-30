package materials

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	materialsapp "orderapp/internal/application/materials"

	"github.com/labstack/echo/v4"
)

type materialListCaptureRepository struct {
	materialsapp.Repository
	command materialsapp.ListCommand
}

func (r *materialListCaptureRepository) List(_ context.Context, command materialsapp.ListCommand) ([]materialsapp.Material, error) {
	r.command = command
	return []materialsapp.Material{}, nil
}

func (r *materialListCaptureRepository) ResolveBoundCustomerID(context.Context, int64) (int64, error) {
	return 0, nil
}

func TestMaterialsListAPIForwardsFuzzySearchAndPageWindow(t *testing.T) {
	repository := &materialListCaptureRepository{}
	e := echo.New()
	registerMaterialsAPI(e, materialsapp.NewService(repository))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/materials?q=烘焙豆&active=active&limit=40&offset=80", nil)
	e.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/materials status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if repository.command.Query != "烘焙豆" || repository.command.Active != "active" || repository.command.Limit != 40 || repository.command.Offset != 80 {
		t.Fatalf("materials query was not forwarded with paging: %+v", repository.command)
	}
}
