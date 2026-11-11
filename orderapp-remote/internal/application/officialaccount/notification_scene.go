package officialaccount

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const notificationScenePrefix = "notify_"

// NotificationScene keeps the complete 256-bit challenge while fitting the
// scene value into 64 characters, including the notification prefix.
func NotificationScene(challenge string) (string, error) {
	raw, err := hex.DecodeString(challenge)
	if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != challenge {
		return "", errors.New("invalid binding challenge")
	}
	return notificationScenePrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// NotificationChallenge accepts both compact scenes and already issued
// legacy hex scenes until their short-lived QR codes expire.
func NotificationChallenge(scene string) (string, error) {
	if !strings.HasPrefix(scene, notificationScenePrefix) {
		return "", errors.New("invalid binding scene")
	}
	value := strings.TrimPrefix(scene, notificationScenePrefix)
	if len(value) == 64 {
		raw, err := hex.DecodeString(value)
		if err == nil && len(raw) == 32 && hex.EncodeToString(raw) == value {
			return value, nil
		}
		return "", errors.New("invalid binding scene")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != value {
		return "", errors.New("invalid binding scene")
	}
	return hex.EncodeToString(raw), nil
}
