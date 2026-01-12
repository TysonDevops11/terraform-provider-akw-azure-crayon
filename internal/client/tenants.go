// Copyright (c) 2024
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"fmt"
	"net/http"
)

// CustomerTenant represents a Crayon customer tenant
type CustomerTenant struct {
	ID     int    `json:"id"`
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

// CustomerTenantsResponse represents the response from CustomerTenants API
type CustomerTenantsResponse struct {
	Items      []CustomerTenant `json:"items"`
	TotalCount int              `json:"totalCount"`
}

// AzurePlan represents a Crayon Azure Plan
type AzurePlan struct {
	ID               int    `json:"id"`
	CustomerTenantID int    `json:"customerTenantId"`
	SubscriptionID   string `json:"subscriptionId"`
}

// GetCustomerTenants retrieves customer tenants for the organization
func (c *Client) GetCustomerTenants() ([]CustomerTenant, error) {
	path := fmt.Sprintf("/api/v1/CustomerTenants?OrganizationId=%d", c.config.OrganizationID)

	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result CustomerTenantsResponse
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

// GetAzurePlan retrieves the Azure Plan for a customer tenant
func (c *Client) GetAzurePlan(customerTenantID int) (*AzurePlan, error) {
	path := fmt.Sprintf("/api/v1/CustomerTenants/%d/azureplan", customerTenantID)

	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result AzurePlan
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
