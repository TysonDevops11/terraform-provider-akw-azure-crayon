# Example: Creating an Azure Subscription with Crayon Provider

terraform {
  required_version = ">= 1.0"

  required_providers {
    crayon = {
      source  = "TysonDevops11/akw-azure-crayon"
      version = "1.0.0"
    }
  }
}

# Configure the Crayon provider
# Can use environment variables instead:
#   - CRAYON_CLIENT_ID, CRAYON_SECRET
#   - CRAYON_USERNAME, CRAYON_PASSWORD (optional)
#   - CRAYON_ORGANIZATION_ID (optional)
provider "crayon" {
  client_id       = var.crayon_client_id
  client_secret   = var.crayon_client_secret
  username        = var.crayon_username
  password        = var.crayon_password
}

# Create a single Azure subscription
resource "crayon_azure_subscription" "example" {
  azure_plan_id  = var.azure_plan_id
  name           = var.subscription_name
  create_timeout = 20
}
