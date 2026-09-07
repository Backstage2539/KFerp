package catalog

import (
	"context"
	"strings"
)

type CopyCustomerCatalogCommand struct {
	CustomerID int64   `json:"customer_id"`
	Mode       string  `json:"mode"`
	ProductIDs []int64 `json:"product_ids"`
	Actor      string  `json:"-"`
}
type CopyCustomerCatalogResult struct {
	CustomerID int64   `json:"customer_id"`
	Created    int     `json:"created"`
	Restored   int     `json:"restored"`
	Unchanged  int     `json:"unchanged"`
	ProductIDs []int64 `json:"product_ids"`
}
type CustomerCatalogNode struct {
	ID                 int64  `json:"id"`
	CustomerID         int64  `json:"customer_id"`
	SourceGroupID      int64  `json:"source_group_id"`
	SourceItemID       int64  `json:"source_item_id"`
	ParentSourceItemID int64  `json:"parent_source_item_id"`
	Name               string `json:"name"`
	Code               string `json:"code"`
	SortOrder          int    `json:"sort_order"`
}
type CustomerCatalog struct {
	CustomerID  int64                     `json:"customer_id"`
	Nodes       []CustomerCatalogNode     `json:"nodes"`
	Assignments []BusinessGroupAssignment `json:"assignments"`
}
type RenameCustomerCatalogNodeCommand struct {
	ID         int64  `json:"id"`
	CustomerID int64  `json:"customer_id"`
	Name       string `json:"name"`
	Actor      string `json:"-"`
}
type CustomerCatalogMigrationResult struct {
	Missing      int     `json:"missing"`
	Applied      int     `json:"applied"`
	ReferenceIDs []int64 `json:"reference_ids"`
}
type CustomerCatalogRepository interface {
	CopyCustomerCatalog(context.Context, CopyCustomerCatalogCommand) (CopyCustomerCatalogResult, error)
	CustomerCatalog(context.Context, int64) (CustomerCatalog, error)
	RenameCustomerCatalogNode(context.Context, RenameCustomerCatalogNodeCommand) error
	MigrateCustomerCatalog(context.Context, bool, string) (CustomerCatalogMigrationResult, error)
}

func (s *Service) customerCatalogRepository() (CustomerCatalogRepository, error) {
	r, ok := s.repo.(CustomerCatalogRepository)
	if !ok {
		return nil, ValidationError{Message: "customer catalog unavailable"}
	}
	return r, nil
}
func (s *Service) CopyCustomerCatalog(ctx context.Context, c CopyCustomerCatalogCommand) (CopyCustomerCatalogResult, error) {
	r, e := s.customerCatalogRepository()
	if e != nil {
		return CopyCustomerCatalogResult{}, e
	}
	if c.CustomerID <= 0 || (c.Mode != "all" && c.Mode != "selected") || (c.Mode == "selected" && len(c.ProductIDs) == 0) {
		return CopyCustomerCatalogResult{}, ValidationError{Message: "请选择有效客户和复制商品范围"}
	}
	return r.CopyCustomerCatalog(ctx, c)
}
func (s *Service) CustomerCatalog(ctx context.Context, id int64) (CustomerCatalog, error) {
	r, e := s.customerCatalogRepository()
	if e != nil {
		return CustomerCatalog{}, e
	}
	if id <= 0 {
		return CustomerCatalog{}, ValidationError{Message: "请选择客户"}
	}
	return r.CustomerCatalog(ctx, id)
}
func (s *Service) RenameCustomerCatalogNode(ctx context.Context, c RenameCustomerCatalogNodeCommand) error {
	r, e := s.customerCatalogRepository()
	if e != nil {
		return e
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.ID <= 0 || c.CustomerID <= 0 || c.Name == "" {
		return ValidationError{Message: "请选择客户分类并填写名称"}
	}
	return r.RenameCustomerCatalogNode(ctx, c)
}
func (s *Service) MigrateCustomerCatalog(ctx context.Context, preview bool, actor string) (CustomerCatalogMigrationResult, error) {
	r, e := s.customerCatalogRepository()
	if e != nil {
		return CustomerCatalogMigrationResult{}, e
	}
	return r.MigrateCustomerCatalog(ctx, preview, actor)
}

func (s *Service) RemoveCustomerCatalogProducts(ctx context.Context, c CopyCustomerCatalogCommand) error {
	r, ok := s.repo.(interface {
		RemoveCustomerCatalogProducts(context.Context, CopyCustomerCatalogCommand) error
	})
	if !ok {
		return ValidationError{Message: "customer catalog unavailable"}
	}
	return r.RemoveCustomerCatalogProducts(ctx, c)
}
