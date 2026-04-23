package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestSanitizedRawQueryRedactsSensitiveValues(t *testing.T) {
	q := url.Values{}
	q.Set("token", "secret")
	q.Set("password", "secret")
	q.Set("order_no", "SO-1")

	got := sanitizedRawQuery(q)
	if strings.Contains(got, "secret") {
		t.Fatalf("sanitizedRawQuery leaked sensitive value: %q", got)
	}
	for _, want := range []string{"token=REDACTED", "password=REDACTED", "order_no=SO-1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("sanitizedRawQuery() = %q, want %q", got, want)
		}
	}
}

func TestShouldSkipOperationLogOnlySkipsFrontendStaticAssets(t *testing.T) {
	e := echo.New()
	cases := []struct {
		path string
		skip bool
	}{
		{path: "/bom-react/assets/index.js", skip: true},
		{path: "/vue-shell/assets/index.css", skip: true},
		{path: "/favicon.ico", skip: true},
		{path: "/assets/customer_assets/1", skip: false},
		{path: "/orders", skip: false},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if got := shouldSkipOperationLog(c); got != tc.skip {
			t.Fatalf("shouldSkipOperationLog(%q) = %v, want %v", tc.path, got, tc.skip)
		}
	}
}
