package orderliststaging

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func WriteExports(dataset Dataset, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outputDir, "dataset.json"), dataset); err != nil {
		return err
	}
	manifest := struct {
		Run      ImportRun      `json:"run"`
		Counts   map[string]int `json:"counts"`
		Statuses map[string]int `json:"review_statuses"`
	}{
		Run: dataset.Run,
		Counts: map[string]int{
			"sheets": len(dataset.Sheets), "raw_orders": len(dataset.RawOrders), "customers": len(dataset.Customers),
			"products": len(dataset.Products), "skus": len(dataset.SKUs), "orders": len(dataset.Orders),
			"order_items": len(dataset.OrderItems), "issues": len(dataset.Issues),
		},
		Statuses: reviewStatusCounts(dataset),
	}
	if err := writeJSON(filepath.Join(outputDir, "manifest.json"), manifest); err != nil {
		return err
	}
	mappings := map[string]SourceKeyAssignment{}
	for _, row := range dataset.RawOrders {
		if row.SourceOrderKey == "" {
			continue
		}
		mappings[row.Fingerprint] = SourceKeyAssignment{
			SheetName: row.SheetName, OriginalSequence: row.SequenceOriginal,
			EffectiveSequence: row.SequenceEffective, DuplicateSuffix: row.DuplicateSuffix,
		}
	}
	if err := writeJSON(filepath.Join(outputDir, "source-key-mapping.json"), mappings); err != nil {
		return err
	}
	if err := writeProtected(filepath.Join(outputDir, "schema.sql"), []byte(StagingSchemaSQL())); err != nil {
		return err
	}
	loadSQL, err := GenerateLoadSQL(dataset)
	if err != nil {
		return err
	}
	if err := writeProtected(filepath.Join(outputDir, "load.sql"), []byte(loadSQL)); err != nil {
		return err
	}

	csvFiles := map[string][][]string{
		"sheet_inventory.csv":  sheetInventoryCSV(dataset.Sheets),
		"raw_orders.csv":       rawOrdersCSV(dataset.RawOrders),
		"customers.csv":        customersCSV(dataset.Customers),
		"customer_aliases.csv": customerAliasesCSV(dataset.CustomerAliases),
		"customer_phones.csv":  customerPhonesCSV(dataset.CustomerPhones),
		"products.csv":         productsCSV(dataset.Products),
		"skus.csv":             skusCSV(dataset.SKUs),
		"product_aliases.csv":  productAliasesCSV(dataset.ProductAliases),
		"orders.csv":           ordersCSV(dataset.Orders),
		"order_items.csv":      orderItemsCSV(dataset.OrderItems),
		"issues.csv":           issuesCSV(dataset.Issues),
	}
	for name, rows := range csvFiles {
		if err := writeCSV(filepath.Join(outputDir, name), rows); err != nil {
			return err
		}
	}
	return nil
}

