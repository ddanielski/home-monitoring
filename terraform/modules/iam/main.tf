variable "project_id" {
  description = "GCP project ID"
  type        = string
}

# Service account for telemetry-api Cloud Run service
resource "google_service_account" "telemetry_api" {
  account_id   = "telemetry-api-sa"
  display_name = "Telemetry API Service Account"
  description  = "Service account for the telemetry-api Cloud Run service"
}

resource "google_project_iam_member" "telemetry_api_firestore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.telemetry_api.email}"
}

resource "google_project_iam_member" "telemetry_api_pubsub_publisher" {
  project = var.project_id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.telemetry_api.email}"
}

resource "google_project_iam_member" "telemetry_api_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.telemetry_api.email}"
}

# Allow the service account to sign JWTs (required for Firebase custom tokens)
resource "google_service_account_iam_member" "telemetry_api_token_creator" {
  service_account_id = google_service_account.telemetry_api.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.telemetry_api.email}"
}

# =============================================================================
# Outputs
# =============================================================================

output "telemetry_api_sa_email" {
  description = "Email of the telemetry API service account"
  value       = google_service_account.telemetry_api.email
}

output "telemetry_api_sa_name" {
  description = "Full name of the telemetry API service account"
  value       = google_service_account.telemetry_api.name
}
