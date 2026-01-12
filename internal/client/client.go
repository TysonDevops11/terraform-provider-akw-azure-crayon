// Copyright (c) 2024
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrAccepted indicates the request was accepted for processing (202) but returned no content
var ErrAccepted = errors.New("request accepted")

// ClientConfig holds the configuration for the Crayon API client
type ClientConfig struct {
	BaseURL           string
	ClientID          string
	ClientSecret      string
	Username          string
	Password          string
	OrganizationID    int64
	AzureClientID     string
	AzureClientSecret string
	AzureTenantID     string
}

// Client is the Crayon API client
type Client struct {
	config        ClientConfig
	httpClient    *http.Client
	token         string
	tokenExp      time.Time
	azureToken    string
	azureTokenExp time.Time
}

// NewClient creates a new Crayon API client
func NewClient(config ClientConfig) (*Client, error) {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// GetOrganizationID returns the configured organization ID
func (c *Client) GetOrganizationID() int64 {
	return c.config.OrganizationID
}

// doRequest performs an authenticated HTTP request
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := c.config.BaseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")


	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// parseResponse parses a JSON response body
func parseResponse[T any](resp *http.Response, result *T) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if len(body) == 0 {
		if resp.StatusCode == http.StatusAccepted {
			return ErrAccepted
		}
		if resp.StatusCode == http.StatusNoContent {
			return nil
		}
		return fmt.Errorf("response body is empty (status %d)", resp.StatusCode)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	return nil
}

// readResponseBody reads the response body without closing the response.
// The caller must close the response body.
func readResponseBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if len(body) == 0 {
		if resp.StatusCode == http.StatusAccepted {
			return nil, ErrAccepted
		}
		if resp.StatusCode == http.StatusNoContent {
			return nil, nil
		}
		return nil, fmt.Errorf("response body is empty (status %d)", resp.StatusCode)
	}

	return body, nil
}

// unmarshalResponse unmarshals a response body into the given result.
func unmarshalResponse[T any](body []byte, statusCode int, result *T) error {
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}
	return nil
}

