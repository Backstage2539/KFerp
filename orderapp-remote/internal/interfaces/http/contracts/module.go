package contracts

import (
	"context"
	contractsapp "orderapp/internal/application/contracts"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Contracts ContractService
}

type ContractService interface {
	ListContracts(context.Context) ([]contractsapp.ContractDocument, error)
	UploadContract(context.Context, contractsapp.UploadContractCommand) (contractsapp.ContractDocument, error)
	UpdateContract(context.Context, contractsapp.UpdateContractCommand) (contractsapp.ContractDocument, error)
	DeleteContract(context.Context, contractsapp.DeleteContractCommand) error
	SaveStampedPDF(context.Context, contractsapp.SaveStampedPDFCommand) (contractsapp.ContractStampedVersion, error)
	LoadContractPDFFile(context.Context, int64) (contractsapp.ContractFile, error)
	LoadStampedPDFFile(context.Context, int64, int64, bool) (contractsapp.ContractFile, error)
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerContractRoutes(e, deps.Contracts)
}
