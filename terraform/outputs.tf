output "cloud_run_url" {
  description = "URL of the deployed Cloud Run service"
  value       = module.cloud_run.service_url
}

output "artifact_registry_url" {
  description = "URL for pushing container images"
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${var.repository_id}"
}

output "telemetry_api_service_account" {
  description = "Service account email for telemetry API"
  value       = module.iam.telemetry_api_sa_email
}

output "pubsub_telemetry_topic" {
  description = "Pub/Sub topic for telemetry events"
  value       = module.pubsub.telemetry_topic_name
}

# Workload Identity Federation outputs (if enabled)
output "workload_identity_provider_resource_name" {
  description = "Full resource name of the Workload Identity Provider (for GitHub Actions)"
  value       = var.github_repository != "" ? module.workload_identity[0].workload_identity_provider_resource_name : null
}

output "cicd_service_account_email" {
  description = "Email of the CI/CD service account for GitHub Actions"
  value       = var.github_repository != "" ? module.workload_identity[0].service_account_email : null
}
