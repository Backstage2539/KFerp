package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	pdfinfra "orderapp/internal/infrastructure/pdf"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/orderbeans"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool                     *pgxpool.Pool
	schema                   string
	assetDir                 string
	renderer                 SalesOrderPDFRenderer
	deliveryNoteRenderer     DeliveryNotePDFRenderer
	combinedSalesRenderer    CombinedSalesOrderPDFRenderer
	combinedDeliveryRenderer CombinedDeliveryNotePDFRenderer
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

type CombinedSalesOrderPDFRenderer interface {
	RenderCombinedSalesOrder(snapshot salesdomain.CombinedSalesOrderSnapshot) ([]byte, error)
	RenderCombinedSalesOrderPreview(snapshot salesdomain.CombinedSalesOrderSnapshot) ([]byte, error)
	RenderCombinedSalesOrderPNG(snapshot salesdomain.CombinedSalesOrderSnapshot) ([]byte, error)
}

type CombinedDeliveryNotePDFRenderer interface {
	RenderCombinedDeliveryNote(snapshot salesdomain.CombinedDeliveryNoteSnapshot) ([]byte, error)
	RenderCombinedDeliveryNotePreview(snapshot salesdomain.CombinedDeliveryNoteSnapshot) ([]byte, error)
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
		renderer := pdfinfra.SalesOrderRenderer{AssetBaseDir: repo.assetDir}
		repo.renderer = renderer
		repo.combinedSalesRenderer = renderer
	}
	if repo.deliveryNoteRenderer == nil {
		renderer := pdfinfra.DeliveryNoteRenderer{AssetBaseDir: repo.assetDir}
		repo.deliveryNoteRenderer = renderer
		repo.combinedDeliveryRenderer = renderer
	}
	if repo.combinedSalesRenderer == nil {
		repo.combinedSalesRenderer = pdfinfra.SalesOrderRenderer{AssetBaseDir: repo.assetDir}
	}
	if repo.combinedDeliveryRenderer == nil {
		repo.combinedDeliveryRenderer = pdfinfra.DeliveryNoteRenderer{AssetBaseDir: repo.assetDir}
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

func normalizeOrderItemDiscountType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "amount", "fixed", "minus":
		return "amount"
	case "unit_amount", "unit", "unit_discount", "per_unit", "unit_price":
		return "unit_amount"
	case "percent", "discount":
		return "percent"
	case "free":
		return "free"
	default:
		return ""
	}
}

type orderDiscountItem struct {
	productKind   string
	salesUnit     string
	quantityBasis string
	specG         int64
	units         int64
}

func orderItemUnitDiscountUnits(item orderDiscountItem, retailOrder bool) float64 {
	units := item.units
	if units <= 0 {
		return 0
	}
	if strings.TrimSpace(item.quantityBasis) == "sales_spec_count" {
		return float64(units)
	}
	if item.productKind == "drip_bag" || item.salesUnit == "bag" || item.salesUnit == "box" {
		return float64(units)
	}
	if retailOrder {
		return float64(units)
	}
	if item.specG <= 0 {
		return float64(units)
	}
	return float64(item.specG*units) / wholesaleDisplayUnitG(item.specG)
}

func orderManualPriceLineTotal(unitPrice float64, item orderDiscountItem, retailOrder bool) float64 {
	if unitPrice <= 0 || item.units <= 0 {
		return 0
	}
	if strings.TrimSpace(item.quantityBasis) == "sales_spec_count" || item.productKind == "drip_bag" || retailOrder {
		return unitPrice * float64(item.units)
	}
	return wholesaleLineTotalFromDisplayUnit(unitPrice, item.specG, item.units)
}

func applyOrderItemDiscount(baseLineTotal float64, discountType string, discountValue float64, discountUnits ...float64) (float64, float64) {
	baseLineTotal = maxFloat(baseLineTotal, 0)
	discountValue = maxFloat(discountValue, 0)
	if baseLineTotal <= 0 {
		return 0, 0
	}
	unitCount := 1.0
	if len(discountUnits) > 0 {
		unitCount = maxFloat(discountUnits[0], 0)
	}
	switch normalizeOrderItemDiscountType(discountType) {
	case "free":
		return baseLineTotal, 0
	case "amount":
		discountAmount := minFloat(discountValue, baseLineTotal)
		return discountAmount, maxFloat(baseLineTotal-discountAmount, 0)
	case "unit_amount":
		discountAmount := minFloat(discountValue*unitCount, baseLineTotal)
		return discountAmount, maxFloat(baseLineTotal-discountAmount, 0)
	case "percent":
		rate := minFloat(discountValue, 100)
		lineTotal := baseLineTotal * rate / 100
		return maxFloat(baseLineTotal-lineTotal, 0), maxFloat(lineTotal, 0)
	default:
		return 0, baseLineTotal
	}
}

func validateCustomerAliasPublishedPrice(customerProductAliasID int64, priceOverride bool, unitPrice float64) error {
	if customerProductAliasID <= 0 || priceOverride || unitPrice > 0 {
		return nil
	}
	return fmt.Errorf("customer product price unpublished")
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (r Repository) orderFulfillmentMarkersTx(ctx context.Context, tx pgx.Tx, customerID int64) (string, string) {
	if customerID <= 0 {
		return "", "finished_goods"
	}
	var hasBindingTable bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.customer_erp_user_bindings", r.schema)).Scan(&hasBindingTable); err != nil || !hasBindingTable {
		return "", "finished_goods"
	}
	var ok bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %[1]s.customer_erp_user_bindings b
			JOIN %[1]s.company_employees e ON e.id=b.employee_id
			LEFT JOIN %[1]s.employee_login_passwords lp ON lp.employee_id=e.id
			JOIN %[1]s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true
			WHERE b.customer_id=$1
			  AND b.status='active'
			  AND e.active=true
			  AND e.account_type='channel_customer'
			  AND COALESCE(lp.login_disabled,false)=false
			  AND (
			      p.capability_template_key IN ('processing_fulfillment','public_sku_direct_ship','channel_direct_ship')
			      OR EXISTS (
			          SELECT 1 FROM %[1]s.customer_capability_templates active_template
			          WHERE active_template.template_key=p.capability_template_key
			            AND active_template.active=true
			            AND (jsonb_array_length(active_template.erp_permissions)>0 OR jsonb_array_length(active_template.erp_view_keys)>0)
			      )
			  )
		)
	`, r.schema), customerID).Scan(&ok)
	if err != nil || !ok {
		return "", "finished_goods"
	}
	return "product_order", "finished_goods"
}

func resolveOrderFulfillmentMarkers(existingPortalServiceCode, existingSourceWarehouse, detectedPortalServiceCode, detectedSourceWarehouse string) (string, string) {
	existingPortalServiceCode = strings.TrimSpace(existingPortalServiceCode)
	existingSourceWarehouse = strings.TrimSpace(existingSourceWarehouse)
	detectedPortalServiceCode = strings.TrimSpace(detectedPortalServiceCode)
	detectedSourceWarehouse = strings.TrimSpace(detectedSourceWarehouse)
	if detectedSourceWarehouse == "" {
		detectedSourceWarehouse = "finished_goods"
	}
	if detectedPortalServiceCode == "" {
		return "", detectedSourceWarehouse
	}
	if existingPortalServiceCode != "" {
		if existingSourceWarehouse == "" {
			existingSourceWarehouse = detectedSourceWarehouse
		}
		return existingPortalServiceCode, existingSourceWarehouse
	}
	return detectedPortalServiceCode, detectedSourceWarehouse
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

func lookupStatusName(ctx context.Context, tx pgx.Tx, schema, table string, statusID int64) (string, error) {
	if statusID <= 0 {
		return "", nil
	}
	var statusName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.%s WHERE id=$1`, schema, table), statusID).Scan(&statusName); err != nil {
		return "", err
	}
	return strings.TrimSpace(statusName), nil
}

func orderPaidStatusRequiresReceipt(statusName string) bool {
	statusName = strings.TrimSpace(statusName)
	return strings.Contains(statusName, "已收款")
}

func orderShippedStatusRequiresLogistics(statusName string) bool {
	statusName = strings.TrimSpace(statusName)
	return strings.Contains(statusName, "已发货")
}

