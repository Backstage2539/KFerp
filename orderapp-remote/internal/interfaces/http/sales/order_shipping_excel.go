package sales

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

type orderShippingExcelFile struct {
	Filename   string
	Path       string
	URL        string
	ShipmentID int64
	ShipmentNo string
}

type orderShippingExcelRequest struct {
	OrderIDs     []int64                       `json:"order_ids"`
	SenderID     int64                         `json:"sender_id"`
	OrderSenders []orderShippingSenderOverride `json:"order_senders"`
}

type orderShippingSenderOverride struct {
	OrderID  int64 `json:"order_id"`
	SenderID int64 `json:"sender_id"`
}

type orderShippingTrackingRequest struct {
	ShipmentID int64                          `json:"shipment_id"`
	Items      []orderShippingTrackingItemAPI `json:"items"`
}

type orderSingleShippingTrackingRequest struct {
	TrackingNo string `json:"tracking_no"`
}

type orderShippingTrackingItemAPI struct {
	OrderID    int64  `json:"order_id"`
	TrackingNo string `json:"tracking_no"`
}

type orderShippingExcelRow struct {
	Data   salesapp.OrderShippingExportData
	Sender salesapp.SenderProfile
}

func registerOrderShippingExcelRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	e.GET("/ship/order_exports/:filename", func(c echo.Context) error {
		filename := filepath.Base(strings.TrimSpace(c.Param("filename")))
		if filename == "." || filename == "" || filename != strings.TrimSpace(c.Param("filename")) {
			return c.String(http.StatusBadRequest, "invalid filename")
		}
		dir := orderShippingExportDir()
		path := filepath.Join(dir, filename)
		if !strings.HasSuffix(path, ".xlsx") {
			return c.String(http.StatusBadRequest, "invalid filename")
		}
		if _, err := os.Stat(path); err != nil {
			return c.String(http.StatusNotFound, "file not found")
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"%s\"", filename))
		return c.File(path)
	})
	e.POST("/api/orders/shipping-excel", func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		var req orderShippingExcelRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		}
		orderIDs := normalizeOrderShippingIDs(req.OrderIDs)
		if len(orderIDs) == 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请选择可发货的订单"})
		}
		file, err := generateOrdersShippingExcel(salesSvc, c, orderIDs, req.SenderID, req.OrderSenders)
		if err != nil {
			if strings.Contains(err.Error(), "尚不可发货") {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "快递录单 Excel 生成失败：" + err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"shipping_excel_url": file.URL,
			"shipment_id":        file.ShipmentID,
			"shipment_no":        file.ShipmentNo,
			"count":              len(orderIDs),
		})
	})
	e.POST("/api/orders/shipping-tracking", func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		var req orderShippingTrackingRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		}
		items := make([]salesapp.ShipmentTrackingItemCommand, 0, len(req.Items))
		for _, item := range req.Items {
			items = append(items, salesapp.ShipmentTrackingItemCommand{
				OrderID:    item.OrderID,
				TrackingNo: item.TrackingNo,
			})
		}
		res, err := salesSvc.FillShipmentTracking(c.Request().Context(), salesapp.FillShipmentTrackingCommand{
			Actor:      support.ActorOf(c),
			ShipmentID: req.ShipmentID,
			Items:      items,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, res)
	})
	e.POST("/api/orders/:id/shipping-tracking", func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		orderID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
		if err != nil || orderID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid order id"})
		}
		var req orderSingleShippingTrackingRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
		}
		res, err := salesSvc.FillOrderTracking(c.Request().Context(), salesapp.FillOrderTrackingCommand{
			Actor:      support.ActorOf(c),
			OrderID:    orderID,
			TrackingNo: req.TrackingNo,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, res)
	})
	e.POST("/api/orders/shipping-tracking-excel", func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		file, err := c.FormFile("file")
		if err != nil {
			file, err = c.FormFile("excel")
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "请选择回传 Excel 文件"})
		}
		src, err := file.Open()
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Excel 文件读取失败"})
		}
		defer src.Close()
		items, err := parseShipmentTrackingExcel(src)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		res, err := salesSvc.FillShipmentTrackingByOrderNo(c.Request().Context(), salesapp.FillShipmentTrackingByOrderNoCommand{
			Actor: support.ActorOf(c),
			Items: items,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, res)
	})
}

