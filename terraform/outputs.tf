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

output "provisioner_service_account" {
  description = "Service account email for device provisioning (impersonate this)"
  value       = module.iam.provisioner_sa_email
}
