package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestMobileAuthRequiresExistingActiveEmployee(t *testing.T) {
	srcBytes, err := os.ReadFile("mobile_auth.go")
	if err != nil {
		t.Fatalf("read mobile_auth.go: %v", err)
	}
	src := string(srcBytes)

	if strings.Contains(src, "ensureEmployeeByPhone") {
		t.Fatalf("mobile auth must not use auto-provisioning employee helper")
	}
	if strings.Contains(src, "INSERT INTO "+`"+schema+"`+".company_employees") {
		t.Fatalf("mobile auth must not insert or reactivate employees from unauthenticated auth routes")
	}
	if !strings.Contains(src, "WHERE active=true AND phone=$1") {
		t.Fatalf("mobile auth employee lookup must require active=true")
	}

	re := regexp.MustCompile(`requireActiveEmployeeByPhone\(c\.Request\(\)\.Context\(\), pool, schema, phone\)`)
	if got := len(re.FindAllString(src, -1)); got != 3 {
		t.Fatalf("password, SMS, and login routes must require an existing active employee; got %d calls", got)
	}
}

func TestSMSSendDoesNotEchoLoginCode(t *testing.T) {
	srcBytes, err := os.ReadFile("mobile_auth.go")
	if err != nil {
		t.Fatalf("read mobile_auth.go: %v", err)
	}
	src := string(srcBytes)

	if strings.Contains(src, `"code": code`) {
		t.Fatalf("SMS send response must not echo login code")
	}
	if !strings.Contains(src, `map[string]any{"ok": true, "expire_minutes": 5}`) {
		t.Fatalf("SMS send should still return a success response with expiry metadata")
	}
}
