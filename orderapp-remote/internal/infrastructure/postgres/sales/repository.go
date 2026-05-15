package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	pdfinfra "orderapp/internal/infrastructure/pdf"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool                 *pgxpool.Pool
	schema               string
	assetDir             string
	renderer             SalesOrderPDFRenderer
	deliveryNoteRenderer DeliveryNotePDFRenderer
}

type SalesOrderPDFRenderer interface {
	Render(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error)
	RenderPreview(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error)
	RenderPNG(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error)
}

type DeliveryNotePDFRenderer interface {
	Render(snapshot salesdomain.DeliveryNoteSnapshot) ([]byte, error)
	RenderPreview(snapshot salesdomain.DeliveryNoteSnapshot) ([]byte, error)
}

type RepositoryOption func(*Repository)

func WithSalesOrderAssetDir(assetDir string) RepositoryOption {
	return func(r *Repository) {
		r.assetDir = assetDir
	}
}

func WithSalesOrderRenderer(renderer SalesOrderPDFRenderer) RepositoryOption {
	return func(r *Repository) {
		r.renderer = renderer
	}
}

func NewRepository(pool *pgxpool.Pool, schema string, opts ...RepositoryOption) Repository {
	repo := Repository{pool: pool, schema: schema, assetDir: "/app/data/assets"}
	for _, opt := range opts {
		opt(&repo)
	}
	if repo.renderer == nil {
		repo.renderer = pdfinfra.SalesOrderRenderer{AssetBaseDir: repo.assetDir}
	}
	if repo.deliveryNoteRenderer == nil {
		repo.deliveryNoteRenderer = pdfinfra.DeliveryNoteRenderer{AssetBaseDir: repo.assetDir}
	}
	return repo
}

func lookupDefaultStatusID(ctx context.Context, tx pgx.Tx, schema, table string, names ...string) int64 {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var id int64
		q := fmt.Sprintf("SELECT id FROM %s.%s WHERE name=$1 ORDER BY id LIMIT 1", schema, table)
		if err := tx.QueryRow(ctx, q, name).Scan(&id); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

type smallBatchPriceRule struct {
	Enabled     bool    `json:"enabled"`
	ThresholdLB float64 `json:"threshold_lb"`
	TierMinLB   float64 `json:"tier_min_lb"`
	TierMaxLB   float64 `json:"tier_max_lb"`
}

type directShipCapabilityConfig struct {
	SmallBatchPriceRule smallBatchPriceRule `json:"small_batch_price_rule"`
}

func (r Repository) customerDirectShipSmallBatchPriceRuleTx(ctx context.Context, tx pgx.Tx, customerID int64) smallBatchPriceRule {
	if customerID <= 0 {
		return smallBatchPriceRule{}
	}
	var hasCapabilities bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.customer_service_capabilities", r.schema)).Scan(&hasCapabilities); err != nil || !hasCapabilities {
		return smallBatchPriceRule{}
	}
	var raw []byte
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT config_json
		FROM %s.customer_service_capabilities
		WHERE customer_id=$1 AND capability_code='direct_ship' AND enabled=true
	`, r.schema), customerID).Scan(&raw)
	if err != nil {
		return smallBatchPriceRule{}
	}
	var config directShipCapabilityConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return smallBatchPriceRule{}
	}
	return normalizeSmallBatchPriceRule(config.SmallBatchPriceRule)
}

func normalizeSmallBatchPriceRule(rule smallBatchPriceRule) smallBatchPriceRule {
	if !rule.Enabled {
		return smallBatchPriceRule{}
	}
	if rule.ThresholdLB <= 0 {
		rule.ThresholdLB = 14
	}
	if rule.TierMinLB <= 0 {
		rule.TierMinLB = 15
	}
	if rule.TierMaxLB <= 0 {
		rule.TierMaxLB = 28
	}
	return rule
}

func orderPaidStatusRequiresPaymentMethod(statusName string) bool {
	statusName = strings.TrimSpace(statusName)
	return strings.Contains(statusName, "已付款") || strings.Contains(statusName, "已收款") || strings.Contains(statusName, "已支付")
}

func normalizeOrderPaymentMethodForStatusTx(ctx context.Context, tx pgx.Tx, schema string, payStatusID int64, raw string) (string, error) {
	method := strings.TrimSpace(raw)
	if payStatusID <= 0 {
		return "", nil
	}
	var statusName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.pay_statuses WHERE id=$1`, schema), payStatusID).Scan(&statusName); err != nil {
		return "", fmt.Errorf("invalid pay_status_id")
	}
	if orderPaidStatusRequiresPaymentMethod(statusName) {
		if method == "" {
			return "", fmt.Errorf("payment_method required")
		}
		return method, nil
	}
	return "", nil
}

func smallBatchTierQuantity(specG int64, qtyLb float64, rule smallBatchPriceRule) (int64, bool) {
	rule = normalizeSmallBatchPriceRule(rule)
	if !rule.Enabled || specG <= 0 || qtyLb <= 0 || qtyLb >= rule.ThresholdLB {
		return 0, false
	}
	targetUnits := int64(math.Ceil(rule.TierMinLB * 454.0 / float64(specG)))
	if targetUnits < 1 {
		targetUnits = 1
	}
	return targetUnits, true
}