func generateOrderShippingExcel(salesSvc *salesapp.Service, c echo.Context, orderID int64) (orderShippingExcelFile, error) {
	return generateOrdersShippingExcel(salesSvc, c, []int64{orderID}, 0, nil)
}

func parseShipmentTrackingExcel(r io.Reader) ([]salesapp.ShipmentTrackingByOrderNoItemCommand, error) {
	wb, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("Excel 文件格式无法解析")
	}
	defer wb.Close()
	sheet := wb.GetSheetName(0)
	if sheet == "" {
		return nil, fmt.Errorf("Excel 文件没有工作表")
	}
	rows, err := wb.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("Excel 工作表读取失败")
	}
	trackingCol, orderCol, remarkCol, dataStart := -1, -1, -1, -1
	for i, row := range rows {
		header := mapTrackingExcelHeaders(row)
		trackingCol = firstHeaderIndex(header, "运单号", "快递单号", "物流单号", "trackingno", "tracking_no", "tracking", "waybill", "mailno", "mail_no")
		orderCol = firstHeaderIndex(header, "备注订单号", "内部订单号", "erp订单号", "用户平台订单号", "订单号", "orderno", "order_no")
		remarkCol = firstHeaderIndex(header, "备注", "remark", "memo")
		if trackingCol >= 0 && (orderCol >= 0 || remarkCol >= 0) {
			dataStart = i + 1
			break
		}
	}
	if dataStart < 0 {
		return nil, fmt.Errorf("Excel 缺少运单号和备注/订单号列")
	}
	items := make([]salesapp.ShipmentTrackingByOrderNoItemCommand, 0, len(rows)-dataStart)
	for _, row := range rows[dataStart:] {
		trackingNo := strings.TrimSpace(rowValue(row, trackingCol))
		if trackingNo == "" {
			continue
		}
		orderNo := ""
		if orderCol >= 0 {
			orderNo = extractShipmentOrderNoCell(rowValue(row, orderCol))
		}
		if orderNo == "" && remarkCol >= 0 {
			orderNo = extractShipmentOrderNo(rowValue(row, remarkCol))
		}
		if orderNo == "" {
			continue
		}
		items = append(items, salesapp.ShipmentTrackingByOrderNoItemCommand{
			OrderNo:    orderNo,
			TrackingNo: trackingNo,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("Excel 未解析到可回填的订单号和快递单号")
	}
	return items, nil
}

func generateOrdersShippingExcel(salesSvc *salesapp.Service, c echo.Context, orderIDs []int64, senderID int64, overrides []orderShippingSenderOverride) (orderShippingExcelFile, error) {
	orderIDs = normalizeOrderShippingIDs(orderIDs)
	if len(orderIDs) == 0 {
		return orderShippingExcelFile{}, fmt.Errorf("order required")
	}
	senderOverrides := normalizeOrderSenderOverrides(overrides)
	senderCache := make(map[int64]salesapp.SenderProfile)
	defaultSender, err := salesSvc.LoadSenderProfileByID(c.Request().Context(), senderID)
	if err != nil {
		return orderShippingExcelFile{}, err
	}
	rows := make([]salesapp.OrderShippingExportData, 0, len(orderIDs))
	excelRows := make([]orderShippingExcelRow, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		data, err := salesSvc.LoadOrderShippingExportData(c.Request().Context(), orderID)
		if err != nil {
			return orderShippingExcelFile{}, err
		}
		if !orderShippingReady(data) {
			return orderShippingExcelFile{}, fmt.Errorf("订单 %s 尚不可发货", firstNonEmpty(data.OrderNo, fmt.Sprintf("%d", data.OrderID)))
		}
		sender := defaultSender
		if overrideSenderID := senderOverrides[orderID]; overrideSenderID > 0 {
			cached, ok := senderCache[overrideSenderID]
			if !ok {
				cached, err = salesSvc.LoadSenderProfileByID(c.Request().Context(), overrideSenderID)
				if err != nil {
					return orderShippingExcelFile{}, err
				}
				senderCache[overrideSenderID] = cached
			}
			sender = cached
		} else if data.SenderID > 0 {
			cached, ok := senderCache[data.SenderID]
			if !ok {
				cached, err = salesSvc.LoadSenderProfileByID(c.Request().Context(), data.SenderID)
				if err != nil {
					return orderShippingExcelFile{}, err
				}
				senderCache[data.SenderID] = cached
			}
			sender = cached
		}
		rows = append(rows, data)
		excelRows = append(excelRows, orderShippingExcelRow{Data: data, Sender: sender})
	}
	tmpl, err := orderShippingTemplatePath()
	if err != nil {
		return orderShippingExcelFile{}, err
	}
	dir := orderShippingExportDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return orderShippingExcelFile{}, err
	}

	wb, err := buildOrdersShippingWorkbookRows(tmpl, excelRows)
	if err != nil {
		return orderShippingExcelFile{}, err
	}
	defer wb.Close()

	filename := ordersShippingFilename(rows)
	path := filepath.Join(dir, filename)
	if err := wb.SaveAs(path); err != nil {
		return orderShippingExcelFile{}, err
	}
	shipmentOrders := make([]salesapp.OrderShipmentOrderCommand, 0, len(excelRows))
	for _, row := range excelRows {
		shipmentOrders = append(shipmentOrders, salesapp.OrderShipmentOrderCommand{
			OrderID:  row.Data.OrderID,
			SenderID: row.Sender.ID,
		})
	}
	shipment, err := salesSvc.CreateOrderShipment(c.Request().Context(), salesapp.CreateOrderShipmentCommand{
		Actor:    support.ActorOf(c),
		SenderID: defaultSender.ID,
		FileURL:  "/ship/order_exports/" + url.PathEscape(filename),
		Orders:   shipmentOrders,
	})
	if err != nil {
		return orderShippingExcelFile{}, err
	}
	return orderShippingExcelFile{
		Filename:   filename,
		Path:       path,
		URL:        "/ship/order_exports/" + url.PathEscape(filename),
		ShipmentID: shipment.ShipmentID,
		ShipmentNo: shipment.ShipmentNo,
	}, nil
}