func validateOrderFulfillmentRequirementsTx(ctx context.Context, tx pgx.Tx, schema string, payStatusID, shipStatusID int64, logisticsCompanyID, logisticsProductID int64, paymentGoodsAmount, paymentShippingAmount float64, paymentVoucherAssetID int64) error {
	shipStatusName, err := lookupStatusName(ctx, tx, schema, "ship_statuses", shipStatusID)
	if err != nil {
		return fmt.Errorf("invalid ship_status_id")
	}
	if orderShippedStatusRequiresLogistics(shipStatusName) {
		if logisticsCompanyID <= 0 {
			return fmt.Errorf("logistics_company_id required")
		}
		if logisticsProductID <= 0 {
			return fmt.Errorf("logistics_product_id required")
		}
		var ok bool
		q := fmt.Sprintf(`
			SELECT EXISTS (
				SELECT 1
				FROM %s.logistics_products p
				JOIN %s.logistics_companies c ON c.id=p.company_id
				WHERE p.id=$1 AND c.id=$2 AND p.active=true AND c.active=true
			)
		`, schema, schema)
		if err := tx.QueryRow(ctx, q, logisticsProductID, logisticsCompanyID).Scan(&ok); err != nil || !ok {
			return fmt.Errorf("invalid logistics product")
		}
	} else {
		logisticsCompanyID = 0
		logisticsProductID = 0
	}
	_ = logisticsCompanyID
	_ = logisticsProductID

	payStatusName, err := lookupStatusName(ctx, tx, schema, "pay_statuses", payStatusID)
	if err != nil {
		return fmt.Errorf("invalid pay_status_id")
	}
	if orderPaidStatusRequiresReceipt(payStatusName) {
		if paymentGoodsAmount <= 0 {
			return fmt.Errorf("payment_goods_amount required")
		}
		if paymentShippingAmount < 0 {
			return fmt.Errorf("invalid payment_shipping_amount")
		}
		if paymentVoucherAssetID <= 0 {
			return fmt.Errorf("payment_voucher_asset_id required")
		}
		var ok bool
		q := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s.sales_order_assets WHERE id=$1 AND kind='payment_voucher')`, schema)
		if err := tx.QueryRow(ctx, q, paymentVoucherAssetID).Scan(&ok); err != nil || !ok {
			return fmt.Errorf("invalid payment_voucher_asset_id")
		}
	}
	return nil
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

func resolveOrderResponsibleParty(ctx context.Context, tx pgx.Tx, schema string, customerID int64) (string, int64, string, error) {
	if customerID <= 0 {
		return "", 0, "", fmt.Errorf("customer_id required")
	}
	var responsibleID int64
	var responsibleName string
	q := fmt.Sprintf(`
		SELECT COALESCE(c.responsible_employee_id,0), COALESCE(e.name,'')
		FROM %s.customers c
		LEFT JOIN %s.company_employees e ON e.id=c.responsible_employee_id
			AND e.active=true
			AND (e.account_type='internal_employee' OR COALESCE(e.account_type,'')='')
		WHERE c.id=$1 AND c.active=true
	`, schema, schema)
	if err := tx.QueryRow(ctx, q, customerID).Scan(&responsibleID, &responsibleName); err != nil {
		return "", 0, "", fmt.Errorf("customer not found")
	}
	if responsibleID <= 0 {
		return "", 0, "", fmt.Errorf("customer responsible employee required")
	}
	responsibleName = strings.TrimSpace(responsibleName)
	if responsibleName == "" {
		return "", 0, "", fmt.Errorf("customer responsible employee not found")
	}
	return "employee", responsibleID, responsibleName, nil
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

func wholesaleLineTotalFromPriceUnit(unitPrice float64, specG int64, units int64, unitG float64) float64 {
	if unitPrice <= 0 || specG <= 0 || units <= 0 {
		return 0
	}
	if unitG <= 0 {
		unitG = wholesaleDisplayUnitG(specG)
	}
	return unitPrice * (float64(specG*units) / unitG)
}

func publishedPricingLineTotal(pricing orderbeans.PublishedPricing, specG int64, units int64) float64 {
	if pricing.UnitPrice <= 0 || units <= 0 {
		return 0
	}
	if strings.TrimSpace(pricing.QuantityBasis) == "sales_spec_count" {
		return pricing.UnitPrice * float64(units)
	}
	return wholesaleLineTotalFromPriceUnit(pricing.UnitPrice, specG, units, pricing.UnitG)
}

func wholesaleTierQuantityForSpec(specG int64, units int64) float64 {
	if specG >= 1000 {
		return float64(specG*units) / 1000.0
	}
	return float64(units)
}

func selectedOrderBeanListPublicationID(cmd salesapp.SaveOrderCommand, listType string) int64 {
	switch strings.TrimSpace(listType) {
	case orderbeans.ListTypeGreen:
		return cmd.GreenBeanListPublicationID
	case orderbeans.ListTypeDrip:
		return cmd.DripBeanListPublicationID
	case orderbeans.ListTypeCommercial:
		if cmd.CommercialBeanListPublicationID > 0 {
			return cmd.CommercialBeanListPublicationID
		}
		return cmd.BeanListPublicationID
	case orderbeans.ListTypeRetail:
		return cmd.BeanListPublicationID
	default:
		return cmd.BeanListPublicationID
	}
}

func orderItemBeanListPublicationID(cmd salesapp.SaveOrderCommand, itemPublicationID int64, listType string) int64 {
	if itemPublicationID > 0 {
		return itemPublicationID
	}
	return selectedOrderBeanListPublicationID(cmd, listType)
}

type orderBeanListCandidate struct {
	ListType               string
	RequestedPublicationID int64
}

func dripOrderBeanListCandidates(cmd salesapp.SaveOrderCommand, itemPublicationID int64, sourceListType string) []orderBeanListCandidate {
	sourceListType = strings.TrimSpace(sourceListType)
	if itemPublicationID > 0 {
		switch sourceListType {
		case orderbeans.ListTypeCommercial, orderbeans.ListTypeDrip:
			return []orderBeanListCandidate{{ListType: sourceListType, RequestedPublicationID: itemPublicationID}}
		default:
			return []orderBeanListCandidate{
				{ListType: orderbeans.ListTypeCommercial, RequestedPublicationID: itemPublicationID},
				{ListType: orderbeans.ListTypeDrip, RequestedPublicationID: itemPublicationID},
			}
		}
	}
	return []orderBeanListCandidate{
		{ListType: orderbeans.ListTypeCommercial, RequestedPublicationID: selectedOrderBeanListPublicationID(cmd, orderbeans.ListTypeCommercial)},
		{ListType: orderbeans.ListTypeDrip, RequestedPublicationID: selectedOrderBeanListPublicationID(cmd, orderbeans.ListTypeDrip)},
	}
}

func orderBeanListTypeFromPriceSource(raw string) string {
	var source struct {
		ListType string `json:"list_type"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &source) != nil {
		return ""
	}
	switch strings.TrimSpace(source.ListType) {
	case orderbeans.ListTypeCommercial, orderbeans.ListTypeRetail, orderbeans.ListTypeGreen, orderbeans.ListTypeDrip:
		return strings.TrimSpace(source.ListType)
	default:
		return ""
	}
}

func orderQuantityBasisFromPriceSource(raw string) string {
	var source struct {
		QuantityBasis string `json:"quantity_basis"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &source) != nil {
		return ""
	}
	return strings.TrimSpace(source.QuantityBasis)
}

func beanListPriceSourceJSON(listType string, usage orderbeans.Usage, productID int64) string {
	source := map[string]any{
		"source":                   "bean_list_publication",
		"list_type":                strings.TrimSpace(listType),
		"bean_list_publication_id": usage.PublicationID,
		"bean_list_version_no":     usage.VersionNo,
		"publication_id":           usage.PublicationID,
		"version":                  usage.VersionNo,
		"product_id":               productID,
	}
	buf, err := json.Marshal(source)
	if err != nil {
		return "{}"
	}
	return string(buf)
}

func beanListPriceSourceJSONWithPricing(listType string, usage orderbeans.Usage, productID int64, pricing orderbeans.PublishedPricing) string {
	source := map[string]any{
		"source":                   "bean_list_publication",
		"list_type":                strings.TrimSpace(listType),
		"bean_list_publication_id": usage.PublicationID,
		"bean_list_version_no":     usage.VersionNo,
		"publication_id":           usage.PublicationID,
		"version":                  usage.VersionNo,
		"product_id":               productID,
	}
	if strings.TrimSpace(pricing.PriceUnit) != "" {
		source["price_unit"] = strings.TrimSpace(pricing.PriceUnit)
	}
	if pricing.UnitG > 0 {
		source["price_unit_g"] = pricing.UnitG
	}
	if pricing.SourcePriceRecordID > 0 {
		source["source_price_record_id"] = pricing.SourcePriceRecordID
	}
	if strings.TrimSpace(pricing.TierLabel) != "" {
		source["tier_label"] = strings.TrimSpace(pricing.TierLabel)
	}
	if pricing.FinalUnitPrice > 0 {
		source["final_unit_price"] = pricing.FinalUnitPrice
	}
	if strings.TrimSpace(pricing.PricingRuleVersion) != "" {
		source["pricing_rule_version"] = strings.TrimSpace(pricing.PricingRuleVersion)
	}
	source["manual_adjusted"] = pricing.ManualAdjusted
	if strings.TrimSpace(pricing.InventoryUnit) != "" {
		source["inventory_unit"] = strings.TrimSpace(pricing.InventoryUnit)
	}
	if strings.TrimSpace(pricing.InventoryConversionJSON) != "" {
		var conversion map[string]any
		if err := json.Unmarshal([]byte(pricing.InventoryConversionJSON), &conversion); err == nil && len(conversion) > 0 {
			source["inventory_conversion_json"] = conversion
		}
	}
	if strings.TrimSpace(pricing.CostSourceSnapshotJSON) != "" {
		var snapshot map[string]any
		if err := json.Unmarshal([]byte(pricing.CostSourceSnapshotJSON), &snapshot); err == nil && len(snapshot) > 0 {
			source["cost_source_snapshot"] = snapshot
		}
	}
	if strings.TrimSpace(pricing.CustomerSnapshotJSON) != "" {
		var snapshot map[string]any
		if err := json.Unmarshal([]byte(pricing.CustomerSnapshotJSON), &snapshot); err == nil && len(snapshot) > 0 {
			source["customer_reference_snapshot"] = snapshot
		}
	}
	if strings.TrimSpace(pricing.QuantityBasis) != "" {
		source["quantity_basis"] = strings.TrimSpace(pricing.QuantityBasis)
	}
	if strings.TrimSpace(pricing.TierQuantityUnit) != "" {
		source["tier_quantity_unit"] = strings.TrimSpace(pricing.TierQuantityUnit)
	}
	if strings.TrimSpace(pricing.EffectiveSalesSpecJSON) != "" {
		var snapshot map[string]any
		if err := json.Unmarshal([]byte(pricing.EffectiveSalesSpecJSON), &snapshot); err == nil && len(snapshot) > 0 {
			source["effective_sales_spec"] = snapshot
		}
	}
	buf, err := json.Marshal(source)
	if err != nil {
		return "{}"
	}
	return string(buf)
}

func normalizeOrderItemProductKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return ""
	}
	if kind == "drip_bag" {
		return "drip_bag"
	}
	return "roasted_bean"
}

func normalizeOrderItemSalesUnit(unit string) string {
	unit = strings.TrimSpace(unit)
	switch unit {
	case "bag", "袋":
		return "bag"
	case "box", "盒":
		return "box"
	default:
		return unit
	}
}

func dripOrderItemLabel(salesUnit string, unitBagCount int64, unitBeanG float64) string {
	switch normalizeOrderItemSalesUnit(salesUnit) {
	case "box":
		if unitBagCount > 0 {
			return fmt.Sprintf("%d袋/盒", unitBagCount)
		}
	case "bag":
		if unitBeanG > 0 {
			return fmt.Sprintf("%s/袋", formatOrderDecimal(unitBeanG, "g"))
		}
	}
	return ""
}

func formatOrderDecimal(v float64, suffix string) string {
	if math.Abs(v-math.Round(v)) < 0.000001 {
		return fmt.Sprintf("%.0f%s", v, suffix)
	}
	return fmt.Sprintf("%.3f%s", v, suffix)
}

func orderItemWeightG(productKind string, salesUnit string, unitBeanG float64, unitBagCount int64, specG int64, units int64) int64 {
	if productKind == "drip_bag" {
		bagsPerUnit := unitBagCount
		if salesUnit != "box" || bagsPerUnit <= 0 {
			bagsPerUnit = 1
		}
		return int64(math.Round(unitBeanG * float64(bagsPerUnit) * float64(units)))
	}
	return specG * units
}

func loadOrderProductUnitDefaultsTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (string, float64, int64) {
	if productID <= 0 {
		return "", 0, 0
	}
	var productKind string
	var unitBeanG float64
	var unitBagCount int64
	q := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
			COALESCE(drip_bag_grams, 0)::float8,
			COALESCE(drip_box_bag_count, 0)
		FROM %s.products
		WHERE id=$1
	`, schema)
	if err := tx.QueryRow(ctx, q, productID).Scan(&productKind, &unitBeanG, &unitBagCount); err != nil {
		return "", 0, 0
	}
	return normalizeOrderItemProductKind(productKind), unitBeanG, unitBagCount
}

