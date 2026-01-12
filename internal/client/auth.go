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
