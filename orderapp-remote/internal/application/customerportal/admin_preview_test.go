package customerportal

import (
	"context"
	"errors"
	"testing"
	"time"
)

type previewRepo struct {
	fakeRepository
	admin           bool
	auditErr        error
	events          []string
	logoutErr       error
	loggedOut       bool
	securityVersion string
	contextErr      error
}

func (r *previewRepo) AdminPreviewSecurityVersion(context.Context, int64, int64) (string, error) {
	if r.securityVersion == "" {
		return "v1", nil
	}
	return r.securityVersion, nil
}

func (r *previewRepo) CurrentContextByToken(_ context.Context, token string) (CurrentContext, error) {
	if r.contextErr != nil {
		return CurrentContext{}, r.contextErr
	}
	if token == "customer-token" {
		return CurrentContext{AccountType: "customer", CurrentCustomerID: 99}, nil
	}
	if token == "other-employee-token" {
		return CurrentContext{AccountType: "employee", EmployeeID: 16, Roles: []string{"admin"}}, nil
	}
	if r.loggedOut && token == "employee-token" {
		return CurrentContext{}, ErrMiniSessionNotFound
	}
	if token != "employee-token" {
		return CurrentContext{}, ErrMiniSessionNotFound
	}
	roles := []string{"sales"}
	if r.admin {
		roles = []string{"admin"}
	}
	return CurrentContext{AccountType: "employee", EmployeeID: 15, Roles: roles}, nil
}
func (r *previewRepo) LogoutPreviewEmployee(_ context.Context, token string, employeeID int64) error {
	if token != "employee-token" || employeeID != 15 {
		return ErrAdminPreviewDenied
	}
	if r.logoutErr != nil {
		return r.logoutErr
	}
	r.loggedOut = true
	return nil
}
func (r *previewRepo) AdminPreviewCustomer(_ context.Context, id int64) (CurrentContext, error) {
	if id != 74 {
		return CurrentContext{}, errors.New("customer not found")
	}
	return CurrentContext{CurrentCustomerID: id, CurrentCustomerName: "customer", RegistrationComplete: true, Capabilities: []Capability{{Code: CapabilityProductOrder, Enabled: true}}}, nil
}
func (r *previewRepo) AdminPreviewAudit(_ context.Context, employeeID, customerID int64, action string) error {
	if r.auditErr != nil {
		return r.auditErr
	}
	if employeeID != 15 || customerID != 74 {
		return errors.New("wrong audit scope")
	}
	r.events = append(r.events, action)
	return nil
}