type concreteOrderProductIdentity struct {
	SKUID             int64
	ParentProductID   int64
	ParentProductName string
	SKUName           string
	SKUCode           string
	ProductKind       string
}

type concreteOrderPublicationSelection struct {
	Strict   bool
	ListType string
	Usage    orderbeans.Usage
	Spec     orderbeans.PublishedProductSpec
	Product  concreteOrderProductIdentity
}

func loadConcreteOrderProductIdentityTx(ctx context.Context, tx pgx.Tx, schema string, skuID int64) (concreteOrderProductIdentity, error) {
	var result concreteOrderProductIdentity
	q := fmt.Sprintf(`
		SELECT p.id,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END,
		       COALESCE(NULLIF(parent.name,''),p.name,''),
		       COALESCE(NULLIF(p.sku_name,''),NULLIF(p.spec_label,''),p.name,''),
		       COALESCE(NULLIF(p.sku_code,''),'SKU-' || p.id::text),
		       COALESCE(NULLIF(p.product_kind,''),'roasted_bean')
		FROM %[1]s.products p
		LEFT JOIN %[1]s.products parent ON parent.id=p.parent_product_id
		WHERE p.id=$1
		  AND COALESCE(p.active,true)=true
		  AND (NOT COALESCE(p.auto_derived_sku,false) OR COALESCE(NULLIF(p.derived_spec_status,''),'active')<>'template_removed')
		  AND (COALESCE(p.parent_product_id,0)=0 OR (parent.id IS NOT NULL AND COALESCE(parent.active,true)=true))
	`, schema)
	if err := tx.QueryRow(ctx, q, skuID).Scan(&result.SKUID, &result.ParentProductID, &result.ParentProductName, &result.SKUName, &result.SKUCode, &result.ProductKind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return concreteOrderProductIdentity{}, fmt.Errorf("商品规格已停用或不存在，请重新选择")
		}
		return concreteOrderProductIdentity{}, err
	}
	return result, nil
}

func resolveConcreteOrderPublicationSelectionTx(ctx context.Context, tx pgx.Tx, schema string, cmd salesapp.SaveOrderCommand, productID, submittedParentProductID int64, productKind string, retailOrder bool, itemPublicationID int64, sourceListType, sourceJSON string) (concreteOrderPublicationSelection, error) {
	if productID <= 0 {
		return concreteOrderPublicationSelection{}, nil
	}
	candidates := make([]orderBeanListCandidate, 0, 2)
	if strings.TrimSpace(productKind) == "drip_bag" {
		candidates = dripOrderBeanListCandidates(cmd, itemPublicationID, sourceListType)
	} else {
		listType := orderbeans.ListTypeForProductKind(productKind, retailOrder)
		candidates = append(candidates, orderBeanListCandidate{
			ListType:               listType,
			RequestedPublicationID: orderItemBeanListPublicationID(cmd, itemPublicationID, listType),
		})
	}
	requestedPublication := false
	for _, candidate := range candidates {
		if candidate.RequestedPublicationID > 0 {
			requestedPublication = true
		}
		usage, spec, err := orderbeans.ResolvePublishedProductSpecForPublication(ctx, tx, schema, cmd.CustomerID, productID, candidate.ListType, candidate.RequestedPublicationID)
		if err != nil {
			return concreteOrderPublicationSelection{}, err
		}
		if usage.PublicationID <= 0 {
			continue
		}
		if !spec.ConcretePublication {
			// A commercial legacy publication still wins over historical drip;
			// the existing pricing path decides whether it contains a usable tier.
			if candidate.RequestedPublicationID > 0 && !spec.ProductFound {
				return concreteOrderPublicationSelection{}, fmt.Errorf("所选价格表版本不包含该商品规格，请重新选择价格表和规格")
			}
			return concreteOrderPublicationSelection{
				ListType: candidate.ListType,
				Usage:    usage,
				Spec:     spec,
			}, nil
		}
		if !spec.ProductFound {
			return concreteOrderPublicationSelection{}, fmt.Errorf("所选价格表版本不包含该商品规格，请重新选择价格表和规格")
		}
		product, err := loadConcreteOrderProductIdentityTx(ctx, tx, schema, productID)
		if err != nil {
			return concreteOrderPublicationSelection{}, err
		}
		if spec.SKUID != product.SKUID {
			return concreteOrderPublicationSelection{}, fmt.Errorf("价格表 SKU 与订单商品规格不一致")
		}
		if spec.ParentProductID != product.ParentProductID {
			return concreteOrderPublicationSelection{}, fmt.Errorf("价格表 SKU 不属于订单商品，请重新选择规格")
		}
		if submittedParentProductID > 0 && submittedParentProductID != product.ParentProductID {
			return concreteOrderPublicationSelection{}, fmt.Errorf("提交的商品与 SKU 归属不一致，请重新选择商品规格")
		}
		if err := validateConcreteOrderPriceSourceIdentity(sourceJSON, usage, spec); err != nil {
			return concreteOrderPublicationSelection{}, err
		}
		return concreteOrderPublicationSelection{
			Strict:   true,
			ListType: candidate.ListType,
			Usage:    usage,
			Spec:     spec,
			Product:  product,
		}, nil
	}
	if requestedPublication {
		return concreteOrderPublicationSelection{}, fmt.Errorf("所选价格表版本无效、无权访问或不包含该商品规格，请重新选择价格表和规格")
	}
	return concreteOrderPublicationSelection{}, nil
}

func validateConcreteOrderPriceSourceIdentity(raw string, usage orderbeans.Usage, spec orderbeans.PublishedProductSpec) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var source map[string]any
	if json.Unmarshal([]byte(raw), &source) != nil {
		return nil
	}
	publicationID := anyInt64(source["publication_id"])
	if publicationID <= 0 {
		publicationID = anyInt64(source["bean_list_publication_id"])
	}
	if publicationID > 0 && publicationID != usage.PublicationID {
		return fmt.Errorf("订单行价格表版本与所选发布版本不一致")
	}
	if sourceSKUID := anyInt64(source["sku_id"]); sourceSKUID > 0 && sourceSKUID != spec.SKUID {
		return fmt.Errorf("订单行 SKU 与价格表规格快照不一致")
	}
	if sourceParentID := anyInt64(source["parent_product_id"]); sourceParentID > 0 && sourceParentID != spec.ParentProductID {
		return fmt.Errorf("订单行商品与价格表规格快照不一致")
	}
	if frozen, ok := source["effective_sales_spec"].(map[string]any); ok {
		if sourceSKUID := anyInt64(frozen["sku_id"]); sourceSKUID > 0 && sourceSKUID != spec.SKUID {
			return fmt.Errorf("订单行有效销售规格 SKU 与价格表不一致")
		}
		for _, field := range []struct {
			key  string
			want string
		}{
			{key: "spec_key", want: spec.SpecKey},
			{key: "spec_name", want: spec.SpecName},
			{key: "spec_label", want: spec.SpecLabel},
			{key: "sales_unit", want: spec.SalesUnit},
			{key: "net_content_unit", want: spec.NetContentUnit},
		} {
			if got := strings.TrimSpace(fmt.Sprint(frozen[field.key])); frozen[field.key] != nil && got != "" && got != strings.TrimSpace(field.want) {
				return fmt.Errorf("订单行有效销售规格 %s 与价格表不一致", field.key)
			}
		}
		if got := anyFloat64(frozen["net_content_qty"]); got > 0 && math.Abs(got-spec.NetContentQty) > 0.000001 {
			return fmt.Errorf("订单行有效销售规格净含量与价格表不一致")
		}
	}
	return nil
}

func anyFloat64(value any) float64 {
	switch value := value.(type) {
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case float64:
		return value
	case json.Number:
		parsed, _ := value.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed
	default:
		return 0
	}
}

func anyInt64(value any) int64 {
	switch value := value.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return parsed
	default:
		return 0
	}
}

func concreteOrderSpecWeightG(spec orderbeans.PublishedProductSpec) int64 {
	if spec.NetContentQty <= 0 {
		return 0
	}
	factor := float64(0)
	switch strings.ToLower(strings.TrimSpace(spec.NetContentUnit)) {
	case "g", "克":
		factor = 1
	case "kg", "千克", "公斤":
		factor = 1000
	case "lb", "lbs", "磅":
		factor = 453.59237
	}
	if factor <= 0 {
		return 0
	}
	return int64(math.Round(spec.NetContentQty * factor))
}

