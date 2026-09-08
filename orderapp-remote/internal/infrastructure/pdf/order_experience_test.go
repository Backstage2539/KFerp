package pdf

import (
	"bytes"
	"encoding/json"
	salesdomain "orderapp/internal/domain/sales"
	"strings"
	"testing"
)

func TestOrderExperienceUTF8HeaderUsesRenderedHeight(t *testing.T) {
	r := SalesOrderRenderer{}
	font, err := r.resolveFontPath()
	if err != nil {
		t.Fatal(err)
	}
	p := newSalesOrderTestPDF(t, font)
	text := "关联订单：" + strings.Repeat("SO-20260908-0009、", 12)
	start := p.GetY()
	lines := salesOrderWrapCellText(p, text, 178)
	writeSalesOrderMetaRow(p, []float64{178}, []string{text}, 6)
	if p.GetY() < start+float64(len(lines))*6 {
		t.Fatalf("header ends at %v, needs at least %v", p.GetY(), start+float64(len(lines))*6)
	}
}

func TestOrderExperiencePaymentNeverReturnsToConfiguredEarlierPage(t *testing.T) {
	r := SalesOrderRenderer{}
	font, err := r.resolveFontPath()
	if err != nil {
		t.Fatal(err)
	}
	p := newSalesOrderTestPDF(t, font)
	p.AddPage()
	p.SetY(180)
	s := salesdomain.SalesOrderSnapshot{Note: "个性化说明仅在末页", PaymentTextBox: salesdomain.SalesOrderLayoutBox{XMM: 16, YMM: 118, WidthMM: 104, HeightMM: 78, PageNumber: 1}}
	r.renderSalesOrderPaymentInfoSectionWithPageBreak(p, s)
	if p.PageCount() != 3 {
		t.Fatalf("payment must move after body to new last page; pages=%d", p.PageCount())
	}
}

func TestOrderExperienceSnapshotKeepsOrderAndSalesNotesDistinct(t *testing.T) {
	var snapshot salesdomain.SalesOrderSnapshot
	if err := json.Unmarshal([]byte(`{"order_note":"订单原始备注","sales_order_note":"销售单专用备注"}`), &snapshot); err != nil {
		t.Fatal(err)
	}
	rows := salesOrderFinancialRows(snapshot)
	found := false
	for _, row := range rows {
		if row.Label == "订单备注" && row.Value == "订单原始备注" {
			found = true
		}
	}
	if !found {
		t.Fatalf("order note missing: %+v", rows)
	}
	var group salesdomain.CombinedSalesOrderGroup
	if err := json.Unmarshal([]byte(`{"order_note":"分组订单备注","sales_order_note":"销售单专用备注"}`), &group); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(combinedSalesOrderGroupNote(group), "订单备注：分组订单备注") {
		t.Fatal("group order note missing")
	}
}

func TestOrderExperiencePageNumbersDoNotCreateExtraPages(t *testing.T) {
	r := SalesOrderRenderer{}
	font, err := r.resolveFontPath()
	if err != nil {
		t.Fatal(err)
	}
	p := newSalesOrderTestPDF(t, font)
	p.AddPage()
	renderSalesOrderDocumentOverlays(p, true)
	if p.PageCount() != 2 {
		t.Fatalf("page number overlay created pages: %d", p.PageCount())
	}
}

func TestOrderExperiencePaymentWrapDoesNotDropChineseCharacters(t *testing.T) {
	r := SalesOrderRenderer{}
	font, err := r.resolveFontPath()
	if err != nil {
		t.Fatal(err)
	}
	p := newSalesOrderTestPDF(t, font)
	p.SetCompression(false)
	renderSalesOrderTextBlock(p, salesdomain.SalesOrderLayoutBox{XMM: 16, YMM: 14, WidthMM: 25, HeightMM: 265}, 14, "说明", []string{strings.Repeat("流", 80)})
	var b bytes.Buffer
	if err := p.Output(&b); err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(b.Bytes(), []byte{0x6d, 0x41}); n < 80 {
		t.Fatalf("Chinese line wrapping dropped characters: got %d, want 80", n)
	}
}
