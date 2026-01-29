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
  description = "List of user emails allowed to access the admin API key secret"
  type        = list(string)
  default     = []
}

variable "github_repository" {
  description = "GitHub repository in format 'owner/repo' for Workload Identity Federation (e.g., 'username/repo')"
  type        = string
  default     = ""
}
