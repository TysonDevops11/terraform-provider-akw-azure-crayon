// Copyright (c) 2024
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"fmt"
	"net/http"
	"time"
)

// AzureSubscription represents a Crayon Azure Subscription
type AzureSubscription struct {
	ID             int    `json:"Id"`
	FriendlyName   string `json:"FriendlyName"`
	SubscriptionID string `json:"PublisherSubscriptionId"`
	Status         string `json:"Status"`
	AzurePlanID    int    `json:"AzurePlanId"`
}

// AzureSubscriptionsResponse represents the list response
type AzureSubscriptionsResponse struct {
	Items      []AzureSubscription `json:"Items"`
	TotalCount int                 `json:"TotalHits"`
}

// CreateAzureSubscriptionRequest represents the request to create a subscription
type CreateAzureSubscriptionRequest struct {
	Name string `json:"name"`
}

// GetAzureSubscriptions retrieves all Azure subscriptions for an Azure Plan
func (c *Client) GetAzureSubscriptions(azurePlanID int) ([]AzureSubscription, error) {
	path := fmt.Sprintf("/api/v1/azureplans/%d/azuresubscriptions?pageSize=1000", azurePlanID)

	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	// Crayon API returns wrapped format {"Items": [...], "TotalHits": N}
	var wrapped AzureSubscriptionsResponse
	if err := unmarshalResponse(body, resp.StatusCode, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Items, nil
}

// GetAzureSubscription retrieves a single Azure subscription by ID
func (c *Client) GetAzureSubscription(azurePlanID, subscriptionID int) (*AzureSubscription, error) {
	path := fmt.Sprintf("/api/v1/azureplans/%d/azuresubscriptions/%d", azurePlanID, subscriptionID)

	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result AzureSubscription
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CreateAzureSubscription creates a new Azure subscription under an Azure Plan
func (c *Client) CreateAzureSubscription(azurePlanID int, name string, timeout time.Duration) (*AzureSubscription, error) {
	path := fmt.Sprintf("/api/v1/azureplans/%d/azuresubscriptions", azurePlanID)

	reqBody := CreateAzureSubscriptionRequest{
		Name: name,
	}

	resp, err := c.doRequest(http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var result AzureSubscription
	err = parseResponse(resp, &result)

	if err == ErrAccepted {
		return c.pollForSubscription(azurePlanID, name, timeout)
	}

	if err != nil {
		return nil, err
	}

	return &result, nil
}

// RenameAzureSubscription renames an Azure subscription
func (c *Client) RenameAzureSubscription(azurePlanID, subscriptionID int, newName string) (*AzureSubscription, error) {
	path := fmt.Sprintf("/api/v1/azureplans/%d/azuresubscriptions/%d/rename", azurePlanID, subscriptionID)

	reqBody := map[string]string{
		"name": newName,
	}

	resp, err := c.doRequest(http.MethodPatch, path, reqBody)
	if err != nil {
		return nil, err
	}

	var result AzureSubscription
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CancelAzureSubscription cancels an Azure subscription
func (c *Client) CancelAzureSubscription(azurePlanID, subscriptionID int) error {
	path := fmt.Sprintf("/api/v1/azureplans/%d/azuresubscriptions/%d/cancel", azurePlanID, subscriptionID)

	resp, err := c.doRequest(http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cancel request failed with status %d", resp.StatusCode)
	}

	return nil
}

// EnableAzureSubscription enables a cancelled Azure subscription
func (c *Client) EnableAzureSubscription(azurePlanID, subscriptionID int) error {
	path := fmt.Sprintf("/api/v1/azureplans/%d/azuresubscriptions/%d/enable", azurePlanID, subscriptionID)

	resp, err := c.doRequest(http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("enable request failed with status %d", resp.StatusCode)
	}

	return nil
}

// pollForSubscription polls for a subscription by name with configurable timeout
func (c *Client) pollForSubscription(azurePlanID int, name string, timeout time.Duration) (*AzureSubscription, error) {
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	pollCount := 0
	fmt.Printf("[DEBUG] Starting to poll for subscription '%s' under Azure Plan %d (timeout: %v)\n", name, azurePlanID, timeout)

	for {
		select {
		case <-timeoutCh:
			return nil, fmt.Errorf("timeout waiting for subscription '%s' to be created (polled %d times)", name, pollCount)
		case <-ticker.C:
			pollCount++
			fmt.Printf("[DEBUG] Poll #%d: Checking for subscription '%s'...\n", pollCount, name)

			subs, err := c.GetAzureSubscriptions(azurePlanID)
			if err != nil {
				fmt.Printf("[DEBUG] Poll #%d: Error getting subscriptions: %v\n", pollCount, err)
				continue
			}

			fmt.Printf("[DEBUG] Poll #%d: Found %d subscriptions in Azure Plan %d:\n", pollCount, len(subs), azurePlanID)
			for i, sub := range subs {
				fmt.Printf("[DEBUG]   [%d] ID=%d, Name='%s', Status='%s', AzureGUID='%s'\n",
					i+1, sub.ID, sub.FriendlyName, sub.Status, sub.SubscriptionID)

				if sub.FriendlyName == name {
					fmt.Printf("[DEBUG] Poll #%d: MATCH FOUND! Subscription '%s' created with ID %d\n", pollCount, name, sub.ID)
					return c.GetAzureSubscription(azurePlanID, sub.ID)
				}
			}
			fmt.Printf("[DEBUG] Poll #%d: No match for '%s' yet, will retry in 15 seconds...\n", pollCount, name)
		}
	}
}