func TestAdminPreviewScopedRevalidatedAndAudited(t *testing.T) {
	ctx := context.Background()
	repo := &previewRepo{admin: true}
	svc := NewService(repo, nil)
	issued, err := svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil || issued.Token == "" || issued.Token == "employee-token" {
		t.Fatalf("start: %+v %v", issued, err)
	}
	current, err := svc.Me(ctx, issued.Token)
	if err != nil || current.AccountType != "preview" || current.CurrentCustomerID != 74 || current.EmployeeID != 0 || len(current.Roles) != 0 {
		t.Fatalf("scope: %+v %v", current, err)
	}
	repo.admin = false
	if _, err = svc.Me(ctx, issued.Token); err == nil {
		t.Fatal("revoked admin still has preview access")
	}
	repo.admin = true
	issued, err = svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.EndAdminPreview(ctx, issued.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Me(ctx, issued.Token); err == nil {
		t.Fatal("closed preview still usable")
	}
	if len(repo.events) != 3 || repo.events[0] != "enter" || repo.events[1] != "enter" || repo.events[2] != "exit" {
		t.Fatalf("audit: %v", repo.events)
	}
}

func TestAdminPreviewRejectsNonAdminAndAuditFailure(t *testing.T) {
	ctx := context.Background()
	repo := &previewRepo{}
	svc := NewService(repo, nil)
	if _, err := svc.StartAdminPreview(ctx, "employee-token", 74); err == nil {
		t.Fatal("non-admin accepted")
	}
	repo.admin = true
	repo.auditErr = errors.New("audit failed")
	if issued, err := svc.StartAdminPreview(ctx, "employee-token", 74); err == nil || issued.Token != "" {
		t.Fatal("issued token despite audit failure")
	}
}

func TestAdminPreviewExpiryAndCustomerScope(t *testing.T) {
	ctx := context.Background()
	repo := &previewRepo{admin: true}
	svc := NewService(repo, nil)
	issued, err := svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.GetServicePage(ctx, issued.Token, "orders", ServicePageFilter{}); err != nil {
		t.Fatal(err)
	}
	if repo.serviceQuery.CustomerID != 74 {
		t.Fatalf("incorrect customer scope: %d", repo.serviceQuery.CustomerID)
	}
	svc.previews.mu.Lock()
	p := svc.previews.entries[issued.Token]
	p.expiresAt = time.Now().Add(-time.Second)
	svc.previews.entries[issued.Token] = p
	svc.previews.mu.Unlock()
	if _, err = svc.Me(ctx, issued.Token); err == nil {
		t.Fatal("expired preview token accepted")
	}
}

func TestAdminPreviewExitAuditFailureStillRevokes(t *testing.T) {
	ctx := context.Background()
	repo := &previewRepo{admin: true}
	svc := NewService(repo, nil)
	issued, err := svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil {
		t.Fatal(err)
	}
	repo.auditErr = errors.New("audit unavailable")
	if err = svc.EndAdminPreview(ctx, issued.Token); err == nil {
		t.Fatal("audit failure was hidden")
	}
	if _, err = svc.Me(ctx, issued.Token); err == nil {
		t.Fatal("failed exit left preview token usable")
	}
}

func TestEmployeeLogoutRevokesAllDerivedPreviews(t *testing.T) {
	ctx := context.Background()
	repo := &previewRepo{admin: true}
	svc := NewService(repo, nil)
	a, err := svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.LogoutPreviewEmployee(ctx, "employee-token"); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{a.Token, b.Token, "employee-token"} {
		if _, err = svc.Me(ctx, token); err == nil {
			t.Fatalf("token still valid after logout")
		}
	}
	if _, err = svc.Me(ctx, "other-employee-token"); err != nil {
		t.Fatal("different employee affected")
	}
	if err = svc.LogoutPreviewEmployee(ctx, "customer-token"); err == nil {
		t.Fatal("customer session accepted by employee logout")
	}
	if _, err = svc.Me(ctx, "customer-token"); err != nil {
		t.Fatal("customer session affected")
	}
}

func TestEmployeeLogoutStoreFailureStillBlocksCurrentInstance(t *testing.T) {
	ctx := context.Background()
	repo := &previewRepo{admin: true}
	svc := NewService(repo, nil)
	issued, err := svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil {
		t.Fatal(err)
	}
	repo.logoutErr = errors.New("database failure")
	if err = svc.LogoutPreviewEmployee(ctx, "employee-token"); err == nil {
		t.Fatal("logout failure hidden")
	}
	for _, token := range []string{issued.Token, "employee-token"} {
		if _, err = svc.Me(ctx, token); err == nil {
			t.Fatal("credential remained valid after failed logout")
		}
	}
	if _, err = svc.StartAdminPreview(ctx, "employee-token", 74); err == nil {
		t.Fatal("logout credential issued new preview")
	}
}

func TestEmployeeLogoutAuthenticationStoreFailureRevokesKnownPreview(t *testing.T) {
	ctx := context.Background()
	repo := &previewRepo{admin: true}
	svc := NewService(repo, nil)
	issued, err := svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil {
		t.Fatal(err)
	}
	repo.contextErr = errors.New("db offline")
	if err = svc.LogoutPreviewEmployee(ctx, "employee-token"); err == nil {
		t.Fatal("unconfirmed logout reported success")
	}
	repo.contextErr = nil
	if _, err = svc.Me(ctx, issued.Token); err == nil {
		t.Fatal("preview survived logout with unavailable authentication store")
	}
	if _, err = svc.Me(ctx, "employee-token"); err == nil {
		t.Fatal("employee token survived failed logout on this instance")
	}
}

func TestAdminPreviewSecurityVersionCannotBeRestored(t *testing.T) {
	ctx := context.Background()
	repo := &previewRepo{admin: true}
	svc := NewService(repo, nil)
	issued, err := svc.StartAdminPreview(ctx, "employee-token", 74)
	if err != nil {
		t.Fatal(err)
	}
	repo.securityVersion = "v2"
	if _, err = svc.Me(ctx, issued.Token); err == nil {
		t.Fatal("changed security version allowed preview")
	}
}
