package materials

import (
	"encoding/json"
	"fmt"
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
	OwnerType            string `json:"owner_type"`
	OwnerCustomerID      int64  `json:"owner_customer_id"`
	CopiedFromMaterialID int64  `json:"copied_from_material_id"`
}

func (r *materialCreateAPIRequest) UnmarshalJSON(data []byte) error {
	var input materialsapp.MaterialInput
	if err := json.Unmarshal(data, &input); err != nil {
		return err
	}
	var extra struct {
		OwnerType       string  `json:"owner_type"`
		OwnerCustomerID int64   `json:"owner_customer_id"`
		CopiedFromID    int64   `json:"copied_from_material_id"`
		CustomerIDs     []int64 `json:"customer_ids"`
	}
	if err := json.Unmarshal(data, &extra); err != nil {
		return err
	}
	r.MaterialInput = input
	if len(extra.CustomerIDs) > 0 {
		return fmt.Errorf("多客户关联已取消，请为物料选择唯一归属")
	}
	r.OwnerType = extra.OwnerType
	r.OwnerCustomerID = extra.OwnerCustomerID
	r.CopiedFromMaterialID = extra.CopiedFromID
	return nil
}

func registerMaterialsAPI(e *echo.Echo, materialsSvc *materialsapp.Service) {
	canAccessMaterial := func(c echo.Context, materialID int64) (bool, error) {
		boundCustomerID, err := materialsSvc.ResolveBoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if err != nil {
			return false, err
		}
		if boundCustomerID <= 0 {
			return true, nil
		}
		rows, err := materialsSvc.List(c.Request().Context(), materialsapp.ListCommand{Active: "all", Limit: 500, IncludeDeprecated: true, OwnerType: materialsapp.OwnerTypeCustomer, CustomerID: boundCustomerID})
		if err != nil {
			return false, err
		}
		for _, row := range rows {
			if row.ID == materialID {
				return true, nil
			}
		}
		return false, nil
	}
	e.GET("/api/materials", func(c echo.Context) error {
		limit := support.IntParam(c, "limit", 200)
		active := strings.TrimSpace(c.QueryParam("active"))
		includeDeprecated := strings.TrimSpace(c.QueryParam("include_deprecated")) == "1"
		if active == "all" {
			includeDeprecated = true
		}
		customerID := int64(support.IntParam(c, "owner_customer_id", 0))
		ownerType := strings.ToLower(strings.TrimSpace(c.QueryParam("owner_type")))
		if customerID == 0 {
			customerID = int64(support.IntParam(c, "customer_id", 0))
		}
		boundCustomerID, err := materialsSvc.ResolveBoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if err != nil {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: err.Error()})
		}
		if boundCustomerID > 0 {
			if customerID > 0 && customerID != boundCustomerID {
				return c.JSON(http.StatusForbidden, ErrorResponse{Error: "customer material scope forbidden"})
			}
			customerID = boundCustomerID
			ownerType = materialsapp.OwnerTypeCustomer
		}
		if ownerType == materialsapp.OwnerTypeCustomer && customerID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "客户归属筛选必须指定一个客户"})
		}
		rows, err := materialsSvc.List(c.Request().Context(), materialsapp.ListCommand{
			Query:             strings.TrimSpace(c.QueryParam("q")),
			Active:            active,
			Limit:             limit,
			IncludeDeprecated: includeDeprecated,
			CustomerID:        customerID,
			OwnerType:         ownerType,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, MaterialListResponse{Rows: rows})
	})

	e.POST("/api/materials", func(c echo.Context) error {
		var req materialCreateAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		boundCustomerID, err := materialsSvc.ResolveBoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if err != nil {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: err.Error()})
		}
		if boundCustomerID > 0 && (req.OwnerType != materialsapp.OwnerTypeCustomer || req.OwnerCustomerID != boundCustomerID) {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: "只能创建当前客户归属的物料"})
		}
		row, err := materialsSvc.Create(c.Request().Context(), materialsapp.CreateCommand{
			Actor:                support.ActorOf(c),
			Input:                req.MaterialInput,
			OwnerType:            req.OwnerType,
			OwnerCustomerID:      req.OwnerCustomerID,
			CopiedFromMaterialID: req.CopiedFromMaterialID,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	retiredMaterialCustomerReferences := func(c echo.Context) error {
		return c.JSON(http.StatusGone, ErrorResponse{Error: "物料客户关联接口已下线；物料现在只有一个明确归属"})
	}
	e.GET("/api/material-customer-references", retiredMaterialCustomerReferences)
	e.POST("/api/material-customer-references", retiredMaterialCustomerReferences)
	e.PUT("/api/material-customer-references/:id", retiredMaterialCustomerReferences)

	e.POST("/api/materials/:id/owner", func(c echo.Context) error {
		id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		boundCustomerID, err := materialsSvc.ResolveBoundCustomerID(c.Request().Context(), support.CurrentEmployeeID(c))
		if err != nil {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: err.Error()})
		}
		if boundCustomerID > 0 {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: "客户账号不能调整物料归属"})
		}
		var req struct {
			OwnerType       string `json:"owner_type"`
			OwnerCustomerID int64  `json:"owner_customer_id"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := materialsSvc.ChangeOwner(c.Request().Context(), materialsapp.ChangeOwnerCommand{Actor: support.ActorOf(c), ID: id, OwnerType: req.OwnerType, OwnerCustomerID: req.OwnerCustomerID})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/materials/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		}
		allowed, accessErr := canAccessMaterial(c, id)
		if accessErr != nil {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: accessErr.Error()})
		}
		if !allowed {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: "不能修改其他客户的物料档案"})
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
		allowed, accessErr := canAccessMaterial(c, id)
		if accessErr != nil {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: accessErr.Error()})
		}
		if !allowed {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: "不能修改其他客户的物料档案"})
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
		for _, materialID := range req.MaterialIDs {
			allowed, accessErr := canAccessMaterial(c, materialID)
			if accessErr != nil {
				return c.JSON(http.StatusForbidden, ErrorResponse{Error: accessErr.Error()})
			}
			if !allowed {
				return c.JSON(http.StatusForbidden, ErrorResponse{Error: "不能修改其他客户的物料档案"})
			}
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
