package appmain

import (
	"testing"

	customerportalhttp "orderapp/internal/interfaces/http/customerportal"
)

func TestCustomerPortalIdentityProviderSelection(t *testing.T) {
	if _, ok := customerPortalIdentityProvider(appConfig{CustomerPortalDevLogin: true, CustomerPortalDevOpenID: "dev-openid-van"}).(customerportalhttp.StaticIdentityProvider); !ok {
		t.Fatal("dev login should use StaticIdentityProvider")
	}
	if _, ok := customerPortalIdentityProvider(appConfig{WechatMiniAppID: "wx-test-app", WechatMiniAppSecret: "wx-test-secret"}).(customerportalhttp.WechatIdentityProvider); !ok {
		t.Fatal("wechat credentials should use WechatIdentityProvider")
	}
	if _, ok := customerPortalIdentityProvider(appConfig{}).(customerportalhttp.DisabledIdentityProvider); !ok {
		t.Fatal("missing credentials should use DisabledIdentityProvider")
	}
}