func resolveOrderResponsibleParty(ctx context.Context, tx pgx.Tx, schema, responsibleType string, responsibleID int64) (string, int64, string, error) {
	typ := strings.TrimSpace(responsibleType)
	if typ == "" && responsibleID == 0 {
		return "", 0, "", nil
	}
	if responsibleID <= 0 {
		return "", 0, "", fmt.Errorf("responsible_id required")
	}
	switch typ {
	case "employee":
		var name string
		q := fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.company_employees WHERE id=$1 AND active=true`, schema)
		if err := tx.QueryRow(ctx, q, responsibleID).Scan(&name); err != nil || strings.TrimSpace(name) == "" {
			return "", 0, "", fmt.Errorf("responsible employee not found")
		}
		return typ, responsibleID, strings.TrimSpace(name), nil
	case "customer":
		var name string
		q := fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.customers WHERE id=$1 AND active=true`, schema)
		if err := tx.QueryRow(ctx, q, responsibleID).Scan(&name); err != nil || strings.TrimSpace(name) == "" {
			return "", 0, "", fmt.Errorf("responsible customer not found")
		}
		return typ, responsibleID, strings.TrimSpace(name), nil
	default:
		return "", 0, "", fmt.Errorf("invalid responsible_type")
	}
}

func wholesaleDisplayUnitG(specG int64) float64 {
	if specG >= 1000 {
		return 1000
	}
	return 454
}

func wholesaleDisplayUnitPriceFromLb(pricePerLb float64, specG int64) float64 {
	unitG := wholesaleDisplayUnitG(specG)
	price := pricePerLb * unitG / 454.0
	if unitG == 1000 {
		return math.Round(price)
	}
	return price
}

func wholesaleLineTotalFromDisplayUnit(unitPrice float64, specG int64, units int64) float64 {
	return unitPrice * (float64(specG*units) / wholesaleDisplayUnitG(specG))
}

