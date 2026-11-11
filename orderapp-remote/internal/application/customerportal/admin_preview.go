package customerportal

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"
)

const previewPrefix = "admin-preview."
const previewTTL = 30 * time.Minute
const maxPreviewSessions = 5000

var ErrAdminPreviewDenied = errors.New("管理员只读预览不可用")

type AdminPreviewSession struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
type previewSession struct {
	employeeToken          string
	employeeID, customerID int64
	expiresAt              time.Time
	securityVersion        string
}
type previewSessions struct {
	mu               sync.Mutex
	entries          map[string]previewSession
	revokedEmployees map[string]struct{}
}
type previewRepository interface {
	AdminPreviewCustomer(context.Context, int64) (CurrentContext, error)
	AdminPreviewAudit(context.Context, int64, int64, string) error
	AdminPreviewSecurityVersion(context.Context, int64, int64) (string, error)
}

func IsAdminPreviewToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), previewPrefix)
}
func isPreviewAdmin(c CurrentContext) bool {
	if c.AccountType != "employee" || c.EmployeeID <= 0 {
		return false
	}
	for _, role := range c.Roles {
		if role == "admin" {
			return true
		}
	}
	return false
}

// StartAdminPreview never exposes the employee token; server memory is the
// authority. Restart or expiry invalidates all temporary preview credentials.
func (s *Service) StartAdminPreview(ctx context.Context, employeeToken string, customerID int64) (AdminPreviewSession, error) {
	if IsAdminPreviewToken(employeeToken) || customerID <= 0 || s.repo == nil || s.employeeSessionRevoked(employeeToken) {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	admin, err := s.repo.CurrentContextByToken(ctx, employeeToken)
	if err != nil || !isPreviewAdmin(admin) {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	repo, ok := s.repo.(previewRepository)
	if !ok {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	if _, err = repo.AdminPreviewCustomer(ctx, customerID); err != nil {
		return AdminPreviewSession{}, err
	}
	securityVersion, err := repo.AdminPreviewSecurityVersion(ctx, admin.EmployeeID, customerID)
	if err != nil || securityVersion == "" {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	var random [32]byte
	if _, err = rand.Read(random[:]); err != nil {
		return AdminPreviewSession{}, err
	}
	token := previewPrefix + base64.RawURLEncoding.EncodeToString(random[:])
	now := time.Now()
	expires := now.Add(previewTTL)
	s.previews.mu.Lock()
	defer s.previews.mu.Unlock()
	if _, revoked := s.previews.revokedEmployees[employeeToken]; revoked {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	if s.previews.entries == nil {
		s.previews.entries = make(map[string]previewSession)
	}
	for key, session := range s.previews.entries {
		if !now.Before(session.expiresAt) {
			delete(s.previews.entries, key)
		}
	}
	if len(s.previews.entries) >= maxPreviewSessions {
		return AdminPreviewSession{}, ErrAdminPreviewDenied
	}
	if err = repo.AdminPreviewAudit(ctx, admin.EmployeeID, customerID, "enter"); err != nil {
		return AdminPreviewSession{}, err
	}
	s.previews.entries[token] = previewSession{employeeToken: employeeToken, employeeID: admin.EmployeeID, customerID: customerID, expiresAt: expires, securityVersion: securityVersion}
	return AdminPreviewSession{Token: token, ExpiresAt: expires}, nil
}

func (s *Service) lookupPreview(token string) (previewSession, bool) {
	s.previews.mu.Lock()
	defer s.previews.mu.Unlock()
	p, ok := s.previews.entries[token]
	if !ok || !time.Now().Before(p.expiresAt) {
		delete(s.previews.entries, token)
		return previewSession{}, false
	}
	return p, true
}
func (s *Service) removePreview(token string) {
	s.previews.mu.Lock()
	defer s.previews.mu.Unlock()
	delete(s.previews.entries, token)
}

func (s *Service) employeeSessionRevoked(token string) bool {
	s.previews.mu.Lock()
	defer s.previews.mu.Unlock()
	_, revoked := s.previews.revokedEmployees[token]
	return revoked
}

func (s *Service) revokeEmployeePreviews(token string, requireKnown bool) {
	s.previews.mu.Lock()
	defer s.previews.mu.Unlock()
	found := false
	for key, entry := range s.previews.entries {
		if entry.employeeToken == token {
			delete(s.previews.entries, key)
			found = true
		}
	}
	if !requireKnown || found {
		if s.previews.revokedEmployees == nil {
			s.previews.revokedEmployees = make(map[string]struct{})
		}
		s.previews.revokedEmployees[token] = struct{}{}
	}
}

type previewEmployeeLogoutRepository interface {
	LogoutPreviewEmployee(context.Context, string, int64) error
}

// LogoutPreviewEmployee invalidates all previews derived from this employee
// session before attempting the durable session and audit transaction. If the
// transaction fails, this process continues denying that employee token.
func (s *Service) LogoutPreviewEmployee(ctx context.Context, token string) error {
	if s.repo == nil || token == "" || IsAdminPreviewToken(token) || s.employeeSessionRevoked(token) {
		return ErrAdminPreviewDenied
	}
	current, err := s.repo.CurrentContextByToken(ctx, token)
	if err != nil || current.AccountType != "employee" || current.EmployeeID <= 0 {
		s.revokeEmployeePreviews(token, true)
		return ErrAdminPreviewDenied
	}
	repo, ok := s.repo.(previewEmployeeLogoutRepository)
	if !ok {
		s.revokeEmployeePreviews(token, false)
		return ErrAdminPreviewDenied
	}
	s.revokeEmployeePreviews(token, false)
	if err = repo.LogoutPreviewEmployee(ctx, token, current.EmployeeID); err != nil {
		return err
	}
	s.previews.mu.Lock()
	delete(s.previews.revokedEmployees, token)
	s.previews.mu.Unlock()
	return nil
}

func (s *Service) previewContext(ctx context.Context, token string) (CurrentContext, error) {
	p, ok := s.lookupPreview(token)
	if !ok {
		return CurrentContext{}, ErrMiniSessionNotFound
	}
	if s.employeeSessionRevoked(p.employeeToken) {
		s.removePreview(token)
		return CurrentContext{}, ErrAdminPreviewDenied
	}
	admin, err := s.repo.CurrentContextByToken(ctx, p.employeeToken)
	if err != nil || !isPreviewAdmin(admin) || admin.EmployeeID != p.employeeID {
		s.removePreview(token)
		return CurrentContext{}, ErrAdminPreviewDenied
	}
	if s.employeeSessionRevoked(p.employeeToken) {
		s.removePreview(token)
		return CurrentContext{}, ErrAdminPreviewDenied
	}
	repo, ok := s.repo.(previewRepository)
	if !ok {
		return CurrentContext{}, ErrAdminPreviewDenied
	}
	current, err := repo.AdminPreviewCustomer(ctx, p.customerID)
	if err != nil {
		s.removePreview(token)
		return CurrentContext{}, err
	}
	if current.CurrentCustomerID != p.customerID {
		s.removePreview(token)
		return CurrentContext{}, ErrAdminPreviewDenied
	}
	version, err := repo.AdminPreviewSecurityVersion(ctx, p.employeeID, p.customerID)
	if err != nil || version == "" || version != p.securityVersion {
		s.removePreview(token)
		return CurrentContext{}, ErrAdminPreviewDenied
	}
	current.AccountType = "preview"
	current.EmployeeID = 0
	current.EmployeeName = ""
	current.Roles = nil
	current.Permissions = nil
	current.Bindings = nil
	current.MiniUserID = 0
	current.RegistrationComplete = true
	if modes, ok := s.repo.(interface {
		FulfillmentModes(context.Context, int64) ([]string, error)
	}); ok {
		current.FulfillmentModes, err = modes.FulfillmentModes(ctx, p.customerID)
		if err != nil {
			return CurrentContext{}, err
		}
		current.Capabilities = ApplyFulfillmentCapabilityDependencies(current.Capabilities, current.FulfillmentModes)
	}
	return normalizeCurrentContext(current), nil
}

func (s *Service) EndAdminPreview(ctx context.Context, token string) error {
	p, ok := s.lookupPreview(token)
	if !ok {
		return ErrMiniSessionNotFound
	}
	admin, err := s.repo.CurrentContextByToken(ctx, p.employeeToken)
	if err != nil || !isPreviewAdmin(admin) || admin.EmployeeID != p.employeeID {
		s.removePreview(token)
		return ErrAdminPreviewDenied
	}
	repo, ok := s.repo.(previewRepository)
	if !ok {
		return ErrAdminPreviewDenied
	}
	if err = repo.AdminPreviewAudit(ctx, p.employeeID, p.customerID, "exit"); err != nil {
		s.removePreview(token)
		return err
	}
	s.removePreview(token)
	return nil
}
