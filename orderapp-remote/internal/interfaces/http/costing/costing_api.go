package costing

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	appcosting "orderapp/internal/application/costing"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

func registerCostingAPI(e *echo.Echo, svc Service, authz support.AuthzService) {
	e.GET("/public/bean-list/:list_type", func(c echo.Context) error {
		listType := c.Param("list_type")
		productTypeCategoryID, err := parseOptionalInt64(c.QueryParam("product_type_category_id"))
		if err != nil {
			return c.HTML(http.StatusBadRequest, renderNoPublishedBeanListPage(listType))
		}
		row, err := svc.PublishedBeanList(c.Request().Context(), appcosting.BeanListPublicationQuery{
			ListType:              listType,
			ProductTypeCategoryID: productTypeCategoryID,
			OwnerType:             "official",
		})
		if err != nil {
			return c.HTML(http.StatusBadRequest, renderNoPublishedBeanListPage(listType))
		}
		if row == nil {
			return c.HTML(http.StatusNotFound, renderNoPublishedBeanListPage(listType))
		}
		page, err := renderPublicBeanListPage(*row)
		if err != nil {
			return c.HTML(http.StatusInternalServerError, renderNoPublishedBeanListPage(listType))
		}
		return c.HTML(http.StatusOK, page)
	})

	e.GET("/api/costing/parameters", func(c echo.Context) error {
		params, err := svc.Parameters(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, params)
	})

	e.GET("/api/costing/settings", func(c echo.Context) error {
		rows, err := svc.Settings(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/costing/settings/:key", func(c echo.Context) error {
		var req struct {
			Value float64 `json:"value"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		row, err := svc.UpdateSetting(c.Request().Context(), appcosting.UpdateParameterCommand{
			Key:   c.Param("key"),
			Value: req.Value,
			Actor: support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/costing/calculate", func(c echo.Context) error {
		var req appcosting.CalculateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		resp, err := svc.Calculate(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.POST("/api/costing/price-explanation", func(c echo.Context) error {
		var req appcosting.PriceExplanationCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		resp, err := svc.ExplainPrice(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.POST("/api/costing/drip-price-explanation", func(c echo.Context) error {
		var req appcosting.DripPriceExplanationCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		resp, err := svc.ExplainDripPrice(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.GET("/api/drip-price-templates", func(c echo.Context) error {
		rows, err := svc.ListDripPriceTemplates(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/drip-price-templates", func(c echo.Context) error {
		var req appcosting.SaveDripPriceTemplateCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.Actor = support.ActorOf(c)
		row, err := svc.SaveDripPriceTemplate(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.PUT("/api/drip-price-templates/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
		}
		var req appcosting.SaveDripPriceTemplateCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.ID = id
		req.Actor = support.ActorOf(c)
		row, err := svc.SaveDripPriceTemplate(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/drip-price-templates/:id/deactivate", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
		}
		if err := svc.DeactivateDripPriceTemplate(c.Request().Context(), appcosting.DeactivateDripPriceTemplateCommand{
			ID:    id,
			Actor: support.ActorOf(c),
		}); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "id": id})
	})

	e.GET("/api/costing/bean-list", func(c echo.Context) error {
		customerID, err := parseOptionalInt64(c.QueryParam("customer_id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid customer_id"})
		}
		resp, err := svc.BeanList(c.Request().Context(), appcosting.BeanListQuery{CustomerID: customerID})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.GET("/api/costing/bean-list/publications", func(c echo.Context) error {
		query, err := beanListPublicationQueryFromRequest(c)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		rows, err := svc.ListBeanListPublications(c.Request().Context(), query)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/costing/bean-list/publications", func(c echo.Context) error {
		if err := requireBeanListPublisher(c, authz); err != nil {
			return err
		}
		var req appcosting.PublishBeanListCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.Actor = support.ActorOf(c)
		ownerType, ownerKey, err := beanListOwnerFromScope(c, req.Scope, req.CustomerID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		req.OwnerType = ownerType
		req.OwnerKey = ownerKey
		row, err := svc.PublishBeanList(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := generateBeanListPublicationPDFAsset(c, svc, row); err != nil {
			return beanListPDFError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/costing/bean-list/drafts", func(c echo.Context) error {
		canPublish, err := currentActorCanPublishBeanList(c, authz)
		if err != nil {
			return err
		}
		var req appcosting.PublishBeanListCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.Actor = support.ActorOf(c)
		scope := req.Scope
		if !canPublish {
			scope = "mine"
			req.Scope = scope
		}
		ownerType, ownerKey, err := beanListOwnerFromScope(c, scope, req.CustomerID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		req.OwnerType = ownerType
		req.OwnerKey = ownerKey
		row, err := svc.SaveBeanListDraft(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := generateBeanListPublicationPDFAsset(c, svc, row); err != nil {
			return beanListPDFError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/costing/bean-list/publications/:id/pdf", func(c echo.Context) error {
		cmd, err := beanListPublicationPDFCommandFromRequest(c)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		cmd.Actor = support.ActorOf(c)
		file, err := svc.GenerateBeanListPublicationPDF(c.Request().Context(), cmd, renderBeanListPublicationPDF)
		if err != nil {
			return beanListPDFError(c, err)
		}
		file.DownloadURL = beanListPublicationPDFDownloadURL(cmd)
		file.Payload = nil
		return c.JSON(http.StatusOK, file)
	})

	e.GET("/api/costing/bean-list/publications/:id/pdf", func(c echo.Context) error {
		cmd, err := beanListPublicationPDFCommandFromRequest(c)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		file, err := svc.LoadBeanListPublicationPDF(c.Request().Context(), cmd)
		if err != nil {
			return beanListPDFError(c, err)
		}
		contentType := strings.TrimSpace(file.ContentType)
		if contentType == "" {
			contentType = "application/pdf"
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(file.Filename, `"`, "")))
		return c.Blob(http.StatusOK, contentType, file.Payload)
	})

	e.POST("/api/costing/bean-list/publications/:id/withdraw", func(c echo.Context) error {
		if err := requireBeanListPublisher(c, authz); err != nil {
			return err
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
		}
		query, err := beanListPublicationQueryFromRequest(c)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := svc.WithdrawBeanList(c.Request().Context(), appcosting.WithdrawBeanListCommand{
			ID:        id,
			OwnerType: query.OwnerType,
			OwnerKey:  query.OwnerKey,
			Actor:     support.ActorOf(c),
		}); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "id": id})
	})

	e.POST("/api/costing/runs", func(c echo.Context) error {
		run, err := svc.CreateRun(c.Request().Context(), support.ActorOf(c))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, run)
	})

	e.POST("/api/costing/runs/:id/publish", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
		}
		if err := svc.PublishRun(c.Request().Context(), support.ActorOf(c), id); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "id": id})
	})
}

