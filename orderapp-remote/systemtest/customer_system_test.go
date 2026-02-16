//go:build system

package systemtest

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
)

// System tests hit a running orderapp instance.
// Usage:
//   ORDERAPP_BASE_URL=http://localhost:8080 ORDERAPP_BASIC_USER=... ORDERAPP_BASIC_PASS=... go test -tags=system ./systemtest -v
func TestCustomerLogoUploadAndFetch(t *testing.T) {
	base := os.Getenv("ORDERAPP_BASE_URL")
	if base == "" {
		t.Skip("ORDERAPP_BASE_URL not set")
	}
	user := os.Getenv("ORDERAPP_BASIC_USER")
	pass := os.Getenv("ORDERAPP_BASIC_PASS")
	custID := os.Getenv("ORDERAPP_TEST_CUSTOMER_ID")
	if custID == "" {
		t.Skip("ORDERAPP_TEST_CUSTOMER_ID not set")
	}

	// upload
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	_ = mw.Close()

	u := fmt.Sprintf("%s/customers/%s/assets/logo", base, custID)
	req, _ := http.NewRequest(http.MethodPost, u, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("unexpected upload status: %d", resp.StatusCode)
	}

	// fetch
	u2 := fmt.Sprintf("%s/assets/customers/%s/logo", base, custID)
	req2, _ := http.NewRequest(http.MethodGet, u2, nil)
	if user != "" || pass != "" {
		req2.SetBasicAuth(user, pass)
	}
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("unexpected fetch status: %d", resp2.StatusCode)
	}
	ct := resp2.Header.Get("Content-Type")
	if ct == "" {
		t.Fatalf("expected content-type")
	}
}
