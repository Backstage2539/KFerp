package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
	"orderapp/internal/infrastructure/postgres/orderweight"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"strings"
	"time"
)

func (r *Repository) modeWarehouse(ctx context.Context, q miniDirectShipQuerier, id int64, mode string, lock bool) (string, error) {
	if mode == "" {
		return "", nil
	}
	settings, err := r.loadFulfillmentSettings(ctx, q, id, lock)
	if err != nil {
		return "", err
	}
	for _, m := range settings.Modes {
		if m.Mode == mode && m.Enabled {
			if mode == "stock" {
				if err = r.validateFulfillmentWarehouse(ctx, q, id, m.WarehouseCode); err != nil {
					return "", err
				}
				return m.WarehouseCode, nil
			}
			if mode == "on_demand" {
				return "", nil
			}
		}
	}
	return "", fmt.Errorf("该客户未启用所选代发场景")
}
func (r *Repository) retainedOutputCandidates(ctx context.Context, q miniDirectShipQuerier, id int64, warehouse string, lock bool) ([]miniProcessingOutputCandidate, error) {
	supply, err := customerstock.ListSupply(ctx, q, r.schema, id, warehouse, lock)
	if err != nil {
		return nil, err
	}
	out := []miniProcessingOutputCandidate{}
	for _, s := range supply {
		if s.AvailableQty > 0 {
			out = append(out, miniProcessingOutputCandidate{EntitlementID: s.ID, ProductID: s.ProductID, BomSpecID: s.BomSpecID, BomVariantID: s.BomVariantID, SpecG: s.SpecG, AvailableQty: s.AvailableQty, Warehouse: s.Warehouse})
		}
	}
	return out, nil
}
func (r *Repository) stockModePlan(ctx context.Context, q miniDirectShipQuerier, cmd app.MiniDirectShipCommand, lock bool) ([]miniPlannedAllocation, []miniPlannedProcessingAllocation, []app.MiniDirectShipShortage, error) {
	warehouse, err := r.modeWarehouse(ctx, q, cmd.CustomerID, cmd.FulfillmentMode, lock)
	if err != nil {
		return nil, nil, nil, err
	}
	stock, err := r.loadMiniCustomerFinishedStock(ctx, q, cmd.CustomerID, lock)
	if err != nil {
		return nil, nil, nil, err
	}
	filtered := []miniStockCandidate{}
	for _, s := range stock.Candidates {
		if s.Warehouse == warehouse {
			filtered = append(filtered, s)
		}
	}
	outputs, err := r.retainedOutputCandidates(ctx, q, cmd.CustomerID, warehouse, lock)
	if err != nil {
		return nil, nil, nil, err
	}
	// An inventory order may reserve only owned physical stock or a concrete retained sale.
	for i := range cmd.Items {
		cmd.Items[i].IsProcessingProduct = true
	}
	a, b, c := planMiniDirectShipFulfillment(cmd.Items, filtered, outputs)
	for i := range c {
		c[i].Blocking = true
		c[i].BlockingReason = "客户已购库存权益不足，请先补购；当前代发草稿已保留"
	}
	return a, b, c, nil
}

