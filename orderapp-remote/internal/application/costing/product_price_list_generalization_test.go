package costing

import "testing"

func TestLegacyBeanListTypeProductTypeName(t *testing.T) {
	for _, tc := range []struct {
		listType string
		want     string
	}{
		{listType: "commercial", want: "熟豆"},
		{listType: "retail", want: "熟豆"},
		{listType: "green", want: "生豆"},
		{listType: "green_bean", want: "生豆"},
		{listType: "drip", want: "挂耳"},
	} {
		if got := LegacyBeanListTypeProductTypeName(tc.listType); got != tc.want {
			t.Fatalf("LegacyBeanListTypeProductTypeName(%q) = %q, want %q", tc.listType, got, tc.want)
		}
	}
}

func TestNormalizeBeanListCommandCarriesProductTypeFields(t *testing.T) {
	cmd, err := normalizeBeanListCommand(PublishBeanListCommand{
		ListType:              "green",
		Version:               "V4.0.1",
		OwnerType:             "official",
		ProductTypeCategoryID: 23,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.ProductTypeCategoryID != 23 {
		t.Fatalf("product type category id = %d, want 23", cmd.ProductTypeCategoryID)
	}
	if cmd.ProductTypeName != "生豆" {
		t.Fatalf("product type name = %q, want 生豆", cmd.ProductTypeName)
	}
}

func TestNormalizeBeanListPublicationQueryAllowsProductTypeCategory(t *testing.T) {
	query, err := normalizeBeanListPublicationQuery(BeanListPublicationQuery{
		ProductTypeCategoryID: 88,
		OwnerType:             "official",
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.ProductTypeCategoryID != 88 {
		t.Fatalf("product type category id = %d, want 88", query.ProductTypeCategoryID)
	}
}
