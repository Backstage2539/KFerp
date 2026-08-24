package bom

import (
	"errors"
	"net/http"
	"strconv"

	bomapp "orderapp/internal/application/bom"
	"orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

type SaveBomRequest struct {
	ProductID        int64    `json:"product_id"`
	ExpectedLossRate *float64 `json:"expected_loss_rate"`
}

type SaveBomItemRequest struct {
	ProductID          int64   `json:"product_id"`
	MaterialID         int64   `json:"material_id"`
	ComponentType      string  `json:"component_type"`
	ComponentProductID int64   `json:"component_product_id"`
	ComponentSpecG     int64   `json:"component_spec_g"`
	ConsumeUnit        string  `json:"consume_unit"`
	QtyPerUnit         float64 `json:"qty_per_unit"`
	RatioPct           float64 `json:"ratio_pct"`
}

type DeleteBomItemRequest struct {
	ProductID int64 `json:"product_id"`
	ID        int64 `json:"id"`
}

type SaveBagSpecMappingRequest struct {
	SpecG      int64 `json:"spec_g"`
	MaterialID int64 `json:"material_id"`
}

type DeleteBagSpecMappingRequest struct {
	SpecG int64 `json:"spec_g"`
}

type CreateBomVersionRequest struct {
	ProductID int64  `json:"product_id"`
	Note      string `json:"note"`
}

type createProductionBomGroupRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type updateProductionBomGroupRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type createProductionBomGroupCategoryRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type updateProductionBomGroupCategoryRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type moveProductionBomGroupRequest struct {
	SortOrder int `json:"sort_order"`
}

type createProductionBomRequest struct {
	Name                  string   `json:"name"`
	OutputType            string   `json:"output_type"`
	OutputID              int64    `json:"output_id"`
	OutputProductID       int64    `json:"output_product_id"`
	OutputMaterialID      int64    `json:"output_material_id"`
	OutputQty             float64  `json:"output_qty"`
	OutputUnit            string   `json:"output_unit"`
	GroupID               int64    `json:"group_id"`
	GroupCategoryID       int64    `json:"group_category_id"`
	ExpectedLossRate      *float64 `json:"expected_loss_rate"`
	SpecTemplateVersionID int64    `json:"spec_template_version_id"`
	MainInputMaterialID   int64    `json:"main_input_material_id"`
}

type updateProductionBomRequest struct {
	Name             string `json:"name"`
	OutputType       string `json:"output_type"`
	OutputID         int64  `json:"output_id"`
	OutputProductID  int64  `json:"output_product_id"`
	OutputMaterialID int64  `json:"output_material_id"`
	OutputUnit       string `json:"output_unit"`
	GroupID          *int64 `json:"group_id"`
	GroupCategoryID  *int64 `json:"group_category_id"`
	Status           string `json:"status"`
}

type copyProductionBomRequest struct {
	Name                  string `json:"name"`
	OutputType            string `json:"output_type"`
	OutputID              int64  `json:"output_id"`
	OutputProductID       int64  `json:"output_product_id"`
	OutputMaterialID      int64  `json:"output_material_id"`
	GroupID               int64  `json:"group_id"`
	GroupCategoryID       int64  `json:"group_category_id"`
	SpecTemplateVersionID int64  `json:"spec_template_version_id"`
	MainInputMaterialID   int64  `json:"main_input_material_id"`
}

type createProductionBomVersionRequest struct {
	SourceVersionID int64  `json:"source_version_id"`
	Note            string `json:"note"`
}

type updateProductionBomVersionDraftRequest struct {
	ExpectedLossRate       *float64                           `json:"expected_loss_rate"`
	MaterialLossRate       *float64                           `json:"material_loss_rate"`
	OutputQty              float64                            `json:"output_qty"`
	OutputUnit             string                             `json:"output_unit"`
	ProcessRouteID         int64                              `json:"process_route_id"`
	Items                  []bomapp.ProductionBomDraftItem    `json:"items"`
	Variants               []bomapp.ProductionBomDraftVariant `json:"variants"`
	SpecialAttrsSchemaJSON string                             `json:"special_attrs_schema_json"`
	SpecialAttrsJSON       string                             `json:"special_attrs_json"`
}

type draftWorkspaceRequest struct {
	Name                   string                             `json:"name"`
	OutputType             string                             `json:"output_type"`
	OutputID               int64                              `json:"output_id"`
	OutputProductID        int64                              `json:"output_product_id"`
	OutputMaterialID       int64                              `json:"output_material_id"`
	OutputUnit             string                             `json:"output_unit"`
	GroupID                *int64                             `json:"group_id"`
	GroupCategoryID        *int64                             `json:"group_category_id"`
	Status                 string                             `json:"status"`
	VersionID              int64                              `json:"version_id"`
	ExpectedLossRate       *float64                           `json:"expected_loss_rate"`
	MaterialLossRate       *float64                           `json:"material_loss_rate"`
	OutputQty              float64                            `json:"output_qty"`
	ProcessRouteID         int64                              `json:"process_route_id"`
	Items                  []bomapp.ProductionBomDraftItem    `json:"items"`
	Variants               []bomapp.ProductionBomDraftVariant `json:"variants"`
	SpecialAttrsSchemaJSON string                             `json:"special_attrs_schema_json"`
	SpecialAttrsJSON       string                             `json:"special_attrs_json"`
	SpecTemplateVersionID  int64                              `json:"spec_template_version_id"`
	MainInputMaterialID    int64                              `json:"main_input_material_id"`
}

type reapplyProductionBomSpecTemplateVersionRequest struct {
	SpecTemplateVersionID int64 `json:"spec_template_version_id"`
	MainInputMaterialID   int64 `json:"main_input_material_id"`
}

type createProductionBomSpecTemplateRequest struct {
	Name string `json:"name"`
}

type createProductionBomSpecTemplateVersionRequest struct {
	SourceVersionID int64  `json:"source_version_id"`
	Note            string `json:"note"`
}

type updateProductionBomSpecTemplateVersionDraftRequest struct {
	Variants []bomapp.ProductionBomSpecTemplateVariant `json:"variants"`
}

type bindProductProductionBomRequest struct {
	BomID                  int64 `json:"bom_id"`
	BomVersionID           int64 `json:"bom_version_id"`
	ProductionBomID        int64 `json:"production_bom_id"`
	ProductionBomVersionID int64 `json:"production_bom_version_id"`
	DefaultProductionBomID int64 `json:"default_production_bom_id"`
}

func (r bindProductProductionBomRequest) normalized() (int64, int64) {
	bomID := r.ProductionBomID
	if bomID <= 0 {
		bomID = r.BomID
	}
	if bomID <= 0 {
		bomID = r.DefaultProductionBomID
	}
	versionID := r.ProductionBomVersionID
	if versionID <= 0 {
		versionID = r.BomVersionID
	}
	return bomID, versionID
}

type ErrorResponse struct {
	Error string `json:"error"`
}

const legacyProductionBomGroupsReadonlyError = "production BOM groups are legacy readonly; use business_group_assignments"

func legacyProductionBomGroupsReadonlyAPI(c echo.Context) error {
	return c.JSON(http.StatusGone, ErrorResponse{Error: legacyProductionBomGroupsReadonlyError})
}

func setDefaultProductionBomAPI(c echo.Context, bomSvc *bomapp.Service) error {
	productID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || productID <= 0 {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
	}
	var req bindProductProductionBomRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}
	bomID, versionID := req.normalized()
	row, err := bomSvc.BindProductProductionBom(c.Request().Context(), bomapp.BindProductProductionBomCommand{ProductID: productID, BomID: bomID, BomVersionID: versionID, DefaultProductionBomID: req.DefaultProductionBomID, Actor: support.ActorOf(c)})
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, row)
}

