package officialaccount

import (
	"encoding/base64"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCallbackRejectsForgedSignatureBeforeDispatch(t *testing.T) {
	h := Handler{Config: Config{Enabled: true, AppID: "wx-official", Token: "test-token-long-enough", AppSecret: "secret", MiniAppID: "mini", AESKey: strings.TrimRight(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))), "=")}}
	e := echo.New()
	e.POST("/callback", h.Callback)
	req := httptest.NewRequest(http.MethodPost, "/callback?signature=wrong&timestamp=1&nonce=n", strings.NewReader("<xml><Event>CLICK</Event><FromUserName>victim</FromUserName><EventKey>ORDERS_RECENT_1</EventKey></xml>"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestEncryptedMessageRejectsWrongAppAndTampering(t *testing.T) {
	key := strings.TrimRight(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))), "=")
	cipher, err := EncryptMessage(key, "wx-a", []byte("<xml>hello</xml>"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecryptMessage(key, "wx-a", cipher)
	if err != nil || string(got) != "<xml>hello</xml>" {
		t.Fatalf("roundtrip %s %v", got, err)
	}
	if _, err = DecryptMessage(key, "wx-b", cipher); err == nil {
		t.Fatal("wrong app accepted")
	}
	if _, err = DecryptMessage(key, "wx-a", cipher[:len(cipher)-5]); err == nil {
		t.Fatal("invalid ciphertext accepted")
	}
}

func TestCallbackRejectsIncompleteConfiguration(t *testing.T) {
	h := Handler{Config: Config{Enabled: true}}
	e := echo.New()
	e.GET("/callback", h.Callback)
	r := httptest.NewRecorder()
	e.ServeHTTP(r, httptest.NewRequest("GET", "/callback?signature="+signature("", "1", "n")+"&timestamp=1&nonce=n&echostr=ok", nil))
	if r.Code != 503 {
		t.Fatalf("incomplete configuration accepted: %d", r.Code)
	}
}
func TestMenuPreviewSignatureIncludesBothDraftAndCurrentMenu(t *testing.T) {
	h := Handler{Config: Config{Token: "test-preview-secret", AppID: "wx"}}
	draft := []byte(`{"button":[{"name":"订单"}]}`)
	current := []byte(`{"button":[]}`)
	sig := h.previewSignature(draft, current, "123")
	for _, other := range []string{h.previewSignature([]byte(`{}`), current, "123"), h.previewSignature(draft, []byte(`{}`), "123"), h.previewSignature(draft, current, "124")} {
		if sig == other {
			t.Fatal("changed menu or expiry reuses preview")
		}
	}
}
