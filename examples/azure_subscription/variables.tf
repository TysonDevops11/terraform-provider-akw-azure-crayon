# Crayon API Credentials
variable "crayon_client_id" {
  description = "OAuth Client ID for Crayon API"
  type        = string
  sensitive   = true
}

variable "crayon_client_secret" {
  description = "OAuth Client Secret for Crayon API"
  type        = string
  sensitive   = true
}

variable "crayon_username" {
  description = "Username for Crayon API (required for password auth)"
  type        = string
}

variable "crayon_password" {
  description = "Password for Crayon API (required for password auth)"
  type        = string
  sensitive   = true
}

# Subscription Configuration
variable "azure_plan_id" {
  description = "The Azure Plan ID from Crayon to create subscriptions under"
  type        = number
}

variable "subscription_name" {
  description = "Name for the Azure subscription"
  type        = string
  default     = "tyson-published-tfprovider-subscrition-01"
}
