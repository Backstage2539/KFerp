package customer

import (
	"errors"
	"strings"
)

type ID int64

// Customer is the CRM aggregate root.
// Keep it small: behavior/validation in domain, persistence in repo.
type Customer struct {
	ID      ID
	Name    string
	RawName string
	Contact string
	Phone   string
	Address string
	Active  bool

	DefaultSourceID    *int
	DefaultOrderTypeID *int
}

var (
	ErrNameRequired = errors.New("customer name required")
)

func (c *Customer) Normalize() {
	c.Name = strings.TrimSpace(c.Name)
	c.RawName = strings.TrimSpace(c.RawName)
	c.Contact = strings.TrimSpace(c.Contact)
	c.Phone = strings.TrimSpace(c.Phone)
	c.Address = strings.TrimSpace(c.Address)
	if c.RawName == "" {
		c.RawName = c.Name
	}
}

func (c Customer) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return ErrNameRequired
	}
	return nil
}
