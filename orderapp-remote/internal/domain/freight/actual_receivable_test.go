package freight

import "testing"

func TestActualReceivable(t *testing.T) {
	for _, c := range []struct {
		a      ActualReceivableInput
		want   int64
		review bool
	}{{ActualReceivableInput{Actual: 1200, Surcharge: 200, Current: 2000}, 1400, false}, {ActualReceivableInput{Actual: 1200, Surcharge: 200, Current: 2000, Paid: 500}, 1400, false}, {ActualReceivableInput{Actual: 1200, Current: 2000, Paid: 2000}, 2000, false}, {ActualReceivableInput{Actual: 1200, Free: true}, 0, false}, {ActualReceivableInput{Actual: 100, Current: 2000, Paid: 500}, 2000, true}, {ActualReceivableInput{Actual: 1200, Current: 2000, UnknownPayment: true}, 2000, true}} {
		got, e := ConfirmActualReceivable(c.a)
		if e != nil || got.TargetCents != c.want || got.Review != c.review {
			t.Fatalf("%+v got %+v err %v", c, got, e)
		}
	}
}
