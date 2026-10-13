package customerportal

import (
	"context"
	"testing"
)

func TestRegistrationAcceptsNewVisitorAndRejectsUnverifiedInput(t *testing.T) {
	repo := &fakeRepository{phoneVerifiedLoginResult: LoginResult{RegistrationComplete: true}}
	svc := NewService(repo, fakeIdentityProvider{identity: MiniIdentity{OpenID: "new-visitor"}, phone: MiniPhoneNumber{PhoneNumber: "13800000001"}})
	result, err := svc.Login(context.Background(), LoginCommand{Mode: "register", Code: "wx-code", PhoneCode: "verified-code", Nickname: "访客"})
	if err != nil {
		t.Fatalf("new visitor registration: %v", err)
	}
	if !result.RegistrationComplete || !repo.phoneVerifiedCommand.Registration || repo.phoneVerifiedCommand.Phone != "13800000001" || len(repo.phoneVerifiedCommand.CredentialHash) != 64 {
		t.Fatal("verified registration contract", result, repo.phoneVerifiedCommand)
	}
	_, err = svc.Login(context.Background(), LoginCommand{Mode: "register", Code: "wx-code", Phone: "13800000001", Nickname: "访客"})
	if err == nil {
		t.Fatal("self declared phone accepted")
	}
}

func TestRegistrationRepositoryFailureIsNotSwallowed(t *testing.T) {
	repo := &fakeRepository{phoneVerifiedErr: ErrPhoneCredentialUsed}
	svc := NewService(repo, fakeIdentityProvider{identity: MiniIdentity{OpenID: "visitor"}, phone: MiniPhoneNumber{PhoneNumber: "13800000001"}})
	_, err := svc.Login(context.Background(), LoginCommand{Mode: "register", Code: "wx", PhoneCode: "duplicate", Nickname: "访客"})
	if err != ErrPhoneCredentialUsed {
		t.Fatalf("repository error swallowed: %v", err)
	}
}