func (r Repository) SaveOrder(ctx context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	od := cmd.OrderDate
	if od.IsZero() {
		return salesapp.SaveOrderResult{}, fmt.Errorf("invalid order_date")
	}
	if cmd.CustomerID <= 0 {
		return salesapp.SaveOrderResult{}, fmt.Errorf("customer required")
	}

	type item struct {
		productID     *int64
		tierID        *int64
		manualPrice   *float64
		name          string
		note          string
		units         int64
		specG         int64
		unit          *string
		spec          *string
		unitPrice     float64
		lineTotal     float64
		priceOverride bool
	}
	items := make([]item, 0, len(cmd.Items))
	for _, src := range cmd.Items {
		name := strings.TrimSpace(src.Name)
		if src.ProductID == nil && name == "" {
			continue
		}
		it := item{
			productID:   src.ProductID,
			tierID:      src.TierID,
			manualPrice: src.ManualPrice,
			name:        name,
			note:        strings.TrimSpace(src.Note),
			units:       src.Units,
			specG:       src.SpecG,
		}
		if it.manualPrice != nil {
			it.priceOverride = true
		}
		if src.SpecG > 0 {
			spec := fmt.Sprintf("%dg", src.SpecG)
			it.spec = &spec
		}
		if unit := strings.TrimSpace(src.Unit); unit != "" {
			it.unit = &unit
		}
		items = append(items, it)
	}
	// Validate: need at least one item with spec+units
	valid := false
	for _, it := range items {
		if it.productID != nil && it.specG > 0 && it.units > 0 {
			valid = true
			break
		}
	}
	if !valid {
		return salesapp.SaveOrderResult{}, fmt.Errorf("at least one item required")
	}

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.orders IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	orderNo := ""
	retailOrder := false
	if cmd.OrderTypeID > 0 {
		var orderTypeName string
		_ = tx.QueryRow(ctx, fmt.Sprintf("SELECT COALESCE(name,'') FROM %s.order_types WHERE id=$1", r.schema), cmd.OrderTypeID).Scan(&orderTypeName)
		retailOrder = isRetailOrderTypeName(orderTypeName)
	}
	smallBatchRule := r.customerDirectShipSmallBatchPriceRuleTx(ctx, tx, cmd.CustomerID)

	// Pricing: wholesale tiers prefer exact package spec tiers, then fall back to
	// bean-list weight tiers so non-454g packaging can still price by total lb.
	totalAmt := 0.0
	orderWeightG := int64(0)
	for idx := range items {
		itemWeightG := items[idx].specG * items[idx].units
		orderWeightG += itemWeightG
		totalG := float64(itemWeightG)
		qtyLb := totalG / 454.0

		if items[idx].manualPrice != nil {
			lineTotal := wholesaleLineTotalFromDisplayUnit(*items[idx].manualPrice, items[idx].specG, items[idx].units)
			if retailOrder {
				lineTotal = *items[idx].manualPrice * float64(items[idx].units)
			}
			items[idx].lineTotal = lineTotal
			items[idx].unitPrice = *items[idx].manualPrice
			items[idx].priceOverride = true
			totalAmt += items[idx].lineTotal
			continue
		} else if retailOrder && items[idx].productID != nil {
			retailPrices := salesdomain.RetailSpecPrices{}
			q := fmt.Sprintf(`SELECT
				COALESCE(retail_price_100g, 0),
				COALESCE(retail_price_200g, 0),
				COALESCE(NULLIF(retail_price_227g,0), default_price, 0),
				COALESCE(retail_price_250g, 0)
				FROM %s.products WHERE id=$1`, r.schema)
			_ = tx.QueryRow(ctx, q, *items[idx].productID).Scan(&retailPrices.Price100G, &retailPrices.Price200G, &retailPrices.Price227G, &retailPrices.Price250G)
			_, lineTotal := salesdomain.RetailLinePriceForSpec(retailPrices, items[idx].specG, items[idx].units)
			items[idx].lineTotal = lineTotal
			if qtyLb > 0 {
				items[idx].unitPrice = lineTotal / qtyLb
			}
			totalAmt += items[idx].lineTotal
			continue
		} else if items[idx].productID != nil {
			// If user selected a tier explicitly
			if items[idx].tierID != nil {
				var tierSpecG int64
				var packagePrice, pricePerLb float64
				q := fmt.Sprintf(`SELECT
					COALESCE(NULLIF(spec_g,0),454),
					COALESCE(NULLIF(price_per_unit,0), NULLIF(price_per_lb,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0),
					COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
					FROM %s.product_price_tiers
					WHERE id=$1 AND active=true`, r.schema)
				if err := tx.QueryRow(ctx, q, *items[idx].tierID).Scan(&tierSpecG, &packagePrice, &pricePerLb); err != nil {
					return salesapp.SaveOrderResult{}, fmt.Errorf("invalid tier")
				}
				_ = tierSpecG
				_ = packagePrice
				items[idx].unitPrice = wholesaleDisplayUnitPriceFromLb(pricePerLb, items[idx].specG)
				items[idx].lineTotal = wholesaleLineTotalFromDisplayUnit(items[idx].unitPrice, items[idx].specG, items[idx].units)
			} else {
				// Auto-match tier by package count for the selected spec(g).
				var tid *int64
				var packagePrice, pricePerLb float64
				tierQty := items[idx].units
				tierQtyLb := qtyLb
				if adjustedQty, ok := smallBatchTierQuantity(items[idx].specG, qtyLb, smallBatchRule); ok {
					tierQty = adjustedQty
					tierQtyLb = float64(items[idx].specG*adjustedQty) / 454.0
				}
				q := fmt.Sprintf(`
							SELECT id,
							       COALESCE(NULLIF(price_per_unit,0), NULLIF(price_per_lb,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0),
							       COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
							FROM %s.product_price_tiers
							WHERE product_id=$1 AND active=true
							  AND COALESCE(NULLIF(spec_g,0),454)=$2
							  AND COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) <= $3
							  AND (COALESCE(NULLIF(max_qty_units,0), max_qty_lb) IS NULL OR COALESCE(NULLIF(max_qty_units,0), max_qty_lb) >= $3)
							ORDER BY COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) DESC
							LIMIT 1
						`, r.schema)
				err := tx.QueryRow(ctx, q, *items[idx].productID, items[idx].specG, tierQty).Scan(&tid, &packagePrice, &pricePerLb)
				if err != nil {
					// fallback: highest tier with min<=qty
					q2 := fmt.Sprintf(`
								SELECT id,
								       COALESCE(NULLIF(price_per_unit,0), NULLIF(price_per_lb,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0),
								       COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
								FROM %s.product_price_tiers
								WHERE product_id=$1 AND active=true
								  AND COALESCE(NULLIF(spec_g,0),454)=$2
								  AND COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) <= $3
								ORDER BY COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) DESC
								LIMIT 1
							`, r.schema)
					if err2 := tx.QueryRow(ctx, q2, *items[idx].productID, items[idx].specG, tierQty).Scan(&tid, &packagePrice, &pricePerLb); err2 != nil {
						// below minimum tier: use minimum tier price
						q3 := fmt.Sprintf(`
									SELECT id,
									       COALESCE(NULLIF(price_per_unit,0), NULLIF(price_per_lb,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0),
									       COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
									FROM %s.product_price_tiers
									WHERE product_id=$1 AND active=true
									  AND COALESCE(NULLIF(spec_g,0),454)=$2
									ORDER BY COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) ASC
									LIMIT 1
								`, r.schema)
						if err3 := tx.QueryRow(ctx, q3, *items[idx].productID, items[idx].specG).Scan(&tid, &packagePrice, &pricePerLb); err3 != nil {
							q4 := fmt.Sprintf(`
										SELECT id,
										       COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
										FROM %s.product_price_tiers
										WHERE product_id=$1 AND active=true
										  AND COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) <= $2
										  AND (
										    COALESCE(NULLIF(max_qty_lb,0), NULLIF(max_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0) IS NULL
										    OR COALESCE(NULLIF(max_qty_lb,0), NULLIF(max_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0) >= $2
										  )
										ORDER BY COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) DESC
										LIMIT 1
									`, r.schema)
							if err4 := tx.QueryRow(ctx, q4, *items[idx].productID, tierQtyLb).Scan(&tid, &pricePerLb); err4 != nil {
								q5 := fmt.Sprintf(`
											SELECT id,
											       COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
											FROM %s.product_price_tiers
											WHERE product_id=$1 AND active=true
											  AND COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) <= $2
											ORDER BY COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) DESC
											LIMIT 1
										`, r.schema)
								if err5 := tx.QueryRow(ctx, q5, *items[idx].productID, tierQtyLb).Scan(&tid, &pricePerLb); err5 != nil {
									q6 := fmt.Sprintf(`
												SELECT id,
												       COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
												FROM %s.product_price_tiers
												WHERE product_id=$1 AND active=true
												ORDER BY COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) ASC
												LIMIT 1
											`, r.schema)
									if err6 := tx.QueryRow(ctx, q6, *items[idx].productID).Scan(&tid, &pricePerLb); err6 != nil {
										pricePerLb = 0
										tid = nil
									}
								}
							}
							packagePrice = 0
						}
					}
				}
				items[idx].tierID = tid
				_ = packagePrice
				items[idx].unitPrice = wholesaleDisplayUnitPriceFromLb(pricePerLb, items[idx].specG)
				items[idx].lineTotal = wholesaleLineTotalFromDisplayUnit(items[idx].unitPrice, items[idx].specG, items[idx].units)
			}
		}

		if items[idx].lineTotal == 0 {
			items[idx].lineTotal = wholesaleLineTotalFromDisplayUnit(items[idx].unitPrice, items[idx].specG, items[idx].units)
		}
		totalAmt += items[idx].lineTotal
	}

	// Amount calculation (items + shipping - discount)
	shippingAmt := cmd.ShippingAmount
	discountAmt := cmd.DiscountAmount
	roundToInt := cmd.RoundToInt
	outsourceFees := [6]float64{
		cmd.OutsourceMaterialFee,
		cmd.OutsourceRoastFee,
		cmd.OutsourcePackagingFee,
		cmd.OutsourceManualFee,
		cmd.OutsourceTaxFee,
		cmd.OutsourceOtherFee,
	}
	outsourceTotal := cmd.OutsourceMaterialFee + cmd.OutsourceRoastFee + cmd.OutsourcePackagingFee + cmd.OutsourceManualFee + cmd.OutsourceTaxFee + cmd.OutsourceOtherFee
	grand0 := totalAmt + shippingAmt - discountAmt + outsourceTotal
	grandTotal, roundingAmt := applyRoundToInt(grand0, roundToInt)

	// 默认付款状态：未选择时自动写入“已付款”（兼容“已收款”命名）。
	payStatusID := cmd.PayStatusID
	if payStatusID == 0 {
		payStatusID = lookupDefaultStatusID(ctx, tx, r.schema, "pay_statuses", "已付款", "已收款")
	}
	paymentMethod, err := normalizeOrderPaymentMethodForStatusTx(ctx, tx, r.schema, payStatusID, cmd.PaymentMethod)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	// 默认发货状态：未选择时自动写入“未发货”。
	shipStatusID := cmd.ShipStatusID
	if shipStatusID == 0 {
		shipStatusID = lookupDefaultStatusID(ctx, tx, r.schema, "ship_statuses", "未发货")
	}

	shipMethod := strings.TrimSpace(cmd.ShipMethod)
	if shipMethod == "" {
		if orderWeightG <= 15000 {
			shipMethod = "sf_small"
		} else {
			shipMethod = "sf_large"
		}
	}
	shipTrackingNo := salesapp.TrackingNumbersSummary(salesapp.NormalizeTrackingNumbers(cmd.ShipTrackingNo))

	responsibleType, responsibleID, responsibleName, err := resolveOrderResponsibleParty(ctx, tx, r.schema, cmd.ResponsibleType, cmd.ResponsibleID)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	editID := cmd.EditID

	insertItemSQL := fmt.Sprintf(`INSERT INTO %s.order_items(order_id,line_no,product_id,price_tier_id,price_overridden,item_name,item_note,qty,unit,spec,unit_price,line_total)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, r.schema)

	var orderID int64
	if editID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT id, order_no FROM %s.orders WHERE id=$1 FOR UPDATE", r.schema), editID).Scan(&orderID, &orderNo); err != nil {
			return salesapp.SaveOrderResult{}, fmt.Errorf("invalid edit_id")
		}
		uq := fmt.Sprintf(`
				UPDATE %s.orders
				SET order_date=$2,
					customer_id=$3,
					source_id=$4,
					order_type_id=$5,
					pay_status_id=$6,
					payment_method=$7,
					ship_status_id=$8,
					ship_method=$9,
					ship_tracking_no=$10,
					notes=$11,
					total_amount=$12,
					shipping_amount=$13,
					discount_amount=$14,
					round_to_int=$15,
					rounding_amount=$16,
					grand_total=$17,
					express_fee=$18,
					outsource_material_fee=$19,
					outsource_roast_fee=$20,
					outsource_packaging_fee=$21,
					outsource_manual_fee=$22,
					outsource_tax_fee=$23,
					outsource_other_fee=$24,
					outsource_total_fee=$25,
					responsible_party_type=$26,
					responsible_party_id=$27,
					responsible_party_name=$28
				WHERE id=$1
			`, r.schema)
		if _, err := tx.Exec(ctx, uq,
			orderID,
			od,
			cmd.CustomerID,
			nullInt(cmd.SourceID),
			nullInt(cmd.OrderTypeID),
			nullInt(payStatusID),
			paymentMethod,
			nullInt(shipStatusID),
			nullText(shipMethod),
			nullText(shipTrackingNo),
			nullText(cmd.Notes),
			totalAmt,
			shippingAmt,
			discountAmt,
			roundToInt,
			roundingAmt,
			grandTotal,
			nullText(cmd.ExpressFee),
			outsourceFees[0],
			outsourceFees[1],
			outsourceFees[2],
			outsourceFees[3],
			outsourceFees[4],
			outsourceFees[5],
			outsourceTotal,
			responsibleType,
			responsibleID,
			responsibleName,
		); err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.order_items WHERE order_id=$1", r.schema), orderID); err != nil {
			return salesapp.SaveOrderResult{}, err
		}
	} else {
		orderNo, err = nextOrderNo(ctx, tx, r.schema, od)
		if err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		insertOrderSQL := fmt.Sprintf(`
				INSERT INTO %s.orders(
					order_date, customer_id,
					source_id, order_type_id, pay_status_id, payment_method, ship_status_id,
					ship_method, ship_tracking_no,
					notes,
					total_amount, shipping_amount, discount_amount,
					round_to_int, rounding_amount, grand_total,
					express_fee,
					outsource_material_fee, outsource_roast_fee, outsource_packaging_fee,
					outsource_manual_fee, outsource_tax_fee, outsource_other_fee, outsource_total_fee,
					responsible_party_type, responsible_party_id, responsible_party_name,
					order_no
				) VALUES (
					$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
					$11,$12,$13,
					$14,$15,$16,
					$17,$18,$19,$20,$21,$22,$23,$24,
					$25,$26,$27,
					$28
				)
				RETURNING id
			`, r.schema)
		err = tx.QueryRow(ctx, insertOrderSQL,
			od,
			cmd.CustomerID,
			nullInt(cmd.SourceID),
			nullInt(cmd.OrderTypeID),
			nullInt(payStatusID),
			paymentMethod,
			nullInt(shipStatusID),
			nullText(shipMethod),
			nullText(shipTrackingNo),
			nullText(cmd.Notes),
			totalAmt,
			shippingAmt,
			discountAmt,
			roundToInt,
			roundingAmt,
			grandTotal,
			nullText(cmd.ExpressFee),
			outsourceFees[0],
			outsourceFees[1],
			outsourceFees[2],
			outsourceFees[3],
			outsourceFees[4],
			outsourceFees[5],
			outsourceTotal,
			responsibleType,
			responsibleID,
			responsibleName,
			orderNo,
		).Scan(&orderID)
		if err != nil {
			return salesapp.SaveOrderResult{}, err
		}
	}
	if _, err := replaceOrderTrackingNumbersTx(ctx, tx, r.schema, orderID, shipTrackingNo, "order_form", cmd.Actor); err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	for idx, it := range items {
		qtyAny := any(nil)
		if it.units > 0 {
			qtyAny = it.units
		}
		if _, err := tx.Exec(ctx, insertItemSQL, orderID, idx+1, it.productID, it.tierID, it.priceOverride, it.name, it.note, qtyAny, it.unit, it.spec, it.unitPrice, it.lineTotal); err != nil {
			return salesapp.SaveOrderResult{}, err
		}
	}

	stockItems := make([]orderStockItem, 0, len(items))
	for _, it := range items {
		if it.productID == nil || *it.productID <= 0 || it.specG <= 0 || it.units <= 0 {
			continue
		}
		stockItems = append(stockItems, orderStockItem{
			ProductID:   *it.productID,
			ProductName: it.name,
			SpecG:       it.specG,
			Units:       it.units,
			NeedG:       it.specG * it.units,
		})
	}
	stockDecision := strings.TrimSpace(cmd.StockBatchDecision)
	if err := r.applyOrderStockDecisionTx(ctx, tx, orderID, stockItems, stockDecision, cmd.Actor); err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	r.logOrderSave(ctx, cmd.Actor, orderID, orderNo, editID > 0)

	return salesapp.SaveOrderResult{OrderID: orderID, OrderNo: orderNo, Edited: editID > 0, StockBatchUsed: stockDecision == "use_batch"}, nil

}

func (r Repository) UpdateHeader(ctx context.Context, id int64, cmd salesapp.UpdateHeaderCommand) error {
	if err := updateOrderHeader(ctx, r.pool, r.schema, id, cmd); err != nil {
		return err
	}
	r.logOrderHeaderUpdate(ctx, cmd.Actor, id)
	return nil
}

func (r Repository) InlineUpdate(ctx context.Context, id int64, actor string, cmd salesapp.InlineUpdateCommand) error {
	req := inlineUpdateRequest{
		OrderTypeID:     cmd.OrderTypeID,
		PayStatusID:     cmd.PayStatusID,
		PaymentMethod:   cmd.PaymentMethod,
		ShipStatusID:    cmd.ShipStatusID,
		ProcessStatusID: cmd.ProcessStatusID,
		Notes:           cmd.Notes,
	}
	return inlineUpdateOrder(ctx, r.pool, r.schema, id, actor, &req)
}

func (r Repository) Void(ctx context.Context, id int64, actor, reason string) error {
	q := fmt.Sprintf("UPDATE %s.orders SET is_void=true, voided_at=now(), void_reason=$2 WHERE id=$1", r.schema)
	if _, err := r.pool.Exec(ctx, q, id, nullText(reason)); err != nil {
		return err
	}
	var rv *string
	if strings.TrimSpace(reason) != "" {
		rv = &reason
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "order", &id, "void", nil, nil, rv, postgresinfra.AuditMeta{"order_id": id})
	return nil
}

func (r Repository) Unvoid(ctx context.Context, id int64, actor string) error {
	q := fmt.Sprintf("UPDATE %s.orders SET is_void=false, voided_at=NULL, void_reason=NULL WHERE id=$1", r.schema)
	if _, err := r.pool.Exec(ctx, q, id); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "order", &id, "unvoid", nil, nil, nil, postgresinfra.AuditMeta{"order_id": id})
	return nil
}

func (r Repository) logOrderSave(ctx context.Context, actor string, orderID int64, orderNo string, edited bool) {
	action := "create"
	field := "created"
	newValue := orderNo
	if edited {
		action = "update"
		field = "order"
		newValue = "updated"
	}
	r.insertOrderAudit(ctx, actor, orderID, field, nil, postgresinfra.StrPtr(newValue))
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "order", &orderID, action, postgresinfra.StrPtr(field), nil, postgresinfra.StrPtr(newValue), postgresinfra.AuditMeta{"order_id": orderID, "order_no": orderNo})
}

func (r Repository) logOrderHeaderUpdate(ctx context.Context, actor string, orderID int64) {
	r.insertOrderAudit(ctx, actor, orderID, "header", nil, postgresinfra.StrPtr("updated"))
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "order", &orderID, "update", postgresinfra.StrPtr("header"), nil, postgresinfra.StrPtr("updated"), postgresinfra.AuditMeta{"order_id": orderID})
}

func (r Repository) insertOrderAudit(ctx context.Context, actor string, orderID int64, field string, oldValue, newValue *string) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}
	_, _ = r.pool.Exec(ctx,
		fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value) VALUES ($1,$2,$3,$4,$5)`, r.schema),
		orderID,
		actor,
		field,
		oldValue,
		newValue,
	)
}