func manualConcreteOrderPriceSourceJSON(selection concreteOrderPublicationSelection, productID int64) string {
	source := map[string]any{
		"source":                   "manual",
		"list_type":                selection.ListType,
		"publication_id":           selection.Usage.PublicationID,
		"bean_list_publication_id": selection.Usage.PublicationID,
		"version_no":               selection.Usage.VersionNo,
		"bean_list_version_no":     selection.Usage.VersionNo,
		"product_id":               productID,
		"sku_id":                   selection.Spec.SKUID,
		"parent_product_id":        selection.Spec.ParentProductID,
		"quantity_basis":           "sales_spec_count",
	}
	if raw := strings.TrimSpace(selection.Spec.EffectiveSalesSpecJSON); raw != "" {
		var snapshot map[string]any
		if json.Unmarshal([]byte(raw), &snapshot) == nil && len(snapshot) > 0 {
			source["effective_sales_spec"] = snapshot
		}
	}
	buf, err := json.Marshal(source)
	if err != nil {
		return `{"source":"manual"}`
	}
	return string(buf)
}

func (r Repository) SaveOrder(ctx context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	od := cmd.OrderDate
	if od.IsZero() {
		return salesapp.SaveOrderResult{}, fmt.Errorf("invalid order_date")
	}
	documentDate := cmd.DocumentDate
	if documentDate.IsZero() {
		documentDate = od
	}
	if cmd.CustomerID <= 0 {
		return salesapp.SaveOrderResult{}, fmt.Errorf("customer required")
	}

	type item struct {
		productID                          *int64
		submittedParentProductID           int64
		customerProductAliasID             int64
		customerProductDisplayNameSnapshot string
		customerItemCodeSnapshot           string
		brandNameSnapshot                  string
		productCodeSnapshot                string
		productNameSnapshot                string
		itemBeanListPublicationID          int64
		itemBeanListVersionNo              string
		tierID                             *int64
		manualPrice                        *float64
		discountType                       string
		discountValue                      float64
		discountAmount                     float64
		baseLineTotal                      float64
		name                               string
		note                               string
		units                              int64
		specG                              int64
		unit                               *string
		spec                               *string
		unitPrice                          float64
		lineTotal                          float64
		priceOverride                      bool
		productKind                        string
		salesUnit                          string
		unitBagCount                       int64
		unitBeanG                          float64
		matchedPriceQty                    float64
		priceSourceJSON                    string
		priceListType                      string
		quantityBasis                      string
	}
	items := make([]item, 0, len(cmd.Items))
	for _, src := range cmd.Items {
		name := strings.TrimSpace(src.Name)
		if src.ProductID == nil && name == "" {
			continue
		}
		it := item{
			productID:                          src.ProductID,
			submittedParentProductID:           src.ParentProductID,
			customerProductAliasID:             src.CustomerProductAliasID,
			customerProductDisplayNameSnapshot: strings.TrimSpace(src.CustomerProductDisplayNameSnapshot),
			customerItemCodeSnapshot:           strings.TrimSpace(src.CustomerItemCodeSnapshot),
			brandNameSnapshot:                  strings.TrimSpace(src.BrandNameSnapshot),
			productCodeSnapshot:                strings.TrimSpace(src.ProductCodeSnapshot),
			productNameSnapshot:                strings.TrimSpace(src.ProductNameSnapshot),
			itemBeanListPublicationID:          src.BeanListPublicationID,
			itemBeanListVersionNo:              strings.TrimSpace(src.BeanListVersionNo),
			tierID:                             src.TierID,
			manualPrice:                        src.ManualPrice,
			discountType:                       normalizeOrderItemDiscountType(src.DiscountType),
			discountValue:                      maxFloat(src.DiscountValue, 0),
			name:                               name,
			note:                               strings.TrimSpace(src.Note),
			units:                              src.Units,
			specG:                              src.SpecG,
			productKind:                        normalizeOrderItemProductKind(src.ProductKind),
			salesUnit:                          normalizeOrderItemSalesUnit(src.SalesUnit),
			unitBagCount:                       src.UnitBagCount,
			unitBeanG:                          src.UnitBeanG,
			priceSourceJSON:                    strings.TrimSpace(src.PriceSourceJSON),
			priceListType:                      orderBeanListTypeFromPriceSource(src.PriceSourceJSON),
			quantityBasis:                      orderQuantityBasisFromPriceSource(src.PriceSourceJSON),
		}
		if it.productKind == "drip_bag" {
			if it.unitBeanG <= 0 && it.specG > 0 {
				it.unitBeanG = float64(it.specG)
			}
			if it.specG <= 0 && it.unitBeanG > 0 {
				it.specG = int64(math.Round(it.unitBeanG))
			}
			if it.unitBagCount <= 0 {
				it.unitBagCount = 1
			}
		}
		if it.manualPrice != nil {
			it.priceOverride = true
		}
		if it.productKind == "drip_bag" {
			spec := dripOrderItemLabel(it.salesUnit, it.unitBagCount, it.unitBeanG)
			it.spec = &spec
		} else if src.SpecG > 0 {
			spec := fmt.Sprintf("%dg", src.SpecG)
			it.spec = &spec
		}
		if unit := strings.TrimSpace(src.Unit); unit != "" {
			it.unit = &unit
			if it.productKind == "drip_bag" && it.salesUnit == "" {
				it.salesUnit = normalizeOrderItemSalesUnit(unit)
			}
		}
		if it.productKind == "drip_bag" && it.salesUnit == "" {
			it.salesUnit = "bag"
		}
		if it.productKind == "drip_bag" {
			spec := dripOrderItemLabel(it.salesUnit, it.unitBagCount, it.unitBeanG)
			it.spec = &spec
		}
		items = append(items, it)
	}
	// Concrete publications restore the authoritative spec after the transaction
	// starts, so only concrete product identity and quantity are required here.
	valid := false
	for _, it := range items {
		if it.productID != nil && *it.productID > 0 && it.units > 0 {
			valid = true
			break
		}
	}
	if !valid {
		return salesapp.SaveOrderResult{}, fmt.Errorf("at least one item required")
	}
	orderSaveDripAuditSummary := func() string {
		dripItems := make([]map[string]any, 0)
		for _, it := range items {
			if it.productKind != "drip_bag" {
				continue
			}
			itemSummary := map[string]any{
				"product_kind":      it.productKind,
				"sales_unit":        it.salesUnit,
				"qty":               it.units,
				"unit_bag_count":    it.unitBagCount,
				"unit_bean_g":       it.unitBeanG,
				"matched_price_qty": it.matchedPriceQty,
				"unit_price":        it.unitPrice,
				"line_total":        it.lineTotal,
			}
			var source map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(it.priceSourceJSON)), &source); err == nil && len(source) > 0 {
				itemSummary["price_source"] = source
			}
			dripItems = append(dripItems, itemSummary)
		}
		if len(dripItems) == 0 {
			return ""
		}
		buf, err := json.Marshal(map[string]any{"drip_items": dripItems})
		if err != nil {
			return ""
		}
		return string(buf)
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

	customerProfile, err := r.requiredOrderCustomerProfileTx(ctx, tx, cmd.CustomerID)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}
	cmd.SourceID = customerProfile.sourceID
	cmd.OrderTypeID = customerProfile.orderTypeID

	orderNo := ""
	retailOrder := false
	if cmd.OrderTypeID > 0 {
		var orderTypeName string
		_ = tx.QueryRow(ctx, fmt.Sprintf("SELECT COALESCE(name,'') FROM %s.order_types WHERE id=$1", r.schema), cmd.OrderTypeID).Scan(&orderTypeName)
		retailOrder = isRetailOrderTypeName(orderTypeName)
	}
	applyItemDiscount := func(idx int) {
		discountUnits := orderItemUnitDiscountUnits(orderDiscountItem{
			productKind:   items[idx].productKind,
			salesUnit:     items[idx].salesUnit,
			quantityBasis: items[idx].quantityBasis,
			specG:         items[idx].specG,
			units:         items[idx].units,
		}, retailOrder)
		items[idx].discountAmount, items[idx].lineTotal = applyOrderItemDiscount(items[idx].baseLineTotal, items[idx].discountType, items[idx].discountValue, discountUnits)
	}
	for idx := range items {
		items[idx].productKind = "roasted"
		if items[idx].productID != nil && *items[idx].productID > 0 {
			items[idx].productKind = productKindForOrderItem(ctx, tx, r.schema, *items[idx].productID)
		}
	}

	// Pricing: wholesale tiers prefer exact package spec tiers, then fall back to
	// bean-list weight tiers so non-454g packaging can still price by total lb.
	totalAmt := 0.0
	itemDiscountAmt := 0.0
	orderWeightG := int64(0)
	for idx := range items {
		if items[idx].productID != nil {
			productKind, unitBeanG, unitBagCount := loadOrderProductUnitDefaultsTx(ctx, tx, r.schema, *items[idx].productID)
			if items[idx].productKind == "" && productKind != "" {
				items[idx].productKind = productKind
			}
			if items[idx].productKind == "drip_bag" {
				if items[idx].unitBeanG <= 0 {
					items[idx].unitBeanG = unitBeanG
				}
				if items[idx].unitBagCount <= 0 {
					items[idx].unitBagCount = unitBagCount
				}
				if items[idx].unitBagCount <= 0 {
					items[idx].unitBagCount = 1
				}
				if items[idx].salesUnit == "" {
					items[idx].salesUnit = "bag"
				}
				if items[idx].specG <= 0 && items[idx].unitBeanG > 0 {
					items[idx].specG = int64(math.Round(items[idx].unitBeanG))
				}
				spec := dripOrderItemLabel(items[idx].salesUnit, items[idx].unitBagCount, items[idx].unitBeanG)
				items[idx].spec = &spec
			}
		}
		if items[idx].productKind == "" {
			items[idx].productKind = "roasted_bean"
		}
		selection := concreteOrderPublicationSelection{}
		if items[idx].productID != nil && *items[idx].productID > 0 {
			if items[idx].submittedParentProductID > 0 {
				identity, identityErr := loadConcreteOrderProductIdentityTx(ctx, tx, r.schema, *items[idx].productID)
				if identityErr != nil {
					return salesapp.SaveOrderResult{}, identityErr
				}
				if identity.ParentProductID != items[idx].submittedParentProductID {
					return salesapp.SaveOrderResult{}, fmt.Errorf("提交的商品与 SKU 归属不一致，请重新选择商品规格")
				}
			}
			selection, err = resolveConcreteOrderPublicationSelectionTx(
				ctx,
				tx,
				r.schema,
				cmd,
				*items[idx].productID,
				items[idx].submittedParentProductID,
				items[idx].productKind,
				retailOrder,
				items[idx].itemBeanListPublicationID,
				items[idx].priceListType,
				items[idx].priceSourceJSON,
			)
			if err != nil {
				return salesapp.SaveOrderResult{}, err
			}
			if items[idx].submittedParentProductID > 0 && selection.Usage.PublicationID <= 0 {
				return salesapp.SaveOrderResult{}, fmt.Errorf("所选价格表版本不包含该商品规格，请重新选择价格表和规格")
			}
		}
		if selection.Strict {
			items[idx].productKind = strings.TrimSpace(selection.Product.ProductKind)
			items[idx].priceListType = selection.ListType
			items[idx].itemBeanListPublicationID = selection.Usage.PublicationID
			items[idx].itemBeanListVersionNo = selection.Usage.VersionNo
			items[idx].quantityBasis = "sales_spec_count"
			if frozenSpecG := concreteOrderSpecWeightG(selection.Spec); frozenSpecG > 0 {
				items[idx].specG = frozenSpecG
			}
			items[idx].salesUnit = normalizeOrderItemSalesUnit(selection.Spec.SalesUnit)
			if items[idx].salesUnit == "" {
				return salesapp.SaveOrderResult{}, fmt.Errorf("价格表商品规格缺少销售单位，请重新发布价格表")
			}
			unit := strings.TrimSpace(selection.Spec.SalesUnit)
			items[idx].unit = &unit
			specLabel := firstNonEmpty(selection.Spec.SpecLabel, selection.Spec.SpecName, selection.Product.SKUName)
			items[idx].spec = &specLabel
			if selection.Spec.UnitBagCount > 0 {
				items[idx].unitBagCount = selection.Spec.UnitBagCount
			}
			if selection.Spec.UnitBeanG > 0 {
				items[idx].unitBeanG = selection.Spec.UnitBeanG
			}
			aliasSnapshot, aliasErr := resolveOrderItemCustomerAliasSnapshotTx(ctx, tx, r.schema, cmd.CustomerID, *items[idx].productID, items[idx].customerProductAliasID)
			if aliasErr != nil {
				return salesapp.SaveOrderResult{}, aliasErr
			}
			items[idx].name = firstNonEmpty(aliasSnapshot.DisplayName, selection.Product.ParentProductName)
			items[idx].productNameSnapshot = selection.Product.ParentProductName
			items[idx].productCodeSnapshot = selection.Product.SKUCode
			if aliasSnapshot.AliasID > 0 {
				items[idx].customerProductAliasID = aliasSnapshot.AliasID
				items[idx].customerProductDisplayNameSnapshot = aliasSnapshot.DisplayName
				items[idx].customerItemCodeSnapshot = aliasSnapshot.CustomerItemCode
				items[idx].brandNameSnapshot = aliasSnapshot.BrandName
			}
			items[idx].matchedPriceQty = float64(items[idx].units)
		} else if items[idx].productID != nil && items[idx].specG <= 0 {
			return salesapp.SaveOrderResult{}, fmt.Errorf("at least one item required")
		}
		itemWeightG := orderItemWeightG(items[idx].productKind, items[idx].salesUnit, items[idx].unitBeanG, items[idx].unitBagCount, items[idx].specG, items[idx].units)
		orderWeightG += itemWeightG

		if items[idx].manualPrice != nil && (*items[idx].manualPrice <= 0 || math.IsNaN(*items[idx].manualPrice) || math.IsInf(*items[idx].manualPrice, 0)) {
			return salesapp.SaveOrderResult{}, fmt.Errorf("手动单价必须大于0")
		}
		if items[idx].manualPrice != nil {
			discountItem := orderDiscountItem{
				productKind:   items[idx].productKind,
				salesUnit:     items[idx].salesUnit,
				quantityBasis: items[idx].quantityBasis,
				specG:         items[idx].specG,
				units:         items[idx].units,
			}
			lineTotal := orderManualPriceLineTotal(*items[idx].manualPrice, discountItem, retailOrder)
			if selection.Strict {
				items[idx].matchedPriceQty = float64(items[idx].units)
				items[idx].priceSourceJSON = manualConcreteOrderPriceSourceJSON(selection, *items[idx].productID)
			} else if items[idx].productKind == "drip_bag" {
				items[idx].matchedPriceQty = float64(items[idx].units)
				items[idx].priceSourceJSON = `{"source":"manual"}`
			}
			items[idx].baseLineTotal = lineTotal
			applyItemDiscount(idx)
			items[idx].unitPrice = *items[idx].manualPrice
			items[idx].priceOverride = true
			totalAmt += items[idx].baseLineTotal
			itemDiscountAmt += items[idx].discountAmount
			continue
		} else if items[idx].productKind == "drip_bag" && items[idx].productID != nil {
			var usage orderbeans.Usage
			var pricing orderbeans.PublishedPricing
			var priceListType string
			for _, candidate := range dripOrderBeanListCandidates(cmd, items[idx].itemBeanListPublicationID, items[idx].priceListType) {
				resolvedUsage, err := orderbeans.ResolveUsageForPublication(ctx, tx, r.schema, cmd.CustomerID, *items[idx].productID, candidate.ListType, candidate.RequestedPublicationID)
				if err != nil {
					return salesapp.SaveOrderResult{}, err
				}
				if resolvedUsage.PublicationID <= 0 {
					continue
				}
				resolvedPricing, err := orderbeans.ResolvePublishedPricingForPublicationWithUnit(ctx, tx, r.schema, cmd.CustomerID, *items[idx].productID, candidate.ListType, resolvedUsage.PublicationID, items[idx].specG, items[idx].units, items[idx].salesUnit, items[idx].unitBagCount)
				if err != nil {
					return salesapp.SaveOrderResult{}, err
				}
				if resolvedPricing.UnitPrice <= 0 {
					return salesapp.SaveOrderResult{}, fmt.Errorf("缺少挂耳价格表价格")
				}
				usage = resolvedUsage
				pricing = resolvedPricing
				priceListType = candidate.ListType
				break
			}
			if usage.PublicationID <= 0 || pricing.UnitPrice <= 0 {
				return salesapp.SaveOrderResult{}, fmt.Errorf("缺少挂耳价格表价格")
			}
			items[idx].tierID = nil
			items[idx].unitPrice = pricing.UnitPrice
			items[idx].quantityBasis = strings.TrimSpace(pricing.QuantityBasis)
			items[idx].baseLineTotal = pricing.UnitPrice * float64(items[idx].units)
			applyItemDiscount(idx)
			items[idx].matchedPriceQty = float64(items[idx].units)
			items[idx].priceListType = priceListType
			items[idx].itemBeanListPublicationID = usage.PublicationID
			items[idx].itemBeanListVersionNo = usage.VersionNo
			items[idx].priceSourceJSON = beanListPriceSourceJSONWithPricing(priceListType, usage, *items[idx].productID, pricing)
			totalAmt += items[idx].baseLineTotal
			itemDiscountAmt += items[idx].discountAmount
			continue
		} else if retailOrder && items[idx].productID != nil {
			requestedPublicationID := orderItemBeanListPublicationID(cmd, items[idx].itemBeanListPublicationID, orderbeans.ListTypeRetail)
			usage, err := orderbeans.ResolveUsageForPublication(ctx, tx, r.schema, cmd.CustomerID, *items[idx].productID, orderbeans.ListTypeRetail, requestedPublicationID)
			if err != nil {
				return salesapp.SaveOrderResult{}, err
			}
			if usage.PublicationID <= 0 {
				return salesapp.SaveOrderResult{}, fmt.Errorf("缺少商品价格表价格")
			}
			pricing, err := orderbeans.ResolvePublishedPricingForPublicationWithUnit(ctx, tx, r.schema, cmd.CustomerID, *items[idx].productID, orderbeans.ListTypeRetail, usage.PublicationID, items[idx].specG, items[idx].units, items[idx].salesUnit, items[idx].unitBagCount)
			if err != nil {
				return salesapp.SaveOrderResult{}, err
			}
			if pricing.UnitPrice <= 0 {
				return salesapp.SaveOrderResult{}, fmt.Errorf("缺少商品价格表价格")
			}
			items[idx].tierID = nil
			items[idx].unitPrice = pricing.UnitPrice
			items[idx].quantityBasis = strings.TrimSpace(pricing.QuantityBasis)
			items[idx].baseLineTotal = publishedPricingLineTotal(pricing, items[idx].specG, items[idx].units)
			applyItemDiscount(idx)
			items[idx].priceListType = orderbeans.ListTypeRetail
			items[idx].itemBeanListPublicationID = usage.PublicationID
			items[idx].itemBeanListVersionNo = usage.VersionNo
			items[idx].priceSourceJSON = beanListPriceSourceJSONWithPricing(orderbeans.ListTypeRetail, usage, *items[idx].productID, pricing)
			totalAmt += items[idx].baseLineTotal
			itemDiscountAmt += items[idx].discountAmount
			continue
		} else if items[idx].productID != nil && items[idx].productKind == "green_bean" {
			requestedPublicationID := orderItemBeanListPublicationID(cmd, items[idx].itemBeanListPublicationID, orderbeans.ListTypeGreen)
			usage, err := orderbeans.ResolveUsageForPublication(ctx, tx, r.schema, cmd.CustomerID, *items[idx].productID, orderbeans.ListTypeGreen, requestedPublicationID)
			if err != nil {
				return salesapp.SaveOrderResult{}, err
			}
			items[idx].tierID = nil
			if usage.PublicationID <= 0 {
				return salesapp.SaveOrderResult{}, fmt.Errorf("缺少生豆豆单价格")
			}
			pricing, err := orderbeans.ResolvePublishedPricingForPublicationWithUnit(ctx, tx, r.schema, cmd.CustomerID, *items[idx].productID, orderbeans.ListTypeGreen, usage.PublicationID, items[idx].specG, items[idx].units, items[idx].salesUnit, items[idx].unitBagCount)
			if err != nil {
				return salesapp.SaveOrderResult{}, err
			}
			if pricing.UnitPrice <= 0 {
				return salesapp.SaveOrderResult{}, fmt.Errorf("缺少生豆豆单价格")
			}
			items[idx].unitPrice = pricing.UnitPrice
			items[idx].quantityBasis = strings.TrimSpace(pricing.QuantityBasis)
			items[idx].priceListType = orderbeans.ListTypeGreen
			items[idx].itemBeanListPublicationID = usage.PublicationID
			items[idx].itemBeanListVersionNo = usage.VersionNo
			items[idx].priceSourceJSON = beanListPriceSourceJSONWithPricing(orderbeans.ListTypeGreen, usage, *items[idx].productID, pricing)
			if items[idx].baseLineTotal <= 0 || strings.TrimSpace(pricing.QuantityBasis) == "sales_spec_count" {
				items[idx].baseLineTotal = publishedPricingLineTotal(pricing, items[idx].specG, items[idx].units)
			}
			applyItemDiscount(idx)
			totalAmt += items[idx].baseLineTotal
			itemDiscountAmt += items[idx].discountAmount
			continue
		} else if items[idx].productID != nil {
			requestedPublicationID := orderItemBeanListPublicationID(cmd, items[idx].itemBeanListPublicationID, orderbeans.ListTypeCommercial)
			usage, err := orderbeans.ResolveUsageForPublication(ctx, tx, r.schema, cmd.CustomerID, *items[idx].productID, orderbeans.ListTypeCommercial, requestedPublicationID)
			if err != nil {
				return salesapp.SaveOrderResult{}, err
			}
			if usage.PublicationID <= 0 {
				return salesapp.SaveOrderResult{}, fmt.Errorf("缺少商品价格表价格")
			}
			pricing, err := orderbeans.ResolvePublishedPricingForPublicationWithUnit(ctx, tx, r.schema, cmd.CustomerID, *items[idx].productID, orderbeans.ListTypeCommercial, usage.PublicationID, items[idx].specG, items[idx].units, items[idx].salesUnit, items[idx].unitBagCount)
			if err != nil {
				return salesapp.SaveOrderResult{}, err
			}
			if pricing.UnitPrice <= 0 {
				return salesapp.SaveOrderResult{}, fmt.Errorf("缺少商品价格表价格")
			}
			items[idx].tierID = nil
			items[idx].unitPrice = pricing.UnitPrice
			items[idx].quantityBasis = strings.TrimSpace(pricing.QuantityBasis)
			items[idx].baseLineTotal = publishedPricingLineTotal(pricing, items[idx].specG, items[idx].units)
			applyItemDiscount(idx)
			items[idx].priceListType = orderbeans.ListTypeCommercial
			items[idx].itemBeanListPublicationID = usage.PublicationID
			items[idx].itemBeanListVersionNo = usage.VersionNo
			items[idx].priceSourceJSON = beanListPriceSourceJSONWithPricing(orderbeans.ListTypeCommercial, usage, *items[idx].productID, pricing)
			totalAmt += items[idx].baseLineTotal
			itemDiscountAmt += items[idx].discountAmount
			continue
		}

		if items[idx].baseLineTotal == 0 {
			items[idx].baseLineTotal = items[idx].lineTotal
		}
		if items[idx].baseLineTotal == 0 {
			items[idx].baseLineTotal = orderManualPriceLineTotal(items[idx].unitPrice, orderDiscountItem{
				productKind:   items[idx].productKind,
				salesUnit:     items[idx].salesUnit,
				quantityBasis: items[idx].quantityBasis,
				specG:         items[idx].specG,
				units:         items[idx].units,
			}, retailOrder)
		}
		applyItemDiscount(idx)
		totalAmt += items[idx].baseLineTotal
		itemDiscountAmt += items[idx].discountAmount
	}

	// Amount calculation (items + shipping - discount)
	shippingAmt := cmd.ShippingAmount
	discountAmt := cmd.DiscountAmount + itemDiscountAmt
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
	if err := validateOrderFulfillmentRequirementsTx(ctx, tx, r.schema, payStatusID, shipStatusID, cmd.LogisticsCompanyID, cmd.LogisticsProductID, cmd.PaymentGoodsAmount, cmd.PaymentShippingAmount, cmd.PaymentVoucherAssetID); err != nil {
		return salesapp.SaveOrderResult{}, err
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

	responsibleType, responsibleID, responsibleName, err := resolveOrderResponsibleParty(ctx, tx, r.schema, cmd.CustomerID)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	editID := cmd.EditID
	portalServiceCode, sourceWarehouse := r.orderFulfillmentMarkersTx(ctx, tx, cmd.CustomerID)
	if cmd.PortalServiceCode != "" {
		portalServiceCode = cmd.PortalServiceCode
	}
	headerPublicationID := selectedOrderBeanListPublicationID(cmd, orderbeans.ListTypeCommercial)
	beanListPublicationID, beanListVersionNo, err := r.resolveOrderBeanListPublicationTx(ctx, tx, cmd.CustomerID, headerPublicationID, orderbeans.ListTypeCommercial)
	if err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	insertItemSQL := fmt.Sprintf(`INSERT INTO %s.order_items(order_id,line_no,product_id,customer_product_alias_id,customer_product_display_name_snapshot,customer_item_code_snapshot,brand_name_snapshot,product_code_snapshot,product_name_snapshot,price_tier_id,price_overridden,product_kind,bean_list_publication_id,bean_list_version_no,item_name,item_note,qty,unit,spec,unit_price,line_total_before_discount,discount_type,discount_value,discount_amount,line_total,sales_unit,unit_bag_count,unit_bean_g,matched_price_qty,price_source_json)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,0),$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30::jsonb)`, r.schema)

	var orderID int64
	if editID > 0 {
		var existingPortalServiceCode, existingSourceWarehouse string
		if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT id, order_no, COALESCE(portal_service_code,''), COALESCE(source_warehouse,'') FROM %s.orders WHERE id=$1 FOR UPDATE", r.schema), editID).Scan(&orderID, &orderNo, &existingPortalServiceCode, &existingSourceWarehouse); err != nil {
			return salesapp.SaveOrderResult{}, fmt.Errorf("invalid edit_id")
		}
		portalServiceCode, sourceWarehouse = resolveOrderFulfillmentMarkers(existingPortalServiceCode, existingSourceWarehouse, portalServiceCode, sourceWarehouse)
		uq := fmt.Sprintf(`
				UPDATE %s.orders
				SET document_date=$2,
					order_date=$3,
					customer_id=$4,
					source_id=$5,
					order_type_id=$6,
					pay_status_id=$7,
					payment_method=$8,
					ship_status_id=$9,
					ship_method=$10,
					ship_tracking_no=$11,
					notes=$12,
					total_amount=$13,
					shipping_amount=$14,
					discount_amount=$15,
					round_to_int=$16,
					rounding_amount=$17,
					grand_total=$18,
					express_fee=$19,
					outsource_material_fee=$20,
					outsource_roast_fee=$21,
					outsource_packaging_fee=$22,
					outsource_manual_fee=$23,
					outsource_tax_fee=$24,
					outsource_other_fee=$25,
						outsource_total_fee=$26,
						responsible_party_type=$27,
						responsible_party_id=$28,
						responsible_party_name=$29,
						portal_service_code=$30,
						source_warehouse=$31,
						bean_list_publication_id=$32,
						bean_list_version_no=$33,
						logistics_company_id=$34,
						logistics_product_id=$35,
						payment_goods_amount=$36,
						payment_shipping_amount=$37,
						payment_voucher_asset_id=$38
					WHERE id=$1
			`, r.schema)
		if _, err := tx.Exec(ctx, uq,
			orderID,
			documentDate,
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
			portalServiceCode,
			sourceWarehouse,
			beanListPublicationID,
			beanListVersionNo,
			cmd.LogisticsCompanyID,
			cmd.LogisticsProductID,
			cmd.PaymentGoodsAmount,
			cmd.PaymentShippingAmount,
			cmd.PaymentVoucherAssetID,
		); err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.order_items WHERE order_id=$1", r.schema), orderID); err != nil {
			return salesapp.SaveOrderResult{}, err
		}
	} else {
		orderNo, err = nextOrderNo(ctx, tx, r.schema, documentDate)
		if err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		insertOrderSQL := fmt.Sprintf(`
				INSERT INTO %s.orders(
					document_date, order_date, customer_id,
					source_id, order_type_id, pay_status_id, payment_method, ship_status_id,
					ship_method, ship_tracking_no,
					notes,
					total_amount, shipping_amount, discount_amount,
					round_to_int, rounding_amount, grand_total,
					express_fee,
					outsource_material_fee, outsource_roast_fee, outsource_packaging_fee,
					outsource_manual_fee, outsource_tax_fee, outsource_other_fee, outsource_total_fee,
						responsible_party_type, responsible_party_id, responsible_party_name,
						portal_service_code, source_warehouse,
						receiver_name, receiver_phone, receiver_address, receiver_company,
						bean_list_publication_id, bean_list_version_no,
						order_no,
						logistics_company_id, logistics_product_id,
						payment_goods_amount, payment_shipping_amount, payment_voucher_asset_id
					) VALUES (
						$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
						$11,$12,$13,
						$14,$15,$16,
						$17,$18,$19,$20,$21,$22,$23,$24,$25,
						$26,$27,$28,
						$29,$30,
						$31,$32,$33,$34,$35,
						$36,$37,
						$38,
						$39,
						$40,$41,$42
					)
					RETURNING id
			`, r.schema)
		err = tx.QueryRow(ctx, insertOrderSQL,
			documentDate,
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
			portalServiceCode,
			sourceWarehouse,
			notNullText(cmd.ReceiverName),
			notNullText(cmd.ReceiverPhone),
			notNullText(cmd.ReceiverAddress),
			notNullText(cmd.ReceiverCompany),
			beanListPublicationID,
			beanListVersionNo,
			orderNo,
			cmd.LogisticsCompanyID,
			cmd.LogisticsProductID,
			cmd.PaymentGoodsAmount,
			cmd.PaymentShippingAmount,
			cmd.PaymentVoucherAssetID,
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
		priceSourceJSON := strings.TrimSpace(it.priceSourceJSON)
		if priceSourceJSON == "" {
			priceSourceJSON = "{}"
		}
		productID := int64(0)
		if it.productID != nil {
			productID = *it.productID
		}
		aliasSnapshot, err := resolveOrderItemCustomerAliasSnapshotTx(ctx, tx, r.schema, cmd.CustomerID, productID, it.customerProductAliasID)
		if err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		if aliasSnapshot.AliasID > 0 {
			it.customerProductAliasID = aliasSnapshot.AliasID
			it.customerProductDisplayNameSnapshot = aliasSnapshot.DisplayName
			it.customerItemCodeSnapshot = aliasSnapshot.CustomerItemCode
			it.brandNameSnapshot = aliasSnapshot.BrandName
		}
		if strings.TrimSpace(it.productCodeSnapshot) == "" {
			it.productCodeSnapshot = aliasSnapshot.ProductCode
		}
		if strings.TrimSpace(it.productNameSnapshot) == "" {
			it.productNameSnapshot = aliasSnapshot.ProductName
		}
		if strings.TrimSpace(it.customerProductDisplayNameSnapshot) == "" && aliasSnapshot.AliasID > 0 {
			it.customerProductDisplayNameSnapshot = aliasSnapshot.DisplayName
		}
		itemListType := strings.TrimSpace(it.priceListType)
		if itemListType == "" {
			itemListType = orderbeans.ListTypeForProductKind(it.productKind, retailOrder)
		}
		itemPublicationID := orderItemBeanListPublicationID(cmd, it.itemBeanListPublicationID, itemListType)
		if !it.priceOverride && productID > 0 && it.unitPrice <= 0 && it.specG > 0 && it.units > 0 {
			publishedPricing, err := orderbeans.ResolvePublishedPricingForPublicationWithUnit(ctx, tx, r.schema, cmd.CustomerID, productID, itemListType, itemPublicationID, int64(it.specG), int64(it.units), it.salesUnit, it.unitBagCount)
			if err != nil {
				return salesapp.SaveOrderResult{}, err
			}
			if publishedPricing.UnitPrice > 0 {
				it.unitPrice = publishedPricing.UnitPrice
				it.quantityBasis = strings.TrimSpace(publishedPricing.QuantityBasis)
				if it.productKind == "drip_bag" {
					it.baseLineTotal = publishedPricing.UnitPrice * float64(it.units)
				} else {
					it.baseLineTotal = publishedPricingLineTotal(publishedPricing, it.specG, it.units)
				}
				if it.lineTotal <= 0 {
					it.lineTotal = it.baseLineTotal
				}
			}
		}
		usage, err := orderbeans.ResolveUsageForPublication(ctx, tx, r.schema, cmd.CustomerID, productID, itemListType, itemPublicationID)
		if err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		if !it.priceOverride && productID > 0 && usage.PublicationID > 0 && strings.TrimSpace(it.priceSourceJSON) == "" {
			priceSourceJSON = beanListPriceSourceJSON(itemListType, usage, productID)
		}
		if it.itemBeanListVersionNo != "" && usage.VersionNo == "" {
			usage.VersionNo = it.itemBeanListVersionNo
		}
		if err := validateCustomerAliasPublishedPrice(it.customerProductAliasID, it.priceOverride, it.unitPrice); err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		productionConfigJSON, err := loadProductProductionConfigSummaryForOrderItemTx(ctx, tx, r.schema, productID)
		if err != nil {
			return salesapp.SaveOrderResult{}, err
		}
		priceSourceJSON = withProductProductionConfigPriceSourceJSON(priceSourceJSON, productionConfigJSON)
		if _, err := tx.Exec(ctx, insertItemSQL, orderID, idx+1, it.productID, it.customerProductAliasID, it.customerProductDisplayNameSnapshot, it.customerItemCodeSnapshot, it.brandNameSnapshot, it.productCodeSnapshot, it.productNameSnapshot, it.tierID, it.priceOverride, it.productKind, usage.PublicationID, usage.VersionNo, it.name, it.note, qtyAny, it.unit, it.spec, it.unitPrice, it.baseLineTotal, it.discountType, it.discountValue, it.discountAmount, it.lineTotal, it.salesUnit, it.unitBagCount, it.unitBeanG, it.matchedPriceQty, priceSourceJSON); err != nil {
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
			NeedG:       orderItemWeightG(it.productKind, it.salesUnit, it.unitBeanG, it.unitBagCount, it.specG, it.units),
		})
	}
	stockDecision := strings.TrimSpace(cmd.StockBatchDecision)
	if err := r.applyOrderStockDecisionTx(ctx, tx, orderID, stockItems, stockDecision, cmd.Actor); err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return salesapp.SaveOrderResult{}, err
	}

	r.logOrderSave(ctx, cmd.Actor, orderID, orderNo, editID > 0, orderSaveDripAuditSummary(), beanListPublicationID, beanListVersionNo)

	return salesapp.SaveOrderResult{OrderID: orderID, OrderNo: orderNo, Edited: editID > 0, StockBatchUsed: stockDecision == "use_batch"}, nil

}