func setDefaultProductionBomOutputAPI(c echo.Context, bomSvc *bomapp.Service, outputType string, outputID int64) error {
	if outputID <= 0 {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid output_id"})
	}
	var req bindProductProductionBomRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}
	bomID, versionID := req.normalized()
	row, err := bomSvc.BindProductionBomOutput(c.Request().Context(), bomapp.BindProductionBomOutputCommand{OutputType: outputType, OutputID: outputID, BomID: bomID, BomVersionID: versionID, Actor: support.ActorOf(c)})
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, row)
}

func registerBomAPI(e *echo.Echo, bomSvc *bomapp.Service) {
	e.GET("/api/production-bom-spec-templates", func(c echo.Context) error {
		rows, err := bomSvc.ListProductionBomSpecTemplates(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.POST("/api/production-bom-spec-templates", func(c echo.Context) error {
		var req createProductionBomSpecTemplateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.CreateProductionBomSpecTemplate(c.Request().Context(), bomapp.CreateProductionBomSpecTemplateCommand{Name: req.Name, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/production-bom-spec-templates/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid spec_template_id"})
		}
		versionID, _ := strconv.ParseInt(c.QueryParam("version_id"), 10, 64)
		row, err := bomSvc.GetProductionBomSpecTemplate(c.Request().Context(), id, versionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return c.JSON(http.StatusNotFound, ErrorResponse{Error: "specification template not found"})
			}
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/production-bom-spec-templates/:id/versions", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid spec_template_id"})
		}
		var req createProductionBomSpecTemplateVersionRequest
		if err := c.Bind(&req); err != nil && !errors.Is(err, echo.ErrUnsupportedMediaType) {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.CreateProductionBomSpecTemplateVersion(c.Request().Context(), bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: id, SourceVersionID: req.SourceVersionID, Note: req.Note, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.PUT("/api/production-bom-spec-template-versions/:id/draft", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid spec_template_version_id"})
		}
		var req updateProductionBomSpecTemplateVersionDraftRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.UpdateProductionBomSpecTemplateVersionDraft(c.Request().Context(), bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: id, Variants: req.Variants, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/production-bom-spec-template-versions/:id/publish", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid spec_template_version_id"})
		}
		if err := bomSvc.PublishProductionBomSpecTemplateVersion(c.Request().Context(), bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/production-bom-groups", func(c echo.Context) error {
		rows, err := bomSvc.ListProductionBomGroups(c.Request().Context(), false)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.POST("/api/production-bom-groups", func(c echo.Context) error {
		return legacyProductionBomGroupsReadonlyAPI(c)
		var req createProductionBomGroupRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.CreateProductionBomGroup(c.Request().Context(), bomapp.CreateProductionBomGroupCommand{Name: req.Name, SortOrder: req.SortOrder, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.PUT("/api/production-bom-groups/:id", func(c echo.Context) error {
		return legacyProductionBomGroupsReadonlyAPI(c)
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid group_id"})
		}
		var req updateProductionBomGroupRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.UpdateProductionBomGroup(c.Request().Context(), bomapp.UpdateProductionBomGroupCommand{ID: id, Name: req.Name, SortOrder: req.SortOrder, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.DELETE("/api/production-bom-groups/:id", func(c echo.Context) error {
		return legacyProductionBomGroupsReadonlyAPI(c)
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid group_id"})
		}
		if err := bomSvc.DeleteProductionBomGroup(c.Request().Context(), bomapp.DeleteProductionBomGroupCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/production-bom-groups/:id/move", func(c echo.Context) error {
		return legacyProductionBomGroupsReadonlyAPI(c)
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid group_id"})
		}
		var req moveProductionBomGroupRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if err := bomSvc.MoveProductionBomGroup(c.Request().Context(), bomapp.MoveProductionBomGroupCommand{ID: id, SortOrder: req.SortOrder, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/production-bom-groups/:id/categories", func(c echo.Context) error {
		return legacyProductionBomGroupsReadonlyAPI(c)
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid group_id"})
		}
		var req createProductionBomGroupCategoryRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.CreateProductionBomGroupCategory(c.Request().Context(), bomapp.CreateProductionBomGroupCategoryCommand{GroupID: id, Name: req.Name, SortOrder: req.SortOrder, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.PUT("/api/production-bom-group-categories/:id", func(c echo.Context) error {
		return legacyProductionBomGroupsReadonlyAPI(c)
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid category_id"})
		}
		var req updateProductionBomGroupCategoryRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.UpdateProductionBomGroupCategory(c.Request().Context(), bomapp.UpdateProductionBomGroupCategoryCommand{ID: id, Name: req.Name, SortOrder: req.SortOrder, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.DELETE("/api/production-bom-group-categories/:id", func(c echo.Context) error {
		return legacyProductionBomGroupsReadonlyAPI(c)
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid category_id"})
		}
		if err := bomSvc.DeleteProductionBomGroupCategory(c.Request().Context(), bomapp.DeleteProductionBomGroupCategoryCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/production-boms", func(c echo.Context) error {
		outputID, _ := strconv.ParseInt(c.QueryParam("output_id"), 10, 64)
		componentID, _ := strconv.ParseInt(c.QueryParam("component_id"), 10, 64)
		rows, err := bomSvc.ListProductionBoms(c.Request().Context(), bomapp.ProductionBomFilter{
			OutputType:    c.QueryParam("output_type"),
			OutputID:      outputID,
			ComponentType: c.QueryParam("component_type"),
			ComponentID:   componentID,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/production-bom-product-usage/:product_id", func(c echo.Context) error {
		productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil || productID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}
		rows, err := bomSvc.ListProductionBomUsageByProduct(c.Request().Context(), productID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.POST("/api/production-boms", func(c echo.Context) error {
		var req createProductionBomRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.CreateProductionBom(c.Request().Context(), bomapp.CreateProductionBomCommand{Name: req.Name, OutputType: req.OutputType, OutputID: req.OutputID, OutputProductID: req.OutputProductID, OutputMaterialID: req.OutputMaterialID, OutputQty: req.OutputQty, OutputUnit: req.OutputUnit, GroupID: req.GroupID, GroupCategoryID: req.GroupCategoryID, ExpectedLossRate: req.ExpectedLossRate, SpecTemplateVersionID: req.SpecTemplateVersionID, MainInputMaterialID: req.MainInputMaterialID, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/production-boms/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid bom_id"})
		}
		versionID, _ := strconv.ParseInt(c.QueryParam("version_id"), 10, 64)
		row, err := bomSvc.GetProductionBomDetail(c.Request().Context(), id, versionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return c.JSON(http.StatusNotFound, ErrorResponse{Error: "production bom not found"})
			}
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.PUT("/api/production-boms/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid bom_id"})
		}
		var req updateProductionBomRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		cmd := bomapp.UpdateProductionBomCommand{ID: id, Name: req.Name, OutputType: req.OutputType, OutputID: req.OutputID, OutputProductID: req.OutputProductID, OutputMaterialID: req.OutputMaterialID, OutputUnit: req.OutputUnit, Status: req.Status, Actor: support.ActorOf(c)}
		cmd.UpdateOutputBinding = req.OutputType != "" || req.OutputID > 0 || req.OutputProductID > 0 || req.OutputMaterialID > 0
		if req.GroupID != nil {
			cmd.GroupID = *req.GroupID
			cmd.UpdateGroupAssignment = true
		}
		if req.GroupCategoryID != nil {
			cmd.GroupCategoryID = *req.GroupCategoryID
			cmd.UpdateGroupAssignment = true
		}
		row, err := bomSvc.UpdateProductionBom(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.PUT("/api/production-boms/:id/draft-workspace", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid bom_id"})
		}
		var req draftWorkspaceRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		cmd := bomapp.ProductionBomDraftWorkspaceCommand{
			Bom:            bomapp.UpdateProductionBomCommand{ID: id, Name: req.Name, OutputType: req.OutputType, OutputID: req.OutputID, OutputProductID: req.OutputProductID, OutputMaterialID: req.OutputMaterialID, OutputUnit: req.OutputUnit, Status: req.Status, Actor: support.ActorOf(c), UpdateOutputBinding: req.OutputType != "" || req.OutputID > 0 || req.OutputProductID > 0 || req.OutputMaterialID > 0, SpecTemplateVersionID: req.SpecTemplateVersionID, MainInputMaterialID: req.MainInputMaterialID},
			Version:        bomapp.UpdateProductionBomVersionDraftCommand{VersionID: req.VersionID, ExpectedLossRate: req.ExpectedLossRate, MaterialLossRate: req.MaterialLossRate, OutputQty: req.OutputQty, OutputUnit: req.OutputUnit, ProcessRouteID: req.ProcessRouteID, Items: req.Items, Variants: req.Variants, SpecialAttrsSchemaJSON: req.SpecialAttrsSchemaJSON, SpecialAttrsJSON: req.SpecialAttrsJSON, Actor: support.ActorOf(c)},
			SpecTemplateID: req.SpecTemplateVersionID, MainInputMaterialID: req.MainInputMaterialID,
		}
		if req.GroupID != nil {
			cmd.Bom.GroupID, cmd.Bom.UpdateGroupAssignment = *req.GroupID, true
		}
		if req.GroupCategoryID != nil {
			cmd.Bom.GroupCategoryID, cmd.Bom.UpdateGroupAssignment = *req.GroupCategoryID, true
		}
		row, err := bomSvc.UpdateProductionBomDraftWorkspace(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/production-boms/:id/copy", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid bom_id"})
		}
		var req copyProductionBomRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.CopyProductionBom(c.Request().Context(), bomapp.CopyProductionBomCommand{ID: id, Name: req.Name, OutputType: req.OutputType, OutputID: req.OutputID, OutputProductID: req.OutputProductID, OutputMaterialID: req.OutputMaterialID, GroupID: req.GroupID, GroupCategoryID: req.GroupCategoryID, SpecTemplateVersionID: req.SpecTemplateVersionID, MainInputMaterialID: req.MainInputMaterialID, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/production-boms/:id/versions", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid bom_id"})
		}
		var req createProductionBomVersionRequest
		if err := c.Bind(&req); err != nil && !errors.Is(err, echo.ErrUnsupportedMediaType) {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.CreateProductionBomVersion(c.Request().Context(), bomapp.CreateProductionBomVersionCommand{BomID: id, SourceVersionID: req.SourceVersionID, Note: req.Note, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.PUT("/api/production-bom-versions/:id/draft", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid version_id"})
		}
		var req updateProductionBomVersionDraftRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.UpdateProductionBomVersionDraft(c.Request().Context(), bomapp.UpdateProductionBomVersionDraftCommand{VersionID: id, ExpectedLossRate: req.ExpectedLossRate, MaterialLossRate: req.MaterialLossRate, OutputQty: req.OutputQty, OutputUnit: req.OutputUnit, ProcessRouteID: req.ProcessRouteID, Items: req.Items, Variants: req.Variants, SpecialAttrsSchemaJSON: req.SpecialAttrsSchemaJSON, SpecialAttrsJSON: req.SpecialAttrsJSON, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/production-bom-versions/:id/spec-template", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid version_id"})
		}
		var req reapplyProductionBomSpecTemplateVersionRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := bomSvc.ReapplyProductionBomSpecTemplateVersion(c.Request().Context(), bomapp.ReapplyProductionBomSpecTemplateVersionCommand{
			VersionID: id, SpecTemplateVersionID: req.SpecTemplateVersionID, MainInputMaterialID: req.MainInputMaterialID, Actor: support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/production-bom-versions/:id/publish", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid version_id"})
		}
		if err := bomSvc.PublishProductionBomVersion(c.Request().Context(), bomapp.PublishProductionBomVersionCommand{VersionID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.PUT("/api/products/:id/default-production-bom", func(c echo.Context) error {
		return setDefaultProductionBomAPI(c, bomSvc)
	})

	e.PUT("/api/products/:id/production-bom-binding", func(c echo.Context) error {
		return setDefaultProductionBomAPI(c, bomSvc)
	})

	e.PUT("/api/materials/:id/default-production-bom", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid output_id"})
		}
		return setDefaultProductionBomOutputAPI(c, bomSvc, "material", id)
	})

	e.PUT("/api/production-bom-outputs/:type/:id/default-production-bom", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid output_id"})
		}
		return setDefaultProductionBomOutputAPI(c, bomSvc, c.Param("type"), id)
	})

	e.GET("/api/bom/list", func(c echo.Context) error {
		rows, err := bomSvc.List(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/bom/detail/:product_id", func(c echo.Context) error {
		productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}
		detail, err := bomSvc.Detail(c.Request().Context(), productID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return c.JSON(http.StatusNotFound, ErrorResponse{Error: "product not found"})
			}
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	})

	e.GET("/api/bom/products", func(c echo.Context) error {
		rows, err := bomSvc.Products(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/bom/materials", func(c echo.Context) error {
		rows, err := bomSvc.Materials(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/bom/bag-spec-mappings", func(c echo.Context) error {
		rows, err := bomSvc.BagSpecMappings(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/bom/versions", func(c echo.Context) error {
		productID, err := strconv.ParseInt(c.QueryParam("product_id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}
		rows, err := bomSvc.ListVersions(c.Request().Context(), productID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/bom/versions", func(c echo.Context) error {
		var req CreateBomVersionRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		version, err := bomSvc.CreateVersion(c.Request().Context(), bomapp.CreateVersionCommand{ProductID: req.ProductID, Note: req.Note, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, version)
	})

	e.POST("/api/bom/versions/:id/activate", func(c echo.Context) error {
		versionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || versionID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid version_id"})
		}
		if err := bomSvc.ActivateVersion(c.Request().Context(), bomapp.ActivateVersionCommand{VersionID: versionID, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/bom/save", func(c echo.Context) error {
		var req SaveBomRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if err := bomSvc.SyncProductYield(c.Request().Context(), bomapp.SyncProductYieldCommand{ProductID: req.ProductID, ExpectedLossRate: req.ExpectedLossRate, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	e.DELETE("/api/bom/:product_id", func(c echo.Context) error {
		productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}
		if err := bomSvc.DeactivateBom(c.Request().Context(), bomapp.DeactivateBomCommand{ProductID: productID, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/bom/:product_id/source", func(c echo.Context) error {
		productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil || productID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}
		var req bomapp.SetBomSourceCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		req.ProductID = productID
		req.Actor = support.ActorOf(c)
		detail, err := bomSvc.SetBomSource(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	})

	e.POST("/api/bom/:product_id/derive-owned", func(c echo.Context) error {
		productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil || productID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}
		detail, err := bomSvc.DeriveOwned(c.Request().Context(), bomapp.DeriveOwnedCommand{ProductID: productID, Actor: support.ActorOf(c)})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	})

	e.POST("/api/bom/item/save", func(c echo.Context) error {
		var req SaveBomItemRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		err := bomSvc.SaveItem(c.Request().Context(), bomapp.SaveItemCommand{
			ProductID:          req.ProductID,
			MaterialID:         req.MaterialID,
			ComponentType:      req.ComponentType,
			ComponentProductID: req.ComponentProductID,
			ComponentSpecG:     req.ComponentSpecG,
			ConsumeUnit:        req.ConsumeUnit,
			QtyPerUnit:         req.QtyPerUnit,
			RatioPct:           req.RatioPct,
			Actor:              support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	e.POST("/api/bom/item/delete", func(c echo.Context) error {
		var req DeleteBomItemRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if err := bomSvc.DeleteItem(c.Request().Context(), bomapp.DeleteItemCommand{ProductID: req.ProductID, ID: req.ID, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	e.POST("/api/bom/bag-spec-mappings/save", func(c echo.Context) error {
		var req SaveBagSpecMappingRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		err := bomSvc.SaveBagSpecMapping(c.Request().Context(), bomapp.SaveBagSpecMappingCommand{
			SpecG:      req.SpecG,
			MaterialID: req.MaterialID,
			Actor:      support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	e.POST("/api/bom/bag-spec-mappings/delete", func(c echo.Context) error {
		var req DeleteBagSpecMappingRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if err := bomSvc.DeleteBagSpecMapping(c.Request().Context(), bomapp.DeleteBagSpecMappingCommand{SpecG: req.SpecG, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})
}
