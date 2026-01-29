terraform {
  required_version = ">= 1.6.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }

  # Uncomment after creating a GCS bucket for state
  # backend "gcs" {
  #   bucket = "home-monitoring-tfstate"
  #   prefix = "tofu/state"
  # }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# Enable required APIs
resource "google_project_service" "apis" {
  for_each = toset([
    "run.googleapis.com",              # Cloud Run
    "firestore.googleapis.com",        # Firestore
    "pubsub.googleapis.com",           # Pub/Sub
    "cloudbuild.googleapis.com",       # Cloud Build (for container builds)
    "artifactregistry.googleapis.com", # Container Registry
    "secretmanager.googleapis.com",    # Secret Manager
    "iam.googleapis.com",              # IAM
    "identitytoolkit.googleapis.com",  # Firebase Auth / Identity Platform
  ])

  service            = each.value
  disable_on_destroy = false
}

resource "google_artifact_registry_repository" "containers" {
  location      = var.region
  repository_id = var.repository_id
  description   = "Container images for home monitoring services"
  format        = "DOCKER"

  depends_on = [google_project_service.apis]
}

module "iam" {
  source     = "./modules/iam"
  project_id = var.project_id

  depends_on = [google_project_service.apis]
}

module "firestore" {
  source     = "./modules/firestore"
  project_id = var.project_id
  region     = var.region

  depends_on = [google_project_service.apis]
}

module "pubsub" {
  source     = "./modules/pubsub"
  project_id = var.project_id

  depends_on = [google_project_service.apis]
}

module "secrets" {
  source     = "./modules/secrets"
  project_id = var.project_id

  telemetry_api_sa_email = module.iam.telemetry_api_sa_email
  provisioner_users      = var.provisioner_users

  depends_on = [google_project_service.apis, module.iam]
}

# Workload Identity Federation for GitHub Actions CI/CD
module "workload_identity" {
  count = length(var.github_repositories) > 0 ? 1 : 0

  source = "./modules/workload-identity"

  project_id            = var.project_id
  github_repositories   = var.github_repositories
  service_account_email = module.secrets.admin_api_key_secret_id # Not used, but required by module

  depends_on = [
    google_project_service.apis,
    module.secrets,
  ]
}

# Grant CI/CD service account access to GitHub Actions API key secret
resource "google_secret_manager_secret_iam_member" "cicd_github_actions_secret_access" {
  count = length(var.github_repositories) > 0 ? 1 : 0

  project   = var.project_id
  secret_id = module.secrets.github_actions_api_key_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${module.workload_identity[0].service_account_email}"

  depends_on = [module.workload_identity]
}

module "cloud_run" {
  source = "./modules/cloud-run"

  project_id         = var.project_id
  region             = var.region
  service_account    = module.iam.telemetry_api_sa_email
  image              = "${var.region}-docker.pkg.dev/${var.project_id}/${var.repository_id}/telemetry-api:${var.image_tag}"
  firestore_database = module.firestore.database_name

  allow_unauthenticated            = var.allow_unauthenticated
  admin_api_key_secret_id          = module.secrets.admin_api_key_secret_id
  github_actions_api_key_secret_id = module.secrets.github_actions_api_key_secret_id

  depends_on = [
    google_project_service.apis,
    google_artifact_registry_repository.containers,
    module.iam,
    module.firestore,
    module.secrets,
  ]
}