func loadProductProductionConfigSummaryForOrderItemTx(ctx context.Context, tx pgx.Tx, schema string, productID int64) (string, error) {
	if productID <= 0 {
		return "{}", nil
	}
	var raw string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		WITH config AS (
			SELECT ppc.product_id,
				       COALESCE(ppc.production_bom_id,0) AS production_bom_id,
				       COALESCE(ppc.production_bom_version_id,0) AS production_bom_version_id,
				       COALESCE(ppc.process_route_id,0) AS process_route_id,
				       COALESCE(ppc.industry_field_template_id,0) AS industry_field_template_id,
				       COALESCE(ppc.expected_loss_rate,0)::float8 AS expected_loss_rate
			FROM %[1]s.product_production_configs ppc
			WHERE ppc.product_id=$1
		),
		fields AS (
			SELECT ppcf.product_id,
			       jsonb_agg(jsonb_build_object(
			         'field_key', ppcf.field_key,
			         'label', ppcf.label,
			         'field_type', ppcf.field_type,
			         'unit', ppcf.unit,
			         'value_text', ppcf.value_text,
				         'value_number', ppcf.value_number,
				         'value_bool', ppcf.value_bool,
				         'template_field_key', COALESCE(ppcf.template_field_key,''),
				         'required', COALESCE(ppcf.required,false),
				         'options_json', COALESCE(ppcf.options_json, '[]'::jsonb),
				         'show_in_price_list', ppcf.show_in_price_list,
			         'sort_order', ppcf.sort_order
			       ) ORDER BY ppcf.sort_order, ppcf.id) AS fields_json
			FROM %[1]s.product_production_config_fields ppcf
			WHERE ppcf.product_id=$1
			GROUP BY ppcf.product_id
		)
		SELECT COALESCE(jsonb_build_object(
			'product_id', c.product_id,
			'production_bom_id', c.production_bom_id,
				'production_bom_version_id', c.production_bom_version_id,
				'process_route_id', c.process_route_id,
				'industry_field_template_id', c.industry_field_template_id,
				'expected_loss_rate', c.expected_loss_rate,
			'fields', COALESCE(f.fields_json, '[]'::jsonb)
		), '{}'::jsonb)::text
		FROM config c
		LEFT JOIN fields f ON f.product_id=c.product_id
	`, schema), productID).Scan(&raw)
	if err == pgx.ErrNoRows {
		return "{}", nil
	}
	if err != nil {
		if strings.Contains(err.Error(), "product_production_configs") {
			return "{}", nil
		}
		return "", err
	}
	return raw, nil
}

func withProductProductionConfigPriceSourceJSON(existing string, configJSON string) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(existing)), &obj); err != nil || obj == nil {
		obj = map[string]any{}
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(configJSON)), &config); err == nil && len(config) > 0 {
		obj["production_config"] = config
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return existing
	}
	return string(b)
}

func (r Repository) resolveOrderBeanListPublicationTx(ctx context.Context, tx pgx.Tx, customerID, requestedID int64, listType string) (int64, string, error) {
	if !relationExistsTx(ctx, tx, fmt.Sprintf("%s.bean_list_publications", r.schema)) {
		return 0, "", nil
	}
	customerKey := fmt.Sprintf("%d", customerID)
	listType = strings.TrimSpace(listType)
	if listType == "" {
		listType = orderbeans.ListTypeCommercial
	}
	var err error
	if requestedID > 0 {
		var version string
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT version_no
			FROM %s.bean_list_publications
			WHERE id=$1 AND status='published'
			  AND publication_purpose='factory_supply'
			  AND list_type=$2
			  AND ((owner_type='customer' AND owner_key=$3) OR owner_type='official')
		`, r.schema), requestedID, listType, customerKey).Scan(&version)
		if err != nil {
			return 0, "", fmt.Errorf("invalid bean list publication")
		}
		return requestedID, version, nil
	}

	var fixedID int64
	if relationExistsTx(ctx, tx, fmt.Sprintf("%s.customer_portal_profiles", r.schema)) {
		err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(p.bean_list_publication_id,0)
				FROM %s.customer_portal_profiles p
				JOIN %s.bean_list_publications b ON b.id=p.bean_list_publication_id
				WHERE p.customer_id=$1 AND p.bean_list_mode='fixed'
				  AND b.owner_type='customer' AND b.owner_key=$2 AND b.status='published'
				  AND b.publication_purpose='factory_supply'
				  AND b.list_type=$3
			`, r.schema, r.schema), customerID, customerKey, listType).Scan(&fixedID)
		if err == nil && fixedID > 0 {
			requestedID = fixedID
		}
	}
	if requestedID <= 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT id
				FROM %s.bean_list_publications
				WHERE owner_type='customer' AND owner_key=$1 AND status='published'
				  AND publication_purpose='factory_supply' AND list_type=$2
				ORDER BY published_at DESC, id DESC
				LIMIT 1
			`, r.schema), customerKey, listType).Scan(&requestedID)
		if err != nil {
			_ = tx.QueryRow(ctx, fmt.Sprintf(`
					SELECT id
					FROM %s.bean_list_publications
					WHERE owner_type='official' AND status='published'
					  AND publication_purpose='factory_supply' AND list_type=$1
					ORDER BY published_at DESC, id DESC
					LIMIT 1
				`, r.schema), listType).Scan(&requestedID)
		}
	}
	if requestedID <= 0 {
		return 0, "", nil
	}
	var version string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT version_no FROM %s.bean_list_publications WHERE id=$1`, r.schema), requestedID).Scan(&version); err != nil {
		return 0, "", nil
	}
	return requestedID, version, nil
}

type requiredOrderCustomerProfile struct {
	customerType string
	sourceID     int64
	orderTypeID  int64
}

func (r Repository) requiredOrderCustomerProfileTx(ctx context.Context, tx pgx.Tx, customerID int64) (requiredOrderCustomerProfile, error) {
	var profile requiredOrderCustomerProfile
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(customer_type,''), COALESCE(default_source_id,0), COALESCE(default_order_type_id,0)
		FROM %s.customers
		WHERE id=$1 AND active=true
	`, r.schema), customerID).Scan(&profile.customerType, &profile.sourceID, &profile.orderTypeID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return requiredOrderCustomerProfile{}, fmt.Errorf("customer required")
		}
		return requiredOrderCustomerProfile{}, err
	}
	missing := make([]string, 0, 3)
	switch strings.TrimSpace(profile.customerType) {
	case "retail", "ecommerce", "wholesale", "channel":
	default:
		missing = append(missing, "客户类型")
	}
	if profile.sourceID <= 0 {
		missing = append(missing, "来源")
	}
	if profile.orderTypeID <= 0 {
		missing = append(missing, "订单类型")
	}
	if len(missing) > 0 {
		return requiredOrderCustomerProfile{}, fmt.Errorf("客户资料缺少%s，请先在客户资料维护", strings.Join(missing, "、"))
	}
	return profile, nil
}

