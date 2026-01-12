# Terraform Provider for Crayon Cloud-iQ

A Terraform provider to manage Azure subscriptions through the Crayon Cloud-iQ API.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for building)
- Crayon Cloud-iQ API credentials

## Building the Provider

```bash
go mod tidy
go build -o terraform-provider-crayon
```

## Installing for Local Development

1. Create a `.terraformrc` file in your home directory:

```hcl
provider_installation {
  dev_overrides {
    "local/crayon" = "/Users/quang2206/Coding/tfprovider"
  }
  direct {}
}
```

2. Build the provider:

```bash
go build -o terraform-provider-crayon
```

## Installation

### From Terraform Registry (Recommended)

```hcl
terraform {
  required_providers {
    crayon = {
      source  = "TysonDevops11/akw-azure-crayon"
      version = "~> 1.0"
    }
  }
}

provider "crayon" {}
```

Then run:

```bash
terraform init
```

### For Local Development

See [Installing for Local Development](#installing-for-local-development) above.

---

## Configuration

### Provider Configuration

```hcl
provider "crayon" {
  # Required
  client_id     = "your-client-id"      # or CRAYON_CLIENT_ID
  client_secret = "your-client-secret"  # or CRAYON_SECRET
  
  # Optional - for password-based auth (like C# CLI)
  username = "your-username"            # or CRAYON_USERNAME
  password = "your-password"            # or CRAYON_PASSWORD
  
  # Optional with defaults
  base_url        = "https://api.crayon.com"  # or CRAYON_BASE_URL
  organization_id = 4051878                   # or CRAYON_ORGANIZATION_ID
}
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `CRAYON_BASE_URL` | API base URL | No (defaults to https://api.crayon.com) |
| `CRAYON_CLIENT_ID` | OAuth client ID | Yes |
| `CRAYON_SECRET` | OAuth client secret | Yes |
| `CRAYON_USERNAME` | Username for password auth | No |
| `CRAYON_PASSWORD` | Password for password auth | No |
| `CRAYON_ORGANIZATION_ID` | Organization ID | No (defaults to 4051878) |

## Resources

### crayon_azure_subscription

Manages an Azure subscription through Crayon Cloud-iQ.

#### Example Usage

```hcl
resource "crayon_azure_subscription" "example" {
  azure_plan_id = YOUR_AZURE_PLAN_ID
  name          = "my-azure-subscription"
}
```

#### Argument Reference

- `azure_plan_id` - (Required) The Azure Plan ID to create the subscription under.
- `name` - (Required) The display name of the subscription.

#### Attribute Reference

- `id` - The internal Crayon ID of the subscription.
- `subscription_id` - The Azure subscription GUID.
- `status` - The current status (active, cancelled, etc.).

#### Import

Import using the format `azure_plan_id:subscription_id`:

```bash
terraform import crayon_azure_subscription.example YOUR_AZURE_PLAN_ID:SUBSCRIPTION_ID
```

## Complete Example

Here's a complete example that creates an Azure subscription and outputs all relevant details:

```hcl
terraform {
  required_providers {
    crayon = {
      source  = "TysonDevops11/akw-azure-crayon"
      version = "~> 1.0"
    }
  }
}

# Configure provider using environment variables:
# - CRAYON_CLIENT_ID
# - CRAYON_SECRET
# - CRAYON_USERNAME (optional)
# - CRAYON_PASSWORD (optional)
provider "crayon" {}

variable "azure_plan_id" {
  description = "The Azure Plan ID to create subscriptions under"
  type        = number
}

# Create an Azure subscription
resource "crayon_azure_subscription" "example" {
  azure_plan_id  = var.azure_plan_id
  name           = "my-azure-subscription"
  create_timeout = 20  # Optional: timeout in minutes (default: 15)
}

# Output the subscription details
output "crayon_id" {
  description = "The internal Crayon subscription ID"
  value       = crayon_azure_subscription.example.id
}

output "subscription_id" {
  description = "The Azure subscription GUID"
  value       = crayon_azure_subscription.example.subscription_id
}

output "status" {
  description = "The subscription status"
  value       = crayon_azure_subscription.example.status
}

output "subscription_name" {
  description = "The subscription display name"
  value       = crayon_azure_subscription.example.name
}
```

Run with:

```bash
export CRAYON_CLIENT_ID="your-client-id"
export CRAYON_SECRET="your-client-secret"

terraform init
terraform plan -var="azure_plan_id=YOUR_AZURE_PLAN_ID"
terraform apply -var="azure_plan_id=YOUR_AZURE_PLAN_ID"
```

## Authentication

The provider supports two authentication methods:

1. **Client Credentials** (recommended for CI/CD):
   - Set `client_id` and `client_secret`
   - Tries this method first

2. **Resource Owner Password** (for backward compatibility with C# CLI):
   - Also set `username` and `password`
   - Used as fallback when username/password are provided

## Development

Based on the C# CLI tool at `infra-alz-subscription_manager`.

### Running Tests

```bash
go test ./...
```

### Generating Documentation

```bash
go generate
```
