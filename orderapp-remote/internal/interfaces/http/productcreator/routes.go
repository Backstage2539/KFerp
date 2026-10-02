package productcreator

import (
	"errors"
	"net/http"
	"sort"
	"strconv"

	app "orderapp/internal/application/productcreator"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Creator *app.Service
	Authz   support.AuthzService
}

func RegisterRoutes(e *echo.Echo, dependencies Dependencies) {
	service := dependencies.Creator
	e.GET("/api/product-creator/modules", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"modules": app.ModuleCatalog()})
	})
	e.GET("/api/product-creator/templates", func(c echo.Context) error {
		rows, err := service.ListTemplates(c.Request().Context())
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/product-creator/templates", func(c echo.Context) error {
		var request struct {
			Name        string       `json:"name"`
			Description string       `json:"description"`
			Workflow    app.Workflow `json:"workflow"`
		}
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		}
		row, err := service.SaveTemplate(c.Request().Context(), app.TemplateSave{Name: request.Name, Description: request.Description, Workflow: request.Workflow, Actor: support.ActorOf(c)})
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusCreated, row)
	})
	e.PUT("/api/product-creator/templates/:id", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		}
		var request struct {
			Revision    int64        `json:"revision"`
			Name        string       `json:"name"`
			Description string       `json:"description"`
			Workflow    app.Workflow `json:"workflow"`
		}
		if err := c.Bind(&request); err != nil || request.Revision <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "revision and template data required"})
		}
		row, err := service.SaveTemplate(c.Request().Context(), app.TemplateSave{ID: id, ExpectedRev: request.Revision, Name: request.Name, Description: request.Description, Workflow: request.Workflow, Actor: support.ActorOf(c)})
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})
	e.POST("/api/product-creator/templates/:id/copy", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		}
		var request struct {
			Name string `json:"name"`
		}
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		}
		row, err := service.CopyTemplate(c.Request().Context(), id, request.Name, support.ActorOf(c))
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusCreated, row)
	})
	e.POST("/api/product-creator/templates/:id/validate", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		}
		issues, err := service.ValidateTemplate(c.Request().Context(), id)
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"valid": len(issues) == 0, "issues": issues})
	})
	e.POST("/api/product-creator/templates/:id/publish", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		}
		var request struct {
			Revision int64 `json:"revision"`
		}
		if err := c.Bind(&request); err != nil || request.Revision <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "revision required"})
		}
		row, err := service.PublishTemplate(c.Request().Context(), id, request.Revision, support.ActorOf(c))
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusCreated, row)
	})
	e.POST("/api/product-creator/templates/:id/disable", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		}
		var request struct {
			Revision int64 `json:"revision"`
		}
		if err := c.Bind(&request); err != nil || request.Revision <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "revision required"})
		}
		if err := service.DisableTemplate(c.Request().Context(), id, request.Revision, support.ActorOf(c)); err != nil {
			return productCreatorError(c, err)
		}
		return c.NoContent(http.StatusNoContent)
	})
	e.GET("/api/product-creator/templates/:id/versions", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		}
		rows, err := service.ListVersions(c.Request().Context(), id)
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.GET("/api/product-creator/templates/:id/runs", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		}
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		rows, err := service.ListTemplateRuns(c.Request().Context(), id, limit)
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.GET("/api/product-creator/runs/recent", func(c echo.Context) error {
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		rows, err := service.ListTemplateRuns(c.Request().Context(), 0, limit)
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/product-creator/runs", func(c echo.Context) error {
		var request struct {
			TemplateID int64 `json:"template_id"`
		}
		if err := c.Bind(&request); err != nil || request.TemplateID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "template_id required"})
		}
		row, err := service.StartRun(c.Request().Context(), request.TemplateID, support.ActorOf(c))
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusCreated, row)
	})
	e.GET("/api/product-creator/runs/:id", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid run id"})
		}
		row, err := service.GetRun(c.Request().Context(), id)
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})
	e.PUT("/api/product-creator/runs/:id/draft", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid run id"})
		}
		var request struct {
			Revision       int64                     `json:"revision"`
			Inputs         map[string]map[string]any `json:"inputs"`
			VariableValues map[string]string         `json:"variable_values"`
		}
		if err := c.Bind(&request); err != nil || request.Revision <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "revision and inputs required"})
		}
		row, err := service.SaveRunDraft(c.Request().Context(), id, request.Revision, request.Inputs, request.VariableValues, support.ActorOf(c))
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})
	e.POST("/api/product-creator/runs/:id/preview", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid run id"})
		}
		var request struct {
			Revision int64 `json:"revision"`
		}
		if err := c.Bind(&request); err != nil || request.Revision <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "revision required"})
		}
		row, err := service.PreviewRun(c.Request().Context(), id, request.Revision, support.ActorOf(c))
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})
	e.POST("/api/product-creator/runs/:id/commit", func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid run id"})
		}
		var request struct {
			Revision int64 `json:"revision"`
		}
		idempotencyKey := c.Request().Header.Get("Idempotency-Key")
		if err := c.Bind(&request); err != nil || request.Revision <= 0 || idempotencyKey == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "revision and idempotency key are required"})
		}
		run, err := service.GetRun(c.Request().Context(), id)
		if err != nil {
			return productCreatorError(c, err)
		}
		if err := requireCreatorPermissions(c, dependencies.Authz, configurationRunPermissions(run)); err != nil {
			return err
		}
		row, err := service.CommitConfiguration(c.Request().Context(), id, request.Revision, idempotencyKey, support.ActorOf(c))
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})
	executeRunStep := func(c echo.Context) error {
		id, ok := positiveID(c, "id")
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid run id"})
		}
		var request struct {
			Revision int64          `json:"revision"`
			Action   string         `json:"action"`
			Inputs   map[string]any `json:"inputs"`
		}
		idempotencyKey := c.Request().Header.Get("Idempotency-Key")
		if err := c.Bind(&request); err != nil || request.Revision <= 0 || request.Action == "" || idempotencyKey == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "revision, action, and idempotency key are required"})
		}
		run, err := service.GetRun(c.Request().Context(), id)
		if err != nil {
			return productCreatorError(c, err)
		}
		var nodeKind app.ModuleKind
		for _, node := range run.Workflow.Nodes {
			if node.ID == c.Param("node_id") {
				nodeKind = node.Kind
				break
			}
		}
		permissions := runStepPermissions(nodeKind, request.Action)
		if err := requireCreatorPermissions(c, dependencies.Authz, permissions); err != nil {
			return err
		}
		row, err := service.ExecuteRunStep(c.Request().Context(), id, request.Revision, c.Param("node_id"), request.Action, idempotencyKey, support.ActorOf(c), request.Inputs)
		if err != nil {
			return productCreatorError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	}
	e.POST("/api/product-creator/runs/:id/nodes/:node_id/execute", executeRunStep)
	e.POST("/api/product-creator/pricing/runs/:id/nodes/:node_id/execute", executeRunStep)
}

