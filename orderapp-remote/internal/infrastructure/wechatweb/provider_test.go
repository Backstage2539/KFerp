package wechatweb

import "testing"

func TestOAuthConfigurationDefaultsAndCallbackOrigin(t *testing.T) {
	t.Setenv("WECHAT_WEB_OAUTH_ENABLED", "")
	t.Setenv("WECHAT_WEB_PUBLIC_ORIGIN", "")
	t.Setenv("WECHAT_OFFICIAL_APP_ID", "")
	t.Setenv("WECHAT_OFFICIAL_APP_SECRET", "")
	if ConfigFromEnv().Ready() {
		t.Fatal("oauth enabled without configuration")
	}
	for _, origin := range []string{"http://pages.example", "https://pages.example/path", "https://user@pages.example", "https://pages.example?q=1", "//pages.example", "https://pages.example#frag"} {
		if (Config{OAuthEnabled: true, PublicOrigin: origin, AppID: "a", AppSecret: "s"}).Ready() {
			t.Fatal("unsafe origin", origin)
		}
	}
	if !(Config{OAuthEnabled: true, PublicOrigin: "https://pages.example", AppID: "a", AppSecret: "s"}).Ready() {
		t.Fatal("valid configuration rejected")
	}
}
