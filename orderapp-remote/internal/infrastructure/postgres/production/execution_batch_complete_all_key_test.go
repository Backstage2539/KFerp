package production

import (
	"strings"
	"testing"
)

func TestBatchChildRequestIDLengthAndIdentity(t *testing.T) {
	outer := strings.Repeat("a", 160)
	a := batchChildRequestID(outer, 1)
	b := batchChildRequestID(outer, 2)
	if len(a) > 160 || a == b || a != batchChildRequestID(outer, 1) {
		t.Fatalf("child keys must be bounded, deterministic and batch-specific: %d %q %q", len(a), a, b)
	}
}
