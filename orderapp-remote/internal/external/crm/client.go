package crm

import (
	"context"
	"net/http"
	"time"
)

// Client scaffolding for integrating external CRM/ERP systems.
// Keep it dependency-free; inject http.Client for tests.

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (c Client) http() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

type CustomerDTO struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
}

func (c Client) Ping(ctx context.Context) error {
	_ = ctx
	// TODO: implement actual endpoint contract once external API is defined.
	return nil
}
