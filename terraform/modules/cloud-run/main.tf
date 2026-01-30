variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
}

variable "service_account" {
  description = "Service account email for the Cloud Run service"
  type        = string
}

variable "image" {
  description = "Container image URL"
  type        = string
}

variable "firestore_database" {
  description = "Firestore database name"
  type        = string
}

variable "allow_unauthenticated" {
  description = "Allow unauthenticated invocations"
  type        = bool
  default     = false
}

variable "admin_api_key_secret_id" {
  description = "Secret Manager secret ID for admin API key"
  type        = string
}

variable "github_actions_api_key_secret_id" {
  description = "Secret Manager secret ID for GitHub Actions API key"
  type        = string
}


resource "google_cloud_run_v2_service" "telemetry_api" {
  name     = "telemetry-api"
  location = var.region

  # Prevent Terraform from resetting traffic on updates
  lifecycle {
    ignore_changes = [
      client,
      client_version,
    ]
  }

  template {
    service_account = var.service_account

    scaling {
      min_instance_count = 0
      max_instance_count = 10
    }

    containers {
      image = var.image

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle = true
      }

      env {
        name  = "GCP_PROJECT"
        value = var.project_id
      }
      env {
        name  = "FIRESTORE_DATABASE"
        value = var.firestore_database
      }
      env {
        name  = "ENVIRONMENT"
        value = "production"
      }
      # Cloud Run service URL for JWT audience
      env {
        name  = "SERVICE_URL"
        value = google_cloud_run_v2_service.telemetry_api.uri
      }
      # Admin API key from Secret Manager
      env {
        name = "ADMIN_API_KEY"
        value_source {
          secret_key_ref {
            secret  = var.admin_api_key_secret_id
            version = "latest"
          }
        }
      }
      # GitHub Actions API key from Secret Manager
      env {
        name = "GITHUB_ACTIONS_API_KEY"
        value_source {
          secret_key_ref {
            secret  = var.github_actions_api_key_secret_id
            version = "latest"
          }
        }
      }

      startup_probe {
        http_get {
          path = "/health"
        }
        initial_delay_seconds = 0
        period_seconds        = 10
        failure_threshold     = 3
        timeout_seconds       = 3
      }

      liveness_probe {
        http_get {
          path = "/health"
        }
        period_seconds    = 30
        failure_threshold = 3
        timeout_seconds   = 3
      }

      ports {
        container_port = 8080
      }
    }

    timeout                          = "300s"
    max_instance_request_concurrency = 80
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  count = var.allow_unauthenticated ? 1 : 0

  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.telemetry_api.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "service_url" {
  description = "URL of the Cloud Run service"
  value       = google_cloud_run_v2_service.telemetry_api.uri
}

output "service_name" {
  description = "Name of the Cloud Run service"
  value       = google_cloud_run_v2_service.telemetry_api.name
}