func buildOrderShippingWorkbook(templatePath string, sender salesapp.SenderProfile, data salesapp.OrderShippingExportData) (*excelize.File, error) {
	return buildOrdersShippingWorkbook(templatePath, sender, []salesapp.OrderShippingExportData{data})
}

func buildOrdersShippingWorkbook(templatePath string, sender salesapp.SenderProfile, rows []salesapp.OrderShippingExportData) (*excelize.File, error) {
	excelRows := make([]orderShippingExcelRow, 0, len(rows))
	for _, data := range rows {
		excelRows = append(excelRows, orderShippingExcelRow{Data: data, Sender: sender})
	}
	return buildOrdersShippingWorkbookRows(templatePath, excelRows)
}

func buildOrdersShippingWorkbookRows(templatePath string, rows []orderShippingExcelRow) (*excelize.File, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("order required")
	}
	wb, err := excelize.OpenFile(templatePath)
	if err != nil {
		return nil, err
	}
	sheet := wb.GetSheetName(0)
	if sheet == "" {
		_ = wb.Close()
		return nil, fmt.Errorf("template has no sheet")
	}

	for i, rowData := range rows {
		row := i + 2
		data := rowData.Data
		sender := rowData.Sender
		goods := strings.TrimSpace(sender.Goods)
		if goods == "" || goods == "咖啡" {
			goods = "茶叶"
		}
		values := map[string]any{
			"A": strings.TrimSpace(data.RecvName),
			"B": strings.TrimSpace(data.RecvPhone),
			"C": strings.TrimSpace(data.RecvAddr),
			"D": strings.TrimSpace(sender.Name),
			"E": strings.TrimSpace(sender.Phone),
			"F": strings.TrimSpace(sender.Addr),
			"G": strings.TrimSpace(data.RecvCompany),
			"H": 1,
			"I": goods,
			"J": 0.1,
			"N": orderShippingRemark(data),
			"O": strings.TrimSpace(sender.Company),
			"P": strings.TrimSpace(sender.BizType),
		}
		for col, value := range values {
			if err := wb.SetCellValue(sheet, fmt.Sprintf("%s%d", col, row), value); err != nil {
				_ = wb.Close()
				return nil, err
			}
		}
	}
	return wb, nil
}