func GenerateLoadSQL(dataset Dataset) (string, error) {
	var b strings.Builder
	b.Grow(1024 * 1024)
	b.WriteString("BEGIN;\nSET LOCAL client_encoding = 'UTF8';\n")
	run := dataset.Run
	fmt.Fprintf(&b, `INSERT INTO raw.import_runs(run_id,source_path,source_sha256,source_bytes,start_period,end_period,created_at,workbook_sheet_count,included_sheet_count,raw_order_count,raw_product_lines)
VALUES (%s,%s,%s,%d,%s,%s,%s,%d,%d,%d,%d)
ON CONFLICT (run_id) DO UPDATE SET source_path=EXCLUDED.source_path,source_sha256=EXCLUDED.source_sha256,source_bytes=EXCLUDED.source_bytes,start_period=EXCLUDED.start_period,end_period=EXCLUDED.end_period,created_at=EXCLUDED.created_at,workbook_sheet_count=EXCLUDED.workbook_sheet_count,included_sheet_count=EXCLUDED.included_sheet_count,raw_order_count=EXCLUDED.raw_order_count,raw_product_lines=EXCLUDED.raw_product_lines,loaded_at=now();
`, sqlText(run.RunID), sqlText(run.SourcePath), sqlText(run.SourceSHA256), run.SourceBytes, sqlText(run.StartPeriod), sqlText(run.EndPeriod), sqlTime(run.CreatedAt), run.WorkbookSheetCount, run.IncludedSheetCount, run.RawOrderCount, run.RawProductLines)

	for _, sheet := range dataset.Sheets {
		fmt.Fprintf(&b, `INSERT INTO raw.sheet_inventory(run_id,sheet_name,period,included,excluded_reason,used_row_count,order_row_count) VALUES (%s,%s,%s,%t,%s,%d,%d) ON CONFLICT (run_id,sheet_name) DO UPDATE SET period=EXCLUDED.period,included=EXCLUDED.included,excluded_reason=EXCLUDED.excluded_reason,used_row_count=EXCLUDED.used_row_count,order_row_count=EXCLUDED.order_row_count;
`, sqlText(run.RunID), sqlText(sheet.SheetName), sqlText(sheet.Period), sheet.Included, sqlText(sheet.ExcludedReason), sheet.UsedRowCount, sheet.OrderRowCount)
	}
	for _, row := range dataset.RawOrders {
		rawJSON, err := json.Marshal(row.RawFields)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, `INSERT INTO raw.order_rows(run_id,source_sheet_name,source_row_number,source_sequence_original,duplicate_suffix,source_sequence_effective,source_order_key,source_fingerprint,order_date_raw,customer_raw,product_raw,raw_fields,review_status) VALUES (%s,%s,%d,%s,%d,%s,%s,%s,%s,%s,%s,%s::jsonb,%s) ON CONFLICT (run_id,source_sheet_name,source_row_number) DO UPDATE SET source_sequence_original=EXCLUDED.source_sequence_original,duplicate_suffix=EXCLUDED.duplicate_suffix,source_sequence_effective=EXCLUDED.source_sequence_effective,source_order_key=EXCLUDED.source_order_key,source_fingerprint=EXCLUDED.source_fingerprint,order_date_raw=EXCLUDED.order_date_raw,customer_raw=EXCLUDED.customer_raw,product_raw=EXCLUDED.product_raw,raw_fields=EXCLUDED.raw_fields,review_status=EXCLUDED.review_status;
`, sqlText(run.RunID), sqlText(row.SheetName), row.SourceRowNumber, sqlText(row.SequenceOriginal), row.DuplicateSuffix, sqlText(row.SequenceEffective), sqlText(row.SourceOrderKey), sqlText(row.Fingerprint), sqlText(row.OrderDateRaw), sqlText(row.CustomerRaw), sqlText(row.ProductRaw), sqlText(string(rawJSON)), sqlText(row.ReviewStatus))
	}

	for _, ref := range dataset.ERPCustomers {
		fmt.Fprintf(&b, `INSERT INTO reference.erp_customers(environment,erp_customer_id,name,phone,active,snapshot_at) VALUES ('development',%d,%s,%s,%t,%s) ON CONFLICT (environment,erp_customer_id) DO UPDATE SET name=EXCLUDED.name,phone=EXCLUDED.phone,active=EXCLUDED.active,snapshot_at=EXCLUDED.snapshot_at;
`, ref.ID, sqlText(ref.Name), sqlText(ref.Phone), ref.Active, sqlTime(run.CreatedAt))
	}
	for _, ref := range dataset.ERPProducts {
		fmt.Fprintf(&b, `INSERT INTO reference.erp_products(environment,erp_product_id,name,product_kind,active,snapshot_at) VALUES ('development',%d,%s,%s,%t,%s) ON CONFLICT (environment,erp_product_id) DO UPDATE SET name=EXCLUDED.name,product_kind=EXCLUDED.product_kind,active=EXCLUDED.active,snapshot_at=EXCLUDED.snapshot_at;
`, ref.ID, sqlText(ref.Name), sqlText(ref.ProductKind), ref.Active, sqlTime(run.CreatedAt))
	}

	for _, customer := range dataset.Customers {
		fmt.Fprintf(&b, `INSERT INTO curated.customers(customer_key,canonical_name,normalized_phone,current_contact,current_address,erp_match_id,erp_match_name,match_method,review_status) VALUES (%s,%s,%s,%s,%s,%d,%s,%s,%s) ON CONFLICT (customer_key) DO UPDATE SET canonical_name=EXCLUDED.canonical_name,normalized_phone=EXCLUDED.normalized_phone,current_contact=EXCLUDED.current_contact,current_address=EXCLUDED.current_address,erp_match_id=EXCLUDED.erp_match_id,erp_match_name=EXCLUDED.erp_match_name,match_method=EXCLUDED.match_method,review_status=EXCLUDED.review_status,updated_at=now();
`, sqlText(customer.CustomerKey), sqlText(customer.CanonicalName), sqlText(customer.NormalizedPhone), sqlText(customer.CurrentContact), sqlText(customer.CurrentAddress), customer.ERPMatchID, sqlText(customer.ERPMatchName), sqlText(customer.MatchMethod), sqlText(customer.ReviewStatus))
	}
	for _, alias := range dataset.CustomerAliases {
		fmt.Fprintf(&b, `INSERT INTO curated.customer_aliases(customer_key,alias,alias_normalized,source_order_key,observed_date) VALUES (%s,%s,%s,%s,%s) ON CONFLICT (customer_key,alias_normalized,source_order_key) DO UPDATE SET alias=EXCLUDED.alias,observed_date=EXCLUDED.observed_date;
`, sqlText(alias.CustomerKey), sqlText(alias.Alias), sqlText(alias.AliasNormalized), sqlText(alias.SourceOrderKey), sqlText(alias.ObservedDate))
	}
	for _, phone := range dataset.CustomerPhones {
		fmt.Fprintf(&b, `INSERT INTO curated.customer_phones(customer_key,phone_raw,phone_normalized,is_primary,source_order_key) VALUES (%s,%s,%s,%t,%s) ON CONFLICT (customer_key,phone_normalized) DO UPDATE SET phone_raw=EXCLUDED.phone_raw,is_primary=EXCLUDED.is_primary,source_order_key=EXCLUDED.source_order_key;
`, sqlText(phone.CustomerKey), sqlText(phone.PhoneRaw), sqlText(phone.PhoneNormalized), phone.IsPrimary, sqlText(phone.SourceOrderKey))
	}
	for _, product := range dataset.Products {
		fmt.Fprintf(&b, `INSERT INTO curated.products(product_key,canonical_name,product_kind,roast_level,erp_match_id,erp_match_name,match_method,match_score,review_status) VALUES (%s,%s,%s,%s,%d,%s,%s,%s,%s) ON CONFLICT (product_key) DO UPDATE SET canonical_name=EXCLUDED.canonical_name,product_kind=EXCLUDED.product_kind,roast_level=EXCLUDED.roast_level,erp_match_id=EXCLUDED.erp_match_id,erp_match_name=EXCLUDED.erp_match_name,match_method=EXCLUDED.match_method,match_score=EXCLUDED.match_score,review_status=EXCLUDED.review_status,updated_at=now();
`, sqlText(product.ProductKey), sqlText(product.CanonicalName), sqlText(product.ProductKind), sqlText(product.RoastLevel), product.ERPMatchID, sqlText(product.ERPMatchName), sqlText(product.MatchMethod), sqlFloat(product.MatchScore), sqlText(product.ReviewStatus))
	}
	for _, sku := range dataset.SKUs {
		fmt.Fprintf(&b, `INSERT INTO curated.skus(sku_key,product_key,spec_name,sales_unit,net_content_qty,net_content_unit,normalized_weight_g,review_status) VALUES (%s,%s,%s,%s,%s,%s,%s,%s) ON CONFLICT (sku_key) DO UPDATE SET product_key=EXCLUDED.product_key,spec_name=EXCLUDED.spec_name,sales_unit=EXCLUDED.sales_unit,net_content_qty=EXCLUDED.net_content_qty,net_content_unit=EXCLUDED.net_content_unit,normalized_weight_g=EXCLUDED.normalized_weight_g,review_status=EXCLUDED.review_status;
`, sqlText(sku.SKUKey), sqlText(sku.ProductKey), sqlText(sku.SpecName), sqlText(sku.SalesUnit), sqlFloat(sku.NetContentQty), sqlText(sku.NetContentUnit), sqlFloat(sku.NormalizedWeightG), sqlText(sku.ReviewStatus))
	}
	for _, alias := range dataset.ProductAliases {
		fmt.Fprintf(&b, `INSERT INTO curated.product_aliases(product_key,sku_key,raw_line,normalized_line,source_order_key,match_method,match_score) VALUES (%s,%s,%s,%s,%s,%s,%s) ON CONFLICT (product_key,sku_key,normalized_line,source_order_key) DO UPDATE SET raw_line=EXCLUDED.raw_line,match_method=EXCLUDED.match_method,match_score=EXCLUDED.match_score;
`, sqlText(alias.ProductKey), sqlText(alias.SKUKey), sqlText(alias.RawLine), sqlText(alias.NormalizedLine), sqlText(alias.SourceOrderKey), sqlText(alias.MatchMethod), sqlFloat(alias.MatchScore))
	}

	if len(dataset.Orders) > 0 {
		keys := make([]string, 0, len(dataset.Orders))
		for _, order := range dataset.Orders {
			keys = append(keys, sqlText(order.SourceOrderKey))
		}
		fmt.Fprintf(&b, "DELETE FROM curated.order_items WHERE source_order_key IN (%s);\n", strings.Join(keys, ","))
	}
	for _, order := range dataset.Orders {
		newSnapshot, err := json.Marshal(order)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, `INSERT INTO raw.order_revisions(source_order_key,old_fingerprint,new_fingerprint,old_snapshot,new_snapshot,detected_run_id) SELECT existing.source_order_key,existing.source_fingerprint,%s,to_jsonb(existing),%s::jsonb,%s FROM curated.orders existing WHERE existing.source_order_key=%s AND existing.source_fingerprint<>%s ON CONFLICT (source_order_key,old_fingerprint,new_fingerprint) DO NOTHING;
`, sqlText(order.SourceFingerprint), sqlText(string(newSnapshot)), sqlText(run.RunID), sqlText(order.SourceOrderKey), sqlText(order.SourceFingerprint))
		fmt.Fprintf(&b, `INSERT INTO curated.orders(source_order_key,sheet_name,sequence_original,sequence_effective,source_row_number,source_fingerprint,order_date,customer_key,customer_raw,order_source_raw,order_type_raw,payment_status_raw,shipment_status_raw,amount_value,amount_raw,amount_derived,shipping_amount_value,shipping_amount_raw,tracking_no_raw,remark_raw,review_status) VALUES (%s,%s,%s,%s,%d,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%t,%s,%s,%s,%s,%s) ON CONFLICT (source_order_key) DO UPDATE SET sheet_name=EXCLUDED.sheet_name,sequence_original=EXCLUDED.sequence_original,sequence_effective=EXCLUDED.sequence_effective,source_row_number=EXCLUDED.source_row_number,source_fingerprint=EXCLUDED.source_fingerprint,order_date=EXCLUDED.order_date,customer_key=EXCLUDED.customer_key,customer_raw=EXCLUDED.customer_raw,order_source_raw=EXCLUDED.order_source_raw,order_type_raw=EXCLUDED.order_type_raw,payment_status_raw=EXCLUDED.payment_status_raw,shipment_status_raw=EXCLUDED.shipment_status_raw,amount_value=EXCLUDED.amount_value,amount_raw=EXCLUDED.amount_raw,amount_derived=EXCLUDED.amount_derived,shipping_amount_value=EXCLUDED.shipping_amount_value,shipping_amount_raw=EXCLUDED.shipping_amount_raw,tracking_no_raw=EXCLUDED.tracking_no_raw,remark_raw=EXCLUDED.remark_raw,review_status=EXCLUDED.review_status,updated_at=now();
`, sqlText(order.SourceOrderKey), sqlText(order.SheetName), sqlText(order.SequenceOriginal), sqlText(order.SequenceEffective), order.SourceRowNumber, sqlText(order.SourceFingerprint), sqlNullableDate(order.OrderDate), sqlText(order.CustomerKey), sqlText(order.CustomerRaw), sqlText(order.OrderSourceRaw), sqlText(order.OrderTypeRaw), sqlText(order.PaymentStatusRaw), sqlText(order.ShipmentStatusRaw), sqlNullableFloat(order.AmountValue), sqlText(order.AmountRaw), order.AmountDerived, sqlNullableFloat(order.ShippingAmountValue), sqlText(order.ShippingAmountRaw), sqlText(order.TrackingNoRaw), sqlText(order.RemarkRaw), sqlText(order.ReviewStatus))
	}
	for _, item := range dataset.OrderItems {
		if item.SourceOrderKey == "" {
			continue
		}
		fmt.Fprintf(&b, `INSERT INTO curated.order_items(source_item_key,source_order_key,line_no,raw_line,product_key,sku_key,parent_name,spec_name,product_kind,roast_level,order_quantity,order_unit,normalized_weight_g,review_status) VALUES (%s,%s,%d,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s) ON CONFLICT (source_item_key) DO UPDATE SET source_order_key=EXCLUDED.source_order_key,line_no=EXCLUDED.line_no,raw_line=EXCLUDED.raw_line,product_key=EXCLUDED.product_key,sku_key=EXCLUDED.sku_key,parent_name=EXCLUDED.parent_name,spec_name=EXCLUDED.spec_name,product_kind=EXCLUDED.product_kind,roast_level=EXCLUDED.roast_level,order_quantity=EXCLUDED.order_quantity,order_unit=EXCLUDED.order_unit,normalized_weight_g=EXCLUDED.normalized_weight_g,review_status=EXCLUDED.review_status;
`, sqlText(item.SourceItemKey), sqlText(item.SourceOrderKey), item.LineNo, sqlText(item.RawLine), sqlText(item.ProductKey), sqlText(item.SKUKey), sqlText(item.ParentName), sqlText(item.SpecName), sqlText(item.ProductKind), sqlText(item.RoastLevel), sqlFloat(item.OrderQuantity), sqlText(item.OrderUnit), sqlFloat(item.NormalizedWeightG), sqlText(item.ReviewStatus))
	}
	for _, issue := range dataset.Issues {
		fmt.Fprintf(&b, `INSERT INTO review.issues(issue_key,run_id,entity_type,entity_key,code,severity,message,source_order_key,sheet_name,source_row_number,review_status) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%d,%s) ON CONFLICT (issue_key) DO UPDATE SET run_id=EXCLUDED.run_id,entity_type=EXCLUDED.entity_type,entity_key=EXCLUDED.entity_key,code=EXCLUDED.code,severity=EXCLUDED.severity,message=EXCLUDED.message,source_order_key=EXCLUDED.source_order_key,sheet_name=EXCLUDED.sheet_name,source_row_number=EXCLUDED.source_row_number,review_status=EXCLUDED.review_status;
`, sqlText(issue.IssueKey), sqlText(run.RunID), sqlText(issue.EntityType), sqlText(issue.EntityKey), sqlText(issue.Code), sqlText(issue.Severity), sqlText(issue.Message), sqlText(issue.SourceOrderKey), sqlText(issue.SheetName), issue.SourceRowNumber, sqlText(issue.ReviewStatus))
	}
	b.WriteString("COMMIT;\n")
	return b.String(), nil
}

