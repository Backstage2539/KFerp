package purchase

import (
	"net/http"
	purchaseapp "orderapp/internal/application/purchase"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

func registerPurchaseAPI(e *echo.Echo, purchaseSvc *purchaseapp.Service) {
	e.GET("/api/purchase/suppliers", func(c echo.Context) error {
		rows, err := purchaseSvc.ListSuppliers(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/purchase/suppliers", func(c echo.Context) error {
		var req struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Contact string `json:"contact"`
			Phone   string `json:"phone"`
			Address string `json:"address"`
			Active  bool   `json:"active"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		}
		row, err := purchaseSvc.SaveSupplier(c.Request().Context(), purchaseapp.SaveSupplierCommand{
			ID: req.ID, Name: req.Name, Contact: req.Contact, Phone: req.Phone, Address: req.Address, Active: req.Active,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})
	e.GET("/api/purchase/orders", func(c echo.Context) error {
		rows, err := purchaseSvc.ListPurchaseOrders(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/purchase/orders", func(c echo.Context) error {
		var req struct {
			SupplierID      int64   `json:"supplier_id"`
			MaterialID      int64   `json:"material_id"`
			QtyG            int64   `json:"qty_g"`
			Qty             float64 `json:"qty"`
			UnitCode        string  `json:"unit_code"`
			QtyUnits        int64   `json:"qty_units"`
			TargetWarehouse string  `json:"target_warehouse"`
			UnitCost        float64 `json:"unit_cost"`
			Note            string  `json:"note"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		}
		row, err := purchaseSvc.CreatePurchaseOrder(c.Request().Context(), purchaseapp.CreatePurchaseOrderCommand{
			SupplierID:      req.SupplierID,
			MaterialID:      req.MaterialID,
			QtyG:            req.QtyG,
			Qty:             req.Qty,
			UnitCode:        req.UnitCode,
			QtyUnits:        req.QtyUnits,
			TargetWarehouse: req.TargetWarehouse,
			UnitCost:        req.UnitCost,
			Note:            req.Note,
			Operator:        support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})
	e.GET("/api/purchase/receipts", func(c echo.Context) error {
		rows, err := purchaseSvc.ListPurchaseReceipts(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})
	e.POST("/api/purchase/receipts", func(c echo.Context) error {
		var req struct {
			PurchaseOrderID int64   `json:"purchase_order_id"`
			SupplierID      int64   `json:"supplier_id"`
			SupplierName    string  `json:"supplier_name"`
			MaterialID      int64   `json:"material_id"`
			QtyG            int64   `json:"qty_g"`
			Qty             float64 `json:"qty"`
			UnitCode        string  `json:"unit_code"`
			QtyUnits        int64   `json:"qty_units"`
			TargetWarehouse string  `json:"target_warehouse"`
			UnitCost        float64 `json:"unit_cost"`
			Note            string  `json:"note"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		}
		row, err := purchaseSvc.CreatePurchaseReceipt(c.Request().Context(), purchaseapp.CreatePurchaseReceiptCommand{
			PurchaseOrderID: req.PurchaseOrderID,
			SupplierID:      req.SupplierID,
			SupplierName:    req.SupplierName,
			MaterialID:      req.MaterialID,
			QtyG:            req.QtyG,
			Qty:             req.Qty,
			UnitCode:        req.UnitCode,
			QtyUnits:        req.QtyUnits,
			TargetWarehouse: req.TargetWarehouse,
			UnitCost:        req.UnitCost,
			Note:            req.Note,
			Operator:        support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})
}
