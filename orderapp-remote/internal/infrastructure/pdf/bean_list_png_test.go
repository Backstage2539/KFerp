package pdf

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestBeanListRendererRenderPNGProducesLongShareImage(t *testing.T) {
	doc := BeanListDocument{
		Title:           "客户品牌销售豆单",
		ListType:        "green",
		VersionNo:       "V2",
		PublishedAt:     "2026-06-06 14:00",
		Changelog:       "首版转售豆单",
		BrandName:       "客户品牌",
		BrandIntro:      "自有渠道销售豆单",
		BackgroundColor: "#f8f1e5",
		Groups: []BeanListGroup{{
			Category: "生豆",
			Items: []BeanListItem{{
				Code:           "ETH-G1",
				Name:           "埃塞瑰夏",
				BadgeLabel:     "推荐",
				RecommendedUse: "手冲",
				Flavor:         "茉莉花 / 柑橘",
				Description:    "客户转售快照价格",
				Prices: []BeanListPrice{{
					Label: "1kg+",
					Value: "112/kg",
					Red:   true,
				}},
			}},
		}},
	}
	body, err := (BeanListRenderer{}).RenderPNG(doc)
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if !bytes.HasPrefix(body, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("body prefix=%q, want PNG signature", body[:8])
	}
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	if img.Bounds().Dx() < 1000 || img.Bounds().Dy() <= img.Bounds().Dx() {
		t.Fatalf("png bounds=%v, want high-resolution long image", img.Bounds())
	}
	writeBeanListRenderArtifacts(t, doc, body)
}

func writeBeanListRenderArtifacts(t *testing.T, doc BeanListDocument, pngBody []byte) {
	t.Helper()
	dir := os.Getenv("KFERP_BEAN_LIST_ARTIFACT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create artifact dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "customer-resale-bean-list.png"), pngBody, 0o644); err != nil {
		t.Fatalf("write png artifact: %v", err)
	}
	pdfBody, err := (BeanListRenderer{}).Render(doc)
	if err != nil {
		t.Fatalf("Render PDF artifact: %v", err)
	}
	if !bytes.HasPrefix(pdfBody, []byte("%PDF")) {
		t.Fatalf("pdf artifact prefix=%q, want %%PDF", pdfBody[:4])
	}
	if err := os.WriteFile(filepath.Join(dir, "customer-resale-bean-list.pdf"), pdfBody, 0o644); err != nil {
		t.Fatalf("write pdf artifact: %v", err)
	}
}
