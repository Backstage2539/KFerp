package sales

import (
	"context"
	"fmt"
)

// EmployeeEntryReader provides independent version probes and data loads for the mini program.
type EmployeeEntryReader interface {
	EmployeeEntryRevision(context.Context, string, int64) (string, error)
	EmployeeEntryData(context.Context, string, int64) (OrderFormData, error)
}

func (s *Service) EmployeeEntryRevision(ctx context.Context, part string, customerID int64) (string, error) {
	repo, ok := s.repo.(EmployeeEntryReader)
	if !ok {
		return "", fmt.Errorf("employee entry reader unavailable")
	}
	return repo.EmployeeEntryRevision(ctx, part, customerID)
}
func (s *Service) EmployeeEntryData(ctx context.Context, part string, customerID int64) (OrderFormData, error) {
	repo, ok := s.repo.(EmployeeEntryReader)
	if !ok {
		return OrderFormData{}, fmt.Errorf("employee entry reader unavailable")
	}
	return repo.EmployeeEntryData(ctx, part, customerID)
}
