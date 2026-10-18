package customerportal

import "testing"

func TestPriceCenterEmployeeRole(t *testing.T) {
	for _, tc := range []struct {
		account string
		roles   []string
		want    bool
	}{{"employee", []string{"sales"}, true}, {"employee", []string{"admin"}, true}, {"employee", []string{"production"}, false}, {"customer", []string{"admin"}, false}} {
		if got := CanUsePriceCenter(CurrentContext{EmployeeID: 9, AccountType: tc.account, Roles: tc.roles}); got != tc.want {
			t.Fatalf("%+v got %v", tc, got)
		}
	}
}

func TestPriceCenterRejectsMissingEmployeeIdentity(t *testing.T) {
	if CanUsePriceCenter(CurrentContext{AccountType: "employee", Roles: []string{"sales"}}) {
		t.Fatal("missing employee identity accepted")
	}
}
