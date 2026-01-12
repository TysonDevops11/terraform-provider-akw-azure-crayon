// Copyright (c) 2024
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"fmt"
	"io"
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
// Uses fire-and-forget approach: returns immediately when API accepts the request (202)
// The subscription will be created asynchronously by Azure/Crayon
func (c *Client) CreateAzureSubscription(azurePlanID int, name string) (*AzureSubscription, error) {
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

	// 202 Accepted means the request was accepted but subscription creation is async
	if err == ErrAccepted {
		fmt.Printf("[INFO] Subscription creation request accepted (HTTP 202). The subscription '%s' is being provisioned.\n", name)
		
		// Always try to poll Azure directly (uses SP if configured, falls back to CLI)
		fmt.Printf("[INFO] Polling Azure ARM to confirm subscription creation...\n")
		guid, pollErr := c.WaitForAzureSubscription(name, 20*time.Minute)
		if pollErr == nil {
			// Found in Azure!
			fmt.Printf("[INFO] Successfully confirmed subscription creation in Azure. GUID: %s\n", guid)
			return &AzureSubscription{
				ID:             0,          // Still unknown until synced to Crayon
				FriendlyName:   name,
				SubscriptionID: guid,       // Real Azure GUID
				Status:         "active",   // Valid in Azure
				AzurePlanID:    azurePlanID,
			}, nil
		}
		
		fmt.Printf("[WARN] Failed to confirm subscription in Azure: %v. Falling back to pending state.\n", pollErr)
		fmt.Printf("[INFO] Note: It may take several minutes for the subscription to appear in Cloud-iQ after Azure provisions it.\n")
		fmt.Printf("[INFO] You can click 'Synchronize' in Cloud-iQ portal or run 'terraform refresh' later to update the state.\n")
		return &AzureSubscription{
			ID:             0,              // Will be populated after sync
			FriendlyName:   name,
			SubscriptionID: "pending",      // Azure GUID not yet available
			Status:         "provisioning", // Indicate it's being created
			AzurePlanID:    azurePlanID,
		}, nil
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

// AzureARMSubscription represents a subscription from Azure ARM API
type AzureARMSubscription struct {
	SubscriptionID string `json:"subscriptionId"`
	DisplayName    string `json:"displayName"`
	State          string `json:"state"`
}

type AzureARMSubscriptionList struct {
	Value []AzureARMSubscription `json:"value"`
}

// WaitForAzureSubscription polls Azure ARM for a subscription with the given name
// Returns the Azure Subscription GUID if found
func (c *Client) WaitForAzureSubscription(name string, timeout time.Duration) (string, error) {
	token, err := c.getAzureToken()
	if err != nil {
		return "", err
	}

	fmt.Printf("[INFO] Polling Azure ARM for subscription '%s' (timeout: %v)...\n", name, timeout)
	
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(30 * time.Second) // Poll Azure every 30s to avoid rate limits
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCh:
			return "", fmt.Errorf("timeout waiting for subscription '%s' to appear in Azure", name)
		case <-ticker.C:
			// List subscriptions: GET https://management.azure.com/subscriptions?api-version=2022-12-01
			req, err := http.NewRequest("GET", "https://management.azure.com/subscriptions?api-version=2022-12-01", nil)
			if err != nil {
				return "", err
			}
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := c.httpClient.Do(req)
			if err != nil {
				fmt.Printf("[WARN] Failed to list Azure subscriptions: %v\n", err)
				continue
			}
			
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			if resp.StatusCode != 200 {
				fmt.Printf("[WARN] Azure API returned status %d: %s\n", resp.StatusCode, string(body))
				continue
			}

			var list AzureARMSubscriptionList
			if err := json.Unmarshal(body, &list); err != nil {
				continue
			}

			for _, sub := range list.Value {
				if sub.DisplayName == name {
					fmt.Printf("[INFO] Found subscription in Azure! GUID: %s\n", sub.SubscriptionID)
					return sub.SubscriptionID, nil
				}
			}
		}
	}
}
// FindAzureSubscriptionByName searches for a subscription by name in an Azure Plan
// Returns the subscription if found, or an error if not found
func (c *Client) FindAzureSubscriptionByName(azurePlanID int, name string) (*AzureSubscription, error) {
	subs, err := c.GetAzureSubscriptions(azurePlanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions: %w", err)
	}

	for _, sub := range subs {
		if sub.FriendlyName == name {
			return &sub, nil
		}
	}

	return nil, fmt.Errorf("subscription '%s' not found in Azure Plan %d", name, azurePlanID)
}
