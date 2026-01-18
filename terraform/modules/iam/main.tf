variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
}

variable "cloud_run_service_name" {
  description = "Name of the Cloud Run service for IAM bindings"
  type        = string
  default     = ""
}

variable "provisioner_users" {
  description = "List of user emails allowed to impersonate the provisioner SA"
  type        = list(string)
  default     = []
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

resource "google_project_iam_member" "telemetry_api_secrets" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.telemetry_api.email}"
}

resource "google_project_iam_member" "telemetry_api_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.telemetry_api.email}"
}

# =============================================================================
# Provisioner Service Account (for device provisioning via impersonation)
# =============================================================================

resource "google_service_account" "provisioner" {
  account_id   = "provisioner"
  display_name = "Device Provisioner"
  description  = "Service account for provisioning IoT devices. Users impersonate this SA."
}

# Allow provisioner SA to invoke Cloud Run (when service exists)
resource "google_cloud_run_v2_service_iam_member" "provisioner_invoker" {
  count = var.cloud_run_service_name != "" ? 1 : 0

  project  = var.project_id
  location = var.region
  name     = var.cloud_run_service_name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.provisioner.email}"
}

# Allow specified users to impersonate the provisioner SA
resource "google_service_account_iam_member" "provisioner_impersonators" {
  for_each = toset(var.provisioner_users)

  service_account_id = google_service_account.provisioner.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "user:${each.value}"
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

output "provisioner_sa_email" {
  description = "Email of the provisioner service account"
  value       = google_service_account.provisioner.email
}