func configurationPermissions(workflow app.Workflow) []string {
	permissions := map[app.ModuleKind]string{
		app.ModuleProduct: "products.write", app.ModuleMaterial: "materials.write",
		app.ModuleBOM: "bom.write", app.ModuleProcess: "bom.write", app.ModulePublish: "bom.write",
	}
	set := map[string]bool{}
	for _, node := range workflow.Nodes {
		if permission := permissions[node.Kind]; permission != "" {
			set[permission] = true
		}
	}
	return sortedPermissionKeys(set)
}

func runStepPermissions(kind app.ModuleKind, action string) []string {
	switch kind {
	case app.ModulePurchase:
		if action == "confirm_receipt" {
			return []string{"purchase.write", "stock.write"}
		}
		return []string{"purchase.write"}
	case app.ModulePricing:
		return []string{"costing.write"}
	default:
		return configurationPermissions(app.Workflow{Nodes: []app.Node{{Kind: kind}}})
	}
}

func requireCreatorPermissions(c echo.Context, authz support.AuthzService, permissions []string) error {
	if len(permissions) == 0 {
		return nil
	}
	actor, ok, err := support.CurrentActor(c, authz)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "auth required"})
	}
	for _, permission := range permissions {
		if !actor.Can(permission) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "permission denied", "permission": permission})
		}
	}
	return nil
}

func sortedPermissionKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for permission := range set {
		out = append(out, permission)
	}
	sort.Strings(out)
	return out
}

func positiveID(c echo.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	return id, err == nil && id > 0
}

func productCreatorError(c echo.Context, err error) error {
	var invalid app.InvalidWorkflowError
	var execution app.ExecutionError
	switch {
	case errors.As(err, &invalid):
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "issues": invalid.Issues})
	case errors.As(err, &execution):
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{"error": err.Error(), "issues": execution.Issues})
	case errors.Is(err, app.ErrConflict):
		return c.JSON(http.StatusConflict, map[string]string{"error": "revision conflict; reload the latest draft"})
	case errors.Is(err, app.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, app.ErrNotPublished):
		return c.JSON(http.StatusConflict, map[string]string{"error": "template has no published version"})
	case errors.Is(err, app.ErrConfigurationExecutorUnavailable):
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
}

func configurationRunPermissions(run app.Run) []string {
	if run.Workflow.Version < 8 {
		return configurationPermissions(run.Workflow)
	}
	inputs := app.ResolveWorkflowInputDefaults(run.Workflow, run.Inputs)
	plan := app.BuildExecutionPlan(run.Workflow, inputs)
	set := map[string]bool{}
	for _, n := range run.Workflow.Nodes {
		if !plan.Active(n.ID) {
			continue
		}
		if app.IsReusedOutput(run.Workflow, n, inputs) {
			if n.Kind == app.ModuleProduct {
				set["products.read"] = true
			} else {
				set["materials.read"] = true
			}
			continue
		}
		if n.Kind == app.ModuleMaterial && n.Config["data_role"] != "output" {
			set["materials.read"] = true
			if rows, ok := inputs[n.ID]["rows"].([]any); ok {
				for _, r := range rows {
					if row, ok := r.(map[string]any); ok && row["action"] != "reuse" {
						set["materials.write"] = true
					}
				}
			}
			continue
		}
		if n.Kind == app.ModuleProduct && n.Config["data_role"] != "output" {
			set["products.read"] = true
			continue
		}
		if n.Kind == app.ModuleProcess {
			set["bom.read"] = true
			continue
		}
		for _, p := range configurationPermissions(app.Workflow{Nodes: []app.Node{n}}) {
			set[p] = true
		}
	}
	return sortedPermissionKeys(set)
}