func (r Repository) ListOutsourceTemplates(ctx context.Context) ([]salesapp.OutsourceTemplate, error) {
	q := fmt.Sprintf(`SELECT id,name,is_default,COALESCE(roast_unit_price,0),COALESCE(bean_pack_unit_price,0),COALESCE(drip_pack_unit_price,0),COALESCE(sc_unit_price,0)
		FROM %s.outsource_templates WHERE active=true ORDER BY is_default DESC, id DESC`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.OutsourceTemplate, 0)
	for rows.Next() {
		var row salesapp.OutsourceTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.IsDefault, &row.RoastUnitPrice, &row.BeanPackUnitPrice, &row.DripPackUnitPrice, &row.SCUnitPrice); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveOutsourceTemplate(ctx context.Context, cmd salesapp.SaveOutsourceTemplateCommand) error {
	if cmd.IsDefault {
		if _, err := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.outsource_templates SET is_default=false WHERE is_default=true`, r.schema)); err != nil {
			return err
		}
	}
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.outsource_templates(name,is_default,roast_unit_price,bean_pack_unit_price,drip_pack_unit_price,sc_unit_price,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (name) DO UPDATE SET
			is_default=excluded.is_default,
			roast_unit_price=excluded.roast_unit_price,
			bean_pack_unit_price=excluded.bean_pack_unit_price,
			drip_pack_unit_price=excluded.drip_pack_unit_price,
			sc_unit_price=excluded.sc_unit_price,
			updated_at=now()`, r.schema),
		cmd.Name, cmd.IsDefault, cmd.RoastUnitPrice, cmd.BeanPackUnitPrice, cmd.DripPackUnitPrice, cmd.SCUnitPrice)
	return err
}

