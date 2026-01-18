variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region for resources"
  type        = string
  default     = "us-west1"
}

variable "environment" {
  description = "Environment name (dev, prod)"
  type        = string
  default     = "dev"
}

variable "image_tag" {
  description = "Container image tag to deploy"
  type        = string
  default     = "latest"
}

variable "allow_unauthenticated" {
  description = "Allow unauthenticated access to Cloud Run"
  type        = bool
  default     = false
}

variable "repository_id" {
  description = "Artifact Registry repository ID"
  type        = string
  default     = "home-monitoring"
}

variable "provisioner_users" {
  description = "List of user emails allowed to impersonate the provisioner service account"
  type        = list(string)
  default     = []
}

variable "cloud_run_service_name" {
  description = "Cloud Run service name (set after first deploy to enable IAM bindings)"
  type        = string
  default     = ""
}

variable "service_url" {
  description = "Cloud Run service URL (set after first deploy for IAM token validation)"
  type        = string
  default     = ""
}

