// Copyright (c) 2024
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/crayon-cloud/terraform-provider-crayon/internal/client"
	"github.com/crayon-cloud/terraform-provider-crayon/internal/resources"
)

// Ensure CrayonProvider satisfies various provider interfaces.
var _ provider.Provider = &CrayonProvider{}

// CrayonProvider defines the provider implementation.
type CrayonProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// CrayonProviderModel describes the provider data model.
type CrayonProviderModel struct {
	BaseURL           types.String `tfsdk:"base_url"`
	ClientID          types.String `tfsdk:"client_id"`
	ClientSecret      types.String `tfsdk:"client_secret"`
	Username          types.String `tfsdk:"username"`
	Password          types.String `tfsdk:"password"`
	OrganizationID    types.Int64  `tfsdk:"organization_id"`
	AzureClientID     types.String `tfsdk:"azure_client_id"`
	AzureClientSecret types.String `tfsdk:"azure_client_secret"`
	AzureTenantID     types.String `tfsdk:"azure_tenant_id"`
}

func (p *CrayonProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "crayon"
	resp.Version = p.version
}

func (p *CrayonProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for Crayon Cloud-iQ API to manage Azure subscriptions.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Description: "Base URL for Crayon API. Can also be set via CRAYON_BASE_URL environment variable. Defaults to https://api.crayon.com",
				Optional:    true,
			},
			"client_id": schema.StringAttribute{
				Description: "OAuth Client ID for Crayon API. Can also be set via CRAYON_CLIENT_ID environment variable.",
				Optional:    true,
			},
			"client_secret": schema.StringAttribute{
				Description: "OAuth Client Secret for Crayon API. Can also be set via CRAYON_SECRET environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"username": schema.StringAttribute{
				Description: "Username for password-based authentication. Can also be set via CRAYON_USERNAME environment variable. Only required if using password auth.",
				Optional:    true,
			},
			"password": schema.StringAttribute{
				Description: "Password for password-based authentication. Can also be set via CRAYON_PASSWORD environment variable. Only required if using password auth.",
				Optional:    true,
				Sensitive:   true,
			},
			"organization_id": schema.Int64Attribute{
				Description: "Crayon Organization ID. Can also be set via CRAYON_ORGANIZATION_ID environment variable. Defaults to 4051878.",
				Optional:    true,
			},
			"azure_client_id": schema.StringAttribute{
				Description: "Azure Service Principal Client ID for direct subscription querying. Can also be set via ARM_CLIENT_ID.",
				Optional:    true,
			},
			"azure_client_secret": schema.StringAttribute{
				Description: "Azure Service Principal Client Secret for direct subscription querying. Can also be set via ARM_CLIENT_SECRET.",
				Optional:    true,
				Sensitive:   true,
			},
			"azure_tenant_id": schema.StringAttribute{
				Description: "Azure Tenant ID for direct subscription querying. Can also be set via ARM_TENANT_ID.",
				Optional:    true,
			},
		},
	}
}

