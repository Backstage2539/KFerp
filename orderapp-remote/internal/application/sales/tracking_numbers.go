package sales

import (
	"regexp"
	"strings"
)

var trackingNumberSeparatorRE = regexp.MustCompile(`[[:space:],;，；、]+`)

func NormalizeTrackingNumbers(raw string) []string {
	parts := trackingNumberSeparatorRE.Split(strings.TrimSpace(raw), -1)
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		no := strings.TrimSpace(part)
		if no == "" || seen[no] {
			continue
		}
		seen[no] = true
		out = append(out, no)
	}
	return out
}

func TrackingNumbersSummary(numbers []string) string {
	return strings.Join(numbers, "\n")
}