type inlineUpdateRequest struct {
	OrderTypeID     string
	PayStatusID     string
	PaymentMethod   string
	ShipStatusID    string
	ProcessStatusID string
	Notes           string
}

func inlineUpdateOrder(ctx context.Context, pool *pgxpool.Pool, schema string, orderID int64, actor string, req *inlineUpdateRequest) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}

	var curOrderType *int64
	var curPay *int64
	var curPaymentMethod *string
	var curShip *int64
	var curProc *int64
	var curNotes *string
	q := fmt.Sprintf("SELECT order_type_id, pay_status_id, payment_method, ship_status_id, process_status_id, notes FROM %s.orders WHERE id=$1", schema)
	if err := pool.QueryRow(ctx, q, orderID).Scan(&curOrderType, &curPay, &curPaymentMethod, &curShip, &curProc, &curNotes); err != nil {
		return err
	}

	parseID := func(s string) (*int64, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		if id <= 0 {
			return nil, nil
		}
		return &id, nil
	}

	nextOrderType, err := parseID(req.OrderTypeID)
	if err != nil {
		return fmt.Errorf("invalid order_type_id")
	}
	nextPay, err := parseID(req.PayStatusID)
	if err != nil {
		return fmt.Errorf("invalid pay_status_id")
	}
	nextShip, err := parseID(req.ShipStatusID)
	if err != nil {
		return fmt.Errorf("invalid ship_status_id")
	}
	nextProc, err := parseID(req.ProcessStatusID)
	if err != nil {
		return fmt.Errorf("invalid process_status_id")
	}
	paymentMethodRaw := strings.TrimSpace(req.PaymentMethod)
	if paymentMethodRaw == "" && eqIntPtr(curPay, nextPay) && curPaymentMethod != nil {
		paymentMethodRaw = strings.TrimSpace(*curPaymentMethod)
	}
	nextNotes := strings.TrimSpace(req.Notes)
	var nextNotesPtr *string
	if nextNotes != "" {
		nextNotesPtr = &nextNotes
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payStatusID := int64(0)
	if nextPay != nil {
		payStatusID = *nextPay
	}
	paymentMethod, err := normalizeOrderPaymentMethodForStatusTx(ctx, tx, schema, payStatusID, paymentMethodRaw)
	if err != nil {
		return err
	}
	var nextPaymentMethod *string
	if paymentMethod != "" {
		nextPaymentMethod = &paymentMethod
	}

	changed := false
	if !eqIntPtr(curOrderType, nextOrderType) || !eqIntPtr(curPay, nextPay) || !eqStrPtr(curPaymentMethod, nextPaymentMethod) || !eqIntPtr(curShip, nextShip) || !eqIntPtr(curProc, nextProc) || !eqStrPtr(curNotes, nextNotesPtr) {
		upd := fmt.Sprintf(`UPDATE %s.orders SET order_type_id=$2, pay_status_id=$3, payment_method=$4, ship_status_id=$5, process_status_id=$6, notes=$7 WHERE id=$1`, schema)
		if _, err := tx.Exec(ctx, upd, orderID, nextOrderType, nextPay, paymentMethod, nextShip, nextProc, nextNotesPtr); err != nil {
			return err
		}
		changed = true
	}

	ins := fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value) VALUES ($1,$2,$3,$4,$5)`, schema)
	logDiff := func(changed bool, field string, oldS, newS *string) error {
		if !changed {
			return nil
		}
		if _, err := tx.Exec(ctx, ins, orderID, actor, field, oldS, newS); err != nil {
			return err
		}
		return postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "order", &orderID, "update", postgresinfra.StrPtr(field), oldS, newS, postgresinfra.AuditMeta{"order_id": orderID})
	}
	if err := logDiff(!eqIntPtr(curOrderType, nextOrderType), "order_type_id", intPtrToStr(curOrderType), intPtrToStr(nextOrderType)); err != nil {
		return err
	}
	if err := logDiff(!eqIntPtr(curPay, nextPay), "pay_status_id", intPtrToStr(curPay), intPtrToStr(nextPay)); err != nil {
		return err
	}
	if err := logDiff(!eqStrPtr(curPaymentMethod, nextPaymentMethod), "payment_method", strPtr(curPaymentMethod), strPtr(nextPaymentMethod)); err != nil {
		return err
	}
	if err := logDiff(!eqIntPtr(curShip, nextShip), "ship_status_id", intPtrToStr(curShip), intPtrToStr(nextShip)); err != nil {
		return err
	}
	if err := logDiff(!eqIntPtr(curProc, nextProc), "process_status_id", intPtrToStr(curProc), intPtrToStr(nextProc)); err != nil {
		return err
	}
	if err := logDiff(!eqStrPtr(curNotes, nextNotesPtr), "notes", strPtr(curNotes), strPtr(nextNotesPtr)); err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return tx.Commit(ctx)
}

func updateOrderHeader(ctx context.Context, pool *pgxpool.Pool, schema string, id int64, req salesapp.UpdateHeaderCommand) error {
	orderDate := strings.TrimSpace(req.OrderDate)
	if orderDate == "" {
		return fmt.Errorf("order_date required")
	}
	if _, err := time.Parse("2006-01-02", orderDate); err != nil {
		return fmt.Errorf("invalid order_date")
	}
	if req.CustomerID <= 0 {
		return fmt.Errorf("customer required")
	}

	ship, err := parseFee(req.ShippingAmount)
	if err != nil {
		return fmt.Errorf("invalid shipping_amount")
	}
	disc, err := parseFee(req.DiscountAmount)
	if err != nil {
		return fmt.Errorf("invalid discount_amount")
	}
	round := strings.TrimSpace(req.RoundToInt) != ""

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(spec,''), COALESCE(qty,0), COALESCE(unit_price,0)
		FROM %s.order_items
		WHERE order_id=$1
		ORDER BY line_no, id
	`, schema), id)
	if err != nil {
		return err
	}
	defer rows.Close()

	type rowItem struct {
		id        int64
		specG     int64
		qty       float64
		unitPrice float64
	}
	old := make([]rowItem, 0)
	for rows.Next() {
		var rid int64
		var spec string
		var qty, up float64
		if err := rows.Scan(&rid, &spec, &qty, &up); err != nil {
			return err
		}
		old = append(old, rowItem{id: rid, specG: parseSpecG(spec), qty: qty, unitPrice: up})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	qtyMap := map[int64]float64{}
	upMap := map[int64]float64{}
	for i := 0; i < len(req.ItemID); i++ {
		rid, err := strconv.ParseInt(strings.TrimSpace(req.ItemID[i]), 10, 64)
		if err != nil || rid <= 0 {
			continue
		}
		q, _ := strconv.ParseFloat(strings.TrimSpace(getStr(req.Qty, i)), 64)
		if q < 0 {
			q = 0
		}
		u, _ := strconv.ParseFloat(strings.TrimSpace(getStr(req.UnitPrice, i)), 64)
		if u < 0 {
			u = 0
		}
		qtyMap[rid] = q
		upMap[rid] = u
	}

	totalAmt := 0.0
	for _, it := range old {
		q := it.qty
		if v, ok := qtyMap[it.id]; ok {
			q = v
		}
		u := it.unitPrice
		if v, ok := upMap[it.id]; ok {
			u = v
		}
		if q <= 0 {
			q = 0
		}
		if u < 0 {
			u = 0
		}
		qtyLb := (float64(it.specG) * q) / 454.0
		line := qtyLb * u
		totalAmt += line
		if _, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.order_items SET qty=$2,unit_price=$3,line_total=$4,price_overridden=true WHERE id=$1", schema), it.id, q, u, line); err != nil {
			return err
		}
	}

	outsourceTotal, outsourceFees, err := calcOutsourceTotal(req)
	if err != nil {
		return err
	}
	grand0 := totalAmt + ship - disc + outsourceTotal
	grandTotal, roundingAmt := applyRoundToInt(grand0, round)
	paymentMethod, err := normalizeOrderPaymentMethodForStatusTx(ctx, tx, schema, req.PayStatusID, req.PaymentMethod)
	if err != nil {
		return err
	}
	q := fmt.Sprintf(`
		UPDATE %s.orders
		SET order_date=$2,
			customer_id=$3,
			source_id=$4,
			order_type_id=$5,
			pay_status_id=$6,
			payment_method=$7,
			ship_status_id=$8,
			notes=$9,
			total_amount=$10,
			shipping_amount=$11,
			discount_amount=$12,
			round_to_int=$13,
			rounding_amount=$14,
			grand_total=$15,
			express_fee=$16,
			outsource_material_fee=$17,
			outsource_roast_fee=$18,
			outsource_packaging_fee=$19,
			outsource_manual_fee=$20,
			outsource_tax_fee=$21,
			outsource_other_fee=$22,
			outsource_total_fee=$23
		WHERE id=$1
	`, schema)
	if _, err := tx.Exec(ctx, q,
		id,
		orderDate,
		req.CustomerID,
		nullInt(req.SourceID),
		nullInt(req.OrderTypeID),
		nullInt(req.PayStatusID),
		paymentMethod,
		nullInt(req.ShipStatusID),
		nullText(req.Notes),
		totalAmt,
		ship,
		disc,
		round,
		roundingAmt,
		grandTotal,
		nullText(req.ExpressFee),
		outsourceFees[0],
		outsourceFees[1],
		outsourceFees[2],
		outsourceFees[3],
		outsourceFees[4],
		outsourceFees[5],
		outsourceTotal,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nextOrderNo(ctx context.Context, tx pgx.Tx, schema string, od time.Time) (string, error) {
	ymd := od.Format("20060102")
	prefix := "SO-" + ymd + "-"
	var maxNo int
	q := fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(right(order_no,4) AS INT)), 0)
		FROM %s.orders
		WHERE order_no LIKE $1
	`, schema)
	if err := tx.QueryRow(ctx, q, prefix+"%").Scan(&maxNo); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, maxNo+1), nil
}

func calcOutsourceTotal(req salesapp.UpdateHeaderCommand) (float64, [6]float64, error) {
	material, err := parseFee(req.OutsourceMaterialFee)
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_material_fee")
	}
	roast, err := parseFee(req.OutsourceRoastFee)
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_roast_fee")
	}
	packaging, err := parseFee(req.OutsourcePackagingFee)
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_packaging_fee")
	}
	manual, err := parseFee(req.OutsourceManualFee)
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_manual_fee")
	}
	tax, err := parseFee(req.OutsourceTaxFee)
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_tax_fee")
	}
	other, err := parseFee(req.OutsourceOtherFee)
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_other_fee")
	}
	fees := [6]float64{material, roast, packaging, manual, tax, other}
	return material + roast + packaging + manual + tax + other, fees, nil
}

func parseFee(v string) (float64, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, err
	}
	return f, nil
}

func applyRoundToInt(total float64, enabled bool) (grand float64, rounding float64) {
	return salesdomain.ApplyRoundToInt(total, enabled)
}

func isRetailOrderTypeName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "零售") || strings.Contains(name, "retail")
}

func parseSpecG(spec string) int64 {
	s := strings.TrimSpace(strings.ToLower(spec))
	s = strings.TrimSuffix(s, "g")
	n, _ := strconv.ParseInt(s, 10, 64)
	if n > 0 {
		return n
	}
	return 0
}

func getStr(s []string, i int) string {
	if i < 0 || i >= len(s) {
		return ""
	}
	return s[i]
}

func nullText(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}

func nullInt(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func eqIntPtr(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func eqStrPtr(a, b *string) bool {
	av := ""
	if a != nil {
		av = *a
	}
	bv := ""
	if b != nil {
		bv = *b
	}
	return av == bv
}

func intPtrToStr(p *int64) *string {
	if p == nil {
		return nil
	}
	s := fmt.Sprintf("%d", *p)
	return &s
}

func strPtr(p *string) *string {
	if p == nil {
		return nil
	}
	s := *p
	return &s
}
