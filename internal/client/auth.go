// Copyright (c) 2024
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"AccessToken"`
	TokenType    string `json:"TokenType"`
	ExpiresIn    int    `json:"ExpiresIn"`
	RefreshToken string `json:"RefreshToken,omitempty"`
	Error        string `json:"Error,omitempty"`
}

// getToken returns a valid access token, refreshing if necessary
func (c *Client) getToken() (string, error) {
	// Return cached token if still valid (with 60 second buffer)
	if c.token != "" && time.Now().Before(c.tokenExp.Add(-60*time.Second)) {
		return c.token, nil
	}

	// Determine which grant type to use
	var token *TokenResponse
	var err error

	if c.config.Username != "" && c.config.Password != "" {
		// Use Resource Owner Password Credentials flow (matches C# GetUserToken)
		token, err = c.getTokenWithPassword()
	} else {
		// Use Client Credentials flow
		token, err = c.getTokenWithClientCredentials()
	}

	if err != nil {
		return "", err
	}

	c.token = token.AccessToken
	c.tokenExp = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	return c.token, nil
}

// getTokenWithClientCredentials uses the client credentials grant type
func (c *Client) getTokenWithClientCredentials() (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("scope", "CustomerApi")

	return c.requestToken(data)
}

// getTokenWithPassword uses the resource owner password credentials grant type
// This matches the C# implementation: GetUserToken(clientId, secret, username, password)
// Crayon API requires: Basic Auth header + grant_type=password + username + password + scope
func (c *Client) getTokenWithPassword() (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", c.config.Username)
	data.Set("password", c.config.Password)
	data.Set("scope", "CustomerApi")

	return c.requestToken(data)
}

// requestToken performs the token request
// Crayon API requires client_id:client_secret as Basic Auth header
func (c *Client) requestToken(data url.Values) (*TokenResponse, error) {
	tokenURL := c.config.BaseURL + "/api/v1/connect/token"
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Add Basic Auth header with base64(client_id:client_secret)
	credentials := c.config.ClientID + ":" + c.config.ClientSecret
	encodedCredentials := base64.StdEncoding.EncodeToString([]byte(credentials))
	req.Header.Set("Authorization", "Basic "+encodedCredentials)


	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}


	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}
// AzureTokenResponse represents the Azure AD OAuth token response
type AzureTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // Usually in seconds
}

// AzureCLITokenResponse represents the response from `az account get-access-token`
type AzureCLITokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresOn   string `json:"expiresOn"`
}

// getAzureToken returns a valid Azure AD access token, refreshing if necessary
// Supports two authentication methods:
// 1. Service Principal (if ARM_CLIENT_ID, ARM_CLIENT_SECRET, ARM_TENANT_ID are set)
// 2. Azure CLI session (fallback - uses `az account get-access-token`)
func (c *Client) getAzureToken() (string, error) {
	// Return cached token if still valid (with 60 second buffer)
	if c.azureToken != "" && time.Now().Before(c.azureTokenExp.Add(-60*time.Second)) {
		return c.azureToken, nil
	}

	// Try Service Principal auth first (if credentials are configured)
	if c.config.AzureClientID != "" && c.config.AzureClientSecret != "" && c.config.AzureTenantID != "" {
		return c.getAzureTokenWithServicePrincipal()
	}

	// Fallback to Azure CLI session
	return c.getAzureTokenWithCLI()
}

// getAzureTokenWithServicePrincipal authenticates using client credentials (Service Principal)
func (c *Client) getAzureTokenWithServicePrincipal() (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.config.AzureTenantID)
	data := url.Values{}
	data.Set("client_id", c.config.AzureClientID)
	data.Set("client_secret", c.config.AzureClientSecret)
	data.Set("grant_type", "client_credentials")
	data.Set("scope", "https://management.azure.com/.default")

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create azure token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("azure token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read azure token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("azure token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp AzureTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse azure token response: %w", err)
	}

	c.azureToken = tokenResp.AccessToken
	c.azureTokenExp = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return c.azureToken, nil
}

// getAzureTokenWithCLI gets a token from the Azure CLI session (az login)
func (c *Client) getAzureTokenWithCLI() (string, error) {
	fmt.Println("[INFO] No Azure Service Principal configured. Using Azure CLI session...")

	cmd := exec.Command("az", "account", "get-access-token", "--resource", "https://management.azure.com", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get token from Azure CLI (run 'az login' first): %w", err)
	}

	var tokenResp AzureCLITokenResponse
	if err := json.Unmarshal(output, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse Azure CLI token response: %w", err)
	}

	c.azureToken = tokenResp.AccessToken
	// Parse expiresOn (format: "2024-01-13 00:45:00.000000")
	// For simplicity, just set expiry to 50 minutes from now
	c.azureTokenExp = time.Now().Add(50 * time.Minute)

	return c.azureToken, nil
}
