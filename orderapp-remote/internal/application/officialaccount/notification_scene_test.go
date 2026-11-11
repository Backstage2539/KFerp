package officialaccount

import (
	"strings"
	"testing"
)

func TestNotificationSceneRoundTripAndLegacy(t *testing.T) {
	challenge := strings.Repeat("01ab", 16)
	scene, err := NotificationScene(challenge)
	if err != nil || len(scene) > 64 {
		t.Fatalf("scene length=%d err=%v", len(scene), err)
	}
	for _, input := range []string{scene, "notify_" + challenge} {
		got, err := NotificationChallenge(input)
		if err != nil || got != challenge {
			t.Fatalf("roundtrip got %q err %v", got, err)
		}
	}
}

func TestNotificationSceneRejectsMalformedChallenges(t *testing.T) {
	for _, challenge := range []string{"", strings.Repeat("z", 64), strings.Repeat("ab", 31), strings.Repeat("AB", 32)} {
		if _, err := NotificationScene(challenge); err == nil {
			t.Fatalf("accepted invalid challenge length %d", len(challenge))
		}
	}
	for _, scene := range []string{"", "notify_", "other_" + strings.Repeat("ab", 32), "notify_" + strings.Repeat("z", 64), "notify_" + strings.Repeat("_", 43)} {
		if _, err := NotificationChallenge(scene); err == nil {
			t.Fatalf("accepted invalid scene length %d", len(scene))
		}
	}
}