func reviewStatusCounts(dataset Dataset) map[string]int {
	counts := map[string]int{ReviewAutoReady: 0, ReviewNeedsReview: 0, ReviewApproved: 0, ReviewExcluded: 0}
	for _, order := range dataset.Orders {
		counts[order.ReviewStatus]++
	}
	return counts
}

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeProtected(path, b)
}

func writeProtected(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func writeCSV(path string, rows [][]string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	writer.UseCRLF = false
	if err := writer.WriteAll(rows); err != nil {
		_ = file.Close()
		return err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func sqlText(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func sqlTime(value time.Time) string {
	if value.IsZero() {
		value = time.Now().UTC()
	}
	return sqlText(value.UTC().Format(time.RFC3339Nano)) + "::timestamptz"
}

func sqlFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func sqlNullableFloat(value *float64) string {
	if value == nil {
		return "NULL"
	}
	return sqlFloat(*value)
}

func sqlNullableDate(value string) string {
	if strings.TrimSpace(value) == "" {
		return "NULL"
	}
	return sqlText(value) + "::date"
}

func sheetInventoryCSV(rows []SheetInventory) [][]string {
	out := [][]string{{"工作表", "月份", "是否纳入", "排除原因", "使用行数", "订单行数"}}
	for _, row := range rows {
		out = append(out, []string{row.SheetName, row.Period, strconv.FormatBool(row.Included), row.ExcludedReason, strconv.Itoa(row.UsedRowCount), strconv.Itoa(row.OrderRowCount)})
	}
	return out
}

func rawOrdersCSV(rows []RawOrder) [][]string {
	out := [][]string{{"工作表", "物理行号", "原序号", "重复后缀", "有效序号", "来源键", "内容指纹", "订单日期", "客户原文", "商品原文", "审核状态"}}
	for _, row := range rows {
		out = append(out, []string{row.SheetName, strconv.Itoa(row.SourceRowNumber), row.SequenceOriginal, strconv.Itoa(row.DuplicateSuffix), row.SequenceEffective, row.SourceOrderKey, row.Fingerprint, row.OrderDate, row.CustomerRaw, row.ProductRaw, row.ReviewStatus})
	}
	return out
}

func customersCSV(rows []Customer) [][]string {
	out := [][]string{{"客户键", "规范名称", "手机号", "联系人", "当前地址原文", "ERP客户ID", "ERP名称", "匹配方式", "审核状态"}}
	for _, row := range rows {
		out = append(out, []string{row.CustomerKey, row.CanonicalName, row.NormalizedPhone, row.CurrentContact, row.CurrentAddress, strconv.FormatInt(row.ERPMatchID, 10), row.ERPMatchName, row.MatchMethod, row.ReviewStatus})
	}
	return out
}

func customerAliasesCSV(rows []CustomerAlias) [][]string {
	out := [][]string{{"客户键", "历史名称", "规范名称键", "来源订单键", "出现日期"}}
	for _, row := range rows {
		out = append(out, []string{row.CustomerKey, row.Alias, row.AliasNormalized, row.SourceOrderKey, row.ObservedDate})
	}
	return out
}

func customerPhonesCSV(rows []CustomerPhone) [][]string {
	out := [][]string{{"客户键", "电话原值", "规范电话", "是否主电话", "来源订单键"}}
	for _, row := range rows {
		out = append(out, []string{row.CustomerKey, row.PhoneRaw, row.PhoneNormalized, strconv.FormatBool(row.IsPrimary), row.SourceOrderKey})
	}
	return out
}

func productsCSV(rows []Product) [][]string {
	out := [][]string{{"父商品键", "规范名称", "商品类型", "烘焙度", "ERP商品ID", "ERP名称", "匹配方式", "匹配分", "审核状态"}}
	for _, row := range rows {
		out = append(out, []string{row.ProductKey, row.CanonicalName, row.ProductKind, row.RoastLevel, strconv.FormatInt(row.ERPMatchID, 10), row.ERPMatchName, row.MatchMethod, sqlFloat(row.MatchScore), row.ReviewStatus})
	}
	return out
}

func skusCSV(rows []SKU) [][]string {
	out := [][]string{{"SKU键", "父商品键", "规格名称", "销售单位", "净含量", "净含量单位", "单规格克数", "审核状态"}}
	for _, row := range rows {
		out = append(out, []string{row.SKUKey, row.ProductKey, row.SpecName, row.SalesUnit, sqlFloat(row.NetContentQty), row.NetContentUnit, sqlFloat(row.NormalizedWeightG), row.ReviewStatus})
	}
	return out
}

func productAliasesCSV(rows []ProductAlias) [][]string {
	out := [][]string{{"父商品键", "SKU键", "原始商品行", "规范商品行", "来源订单键", "匹配方式", "匹配分"}}
	for _, row := range rows {
		out = append(out, []string{row.ProductKey, row.SKUKey, row.RawLine, row.NormalizedLine, row.SourceOrderKey, row.MatchMethod, sqlFloat(row.MatchScore)})
	}
	return out
}

func ordersCSV(rows []Order) [][]string {
	out := [][]string{{"来源订单键", "工作表", "原序号", "有效序号", "物理行", "订单日期", "客户键", "客户原文", "订单来源", "订单类型", "付款状态", "发货状态", "金额", "金额原文", "是否派生金额", "运费", "运费原文", "单号", "备注", "审核状态"}}
	for _, row := range rows {
		out = append(out, []string{row.SourceOrderKey, row.SheetName, row.SequenceOriginal, row.SequenceEffective, strconv.Itoa(row.SourceRowNumber), row.OrderDate, row.CustomerKey, row.CustomerRaw, row.OrderSourceRaw, row.OrderTypeRaw, row.PaymentStatusRaw, row.ShipmentStatusRaw, floatPointerText(row.AmountValue), row.AmountRaw, strconv.FormatBool(row.AmountDerived), floatPointerText(row.ShippingAmountValue), row.ShippingAmountRaw, row.TrackingNoRaw, row.RemarkRaw, row.ReviewStatus})
	}
	return out
}

func orderItemsCSV(rows []OrderItem) [][]string {
	out := [][]string{{"来源明细键", "来源订单键", "行号", "原始商品行", "父商品键", "SKU键", "父商品名称", "规格", "商品类型", "烘焙度", "数量", "单位", "总重量g", "审核状态"}}
	for _, row := range rows {
		out = append(out, []string{row.SourceItemKey, row.SourceOrderKey, strconv.Itoa(row.LineNo), row.RawLine, row.ProductKey, row.SKUKey, row.ParentName, row.SpecName, row.ProductKind, row.RoastLevel, sqlFloat(row.OrderQuantity), row.OrderUnit, sqlFloat(row.NormalizedWeightG), row.ReviewStatus})
	}
	return out
}

func issuesCSV(rows []Issue) [][]string {
	out := [][]string{{"问题键", "实体类型", "实体键", "问题代码", "严重度", "说明", "来源订单键", "工作表", "物理行", "审核状态"}}
	for _, row := range rows {
		out = append(out, []string{row.IssueKey, row.EntityType, row.EntityKey, row.Code, row.Severity, row.Message, row.SourceOrderKey, row.SheetName, strconv.Itoa(row.SourceRowNumber), row.ReviewStatus})
	}
	return out
}

func floatPointerText(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func sortedMapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compactJSON(value any) string {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return strings.TrimSpace(b.String())
}
