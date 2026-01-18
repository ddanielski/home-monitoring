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
  region     = var.region

  # These are set after initial deploy (circular dependency with cloud_run)
  cloud_run_service_name = var.cloud_run_service_name
  provisioner_users      = var.provisioner_users

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

module "cloud_run" {
  source = "./modules/cloud-run"

  project_id         = var.project_id
  region             = var.region
  service_account    = module.iam.telemetry_api_sa_email
  image              = "${var.region}-docker.pkg.dev/${var.project_id}/${var.repository_id}/telemetry-api:${var.image_tag}"
  firestore_database = module.firestore.database_name

  allow_unauthenticated = var.allow_unauthenticated
  # The provisioner SA is the only email allowed to provision devices
  provisioner_emails    = module.iam.provisioner_sa_email
  service_url           = var.service_url

  depends_on = [
    google_project_service.apis,
    google_artifact_registry_repository.containers,
    module.iam,
    module.firestore,
  ]
}
