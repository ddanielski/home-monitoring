variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "pool_id" {
  description = "Workload Identity Pool ID"
  type        = string
  default     = "github-actions-pool"
}

variable "provider_id" {
  description = "Workload Identity Provider ID"
  type        = string
  default     = "github-actions-provider"
}

variable "github_repositories" {
  description = "List of GitHub repositories in format 'owner/repo' (e.g., ['username/repo1', 'username/repo2'])"
  type        = list(string)
}

variable "service_account_email" {
  description = "Email of the service account to bind to the provider (deprecated, not used)"
  type        = string
  default     = ""
}

# Enable IAM Credentials API (required for Workload Identity)
resource "google_project_service" "iam_credentials" {
  service            = "iamcredentials.googleapis.com"
  disable_on_destroy = false
}

# Enable Service Usage API (required for Workload Identity Pool)
resource "google_project_service" "service_usage" {
  service            = "serviceusage.googleapis.com"
  disable_on_destroy = false
}

# Workload Identity Pool
resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = var.pool_id
  display_name              = "GitHub Actions Pool"
  description               = "Workload Identity Pool for GitHub Actions CI/CD"

  depends_on = [
    google_project_service.iam_credentials,
    google_project_service.service_usage,
  ]
}

# Workload Identity Provider (GitHub OIDC)
resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = var.provider_id
  display_name                       = "GitHub Actions Provider"
  description                        = "OIDC provider for GitHub Actions"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.actor"      = "assertion.actor"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_condition = "assertion.repository in [${join(", ", [for repo in var.github_repositories : "\"${repo}\""])}]"
}

# Service Account for CI/CD
resource "google_service_account" "cicd" {
  account_id   = "github-actions-cicd"
  display_name = "GitHub Actions CI/CD Service Account"
  description  = "Service account for GitHub Actions CI/CD pipeline"
}

# Allow Workload Identity Provider to impersonate the CI/CD service account
# Create IAM bindings for each repository
resource "google_service_account_iam_member" "workload_identity_binding" {
  for_each = toset(var.github_repositories)

  service_account_id = google_service_account.cicd.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${data.google_project.project.number}/locations/global/workloadIdentityPools/${var.pool_id}/attribute.repository/${each.value}"
}

# Allow CI/CD service account to view Cloud Run services (to fetch service URL dynamically)
resource "google_project_iam_member" "cicd_run_viewer" {
  project = var.project_id
  role    = "roles/run.viewer"
  member  = "serviceAccount:${google_service_account.cicd.email}"
}


# Data source to get project number (required for principalSet)
data "google_project" "project" {
  project_id = var.project_id
}

# =============================================================================
# Outputs
# =============================================================================

output "workload_identity_pool_id" {
  description = "Workload Identity Pool ID"
  value       = google_iam_workload_identity_pool.github.workload_identity_pool_id
}

output "workload_identity_provider_id" {
  description = "Workload Identity Provider ID"
  value       = google_iam_workload_identity_pool_provider.github.workload_identity_pool_provider_id
}

output "service_account_email" {
  description = "Email of the CI/CD service account"
  value       = google_service_account.cicd.email
}

output "workload_identity_provider_resource_name" {
  description = "Full resource name of the Workload Identity Provider (for use in GitHub Actions)"
  value       = google_iam_workload_identity_pool_provider.github.name
}
