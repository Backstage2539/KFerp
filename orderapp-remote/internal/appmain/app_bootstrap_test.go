package appmain

import "testing"

func TestShouldInstallBasicAuthHonorsDisableFlag(t *testing.T) {
	if shouldInstallBasicAuth(appConfig{DisableBasicAuth: true}) {
		t.Fatal("shouldInstallBasicAuth = true, want false when DisableBasicAuth is enabled")
	}
	if !shouldInstallBasicAuth(appConfig{}) {
		t.Fatal("shouldInstallBasicAuth = false, want true by default")
	}
}
