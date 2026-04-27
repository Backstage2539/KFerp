package sales

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

type orderShippingExcelFile struct {
	Filename string
	Path     string
	URL      string
}

func registerOrderShippingExcelRoutes(e *echo.Echo, _ *salesapp.Service) {
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
}

func generateOrderShippingExcel(salesSvc *salesapp.Service, c echo.Context, orderID int64) (orderShippingExcelFile, error) {
	data, err := salesSvc.LoadOrderShippingExportData(c.Request().Context(), orderID)
	if err != nil {
		return orderShippingExcelFile{}, err
	}
	sender, err := salesSvc.LoadSenderProfile(c.Request().Context())
	if err != nil {
		return orderShippingExcelFile{}, err
	}
	tmpl, err := orderShippingTemplatePath()
	if err != nil {
		return orderShippingExcelFile{}, err
	}
	dir := orderShippingExportDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return orderShippingExcelFile{}, err
	}

	wb, err := buildOrderShippingWorkbook(tmpl, sender, data)
	if err != nil {
		return orderShippingExcelFile{}, err
	}
	defer wb.Close()

	filename := orderShippingFilename(data)
	path := filepath.Join(dir, filename)
	if err := wb.SaveAs(path); err != nil {
		return orderShippingExcelFile{}, err
	}
	return orderShippingExcelFile{
		Filename: filename,
		Path:     path,
		URL:      "/ship/order_exports/" + url.PathEscape(filename),
	}, nil
}

func buildOrderShippingWorkbook(templatePath string, sender salesapp.SenderProfile, data salesapp.OrderShippingExportData) (*excelize.File, error) {
	wb, err := excelize.OpenFile(templatePath)
	if err != nil {
		return nil, err
	}
	sheet := wb.GetSheetName(0)
	if sheet == "" {
		_ = wb.Close()
		return nil, fmt.Errorf("template has no sheet")
	}

	goods := strings.TrimSpace(sender.Goods)
	if goods == "" || goods == "咖啡" {
		goods = "茶叶"
	}
	row := 2
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
		price := strings.TrimSpace(item.UnitPrice)
		total := strings.TrimSpace(item.LineTotal)
		line := fmt.Sprintf("%s %s x%s%s", name, spec, qty, unit)
		if price != "" {
			line += " 单价" + price
		}
		if total != "" {
			line += " 小计" + total
		}
		parts = append(parts, strings.TrimSpace(line))
	}
	return strings.Join(parts, "；")
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