func generateBeanListPublicationPDFAsset(c echo.Context, svc Service, row *appcosting.BeanListPublication) error {
	if row == nil || row.ID <= 0 {
		return nil
	}
	_, err := svc.GenerateBeanListPublicationPDF(c.Request().Context(), appcosting.BeanListPublicationPDFCommand{
		PublicationID: row.ID,
		Query: appcosting.BeanListPublicationQuery{
			ListType:              row.ListType,
			ProductTypeCategoryID: row.ProductTypeCategoryID,
			OwnerType:             row.OwnerType,
			OwnerKey:              row.OwnerKey,
		},
		Actor: support.ActorOf(c),
	}, renderBeanListPublicationPDF)
	return err
}

func beanListPublicationPDFCommandFromRequest(c echo.Context) (appcosting.BeanListPublicationPDFCommand, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return appcosting.BeanListPublicationPDFCommand{}, fmt.Errorf("invalid id")
	}
	query, err := beanListPublicationQueryFromRequest(c)
	if err != nil {
		return appcosting.BeanListPublicationPDFCommand{}, err
	}
	return appcosting.BeanListPublicationPDFCommand{
		PublicationID: id,
		Query:         query,
	}, nil
}

func beanListPublicationPDFDownloadURL(cmd appcosting.BeanListPublicationPDFCommand) string {
	params := url.Values{}
	listType := strings.TrimSpace(cmd.Query.ListType)
	if listType == "" {
		listType = "commercial"
	}
	params.Set("list_type", listType)
	scope := strings.TrimSpace(cmd.Query.Scope)
	if scope == "" {
		scope = "official"
	}
	params.Set("scope", scope)
	if scope == "customer" {
		params.Set("customer_id", strconv.FormatInt(cmd.Query.CustomerID, 10))
	}
	if cmd.Query.ProductTypeCategoryID > 0 {
		params.Set("product_type_category_id", strconv.FormatInt(cmd.Query.ProductTypeCategoryID, 10))
	}
	return fmt.Sprintf("/api/costing/bean-list/publications/%d/pdf?%s", cmd.PublicationID, params.Encode())
}

