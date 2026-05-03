package customerportal

import (
	"context"
	"errors"
	"testing"
)

type fakeIdentityProvider struct {
	identity MiniIdentity
	err      error
}

func (p fakeIdentityProvider) Resolve(ctx context.Context, code string) (MiniIdentity, error) {
	if p.err != nil {
		return MiniIdentity{}, p.err
	}
	return p.identity, nil
}

type fakeRepository struct {
	loginResult  LoginResult
	loginCommand CreateLoginSessionCommand
	context      CurrentContext
	session      string
	err          error
	switchErr    error
}

func (r *fakeRepository) CreateLoginSession(ctx context.Context, cmd CreateLoginSessionCommand) (LoginResult, error) {
	r.loginCommand = cmd
	if r.err != nil {
		return LoginResult{}, r.err
	}
	return r.loginResult, nil
}

func (r *fakeRepository) CurrentContextByToken(ctx context.Context, token string) (CurrentContext, error) {
	r.session = token
	if r.err != nil {
		return CurrentContext{}, r.err
	}
	return r.context, nil
}

func (r *fakeRepository) SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error) {
	r.session = token
	if r.switchErr != nil {
		return CurrentContext{}, r.switchErr
	}
	r.context.CurrentCustomerID = customerID
	return r.context, nil
}

func TestLoginRejectsEmptyCode(t *testing.T) {
	svc := NewService(&fakeRepository{}, fakeIdentityProvider{})
	_, err := svc.Login(context.Background(), LoginCommand{})
	if err == nil || err.Error() != "code required" {
		t.Fatalf("Login() err=%v, want code required", err)
	}
}

func TestLoginCreatesSessionFromResolvedIdentity(t *testing.T) {
	repo := &fakeRepository{loginResult: LoginResult{Token: "mini-token", MiniUserID: 9}}
	svc := NewService(repo, fakeIdentityProvider{identity: MiniIdentity{OpenID: " openid-1 ", UnionID: " union-1 "}})
	got, err := svc.Login(context.Background(), LoginCommand{Code: "wx-code", Phone: " 13800138000 ", Nickname: " 客户 "})
	if err != nil {
		t.Fatalf("Login() err=%v", err)
	}
	if got.Token != "mini-token" || got.MiniUserID != 9 {
		t.Fatalf("Login()=%+v", got)
	}
	if repo.loginCommand.OpenID != "openid-1" || repo.loginCommand.UnionID != "union-1" || repo.loginCommand.Phone != "13800138000" || repo.loginCommand.Nickname != "客户" {
		t.Fatalf("CreateLoginSession() cmd=%+v", repo.loginCommand)
	}
}

func TestMeRequiresTokenAndReturnsBoundCapabilities(t *testing.T) {
	repo := &fakeRepository{context: CurrentContext{
		MiniUserID:          8,
		CurrentCustomerID:   7,
		CurrentCustomerName: "品牌客户",
		Bindings:            []CustomerBinding{{CustomerID: 7, CustomerName: "品牌客户", Role: "owner", Status: "approved"}},
		Capabilities:        []Capability{{Code: CapabilityDirectShip, Enabled: true}, {Code: CapabilitySettlement, Enabled: true}},
	}}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.Me(context.Background(), "mini-token")
	if err != nil {
		t.Fatalf("Me() err=%v", err)
	}
	if got.CurrentCustomerID != 7 || !got.HasCapability(CapabilityDirectShip) || !got.HasCapability(CapabilitySettlement) {
		t.Fatalf("Me()=%+v", got)
	}
}

func TestSwitchCustomerRejectsUnauthorizedBinding(t *testing.T) {
	repo := &fakeRepository{switchErr: ErrCustomerBindingNotFound}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.SwitchCurrentCustomer(context.Background(), "mini-token", 99)
	if !errors.Is(err, ErrCustomerBindingNotFound) {
		t.Fatalf("SwitchCurrentCustomer() err=%v", err)
	}
}