func (r *Repository) submitExplicitDirectShipOrderTx(ctx context.Context, tx pgx.Tx, cmd app.SubmitCustomerDirectShipOrderCommand) (app.DirectShipOrderSummary, error) {
	if cmd.DeliveryMode == "customer_stock" {
		if cmd.PortalServiceCode != "product_order" {
			return app.DirectShipOrderSummary{}, fmt.Errorf("留厂补购必须从商品下单提交")
		}
		warehouse, e := r.modeWarehouse(ctx, tx, cmd.CustomerID, "stock", true)
		if e != nil {
			return app.DirectShipOrderSummary{}, e
		}
		if warehouse != strings.TrimSpace(cmd.DeliveryWarehouseCode) {
			return app.DirectShipOrderSummary{}, fmt.Errorf("留厂目标仓必须为当前客户绑定的库存代发仓")
		}
	}
	if cmd.FulfillmentMode != "stock" {
		date, e := time.Parse("2006-01-02", cmd.OrderDate)
		if e != nil {
			date = time.Now()
		}
		save := salesapp.SaveOrderCommand{FreightProtocol: cmd.FreightProtocol, FreightQuoteToken: cmd.FreightQuoteToken, ShippingProvince: cmd.ShippingProvince, ShippingCity: cmd.ShippingCity, ShippingDistrict: cmd.ShippingDistrict, CustomerID: cmd.CustomerID, DocumentDate: date, OrderDate: date, Actor: cmd.Actor, PortalServiceCode: cmd.PortalServiceCode, OrdersScope: "fulfillment", SelectedPriceTableIDs: cmd.SelectedPriceTableIDs, ReceiverName: cmd.ReceiverName, ReceiverPhone: cmd.ReceiverPhone, ReceiverAddress: cmd.ReceiverAddress, ReceiverCompany: cmd.ReceiverCompany, Notes: cmd.Note, DeliveryMode: cmd.DeliveryMode, DeliveryWarehouseCode: cmd.DeliveryWarehouseCode, FulfillmentMode: cmd.FulfillmentMode}
		save.PayStatusID = customerFulfillmentStatusID(ctx, tx, r.schema, "pay_statuses", "未付款", "未收款")
		save.ShipStatusID = customerFulfillmentStatusID(ctx, tx, r.schema, "ship_statuses", "未发货", "待发货")
		for _, it := range cmd.Items {
			id := it.ProductID
			save.Items = append(save.Items, salesapp.OrderItemCommand{ProductID: &id, Name: it.ProductName, BomSpecID: it.BomSpecID, BomVariantID: it.BomVariantID, SpecG: it.SpecG, Units: it.QuantityUnits, Unit: it.InventoryUnit, SalesUnit: it.SalesUnit})
		}
		result, e := postgressales.NewRepository(r.pool, r.schema).WithOrderTransaction(tx).SaveOrder(ctx, save)
		if e != nil {
			return app.DirectShipOrderSummary{}, e
		}
		return app.DirectShipOrderSummary{OrderID: result.OrderID, OrderNo: result.OrderNo, OrderDate: cmd.OrderDate, ReceiverAddress: cmd.ReceiverAddress, Status: "submitted", ItemCount: len(cmd.Items)}, nil
	}
	warehouse, e := r.modeWarehouse(ctx, tx, cmd.CustomerID, "stock", true)
	if e != nil {
		return app.DirectShipOrderSummary{}, e
	}
	var id int64
	date := cmd.OrderDate
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,order_date,document_date,customer_id,portal_service_code,fulfillment_mode,source_warehouse,receiver_name,receiver_phone,receiver_address,receiver_company,notes,total_amount,grand_total)VALUES('', $1::date,$1::date,$2,'direct_ship','stock',$3,$4,$5,$6,$7,$8,0,0)RETURNING id`, r.schema), date, cmd.CustomerID, warehouse, cmd.ReceiverName, cmd.ReceiverPhone, cmd.ReceiverAddress, cmd.ReceiverCompany, cmd.Note).Scan(&id)
	if err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	no := fmt.Sprintf("SO-%s-%06d", strings.ReplaceAll(date, "-", ""), id)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET order_no=$2 WHERE id=$1`, r.schema), id, no); err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	for idx, it := range cmd.Items {
		spec := it.Spec
		if spec == "" && it.SpecG > 0 {
			spec = fmt.Sprintf("%dg", it.SpecG)
		}
		unit := it.InventoryUnit
		if unit == "" {
			unit = "bag"
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty,unit,sales_unit,unit_price,line_total,price_source_json)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,0,0,'{"source":"prepaid_customer_stock"}'::jsonb)`, r.schema), id, idx+1, it.ProductID, it.BomSpecID, it.BomVariantID, it.ProductName, spec, it.QuantityUnits, unit); err != nil {
			return app.DirectShipOrderSummary{}, err
		}
	}
	if err := orderweight.FreezeOrder(ctx, tx, r.schema, id); err != nil {
		return app.DirectShipOrderSummary{}, err
	}
	return app.DirectShipOrderSummary{OrderID: id, OrderNo: no, OrderDate: date, ReceiverAddress: cmd.ReceiverAddress, Status: "submitted", ItemCount: len(cmd.Items)}, nil
}

func (r *Repository) requireExplicitConfiguredMode(ctx context.Context, q miniDirectShipQuerier, cmd app.MiniDirectShipCommand) error {
	if cmd.UsageCode == "product_order" {
		if cmd.FulfillmentMode != "" {
			return fmt.Errorf("商品下单不能使用代发场景")
		}
		return nil
	}
	if cmd.FulfillmentMode == "" {
		settings, e := r.loadFulfillmentSettings(ctx, q, cmd.CustomerID, false)
		if e != nil {
			return e
		}
		if settings.Revision > 0 {
			return fmt.Errorf("请选择已开通的代发场景")
		}
	}
	if cmd.DeliveryMode != "" || cmd.DeliveryWarehouseCode != "" {
		return fmt.Errorf("代发不能设置销售留厂交付方式")
	}
	return nil
}