func beanListPDFError(c echo.Context, err error) error {
	if errors.Is(err, appcosting.ErrBeanListPublicationNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "bean list PDF not found"})
	}
	return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func requireBeanListPublisher(c echo.Context, authz support.AuthzService) error {
	canPublish, err := currentActorCanPublishBeanList(c, authz)
	if err != nil {
		return err
	}
	if !canPublish {
		return echo.NewHTTPError(http.StatusForbidden, map[string]string{"error": "only admins can publish bean lists"})
	}
	return nil
}

func currentActorCanPublishBeanList(c echo.Context, authz support.AuthzService) (bool, error) {
	actor, ok, err := support.CurrentActor(c, authz)
	if err != nil {
		return false, echo.NewHTTPError(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	if !ok {
		return false, echo.NewHTTPError(http.StatusUnauthorized, map[string]string{"error": "auth required"})
	}
	return actor.IsAdmin() || actor.Can("auth.manage"), nil
}

func beanListPublicationQueryFromRequest(c echo.Context) (appcosting.BeanListPublicationQuery, error) {
	customerID, err := parseOptionalInt64(c.QueryParam("customer_id"))
	if err != nil {
		return appcosting.BeanListPublicationQuery{}, fmt.Errorf("invalid customer_id")
	}
	productTypeCategoryID, err := parseOptionalInt64(c.QueryParam("product_type_category_id"))
	if err != nil {
		return appcosting.BeanListPublicationQuery{}, fmt.Errorf("invalid product_type_category_id")
	}
	scope := strings.TrimSpace(c.QueryParam("scope"))
	ownerType, ownerKey, err := beanListOwnerFromScope(c, scope, customerID)
	if err != nil {
		return appcosting.BeanListPublicationQuery{}, err
	}
	return appcosting.BeanListPublicationQuery{
		ListType:              c.QueryParam("list_type"),
		ProductTypeCategoryID: productTypeCategoryID,
		Scope:                 scope,
		CustomerID:            customerID,
		OwnerType:             ownerType,
		OwnerKey:              ownerKey,
	}, nil
}

func beanListOwnerFromScope(c echo.Context, scope string, customerID int64) (string, string, error) {
	switch strings.TrimSpace(scope) {
	case "", "official":
		return "official", "", nil
	case "customer":
		if customerID <= 0 {
			return "", "", fmt.Errorf("customer_id required")
		}
		return "customer", strconv.FormatInt(customerID, 10), nil
	case "mine":
		if v := c.Get("employee_id"); v != nil {
			if id, ok := v.(int64); ok && id > 0 {
				return "actor", fmt.Sprintf("employee:%d", id), nil
			}
		}
		return "actor", "actor:" + support.ActorOf(c), nil
	default:
		return "", "", fmt.Errorf("invalid scope")
	}
}

func parseOptionalInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
