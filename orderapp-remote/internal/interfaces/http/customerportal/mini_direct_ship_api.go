package customerportal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"
	excelinfra "orderapp/internal/infrastructure/excel"
	pdfinfra "orderapp/internal/infrastructure/pdf"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type MiniCustomerFulfillment interface {
	MiniDirectShipCatalog(context.Context, customerfulfillmentapp.MiniDirectShipCatalogQuery) (customerfulfillmentapp.MiniDirectShipCatalog, error)
	PreviewMiniDirectShip(context.Context, customerfulfillmentapp.MiniDirectShipCommand) (customerfulfillmentapp.MiniDirectShipPreview, error)
	SubmitMiniDirectShip(context.Context, customerfulfillmentapp.MiniDirectShipCommand) (customerfulfillmentapp.MiniDirectShipRequest, error)
	ListMiniDirectShipRequests(context.Context, customerfulfillmentapp.MiniDirectShipListQuery) (customerfulfillmentapp.MiniDirectShipListResult, error)
	GetMiniDirectShipRequest(context.Context, int64, int64) (customerfulfillmentapp.MiniDirectShipRequest, error)
	CancelMiniDirectShipRequest(context.Context, int64, int64, string) (customerfulfillmentapp.MiniDirectShipRequest, error)
	ListCustomerCentralInventory(context.Context, customerfulfillmentapp.CustomerInventoryListQuery) (customerfulfillmentapp.CustomerInventoryListResult, error)
	ListCustomerCentralInventoryBatches(context.Context, customerfulfillmentapp.CustomerInventoryBatchQuery) ([]customerfulfillmentapp.CustomerInventoryBatch, error)
	ListCustomerAssetInventory(context.Context, customerfulfillmentapp.CustomerAssetInventoryQuery) ([]customerfulfillmentapp.CustomerAssetInventory, error)
	ListCustomerAssetInventoryLedger(context.Context, customerfulfillmentapp.CustomerAssetInventoryLedgerQuery) ([]customerfulfillmentapp.CustomerAssetInventoryLedgerEntry, error)
	CustomerAccount(context.Context, customerfulfillmentapp.AccountQuery) (customerfulfillmentapp.AccountData, error)
	ConfirmCustomerStatement(context.Context, customerfulfillmentapp.ConfirmCustomerStatementCommand) (customerfulfillmentapp.AccountSettlement, error)
	CreateCustomerStatementDispute(context.Context, customerfulfillmentapp.CreateCustomerStatementDisputeCommand) (customerfulfillmentapp.AccountStatementDispute, error)
	ReplyCustomerStatementDispute(context.Context, customerfulfillmentapp.ReplyCustomerStatementDisputeCommand) (customerfulfillmentapp.AccountStatementDispute, error)
}

type MiniProductOrderFulfillment interface {
	MiniProductOrderCatalog(context.Context, customerfulfillmentapp.MiniDirectShipCatalogQuery) (customerfulfillmentapp.MiniDirectShipCatalog, error)
	PreviewMiniProductOrder(context.Context, customerfulfillmentapp.MiniDirectShipCommand) (customerfulfillmentapp.MiniDirectShipPreview, error)
	SubmitMiniProductOrder(context.Context, customerfulfillmentapp.MiniDirectShipCommand) (customerfulfillmentapp.MiniDirectShipRequest, error)
}