func relationExistsTx(ctx context.Context, tx pgx.Tx, relation string) bool {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, relation).Scan(&exists); err != nil {
		return false
	}
	return exists
}

type orderItemCustomerAliasSnapshot struct {
	AliasID             int64
	DisplayName         string
	CustomerItemCode    string
	BrandName           string
	ProductCode         string
	ProductName         string
	DisplayCategoryID   int64
	DisplayCategoryName string
}

func resolveOrderItemCustomerAliasSnapshotTx(ctx context.Context, tx pgx.Tx, schema string, customerID, productID, requestedAliasID int64) (orderItemCustomerAliasSnapshot, error) {
	snap := orderItemCustomerAliasSnapshot{ProductCode: fmt.Sprintf("SKU-%d", productID)}
	if productID <= 0 {
		return orderItemCustomerAliasSnapshot{}, nil
	}
	effectiveParentID := productID
	_ = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(p.sku_code,''),'SKU-' || p.id::text),
		       CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END,
		       COALESCE(NULLIF(parent.name,''),p.name,'')
		FROM %[1]s.products p
		LEFT JOIN %[1]s.products parent ON parent.id=p.parent_product_id
		WHERE p.id=$1
	`, schema), productID).Scan(&snap.ProductCode, &effectiveParentID, &snap.ProductName)
	if !relationExistsTx(ctx, tx, fmt.Sprintf("%s.customer_product_aliases", schema)) {
		if requestedAliasID > 0 {
			return orderItemCustomerAliasSnapshot{}, fmt.Errorf("客户商品别名与当前客户或父商品不匹配，或已停用")
		}
		return snap, nil
	}

	q := fmt.Sprintf(`
		SELECT a.id,
		       COALESCE(NULLIF(a.display_name,''), effective_parent.name, alias_product.name, ''),
		       COALESCE(a.customer_item_code,''),
		       COALESCE(a.brand_name,''),
		       COALESCE(effective_parent.name, alias_product.name, ''),
		       COALESCE(a.display_category_id,0),
		       ''
		FROM %[1]s.customer_product_aliases a
		JOIN %[1]s.products alias_product ON alias_product.id=a.product_id
		LEFT JOIN %[1]s.products effective_parent ON effective_parent.id=CASE WHEN COALESCE(alias_product.parent_product_id,0)>0 THEN alias_product.parent_product_id ELSE alias_product.id END
		WHERE a.customer_id=$1
		  AND CASE WHEN COALESCE(alias_product.parent_product_id,0)>0 THEN alias_product.parent_product_id ELSE alias_product.id END=$2
		  AND a.active=true
	`, schema)
	args := []any{customerID, effectiveParentID}
	if requestedAliasID > 0 {
		q += " AND a.id=$3"
		args = append(args, requestedAliasID)
	}
	q += " ORDER BY a.include_in_price_list DESC, a.sort_order, a.id LIMIT 1"
	err := tx.QueryRow(ctx, q, args...).Scan(
		&snap.AliasID,
		&snap.DisplayName,
		&snap.CustomerItemCode,
		&snap.BrandName,
		&snap.ProductName,
		&snap.DisplayCategoryID,
		&snap.DisplayCategoryName,
	)
	if err == pgx.ErrNoRows {
		if requestedAliasID > 0 {
			return orderItemCustomerAliasSnapshot{}, fmt.Errorf("客户商品别名与当前客户或父商品不匹配，或已停用")
		}
		return snap, nil
	}
	if err != nil {
		return orderItemCustomerAliasSnapshot{}, err
	}
	return snap, nil
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

func productKindForOrderItem(ctx context.Context, tx pgx.Tx, schema string, productID int64) string {
	if productID <= 0 {
		return "roasted"
	}
	var kind string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(product_kind,''),'roasted')
		FROM %s.products
		WHERE id=$1
	`, schema), productID).Scan(&kind); err != nil {
		return "roasted"
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "roasted"
	}
	return kind
}

