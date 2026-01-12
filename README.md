# Terraform Provider for Crayon Cloud-iQ

A Terraform provider to manage Azure subscriptions through the Crayon Cloud-iQ API.

## Features

- **v1.1.0+**: Direct Azure ARM polling for faster subscription verification
- Supports Azure Service Principal or Azure CLI authentication for polling
- Fire-and-forget subscription creation with async confirmation

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- Crayon Cloud-iQ API credentials
- (Optional) Azure Service Principal for direct Azure polling

## Installation

```hcl
terraform {
  required_providers {
    crayon = {
      source  = "TysonDevops11/akw-azure-crayon"
      version = "~> 1.1"
    }
  }
}

provider "crayon" {}
```

Then run:

```bash
terraform init
```

## Configuration

### Provider Configuration

```hcl
provider "crayon" {
  # Required - Crayon API credentials
  client_id     = "your-client-id"      # or CRAYON_CLIENT_ID
  client_secret = "your-client-secret"  # or CRAYON_SECRET
  
  # Optional - for password-based auth
  username = "your-username"            # or CRAYON_USERNAME
  password = "your-password"            # or CRAYON_PASSWORD
  
  # Optional with defaults
  base_url        = "https://api.crayon.com"  # or CRAYON_BASE_URL
  organization_id = 4051878                   # or CRAYON_ORGANIZATION_ID

  # Optional - Azure credentials for direct polling (v1.1.0+)
  azure_client_id     = "..."  # or ARM_CLIENT_ID
  azure_client_secret = "..."  # or ARM_CLIENT_SECRET  
  azure_tenant_id     = "..."  # or ARM_TENANT_ID
}
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `CRAYON_CLIENT_ID` | OAuth client ID | Yes |
| `CRAYON_SECRET` | OAuth client secret | Yes |
| `CRAYON_USERNAME` | Username for password auth | No |
| `CRAYON_PASSWORD` | Password for password auth | No |
| `CRAYON_BASE_URL` | API base URL | No (defaults to https://api.crayon.com) |
| `CRAYON_ORGANIZATION_ID` | Organization ID | No (defaults to 4051878) |
| `ARM_CLIENT_ID` | Azure SP Client ID for polling | No |
| `ARM_CLIENT_SECRET` | Azure SP Client Secret | No |
| `ARM_TENANT_ID` | Azure Tenant ID | No |

## Resources

### crayon_azure_subscription

Manages an Azure subscription through Crayon Cloud-iQ.

#### Example Usage

```hcl
resource "crayon_azure_subscription" "example" {
  azure_plan_id  = 873834
  name           = "my-azure-subscription"
  create_timeout = 20  # Optional: timeout in minutes (default: 15)
}
```

#### Argument Reference

- `azure_plan_id` - (Required) The Azure Plan ID to create the subscription under.
- `name` - (Required) The display name of the subscription.
- `create_timeout` - (Optional) Timeout in minutes for subscription creation. Default: 15.

#### Attribute Reference

- `id` - The internal Crayon ID of the subscription.
- `subscription_id` - The Azure subscription GUID.
- `status` - The current status (active, cancelled, etc.).

#### Import

```bash
terraform import crayon_azure_subscription.example AZURE_PLAN_ID:SUBSCRIPTION_ID
```

## Azure Polling (v1.1.0+)

When creating a subscription, the provider polls Azure ARM API to confirm the subscription exists. This is faster and more reliable than waiting for Cloud-iQ sync.

### Authentication Priority

1. **Service Principal** (if `ARM_CLIENT_ID`, `ARM_CLIENT_SECRET`, `ARM_TENANT_ID` are set)
2. **Azure CLI** (fallback - uses your `az login` session)

### Pending State

If the subscription isn't found in Azure within the timeout, the resource will be in a "pending" state:
- `id` = `pending-<subscription-name>`
- `subscription_id` = Azure GUID (if found) or `pending`

Run `terraform refresh` after Cloud-iQ syncs to get the real Crayon ID.

## Complete Example

```hcl
terraform {
  required_providers {
    crayon = {
      source  = "TysonDevops11/akw-azure-crayon"
      version = "~> 1.1"
    }
  }
}

provider "crayon" {}

resource "crayon_azure_subscription" "example" {
  azure_plan_id  = var.azure_plan_id
  name           = "my-azure-subscription"
  create_timeout = 20
}

output "crayon_id" {
  value = crayon_azure_subscription.example.id
}

output "subscription_id" {
  value = crayon_azure_subscription.example.subscription_id
}

output "status" {
  value = crayon_azure_subscription.example.status
}
```

Run with:

```bash
export CRAYON_CLIENT_ID="your-client-id"
export CRAYON_SECRET="your-client-secret"
export ARM_CLIENT_ID="your-azure-sp-client-id"         # Optional
export ARM_CLIENT_SECRET="your-azure-sp-client-secret" # Optional
export ARM_TENANT_ID="your-azure-tenant-id"            # Optional

terraform init
terraform apply -var="azure_plan_id=YOUR_AZURE_PLAN_ID"
```

## Authentication

### Crayon API
- **Client Credentials**: Set `client_id` and `client_secret`
- **Password Auth**: Also set `username` and `password` (for C# CLI compatibility)

### Azure Polling
- **Service Principal**: Set `ARM_CLIENT_ID`, `ARM_CLIENT_SECRET`, `ARM_TENANT_ID`
- **Azure CLI**: Run `az login` before terraform apply (fallback)