func registerMiniCustomerFulfillmentAPI(e *echo.Echo, portal Service, fulfillment MiniCustomerFulfillment) {
	e.GET("/api/mini/product-orders/catalog", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityProductOrder)
		if err != nil || !allowed {
			return err
		}
		productOrders, ok := fulfillment.(MiniProductOrderFulfillment)
		if !ok {
			return miniInternalError(c)
		}
		result, err := productOrders.MiniProductOrderCatalog(c.Request().Context(), customerfulfillmentapp.MiniDirectShipCatalogQuery{CustomerID: current.CurrentCustomerID, Q: c.QueryParam("q"), Category: c.QueryParam("category")})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})
	e.POST("/api/mini/product-orders/preview", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityProductOrder)
		if err != nil || !allowed {
			return err
		}
		productOrders, ok := fulfillment.(MiniProductOrderFulfillment)
		if !ok {
			return miniInternalError(c)
		}
		var cmd customerfulfillmentapp.MiniDirectShipCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		bindMiniDirectShipPrincipal(c, current, &cmd)
		result, err := productOrders.PreviewMiniProductOrder(c.Request().Context(), cmd)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})
	e.POST("/api/mini/product-orders", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityProductOrder)
		if err != nil || !allowed {
			return err
		}
		productOrders, ok := fulfillment.(MiniProductOrderFulfillment)
		if !ok {
			return miniInternalError(c)
		}
		var cmd customerfulfillmentapp.MiniDirectShipCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		bindMiniDirectShipPrincipal(c, current, &cmd)
		result, err := productOrders.SubmitMiniProductOrder(c.Request().Context(), cmd)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusCreated, result)
	})

	e.GET("/api/mini/direct-ship/catalog", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		result, err := fulfillment.MiniDirectShipCatalog(c.Request().Context(), customerfulfillmentapp.MiniDirectShipCatalogQuery{
			CustomerID: current.CurrentCustomerID,
			Q:          c.QueryParam("q"),
			Category:   c.QueryParam("category"),
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/direct-ship/preview", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		var cmd customerfulfillmentapp.MiniDirectShipCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		bindMiniDirectShipPrincipal(c, current, &cmd)
		result, err := fulfillment.PreviewMiniDirectShip(c.Request().Context(), cmd)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/direct-ship/requests", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		var cmd customerfulfillmentapp.MiniDirectShipCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		bindMiniDirectShipPrincipal(c, current, &cmd)
		result, err := fulfillment.SubmitMiniDirectShip(c.Request().Context(), cmd)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusCreated, result)
	})

	e.GET("/api/mini/direct-ship/requests", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		page, _ := strconv.Atoi(c.QueryParam("page"))
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		result, err := fulfillment.ListMiniDirectShipRequests(c.Request().Context(), customerfulfillmentapp.MiniDirectShipListQuery{
			CustomerID:  current.CurrentCustomerID,
			Q:           c.QueryParam("q"),
			ShippedFrom: c.QueryParam("shipped_from"),
			ShippedTo:   c.QueryParam("shipped_to"),
			Page:        page,
			Limit:       limit,
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/mini/direct-ship/requests/:id", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		requestID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || requestID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		row, err := fulfillment.GetMiniDirectShipRequest(c.Request().Context(), current.CurrentCustomerID, requestID)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"request": row})
	})

	e.POST("/api/mini/direct-ship/requests/:id/cancel", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityDirectShip)
		if err != nil || !allowed {
			return err
		}
		requestID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || requestID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		row, err := fulfillment.CancelMiniDirectShipRequest(c.Request().Context(), current.CurrentCustomerID, requestID, miniCustomerFulfillmentActor(current))
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/mini/customer-inventory", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityInventoryCustody, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		rawPage := strings.TrimSpace(c.QueryParam("page"))
		rawLimit := strings.TrimSpace(c.QueryParam("limit"))
		page, _ := strconv.Atoi(rawPage)
		limit, _ := strconv.Atoi(rawLimit)
		queryText := c.QueryParam("q")
		result, err := fulfillment.ListCustomerCentralInventory(c.Request().Context(), customerfulfillmentapp.CustomerInventoryListQuery{
			CustomerID: current.CurrentCustomerID,
			Q:          queryText,
			Page:       page,
			Limit:      limit,
			LegacyAll:  strings.TrimSpace(queryText) == "" && rawPage == "" && rawLimit == "",
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/mini/customer-inventory/:product_id/batches", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityInventoryCustody, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		productID, err := strconv.ParseInt(strings.TrimSpace(c.Param("product_id")), 10, 64)
		if err != nil || productID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		parseOptionalPositiveID := func(name string) (int64, bool) {
			raw := strings.TrimSpace(c.QueryParam(name))
			if raw == "" {
				return 0, true
			}
			value, parseErr := strconv.ParseInt(raw, 10, 64)
			return value, parseErr == nil && value > 0
		}
		bomSpecID, validBomSpecID := parseOptionalPositiveID("bom_spec_id")
		bomVariantID, validBomVariantID := parseOptionalPositiveID("bom_variant_id")
		specG, validSpecG := parseOptionalPositiveID("spec_g")
		canonical := bomSpecID > 0 || bomVariantID > 0
		if !validBomSpecID || !validBomVariantID || !validSpecG || (canonical && (bomSpecID <= 0 || specG > 0)) || (!canonical && specG <= 0) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		rows, err := fulfillment.ListCustomerCentralInventoryBatches(c.Request().Context(), customerfulfillmentapp.CustomerInventoryBatchQuery{
			CustomerID: current.CurrentCustomerID, ProductID: productID,
			BomSpecID: bomSpecID, BomVariantID: bomVariantID, SpecG: specG,
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.GET("/api/mini/customer-inventory/assets", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityInventoryCustody, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		rows, err := fulfillment.ListCustomerAssetInventory(c.Request().Context(), customerfulfillmentapp.CustomerAssetInventoryQuery{
			CustomerID: current.CurrentCustomerID, InventoryType: c.QueryParam("type"), Q: c.QueryParam("q"),
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.GET("/api/mini/customer-inventory/assets/:inventory_type/:item_id/ledger", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilityInventoryCustody, customerportalapp.CapabilityProcessing)
		if err != nil || !allowed {
			return err
		}
		itemID, parseErr := strconv.ParseInt(strings.TrimSpace(c.Param("item_id")), 10, 64)
		if parseErr != nil || itemID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		bomSpecID, _ := strconv.ParseInt(strings.TrimSpace(c.QueryParam("bom_spec_id")), 10, 64)
		specG, _ := strconv.ParseInt(strings.TrimSpace(c.QueryParam("spec_g")), 10, 64)
		limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
		if fulfillment == nil {
			return miniInternalError(c)
		}
		rows, err := fulfillment.ListCustomerAssetInventoryLedger(c.Request().Context(), customerfulfillmentapp.CustomerAssetInventoryLedgerQuery{
			CustomerID: current.CurrentCustomerID, InventoryType: c.Param("inventory_type"), ItemID: itemID,
			BomSpecID: bomSpecID, SpecG: specG, Limit: limit,
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	accountHandler := func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilitySettlement)
		if err != nil || !allowed {
			return err
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		page, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("page")))
		limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
		query, err := customerfulfillmentapp.NormalizeAccountQuery(customerfulfillmentapp.AccountQuery{
			CustomerID: current.CurrentCustomerID, Query: strings.TrimSpace(c.QueryParam("q")),
			DateFrom: c.QueryParam("date_from"), DateTo: c.QueryParam("date_to"), Period: c.QueryParam("period"),
			Anchor: c.QueryParam("anchor"), PayStatus: c.QueryParam("pay_status"), ShipStatus: c.QueryParam("ship_status"),
			Page: page, Limit: limit,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		data, err := fulfillment.CustomerAccount(c.Request().Context(), query)
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		path := c.Request().URL.Path
		if strings.HasSuffix(path, ".pdf") || strings.HasSuffix(path, ".xlsx") {
			var body []byte
			contentType := "application/pdf"
			filename := "customer-statement.pdf"
			if strings.HasSuffix(path, ".xlsx") {
				body, err = excelinfra.RenderCustomerAccount(data)
				contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
				filename = "customer-statement.xlsx"
			} else {
				body, err = pdfinfra.RenderCustomerAccount(data)
			}
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "账单生成失败"})
			}
			c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			return c.Blob(http.StatusOK, contentType, body)
		}
		data.Paginate(query.Page, query.Limit)
		return c.JSON(http.StatusOK, data)
	}
	e.GET("/api/mini/customer-account", accountHandler)
	e.GET("/api/mini/customer-account/statements.pdf", accountHandler)
	e.GET("/api/mini/customer-account/statements.xlsx", accountHandler)

	e.POST("/api/mini/customer-account/settlements/:id/confirm", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilitySettlement)
		if err != nil || !allowed {
			return err
		}
		settlementID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		var req struct {
			StatementRevision string `json:"statement_revision"`
		}
		if err != nil || settlementID <= 0 || c.Bind(&req) != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		actor := strings.TrimSpace(current.EmployeeName)
		if actor == "" {
			actor = fmt.Sprintf("mini_user:%d", current.MiniUserID)
		}
		row, err := fulfillment.ConfirmCustomerStatement(c.Request().Context(), customerfulfillmentapp.ConfirmCustomerStatementCommand{
			CustomerID: current.CurrentCustomerID, SettlementID: settlementID, StatementRevision: req.StatementRevision,
			MiniUserID: current.MiniUserID, Actor: actor,
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/mini/customer-account/settlements/:id/disputes", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, portal, customerportalapp.CapabilitySettlement)
		if err != nil || !allowed {
			return err
		}
		settlementID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		var req struct {
			StatementRevision string `json:"statement_revision"`
			FeeItemID         int64  `json:"fee_item_id"`
			Reason            string `json:"reason"`
		}
		if err != nil || settlementID <= 0 || c.Bind(&req) != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		if fulfillment == nil {
			return miniInternalError(c)
		}
		actor := strings.TrimSpace(current.EmployeeName)
		if actor == "" {
			actor = fmt.Sprintf("mini_user:%d", current.MiniUserID)
		}
		row, err := fulfillment.CreateCustomerStatementDispute(c.Request().Context(), customerfulfillmentapp.CreateCustomerStatementDisputeCommand{
			CustomerID: current.CurrentCustomerID, SettlementID: settlementID, FeeItemID: req.FeeItemID,
			StatementRevision: req.StatementRevision, Reason: req.Reason, MiniUserID: current.MiniUserID, Actor: actor,
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusCreated, row)
	})

	e.GET("/api/customer-portal/admin/customers/:id/statement-disputes", func(c echo.Context) error {
		if fulfillment == nil {
			return miniInternalError(c)
		}
		customerID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || customerID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		data, err := fulfillment.CustomerAccount(c.Request().Context(), customerfulfillmentapp.AccountQuery{CustomerID: customerID, Page: 1, Limit: 200})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		rows := make([]map[string]any, 0)
		for _, settlement := range data.Settlements {
			feeNames := make(map[int64]string, len(settlement.Fees))
			for _, fee := range settlement.Fees {
				feeNames[fee.ID] = fee.FeeName
			}
			for _, dispute := range settlement.Disputes {
				rows = append(rows, map[string]any{
					"id": dispute.ID, "customer_id": customerID, "customer_name": data.CustomerName,
					"settlement_id": settlement.ID, "settlement_no": settlement.SettlementNo,
					"fee_item_id": dispute.FeeItemID, "fee_name": feeNames[dispute.FeeItemID],
					"reason": dispute.Reason, "status": dispute.Status, "created_at": dispute.CreatedAt,
					"reply": dispute.Reply, "replied_by": dispute.RepliedBy, "replied_at": dispute.RepliedAt,
				})
			}
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/customer-portal/admin/statement-disputes/:id/reply", func(c echo.Context) error {
		if fulfillment == nil {
			return miniInternalError(c)
		}
		disputeID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		var req struct {
			Reply  string `json:"reply"`
			Status string `json:"status"`
		}
		if err != nil || disputeID <= 0 || c.Bind(&req) != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		row, err := fulfillment.ReplyCustomerStatementDispute(c.Request().Context(), customerfulfillmentapp.ReplyCustomerStatementDisputeCommand{
			DisputeID: disputeID, Reply: req.Reply, Status: req.Status, Actor: supporthttp.ActorOf(c),
		})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})
}

func requireMiniCustomerFulfillmentContext(c echo.Context, portal Service, capabilities ...string) (customerportalapp.CurrentContext, bool, error) {
	if portal == nil {
		return customerportalapp.CurrentContext{}, false, miniInternalError(c)
	}
	token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
	if token == "" {
		return customerportalapp.CurrentContext{}, false, c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
	}
	current, err := portal.Me(c.Request().Context(), token)
	if err != nil {
		return customerportalapp.CurrentContext{}, false, miniSessionError(c, err)
	}
	if current.CurrentCustomerID <= 0 {
		return customerportalapp.CurrentContext{}, false, c.JSON(http.StatusForbidden, map[string]string{"error": "customer binding not found"})
	}
	if !current.HasAnyCapability(capabilities) {
		return customerportalapp.CurrentContext{}, false, c.JSON(http.StatusForbidden, map[string]string{"error": "capability not enabled"})
	}
	return current, true, nil
}

func bindMiniDirectShipPrincipal(c echo.Context, current customerportalapp.CurrentContext, cmd *customerfulfillmentapp.MiniDirectShipCommand) {
	cmd.CustomerID = current.CurrentCustomerID
	cmd.EmployeeID = current.EmployeeID
	cmd.MiniUserID = current.MiniUserID
	cmd.Actor = miniCustomerFulfillmentActor(current)
	if cmd.IdempotencyKey == "" {
		cmd.IdempotencyKey = strings.TrimSpace(c.Request().Header.Get("Idempotency-Key"))
	}
}

func miniCustomerFulfillmentActor(current customerportalapp.CurrentContext) string {
	if current.EmployeeID > 0 {
		return fmt.Sprintf("mini_employee:%d", current.EmployeeID)
	}
	return fmt.Sprintf("mini_user:%d", current.MiniUserID)
}

func miniCustomerFulfillmentError(c echo.Context, err error) error {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "shipped_from invalid"):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "发货开始日期格式不正确，请使用 YYYY-MM-DD"})
	case strings.Contains(message, "shipped_to invalid"):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "发货结束日期格式不正确，请使用 YYYY-MM-DD"})
	case strings.Contains(message, "shipment date range invalid"):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "发货开始日期不能晚于结束日期"})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipStockInsufficient):
		return c.JSON(http.StatusConflict, map[string]string{"error": "当前客户成品仓库存不足，无法提交发货"})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipIdempotency):
		return c.JSON(http.StatusConflict, map[string]string{"error": "该发货请求已提交，不能使用同一请求编号修改内容"})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipPriceChanged):
		return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipCannotCancel):
		return c.JSON(http.StatusConflict, map[string]string{"error": "订单已进入 ERP 履约或已经发货，不能在小程序直接取消"})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipRequestNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "未找到该发货申请"})
	case errors.Is(err, customerfulfillmentapp.ErrMiniDirectShipUnavailable):
		return miniInternalError(c)
	case strings.Contains(message, "账单内容已更新") || strings.Contains(message, "未处理异议"):
		return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
	case strings.Contains(message, "账单") || strings.Contains(message, "异议") || strings.Contains(message, "费用项"):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	case strings.Contains(message, "价格表") || strings.Contains(message, "当前有效数量档位"):
		return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
	case miniCustomerFulfillmentValidationError(err):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "发货信息不完整，请检查收件信息和商品数量"})
	default:
		return miniInternalError(c)
	}
}

func miniCustomerFulfillmentValidationError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, " required") || strings.Contains(message, " invalid") || strings.Contains(message, " too long") || strings.Contains(message, " unavailable")
}
