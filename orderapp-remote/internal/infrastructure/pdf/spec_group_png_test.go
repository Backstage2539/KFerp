package pdf

import (
	"bytes"
	"image/png"
	"testing"
)

func TestBeanListPNGShowsSpecAttributesAndMakesRoom(t *testing.T) {
	item := BeanListItem{Name: "模拟咖啡", Prices: []BeanListPrice{{Label: "基础价", Value: "33.80/袋"}}}
	original := BeanListDocument{Title: "价格表", Groups: []BeanListGroup{{Items: []BeanListItem{item}}}}
	base, err := (BeanListRenderer{}).RenderPNG(original)
	if err != nil {
		t.Fatal(err)
	}
	item.AttributeLines = []string{"规格：227g", "规格：100g", "规格：50g", "烘焙度：中深烘"}
	original.Groups[0].Items[0] = item
	updated, err := (BeanListRenderer{}).RenderPNG(original)
	if err != nil {
		t.Fatal(err)
	}
	baseImg, err := png.Decode(bytes.NewReader(base))
	if err != nil {
		t.Fatal(err)
	}
	updatedImg, err := png.Decode(bytes.NewReader(updated))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := updatedImg.At(200, 600).RGBA()
	if r < 0x8000 || g < 0x8000 || b < 0x8000 {
		t.Fatalf("price card background is not readable: r=%d g=%d b=%d", r, g, b)
	}
	if bytes.Equal(base, updated) {
		t.Fatalf("attributes must be visible in the rendered image: base=%v updated=%v", baseImg.Bounds(), updatedImg.Bounds())
	}
}
