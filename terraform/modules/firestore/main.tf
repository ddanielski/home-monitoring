variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
}

# Map regions to Firestore locations
# Firestore has specific location options
locals {
  firestore_location_map = {
    "us-west1"     = "us-west1"
    "us-central1"  = "nam5"
    "us-east1"     = "nam5"
    "europe-west1" = "eur3"
    "asia-east1"   = "asia-east1"
  }
  firestore_location = lookup(local.firestore_location_map, var.region, "nam5")
}

# Firestore database in Native mode
resource "google_firestore_database" "main" {
  project     = var.project_id
  name        = "(default)"
  location_id = local.firestore_location
  type        = "FIRESTORE_NATIVE"

}

output "database_name" {
  description = "Firestore database name"
  value       = google_firestore_database.main.name
}

output "database_location" {
  description = "Firestore database location"
  value       = google_firestore_database.main.location_id
}
