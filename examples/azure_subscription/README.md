# Example: Azure Subscription with Crayon Provider

This example demonstrates how to create Azure subscriptions using the `akw-azure-crayon` Terraform provider.

## Prerequisites

1. A Crayon Cloud-iQ account with API credentials
2. An Azure Plan ID from your Crayon account
3. Terraform >= 1.0

## Usage

### 1. Set up credentials

Set the required environment variables:

```bash
export CRAYON_CLIENT_ID="your-client-id"
export CRAYON_SECRET="your-client-secret"

# Optional: for password-based authentication
export CRAYON_USERNAME="your-username"
export CRAYON_PASSWORD="your-password"
```

### 2. Create a terraform.tfvars file

```hcl
azure_plan_id = 873834  # Replace with your Azure Plan ID
```

### 3. Initialize and apply

```bash
terraform init
terraform plan
terraform apply
```

## Importing Existing Subscriptions

You can import existing subscriptions using the format `azure_plan_id:subscription_id`:

```bash
terraform import akw-azure-crayon_azure_subscription.dev "873834:12345"
```

## Resources

| Resource | Description |
|----------|-------------|
| `akw-azure-crayon_azure_subscription` | Manages an Azure subscription through Crayon Cloud-iQ |

### Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `azure_plan_id` | number | Yes | The Azure Plan ID to create the subscription under |
| `name` | string | Yes | The display name of the Azure subscription |
| `create_timeout` | number | No | Timeout in minutes for subscription creation (default: 10) |
| `id` | string | Computed | The internal Crayon ID of the subscription |
| `subscription_id` | string | Computed | The Azure subscription GUID |
| `status` | string | Computed | The current status of the subscription |