func (r Repository) Void(ctx context.Context, id int64, actor, reason string) error {
	_, err := r.VoidMany(ctx, []int64{id}, actor, reason)
	return err
}

func (r Repository) VoidMany(ctx context.Context, ids []int64, actor, reason string) (int, error) {
	q := fmt.Sprintf("UPDATE %s.orders SET is_void=true, voided_at=now(), void_reason=$2 WHERE id = ANY($1) AND COALESCE(is_void,false)=false RETURNING id", r.schema)
	rows, err := r.pool.Query(ctx, q, ids, nullText(reason))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	updatedIDs := make([]int64, 0, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		updatedIDs = append(updatedIDs, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	var rv *string
	if strings.TrimSpace(reason) != "" {
		rv = &reason
	}
	for _, id := range updatedIDs {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "order", &id, "void", nil, nil, rv, postgresinfra.AuditMeta{"order_id": id})
	}
	return len(updatedIDs), nil
}

func (r Repository) logOrderSave(ctx context.Context, actor string, orderID int64, orderNo string, edited bool, extraNewValue string, beanListPublicationID int64, beanListVersionNo string) {
	action := "create"
	field := "created"
	newValue := orderNo
	if edited {
		action = "update"
		field = "order"
		newValue = "updated"
	}
	if strings.TrimSpace(extraNewValue) != "" {
		newValue = strings.TrimSpace(newValue + " " + extraNewValue)
	}
	r.insertOrderAudit(ctx, actor, orderID, field, nil, postgresinfra.StrPtr(newValue))
	meta := postgresinfra.AuditMeta{"order_id": orderID, "order_no": orderNo}
	if beanListPublicationID > 0 {
		meta["bean_list_publication_id"] = beanListPublicationID
		meta["bean_list_version_no"] = beanListVersionNo
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "order", &orderID, action, postgresinfra.StrPtr(field), nil, postgresinfra.StrPtr(newValue), meta)
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
	documentDate := strings.TrimSpace(req.DocumentDate)
	if documentDate == "" {
		documentDate = orderDate
	}
	if _, err := time.Parse("2006-01-02", documentDate); err != nil {
		return fmt.Errorf("invalid document_date")
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
	paymentGoodsAmount, err := parseFee(req.PaymentGoodsAmount)
	if err != nil {
		return fmt.Errorf("invalid payment_goods_amount")
	}
	paymentShippingAmount, err := parseFee(req.PaymentShippingAmount)
	if err != nil {
		return fmt.Errorf("invalid payment_shipping_amount")
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
	if err := validateOrderFulfillmentRequirementsTx(ctx, tx, schema, req.PayStatusID, req.ShipStatusID, req.LogisticsCompanyID, req.LogisticsProductID, paymentGoodsAmount, paymentShippingAmount, req.PaymentVoucherAssetID); err != nil {
		return err
	}
	shipTrackingNo := salesapp.TrackingNumbersSummary(salesapp.NormalizeTrackingNumbers(req.ShipTrackingNo))
	q := fmt.Sprintf(`
		UPDATE %s.orders
		SET document_date=$2,
			order_date=$3,
			customer_id=$4,
			source_id=$5,
			order_type_id=$6,
			pay_status_id=$7,
			payment_method=$8,
			ship_status_id=$9,
			notes=$10,
			total_amount=$11,
			shipping_amount=$12,
			discount_amount=$13,
			round_to_int=$14,
			rounding_amount=$15,
			grand_total=$16,
			express_fee=$17,
			outsource_material_fee=$18,
			outsource_roast_fee=$19,
			outsource_packaging_fee=$20,
			outsource_manual_fee=$21,
			outsource_tax_fee=$22,
			outsource_other_fee=$23,
			outsource_total_fee=$24,
			ship_method=$25,
			ship_tracking_no=$26,
			logistics_company_id=$27,
			logistics_product_id=$28,
			payment_goods_amount=$29,
			payment_shipping_amount=$30,
			payment_voucher_asset_id=$31
		WHERE id=$1
	`, schema)
	if _, err := tx.Exec(ctx, q,
		id,
		documentDate,
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
		nullText(req.ShipMethod),
		nullText(shipTrackingNo),
		req.LogisticsCompanyID,
		req.LogisticsProductID,
		paymentGoodsAmount,
		paymentShippingAmount,
		req.PaymentVoucherAssetID,
	); err != nil {
		return err
	}
	if _, err := replaceOrderTrackingNumbersTx(ctx, tx, schema, id, shipTrackingNo, "order_header", req.Actor); err != nil {
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

func notNullText(v string) string {
	return strings.TrimSpace(v)
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