func orderShippingTemplatePath() (string, error) {
	candidates := []string{
		strings.TrimSpace(os.Getenv("ORDER_SHIP_TEMPLATE")),
		strings.TrimSpace(os.Getenv("SF_SMALL_TEMPLATE")),
		"/app/data/ship_temp.xlsx",
		"/app/docs/ship_temp.xlsx",
		"/data/ship_temp.xlsx",
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("shipping template not found")
}

func orderShippingExportDir() string {
	if dir := strings.TrimSpace(os.Getenv("ORDER_SHIP_EXPORT_DIR")); dir != "" {
		return dir
	}
	return "/app/data/shipping_exports"
}

func orderShippingFilename(data salesapp.OrderShippingExportData) string {
	date := strings.ReplaceAll(strings.TrimSpace(data.OrderDate), "-", "")
	if date == "" {
		date = time.Now().Format("20060102")
	}
	name := sanitizeShippingFilenamePart(firstNonEmpty(data.CustomerName, data.RecvName, "customer"))
	orderNo := sanitizeShippingFilenamePart(firstNonEmpty(data.OrderNo, fmt.Sprintf("%d", data.OrderID)))
	return fmt.Sprintf("ship_%s_%s_%s.xlsx", date, name, orderNo)
}

func ordersShippingFilename(rows []salesapp.OrderShippingExportData) string {
	if len(rows) == 1 {
		return orderShippingFilename(rows[0])
	}
	date := ""
	if len(rows) > 0 {
		date = strings.ReplaceAll(strings.TrimSpace(rows[0].OrderDate), "-", "")
	}
	if date == "" {
		date = time.Now().Format("20060102")
	}
	return fmt.Sprintf("ship_%s_%d_orders.xlsx", date, len(rows))
}

func orderShippingRemark(data salesapp.OrderShippingExportData) string {
	parts := make([]string, 0, len(data.Items)+1)
	if strings.TrimSpace(data.OrderNo) != "" {
		parts = append(parts, strings.TrimSpace(data.OrderNo))
	}
	for _, item := range data.Items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = "商品"
		}
		spec := strings.TrimSpace(item.Spec)
		qty := strings.TrimSpace(item.Qty)
		unit := strings.TrimSpace(item.Unit)
		if unit == "" {
			unit = "件"
		}
		line := fmt.Sprintf("%s %s x%s%s", name, spec, qty, unit)
		parts = append(parts, strings.TrimSpace(line))
	}
	return strings.Join(parts, "；")
}

func orderShippingReady(data salesapp.OrderShippingExportData) bool {
	status := strings.TrimSpace(data.ProcessStatus)
	return strings.Contains(status, "生产完成") || status == "无需生产" || status == "库存待发货"
}

func normalizeOrderShippingIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func normalizeOrderSenderOverrides(overrides []orderShippingSenderOverride) map[int64]int64 {
	out := make(map[int64]int64, len(overrides))
	for _, override := range overrides {
		if override.OrderID <= 0 || override.SenderID <= 0 {
			continue
		}
		out[override.OrderID] = override.SenderID
	}
	return out
}

func mapTrackingExcelHeaders(row []string) map[string]int {
	out := make(map[string]int, len(row))
	for i, cell := range row {
		key := normalizeTrackingExcelHeader(cell)
		if key == "" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = i
		}
	}
	return out
}

func normalizeTrackingExcelHeader(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	replacer := strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
		"（选填）", "",
		"(选填)", "",
		"（可选）", "",
		"(可选)", "",
	)
	return replacer.Replace(value)
}

func firstHeaderIndex(header map[string]int, aliases ...string) int {
	for _, alias := range aliases {
		if idx, ok := header[normalizeTrackingExcelHeader(alias)]; ok {
			return idx
		}
	}
	return -1
}

func rowValue(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

var shipmentOrderNoPattern = regexp.MustCompile(`SO[-A-Za-z0-9]+`)

func extractShipmentOrderNo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if match := shipmentOrderNoPattern.FindString(value); match != "" {
		return strings.TrimSpace(match)
	}
	return ""
}

func extractShipmentOrderNoCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if orderNo := extractShipmentOrderNo(value); orderNo != "" {
		return orderNo
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var unsafeFilenameChars = regexp.MustCompile(`[\\/:*?"<>|\s]+`)

func sanitizeShippingFilenamePart(value string) string {
	value = strings.TrimSpace(value)
	value = unsafeFilenameChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, "._")
	if value == "" {
		return "unknown"
	}
	return value
}
