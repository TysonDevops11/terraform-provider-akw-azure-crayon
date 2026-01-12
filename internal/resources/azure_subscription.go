// Copyright (c) 2024
// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/crayon-cloud/terraform-provider-crayon/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AzureSubscriptionResource{}
var _ resource.ResourceWithImportState = &AzureSubscriptionResource{}

func NewAzureSubscriptionResource() resource.Resource {
	return &AzureSubscriptionResource{}
}

// AzureSubscriptionResource defines the resource implementation.
type AzureSubscriptionResource struct {
	client *client.Client
}

// AzureSubscriptionResourceModel describes the resource data model.
type AzureSubscriptionResourceModel struct {
	ID             types.String `tfsdk:"id"`
	AzurePlanID    types.Int64  `tfsdk:"azure_plan_id"`
	Name           types.String `tfsdk:"name"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	Status         types.String `tfsdk:"status"`
	CreateTimeout  types.Int64  `tfsdk:"create_timeout"`
}

func (r *AzureSubscriptionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_azure_subscription"
}

func (r *AzureSubscriptionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Azure Subscription through Crayon Cloud-iQ API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The internal Crayon ID of the subscription.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"azure_plan_id": schema.Int64Attribute{
				Description: "The Azure Plan ID to create the subscription under.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The display name of the Azure subscription.",
				Required:    true,
			},
			"subscription_id": schema.StringAttribute{
				Description: "The Azure subscription GUID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Description: "The current status of the subscription (e.g., active, cancelled).",
				Computed:    true,
			},
			"create_timeout": schema.Int64Attribute{
				Description: "Timeout in minutes for waiting for subscription creation. Default is 10 minutes.",
				Optional:    true,
			},
		},
	}
}

func (r *AzureSubscriptionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *AzureSubscriptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AzureSubscriptionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Azure subscription", map[string]interface{}{
		"azure_plan_id": data.AzurePlanID.ValueInt64(),
		"name":          data.Name.ValueString(),
	})

	// Create the subscription via Crayon API (fire-and-forget approach)
	subscription, err := r.client.CreateAzureSubscription(
		int(data.AzurePlanID.ValueInt64()),
		data.Name.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Azure Subscription",
			"Could not create subscription, unexpected error: "+err.Error(),
		)
		return
	}

	// Map response to model
	// Note: For async creation (202), ID will be 0 and SubscriptionID will be "pending"
	if subscription.ID == 0 {
		// Async creation - use name as temporary ID and add warning
		data.ID = types.StringValue("pending-" + data.Name.ValueString())
		resp.Diagnostics.AddWarning(
			"Subscription Creation In Progress",
			"The subscription creation request was accepted but is being provisioned asynchronously. "+
				"The subscription will appear in Cloud-iQ after Azure provisions it and Cloud-iQ syncs. "+
				"You can click 'Synchronize' in the Cloud-iQ portal or run 'terraform refresh' later to update the state.",
		)
	} else {
		data.ID = types.StringValue(strconv.Itoa(subscription.ID))
	}
	data.SubscriptionID = types.StringValue(subscription.SubscriptionID)
	data.Status = types.StringValue(subscription.Status)

	tflog.Info(ctx, "Created Azure subscription", map[string]interface{}{
		"id":              subscription.ID,
		"subscription_id": subscription.SubscriptionID,
		"status":          subscription.Status,
	})

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureSubscriptionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AzureSubscriptionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idValue := data.ID.ValueString()
	azurePlanID := int(data.AzurePlanID.ValueInt64())

	// Check if this is a pending subscription (created async, not yet synced)
	if strings.HasPrefix(idValue, "pending-") {
		subscriptionName := strings.TrimPrefix(idValue, "pending-")
		tflog.Debug(ctx, "Checking for pending subscription sync", map[string]interface{}{
			"name":          subscriptionName,
			"azure_plan_id": azurePlanID,
		})

		// Try to find the subscription by name in Cloud-iQ
		subscription, err := r.client.FindAzureSubscriptionByName(azurePlanID, subscriptionName)
		if err != nil {
			// Subscription not yet synced - keep the pending state
			tflog.Info(ctx, "Subscription not yet synced to Cloud-iQ", map[string]interface{}{
				"name": subscriptionName,
			})
			resp.Diagnostics.AddWarning(
				"Subscription Still Pending",
				"The subscription '"+subscriptionName+"' has not yet appeared in Cloud-iQ. "+
					"Please click 'Synchronize' in the Cloud-iQ portal or wait for automatic sync, then run 'terraform refresh'.",
			)
			// Keep current state as-is
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}

		// Subscription found! Update the state with real values
		tflog.Info(ctx, "Subscription synced to Cloud-iQ", map[string]interface{}{
			"id":              subscription.ID,
			"subscription_id": subscription.SubscriptionID,
		})
		data.ID = types.StringValue(strconv.Itoa(subscription.ID))
		data.SubscriptionID = types.StringValue(subscription.SubscriptionID)
		data.Status = types.StringValue(subscription.Status)
		data.Name = types.StringValue(subscription.FriendlyName)

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Normal read for non-pending subscriptions
	subscriptionID, err := strconv.Atoi(idValue)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Azure Subscription",
			"Could not parse subscription ID: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "Reading Azure subscription", map[string]interface{}{
		"id":            subscriptionID,
		"azure_plan_id": azurePlanID,
	})

	// Get subscription from API
	subscription, err := r.client.GetAzureSubscription(azurePlanID, subscriptionID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Azure Subscription",
			"Could not read subscription ID "+idValue+": "+err.Error(),
		)
		return
	}

	// Update model with fresh data
	data.Name = types.StringValue(subscription.FriendlyName)
	data.SubscriptionID = types.StringValue(subscription.SubscriptionID)
	data.Status = types.StringValue(subscription.Status)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureSubscriptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AzureSubscriptionResourceModel
	var state AzureSubscriptionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idValue := state.ID.ValueString()

	// Check if this is still a pending subscription
	if strings.HasPrefix(idValue, "pending-") {
		resp.Diagnostics.AddError(
			"Cannot Update Pending Subscription",
			"The subscription has not yet synced to Cloud-iQ. Please run 'terraform refresh' after "+
				"clicking 'Synchronize' in Cloud-iQ portal, then try the update again.",
		)
		return
	}

	// Parse ID
	subscriptionID, err := strconv.Atoi(idValue)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Azure Subscription",
			"Could not parse subscription ID: "+err.Error(),
		)
		return
	}

	// Check if name changed
	if data.Name.ValueString() != state.Name.ValueString() {
		tflog.Debug(ctx, "Renaming Azure subscription", map[string]interface{}{
			"id":       subscriptionID,
			"old_name": state.Name.ValueString(),
			"new_name": data.Name.ValueString(),
		})

		// Rename subscription
		subscription, err := r.client.RenameAzureSubscription(
			int(data.AzurePlanID.ValueInt64()),
			subscriptionID,
			data.Name.ValueString(),
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Azure Subscription",
				"Could not rename subscription: "+err.Error(),
			)
			return
		}

		data.ID = state.ID
		// Preserve SubscriptionID from state - rename API may not return it
		if subscription.SubscriptionID != "" {
			data.SubscriptionID = types.StringValue(subscription.SubscriptionID)
		} else {
			data.SubscriptionID = state.SubscriptionID
		}
		// Preserve Status from state if API doesn't return it
		if subscription.Status != "" {
			data.Status = types.StringValue(subscription.Status)
		} else {
			data.Status = state.Status
		}

		tflog.Info(ctx, "Renamed Azure subscription", map[string]interface{}{
			"id":       subscriptionID,
			"new_name": data.Name.ValueString(),
		})
	} else {
		// No changes, preserve state
		data.ID = state.ID
		data.SubscriptionID = state.SubscriptionID
		data.Status = state.Status
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureSubscriptionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AzureSubscriptionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idValue := data.ID.ValueString()

	// Check if this is a pending subscription
	if strings.HasPrefix(idValue, "pending-") {
		// Pending subscription - can't cancel via API since we don't have the Crayon ID
		// Just remove from state. The subscription may or may not exist in Azure.
		tflog.Warn(ctx, "Deleting pending subscription from state only (no Crayon ID available)", map[string]interface{}{
			"name": data.Name.ValueString(),
		})
		resp.Diagnostics.AddWarning(
			"Subscription Removed From State Only",
			"The subscription was still pending sync to Cloud-iQ. It has been removed from Terraform state "+
				"but may still exist in Azure. Check Azure portal and Cloud-iQ to verify.",
		)
		return
	}

	// Parse ID
	subscriptionID, err := strconv.Atoi(idValue)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Azure Subscription",
			"Could not parse subscription ID: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "Cancelling Azure subscription", map[string]interface{}{
		"id":            subscriptionID,
		"azure_plan_id": data.AzurePlanID.ValueInt64(),
	})

	// Cancel the subscription via Crayon API
	err = r.client.CancelAzureSubscription(
		int(data.AzurePlanID.ValueInt64()),
		subscriptionID,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Azure Subscription",
			"Could not cancel subscription, unexpected error: "+err.Error(),
		)
		return
	}

	tflog.Info(ctx, "Cancelled Azure subscription", map[string]interface{}{
		"id": subscriptionID,
	})
}

func (r *AzureSubscriptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "azure_plan_id:subscription_id"
	// Example: "873834:12345"
	
	idParts := splitImportID(req.ID)
	if len(idParts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in format 'azure_plan_id:subscription_id'. Got: "+req.ID,
		)
		return
	}

	azurePlanID, err := strconv.ParseInt(idParts[0], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Could not parse azure_plan_id: "+err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("azure_plan_id"), azurePlanID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), idParts[1])...)
}

func splitImportID(id string) []string {
	var result []string
	var current string
	for _, c := range id {
		if c == ':' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	result = append(result, current)
	return result
}