func (p *CrayonProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Crayon client")

	var config CrayonProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get configuration values with environment variable fallbacks
	baseURL := getConfigValue(config.BaseURL.ValueString(), "CRAYON_BASE_URL", "https://api.crayon.com")
	clientID := getConfigValue(config.ClientID.ValueString(), "CRAYON_CLIENT_ID", "")
	clientSecret := getConfigValue(config.ClientSecret.ValueString(), "CRAYON_SECRET", "")
	username := getConfigValue(config.Username.ValueString(), "CRAYON_USERNAME", "")
	
	// Password: check config first, then CRAYON_PASSWORD, then CRAYON_PASSWORD_BASE64_ENCODED (for C# CLI compatibility)
	password := config.Password.ValueString()
	if password == "" {
		if envPassword := os.Getenv("CRAYON_PASSWORD"); envPassword != "" {
			password = envPassword
		} else if envPasswordB64 := os.Getenv("CRAYON_PASSWORD_BASE64_ENCODED"); envPasswordB64 != "" {
			// Decode base64 password (C# CLI compatibility)
			decoded, err := base64.StdEncoding.DecodeString(envPasswordB64)
			if err == nil {
				password = string(decoded)
			}
		}
	}

	// Handle organization ID
	var organizationID int64 = 4051878 // Default value
	if !config.OrganizationID.IsNull() {
		organizationID = config.OrganizationID.ValueInt64()
	} else if envOrgID := os.Getenv("CRAYON_ORGANIZATION_ID"); envOrgID != "" {
		// Parse from env var if needed
		var parsedID int64
		if _, err := parseIntFromEnv(envOrgID, &parsedID); err == nil {
			organizationID = parsedID
		}
	}

	// Azure Credentials for direct querying (Optional but recommended for faster updates)
	azureClientID := getConfigValue(config.AzureClientID.ValueString(), "ARM_CLIENT_ID", "")
	azureClientSecret := getConfigValue(config.AzureClientSecret.ValueString(), "ARM_CLIENT_SECRET", "")
	azureTenantID := getConfigValue(config.AzureTenantID.ValueString(), "ARM_TENANT_ID", "")

	// Validate Azure credentials if partially set
	if (azureClientID != "" || azureClientSecret != "" || azureTenantID != "") &&
		(azureClientID == "" || azureClientSecret == "" || azureTenantID == "") {
		resp.Diagnostics.AddWarning(
			"Incomplete Azure Configuration",
			"To enable direct Azure subscription polling, all three Azure credentials must be provided: "+
				"azure_client_id, azure_client_secret, and azure_tenant_id (or via ARM_* env vars). "+
				"Falling back to Crayon-only polling (slower).",
		)
	}

	// Validate required configuration
	if clientID == "" {
		resp.Diagnostics.AddError(
			"Missing Client ID",
			"The provider requires a client_id to be set either in the provider configuration or via CRAYON_CLIENT_ID environment variable.",
		)
	}
	if clientSecret == "" {
		resp.Diagnostics.AddError(
			"Missing Client Secret",
			"The provider requires a client_secret to be set either in the provider configuration or via CRAYON_SECRET environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Crayon client", map[string]interface{}{
		"base_url":        baseURL,
		"organization_id": organizationID,
		"has_username":    username != "",
		"has_azure_creds": azureClientID != "",
	})

	// Create client with dual-auth support
	crayonClient, err := client.NewClient(client.ClientConfig{
		BaseURL:           baseURL,
		ClientID:          clientID,
		ClientSecret:      clientSecret,
		Username:          username,
		Password:          password,
		OrganizationID:    organizationID,
		AzureClientID:     azureClientID,
		AzureClientSecret: azureClientSecret,
		AzureTenantID:     azureTenantID,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Crayon API Client",
			"An unexpected error occurred when creating the Crayon API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Error: "+err.Error(),
		)
		return
	}

	// Make the client available to resources and data sources
	resp.DataSourceData = crayonClient
	resp.ResourceData = crayonClient

	tflog.Info(ctx, "Configured Crayon client", map[string]interface{}{
		"base_url": baseURL,
	})
}

func (p *CrayonProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewAzureSubscriptionResource,
	}
}

func (p *CrayonProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// Data sources can be added here in the future
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CrayonProvider{
			version: version,
		}
	}
}

// Helper functions

func getConfigValue(configValue, envVar, defaultValue string) string {
	if configValue != "" {
		return configValue
	}
	if envValue := os.Getenv(envVar); envValue != "" {
		return envValue
	}
	return defaultValue
}

func parseIntFromEnv(value string, result *int64) (bool, error) {
	if value == "" {
		return false, nil
	}
	var n int64
	_, err := parseIntString(value, &n)
	if err != nil {
		return false, err
	}
	*result = n
	return true, nil
}

func parseIntString(s string, result *int64) (bool, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
		n = n*10 + int64(c-'0')
	}
	*result = n
	return true, nil
}
