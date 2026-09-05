package materials

import (
	"encoding/json"
	"net/http"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	materialsapp "orderapp/internal/application/materials"

	"github.com/labstack/echo/v4"
)

type MaterialListResponse struct {
	Rows []materialsapp.Material `json:"rows"`
}

type materialCreateAPIRequest struct {
	materialsapp.MaterialInput
	CustomerIDs []int64 `json:"customer_ids"`
}

func (r *materialCreateAPIRequest) UnmarshalJSON(data []byte) error {
	var input materialsapp.MaterialInput
	if err := json.Unmarshal(data, &input); err != nil {
		return err
	}
	var extra struct {
		CustomerIDs []int64 `json:"customer_ids"`
	}
	if err := json.Unmarshal(data, &extra); err != nil {
		return err
	}
	r.MaterialInput = input
	r.CustomerIDs = extra.CustomerIDs
	return nil
}

func registerMaterialsAPI(e *echo.Echo, materialsSvc *materialsapp.Service) {
	e.GET("/api/materials", func(c echo.Context) error {
		limit := support.IntParam(c, "limit", 200)
		active := strings.TrimSpace(c.QueryParam("active"))
		includeDeprecated := strings.TrimSpace(c.QueryParam("include_deprecated")) == "1"
		if active == "all" {
			includeDeprecated = true
		}
		customerID := int64(support.IntParam(c, "customer_id", 0))
		boundCustomerID, err := materialsSvc.ResolveBoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if err != nil {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: err.Error()})
		}
		if boundCustomerID > 0 {
			if customerID > 0 && customerID != boundCustomerID {
				return c.JSON(http.StatusForbidden, ErrorResponse{Error: "customer material scope forbidden"})
			}
			customerID = boundCustomerID
		}
		rows, err := materialsSvc.List(c.Request().Context(), materialsapp.ListCommand{
			Query:             strings.TrimSpace(c.QueryParam("q")),
			Active:            active,
			Limit:             limit,
			IncludeDeprecated: includeDeprecated,
			CustomerID:        customerID,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, MaterialListResponse{Rows: rows})
	})

	e.POST("/api/materials", func(c echo.Context) error {
		var req materialCreateAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := materialsSvc.Create(c.Request().Context(), materialsapp.CreateCommand{
			Actor:       support.ActorOf(c),
			Input:       req.MaterialInput,
			CustomerIDs: req.CustomerIDs,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/material-customer-references", func(c echo.Context) error {
		materialID := int64(support.IntParam(c, "material_id", 0))
		customerID := int64(support.IntParam(c, "customer_id", 0))
		boundCustomerID, err := materialsSvc.ResolveBoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if err != nil {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: err.Error()})
		}
		if boundCustomerID > 0 {
			if customerID > 0 && customerID != boundCustomerID {
				return c.JSON(http.StatusForbidden, ErrorResponse{Error: "customer material scope forbidden"})
			}
			customerID = boundCustomerID
		}
		rows, err := materialsSvc.ListCustomerReferences(c.Request().Context(), materialID, customerID, c.QueryParam("active"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows, "references": rows})
	})

	saveMaterialCustomerReference := func(c echo.Context) error {
		var req materialsapp.SaveMaterialCustomerReferenceCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if strings.TrimSpace(c.Param("id")) == "" {
			id, err = 0, nil
		}
		if err != nil || id < 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		req.ID = id
		req.Actor = support.ActorOf(c)
		row, err := materialsSvc.SaveCustomerReference(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"reference": row})
	}
	e.POST("/api/material-customer-references", saveMaterialCustomerReference)
	e.PUT("/api/material-customer-references/:id", saveMaterialCustomerReference)

	e.POST("/api/materials/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		var req materialsapp.MaterialInput
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := materialsSvc.Update(c.Request().Context(), materialsapp.UpdateCommand{
			Actor: support.ActorOf(c),
			ID:    id,
			Input: req,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/materials/:id/deprecate", func(c echo.Context) error {
		id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		row, err := materialsSvc.Deprecate(c.Request().Context(), materialsapp.DeprecateCommand{
			Actor: support.ActorOf(c),
			ID:    id,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/material-classification-groups", func(c echo.Context) error {
		rows, err := materialsSvc.ListClassificationGroups(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/material-classification-groups", func(c echo.Context) error {
		var req struct {
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := materialsSvc.SaveClassificationGroup(c.Request().Context(), materialsapp.SaveClassificationGroupCommand{
			Actor:     support.ActorOf(c),
			Name:      req.Name,
			SortOrder: req.SortOrder,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.PUT("/api/material-classification-groups/:id", func(c echo.Context) error {
		id, err := parsePositiveID(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		var req struct {
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := materialsSvc.SaveClassificationGroup(c.Request().Context(), materialsapp.SaveClassificationGroupCommand{
			Actor:     support.ActorOf(c),
			ID:        id,
			Name:      req.Name,
			SortOrder: req.SortOrder,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.DELETE("/api/material-classification-groups/:id", func(c echo.Context) error {
		id, err := parsePositiveID(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		if err := materialsSvc.DeleteClassificationGroup(c.Request().Context(), materialsapp.DeleteClassificationGroupCommand{Actor: support.ActorOf(c), ID: id}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/material-classification-groups/:group_id/categories", func(c echo.Context) error {
		groupID, err := parsePositiveID(c.Param("group_id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid group_id"})
		}
		var req struct {
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := materialsSvc.SaveClassificationCategory(c.Request().Context(), materialsapp.SaveClassificationCategoryCommand{
			Actor:     support.ActorOf(c),
			GroupID:   groupID,
			Name:      req.Name,
			SortOrder: req.SortOrder,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.PUT("/api/material-classification-group-categories/:id", func(c echo.Context) error {
		id, err := parsePositiveID(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		var req struct {
			GroupID   int64  `json:"group_id"`
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := materialsSvc.SaveClassificationCategory(c.Request().Context(), materialsapp.SaveClassificationCategoryCommand{
			Actor:     support.ActorOf(c),
			ID:        id,
			GroupID:   req.GroupID,
			Name:      req.Name,
			SortOrder: req.SortOrder,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.DELETE("/api/material-classification-group-categories/:id", func(c echo.Context) error {
		id, err := parsePositiveID(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		if err := materialsSvc.DeleteClassificationCategory(c.Request().Context(), materialsapp.DeleteClassificationCategoryCommand{Actor: support.ActorOf(c), ID: id}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/material-classification-assignments", func(c echo.Context) error {
		var req struct {
			MaterialIDs []int64 `json:"material_ids"`
			GroupID     int64   `json:"group_id"`
			CategoryID  int64   `json:"category_id"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if err := materialsSvc.AssignClassification(c.Request().Context(), materialsapp.AssignClassificationCommand{
			Actor:       support.ActorOf(c),
			MaterialIDs: req.MaterialIDs,
			GroupID:     req.GroupID,
			CategoryID:  req.CategoryID,
		}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}

func parsePositiveID(v string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}
