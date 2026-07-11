package company

import (
	"errors"
	"testing"

	companyapp "orderapp/internal/application/company"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapEmployeeWriteErrorMapsPhoneUniqueViolation(t *testing.T) {
	err := mapEmployeeWriteError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "company_employees_phone_uq",
	})
	if !errors.Is(err, companyapp.ErrEmployeePhoneAlreadyUsed) {
		t.Fatalf("mapEmployeeWriteError() error = %v", err)
	}
}

func TestMapEmployeeWriteErrorPreservesOtherErrors(t *testing.T) {
	want := errors.New("write failed")
	if got := mapEmployeeWriteError(want); !errors.Is(got, want) {
		t.Fatalf("mapEmployeeWriteError() = %v, want %v", got, want)
	}
}
