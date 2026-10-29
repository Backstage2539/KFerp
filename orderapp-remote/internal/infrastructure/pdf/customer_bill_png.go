package pdf

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	app "orderapp/internal/application/customerfulfillment"
)

// RenderCustomerBillPNG rasterizes the same bill PDF and places the saved
// bill-specific stamp in PDF point coordinates. It never returns a partial image.
func RenderCustomerBillPNG(ctx context.Context, bill app.BillingBill, settings app.BillingPDFSettings, seal []byte) ([]byte, error) {
	data, err := RenderCustomerBillStamped(bill, settings, seal)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "kferp-bill-png-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "bill.pdf")
	if err = os.WriteFile(input, data, 0600); err != nil {
		return nil, err
	}
	renderCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	info, err := exec.CommandContext(renderCtx, "pdfinfo", input).Output()
	if err != nil {
		return nil, fmt.Errorf("账单页数读取失败: %w", err)
	}
	pageCount := 0
	for _, line := range strings.Split(string(info), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			pageCount, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Pages:")))
			break
		}
	}
	if pageCount < 1 || pageCount > 24 {
		return nil, fmt.Errorf("账单过长，无法生成完整图片，请下载 PDF")
	}
	command := exec.CommandContext(renderCtx, "pdftoppm", "-png", "-r", "144", input, filepath.Join(dir, "page"))
	if out, err := command.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("账单图片生成失败: %w (%s)", err, string(out))
	}
	paths, err := filepath.Glob(filepath.Join(dir, "page-*.png"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if len(paths) != pageCount {
		return nil, fmt.Errorf("账单图片页数不完整")
	}
	width, height := 0, 0
	sizes := make([]image.Point, 0, len(paths))
	for _, path := range paths {
		f, e := os.Open(path)
		if e != nil {
			return nil, e
		}
		cfg, _, e := image.DecodeConfig(f)
		f.Close()
		if e != nil {
			return nil, e
		}
		if cfg.Width > width {
			width = cfg.Width
		}
		height += cfg.Height
		sizes = append(sizes, image.Pt(cfg.Width, cfg.Height))
		if width > 32767 || height > 32767 || int64(width)*int64(height) > 100000000 {
			return nil, fmt.Errorf("账单过长，无法生成完整图片，请下载 PDF")
		}
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	y := 0
	for index, path := range paths {
		f, e := os.Open(path)
		if e != nil {
			return nil, e
		}
		page, _, e := image.Decode(f)
		f.Close()
		if e != nil {
			return nil, e
		}
		draw.Draw(canvas, image.Rect(0, y, sizes[index].X, y+sizes[index].Y), page, page.Bounds().Min, draw.Src)
		y += sizes[index].Y
	}
	var out bytes.Buffer
	if err = png.Encode(&out, canvas); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
