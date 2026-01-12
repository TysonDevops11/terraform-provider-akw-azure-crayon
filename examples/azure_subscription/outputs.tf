# Outputs
output "subscription_id" {
  description = "The Azure subscription GUID"
  value       = crayon_azure_subscription.example.subscription_id
}

output "crayon_id" {
  description = "The internal Crayon ID"
  value       = crayon_azure_subscription.example.id
}

output "status" {
  description = "Subscription status"
  value       = crayon_azure_subscription.example.status
}
