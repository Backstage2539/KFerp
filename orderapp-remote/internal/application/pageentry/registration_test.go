package pageentry

import (
	"encoding/json"
	"testing"
)

func TestRegisteredPriceVisibility(t *testing.T) {
	var d Document
	json.Unmarshal([]byte(`{"name":"普通豆单","kind":"price","visibility":"registered","publication_id":1,"bean_center":true,"bean_sort":20}`), &d)
	if err := Validate(d, true); err != nil {
		t.Fatalf("registered official price should be supported: %v", err)
	}
	d.Kind = "article"
	if Validate(d, true) == nil {
		t.Fatal("registration is only for price pages")
	}
}
