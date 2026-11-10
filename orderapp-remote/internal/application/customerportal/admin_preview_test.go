package customerportal

import (
	"context"
	"errors"
	"testing"
	"time"
)

type previewRepo struct {
	fakeRepository
	admin    bool
	auditErr error
	events   []string
}

func (r *previewRepo) CurrentContextByToken(_ context.Context, token string) (CurrentContext, error) {
	if token != "employee-token" {
		return CurrentContext{}, ErrMiniSessionNotFound
	}
	roles := []string{"sales"}
	if r.admin {
		roles = []string{"admin"}
	}
	return CurrentContext{AccountType: "employee", EmployeeID: 15, Roles: roles}, nil
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
